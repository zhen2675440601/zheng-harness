package memory

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Scope string

const (
	ScopeSession Scope = "session"
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

type Type string

const (
	TypePreference Type = "preference"
	TypeFact       Type = "fact"
	TypeSummary    Type = "summary"
)

type Entry struct {
	ID         int64
	SessionID  string
	Scope      Scope
	Type       Type
	Key        string
	Value      string
	Source     string
	Confidence int
	Provenance string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Query struct {
	SessionID string
	Scope     Scope
	Type      Type
	Key       string
	Limit     int
}

var (
	ErrInvalidEntry      = errors.New("memory entry failed validation")
	ErrGlobalScopeWrite  = errors.New("global memory is read-only")
	ErrSessionScopeMatch = errors.New("session scope requires matching session id")
)

func ValidateEntry(entry Entry) error {
	if !isValidScope(entry.Scope) {
		return fmt.Errorf("%w: invalid scope %q", ErrInvalidEntry, entry.Scope)
	}
	if !isValidType(entry.Type) {
		return fmt.Errorf("%w: invalid type %q", ErrInvalidEntry, entry.Type)
	}
	if strings.TrimSpace(entry.Key) == "" {
		return fmt.Errorf("%w: key must not be empty", ErrInvalidEntry)
	}
	if strings.TrimSpace(entry.Value) == "" {
		return fmt.Errorf("%w: value must not be empty", ErrInvalidEntry)
	}
	if strings.TrimSpace(entry.Source) == "" {
		return fmt.Errorf("%w: source must not be empty", ErrInvalidEntry)
	}
	if entry.Confidence < 0 || entry.Confidence > 100 {
		return fmt.Errorf("%w: confidence must be between 0 and 100", ErrInvalidEntry)
	}
	if entry.Scope == ScopeSession && strings.TrimSpace(entry.SessionID) == "" {
		return fmt.Errorf("%w: session scope requires session id", ErrInvalidEntry)
	}
	if entry.Scope == ScopeGlobal {
		return ErrGlobalScopeWrite
	}
	return nil
}

func ValidateQuery(query Query) error {
	if query.Scope != "" && !isValidScope(query.Scope) {
		return fmt.Errorf("invalid query scope %q", query.Scope)
	}
	if query.Type != "" && !isValidType(query.Type) {
		return fmt.Errorf("invalid query type %q", query.Type)
	}
	if query.Scope == ScopeSession && strings.TrimSpace(query.SessionID) == "" {
		return ErrSessionScopeMatch
	}
	if query.Limit < 0 {
		return errors.New("query limit must not be negative")
	}
	return nil
}

func isValidScope(scope Scope) bool {
	switch scope {
	case ScopeSession, ScopeProject, ScopeGlobal:
		return true
	default:
		return false
	}
}

func isValidType(kind Type) bool {
	switch kind {
	case TypePreference, TypeFact, TypeSummary:
		return true
	default:
		return false
	}
}
