package domain

import "time"

// SafetyLevel 对工具或插件工具的风险级别进行分类。
type SafetyLevel string

const (
	SafetyLevelLow    SafetyLevel = "low"
	SafetyLevelMedium SafetyLevel = "medium"
	SafetyLevelHigh   SafetyLevel = "high"
)

// ToolCall 请求执行一个带显式输入的命名工具。
type ToolCall struct {
	Name    string
	Input   string
	Timeout time.Duration
}

// ToolInfo 是面向提示词暴露的工具定义子集。
type ToolInfo struct {
	Name        string
	Description string
	Schema      string
}

// ToolResult 是工具执行器返回的标准化结果。
type ToolResult struct {
	ToolName string
	Output   string
	Error    string
	Duration time.Duration
}
