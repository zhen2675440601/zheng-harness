package adapters

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
)

func TestFileAdapterListDirMarksDirectories(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "dir", "file.txt"), "content")
	adapter := NewFileAdapter(workspace)

	result, err := adapter.ListDir(context.Background(), domain.ToolCall{Name: "list_dir", Input: "."})
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}
	if !strings.Contains(result.Output, "dir/") {
		t.Fatalf("ListDir() output = %q, want dir/ entry", result.Output)
	}
}

func TestFileAdapterListDirHonorsCancellation(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "file.txt"), "content")
	adapter := NewFileAdapter(workspace)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := adapter.ListDir(ctx, domain.ToolCall{Name: "list_dir", Input: "."})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListDir() error = %v, want context canceled", err)
	}
}

func TestFileAdapterWriteFileRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	adapter := NewFileAdapter(t.TempDir())
	_, err := adapter.WriteFile(context.Background(), domain.ToolCall{Name: "write_file", Input: "missing-body"})
	if err == nil || !strings.Contains(err.Error(), "path and body") {
		t.Fatalf("WriteFile() error = %v, want malformed input error", err)
	}
}

func TestShellAdapterRejectsEmptyAndMalformedCommands(t *testing.T) {
	t.Parallel()

	adapter := NewShellAdapter(t.TempDir())
	_, err := adapter.Exec(context.Background(), domain.ToolCall{Name: "exec_command", Input: "   "})
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("Exec() empty error = %v, want empty command rejection", err)
	}

	_, err = adapter.Exec(context.Background(), domain.ToolCall{Name: "exec_command", Input: `git commit -m "unterminated`})
	if err == nil || !strings.Contains(err.Error(), "failed to parse command") {
		t.Fatalf("Exec() parse error = %v, want parse rejection", err)
	}
}

func TestGlobAdapterRejectsAbsolutePattern(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	adapter := NewGlobAdapter(workspace)
	absPattern, err := filepath.Abs(filepath.Join(workspace, "*.go"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	_, err = adapter.Glob(context.Background(), domain.ToolCall{Name: "glob", Input: absPattern})
	if err == nil || !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("Glob() error = %v, want workspace escape rejection", err)
	}
}
