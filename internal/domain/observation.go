package domain

// Observation is the normalized runtime understanding after an action.
type Observation struct {
	Summary       string
	ToolResult    *ToolResult
	FinalResponse string
}
