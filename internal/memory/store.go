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

// Store 按校验规则持久化受约束的记忆条目。
type Store struct {
	db         *sql.DB
	projectKey string
}

// NewStore 构造一个由 SQLite 支撑的记忆存储。
func NewStore(database *store.Database, projectKey string) *Store {
	return &Store{db: database.SQL(), projectKey: projectKey}
}

// Remember 实现 domain.MemoryStore，同时阻止自主写入。
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

// Write 在通过策略校验后持久化显式记忆条目。
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

// LoadRelevant 返回对某个会话可见且可检查的相关记忆条目。
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
		parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at for memory entry %q: %w", entry.ID, err)
		}
		parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse last_used_at for memory entry %q: %w", entry.ID, err)
		}
		entry.CreatedAt = parsedCreatedAt
		entry.UpdatedAt = parsedUpdatedAt
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
