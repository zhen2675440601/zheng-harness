package verify_test

import (
	"context"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/verify"
)

func TestVerifierAcceptsProvenSuccess(t *testing.T) {
	t.Parallel()

	v := verify.NewVerifier(verify.Policy{MaxFailures: 2, Checks: []verify.CheckKind{verify.CheckKindEvidence}})
	result, err := v.Verify(context.Background(), domain.Task{ID: "task-1"}, domain.Session{ID: "session-1"}, domain.Plan{ID: "plan-1"}, nil, domain.Observation{
		Summary:       "go test pass, build ok, lint ok",
		FinalResponse: "done",
		ToolResult:    &domain.ToolResult{Output: "go test ./... PASS\nbuild ok\nlint ok"},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected verification success, got %+v", result)
	}
	if !strings.Contains(result.Reason, verify.VerificationSuccess) {
		t.Fatalf("reason = %q, want success taxonomy", result.Reason)
	}
}

func TestVerifierRejectsFalseSuccess(t *testing.T) {
	t.Parallel()

	v := verify.NewVerifier(verify.Policy{MaxFailures: 1, Checks: []verify.CheckKind{verify.CheckKindEvidence, verify.CheckKindTest}})
	steps := []domain.Step{{Verification: domain.VerificationResult{Passed: false, Reason: "earlier failure"}}}
	result, err := v.Verify(context.Background(), domain.Task{ID: "task-2"}, domain.Session{ID: "session-2"}, domain.Plan{ID: "plan-2"}, steps, domain.Observation{
		Summary:       "complete",
		FinalResponse: "done",
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected verification failure, got %+v", result)
	}
	if !strings.Contains(result.Reason, verify.VerificationFailed) {
		t.Fatalf("reason = %q, want verification_failed taxonomy", result.Reason)
	}
	if !strings.Contains(result.Reason, "fix failed checks") && !strings.Contains(result.Reason, "gather stronger evidence") {
		t.Fatalf("reason = %q, want correction instruction", result.Reason)
	}
}

func TestVerifierAcceptsStructuredCommandEvidence(t *testing.T) {
	t.Parallel()

	v := verify.NewVerifier(verify.DefaultPolicy())
	result, err := v.Verify(context.Background(), domain.Task{ID: "task-3"}, domain.Session{ID: "session-3"}, domain.Plan{ID: "plan-3"}, nil, domain.Observation{
		Summary:       "commands executed",
		FinalResponse: "done",
		ToolResult: &domain.ToolResult{Output: strings.Join([]string{
			"COMMAND: go test ./...",
			"EXIT_CODE: 0",
			"OUTPUT_BEGIN",
			"ok zheng-harness/internal/tools",
			"OUTPUT_END",
			"COMMAND: go build ./...",
			"EXIT_CODE: 0",
			"OUTPUT_BEGIN",
			"",
			"OUTPUT_END",
			"COMMAND: go vet ./...",
			"EXIT_CODE: 0",
			"OUTPUT_BEGIN",
			"",
			"OUTPUT_END",
		}, "\n")},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected verification success, got %+v", result)
	}
}

func TestVerifierRejectsFailedStructuredCommandEvidence(t *testing.T) {
	t.Parallel()

	v := verify.NewVerifier(verify.DefaultPolicy())
	result, err := v.Verify(context.Background(), domain.Task{ID: "task-4"}, domain.Session{ID: "session-4"}, domain.Plan{ID: "plan-4"}, nil, domain.Observation{
		Summary:       "commands executed",
		FinalResponse: "done",
		ToolResult: &domain.ToolResult{Output: strings.Join([]string{
			"COMMAND: go test ./...",
			"EXIT_CODE: 1",
			"OUTPUT_BEGIN",
			"FAIL zheng-harness/internal/tools",
			"OUTPUT_END",
			"COMMAND: go build ./...",
			"EXIT_CODE: 0",
			"OUTPUT_BEGIN",
			"",
			"OUTPUT_END",
			"COMMAND: go vet ./...",
			"EXIT_CODE: 0",
			"OUTPUT_BEGIN",
			"",
			"OUTPUT_END",
		}, "\n")},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected verification failure, got %+v", result)
	}
}

func TestVerifierDoesNotFallbackAcrossUnrelatedStructuredExitCodes(t *testing.T) {
	t.Parallel()

	v := verify.NewVerifier(verify.Policy{MaxFailures: 2, Checks: []verify.CheckKind{verify.CheckKindTest}})
	result, err := v.Verify(context.Background(), domain.Task{ID: "task-5"}, domain.Session{ID: "session-5"}, domain.Plan{ID: "plan-5"}, nil, domain.Observation{
		Summary:       "commands executed",
		FinalResponse: "done",
		ToolResult: &domain.ToolResult{Output: strings.Join([]string{
			"COMMAND: go test ./...",
			"EXIT_CODE: 1",
			"OUTPUT_BEGIN",
			"FAIL",
			"OUTPUT_END",
			"COMMAND: go build ./...",
			"EXIT_CODE: 0",
			"OUTPUT_BEGIN",
			"",
			"OUTPUT_END",
		}, "\n")},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected verification failure when go test exit code is non-zero, got %+v", result)
	}
}
