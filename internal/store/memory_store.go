package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

type SQLiteMemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(dbPath string) (*SQLiteMemoryStore, error) {
	db, err := openSQLite(dbPath)
	if err != nil {
		return nil, err
	}
	return &SQLiteMemoryStore{db: db}, nil
}

func (s *SQLiteMemoryStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteMemoryStore) Remember(ctx context.Context, sessionID string, observation domain.Observation) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite memory store is not initialized")
	}
	if strings.TrimSpace(observation.Summary) == "" && observation.ToolResult == nil {
		return nil
	}

	value := observation.Summary
	if observation.ToolResult != nil && observation.ToolResult.Output != "" {
		value = observation.ToolResult.Output
	}
	source := "runtime"
	if observation.ToolResult != nil && observation.ToolResult.ToolName != "" {
		source = "tool:" + observation.ToolResult.ToolName
	}

	now := time.Now().UTC()
	entry := domain.MemoryEntry{
		SessionID:  sessionID,
		Scope:      domain.MemoryScopeSession,
		Type:       domain.MemoryTypeSummary,
		Key:        "observation-" + formatTimestamp(now),
		Content:    value,
		Source:     source,
		Confidence: 50,
		Provenance: "runtime.Remember",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := validateMemoryEntry(entry); err != nil {
		return fmt.Errorf("validate runtime observation: %w", err)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_entries (session_id, scope, type, key, value, source, confidence, provenance, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nullableSessionID(entry.SessionID), string(entry.Scope), string(entry.Type), entry.Key, entry.Content, entry.Source, entry.Confidence, nullableString(entry.Provenance), entry.CreatedAt.UTC(), entry.UpdatedAt.UTC())
	if err != nil {
		return fmt.Errorf("persist runtime observation: %w", err)
	}
	return nil
}

func formatTimestamp(t time.Time) string {
	return t.Format("20060102150405")
}

func (s *SQLiteMemoryStore) Write(ctx context.Context, entry domain.MemoryEntry) (domain.MemoryEntry, error) {
	if s == nil || s.db == nil {
		return domain.MemoryEntry{}, errors.New("sqlite memory store is not initialized")
	}
	if err := validateMemoryEntry(entry); err != nil {
		return domain.MemoryEntry{}, err
	}

	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = entry.CreatedAt
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_entries (session_id, scope, type, key, value, source, confidence, provenance, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nullableSessionID(entry.SessionID), string(entry.Scope), string(entry.Type), entry.Key, entry.Content, entry.Source, entry.Confidence, nullableString(entry.Provenance), entry.CreatedAt.UTC(), entry.UpdatedAt.UTC())
	if err != nil {
		return domain.MemoryEntry{}, fmt.Errorf("write memory entry %q: %w", entry.Key, err)
	}
	entryID, err := result.LastInsertId()
	if err != nil {
		return domain.MemoryEntry{}, fmt.Errorf("read inserted memory entry id: %w", err)
	}
	entry.ID = strconv.FormatInt(entryID, 10)
	return entry, nil
}

func (s *SQLiteMemoryStore) Recall(ctx context.Context, query domain.RecallQuery) ([]domain.MemoryEntry, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite memory store is not initialized")
	}
	if err := validateRecallQuery(query); err != nil {
		return nil, err
	}

	statement := `
		SELECT id, session_id, scope, type, key, value, source, confidence, provenance, created_at, updated_at
		FROM memory_entries
		WHERE 1 = 1
	`
	args := make([]any, 0, 5)

	if query.Scope != "" {
		statement += ` AND scope = ?`
		args = append(args, string(query.Scope))
	}
	if query.Scope == domain.MemoryScopeSession {
		statement += ` AND session_id = ?`
		args = append(args, query.SessionID)
	}
	if query.Type != "" {
		statement += ` AND type = ?`
		args = append(args, string(query.Type))
	}
	if strings.TrimSpace(query.Key) != "" {
		statement += ` AND key = ?`
		args = append(args, query.Key)
	}

	statement += ` ORDER BY updated_at DESC, id DESC`
	if query.Limit > 0 {
		statement += ` LIMIT ?`
		args = append(args, query.Limit)
	}

	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("recall memory entries: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.MemoryEntry, 0)
	for rows.Next() {
		var (
			entry      domain.MemoryEntry
			sessionID  sql.NullString
			provenance sql.NullString
			scope      string
			entryType  string
		)
		if err := rows.Scan(&entry.ID, &sessionID, &scope, &entryType, &entry.Key, &entry.Content, &entry.Source, &entry.Confidence, &provenance, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recalled memory entry: %w", err)
		}
		entry.SessionID = sessionID.String
		entry.Scope = domain.MemoryScope(scope)
		entry.Type = domain.MemoryType(entryType)
		entry.Provenance = provenance.String
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recalled memory entries: %w", err)
	}
	return entries, nil
}

func validateMemoryEntry(entry domain.MemoryEntry) error {
	if strings.TrimSpace(entry.Key) == "" {
		return errors.New("memory entry failed validation: key must not be empty")
	}
	if strings.TrimSpace(entry.Content) == "" {
		return errors.New("memory entry failed validation: content must not be empty")
	}
	if strings.TrimSpace(entry.Source) == "" {
		return errors.New("memory entry failed validation: source must not be empty")
	}
	if entry.Confidence < 0 || entry.Confidence > 100 {
		return errors.New("memory entry failed validation: confidence must be between 0 and 100")
	}
	switch entry.Scope {
	case domain.MemoryScopeSession, domain.MemoryScopeProject, domain.MemoryScopeGlobal:
	default:
		return fmt.Errorf("memory entry failed validation: invalid scope %q", entry.Scope)
	}
	switch entry.Type {
	case domain.MemoryTypePreference, domain.MemoryTypeFact, domain.MemoryTypeSummary:
	default:
		return fmt.Errorf("memory entry failed validation: invalid type %q", entry.Type)
	}
	if entry.Scope == domain.MemoryScopeSession && strings.TrimSpace(entry.SessionID) == "" {
		return errors.New("memory entry failed validation: session scope requires session id")
	}
	if entry.Scope == domain.MemoryScopeGlobal {
		return errors.New("global memory is read-only")
	}
	return nil
}

func validateRecallQuery(query domain.RecallQuery) error {
	if query.Scope != "" {
		switch query.Scope {
		case domain.MemoryScopeSession, domain.MemoryScopeProject, domain.MemoryScopeGlobal:
		default:
			return fmt.Errorf("invalid query scope %q", query.Scope)
		}
	}
	if query.Type != "" {
		switch query.Type {
		case domain.MemoryTypePreference, domain.MemoryTypeFact, domain.MemoryTypeSummary:
		default:
			return fmt.Errorf("invalid query type %q", query.Type)
		}
	}
	if query.Scope == domain.MemoryScopeSession && strings.TrimSpace(query.SessionID) == "" {
		return errors.New("session scope requires matching session id")
	}
	if query.Limit < 0 {
		return errors.New("query limit must not be negative")
	}
	return nil
}

func nullableSessionID(sessionID string) any {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func nullableString(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
