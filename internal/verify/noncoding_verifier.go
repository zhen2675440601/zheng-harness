package verify

import (
	"context"
	"fmt"
	"strings"

	"zheng-harness/internal/domain"
)

// ResearchVerifier evaluates structured evidence completeness and consistency.
type ResearchVerifier struct{}

// Verify implements domain.Verifier.
func (ResearchVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	if observation.ToolResult != nil && strings.TrimSpace(observation.ToolResult.Error) != "" {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: observation.ToolResult.Error}, nil
	}

	evidence := observation.Evidence
	if evidence == nil || evidence.Research == nil {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusNotApplicable, Reason: "verification not applicable yet: research evidence pending"}, nil
	}

	if reason := validateResearchEvidence(*evidence.Research); reason != "" {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: reason}, nil
	}

	return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "research evidence is complete and consistent"}, nil
}

// FileWorkflowVerifier validates expected file-state and result conditions.
type FileWorkflowVerifier struct{}

// Verify implements domain.Verifier.
func (FileWorkflowVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	if observation.ToolResult != nil && strings.TrimSpace(observation.ToolResult.Error) != "" {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: observation.ToolResult.Error}, nil
	}

	evidence := observation.Evidence
	if evidence == nil || evidence.FileWorkflow == nil {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusNotApplicable, Reason: "verification not applicable yet: file workflow evidence pending"}, nil
	}

	if reason := validateFileWorkflowEvidence(*evidence.FileWorkflow); reason != "" {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: reason}, nil
	}

	return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "file workflow evidence satisfies expected conditions"}, nil
}

func validateResearchEvidence(evidence domain.ResearchEvidence) string {
	if strings.TrimSpace(evidence.Conclusion) == "" {
		return "research evidence missing conclusion"
	}
	if len(evidence.Sources) == 0 {
		return "research evidence missing sources"
	}
	if len(evidence.Findings) == 0 {
		return "research evidence missing findings"
	}

	sources := make(map[string]domain.EvidenceSource, len(evidence.Sources))
	for _, source := range evidence.Sources {
		id := strings.TrimSpace(source.ID)
		if id == "" {
			return "research evidence source missing id"
		}
		if _, exists := sources[id]; exists {
			return fmt.Sprintf("research evidence has duplicate source id %q", id)
		}
		if strings.TrimSpace(source.Locator) == "" {
			return fmt.Sprintf("research evidence source %q missing locator", id)
		}
		sources[id] = source
	}

	for _, finding := range evidence.Findings {
		if strings.TrimSpace(finding.Claim) == "" {
			return "research evidence finding missing claim"
		}
		if len(finding.SupportingSourceIDs) == 0 {
			return fmt.Sprintf("research finding %q missing supporting sources", finding.Claim)
		}
		for _, sourceID := range finding.SupportingSourceIDs {
			id := strings.TrimSpace(sourceID)
			if id == "" {
				return fmt.Sprintf("research finding %q contains empty supporting source id", finding.Claim)
			}
			if _, ok := sources[id]; !ok {
				return fmt.Sprintf("research finding %q references unknown source %q", finding.Claim, id)
			}
		}
	}

	return ""
}

func validateFileWorkflowEvidence(evidence domain.FileWorkflowEvidence) string {
	if len(evidence.Expectations) == 0 {
		return "file workflow evidence missing expectations"
	}
	if len(evidence.Results) == 0 {
		return "file workflow evidence missing results"
	}

	results := make(map[string]domain.FileResult, len(evidence.Results))
	for _, result := range evidence.Results {
		path := normalizeFilePath(result.Path)
		if path == "" {
			return "file workflow result missing path"
		}
		if _, exists := results[path]; exists {
			return fmt.Sprintf("file workflow evidence has duplicate result for %q", path)
		}
		results[path] = result
	}

	for _, expectation := range evidence.Expectations {
		path := normalizeFilePath(expectation.Path)
		if path == "" {
			return "file workflow expectation missing path"
		}
		result, ok := results[path]
		if !ok {
			return fmt.Sprintf("file workflow evidence missing result for %q", path)
		}
		if result.Exists != expectation.ShouldExist {
			return fmt.Sprintf("file workflow result for %q existence mismatch", path)
		}
		if strings.TrimSpace(result.Error) != "" {
			return fmt.Sprintf("file workflow result for %q reported error: %s", path, strings.TrimSpace(result.Error))
		}
		if expectation.ShouldExist {
			for _, fragment := range expectation.RequiredContents {
				part := strings.TrimSpace(fragment)
				if part == "" {
					continue
				}
				if !strings.Contains(result.Content, part) {
					return fmt.Sprintf("file workflow result for %q missing required content %q", path, part)
				}
			}
		}
	}

	return ""
}

func normalizePolicyToken(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeFilePath(path string) string {
	return strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
}
