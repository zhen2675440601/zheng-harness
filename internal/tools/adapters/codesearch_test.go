package adapters

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
)

func TestCodeSearchByLanguage(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "pkg", "main.go"), "package main\nfunc Target() {}\n")
	writeTestFile(t, filepath.Join(workspace, "pkg", "main.py"), "def Target():\n    pass\n")

	adapter := NewCodeSearchAdapter(workspace)
	result, err := adapter.Search(context.Background(), domain.ToolCall{
		Name:  "code_search",
		Input: `{"pattern":"Target","language":"go"}`,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if strings.TrimSpace(result.Output) != "pkg/main.go" {
		t.Fatalf("Search() output = %q, want only Go match", result.Output)
	}
}

func TestCodeSearchContentMode(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "pkg", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func helper() {}",
		"func Target() {}",
		"func after() {}",
		"func tail() {}",
	}, "\n")+"\n")

	adapter := NewCodeSearchAdapter(workspace)
	result, err := adapter.Search(context.Background(), domain.ToolCall{
		Name:  "code_search",
		Input: `{"pattern":"Target","language":"go","output_mode":"content"}`,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, want := range []string{
		"pkg/main.go:2: ",
		"pkg/main.go:3: func helper() {}",
		"pkg/main.go:4: func Target() {}",
		"pkg/main.go:5: func after() {}",
		"pkg/main.go:6: func tail() {}",
	} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("Search() output = %q, want context line %q", result.Output, want)
		}
	}
}

func TestCodeSearchExcludesVendor(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "vendor", "lib.go"), "package vendor\nfunc Target() {}\n")
	writeTestFile(t, filepath.Join(workspace, "app", "main.go"), "package app\nfunc Target() {}\n")

	adapter := NewCodeSearchAdapter(workspace)
	result, err := adapter.Search(context.Background(), domain.ToolCall{
		Name:  "code_search",
		Input: `{"pattern":"Target","language":"go"}`,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if strings.Contains(result.Output, "vendor/lib.go") {
		t.Fatalf("Search() output = %q, want vendor directory excluded", result.Output)
	}
	if strings.TrimSpace(result.Output) != "app/main.go" {
		t.Fatalf("Search() output = %q, want only app/main.go", result.Output)
	}
}

func TestCodeSearchMaxResults(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "pkg", "a.go"), "package main\nfunc Target() {}\n")
	writeTestFile(t, filepath.Join(workspace, "pkg", "b.go"), "package main\nfunc Target() {}\n")
	writeTestFile(t, filepath.Join(workspace, "pkg", "c.go"), "package main\nfunc Target() {}\n")

	adapter := NewCodeSearchAdapter(workspace)
	result, err := adapter.Search(context.Background(), domain.ToolCall{
		Name:  "code_search",
		Input: `{"pattern":"Target","language":"go","max_results":2}`,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	if len(lines) != 2 {
		t.Fatalf("Search() returned %d lines, want 2; output=%q", len(lines), result.Output)
	}
	if lines[0] != "pkg/a.go" || lines[1] != "pkg/b.go" {
		t.Fatalf("Search() output = %q, want first two sorted files", result.Output)
	}
}
