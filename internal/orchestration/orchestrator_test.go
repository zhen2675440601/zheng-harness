package orchestration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrchestratorBoundedConcurrency(t *testing.T) {
	t.Parallel()

	decomposition := TaskDecomposition{
		TaskID: "task-1",
		Subtasks: []Subtask{
			{ID: "a", Description: "a", Status: SubtaskStatusPending},
			{ID: "b", Description: "b", Status: SubtaskStatusPending},
			{ID: "c", Description: "c", Status: SubtaskStatusPending},
			{ID: "d", Description: "d", Status: SubtaskStatusPending},
		},
	}

	var current atomic.Int32
	var peak atomic.Int32
	release := make(chan struct{})

	orch := Orchestrator{
		MaxWorkers: 2,
		WorkerFactory: func(subtask Subtask) Worker {
			return NewWorker(func(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
				now := current.Add(1)
				defer current.Add(-1)
				for {
					seen := peak.Load()
					if now <= seen || peak.CompareAndSwap(seen, now) {
						break
					}
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-release:
					return nil
				}
			})
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := orch.Start(ctx); err != nil {
		 t.Fatalf("Start() error = %v", err)
	}
	defer orch.Stop()

	if err := orch.SubmitTask(ctx, decomposition); err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}

	deadline := time.After(300 * time.Millisecond)
	for peak.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("peak concurrency = %d, want 2", peak.Load())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	if got := peak.Load(); got > 2 {
		t.Fatalf("peak concurrency = %d, want <= 2", got)
	}

	for range decomposition.Subtasks {
		release <- struct{}{}
	}
	orch.Stop()
	if err := orch.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	results := collectResults(orch.ResultChannel)
	if len(results) != len(decomposition.Subtasks) {
		t.Fatalf("result count = %d, want %d", len(results), len(decomposition.Subtasks))
	}
	for _, result := range results {
		if result.Status != SubtaskStatusCompleted {
			t.Fatalf("result for %s status = %q, want %q", result.SubtaskID, result.Status, SubtaskStatusCompleted)
		}
	}
	if got := peak.Load(); got != 2 {
		t.Fatalf("peak concurrency = %d, want 2", got)
	}
	if len(orch.Workers) != 0 {
		t.Fatalf("Workers still registered = %d, want 0", len(orch.Workers))
	}
}

func TestOrchestratorCancelPropagation(t *testing.T) {
	t.Parallel()

	decomposition := TaskDecomposition{
		TaskID: "task-cancel",
		Subtasks: []Subtask{{ID: "a", Description: "a", Status: SubtaskStatusPending}},
	}

	workerStarted := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	orch := Orchestrator{
		MaxWorkers: 1,
		WorkerFactory: func(subtask Subtask) Worker {
			return NewWorker(func(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
				close(workerStarted)
				<-ctx.Done()
				return ctx.Err()
			})
		},
	}
	if err := orch.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := orch.SubmitTask(context.Background(), decomposition); err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}

	select {
	case <-workerStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("worker did not start")
	}

	cancel()
	if err := orch.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	results := collectResults(orch.ResultChannel)
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if !errors.Is(results[0].Err, context.Canceled) {
		t.Fatalf("result error = %v, want context canceled", results[0].Err)
	}
	if results[0].Status != SubtaskStatusFailed {
		t.Fatalf("result status = %q, want %q", results[0].Status, SubtaskStatusFailed)
	}
	if len(orch.Workers) != 0 {
		t.Fatalf("Workers still registered = %d, want 0", len(orch.Workers))
	}
}

func TestOrchestratorWorkerFailure(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	decomposition := TaskDecomposition{
		TaskID: "task-fail",
		Subtasks: []Subtask{
			{ID: "a", Description: "a", Status: SubtaskStatusPending},
			{ID: "b", Description: "b", Status: SubtaskStatusPending},
		},
	}

	var executed sync.Map
	orch := Orchestrator{
		MaxWorkers: 2,
		WorkerFactory: func(subtask Subtask) Worker {
			if subtask.ID == "a" {
				return NewWorker(func(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
					executed.Store(subtask.ID, true)
					return boom
				})
			}
			return NewWorker(func(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
				executed.Store(subtask.ID, true)
				return nil
			})
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := orch.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer orch.Stop()

	if err := orch.SubmitTask(ctx, decomposition); err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}

	orch.Stop()
	err := orch.Wait()
	if !errors.Is(err, boom) {
		t.Fatalf("Wait() error = %v, want %v", err, boom)
	}

	results := collectResults(orch.ResultChannel)
	if len(results) == 0 {
		t.Fatal("result count = 0, want >= 1")
	}
	if len(orch.Workers) != 0 {
		t.Fatalf("Workers still registered = %d, want 0", len(orch.Workers))
	}

	seenFailure := false
	seenFailureStatus := false
	for _, result := range results {
		if errors.Is(result.Err, boom) {
			seenFailure = true
			if result.Status == SubtaskStatusFailed {
				seenFailureStatus = true
			}
		}
	}
	if !seenFailure {
		t.Fatalf("results = %#v, want failure result", results)
	}
	if !seenFailureStatus {
		t.Fatalf("results = %#v, want failed status for boom result", results)
	}
	_, ranA := executed.Load("a")
	if !ranA {
		t.Fatal("failing worker did not execute")
	}
}

func TestOrchestratorAllSucceed(t *testing.T) {
	t.Parallel()

	decomposition := TaskDecomposition{
		TaskID: "task-success",
		Subtasks: []Subtask{
			{ID: "plan", Description: "plan", Status: SubtaskStatusPending},
			{ID: "execute", Description: "execute", Dependencies: []string{"plan"}, Status: SubtaskStatusPending},
			{ID: "verify", Description: "verify", Dependencies: []string{"execute"}, Status: SubtaskStatusPending},
		},
	}

	var mu sync.Mutex
	sequence := make([]string, 0, 9)
	orch := Orchestrator{
		WorkerFactory: func(subtask Subtask) Worker {
			return &recordingWorker{
				subtaskID: subtask.ID,
				sequence:  &sequence,
				mu:        &mu,
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := orch.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer orch.Stop()

	if err := orch.SubmitTask(ctx, decomposition); err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}
	orch.Stop()
	if err := orch.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	results := collectResults(orch.ResultChannel)
	if len(results) != 3 {
		t.Fatalf("result count = %d, want 3", len(results))
	}
	ids := make([]string, 0, len(results))
	for _, result := range results {
		if result.Status != SubtaskStatusCompleted {
			t.Fatalf("result for %s status = %q, want %q", result.SubtaskID, result.Status, SubtaskStatusCompleted)
		}
		if result.Err != nil {
			t.Fatalf("result for %s error = %v, want nil", result.SubtaskID, result.Err)
		}
		if !result.WorkerTerminated {
			t.Fatalf("result for %s WorkerTerminated = false, want true", result.SubtaskID)
		}
		ids = append(ids, result.SubtaskID)
	}
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{"execute", "plan", "verify"}) {
		t.Fatalf("result ids = %v, want [execute plan verify]", ids)
	}

	mu.Lock()
	gotSequence := append([]string(nil), sequence...)
	mu.Unlock()
	if !isOrderedSubsequence(gotSequence,
		"plan:plan", "plan:execute", "plan:verify",
		"execute:plan", "execute:execute", "execute:verify",
		"verify:plan", "verify:execute", "verify:verify",
	) {
		t.Fatalf("worker lifecycle sequence = %v", gotSequence)
	}
	if len(orch.Workers) != 0 {
		t.Fatalf("Workers still registered = %d, want 0", len(orch.Workers))
	}
	if orch.MaxWorkers != defaultMaxWorkers {
		t.Fatalf("MaxWorkers = %d, want %d", orch.MaxWorkers, defaultMaxWorkers)
	}
}

type recordingWorker struct {
	subtaskID string
	sequence  *[]string
	mu        *sync.Mutex
}

func (w *recordingWorker) Plan(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
	return w.record(ctx, "plan")
}

func (w *recordingWorker) Execute(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
	return w.record(ctx, "execute")
}

func (w *recordingWorker) Verify(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
	return w.record(ctx, "verify")
}

func (w *recordingWorker) record(ctx context.Context, stage string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	*w.sequence = append(*w.sequence, fmt.Sprintf("%s:%s", w.subtaskID, stage))
	return nil
}

func collectResults(results <-chan WorkerResult) []WorkerResult {
	collected := make([]WorkerResult, 0)
	for result := range results {
		collected = append(collected, result)
	}
	return collected
}

func isOrderedSubsequence(sequence []string, want ...string) bool {
	if len(sequence) < len(want) {
		return false
	}
	index := 0
	for _, item := range sequence {
		if item == want[index] {
			index++
			if index == len(want) {
				return true
			}
		}
	}
	return false
}
