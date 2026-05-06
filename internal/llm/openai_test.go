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

func TestOpenAIAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("bad-key", server.URL, "gpt-4.1-mini")

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("error = %v, want explicit authentication failure", err)
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error = %v, want provider error detail", err)
	}
}

func TestOpenAITransportFailure(t *testing.T) {
	t.Parallel()

	provider := NewOpenAIProvider("test-key", "https://127.0.0.1:1", "gpt-4.1-mini")
	provider.httpClient.Timeout = 20 * time.Millisecond
	provider.maxRetries = 0

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected transport error, got nil")
	}
	if !strings.Contains(err.Error(), "openai transport failed") {
		t.Fatalf("error = %v, want deterministic transport failure", err)
	}
}

func TestOpenAIMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL, "gpt-4.1-mini")

	_, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err == nil {
		t.Fatalf("expected malformed response error, got nil")
	}
	if !strings.Contains(err.Error(), "malformed response") {
		t.Fatalf("error = %v, want malformed response classification", err)
	}
}

func TestOpenAIRateLimitRetry(t *testing.T) {
	t.Parallel()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit"}}`))
			return
		}

		_, _ = w.Write([]byte(`{"model":"gpt-4.1-mini","choices":[{"message":{"content":"retry ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL, "gpt-4.1-mini")

	resp, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err != nil {
		t.Fatalf("generate after retry: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
	if resp.Output != "retry ok" {
		t.Fatalf("output = %q, want retry ok", resp.Output)
	}
}

func TestOpenAIParsesResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want Bearer test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-4.1-mini","choices":[{"message":{"content":"hello world"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL, "gpt-4.1-mini")

	resp, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want gpt-4.1-mini", resp.Model)
	}
	if resp.Output != "hello world" {
		t.Fatalf("output = %q, want hello world", resp.Output)
	}
	if resp.StopReason != "stop" {
		t.Fatalf("stop reason = %q, want stop", resp.StopReason)
	}
}

func TestOpenAICompatibleEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"compatible endpoint"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL+"/v1", "deepseek-chat")

	resp, err := provider.Generate(context.Background(), Request{SystemPrompt: "sys", Input: "hello"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Output != "compatible endpoint" {
		t.Fatalf("output = %q, want compatible endpoint", resp.Output)
	}
}

func TestOpenAIStreamMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {not-json}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL, "gpt-4.1-mini")
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

func TestOpenAIStreamAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("bad-key", server.URL, "gpt-4.1-mini")
	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		return nil
	})
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error = %v, want deterministic auth failure with detail", err)
	}
}

func TestOpenAIStreamTransportFailure(t *testing.T) {
	t.Parallel()

	provider := NewOpenAIProvider("test-key", "https://127.0.0.1:1", "gpt-4.1-mini")
	provider.httpClient.Timeout = 20 * time.Millisecond
	provider.maxRetries = 0

	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		return nil
	})
	if err == nil {
		t.Fatalf("expected transport error, got nil")
	}
	if !strings.Contains(err.Error(), "openai transport failed") {
		t.Fatalf("error = %v, want deterministic transport failure", err)
	}
}

func TestOpenAIStreamEmitterErrorPropagates(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("emit failed")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL, "gpt-4.1-mini")
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
