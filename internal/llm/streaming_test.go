package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
)

func TestStreamFallbackEmitsTokenDeltaAndComplete(t *testing.T) {
	t.Parallel()

	var events []domain.StreamingEvent
	err := StreamFallback(context.Background(), func(_ context.Context, _ Request) (Response, error) {
		return Response{Model: "stub", Output: "hello world", StopReason: "stop"}, nil
	}, Request{SystemPrompt: "sys", Input: "hi"}, func(event domain.StreamingEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamFallback: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Type != domain.EventTokenDelta {
		t.Fatalf("first event type = %q, want %q", events[0].Type, domain.EventTokenDelta)
	}
	var token domain.TokenDeltaPayload
	if err := events[0].GetPayload(&token); err != nil {
		t.Fatalf("decode token payload: %v", err)
	}
	if token.Content != "hello world" {
		t.Fatalf("token content = %q, want hello world", token.Content)
	}
	if events[1].Type != domain.EventSessionComplete {
		t.Fatalf("second event type = %q, want %q", events[1].Type, domain.EventSessionComplete)
	}
}

func TestStreamCancelStopsEmission(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var calls int
	err := StreamFallback(ctx, func(_ context.Context, _ Request) (Response, error) {
		return Response{Output: "hello"}, nil
	}, Request{}, func(event domain.StreamingEvent) error {
		calls++
		if event.Type == domain.EventTokenDelta {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if calls != 1 {
		t.Fatalf("emit calls = %d, want 1", calls)
	}
}

func TestSSEParserBasicFunctionality(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("data: first\n\ndata: second\n\ndata: [DONE]\n\n")
	var chunks []string
	err := ParseSSE(context.Background(), input, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("ParseSSE: %v", err)
	}
	want := []string{"first", "second"}
	if !reflect.DeepEqual(chunks, want) {
		t.Fatalf("chunks = %#v, want %#v", chunks, want)
	}
}

func TestSSEParserMalformed(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("event: message\ninvalid line\ndata: ok\n\n:data comment\ndata: [DONE]\n\n")
	var chunks []string
	err := ParseSSE(context.Background(), input, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("ParseSSE: %v", err)
	}
	want := []string{"ok"}
	if !reflect.DeepEqual(chunks, want) {
		t.Fatalf("chunks = %#v, want %#v", chunks, want)
	}
}

func TestOpenAIStreamParsesSSE(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept = %q, want text/event-stream", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-key", server.URL, "gpt-4.1-mini")
	var events []domain.StreamingEvent
	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	assertStreamSequence(t, events, []string{"hello", " world"})
}

func TestAnthropicStreamParsesSSE(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"message_start\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\" world\"}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewAnthropicProvider("key", server.URL, "claude-3-5-sonnet")
	var events []domain.StreamingEvent
	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	assertStreamSequence(t, events, []string{"hello", " world"})
}

func TestDashScopeStreamParsesSSE(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\" world\"}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewDashScopeProvider("qwen3.6-plus", server.URL, "key")
	var events []domain.StreamingEvent
	err := provider.Stream(context.Background(), Request{SystemPrompt: "sys", Input: "hello"}, func(event domain.StreamingEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	assertStreamSequence(t, events, []string{"hello", " world"})
}

func assertStreamSequence(t *testing.T, events []domain.StreamingEvent, wantTokens []string) {
	t.Helper()
	if len(events) != len(wantTokens)+1 {
		t.Fatalf("events = %d, want %d", len(events), len(wantTokens)+1)
	}
	for i, want := range wantTokens {
		if events[i].Type != domain.EventTokenDelta {
			t.Fatalf("event[%d].type = %q, want %q", i, events[i].Type, domain.EventTokenDelta)
		}
		var payload domain.TokenDeltaPayload
		if err := events[i].GetPayload(&payload); err != nil {
			t.Fatalf("decode payload %d: %v", i, err)
		}
		if payload.Content != want {
			t.Fatalf("payload[%d].content = %q, want %q", i, payload.Content, want)
		}
	}
	if events[len(events)-1].Type != domain.EventSessionComplete {
		t.Fatalf("last event type = %q, want %q", events[len(events)-1].Type, domain.EventSessionComplete)
	}
}
