package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  status TEXT NOT NULL,
  lifecycle_phase TEXT,
  is_active INTEGER NOT NULL DEFAULT 0,
  is_terminal INTEGER NOT NULL DEFAULT 0,
  is_resumable INTEGER NOT NULL DEFAULT 0,
  config_json TEXT,
  provenance_json TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  terminated_reason TEXT
);

CREATE TABLE IF NOT EXISTS plans (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS steps (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  step_index INTEGER NOT NULL,
  action_json TEXT NOT NULL,
  observation_json TEXT NOT NULL,
  provenance_json TEXT,
  verification_json TEXT,
  created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS memory_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT,
  scope TEXT NOT NULL CHECK(scope IN ('session','project','global')),
  type TEXT NOT NULL CHECK(type IN ('preference','fact','summary')),
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  source TEXT NOT NULL,
  confidence INTEGER NOT NULL DEFAULT 50 CHECK(confidence >= 0 AND confidence <= 100),
  provenance TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_steps_session ON steps(session_id);
CREATE INDEX IF NOT EXISTS idx_memory_session ON memory_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_memory_key ON memory_entries(key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
`

func openSQLite(dbPath string) (*sql.DB, error) {
	return openSQLiteWithOptions(dbPath, SQLiteOptions{})
}

func initializeSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		return fmt.Errorf("initialize sqlite schema: %w", err)
	}
	if err := ensureSessionColumn(ctx, db, "lifecycle_phase", `ALTER TABLE sessions ADD COLUMN lifecycle_phase TEXT`); err != nil {
		return err
	}
	if err := ensureSessionColumn(ctx, db, "is_active", `ALTER TABLE sessions ADD COLUMN is_active INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if err := ensureSessionColumn(ctx, db, "is_terminal", `ALTER TABLE sessions ADD COLUMN is_terminal INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if err := ensureSessionColumn(ctx, db, "is_resumable", `ALTER TABLE sessions ADD COLUMN is_resumable INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	return nil
}

func ensureSessionColumn(ctx context.Context, db *sql.DB, columnName, alterStmt string) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(sessions)`)
	if err != nil {
		return fmt.Errorf("inspect sqlite schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return fmt.Errorf("scan sqlite schema: %w", err)
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, alterStmt); err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}
	return nil
}
