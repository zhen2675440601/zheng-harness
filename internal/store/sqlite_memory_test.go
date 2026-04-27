package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/store"
)

func TestMemoryPolicyRejectsInvalidEntry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.db")

	memoryStore, err := store.NewMemoryStore(dbPath)
	if err != nil {
		t.Fatalf("NewMemoryStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = memoryStore.Close()
	})

	_, err = memoryStore.Write(ctx, domain.MemoryEntry{
		Scope:      domain.MemoryScopeSession,
		Type:       domain.MemoryTypeFact,
		Key:        "missing.session",
		Content:    "value",
		Source:     "unit-test",
		Confidence: 101,
	})
	if err == nil {
		t.Fatal("Write() error = nil, want validation failure")
	}

	if _, err := memoryStore.Write(ctx, domain.MemoryEntry{
		Scope:      domain.MemoryScopeGlobal,
		Type:       domain.MemoryTypeSummary,
		Key:        "readonly.global",
		Content:    "should be rejected",
		Source:     "unit-test",
		Confidence: 80,
	}); err == nil {
		t.Fatal("Write() global scope error = nil, want readonly rejection")
	}
}

func TestRememberPersistsObservationAsMemoryEntry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.db")

	memoryStore, err := store.NewMemoryStore(dbPath)
	if err != nil {
		t.Fatalf("NewMemoryStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = memoryStore.Close()
	})

	if err := memoryStore.Remember(ctx, "session-remember", domain.Observation{
		Summary: "tool output summary",
		ToolResult: &domain.ToolResult{
			ToolName: "read_file",
			Output:   "runtime observation output",
		},
	}); err != nil {
		t.Fatalf("Remember() error = %v", err)
	}

	entries, err := memoryStore.Recall(ctx, domain.RecallQuery{
		SessionID: "session-remember",
		Scope:     domain.MemoryScopeSession,
		Type:      domain.MemoryTypeSummary,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.SessionID != "session-remember" {
		t.Fatalf("entry.SessionID = %q, want %q", entry.SessionID, "session-remember")
	}
	if entry.Scope != domain.MemoryScopeSession {
		t.Fatalf("entry.Scope = %q, want %q", entry.Scope, domain.MemoryScopeSession)
	}
	if entry.Type != domain.MemoryTypeSummary {
		t.Fatalf("entry.Type = %q, want %q", entry.Type, domain.MemoryTypeSummary)
	}
	if entry.Content != "runtime observation output" {
		t.Fatalf("entry.Content = %q, want %q", entry.Content, "runtime observation output")
	}
	if entry.Source != "tool:read_file" {
		t.Fatalf("entry.Source = %q, want %q", entry.Source, "tool:read_file")
	}
	if entry.Confidence != 50 {
		t.Fatalf("entry.Confidence = %d, want 50", entry.Confidence)
	}
	if entry.Provenance != "runtime.Remember" {
		t.Fatalf("entry.Provenance = %q, want %q", entry.Provenance, "runtime.Remember")
	}
	if entry.Key == "" {
		t.Fatal("entry.Key = empty, want generated observation key")
	}
}
