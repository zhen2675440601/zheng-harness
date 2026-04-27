package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"zheng-harness/internal/domain"
)

// SessionRepository implements domain.SessionStore over SQLite.
type SessionRepository struct {
	db    *sql.DB
	steps *StepRepository
}

// ResumeState contains the persisted session and step history for resume flows.
type ResumeState struct {
	Session domain.Session
	Steps   []domain.Step
}

// NewSessionRepository constructs a SQLite-backed session repository.
func NewSessionRepository(database *Database) *SessionRepository {
	return &SessionRepository{db: database.SQL(), steps: NewStepRepository(database)}
}

// SaveSession stores or updates a session record.
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

// SavePlan stores or updates a plan summary.
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

// AppendStep delegates step persistence to the step repository.
func (r *SessionRepository) AppendStep(ctx context.Context, sessionID string, step domain.Step) error {
	return r.steps.Append(ctx, sessionID, step)
}

// Resume restores a session and all persisted steps.
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
	session.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	session.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)

	steps, err := r.steps.LoadBySession(ctx, sessionID)
	if err != nil {
		return ResumeState{}, err
	}
	return ResumeState{Session: session, Steps: steps}, nil
}
