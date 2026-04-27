package memory_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/memory"
	"zheng-harness/internal/store"
)

func TestSessionPersistenceAndResume(t *testing.T) {
	t.Parallel()

	db, repo, mem := newPersistenceFixture(t)
	defer db.Close()

	now := time.Date(2026, 4, 24, 18, 0, 0, 0, time.UTC)
	session := domain.Session{ID: "session-1", TaskID: "task-1", Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now}
	plan := domain.Plan{ID: "plan-1", TaskID: session.TaskID, Summary: "persist state", CreatedAt: now}
	step := domain.Step{
		Index: 1,
		Action: domain.Action{
			Type:    domain.ActionTypeToolCall,
			Summary: "Read file",
			ToolCall: &domain.ToolCall{Name: "read_file", Input: "README.md", Timeout: time.Second},
		},
		Observation: domain.Observation{
			Summary:       "remember: stable preference",
			FinalResponse: "done",
			ToolResult:    &domain.ToolResult{ToolName: "read_file", Output: "README present", Duration: time.Millisecond},
		},
		Verification: domain.VerificationResult{Passed: true, Reason: "verified"},
	}

	ctx := context.Background()
	if err := repo.SaveSession(ctx, session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := repo.SavePlan(ctx, plan); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	if err := repo.AppendStep(ctx, session.ID, step); err != nil {
		t.Fatalf("append step: %v", err)
	}
	if err := mem.Remember(ctx, session.ID, step.Observation); err != nil {
		t.Fatalf("remember: %v", err)
	}

	resumed, err := repo.Resume(ctx, session.ID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.Session.ID != session.ID {
		t.Fatalf("session id = %q, want %q", resumed.Session.ID, session.ID)
	}
	if len(resumed.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(resumed.Steps))
	}
	if resumed.Steps[0].Observation.ToolResult == nil || resumed.Steps[0].Observation.ToolResult.Output != "README present" {
		t.Fatalf("resumed step tool output mismatch: %+v", resumed.Steps[0].Observation.ToolResult)
	}

	entries, err := mem.LoadRelevant(ctx, session.ID)
	if err != nil {
		t.Fatalf("load memory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("memory count = %d, want 1", len(entries))
	}
	if entries[0].Scope != domain.MemoryScopeSession || entries[0].Type != domain.MemoryTypeSummary {
		t.Fatalf("memory entry = %+v, want session summary", entries[0])
	}
}

func TestMemoryPolicyRejectsInvalidEntry(t *testing.T) {
	t.Parallel()

	db, _, mem := newPersistenceFixture(t)
	defer db.Close()

	err := mem.Write(context.Background(), memory.Entry{
		ID:         "entry-1",
		SessionID:  "session-1",
		ProjectKey: "project-1",
		Scope:      "unknown",
		Type:       domain.MemoryTypeFact,
		Key:        "bad.entry",
		Content:    "bad",
		Source:     "unit-test",
		Confidence: 90,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected invalid scope error")
	}
}

func newPersistenceFixture(t *testing.T) (*store.Database, *store.SessionRepository, *memory.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db, store.NewSessionRepository(db), memory.NewStore(db, "project-1")
}
