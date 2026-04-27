package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTaskCategoryNormalizeDefaultsToGeneral(t *testing.T) {
	t.Parallel()

	if got := TaskCategory("").Normalize(); got != TaskCategoryGeneral {
		t.Fatalf("empty category = %q, want %q", got, TaskCategoryGeneral)
	}

	if got := TaskCategory("unknown").Normalize(); got != TaskCategoryGeneral {
		t.Fatalf("unknown category = %q, want %q", got, TaskCategoryGeneral)
	}

	if got := TaskCategoryCoding.Normalize(); got != TaskCategoryCoding {
		t.Fatalf("coding category = %q, want %q", got, TaskCategoryCoding)
	}
}

func TestTaskCategoryOrDefaultUsesGeneralForZeroValue(t *testing.T) {
	t.Parallel()

	task := Task{}
	if got := task.CategoryOrDefault(); got != TaskCategoryGeneral {
		t.Fatalf("CategoryOrDefault() = %q, want %q", got, TaskCategoryGeneral)
	}
}

func TestTaskUnmarshalJSONDefaultsMissingCategoryToGeneral(t *testing.T) {
	t.Parallel()

	var task Task
	if err := json.Unmarshal([]byte(`{"ID":"task-1","Description":"inspect repo","Goal":"propose next step"}`), &task); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}

	if task.Category != TaskCategoryGeneral {
		t.Fatalf("Category = %q, want %q", task.Category, TaskCategoryGeneral)
	}
	if task.ProtocolHint != "" {
		t.Fatalf("ProtocolHint = %q, want empty", task.ProtocolHint)
	}
	if task.VerificationPolicy != "" {
		t.Fatalf("VerificationPolicy = %q, want empty", task.VerificationPolicy)
	}
}

func TestTaskUnmarshalJSONNormalizesUnknownCategory(t *testing.T) {
	t.Parallel()

	var task Task
	if err := json.Unmarshal([]byte(`{"ID":"task-2","Category":"unsupported"}`), &task); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}

	if task.Category != TaskCategoryGeneral {
		t.Fatalf("Category = %q, want %q", task.Category, TaskCategoryGeneral)
	}
}

func TestTaskMarshalJSONEmitsNormalizedCategory(t *testing.T) {
	t.Parallel()

	createdAt := time.Unix(1700000000, 0).UTC()
	raw, err := json.Marshal(Task{
		ID:                 "task-3",
		Description:        "inspect repo",
		Goal:               "propose next step",
		ProtocolHint:       "cli-first",
		VerificationPolicy: "default",
		CreatedAt:          createdAt,
	})
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode marshaled task: %v", err)
	}

	if got := payload["Category"]; got != string(TaskCategoryGeneral) {
		t.Fatalf("Category field = %#v, want %q", got, TaskCategoryGeneral)
	}
	if got := payload["ProtocolHint"]; got != "cli-first" {
		t.Fatalf("ProtocolHint field = %#v, want %q", got, "cli-first")
	}
	if got := payload["VerificationPolicy"]; got != "default" {
		t.Fatalf("VerificationPolicy field = %#v, want %q", got, "default")
	}
}
