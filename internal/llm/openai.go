package llm

import "context"

// OpenAIProvider is a stub adapter boundary for a future OpenAI SDK binding.
type OpenAIProvider struct {
	model string
}

// NewOpenAIProvider constructs the OpenAI adapter without leaking SDK details.
func NewOpenAIProvider(model string) OpenAIProvider {
	return OpenAIProvider{model: model}
}

func (p OpenAIProvider) Name() string {
	return "openai"
}

func (p OpenAIProvider) Model() string {
	return p.model
}

func (p OpenAIProvider) Generate(_ context.Context, request Request) (Response, error) {
	return Response{
		Model:      p.model,
		Output:     "openai stub response: " + request.Input,
		StopReason: "stub_complete",
	}, nil
}
