package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"zheng-harness/internal/domain"
)

type SQLiteSessionStore struct {
	db *sql.DB
}

type storedTask struct {
	Description        string              `json:"description,omitempty"`
	Goal               string              `json:"goal,omitempty"`
	Category           domain.TaskCategory `json:"category,omitempty"`
	ProtocolHint       string              `json:"protocol_hint,omitempty"`
	VerificationPolicy string              `json:"verification_policy,omitempty"`
}

type storedSessionMetadata struct {
	Task *storedTask `json:"task,omitempty"`
}

type storedPlan struct {
	Summary string        `json:"summary"`
	Steps   []domain.Step `json:"steps"`
}

func NewSQLiteSessionStore(dbPath string) (*SQLiteSessionStore, error) {
	db, err := openSQLite(dbPath)
	if err != nil {
		return nil, err
	}
	return &SQLiteSessionStore{db: db}, nil
}

func (s *SQLiteSessionStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteSessionStore) SaveSession(ctx context.Context, session domain.Session) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite session store is not initialized")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, task_id, status, config_json, created_at, updated_at, terminated_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			status = excluded.status,
			updated_at = excluded.updated_at,
			terminated_reason = excluded.terminated_reason
	`, session.ID, session.TaskID, string(session.Status), nil, session.CreatedAt.UTC(), session.UpdatedAt.UTC(), nil)
	if err != nil {
		return fmt.Errorf("save session %q: %w", session.ID, err)
	}
	return nil
}

func (s *SQLiteSessionStore) SaveTask(ctx context.Context, sessionID string, task domain.Task) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite session store is not initialized")
	}

	metadata, err := json.Marshal(storedSessionMetadata{Task: newStoredTask(task)})
	if err != nil {
		return fmt.Errorf("marshal task metadata for session %q: %w", sessionID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE sessions
		SET config_json = ?, updated_at = CASE WHEN updated_at > created_at THEN updated_at ELSE created_at END
		WHERE id = ?
	`, string(metadata), sessionID)
	if err != nil {
		return fmt.Errorf("save task metadata for session %q: %w", sessionID, err)
	}
	return nil
}

func (s *SQLiteSessionStore) SavePlan(ctx context.Context, plan domain.Plan) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite session store is not initialized")
	}

	content, err := json.Marshal(storedPlan{Summary: plan.Summary, Steps: plan.Steps})
	if err != nil {
		return fmt.Errorf("marshal plan %q: %w", plan.ID, err)
	}

	updatedAt := plan.CreatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO plans (id, task_id, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			content = excluded.content,
			updated_at = excluded.updated_at
	`, plan.ID, plan.TaskID, string(content), plan.CreatedAt.UTC(), updatedAt.UTC())
	if err != nil {
		return fmt.Errorf("save plan %q: %w", plan.ID, err)
	}
	return nil
}

func (s *SQLiteSessionStore) AppendStep(ctx context.Context, sessionID string, step domain.Step) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite session store is not initialized")
	}

	actionJSON, err := json.Marshal(step.Action)
	if err != nil {
		return fmt.Errorf("marshal step action: %w", err)
	}
	observationJSON, err := json.Marshal(step.Observation)
	if err != nil {
		return fmt.Errorf("marshal step observation: %w", err)
	}
	verificationJSON, err := json.Marshal(step.Verification)
	if err != nil {
		return fmt.Errorf("marshal step verification: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO steps (session_id, step_index, action_json, observation_json, verification_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, sessionID, step.Index, string(actionJSON), string(observationJSON), string(verificationJSON), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("append step %d for session %q: %w", step.Index, sessionID, err)
	}
	return nil
}

func (s *SQLiteSessionStore) ResumeSession(ctx context.Context, sessionID string) (domain.Session, domain.Plan, []domain.Step, error) {
	if s == nil || s.db == nil {
		return domain.Session{}, domain.Plan{}, nil, errors.New("sqlite session store is not initialized")
	}

	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}
	plan, err := s.loadLatestPlan(ctx, session.TaskID)
	if err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}
	steps, err := s.loadSteps(ctx, sessionID)
	if err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}
	return session, plan, steps, nil
}

func (s *SQLiteSessionStore) LoadTask(ctx context.Context, sessionID string) (domain.Task, bool, error) {
	if s == nil || s.db == nil {
		return domain.Task{}, false, errors.New("sqlite session store is not initialized")
	}

	session, metadataTask, err := s.loadSessionRecord(ctx, sessionID)
	if err != nil {
		return domain.Task{}, false, err
	}
	plan, err := s.loadLatestPlan(ctx, session.TaskID)
	if err != nil {
		return domain.Task{}, false, err
	}

	task := domain.Task{
		ID:          session.TaskID,
		Description: plan.Summary,
		Goal:        plan.Summary,
		CreatedAt:   session.CreatedAt,
	}

	stored := metadataTask
	if stored == nil {
		return task.Normalize(), false, nil
	}

	task.Category = stored.Category
	if stored.Description != "" {
		task.Description = stored.Description
	}
	if stored.Goal != "" {
		task.Goal = stored.Goal
	}
	task.ProtocolHint = stored.ProtocolHint
	task.VerificationPolicy = stored.VerificationPolicy
	return task.Normalize(), true, nil
}

func (s *SQLiteSessionStore) loadSession(ctx context.Context, sessionID string) (domain.Session, error) {
	session, _, err := s.loadSessionRecord(ctx, sessionID)
	return session, err
}

func (s *SQLiteSessionStore) loadSessionRecord(ctx context.Context, sessionID string) (domain.Session, *storedTask, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, status, config_json, created_at, updated_at
		FROM sessions
		WHERE id = ?
	`, sessionID)

	var session domain.Session
	var status string
	var configJSON sql.NullString
	if err := row.Scan(&session.ID, &session.TaskID, &status, &configJSON, &session.CreatedAt, &session.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Session{}, nil, fmt.Errorf("session %q not found: %w", sessionID, err)
		}
		return domain.Session{}, nil, fmt.Errorf("load session %q: %w", sessionID, err)
	}
	session.Status = domain.SessionStatus(status)

	metadata, err := parseStoredSessionMetadata(configJSON)
	if err != nil {
		return domain.Session{}, nil, fmt.Errorf("load session %q metadata: %w", sessionID, err)
	}
	return session, metadata.Task, nil
}

func (s *SQLiteSessionStore) loadLatestPlan(ctx context.Context, taskID string) (domain.Plan, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, content, created_at
		FROM plans
		WHERE task_id = ?
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1
	`, taskID)

	var plan domain.Plan
	var content string
	if err := row.Scan(&plan.ID, &plan.TaskID, &content, &plan.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Plan{}, fmt.Errorf("plan for task %q not found: %w", taskID, err)
		}
		return domain.Plan{}, fmt.Errorf("load latest plan for task %q: %w", taskID, err)
	}

	var stored storedPlan
	if err := json.Unmarshal([]byte(content), &stored); err != nil {
		return domain.Plan{}, fmt.Errorf("unmarshal plan %q: %w", plan.ID, err)
	}
	plan.Summary = stored.Summary
	plan.Steps = stored.Steps
	return plan, nil
}

func newStoredTask(task domain.Task) *storedTask {
	normalized := task.Normalize()
	if normalized.Category == domain.TaskCategoryGeneral && normalized.ProtocolHint == "" && normalized.VerificationPolicy == "" {
		return nil
	}
	return &storedTask{
		Description:        normalized.Description,
		Goal:               normalized.Goal,
		Category:           normalized.Category,
		ProtocolHint:       normalized.ProtocolHint,
		VerificationPolicy: normalized.VerificationPolicy,
	}
}

func parseStoredSessionMetadata(raw sql.NullString) (storedSessionMetadata, error) {
	if !raw.Valid || raw.String == "" {
		return storedSessionMetadata{}, nil
	}
	var metadata storedSessionMetadata
	if err := json.Unmarshal([]byte(raw.String), &metadata); err != nil {
		return storedSessionMetadata{}, err
	}
	if metadata.Task != nil {
		metadata.Task.Category = metadata.Task.Category.Normalize()
	}
	return metadata, nil
}

func (s *SQLiteSessionStore) loadSteps(ctx context.Context, sessionID string) ([]domain.Step, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT step_index, action_json, observation_json, verification_json
		FROM steps
		WHERE session_id = ?
		ORDER BY step_index ASC, id ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query steps for session %q: %w", sessionID, err)
	}
	defer rows.Close()

	steps := make([]domain.Step, 0)
	for rows.Next() {
		var (
			step             domain.Step
			actionJSON       string
			observationJSON  string
			verificationJSON string
		)
		if err := rows.Scan(&step.Index, &actionJSON, &observationJSON, &verificationJSON); err != nil {
			return nil, fmt.Errorf("scan step for session %q: %w", sessionID, err)
		}
		if err := json.Unmarshal([]byte(actionJSON), &step.Action); err != nil {
			return nil, fmt.Errorf("unmarshal action for session %q: %w", sessionID, err)
		}
		if err := json.Unmarshal([]byte(observationJSON), &step.Observation); err != nil {
			return nil, fmt.Errorf("unmarshal observation for session %q: %w", sessionID, err)
		}
		if verificationJSON != "" {
			if err := json.Unmarshal([]byte(verificationJSON), &step.Verification); err != nil {
				return nil, fmt.Errorf("unmarshal verification for session %q: %w", sessionID, err)
			}
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate steps for session %q: %w", sessionID, err)
	}
	return steps, nil
}
