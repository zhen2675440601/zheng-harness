package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"zheng-harness/internal/domain"
	memorypolicy "zheng-harness/internal/memory"
)

var errRetryBudgetExceeded = errors.New("runtime retry budget exceeded")

type toolInfoLister interface {
	ListToolInfo() []domain.ToolInfo
}

type toolExecutorWithRegistry interface {
	Registry() toolInfoLister
}

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

	maxSteps := max(e.MaxSteps, 1)
	maxRetries := max(e.MaxRetries, 0)
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
	tools := e.listToolInfo()
	memory := e.recallMemory(ctx, task, session)
	_ = tools
	plan, err := e.Model.CreatePlan(ctx, task, session, memory)
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
	tools := e.listToolInfo()
	memory := e.recallMemory(ctx, task, session)
	action, err := e.Model.NextAction(ctx, task, session, plan, steps, memory, tools)
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

func (e Engine) recallMemory(ctx context.Context, task domain.Task, session domain.Session) []domain.MemoryEntry {
	const recallLimit = 10

	queries := []domain.RecallQuery{
		{SessionID: session.ID, Scope: memorypolicy.ScopeSession, Type: memorypolicy.TypeFact, Limit: recallLimit},
		{SessionID: session.ID, Scope: memorypolicy.ScopeSession, Type: memorypolicy.TypeSummary, Limit: recallLimit},
		{SessionID: session.ID, Scope: memorypolicy.ScopeProject, Type: memorypolicy.TypeFact, Limit: recallLimit},
		{SessionID: session.ID, Scope: memorypolicy.ScopeProject, Type: memorypolicy.TypeSummary, Limit: recallLimit},
	}

	combined := make([]domain.MemoryEntry, 0, recallLimit)
	seen := map[string]struct{}{}
	for _, query := range queries {
		entries, err := e.Memory.Recall(ctx, query)
		if err != nil {
			log.Printf("runtime: memory recall failed: %v", err)
			return nil
		}
		for _, entry := range entries {
			key := fmt.Sprintf("%d|%s|%s|%s|%s|%s", entry.ID, entry.SessionID, entry.Key, entry.Value, entry.Source, entry.Provenance)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			combined = append(combined, entry)
		}
	}

	if len(combined) == 0 {
		return nil
	}

	keywords := tokenizeKeywords(task.Description)
	if len(keywords) == 0 {
		if len(combined) > recallLimit {
			return combined[:recallLimit]
		}
		return combined
	}

	type scoredEntry struct {
		entry domain.MemoryEntry
		score int
	}
	scored := make([]scoredEntry, 0, len(combined))
	for _, entry := range combined {
		haystack := strings.ToLower(strings.Join([]string{entry.Key, entry.Value, entry.Source, entry.Provenance}, " "))
		score := 0
		for _, keyword := range keywords {
			if strings.Contains(haystack, keyword) {
				score++
			}
		}
		scored = append(scored, scoredEntry{entry: entry, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	recalled := make([]domain.MemoryEntry, 0, recallLimit)
	for _, item := range scored {
		recalled = append(recalled, item.entry)
		if len(recalled) == recallLimit {
			break
		}
	}
	return recalled
}

func tokenizeKeywords(text string) []string {
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) == 0 {
		return nil
	}
	keywords := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		trimmed := strings.Trim(field, " .,;:!?()[]{}\"'`")
		if len(trimmed) < 3 {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		keywords = append(keywords, trimmed)
	}
	return keywords
}

func (e Engine) listToolInfo() []domain.ToolInfo {
	provider, ok := e.Tools.(toolExecutorWithRegistry)
	if !ok {
		return nil
	}
	registry := provider.Registry()
	if registry == nil {
		return nil
	}
	return registry.ListToolInfo()
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
