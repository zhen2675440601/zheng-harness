package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"zheng-harness/internal/domain"
	memorypolicy "zheng-harness/internal/memory"
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

func (s *SQLiteMemoryStore) Remember(_ context.Context, _ string, _ domain.Observation) error {
	return nil
}

func (s *SQLiteMemoryStore) Write(ctx context.Context, entry memorypolicy.Entry) (memorypolicy.Entry, error) {
	if s == nil || s.db == nil {
		return memorypolicy.Entry{}, errors.New("sqlite memory store is not initialized")
	}
	if err := memorypolicy.ValidateEntry(entry); err != nil {
		return memorypolicy.Entry{}, err
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
	`, nullableSessionID(entry.SessionID), string(entry.Scope), string(entry.Type), entry.Key, entry.Value, entry.Source, entry.Confidence, nullableString(entry.Provenance), entry.CreatedAt.UTC(), entry.UpdatedAt.UTC())
	if err != nil {
		return memorypolicy.Entry{}, fmt.Errorf("write memory entry %q: %w", entry.Key, err)
	}
	entryID, err := result.LastInsertId()
	if err != nil {
		return memorypolicy.Entry{}, fmt.Errorf("read inserted memory entry id: %w", err)
	}
	entry.ID = entryID
	return entry, nil
}

func (s *SQLiteMemoryStore) Recall(ctx context.Context, query memorypolicy.Query) ([]memorypolicy.Entry, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite memory store is not initialized")
	}
	if err := memorypolicy.ValidateQuery(query); err != nil {
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
	if query.Scope == memorypolicy.ScopeSession {
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

	entries := make([]memorypolicy.Entry, 0)
	for rows.Next() {
		var (
			entry      memorypolicy.Entry
			sessionID  sql.NullString
			provenance sql.NullString
			scope      string
			entryType  string
		)
		if err := rows.Scan(&entry.ID, &sessionID, &scope, &entryType, &entry.Key, &entry.Value, &entry.Source, &entry.Confidence, &provenance, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recalled memory entry: %w", err)
		}
		entry.SessionID = sessionID.String
		entry.Scope = memorypolicy.Scope(scope)
		entry.Type = memorypolicy.Type(entryType)
		entry.Provenance = provenance.String
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recalled memory entries: %w", err)
	}
	return entries, nil
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
