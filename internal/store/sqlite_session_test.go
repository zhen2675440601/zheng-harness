package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
			Status: domain.VerificationStatusPassed,
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

	entry, err := memoryStore.Write(ctx, domain.MemoryEntry{
		SessionID:  session.ID,
		Scope:      domain.MemoryScopeSession,
		Type:       domain.MemoryTypeFact,
		Key:        "repo.layout",
		Content:    "runtime persists steps",
		Source:     "unit-test",
		Confidence: 90,
		Provenance: "TestSessionPersistenceAndResume",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if entry.ID == "" {
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

	remembered, err := memoryStore.Recall(ctx, domain.RecallQuery{
		SessionID: session.ID,
		Scope:     domain.MemoryScopeSession,
		Key:       "repo.layout",
	})
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(remembered) != 1 {
		t.Fatalf("len(remembered) = %d, want 1", len(remembered))
	}
	if remembered[0].Content != "runtime persists steps" {
		t.Fatalf("remembered value = %q, want %q", remembered[0].Content, "runtime persists steps")
	}

	blocked, err := memoryStore.Recall(ctx, domain.RecallQuery{
		SessionID: "other-session",
		Scope:     domain.MemoryScopeSession,
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
	implicit, err := memoryStore.Recall(ctx, domain.RecallQuery{
		SessionID: session.ID,
		Scope:     domain.MemoryScopeSession,
	})
	if err != nil {
		t.Fatalf("Recall() after Remember error = %v", err)
	}
	if len(implicit) != 2 {
		t.Fatalf("implicit Remember() should create a persisted entry; got %d", len(implicit))
	}
	if implicit[0].Content != "implicit observation only" && implicit[1].Content != "implicit observation only" {
		t.Fatalf("implicit Remember() did not persist observation value; entries = %#v", implicit)
	}
}

func TestSessionTaskMetadataPersistsAndReloads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "session-task.db")

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sessionStore.Close()
	})

	now := time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)
	session := domain.Session{
		ID:        "session-task-meta",
		TaskID:    "task-task-meta",
		Status:    domain.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan := domain.Plan{
		ID:        "plan-task-meta",
		TaskID:    session.TaskID,
		Summary:   "prepare artifact handoff",
		CreatedAt: now,
	}
	task := domain.Task{
		ID:                 session.TaskID,
		Description:        plan.Summary,
		Goal:               plan.Summary,
		Category:           domain.TaskCategoryFileWorkflow,
		ProtocolHint:       "artifact-tracking file workflow",
		VerificationPolicy: "state_output",
		CreatedAt:          now,
	}

	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(ctx, session.ID, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, plan); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	loadedTask, ok, err := sessionStore.LoadTask(ctx, session.ID)
	if err != nil {
		t.Fatalf("LoadTask() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadTask() ok = false, want true")
	}
	if !reflect.DeepEqual(loadedTask, task.Normalize()) {
		t.Fatalf("loaded task mismatch: got %#v want %#v", loadedTask, task.Normalize())
	}

	var configJSON string
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.QueryRowContext(ctx, `SELECT config_json FROM sessions WHERE id = ?`, session.ID).Scan(&configJSON); err != nil {
		t.Fatalf("query config_json: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(configJSON), &payload); err != nil {
		t.Fatalf("unmarshal config_json: %v", err)
	}
	taskPayload, ok := payload["task"].(map[string]any)
	if !ok {
		t.Fatalf("config_json.task = %#v, want object", payload["task"])
	}
	if got := taskPayload["category"]; got != string(domain.TaskCategoryFileWorkflow) {
		t.Fatalf("config_json.task.category = %#v, want %q", got, domain.TaskCategoryFileWorkflow)
	}
	if got := taskPayload["verification_policy"]; got != "state_output" {
		t.Fatalf("config_json.task.verification_policy = %#v, want state_output", got)
	}
}

func TestLoadTaskFallsBackForLegacySessionsWithoutMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy-session.db")

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sessionStore.Close()
	})

	now := time.Date(2026, time.April, 26, 10, 0, 0, 0, time.UTC)
	session := domain.Session{
		ID:        "legacy-session",
		TaskID:    "legacy-task",
		Status:    domain.SessionStatusSuccess,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan := domain.Plan{
		ID:        "legacy-plan",
		TaskID:    session.TaskID,
		Summary:   "legacy persisted plan summary",
		CreatedAt: now,
	}

	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, plan); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	loadedTask, ok, err := sessionStore.LoadTask(ctx, session.ID)
	if err != nil {
		t.Fatalf("LoadTask() error = %v", err)
	}
	if ok {
		t.Fatal("LoadTask() ok = true, want false for legacy session")
	}
	if loadedTask.ID != session.TaskID {
		t.Fatalf("loaded task ID = %q, want %q", loadedTask.ID, session.TaskID)
	}
	if loadedTask.Description != plan.Summary || loadedTask.Goal != plan.Summary {
		t.Fatalf("loaded task summary fallback mismatch: got %#v want %q", loadedTask, plan.Summary)
	}
	if loadedTask.Category != domain.TaskCategoryGeneral {
		t.Fatalf("loaded task category = %q, want %q", loadedTask.Category, domain.TaskCategoryGeneral)
	}
}

func TestResumeEligibilityUsesPersistedLifecycleState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "resume-eligibility.db")

	sessionStore, err := store.NewSQLiteSessionStoreWithOptions(dbPath, store.SQLiteOptions{EnableWAL: true})
	if err != nil {
		t.Fatalf("NewSQLiteSessionStoreWithOptions() error = %v", err)
	}
	t.Cleanup(func() { _ = sessionStore.Close() })

	now := time.Date(2026, time.May, 1, 9, 0, 0, 0, time.UTC)
	basePlan := domain.Plan{ID: "plan-resume", TaskID: "resume-task", Summary: "resume task summary", CreatedAt: now}

	if err := sessionStore.SavePlan(ctx, basePlan); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	testCases := []struct {
		name           string
		session        domain.Session
		wantPhase      store.SessionLifecyclePhase
		wantActive     bool
		wantTerminal   bool
		wantResumable  bool
		wantResumeErr  bool
	}{
		{name: "pending queued resumable", session: domain.Session{ID: "pending-session", TaskID: "resume-task", Status: domain.SessionStatusPending, CreatedAt: now, UpdatedAt: now}, wantPhase: store.SessionLifecycleQueued, wantResumable: true},
		{name: "running active not resumable", session: domain.Session{ID: "running-session", TaskID: "resume-task", Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now}, wantPhase: store.SessionLifecycleRunning, wantActive: true, wantResumeErr: true},
		{name: "interrupted cancelled resumable", session: domain.Session{ID: "interrupted-session", TaskID: "resume-task", Status: domain.SessionStatusInterrupted, CreatedAt: now, UpdatedAt: now}, wantPhase: store.SessionLifecycleCancelled, wantResumable: true},
		{name: "success completed terminal", session: domain.Session{ID: "success-session", TaskID: "resume-task", Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, wantPhase: store.SessionLifecycleCompleted, wantTerminal: true, wantResumeErr: true},
		{name: "fatal failed terminal", session: domain.Session{ID: "fatal-session", TaskID: "resume-task", Status: domain.SessionStatusFatalError, CreatedAt: now, UpdatedAt: now}, wantPhase: store.SessionLifecycleFailed, wantTerminal: true, wantResumeErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := sessionStore.SaveSession(ctx, tc.session); err != nil {
				t.Fatalf("SaveSession() error = %v", err)
			}
			inspect, err := sessionStore.InspectSession(ctx, tc.session.ID)
			if err != nil {
				t.Fatalf("InspectSession() error = %v", err)
			}
			if inspect.Lifecycle.Phase != tc.wantPhase {
				t.Fatalf("lifecycle phase = %q, want %q", inspect.Lifecycle.Phase, tc.wantPhase)
			}
			if inspect.Lifecycle.Active != tc.wantActive {
				t.Fatalf("lifecycle active = %v, want %v", inspect.Lifecycle.Active, tc.wantActive)
			}
			if inspect.Lifecycle.Terminal != tc.wantTerminal {
				t.Fatalf("lifecycle terminal = %v, want %v", inspect.Lifecycle.Terminal, tc.wantTerminal)
			}
			if inspect.Lifecycle.Resumable != tc.wantResumable {
				t.Fatalf("lifecycle resumable = %v, want %v", inspect.Lifecycle.Resumable, tc.wantResumable)
			}
			_, _, _, err = sessionStore.ResumeSession(ctx, tc.session.ID)
			if err != nil {
				t.Fatalf("ResumeSession() state load error = %v", err)
			}
			if tc.wantResumeErr == inspect.Lifecycle.Resumable {
				t.Fatalf("resume policy mismatch: wantResumeErr=%v lifecycle=%#v", tc.wantResumeErr, inspect.Lifecycle)
			}
		})
	}

	resumable := domain.Session{ID: "transition-session", TaskID: "resume-task", Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now}
	if err := sessionStore.SaveSession(ctx, resumable); err != nil {
		t.Fatalf("SaveSession(running) error = %v", err)
	}
	if err := sessionStore.SaveTask(ctx, resumable.ID, domain.Task{ID: resumable.TaskID, Description: "transition", Goal: "transition", Category: domain.TaskCategoryCoding, CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask(running) error = %v", err)
	}
	resumable.Status = domain.SessionStatusInterrupted
	resumable.UpdatedAt = now.Add(time.Minute)
	if err := sessionStore.SaveSession(ctx, resumable); err != nil {
		t.Fatalf("SaveSession(interrupted) error = %v", err)
	}
	if err := sessionStore.SaveTask(ctx, resumable.ID, domain.Task{ID: resumable.TaskID, Description: "transition updated", Goal: "transition updated", Category: domain.TaskCategoryCoding, CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask(interrupted) error = %v", err)
	}
	inspect, err := sessionStore.InspectSession(ctx, resumable.ID)
	if err != nil {
		t.Fatalf("InspectSession(transition) error = %v", err)
	}
	if inspect.Lifecycle.Phase != store.SessionLifecycleCancelled || !inspect.Lifecycle.Resumable || inspect.Lifecycle.Active {
		t.Fatalf("transition lifecycle = %#v, want cancelled resumable inactive", inspect.Lifecycle)
	}
}
