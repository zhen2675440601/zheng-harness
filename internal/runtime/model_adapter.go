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

// ModelAdapter bridges provider-oriented LLM calls into the domain.Model port.
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

// NewModelAdapter wraps an llm.Provider behind the domain.Model boundary.
func NewModelAdapter(provider llm.Provider) *ModelAdapter {
	systemPrompt, _ := prompts.SystemPrompt(prompts.DefaultSystemPromptVersion)
	return &ModelAdapter{provider: provider, systemPrompt: strings.TrimSpace(systemPrompt)}
}

func (m *ModelAdapter) CreatePlan(ctx context.Context, task domain.Task, session domain.Session) (domain.Plan, error) {
	response, err := m.generate(ctx, prompts.BuildCreatePlanInput(task, session))
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

func (m *ModelAdapter) NextAction(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step) (domain.Action, error) {
	response, err := m.generate(ctx, prompts.BuildNextActionInput(task, session, plan, steps))
	if err != nil {
		return domain.Action{}, err
	}

	var payload actionResponse
	if err := decodeJSONResponse(response.Output, &payload); err != nil {
		return domain.Action{}, fmt.Errorf("decode next action response: %w", err)
	}

	summary := strings.TrimSpace(payload.Summary)
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
		finalResponse := strings.TrimSpace(payload.Response)
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
	response, err := m.provider.Generate(ctx, llm.Request{
		SystemPrompt: m.systemPrompt,
		Input:        input,
	})
	if err != nil {
		return llm.Response{}, fmt.Errorf("generate %s response: %w", m.provider.Name(), err)
	}
	return response, nil
}

func decodeJSONResponse(raw string, target any) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("empty model response")
	}

	// Remove markdown code block wrapper if present
	// Handles ```json\n{...}\n``` and ```\n{...}\n```
	if strings.HasPrefix(trimmed, "```") {
		// Find the end of the first line (after ```json or just ```)
		firstLineEnd := strings.Index(trimmed, "\n")
		if firstLineEnd != -1 {
			// Find the closing ```
			closingIndex := strings.LastIndex(trimmed, "```")
			if closingIndex > firstLineEnd {
				// Extract content between first line and closing
				trimmed = strings.TrimSpace(trimmed[firstLineEnd+1:closingIndex])
			}
		}
	}

	if err := json.Unmarshal([]byte(trimmed), target); err != nil {
		return fmt.Errorf("parse JSON output %q: %w", trimmed, err)
	}
	return nil
}
