package domain

// Observation is the normalized runtime understanding after an action.
type Observation struct {
	Summary       string
	ToolResult    *ToolResult
	FinalResponse string
	Evidence      *Evidence
}

// Evidence captures structured verifier input for non-coding tasks.
type Evidence struct {
	Research     *ResearchEvidence
	FileWorkflow *FileWorkflowEvidence
}

// ResearchEvidence records inspectable support for research-oriented conclusions.
type ResearchEvidence struct {
	Sources    []EvidenceSource
	Findings   []EvidenceFinding
	Conclusion string
}

// EvidenceSource identifies one concrete source used during research.
type EvidenceSource struct {
	ID      string
	Kind    string
	Locator string
	Excerpt string
}

// EvidenceFinding records one claim and the sources that support it.
type EvidenceFinding struct {
	Claim             string
	SupportingSourceIDs []string
}

// FileWorkflowEvidence records expected and observed file workflow outcomes.
type FileWorkflowEvidence struct {
	Expectations []FileExpectation
	Results      []FileResult
	Summary      string
}

// FileExpectation describes one expected file-state condition.
type FileExpectation struct {
	Path             string
	ShouldExist      bool
	RequiredContents []string
}

// FileResult describes the observed outcome for one file path.
type FileResult struct {
	Path    string
	Exists  bool
	Content string
	Error   string
}
