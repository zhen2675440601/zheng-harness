package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

const (
	anthropicDefaultTimeout    = 30 * time.Second
	anthropicDefaultAPIVersion = "2023-06-01"
	anthropicDefaultMaxTokens  = 4096
	anthropicDefaultMaxRetries = 2
)

// AnthropicProvider 基于 Anthropic 的 Messages API 实现 Provider 契约。
type AnthropicProvider struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	maxRetries  int
	apiVersion  string
	backoffBase time.Duration
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicGenerateRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicStreamEvent struct {
	Type    string                     `json:"type"`
	Delta   *anthropicTextDelta        `json:"delta,omitempty"`
	Error   *anthropicErrorEnvelope    `json:"error,omitempty"`
	Message *anthropicGenerateResponse `json:"message,omitempty"`
}

type anthropicTextDelta struct {
	Text string `json:"text"`
}

type anthropicErrorEnvelope struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type anthropicGenerateResponse struct {
	Content    []anthropicContent      `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Model      string                  `json:"model"`
	Error      *anthropicErrorEnvelope `json:"error,omitempty"`
}

// NewAnthropicProvider 构造 Anthropic 适配器。
func NewAnthropicProvider(apiKey, baseURL, model string) *AnthropicProvider {
	provider := &AnthropicProvider{
		httpClient:  &http.Client{Timeout: anthropicDefaultTimeout},
		maxRetries:  anthropicDefaultMaxRetries,
		apiVersion:  anthropicDefaultAPIVersion,
		backoffBase: time.Second,
		apiKey:      strings.TrimSpace(apiKey),
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		model:       strings.TrimSpace(model),
	}

	if provider.baseURL == "" {
		provider.baseURL = "https://api.anthropic.com/v1"
	}

	return provider
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Model() string {
	return p.model
}

func (p *AnthropicProvider) Generate(ctx context.Context, request Request) (Response, error) {
	if err := p.validate(); err != nil {
		return Response{}, err
	}

	payload := anthropicGenerateRequest{
		Model:  p.model,
		System: request.SystemPrompt,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: request.Input,
			},
		},
		MaxTokens: anthropicDefaultMaxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	endpoint := p.baseURL + "/messages"
	for attempt := 0; ; attempt++ {
		apiResponse, statusCode, err := p.send(ctx, endpoint, body, false)
		if err != nil {
			var transportErr *anthropicTransportError
			if !errors.As(err, &transportErr) {
				return Response{}, err
			}
			if attempt < p.maxRetries {
				if waitErr := p.waitBackoff(ctx, attempt); waitErr != nil {
					return Response{}, fmt.Errorf("anthropic transport failed: %w", waitErr)
				}
				continue
			}
			return Response{}, fmt.Errorf("anthropic transport failed: %w", err)
		}

		switch {
		case statusCode == http.StatusUnauthorized:
			return Response{}, fmt.Errorf("anthropic authentication failed: %s", anthropicErrorMessage(apiResponse))
		case statusCode == http.StatusTooManyRequests || statusCode == 529 || statusCode >= http.StatusInternalServerError:
			if attempt < p.maxRetries {
				if err := p.waitBackoff(ctx, attempt); err != nil {
					return Response{}, fmt.Errorf("anthropic transport failed: %w", err)
				}
				continue
			}
			return Response{}, fmt.Errorf("anthropic request failed with status %d: %s", statusCode, anthropicErrorMessage(apiResponse))
		case statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices:
			return Response{}, fmt.Errorf("anthropic request failed with status %d: %s", statusCode, anthropicErrorMessage(apiResponse))
		}

		output := anthropicOutputText(apiResponse.Content)
		if output == "" {
			return Response{}, errors.New("anthropic malformed response: content must contain text")
		}

		model := apiResponse.Model
		if model == "" {
			model = p.model
		}

		return Response{
			Model:      model,
			Output:     output,
			StopReason: apiResponse.StopReason,
		}, nil
	}
}

func (p *AnthropicProvider) Stream(ctx context.Context, request Request, emit func(domain.StreamingEvent) error) error {
	if err := p.validate(); err != nil {
		return err
	}

	payload := anthropicGenerateRequest{
		Model:     p.model,
		System:    request.SystemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: request.Input}},
		MaxTokens: anthropicDefaultMaxTokens,
		Stream:    true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal anthropic stream request: %w", err)
	}

	endpoint := p.baseURL + "/messages"
	for attempt := 0; ; attempt++ {
		err := p.sendStream(ctx, endpoint, body, emit)
		if err == nil {
			return nil
		}

		var retryErr *anthropicRetryableStreamError
		if !errors.As(err, &retryErr) {
			var transportErr *anthropicTransportError
			if errors.As(err, &transportErr) {
				return fmt.Errorf("anthropic transport failed: %w", transportErr)
			}
			return err
		}

		statusCode := retryErr.statusCode
		if attempt < p.maxRetries {
			if err := p.waitBackoff(ctx, attempt); err != nil {
				return fmt.Errorf("anthropic transport failed: %w", err)
			}
			continue
		}
		return fmt.Errorf("anthropic stream request failed with status %d: %s", statusCode, retryErr.message)
	}
}

func (p *AnthropicProvider) validate() error {
	if p.model == "" {
		return errors.New("anthropic model must not be empty")
	}
	if p.baseURL == "" {
		return errors.New("anthropic base URL must not be empty")
	}
	if p.apiKey == "" {
		return errors.New("anthropic API key must not be empty")
	}
	return nil
}

func (p *AnthropicProvider) send(ctx context.Context, endpoint string, body []byte, stream bool) (anthropicGenerateResponse, int, error) {
	httpResponse, err := p.sendRequest(ctx, endpoint, body, stream)
	if err != nil {
		return anthropicGenerateResponse{}, 0, err
	}
	defer httpResponse.Body.Close()
	return anthropicReadResponse(httpResponse, stream)
}

func (p *AnthropicProvider) sendRequest(ctx context.Context, endpoint string, body []byte, stream bool) (*http.Response, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	if stream {
		httpRequest.Header.Set("Accept", "text/event-stream")
	} else {
		httpRequest.Header.Set("Accept", "application/json")
	}
	httpRequest.Header.Set("x-api-key", p.apiKey)
	httpRequest.Header.Set("anthropic-version", p.apiVersion)

	httpResponse, err := p.httpClient.Do(httpRequest)
	if err != nil {
		return nil, &anthropicTransportError{err: err}
	}
	return httpResponse, nil
}

func (p *AnthropicProvider) waitBackoff(ctx context.Context, attempt int) error {
	if attempt >= p.maxRetries {
		return nil
	}
	base := p.backoffBase
	if base <= 0 {
		base = time.Second
	}
	backoff := base << attempt
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("anthropic retry canceled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (p *AnthropicProvider) sendStream(ctx context.Context, endpoint string, body []byte, emit func(domain.StreamingEvent) error) error {
	httpResponse, err := p.sendRequest(ctx, endpoint, body, true)
	if err != nil {
		return err
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode == http.StatusUnauthorized || httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode == 529 || httpResponse.StatusCode >= http.StatusInternalServerError || httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		apiResponse, _, err := anthropicReadResponse(httpResponse, true)
		if err != nil {
			return err
		}

		switch {
		case httpResponse.StatusCode == http.StatusUnauthorized:
			return fmt.Errorf("anthropic authentication failed: %s", anthropicErrorMessage(apiResponse))
		case httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode == 529 || httpResponse.StatusCode >= http.StatusInternalServerError:
			return &anthropicRetryableStreamError{statusCode: httpResponse.StatusCode, message: anthropicErrorMessage(apiResponse)}
		default:
			return fmt.Errorf("anthropic stream request failed with status %d: %s", httpResponse.StatusCode, anthropicErrorMessage(apiResponse))
		}
	}

	if err := ParseSSE(ctx, httpResponse.Body, func(chunk string) error {
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(chunk), &event); err != nil {
			return fmt.Errorf("anthropic malformed stream response: %w", err)
		}

		if event.Error != nil {
			return fmt.Errorf("anthropic stream error: %s", strings.TrimSpace(event.Error.Message))
		}

		if event.Type != "content_block_delta" || event.Delta == nil || strings.TrimSpace(event.Delta.Text) == "" {
			return nil
		}

		streamEvent, err := domain.TokenDelta(0, event.Delta.Text)
		if err != nil {
			return fmt.Errorf("create anthropic token event: %w", err)
		}
		return emit(*streamEvent)
	}); err != nil {
		return err
	}

	completeEvent, err := domain.SessionComplete("", "success")
	if err != nil {
		return fmt.Errorf("create anthropic session complete event: %w", err)
	}
	return emit(*completeEvent)
}

func anthropicReadResponse(httpResponse *http.Response, stream bool) (anthropicGenerateResponse, int, error) {
	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		if stream {
			return anthropicGenerateResponse{}, httpResponse.StatusCode, fmt.Errorf("read anthropic stream response: %w", err)
		}
		return anthropicGenerateResponse{}, httpResponse.StatusCode, fmt.Errorf("read anthropic response: %w", err)
	}

	var apiResponse anthropicGenerateResponse
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
			if stream {
				return anthropicGenerateResponse{}, httpResponse.StatusCode, fmt.Errorf("anthropic malformed stream response: %w", err)
			}
			return anthropicGenerateResponse{}, httpResponse.StatusCode, fmt.Errorf("anthropic malformed response: %w", err)
		}
	}

	return apiResponse, httpResponse.StatusCode, nil
}

type anthropicRetryableStreamError struct {
	statusCode int
	message    string
}

type anthropicTransportError struct {
	err error
}

func (e *anthropicRetryableStreamError) Error() string {
	return fmt.Sprintf("anthropic stream retryable status %d: %s", e.statusCode, e.message)
}

func (e *anthropicTransportError) Error() string {
	return e.err.Error()
}

func (e *anthropicTransportError) Unwrap() error {
	return e.err
}

func anthropicErrorMessage(response anthropicGenerateResponse) string {
	if response.Error != nil {
		message := strings.TrimSpace(response.Error.Message)
		if message != "" {
			return message
		}
		if response.Error.Type != "" {
			return response.Error.Type
		}
	}

	return "request failed"
}

func anthropicOutputText(content []anthropicContent) string {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		if item.Type != "text" {
			continue
		}

		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}

		parts = append(parts, text)
	}

	return strings.Join(parts, "\n")
}
