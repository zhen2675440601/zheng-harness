package runtime_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
)

func TestEventChannelBufferOverflow(t *testing.T) {
	t.Parallel()

	ec := runtime.NewEventChannel(1)
	first, err := domain.StepComplete(1, "first")
	if err != nil {
		t.Fatalf("first event: %v", err)
	}
	second, err := domain.StepComplete(2, "second")
	if err != nil {
		t.Fatalf("second event: %v", err)
	}

	if err := ec.Emit(*first); err != nil {
		t.Fatalf("emit first: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- ec.Emit(*second)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("emit second: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("second emit blocked on full buffer")
	}

	select {
	case got := <-ec.Events():
		if got.StepIndex != 1 {
			t.Fatalf("step index = %d, want 1", got.StepIndex)
		}
	default:
		t.Fatal("expected first event in buffer")
	}

	select {
	case <-ec.Events():
		t.Fatal("expected overflow event to be dropped")
	default:
	}

	ec.Close()
	for range ec.Events() {
	}
	if err := ec.Emit(*first); err == nil {
		t.Fatal("expected closed channel error")
	}
	if err := ec.Emit(*first); err.Error() != "runtime event channel closed" {
		t.Fatalf("closed emit error = %v, want runtime event channel closed", err)
	}
	if _, ok := <-ec.Events(); ok {
		t.Fatal("expected closed channel to be closed")
	}
}

func TestEventChannelCloseDetection(t *testing.T) {
	t.Parallel()

	ec := runtime.NewEventChannel(1)
	ec.Close()
	if _, ok := <-ec.Events(); ok {
		t.Fatal("expected closed events channel")
	}
}

func TestEventChannelEmitAfterCloseError(t *testing.T) {
	t.Parallel()

	ec := runtime.NewEventChannel(1)
	event, err := domain.StepComplete(1, "done")
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	ec.Close()

	err = ec.Emit(*event)
	if err == nil {
		t.Fatal("expected emit after close error")
	}
	if got := err.Error(); got != "runtime event channel closed" {
		t.Fatalf("emit after close error = %q, want runtime event channel closed", got)
	}
}

func TestEventChannelConcurrentEmit(t *testing.T) {
	t.Parallel()

	const total = 128
	ec := runtime.NewEventChannel(total)

	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			event, err := domain.StepComplete(index+1, "concurrent")
			if err != nil {
				t.Errorf("event %d: %v", index, err)
				return
			}
			if err := ec.Emit(*event); err != nil {
				t.Errorf("emit %d: %v", index, err)
			}
		}(i)
	}
	wg.Wait()

	count := 0
	for {
		select {
		case <-ec.Events():
			count++
		default:
			if count != total {
				t.Fatalf("received %d events, want %d", count, total)
			}
			ec.Close()
			return
		}
	}
}

func TestRunStreamEmitsEventsAndClosesChannel(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	task := domain.Task{ID: "task-stream", Description: "inspect repository", Goal: "done", CreatedAt: fixedTime}

	engine := runtime.Engine{
		Model: &streamingFakeModel{
			plans: []domain.Plan{{ID: "plan-1", TaskID: task.ID, Summary: "plan summary"}},
			actions: []domain.Action{{
				Type:    domain.ActionTypeToolCall,
				Summary: "read repository metadata",
				ToolCall: &domain.ToolCall{Name: "read_file", Input: "README.md"},
			}},
			observations: []domain.Observation{{Summary: "observation summary", FinalResponse: "done"}},
		},
		Tools:          &streamingFakeToolExecutor{result: domain.ToolResult{ToolName: "read_file", Output: "contents"}},
		Memory:         &streamingFakeMemoryStore{},
		Sessions:       &streamingFakeSessionStore{},
		Verifier:       &streamingFakeVerifier{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "ok"}},
		Clock:          streamingFixedClock(fixedTime),
		MaxSteps:       1,
		MaxRetries:     0,
		SessionTimeout: time.Minute,
	}

	events, session, plan, steps, err := engine.RunStream(context.Background(), task)
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}
	if events == nil {
		t.Fatal("expected event channel")
	}
	if session != (domain.Session{}) || plan.ID != "" || plan.TaskID != "" || plan.Summary != "" || len(plan.Steps) != 0 || !plan.CreatedAt.IsZero() || steps != nil {
		t.Fatal("RunStream should return immediately with zero-value results")
	}

	received := make([]domain.StreamingEvent, 0, 5)
	for event := range events.Events() {
		received = append(received, event)
	}

	if len(received) != 5 {
		t.Fatalf("received %d events, want 5", len(received))
	}
	if got := received[0].Type; got != domain.EventStepComplete {
		t.Fatalf("event 0 type = %q, want %q", got, domain.EventStepComplete)
	}
	if got := received[0].StepIndex; got != 0 {
		t.Fatalf("plan event step index = %d, want 0", got)
	}
	if got := received[1].Type; got != domain.EventToolStart {
		t.Fatalf("event 1 type = %q, want %q", got, domain.EventToolStart)
	}
	if got := received[2].Type; got != domain.EventToolEnd {
		t.Fatalf("event 2 type = %q, want %q", got, domain.EventToolEnd)
	}
	if got := received[3].Type; got != domain.EventStepComplete {
		t.Fatalf("event 3 type = %q, want %q", got, domain.EventStepComplete)
	}
	if got := received[4].Type; got != domain.EventSessionComplete {
		t.Fatalf("event 4 type = %q, want %q", got, domain.EventSessionComplete)
	}

	var planPayload domain.StepCompletePayload
	if err := received[0].GetPayload(&planPayload); err != nil {
		t.Fatalf("decode plan payload: %v", err)
	}
	if planPayload.StepSummary != "plan summary" {
		t.Fatalf("plan summary = %q, want plan summary", planPayload.StepSummary)
	}

	var toolStart domain.ToolStartPayload
	if err := received[1].GetPayload(&toolStart); err != nil {
		t.Fatalf("decode tool start payload: %v", err)
	}
	if toolStart.ToolName != "read_file" || toolStart.Input != "README.md" {
		t.Fatalf("tool start payload = %+v, want read_file/README.md", toolStart)
	}

	var toolEnd domain.ToolEndPayload
	if err := received[2].GetPayload(&toolEnd); err != nil {
		t.Fatalf("decode tool end payload: %v", err)
	}
	if toolEnd.ToolName != "read_file" || toolEnd.Output != "contents" {
		t.Fatalf("tool end payload = %+v, want read_file/contents", toolEnd)
	}

	var observationPayload domain.StepCompletePayload
	if err := received[3].GetPayload(&observationPayload); err != nil {
		t.Fatalf("decode observation payload: %v", err)
	}
	if observationPayload.StepSummary != "observation summary" {
		t.Fatalf("observation summary = %q, want observation summary", observationPayload.StepSummary)
	}

	var completePayload domain.SessionCompletePayload
	if err := received[4].GetPayload(&completePayload); err != nil {
		t.Fatalf("decode session complete payload: %v", err)
	}
	if completePayload.Status != string(domain.SessionStatusSuccess) {
		t.Fatalf("session status = %q, want %q", completePayload.Status, domain.SessionStatusSuccess)
	}
}

func TestRunStreamEmitsErrorOnFailure(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 28, 9, 5, 0, 0, time.UTC)
	task := domain.Task{ID: "task-stream-error", Description: "inspect repository", Goal: "done", CreatedAt: fixedTime}

	engine := runtime.Engine{
		Model:          &streamingFakeModel{planErr: errors.New("plan exploded")},
		Tools:          &streamingFakeToolExecutor{},
		Memory:         &streamingFakeMemoryStore{},
		Sessions:       &streamingFakeSessionStore{},
		Verifier:       &streamingFakeVerifier{},
		Clock:          streamingFixedClock(fixedTime),
		MaxSteps:       1,
		MaxRetries:     0,
		SessionTimeout: time.Minute,
	}

	events, _, _, _, err := engine.RunStream(context.Background(), task)
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}

	received := make([]domain.StreamingEvent, 0, 2)
	for event := range events.Events() {
		received = append(received, event)
	}

	if len(received) != 2 {
		t.Fatalf("received %d events, want 2", len(received))
	}
	if received[0].Type != domain.EventError {
		t.Fatalf("first event type = %q, want %q", received[0].Type, domain.EventError)
	}
	if received[1].Type != domain.EventSessionComplete {
		t.Fatalf("second event type = %q, want %q", received[1].Type, domain.EventSessionComplete)
	}

	var errorPayload domain.ErrorPayload
	if err := received[0].GetPayload(&errorPayload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if errorPayload.Message != "plan exploded" {
		t.Fatalf("error message = %q, want plan exploded", errorPayload.Message)
	}
}

type streamingFakeModel struct {
	plans        []domain.Plan
	actions      []domain.Action
	observations []domain.Observation
	planErr      error
	createCalls  int
	actionCalls  int
	observeCalls int
}

func (f *streamingFakeModel) CreatePlan(_ context.Context, _ domain.Task, _ domain.Session, _ []domain.MemoryEntry) (domain.Plan, error) {
	if f.planErr != nil {
		return domain.Plan{}, f.planErr
	}
	if f.createCalls >= len(f.plans) {
		return domain.Plan{}, errors.New("unexpected CreatePlan call")
	}
	plan := f.plans[f.createCalls]
	f.createCalls++
	return plan, nil
}

func (f *streamingFakeModel) NextAction(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ []domain.MemoryEntry, _ []domain.ToolInfo) (domain.Action, error) {
	if f.actionCalls >= len(f.actions) {
		return domain.Action{}, errors.New("unexpected NextAction call")
	}
	action := f.actions[f.actionCalls]
	f.actionCalls++
	return action, nil
}

func (f *streamingFakeModel) Observe(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ domain.Action, result *domain.ToolResult) (domain.Observation, error) {
	if f.observeCalls >= len(f.observations) {
		return domain.Observation{}, errors.New("unexpected Observe call")
	}
	observation := f.observations[f.observeCalls]
	f.observeCalls++
	observation.ToolResult = result
	return observation, nil
}

type streamingFakeToolExecutor struct {
	result domain.ToolResult
	err    error
}

func (f *streamingFakeToolExecutor) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	result := f.result
	if result.ToolName == "" {
		result.ToolName = call.Name
	}
	return result, f.err
}

type streamingFakeMemoryStore struct{}

func (f *streamingFakeMemoryStore) Remember(_ context.Context, _ string, _ domain.Observation) error {
	return nil
}

func (f *streamingFakeMemoryStore) Recall(_ context.Context, _ domain.RecallQuery) ([]domain.MemoryEntry, error) {
	return nil, nil
}

type streamingFakeSessionStore struct{}

func (f *streamingFakeSessionStore) SaveSession(_ context.Context, _ domain.Session) error { return nil }
func (f *streamingFakeSessionStore) SavePlan(_ context.Context, _ domain.Plan) error       { return nil }
func (f *streamingFakeSessionStore) AppendStep(_ context.Context, _ string, _ domain.Step) error {
	return nil
}

type streamingFakeVerifier struct {
	result domain.VerificationResult
}

func (f *streamingFakeVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) (domain.VerificationResult, error) {
	return f.result, nil
}

func streamingFixedClock(timestamp time.Time) func() time.Time {
	return func() time.Time { return timestamp }
}
