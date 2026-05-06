package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"zheng-harness/internal/domain"
)

const (
	// AgentStrategyPluginContractVersion pins the host-controlled strategy contract version.
	AgentStrategyPluginContractVersion = "1.0.0"
	// BuiltinAgentStrategyHostDefault identifies the built-in runtime strategy registered through the same seam as plugins.
	BuiltinAgentStrategyHostDefault = "host_default"
)

var (
	ErrInvalidAgentStrategyResponse       = errors.New("invalid agent strategy response")
	ErrAgentStrategyNotFound              = errors.New("agent strategy not found")
	ErrAgentStrategyContractVersionMismatch = errors.New("agent strategy plugin contract version mismatch")
)

// AgentStrategyPlugin extends planning/execution policy only.
//
// Host authority remains outside this contract: strategy plugins cannot orchestrate child agents,
// cannot access cancellation handles, cannot persist session state, and cannot mutate task graphs.
// The host exposes only sanitized snapshots plus an allowlisted tool catalog, then validates every response.
type AgentStrategyPlugin interface {
	Metadata() domain.PluginMetadata
	CreatePlan(ctx context.Context, input AgentPlanContext) (AgentPlanDecision, error)
	NextAction(ctx context.Context, input AgentActionContext) (AgentActionDecision, error)
	Observe(ctx context.Context, input AgentObservationContext) (AgentObservationDecision, error)
}

type AgentStrategyFactory func() (AgentStrategyPlugin, error)

type AgentStrategyDescriptor struct {
	ID      string
	Factory AgentStrategyFactory
}

// AgentStrategyRegistry stores host-approved strategy factories under deterministic IDs.
type AgentStrategyRegistry struct {
	mu        sync.RWMutex
	entries   map[string]AgentStrategyDescriptor
	defaultID string
}

// AgentPlanContext is the only planning input a strategy may observe.
type AgentPlanContext struct {
	Task    domain.Task
	Session domain.Session
	Memory  []domain.MemoryEntry
}

// AgentPlanDecision is the only planning output a strategy may return.
type AgentPlanDecision struct {
	Plan domain.Plan
}

// AgentActionContext is the only execution-planning input a strategy may observe.
// AllowedTools is host-owned and represents the complete authority boundary for tool selection.
type AgentActionContext struct {
	Task         domain.Task
	Session      domain.Session
	Plan         domain.Plan
	Steps        []domain.Step
	Memory       []domain.MemoryEntry
	AllowedTools []domain.ToolInfo
}

// AgentActionDecision is the only execution decision a strategy may return.
// It may select one allowlisted tool call or a bounded response/complete/request-input action.
type AgentActionDecision struct {
	Action domain.Action
}

// AgentObservationContext is the only post-execution input a strategy may observe.
// ToolResult is host-produced; strategies may read it but may not replace or forge it.
type AgentObservationContext struct {
	Task       domain.Task
	Session    domain.Session
	Plan       domain.Plan
	Action     domain.Action
	ToolResult *domain.ToolResult
}

// AgentObservationDecision is the only observation output a strategy may return.
// ToolResult must remain nil because the host owns actual tool execution records.
type AgentObservationDecision struct {
	Observation domain.Observation
}

// NewAgentStrategyRegistry constructs an empty deterministic strategy registry.
func NewAgentStrategyRegistry() *AgentStrategyRegistry {
	return &AgentStrategyRegistry{
		entries:   make(map[string]AgentStrategyDescriptor),
		defaultID: BuiltinAgentStrategyHostDefault,
	}
}

func (r *AgentStrategyRegistry) Register(id string, factory AgentStrategyFactory) error {
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return errors.New("agent strategy id must not be empty")
	}
	if factory == nil {
		return errors.New("agent strategy factory must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[normalizedID]; exists {
		return fmt.Errorf("duplicate agent strategy id %q", normalizedID)
	}
	r.entries[normalizedID] = AgentStrategyDescriptor{ID: normalizedID, Factory: factory}
	return nil
}

func (r *AgentStrategyRegistry) Get(id string) (AgentStrategyDescriptor, bool) {
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" || r == nil {
		return AgentStrategyDescriptor{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, ok := r.entries[normalizedID]
	return descriptor, ok
}

func (r *AgentStrategyRegistry) DefaultID() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultID
}

func (r *AgentStrategyRegistry) Clone() *AgentStrategyRegistry {
	if r == nil {
		return NewAgentStrategyRegistry()
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	clone := &AgentStrategyRegistry{
		entries:   make(map[string]AgentStrategyDescriptor, len(r.entries)),
		defaultID: r.defaultID,
	}
	for id, descriptor := range r.entries {
		clone.entries[id] = descriptor
	}
	return clone
}

func (r *AgentStrategyRegistry) Resolve(id string) (AgentStrategyPlugin, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: registry is nil", ErrAgentStrategyNotFound)
	}
	selectedID := strings.TrimSpace(id)
	r.mu.RLock()
	if selectedID == "" {
		selectedID = r.defaultID
	}
	descriptor, ok := r.entries[selectedID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrAgentStrategyNotFound, selectedID)
	}
	strategy, err := descriptor.Factory()
	if err != nil {
		return nil, err
	}
	if strategy == nil {
		return nil, fmt.Errorf("agent strategy %q factory returned nil strategy", selectedID)
	}
	metadata := strategy.Metadata().Normalize()
	if err := validateAgentStrategyMetadata(metadata); err != nil {
		return nil, err
	}
	if metadata.LogicalID != descriptor.ID {
		return nil, fmt.Errorf("agent strategy metadata logical id %q must match registry id %q", metadata.LogicalID, descriptor.ID)
	}
	return strategy, nil
}

func validateAgentStrategyMetadata(metadata domain.PluginMetadata) error {
	if err := metadata.Validate(); err != nil {
		return err
	}
	if metadata.Family != domain.PluginFamilyAgentStrategy {
		return fmt.Errorf("agent strategy metadata family must be %q, got %q", domain.PluginFamilyAgentStrategy, metadata.Family)
	}
	if metadata.ContractVersion != AgentStrategyPluginContractVersion {
		return fmt.Errorf("%w: plugin=%q expected=%q", ErrAgentStrategyContractVersionMismatch, metadata.ContractVersion, AgentStrategyPluginContractVersion)
	}
	return nil
}

func validateAgentPlanDecision(task domain.Task, decision AgentPlanDecision) (domain.Plan, error) {
	plan := decision.Plan
	if strings.TrimSpace(plan.TaskID) != "" && plan.TaskID != task.ID {
		return domain.Plan{}, fmt.Errorf("%w: plan task_id %q does not match task %q", ErrInvalidAgentStrategyResponse, plan.TaskID, task.ID)
	}
	return plan, nil
}

func validateAgentActionDecision(input AgentActionContext, decision AgentActionDecision) (domain.Action, error) {
	action := decision.Action
	switch action.Type {
	case domain.ActionTypeToolCall:
		if action.ToolCall == nil {
			return domain.Action{}, fmt.Errorf("%w: tool_call action requires tool call payload", ErrInvalidAgentStrategyResponse)
		}
		if !isAllowedTool(action.ToolCall.Name, input.AllowedTools) {
			return domain.Action{}, fmt.Errorf("%w: tool %q is not in host allowlist", ErrInvalidAgentStrategyResponse, action.ToolCall.Name)
		}
	case domain.ActionTypeRespond, domain.ActionTypeRequestInput, domain.ActionTypeComplete:
		if action.ToolCall != nil {
			return domain.Action{}, fmt.Errorf("%w: action type %q must not include tool call payload", ErrInvalidAgentStrategyResponse, action.Type)
		}
	default:
		return domain.Action{}, fmt.Errorf("%w: unsupported action type %q", ErrInvalidAgentStrategyResponse, action.Type)
	}
	return action, nil
}

func validateAgentObservationDecision(decision AgentObservationDecision) (domain.Observation, error) {
	observation := decision.Observation
	if observation.ToolResult != nil {
		return domain.Observation{}, fmt.Errorf("%w: observation must not include tool result payload", ErrInvalidAgentStrategyResponse)
	}
	return observation, nil
}

func isAllowedTool(name string, tools []domain.ToolInfo) bool {
	if len(tools) == 0 {
		return true
	}
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func newBuiltinModelAgentStrategy(model domain.Model) AgentStrategyPlugin {
	return builtinModelAgentStrategy{model: model}
}

type builtinModelAgentStrategy struct {
	model domain.Model
}

func (s builtinModelAgentStrategy) Metadata() domain.PluginMetadata {
	return domain.PluginMetadata{
		Family:                domain.PluginFamilyAgentStrategy,
		LogicalID:             BuiltinAgentStrategyHostDefault,
		DisplayName:           "Host Default Runtime Strategy",
		ContractVersion:       AgentStrategyPluginContractVersion,
		ImplementationVersion: "1.0.0",
		ExecutionMode:         domain.PluginExecutionModeNative,
		SourcePath:            "builtin://runtime/model",
	}
}

func (s builtinModelAgentStrategy) CreatePlan(ctx context.Context, input AgentPlanContext) (AgentPlanDecision, error) {
	plan, err := s.model.CreatePlan(ctx, input.Task, input.Session, input.Memory)
	if err != nil {
		return AgentPlanDecision{}, err
	}
	return AgentPlanDecision{Plan: plan}, nil
}

func (s builtinModelAgentStrategy) NextAction(ctx context.Context, input AgentActionContext) (AgentActionDecision, error) {
	action, err := s.model.NextAction(ctx, input.Task, input.Session, input.Plan, input.Steps, input.Memory, input.AllowedTools)
	if err != nil {
		return AgentActionDecision{}, err
	}
	return AgentActionDecision{Action: action}, nil
}

func (s builtinModelAgentStrategy) Observe(ctx context.Context, input AgentObservationContext) (AgentObservationDecision, error) {
	observation, err := s.model.Observe(ctx, input.Task, input.Session, input.Plan, input.Action, input.ToolResult)
	if err != nil {
		return AgentObservationDecision{}, err
	}
	observation.ToolResult = nil
	return AgentObservationDecision{Observation: observation}, nil
}
