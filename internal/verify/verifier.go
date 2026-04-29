package verify

import (
	"context"
	"fmt"
	"strings"

	"zheng-harness/internal/domain"
)

// Verifier 应用策略驱动的证据检查与有界自我纠正。
type Verifier struct {
	policy Policy
	checks map[CheckKind]Check
}

// NewVerifier 构造一个带内置检查项的验证器。
func NewVerifier(policy Policy) *Verifier {
	defaults := DefaultPolicy()
	if policy.MaxFailures <= 0 {
		policy.MaxFailures = defaults.MaxFailures
	}
	if len(policy.Checks) == 0 {
		policy.Checks = defaults.Checks
	}
	return &Verifier{
		policy: policy,
		checks: map[CheckKind]Check{
			CheckKindEvidence: EvidenceCheck{},
			CheckKindTest:     TestCheck{},
			CheckKindBuild:    BuildCheck{},
			CheckKindLint:     LintCheck{},
		},
	}
}

// Verify 实现 domain.Verifier。
func (v *Verifier) Verify(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	failedChecks := make([]string, 0)
	category := VerificationSuccess

	for _, kind := range v.policy.Checks {
		check, ok := v.checks[kind]
		if !ok {
			return domain.VerificationResult{}, fmt.Errorf("verification check %q not configured", kind)
		}
		result := check.Run(ctx, task, session, plan, steps, observation)
		if !result.Passed {
			failedChecks = append(failedChecks, result.Name+": "+result.Details)
		}
	}

	if len(failedChecks) > 0 {
		category = v.classifyFailure(observation, failedChecks)
		if v.failureCount(steps) >= v.policy.MaxFailures {
			category = VerificationFailed
		}
		return domain.VerificationResult{
			Passed: false,
			Status: domain.VerificationStatusFailed,
			Reason: category + ": " + CorrectionInstruction(category, failedChecks),
		}, nil
	}

	return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: VerificationSuccess + ": evidence confirms completion claim"}, nil
}

func (v *Verifier) classifyFailure(observation domain.Observation, failedChecks []string) string {
	text := strings.ToLower(observation.FinalResponse + "\n" + observation.Summary)
	if strings.Contains(text, "done") || strings.Contains(text, "complete") || strings.Contains(text, "success") {
		return VerificationContradiction
	}
	if hasEvidenceFailure(failedChecks) {
		return InsufficientEvidence
	}
	return VerificationFailed
}

func (v *Verifier) failureCount(steps []domain.Step) int {
	count := 0
	for _, step := range steps {
		if !step.Verification.Passed {
			count++
		}
	}
	return count
}

func hasEvidenceFailure(failed []string) bool {
	for _, item := range failed {
		if strings.HasPrefix(item, string(CheckKindEvidence)+":") {
			return true
		}
	}
	return false
}
