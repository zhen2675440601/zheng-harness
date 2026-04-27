package prompts

import (
	"encoding/json"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
)

func TestBuildNextActionInputIncludesTools(t *testing.T) {
	t.Parallel()

	input := BuildNextActionInput(
		domain.Task{ID: "task-1", Description: "desc", Goal: "goal"},
		domain.Session{ID: "session-1", Status: domain.SessionStatusRunning},
		domain.Plan{ID: "plan-1", Summary: "plan"},
		nil,
		[]domain.ToolInfo{{Name: "bash", Description: "run shell", Schema: "{\"type\":\"object\"}"}},
		nil,
	)

	payload := decodePromptJSON(t, input)
	toolsRaw, ok := payload["tools"]
	if !ok {
		t.Fatal("expected tools field")
	}
	tools, ok := toolsRaw.([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want one entry", toolsRaw)
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("tool entry type = %T, want map[string]any", tools[0])
	}
	if got := tool["name"]; got != "bash" {
		t.Fatalf("tool name = %v, want bash", got)
	}
}

func TestBuildNextActionInputIncludesMemoryWhenProvided(t *testing.T) {
	t.Parallel()

	input := BuildNextActionInput(
		domain.Task{ID: "task-2", Description: "desc", Goal: "goal"},
		domain.Session{ID: "session-2", Status: domain.SessionStatusRunning},
		domain.Plan{ID: "plan-2", Summary: "plan"},
		nil,
		nil,
		[]domain.MemoryEntry{{
			Scope:      domain.MemoryScopeProject,
			Type:       domain.MemoryTypeFact,
			Content:    "repo uses sqlite",
			Confidence: 88,
			Source:     "step-3",
		}},
	)

	payload := decodePromptJSON(t, input)
	memoryRaw, ok := payload["memory"]
	if !ok {
		t.Fatal("expected memory field")
	}
	entries, ok := memoryRaw.([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("memory = %#v, want one entry", memoryRaw)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("memory entry type = %T, want map[string]any", entries[0])
	}
	if got := entry["content"]; got != "repo uses sqlite" {
		t.Fatalf("memory content = %v, want repo uses sqlite", got)
	}
}

func TestBuildNextActionInputOmitsMemoryWhenEmpty(t *testing.T) {
	t.Parallel()

	input := BuildNextActionInput(
		domain.Task{ID: "task-3", Description: "desc", Goal: "goal"},
		domain.Session{ID: "session-3", Status: domain.SessionStatusRunning},
		domain.Plan{ID: "plan-3", Summary: "plan"},
		nil,
		nil,
		nil,
	)

	payload := decodePromptJSON(t, input)
	if _, ok := payload["memory"]; ok {
		t.Fatal("memory field should be omitted when no entries")
	}
}

func TestBuildCreatePlanInputIncludesMemoryWhenProvided(t *testing.T) {
	t.Parallel()

	input := BuildCreatePlanInput(
		domain.Task{ID: "task-4", Description: "desc", Goal: "goal"},
		domain.Session{ID: "session-4", Status: domain.SessionStatusRunning},
		[]domain.MemoryEntry{{
			Scope:      domain.MemoryScopeSession,
			Type:       domain.MemoryTypeSummary,
			Content:    "user prefers concise outputs",
			Confidence: 92,
			Source:     "step-5",
		}},
	)

	payload := decodePromptJSON(t, input)
	memoryRaw, ok := payload["memory"]
	if !ok {
		t.Fatal("expected memory field")
	}
	entries, ok := memoryRaw.([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("memory = %#v, want one entry", memoryRaw)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("memory entry type = %T, want map[string]any", entries[0])
	}
	if got := entry["content"]; got != "user prefers concise outputs" {
		t.Fatalf("memory content = %v, want user prefers concise outputs", got)
	}
}

func TestBuildNextActionInputOmitsToolsWhenEmptyOrBlank(t *testing.T) {
	t.Parallel()

	input := BuildNextActionInput(
		domain.Task{ID: "task-5", Description: "desc", Goal: "goal"},
		domain.Session{ID: "session-5", Status: domain.SessionStatusRunning},
		domain.Plan{ID: "plan-5", Summary: "plan"},
		nil,
		[]domain.ToolInfo{{Name: "   ", Description: "ignored", Schema: "{}"}},
		nil,
	)

	payload := decodePromptJSON(t, input)
	if _, ok := payload["tools"]; ok {
		t.Fatal("tools field should be omitted when no usable tools exist")
	}
}

func TestBuildCreatePlanInputOmitsMemoryWhenEntriesAreBlank(t *testing.T) {
	t.Parallel()

	input := BuildCreatePlanInput(
		domain.Task{ID: "task-6", Description: "desc", Goal: "goal"},
		domain.Session{ID: "session-6", Status: domain.SessionStatusRunning},
		[]domain.MemoryEntry{{
			Scope:      domain.MemoryScopeProject,
			Type:       domain.MemoryTypeFact,
			Content:    "   ",
			Confidence: 50,
			Source:     "step-1",
		}},
	)

	payload := decodePromptJSON(t, input)
	if _, ok := payload["memory"]; ok {
		t.Fatal("memory field should be omitted when entries are blank")
	}
}

func TestBuildObserveInputIncludesToolErrorsAndNilToolCall(t *testing.T) {
	t.Parallel()

	input := BuildObserveInput(
		domain.Task{ID: "task-7", Description: "desc", Goal: "goal"},
		domain.Session{ID: "session-7", Status: domain.SessionStatusRunning},
		domain.Plan{ID: "plan-7", Summary: "plan"},
		domain.Action{Type: domain.ActionTypeRespond, Summary: "respond", Response: "done"},
		&domain.ToolResult{ToolName: "grep_search", Output: "out", Error: "regex invalid"},
	)

	payload := decodePromptJSON(t, input)
	action, ok := payload["action"].(map[string]any)
	if !ok {
		t.Fatalf("action payload = %#v, want map", payload["action"])
	}
	if action["tool_call"] != nil {
		t.Fatalf("tool_call = %#v, want nil for respond action", action["tool_call"])
	}
	toolResult, ok := payload["tool_result"].(map[string]any)
	if !ok {
		t.Fatalf("tool_result payload = %#v, want map", payload["tool_result"])
	}
	if got := toolResult["error"]; got != "regex invalid" {
		t.Fatalf("tool result error = %v, want regex invalid", got)
	}
}

func TestBuildNextActionInputIncludesTaskProtocolContextAndExpandedContract(t *testing.T) {
	t.Parallel()

	input := BuildNextActionInput(
		domain.Task{
			ID:                 "task-protocol",
			Description:        "review evidence",
			Goal:               "decide next protocol action",
			Category:           domain.TaskCategoryResearch,
			ProtocolHint:       "evidence_based",
			VerificationPolicy: "evidence_based",
		},
		domain.Session{ID: "session-protocol", Status: domain.SessionStatusRunning},
		domain.Plan{ID: "plan-protocol", Summary: "plan"},
		nil,
		nil,
		nil,
	)

	payload := decodePromptJSON(t, input)
	task, ok := payload["task"].(map[string]any)
	if !ok {
		t.Fatalf("task payload = %#v, want map", payload["task"])
	}
	if got := task["type"]; got != string(domain.TaskCategoryResearch) {
		t.Fatalf("task type = %v, want %q", got, domain.TaskCategoryResearch)
	}
	protocol, ok := task["protocol"].(map[string]any)
	if !ok {
		t.Fatalf("protocol payload = %#v, want map", task["protocol"])
	}
	if got := protocol["category"]; got != string(domain.TaskCategoryResearch) {
		t.Fatalf("protocol category = %v, want %q", got, domain.TaskCategoryResearch)
	}
	if got := protocol["hint"]; got != "evidence_based" {
		t.Fatalf("protocol hint = %v, want evidence_based", got)
	}
	if got := protocol["verification_policy"]; got != "evidence_based" {
		t.Fatalf("verification policy = %v, want evidence_based", got)
	}
	instructions, ok := payload["instructions"].([]any)
	if !ok {
		t.Fatalf("instructions payload = %#v, want []any", payload["instructions"])
	}
	joined := joinInstructionLines(instructions)
	for _, want := range []string{"request_input", "complete", "do not assume the task is code-focused"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("instructions %q missing %q", joined, want)
		}
	}
}

func TestBuildCreatePlanInputIncludesGeneralTaskProtocolDefaults(t *testing.T) {
	t.Parallel()

	input := BuildCreatePlanInput(
		domain.Task{ID: "task-general", Description: "organize files", Goal: "produce safe plan"},
		domain.Session{ID: "session-general", Status: domain.SessionStatusPending},
		nil,
	)

	payload := decodePromptJSON(t, input)
	task, ok := payload["task"].(map[string]any)
	if !ok {
		t.Fatalf("task payload = %#v, want map", payload["task"])
	}
	if got := task["type"]; got != string(domain.TaskCategoryGeneral) {
		t.Fatalf("task type = %v, want %q", got, domain.TaskCategoryGeneral)
	}
	protocol, ok := task["protocol"].(map[string]any)
	if !ok {
		t.Fatalf("protocol payload = %#v, want map", task["protocol"])
	}
	if got := protocol["category"]; got != string(domain.TaskCategoryGeneral) {
		t.Fatalf("protocol category = %v, want %q", got, domain.TaskCategoryGeneral)
	}
	if _, ok := protocol["hint"]; ok {
		t.Fatal("protocol hint should be omitted when blank")
	}
	if _, ok := protocol["verification_policy"]; ok {
		t.Fatal("verification policy should be omitted when blank")
	}
}

func joinInstructionLines(lines []any) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		text, _ := line.(string)
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

func decodePromptJSON(t *testing.T, raw string) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal prompt JSON: %v", err)
	}
	return payload
}
