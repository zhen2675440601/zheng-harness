package store_test

import (
	"context"
	"path/filepath"
	"testing"

	memorypolicy "zheng-harness/internal/memory"
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

	_, err = memoryStore.Write(ctx, memorypolicy.Entry{
		Scope:      memorypolicy.ScopeSession,
		Type:       memorypolicy.TypeFact,
		Key:        "missing.session",
		Value:      "value",
		Source:     "unit-test",
		Confidence: 101,
	})
	if err == nil {
		t.Fatal("Write() error = nil, want validation failure")
	}

	if _, err := memoryStore.Write(ctx, memorypolicy.Entry{
		Scope:      memorypolicy.ScopeGlobal,
		Type:       memorypolicy.TypeSummary,
		Key:        "readonly.global",
		Value:      "should be rejected",
		Source:     "unit-test",
		Confidence: 80,
	}); err == nil {
		t.Fatal("Write() global scope error = nil, want readonly rejection")
	}
}
