package tools

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/tools/adapters"
)

// SafetyPolicy 强制执行本地工具使用边界。
type SafetyPolicy struct {
	WorkspaceRoot     string
	AllowedCommands   []string
	DeniedCommands    []string
	AllowedDomains    []string
	AllowedReadRoots  []string
	AllowedWriteRoots []string
	AllowedPluginPaths []string
	PluginCapabilities []string
}

// Validate 检查工具调用是否遵循配置好的安全策略。
func (p SafetyPolicy) Validate(def ToolDefinition, call domain.ToolCall) error {
	root, err := filepath.Abs(p.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}

	if p.WorkspaceRoot == "" {
		return fmt.Errorf("workspace root must not be empty")
	}

	switch def.Name {
	case "read_file", "list_dir", "grep_search":
		return p.validatePathPayload(call.Input, root, p.readRoots(root))
	case "write_file", "edit_file":
		pathPart := strings.SplitN(call.Input, "\n", 2)[0]
		return p.validatePathPayload(strings.TrimSpace(pathPart), root, p.writeRoots(root))
	case "web_fetch":
		return p.validateWebFetchPayload(call.Input)
	case "exec_command":
		return p.validateCommand(call.Input)
	default:
		return nil
	}
}

func (p SafetyPolicy) validatePathPayload(raw string, workspaceRoot string, allowedRoots []string) error {
	target := strings.TrimSpace(raw)
	if target == "" {
		return fmt.Errorf("path payload must not be empty")
	}
	resolved, err := filepath.Abs(filepath.Join(workspaceRoot, target))
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	cleanResolved := filepath.Clean(resolved)
	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root)
		if cleanResolved == cleanRoot || strings.HasPrefix(cleanResolved, cleanRoot+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("path %q escapes allowed roots", target)
}

func (p SafetyPolicy) validateCommand(raw string) error {
	command := strings.TrimSpace(raw)
	if command == "" {
		return fmt.Errorf("command must not be empty")
	}
	// 在执行任何解析前拒绝命令链式操作符。
	if strings.Contains(command, "&&") || strings.Contains(command, "||") || strings.Contains(command, ";") {
		return fmt.Errorf("command chaining (&&, ||, ;) is not allowed")
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("command must not be empty")
	}
	allowed := append([]string(nil), p.AllowedCommands...)
	sort.Strings(allowed)
	executable := fields[0]
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, executable) {
			denied := append([]string(nil), p.DeniedCommands...)
			sort.Strings(denied)
			for _, blocked := range denied {
				if strings.EqualFold(blocked, executable) {
					return fmt.Errorf("command is explicitly denied for safety")
				}
			}
			return nil
		}
	}
	return fmt.Errorf("command %q is not allowlisted", executable)
}

func (p SafetyPolicy) validateWebFetchPayload(raw string) error {
	input, err := adapters.ParseWebFetchInput(raw)
	if err != nil {
		return err
	}
	parsed, err := adapters.ValidateWebFetchURL(input.URL)
	if err != nil {
		return err
	}
	return p.validateAllowedDomain(parsed)
}

// ValidatePluginPath 检查插件路径是否落在允许的插件目录内。
func (p SafetyPolicy) ValidatePluginPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("plugin path must not be empty")
	}
	if len(p.AllowedPluginPaths) == 0 {
		return nil
	}
	if p.WorkspaceRoot == "" {
		return fmt.Errorf("workspace root must not be empty")
	}
	root, err := filepath.Abs(p.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}
	return p.validatePathPayload(path, root, p.pluginRoots(root))
}

// DeclaresPluginCapability reports whether the capability is declared by policy.
// Empty declarations mean capabilities are currently unrestricted.
func (p SafetyPolicy) DeclaresPluginCapability(capability string) bool {
	if len(p.PluginCapabilities) == 0 {
		return true
	}
	trimmed := strings.TrimSpace(capability)
	if trimmed == "" {
		return false
	}
	for _, candidate := range p.PluginCapabilities {
		if strings.EqualFold(strings.TrimSpace(candidate), trimmed) {
			return true
		}
	}
	return false
}

// ValidatePluginCapabilities checks whether every declared plugin capability is allowed by policy.
// When policy capabilities are configured, plugins must declare at least one capability and every
// declared capability must be present in the allowlist.
func (p SafetyPolicy) ValidatePluginCapabilities(capabilities []string) error {
	if len(p.PluginCapabilities) == 0 {
		return nil
	}
	if len(capabilities) == 0 {
		return fmt.Errorf("plugin capabilities must be declared")
	}
	for _, capability := range capabilities {
		trimmed := strings.TrimSpace(capability)
		if trimmed == "" {
			return fmt.Errorf("plugin capability must not be empty")
		}
		if !p.DeclaresPluginCapability(trimmed) {
			return fmt.Errorf("plugin capability %q is not allowed", trimmed)
		}
	}
	return nil
}

func (p SafetyPolicy) validateAllowedDomain(parsed interface{ Hostname() string }) error {
	if len(p.AllowedDomains) == 0 {
		return nil
	}
	hostname := strings.ToLower(parsed.Hostname())
	for _, domain := range p.AllowedDomains {
		if strings.EqualFold(strings.TrimSpace(domain), hostname) {
			return nil
		}
	}
	return fmt.Errorf("web_fetch domain %q is not allowed", parsed.Hostname())
}

func (p SafetyPolicy) readRoots(workspaceRoot string) []string {
	if len(p.AllowedReadRoots) == 0 {
		return []string{workspaceRoot}
	}
	return p.resolveRoots(workspaceRoot, p.AllowedReadRoots)
}

func (p SafetyPolicy) writeRoots(workspaceRoot string) []string {
	if len(p.AllowedWriteRoots) == 0 {
		return []string{workspaceRoot}
	}
	return p.resolveRoots(workspaceRoot, p.AllowedWriteRoots)
}

func (p SafetyPolicy) pluginRoots(workspaceRoot string) []string {
	return p.resolveRoots(workspaceRoot, p.AllowedPluginPaths)
}

func (p SafetyPolicy) resolveRoots(workspaceRoot string, roots []string) []string {
	resolved := make([]string, 0, len(roots))
	for _, root := range roots {
		path := root
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspaceRoot, path)
		}
		resolved = append(resolved, filepath.Clean(path))
	}
	return resolved
}
