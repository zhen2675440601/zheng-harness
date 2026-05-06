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

// TaskAwareVerifier 优先依据任务元数据分发验证，其次再使用兼容性回退。
type TaskAwareVerifier struct {
	fallbackPolicy string
	mode           string
	executor       domain.ToolExecutor
	registry       verifierRegistry
	resolved       domain.Verifier
}

type verifierRegistry interface {
	Resolve(id, mode string, executor domain.ToolExecutor) (domain.Verifier, error)
}

// NewTaskAwareVerifier 构造统一的任务感知型验证边界。
func NewTaskAwareVerifier(mode string, executor domain.ToolExecutor) *TaskAwareVerifier {
	return NewTaskAwareVerifierWithRegistry(mode, executor, nil)
}

// NewTaskAwareVerifierWithRegistry constructs a task-aware verifier using a host-owned verifier registry.
func NewTaskAwareVerifierWithRegistry(mode string, executor domain.ToolExecutor, registry verifierRegistry) *TaskAwareVerifier {
	fallbackPolicy := PolicyCommandBacked
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "strict", "standard", "":
		fallbackPolicy = PolicyCommandBacked
	default:
		fallbackPolicy = PolicyCommandBacked
	}
	if registry == nil {
		registry = newBuiltinVerifierRegistry()
	}

	return &TaskAwareVerifier{
		fallbackPolicy: fallbackPolicy,
		mode:           mode,
		executor:       executor,
		registry:       registry,
	}
}

// Verify 实现 domain.Verifier。
func (v *TaskAwareVerifier) Verify(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	if rawPolicy := strings.TrimSpace(task.VerificationPolicy); rawPolicy != "" && normalizeVerificationPolicy(rawPolicy) == "" {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: fmt.Sprintf("verification policy %q not configured", rawPolicy)}, nil
	}
	policy := v.selectPolicy(task)
	strategy, err := v.registry.Resolve(policy, v.mode, v.executor)
	if err != nil {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: fmt.Sprintf("verification policy %q not configured", policy)}, nil
	}
	v.resolved = strategy
	return strategy.Verify(ctx, task, session, plan, steps, observation)
}

func (v *TaskAwareVerifier) VerifierProvenance() *domain.PluginMetadata {
	carrier, ok := v.resolved.(verifierProvenanceCarrier)
	if !ok || carrier == nil {
		return nil
	}
	return carrier.VerifierProvenance()
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
	case PolicyStateOutput, "state", "output", "checklist", "file_workflow", "file-workflow":
		return PolicyStateOutput
	default:
		return ""
	}
}

type builtinVerifierRegistry struct {
	entries map[string]func(mode string, executor domain.ToolExecutor) domain.Verifier
}

func newBuiltinVerifierRegistry() verifierRegistry {
	return builtinVerifierRegistry{
		entries: map[string]func(mode string, executor domain.ToolExecutor) domain.Verifier{
			PolicyCommandBacked: func(_ string, executor domain.ToolExecutor) domain.Verifier {
				return NewCommandVerifier(executor)
			},
			PolicyEvidenceBased: func(_ string, _ domain.ToolExecutor) domain.Verifier {
				return ResearchVerifier{}
			},
			PolicyStateOutput: func(_ string, _ domain.ToolExecutor) domain.Verifier {
				return FileWorkflowVerifier{}
			},
		},
	}
}

func (r builtinVerifierRegistry) Resolve(id, mode string, executor domain.ToolExecutor) (domain.Verifier, error) {
	selectedID := strings.TrimSpace(id)
	if selectedID == "" {
		selectedID = PolicyCommandBacked
	}
	factory, ok := r.entries[selectedID]
	if !ok {
		return nil, fmt.Errorf("verifier %q not found", selectedID)
	}
	return factory(mode, executor), nil
}
