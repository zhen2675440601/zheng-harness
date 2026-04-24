package domain

import "time"

// SessionStatus describes the current runtime state for a task execution.
type SessionStatus string

const (
	SessionStatusPending            SessionStatus = "pending"
	SessionStatusRunning            SessionStatus = "running"
	SessionStatusSuccess            SessionStatus = "success"
	SessionStatusVerificationFailed SessionStatus = "verification_failed"
	SessionStatusBudgetExceeded     SessionStatus = "budget_exceeded"
	SessionStatusFatalError         SessionStatus = "fatal_error"
	SessionStatusInterrupted        SessionStatus = "interrupted"
)

// Session tracks a single runtime attempt for a task.
type Session struct {
	ID        string
	TaskID    string
	Status    SessionStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
