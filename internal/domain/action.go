package domain

// ActionType identifies the kind of action chosen by the model.
type ActionType string

const (
	ActionTypeRespond  ActionType = "respond"
	ActionTypeToolCall ActionType = "tool_call"
)

// Action is the next decision emitted by the model/runtime loop.
type Action struct {
	Type     ActionType
	Summary  string
	ToolCall *ToolCall
	Response string
}
