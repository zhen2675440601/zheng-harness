package runtime

import (
	"sort"
	"strings"

	"zheng-harness/internal/domain"
)

const (
	VerifierPolicyCommand  = "command"
	VerifierPolicyEvidence = "evidence"
	VerifierPolicyChecklist = "checklist"
)

// TaskProtocolCompatibilityDefaults defines additive defaults for older task payloads.
type TaskProtocolCompatibilityDefaults struct {
	ProtocolHint       string
	VerificationPolicy string
}

// TaskProtocolMetadata describes the runtime protocol shape for a supported task type.
type TaskProtocolMetadata struct {
	TaskType            domain.TaskCategory
	VerifierPolicy      string
	PromptingHints      []string
	CompatibilityDefaults TaskProtocolCompatibilityDefaults
}

// ResolvedTaskProtocol contains normalized task data plus explicit protocol metadata.
type ResolvedTaskProtocol struct {
	Task     domain.Task
	Metadata TaskProtocolMetadata
}

// TaskRegistry resolves supported task types to static protocol metadata.
type TaskRegistry struct {
	entries   map[domain.TaskCategory]TaskProtocolMetadata
	fallback  TaskProtocolMetadata
	listOrder []domain.TaskCategory
}

var staticTaskProtocolMetadata = map[domain.TaskCategory]TaskProtocolMetadata{
	domain.TaskCategoryCoding: {
		TaskType:       domain.TaskCategoryCoding,
		VerifierPolicy: VerifierPolicyCommand,
		PromptingHints: []string{
			"Prefer repository-backed evidence and executable validation.",
			"Use tool calls only when they advance the code or test state.",
		},
		CompatibilityDefaults: TaskProtocolCompatibilityDefaults{
			ProtocolHint:       "cli-first coding workflow",
			VerificationPolicy: VerifierPolicyCommand,
		},
	},
	domain.TaskCategoryResearch: {
		TaskType:       domain.TaskCategoryResearch,
		VerifierPolicy: VerifierPolicyEvidence,
		PromptingHints: []string{
			"Ground claims in gathered evidence before responding.",
			"Prefer concise synthesis over speculative implementation steps.",
		},
		CompatibilityDefaults: TaskProtocolCompatibilityDefaults{
			ProtocolHint:       "evidence-first research workflow",
			VerificationPolicy: VerifierPolicyEvidence,
		},
	},
	domain.TaskCategoryFileWorkflow: {
		TaskType:       domain.TaskCategoryFileWorkflow,
		VerifierPolicy: VerifierPolicyChecklist,
		PromptingHints: []string{
			"Track file inputs, outputs, and handoff state explicitly.",
			"Confirm artifact paths and completion criteria before finishing.",
		},
		CompatibilityDefaults: TaskProtocolCompatibilityDefaults{
			ProtocolHint:       "artifact-tracking file workflow",
			VerificationPolicy: VerifierPolicyChecklist,
		},
	},
}

var defaultFallbackTaskProtocolMetadata = TaskProtocolMetadata{
	TaskType:       domain.TaskCategoryGeneral,
	VerifierPolicy: VerifierPolicyEvidence,
	PromptingHints: []string{
		"Use a conservative general-purpose workflow when no specialized protocol exists.",
	},
	CompatibilityDefaults: TaskProtocolCompatibilityDefaults{
		ProtocolHint:       "general compatibility workflow",
		VerificationPolicy: VerifierPolicyEvidence,
	},
}

// NewTaskRegistry constructs the static task-type registry.
func NewTaskRegistry() *TaskRegistry {
	entries := make(map[domain.TaskCategory]TaskProtocolMetadata, len(staticTaskProtocolMetadata))
	order := make([]domain.TaskCategory, 0, len(staticTaskProtocolMetadata))
	for category, metadata := range staticTaskProtocolMetadata {
		entries[category] = cloneTaskProtocolMetadata(metadata)
		order = append(order, category)
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i] < order[j]
	})
	return &TaskRegistry{
		entries:   entries,
		fallback:  cloneTaskProtocolMetadata(defaultFallbackTaskProtocolMetadata),
		listOrder: order,
	}
}

// ResolveCategory returns explicit metadata for a task type or the deterministic fallback.
func (r *TaskRegistry) ResolveCategory(taskType domain.TaskCategory) TaskProtocolMetadata {
	if r == nil {
		return cloneTaskProtocolMetadata(defaultFallbackTaskProtocolMetadata)
	}
	if metadata, ok := r.entries[taskType.Normalize()]; ok {
		return cloneTaskProtocolMetadata(metadata)
	}
	return cloneTaskProtocolMetadata(r.fallback)
}

// Resolve normalizes a task and applies compatibility defaults from the registry boundary.
func (r *TaskRegistry) Resolve(task domain.Task) ResolvedTaskProtocol {
	metadata := r.ResolveCategory(task.CategoryOrDefault())
	resolved := task.Normalize()
	resolved.Category = metadata.TaskType
	if strings.TrimSpace(resolved.ProtocolHint) == "" {
		resolved.ProtocolHint = metadata.CompatibilityDefaults.ProtocolHint
	}
	if strings.TrimSpace(resolved.VerificationPolicy) == "" {
		resolved.VerificationPolicy = metadata.CompatibilityDefaults.VerificationPolicy
	}
	return ResolvedTaskProtocol{Task: resolved, Metadata: metadata}
}

// List returns task protocol metadata in stable task-type order.
func (r *TaskRegistry) List() []TaskProtocolMetadata {
	if r == nil {
		return nil
	}
	items := make([]TaskProtocolMetadata, 0, len(r.listOrder))
	for _, category := range r.listOrder {
		items = append(items, cloneTaskProtocolMetadata(r.entries[category]))
	}
	return items
}

func cloneTaskProtocolMetadata(metadata TaskProtocolMetadata) TaskProtocolMetadata {
	metadata.PromptingHints = append([]string(nil), metadata.PromptingHints...)
	return metadata
}
