package domain

import "time"

// MemoryScope 定义记忆条目的持久化边界。
type MemoryScope string

const (
	MemoryScopeSession MemoryScope = "session"
	MemoryScopeProject MemoryScope = "project"
	MemoryScopeGlobal  MemoryScope = "global"
)

// MemoryType 对记忆内容的类型进行分类。
type MemoryType string

const (
	MemoryTypePreference MemoryType = "preference"
	MemoryTypeFact       MemoryType = "fact"
	MemoryTypeSummary    MemoryType = "summary"
)

// MemoryEntry 是带有来源与置信度的持久化记忆记录。
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

// RecallQuery 用于在回忆时筛选记忆条目。
type RecallQuery struct {
	SessionID string
	Scope     MemoryScope
	Type      MemoryType
	Key       string
	Source    string
	Limit     int
	Prefix    string
}
