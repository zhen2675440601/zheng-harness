package verify

import "strings"

const (
	VerificationSuccess     = "verification_success"
	VerificationFailed      = "verification_failed"
	InsufficientEvidence    = "insufficient_evidence"
	VerificationContradiction = "contradiction"
)

// CheckKind identifies a built-in verification action.
type CheckKind string

const (
	CheckKindEvidence CheckKind = "evidence"
	CheckKindTest     CheckKind = "test"
	CheckKindBuild    CheckKind = "build"
	CheckKindLint     CheckKind = "lint"
)

// Policy controls which checks run and how many failed verifications are tolerated.
type Policy struct {
	MaxFailures int
	Checks      []CheckKind
}

// DefaultPolicy provides bounded verification behavior for the MVP.
func DefaultPolicy() Policy {
	return Policy{
		MaxFailures: 2,
		Checks:      []CheckKind{CheckKindEvidence, CheckKindTest, CheckKindBuild, CheckKindLint},
	}
}

// CorrectionInstruction returns a bounded remediation message for the failure.
func CorrectionInstruction(category string, failedChecks []string) string {
	base := "gather stronger evidence and retry within budget"
	if len(failedChecks) > 0 {
		base = "fix failed checks: " + strings.Join(failedChecks, ", ")
	}
	switch category {
	case VerificationContradiction:
		return "agent claim contradicts evidence; " + base
	case InsufficientEvidence:
		return "completion claim lacks proof; " + base
	case VerificationFailed:
		return "verification failed; " + base
	default:
		return "verification succeeded"
	}
}
