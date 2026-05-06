package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

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

// ProviderPluginContract 扩展运行时可见的 provider 边界，
// 为 host/plugin 注册表补充稳定的逻辑 provider ID。
type ProviderPluginContract interface {
	Provider
	ProviderID() string
	Metadata() domain.PluginMetadata
}

// ProviderFactory 通过统一注册缝隙构造一个 provider 实例。
type ProviderFactory func(ProviderConfig) (ProviderPluginContract, error)

// ProviderResolver 定义 host-owned provider 选择缝隙。
type ProviderResolver interface {
	Resolve(id string, cfg ProviderConfig) (Provider, error)
}

// ProviderConfig 是 LLM 适配器所需的精简配置依赖。
type ProviderConfig interface {
	GetModel() string
	GetProvider() string
	GetProviderType() string
	GetAPIKey() string
	GetBaseURL() string
}

var (
	errUnsupportedProviderType = errors.New("unsupported provider type")
	defaultProviderResolverMu sync.RWMutex
	defaultProviderResolver   ProviderResolver = newBuiltinProviderRegistry()
)

// DefaultProviderResolver 返回当前生效的 provider 注册表缝隙。
func DefaultProviderResolver() ProviderResolver {
	defaultProviderResolverMu.RLock()
	defer defaultProviderResolverMu.RUnlock()
	return defaultProviderResolver
}

// SetDefaultProviderResolver 为 host/plugin 集成替换默认 provider 注册表。
func SetDefaultProviderResolver(resolver ProviderResolver) {
	defaultProviderResolverMu.Lock()
	defer defaultProviderResolverMu.Unlock()
	if resolver == nil {
		defaultProviderResolver = newBuiltinProviderRegistry()
		return
	}
	defaultProviderResolver = resolver
}

// NewProvider 根据配置选择一个隐藏 SDK 细节的 provider 适配器。
func NewProvider(cfg ProviderConfig) (Provider, error) {
	selectedID := strings.TrimSpace(cfg.GetProviderType())
	return DefaultProviderResolver().Resolve(selectedID, cfg)
}

// AdaptProviderPlugin 将现有内置 provider 包装成 provider plugin contract。
func AdaptProviderPlugin(id string, provider Provider) ProviderPluginContract {
	trimmedID := strings.TrimSpace(id)
	return providerPluginAdapter{
		id:       trimmedID,
		provider: provider,
		metadata: domain.PluginMetadata{
			Family:                domain.PluginFamilyProvider,
			LogicalID:             trimmedID,
			DisplayName:           provider.Name(),
			ContractVersion:       "1.0.0",
			ImplementationVersion: "1.0.0",
			ExecutionMode:         domain.PluginExecutionModeNative,
			SourcePath:            "builtin://llm/" + trimmedID,
		},
	}
}

type providerPluginAdapter struct {
	id       string
	provider Provider
	metadata domain.PluginMetadata
}

func (a providerPluginAdapter) ProviderID() string {
	return a.id
}

func (a providerPluginAdapter) Name() string {
	return a.provider.Name()
}

func (a providerPluginAdapter) Model() string {
	return a.provider.Model()
}

func (a providerPluginAdapter) Generate(ctx context.Context, request Request) (Response, error) {
	return a.provider.Generate(ctx, request)
}

func (a providerPluginAdapter) Stream(ctx context.Context, request Request, emit func(domain.StreamingEvent) error) error {
	return a.provider.Stream(ctx, request, emit)
}

func (a providerPluginAdapter) Metadata() domain.PluginMetadata {
	return a.metadata.Normalize()
}

type builtinProviderRegistry struct {
	defaultID string
	entries   map[string]ProviderFactory
}

func newBuiltinProviderRegistry() ProviderResolver {
	r := &builtinProviderRegistry{
		defaultID: config.ProviderOpenAI,
		entries:   make(map[string]ProviderFactory),
	}
	r.mustRegister(config.ProviderOpenAI, func(cfg ProviderConfig) (ProviderPluginContract, error) {
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return AdaptProviderPlugin(config.ProviderOpenAI, NewOpenAIProvider(cfg.GetAPIKey(), baseURL, cfg.GetModel())), nil
	})
	r.mustRegister(config.ProviderAnthropic, func(cfg ProviderConfig) (ProviderPluginContract, error) {
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		return AdaptProviderPlugin(config.ProviderAnthropic, NewAnthropicProvider(cfg.GetAPIKey(), baseURL, cfg.GetModel())), nil
	})
	r.mustRegister(config.ProviderDashScope, func(cfg ProviderConfig) (ProviderPluginContract, error) {
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1"
		}
		return AdaptProviderPlugin(config.ProviderDashScope, NewDashScopeProvider(cfg.GetModel(), baseURL, cfg.GetAPIKey())), nil
	})
	return r
}

func (r *builtinProviderRegistry) Resolve(id string, cfg ProviderConfig) (Provider, error) {
	if r == nil {
		return nil, fmt.Errorf("unsupported provider type %q", strings.TrimSpace(id))
	}
	selectedID := strings.TrimSpace(id)
	if selectedID == "" {
		selectedID = r.defaultID
	}
	factory, ok := r.entries[selectedID]
	if !ok {
		return nil, fmt.Errorf("%w %q", errUnsupportedProviderType, selectedID)
	}
	provider, err := factory(cfg)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func (r *builtinProviderRegistry) mustRegister(id string, factory ProviderFactory) {
	r.entries[strings.TrimSpace(id)] = factory
}
