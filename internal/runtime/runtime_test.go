package runtime

import (
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestOverflowSignalForEventPrefersTerminalSessionEvent(t *testing.T) {
	t.Parallel()

	complete, err := domain.SessionComplete("session-overflow-terminal", "success")
	if err != nil {
		t.Fatalf("SessionComplete() error = %v", err)
	}

	overflow := overflowSignalForEvent(*complete)
	if overflow == nil {
		t.Fatal("overflow signal = nil")
	}
	if overflow.Type != domain.EventSessionComplete {
		t.Fatalf("overflow type = %q, want %q", overflow.Type, domain.EventSessionComplete)
	}
	var payload domain.SessionCompletePayload
	if err := overflow.GetPayload(&payload); err != nil {
		t.Fatalf("decode overflow payload: %v", err)
	}
	if payload.SessionID != "session-overflow-terminal" || payload.Status != "success" {
		t.Fatalf("overflow payload = %+v, want terminal session payload", payload)
	}
	if overflow.Timestamp.IsZero() {
		t.Fatal("overflow timestamp should be preserved")
	}
	if time.Since(overflow.Timestamp) > time.Hour {
		t.Fatalf("overflow timestamp = %v, want recent event timestamp", overflow.Timestamp)
	}
}
