package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
)

func TestModelAdapterNextActionParsesExpandedActionTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		output      string
		wantType    domain.ActionType
		wantSummary string
		wantReply   string
	}{
		{
			name:        "request input",
			output:      `{"type":"request_input","summary":"need approval","response":"Please provide approval to continue."}`,
			wantType:    domain.ActionTypeRequestInput,
			wantSummary: "need approval",
			wantReply:   "Please provide approval to continue.",
		},
		{
			name:        "complete",
			output:      `{"type":"complete","summary":"task complete","response":"All requested work is complete."}`,
			wantType:    domain.ActionTypeComplete,
			wantSummary: "task complete",
			wantReply:   "All requested work is complete.",
		},
		{
			name:        "respond still supported",
			output:      `{"type":"respond","summary":"answer ready","response":"Here is the answer."}`,
			wantType:    domain.ActionTypeRespond,
			wantSummary: "answer ready",
			wantReply:   "Here is the answer.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := &stubProvider{output: tt.output}
			adapter := NewModelAdapter(provider)

			action, err := adapter.NextAction(context.Background(), sampleTask(), sampleSession(), samplePlan(), nil, nil, nil)
			if err != nil {
				t.Fatalf("NextAction: %v", err)
			}
			if action.Type != tt.wantType {
				t.Fatalf("action type = %q, want %q", action.Type, tt.wantType)
			}
			if action.Summary != tt.wantSummary {
				t.Fatalf("summary = %q, want %q", action.Summary, tt.wantSummary)
			}
			if action.Response != tt.wantReply {
				t.Fatalf("response = %q, want %q", action.Response, tt.wantReply)
			}
			if action.ToolCall != nil {
				t.Fatalf("tool_call = %#v, want nil", action.ToolCall)
			}
		})
	}
}

func TestModelAdapterNextActionPreservesToolCallAndPromptProtocolContext(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{output: `{"type":"tool_call","summary":"inspect file","tool_call":{"name":"read","input":"README.md","timeout":"2s"}}`}
	adapter := NewModelAdapter(provider)

	action, err := adapter.NextAction(context.Background(), sampleTask(), sampleSession(), samplePlan(), nil, nil, []domain.ToolInfo{{Name: "read", Description: "read file", Schema: "{}"}})
	if err != nil {
		t.Fatalf("NextAction: %v", err)
	}
	if action.Type != domain.ActionTypeToolCall {
		t.Fatalf("action type = %q, want %q", action.Type, domain.ActionTypeToolCall)
	}
	if action.ToolCall == nil {
		t.Fatal("tool_call missing")
	}
	if action.ToolCall.Timeout != 2*time.Second {
		t.Fatalf("timeout = %v, want 2s", action.ToolCall.Timeout)
	}
	if !strings.Contains(provider.lastInput, `"type":"research"`) {
		t.Fatalf("provider input missing task type context: %s", provider.lastInput)
	}
	if !strings.Contains(provider.lastInput, `request_input`) || !strings.Contains(provider.lastInput, `complete`) {
		t.Fatalf("provider input missing expanded action contract: %s", provider.lastInput)
	}
}

func TestModelAdapterNextActionRejectsInvalidExpandedActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		wantErr string
	}{
		{
			name:    "request_input missing response",
			output:  `{"type":"request_input","summary":"need approval"}`,
			wantErr: "request_input action missing response",
		},
		{
			name:    "complete missing response",
			output:  `{"type":"complete","summary":"done"}`,
			wantErr: "complete action missing response",
		},
		{
			name:    "unsupported action",
			output:  `{"type":"delegate","summary":"nope"}`,
			wantErr: `unsupported action type "delegate"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			adapter := NewModelAdapter(&stubProvider{output: tt.output})
			_, err := adapter.NextAction(context.Background(), sampleTask(), sampleSession(), samplePlan(), nil, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestModelAdapterGenerateStreamsProviderOutputWhenEmitterPresent(t *testing.T) {
	t.Parallel()

	provider := &streamStubProvider{events: []llm.StreamingEvent{
		mustTokenEvent(t, 0, "{\"type\":\"respond\",\"summary\":\"hello"),
		mustTokenEvent(t, 0, "\",\"response\":\"streamed reply\"}"),
		mustSessionCompleteEvent(t, "", "success"),
	}}
	adapter := NewModelAdapter(provider)

	var streamed []domain.StreamingEvent
	ctx := withStreamEventEmitter(context.Background(), func(event domain.StreamingEvent) error {
		streamed = append(streamed, event)
		return nil
	})

	action, err := adapter.NextAction(ctx, sampleTask(), sampleSession(), samplePlan(), nil, nil, nil)
	if err != nil {
		t.Fatalf("NextAction: %v", err)
	}
	if !provider.streamCalled {
		t.Fatal("expected provider.Stream to be used")
	}
	if provider.generateCalled {
		t.Fatal("Generate should not be used when streaming emitter is present")
	}
	if action.Type != domain.ActionTypeRespond || action.Response != "streamed reply" {
		t.Fatalf("action = %+v, want streamed respond action", action)
	}
	if len(streamed) != 2 {
		t.Fatalf("streamed events = %d, want 2 token events", len(streamed))
	}
	for i, event := range streamed {
		if event.Type != domain.EventTokenDelta {
			t.Fatalf("event[%d].type = %q, want token_delta", i, event.Type)
		}
	}
}

func TestModelAdapterGenerateStreamFallbackUsesGenerateWhenProviderStreamsViaFallback(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{output: `{"type":"respond","summary":"hello","response":"fallback reply"}`}
	adapter := NewModelAdapter(provider)

	var streamed []domain.StreamingEvent
	ctx := withStreamEventEmitter(context.Background(), func(event domain.StreamingEvent) error {
		streamed = append(streamed, event)
		return nil
	})

	action, err := adapter.NextAction(ctx, sampleTask(), sampleSession(), samplePlan(), nil, nil, nil)
	if err != nil {
		t.Fatalf("NextAction: %v", err)
	}
	if action.Response != "fallback reply" {
		t.Fatalf("response = %q, want fallback reply", action.Response)
	}
	if len(streamed) != 1 {
		t.Fatalf("streamed events = %d, want 1 token event", len(streamed))
	}
	if streamed[0].Type != domain.EventTokenDelta {
		t.Fatalf("event type = %q, want token_delta", streamed[0].Type)
	}
	if provider.lastInput == "" {
		t.Fatal("expected fallback stream to invoke Generate")
	}
}

func TestModelAdapterGenerateStreamPropagatesTokenDecodeError(t *testing.T) {
	t.Parallel()

	provider := &streamStubProvider{events: []llm.StreamingEvent{{
		Type:      domain.EventTokenDelta,
		StepIndex: 0,
		Payload:   domain.EventPayload([]byte("{")),
	}, mustSessionCompleteEvent(t, "", "success")}}
	adapter := NewModelAdapter(provider)

	ctx := withStreamEventEmitter(context.Background(), func(event domain.StreamingEvent) error { return nil })
	_, err := adapter.NextAction(ctx, sampleTask(), sampleSession(), samplePlan(), nil, nil, nil)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode stub token delta") {
		t.Fatalf("error = %q, want token decode context", err)
	}
}

type stubProvider struct {
	output    string
	lastInput string
}

func (s *stubProvider) Name() string  { return "stub" }
func (s *stubProvider) Model() string { return "stub-model" }

func (s *stubProvider) Generate(_ context.Context, request llm.Request) (llm.Response, error) {
	s.lastInput = request.Input
	return llm.Response{Model: s.Model(), Output: s.output, StopReason: "stop"}, nil
}

func (s *stubProvider) Stream(ctx context.Context, request llm.Request, emit func(llm.StreamingEvent) error) error {
	return llm.StreamFallback(ctx, s.Generate, request, emit)
}

type streamStubProvider struct {
	events         []llm.StreamingEvent
	streamErr      error
	generateCalled bool
	streamCalled   bool
	lastInput      string
}

func (s *streamStubProvider) Name() string  { return "stub" }
func (s *streamStubProvider) Model() string { return "stub-model" }

func (s *streamStubProvider) Generate(_ context.Context, request llm.Request) (llm.Response, error) {
	s.generateCalled = true
	s.lastInput = request.Input
	return llm.Response{}, errors.New("unexpected Generate call")
}

func (s *streamStubProvider) Stream(_ context.Context, request llm.Request, emit func(llm.StreamingEvent) error) error {
	s.streamCalled = true
	s.lastInput = request.Input
	if s.streamErr != nil {
		return s.streamErr
	}
	for _, event := range s.events {
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}

func mustTokenEvent(t *testing.T, stepIndex int, content string) llm.StreamingEvent {
	t.Helper()
	event, err := domain.TokenDelta(stepIndex, content)
	if err != nil {
		t.Fatalf("token event: %v", err)
	}
	return *event
}

func mustSessionCompleteEvent(t *testing.T, sessionID, status string) llm.StreamingEvent {
	t.Helper()
	event, err := domain.SessionComplete(sessionID, status)
	if err != nil {
		t.Fatalf("session complete event: %v", err)
	}
	return *event
}

func sampleTask() domain.Task {
	return domain.Task{
		ID:                 "task-1",
		Description:        "review evidence",
		Goal:               "decide next step",
		Category:           domain.TaskCategoryResearch,
		ProtocolHint:       "evidence_based",
		VerificationPolicy: "evidence_based",
	}
}

func sampleSession() domain.Session {
	return domain.Session{ID: "session-1", Status: domain.SessionStatusRunning}
}

func samplePlan() domain.Plan {
	return domain.Plan{ID: "plan-1", Summary: "collect evidence"}
}
