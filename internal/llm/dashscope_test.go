package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDashScopeProviderGenerateTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"qwen","stop_reason":"stop","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()

	provider := NewDashScopeProvider("qwen", server.URL, "key")
	provider.client.Timeout = 50 * time.Millisecond

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "send dashscope request") {
		t.Fatalf("timeout error = %v, want request send failure wrapper", err)
	}
}

func TestDashScopeProviderGenerateNon2xx(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit","message":"rate limited"}}`))
	}))
	defer server.Close()

	provider := NewDashScopeProvider("qwen", server.URL, "key")

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected non-2xx error, got nil")
	}
	if !strings.Contains(err.Error(), "status 429") {
		t.Fatalf("non-2xx error = %v, want status code", err)
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("non-2xx error = %v, want envelope message", err)
	}
}

func TestDashScopeProviderGenerateEmptyContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"qwen","stop_reason":"stop","content":[{"type":"text","text":"   "}]}`))
	}))
	defer server.Close()

	provider := NewDashScopeProvider("qwen", server.URL, "key")

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected empty-content error, got nil")
	}
	if !strings.Contains(err.Error(), "no text output") {
		t.Fatalf("empty-content error = %v, want no text output", err)
	}
}

func TestDashScopeProviderGenerateSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "key" {
			t.Fatalf("x-api-key = %q, want key", got)
		}
		if got := r.Header.Get("anthropic-version"); got != dashScopeAnthropicVersionHeader {
			t.Fatalf("anthropic-version = %q, want %q", got, dashScopeAnthropicVersionHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"model":"%s","stop_reason":"end_turn","content":[{"type":"text","text":"hello"}]}`, "qwen3.6-plus")))
	}))
	defer server.Close()

	provider := NewDashScopeProvider("qwen3.6-plus", server.URL, "key")

	resp, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Model != "qwen3.6-plus" {
		t.Fatalf("model = %q, want qwen3.6-plus", resp.Model)
	}
	if resp.Output != "hello" {
		t.Fatalf("output = %q, want hello", resp.Output)
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("stop reason = %q, want end_turn", resp.StopReason)
	}
}
