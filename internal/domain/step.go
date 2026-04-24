package domain

// Step records one action/observation/verification cycle in a session.
type Step struct {
	Index        int
	Action       Action
	Observation  Observation
	Verification VerificationResult
}
