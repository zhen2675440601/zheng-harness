package domain

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestStreamingEventMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	timestamp := time.Unix(1700000000, 123456789).UTC()

	tests := []struct {
		name    string
		event   StreamingEvent
		payload any
	}{
		{
			name: "token delta",
			event: mustStreamingEvent(t, EventTokenDelta, 1, TokenDeltaPayload{Content: "hello"}, timestamp),
			payload: TokenDeltaPayload{Content: "hello"},
		},
		{
			name: "tool start",
			event: mustStreamingEvent(t, EventToolStart, 2, ToolStartPayload{ToolName: "grep", Input: "pattern"}, timestamp),
			payload: ToolStartPayload{ToolName: "grep", Input: "pattern"},
		},
		{
			name: "tool end",
			event: mustStreamingEvent(t, EventToolEnd, 2, ToolEndPayload{ToolName: "grep", Output: "match", Error: ""}, timestamp),
			payload: ToolEndPayload{ToolName: "grep", Output: "match", Error: ""},
		},
		{
			name: "step complete",
			event: mustStreamingEvent(t, EventStepComplete, 3, StepCompletePayload{StepSummary: "finished step"}, timestamp),
			payload: StepCompletePayload{StepSummary: "finished step"},
		},
		{
			name: "error",
			event: mustStreamingEvent(t, EventError, 4, ErrorPayload{Message: "boom"}, timestamp),
			payload: ErrorPayload{Message: "boom"},
		},
		{
			name: "session complete",
			event: mustStreamingEvent(t, EventSessionComplete, 0, SessionCompletePayload{SessionID: "session-1", Status: "success"}, timestamp),
			payload: SessionCompletePayload{SessionID: "session-1", Status: "success"},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := json.Marshal(tc.event)
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}

			var got StreamingEvent
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal event: %v", err)
			}

			if got.Type != tc.event.Type {
				t.Fatalf("Type = %q, want %q", got.Type, tc.event.Type)
			}
			if got.StepIndex != tc.event.StepIndex {
				t.Fatalf("StepIndex = %d, want %d", got.StepIndex, tc.event.StepIndex)
			}
			if !got.Timestamp.Equal(tc.event.Timestamp) {
				t.Fatalf("Timestamp = %s, want %s", got.Timestamp, tc.event.Timestamp)
			}

			decoded := newPayloadValue(tc.payload)
			if err := got.GetPayload(decoded); err != nil {
				t.Fatalf("decode payload: %v", err)
			}

			if !reflect.DeepEqual(reflect.Indirect(reflect.ValueOf(decoded)).Interface(), tc.payload) {
				t.Fatalf("payload = %#v, want %#v", reflect.Indirect(reflect.ValueOf(decoded)).Interface(), tc.payload)
			}

			if !bytes.Equal(got.Payload, tc.event.Payload) {
				t.Fatalf("Payload = %s, want %s", string(got.Payload), string(tc.event.Payload))
			}
		})
	}
}

func TestStreamingEventOrdering(t *testing.T) {
	t.Parallel()

	events := []*StreamingEvent{
		mustEventFromFactory(t, func() (*StreamingEvent, error) { return TokenDelta(1, "first token") }),
		mustEventFromFactory(t, func() (*StreamingEvent, error) { return ToolStart(1, "grep", "needle") }),
		mustEventFromFactory(t, func() (*StreamingEvent, error) { return ToolEnd(1, "grep", "found", "") }),
		mustEventFromFactory(t, func() (*StreamingEvent, error) { return StepComplete(1, "step done") }),
	}

	for i := 1; i < len(events); i++ {
		prev := events[i-1]
		curr := events[i]

		if curr.StepIndex != prev.StepIndex {
			t.Fatalf("event %d step index = %d, want %d", i, curr.StepIndex, prev.StepIndex)
		}
		if curr.Timestamp.Before(prev.Timestamp) {
			t.Fatalf("event %d timestamp %s is before previous %s", i, curr.Timestamp, prev.Timestamp)
		}
	}
}

func TestStreamingEventGetPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		factory func() (*StreamingEvent, error)
		want    any
	}{
		{
			name:    "token delta",
			factory: func() (*StreamingEvent, error) { return TokenDelta(1, "hello") },
			want:    TokenDeltaPayload{Content: "hello"},
		},
		{
			name:    "tool start",
			factory: func() (*StreamingEvent, error) { return ToolStart(1, "grep", "needle") },
			want:    ToolStartPayload{ToolName: "grep", Input: "needle"},
		},
		{
			name:    "tool end",
			factory: func() (*StreamingEvent, error) { return ToolEnd(1, "grep", "found", "") },
			want:    ToolEndPayload{ToolName: "grep", Output: "found", Error: ""},
		},
		{
			name:    "step complete",
			factory: func() (*StreamingEvent, error) { return StepComplete(1, "done") },
			want:    StepCompletePayload{StepSummary: "done"},
		},
		{
			name:    "error",
			factory: func() (*StreamingEvent, error) { return Error(1, "boom") },
			want:    ErrorPayload{Message: "boom"},
		},
		{
			name:    "session complete",
			factory: func() (*StreamingEvent, error) { return SessionComplete("session-1", "success") },
			want:    SessionCompletePayload{SessionID: "session-1", Status: "success"},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			event := mustEventFromFactory(t, tc.factory)
			decoded := newPayloadValue(tc.want)

			if err := event.GetPayload(decoded); err != nil {
				t.Fatalf("GetPayload() error = %v", err)
			}

			got := reflect.Indirect(reflect.ValueOf(decoded)).Interface()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("GetPayload() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func mustStreamingEvent(t *testing.T, eventType StreamingEventType, stepIndex int, payload any, timestamp time.Time) StreamingEvent {
	t.Helper()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return StreamingEvent{
		Type:      eventType,
		StepIndex: stepIndex,
		Payload:   payloadBytes,
		Timestamp: timestamp,
	}
}

func mustNewEvent(t *testing.T, event *StreamingEvent, err error) *StreamingEvent {
	t.Helper()

	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	return event
}

func mustEventFromFactory(t *testing.T, factory func() (*StreamingEvent, error)) *StreamingEvent {
	t.Helper()

	event, err := factory()
	return mustNewEvent(t, event, err)
}

func newPayloadValue(payload any) any {
	return reflect.New(reflect.TypeOf(payload)).Interface()
}
