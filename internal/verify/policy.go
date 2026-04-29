package verify

import "strings"

const (
	VerificationSuccess     = "verification_success"
	VerificationFailed      = "verification_failed"
	InsufficientEvidence    = "insufficient_evidence"
	VerificationContradiction = "contradiction"
)

// CheckKind 标识一种内置验证动作。
type CheckKind string

const (
	CheckKindEvidence CheckKind = "evidence"
	CheckKindTest     CheckKind = "test"
	CheckKindBuild    CheckKind = "build"
	CheckKindLint     CheckKind = "lint"
)

// Policy 控制应执行哪些检查以及可容忍多少次验证失败。
type Policy struct {
	MaxFailures int
	Checks      []CheckKind
}

// DefaultPolicy 为 MVP 提供有界的验证行为。
func DefaultPolicy() Policy {
	return Policy{
		MaxFailures: 2,
		Checks:      []CheckKind{CheckKindEvidence, CheckKindTest, CheckKindBuild, CheckKindLint},
	}
}

// CorrectionInstruction 为失败返回一条有界的修正说明。
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
