package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"

	VerifyModeOff      = "off"
	VerifyModeStandard = "standard"
	VerifyModeStrict   = "strict"
)

// Config contains runtime-selectable process settings.
type Config struct {
	Model         string
	Provider      string
	MaxSteps      int
	StepTimeout   time.Duration
	MemoryLimitMB int
	VerifyMode    string
}

// GetModel exposes the selected model through the provider-boundary contract.
func (c Config) GetModel() string {
	return c.Model
}

// GetProvider exposes the selected provider through the provider-boundary contract.
func (c Config) GetProvider() string {
	return c.Provider
}

// Default returns the baseline CLI/runtime configuration.
func Default() Config {
	return Config{
		Model:         "gpt-4.1-mini",
		Provider:      ProviderOpenAI,
		MaxSteps:      8,
		StepTimeout:   30 * time.Second,
		MemoryLimitMB: 256,
		VerifyMode:    VerifyModeStandard,
	}
}

// Load reads config from environment and optional CLI flags.
func Load(args []string) (Config, error) {
	cfg := Default()

	if value := strings.TrimSpace(os.Getenv("ZHENG_MODEL")); value != "" {
		cfg.Model = value
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_PROVIDER")); value != "" {
		cfg.Provider = strings.ToLower(value)
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_MAX_STEPS")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse ZHENG_MAX_STEPS: %w", err)
		}
		cfg.MaxSteps = parsed
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_STEP_TIMEOUT")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse ZHENG_STEP_TIMEOUT: %w", err)
		}
		cfg.StepTimeout = parsed
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_MEMORY_LIMIT_MB")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse ZHENG_MEMORY_LIMIT_MB: %w", err)
		}
		cfg.MemoryLimitMB = parsed
	}
	if value := strings.TrimSpace(os.Getenv("ZHENG_VERIFY_MODE")); value != "" {
		cfg.VerifyMode = strings.ToLower(value)
	}

	fs := flag.NewFlagSet("zheng-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.Model, "model", cfg.Model, "model identifier")
	fs.StringVar(&cfg.Provider, "provider", cfg.Provider, "provider identifier")
	fs.IntVar(&cfg.MaxSteps, "max-steps", cfg.MaxSteps, "maximum runtime steps")
	fs.DurationVar(&cfg.StepTimeout, "step-timeout", cfg.StepTimeout, "maximum duration per step")
	fs.IntVar(&cfg.MemoryLimitMB, "memory-limit-mb", cfg.MemoryLimitMB, "memory budget in megabytes")
	fs.StringVar(&cfg.VerifyMode, "verify-mode", cfg.VerifyMode, "verification mode")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	cfg.VerifyMode = strings.ToLower(strings.TrimSpace(cfg.VerifyMode))
	cfg.Model = strings.TrimSpace(cfg.Model)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate enforces fast-fail config rules before runtime startup.
func (c Config) Validate() error {
	if c.Model == "" {
		return errors.New("model must not be empty")
	}
	if c.MaxSteps <= 0 {
		return errors.New("max steps must be greater than zero")
	}
	if c.StepTimeout <= 0 {
		return errors.New("step timeout must be greater than zero")
	}
	if c.MemoryLimitMB <= 0 {
		return errors.New("memory limit must be greater than zero")
	}

	switch c.Provider {
	case ProviderOpenAI, ProviderAnthropic:
	default:
		return fmt.Errorf("unsupported provider %q", c.Provider)
	}

	switch c.VerifyMode {
	case VerifyModeOff, VerifyModeStandard, VerifyModeStrict:
	default:
		return fmt.Errorf("unsupported verify mode %q", c.VerifyMode)
	}

	return nil
}
