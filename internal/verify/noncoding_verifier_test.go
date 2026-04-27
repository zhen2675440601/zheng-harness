package verify

import (
	"context"
	"testing"

	"zheng-harness/internal/domain"
)

func TestResearchVerifierPassesWithCompleteConsistentEvidence(t *testing.T) {
	t.Parallel()

	result, err := (ResearchVerifier{}).Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "The policy supports the proposed change.",
			Sources: []domain.EvidenceSource{{ID: "src-1", Kind: "spec", Locator: "docs/spec.md", Excerpt: "Policy allows the change."}, {ID: "src-2", Kind: "note", Locator: "notes/summary.md", Excerpt: "No conflicts found."}},
			Findings: []domain.EvidenceFinding{{Claim: "Policy allows the change.", SupportingSourceIDs: []string{"src-1"}}, {Claim: "No conflicts found.", SupportingSourceIDs: []string{"src-2"}}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected pass, got %+v", result)
	}
	if result.Status != domain.VerificationStatusPassed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusPassed)
	}
}

func TestResearchVerifierFailsWhenFindingReferencesUnknownSource(t *testing.T) {
	t.Parallel()

	result, err := (ResearchVerifier{}).Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{Research: &domain.ResearchEvidence{
			Conclusion: "The policy supports the proposed change.",
			Sources: []domain.EvidenceSource{{ID: "src-1", Kind: "spec", Locator: "docs/spec.md"}},
			Findings: []domain.EvidenceFinding{{Claim: "Policy allows the change.", SupportingSourceIDs: []string{"src-2"}}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected failure, got %+v", result)
	}
	if result.Status != domain.VerificationStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusFailed)
	}
}

func TestFileWorkflowVerifierPassesWhenResultsMatchExpectations(t *testing.T) {
	t.Parallel()

	result, err := (FileWorkflowVerifier{}).Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{FileWorkflow: &domain.FileWorkflowEvidence{
			Expectations: []domain.FileExpectation{{Path: "docs/output.txt", ShouldExist: true, RequiredContents: []string{"done"}}, {Path: "docs/archive.txt", ShouldExist: false}},
			Results: []domain.FileResult{{Path: "docs/output.txt", Exists: true, Content: "task done"}, {Path: "docs/archive.txt", Exists: false}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("expected pass, got %+v", result)
	}
	if result.Status != domain.VerificationStatusPassed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusPassed)
	}
}

func TestFileWorkflowVerifierFailsWhenExpectationMismatchExists(t *testing.T) {
	t.Parallel()

	result, err := (FileWorkflowVerifier{}).Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{
		Evidence: &domain.Evidence{FileWorkflow: &domain.FileWorkflowEvidence{
			Expectations: []domain.FileExpectation{{Path: "docs/output.txt", ShouldExist: true, RequiredContents: []string{"done"}}},
			Results: []domain.FileResult{{Path: "docs/output.txt", Exists: true, Content: "pending"}},
		}},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Passed {
		t.Fatalf("expected failure, got %+v", result)
	}
	if result.Status != domain.VerificationStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, domain.VerificationStatusFailed)
	}
}
