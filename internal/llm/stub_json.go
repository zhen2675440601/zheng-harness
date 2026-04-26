package llm

import "strings"

// buildStubJSONOutput keeps stub providers compatible with ModelAdapter's
// JSON contract so provider switching does not break runtime parsing.
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
