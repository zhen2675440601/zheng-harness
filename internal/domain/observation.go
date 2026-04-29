package domain

// Observation 是动作执行后运行时的标准化理解结果。
type Observation struct {
	Summary       string
	ToolResult    *ToolResult
	FinalResponse string
	Evidence      *Evidence
}

// Evidence 为非编码任务记录结构化的验证器输入。
type Evidence struct {
	Research     *ResearchEvidence
	FileWorkflow *FileWorkflowEvidence
}

// ResearchEvidence 记录面向研究结论的可检查支撑证据。
type ResearchEvidence struct {
	Sources    []EvidenceSource
	Findings   []EvidenceFinding
	Conclusion string
}

// EvidenceSource 标识研究过程中使用的一个具体来源。
type EvidenceSource struct {
	ID      string
	Kind    string
	Locator string
	Excerpt string
}

// EvidenceFinding 记录一条结论及其支撑来源。
type EvidenceFinding struct {
	Claim             string
	SupportingSourceIDs []string
}

// FileWorkflowEvidence 记录文件工作流中预期与实际观察到的结果。
type FileWorkflowEvidence struct {
	Expectations []FileExpectation
	Results      []FileResult
	Summary      string
}

// FileExpectation 描述一个预期的文件状态条件。
type FileExpectation struct {
	Path             string
	ShouldExist      bool
	RequiredContents []string
}

// FileResult 描述某个文件路径的实际观察结果。
type FileResult struct {
	Path    string
	Exists  bool
	Content string
	Error   string
}
