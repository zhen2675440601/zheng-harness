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
	dashScopeDefaultTimeout          = 30 * time.Second
	dashScopeAnthropicVersionHeader = "2023-06-01"
	dashScopeMaxTokens              = 4096
)

// DashScopeProvider implements the Provider contract against DashScope's
// Anthropic-compatible API.
type DashScopeProvider struct {
	model   string
	baseURL string
	apiKey  string
	client  *http.Client
}

type dashScopeMessageRequest struct {
	Role    string                    `json:"role"`
	Content []dashScopeContentRequest `json:"content"`
}

type dashScopeContentRequest struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type dashScopeGenerateRequest struct {
	Model     string                    `json:"model"`
	System    string                    `json:"system,omitempty"`
	Messages  []dashScopeMessageRequest `json:"messages"`
	MaxTokens int                       `json:"max_tokens"`
}

type dashScopeGenerateResponse struct {
	Model      string                     `json:"model"`
	StopReason string                     `json:"stop_reason"`
	Content    []dashScopeContentResponse `json:"content"`
	Error      *dashScopeErrorEnvelope    `json:"error,omitempty"`
}

type dashScopeContentResponse struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type dashScopeErrorEnvelope struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewDashScopeProvider constructs a DashScope adapter using caller-provided
// configuration instead of embedding credentials in exported methods.
func NewDashScopeProvider(model, baseURL, apiKey string) DashScopeProvider {
	return DashScopeProvider{
		model:   strings.TrimSpace(model),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client: &http.Client{
			Timeout: dashScopeDefaultTimeout,
		},
	}
}

func (p DashScopeProvider) Name() string {
	return "dashscope"
}

func (p DashScopeProvider) Model() string {
	return p.model
}

func (p DashScopeProvider) Generate(ctx context.Context, request Request) (Response, error) {
	if p.model == "" {
		return Response{}, errors.New("dashscope model must not be empty")
	}
	if p.baseURL == "" {
		return Response{}, errors.New("dashscope base URL must not be empty")
	}
	if p.apiKey == "" {
		return Response{}, errors.New("dashscope API key must not be empty")
	}

	payload := dashScopeGenerateRequest{
		Model:  p.model,
		System: request.SystemPrompt,
		Messages: []dashScopeMessageRequest{
			{
				Role: "user",
				Content: []dashScopeContentRequest{
					{
						Type: "text",
						Text: request.Input,
					},
				},
			},
		},
		MaxTokens: dashScopeMaxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal dashscope request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create dashscope request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("x-api-key", p.apiKey)
	httpRequest.Header.Set("anthropic-version", dashScopeAnthropicVersionHeader)

	httpResponse, err := p.client.Do(httpRequest)
	if err != nil {
		return Response{}, fmt.Errorf("send dashscope request: %w", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read dashscope response: %w", err)
	}

	var apiResponse dashScopeGenerateResponse
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return Response{}, fmt.Errorf("decode dashscope response: %w", err)
	}

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return Response{}, fmt.Errorf("dashscope request failed with status %d: %s", httpResponse.StatusCode, dashScopeErrorMessage(apiResponse))
	}

	output := dashScopeOutputText(apiResponse.Content)
	if output == "" {
		return Response{}, errors.New("dashscope response contained no text output")
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

func dashScopeErrorMessage(response dashScopeGenerateResponse) string {
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

func dashScopeOutputText(content []dashScopeContentResponse) string {
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
