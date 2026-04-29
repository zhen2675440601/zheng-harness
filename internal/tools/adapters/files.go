package adapters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

// FileAdapter 在固定工作区根目录下执行文件与目录操作。
type FileAdapter struct {
	workspaceRoot string
}

// NewFileAdapter 构造一个限定在单个工作区内的文件适配器。
func NewFileAdapter(workspaceRoot string) FileAdapter {
	return FileAdapter{workspaceRoot: workspaceRoot}
}

func (a FileAdapter) ListDir(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	path, err := a.resolve(call.Input)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, ctx.Err()
		default:
		}
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	return domain.ToolResult{ToolName: call.Name, Output: strings.Join(lines, "\n"), Duration: time.Since(start)}, nil
}

func (a FileAdapter) ReadFile(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	path, err := a.resolve(call.Input)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	return domain.ToolResult{ToolName: call.Name, Output: string(content), Duration: time.Since(start)}, nil
}

func (a FileAdapter) WriteFile(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	parts := strings.SplitN(call.Input, "\n", 2)
	if len(parts) != 2 {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("write_file input must contain path and body separated by newline")
	}
	path, err := a.resolve(parts[0])
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	if err := os.WriteFile(path, []byte(parts[1]), 0o644); err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	return domain.ToolResult{ToolName: call.Name, Output: "ok", Duration: time.Since(start)}, nil
}

func (a FileAdapter) EditFile(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	parts := strings.SplitN(call.Input, "\n", 2)
	if len(parts) != 2 {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("edit_file input must contain path and edit blocks")
	}

	path, err := a.resolve(parts[0])
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	const oldMarker = "<<<OLD\n"
	const newMarker = "\n<<<NEW\n"
	payload := parts[1]
	if !strings.HasPrefix(payload, oldMarker) {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("edit_file input must contain <<<OLD and <<<NEW blocks")
	}
	remainder := strings.TrimPrefix(payload, oldMarker)
	idx := strings.Index(remainder, newMarker)
	if idx == -1 {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("edit_file input must contain <<<OLD and <<<NEW blocks")
	}

	oldText := remainder[:idx]
	newText := remainder[idx+len(newMarker):]
	if oldText == "" {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("edit_file old text must not be empty")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	body := string(content)
	occurrences := strings.Count(body, oldText)
	if occurrences == 0 {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("edit_file old text not found in target file")
	}
	if occurrences > 1 {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("edit_file old text is ambiguous (%d matches)", occurrences)
	}

	updated := strings.Replace(body, oldText, newText, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	return domain.ToolResult{ToolName: call.Name, Output: "ok", Duration: time.Since(start)}, nil
}

func (a FileAdapter) resolve(raw string) (string, error) {
	root, err := filepath.Abs(a.workspaceRoot)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "."
	}
	target := filepath.Clean(filepath.Join(root, trimmed))
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", raw)
	}
	return target, nil
}
