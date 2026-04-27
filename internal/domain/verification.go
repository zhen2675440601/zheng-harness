package domain

import "encoding/json"

// VerificationStatus captures the normalized verification outcome taxonomy.
type VerificationStatus string

const (
	VerificationStatusPassed        VerificationStatus = "passed"
	VerificationStatusFailed        VerificationStatus = "failed"
	VerificationStatusNotApplicable VerificationStatus = "not_applicable"
)

// VerificationResult captures whether the last observation satisfies checks.
type VerificationResult struct {
	Passed bool
	Status VerificationStatus
	Reason string
}

// StatusOrDefault backfills additive verification status for older payloads.
func (r VerificationResult) StatusOrDefault() VerificationStatus {
	if r.Status != "" {
		return r.Status
	}
	if r.Passed {
		return VerificationStatusPassed
	}
	return VerificationStatusFailed
}

// Normalize applies additive compatibility defaults for verification metadata.
func (r VerificationResult) Normalize() VerificationResult {
	r.Status = r.StatusOrDefault()
	return r
}

// MarshalJSON emits normalized verification status while preserving field names.
func (r VerificationResult) MarshalJSON() ([]byte, error) {
	type verificationJSON VerificationResult
	return json.Marshal(verificationJSON(r.Normalize()))
}

// UnmarshalJSON backfills additive verification metadata for older persisted payloads.
func (r *VerificationResult) UnmarshalJSON(data []byte) error {
	type verificationJSON VerificationResult
	var decoded verificationJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = VerificationResult(decoded).Normalize()
	return nil
}
