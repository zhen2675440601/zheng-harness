package adapters

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
)

func TestSearchAdapterGrepSupportsRegexFlagsAndModes(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "a.go"), "package main\nfunc Alpha() {}\nfunc beta() {}\n")
	writeTestFile(t, filepath.Join(workspace, "notes.txt"), "Alpha\nALPHA\nbeta\n")
	writeTestFile(t, filepath.Join(workspace, ".git", "ignored.txt"), "Alpha\n")

	adapter := NewSearchAdapter(workspace)

	filesResult, err := adapter.Grep(context.Background(), domain.ToolCall{Name: "grep_search", Input: "alpha\ni"})
	if err != nil {
		t.Fatalf("Grep() files_with_matches error = %v", err)
	}
	if got := strings.TrimSpace(filesResult.Output); got != "a.go\nnotes.txt" && got != "notes.txt\na.go" {
		t.Fatalf("files output = %q, want both matching files", filesResult.Output)
	}

	contentResult, err := adapter.Grep(context.Background(), domain.ToolCall{Name: "grep_search", Input: "alpha\ni\ncontent"})
	if err != nil {
		t.Fatalf("Grep() content error = %v", err)
	}
	if !strings.Contains(contentResult.Output, "a.go:2: func Alpha() {}") {
		t.Fatalf("content output = %q, want line-numbered a.go match", contentResult.Output)
	}
	if !strings.Contains(contentResult.Output, "notes.txt:1: Alpha") || !strings.Contains(contentResult.Output, "notes.txt:2: ALPHA") {
		t.Fatalf("content output = %q, want line-numbered text matches", contentResult.Output)
	}

	countResult, err := adapter.Grep(context.Background(), domain.ToolCall{Name: "grep_search", Input: "alpha\ni\ncount"})
	if err != nil {
		t.Fatalf("Grep() count error = %v", err)
	}
	if !strings.Contains(countResult.Output, "a.go:1") || !strings.Contains(countResult.Output, "notes.txt:2") {
		t.Fatalf("count output = %q, want per-file counts", countResult.Output)
	}
}

func TestSearchAdapterGrepSupportsIncludeGlobAndRegexErrors(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "pkg", "alpha.go"), "func Alpha() {}\n")
	writeTestFile(t, filepath.Join(workspace, "pkg", "alpha.txt"), "Alpha text\n")

	adapter := NewSearchAdapter(workspace)

	result, err := adapter.Grep(context.Background(), domain.ToolCall{Name: "grep_search", Input: "Alpha\n\nfiles_with_matches\n**/*.go"})
	if err != nil {
		t.Fatalf("Grep() include glob error = %v", err)
	}
	if got := strings.TrimSpace(result.Output); got != "pkg/alpha.go" {
		t.Fatalf("include glob output = %q, want only go file", result.Output)
	}

	invalidRegex, err := adapter.Grep(context.Background(), domain.ToolCall{Name: "grep_search", Input: "[broken"})
	if err != nil {
		t.Fatalf("Grep() invalid regex err = %v, want nil error return", err)
	}
	if invalidRegex.Error == "" {
		t.Fatalf("invalid regex result error = %q, want populated error field", invalidRegex.Error)
	}

	invalidMode, err := adapter.Grep(context.Background(), domain.ToolCall{Name: "grep_search", Input: "Alpha\n\nsummary"})
	if err != nil {
		t.Fatalf("Grep() invalid mode err = %v, want nil error return", err)
	}
	if !strings.Contains(invalidMode.Error, "unsupported grep_search output mode") {
		t.Fatalf("invalid mode result error = %q, want output mode error", invalidMode.Error)
	}
}
