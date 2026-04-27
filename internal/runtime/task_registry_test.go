package runtime

import (
	"testing"

	"zheng-harness/internal/domain"
)

func TestTaskRegistryResolveSupportedCategory(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()
	resolved := registry.Resolve(domain.Task{ID: "task-1", Category: domain.TaskCategoryCoding})

	if resolved.Task.Category != domain.TaskCategoryCoding {
		t.Fatalf("resolved category = %q, want %q", resolved.Task.Category, domain.TaskCategoryCoding)
	}
	if resolved.Metadata.TaskType != domain.TaskCategoryCoding {
		t.Fatalf("metadata task type = %q, want %q", resolved.Metadata.TaskType, domain.TaskCategoryCoding)
	}
	if resolved.Metadata.VerifierPolicy != VerifierPolicyCommand {
		t.Fatalf("verifier policy = %q, want %q", resolved.Metadata.VerifierPolicy, VerifierPolicyCommand)
	}
	if resolved.Task.VerificationPolicy != VerifierPolicyCommand {
		t.Fatalf("task verification policy = %q, want %q", resolved.Task.VerificationPolicy, VerifierPolicyCommand)
	}
	if resolved.Task.ProtocolHint == "" {
		t.Fatal("protocol hint = empty, want default hint")
	}
	if len(resolved.Metadata.PromptingHints) == 0 {
		t.Fatal("prompting hints = empty, want static hints")
	}
}

func TestTaskRegistryResolveSupportedCategoriesUseExplicitPolicies(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()
	tests := []struct {
		name   string
		task   domain.TaskCategory
		policy string
	}{
		{name: "coding", task: domain.TaskCategoryCoding, policy: VerifierPolicyCommand},
		{name: "research", task: domain.TaskCategoryResearch, policy: VerifierPolicyEvidence},
		{name: "file workflow", task: domain.TaskCategoryFileWorkflow, policy: VerifierPolicyChecklist},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resolved := registry.Resolve(domain.Task{ID: "task-" + string(tc.task), Category: tc.task})
			if resolved.Task.Category != tc.task {
				t.Fatalf("resolved category = %q, want %q", resolved.Task.Category, tc.task)
			}
			if resolved.Metadata.TaskType != tc.task {
				t.Fatalf("metadata task type = %q, want %q", resolved.Metadata.TaskType, tc.task)
			}
			if resolved.Metadata.VerifierPolicy != tc.policy {
				t.Fatalf("metadata verifier policy = %q, want %q", resolved.Metadata.VerifierPolicy, tc.policy)
			}
			if resolved.Task.VerificationPolicy != tc.policy {
				t.Fatalf("task verification policy = %q, want %q", resolved.Task.VerificationPolicy, tc.policy)
			}
			if resolved.Task.ProtocolHint == "" {
				t.Fatal("protocol hint = empty, want compatibility default")
			}
		})
	}
}

func TestTaskRegistryResolveUnknownCategoryUsesDeterministicFallback(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()
	resolved := registry.Resolve(domain.Task{ID: "task-2", Category: domain.TaskCategory("unsupported")})

	if resolved.Task.Category != domain.TaskCategoryGeneral {
		t.Fatalf("resolved category = %q, want %q", resolved.Task.Category, domain.TaskCategoryGeneral)
	}
	if resolved.Metadata.TaskType != domain.TaskCategoryGeneral {
		t.Fatalf("metadata task type = %q, want %q", resolved.Metadata.TaskType, domain.TaskCategoryGeneral)
	}
	if resolved.Metadata.VerifierPolicy != VerifierPolicyEvidence {
		t.Fatalf("verifier policy = %q, want %q", resolved.Metadata.VerifierPolicy, VerifierPolicyEvidence)
	}
	if resolved.Task.ProtocolHint != defaultFallbackTaskProtocolMetadata.CompatibilityDefaults.ProtocolHint {
		t.Fatalf("protocol hint = %q, want %q", resolved.Task.ProtocolHint, defaultFallbackTaskProtocolMetadata.CompatibilityDefaults.ProtocolHint)
	}
	if resolved.Task.VerificationPolicy != defaultFallbackTaskProtocolMetadata.CompatibilityDefaults.VerificationPolicy {
		t.Fatalf("verification policy = %q, want %q", resolved.Task.VerificationPolicy, defaultFallbackTaskProtocolMetadata.CompatibilityDefaults.VerificationPolicy)
	}
}

func TestTaskRegistryResolvePreservesExplicitTaskOverrides(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()
	resolved := registry.Resolve(domain.Task{
		ID:                 "task-3",
		Category:           domain.TaskCategoryResearch,
		ProtocolHint:       "custom-hint",
		VerificationPolicy: "manual-review",
	})

	if resolved.Task.ProtocolHint != "custom-hint" {
		t.Fatalf("protocol hint = %q, want custom-hint", resolved.Task.ProtocolHint)
	}
	if resolved.Task.VerificationPolicy != "manual-review" {
		t.Fatalf("verification policy = %q, want manual-review", resolved.Task.VerificationPolicy)
	}
	if resolved.Metadata.TaskType != domain.TaskCategoryResearch {
		t.Fatalf("metadata task type = %q, want %q", resolved.Metadata.TaskType, domain.TaskCategoryResearch)
	}
	if resolved.Metadata.VerifierPolicy != VerifierPolicyEvidence {
		t.Fatalf("metadata verifier policy = %q, want %q", resolved.Metadata.VerifierPolicy, VerifierPolicyEvidence)
	}
}

func TestTaskRegistryListStableOrder(t *testing.T) {
	t.Parallel()

	registry := NewTaskRegistry()
	list := registry.List()
	if len(list) != 3 {
		t.Fatalf("list len = %d, want 3", len(list))
	}
	want := []domain.TaskCategory{
		domain.TaskCategoryCoding,
		domain.TaskCategoryFileWorkflow,
		domain.TaskCategoryResearch,
	}
	for i, category := range want {
		if list[i].TaskType != category {
			t.Fatalf("list[%d] = %q, want %q", i, list[i].TaskType, category)
		}
	}
}
