package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderDashScope = "dashscope"

	VerifyModeOff      = "off"
	VerifyModeStandard = "standard"
	VerifyModeStrict   = "strict"
)

// Config contains runtime-selectable process settings.
type Config struct {
	DefaultProvider string
	Providers       map[string]ProviderSettings
	Runtime         RuntimeSettings
	Provider        string
}

type ProviderSettings struct {
	Type    string `json:"type"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

type RuntimeSettings struct {
	MaxSteps      int           `json:"max_steps"`
	StepTimeout   time.Duration `json:"step_timeout"`
	MemoryLimitMB int           `json:"memory_limit_mb"`
	VerifyMode    string        `json:"verify_mode"`
	AllowedCommands []string    `json:"allowed_commands"`
}

type fileConfig struct {
	DefaultProvider *string                       `json:"default_provider"`
	Providers       map[string]providerFileConfig `json:"providers"`
	Runtime         *runtimeFileConfig            `json:"runtime"`

	Model         *string `json:"model"`
	Provider      *string `json:"provider"`
	MaxSteps      *int    `json:"max_steps"`
	StepTimeout   *string `json:"step_timeout"`
	MemoryLimitMB *int    `json:"memory_limit_mb"`
	VerifyMode    *string `json:"verify_mode"`
	APIKey        *string `json:"api_key"`
	BaseURL       *string `json:"base_url"`
}

type providerFileConfig struct {
	Type    *string `json:"type"`
	Model   *string `json:"model"`
	APIKey  *string `json:"api_key"`
	BaseURL *string `json:"base_url"`
}

type runtimeFileConfig struct {
	MaxSteps      *int    `json:"max_steps"`
	StepTimeout   *string `json:"step_timeout"`
	MemoryLimitMB *int    `json:"memory_limit_mb"`
	VerifyMode    *string `json:"verify_mode"`
	AllowedCommands []string `json:"allowed_commands"`
}

// GetModel exposes the selected model through the provider-boundary contract.
func (c Config) GetModel() string {
	return c.selectedProviderSettings().Model
}

// GetProvider exposes the selected provider through the provider-boundary contract.
func (c Config) GetProvider() string {
	return c.Provider
}

// GetProviderType exposes the selected provider type through the provider-boundary contract.
func (c Config) GetProviderType() string {
	return c.selectedProviderSettings().Type
}

// GetAPIKey exposes the API key for LLM providers.
func (c Config) GetAPIKey() string {
	return c.selectedProviderSettings().APIKey
}

// GetBaseURL exposes the base URL for LLM API endpoints.
func (c Config) GetBaseURL() string {
	return c.selectedProviderSettings().BaseURL
}

// Default returns the baseline CLI/runtime configuration.
func Default() Config {
	defaultProvider := ProviderOpenAI
	return Config{
		DefaultProvider: defaultProvider,
		Provider:        defaultProvider,
		Providers: map[string]ProviderSettings{
			defaultProvider: {
				Type:  ProviderOpenAI,
				Model: "gpt-4.1-mini",
			},
		},
		Runtime: RuntimeSettings{
			MaxSteps:      8,
			StepTimeout:   30 * time.Second,
			MemoryLimitMB: 256,
			VerifyMode:    VerifyModeStandard,
		},
	}
}

// Load reads config using precedence: defaults < config file < environment < CLI flags.
func Load(args []string) (Config, error) {
	cfg := Default()

	configPath, configPathRequired, err := resolveConfigPath(args)
	if err != nil {
		return Config{}, err
	}

	// First load config file (defaults < config file)
	if configPath != "" {
		if err := loadFromFile(&cfg, configPath, configPathRequired); err != nil {
			return Config{}, err
		}
	}

	// Then apply environment variables (config file < environment)
	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}

	configFlagDefault := configPath

	fs := flag.NewFlagSet("zheng-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&configFlagDefault, "config", configFlagDefault, "path to JSON config file")
	model := cfg.GetModel()
	provider := cfg.Provider
	maxSteps := cfg.Runtime.MaxSteps
	stepTimeout := cfg.Runtime.StepTimeout
	memoryLimitMB := cfg.Runtime.MemoryLimitMB
	verifyMode := cfg.Runtime.VerifyMode
	apiKey := cfg.GetAPIKey()
	baseURL := cfg.GetBaseURL()

	fs.StringVar(&model, "model", model, "model identifier")
	fs.StringVar(&provider, "provider", provider, "provider identifier")
	fs.IntVar(&maxSteps, "max-steps", maxSteps, "maximum runtime steps")
	fs.DurationVar(&stepTimeout, "step-timeout", stepTimeout, "maximum duration per step")
	fs.IntVar(&memoryLimitMB, "memory-limit-mb", memoryLimitMB, "memory budget in megabytes")
	fs.StringVar(&verifyMode, "verify-mode", verifyMode, "verification mode")
	fs.StringVar(&apiKey, "api-key", apiKey, "API key for LLM provider")
	fs.StringVar(&baseURL, "base-url", baseURL, "base URL for LLM API endpoint")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

provider = strings.ToLower(strings.TrimSpace(provider))
	cfg.Provider = provider
	upsertSelectedProvider(&cfg)
	// Only update provider settings if provider exists in config
	// This prevents creating new providers from CLI args alone
	if _, exists := cfg.Providers[provider]; exists {
		selected := cfg.Providers[provider]
		// Only update model/apiKey/baseURL if CLI flag provided a non-empty value.
		// This preserves values from file/env that are more specific.
		if model != "" {
			selected.Model = strings.TrimSpace(model)
		}
		if apiKey != "" {
			selected.APIKey = strings.TrimSpace(apiKey)
		}
		if baseURL != "" {
			selected.BaseURL = strings.TrimSpace(baseURL)
		}
		if selected.Type == "" {
			selected.Type = inferProviderType(provider)
		}
		cfg.Providers[provider] = selected
	}
	cfg.Runtime.MaxSteps = maxSteps
	cfg.Runtime.StepTimeout = stepTimeout
	cfg.Runtime.MemoryLimitMB = memoryLimitMB
	cfg.Runtime.VerifyMode = strings.ToLower(strings.TrimSpace(verifyMode))
	if strings.TrimSpace(cfg.DefaultProvider) == "" {
		cfg.DefaultProvider = cfg.Provider
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyEnv(cfg *Config) error {
	if value := strings.TrimSpace(os.Getenv("ZHENG_PROVIDER")); value != "" {
		cfg.Provider = strings.ToLower(value)
		upsertSelectedProvider(cfg)
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_MODEL")); value != "" {
		selected := cfg.selectedProviderSettings()
		selected.Model = value
		cfg.Providers[cfg.Provider] = selected
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_MAX_STEPS")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse ZHENG_MAX_STEPS: %w", err)
		}
		cfg.Runtime.MaxSteps = parsed
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_STEP_TIMEOUT")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse ZHENG_STEP_TIMEOUT: %w", err)
		}
		cfg.Runtime.StepTimeout = parsed
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_MEMORY_LIMIT_MB")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse ZHENG_MEMORY_LIMIT_MB: %w", err)
		}
		cfg.Runtime.MemoryLimitMB = parsed
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_VERIFY_MODE")); value != "" {
		cfg.Runtime.VerifyMode = strings.ToLower(value)
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_API_KEY")); value != "" {
		selected := cfg.selectedProviderSettings()
		selected.APIKey = value
		cfg.Providers[cfg.Provider] = selected
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_BASE_URL")); value != "" {
		selected := cfg.selectedProviderSettings()
		selected.BaseURL = value
		cfg.Providers[cfg.Provider] = selected
	}

	return nil
}

func loadFromFile(cfg *Config, path string, required bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	var parsed fileConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}

	if parsed.DefaultProvider != nil {
		cfg.DefaultProvider = strings.ToLower(strings.TrimSpace(*parsed.DefaultProvider))
	}
	if len(parsed.Providers) > 0 {
		providers := make(map[string]ProviderSettings, len(parsed.Providers))
		for name, provider := range parsed.Providers {
			trimmedName := strings.ToLower(strings.TrimSpace(name))
			if trimmedName == "" {
				continue
			}
			providers[trimmedName] = ProviderSettings{
				Type:    normalizeProviderType(provider.Type, trimmedName),
				Model:   trimStringPointer(provider.Model),
				APIKey:  trimStringPointer(provider.APIKey),
				BaseURL: trimStringPointer(provider.BaseURL),
			}
		}
		if len(providers) > 0 {
			cfg.Providers = providers
		}
	}
	if parsed.Runtime != nil {
		if parsed.Runtime.MaxSteps != nil {
			cfg.Runtime.MaxSteps = *parsed.Runtime.MaxSteps
		}
		if parsed.Runtime.StepTimeout != nil {
			value, err := parseDurationField(*parsed.Runtime.StepTimeout, path, "runtime.step_timeout")
			if err != nil {
				return err
			}
			cfg.Runtime.StepTimeout = value
		}
		if parsed.Runtime.MemoryLimitMB != nil {
			cfg.Runtime.MemoryLimitMB = *parsed.Runtime.MemoryLimitMB
		}
		if parsed.Runtime.VerifyMode != nil {
			cfg.Runtime.VerifyMode = strings.ToLower(strings.TrimSpace(*parsed.Runtime.VerifyMode))
		}
		if parsed.Runtime.AllowedCommands != nil {
			cfg.Runtime.AllowedCommands = normalizeCommandList(parsed.Runtime.AllowedCommands)
		}
	}

	if parsed.Provider != nil || parsed.Model != nil || parsed.APIKey != nil || parsed.BaseURL != nil || parsed.MaxSteps != nil || parsed.StepTimeout != nil || parsed.MemoryLimitMB != nil || parsed.VerifyMode != nil {
		legacyProvider := cfg.Provider
		if parsed.Provider != nil {
			legacyProvider = strings.ToLower(strings.TrimSpace(*parsed.Provider))
		}
		if legacyProvider == "" {
			legacyProvider = cfg.DefaultProvider
		}
		if legacyProvider == "" {
			legacyProvider = ProviderOpenAI
		}

		settings := cfg.Providers[legacyProvider]
		if settings.Type == "" {
			settings.Type = inferProviderType(legacyProvider)
		}
		if parsed.Model != nil {
			settings.Model = strings.TrimSpace(*parsed.Model)
		}
		if parsed.APIKey != nil {
			settings.APIKey = strings.TrimSpace(*parsed.APIKey)
		}
		if parsed.BaseURL != nil {
			settings.BaseURL = strings.TrimSpace(*parsed.BaseURL)
		}
		cfg.Providers[legacyProvider] = settings
		cfg.DefaultProvider = legacyProvider
		cfg.Provider = legacyProvider

		if parsed.MaxSteps != nil {
			cfg.Runtime.MaxSteps = *parsed.MaxSteps
		}
		if parsed.StepTimeout != nil {
			value, err := parseDurationField(*parsed.StepTimeout, path, "step_timeout")
			if err != nil {
				return err
			}
			cfg.Runtime.StepTimeout = value
		}
		if parsed.MemoryLimitMB != nil {
			cfg.Runtime.MemoryLimitMB = *parsed.MemoryLimitMB
		}
		if parsed.VerifyMode != nil {
			cfg.Runtime.VerifyMode = strings.ToLower(strings.TrimSpace(*parsed.VerifyMode))
		}
	}

// After loading providers, ensure cfg.Provider is valid
	// If current provider doesn't exist in new providers map, switch to default_provider
	if cfg.Provider != "" && cfg.Providers != nil {
		if _, ok := cfg.Providers[cfg.Provider]; !ok {
			// Current provider not in new providers, switch to default
			cfg.Provider = ""
		}
	}
	if cfg.Provider == "" {
		cfg.Provider = cfg.DefaultProvider
	}
	if cfg.Provider == "" {
		cfg.Provider = firstProviderName(cfg.Providers)
	}
	upsertSelectedProvider(cfg)
	if cfg.DefaultProvider == "" {
		cfg.DefaultProvider = cfg.Provider
	}

	return nil
}

func normalizeCommandList(commands []string) []string {
	if commands == nil {
		return nil
	}
	normalized := make([]string, 0, len(commands))
	for _, command := range commands {
		trimmed := strings.TrimSpace(command)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func resolveConfigPath(args []string) (string, bool, error) {
	path, found, err := extractConfigPath(args)
	if err != nil {
		return "", false, err
	}
	if found {
		resolved, err := expandPath(path)
		if err != nil {
			return "", false, err
		}
		if strings.TrimSpace(resolved) == "" {
			return "", false, errors.New("config path must not be empty")
		}
		return resolved, true, nil
	}

	for _, candidate := range []string{"zheng.json", filepath.Join("~", ".zheng", "config.json")} {
		resolved, err := expandPath(candidate)
		if err != nil {
			return "", false, err
		}
		info, statErr := os.Stat(resolved)
		if statErr == nil && !info.IsDir() {
			return resolved, false, nil
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", false, fmt.Errorf("stat config file %q: %w", resolved, statErr)
		}
	}

	return "", false, nil
}

func extractConfigPath(args []string) (string, bool, error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-config" || arg == "--config":
			if i+1 >= len(args) {
				return "", false, errors.New("flag needs an argument: -config")
			}
			return args[i+1], true, nil
		case strings.HasPrefix(arg, "-config="):
			return strings.TrimPrefix(arg, "-config="), true, nil
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config="), true, nil
		}
	}

	return "", false, nil
}

func expandPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	if trimmed == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return filepath.Join(home, trimmed[2:]), nil
	}
	return trimmed, nil
}

// Validate enforces fast-fail config rules before runtime startup.
func (c Config) Validate() error {
	if strings.TrimSpace(c.DefaultProvider) == "" {
		return errors.New("default provider must not be empty")
	}
	if len(c.Providers) == 0 {
		return errors.New("providers must not be empty")
	}
	if _, ok := c.Providers[c.DefaultProvider]; !ok {
		return fmt.Errorf("default provider %q not found", c.DefaultProvider)
	}
	selected := c.selectedProviderSettings()
	if strings.TrimSpace(c.Provider) == "" {
		return errors.New("provider must not be empty")
	}
	if _, ok := c.Providers[c.Provider]; !ok {
		return fmt.Errorf("provider %q not found", c.Provider)
	}
	// Check provider type first, before model, so invalid provider type errors
	// take precedence over empty model errors.
	switch selected.Type {
	case ProviderOpenAI, ProviderAnthropic:
		// these providers use default stubs for now
	case ProviderDashScope:
		if selected.APIKey == "" {
			return errors.New("dashscope provider requires API key (set ZHENG_API_KEY or --api-key)")
		}
	default:
		return fmt.Errorf("unsupported provider type %q", selected.Type)
	}
	if selected.Model == "" {
		return errors.New("model must not be empty")
	}
	if c.Runtime.MaxSteps <= 0 {
		return errors.New("max steps must be greater than zero")
	}
	if c.Runtime.StepTimeout <= 0 {
		return errors.New("step timeout must be greater than zero")
	}
	if c.Runtime.MemoryLimitMB <= 0 {
		return errors.New("memory limit must be greater than zero")
	}

	switch c.Runtime.VerifyMode {
	case VerifyModeOff, VerifyModeStandard, VerifyModeStrict:
	default:
		return fmt.Errorf("unsupported verify mode %q", c.Runtime.VerifyMode)
	}

	return nil
}

func (c Config) selectedProviderSettings() ProviderSettings {
	if c.Providers == nil {
		return ProviderSettings{}
	}
	return c.Providers[c.Provider]
}

func upsertSelectedProvider(cfg *Config) {
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderSettings)
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = cfg.DefaultProvider
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = ProviderOpenAI
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	cfg.Provider = provider
	// Only update existing provider, don't create new ones
	// Validate will catch missing providers
	if _, exists := cfg.Providers[provider]; exists {
		settings := cfg.Providers[provider]
		if settings.Type == "" {
			settings.Type = inferProviderType(provider)
		}
		cfg.Providers[provider] = settings
	}
}

func inferProviderType(providerName string) string {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	switch providerName {
	case ProviderOpenAI, ProviderAnthropic, ProviderDashScope:
		return providerName
	default:
		return ProviderOpenAI
	}
}

func normalizeProviderType(value *string, providerName string) string {
	trimmed := trimStringPointer(value)
	if trimmed == "" {
		return inferProviderType(providerName)
	}
	return strings.ToLower(trimmed)
}

func trimStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func parseDurationField(raw, path, field string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse %s from config file %q: %w", field, path, err)
	}
	return value, nil
}

func firstProviderName(providers map[string]ProviderSettings) string {
	for name := range providers {
		return name
	}
	return ""
}
