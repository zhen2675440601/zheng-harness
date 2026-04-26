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
)

const (
	openAIDefaultTimeout = 30 * time.Second
	openAIDefaultBaseURL = "https://api.openai.com/v1"
)

// OpenAIProvider implements the Provider contract against OpenAI-compatible
// chat completion endpoints.
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
}

type openAIChatCompletionResponse struct {
	Model   string               `json:"model"`
	Choices []openAIChoice       `json:"choices"`
	Error   *openAIErrorEnvelope `json:"error,omitempty"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIErrorEnvelope struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewOpenAIProvider constructs the OpenAI adapter without leaking SDK details.
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
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return Response{}, fmt.Errorf("create openai request: %w", err)
		}

		httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Accept", "application/json")

		httpResponse, err := p.httpClient.Do(httpRequest)
		if err != nil {
			if attempt < p.maxRetries {
				if waitErr := openAIBackoffWait(ctx, attempt); waitErr != nil {
					return Response{}, fmt.Errorf("send openai request: %w", waitErr)
				}
				continue
			}
			return Response{}, fmt.Errorf("send openai request: %w", err)
		}

		responseBody, readErr := io.ReadAll(httpResponse.Body)
		httpResponse.Body.Close()
		if readErr != nil {
			return Response{}, fmt.Errorf("read openai response: %w", readErr)
		}

		var apiResponse openAIChatCompletionResponse
		if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
			return Response{}, fmt.Errorf("decode openai response: %w", err)
		}

		if httpResponse.StatusCode == http.StatusUnauthorized {
			return Response{}, errors.New("openai authentication failed: check API key")
		}

		if httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode >= http.StatusInternalServerError {
			if attempt < p.maxRetries {
				if waitErr := openAIBackoffWait(ctx, attempt); waitErr != nil {
					return Response{}, fmt.Errorf("retry openai request: %w", waitErr)
				}
				continue
			}
			return Response{}, fmt.Errorf("openai request failed with status %d: %s", httpResponse.StatusCode, openAIErrorMessage(apiResponse))
		}

		if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
			return Response{}, fmt.Errorf("openai request failed with status %d: %s", httpResponse.StatusCode, openAIErrorMessage(apiResponse))
		}

		if len(apiResponse.Choices) == 0 {
			return Response{}, errors.New("openai response contained no choices")
		}

		output := strings.TrimSpace(apiResponse.Choices[0].Message.Content)
		if output == "" {
			return Response{}, errors.New("openai response contained empty message content")
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
