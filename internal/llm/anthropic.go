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
	if p.model == "" {
		return Response{}, errors.New("anthropic model must not be empty")
	}
	if p.baseURL == "" {
		return Response{}, errors.New("anthropic base URL must not be empty")
	}
	if p.apiKey == "" {
		return Response{}, errors.New("anthropic API key must not be empty")
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
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		apiResponse, statusCode, err := p.send(ctx, endpoint, body)
		if err != nil {
			return Response{}, err
		}

		switch {
		case statusCode == http.StatusUnauthorized:
			return Response{}, fmt.Errorf("anthropic authentication failed (status 401): %s", anthropicErrorMessage(apiResponse))
		case statusCode == http.StatusTooManyRequests || statusCode == 529 || statusCode >= http.StatusInternalServerError:
			if attempt == p.maxRetries {
				if statusCode == 529 {
					return Response{}, fmt.Errorf("anthropic service overloaded, retrying: status %d: %s", statusCode, anthropicErrorMessage(apiResponse))
				}
				return Response{}, fmt.Errorf("anthropic request failed after retries with status %d: %s", statusCode, anthropicErrorMessage(apiResponse))
			}

			if statusCode == 529 {
				lastErr = fmt.Errorf("anthropic service overloaded, retrying")
			} else {
				lastErr = fmt.Errorf("anthropic request failed with status %d, retrying: %s", statusCode, anthropicErrorMessage(apiResponse))
			}

			if err := p.waitBackoff(ctx, attempt); err != nil {
				return Response{}, err
			}
			continue
		case statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices:
			return Response{}, fmt.Errorf("anthropic request failed with status %d: %s", statusCode, anthropicErrorMessage(apiResponse))
		}

		output := anthropicOutputText(apiResponse.Content)
		if output == "" {
			return Response{}, errors.New("anthropic response contained no text output")
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

	if lastErr != nil {
		return Response{}, lastErr
	}

	return Response{}, errors.New("anthropic request failed")
}

func (p *AnthropicProvider) Stream(ctx context.Context, request Request, emit func(domain.StreamingEvent) error) error {
	if p.model == "" {
		return errors.New("anthropic model must not be empty")
	}
	if p.baseURL == "" {
		return errors.New("anthropic base URL must not be empty")
	}
	if p.apiKey == "" {
		return errors.New("anthropic API key must not be empty")
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
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		err := p.sendStream(ctx, endpoint, body, emit)
		if err == nil {
			return nil
		}

		var retryErr *anthropicRetryableStreamError
		if !errors.As(err, &retryErr) {
			return err
		}

		statusCode := retryErr.statusCode
		if attempt == p.maxRetries {
			if statusCode == 529 {
				return fmt.Errorf("anthropic service overloaded, retrying: status %d: %s", statusCode, retryErr.message)
			}
			return fmt.Errorf("anthropic request failed after retries with status %d: %s", statusCode, retryErr.message)
		}

		if statusCode == 529 {
			lastErr = fmt.Errorf("anthropic service overloaded, retrying")
		} else {
			lastErr = fmt.Errorf("anthropic request failed with status %d, retrying: %s", statusCode, retryErr.message)
		}

		if err := p.waitBackoff(ctx, attempt); err != nil {
			return err
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return errors.New("anthropic request failed")
}

func (p *AnthropicProvider) send(ctx context.Context, endpoint string, body []byte) (anthropicGenerateResponse, int, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return anthropicGenerateResponse{}, 0, fmt.Errorf("create anthropic request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-api-key", p.apiKey)
	httpRequest.Header.Set("anthropic-version", p.apiVersion)

	httpResponse, err := p.httpClient.Do(httpRequest)
	if err != nil {
		return anthropicGenerateResponse{}, 0, fmt.Errorf("send anthropic request: %w", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return anthropicGenerateResponse{}, httpResponse.StatusCode, fmt.Errorf("read anthropic response: %w", err)
	}

	var apiResponse anthropicGenerateResponse
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
			return anthropicGenerateResponse{}, httpResponse.StatusCode, fmt.Errorf("decode anthropic response: %w", err)
		}
	}

	return apiResponse, httpResponse.StatusCode, nil
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
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create anthropic stream request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	httpRequest.Header.Set("x-api-key", p.apiKey)
	httpRequest.Header.Set("anthropic-version", p.apiVersion)

	httpResponse, err := p.httpClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("send anthropic stream request: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode == http.StatusUnauthorized || httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode == 529 || httpResponse.StatusCode >= http.StatusInternalServerError || httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		responseBody, err := io.ReadAll(httpResponse.Body)
		if err != nil {
			return fmt.Errorf("read anthropic stream response: %w", err)
		}

		var apiResponse anthropicGenerateResponse
		if len(responseBody) > 0 {
			if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
				return fmt.Errorf("decode anthropic stream response: %w", err)
			}
		}

		switch {
		case httpResponse.StatusCode == http.StatusUnauthorized:
			return fmt.Errorf("anthropic authentication failed (status 401): %s", anthropicErrorMessage(apiResponse))
		case httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode == 529 || httpResponse.StatusCode >= http.StatusInternalServerError:
			return &anthropicRetryableStreamError{statusCode: httpResponse.StatusCode, message: anthropicErrorMessage(apiResponse)}
		default:
			return fmt.Errorf("anthropic request failed with status %d: %s", httpResponse.StatusCode, anthropicErrorMessage(apiResponse))
		}
	}

	if err := ParseSSE(ctx, httpResponse.Body, func(chunk string) error {
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(chunk), &event); err != nil {
			return fmt.Errorf("decode anthropic stream chunk: %w", err)
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

type anthropicRetryableStreamError struct {
	statusCode int
	message    string
}

func (e *anthropicRetryableStreamError) Error() string {
	return fmt.Sprintf("anthropic stream retryable status %d: %s", e.statusCode, e.message)
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
	if len(content) == 0 {
		return ""
	}
	if content[0].Type != "text" {
		return ""
	}
	return strings.TrimSpace(content[0].Text)
}
