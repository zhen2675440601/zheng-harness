package domain

import "encoding/json"

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
	return json.Marshal(verificationJSON(r.Normalize()))
}

// UnmarshalJSON 为旧的持久化载荷补齐增量验证元数据。
func (r *VerificationResult) UnmarshalJSON(data []byte) error {
	type verificationJSON VerificationResult
	var decoded verificationJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = VerificationResult(decoded).Normalize()
	return nil
}
