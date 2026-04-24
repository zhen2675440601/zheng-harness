package domain

// VerificationResult captures whether the last observation satisfies checks.
type VerificationResult struct {
	Passed bool
	Reason string
}
