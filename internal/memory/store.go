package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/store"
)

// Store persists constrained memory entries with validation rules.
type Store struct {
	db         *sql.DB
	projectKey string
}

// NewStore constructs a memory store backed by SQLite.
func NewStore(database *store.Database, projectKey string) *Store {
	return &Store{db: database.SQL(), projectKey: projectKey}
}

// Remember implements domain.MemoryStore while preventing autonomous writes.
func (s *Store) Remember(ctx context.Context, sessionID string, observation domain.Observation) error {
	text := strings.TrimSpace(observation.Summary)
	if !strings.HasPrefix(strings.ToLower(text), "remember:") {
		return nil
	}
	entry := Entry{
		ID:         fmt.Sprintf("%s-%d", sessionID, time.Now().UTC().UnixNano()),
		SessionID:  sessionID,
		ProjectKey: s.projectKey,
		Scope:      domain.MemoryScopeSession,
		Type:       domain.MemoryTypeSummary,
		Key:        fmt.Sprintf("observation-%d", time.Now().UTC().UnixNano()),
		Content:    strings.TrimSpace(strings.TrimPrefix(text, "remember:")),
		Source:     "runtime_observation",
		Confidence: 50,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if entry.Content == "" {
		return nil
	}
	return s.Write(ctx, entry)
}

// Write persists an explicit memory entry after policy validation.
func (s *Store) Write(ctx context.Context, entry Entry) error {
	if err := ValidateEntry(entry); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO memory_entries (
  id, session_id, project_key, scope, memory_type, content, source, confidence, created_at, last_used_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		entry.ID,
		entry.SessionID,
		entry.ProjectKey,
		entry.Scope,
		entry.Type,
		entry.Content,
		entry.Source,
		entry.Confidence,
		entry.CreatedAt.UTC().Format(time.RFC3339Nano),
		entry.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// LoadRelevant returns inspectable memory entries visible to a session.
func (s *Store) LoadRelevant(ctx context.Context, sessionID string) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, project_key, scope, memory_type, content, source, confidence, created_at, last_used_at
FROM memory_entries
WHERE session_id = ? OR (project_key = ? AND scope IN (?, ?))
ORDER BY last_used_at DESC
`, sessionID, s.projectKey, domain.MemoryScopeProject, domain.MemoryScopeGlobal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var createdAt, updatedAt string
		if err := rows.Scan(
			&entry.ID,
			&entry.SessionID,
			&entry.ProjectKey,
			&entry.Scope,
			&entry.Type,
			&entry.Content,
			&entry.Source,
			&entry.Confidence,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		entry.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		entry.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
