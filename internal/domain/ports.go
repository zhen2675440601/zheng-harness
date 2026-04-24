package domain

import "context"

// Model owns planning and action selection for a task/session.
type Model interface {
	CreatePlan(ctx context.Context, task Task, session Session) (Plan, error)
	NextAction(ctx context.Context, task Task, session Session, plan Plan, steps []Step) (Action, error)
	Observe(ctx context.Context, task Task, session Session, plan Plan, action Action, result *ToolResult) (Observation, error)
}

// ToolExecutor runs approved tool calls and normalizes the result.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}

// MemoryStore persists inspectable observations for later reuse.
type MemoryStore interface {
	Remember(ctx context.Context, sessionID string, observation Observation) error
}

// SessionStore persists sessions, plans, and step history.
type SessionStore interface {
	SaveSession(ctx context.Context, session Session) error
	SavePlan(ctx context.Context, plan Plan) error
	AppendStep(ctx context.Context, sessionID string, step Step) error
}

// Verifier evaluates whether the latest observation satisfies the task.
type Verifier interface {
	Verify(ctx context.Context, task Task, session Session, plan Plan, steps []Step, observation Observation) (VerificationResult, error)
}
