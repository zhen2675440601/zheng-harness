package orchestration

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
)

// WorkerAgent executes one subtask using a scoped runtime-backed PEV loop.
type WorkerAgent struct {
	ID      string
	Subtask Subtask
	Engine  runtime.Engine
	Result  WorkerResult
	Status  SubtaskStatus

	ResultChannel chan<- WorkerResult

	mu         sync.Mutex
	reported   bool
	terminated bool
	steps      []domain.Step
	session    domain.Session
	plan       domain.Plan
	cancel     context.CancelFunc
	ctx        context.Context
}

// NewWorkerAgent constructs a scoped worker bound to one subtask.
func NewWorkerAgent(id string, subtask Subtask, engine runtime.Engine, resultCh chan<- WorkerResult) *WorkerAgent {
	return &WorkerAgent{
		ID:            id,
		Subtask:       subtask.Normalize(),
		Engine:        engine,
		Status:        SubtaskStatusPending,
		ResultChannel: resultCh,
		Result: WorkerResult{
			SubtaskID: subtask.ID,
			Status:    SubtaskStatusPending,
		},
	}
}

// Plan validates the assigned subtask and prepares scoped runtime state.
func (w *WorkerAgent) Plan(ctx context.Context, subtask Subtask, _ TaskDecomposition) error {
	if ctx == nil {
		return errors.New("worker context is required")
	}
	subtask = subtask.Normalize()
	if err := subtask.Validate(); err != nil {
		w.fail(subtask, err)
		return err
	}
	if err := w.validateEngine(); err != nil {
		w.fail(subtask, err)
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.Subtask = subtask
	w.Status = SubtaskStatusRunning
	w.Result.TaskID = w.taskID(subtask)
	w.Result.SubtaskID = subtask.ID
	w.Result.Status = SubtaskStatusRunning
	w.Result.Err = nil
	w.Result.Output = ""
	w.Result.SessionStatus = ""
	w.Result.VerificationPassed = false
	w.Result.WorkerTerminated = false
	w.ctx = runCtx
	w.cancel = cancel
	w.mu.Unlock()
	return nil
}

// Execute runs a scoped plan-execute-verify runtime loop for the subtask.
func (w *WorkerAgent) Execute(ctx context.Context, subtask Subtask, _ TaskDecomposition) error {
	runCtx := ctx
	w.mu.Lock()
	if w.ctx != nil {
		runCtx = w.ctx
	}
	w.mu.Unlock()

	task := domain.Task{
		ID:          subtask.ID,
		Description: subtask.Description,
		Goal:        coalesce(subtask.ExpectedOutput, subtask.Input, subtask.Description),
	}

	session, plan, steps, err := w.Engine.Run(runCtx, task)
	w.mu.Lock()
	w.session = session
	w.plan = plan
	w.steps = append([]domain.Step(nil), steps...)
	w.mu.Unlock()

	if err != nil {
		w.fail(subtask, err)
		return err
	}
	w.captureSuccess(subtask, session, steps)
	return nil
}

// Verify checks that runtime execution ended in a verified success state.
func (w *WorkerAgent) Verify(_ context.Context, subtask Subtask, _ TaskDecomposition) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.Result.Err != nil {
		return w.Result.Err
	}
	if w.session.Status != domain.SessionStatusSuccess {
		err := fmt.Errorf("worker subtask %q ended with session status %q", subtask.ID, w.session.Status)
		w.Status = SubtaskStatusFailed
		w.Result.Status = SubtaskStatusFailed
		w.Result.SessionStatus = w.session.Status
		w.Result.Err = err
		return err
	}
	if len(w.steps) == 0 {
		err := fmt.Errorf("worker subtask %q produced no execution steps", subtask.ID)
		w.Status = SubtaskStatusFailed
		w.Result.Status = SubtaskStatusFailed
		w.Result.Err = err
		return err
	}
	verification := w.steps[len(w.steps)-1].Verification.Normalize()
	if !verification.Passed {
		err := fmt.Errorf("worker subtask %q verification failed: %s", subtask.ID, verification.Reason)
		w.Status = SubtaskStatusFailed
		w.Result.Status = SubtaskStatusFailed
		w.Result.VerificationPassed = false
		w.Result.Err = err
		return err
	}
	w.Status = SubtaskStatusCompleted
	w.Result.Status = SubtaskStatusCompleted
	w.Result.SessionStatus = w.session.Status
	w.Result.VerificationPassed = true
	w.Result.Output = summarizeWorkerOutput(w.steps)
	return nil
}

// Report sends one terminal result to the orchestrator.
func (w *WorkerAgent) Report() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.reported {
		return nil
	}
	if w.ResultChannel == nil {
		return errors.New("worker result channel is required")
	}
	w.Result.WorkerTerminated = w.terminated || w.Result.WorkerTerminated
	w.ResultChannel <- w.Result
	w.reported = true
	return nil
}

// CurrentResult returns the latest worker result snapshot.
func (w *WorkerAgent) CurrentResult() WorkerResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := w.Result
	result.WorkerTerminated = w.terminated || result.WorkerTerminated
	return result
}

// Terminate cancels scoped execution and marks the worker finished.
func (w *WorkerAgent) Terminate() {
	w.mu.Lock()
	if w.terminated {
		w.mu.Unlock()
		return
	}
	w.terminated = true
	cancel := w.cancel
	if w.Status == SubtaskStatusRunning && w.Result.Status == SubtaskStatusRunning {
		w.Status = SubtaskStatusFailed
		w.Result.Status = SubtaskStatusFailed
		if w.Result.Err == nil {
			w.Result.Err = context.Canceled
		}
	}
	w.Result.WorkerTerminated = true
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (w *WorkerAgent) validateEngine() error {
	if w.Engine.Model == nil || w.Engine.Tools == nil || w.Engine.Memory == nil || w.Engine.Sessions == nil || w.Engine.Verifier == nil {
		return errors.New("worker engine requires all runtime dependencies")
	}
	return nil
}

func (w *WorkerAgent) captureSuccess(subtask Subtask, session domain.Session, steps []domain.Step) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = SubtaskStatusCompleted
	w.Result.TaskID = w.taskID(subtask)
	w.Result.SubtaskID = subtask.ID
	w.Result.Status = SubtaskStatusCompleted
	w.Result.SessionStatus = session.Status
	w.Result.VerificationPassed = len(steps) > 0 && steps[len(steps)-1].Verification.Normalize().Passed
	w.Result.Output = summarizeWorkerOutput(steps)
	w.Result.Err = nil
}

func (w *WorkerAgent) fail(subtask Subtask, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = SubtaskStatusFailed
	w.Result.TaskID = w.taskID(subtask)
	w.Result.SubtaskID = subtask.ID
	w.Result.Status = SubtaskStatusFailed
	w.Result.Err = err
	if w.session.Status != "" {
		w.Result.SessionStatus = w.session.Status
	}
	if len(w.steps) > 0 {
		w.Result.VerificationPassed = w.steps[len(w.steps)-1].Verification.Normalize().Passed
		w.Result.Output = summarizeWorkerOutput(w.steps)
	}
}

func (w *WorkerAgent) taskID(subtask Subtask) string {
	if w.Result.TaskID != "" {
		return w.Result.TaskID
	}
	if w.ID != "" {
		return w.ID
	}
	return subtask.ID
}

func summarizeWorkerOutput(steps []domain.Step) string {
	if len(steps) == 0 {
		return ""
	}
	last := steps[len(steps)-1]
	if last.Observation.FinalResponse != "" {
		return last.Observation.FinalResponse
	}
	if last.Observation.Summary != "" {
		return last.Observation.Summary
	}
	if last.Observation.ToolResult != nil && last.Observation.ToolResult.Output != "" {
		return last.Observation.ToolResult.Output
	}
	return last.Action.Summary
}

func coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
