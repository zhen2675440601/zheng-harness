package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/tools/adapters"
)

// Executor 通过注册表与安全策略分发工具调用。
type Executor struct {
	registry *Registry
	policy   SafetyPolicy
}

var defaultAllowedCommands = []string{
	"go", "git", "pwd", "ls", "dir",
	"npm", "node", "npx", "yarn", "pnpm",
	"python", "python3", "pip", "pip3", "uv",
	"make", "cargo", "rustc",
	"docker", "docker-compose",
	"cat", "head", "tail", "echo",
	"mkdir", "cp", "mv", "env", "which", "ctest",
}

type executorOptions struct {
	allowedCommands      []string
	extraAllowedCommands []string
}

// ExecutorOption 用于定制执行器的安全设置。
type ExecutorOption func(*executorOptions)

// WithAllowedCommands 在非空时覆盖默认命令允许列表。
func WithAllowedCommands(commands []string) ExecutorOption {
	return func(opts *executorOptions) {
		opts.allowedCommands = append([]string(nil), commands...)
	}
}

// WithExtraAllowedCommands 将命令追加到当前允许列表。
func WithExtraAllowedCommands(commands []string) ExecutorOption {
	return func(opts *executorOptions) {
		opts.extraAllowedCommands = append(opts.extraAllowedCommands, commands...)
	}
}

// NewExecutor 构造工具执行器并注册内置工具集。
func NewExecutor(workspaceRoot string, options ...ExecutorOption) (*Executor, error) {
	registry := NewRegistry()
	opts := executorOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	policy := SafetyPolicy{
		WorkspaceRoot:     workspaceRoot,
		AllowedCommands:   buildAllowedCommands(opts.allowedCommands, opts.extraAllowedCommands),
		DeniedCommands:    []string{"rm"},
		AllowedDomains:    nil,
		AllowedReadRoots:  []string{"."},
		AllowedWriteRoots: []string{"."},
	}

	for _, def := range builtinDefinitions(workspaceRoot, policy) {
		if err := registry.Register(def); err != nil {
			return nil, err
		}
	}

	return &Executor{registry: registry, policy: policy}, nil
}

// Execute 实现 domain.ToolExecutor。
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

// Registry 暴露已注册定义，便于检查与测试。
func (e *Executor) Registry() *Registry {
	return e.registry
}

// Policy 暴露执行器的安全策略，便于测试与装配。
func (e *Executor) Policy() SafetyPolicy {
	return e.policy
}

func builtinDefinitions(workspaceRoot string, policy SafetyPolicy) []ToolDefinition {
	fileAdapter := adapters.NewFileAdapter(workspaceRoot)
	globAdapter := adapters.NewGlobAdapter(workspaceRoot)
	codeSearchAdapter := adapters.NewCodeSearchAdapter(workspaceRoot)
	searchAdapter := adapters.NewSearchAdapter(workspaceRoot)
	shellAdapter := adapters.NewShellAdapter(workspaceRoot)
	interactiveAdapter := adapters.NewInteractiveAdapter(os.Stdin, os.Stdout)
	webAdapter := adapters.NewWebAdapter(policy.AllowedDomains)

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
			Name:           "edit_file",
			Description:    "Edit file content by replacing one unique text occurrence",
			Schema:         `{"type":"string","description":"first line is relative file path; remaining payload is <<<OLD\n<multiline old text>\n<<<NEW\n<multiline new text>"}`,
			DefaultTimeout: 5 * time.Second,
			SafetyLevel:    SafetyLevelMedium,
			Handler:        fileAdapter.EditFile,
		},
		{
			Name:           "glob",
			Description:    "Find files matching a glob pattern (e.g., **/*.go, src/**/*.ts, *.json)",
			Schema:         `{"type":"string","description":"glob pattern to match files (supports ** for recursive)"}`,
			DefaultTimeout: 5 * time.Second,
			SafetyLevel:    SafetyLevelLow,
			Handler:        globAdapter.Glob,
		},
		{
			Name:           "code_search",
			Description:    "Search source code with language-aware filtering",
			Schema:         `{"pattern": "string (required)", "language": "string (optional)", "output_mode": "string (optional: content|files_with_matches|count)", "max_results": "int (optional, default 50)"}`,
			DefaultTimeout: 10 * time.Second,
			SafetyLevel:    SafetyLevelLow,
			Handler:        codeSearchAdapter.Search,
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
			Name:           "ask_user",
			Description:    "Prompt the CLI user for interactive input",
			Schema:         `{"question": "string (required)", "options": "[]string (optional)"}`,
			DefaultTimeout: 300 * time.Second,
			SafetyLevel:    SafetyLevelLow,
			Handler:        interactiveAdapter.AskUser,
		},
		{
			Name:           "web_fetch",
			Description:    "Fetch a web page over HTTP or HTTPS",
			Schema:         `{"url": "string (required)", "max_length": "int (optional, default 10000)"}`,
			DefaultTimeout: 15 * time.Second,
			SafetyLevel:    SafetyLevelMedium,
			Handler:        webAdapter.Fetch,
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

func buildAllowedCommands(configured []string, extras []string) []string {
	base := defaultAllowedCommands
	if len(configured) > 0 {
		base = configured
	}

	seen := make(map[string]struct{}, len(base)+len(extras))
	allowed := make([]string, 0, len(base)+len(extras))
	for _, command := range append(append([]string(nil), base...), extras...) {
		trimmed := strings.TrimSpace(command)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		allowed = append(allowed, trimmed)
	}
	return allowed
}
