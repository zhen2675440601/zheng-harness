package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"zheng-harness/internal/domain"
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
	if !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("auth error = %v, want provider detail", err)
	}
}

func TestAnthropicTransportFailure(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test-key", "https://127.0.0.1:1", "claude-3-5-sonnet")
	provider.httpClient.Timeout = 20 * time.Millisecond
	provider.maxRetries = 0

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected transport error, got nil")
	}
	if !strings.Contains(err.Error(), "anthropic transport failed") {
		t.Fatalf("error = %v, want deterministic transport failure", err)
	}
}

func TestAnthropicMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("key", server.URL, "claude-3-5-sonnet")

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected malformed response error, got nil")
	}
	if !strings.Contains(err.Error(), "malformed response") {
		t.Fatalf("error = %v, want malformed response classification", err)
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

func TestAnthropicStreamMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {not-json}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewAnthropicProvider("key", server.URL, "claude-3-5-sonnet")
	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		return nil
	})
	if err == nil {
		t.Fatalf("expected malformed stream error, got nil")
	}
	if !strings.Contains(err.Error(), "malformed stream response") {
		t.Fatalf("error = %v, want malformed stream classification", err)
	}
}

func TestAnthropicStreamAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("bad-key", server.URL, "claude-3-5-sonnet")
	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		return nil
	})
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("error = %v, want deterministic auth failure with detail", err)
	}
}

func TestAnthropicStreamTransportFailure(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider("test-key", "https://127.0.0.1:1", "claude-3-5-sonnet")
	provider.httpClient.Timeout = 20 * time.Millisecond
	provider.maxRetries = 0

	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		return nil
	})
	if err == nil {
		t.Fatalf("expected transport error, got nil")
	}
	if !strings.Contains(err.Error(), "anthropic transport failed") {
		t.Fatalf("error = %v, want deterministic transport failure", err)
	}
}

func TestAnthropicStreamEmitterErrorPropagates(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("emit failed")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewAnthropicProvider("key", server.URL, "claude-3-5-sonnet")
	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		if event.Type == domain.EventTokenDelta {
			return wantErr
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want emitter error", err)
	}
}
