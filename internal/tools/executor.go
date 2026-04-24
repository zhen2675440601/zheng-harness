package tools

import (
	"context"
	"fmt"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/tools/adapters"
)

// Executor routes tool calls through the registry and safety policy.
type Executor struct {
	registry *Registry
	policy   SafetyPolicy
}

// NewExecutor constructs a tool executor and registers the built-in toolset.
func NewExecutor(workspaceRoot string) (*Executor, error) {
	registry := NewRegistry()
	policy := SafetyPolicy{
		WorkspaceRoot:     workspaceRoot,
		AllowedCommands:   []string{"go", "git", "pwd", "ls", "dir"},
		AllowedReadRoots:  []string{"."},
		AllowedWriteRoots: []string{"."},
	}

	for _, def := range builtinDefinitions(workspaceRoot) {
		if err := registry.Register(def); err != nil {
			return nil, err
		}
	}

	return &Executor{registry: registry, policy: policy}, nil
}

// Execute implements domain.ToolExecutor.
func (e *Executor) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	def, ok := e.registry.Get(call.Name)
	if !ok {
		return domain.ToolResult{ToolName: call.Name}, fmt.Errorf("tool %q is not registered", call.Name)
	}
	if err := e.policy.Validate(def, call); err != nil {
		return domain.ToolResult{ToolName: call.Name}, err
	}

	timeout := call.Timeout
	if timeout <= 0 {
		timeout = def.DefaultTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return def.Handler(execCtx, call)
}

// Registry exposes the registered definitions for inspection/testing.
func (e *Executor) Registry() *Registry {
	return e.registry
}

func builtinDefinitions(workspaceRoot string) []ToolDefinition {
	fileAdapter := adapters.NewFileAdapter(workspaceRoot)
	searchAdapter := adapters.NewSearchAdapter(workspaceRoot)
	shellAdapter := adapters.NewShellAdapter(workspaceRoot)

	return []ToolDefinition{
		{
			Name:           "list_dir",
			Description:    "List directory contents within the workspace",
			Schema:         `{"type":"string","description":"relative directory path"}`,
			DefaultTimeout: 5 * time.Second,
			SafetyLevel:    SafetyLevelLow,
			Handler:        fileAdapter.ListDir,
		},
		{
			Name:           "read_file",
			Description:    "Read a file within the workspace",
			Schema:         `{"type":"string","description":"relative file path"}`,
			DefaultTimeout: 5 * time.Second,
			SafetyLevel:    SafetyLevelLow,
			Handler:        fileAdapter.ReadFile,
		},
		{
			Name:           "write_file",
			Description:    "Write content to a file within the workspace",
			Schema:         `{"type":"string","description":"first line is relative file path; remaining content is file body"}`,
			DefaultTimeout: 5 * time.Second,
			SafetyLevel:    SafetyLevelMedium,
			Handler:        fileAdapter.WriteFile,
		},
		{
			Name:           "grep_search",
			Description:    "Search text under the workspace",
			Schema:         `{"type":"string","description":"search term or regular expression literal"}`,
			DefaultTimeout: 5 * time.Second,
			SafetyLevel:    SafetyLevelLow,
			Handler:        searchAdapter.Grep,
		},
		{
			Name:           "exec_command",
			Description:    "Execute an allowlisted local command",
			Schema:         `{"type":"string","description":"command line beginning with an allowlisted executable"}`,
			DefaultTimeout: 10 * time.Second,
			SafetyLevel:    SafetyLevelHigh,
			Handler:        shellAdapter.Exec,
		},
	}
}
