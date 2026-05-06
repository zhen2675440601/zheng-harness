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
	openAIDefaultTimeout = 30 * time.Second
	openAIDefaultBaseURL = "https://api.openai.com/v1"
)

// OpenAIProvider 基于 OpenAI 兼容的 chat completion 端点实现 Provider 契约。
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	maxRetries int
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
}

type openAIChatCompletionResponse struct {
	Model   string               `json:"model"`
	Choices []openAIChoice       `json:"choices"`
	Error   *openAIErrorEnvelope `json:"error,omitempty"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	Delta        openAIMessage `json:"delta"`
	FinishReason string        `json:"finish_reason"`
}

type openAIErrorEnvelope struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewOpenAIProvider 构造 OpenAI 适配器且不暴露 SDK 细节。
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalizedBaseURL == "" {
		normalizedBaseURL = openAIDefaultBaseURL
	}

	return &OpenAIProvider{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: normalizedBaseURL,
		model:   strings.TrimSpace(model),
		httpClient: &http.Client{
			Timeout: openAIDefaultTimeout,
		},
		maxRetries: 2,
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Model() string {
	return p.model
}

func (p *OpenAIProvider) Generate(ctx context.Context, request Request) (Response, error) {
	if p.model == "" {
		return Response{}, errors.New("openai model must not be empty")
	}
	if p.baseURL == "" {
		return Response{}, errors.New("openai base URL must not be empty")
	}
	if p.apiKey == "" {
		return Response{}, errors.New("openai API key must not be empty")
	}

	payload := openAIChatCompletionRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "system", Content: request.SystemPrompt},
			{Role: "user", Content: request.Input},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal openai request: %w", err)
	}

	endpoint := p.baseURL + "/chat/completions"

	for attempt := 0; ; attempt++ {
		apiResponse, statusCode, err := p.send(ctx, endpoint, body, false)
		if err != nil {
			if attempt < p.maxRetries {
				if waitErr := openAIBackoffWait(ctx, attempt); waitErr != nil {
					return Response{}, fmt.Errorf("openai transport failed: %w", waitErr)
				}
				continue
			}
			return Response{}, fmt.Errorf("openai transport failed: %w", err)
		}

		if statusCode == http.StatusUnauthorized {
			return Response{}, fmt.Errorf("openai authentication failed: %s", openAIErrorMessage(apiResponse))
		}

		if statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError {
			if attempt < p.maxRetries {
				if waitErr := openAIBackoffWait(ctx, attempt); waitErr != nil {
					return Response{}, fmt.Errorf("openai transport failed: %w", waitErr)
				}
				continue
			}
			return Response{}, fmt.Errorf("openai request failed with status %d: %s", statusCode, openAIErrorMessage(apiResponse))
		}

		if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
			return Response{}, fmt.Errorf("openai request failed with status %d: %s", statusCode, openAIErrorMessage(apiResponse))
		}

		if len(apiResponse.Choices) == 0 {
			return Response{}, errors.New("openai malformed response: choices must not be empty")
		}

		output := strings.TrimSpace(apiResponse.Choices[0].Message.Content)
		if output == "" {
			return Response{}, errors.New("openai malformed response: message content must not be empty")
		}

		model := apiResponse.Model
		if model == "" {
			model = p.model
		}

		return Response{
			Model:      model,
			Output:     output,
			StopReason: apiResponse.Choices[0].FinishReason,
		}, nil
	}
}

func (p *OpenAIProvider) Stream(ctx context.Context, request Request, emit func(domain.StreamingEvent) error) error {
	if p.model == "" {
		return errors.New("openai model must not be empty")
	}
	if p.baseURL == "" {
		return errors.New("openai base URL must not be empty")
	}
	if p.apiKey == "" {
		return errors.New("openai API key must not be empty")
	}

	payload := openAIChatCompletionRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "system", Content: request.SystemPrompt},
			{Role: "user", Content: request.Input},
		},
		Stream: true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal openai stream request: %w", err)
	}

	endpoint := p.baseURL + "/chat/completions"

	for attempt := 0; ; attempt++ {
		httpResponse, err := p.sendStreamRequest(ctx, endpoint, body)
		if err != nil {
			if attempt < p.maxRetries {
				if waitErr := openAIBackoffWait(ctx, attempt); waitErr != nil {
					return fmt.Errorf("openai transport failed: %w", waitErr)
				}
				continue
			}
			return fmt.Errorf("openai transport failed: %w", err)
		}

		streamErr := p.handleStreamResponse(ctx, httpResponse, emit)
		httpResponse.Body.Close()
		if streamErr == nil {
			return nil
		}

		if retryable, statusCode, message := openAIStreamRetryDecision(streamErr); retryable {
			if attempt < p.maxRetries {
				if waitErr := openAIBackoffWait(ctx, attempt); waitErr != nil {
					return fmt.Errorf("openai transport failed: %w", waitErr)
				}
				continue
			}
			return fmt.Errorf("openai stream request failed with status %d: %s", statusCode, message)
		}

		return streamErr
	}
}

func (p *OpenAIProvider) send(ctx context.Context, endpoint string, body []byte, stream bool) (openAIChatCompletionResponse, int, error) {
	httpResponse, err := p.sendRequest(ctx, endpoint, body, stream)
	if err != nil {
		return openAIChatCompletionResponse{}, 0, err
	}
	defer httpResponse.Body.Close()
	return openAIReadResponse(httpResponse, stream)
}

func (p *OpenAIProvider) sendStreamRequest(ctx context.Context, endpoint string, body []byte) (*http.Response, error) {
	return p.sendRequest(ctx, endpoint, body, true)
}

func (p *OpenAIProvider) sendRequest(ctx context.Context, endpoint string, body []byte, stream bool) (*http.Response, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create openai request: %w", err)
	}

	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	if stream {
		httpRequest.Header.Set("Accept", "text/event-stream")
	} else {
		httpRequest.Header.Set("Accept", "application/json")
	}

	return p.httpClient.Do(httpRequest)
}

func (p *OpenAIProvider) handleStreamResponse(ctx context.Context, httpResponse *http.Response, emit func(domain.StreamingEvent) error) error {
	if httpResponse.StatusCode == http.StatusUnauthorized || httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode >= http.StatusInternalServerError || httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		apiResponse, _, err := openAIReadResponse(httpResponse, true)
		if err != nil {
			return err
		}

		switch {
		case httpResponse.StatusCode == http.StatusUnauthorized:
			return fmt.Errorf("openai authentication failed: %s", openAIErrorMessage(apiResponse))
		case httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode >= http.StatusInternalServerError:
			return &openAIStreamHTTPError{statusCode: httpResponse.StatusCode, message: openAIErrorMessage(apiResponse)}
		default:
			return fmt.Errorf("openai stream request failed with status %d: %s", httpResponse.StatusCode, openAIErrorMessage(apiResponse))
		}
	}

	parserErr := ParseSSE(ctx, httpResponse.Body, func(chunk string) error {
		var apiResponse openAIChatCompletionResponse
		if err := json.Unmarshal([]byte(chunk), &apiResponse); err != nil {
			return fmt.Errorf("openai malformed stream response: %w", err)
		}

		for _, choice := range apiResponse.Choices {
			content := choice.Delta.Content
			if strings.TrimSpace(content) == "" {
				continue
			}

			event, err := domain.TokenDelta(0, content)
			if err != nil {
				return fmt.Errorf("create openai token event: %w", err)
			}
			if err := emit(*event); err != nil {
				return err
			}
		}
		return nil
	})
	if parserErr != nil {
		return parserErr
	}

	completeEvent, err := domain.SessionComplete("", "success")
	if err != nil {
		return fmt.Errorf("create openai session complete event: %w", err)
	}
	if err := emit(*completeEvent); err != nil {
		return err
	}

	return nil
}

func openAIReadResponse(httpResponse *http.Response, stream bool) (openAIChatCompletionResponse, int, error) {
	responseBody, readErr := io.ReadAll(httpResponse.Body)
	if readErr != nil {
		if stream {
			return openAIChatCompletionResponse{}, httpResponse.StatusCode, fmt.Errorf("read openai stream response: %w", readErr)
		}
		return openAIChatCompletionResponse{}, httpResponse.StatusCode, fmt.Errorf("read openai response: %w", readErr)
	}

	var apiResponse openAIChatCompletionResponse
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
			if stream {
				return openAIChatCompletionResponse{}, httpResponse.StatusCode, fmt.Errorf("openai malformed stream response: %w", err)
			}
			return openAIChatCompletionResponse{}, httpResponse.StatusCode, fmt.Errorf("openai malformed response: %w", err)
		}
	}

	return apiResponse, httpResponse.StatusCode, nil
}

type openAIStreamHTTPError struct {
	statusCode int
	message    string
}

func (e *openAIStreamHTTPError) Error() string {
	return fmt.Sprintf("openai stream retryable status %d: %s", e.statusCode, e.message)
}

func openAIStreamRetryDecision(err error) (bool, int, string) {
	httpErr, ok := err.(*openAIStreamHTTPError)
	if ok {
		return true, httpErr.statusCode, httpErr.message
	}
	return false, 0, ""
}

func openAIBackoffWait(ctx context.Context, attempt int) error {
	backoff := time.Second << attempt
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func openAIErrorMessage(response openAIChatCompletionResponse) string {
	if response.Error == nil {
		return "request failed"
	}

	message := strings.TrimSpace(response.Error.Message)
	if message != "" {
		return message
	}

	typeName := strings.TrimSpace(response.Error.Type)
	if typeName != "" {
		return typeName
	}

	return "request failed"
}
