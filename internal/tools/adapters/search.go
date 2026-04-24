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

// SearchAdapter performs simple textual searches in workspace files.
type SearchAdapter struct {
	workspaceRoot string
}

// NewSearchAdapter constructs a workspace-scoped search adapter.
func NewSearchAdapter(workspaceRoot string) SearchAdapter {
	return SearchAdapter{workspaceRoot: workspaceRoot}
}

func (a SearchAdapter) Grep(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	term := strings.TrimSpace(call.Input)
	if term == "" {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("search term must not be empty")
	}

	root, err := filepath.Abs(a.workspaceRoot)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	var matches []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".sisyphus" {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(content), term) {
			rel, _ := filepath.Rel(root, path)
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	return domain.ToolResult{ToolName: call.Name, Output: strings.Join(matches, "\n"), Duration: time.Since(start)}, nil
}
