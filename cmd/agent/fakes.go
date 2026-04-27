package main

import (
	"context"
	"fmt"
	"time"

	"zheng-harness/internal/domain"
)

type FakeModel struct {
	Delay      time.Duration
	LastTools  []domain.ToolInfo
	LastMemory []domain.MemoryEntry
}

func (m *FakeModel) CreatePlan(_ context.Context, task domain.Task, _ domain.Session, memory []domain.MemoryEntry) (domain.Plan, error) {
	m.LastMemory = append([]domain.MemoryEntry(nil), memory...)
	return domain.Plan{
		ID:        "plan-" + task.ID,
		TaskID:    task.ID,
		Summary:   fmt.Sprintf("Complete task input: %s", task.Description),
		Steps:     []domain.Step{{Index: 1, Action: domain.Action{Type: domain.ActionTypeRespond, Summary: "Produce a deterministic final response"}}},
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (m *FakeModel) NextAction(ctx context.Context, task domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, memory []domain.MemoryEntry, tools []domain.ToolInfo) (domain.Action, error) {
	m.LastMemory = append([]domain.MemoryEntry(nil), memory...)
	m.LastTools = append([]domain.ToolInfo(nil), tools...)
	if err := waitForDelay(ctx, m.Delay); err != nil {
		return domain.Action{}, err
	}
	return domain.Action{
		Type:     domain.ActionTypeRespond,
		Summary:  "Return a stubbed completion response",
		Response: fmt.Sprintf("completed task: %s", task.Description),
	}, nil
}

func (m *FakeModel) Observe(ctx context.Context, task domain.Task, _ domain.Session, _ domain.Plan, action domain.Action, result *domain.ToolResult) (domain.Observation, error) {
	if err := waitForDelay(ctx, m.Delay); err != nil {
		return domain.Observation{}, err
	}
	return domain.Observation{
		Summary:       fmt.Sprintf("Processed task input %q", task.Description),
		ToolResult:    result,
		FinalResponse: action.Response,
	}, nil
}

type FakeVerifier struct{}

type FakeToolExecutor struct{}

func (FakeVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	if observation.FinalResponse != "" {
		return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "final response recorded"}, nil
	}
	return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: "final response missing"}, nil
}

func (FakeToolExecutor) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{
		ToolName: call.Name,
		Output:   fmt.Sprintf("fake tool executed: %s (%s)", call.Name, call.Input),
	}, nil
}

func waitForDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
