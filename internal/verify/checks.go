package verify

import (
	"context"
	"fmt"
	"os"
	"strings"

	"zheng-harness/internal/domain"
)

// CheckResult captures the outcome of one verification action.
type CheckResult struct {
	Name    string
	Passed  bool
	Details string
}

// Check executes one verification action.
type Check interface {
	Name() string
	Run(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, observation domain.Observation) CheckResult
}

// EvidenceCheck ensures the observation contains inspectable evidence.
type EvidenceCheck struct{}

func (EvidenceCheck) Name() string { return string(CheckKindEvidence) }

func (EvidenceCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) CheckResult {
	if observation.ToolResult != nil {
		if observation.ToolResult.Error != "" {
			return CheckResult{Name: string(CheckKindEvidence), Passed: false, Details: observation.ToolResult.Error}
		}
		if strings.TrimSpace(observation.ToolResult.Output) != "" {
			return CheckResult{Name: string(CheckKindEvidence), Passed: true, Details: "tool evidence present"}
		}
	}
	if strings.TrimSpace(observation.FinalResponse) != "" {
		return CheckResult{Name: string(CheckKindEvidence), Passed: true, Details: "final response present"}
	}
	return CheckResult{Name: string(CheckKindEvidence), Passed: false, Details: "no evidence attached to completion claim"}
}

// TestCheck validates evidence from a claimed test run.
type TestCheck struct{}

func (TestCheck) Name() string { return string(CheckKindTest) }

func (TestCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) CheckResult {
	text := strings.ToLower(combinedEvidence(observation))
	if strings.Contains(text, "go test") && strings.Contains(text, "pass") {
		return CheckResult{Name: string(CheckKindTest), Passed: true, Details: "test evidence found"}
	}
	return CheckResult{Name: string(CheckKindTest), Passed: false, Details: "missing passing test evidence"}
}

// BuildCheck validates evidence from a claimed build run.
type BuildCheck struct{}

func (BuildCheck) Name() string { return string(CheckKindBuild) }

func (BuildCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) CheckResult {
	text := strings.ToLower(combinedEvidence(observation))
	if strings.Contains(text, "build ok") || strings.Contains(text, "go build") {
		return CheckResult{Name: string(CheckKindBuild), Passed: true, Details: "build evidence found"}
	}
	return CheckResult{Name: string(CheckKindBuild), Passed: false, Details: "missing successful build evidence"}
}

// LintCheck validates evidence from a claimed lint run.
type LintCheck struct{}

func (LintCheck) Name() string { return string(CheckKindLint) }

func (LintCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) CheckResult {
	text := strings.ToLower(combinedEvidence(observation))
	if strings.Contains(text, "lint ok") || strings.Contains(text, "go vet") {
		return CheckResult{Name: string(CheckKindLint), Passed: true, Details: "lint evidence found"}
	}
	return CheckResult{Name: string(CheckKindLint), Passed: false, Details: "missing successful lint evidence"}
}

// FileExistsCheck verifies that a file path referenced by evidence exists.
type FileExistsCheck struct {
	Path string
}

func (c FileExistsCheck) Name() string { return "file_exists" }

func (c FileExistsCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) CheckResult {
	if strings.TrimSpace(c.Path) == "" {
		return CheckResult{Name: c.Name(), Passed: false, Details: "file path must not be empty"}
	}
	if _, err := os.Stat(c.Path); err != nil {
		return CheckResult{Name: c.Name(), Passed: false, Details: fmt.Sprintf("file missing: %v", err)}
	}
	return CheckResult{Name: c.Name(), Passed: true, Details: "file exists"}
}

func combinedEvidence(observation domain.Observation) string {
	parts := []string{observation.Summary, observation.FinalResponse}
	if observation.ToolResult != nil {
		parts = append(parts, observation.ToolResult.Output, observation.ToolResult.Error)
	}
	return strings.Join(parts, "\n")
}
