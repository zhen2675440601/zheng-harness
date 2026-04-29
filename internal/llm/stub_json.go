package llm

import "strings"

// buildStubJSONOutput 使 stub provider 与 ModelAdapter 的 JSON 契约保持兼容，
// 从而避免切换 provider 时破坏运行时解析。
func buildStubJSONOutput(request Request) string {
	input := strings.ToLower(strings.TrimSpace(request.Input))

	switch {
	case strings.Contains(input, `"operation":"create_plan"`) || strings.Contains(input, `"operation": "create_plan"`):
		return `{"summary":"stub plan","steps":["produce deterministic response"]}`
	case strings.Contains(input, `"operation":"next_action"`) || strings.Contains(input, `"operation": "next_action"`):
		return `{"type":"respond","summary":"respond with deterministic output","response":"stub response"}`
	case strings.Contains(input, `"operation":"observe"`) || strings.Contains(input, `"operation": "observe"`):
		return `{"summary":"stub observation","final_response":"stub response"}`
	default:
		return `{"summary":"stub plan","steps":["produce deterministic response"]}`
	}
}
