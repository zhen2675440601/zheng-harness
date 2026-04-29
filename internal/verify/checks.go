package verify

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"zheng-harness/internal/domain"
)

// CheckResult 记录一次验证动作的结果。
type CheckResult struct {
	Name    string
	Passed  bool
	Details string
}

// Check 执行一次验证动作。
type Check interface {
	Name() string
	Run(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, observation domain.Observation) CheckResult
}

// EvidenceCheck 确保观察结果包含可检查的证据。
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

// TestCheck 验证声称的测试运行所提供的证据。
type TestCheck struct{}

func (TestCheck) Name() string { return string(CheckKindTest) }

func (TestCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) CheckResult {
	text := strings.ToLower(combinedEvidence(observation))
	if commandSucceeded(text, "go test") {
		return CheckResult{Name: string(CheckKindTest), Passed: true, Details: "test evidence found"}
	}
	return CheckResult{Name: string(CheckKindTest), Passed: false, Details: "missing passing test evidence"}
}

// BuildCheck 验证声称的构建运行所提供的证据。
type BuildCheck struct{}

func (BuildCheck) Name() string { return string(CheckKindBuild) }

func (BuildCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) CheckResult {
	text := strings.ToLower(combinedEvidence(observation))
	if commandSucceeded(text, "go build") || strings.Contains(text, "build ok") {
		return CheckResult{Name: string(CheckKindBuild), Passed: true, Details: "build evidence found"}
	}
	return CheckResult{Name: string(CheckKindBuild), Passed: false, Details: "missing successful build evidence"}
}

// LintCheck 验证声称的 lint 运行所提供的证据。
type LintCheck struct{}

func (LintCheck) Name() string { return string(CheckKindLint) }

func (LintCheck) Run(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) CheckResult {
	text := strings.ToLower(combinedEvidence(observation))
	if commandSucceeded(text, "go vet") || strings.Contains(text, "lint ok") {
		return CheckResult{Name: string(CheckKindLint), Passed: true, Details: "lint evidence found"}
	}
	return CheckResult{Name: string(CheckKindLint), Passed: false, Details: "missing successful lint evidence"}
}

// FileExistsCheck 验证证据中引用的文件路径确实存在。
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

func commandSucceeded(text string, commandPrefix string) bool {
	records := parseCommandRecords(text)
	if len(records) > 0 {
		matched := false
		for _, record := range records {
			if strings.HasPrefix(record.Command, commandPrefix) {
				matched = true
				return record.ExitCode == 0
			}
		}
		if matched || len(records) > 0 {
			return false
		}
	}

	if strings.Contains(text, commandPrefix) {
		if strings.Contains(text, "exit_code: 0") {
			return true
		}
		if strings.Contains(text, "pass") || strings.Contains(text, "ok") {
			return true
		}
	}

	return false
}

type commandRecord struct {
	Command  string
	ExitCode int
}

func parseCommandRecords(text string) []commandRecord {
	lines := strings.Split(text, "\n")
	records := make([]commandRecord, 0)
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		lowerLine := strings.ToLower(line)
		if !strings.HasPrefix(lowerLine, "command:") {
			continue
		}

		command := strings.TrimSpace(line[len("command:"):])
		exitCode := -1
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			lowerNext := strings.ToLower(next)
			if strings.HasPrefix(lowerNext, "command:") {
				break
			}
			if strings.HasPrefix(lowerNext, "exit_code:") {
				rawCode := strings.TrimSpace(next[len("exit_code:"):])
				parsed, err := strconv.Atoi(rawCode)
				if err == nil {
					exitCode = parsed
				}
				break
			}
		}

		records = append(records, commandRecord{Command: command, ExitCode: exitCode})
	}

	return records
}
