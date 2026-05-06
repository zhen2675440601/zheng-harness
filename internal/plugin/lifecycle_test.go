package plugin

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	"zheng-harness/internal/verify"
)

func TestPluginInvalidMetadataFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    string
		wantMsg string
	}{
		{name: "missing contract version", mode: "missing_contract_version", wantMsg: "plugin contract version must not be empty"},
		{name: "missing plugin name", mode: "missing_name", wantMsg: "plugin name must not be empty"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := newEchoPluginLoader(t, tc.mode, nil, 0).Load(context.Background())
			if err == nil {
				t.Fatal("expected Load() error")
			}
			if !errors.Is(err, ErrExternalPluginProtocol) {
				t.Fatalf("Load() error = %v, want %v", err, ErrExternalPluginProtocol)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("Load() error = %v, want message containing %q", err, tc.wantMsg)
			}
		})
	}
}

func TestPluginStartupTimeoutFailsClosed(t *testing.T) {
	t.Parallel()

	loader := newEchoPluginLoader(t, "slow_initialize", nil, 50*time.Millisecond)
	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected Load() timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Load() error = %v, want timeout", err)
	}
}

func TestPluginShutdownFailure(t *testing.T) {
	t.Parallel()

	tool := loadEchoPlugin(t, "shutdown_error", nil, 0)
	err := tool.Close()
	if err == nil {
		t.Fatal("expected Close() error")
	}
	if !strings.Contains(err.Error(), "remote shutdown error") {
		t.Fatalf("Close() error = %v, want shutdown failure details", err)
	}
	if !strings.Contains(err.Error(), "shutdown refused") {
		t.Fatalf("Close() error = %v, want plugin shutdown message", err)
	}
}

func TestUnsupportedNativeMode(t *testing.T) {
	t.Parallel()

	loader := NativeLoader{Path: "plugin.so"}
	_, err := loader.Load(context.Background())
	if runtime.GOOS == "windows" {
		if !errors.Is(err, ErrNativePluginsUnsupported) {
			t.Fatalf("Load() error = %v, want %v", err, ErrNativePluginsUnsupported)
		}
		return
	}
	if err == nil {
		t.Fatal("expected Load() error on missing native artifact")
	}
	if !strings.Contains(err.Error(), `open native plugin "plugin.so"`) {
		t.Fatalf("Load() error = %v, want native load failure details", err)
	}
}

func TestFailClosedSelection(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	providerCfg := stubProviderConfig{providerType: config.ProviderOpenAI, model: "gpt-4.1-mini", apiKey: "secret"}

	if provider, err := manager.ResolveProvider("", providerCfg); err != nil {
		t.Fatalf("ResolveProvider(default) error = %v", err)
	} else if provider.Name() != config.ProviderOpenAI {
		t.Fatalf("ResolveProvider(default).Name() = %q, want %q", provider.Name(), config.ProviderOpenAI)
	}

	if _, err := manager.ResolveProvider("missing-provider", providerCfg); err == nil {
		t.Fatal("expected ResolveProvider() error")
	} else {
		if !errors.Is(err, ErrRegistryEntryNotFound) {
			t.Fatalf("ResolveProvider() error = %v, want %v", err, ErrRegistryEntryNotFound)
		}
		if !strings.Contains(err.Error(), `provider "missing-provider"`) {
			t.Fatalf("ResolveProvider() error = %v, want missing provider detail", err)
		}
	}

	verifier, err := manager.ResolveVerifier("", "standard", &stubToolExecutor{})
	if err != nil {
		t.Fatalf("ResolveVerifier(default) error = %v", err)
	}
	if verifier == nil {
		t.Fatal("ResolveVerifier(default) returned nil verifier")
	}

	if _, err := manager.ResolveVerifier("missing-verifier", "standard", &stubToolExecutor{}); err == nil {
		t.Fatal("expected ResolveVerifier() error")
	} else {
		if !errors.Is(err, ErrRegistryEntryNotFound) {
			t.Fatalf("ResolveVerifier() error = %v, want %v", err, ErrRegistryEntryNotFound)
		}
		if !strings.Contains(err.Error(), `verifier "missing-verifier"`) {
			t.Fatalf("ResolveVerifier() error = %v, want missing verifier detail", err)
		}
	}

	strategy, err := manager.ResolveAgentStrategy("")
	if err != nil {
		t.Fatalf("ResolveAgentStrategy(default) error = %v", err)
	}
	if strategy == nil || strategy.StrategyID() != BuiltinAgentStrategyHostDefault {
		t.Fatalf("ResolveAgentStrategy(default) = %#v, want %q", strategy, BuiltinAgentStrategyHostDefault)
	}

	if _, err := manager.ResolveAgentStrategy("missing-strategy"); err == nil {
		t.Fatal("expected ResolveAgentStrategy() error")
	} else {
		if !errors.Is(err, ErrRegistryEntryNotFound) {
			t.Fatalf("ResolveAgentStrategy() error = %v, want %v", err, ErrRegistryEntryNotFound)
		}
		if !strings.Contains(err.Error(), `agent strategy "missing-strategy"`) {
			t.Fatalf("ResolveAgentStrategy() error = %v, want missing strategy detail", err)
		}
	}
}

func TestCollisionPolicyFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*PluginManager) error
		want string
	}{
		{
			name: "provider collision",
			run: func(manager *PluginManager) error {
			return manager.Providers.Register(config.ProviderOpenAI, RegistrationMetadata{DisplayName: "plugin openai", Source: RegistrationSourcePlugin, Path: "/plugins/openai"}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
				return llm.AdaptProviderPlugin("plugin-openai", stubProvider{name: "plugin-openai"}), nil
			})
		},
			want: "cannot replace",
		},
		{
			name: "verifier collision",
			run: func(manager *PluginManager) error {
				return manager.Verifiers.Register(verify.PolicyCommandBacked, RegistrationMetadata{DisplayName: "plugin verifier", Source: RegistrationSourcePlugin, Path: "/plugins/verifier"}, func(mode string, executor domain.ToolExecutor) domain.Verifier {
					return nil
				})
			},
			want: "cannot replace",
		},
		{
			name: "agent strategy collision",
			run: func(manager *PluginManager) error {
				return manager.AgentStrategies.Register(BuiltinAgentStrategyHostDefault, RegistrationMetadata{DisplayName: "plugin host default", Source: RegistrationSourcePlugin, Path: "/plugins/agent"}, func() (AgentStrategy, error) {
					return staticAgentStrategy{id: "plugin-host-default"}, nil
				})
			},
			want: "cannot replace",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := NewManager(t.TempDir())
			err := tc.run(manager)
			if err == nil {
				t.Fatal("expected collision error")
			}
			if !errors.Is(err, ErrDuplicateRegistryID) {
				t.Fatalf("collision error = %v, want %v", err, ErrDuplicateRegistryID)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("collision error = %v, want message containing %q", err, tc.want)
			}
		})
	}
}
