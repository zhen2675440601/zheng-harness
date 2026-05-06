package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type SQLiteOptions struct {
	EnableWAL bool
}

func openSQLiteWithOptions(dbPath string, opts SQLiteOptions) (*sql.DB, error) {
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
	if err := applySQLiteOptions(context.Background(), db, opts); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func applySQLiteOptions(ctx context.Context, db *sql.DB, opts SQLiteOptions) error {
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if !opts.EnableWAL {
		return nil
	}
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	return nil
}
