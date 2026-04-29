//go:build smoke

package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestDashScopeSmoke makes a real HTTP call to DashScope API with valid credentials.
// Requires: DASHSCOPE_API_KEY environment variable to be set.
// This test is opt-in via build tag and not part of normal CI.
func TestDashScopeSmoke(t *testing.T) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		t.Skip("DASHSCOPE_API_KEY not set, skipping smoke test")
	}

	provider := NewDashScopeProvider(
		"qwen3.6-plus",
		"https://coding.dashscope.aliyuncs.com/apps/anthropic/v1",
		apiKey,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.Generate(ctx, Request{
		SystemPrompt: "You are a helpful assistant.",
		Input:        "Respond with exactly: OK",
	})

	if err != nil {
		t.Fatalf("DashScope API call failed: %v", err)
	}

	if resp.Output == "" {
		t.Fatal("Expected non-empty output from DashScope")
	}

	t.Logf("DashScope response: model=%s, output=%s", resp.Model, resp.Output)
}

// TestOpenAISmoke makes a real HTTP call to OpenAI API with valid credentials.
// Requires: OPENAI_API_KEY environment variable to be set.
func TestOpenAISmoke(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping smoke test")
	}

	provider := NewOpenAIProvider(
		apiKey,
		"https://api.openai.com/v1",
		"gpt-4.1-mini",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.Generate(ctx, Request{
		SystemPrompt: "You are a helpful assistant.",
		Input:        "Respond with exactly: OK",
	})

	if err != nil {
		t.Fatalf("OpenAI API call failed: %v", err)
	}

	if resp.Output == "" {
		t.Fatal("Expected non-empty output from OpenAI")
	}

	t.Logf("OpenAI response: model=%s, output=%s", resp.Model, resp.Output)
}

// TestAnthropicSmoke makes a real HTTP call to Anthropic API with valid credentials.
// Requires: ANTHROPIC_API_KEY environment variable to be set.
func TestAnthropicSmoke(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping smoke test")
	}

	provider := NewAnthropicProvider(
		apiKey,
		"https://api.anthropic.com/v1",
		"claude-sonnet-4-20250514",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.Generate(ctx, Request{
		SystemPrompt: "You are a helpful assistant.",
		Input:        "Respond with exactly: OK",
	})

	if err != nil {
		t.Fatalf("Anthropic API call failed: %v", err)
	}

	if resp.Output == "" {
		t.Fatal("Expected non-empty output from Anthropic")
	}

	t.Logf("Anthropic response: model=%s, output=%s", resp.Model, resp.Output)
}