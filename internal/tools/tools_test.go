package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"zheng-harness/internal/config"
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

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\nhello harness\n<<<NEW\nhello edit tool",
	})
	if err != nil {
		t.Fatalf("edit file: %v", err)
	}

	edited, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "read_file",
		Input: "notes.txt",
	})
	if err != nil {
		t.Fatalf("read edited file: %v", err)
	}
	if edited.Output != "hello edit tool" {
		t.Fatalf("edited output = %q, want replaced content", edited.Output)
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
	if len(defs) != 7 {
		t.Fatalf("definition count = %d, want 7", len(defs))
	}
}

func TestGlobRecursive(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "main.go\npackage main",
	})
	if err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: filepath.ToSlash(filepath.Join("pkg", "nested", "helper.go")) + "\npackage nested",
	})
	if err != nil {
		t.Fatalf("write helper.go: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "config.json\n{}",
	})
	if err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "glob",
		Input: "**/*.go",
	})
	if err != nil {
		t.Fatalf("glob recursive: %v", err)
	}

	paths := strings.Split(strings.TrimSpace(result.Output), "\n")
	if len(paths) != 2 {
		t.Fatalf("glob result count = %d, want 2; output=%q", len(paths), result.Output)
	}
	joined := "\n" + result.Output + "\n"
	if !strings.Contains(joined, "\nmain.go\n") {
		t.Fatalf("glob output = %q, want main.go", result.Output)
	}
	if !strings.Contains(joined, "\n"+filepath.ToSlash(filepath.Join("pkg", "nested", "helper.go"))+"\n") {
		t.Fatalf("glob output = %q, want nested helper.go", result.Output)
	}
}

func TestGlobNoMatch(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "glob",
		Input: "*.xyz",
	})
	if err != nil {
		t.Fatalf("glob no match: %v", err)
	}
	if result.Output != "" {
		t.Fatalf("glob output = %q, want empty output", result.Output)
	}
}

func TestGrepSearchSupportsRegexModesAndIncludeGlob(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "alpha.txt\nAlpha\nALPHA\nbeta",
	})
	if err != nil {
		t.Fatalf("write alpha.txt: %v", err)
	}
	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: filepath.ToSlash(filepath.Join("pkg", "main.go")) + "\npackage main\nfunc Alpha() {}",
	})
	if err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	content, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "grep_search",
		Input: "alpha\ni\ncontent",
	})
	if err != nil {
		t.Fatalf("grep content: %v", err)
	}
	if !strings.Contains(content.Output, "alpha.txt:1: Alpha") || !strings.Contains(content.Output, filepath.ToSlash(filepath.Join("pkg", "main.go"))+":2: func Alpha() {}") {
		t.Fatalf("content output = %q, want line-numbered regex matches", content.Output)
	}

	count, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "grep_search",
		Input: "alpha\ni\ncount\n**/*.go",
	})
	if err != nil {
		t.Fatalf("grep count: %v", err)
	}
	if got := strings.TrimSpace(count.Output); got != filepath.ToSlash(filepath.Join("pkg", "main.go"))+":1" {
		t.Fatalf("count output = %q, want only go file count", count.Output)
	}
}

func TestGrepSearchReportsInvalidRegexViaResultError(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	result, err := executor.Execute(context.Background(), domain.ToolCall{Name: "grep_search", Input: "[invalid"})
	if err != nil {
		t.Fatalf("grep invalid regex err = %v, want nil", err)
	}
	if result.Error == "" {
		t.Fatalf("result error = %q, want invalid regex message", result.Error)
	}
}

func TestGlobWorkspaceBoundary(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "glob",
		Input: "..\\**\\*",
	})
	if err == nil || !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("glob boundary error = %v, want workspace escape rejection", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: filepath.ToSlash(filepath.Join("safe", "inside.txt")) + "\nok",
	})
	if err != nil {
		t.Fatalf("write safe file: %v", err)
	}

	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "glob",
		Input: "**/*",
	})
	if err != nil {
		t.Fatalf("glob all files: %v", err)
	}
	if strings.Contains(result.Output, "..") {
		t.Fatalf("glob output = %q, should not contain parent traversal", result.Output)
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
		Name:  "edit_file",
		Input: "..\\secret.txt\n<<<OLD\nold\n<<<NEW\nnew",
	})
	if err == nil || !strings.Contains(err.Error(), "escapes allowed roots") {
		t.Fatalf("edit path traversal error = %v, want escape rejection", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "powershell -NoProfile Get-Date",
	})
	if err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("command error = %v, want allowlist rejection", err)
	}
}

func TestExpandedDefaultAllowlistAllowsNPM(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "npm test",
	})
	if err == nil || strings.Contains(err.Error(), "not allowlisted") || strings.Contains(err.Error(), "explicitly denied") {
		t.Fatalf("npm command error = %v, want allowlisted execution attempt", err)
	}
}

func TestRMIsAlwaysRejectedEvenWhenExplicitlyAllowed(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace, tools.WithAllowedCommands([]string{"go", "rm"}))
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "rm -rf /tmp/test",
	})
	if err == nil || !strings.Contains(err.Error(), "explicitly denied for safety") {
		t.Fatalf("rm command error = %v, want explicit deny", err)
	}
}

func TestCustomAllowedCommandsOverrideDefaults(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace, tools.WithAllowedCommands([]string{"go"}))
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "npm test",
	})
	if err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("npm rejection error = %v, want allowlist rejection", err)
	}
}

func TestConfigAllowedCommandsRestrictExecutor(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cfg := config.Default()
	cfg.Runtime.AllowedCommands = []string{"go"}
	executor, err := tools.NewExecutor(workspace, tools.WithAllowedCommands(cfg.Runtime.AllowedCommands))
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "npm test",
	})
	if err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("npm rejection error = %v, want allowlist rejection", err)
	}
}

func TestEditFileRejectsMalformedOrAmbiguousInput(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\nonly-old-text",
	})
	if err == nil || !strings.Contains(err.Error(), "must contain <<<OLD and <<<NEW blocks") {
		t.Fatalf("malformed edit input error = %v, want schema rejection", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "notes.txt\nalpha beta alpha",
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\nalpha\n<<<NEW\ngamma",
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous edit input error = %v, want ambiguity rejection", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\n",
	})
	if err == nil || !strings.Contains(err.Error(), "must contain <<<OLD and <<<NEW blocks") {
		t.Fatalf("empty old text framing error = %v, want schema rejection", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\n\n<<<NEW\nreplacement",
	})
	if err == nil || !strings.Contains(err.Error(), "old text must not be empty") {
		t.Fatalf("empty old text error = %v, want empty old text rejection", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\ndoes-not-exist\n<<<NEW\nreplacement",
	})
	if err == nil || !strings.Contains(err.Error(), "old text not found") {
		t.Fatalf("not-found error = %v, want old text not found rejection", err)
	}
}

func TestEditFileSupportsMultilineNewText(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "notes.txt\nlineA",
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\nlineA\n<<<NEW\nline1\nline2",
	})
	if err != nil {
		t.Fatalf("edit file multiline new text: %v", err)
	}

	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "read_file",
		Input: "notes.txt",
	})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if result.Output != "line1\nline2" {
		t.Fatalf("multiline replacement output = %q, want multiline content", result.Output)
	}
}

func TestEditFileSupportsMultilineOldText(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "notes.txt\nbefore\nline-1\nline-2\nafter",
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\nline-1\nline-2\n<<<NEW\nreplacement",
	})
	if err != nil {
		t.Fatalf("edit file multiline old text: %v", err)
	}

	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "read_file",
		Input: "notes.txt",
	})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if result.Output != "before\nreplacement\nafter" {
		t.Fatalf("multiline old text replacement output = %q, want replaced block", result.Output)
	}
}

func TestEditFileRejectsAmbiguousMultilineOldText(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "write_file",
		Input: "notes.txt\nblock-a\nblock-b\nmid\nblock-a\nblock-b",
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\nblock-a\nblock-b\n<<<NEW\nreplacement",
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("multiline ambiguity error = %v, want ambiguity rejection", err)
	}
}

func TestEditFileRejectsMalformedBlockPayload(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "edit_file",
		Input: "notes.txt\n<<<OLD\nold text without new marker",
	})
	if err == nil || !strings.Contains(err.Error(), "must contain <<<OLD and <<<NEW blocks") {
		t.Fatalf("missing new marker error = %v, want block format rejection", err)
	}
}

func TestExecCommandParsesQuotedArguments(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	// Test that quoted arguments are preserved as single arguments
	// git commit -m "hello world" should split correctly
	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: `git commit -m "hello world"`,
	})
	// git may fail if not in a repo, but the command parsing should work
	// The error should NOT be about parsing the command
	if err != nil && strings.Contains(err.Error(), "failed to parse command") {
		t.Fatalf("unexpected parse error: %v", err)
	}
	// Verify the command was attempted (not rejected for parsing)
	if !strings.Contains(result.Output, "COMMAND: git commit -m \"hello world\"") {
		t.Fatalf("output should contain the raw command, got: %s", result.Output)
	}
}

func TestExecCommandRejectsCommandChaining(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "ampersand chaining",
			input:   "go test && rm -rf /",
			wantErr: "command chaining",
		},
		{
			name:    "or chaining",
			input:   "go test || echo failed",
			wantErr: "command chaining",
		},
		{
			name:    "semicolon chaining",
			input:   "go test ; rm -rf /",
			wantErr: "command chaining",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executor.Execute(context.Background(), domain.ToolCall{
				Name:  "exec_command",
				Input: tt.input,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestExecCommandBackwardCompatibility(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	// Simple command with glob pattern should still work
	result, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "go test ./...",
	})
	// go test ./... may fail if there are no test files, but parsing should work
	if err != nil && strings.Contains(err.Error(), "failed to parse command") {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !strings.Contains(result.Output, "COMMAND: go test ./...") {
		t.Fatalf("output should contain the raw command, got: %s", result.Output)
	}
}

func TestExecCommandRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor, err := tools.NewExecutor(workspace)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	// Unmatched quote should return an error, not panic
	_, err = executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: `git commit -m "unmatched`,
	})
	if err == nil {
		t.Fatalf("expected error for unmatched quote, got nil")
	}
	// The error should be a parsing error, not a safety policy rejection
	if !strings.Contains(err.Error(), "parse command") && !strings.Contains(err.Error(), "unclosed") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}
