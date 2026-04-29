package orchestration

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"zheng-harness/internal/domain"
)

const defaultAggregationTimeout = 30 * time.Second

// AggregationStrategy controls how subtask results roll up.
type AggregationStrategy string

const (
	AggregationStrategyAllSucceed AggregationStrategy = "all_succeed"
	AggregationStrategyBestEffort AggregationStrategy = "best_effort"
)

// Normalize returns a supported aggregation strategy.
func (s AggregationStrategy) Normalize() AggregationStrategy {
	switch s {
	case AggregationStrategyBestEffort:
		return s
	default:
		return AggregationStrategyAllSucceed
	}
}

// AggregationStatus summarizes the overall aggregation outcome.
type AggregationStatus string

const (
	AggregationStatusSucceeded      AggregationStatus = "succeeded"
	AggregationStatusPartialSuccess AggregationStatus = "partial_success"
	AggregationStatusFailed         AggregationStatus = "failed"
	AggregationStatusTimedOut       AggregationStatus = "timed_out"
)

// AggregatedSubtaskResult stores one normalized subtask outcome.
type AggregatedSubtaskResult struct {
	SubtaskID          string
	Status             SubtaskStatus
	Output             string
	Error              error
	VerificationStatus domain.VerificationStatus
	Duration           time.Duration
}

// AggregationResult captures individual subtask outcomes and overall status.
type AggregationResult struct {
	Strategy  AggregationStrategy
	Status    AggregationStatus
	Results   map[string]AggregatedSubtaskResult
	Completed int
	Failed    int
	TimedOut  bool
	Summary   string
}

// Aggregator aggregates subtask results under one strategy.
type Aggregator struct {
	Strategy AggregationStrategy
	Results  map[string]AggregatedSubtaskResult
	Timeout  time.Duration

	mu sync.Mutex
}

// Aggregate folds subtask results into one overall aggregation result.
func (a *Aggregator) Aggregate(results []TaskResult) (AggregationResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ensureDefaultsLocked()

	aggregated := AggregationResult{
		Strategy: a.Strategy.Normalize(),
		Results:  make(map[string]AggregatedSubtaskResult, len(results)),
	}

	failureIDs := make([]string, 0)
	for _, result := range results {
		normalized := normalizeTaskResult(result)
		a.Results[normalized.SubtaskID] = normalized
		aggregated.Results[normalized.SubtaskID] = normalized
		aggregated.Completed++
		if normalized.Status == SubtaskStatusFailed {
			aggregated.Failed++
			failureIDs = append(failureIDs, normalized.SubtaskID)
		}
	}

	sort.Strings(failureIDs)
	switch aggregated.Strategy {
	case AggregationStrategyBestEffort:
		if aggregated.Failed > 0 {
			aggregated.Status = AggregationStatusPartialSuccess
			aggregated.Summary = fmt.Sprintf("%d of %d subtasks failed: %s", aggregated.Failed, aggregated.Completed, strings.Join(failureIDs, ", "))
			return aggregated, nil
		}
		aggregated.Status = AggregationStatusSucceeded
		aggregated.Summary = fmt.Sprintf("all %d subtasks succeeded", aggregated.Completed)
		return aggregated, nil
	default:
		if aggregated.Failed > 0 {
			aggregated.Status = AggregationStatusFailed
			aggregated.Summary = fmt.Sprintf("%d of %d subtasks failed: %s", aggregated.Failed, aggregated.Completed, strings.Join(failureIDs, ", "))
			return aggregated, fmt.Errorf("all-succeed aggregation failed: %s", aggregated.Summary)
		}
		aggregated.Status = AggregationStatusSucceeded
		aggregated.Summary = fmt.Sprintf("all %d subtasks succeeded", aggregated.Completed)
		return aggregated, nil
	}
}

// Collect gathers an expected number of task results with timeout protection.
func (a *Aggregator) Collect(ctx context.Context, resultCh <-chan TaskResult, expected int) (AggregationResult, error) {
	if expected < 0 {
		expected = 0
	}
	if ctx == nil {
		ctx = context.Background()
	}

	timeout := a.timeoutOrDefault()
	collectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	collected := make([]TaskResult, 0, expected)
	for len(collected) < expected {
		select {
		case <-ctx.Done():
			aggregated, _ := a.Aggregate(collected)
			aggregated.Status = AggregationStatusTimedOut
			aggregated.TimedOut = true
			aggregated.Summary = timeoutSummary(len(collected), expected)
			return aggregated, ctx.Err()
		case <-collectCtx.Done():
			aggregated, _ := a.Aggregate(collected)
			aggregated.Status = AggregationStatusTimedOut
			aggregated.TimedOut = true
			aggregated.Summary = timeoutSummary(len(collected), expected)
			return aggregated, fmt.Errorf("aggregation timed out after %s: %w", timeout, collectCtx.Err())
		case result, ok := <-resultCh:
			if !ok {
				aggregated, err := a.Aggregate(collected)
				if len(collected) < expected {
					aggregated.Status = AggregationStatusTimedOut
					aggregated.TimedOut = true
					aggregated.Summary = timeoutSummary(len(collected), expected)
					return aggregated, fmt.Errorf("result channel closed after %d of %d results", len(collected), expected)
				}
				return aggregated, err
			}
			collected = append(collected, result)
		}
	}

	return a.Aggregate(collected)
}

func (a *Aggregator) timeoutOrDefault() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ensureDefaultsLocked()
	return a.Timeout
}

func (a *Aggregator) ensureDefaultsLocked() {
	if a.Strategy == "" {
		a.Strategy = AggregationStrategyAllSucceed
	} else {
		a.Strategy = a.Strategy.Normalize()
	}
	if a.Results == nil {
		a.Results = make(map[string]AggregatedSubtaskResult)
	}
	if a.Timeout <= 0 {
		a.Timeout = defaultAggregationTimeout
	}
}

func normalizeTaskResult(result TaskResult) AggregatedSubtaskResult {
	normalized := AggregatedSubtaskResult{
		SubtaskID:          result.SubtaskID,
		Output:             result.Output,
		Error:              result.Error,
		VerificationStatus: result.VerificationStatus,
		Duration:           result.Duration,
		Status:             SubtaskStatusCompleted,
	}
	if result.Error != nil || errors.Is(result.Error, context.Canceled) || result.VerificationStatus == domain.VerificationStatusFailed {
		normalized.Status = SubtaskStatusFailed
	}
	return normalized
}

func timeoutSummary(collected, expected int) string {
	return fmt.Sprintf("timed out waiting for results: collected %d of %d", collected, expected)
}
