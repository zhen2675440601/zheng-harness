package store

import (
	"context"
	"database/sql"
	"time"

	"zheng-harness/internal/domain"
)

// StepRepository 持久化并恢复可检查的运行时步骤。
type StepRepository struct {
	db *sql.DB
}

// NewStepRepository 构造步骤仓储。
func NewStepRepository(database *Database) *StepRepository {
	return &StepRepository{db: database.SQL()}
}

// Append 存储单条步骤/事件记录。
func (r *StepRepository) Append(ctx context.Context, sessionID string, step domain.Step) error {
	var toolName, toolInput, toolOutput, toolError string
	var toolTimeout, toolDuration int64
	if step.Action.ToolCall != nil {
		toolName = step.Action.ToolCall.Name
		toolInput = step.Action.ToolCall.Input
		toolTimeout = int64(step.Action.ToolCall.Timeout)
	}
	if step.Observation.ToolResult != nil {
		toolOutput = step.Observation.ToolResult.Output
		toolError = step.Observation.ToolResult.Error
		toolDuration = int64(step.Observation.ToolResult.Duration)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT OR REPLACE INTO steps (
  session_id, step_index, action_type, action_summary, action_response,
  tool_name, tool_input, tool_timeout_ns, observation_summary, observation_final_response,
  tool_output, tool_error, tool_duration_ns, verification_passed, verification_reason, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		sessionID,
		step.Index,
		string(step.Action.Type),
		step.Action.Summary,
		step.Action.Response,
		toolName,
		toolInput,
		toolTimeout,
		step.Observation.Summary,
		step.Observation.FinalResponse,
		toolOutput,
		toolError,
		toolDuration,
		boolToInt(step.Verification.Passed),
		step.Verification.Reason,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

// LoadBySession 按顺序恢复某个会话的全部已持久化步骤。
func (r *StepRepository) LoadBySession(ctx context.Context, sessionID string) ([]domain.Step, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT step_index, action_type, action_summary, action_response,
       tool_name, tool_input, tool_timeout_ns,
       observation_summary, observation_final_response,
       tool_output, tool_error, tool_duration_ns,
       verification_passed, verification_reason
FROM steps
WHERE session_id = ?
ORDER BY step_index ASC
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	steps := make([]domain.Step, 0)
	for rows.Next() {
		var (
			stepIndex                                           int
			actionType, actionSummary, actionResponse           string
			toolName, toolInput                                 string
			toolTimeout, toolDuration                           int64
			observationSummary, observationFinalResponse        string
			toolOutput, toolError, verificationReason           string
			verificationPassed                                  int
		)
		if err := rows.Scan(
			&stepIndex,
			&actionType,
			&actionSummary,
			&actionResponse,
			&toolName,
			&toolInput,
			&toolTimeout,
			&observationSummary,
			&observationFinalResponse,
			&toolOutput,
			&toolError,
			&toolDuration,
			&verificationPassed,
			&verificationReason,
		); err != nil {
			return nil, err
		}

		step := domain.Step{
			Index: stepIndex,
			Action: domain.Action{
				Type:     domain.ActionType(actionType),
				Summary:  actionSummary,
				Response: actionResponse,
			},
			Observation: domain.Observation{
				Summary:       observationSummary,
				FinalResponse: observationFinalResponse,
			},
			Verification: domain.VerificationResult{
				Passed: verificationPassed == 1,
				Reason: verificationReason,
			},
		}
		if toolName != "" {
			step.Action.ToolCall = &domain.ToolCall{Name: toolName, Input: toolInput, Timeout: time.Duration(toolTimeout)}
			step.Observation.ToolResult = &domain.ToolResult{ToolName: toolName, Output: toolOutput, Error: toolError, Duration: time.Duration(toolDuration)}
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
