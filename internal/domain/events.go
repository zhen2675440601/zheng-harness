package domain

import (
	stdjson "encoding/json"
	"time"
)

// StreamingEventType represents the category of streaming event
type StreamingEventType string

const (
	EventTokenDelta      StreamingEventType = "token_delta"      // Incremental text chunk
	EventToolStart       StreamingEventType = "tool_start"       // Tool call initiated
	EventToolEnd         StreamingEventType = "tool_end"         // Tool call completed
	EventStepComplete    StreamingEventType = "step_complete"    // Step finished (with Step summary)
	EventError           StreamingEventType = "error"            // Error occurred
	EventSessionComplete StreamingEventType = "session_complete" // Session finished
)

// StreamingEvent is the unified event structure for all streaming outputs
type StreamingEvent struct {
	Type      StreamingEventType `json:"type"`
	StepIndex int                `json:"step_index,omitempty"`
	Payload   EventPayload       `json:"payload"`
	Timestamp time.Time          `json:"timestamp"`
}

// EventPayload keeps raw JSON payload bytes while avoiding dynamic map types.
type EventPayload []byte

func (p EventPayload) MarshalJSON() ([]byte, error) {
	if len(p) == 0 {
		return []byte("null"), nil
	}
	return p, nil
}

func (p *EventPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return nil
	}
	*p = append((*p)[:0], data...)
	return nil
}

// TokenDeltaPayload carries incremental text content
type TokenDeltaPayload struct {
	Content string `json:"content"`
}

// ToolStartPayload indicates a tool call has started
type ToolStartPayload struct {
	ToolName string `json:"tool_name"`
	Input    string `json:"input"`
}

// ToolEndPayload indicates a tool call has completed
type ToolEndPayload struct {
	ToolName string `json:"tool_name"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}

// StepCompletePayload carries step completion summary
type StepCompletePayload struct {
	StepSummary string `json:"step_summary"`
}

// ErrorPayload carries error information
type ErrorPayload struct {
	Message string `json:"message"`
}

// SessionCompletePayload carries final session status
type SessionCompletePayload struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"` // "success", "failed", "verification_failed", "timeout", "cancelled"
}

// NewStreamingEvent creates a new streaming event with timestamp
func NewStreamingEvent(eventType StreamingEventType, stepIndex int, payload any) (*StreamingEvent, error) {
	payloadBytes, err := stdjson.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &StreamingEvent{
		Type:      eventType,
		StepIndex: stepIndex,
		Payload:   EventPayload(payloadBytes),
		Timestamp: time.Now(),
	}, nil
}

// TokenDelta creates a token delta event
func TokenDelta(stepIndex int, content string) (*StreamingEvent, error) {
	return NewStreamingEvent(EventTokenDelta, stepIndex, TokenDeltaPayload{Content: content})
}

// ToolStart creates a tool start event
func ToolStart(stepIndex int, toolName, input string) (*StreamingEvent, error) {
	return NewStreamingEvent(EventToolStart, stepIndex, ToolStartPayload{
		ToolName: toolName,
		Input:    input,
	})
}

// ToolEnd creates a tool end event
func ToolEnd(stepIndex int, toolName, output, err string) (*StreamingEvent, error) {
	return NewStreamingEvent(EventToolEnd, stepIndex, ToolEndPayload{
		ToolName: toolName,
		Output:   output,
		Error:    err,
	})
}

// StepComplete creates a step complete event
func StepComplete(stepIndex int, summary string) (*StreamingEvent, error) {
	return NewStreamingEvent(EventStepComplete, stepIndex, StepCompletePayload{StepSummary: summary})
}

// Error creates an error event
func Error(stepIndex int, message string) (*StreamingEvent, error) {
	return NewStreamingEvent(EventError, stepIndex, ErrorPayload{Message: message})
}

// SessionComplete creates a session complete event
func SessionComplete(sessionID, status string) (*StreamingEvent, error) {
	return NewStreamingEvent(EventSessionComplete, 0, SessionCompletePayload{
		SessionID: sessionID,
		Status:    status,
	})
}

// GetPayload decodes the payload into the given struct
func (e *StreamingEvent) GetPayload(v any) error {
	return stdjson.Unmarshal(e.Payload, v)
}
