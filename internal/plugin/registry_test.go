package plugin

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	"zheng-harness/internal/verify"
)

func TestRegistryBuiltinsAvailableWithoutPlugins(t *testing.T) {
	t.Parallel()

	manager := NewManager("")
	if manager.Providers == nil || manager.Verifiers == nil || manager.AgentStrategies == nil {
		t.Fatal("expected built-in family registries to be initialized")
	}

	providerCfg := stubProviderConfig{providerType: config.ProviderOpenAI, model: "gpt-4.1-mini", apiKey: "secret"}
	provider, err := manager.ResolveProvider("", providerCfg)
	if err != nil {
		t.Fatalf("ResolveProvider() error = %v", err)
	}
	if provider.Name() != config.ProviderOpenAI {
		t.Fatalf("provider.Name() = %q, want %q", provider.Name(), config.ProviderOpenAI)
	}

	verifierStrategy, err := manager.ResolveVerifier("", "standard", &stubToolExecutor{})
	if err != nil {
		t.Fatalf("ResolveVerifier() error = %v", err)
	}
	if _, ok := verifierStrategy.(*verify.CommandVerifier); !ok {
		t.Fatalf("default verifier = %T, want *verify.CommandVerifier", verifierStrategy)
	}
	if got := manager.Verifiers.DefaultID(); got != verify.PolicyCommandBacked {
		t.Fatalf("Verifiers.DefaultID() = %q, want %q", got, verify.PolicyCommandBacked)
	}

	strategy, err := manager.ResolveAgentStrategy("")
	if err != nil {
		t.Fatalf("ResolveAgentStrategy() error = %v", err)
	}
	if strategy.StrategyID() != BuiltinAgentStrategyHostDefault {
		t.Fatalf("strategy.StrategyID() = %q, want %q", strategy.StrategyID(), BuiltinAgentStrategyHostDefault)
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	providers := NewProviderRegistry()
	err := providers.Register(config.ProviderOpenAI, RegistrationMetadata{DisplayName: "plugin openai", Source: RegistrationSourcePlugin, Path: "/plugins/openai"}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
		return llm.AdaptProviderPlugin(config.ProviderOpenAI, llm.NewOpenAIProvider(cfg.GetAPIKey(), cfg.GetBaseURL(), cfg.GetModel())), nil
	})
	if !errors.Is(err, ErrDuplicateRegistryID) {
		t.Fatalf("provider Register() error = %v, want %v", err, ErrDuplicateRegistryID)
	}
	if !strings.Contains(err.Error(), "builtin") || !strings.Contains(err.Error(), "plugin") {
		t.Fatalf("provider duplicate error = %q, want builtin/plugin detail", err)
	}

	verifiers := NewVerifierRegistry()
	err = verifiers.Register(verify.PolicyCommandBacked, RegistrationMetadata{DisplayName: "plugin command", Source: RegistrationSourcePlugin, Path: "/plugins/verify"}, func(mode string, executor domain.ToolExecutor) domain.Verifier {
		return verify.NewCommandVerifier(executor)
	})
	if !errors.Is(err, ErrDuplicateRegistryID) {
		t.Fatalf("verifier Register() error = %v, want %v", err, ErrDuplicateRegistryID)
	}

	err = verifiers.Register("plugin-defined-policy", RegistrationMetadata{DisplayName: "plugin custom policy", Source: RegistrationSourcePlugin, Path: "/plugins/verify"}, func(mode string, executor domain.ToolExecutor) domain.Verifier {
		return verify.NewCommandVerifier(executor)
	})
	if !errors.Is(err, verify.ErrUnauthorizedVerificationPolicy) {
		t.Fatalf("verifier Register(custom) error = %v, want %v", err, verify.ErrUnauthorizedVerificationPolicy)
	}

	agents := NewAgentStrategyRegistry()
	err = agents.Register(BuiltinAgentStrategyHostDefault, RegistrationMetadata{DisplayName: "plugin host default", Source: RegistrationSourcePlugin, Path: "/plugins/agent"}, func() (AgentStrategy, error) {
		return staticAgentStrategy{id: "plugin-host-default"}, nil
	})
	if !errors.Is(err, ErrDuplicateRegistryID) {
		t.Fatalf("agent Register() error = %v, want %v", err, ErrDuplicateRegistryID)
	}
}

func TestRegistryFamilySeparation(t *testing.T) {
	t.Parallel()

	providers := NewProviderRegistry()
	agents := NewAgentStrategyRegistry()

	const sharedID = "shared-id"
	if err := providers.Register(sharedID, RegistrationMetadata{DisplayName: "provider plugin", Source: RegistrationSourcePlugin}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
		return llm.AdaptProviderPlugin(sharedID, stubProvider{name: sharedID}), nil
	}); err != nil {
		t.Fatalf("providers.Register() error = %v", err)
	}
	if err := agents.Register(sharedID, RegistrationMetadata{DisplayName: "agent plugin", Source: RegistrationSourcePlugin}, func() (AgentStrategy, error) {
		return staticAgentStrategy{id: sharedID}, nil
	}); err != nil {
		t.Fatalf("agents.Register() error = %v", err)
	}

	if _, ok := providers.Get(sharedID); !ok {
		t.Fatal("provider registry missing shared id")
	}
	if _, ok := agents.Get(sharedID); !ok {
		t.Fatal("agent strategy registry missing shared id")
	}
}

func TestRegistryResolveUsesExplicitSelectionBeforeDefault(t *testing.T) {
	t.Parallel()

	providers := NewProviderRegistry()
	if err := providers.Register("custom-provider", RegistrationMetadata{DisplayName: "custom provider", Source: RegistrationSourcePlugin}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
		return llm.AdaptProviderPlugin("custom-provider", stubProvider{name: "custom-provider"}), nil
	}); err != nil {
		t.Fatalf("providers.Register() error = %v", err)
	}

	provider, err := providers.Resolve("custom-provider", stubProviderConfig{providerType: config.ProviderOpenAI, model: "gpt"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if provider.Name() != "custom-provider" {
		t.Fatalf("provider.Name() = %q, want %q", provider.Name(), "custom-provider")
	}

	defaultProvider, err := providers.Resolve("", stubProviderConfig{providerType: config.ProviderOpenAI, model: "gpt", apiKey: "secret"})
	if err != nil {
		t.Fatalf("Resolve(default) error = %v", err)
	}
	if defaultProvider.Name() != config.ProviderOpenAI {
		t.Fatalf("default provider.Name() = %q, want %q", defaultProvider.Name(), config.ProviderOpenAI)
	}
}

func TestVerifierRegistryRegisterPluginContractRejectsContractMismatch(t *testing.T) {
	t.Parallel()

	verifiers := NewVerifierRegistry()
	err := verifiers.RegisterPluginContract(stubPluginVerifierContract{
		metadata: domain.PluginMetadata{
			Family:          domain.PluginFamilyVerifier,
			LogicalID:       "evidence-plugin",
			DisplayName:     "Evidence Plugin",
			ContractVersion: "0.9.0",
			ExecutionMode:   domain.PluginExecutionModeExternal,
			SourcePath:      "/plugins/evidence-plugin",
		},
		policyID: verify.PolicyEvidenceBased,
	})
	if !errors.Is(err, ErrVerifierContractVersionMismatch) {
		t.Fatalf("RegisterPluginContract() error = %v, want %v", err, ErrVerifierContractVersionMismatch)
	}
}

func TestVerifierRegistryRegisterPluginContractAllowsPredeclaredPolicy(t *testing.T) {
	t.Parallel()

	verifiers := NewVerifierRegistry()
	err := verifiers.RegisterPluginContract(stubPluginVerifierContract{
		metadata: domain.PluginMetadata{
			Family:          domain.PluginFamilyVerifier,
			LogicalID:       "evidence-plugin",
			DisplayName:     "Evidence Plugin",
			ContractVersion: verify.VerifierPluginContractVersion,
			ExecutionMode:   domain.PluginExecutionModeExternal,
			SourcePath:      "/plugins/evidence-plugin",
		},
		policyID: verify.PolicyEvidenceBased,
		verifier: verify.ResearchVerifier{},
	})
	if err != nil {
		t.Fatalf("RegisterPluginContract() error = %v", err)
	}
	resolved, err := verifiers.Resolve(verify.PolicyEvidenceBased, "standard", &stubToolExecutor{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved == nil {
		t.Fatal("Resolve() returned nil verifier")
	}
}

func TestRegistryConcurrentRegistrationAndLookup(t *testing.T) {
	t.Parallel()

	providers := &ProviderRegistry{entries: make(map[string]ProviderDescriptor), defaultID: "default"}

	const total = 24
	var wg sync.WaitGroup
	wg.Add(total)

	for i := range total {
		id := makeProviderID(i)
		go func() {
			defer wg.Done()
			if err := providers.Register(id, RegistrationMetadata{DisplayName: id, Source: RegistrationSourcePlugin}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
				return llm.AdaptProviderPlugin(id, stubProvider{name: id}), nil
			}); err != nil {
				t.Errorf("Register(%q) error = %v", id, err)
				return
			}
			if descriptor, ok := providers.Get(id); !ok || descriptor.ID != id {
				t.Errorf("Get(%q) = (%+v, %v), want registered descriptor", id, descriptor, ok)
			}
		}()
	}
	wg.Wait()

	for i := range total {
		id := makeProviderID(i)
		if _, ok := providers.Get(id); !ok {
			t.Fatalf("provider registry missing %q after concurrent registration", id)
		}
	}
}

func TestRegistryLookupUnknownIDFailsDeterministically(t *testing.T) {
	t.Parallel()

	providers := NewProviderRegistry()
	_, err := providers.Resolve("missing-provider", stubProviderConfig{providerType: config.ProviderOpenAI})
	if !errors.Is(err, ErrRegistryEntryNotFound) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrRegistryEntryNotFound)
	}
}

type stubProviderConfig struct {
	model        string
	provider     string
	providerType string
	apiKey       string
	baseURL      string
}

func (c stubProviderConfig) GetModel() string        { return c.model }
func (c stubProviderConfig) GetProvider() string     { return c.provider }
func (c stubProviderConfig) GetProviderType() string { return c.providerType }
func (c stubProviderConfig) GetAPIKey() string       { return c.apiKey }
func (c stubProviderConfig) GetBaseURL() string      { return c.baseURL }

type stubProvider struct {
	name string
}

func (p stubProvider) Name() string  { return p.name }
func (p stubProvider) Model() string { return "stub-model" }
func (p stubProvider) Generate(_ context.Context, _ llm.Request) (llm.Response, error) {
	return llm.Response{Model: p.Model(), Output: "ok"}, nil
}
func (p stubProvider) Stream(_ context.Context, _ llm.Request, _ func(domain.StreamingEvent) error) error {
	return nil
}

type stubToolExecutor struct{}

func (stubToolExecutor) Execute(_ context.Context, _ domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{}, nil
}

type stubPluginVerifierContract struct {
	metadata domain.PluginMetadata
	policyID string
	verifier domain.Verifier
}

func (s stubPluginVerifierContract) Metadata() domain.PluginMetadata { return s.metadata }
func (s stubPluginVerifierContract) PolicyID() string               { return s.policyID }
func (s stubPluginVerifierContract) NewVerifier(_ string, _ domain.ToolExecutor) domain.Verifier {
	if s.verifier != nil {
		return s.verifier
	}
	return verify.ResearchVerifier{}
}

func makeProviderID(i int) string {
	return "provider-" + strconv.Itoa(i)
}
