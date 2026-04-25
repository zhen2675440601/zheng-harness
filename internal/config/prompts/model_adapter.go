package prompts

import (
	"encoding/json"
	"fmt"

	"zheng-harness/internal/domain"
)

// BuildCreatePlanInput renders the provider input for plan generation.
func BuildCreatePlanInput(task domain.Task, session domain.Session) string {
	return mustMarshalPrompt(map[string]any{
		"operation": "create_plan",
		"instructions": []string{
			"Return JSON only.",
			"Use shape: {\"summary\": string, \"steps\": []string}.",
			"Keep the summary concise and the steps actionable.",
		},
		"task": map[string]any{
			"id":          task.ID,
			"description": task.Description,
			"goal":        task.Goal,
		},
		"session": map[string]any{
			"id":     session.ID,
			"status": session.Status,
		},
	})
}

// BuildNextActionInput renders the provider input for action selection.
func BuildNextActionInput(task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step) string {
	history := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		history = append(history, map[string]any{
			"index":               step.Index,
			"action_summary":      step.Action.Summary,
			"action_type":         step.Action.Type,
			"observation":         step.Observation.Summary,
			"final_response":      step.Observation.FinalResponse,
			"verification_passed": step.Verification.Passed,
			"verification_reason": step.Verification.Reason,
		})
	}

	return mustMarshalPrompt(map[string]any{
		"operation": "next_action",
		"instructions": []string{
			"Return JSON only.",
			"Use shape: {\"type\": \"respond\"|\"tool_call\", \"summary\": string, \"response\": string, \"tool_call\": {\"name\": string, \"input\": string, \"timeout\": string}}.",
			"When type is respond, provide response and omit tool_call.",
			"When type is tool_call, provide tool_call and omit response.",
		},
		"task": map[string]any{
			"id":          task.ID,
			"description": task.Description,
			"goal":        task.Goal,
		},
		"session": map[string]any{
			"id":     session.ID,
			"status": session.Status,
		},
		"plan": map[string]any{
			"id":      plan.ID,
			"summary": plan.Summary,
		},
		"history": history,
	})
}

// BuildObserveInput renders the provider input for post-action observation.
func BuildObserveInput(task domain.Task, session domain.Session, plan domain.Plan, action domain.Action, result *domain.ToolResult) string {
	toolResult := map[string]any(nil)
	if result != nil {
		toolResult = map[string]any{
			"tool_name": result.ToolName,
			"output":    result.Output,
			"error":     result.Error,
			"duration":  result.Duration.String(),
		}
	}

	toolCall := map[string]any(nil)
	if action.ToolCall != nil {
		toolCall = map[string]any{
			"name":    action.ToolCall.Name,
			"input":   action.ToolCall.Input,
			"timeout": action.ToolCall.Timeout.String(),
		}
	}

	return mustMarshalPrompt(map[string]any{
		"operation": "observe",
		"instructions": []string{
			"Return JSON only.",
			"Use shape: {\"summary\": string, \"final_response\": string}.",
			"Summarize the outcome of the action using the tool result if present.",
			"Set final_response when the user-facing answer is ready.",
		},
		"task": map[string]any{
			"id":          task.ID,
			"description": task.Description,
			"goal":        task.Goal,
		},
		"session": map[string]any{
			"id":     session.ID,
			"status": session.Status,
		},
		"plan": map[string]any{
			"id":      plan.ID,
			"summary": plan.Summary,
		},
		"action": map[string]any{
			"type":      action.Type,
			"summary":   action.Summary,
			"response":  action.Response,
			"tool_call": toolCall,
		},
		"tool_result": toolResult,
	})
}

func mustMarshalPrompt(payload map[string]any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("marshal prompt payload: %v", err))
	}
	return string(data)
}
