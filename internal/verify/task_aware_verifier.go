package verify

import (
	"context"
	"fmt"
	"strings"

	"zheng-harness/internal/domain"
)

const (
	PolicyCommandBacked = "command"
	PolicyEvidenceBased = "evidence"
	PolicyStateOutput   = "state_output"
)

// TaskAwareVerifier dispatches verification by task metadata first, then compatibility fallback.
type TaskAwareVerifier struct {
	fallbackPolicy string
	strategies     map[string]domain.Verifier
}

// NewTaskAwareVerifier constructs a central task-aware verification boundary.
func NewTaskAwareVerifier(mode string, executor domain.ToolExecutor) *TaskAwareVerifier {
	fallbackPolicy := PolicyCommandBacked
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "strict", "standard", "":
		fallbackPolicy = PolicyCommandBacked
	default:
		fallbackPolicy = PolicyCommandBacked
	}

	return &TaskAwareVerifier{
		fallbackPolicy: fallbackPolicy,
		strategies: map[string]domain.Verifier{
			PolicyCommandBacked: NewCommandVerifier(executor),
			PolicyEvidenceBased: ResearchVerifier{},
			PolicyStateOutput:   FileWorkflowVerifier{},
		},
	}
}

// Verify implements domain.Verifier.
func (v *TaskAwareVerifier) Verify(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	policy := v.selectPolicy(task)
	strategy, ok := v.strategies[policy]
	if !ok {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: fmt.Sprintf("verification policy %q not configured", policy)}, nil
	}
	return strategy.Verify(ctx, task, session, plan, steps, observation)
}

func (v *TaskAwareVerifier) selectPolicy(task domain.Task) string {
	if policy := normalizeVerificationPolicy(task.VerificationPolicy); policy != "" {
		return policy
	}

	switch task.CategoryOrDefault() {
	case domain.TaskCategoryCoding:
		return PolicyCommandBacked
	case domain.TaskCategoryResearch:
		return PolicyEvidenceBased
	case domain.TaskCategoryFileWorkflow:
		return PolicyStateOutput
	}

	return v.fallbackPolicy
}

func normalizeVerificationPolicy(raw string) string {
	text := normalizePolicyToken(raw)
	switch text {
	case "", "default":
		return ""
	case PolicyCommandBacked, "command_based", "coding", "code", "exec_command":
		return PolicyCommandBacked
	case PolicyEvidenceBased, "evidence_based", "research":
		return PolicyEvidenceBased
	case PolicyStateOutput, "state", "output", "file_workflow", "file-workflow":
		return PolicyStateOutput
	default:
		return ""
	}
}
