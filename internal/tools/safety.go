package tools

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"zheng-harness/internal/domain"
)

// SafetyPolicy enforces local tool usage boundaries.
type SafetyPolicy struct {
	WorkspaceRoot    string
	AllowedCommands  []string
	AllowedReadRoots []string
	AllowedWriteRoots []string
}

// Validate checks whether the tool call respects the configured safety policy.
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
	case "write_file":
		pathPart := strings.SplitN(call.Input, "\n", 2)[0]
		return p.validatePathPayload(strings.TrimSpace(pathPart), root, p.writeRoots(root))
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
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("command must not be empty")
	}
	allowed := append([]string(nil), p.AllowedCommands...)
	sort.Strings(allowed)
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, fields[0]) {
			return nil
		}
	}
	return fmt.Errorf("command %q is not allowlisted", fields[0])
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
