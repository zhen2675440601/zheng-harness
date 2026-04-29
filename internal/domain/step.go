package domain

// Step 记录会话中的一次动作/观察/验证循环。
type Step struct {
	Index        int
	Action       Action
	Observation  Observation
	Verification VerificationResult
}
