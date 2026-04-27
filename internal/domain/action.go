package domain

// ActionType identifies the kind of action chosen by the model.
type ActionType string

const (
	ActionTypeRespond ActionType = "respond"
	// ActionTypeToolCall means the runtime should execute a tool invocation.
	ActionTypeToolCall ActionType = "tool_call"
	// ActionTypeRequestInput means progress is blocked on external input; it does not mean verification passed.
	ActionTypeRequestInput ActionType = "request_input"
	// ActionTypeComplete means the task is intended to be complete; it does not request tool execution.
	ActionTypeComplete ActionType = "complete"
)

// Action is the next decision emitted by the model/runtime loop.
type Action struct {
	Type     ActionType
	Summary  string
	ToolCall *ToolCall
	Response string
}
