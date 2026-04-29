package domain

// ActionType 标识模型选择的动作类型。
type ActionType string

const (
	ActionTypeRespond ActionType = "respond"
	// ActionTypeToolCall 表示运行时应执行一次工具调用。
	ActionTypeToolCall ActionType = "tool_call"
	// ActionTypeRequestInput 表示进度被外部输入阻塞；这并不意味着验证已通过。
	ActionTypeRequestInput ActionType = "request_input"
	// ActionTypeComplete 表示任务预期已完成；它不会请求执行工具。
	ActionTypeComplete ActionType = "complete"
)

// Action 是模型/运行时循环输出的下一步决策。
type Action struct {
	Type     ActionType
	Summary  string
	ToolCall *ToolCall
	Response string
}
