package domain

import (
	"encoding/json"
	"time"
)

// TaskCategory 对任务所期望的通用协议形态进行分类。
type TaskCategory string

const (
	TaskCategoryCoding       TaskCategory = "coding"
	TaskCategoryResearch     TaskCategory = "research"
	TaskCategoryFileWorkflow TaskCategory = "file_workflow"
	TaskCategoryGeneral      TaskCategory = "general"
)

// Normalize 返回一个确定性的受支持类别。
func (c TaskCategory) Normalize() TaskCategory {
	switch c {
	case TaskCategoryCoding, TaskCategoryResearch, TaskCategoryFileWorkflow, TaskCategoryGeneral:
		return c
	default:
		return TaskCategoryGeneral
	}
}

// Task 是 agent 正在努力满足的用户请求。
type Task struct {
	ID                 string
	Description        string
	Goal               string
	Category           TaskCategory
	ProtocolHint       string
	VerificationPolicy string
	CreatedAt          time.Time
}

// CategoryOrDefault 即使在存储值为空或未知时也会返回一个受支持的类别。
func (t Task) CategoryOrDefault() TaskCategory {
	return t.Category.Normalize()
}

// Normalize 为任务元数据应用增量兼容默认值。
func (t Task) Normalize() Task {
	t.Category = t.CategoryOrDefault()
	return t
}

// MarshalJSON 在输出标准化类别时保留向后兼容的字段名。
func (t Task) MarshalJSON() ([]byte, error) {
	type taskJSON Task
	return json.Marshal(taskJSON(t.Normalize()))
}

// UnmarshalJSON 为旧的持久化载荷补齐增量任务元数据。
func (t *Task) UnmarshalJSON(data []byte) error {
	type taskJSON Task
	var decoded taskJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = Task(decoded).Normalize()
	return nil
}
