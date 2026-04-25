package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/llm"
)

func TestValidConfigAndProviderBoundary(t *testing.T) {
	t.Setenv("ZHENG_MODEL", "gpt-4.1")
	t.Setenv("ZHENG_PROVIDER", "backup")
	t.Setenv("ZHENG_MAX_STEPS", "12")
	t.Setenv("ZHENG_STEP_TIMEOUT", "45s")
	t.Setenv("ZHENG_MEMORY_LIMIT_MB", "512")
	t.Setenv("ZHENG_VERIFY_MODE", config.VerifyModeStrict)

	cfg, err := config.Load([]string{"-model", "claude-3-5-sonnet", "-provider", config.ProviderOpenAI})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.GetModel() != "claude-3-5-sonnet" {
		t.Fatalf("model = %q, want claude-3-5-sonnet", cfg.GetModel())
	}
	if cfg.Provider != config.ProviderOpenAI {
		t.Fatalf("provider = %q, want %q", cfg.Provider, config.ProviderOpenAI)
	}
	if cfg.GetProviderType() != config.ProviderOpenAI {
		t.Fatalf("provider type = %q, want %q", cfg.GetProviderType(), config.ProviderOpenAI)
	}
	if cfg.Runtime.MaxSteps != 12 {
		t.Fatalf("max steps = %d, want 12", cfg.Runtime.MaxSteps)
	}
	if cfg.Runtime.StepTimeout != 45*time.Second {
		t.Fatalf("step timeout = %s, want 45s", cfg.Runtime.StepTimeout)
	}
	if cfg.Runtime.MemoryLimitMB != 512 {
		t.Fatalf("memory limit = %d, want 512", cfg.Runtime.MemoryLimitMB)
	}
	if cfg.Runtime.VerifyMode != config.VerifyModeStrict {
		t.Fatalf("verify mode = %q, want %q", cfg.Runtime.VerifyMode, config.VerifyModeStrict)
	}

	provider, err := llm.NewProvider(cfg)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if provider.Name() != config.ProviderOpenAI {
		t.Fatalf("provider name = %q, want %q", provider.Name(), config.ProviderOpenAI)
	}
}

func TestLoadUsesLegacyConfigFileWithFlagOverrides(t *testing.T) {
	// Note: No environment variables set, so config file values take precedence
	// then CLI flags override

	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"provider": "dashscope",
		"model": "file-model",
		"api_key": "file-key",
		"base_url": "https://file.example.com",
		"max_steps": 9,
		"step_timeout": "1m",
		"memory_limit_mb": 768,
		"verify_mode": "strict"
	}`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := config.Load([]string{"-config", configPath, "-model", "cli-model", "-provider", "dashscope", "-verify-mode", config.VerifyModeStandard})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.DefaultProvider != config.ProviderDashScope {
		t.Fatalf("default provider = %q, want %q", cfg.DefaultProvider, config.ProviderDashScope)
	}
	if cfg.Provider != config.ProviderDashScope {
		t.Fatalf("provider = %q, want %q", cfg.Provider, config.ProviderDashScope)
	}
	if cfg.GetModel() != "cli-model" {
		t.Fatalf("model = %q, want cli-model", cfg.GetModel())
	}
	if cfg.GetAPIKey() != "file-key" {
		t.Fatalf("api key = %q, want file-key", cfg.GetAPIKey())
	}
	if cfg.GetBaseURL() != "https://file.example.com" {
		t.Fatalf("base url = %q, want file value", cfg.GetBaseURL())
	}
	if cfg.Runtime.MaxSteps != 9 {
		t.Fatalf("max steps = %d, want 9", cfg.Runtime.MaxSteps)
	}
	if cfg.Runtime.StepTimeout != time.Minute {
		t.Fatalf("step timeout = %s, want 1m", cfg.Runtime.StepTimeout)
	}
	if cfg.Runtime.MemoryLimitMB != 768 {
		t.Fatalf("memory limit = %d, want 768", cfg.Runtime.MemoryLimitMB)
	}
	if cfg.Runtime.VerifyMode != config.VerifyModeStandard {
		t.Fatalf("verify mode = %q, want %q", cfg.Runtime.VerifyMode, config.VerifyModeStandard)
	}
}

func TestLoadUsesMultiProviderConfigAndSwitchesProvider(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "dashscope",
		"providers": {
			"dashscope": {
				"type": "dashscope",
				"model": "qwen3.6-plus",
				"api_key": "dash-key",
				"base_url": "https://dash.example.com"
			},
			"openai": {
				"type": "openai",
				"model": "gpt-4.1-mini",
				"api_key": "openai-key",
				"base_url": "https://api.openai.com/v1"
			}
		},
		"runtime": {
			"max_steps": 6,
			"step_timeout": "40s",
			"memory_limit_mb": 300,
			"verify_mode": "standard"
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := config.Load([]string{"-config", configPath, "-provider", "openai"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.DefaultProvider != "dashscope" {
		t.Fatalf("default provider = %q, want dashscope", cfg.DefaultProvider)
	}
	if cfg.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", cfg.Provider)
	}
	if cfg.GetProviderType() != config.ProviderOpenAI {
		t.Fatalf("provider type = %q, want openai", cfg.GetProviderType())
	}
	if cfg.GetModel() != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want gpt-4.1-mini", cfg.GetModel())
	}
	if cfg.GetAPIKey() != "openai-key" {
		t.Fatalf("api key = %q, want openai-key", cfg.GetAPIKey())
	}
	if cfg.Runtime.MaxSteps != 6 {
		t.Fatalf("max steps = %d, want 6", cfg.Runtime.MaxSteps)
	}
	if cfg.Runtime.StepTimeout != 40*time.Second {
		t.Fatalf("step timeout = %s, want 40s", cfg.Runtime.StepTimeout)
	}

	provider, err := llm.NewProvider(cfg)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if provider.Name() != config.ProviderOpenAI {
		t.Fatalf("provider name = %q, want %q", provider.Name(), config.ProviderOpenAI)
	}
}

func TestLoadUsesDefaultConfigPath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "zheng.json"), []byte(`{
		"default_provider": "dashscope",
		"providers": {
			"dashscope": {
				"type": "dashscope",
				"model": "file-model",
				"api_key": "file-key"
			}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write default config file: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	// Use the same provider that exists in the config file
	t.Setenv("ZHENG_MODEL", "env-model")

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Provider != config.ProviderDashScope {
		t.Fatalf("provider = %q, want %q", cfg.Provider, config.ProviderDashScope)
	}
	if cfg.GetModel() != "env-model" {
		t.Fatalf("model = %q, want env-model", cfg.GetModel())
	}
	if cfg.GetAPIKey() != "file-key" {
		t.Fatalf("api key = %q, want file-key", cfg.GetAPIKey())
	}
}

func TestLoadConfigFlagRequiresExistingFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.json")
	_, err := config.Load([]string{"-config", missingPath})
	if err == nil {
		t.Fatal("expected missing config file to fail")
	}
	if !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("error = %v, want read config file error", err)
	}
}

func TestInvalidConfigFailsFast(t *testing.T) {
	t.Run("invalid provider type", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "zheng.json")
		if err := os.WriteFile(configPath, []byte(`{
			"default_provider": "custom",
			"providers": {
				"custom": {
					"type": "invalid",
					"model": "x"
				}
			}
		}`), 0o600); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		_, err := config.Load([]string{"-config", configPath})
		if err == nil {
			t.Fatal("expected config load to fail with invalid provider type")
		}
		if !strings.Contains(err.Error(), "unsupported provider type") {
			t.Fatalf("error = %v, want unsupported provider type error", err)
		}
	})

	t.Run("zero max steps via flags", func(t *testing.T) {
		_, err := config.Load([]string{"-model", "gpt-4.1", "-provider", config.ProviderOpenAI, "-max-steps", "0"})
		if err == nil {
			t.Fatal("expected flag override validation to fail")
		}
		if !strings.Contains(err.Error(), "max steps") {
			t.Fatalf("error = %v, want max steps validation error", err)
		}
	})

	t.Run("empty model", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "zheng.json")
		if err := os.WriteFile(configPath, []byte(`{
			"default_provider": "openai",
			"providers": {
				"openai": {
					"type": "openai",
					"model": ""
				}
			}
		}`), 0o600); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		_, err := config.Load([]string{"-config", configPath})
		if err == nil {
			t.Fatal("expected config load to fail with empty model")
		}
		if !strings.Contains(err.Error(), "model must not be empty") {
			t.Fatalf("error = %v, want model validation error", err)
		}
	})

	t.Run("missing selected provider", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "zheng.json")
		if err := os.WriteFile(configPath, []byte(`{
			"default_provider": "dashscope",
			"providers": {
				"dashscope": {
					"type": "dashscope",
					"model": "qwen3.6-plus",
					"api_key": "dash-key"
				}
			}
		}`), 0o600); err != nil {
			t.Fatalf("write config file: %v", err)
		}

		_, err := config.Load([]string{"-config", configPath, "-provider", "openai"})
		if err == nil {
			t.Fatal("expected missing selected provider to fail")
		}
		if !strings.Contains(err.Error(), "provider \"openai\" not found") {
			t.Fatalf("error = %v, want provider not found error", err)
		}
	})
}
