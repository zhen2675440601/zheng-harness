package orchestration

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSubtaskNormalizeDefaultsUnknownStatusToPending(t *testing.T) {
	t.Parallel()

	subtask := Subtask{ID: "subtask-1", Description: "inspect repo", Status: SubtaskStatus("unknown")}.Normalize()
	if subtask.Status != SubtaskStatusPending {
		t.Fatalf("Status = %q, want %q", subtask.Status, SubtaskStatusPending)
	}
}

func TestSubtaskUnmarshalJSONDefaultsMissingStatusToPending(t *testing.T) {
	t.Parallel()

	var subtask Subtask
	if err := json.Unmarshal([]byte(`{"id":"subtask-1","description":"inspect repo"}`), &subtask); err != nil {
		t.Fatalf("unmarshal subtask: %v", err)
	}

	if subtask.Status != SubtaskStatusPending {
		t.Fatalf("Status = %q, want %q", subtask.Status, SubtaskStatusPending)
	}
}

func TestSubtaskMarshalJSONEmitsNormalizedStatus(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(Subtask{ID: "subtask-1", Description: "inspect repo", Status: SubtaskStatus("bad")})
	if err != nil {
		t.Fatalf("marshal subtask: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode marshaled subtask: %v", err)
	}

	if got := payload["status"]; got != string(SubtaskStatusPending) {
		t.Fatalf("status = %#v, want %q", got, SubtaskStatusPending)
	}
}

func TestSubtaskValidateRejectsSelfDependency(t *testing.T) {
	t.Parallel()

	err := Subtask{
		ID:           "subtask-1",
		Description:  "inspect repo",
		Dependencies: []string{"subtask-1"},
		Status:       SubtaskStatusPending,
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want self dependency error")
	}
	if !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Fatalf("Validate() error = %q, want self dependency message", err)
	}
}

func TestSubtaskValidateRejectsDuplicateDependencies(t *testing.T) {
	t.Parallel()

	err := Subtask{
		ID:           "subtask-1",
		Description:  "inspect repo",
		Dependencies: []string{"subtask-2", "subtask-2"},
		Status:       SubtaskStatusPending,
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate dependency error")
	}
	if !strings.Contains(err.Error(), `duplicate dependency "subtask-2"`) {
		t.Fatalf("Validate() error = %q, want duplicate dependency message", err)
	}
}

func TestSubtaskValidateRejectsMissingDescription(t *testing.T) {
	t.Parallel()

	err := Subtask{ID: "subtask-1", Status: SubtaskStatusPending}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want description error")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("Validate() error = %q, want description error", err)
	}
}

func TestSubtaskValidateRejectsUnsupportedStatus(t *testing.T) {
	t.Parallel()

	err := Subtask{ID: "subtask-1", Description: "inspect repo", Status: SubtaskStatus("paused")}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want status error")
	}
	if !strings.Contains(err.Error(), `unsupported status "paused"`) {
		t.Fatalf("Validate() error = %q, want unsupported status error", err)
	}
}

func TestSubtaskValidateAcceptsKnownDependencies(t *testing.T) {
	t.Parallel()

	err := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "inspect repo", Status: SubtaskStatusPending},
			{ID: "subtask-2", Description: "write summary", Dependencies: []string{"subtask-1"}, Status: SubtaskStatusPending},
		},
	}.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestSubtaskSerializationRoundTrip(t *testing.T) {
	t.Parallel()

	want := Subtask{
		ID:             "subtask-1",
		Description:    "inspect repo",
		Input:          "repository path",
		ExpectedOutput: "summary",
		Dependencies:   []string{"subtask-0"},
		Status:         SubtaskStatusRunning,
	}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal subtask: %v", err)
	}

	var got Subtask
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal subtask: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip subtask = %#v, want %#v", got, want)
	}
}

func TestTaskDecompositionValidateRejectsDuplicateSubtaskIDs(t *testing.T) {
	t.Parallel()

	err := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "inspect", Status: SubtaskStatusPending},
			{ID: "subtask-1", Description: "summarize", Status: SubtaskStatusPending},
		},
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate subtask id error")
	}
	if !strings.Contains(err.Error(), `duplicate subtask id "subtask-1"`) {
		t.Fatalf("Validate() error = %q, want duplicate id message", err)
	}
}

func TestTaskDecompositionValidateRejectsUnknownDependencies(t *testing.T) {
	t.Parallel()

	err := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "inspect", Dependencies: []string{"missing"}, Status: SubtaskStatusPending},
		},
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want unknown dependency error")
	}
	if !strings.Contains(err.Error(), `references unknown dependency "missing"`) {
		t.Fatalf("Validate() error = %q, want unknown dependency message", err)
	}
}

func TestTaskDecompositionValidateRejectsUnknownEdgeReferences(t *testing.T) {
	t.Parallel()

	err := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "inspect", Status: SubtaskStatusPending},
		},
		DAG: []Dependency{{From: "subtask-1", To: "missing", Type: DependencyTypeSequential}},
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want unknown edge reference error")
	}
	if !strings.Contains(err.Error(), `dependency references unknown subtask "missing"`) {
		t.Fatalf("Validate() error = %q, want unknown edge reference message", err)
	}
}

func TestTaskDecompositionValidateRejectsCircularDependenciesFromSubtasks(t *testing.T) {
	t.Parallel()

	err := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "first", Dependencies: []string{"subtask-2"}, Status: SubtaskStatusPending},
			{ID: "subtask-2", Description: "second", Dependencies: []string{"subtask-1"}, Status: SubtaskStatusPending},
		},
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("Validate() error = %q, want circular dependency message", err)
	}
}

func TestTaskDecompositionValidateRejectsCircularDependenciesFromEdges(t *testing.T) {
	t.Parallel()

	err := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "first", Status: SubtaskStatusPending},
			{ID: "subtask-2", Description: "second", Status: SubtaskStatusPending},
		},
		DAG: []Dependency{
			{From: "subtask-1", To: "subtask-2", Type: DependencyTypeSequential},
			{From: "subtask-2", To: "subtask-1", Type: DependencyTypeDependsOn},
		},
	}.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("Validate() error = %q, want circular dependency message", err)
	}
}

func TestTaskDecompositionValidateAllowsParallelEdges(t *testing.T) {
	t.Parallel()

	err := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "first", Status: SubtaskStatusPending},
			{ID: "subtask-2", Description: "second", Status: SubtaskStatusPending},
		},
		DAG: []Dependency{{From: "subtask-1", To: "subtask-2", Type: DependencyTypeParallelWith}},
	}.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestTaskDecompositionSerializationRoundTrip(t *testing.T) {
	t.Parallel()

	want := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "subtask-1", Description: "inspect repo", Input: "repo path", ExpectedOutput: "summary", Status: SubtaskStatusCompleted},
			{ID: "subtask-2", Description: "write plan", Dependencies: []string{"subtask-1"}, Status: SubtaskStatusPending},
		},
		DAG: []Dependency{
			{From: "subtask-1", To: "subtask-2", Type: DependencyTypeSequential},
			{From: "subtask-1", To: "subtask-2", Type: DependencyTypeParallelWith},
		},
		Metadata: map[string]string{"source": "planner", "strategy": "dag"},
	}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal decomposition: %v", err)
	}

	var got TaskDecomposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal decomposition: %v", err)
	}

	if got.TaskID != want.TaskID {
		t.Fatalf("TaskID = %q, want %q", got.TaskID, want.TaskID)
	}
	if len(got.Subtasks) != len(want.Subtasks) {
		t.Fatalf("len(Subtasks) = %d, want %d", len(got.Subtasks), len(want.Subtasks))
	}
	for i := range want.Subtasks {
		if !reflect.DeepEqual(got.Subtasks[i], want.Subtasks[i]) {
			t.Fatalf("Subtasks[%d] = %#v, want %#v", i, got.Subtasks[i], want.Subtasks[i])
		}
	}
	if len(got.DAG) != len(want.DAG) {
		t.Fatalf("len(DAG) = %d, want %d", len(got.DAG), len(want.DAG))
	}
	for i := range want.DAG {
		if got.DAG[i] != want.DAG[i] {
			t.Fatalf("DAG[%d] = %#v, want %#v", i, got.DAG[i], want.DAG[i])
		}
	}
	if len(got.Metadata) != len(want.Metadata) {
		t.Fatalf("len(Metadata) = %d, want %d", len(got.Metadata), len(want.Metadata))
	}
	for key, wantValue := range want.Metadata {
		if got.Metadata[key] != wantValue {
			t.Fatalf("Metadata[%q] = %q, want %q", key, got.Metadata[key], wantValue)
		}
	}
}
