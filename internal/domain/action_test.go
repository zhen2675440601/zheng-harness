package domain

import (
	"encoding/json"
	"testing"
)

func TestActionTypeValues(t *testing.T) {
	tests := []struct {
		name string
		got  ActionType
		want string
	}{
		{name: "respond", got: ActionTypeRespond, want: "respond"},
		{name: "tool call", got: ActionTypeToolCall, want: "tool_call"},
		{name: "request input", got: ActionTypeRequestInput, want: "request_input"},
		{name: "complete", got: ActionTypeComplete, want: "complete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.got) != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, tt.got)
			}
		})
	}
}

func TestActionTypeJSONParsing(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    ActionType
	}{
		{name: "respond", payload: `{"type":"respond"}`, want: ActionTypeRespond},
		{name: "tool call", payload: `{"type":"tool_call"}`, want: ActionTypeToolCall},
		{name: "request input", payload: `{"type":"request_input"}`, want: ActionTypeRequestInput},
		{name: "complete", payload: `{"type":"complete"}`, want: ActionTypeComplete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				Type ActionType `json:"type"`
			}

			if err := json.Unmarshal([]byte(tt.payload), &got); err != nil {
				t.Fatalf("unmarshal action type: %v", err)
			}

			if got.Type != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got.Type)
			}
		})
	}
}
