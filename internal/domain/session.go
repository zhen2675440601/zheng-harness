package domain

import "time"

// SessionStatus 描述任务执行时当前的运行时状态。
type SessionStatus string

const (
	SessionStatusPending            SessionStatus = "pending"
	SessionStatusRunning            SessionStatus = "running"
	SessionStatusBlockedInput       SessionStatus = "blocked_input"
	SessionStatusSuccess            SessionStatus = "success"
	SessionStatusVerificationFailed SessionStatus = "verification_failed"
	SessionStatusBudgetExceeded     SessionStatus = "budget_exceeded"
	SessionStatusFatalError         SessionStatus = "fatal_error"
	SessionStatusInterrupted        SessionStatus = "interrupted"
)

// Session 跟踪任务的一次运行时尝试。
type Session struct {
	ID        string
	TaskID    string
	Status    SessionStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
