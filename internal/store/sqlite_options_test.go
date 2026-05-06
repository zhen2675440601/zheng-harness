package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestNewSQLiteSessionStoreWithOptionsEnablesWAL(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteSessionStoreWithOptions(t.TempDir()+"/wal.db", SQLiteOptions{EnableWAL: true})
	if err != nil {
		t.Fatalf("NewSQLiteSessionStoreWithOptions() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	var mode string
	if err := store.db.QueryRow(`PRAGMA journal_mode;`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode scan error = %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestSQLiteWALSupportsConcurrentSessionWrites(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "concurrent-wal.db")
	writerA, err := NewSQLiteSessionStoreWithOptions(dbPath, SQLiteOptions{EnableWAL: true})
	if err != nil {
		t.Fatalf("NewSQLiteSessionStoreWithOptions(writerA) error = %v", err)
	}
	t.Cleanup(func() { _ = writerA.Close() })
	writerB, err := NewSQLiteSessionStoreWithOptions(dbPath, SQLiteOptions{EnableWAL: true})
	if err != nil {
		t.Fatalf("NewSQLiteSessionStoreWithOptions(writerB) error = %v", err)
	}
	t.Cleanup(func() { _ = writerB.Close() })

	stores := []*SQLiteSessionStore{writerA, writerB}
	for i, s := range stores {
		sessionID := fmt.Sprintf("session-%d", i)
		now := time.Date(2026, time.May, 2, 10, 0, i, 0, time.UTC)
		if err := s.SaveSession(ctx, domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatalf("SaveSession(%s) error = %v", sessionID, err)
		}
		if err := s.SavePlan(ctx, domain.Plan{ID: "plan-" + sessionID, TaskID: sessionID, Summary: sessionID, CreatedAt: now}); err != nil {
			t.Fatalf("SavePlan(%s) error = %v", sessionID, err)
		}
	}

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for i, s := range stores {
		wg.Add(1)
		go func(idx int, current *SQLiteSessionStore) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", idx)
			for stepIndex := 1; stepIndex <= 25; stepIndex++ {
				step := domain.Step{
					Index: stepIndex,
					Action: domain.Action{Type: domain.ActionTypeRespond, Summary: fmt.Sprintf("step-%d", stepIndex)},
					Observation: domain.Observation{Summary: fmt.Sprintf("observation-%d", stepIndex)},
					Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed},
				}
				if err := current.AppendStep(ctx, sessionID, step); err != nil {
					errCh <- fmt.Errorf("append %s step %d: %w", sessionID, stepIndex, err)
					return
				}
			}
		}(i, s)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	for i, s := range stores {
		inspect, err := s.InspectSession(ctx, fmt.Sprintf("session-%d", i))
		if err != nil {
			t.Fatalf("InspectSession(session-%d) error = %v", i, err)
		}
		if len(inspect.Steps) != 25 {
			t.Fatalf("session-%d steps = %d, want 25", i, len(inspect.Steps))
		}
	}
}

func TestSQLiteWALKeepsInspectReadableDuringActiveWritesAndAfterWriterShutdown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "inspect-wal.db")
	writer, err := NewSQLiteSessionStoreWithOptions(dbPath, SQLiteOptions{EnableWAL: true})
	if err != nil {
		t.Fatalf("NewSQLiteSessionStoreWithOptions(writer) error = %v", err)
	}
	reader, err := NewSQLiteSessionStoreWithOptions(dbPath, SQLiteOptions{EnableWAL: true})
	if err != nil {
		t.Fatalf("NewSQLiteSessionStoreWithOptions(reader) error = %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	now := time.Date(2026, time.May, 2, 11, 0, 0, 0, time.UTC)
	if err := writer.SaveSession(ctx, domain.Session{ID: "session-inspect", TaskID: "task-inspect", Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := writer.SavePlan(ctx, domain.Plan{ID: "plan-inspect", TaskID: "task-inspect", Summary: "inspect while writing", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	if err := writer.AppendStep(ctx, "session-inspect", domain.Step{Index: 1, Action: domain.Action{Type: domain.ActionTypeRespond, Summary: "first"}, Observation: domain.Observation{Summary: "written"}, Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}}); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	tx, err := writer.RawDB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, now.Add(time.Minute), "session-inspect"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("tx ExecContext() error = %v", err)
	}

	inspect, err := reader.InspectSession(ctx, "session-inspect")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("InspectSession() during active writer error = %v", err)
	}
	if inspect.Session.ID != "session-inspect" || len(inspect.Steps) != 1 {
		_ = tx.Rollback()
		t.Fatalf("InspectSession() during active writer = %#v, want readable committed state", inspect)
	}

	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	inspect, err = reader.InspectSession(ctx, "session-inspect")
	if err != nil {
		t.Fatalf("InspectSession() after writer shutdown error = %v", err)
	}
	if inspect.Session.Status != domain.SessionStatusRunning {
		t.Fatalf("status after writer shutdown = %q, want %q", inspect.Session.Status, domain.SessionStatusRunning)
	}
}
