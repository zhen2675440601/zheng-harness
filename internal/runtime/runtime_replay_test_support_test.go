package runtime_test

import (
	"context"
	"errors"
	"time"

	"zheng-harness/internal/domain"
)

func fixedClock(timestamp time.Time) func() time.Time {
	return func() time.Time { return timestamp }
}

type fakeModel struct {
	plans        []domain.Plan
	actions      []domain.Action
	observations []domain.Observation
	planErr      error
	createCalls  int
	actionCalls  int
	observeCalls int
}

func (f *fakeModel) CreatePlan(_ context.Context, _ domain.Task, _ domain.Session, _ []domain.MemoryEntry) (domain.Plan, error) {
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

func (f *fakeModel) NextAction(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ []domain.MemoryEntry, _ []domain.ToolInfo) (domain.Action, error) {
	if f.actionCalls >= len(f.actions) {
		return domain.Action{}, errors.New("unexpected NextAction call")
	}
	action := f.actions[f.actionCalls]
	f.actionCalls++
	return action, nil
}

func (f *fakeModel) Observe(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ domain.Action, result *domain.ToolResult) (domain.Observation, error) {
	if f.observeCalls >= len(f.observations) {
		return domain.Observation{}, errors.New("unexpected Observe call")
	}
	observation := f.observations[f.observeCalls]
	f.observeCalls++
	observation.ToolResult = result
	return observation, nil
}

type fakeToolExecutor struct {
	results   []domain.ToolResult
	errs      []error
	executeFn func(context.Context, domain.ToolCall, int) (domain.ToolResult, error)
	calls     int
}

func (f *fakeToolExecutor) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	if f.executeFn != nil {
		return f.executeFn(ctx, call, f.calls)
	}
	if f.calls >= len(f.results) {
		return domain.ToolResult{}, errors.New("unexpected Execute call")
	}
	result := f.results[f.calls]
	var err error
	if f.calls < len(f.errs) {
		err = f.errs[f.calls]
	}
	f.calls++
	if result.ToolName == "" {
		result.ToolName = call.Name
	}
	return result, err
}

type fakeMemoryStore struct {
	remembered []domain.Observation
	entries    []domain.MemoryEntry
	rememberErr error
	recallErr  error
}

func (f *fakeMemoryStore) Remember(_ context.Context, _ string, observation domain.Observation) error {
	if f.rememberErr != nil {
		return f.rememberErr
	}
	f.remembered = append(f.remembered, observation)
	return nil
}

func (f *fakeMemoryStore) Recall(_ context.Context, _ domain.RecallQuery) ([]domain.MemoryEntry, error) {
	if f.recallErr != nil {
		return nil, f.recallErr
	}
	return append([]domain.MemoryEntry(nil), f.entries...), nil
}

type fakeSessionStore struct {
	savedSessions []domain.Session
	savedPlans    []domain.Plan
	savedSteps    map[string][]domain.Step
	appendErr     error
	saveErr       error
}

func (f *fakeSessionStore) SaveSession(_ context.Context, session domain.Session) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedSessions = append(f.savedSessions, session)
	return nil
}

func (f *fakeSessionStore) SavePlan(_ context.Context, plan domain.Plan) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedPlans = append(f.savedPlans, plan)
	return nil
}

func (f *fakeSessionStore) AppendStep(_ context.Context, sessionID string, step domain.Step) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	if f.savedSteps == nil {
		f.savedSteps = make(map[string][]domain.Step)
	}
	f.savedSteps[sessionID] = append(f.savedSteps[sessionID], step)
	return nil
}

type fakeVerifier struct {
	results []domain.VerificationResult
	calls   int
	err     error
}

func (f *fakeVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) (domain.VerificationResult, error) {
	if f.err != nil {
		return domain.VerificationResult{}, f.err
	}
	if f.calls >= len(f.results) {
		return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}, nil
	}
	result := f.results[f.calls]
	f.calls++
	return result, nil
}
