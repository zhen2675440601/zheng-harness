package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

// ShellAdapter executes allowlisted commands in the workspace.
type ShellAdapter struct {
	workspaceRoot string
}

// NewShellAdapter constructs a shell adapter scoped to a workspace.
func NewShellAdapter(workspaceRoot string) ShellAdapter {
	return ShellAdapter{workspaceRoot: workspaceRoot}
}

func (a ShellAdapter) Exec(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	fields := strings.Fields(strings.TrimSpace(call.Input))
	if len(fields) == 0 {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("command must not be empty")
	}
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	cmd.Dir = filepath.Clean(a.workspaceRoot)
	output, err := cmd.CombinedOutput()
	result := domain.ToolResult{ToolName: call.Name, Output: string(output), Duration: time.Since(start)}
	if err != nil {
		return result, err
	}
	return result, nil
}
