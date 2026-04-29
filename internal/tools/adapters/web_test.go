package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestWebFetchSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "hello from web")
	}))
	defer server.Close()

	adapter := NewWebAdapter(nil)
	result, err := adapter.Fetch(context.Background(), domain.ToolCall{
		Name:  "web_fetch",
		Input: `{"url":"` + server.URL + `"}`,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Output != "hello from web" {
		t.Fatalf("Fetch() output = %q, want full response", result.Output)
	}
}

func TestWebFetchTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = fmt.Fprint(w, "too slow")
	}))
	defer server.Close()

	adapter := NewWebAdapter(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := adapter.Fetch(ctx, domain.ToolCall{
		Name:  "web_fetch",
		Input: `{"url":"` + server.URL + `"}`,
	})
	if err == nil {
		t.Fatal("Fetch() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Fetch() error = %v, want context deadline exceeded", err)
	}
}

func TestWebFetchBlockedDomain(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	adapter := NewWebAdapter([]string{"example.com"})
	_, err := adapter.Fetch(context.Background(), domain.ToolCall{
		Name:  "web_fetch",
		Input: `{"url":"` + server.URL + `"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("Fetch() error = %v, want blocked domain error", err)
	}
}

func TestWebFetchInvalidURL(t *testing.T) {
	t.Parallel()

	adapter := NewWebAdapter(nil)
	_, err := adapter.Fetch(context.Background(), domain.ToolCall{
		Name:  "web_fetch",
		Input: `{"url":"file:///etc/passwd"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "only supports http and https") {
		t.Fatalf("Fetch() error = %v, want invalid URL scheme error", err)
	}
}

func TestWebFetchTruncation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("abcdef", 4))
	}))
	defer server.Close()

	adapter := NewWebAdapter(nil)
	result, err := adapter.Fetch(context.Background(), domain.ToolCall{
		Name:  "web_fetch",
		Input: `{"url":"` + server.URL + `","max_length":5}`,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Output != "abcde" {
		t.Fatalf("Fetch() output = %q, want truncated body", result.Output)
	}
}
