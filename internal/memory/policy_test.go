package memory

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateEntryAcceptsProjectAndSessionScopes(t *testing.T) {
	t.Parallel()

	entries := []Entry{
		{SessionID: "session-1", Scope: ScopeSession, Type: TypeFact, Key: "repo", Content: "uses sqlite", Source: "step-1", Confidence: 90},
		{Scope: ScopeProject, Type: TypeSummary, Key: "style", Content: "concise", Source: "step-2", Confidence: 50},
	}

	for _, entry := range entries {
		entry := entry
		t.Run(string(entry.Scope), func(t *testing.T) {
			t.Parallel()
			if err := ValidateEntry(entry); err != nil {
				t.Fatalf("ValidateEntry() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateEntryRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		entry Entry
		want string
	}{
		{name: "invalid scope", entry: Entry{Scope: Scope("bad"), Type: TypeFact, Key: "k", Content: "v", Source: "s", Confidence: 1}, want: "invalid scope"},
		{name: "invalid type", entry: Entry{Scope: ScopeProject, Type: Type("bad"), Key: "k", Content: "v", Source: "s", Confidence: 1}, want: "invalid type"},
		{name: "empty key", entry: Entry{Scope: ScopeProject, Type: TypeFact, Content: "v", Source: "s", Confidence: 1}, want: "key must not be empty"},
		{name: "empty content", entry: Entry{Scope: ScopeProject, Type: TypeFact, Key: "k", Source: "s", Confidence: 1}, want: "content must not be empty"},
		{name: "empty source", entry: Entry{Scope: ScopeProject, Type: TypeFact, Key: "k", Content: "v", Confidence: 1}, want: "source must not be empty"},
		{name: "bad confidence", entry: Entry{Scope: ScopeProject, Type: TypeFact, Key: "k", Content: "v", Source: "s", Confidence: 101}, want: "confidence must be between 0 and 100"},
		{name: "missing session id", entry: Entry{Scope: ScopeSession, Type: TypeFact, Key: "k", Content: "v", Source: "s", Confidence: 1}, want: "session scope requires session id"},
	}

	for _, tt := range tests {
		tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateEntry(tt.entry)
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("ValidateEntry() error = %v, want containing %q", err, tt.want)
				}
			})
		}
}

func TestValidateEntryRejectsGlobalScopeWrite(t *testing.T) {
	t.Parallel()

	err := ValidateEntry(Entry{Scope: ScopeGlobal, Type: TypeFact, Key: "k", Content: "v", Source: "s", Confidence: 1})
	if !errors.Is(err, ErrGlobalScopeWrite) {
		t.Fatalf("ValidateEntry() error = %v, want ErrGlobalScopeWrite", err)
	}
}

func TestValidateQueryValidationRules(t *testing.T) {
	t.Parallel()

	valid := Query{SessionID: "session-1", Scope: ScopeSession, Type: TypeFact, Key: "repo", Limit: 10}
	if err := ValidateQuery(valid); err != nil {
		t.Fatalf("ValidateQuery() error = %v, want nil", err)
	}

	tests := []struct {
		name string
		query Query
		want error
		wantText string
	}{
		{name: "invalid scope", query: Query{Scope: Scope("bad")}, wantText: "invalid query scope"},
		{name: "invalid type", query: Query{Type: Type("bad")}, wantText: "invalid query type"},
		{name: "missing session", query: Query{Scope: ScopeSession}, want: ErrSessionScopeMatch},
		{name: "negative limit", query: Query{Limit: -1}, wantText: "must not be negative"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateQuery(tt.query)
			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("ValidateQuery() error = %v, want %v", err, tt.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("ValidateQuery() error = %v, want containing %q", err, tt.wantText)
			}
		})
	}
}
