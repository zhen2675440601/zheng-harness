package adapters

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestAskUserFreeText(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	adapter := NewInteractiveAdapter(strings.NewReader("hello world\n"), &output)

	result, err := adapter.AskUser(context.Background(), domain.ToolCall{
		Name:  "ask_user",
		Input: `{"question":"What is your name?"}`,
	})
	if err != nil {
		t.Fatalf("AskUser() error = %v", err)
	}
	if result.Output != "hello world" {
		t.Fatalf("AskUser() output = %q, want free-text response", result.Output)
	}
	if !strings.Contains(output.String(), "What is your name?") {
		t.Fatalf("AskUser() prompt output = %q, want question", output.String())
	}
}

func TestAskUserWithOptions(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	adapter := NewInteractiveAdapter(strings.NewReader("2\n"), &output)

	result, err := adapter.AskUser(context.Background(), domain.ToolCall{
		Name:  "ask_user",
		Input: `{"question":"Pick one","options":["alpha","beta"]}`,
	})
	if err != nil {
		t.Fatalf("AskUser() error = %v", err)
	}
	if result.Output != "beta" {
		t.Fatalf("AskUser() output = %q, want selected option", result.Output)
	}
	if !strings.Contains(output.String(), "1. alpha") || !strings.Contains(output.String(), "2. beta") {
		t.Fatalf("AskUser() prompt output = %q, want numbered options", output.String())
	}
}

func TestAskUserInvalidOption(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	adapter := NewInteractiveAdapter(strings.NewReader("4\n0\nwrong\n"), &output)

	_, err := adapter.AskUser(context.Background(), domain.ToolCall{
		Name:  "ask_user",
		Input: `{"question":"Pick one","options":["alpha","beta"]}`,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid choice after 3 attempts") {
		t.Fatalf("AskUser() error = %v, want invalid-attempts error", err)
	}
	if count := strings.Count(output.String(), "Invalid choice."); count != 3 {
		t.Fatalf("AskUser() invalid choice count = %d, want 3; output=%q", count, output.String())
	}
}

func TestAskUserTimeout(t *testing.T) {
	t.Parallel()

	reader, writer := io.Pipe()
	defer writer.Close()

	var output strings.Builder
	adapter := NewInteractiveAdapter(reader, &output)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := adapter.AskUser(ctx, domain.ToolCall{
			Name:  "ask_user",
			Input: `{"question":"Waiting"}`,
		})
		errCh <- err
	}()

	err := <-errCh
	_ = writer.Close()
	if err == nil || err.Error() != "user did not respond in time" {
		t.Fatalf("AskUser() error = %v, want timeout error", err)
	}
}
