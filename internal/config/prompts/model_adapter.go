package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"zheng-harness/internal/domain"
)

const maxToolSchemaLength = 1200

// BuildCreatePlanInput 生成用于创建计划的 provider 输入。
func BuildCreatePlanInput(task domain.Task, session domain.Session, memory []domain.MemoryEntry) string {
	payload := map[string]any{
		"operation": "create_plan",
		"instructions": []string{
			"Return JSON only.",
			"Use shape: {\"summary\": string, \"steps\": []string}.",
			"Keep the summary concise and the steps actionable.",
		},
		"task":    buildTaskPayload(task),
		"session": map[string]any{
			"id":     session.ID,
			"status": session.Status,
		},
	}

	if memoryPayload := buildMemoryPayload(memory); len(memoryPayload) > 0 {
		payload["memory"] = memoryPayload
	}

	return mustMarshalPrompt(payload)
}

// BuildNextActionInput 生成用于选择下一步动作的 provider 输入。
func BuildNextActionInput(task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, tools []domain.ToolInfo, memory []domain.MemoryEntry) string {
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

	payload := map[string]any{
		"operation": "next_action",
		"instructions": []string{
			"Return JSON only.",
			"Use shape: {\"type\": \"respond\"|\"tool_call\"|\"request_input\"|\"complete\", \"summary\": string, \"response\": string, \"tool_call\": {\"name\": string, \"input\": string, \"timeout\": string}}.",
			"When type is respond, provide response and omit tool_call.",
			"When type is tool_call, provide tool_call and omit response.",
			"When type is request_input, provide response describing the exact missing external input and omit tool_call.",
			"When type is complete, provide response containing the final user-facing completion message and omit tool_call.",
			"Choose the action type that best fits the task protocol; do not assume the task is code-focused.",
		},
		"task":    buildTaskPayload(task),
		"session": map[string]any{
			"id":     session.ID,
			"status": session.Status,
		},
		"plan": map[string]any{
			"id":      plan.ID,
			"summary": plan.Summary,
		},
		"history": history,
	}

	if toolsPayload := buildToolsPayload(tools); len(toolsPayload) > 0 {
		payload["tools"] = toolsPayload
	}
	if memoryPayload := buildMemoryPayload(memory); len(memoryPayload) > 0 {
		payload["memory"] = memoryPayload
	}

	return mustMarshalPrompt(payload)
}

// BuildObserveInput 生成用于动作执行后观察的 provider 输入。
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
		"task": buildTaskPayload(task),
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

func buildToolsPayload(tools []domain.ToolInfo) []map[string]any {
	if len(tools) == 0 {
		return nil
	}

	payload := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		payload = append(payload, map[string]any{
			"name":        name,
			"description": strings.TrimSpace(tool.Description),
			"schema":      truncateToolSchema(tool.Schema),
		})
	}

	if len(payload) == 0 {
		return nil
	}
	return payload
}

func buildMemoryPayload(memory []domain.MemoryEntry) []map[string]any {
	if len(memory) == 0 {
		return nil
	}

	payload := make([]map[string]any, 0, len(memory))
	for _, entry := range memory {
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			continue
		}
		payload = append(payload, map[string]any{
			"scope":      entry.Scope,
			"type":       entry.Type,
			"content":    content,
			"confidence": entry.Confidence,
			"source":     strings.TrimSpace(entry.Source),
		})
	}

	if len(payload) == 0 {
		return nil
	}
	return payload
}

func buildTaskPayload(task domain.Task) map[string]any {
	category := task.CategoryOrDefault()
	protocolHint := strings.TrimSpace(task.ProtocolHint)
	verificationPolicy := strings.TrimSpace(task.VerificationPolicy)

	protocol := map[string]any{
		"category": category,
	}
	if protocolHint != "" {
		protocol["hint"] = protocolHint
	}
	if verificationPolicy != "" {
		protocol["verification_policy"] = verificationPolicy
	}

	return map[string]any{
		"id":          task.ID,
		"description": task.Description,
		"goal":        task.Goal,
		"type":        category,
		"protocol":    protocol,
	}
}

func truncateToolSchema(schema string) string {
	trimmed := strings.TrimSpace(schema)
	if len(trimmed) <= maxToolSchemaLength {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxToolSchemaLength]) + "…(truncated)"
}
