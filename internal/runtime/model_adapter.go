package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"zheng-harness/internal/config/prompts"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
)

// ModelAdapter 将面向 provider 的 LLM 调用桥接到 domain.Model 端口。
type ModelAdapter struct {
	provider     llm.Provider
	systemPrompt string
}

type planResponse struct {
	Summary string   `json:"summary"`
	Steps   []string `json:"steps"`
}

type actionResponse struct {
	Type     string              `json:"type"`
	Summary  string              `json:"summary"`
	Response string              `json:"response"`
	ToolCall *actionToolCallBody `json:"tool_call,omitempty"`
}

type actionToolCallBody struct {
	Name    string `json:"name"`
	Input   string `json:"input"`
	Timeout string `json:"timeout,omitempty"`
}

type observationResponse struct {
	Summary       string `json:"summary"`
	FinalResponse string `json:"final_response,omitempty"`
}

// NewModelAdapter 在 domain.Model 边界之后封装 llm.Provider。
func NewModelAdapter(provider llm.Provider) *ModelAdapter {
	systemPrompt, _ := prompts.SystemPrompt(prompts.DefaultSystemPromptVersion)
	return &ModelAdapter{provider: provider, systemPrompt: strings.TrimSpace(systemPrompt)}
}

func (m *ModelAdapter) CreatePlan(ctx context.Context, task domain.Task, session domain.Session, memory []domain.MemoryEntry) (domain.Plan, error) {
	response, err := m.generate(ctx, buildCreatePlanInput(task, session, memory))
	if err != nil {
		return domain.Plan{}, err
	}

	var payload planResponse
	if err := decodeJSONResponse(response.Output, &payload); err != nil {
		return domain.Plan{}, fmt.Errorf("decode create plan response: %w", err)
	}

	summary := strings.TrimSpace(payload.Summary)
	if summary == "" {
		summary = strings.TrimSpace(task.Description)
	}

	steps := make([]domain.Step, 0, len(payload.Steps))
	for index, stepSummary := range payload.Steps {
		trimmed := strings.TrimSpace(stepSummary)
		if trimmed == "" {
			continue
		}
		steps = append(steps, domain.Step{
			Index: index + 1,
			Action: domain.Action{
				Type:    domain.ActionTypeRespond,
				Summary: trimmed,
			},
		})
	}

	return domain.Plan{
		ID:      "plan-" + task.ID,
		TaskID:  task.ID,
		Summary: summary,
		Steps:   steps,
	}, nil
}

func (m *ModelAdapter) NextAction(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, memory []domain.MemoryEntry, tools []domain.ToolInfo) (domain.Action, error) {
	response, err := m.generate(ctx, buildNextActionInput(task, session, plan, steps, memory, tools))
	if err != nil {
		return domain.Action{}, err
	}

	var payload actionResponse
	if err := decodeJSONResponse(response.Output, &payload); err != nil {
		return domain.Action{}, fmt.Errorf("decode next action response: %w", err)
	}

	summary := strings.TrimSpace(payload.Summary)
	responseText := strings.TrimSpace(payload.Response)
	switch strings.ToLower(strings.TrimSpace(payload.Type)) {
	case string(domain.ActionTypeToolCall):
		if payload.ToolCall == nil {
			return domain.Action{}, fmt.Errorf("tool_call action missing tool_call body")
		}
		toolName := strings.TrimSpace(payload.ToolCall.Name)
		if toolName == "" {
			return domain.Action{}, fmt.Errorf("tool_call action missing tool name")
		}
		var timeout time.Duration
		if trimmed := strings.TrimSpace(payload.ToolCall.Timeout); trimmed != "" {
			parsed, err := time.ParseDuration(trimmed)
			if err != nil {
				return domain.Action{}, fmt.Errorf("parse tool timeout: %w", err)
			}
			timeout = parsed
		}
		if summary == "" {
			summary = "Call tool " + toolName
		}
		return domain.Action{
			Type:    domain.ActionTypeToolCall,
			Summary: summary,
			ToolCall: &domain.ToolCall{
				Name:    toolName,
				Input:   payload.ToolCall.Input,
				Timeout: timeout,
			},
		}, nil
	case "", string(domain.ActionTypeRespond):
		finalResponse := responseText
		if finalResponse == "" {
			finalResponse = summary
		}
		if summary == "" {
			summary = finalResponse
		}
		if finalResponse == "" {
			return domain.Action{}, fmt.Errorf("respond action missing response")
		}
		return domain.Action{
			Type:     domain.ActionTypeRespond,
			Summary:  summary,
			Response: finalResponse,
		}, nil
	case string(domain.ActionTypeRequestInput):
		if summary == "" {
			summary = responseText
		}
		if responseText == "" {
			return domain.Action{}, fmt.Errorf("request_input action missing response")
		}
		return domain.Action{
			Type:     domain.ActionTypeRequestInput,
			Summary:  summary,
			Response: responseText,
		}, nil
	case string(domain.ActionTypeComplete):
		if summary == "" {
			summary = responseText
		}
		if responseText == "" {
			return domain.Action{}, fmt.Errorf("complete action missing response")
		}
		return domain.Action{
			Type:     domain.ActionTypeComplete,
			Summary:  summary,
			Response: responseText,
		}, nil
	default:
		return domain.Action{}, fmt.Errorf("unsupported action type %q", payload.Type)
	}
}

func (m *ModelAdapter) Observe(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, action domain.Action, result *domain.ToolResult) (domain.Observation, error) {
	response, err := m.generate(ctx, prompts.BuildObserveInput(task, session, plan, action, result))
	if err != nil {
		return domain.Observation{}, err
	}

	var payload observationResponse
	if err := decodeJSONResponse(response.Output, &payload); err != nil {
		return domain.Observation{}, fmt.Errorf("decode observation response: %w", err)
	}

	summary := strings.TrimSpace(payload.Summary)
	if summary == "" {
		summary = strings.TrimSpace(action.Summary)
	}
	finalResponse := strings.TrimSpace(payload.FinalResponse)
	if finalResponse == "" && action.Type == domain.ActionTypeRespond {
		finalResponse = strings.TrimSpace(action.Response)
	}

	return domain.Observation{
		Summary:       summary,
		ToolResult:    result,
		FinalResponse: finalResponse,
	}, nil
}

func (m *ModelAdapter) generate(ctx context.Context, input string) (llm.Response, error) {
	if m == nil || m.provider == nil {
		return llm.Response{}, fmt.Errorf("model adapter requires provider")
	}
	request := llm.Request{
		SystemPrompt: m.systemPrompt,
		Input:        input,
	}
	if emit := streamEmitterFromContext(ctx); emit != nil {
		response, err := m.generateStream(ctx, request, emit)
		if err != nil {
			return llm.Response{}, err
		}
		return response, nil
	}
	response, err := m.provider.Generate(ctx, request)
	if err != nil {
		return llm.Response{}, fmt.Errorf("generate %s response: %w", m.provider.Name(), err)
	}
	return response, nil
}

func (m *ModelAdapter) generateStream(ctx context.Context, request llm.Request, emit func(llm.StreamingEvent) error) (llm.Response, error) {
	var output strings.Builder
	err := m.provider.Stream(ctx, request, func(event llm.StreamingEvent) error {
		if event.Type == domain.EventTokenDelta {
			var payload domain.TokenDeltaPayload
			if err := event.GetPayload(&payload); err != nil {
				return fmt.Errorf("decode %s token delta: %w", m.provider.Name(), err)
			}
			output.WriteString(payload.Content)
		}
		if emit == nil || event.Type == domain.EventSessionComplete {
			return nil
		}
		return emit(event)
	})
	if err != nil {
		return llm.Response{}, fmt.Errorf("stream %s response: %w", m.provider.Name(), err)
	}
	return llm.Response{Model: m.provider.Model(), Output: output.String(), StopReason: "stream_complete"}, nil
}

func decodeJSONResponse[T any](raw string, target *T) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("empty model response")
	}

	// 如存在 Markdown 代码块包装，则先移除。
	// 处理 ```json\n{...}\n``` 与 ```\n{...}\n``` 两种形式。
	if strings.HasPrefix(trimmed, "```") {
		// 查找第一行末尾（位于 ```json 或 ``` 之后）。
		firstLineEnd := strings.Index(trimmed, "\n")
		if firstLineEnd != -1 {
			// 查找结束的 ```。
			closingIndex := strings.LastIndex(trimmed, "```")
			if closingIndex > firstLineEnd {
				// 提取第一行之后到结束标记之前的内容。
				trimmed = strings.TrimSpace(trimmed[firstLineEnd+1:closingIndex])
			}
		}
	}

	if err := json.Unmarshal([]byte(trimmed), target); err != nil {
		return fmt.Errorf("parse JSON output %q: %w", trimmed, err)
	}
	return nil
}

func buildCreatePlanInput(task domain.Task, session domain.Session, memory []domain.MemoryEntry) string {
	return prompts.BuildCreatePlanInput(task, session, memory)
}

func buildNextActionInput(task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, memory []domain.MemoryEntry, tools []domain.ToolInfo) string {
	return prompts.BuildNextActionInput(task, session, plan, steps, tools, memory)
}
