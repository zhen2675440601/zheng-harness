package domain

import (
	"encoding/json"
	"time"
)

// TaskCategory classifies the general protocol shape a task expects.
type TaskCategory string

const (
	TaskCategoryCoding       TaskCategory = "coding"
	TaskCategoryResearch     TaskCategory = "research"
	TaskCategoryFileWorkflow TaskCategory = "file_workflow"
	TaskCategoryGeneral      TaskCategory = "general"
)

// Normalize returns a deterministic supported category.
func (c TaskCategory) Normalize() TaskCategory {
	switch c {
	case TaskCategoryCoding, TaskCategoryResearch, TaskCategoryFileWorkflow, TaskCategoryGeneral:
		return c
	default:
		return TaskCategoryGeneral
	}
}

// Task is the user request the agent is working to satisfy.
type Task struct {
	ID                 string
	Description        string
	Goal               string
	Category           TaskCategory
	ProtocolHint       string
	VerificationPolicy string
	CreatedAt          time.Time
}

// CategoryOrDefault returns a supported category even when the stored value is empty or unknown.
func (t Task) CategoryOrDefault() TaskCategory {
	return t.Category.Normalize()
}

// Normalize applies additive compatibility defaults for task metadata.
func (t Task) Normalize() Task {
	t.Category = t.CategoryOrDefault()
	return t
}

// MarshalJSON preserves backward-compatible field names while emitting normalized categories.
func (t Task) MarshalJSON() ([]byte, error) {
	type taskJSON Task
	return json.Marshal(taskJSON(t.Normalize()))
}

// UnmarshalJSON backfills additive task metadata for older persisted payloads.
func (t *Task) UnmarshalJSON(data []byte) error {
	type taskJSON Task
	var decoded taskJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = Task(decoded).Normalize()
	return nil
}
