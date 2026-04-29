package adapters

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"zheng-harness/internal/domain"

	"github.com/kballard/go-shellquote"
)

// ShellAdapter 在工作区内执行允许列表中的命令。
type ShellAdapter struct {
	workspaceRoot string
}

// NewShellAdapter 构造一个限定在工作区内的 shell 适配器。
func NewShellAdapter(workspaceRoot string) ShellAdapter {
	return ShellAdapter{workspaceRoot: workspaceRoot}
}

func (a ShellAdapter) Exec(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	commandLine := strings.TrimSpace(call.Input)
	if commandLine == "" {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("command must not be empty")
	}
	fields, err := shellquote.Split(commandLine)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("failed to parse command: %w", err)
	}
	if len(fields) == 0 {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("command must not be empty")
	}
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	cmd.Dir = filepath.Clean(a.workspaceRoot)
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	structured := strings.Join([]string{
		"COMMAND: " + commandLine,
		"EXIT_CODE: " + strconv.Itoa(exitCode),
		"OUTPUT_BEGIN",
		string(output),
		"OUTPUT_END",
	}, "\n")

	result := domain.ToolResult{ToolName: call.Name, Output: structured, Duration: time.Since(start)}
	if err != nil {
		return result, err
	}
	return result, nil
}
