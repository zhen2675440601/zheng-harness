package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAnthropicAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("bad-key", server.URL, "claude-3-5-sonnet")

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("auth error = %v, want explicit authentication failed message", err)
	}
}

func TestAnthropicOverloadedRetry(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		w.Header().Set("Content-Type", "application/json")
		if current == 1 {
			w.WriteHeader(529)
			_, _ = w.Write([]byte(`{"error":{"type":"overloaded_error","message":"overloaded"}}`))
			return
		}

		_, _ = w.Write([]byte(`{"model":"claude-3-5-sonnet","stop_reason":"end_turn","content":[{"type":"text","text":"retry ok"}]}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("good-key", server.URL, "claude-3-5-sonnet")
	provider.backoffBase = time.Millisecond

	resp, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err != nil {
		t.Fatalf("generate with retry: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if resp.Output != "retry ok" {
		t.Fatalf("output = %q, want retry ok", resp.Output)
	}
}

func TestAnthropicParsesResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/messages" {
			t.Fatalf("path = %s, want /messages", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "key" {
			t.Fatalf("x-api-key = %q, want key", got)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicDefaultAPIVersion {
			t.Fatalf("anthropic-version = %q, want %q", got, anthropicDefaultAPIVersion)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q, want application/json", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-3-5-sonnet","stop_reason":"end_turn","content":[{"type":"text","text":"hello from anthropic"}]}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("key", server.URL, "claude-3-5-sonnet")

	resp, err := provider.Generate(context.Background(), Request{SystemPrompt: "You are concise.", Input: "hi"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Model != "claude-3-5-sonnet" {
		t.Fatalf("model = %q, want claude-3-5-sonnet", resp.Model)
	}
	if resp.Output != "hello from anthropic" {
		t.Fatalf("output = %q, want hello from anthropic", resp.Output)
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("stop reason = %q, want end_turn", resp.StopReason)
	}
}
