package verify

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

const defaultVerificationCommandTimeout = 60 * time.Second

// CommandVerifier executes real verification commands through a ToolExecutor.
type CommandVerifier struct {
	executor domain.ToolExecutor
	timeout  time.Duration
}

// NewCommandVerifier constructs a command-backed verifier.
func NewCommandVerifier(executor domain.ToolExecutor) *CommandVerifier {
	return &CommandVerifier{executor: executor, timeout: defaultVerificationCommandTimeout}
}

// Verify implements domain.Verifier.
func (v *CommandVerifier) Verify(ctx context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) (domain.VerificationResult, error) {
	if v.executor == nil {
		return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: "verification command not available"}, nil
	}

	commands := []string{"go test ./...", "go build ./...", "go vet ./..."}
	reasons := make([]string, 0, len(commands))

	for _, command := range commands {
		result, err := v.runCommand(ctx, command)
		if err != nil {
			if isCommandUnavailableError(err) {
				return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: "verification command not available"}, nil
			}
			if strings.TrimSpace(result.Output) != "" {
				reasons = append(reasons, result.Output)
			} else {
				reasons = append(reasons, fmt.Sprintf("COMMAND: %s\nERROR: %v", command, err))
			}
			return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: strings.Join(reasons, "\n")}, nil
		}

		reasons = append(reasons, result.Output)
		records := parseStructuredCommandRecords(result.Output)
		if len(records) == 0 {
			return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: strings.Join(reasons, "\n")}, nil
		}
		last := records[len(records)-1]
		if last.ExitCode != 0 {
			return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: strings.Join(reasons, "\n")}, nil
		}
	}

	return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: strings.Join(reasons, "\n")}, nil
}

func (v *CommandVerifier) runCommand(ctx context.Context, command string) (domain.ToolResult, error) {
	timeout := v.timeout
	if timeout <= 0 {
		timeout = defaultVerificationCommandTimeout
	}
	verifyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := v.executor.Execute(verifyCtx, domain.ToolCall{
		Name:    "exec_command",
		Input:   command,
		Timeout: timeout,
	})
	if err != nil {
		if verifyCtx.Err() == context.DeadlineExceeded || errorsContainDeadline(err) {
			if strings.TrimSpace(result.Output) == "" {
				result.Output = fmt.Sprintf("COMMAND: %s\nEXIT_CODE: -1\nOUTPUT_BEGIN\nverification command timed out after %s\nOUTPUT_END", command, timeout)
			}
		}
	}
	return result, err
}

func errorsContainDeadline(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, context.DeadlineExceeded.Error()) || strings.Contains(text, "timeout")
}

func isCommandUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "not allowlisted") ||
		strings.Contains(text, "is not registered") ||
		strings.Contains(text, "executable file not found") ||
		strings.Contains(text, "is not recognized") ||
		strings.Contains(text, "not found")
}

type structuredCommandRecord struct {
	Command  string
	ExitCode int
}

func parseStructuredCommandRecords(text string) []structuredCommandRecord {
	lines := strings.Split(text, "\n")
	records := make([]structuredCommandRecord, 0)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "COMMAND:") {
			continue
		}

		command := strings.TrimSpace(strings.TrimPrefix(line, "COMMAND:"))
		exitCode := -1

		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if strings.HasPrefix(next, "COMMAND:") {
				break
			}
			if strings.HasPrefix(next, "EXIT_CODE:") {
				raw := strings.TrimSpace(strings.TrimPrefix(next, "EXIT_CODE:"))
				parsed, err := strconv.Atoi(raw)
				if err == nil {
					exitCode = parsed
				}
				break
			}
		}

		records = append(records, structuredCommandRecord{Command: command, ExitCode: exitCode})
	}

	return records
}
