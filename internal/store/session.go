package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"zheng-harness/internal/domain"
)

// SessionRepository 基于 SQLite 实现 domain.SessionStore。
type SessionRepository struct {
	db    *sql.DB
	steps *StepRepository
}

// ResumeState 包含用于恢复流程的已持久化会话及步骤历史。
type ResumeState struct {
	Session domain.Session
	Steps   []domain.Step
}

// NewSessionRepository 构造一个由 SQLite 支撑的会话仓储。
func NewSessionRepository(database *Database) *SessionRepository {
	return &SessionRepository{db: database.SQL(), steps: NewStepRepository(database)}
}

// SaveSession 存储或更新一条会话记录。
func (r *SessionRepository) SaveSession(ctx context.Context, session domain.Session) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO sessions (id, task_id, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  task_id = excluded.task_id,
  status = excluded.status,
  created_at = excluded.created_at,
  updated_at = excluded.updated_at
`,
		session.ID,
		session.TaskID,
		string(session.Status),
		session.CreatedAt.UTC().Format(time.RFC3339Nano),
		session.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// SavePlan 存储或更新计划摘要。
func (r *SessionRepository) SavePlan(ctx context.Context, plan domain.Plan) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO plans (id, task_id, summary, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  task_id = excluded.task_id,
  summary = excluded.summary,
  created_at = excluded.created_at
`,
		plan.ID,
		plan.TaskID,
		plan.Summary,
		plan.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// AppendStep 将步骤持久化委托给步骤仓储。
func (r *SessionRepository) AppendStep(ctx context.Context, sessionID string, step domain.Step) error {
	return r.steps.Append(ctx, sessionID, step)
}

// Resume 恢复一个会话及其所有已持久化步骤。
func (r *SessionRepository) Resume(ctx context.Context, sessionID string) (ResumeState, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, task_id, status, created_at, updated_at
FROM sessions
WHERE id = ?
`, sessionID)

	var (
		session              domain.Session
		status               string
		createdAt, updatedAt string
	)
	if err := row.Scan(&session.ID, &session.TaskID, &status, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResumeState{}, err
		}
		return ResumeState{}, err
	}
	session.Status = domain.SessionStatus(status)
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return ResumeState{}, err
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return ResumeState{}, err
	}
	session.CreatedAt = parsedCreatedAt
	session.UpdatedAt = parsedUpdatedAt

	steps, err := r.steps.LoadBySession(ctx, sessionID)
	if err != nil {
		return ResumeState{}, err
	}
	return ResumeState{Session: session, Steps: steps}, nil
}
