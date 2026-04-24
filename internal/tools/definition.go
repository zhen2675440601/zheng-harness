package tools

import (
	"context"
	"time"

	"zheng-harness/internal/domain"
)

// SafetyLevel classifies the risk profile of a tool.
type SafetyLevel string

const (
	SafetyLevelLow    SafetyLevel = "low"
	SafetyLevelMedium SafetyLevel = "medium"
	SafetyLevelHigh   SafetyLevel = "high"
)

// ToolHandler executes a validated tool call.
type ToolHandler func(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error)

// ToolDefinition describes one built-in tool and its execution contract.
type ToolDefinition struct {
	Name           string
	Description    string
	Schema         string
	DefaultTimeout time.Duration
	SafetyLevel    SafetyLevel
	Handler        ToolHandler
}
