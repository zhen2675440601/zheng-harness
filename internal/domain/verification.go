package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VerificationStatus 记录标准化的验证结果分类。
type VerificationStatus string

const (
	VerificationStatusPassed        VerificationStatus = "passed"
	VerificationStatusFailed        VerificationStatus = "failed"
	VerificationStatusNotApplicable VerificationStatus = "not_applicable"
)

// VerificationResult 记录最近一次观察是否通过各项检查。
type VerificationResult struct {
	Passed bool
	Status VerificationStatus
	Reason string
}

// Validate enforces the host-owned verification result schema.
func (r VerificationResult) Validate() error {
	r = r.Normalize()
	if strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("verification reason must not be empty")
	}
	switch r.Status {
	case VerificationStatusPassed:
		if !r.Passed {
			return fmt.Errorf("verification status %q requires passed=true", r.Status)
		}
	case VerificationStatusFailed, VerificationStatusNotApplicable:
		if r.Passed {
			return fmt.Errorf("verification status %q requires passed=false", r.Status)
		}
	default:
		return fmt.Errorf("unknown verification status %q", r.Status)
	}
	return nil
}

// StatusOrDefault 为旧载荷补齐增量验证状态。
func (r VerificationResult) StatusOrDefault() VerificationStatus {
	if r.Status != "" {
		return r.Status
	}
	if r.Passed {
		return VerificationStatusPassed
	}
	return VerificationStatusFailed
}

// Normalize 为验证元数据应用增量兼容默认值。
func (r VerificationResult) Normalize() VerificationResult {
	r.Status = r.StatusOrDefault()
	return r
}

// MarshalJSON 在保留字段名的同时输出标准化验证状态。
func (r VerificationResult) MarshalJSON() ([]byte, error) {
	type verificationJSON VerificationResult
	normalized := r.Normalize()
	// Allow empty Reason for backward compatibility with old persisted data
	// Only validate reason when it's explicitly set
	if strings.TrimSpace(normalized.Reason) == "" && normalized.Status != "" {
		// For old data with empty reason, provide a default reason
		normalized.Reason = "verified"
	}
	return json.Marshal(verificationJSON(normalized))
}

// UnmarshalJSON 为旧的持久化载荷补齐增量验证元数据。
func (r *VerificationResult) UnmarshalJSON(data []byte) error {
	type verificationJSON VerificationResult
	var decoded verificationJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	normalized := VerificationResult(decoded).Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	*r = normalized
	return nil
}
