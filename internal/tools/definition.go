package tools

import (
	"context"
	"time"

	"zheng-harness/internal/domain"
)

// SafetyLevel 对工具的风险级别进行分类。
type SafetyLevel = domain.SafetyLevel

const (
	SafetyLevelLow    = domain.SafetyLevelLow
	SafetyLevelMedium = domain.SafetyLevelMedium
	SafetyLevelHigh   = domain.SafetyLevelHigh
)

// ToolHandler 执行已通过校验的工具调用。
type ToolHandler func(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error)

// ToolDefinition 描述一个内置工具及其执行契约。
type ToolDefinition struct {
	Name           string
	Description    string
	Schema         string
	DefaultTimeout time.Duration
	SafetyLevel    SafetyLevel
	Handler        ToolHandler
}
