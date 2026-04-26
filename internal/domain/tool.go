package domain

import "time"

// ToolCall requests execution of a named tool with explicit input.
type ToolCall struct {
	Name    string
	Input   string
	Timeout time.Duration
}

// ToolInfo is the prompt-facing subset of a tool definition.
type ToolInfo struct {
	Name        string
	Description string
	Schema      string
}

// ToolResult is the normalized outcome returned by a tool executor.
type ToolResult struct {
	ToolName string
	Output   string
	Error    string
	Duration time.Duration
}
