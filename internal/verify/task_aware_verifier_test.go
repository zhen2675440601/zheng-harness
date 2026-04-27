package verify

import (
	"context"
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

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryResearch}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "Both sources agree on the release date.",
			Sources: []domain.EvidenceSource{{ID: "src-1", Kind: "doc", Locator: "docs/source-a", Excerpt: "Release date listed as 2026-04-27."}, {ID: "src-2", Kind: "doc", Locator: "docs/source-b", Excerpt: "Published on 2026-04-27."}},
			Findings: []domain.EvidenceFinding{{Claim: "Release date is 2026-04-27.", SupportingSourceIDs: []string{"src-1", "src-2"}}},
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
}

func TestTaskAwareVerifierUsesStateOutputVerifierForFileWorkflowTasks(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryFileWorkflow}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{FileWorkflow: &domain.FileWorkflowEvidence{
			Summary: "Updated requested file.",
			Expectations: []domain.FileExpectation{{Path: "docs/output.txt", ShouldExist: true, RequiredContents: []string{"done"}}},
			Results: []domain.FileResult{{Path: "docs/output.txt", Exists: true, Content: "task done\n"}},
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

func TestTaskAwareVerifierPrefersExplicitVerificationPolicy(t *testing.T) {
	t.Parallel()

	executor := &stubToolExecutor{}
	v := NewTaskAwareVerifier("standard", executor)
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryCoding, VerificationPolicy: PolicyEvidenceBased}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "Manual review found no contradictions.",
			Sources: []domain.EvidenceSource{{ID: "src-1", Kind: "note", Locator: "notes/review", Excerpt: "Review complete."}},
			Findings: []domain.EvidenceFinding{{Claim: "Review complete.", SupportingSourceIDs: []string{"src-1"}}},
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

func TestTaskAwareVerifierUsesExplicitVerificationPolicyWhenCategoryIsGeneral(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryGeneral, VerificationPolicy: PolicyStateOutput}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{FileWorkflow: &domain.FileWorkflowEvidence{
			Expectations: []domain.FileExpectation{{Path: "exports/report.txt", ShouldExist: true}},
			Results: []domain.FileResult{{Path: "exports/report.txt", Exists: true, Content: "artifact ready"}},
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

func TestTaskAwareVerifierFailsResearchEvidenceWhenSourceReferenceUnknown(t *testing.T) {
	t.Parallel()

	v := NewTaskAwareVerifier("standard", &stubToolExecutor{})
	result, err := v.Verify(context.Background(), domain.Task{Category: domain.TaskCategoryResearch}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "A conclusion exists.",
			Sources: []domain.EvidenceSource{{ID: "src-1", Kind: "doc", Locator: "docs/source-a"}},
			Findings: []domain.EvidenceFinding{{Claim: "Claim references missing source.", SupportingSourceIDs: []string{"src-2"}}},
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
			Results: []domain.FileResult{{Path: "docs/output.txt", Exists: true, Content: "pending"}},
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
