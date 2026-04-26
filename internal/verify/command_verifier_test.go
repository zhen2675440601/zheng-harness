package verify

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestCommandVerifierConfirmsPass(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{
		results: map[string]domain.ToolResult{
			"go test ./...":  structuredResult("go test ./...", 0, "ok"),
			"go build ./...": structuredResult("go build ./...", 0, ""),
			"go vet ./...":   structuredResult("go vet ./...", 0, ""),
		},
	}

	v := NewCommandVerifier(executor)
	result, err := v.Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected pass, got %+v", result)
	}
	if len(executor.calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(executor.calls))
	}
}

func TestCommandVerifierDetectsFailure(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{
		results: map[string]domain.ToolResult{
			"go test ./...": structuredResult("go test ./...", 1, "FAIL"),
		},
		errs: map[string]error{
			"go test ./...": errors.New("exit status 1"),
		},
	}

	v := NewCommandVerifier(executor)
	result, err := v.Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected failure, got %+v", result)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(executor.calls))
	}
}

func TestCommandVerifierTimeout(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{
		executeFn: func(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
			<-ctx.Done()
			return domain.ToolResult{ToolName: call.Name}, ctx.Err()
		},
	}

	v := NewCommandVerifier(executor)
	v.timeout = 20 * time.Millisecond

	start := time.Now()
	result, err := v.Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected timeout failure, got %+v", result)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("timeout took too long: %s", time.Since(start))
	}
}

type stubToolExecutor struct {
	results   map[string]domain.ToolResult
	errs      map[string]error
	executeFn func(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error)
	calls     []domain.ToolCall
}

func (s *stubToolExecutor) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	s.calls = append(s.calls, call)
	if s.executeFn != nil {
		return s.executeFn(ctx, call)
	}
	if result, ok := s.results[call.Input]; ok {
		return result, s.errs[call.Input]
	}
	return structuredResult(call.Input, 0, ""), nil
}

func structuredResult(command string, exitCode int, output string) domain.ToolResult {
	return domain.ToolResult{
		ToolName: "exec_command",
		Output: fmt.Sprintf("COMMAND: %s\nEXIT_CODE: %d\nOUTPUT_BEGIN\n%s\nOUTPUT_END", command, exitCode, output),
	}
}
