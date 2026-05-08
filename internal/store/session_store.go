package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

type SQLiteSessionStore struct {
	db *sql.DB
}

type InspectState struct {
	Session domain.Session
	Task    domain.Task
	Plan    domain.Plan
	Steps   []domain.Step
	Lifecycle PersistedSessionLifecycle
}

type ListSessionsParams struct {
	Page     int
	PageSize int
	Status   string
}

type ListSessionsResult struct {
	Sessions []SessionSummary
	Total    int
	Page     int
	PageSize int
}

type SessionSummary struct {
	SessionID string
	Status    string
	Task      string
	TaskType  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SessionLifecyclePhase string

const (
	SessionLifecycleQueued    SessionLifecyclePhase = "queued"
	SessionLifecycleRunning   SessionLifecyclePhase = "running"
	SessionLifecycleCompleted SessionLifecyclePhase = "completed"
	SessionLifecycleFailed    SessionLifecyclePhase = "failed"
	SessionLifecycleCancelled SessionLifecyclePhase = "cancelled"
)

type PersistedSessionLifecycle struct {
	Phase      SessionLifecyclePhase
	Active     bool
	Terminal   bool
	Resumable  bool
}

type storedTask struct {
	Description        string              `json:"description,omitempty"`
	Goal               string              `json:"goal,omitempty"`
	Category           domain.TaskCategory `json:"category,omitempty"`
	ProtocolHint       string              `json:"protocol_hint,omitempty"`
	VerificationPolicy string              `json:"verification_policy,omitempty"`
}

type storedSessionMetadata struct {
	Task       *storedTask        `json:"task,omitempty"`
	Provenance *domain.Provenance `json:"provenance,omitempty"`
}

type storedPlan struct {
	Summary string        `json:"summary"`
	Steps   []domain.Step `json:"steps"`
}

func NewSQLiteSessionStore(dbPath string) (*SQLiteSessionStore, error) {
	return NewSQLiteSessionStoreWithOptions(dbPath, SQLiteOptions{})
}

func NewSQLiteSessionStoreWithOptions(dbPath string, opts SQLiteOptions) (*SQLiteSessionStore, error) {
	db, err := openSQLiteWithOptions(dbPath, opts)
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
	metadata, err := s.loadSessionMetadata(ctx, session.ID)
	if err != nil {
		return fmt.Errorf("load existing session metadata for %q: %w", session.ID, err)
	}
	metadata.Provenance = mergeStoredProvenance(metadata.Provenance, session.Provenance)
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal session metadata for %q: %w", session.ID, err)
	}
	provenanceJSON, err := marshalProvenance(metadata.Provenance)
	if err != nil {
		return fmt.Errorf("marshal session provenance for %q: %w", session.ID, err)
	}

	lifecycle := lifecycleForStatus(session.Status)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, task_id, status, lifecycle_phase, is_active, is_terminal, is_resumable, config_json, provenance_json, created_at, updated_at, terminated_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			task_id = excluded.task_id,
			status = excluded.status,
			lifecycle_phase = excluded.lifecycle_phase,
			is_active = excluded.is_active,
			is_terminal = excluded.is_terminal,
			is_resumable = excluded.is_resumable,
			config_json = excluded.config_json,
			provenance_json = excluded.provenance_json,
			updated_at = excluded.updated_at,
			terminated_reason = excluded.terminated_reason
	`, session.ID, session.TaskID, string(session.Status), string(lifecycle.Phase), boolToSQLiteInt(lifecycle.Active), boolToSQLiteInt(lifecycle.Terminal), boolToSQLiteInt(lifecycle.Resumable), nullableMetadataJSON(metadataJSON), nullableString(provenanceJSON), session.CreatedAt.UTC(), session.UpdatedAt.UTC(), lifecycle.terminatedReasonValue())
	if err != nil {
		return fmt.Errorf("save session %q: %w", session.ID, err)
	}
	return nil
}

func (s *SQLiteSessionStore) RawDB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *SQLiteSessionStore) SaveTask(ctx context.Context, sessionID string, task domain.Task) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite session store is not initialized")
	}

	existingMetadata, err := s.loadSessionMetadata(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load existing session metadata for session %q: %w", sessionID, err)
	}
	existingMetadata.Task = newStoredTask(task)
	metadata, err := json.Marshal(existingMetadata)
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
	provenanceJSON, err := marshalProvenance(step.Provenance)
	if err != nil {
		return fmt.Errorf("marshal step provenance: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO steps (session_id, step_index, action_json, observation_json, provenance_json, verification_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, sessionID, step.Index, string(actionJSON), string(observationJSON), nullableString(provenanceJSON), string(verificationJSON), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("append step %d for session %q: %w", step.Index, sessionID, err)
	}
	return nil
}

func (s *SQLiteSessionStore) ResumeSession(ctx context.Context, sessionID string) (domain.Session, domain.Plan, []domain.Step, error) {
	if s == nil || s.db == nil {
		return domain.Session{}, domain.Plan{}, nil, errors.New("sqlite session store is not initialized")
	}

	session, _, err := s.loadSession(ctx, sessionID)
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

	session, metadataTask, _, err := s.loadSessionRecord(ctx, sessionID)
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

func (s *SQLiteSessionStore) InspectSession(ctx context.Context, sessionID string) (InspectState, error) {
	if s == nil || s.db == nil {
		return InspectState{}, errors.New("sqlite session store is not initialized")
	}

	session, metadataTask, lifecycle, err := s.loadSessionRecord(ctx, sessionID)
	if err != nil {
		return InspectState{}, err
	}
	steps, err := s.loadSteps(ctx, sessionID)
	if err != nil {
		return InspectState{}, err
	}
	plan, err := s.loadLatestPlan(ctx, session.TaskID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), "not found") {
			return InspectState{}, err
		}
		plan = domain.Plan{TaskID: session.TaskID}
	}
	task := domain.Task{
		ID:          session.TaskID,
		Description: plan.Summary,
		Goal:        plan.Summary,
		CreatedAt:   session.CreatedAt,
	}
	if metadataTask != nil {
		task.Category = metadataTask.Category
		if metadataTask.Description != "" {
			task.Description = metadataTask.Description
		}
		if metadataTask.Goal != "" {
			task.Goal = metadataTask.Goal
		}
		task.ProtocolHint = metadataTask.ProtocolHint
		task.VerificationPolicy = metadataTask.VerificationPolicy
	}
	return InspectState{Session: session, Task: task.Normalize(), Plan: plan, Steps: steps, Lifecycle: lifecycle}, nil
}

func (s *SQLiteSessionStore) ListSessions(ctx context.Context, params ListSessionsParams) (*ListSessionsResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite session store is not initialized")
	}
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	status := strings.TrimSpace(params.Status)

	whereSQL := ""
	args := make([]any, 0, 3)
	if status != "" {
		statuses, err := listSessionStatusesForFilter(status)
		if err != nil {
			return nil, err
		}
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, string(status))
		}
		whereSQL = " WHERE status IN (" + strings.Join(placeholders, ", ") + ")"
	}

	countQuery := `SELECT COUNT(*) FROM sessions` + whereSQL
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count sessions: %w", err)
	}

	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, status, config_json, created_at, updated_at
		FROM sessions`+whereSQL+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	summaries := make([]SessionSummary, 0)
	for rows.Next() {
		var (
			summary    SessionSummary
			configJSON sql.NullString
		)
		if err := rows.Scan(&summary.SessionID, &summary.Status, &configJSON, &summary.CreatedAt, &summary.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan session summary: %w", err)
		}
		metadata, err := parseStoredSessionMetadata(configJSON)
		if err != nil {
			return nil, fmt.Errorf("parse session %q metadata: %w", summary.SessionID, err)
		}
		if metadata.Task != nil {
			summary.Task = strings.TrimSpace(metadata.Task.Description)
			summary.TaskType = string(metadata.Task.Category.Normalize())
		}
		if summary.TaskType == "" {
			summary.TaskType = string(domain.TaskCategoryGeneral)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session summaries: %w", err)
	}

	return &ListSessionsResult{Sessions: summaries, Total: total, Page: page, PageSize: pageSize}, nil
}

func listSessionStatusesForFilter(status string) ([]domain.SessionStatus, error) {
	switch strings.TrimSpace(status) {
	case "created":
		return []domain.SessionStatus{domain.SessionStatusPending}, nil
	case "running":
		return []domain.SessionStatus{domain.SessionStatusRunning, domain.SessionStatusBlockedInput}, nil
	case "completed":
		return []domain.SessionStatus{domain.SessionStatusSuccess}, nil
	case "cancelled":
		return []domain.SessionStatus{domain.SessionStatusInterrupted}, nil
	case "failed":
		return []domain.SessionStatus{domain.SessionStatusVerificationFailed, domain.SessionStatusBudgetExceeded, domain.SessionStatusFatalError}, nil
	default:
		return nil, fmt.Errorf("unsupported session status filter %q", status)
	}
}

func (s *SQLiteSessionStore) loadSession(ctx context.Context, sessionID string) (domain.Session, PersistedSessionLifecycle, error) {
	session, _, lifecycle, err := s.loadSessionRecord(ctx, sessionID)
	return session, lifecycle, err
}

func (s *SQLiteSessionStore) loadSessionRecord(ctx context.Context, sessionID string) (domain.Session, *storedTask, PersistedSessionLifecycle, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, status, lifecycle_phase, is_active, is_terminal, is_resumable, config_json, created_at, updated_at
		FROM sessions
		WHERE id = ?
	`, sessionID)

	var session domain.Session
	var status string
	var lifecyclePhase sql.NullString
	var activeValue sql.NullInt64
	var terminalValue sql.NullInt64
	var resumableValue sql.NullInt64
	var configJSON sql.NullString
	if err := row.Scan(&session.ID, &session.TaskID, &status, &lifecyclePhase, &activeValue, &terminalValue, &resumableValue, &configJSON, &session.CreatedAt, &session.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Session{}, nil, PersistedSessionLifecycle{}, fmt.Errorf("session %q not found: %w", sessionID, err)
		}
		return domain.Session{}, nil, PersistedSessionLifecycle{}, fmt.Errorf("load session %q: %w", sessionID, err)
	}
	session.Status = domain.SessionStatus(status)
	lifecycle := persistedLifecycleFromRow(session.Status, lifecyclePhase, activeValue, terminalValue, resumableValue)

	metadata, err := parseStoredSessionMetadata(configJSON)
	if err != nil {
		return domain.Session{}, nil, PersistedSessionLifecycle{}, fmt.Errorf("load session %q metadata: %w", sessionID, err)
	}
	row = s.db.QueryRowContext(ctx, `SELECT provenance_json FROM sessions WHERE id = ?`, sessionID)
	var provenanceJSON sql.NullString
	if err := row.Scan(&provenanceJSON); err != nil {
		return domain.Session{}, nil, PersistedSessionLifecycle{}, fmt.Errorf("load session %q provenance: %w", sessionID, err)
	}
	persistedProvenance, err := parseProvenance(provenanceJSON)
	if err != nil {
		return domain.Session{}, nil, PersistedSessionLifecycle{}, fmt.Errorf("load session %q provenance: %w", sessionID, err)
	}
	session.Provenance = mergeStoredProvenance(metadata.Provenance, persistedProvenance)
	return session, metadata.Task, lifecycle, nil
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
	if metadata.Provenance != nil {
		normalized := metadata.Provenance.Normalize()
		metadata.Provenance = &normalized
	}
	return metadata, nil
}

func (s *SQLiteSessionStore) loadSteps(ctx context.Context, sessionID string) ([]domain.Step, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT step_index, action_json, observation_json, provenance_json, verification_json
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
			provenanceJSON   sql.NullString
			verificationJSON string
		)
		if err := rows.Scan(&step.Index, &actionJSON, &observationJSON, &provenanceJSON, &verificationJSON); err != nil {
			return nil, fmt.Errorf("scan step for session %q: %w", sessionID, err)
		}
		if err := json.Unmarshal([]byte(actionJSON), &step.Action); err != nil {
			return nil, fmt.Errorf("unmarshal action for session %q: %w", sessionID, err)
		}
		if err := json.Unmarshal([]byte(observationJSON), &step.Observation); err != nil {
			return nil, fmt.Errorf("unmarshal observation for session %q: %w", sessionID, err)
		}
		step.Provenance, err = parseProvenance(provenanceJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal provenance for session %q: %w", sessionID, err)
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

func (s *SQLiteSessionStore) loadSessionMetadata(ctx context.Context, sessionID string) (storedSessionMetadata, error) {
	if sessionID == "" {
		return storedSessionMetadata{}, nil
	}
	row := s.db.QueryRowContext(ctx, `SELECT config_json FROM sessions WHERE id = ?`, sessionID)
	var configJSON sql.NullString
	if err := row.Scan(&configJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedSessionMetadata{}, nil
		}
		return storedSessionMetadata{}, err
	}
	return parseStoredSessionMetadata(configJSON)
}

func nullableMetadataJSON(payload []byte) any {
	if len(payload) == 0 || string(payload) == "{}" {
		return nil
	}
	return string(payload)
}

func boolToSQLiteInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func persistedLifecycleFromRow(status domain.SessionStatus, phase sql.NullString, active sql.NullInt64, terminal sql.NullInt64, resumable sql.NullInt64) PersistedSessionLifecycle {
	derived := lifecycleForStatus(status)
	if phase.Valid && phase.String != "" {
		derived.Phase = SessionLifecyclePhase(phase.String)
	}
	if active.Valid {
		derived.Active = active.Int64 != 0
	}
	if terminal.Valid {
		derived.Terminal = terminal.Int64 != 0
	}
	if resumable.Valid {
		derived.Resumable = resumable.Int64 != 0
	}
	return derived.PersistedSessionLifecycle
}

type lifecycleRecord struct {
	PersistedSessionLifecycle
	terminatedReason string
}

func (l lifecycleRecord) terminatedReasonValue() any {
	if strings.TrimSpace(l.terminatedReason) == "" {
		return nil
	}
	return l.terminatedReason
}

func lifecycleForStatus(status domain.SessionStatus) lifecycleRecord {
	switch status {
	case domain.SessionStatusPending:
		return lifecycleRecord{PersistedSessionLifecycle: PersistedSessionLifecycle{Phase: SessionLifecycleQueued, Active: false, Terminal: false, Resumable: true}}
	case domain.SessionStatusRunning, domain.SessionStatusBlockedInput:
		return lifecycleRecord{PersistedSessionLifecycle: PersistedSessionLifecycle{Phase: SessionLifecycleRunning, Active: true, Terminal: false, Resumable: false}}
	case domain.SessionStatusSuccess:
		return lifecycleRecord{PersistedSessionLifecycle: PersistedSessionLifecycle{Phase: SessionLifecycleCompleted, Active: false, Terminal: true, Resumable: false}, terminatedReason: string(status)}
	case domain.SessionStatusInterrupted:
		return lifecycleRecord{PersistedSessionLifecycle: PersistedSessionLifecycle{Phase: SessionLifecycleCancelled, Active: false, Terminal: false, Resumable: true}, terminatedReason: string(status)}
	case domain.SessionStatusVerificationFailed, domain.SessionStatusBudgetExceeded, domain.SessionStatusFatalError:
		return lifecycleRecord{PersistedSessionLifecycle: PersistedSessionLifecycle{Phase: SessionLifecycleFailed, Active: false, Terminal: true, Resumable: false}, terminatedReason: string(status)}
	default:
		return lifecycleRecord{PersistedSessionLifecycle: PersistedSessionLifecycle{Phase: SessionLifecycleQueued, Active: false, Terminal: false, Resumable: true}}
	}
}
