package domain

import "time"

// Plan captures the current intended sequence of work for a task.
type Plan struct {
	ID        string
	TaskID    string
	Summary   string
	Steps     []Step
	CreatedAt time.Time
}
