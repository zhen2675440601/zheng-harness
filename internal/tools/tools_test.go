package tools_test

import (
	"context"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/tools"
)

func TestAllowedToolExecution(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "notes.txt\nhello harness",
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "read_file",
		Input: "notes.txt",
	})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if result.Output != "hello harness" {
		t.Fatalf("output = %q, want file contents", result.Output)
	}

	search, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "grep_search",
		Input: "hello",
	})
	if err != nil {
		t.Fatalf("grep search: %v", err)
	}
	if !strings.Contains(search.Output, "notes.txt") {
		t.Fatalf("search output = %q, want notes.txt", search.Output)
	}

	defs := executor.Registry().List()
	if len(defs) != 5 {
		t.Fatalf("definition count = %d, want 5", len(defs))
	}
}

func TestForbiddenToolExecution(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "read_file",
		Input: "..\\secret.txt",
	})
	if err == nil || !strings.Contains(err.Error(), "escapes allowed roots") {
		t.Fatalf("path traversal error = %v, want escape rejection", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "powershell -NoProfile Get-Date",
	})
	if err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("command error = %v, want allowlist rejection", err)
	}
}
