package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
)

func TestWorkerAgentExecuteSuccess(t *testing.T) {
	t.Parallel()

	subtask := Subtask{ID: "subtask-success", Description: "summarize findings", ExpectedOutput: "deliver final answer", Status: SubtaskStatusPending}
	results := make(chan WorkerResult, 1)
	worker := NewWorkerAgent("task-success", subtask, runtime.Engine{
		Model: &workerFakeModel{
			plan:        domain.Plan{ID: "plan-1", TaskID: subtask.ID, Summary: "run once"},
			action:      domain.Action{Type: domain.ActionTypeRespond, Summary: "respond", Response: "done"},
			observation: domain.Observation{Summary: "completed", FinalResponse: "done"},
		},
		Tools:          &workerFakeToolExecutor{},
		Memory:         &workerFakeMemoryStore{},
		Sessions:       &workerFakeSessionStore{},
		Verifier:       &workerFakeVerifier{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "ok"}},
		Clock:          workerFixedClock(),
		MaxSteps:       1,
		MaxRetries:     0,
		SessionTimeout: time.Minute,
	}, results)

	decomposition := TaskDecomposition{TaskID: "task-success", Subtasks: []Subtask{subtask}}
	if err := worker.Plan(context.Background(), subtask, decomposition); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if err := worker.Execute(context.Background(), subtask, decomposition); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if err := worker.Verify(context.Background(), subtask, decomposition); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	worker.Terminate()
	if err := worker.Report(); err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	result := <-results
	if worker.Status != SubtaskStatusCompleted {
		t.Fatalf("worker status = %q, want %q", worker.Status, SubtaskStatusCompleted)
	}
	if result.Status != SubtaskStatusCompleted {
		t.Fatalf("result status = %q, want %q", result.Status, SubtaskStatusCompleted)
	}
	if result.Output != "done" {
		t.Fatalf("result output = %q, want done", result.Output)
	}
	if result.SessionStatus != domain.SessionStatusSuccess {
		t.Fatalf("session status = %q, want %q", result.SessionStatus, domain.SessionStatusSuccess)
	}
	if !result.VerificationPassed {
		t.Fatal("verification should pass")
	}
	if !result.WorkerTerminated {
		t.Fatal("worker should be terminated after report")
	}
	if result.Err != nil {
		t.Fatalf("result err = %v, want nil", result.Err)
	}
}

func TestWorkerAgentExecuteFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	subtask := Subtask{ID: "subtask-failure", Description: "fail execution", Status: SubtaskStatusPending}
	worker := NewWorkerAgent("task-failure", subtask, runtime.Engine{
		Model: &workerFakeModel{
			plan:      domain.Plan{ID: "plan-1", TaskID: subtask.ID, Summary: "run once"},
			actionErr: boom,
		},
		Tools: &workerFakeToolExecutor{}, Memory: &workerFakeMemoryStore{}, Sessions: &workerFakeSessionStore{}, Verifier: &workerFakeVerifier{}, Clock: workerFixedClock(), MaxSteps: 1, MaxRetries: 0, SessionTimeout: time.Minute,
	}, make(chan WorkerResult, 1))

	decomposition := TaskDecomposition{TaskID: "task-failure", Subtasks: []Subtask{subtask}}
	if err := worker.Plan(context.Background(), subtask, decomposition); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	err := worker.Execute(context.Background(), subtask, decomposition)
	if !errors.Is(err, boom) {
		t.Fatalf("Execute() error = %v, want %v", err, boom)
	}
	result := worker.CurrentResult()
	if worker.Status != SubtaskStatusFailed {
		t.Fatalf("worker status = %q, want %q", worker.Status, SubtaskStatusFailed)
	}
	if result.Status != SubtaskStatusFailed {
		t.Fatalf("result status = %q, want %q", result.Status, SubtaskStatusFailed)
	}
	if !errors.Is(result.Err, boom) {
		t.Fatalf("result err = %v, want %v", result.Err, boom)
	}
}

func TestWorkerAgentTermination(t *testing.T) {
	t.Parallel()

	results := make(chan WorkerResult, 1)
	subtask := Subtask{ID: "subtask-terminate", Description: "terminate worker", Status: SubtaskStatusPending}
	worker := NewWorkerAgent("task-terminate", subtask, runtime.Engine{
		Model: &workerFakeModel{
			plan:        domain.Plan{ID: "plan-1", TaskID: subtask.ID, Summary: "wait"},
			action:      domain.Action{Type: domain.ActionTypeRespond, Summary: "respond", Response: "done"},
			observation: domain.Observation{Summary: "done", FinalResponse: "done"},
		},
		Tools:          &workerFakeToolExecutor{},
		Memory:         &workerFakeMemoryStore{},
		Sessions:       &workerFakeSessionStore{},
		Verifier:       &workerFakeVerifier{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "ok"}},
		Clock:          workerFixedClock(),
		MaxSteps:       1,
		MaxRetries:     0,
		SessionTimeout: time.Minute,
	}, results)

	if err := worker.Plan(context.Background(), subtask, TaskDecomposition{TaskID: "task-terminate", Subtasks: []Subtask{subtask}}); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	worker.Terminate()
	if err := worker.Report(); err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	result := <-results
	if !result.WorkerTerminated {
		t.Fatal("WorkerTerminated = false, want true")
	}
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("result err = %v, want context canceled", result.Err)
	}
}

func TestWorkerAgentCancellation(t *testing.T) {
	t.Parallel()

	results := make(chan WorkerResult, 1)
	started := make(chan struct{})
	subtask := Subtask{ID: "subtask-cancel", Description: "cancel execution", Status: SubtaskStatusPending}
	worker := NewWorkerAgent("task-cancel", subtask, runtime.Engine{
		Model: &workerFakeModel{
			plan: domain.Plan{ID: "plan-1", TaskID: subtask.ID, Summary: "block until canceled"},
			actionFn: func(ctx context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ []domain.MemoryEntry, _ []domain.ToolInfo) (domain.Action, error) {
				close(started)
				<-ctx.Done()
				return domain.Action{}, ctx.Err()
			},
		},
		Tools:          &workerFakeToolExecutor{},
		Memory:         &workerFakeMemoryStore{},
		Sessions:       &workerFakeSessionStore{},
		Verifier:       &workerFakeVerifier{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "ok"}},
		Clock:          workerFixedClock(),
		MaxSteps:       1,
		MaxRetries:     0,
		SessionTimeout: time.Minute,
	}, results)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := worker.Plan(ctx, subtask, TaskDecomposition{TaskID: "task-cancel", Subtasks: []Subtask{subtask}}); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.Execute(ctx, subtask, TaskDecomposition{TaskID: "task-cancel", Subtasks: []Subtask{subtask}})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}

	cancel()
	err := <-errCh
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context canceled", err)
	}
	result := worker.CurrentResult()
	if result.Status != SubtaskStatusFailed {
		t.Fatalf("result status = %q, want %q", result.Status, SubtaskStatusFailed)
	}
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("result err = %v, want context canceled", result.Err)
	}
	worker.Terminate()
}

type workerFakeModel struct {
	plan        domain.Plan
	action      domain.Action
	observation domain.Observation
	planErr     error
	actionErr   error
	observeErr  error
	actionFn    func(context.Context, domain.Task, domain.Session, domain.Plan, []domain.Step, []domain.MemoryEntry, []domain.ToolInfo) (domain.Action, error)
}

func (m *workerFakeModel) CreatePlan(_ context.Context, _ domain.Task, _ domain.Session, _ []domain.MemoryEntry) (domain.Plan, error) {
	if m.planErr != nil {
		return domain.Plan{}, m.planErr
	}
	return m.plan, nil
}

func (m *workerFakeModel) NextAction(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, memory []domain.MemoryEntry, tools []domain.ToolInfo) (domain.Action, error) {
	if m.actionFn != nil {
		return m.actionFn(ctx, task, session, plan, steps, memory, tools)
	}
	if m.actionErr != nil {
		return domain.Action{}, m.actionErr
	}
	return m.action, nil
}

func (m *workerFakeModel) Observe(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ domain.Action, result *domain.ToolResult) (domain.Observation, error) {
	if m.observeErr != nil {
		return domain.Observation{}, m.observeErr
	}
	observation := m.observation
	observation.ToolResult = result
	return observation, nil
}

type workerFakeToolExecutor struct{}

func (e *workerFakeToolExecutor) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{ToolName: call.Name}, nil
}

type workerFakeMemoryStore struct{}

func (s *workerFakeMemoryStore) Remember(_ context.Context, _ string, _ domain.Observation) error { return nil }
func (s *workerFakeMemoryStore) Recall(_ context.Context, _ domain.RecallQuery) ([]domain.MemoryEntry, error) {
	return nil, nil
}

type workerFakeSessionStore struct{}

func (s *workerFakeSessionStore) SaveSession(_ context.Context, _ domain.Session) error { return nil }
func (s *workerFakeSessionStore) SavePlan(_ context.Context, _ domain.Plan) error       { return nil }
func (s *workerFakeSessionStore) AppendStep(_ context.Context, _ string, _ domain.Step) error {
	return nil
}

type workerFakeVerifier struct {
	result domain.VerificationResult
	err    error
}

func (v *workerFakeVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) (domain.VerificationResult, error) {
	return v.result, v.err
}

func workerFixedClock() func() time.Time {
	return func() time.Time {
		return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	}
}
