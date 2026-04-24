package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"zheng-harness/internal/domain"
)

var errRetryBudgetExceeded = errors.New("runtime retry budget exceeded")

// Engine coordinates a bounded multi-step runtime loop using domain ports only.
type Engine struct {
	Model          domain.Model
	Tools          domain.ToolExecutor
	Memory         domain.MemoryStore
	Sessions       domain.SessionStore
	Verifier       domain.Verifier
	Clock          func() time.Time
	MaxSteps       int
	MaxRetries     int
	SessionTimeout time.Duration
}

// Run executes a bounded plan-execute-verify loop until success or termination.
func (e Engine) Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	if e.Model == nil || e.Tools == nil || e.Memory == nil || e.Sessions == nil || e.Verifier == nil {
		return domain.Session{}, domain.Plan{}, nil, fmt.Errorf("runtime engine requires all dependencies")
	}

	now := time.Now
	if e.Clock != nil {
		now = e.Clock
	}

	maxSteps := e.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 1
	}
	maxRetries := e.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	sessionTimeout := e.SessionTimeout
	if sessionTimeout <= 0 {
		sessionTimeout = 5 * time.Minute
	}

	timestamp := now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = timestamp
	}

	session := domain.Session{
		ID:        task.ID + "-session",
		TaskID:    task.ID,
		Status:    domain.SessionStatusRunning,
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}

	if err := e.Sessions.SaveSession(ctx, session); err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}

	plan, err := e.createPlan(ctx, task, session, timestamp)
	if err != nil {
		return e.failSession(ctx, session, domain.SessionStatusFatalError, domain.Plan{}, nil, err)
	}

	steps := make([]domain.Step, 0, maxSteps)
	retries := 0
	deadline := timestamp.Add(sessionTimeout)

	for stepIndex := 1; stepIndex <= maxSteps; stepIndex++ {
		if err := ctx.Err(); err != nil {
			return e.failSession(ctx, session, domain.SessionStatusInterrupted, plan, steps, err)
		}
		if !now().Before(deadline) {
			return e.failSession(ctx, session, domain.SessionStatusInterrupted, plan, steps, context.DeadlineExceeded)
		}

		stepCtx, cancel := context.WithDeadline(ctx, deadline)
		action, observation, verification, err := e.executeIteration(stepCtx, task, session, plan, steps)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return e.failSession(ctx, session, domain.SessionStatusInterrupted, plan, steps, err)
			}
			return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, steps, err)
		}

		step := domain.Step{
			Index:        stepIndex,
			Action:       action,
			Observation:  observation,
			Verification: verification,
		}
		steps = append(steps, step)

		if err := e.recordStep(ctx, session.ID, step, observation); err != nil {
			return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, steps, err)
		}

		session.UpdatedAt = now()
		if verification.Passed {
			session.Status = domain.SessionStatusSuccess
			if err := e.Sessions.SaveSession(ctx, session); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			return session, plan, steps, nil
		}

		if stepIndex == maxSteps {
			return e.failSession(ctx, session, domain.SessionStatusBudgetExceeded, plan, steps, nil)
		}

		retries++
		if retries > maxRetries {
			return e.failSession(ctx, session, domain.SessionStatusVerificationFailed, plan, steps, errRetryBudgetExceeded)
		}

		plan, err = e.createPlan(ctx, task, session, now())
		if err != nil {
			return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, steps, err)
		}
	}

	return e.failSession(ctx, session, domain.SessionStatusBudgetExceeded, plan, steps, nil)
}

func (e Engine) createPlan(ctx context.Context, task domain.Task, session domain.Session, createdAt time.Time) (domain.Plan, error) {
	plan, err := e.Model.CreatePlan(ctx, task, session)
	if err != nil {
		return domain.Plan{}, err
	}
	if plan.TaskID == "" {
		plan.TaskID = task.ID
	}
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = createdAt
	}
	if err := e.Sessions.SavePlan(ctx, plan); err != nil {
		return domain.Plan{}, err
	}
	return plan, nil
}

func (e Engine) executeIteration(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step) (domain.Action, domain.Observation, domain.VerificationResult, error) {
	action, err := e.Model.NextAction(ctx, task, session, plan, steps)
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}

	var result *domain.ToolResult
	if action.ToolCall != nil {
		executed, execErr := e.Tools.Execute(ctx, *action.ToolCall)
		if execErr != nil {
			executed.Error = execErr.Error()
		}
		result = &executed
	}

	observation, err := e.Model.Observe(ctx, task, session, plan, action, result)
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}

	verification, err := e.Verifier.Verify(ctx, task, session, plan, steps, observation)
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}

	return action, observation, verification, nil
}

func (e Engine) recordStep(ctx context.Context, sessionID string, step domain.Step, observation domain.Observation) error {
	if err := e.Sessions.AppendStep(ctx, sessionID, step); err != nil {
		return err
	}
	if err := e.Memory.Remember(ctx, sessionID, observation); err != nil {
		return err
	}
	return nil
}

func (e Engine) failSession(ctx context.Context, session domain.Session, status domain.SessionStatus, plan domain.Plan, steps []domain.Step, cause error) (domain.Session, domain.Plan, []domain.Step, error) {
	now := time.Now
	if e.Clock != nil {
		now = e.Clock
	}
	session.Status = status
	session.UpdatedAt = now()
	if err := e.Sessions.SaveSession(ctx, session); err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}
	return session, plan, steps, cause
}
