package verify

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"zheng-harness/internal/domain"
)

func TestTaskAwareVerifierUsesCommandVerifierForCodingTasks(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{
		results: map[string]domain.ToolResult{
			"go test ./...":  structuredResult("go test ./...", 0, "ok"),
			"go build ./...": structuredResult("go build ./...", 0, ""),
			"go vet ./...":   structuredResult("go vet ./...", 0, ""),
		},
	}

	v := NewTaskAwareVerifier("standard", executor)
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryCoding}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected pass, got %+v", result)
	}
	if got := len(executor.calls); got != 3 {
		t.Fatalf("command verifier calls = %d, want 3", got)
	}
	if result.Status != domain.VerificationStatusPassed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusPassed)
	}
}

func TestTaskAwareVerifierUsesEvidenceVerifierForResearchTasks(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{}
	v := NewTaskAwareVerifier("standard", executor)
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryResearch}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "Both sources agree on the release date.",
			Sources:    []domain.EvidenceSource{{ID: "src-1", Kind: "doc", Locator: "docs/source-a", Excerpt: "Release date listed as 2026-04-27."}, {ID: "src-2", Kind: "doc", Locator: "docs/source-b", Excerpt: "Published on 2026-04-27."}},
			Findings:   []domain.EvidenceFinding{{Claim: "Release date is 2026-04-27.", SupportingSourceIDs: []string{"src-1", "src-2"}}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected research evidence to pass, got %+v", result)
	}
	if result.Status != domain.VerificationStatusPassed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusPassed)
	}
	if got := len(executor.calls); got != 0 {
		t.Fatalf("command verifier calls = %d, want 0 for research tasks", got)
	}
}

func TestTaskAwareVerifierUsesStateOutputVerifierForFileWorkflowTasks(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{}
	v := NewTaskAwareVerifier("standard", executor)
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryFileWorkflow}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{FileWorkflow: &domain.FileWorkflowEvidence{
			Summary:      "Updated requested file.",
			Expectations: []domain.FileExpectation{{Path: "docs/output.txt", ShouldExist: true, RequiredContents: []string{"done"}}},
			Results:      []domain.FileResult{{Path: "docs/output.txt", Exists: true, Content: "task done\n"}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected file workflow verification to pass, got %+v", result)
	}
	if result.Status != domain.VerificationStatusPassed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusPassed)
	}
	if got := len(executor.calls); got != 0 {
		t.Fatalf("command verifier calls = %d, want 0 for file workflow tasks", got)
	}
}

func TestTaskAwareVerifierRepresentsNotApplicableYet(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryResearch}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected not-applicable result, got %+v", result)
	}
	if result.Status != domain.VerificationStatusNotApplicable {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusNotApplicable)
	}
	if result.Reason == "" {
		t.Fatal("expected not-applicable reason")
	}
}

func TestTaskAwareVerifierFallsBackToCompatibilityPolicyWhenTaskMetadataMissing(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{
		results: map[string]domain.ToolResult{
			"go test ./...":  structuredResult("go test ./...", 0, "ok"),
			"go build ./...": structuredResult("go build ./...", 0, ""),
			"go vet ./...":   structuredResult("go vet ./...", 0, ""),
		},
	}

	v := NewTaskAwareVerifier("standard", executor)
	result, err := v.Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected compatibility fallback to pass, got %+v", result)
	}
	if got := len(executor.calls); got != 3 {
		t.Fatalf("compatibility fallback calls = %d, want 3", got)
	}
}

func TestTaskAwareVerifierFallsBackDeterministicallyForUnknownCategory(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{
		results: map[string]domain.ToolResult{
			"go test ./...":  structuredResult("go test ./...", 0, "ok"),
			"go build ./...": structuredResult("go build ./...", 0, ""),
			"go vet ./...":   structuredResult("go vet ./...", 0, ""),
		},
	}

	v := NewTaskAwareVerifier("standard", executor)
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategory("unsupported")}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected fallback verification to pass, got %+v", result)
	}
	if got := len(executor.calls); got != 3 {
		t.Fatalf("fallback command verifier calls = %d, want 3", got)
	}
}

func TestTaskAwareVerifierPrefersExplicitVerificationPolicy(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{}
	v := NewTaskAwareVerifier("standard", executor)
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryCoding, VerificationPolicy: PolicyEvidenceBased}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "Manual review found no contradictions.",
			Sources:    []domain.EvidenceSource{{ID: "src-1", Kind: "note", Locator: "notes/review", Excerpt: "Review complete."}},
			Findings:   []domain.EvidenceFinding{{Claim: "Review complete.", SupportingSourceIDs: []string{"src-1"}}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected explicit policy override to pass, got %+v", result)
	}
	if got := len(executor.calls); got != 0 {
		t.Fatalf("command executor calls = %d, want 0 when policy overrides", got)
	}
}

func TestTaskAwareVerifierCategoryDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		task     domain.Task
		wantID   string
		wantMode string
	}{
		{name: "coding defaults to command", task: domain.Task{Category: domain.TaskCategoryCoding}, wantID: PolicyCommandBacked, wantMode: "strict"},
		{name: "research defaults to evidence", task: domain.Task{Category: domain.TaskCategoryResearch}, wantID: PolicyEvidenceBased, wantMode: "strict"},
		{name: "file workflow defaults to state output", task: domain.Task{Category: domain.TaskCategoryFileWorkflow}, wantID: PolicyStateOutput, wantMode: "strict"},
		{name: "general falls back to host default", task: domain.Task{Category: domain.TaskCategoryGeneral}, wantID: PolicyCommandBacked, wantMode: "strict"},
		{name: "unknown category normalizes to host default", task: domain.Task{Category: domain.TaskCategory("unsupported")}, wantID: PolicyCommandBacked, wantMode: "strict"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			registry := &capturingVerifierRegistry{resolvedVerifier: stubVerifier{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}}}
			v := NewTaskAwareVerifierWithRegistry(tc.wantMode, &stubToolExecutor{}, registry)

			result, err := v.Verify(context.Background(), tc.task, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if !result.Passed {
				t.Fatalf("expected pass, got %+v", result)
			}
			if registry.lastID != tc.wantID {
				t.Fatalf("registry id = %q, want %q", registry.lastID, tc.wantID)
			}
			if registry.lastMode != tc.wantMode {
				t.Fatalf("registry mode = %q, want %q", registry.lastMode, tc.wantMode)
			}
		})
	}
}

func TestTaskAwareVerifierUnknownPolicyFails(t *testing.T) {
	t.Parallel()

	registry := &capturingVerifierRegistry{resolveErr: errors.New("missing verifier")}
	v := NewTaskAwareVerifierWithRegistry("standard", &stubToolExecutor{}, registry)

	result, err := v.Verify(context.Background(), domain.Task{VerificationPolicy: PolicyEvidenceBased}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected failure, got %+v", result)
	}
	if result.Status != domain.VerificationStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusFailed)
	}
	if got, want := result.Reason, fmt.Sprintf("verification policy %q not configured", PolicyEvidenceBased); got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
}

func TestTaskAwareVerifierDispatchesPluginBackedPolicy(t *testing.T) {
	t.Parallel()

	registry := &capturingVerifierRegistry{resolvedVerifier: stubVerifier{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "plugin dispatched"}}}
	v := NewTaskAwareVerifierWithRegistry("standard", &stubToolExecutor{}, registry)

	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryCoding, VerificationPolicy: "command_based"}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected plugin-backed dispatch to pass, got %+v", result)
	}
	if registry.lastID != PolicyCommandBacked {
		t.Fatalf("registry id = %q, want %q", registry.lastID, PolicyCommandBacked)
	}
}

func TestTaskAwareVerifierUsesExplicitVerificationPolicyWhenCategoryIsGeneral(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryGeneral, VerificationPolicy: PolicyStateOutput}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{FileWorkflow: &domain.FileWorkflowEvidence{
			Expectations: []domain.FileExpectation{{Path: "exports/report.txt", ShouldExist: true}},
			Results:      []domain.FileResult{{Path: "exports/report.txt", Exists: true, Content: "artifact ready"}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected explicit verification policy dispatch to pass, got %+v", result)
	}
	if result.Status != domain.VerificationStatusPassed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusPassed)
	}
}

func TestTaskAwareVerifierRejectsUnknownExplicitVerificationPolicy(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryGeneral, VerificationPolicy: "plugin-defined-policy"}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected unknown policy verification to fail, got %+v", result)
	}
	if result.Status != domain.VerificationStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusFailed)
	}
	if result.Reason != `verification policy "plugin-defined-policy" not configured` {
		t.Fatalf("reason = %q, want unknown policy failure", result.Reason)
	}
}

func TestTaskAwareVerifierFailsResearchEvidenceWhenSourceReferenceUnknown(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryResearch}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "A conclusion exists.",
			Sources:    []domain.EvidenceSource{{ID: "src-1", Kind: "doc", Locator: "docs/source-a"}},
			Findings:   []domain.EvidenceFinding{{Claim: "Claim references missing source.", SupportingSourceIDs: []string{"src-2"}}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected research verification failure, got %+v", result)
	}
	if result.Status != domain.VerificationStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusFailed)
	}
}

func TestTaskAwareVerifierFailsFileWorkflowWhenRequiredContentMissing(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryFileWorkflow}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{FileWorkflow: &domain.FileWorkflowEvidence{
			Expectations: []domain.FileExpectation{{Path: "docs/output.txt", ShouldExist: true, RequiredContents: []string{"done"}}},
			Results:      []domain.FileResult{{Path: "docs/output.txt", Exists: true, Content: "pending"}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected file workflow verification failure, got %+v", result)
	}
	if result.Status != domain.VerificationStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusFailed)
	}
}

type capturingVerifierRegistry struct {
	lastID           string
	lastMode         string
	lastExecutor     domain.ToolExecutor
	resolvedVerifier domain.Verifier
	resolveErr       error
}

func (r *capturingVerifierRegistry) Resolve(id, mode string, executor domain.ToolExecutor) (domain.Verifier, error) {
	r.lastID = id
	r.lastMode = mode
	r.lastExecutor = executor
	if r.resolveErr != nil {
		return nil, r.resolveErr
	}
	if r.resolvedVerifier == nil {
		return stubVerifier{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}}, nil
	}
	return r.resolvedVerifier, nil
}

func (r *capturingVerifierRegistry) DefaultID() string {
	return PolicyCommandBacked
}

type stubVerifier struct {
	result domain.VerificationResult
	err    error
}

func (v stubVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) (domain.VerificationResult, error) {
	return v.result, v.err
}
