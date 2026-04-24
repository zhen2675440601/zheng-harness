package domain

import "time"

// Task is the user request the agent is working to satisfy.
type Task struct {
	ID          string
	Description string
	Goal        string
	CreatedAt   time.Time
}
