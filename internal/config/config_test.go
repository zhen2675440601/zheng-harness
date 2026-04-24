package config_test

import (
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/llm"
)

func TestValidConfigAndProviderBoundary(t *testing.T) {
	t.Setenv("ZHENG_MODEL", "gpt-4.1")
	t.Setenv("ZHENG_PROVIDER", config.ProviderAnthropic)
	t.Setenv("ZHENG_MAX_STEPS", "12")
	t.Setenv("ZHENG_STEP_TIMEOUT", "45s")
	t.Setenv("ZHENG_MEMORY_LIMIT_MB", "512")
	t.Setenv("ZHENG_VERIFY_MODE", config.VerifyModeStrict)

	cfg, err := config.Load([]string{"-model", "claude-3-5-sonnet", "-provider", config.ProviderOpenAI})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Model != "claude-3-5-sonnet" {
		t.Fatalf("model = %q, want claude-3-5-sonnet", cfg.Model)
	}
	if cfg.Provider != config.ProviderOpenAI {
		t.Fatalf("provider = %q, want %q", cfg.Provider, config.ProviderOpenAI)
	}
	if cfg.MaxSteps != 12 {
		t.Fatalf("max steps = %d, want 12", cfg.MaxSteps)
	}
	if cfg.StepTimeout != 45*time.Second {
		t.Fatalf("step timeout = %s, want 45s", cfg.StepTimeout)
	}
	if cfg.MemoryLimitMB != 512 {
		t.Fatalf("memory limit = %d, want 512", cfg.MemoryLimitMB)
	}
	if cfg.VerifyMode != config.VerifyModeStrict {
		t.Fatalf("verify mode = %q, want %q", cfg.VerifyMode, config.VerifyModeStrict)
	}

	provider, err := llm.NewProvider(cfg)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if provider.Name() != config.ProviderOpenAI {
		t.Fatalf("provider name = %q, want %q", provider.Name(), config.ProviderOpenAI)
	}
}

func TestInvalidConfigFailsFast(t *testing.T) {
	t.Setenv("ZHENG_MODEL", "")
	t.Setenv("ZHENG_PROVIDER", "invalid")
	t.Setenv("ZHENG_MAX_STEPS", "0")

	_, err := config.Load(nil)
	if err == nil {
		t.Fatal("expected config load to fail")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("error = %v, want provider validation error", err)
	}

	_, err = config.Load([]string{"-model", "gpt-4.1", "-provider", config.ProviderOpenAI, "-max-steps", "0"})
	if err == nil {
		t.Fatal("expected flag override validation to fail")
	}
	if !strings.Contains(err.Error(), "max steps") {
		t.Fatalf("error = %v, want max steps validation error", err)
	}
}
