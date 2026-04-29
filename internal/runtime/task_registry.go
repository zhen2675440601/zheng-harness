package runtime

import (
	"sort"
	"strings"

	"zheng-harness/internal/domain"
)

const (
	VerifierPolicyCommand  = "command"
	VerifierPolicyEvidence = "evidence"
	VerifierPolicyStateOutput = "state_output"
)

// TaskProtocolCompatibilityDefaults 为旧任务载荷定义增量默认值。
type TaskProtocolCompatibilityDefaults struct {
	ProtocolHint       string
	VerificationPolicy string
}

// TaskProtocolMetadata 描述受支持任务类型的运行时协议形态。
type TaskProtocolMetadata struct {
	TaskType            domain.TaskCategory
	VerifierPolicy      string
	PromptingHints      []string
	CompatibilityDefaults TaskProtocolCompatibilityDefaults
}

// ResolvedTaskProtocol 包含标准化任务数据以及显式协议元数据。
type ResolvedTaskProtocol struct {
	Task     domain.Task
	Metadata TaskProtocolMetadata
}

// TaskRegistry 将受支持的任务类型解析为静态协议元数据。
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
		VerifierPolicy: VerifierPolicyStateOutput,
		PromptingHints: []string{
			"Track file inputs, outputs, and handoff state explicitly.",
			"Confirm artifact paths and completion criteria before finishing.",
		},
		CompatibilityDefaults: TaskProtocolCompatibilityDefaults{
			ProtocolHint:       "artifact-tracking file workflow",
			VerificationPolicy: VerifierPolicyStateOutput,
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

// NewTaskRegistry 构造静态任务类型注册表。
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

// ResolveCategory 返回某任务类型的显式元数据，或返回确定性的回退结果。
func (r *TaskRegistry) ResolveCategory(taskType domain.TaskCategory) TaskProtocolMetadata {
	if r == nil {
		return cloneTaskProtocolMetadata(defaultFallbackTaskProtocolMetadata)
	}
	if metadata, ok := r.entries[taskType.Normalize()]; ok {
		return cloneTaskProtocolMetadata(metadata)
	}
	return cloneTaskProtocolMetadata(r.fallback)
}

// Resolve 标准化任务，并应用注册表边界定义的兼容默认值。
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

// List 按稳定的任务类型顺序返回任务协议元数据。
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
