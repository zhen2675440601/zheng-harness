package orchestration

import (
	"reflect"
	"testing"
)

func TestSchedulerTopologicalSort(t *testing.T) {
	t.Parallel()

	scheduler := mustScheduler(t, TaskDecomposition{
		TaskID: "task-topological",
		Subtasks: []Subtask{
			{ID: "plan", Description: "plan", Status: SubtaskStatusPending},
			{ID: "fetch", Description: "fetch", Dependencies: []string{"plan"}, Status: SubtaskStatusPending},
			{ID: "build", Description: "build", Status: SubtaskStatusPending},
			{ID: "verify", Description: "verify", Status: SubtaskStatusPending},
		},
		DAG: []Dependency{
			{From: "fetch", To: "verify", Type: DependencyTypeDependsOn},
			{From: "build", To: "verify", Type: DependencyTypeSequential},
		},
	})

	ordered, err := scheduler.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() error = %v", err)
	}
	if got := subtaskIDs(ordered); !reflect.DeepEqual(got, []string{"plan", "fetch", "build", "verify"}) {
		t.Fatalf("TopologicalSort() ids = %v, want [plan fetch build verify]", got)
	}
}

func TestSchedulerReadySubtasks(t *testing.T) {
	t.Parallel()

	scheduler := mustScheduler(t, TaskDecomposition{
		TaskID: "task-ready",
		Subtasks: []Subtask{
			{ID: "plan", Description: "plan", Status: SubtaskStatusPending},
			{ID: "build", Description: "build", Status: SubtaskStatusPending},
			{ID: "verify", Description: "verify", Dependencies: []string{"plan"}, Status: SubtaskStatusPending},
		},
		DAG: []Dependency{{From: "build", To: "verify", Type: DependencyTypeDependsOn}},
	})

	firstBatch := scheduler.Schedule()
	if got := subtaskIDs(firstBatch); !reflect.DeepEqual(got, []string{"plan", "build"}) {
		t.Fatalf("Schedule() first batch = %v, want [plan build]", got)
	}
	if got := scheduler.Schedule(); got != nil {
		t.Fatalf("Schedule() second call before completion = %v, want nil", subtaskIDs(got))
	}

	if got := scheduler.MarkComplete("plan"); got != nil {
		t.Fatalf("MarkComplete(plan) = %v, want nil", subtaskIDs(got))
	}
	newlyReady := scheduler.MarkComplete("build")
	if got := subtaskIDs(newlyReady); !reflect.DeepEqual(got, []string{"verify"}) {
		t.Fatalf("MarkComplete(build) = %v, want [verify]", got)
	}
	if got := subtaskIDs(scheduler.Schedule()); !reflect.DeepEqual(got, []string{"verify"}) {
		t.Fatalf("Schedule() ready batch = %v, want [verify]", got)
	}
	if got := scheduler.MarkComplete("build"); got != nil {
		t.Fatalf("MarkComplete(build) repeated = %v, want nil", subtaskIDs(got))
	}
	if got := scheduler.Schedule(); got != nil {
		t.Fatalf("Schedule() after draining queue = %v, want nil", subtaskIDs(got))
	}
}

func TestSchedulerParallelExecution(t *testing.T) {
	t.Parallel()

	scheduler := mustScheduler(t, TaskDecomposition{
		TaskID: "task-parallel",
		Subtasks: []Subtask{
			{ID: "extract", Description: "extract", Status: SubtaskStatusPending},
			{ID: "transform", Description: "transform", Status: SubtaskStatusPending},
			{ID: "load", Description: "load", Status: SubtaskStatusPending},
		},
		DAG: []Dependency{
			{From: "extract", To: "transform", Type: DependencyTypeParallelWith},
			{From: "transform", To: "load", Type: DependencyTypeDependsOn},
		},
	})

	ready := scheduler.Schedule()
	if got := subtaskIDs(ready); !reflect.DeepEqual(got, []string{"extract", "transform"}) {
		t.Fatalf("Schedule() parallel roots = %v, want [extract transform]", got)
	}
	if got := scheduler.MarkComplete("extract"); got != nil {
		t.Fatalf("MarkComplete(extract) = %v, want nil", subtaskIDs(got))
	}
	newlyReady := scheduler.MarkComplete("transform")
	if got := subtaskIDs(newlyReady); !reflect.DeepEqual(got, []string{"load"}) {
		t.Fatalf("MarkComplete(transform) = %v, want [load]", got)
	}
	if got := subtaskIDs(scheduler.Schedule()); !reflect.DeepEqual(got, []string{"load"}) {
		t.Fatalf("Schedule() after parallel completion = %v, want [load]", got)
	}
}

func TestSchedulerDependencyWait(t *testing.T) {
	t.Parallel()

	scheduler := mustScheduler(t, TaskDecomposition{
		TaskID: "task-wait",
		Subtasks: []Subtask{
			{ID: "a", Description: "a", Status: SubtaskStatusPending},
			{ID: "b", Description: "b", Status: SubtaskStatusPending},
			{ID: "c", Description: "c", Status: SubtaskStatusPending},
			{ID: "final", Description: "final", Status: SubtaskStatusPending},
		},
		DAG: []Dependency{
			{From: "a", To: "final", Type: DependencyTypeDependsOn},
			{From: "b", To: "final", Type: DependencyTypeSequential},
			{From: "c", To: "final", Type: DependencyTypeDependsOn},
		},
	})

	if got := subtaskIDs(scheduler.Schedule()); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Schedule() roots = %v, want [a b c]", got)
	}
	for _, id := range []string{"a", "b"} {
		if got := scheduler.MarkComplete(id); got != nil {
			t.Fatalf("MarkComplete(%s) = %v, want nil", id, subtaskIDs(got))
		}
		if got := scheduler.Schedule(); got != nil {
			t.Fatalf("Schedule() while waiting after %s = %v, want nil", id, subtaskIDs(got))
		}
	}
	newlyReady := scheduler.MarkComplete("c")
	if got := subtaskIDs(newlyReady); !reflect.DeepEqual(got, []string{"final"}) {
		t.Fatalf("MarkComplete(c) = %v, want [final]", got)
	}
	if got := subtaskIDs(scheduler.Schedule()); !reflect.DeepEqual(got, []string{"final"}) {
		t.Fatalf("Schedule() after all dependencies = %v, want [final]", got)
	}
}

func mustScheduler(t *testing.T, decomposition TaskDecomposition) *DAGScheduler {
	t.Helper()
	scheduler, err := NewDAGScheduler(decomposition)
	if err != nil {
		t.Fatalf("NewDAGScheduler() error = %v", err)
	}
	return scheduler
}

func subtaskIDs(subtasks []Subtask) []string {
	if len(subtasks) == 0 {
		return nil
	}
	ids := make([]string, 0, len(subtasks))
	for _, subtask := range subtasks {
		ids = append(ids, subtask.ID)
	}
	return ids
}
