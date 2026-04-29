package orchestration

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"zheng-harness/internal/domain"

	"golang.org/x/sync/errgroup"
)

const defaultMaxWorkers = 4

// Worker executes a scoped plan-execute-verify loop for one subtask.
type Worker interface {
	Plan(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error
	Execute(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error
	Verify(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error
}

// WorkerFactory creates a worker for a subtask.
type WorkerFactory func(subtask Subtask) Worker

// WorkerFunc adapts a simple execute callback into a Worker.
type WorkerFunc func(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error

type workerFuncAdapter struct {
	execute WorkerFunc
}

func (w workerFuncAdapter) Plan(context.Context, Subtask, TaskDecomposition) error   { return nil }
func (w workerFuncAdapter) Verify(context.Context, Subtask, TaskDecomposition) error { return nil }
func (w workerFuncAdapter) Execute(ctx context.Context, subtask Subtask, decomposition TaskDecomposition) error {
	if w.execute == nil {
		return nil
	}
	return w.execute(ctx, subtask, decomposition)
}

// NewWorker wraps a simple execute callback into a lifecycle-compatible Worker.
func NewWorker(fn WorkerFunc) Worker {
	return workerFuncAdapter{execute: fn}
}

// WorkerResult captures the final report from one worker execution.
type WorkerResult struct {
	TaskID            string
	SubtaskID         string
	Status            SubtaskStatus
	Output            string
	SessionStatus     domain.SessionStatus
	VerificationPassed bool
	Err               error
	WorkerTerminated bool
}

type workerReporter interface {
	Report() error
	CurrentResult() WorkerResult
}

type terminableWorker interface {
	Terminate()
}

// Orchestrator coordinates bounded concurrent worker execution for decompositions.
type Orchestrator struct {
	MaxWorkers    int
	Workers       map[string]Worker
	TaskChannel   chan TaskDecomposition
	ResultChannel chan WorkerResult
	WorkerFactory WorkerFactory

	errgroup *errgroup.Group
	ctx      context.Context
	cancel   context.CancelFunc

	mu      sync.Mutex
	started bool
	closed  bool
	waitMu  sync.Mutex
}

// Cancel propagates cancellation to the processing loop and active workers.
func (o *Orchestrator) Cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancel != nil {
		o.cancel()
	}
}

// Start initializes channels, errgroup state, and the processing loop.
func (o *Orchestrator) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("orchestrator context is required")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.started {
		return errors.New("orchestrator already started")
	}
	if o.WorkerFactory == nil {
		return errors.New("orchestrator worker factory is required")
	}
	if o.MaxWorkers <= 0 {
		o.MaxWorkers = defaultMaxWorkers
	}
	if o.Workers == nil {
		o.Workers = make(map[string]Worker)
	}
	if o.TaskChannel == nil {
		o.TaskChannel = make(chan TaskDecomposition, o.MaxWorkers)
	}
	if o.ResultChannel == nil {
		o.ResultChannel = make(chan WorkerResult, 1024)
	}

	o.ctx, o.cancel = context.WithCancel(ctx)
	o.errgroup, o.ctx = errgroup.WithContext(o.ctx)
	o.errgroup.Go(func() error {
		defer o.closeResultChannel()
		for {
			select {
			case <-o.ctx.Done():
				return o.ctx.Err()
			case decomposition, ok := <-o.TaskChannel:
				if !ok {
					return nil
				}
				if err := o.executeDecomposition(o.ctx, decomposition); err != nil {
					return err
				}
			}
		}
	})
	o.started = true
	return nil
}

// SubmitTask queues a decomposition for execution.
func (o *Orchestrator) SubmitTask(ctx context.Context, decomposition TaskDecomposition) error {
	o.mu.Lock()
	started := o.started
	taskCh := o.TaskChannel
	runCtx := o.ctx
	o.mu.Unlock()
	if !started {
		return errors.New("orchestrator not started")
	}
	if err := decomposition.Validate(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-runCtx.Done():
		return runCtx.Err()
	case taskCh <- decomposition:
		return nil
	}
}

// Stop gracefully closes the task queue.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.TaskChannel != nil && !o.closed {
		close(o.TaskChannel)
		o.closed = true
	}
}

// Wait blocks until the processing loop exits.
func (o *Orchestrator) Wait() error {
	o.waitMu.Lock()
	g := o.errgroup
	o.waitMu.Unlock()
	if g == nil {
		return nil
	}
	err := g.Wait()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (o *Orchestrator) executeDecomposition(ctx context.Context, decomposition TaskDecomposition) error {
	if len(decomposition.Subtasks) == 0 {
		return nil
	}

	subtasks := make(map[string]Subtask, len(decomposition.Subtasks))
	remainingDeps := make(map[string]int, len(decomposition.Subtasks))
	dependents := make(map[string][]string, len(decomposition.Subtasks))
	for _, subtask := range decomposition.Subtasks {
		subtasks[subtask.ID] = subtask
		remainingDeps[subtask.ID] = len(subtask.Dependencies)
		for _, dependencyID := range subtask.Dependencies {
			dependents[dependencyID] = append(dependents[dependencyID], subtask.ID)
		}
	}
	for _, edge := range decomposition.DAG {
		if edge.Type == DependencyTypeParallelWith {
			continue
		}
		remainingDeps[edge.To]++
		dependents[edge.From] = append(dependents[edge.From], edge.To)
	}

	ready := make([]string, 0, len(decomposition.Subtasks))
	for _, subtask := range decomposition.Subtasks {
		if remainingDeps[subtask.ID] == 0 {
			ready = append(ready, subtask.ID)
		}
	}
	if len(ready) == 0 {
		return fmt.Errorf("task decomposition %q has no runnable root subtasks", decomposition.TaskID)
	}

	group, workerCtx := errgroup.WithContext(ctx)
	semaphore := make(chan struct{}, o.MaxWorkers)
	results := make(chan WorkerResult, len(decomposition.Subtasks))
	started := make(map[string]bool, len(decomposition.Subtasks))
	completed := 0

	launch := func(subtaskID string) {
		if started[subtaskID] {
			return
		}
		started[subtaskID] = true
		subtask := subtasks[subtaskID]
		group.Go(func() error {
			select {
			case semaphore <- struct{}{}:
			case <-workerCtx.Done():
				return workerCtx.Err()
			}
			defer func() { <-semaphore }()

			result := o.runWorker(workerCtx, decomposition, subtask)
			results <- result
			return result.Err
		})
	}

	for _, subtaskID := range ready {
		launch(subtaskID)
	}

	for completed < len(decomposition.Subtasks) {
		select {
		case <-workerCtx.Done():
			err := group.Wait()
			if err == nil {
				return workerCtx.Err()
			}
			return err
		case result := <-results:
			completed++
			if result.Err != nil {
				return group.Wait()
			}
			for _, dependentID := range dependents[result.SubtaskID] {
				remainingDeps[dependentID]--
				if remainingDeps[dependentID] == 0 {
					launch(dependentID)
				}
			}
		}
	}

	return group.Wait()
}

func (o *Orchestrator) runWorker(ctx context.Context, decomposition TaskDecomposition, subtask Subtask) WorkerResult {
	worker := o.WorkerFactory(subtask)
	if worker == nil {
		result := WorkerResult{TaskID: decomposition.TaskID, SubtaskID: subtask.ID, Status: SubtaskStatusFailed, Err: fmt.Errorf("worker factory returned nil for subtask %q", subtask.ID), WorkerTerminated: true}
		o.publishWorkerResult(ctx, worker, result)
		return result
	}

	o.registerWorker(subtask.ID, worker)
	defer o.unregisterWorker(subtask.ID)
	defer o.terminateWorker(worker)

	result := WorkerResult{TaskID: decomposition.TaskID, SubtaskID: subtask.ID, Status: SubtaskStatusCompleted}
	if err := worker.Plan(ctx, subtask, decomposition); err != nil {
		result.Status = SubtaskStatusFailed
		result.Err = err
		result.WorkerTerminated = true
		result = mergeReportedWorkerResult(worker, result)
		o.publishWorkerResult(ctx, worker, result)
		return result
	}
	if err := worker.Execute(ctx, subtask, decomposition); err != nil {
		result.Status = SubtaskStatusFailed
		result.Err = err
		result.WorkerTerminated = true
		result = mergeReportedWorkerResult(worker, result)
		o.publishWorkerResult(ctx, worker, result)
		return result
	}
	if err := worker.Verify(ctx, subtask, decomposition); err != nil {
		result.Status = SubtaskStatusFailed
		result.Err = err
		result.WorkerTerminated = true
		result = mergeReportedWorkerResult(worker, result)
		o.publishWorkerResult(ctx, worker, result)
		return result
	}
	result.WorkerTerminated = true
	result = mergeReportedWorkerResult(worker, result)
	o.publishWorkerResult(ctx, worker, result)
	return result
}

func mergeReportedWorkerResult(worker Worker, fallback WorkerResult) WorkerResult {
	reporter, ok := worker.(workerReporter)
	if !ok {
		return fallback
	}
	reported := reporter.CurrentResult()
	if reported.TaskID == "" {
		reported.TaskID = fallback.TaskID
	}
	if reported.SubtaskID == "" {
		reported.SubtaskID = fallback.SubtaskID
	}
	if reported.Status == "" {
		reported.Status = fallback.Status
	}
	if reported.Output == "" {
		reported.Output = fallback.Output
	}
	if reported.SessionStatus == "" {
		reported.SessionStatus = fallback.SessionStatus
	}
	if reported.Err == nil {
		reported.Err = fallback.Err
	}
	if !reported.WorkerTerminated {
		reported.WorkerTerminated = fallback.WorkerTerminated
	}
	if !reported.VerificationPassed {
		reported.VerificationPassed = fallback.VerificationPassed
	}
	return reported
}

func (o *Orchestrator) publishWorkerResult(ctx context.Context, worker Worker, result WorkerResult) {
	reporter, ok := worker.(workerReporter)
	if ok {
		if err := reporter.Report(); err == nil {
			return
		}
	}
	o.reportResult(ctx, result)
}

func (o *Orchestrator) terminateWorker(worker Worker) {
	terminable, ok := worker.(terminableWorker)
	if !ok {
		return
	}
	terminable.Terminate()
}

func (o *Orchestrator) reportResult(_ context.Context, result WorkerResult) {
	if o.ResultChannel == nil {
		return
	}
	o.ResultChannel <- result
}

func (o *Orchestrator) registerWorker(id string, worker Worker) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Workers[id] = worker
}

func (o *Orchestrator) unregisterWorker(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.Workers, id)
}

func (o *Orchestrator) closeResultChannel() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ResultChannel != nil {
		close(o.ResultChannel)
	}
}
