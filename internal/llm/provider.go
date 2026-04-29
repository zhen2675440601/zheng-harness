package llm

import (
	"context"
	"fmt"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
)

// Request 是与 provider 无关的推理请求结构。
type Request struct {
	SystemPrompt string
	Input        string
}

// Response 是返回给运行时调用方的标准化 provider 输出。
type Response struct {
	Model      string
	Output     string
	StopReason string
}

// Provider 将 provider SDK 的细节与 domain/runtime 包隔离开。
type Provider interface {
	Name() string
	Model() string
	Generate(ctx context.Context, request Request) (Response, error)
	Stream(ctx context.Context, request Request, emit func(domain.StreamingEvent) error) error
}

// ProviderConfig 是 LLM 适配器所需的精简配置依赖。
type ProviderConfig interface {
	GetModel() string
	GetProvider() string
	GetProviderType() string
	GetAPIKey() string
	GetBaseURL() string
}

// NewProvider 根据配置选择一个隐藏 SDK 细节的 provider 适配器。
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.GetProviderType() {
	case config.ProviderOpenAI:
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return NewOpenAIProvider(cfg.GetAPIKey(), baseURL, cfg.GetModel()), nil
	case config.ProviderAnthropic:
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		return NewAnthropicProvider(cfg.GetAPIKey(), baseURL, cfg.GetModel()), nil
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
