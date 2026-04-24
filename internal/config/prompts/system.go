package prompts

const (
	// SystemPromptVersionV1_0 is the initial versioned system policy prompt.
	SystemPromptVersionV1_0 = "v1.0"

	// DefaultSystemPromptVersion is the active prompt version for CLI/runtime wiring.
	DefaultSystemPromptVersion = SystemPromptVersionV1_0

	// SystemPromptV1_0 centralizes the baseline coding-agent policy text.
	SystemPromptV1_0 = `You are zheng-agent, a CLI-first coding agent.
Stay within configured tool and verification boundaries.
Prefer explicit evidence, bounded steps, and inspectable outputs.`
)

// SystemPrompt returns a versioned system prompt.
func SystemPrompt(version string) (string, bool) {
	switch version {
	case SystemPromptVersionV1_0:
		return SystemPromptV1_0, true
	default:
		return "", false
	}
}
