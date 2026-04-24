package runtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
)

func TestRuntimeWithFakes(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 24, 16, 30, 0, 0, time.UTC)
	task := domain.Task{
		ID:          "task-1",
		Description: "inspect repository",
		Goal:        "propose next step",
		CreatedAt:   fixedTime,
	}

	model := &fakeModel{
		plans: []domain.Plan{{
			ID:        "plan-1",
			TaskID:    task.ID,
			Summary:   "Use one tool call then summarize findings.",
			CreatedAt: fixedTime,
		}},
		actions: []domain.Action{{
			Type:    domain.ActionTypeToolCall,
			Summary: "Read repository metadata",
			ToolCall: &domain.ToolCall{
				Name:    "read_file",
				Input:   "README.md",
				Timeout: 5 * time.Second,
			},
		}},
		observations: []domain.Observation{{
			Summary:       "Repository notes captured",
			FinalResponse: "Next step: define strongly typed contracts.",
		}},
	}

	tools := &fakeToolExecutor{results: []domain.ToolResult{{
		ToolName: "read_file",
		Output:   "project bootstrap complete",
		Duration: 20 * time.Millisecond,
	}}}
	memory := &fakeMemoryStore{}
	sessions := &fakeSessionStore{}
	verifier := &fakeVerifier{results: []domain.VerificationResult{{Passed: true, Reason: "deterministic fake accepted output"}}}

	engine := runtime.Engine{
		Model:          model,
		Tools:          tools,
		Memory:         memory,
		Sessions:       sessions,
		Verifier:       verifier,
		Clock:          fixedClock(fixedTime),
		MaxSteps:       3,
		MaxRetries:     1,
		SessionTimeout: time.Minute,
	}

	session, plan, steps, err := engine.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("session status = %q, want %q", session.Status, domain.SessionStatusSuccess)
	}
	if plan.ID != "plan-1" {
		t.Fatalf("plan id = %q, want plan-1", plan.ID)
	}
	if len(steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(steps))
	}
	if got := steps[0].Action.ToolCall.Name; got != "read_file" {
		t.Fatalf("tool call name = %q, want read_file", got)
	}
	if got := steps[0].Observation.ToolResult.Output; got != "project bootstrap complete" {
		t.Fatalf("tool output = %q, want deterministic output", got)
	}
	if !steps[0].Verification.Passed {
		t.Fatalf("verification should pass")
	}
	if model.createPlanCalls != 1 || model.nextActionCalls != 1 || model.observeCalls != 1 {
		t.Fatalf("unexpected model call counts: %+v", model)
	}
	if tools.executeCalls != 1 {
		t.Fatalf("tool execute calls = %d, want 1", tools.executeCalls)
	}
	if verifier.called != 1 {
		t.Fatalf("verifier calls = %d, want 1", verifier.called)
	}
	if len(memory.remembered) != 1 {
		t.Fatalf("memory observations = %d, want 1", len(memory.remembered))
	}
	if len(sessions.steps) != 1 {
		t.Fatalf("persisted steps = %d, want 1", len(sessions.steps))
	}
	if got := sessions.savedSessions[len(sessions.savedSessions)-1].Status; got != domain.SessionStatusSuccess {
		t.Fatalf("final saved session status = %q, want success", got)
	}
}

func TestRuntimeCompletesSuccessfulSession(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 24, 16, 35, 0, 0, time.UTC)
	task := domain.Task{ID: "task-success", Description: "fix issue", Goal: "done", CreatedAt: fixedTime}

	engine := runtime.Engine{
		Model: &fakeModel{
			plans: []domain.Plan{{ID: "plan-1", TaskID: task.ID, Summary: "try once"}},
			actions: []domain.Action{{Type: domain.ActionTypeRespond, Summary: "respond", Response: "done"}},
			observations: []domain.Observation{{Summary: "completed", FinalResponse: "done"}},
		},
		Tools:          &fakeToolExecutor{},
		Memory:         &fakeMemoryStore{},
		Sessions:       &fakeSessionStore{},
		Verifier:       &fakeVerifier{results: []domain.VerificationResult{{Passed: true, Reason: "ok"}}},
		Clock:          fixedClock(fixedTime),
		MaxSteps:       2,
		MaxRetries:     1,
		SessionTimeout: time.Minute,
	}

	session, _, steps, err := engine.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusSuccess)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
}

func TestRuntimeStopsOnBudgetExceeded(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 24, 16, 40, 0, 0, time.UTC)
	task := domain.Task{ID: "task-budget", Description: "inspect", Goal: "never verifies", CreatedAt: fixedTime}

	model := &fakeModel{
		plans: []domain.Plan{
			{ID: "plan-1", TaskID: task.ID, Summary: "first attempt"},
			{ID: "plan-2", TaskID: task.ID, Summary: "second attempt"},
		},
		actions: []domain.Action{
			{Type: domain.ActionTypeRespond, Summary: "attempt 1", Response: "still working"},
			{Type: domain.ActionTypeRespond, Summary: "attempt 2", Response: "still working"},
		},
		observations: []domain.Observation{
			{Summary: "attempt 1"},
			{Summary: "attempt 2"},
		},
	}

	engine := runtime.Engine{
		Model:          model,
		Tools:          &fakeToolExecutor{},
		Memory:         &fakeMemoryStore{},
		Sessions:       &fakeSessionStore{},
		Verifier:       &fakeVerifier{results: []domain.VerificationResult{{Passed: false, Reason: "missing evidence"}, {Passed: false, Reason: "still missing"}}},
		Clock:          fixedClock(fixedTime),
		MaxSteps:       2,
		MaxRetries:     5,
		SessionTimeout: time.Minute,
	}

	session, plan, steps, err := engine.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("run error = %v, want nil on budget exhaustion", err)
	}
	if session.Status != domain.SessionStatusBudgetExceeded {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusBudgetExceeded)
	}
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	if plan.ID != "plan-2" {
		t.Fatalf("final plan = %q, want plan-2", plan.ID)
	}
}

func TestRuntimeStopsOnRetryBudgetExceeded(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 24, 16, 45, 0, 0, time.UTC)
	task := domain.Task{ID: "task-retry", Description: "repair", Goal: "verified", CreatedAt: fixedTime}

	model := &fakeModel{
		plans: []domain.Plan{
			{ID: "plan-1", TaskID: task.ID, Summary: "first attempt"},
			{ID: "plan-2", TaskID: task.ID, Summary: "retry attempt"},
		},
		actions: []domain.Action{
			{Type: domain.ActionTypeRespond, Summary: "attempt 1", Response: "try 1"},
			{Type: domain.ActionTypeRespond, Summary: "attempt 2", Response: "try 2"},
		},
		observations: []domain.Observation{
			{Summary: "failed attempt 1"},
			{Summary: "failed attempt 2"},
		},
	}

	engine := runtime.Engine{
		Model:          model,
		Tools:          &fakeToolExecutor{},
		Memory:         &fakeMemoryStore{},
		Sessions:       &fakeSessionStore{},
		Verifier:       &fakeVerifier{results: []domain.VerificationResult{{Passed: false, Reason: "bad output"}, {Passed: false, Reason: "bad output again"}}},
		Clock:          fixedClock(fixedTime),
		MaxSteps:       5,
		MaxRetries:     1,
		SessionTimeout: time.Minute,
	}

	session, _, steps, err := engine.Run(context.Background(), task)
	if err == nil {
		t.Fatal("expected retry budget error")
	}
	if got := err.Error(); got != "runtime retry budget exceeded" {
		t.Fatalf("error = %q, want retry budget error", got)
	}
	if session.Status != domain.SessionStatusVerificationFailed {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusVerificationFailed)
	}
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	if got := steps[len(steps)-1].Verification.Reason; got != "bad output again" {
		t.Fatalf("last verification reason = %q, want retry failure reason", got)
	}
}

func TestRuntimeInterruptsWhenContextCancelled(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 24, 16, 50, 0, 0, time.UTC)
	task := domain.Task{ID: "task-interrupt", Description: "stop", Goal: "cancelled", CreatedAt: fixedTime}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := runtime.Engine{
		Model: &fakeModel{
			plans: []domain.Plan{{ID: "plan-1", TaskID: task.ID, Summary: "unused"}},
		},
		Tools:          &fakeToolExecutor{},
		Memory:         &fakeMemoryStore{},
		Sessions:       &fakeSessionStore{},
		Verifier:       &fakeVerifier{},
		Clock:          fixedClock(fixedTime),
		MaxSteps:       1,
		MaxRetries:     0,
		SessionTimeout: time.Minute,
	}

	session, _, steps, err := engine.Run(ctx, task)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if session.Status != domain.SessionStatusInterrupted {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusInterrupted)
	}
	if len(steps) != 0 {
		t.Fatalf("steps = %d, want 0", len(steps))
	}
}

func TestRuntimeReturnsFatalErrorOnModelFailure(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 24, 16, 55, 0, 0, time.UTC)
	task := domain.Task{ID: "task-fatal", Description: "break", Goal: "error", CreatedAt: fixedTime}

	boom := errors.New("model exploded")
	engine := runtime.Engine{
		Model: &fakeModel{
			plans:     []domain.Plan{{ID: "plan-1", TaskID: task.ID, Summary: "first"}},
			actionErr: boom,
		},
		Tools:          &fakeToolExecutor{},
		Memory:         &fakeMemoryStore{},
		Sessions:       &fakeSessionStore{},
		Verifier:       &fakeVerifier{},
		Clock:          fixedClock(fixedTime),
		MaxSteps:       2,
		MaxRetries:     1,
		SessionTimeout: time.Minute,
	}

	session, _, steps, err := engine.Run(context.Background(), task)
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want %v", err, boom)
	}
	if session.Status != domain.SessionStatusFatalError {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusFatalError)
	}
	if len(steps) != 0 {
		t.Fatalf("steps = %d, want 0", len(steps))
	}
}

type fakeModel struct {
	plans           []domain.Plan
	actions         []domain.Action
	observations    []domain.Observation
	planErr         error
	actionErr       error
	observeErr      error
	createPlanCalls int
	nextActionCalls int
	observeCalls    int
}

func (f *fakeModel) CreatePlan(_ context.Context, _ domain.Task, _ domain.Session) (domain.Plan, error) {
	if f.planErr != nil {
		return domain.Plan{}, f.planErr
	}
	if f.createPlanCalls >= len(f.plans) {
		return domain.Plan{}, errors.New("unexpected CreatePlan call")
	}
	plan := f.plans[f.createPlanCalls]
	f.createPlanCalls++
	return plan, nil
}

func (f *fakeModel) NextAction(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step) (domain.Action, error) {
	if f.actionErr != nil {
		return domain.Action{}, f.actionErr
	}
	if f.nextActionCalls >= len(f.actions) {
		return domain.Action{}, errors.New("unexpected NextAction call")
	}
	action := f.actions[f.nextActionCalls]
	f.nextActionCalls++
	return action, nil
}

func (f *fakeModel) Observe(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ domain.Action, result *domain.ToolResult) (domain.Observation, error) {
	if f.observeErr != nil {
		return domain.Observation{}, f.observeErr
	}
	if f.observeCalls >= len(f.observations) {
		return domain.Observation{}, errors.New("unexpected Observe call")
	}
	observation := f.observations[f.observeCalls]
	f.observeCalls++
	observation.ToolResult = result
	return observation, nil
}

type fakeToolExecutor struct {
	results      []domain.ToolResult
	executeCalls int
}

func (f *fakeToolExecutor) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	var result domain.ToolResult
	if f.executeCalls < len(f.results) {
		result = f.results[f.executeCalls]
	}
	f.executeCalls++
	if result.ToolName == "" {
		result.ToolName = call.Name
	}
	return result, nil
}

type fakeMemoryStore struct {
	remembered []domain.Observation
}

func (f *fakeMemoryStore) Remember(_ context.Context, _ string, observation domain.Observation) error {
	f.remembered = append(f.remembered, observation)
	return nil
}

type fakeSessionStore struct {
	savedSessions []domain.Session
	savedPlans    []domain.Plan
	steps         []domain.Step
}

func (f *fakeSessionStore) SaveSession(_ context.Context, session domain.Session) error {
	f.savedSessions = append(f.savedSessions, session)
	return nil
}

func (f *fakeSessionStore) SavePlan(_ context.Context, plan domain.Plan) error {
	f.savedPlans = append(f.savedPlans, plan)
	return nil
}

func (f *fakeSessionStore) AppendStep(_ context.Context, _ string, step domain.Step) error {
	f.steps = append(f.steps, step)
	return nil
}

type fakeVerifier struct {
	results []domain.VerificationResult
	called  int
}

func (f *fakeVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) (domain.VerificationResult, error) {
	if f.called >= len(f.results) {
		return domain.VerificationResult{}, errors.New("unexpected Verify call")
	}
	result := f.results[f.called]
	f.called++
	return result, nil
}

func fixedClock(timestamp time.Time) func() time.Time {
	return func() time.Time {
		return timestamp
	}
}
