package domain

import "time"

// MemoryScope defines the persistence boundary for a memory entry.
type MemoryScope string

const (
	MemoryScopeSession MemoryScope = "session"
	MemoryScopeProject MemoryScope = "project"
	MemoryScopeGlobal  MemoryScope = "global"
)

// MemoryType classifies the kind of memory content.
type MemoryType string

const (
	MemoryTypePreference MemoryType = "preference"
	MemoryTypeFact       MemoryType = "fact"
	MemoryTypeSummary    MemoryType = "summary"
)

// MemoryEntry is a persisted memory record with provenance and confidence.
type MemoryEntry struct {
	ID         string
	SessionID  string
	ProjectKey string
	Scope      MemoryScope
	Type       MemoryType
	Key        string
	Content    string
	Source     string
	Confidence int
	Provenance string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  *time.Time
}

// RecallQuery filters memory entries during recall.
type RecallQuery struct {
	SessionID string
	Scope     MemoryScope
	Type      MemoryType
	Key       string
	Source    string
	Limit     int
	Prefix    string
}
