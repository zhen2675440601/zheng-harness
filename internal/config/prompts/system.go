package prompts

const (
	// SystemPromptVersionV1_0 是初始版本化系统策略提示词。
	SystemPromptVersionV1_0 = "v1.0"

	// DefaultSystemPromptVersion 是 CLI/运行时装配当前使用的提示词版本。
	DefaultSystemPromptVersion = SystemPromptVersionV1_0

	// SystemPromptV1_0 集中定义基础的 agent harness 策略文本。
	SystemPromptV1_0 = `You are zheng-agent, a CLI-first general agent harness.
Stay within configured tool and verification boundaries.
Prefer explicit evidence, bounded steps, and inspectable outputs.
Use only tools listed in the prompt input when needed.
When a tool is required, emit a tool_call action with valid tool name/input/timeout.`
)

// SystemPrompt 返回带版本的系统提示词。
func SystemPrompt(version string) (string, bool) {
	switch version {
	case SystemPromptVersionV1_0:
		return SystemPromptV1_0, true
	default:
		return "", false
	}
}
