package domain

import "time"

// Plan 记录任务当前预期的工作步骤序列。
type Plan struct {
	ID        string
	TaskID    string
	Summary   string
	Steps     []Step
	CreatedAt time.Time
}
