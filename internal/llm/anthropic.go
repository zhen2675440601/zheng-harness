package llm

import "context"

// AnthropicProvider is a stub adapter boundary for a future Anthropic SDK binding.
type AnthropicProvider struct {
	model string
}

// NewAnthropicProvider constructs the Anthropic adapter without leaking SDK details.
func NewAnthropicProvider(model string) AnthropicProvider {
	return AnthropicProvider{model: model}
}

func (p AnthropicProvider) Name() string {
	return "anthropic"
}

func (p AnthropicProvider) Model() string {
	return p.model
}

func (p AnthropicProvider) Generate(_ context.Context, request Request) (Response, error) {
	return Response{
		Model:      p.model,
		Output:     "anthropic stub response: " + request.Input,
		StopReason: "stub_complete",
	}, nil
}
