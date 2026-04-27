package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS plans (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    summary TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS steps (
    session_id TEXT NOT NULL,
    step_index INTEGER NOT NULL,
    action_type TEXT NOT NULL,
    action_summary TEXT NOT NULL,
    action_response TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tool_input TEXT NOT NULL,
    tool_timeout_ns INTEGER NOT NULL,
    observation_summary TEXT NOT NULL,
    observation_final_response TEXT NOT NULL,
    tool_output TEXT NOT NULL,
    tool_error TEXT NOT NULL,
    tool_duration_ns INTEGER NOT NULL,
    verification_passed INTEGER NOT NULL,
    verification_reason TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (session_id, step_index),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE IF NOT EXISTS artifacts (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    path TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE IF NOT EXISTS memory_entries (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    project_key TEXT NOT NULL,
    scope TEXT NOT NULL,
    memory_type TEXT NOT NULL,
    content TEXT NOT NULL,
    source TEXT NOT NULL,
    confidence REAL NOT NULL,
    created_at TEXT NOT NULL,
    last_used_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_steps_session ON steps(session_id, step_index);
CREATE INDEX IF NOT EXISTS idx_memory_lookup ON memory_entries(project_key, scope, memory_type, last_used_at);
`

// Database owns the SQLite connection and schema initialization.
type Database struct {
	db *sql.DB
}

// Open opens and initializes a SQLite database.
func Open(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	database := &Database{db: db}
	if err := database.Init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return database, nil
}

// Init ensures the required schema exists.
func (d *Database) Init(ctx context.Context) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("database is not initialized")
	}
	_, err := d.db.ExecContext(ctx, schema)
	return err
}

// SQL exposes the underlying database handle for repositories.
func (d *Database) SQL() *sql.DB {
	if d == nil {
		return nil
	}
	return d.db
}

// Close releases the database handle.
func (d *Database) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}
