package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestAggregationAllSucceed(t *testing.T) {
	t.Parallel()

	aggregator := &Aggregator{Strategy: AggregationStrategyAllSucceed}
	boom := errors.New("boom")

	result, err := aggregator.Aggregate([]TaskResult{
		{SubtaskID: "a", Output: "ok", VerificationStatus: domain.VerificationStatusPassed, Duration: 10 * time.Millisecond},
		{SubtaskID: "b", Error: boom, VerificationStatus: domain.VerificationStatusFailed, Duration: 20 * time.Millisecond},
	})
	if err == nil {
		t.Fatal("Aggregate() error = nil, want failure")
	}
	if result.Status != AggregationStatusFailed {
		t.Fatalf("Aggregate() status = %q, want %q", result.Status, AggregationStatusFailed)
	}
	if result.Failed != 1 {
		t.Fatalf("Aggregate() failed = %d, want 1", result.Failed)
	}
	if result.Completed != 2 {
		t.Fatalf("Aggregate() completed = %d, want 2", result.Completed)
	}
	if !strings.Contains(result.Summary, "b") {
		t.Fatalf("Aggregate() summary = %q, want failed subtask id", result.Summary)
	}
	if got := result.Results["a"].Status; got != SubtaskStatusCompleted {
		t.Fatalf("result a status = %q, want %q", got, SubtaskStatusCompleted)
	}
	if got := result.Results["b"].Status; got != SubtaskStatusFailed {
		t.Fatalf("result b status = %q, want %q", got, SubtaskStatusFailed)
	}
	if !errors.Is(result.Results["b"].Error, boom) {
		t.Fatalf("result b err = %v, want %v", result.Results["b"].Error, boom)
	}
	if got := aggregator.Results["b"].Status; got != SubtaskStatusFailed {
		t.Fatalf("aggregator stored result status = %q, want %q", got, SubtaskStatusFailed)
	}
}

func TestAggregationBestEffort(t *testing.T) {
	t.Parallel()

	aggregator := &Aggregator{Strategy: AggregationStrategyBestEffort}
	boom := errors.New("boom")

	result, err := aggregator.Aggregate([]TaskResult{
		{SubtaskID: "a", Output: "ok", VerificationStatus: domain.VerificationStatusPassed},
		{SubtaskID: "b", Error: boom, VerificationStatus: domain.VerificationStatusFailed},
		{SubtaskID: "c", Output: "still useful", VerificationStatus: domain.VerificationStatusPassed},
	})
	if err != nil {
		t.Fatalf("Aggregate() error = %v, want nil", err)
	}
	if result.Status != AggregationStatusPartialSuccess {
		t.Fatalf("Aggregate() status = %q, want %q", result.Status, AggregationStatusPartialSuccess)
	}
	if result.Failed != 1 {
		t.Fatalf("Aggregate() failed = %d, want 1", result.Failed)
	}
	if result.Completed != 3 {
		t.Fatalf("Aggregate() completed = %d, want 3", result.Completed)
	}
	if len(result.Results) != 3 {
		t.Fatalf("Aggregate() result count = %d, want 3", len(result.Results))
	}
	if !strings.Contains(result.Summary, "1 of 3 subtasks failed") {
		t.Fatalf("Aggregate() summary = %q, want partial failure summary", result.Summary)
	}
	if got := result.Results["c"].Output; got != "still useful" {
		t.Fatalf("result c output = %q, want still useful", got)
	}
}

func TestAggregationTimeout(t *testing.T) {
	t.Parallel()

	aggregator := &Aggregator{Strategy: AggregationStrategyBestEffort, Timeout: 20 * time.Millisecond}
	resultCh := make(chan TaskResult, 1)
	resultCh <- TaskResult{SubtaskID: "a", Output: "ok", VerificationStatus: domain.VerificationStatusPassed}

	result, err := aggregator.Collect(context.Background(), resultCh, 2)
	if err == nil {
		t.Fatal("Collect() error = nil, want timeout")
	}
	if result.Status != AggregationStatusTimedOut {
		t.Fatalf("Collect() status = %q, want %q", result.Status, AggregationStatusTimedOut)
	}
	if !result.TimedOut {
		t.Fatal("Collect() TimedOut = false, want true")
	}
	if result.Completed != 1 {
		t.Fatalf("Collect() completed = %d, want 1", result.Completed)
	}
	if len(result.Results) != 1 {
		t.Fatalf("Collect() result count = %d, want 1", len(result.Results))
	}
	if got := result.Results["a"].Status; got != SubtaskStatusCompleted {
		t.Fatalf("collected result status = %q, want %q", got, SubtaskStatusCompleted)
	}
	if !strings.Contains(result.Summary, "collected 1 of 2") {
		t.Fatalf("Collect() summary = %q, want timeout summary", result.Summary)
	}
}
