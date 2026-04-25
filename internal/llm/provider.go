package llm

import (
	"context"
	"fmt"

	"zheng-harness/internal/config"
)

// Request is the provider-agnostic inference request shape.
type Request struct {
	SystemPrompt string
	Input        string
}

// Response is the normalized provider output returned to runtime callers.
type Response struct {
	Model   string
	Output  string
	StopReason string
}

// Provider hides provider SDK details from domain/runtime packages.
type Provider interface {
	Name() string
	Model() string
	Generate(ctx context.Context, request Request) (Response, error)
}

// ProviderConfig is the narrow config dependency required by LLM adapters.
type ProviderConfig interface {
	GetModel() string
	GetProvider() string
	GetProviderType() string
	GetAPIKey() string
	GetBaseURL() string
}

// NewProvider selects an SDK-hiding provider adapter from config.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.GetProviderType() {
	case config.ProviderOpenAI:
		return NewOpenAIProvider(cfg.GetModel()), nil
	case config.ProviderAnthropic:
		return NewAnthropicProvider(cfg.GetModel()), nil
	case config.ProviderDashScope:
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1"
		}
		return NewDashScopeProvider(cfg.GetModel(), baseURL, cfg.GetAPIKey()), nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", cfg.GetProviderType())
	}
}
