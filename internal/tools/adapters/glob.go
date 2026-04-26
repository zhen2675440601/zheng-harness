package adapters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v2"

	"zheng-harness/internal/domain"
)

// GlobAdapter performs workspace-scoped file pattern matching.
type GlobAdapter struct {
	workspaceRoot string
}

// NewGlobAdapter constructs a workspace-scoped glob adapter.
func NewGlobAdapter(workspaceRoot string) GlobAdapter {
	return GlobAdapter{workspaceRoot: workspaceRoot}
}

func (a GlobAdapter) Glob(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	pattern := strings.TrimSpace(call.Input)
	if pattern == "" {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("glob pattern must not be empty")
	}
	if err := validateGlobPattern(pattern); err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	root, err := filepath.Abs(a.workspaceRoot)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	globPattern := filepath.Join(root, filepath.FromSlash(pattern))
	matches, err := doublestar.Glob(globPattern)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	results := make([]string, 0, len(matches))
	for _, match := range matches {
		select {
		case <-ctx.Done():
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, ctx.Err()
		default:
		}

		resolved, err := filepath.Abs(match)
		if err != nil {
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
		}
		if !isWithinWorkspace(root, resolved) {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil {
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
		}
		results = append(results, filepath.ToSlash(rel))
	}

	return domain.ToolResult{ToolName: call.Name, Output: strings.Join(results, "\n"), Duration: time.Since(start)}, nil
}

func validateGlobPattern(pattern string) error {
	if filepath.IsAbs(pattern) {
		return fmt.Errorf("glob pattern %q escapes workspace", pattern)
	}
	normalized := strings.ReplaceAll(pattern, `\`, "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return fmt.Errorf("glob pattern %q escapes workspace", pattern)
		}
	}
	return nil
}

func isWithinWorkspace(root string, target string) bool {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)
	return cleanTarget == cleanRoot || strings.HasPrefix(cleanTarget, cleanRoot+string(filepath.Separator))
}
