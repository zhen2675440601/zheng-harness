package llm

import (
	"errors"
	"strings"
	"testing"
)

func TestProviderPluginBuiltinsUseRegistry(t *testing.T) {
	t.Parallel()

	resolver := &recordingProviderResolver{
		provider: AdaptProviderPlugin("openai", NewOpenAIProvider("secret", "", "gpt-4.1-mini")),
	}
	previous := DefaultProviderResolver()
	SetDefaultProviderResolver(resolver)
	t.Cleanup(func() {
		SetDefaultProviderResolver(previous)
	})

	provider, err := NewProvider(stubProviderConfig{providerType: "openai", model: "gpt-4.1-mini", apiKey: "secret"})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if !resolver.called {
		t.Fatal("expected NewProvider() to resolve through registry-backed seam")
	}
	if resolver.id != "openai" {
		t.Fatalf("resolved id = %q, want openai", resolver.id)
	}
	if provider.Name() != "openai" {
		t.Fatalf("provider.Name() = %q, want openai", provider.Name())
	}
}

func TestProviderPluginUnknownProviderFailsDeterministically(t *testing.T) {
	t.Parallel()

	provider, err := NewProvider(stubProviderConfig{providerType: "missing-provider", model: "gpt-4.1-mini", apiKey: "secret"})
	if err == nil {
		t.Fatal("expected NewProvider() error")
	}
	if provider != nil {
		t.Fatalf("provider = %#v, want nil", provider)
	}
	if !strings.Contains(err.Error(), `unsupported provider type "missing-provider"`) {
		t.Fatalf("NewProvider() error = %v, want unsupported provider detail", err)
	}
	if !errors.Is(err, errUnsupportedProviderType) {
		t.Fatalf("NewProvider() error = %v, want %v", err, errUnsupportedProviderType)
	}
}

type recordingProviderResolver struct {
	called   bool
	id       string
	provider Provider
	err      error
}

func (r *recordingProviderResolver) Resolve(id string, _ ProviderConfig) (Provider, error) {
	r.called = true
	r.id = id
	return r.provider, r.err
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
