package domain

import "context"

// Model 负责任务/会话的计划制定与动作选择。
type Model interface {
	CreatePlan(ctx context.Context, task Task, session Session, memory []MemoryEntry) (Plan, error)
	NextAction(ctx context.Context, task Task, session Session, plan Plan, steps []Step, memory []MemoryEntry, tools []ToolInfo) (Action, error)
	Observe(ctx context.Context, task Task, session Session, plan Plan, action Action, result *ToolResult) (Observation, error)
}

// ToolExecutor 执行已批准的工具调用并标准化结果。
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}

// MemoryStore 持久化可检查的观察结果以供后续复用。
type MemoryStore interface {
	Remember(ctx context.Context, sessionID string, observation Observation) error
	Recall(ctx context.Context, query RecallQuery) ([]MemoryEntry, error)
}

// SessionStore 持久化会话、计划以及步骤历史。
type SessionStore interface {
	SaveSession(ctx context.Context, session Session) error
	SavePlan(ctx context.Context, plan Plan) error
	AppendStep(ctx context.Context, sessionID string, step Step) error
}

// Verifier 评估最新观察结果是否满足任务要求。
type Verifier interface {
	Verify(ctx context.Context, task Task, session Session, plan Plan, steps []Step, observation Observation) (VerificationResult, error)
}
