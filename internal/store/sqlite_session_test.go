package store_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	memorypolicy "zheng-harness/internal/memory"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/store"
)

func TestSessionPersistenceAndResume(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "session.db")

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sessionStore.Close()
	})

	memoryStore, err := store.NewMemoryStore(dbPath)
	if err != nil {
		t.Fatalf("NewMemoryStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = memoryStore.Close()
	})

	now := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	session := domain.Session{
		ID:        "session-1",
		TaskID:    "task-1",
		Status:    domain.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan := domain.Plan{
		ID:        "plan-1",
		TaskID:    session.TaskID,
		Summary:   "inspect repository and persist progress",
		CreatedAt: now,
	}
	step := domain.Step{
		Index: 1,
		Action: domain.Action{
			Type:    domain.ActionTypeToolCall,
			Summary: "read runtime implementation",
			ToolCall: &domain.ToolCall{
				Name:    "read_file",
				Input:   "internal/runtime/runtime.go",
				Timeout: 2 * time.Second,
			},
		},
		Observation: domain.Observation{
			Summary:       "runtime loop persists session state",
			FinalResponse: "resume should rebuild session history",
			ToolResult: &domain.ToolResult{
				ToolName: "read_file",
				Output:   "runtime.go contents",
				Duration: 150 * time.Millisecond,
			},
		},
		Verification: domain.VerificationResult{
			Passed: true,
			Reason: "step evidence recorded",
		},
	}

	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, plan); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	if err := sessionStore.AppendStep(ctx, session.ID, step); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	entry, err := memoryStore.Write(ctx, memorypolicy.Entry{
		SessionID:  session.ID,
		Scope:      memorypolicy.ScopeSession,
		Type:       memorypolicy.TypeFact,
		Key:        "repo.layout",
		Value:      "runtime persists steps",
		Source:     "unit-test",
		Confidence: 90,
		Provenance: "TestSessionPersistenceAndResume",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if entry.ID == 0 {
		t.Fatalf("Write() returned empty entry ID")
	}

	resumedSession, resumedPlan, resumedSteps, err := sessionStore.ResumeSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}

	if resumedSession != session {
		t.Fatalf("resumed session mismatch: got %#v want %#v", resumedSession, session)
	}
	if !reflect.DeepEqual(resumedPlan, plan) {
		t.Fatalf("resumed plan mismatch: got %#v want %#v", resumedPlan, plan)
	}
	if len(resumedSteps) != 1 {
		t.Fatalf("len(resumedSteps) = %d, want 1", len(resumedSteps))
	}
	if !reflect.DeepEqual(resumedSteps[0], step) {
		t.Fatalf("resumed step mismatch: got %#v want %#v", resumedSteps[0], step)
	}

	remembered, err := memoryStore.Recall(ctx, memorypolicy.Query{
		SessionID: session.ID,
		Scope:     memorypolicy.ScopeSession,
		Key:       "repo.layout",
	})
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(remembered) != 1 {
		t.Fatalf("len(remembered) = %d, want 1", len(remembered))
	}
	if remembered[0].Value != "runtime persists steps" {
		t.Fatalf("remembered value = %q, want %q", remembered[0].Value, "runtime persists steps")
	}

	blocked, err := memoryStore.Recall(ctx, memorypolicy.Query{
		SessionID: "other-session",
		Scope:     memorypolicy.ScopeSession,
		Key:       "repo.layout",
	})
	if err != nil {
		t.Fatalf("Recall() cross-session error = %v", err)
	}
	if len(blocked) != 0 {
		t.Fatalf("cross-session recall returned %d entries, want 0", len(blocked))
	}
	if err := memoryStore.Remember(ctx, session.ID, domain.Observation{Summary: "implicit observation only"}); err != nil {
		t.Fatalf("Remember() error = %v", err)
	}
	implicit, err := memoryStore.Recall(ctx, memorypolicy.Query{
		SessionID: session.ID,
		Scope:     memorypolicy.ScopeSession,
	})
	if err != nil {
		t.Fatalf("Recall() after Remember error = %v", err)
	}
	if len(implicit) != 2 {
		t.Fatalf("implicit Remember() should create a persisted entry; got %d", len(implicit))
	}
	if implicit[0].Value != "implicit observation only" && implicit[1].Value != "implicit observation only" {
		t.Fatalf("implicit Remember() did not persist observation value; entries = %#v", implicit)
	}
}
