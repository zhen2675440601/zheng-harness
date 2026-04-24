package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  status TEXT NOT NULL,
  config_json TEXT,
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

CREATE INDEX IF NOT EXISTS idx_steps_session ON steps(session_id);
CREATE INDEX IF NOT EXISTS idx_memory_session ON memory_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_memory_key ON memory_entries(key);
`

func openSQLite(dbPath string) (*sql.DB, error) {
	trimmed := strings.TrimSpace(dbPath)
	if trimmed == "" {
		return nil, fmt.Errorf("db path must not be empty")
	}

	db, err := sql.Open("sqlite", trimmed)
	if err != nil {
		return nil, err
	}

	if err := initializeSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func initializeSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		return fmt.Errorf("initialize sqlite schema: %w", err)
	}
	return nil
}
