package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

var errRetryBudgetExceeded = errors.New("runtime retry budget exceeded")

type streamContextKey struct{}

type streamEventEmitter func(domain.StreamingEvent) error

type verifierProvenanceCarrier interface {
	VerifierProvenance() *domain.PluginMetadata
}

type providerProvenanceCarrier interface {
	ProviderProvenance() *domain.PluginMetadata
}

type agentStrategyMetadataCarrier interface {
	Metadata() domain.PluginMetadata
}

type iterationOutcome struct {
	terminal bool
	status   domain.SessionStatus
	error    error
}

type toolInfoLister interface {
	ListToolInfo() []domain.ToolInfo
}

type toolExecutorWithRegistry interface {
	Registry() toolInfoLister
}

// Engine 仅通过 domain 端口协调一个有界的多步骤运行时循环。
type Engine struct {
	Model          domain.Model
	Tools          domain.ToolExecutor
	Memory         domain.MemoryStore
	Sessions       domain.SessionStore
	Verifier       domain.Verifier
	AgentStrategies *AgentStrategyRegistry
	AgentStrategyID string
	EventChannel   *EventChannel
	Clock          func() time.Time
	MaxSteps       int
	MaxRetries     int
	SessionTimeout time.Duration
}

// Run 执行一个有界的计划-执行-验证循环，直到成功或终止。
func (e Engine) Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	if !e.hasRequiredDependencies() {
		return domain.Session{}, domain.Plan{}, nil, fmt.Errorf("runtime engine requires all dependencies")
	}

	now := time.Now
	if e.Clock != nil {
		now = e.Clock
	}

	maxSteps := max(e.MaxSteps, 1)
	maxRetries := max(e.MaxRetries, 0)
	sessionTimeout := e.SessionTimeout
	if sessionTimeout <= 0 {
		sessionTimeout = 5 * time.Minute
	}

	timestamp := now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = timestamp
	}
	protocol := e.resolveTaskProtocol(task)
	task = protocol.Task
	strategy, err := e.resolveAgentStrategy()
	if err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}

	session := domain.Session{
		ID:        task.ID + "-session",
		TaskID:    task.ID,
		Status:    domain.SessionStatusRunning,
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}
	if metadata := agentStrategyMetadata(strategy); metadata != nil {
		session.Provenance = mergeProvenance(session.Provenance, *metadata)
	}
	if metadata := providerProvenanceFromModel(e.Model); metadata != nil {
		session.Provenance = mergeProvenance(session.Provenance, *metadata)
	}

	if err := e.Sessions.SaveSession(ctx, session); err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}

	plan, err := e.createPlan(ctx, protocol, session, timestamp)
	if err != nil {
		return e.failSession(ctx, session, domain.SessionStatusFatalError, domain.Plan{}, nil, err)
	}
	if err := e.emitStepCompleteEvent(0, plan.Summary); err != nil {
		return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, nil, err)
	}

	steps := make([]domain.Step, 0, maxSteps)
	retries := 0
	deadline := time.Now().Add(sessionTimeout)

	for stepIndex := 1; stepIndex <= maxSteps; stepIndex++ {
		if err := ctx.Err(); err != nil {
			return e.failSession(ctx, session, domain.SessionStatusInterrupted, plan, steps, err)
		}
		if !time.Now().Before(deadline) {
			return e.failSession(ctx, session, domain.SessionStatusInterrupted, plan, steps, context.DeadlineExceeded)
		}

		stepCtx, cancel := context.WithDeadline(ctx, deadline)
		action, observation, verification, err := e.executeIteration(stepCtx, protocol, session, plan, steps)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return e.failSession(ctx, session, domain.SessionStatusInterrupted, plan, steps, err)
			}
			return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, steps, err)
		}

		step := domain.Step{
			Index:        stepIndex,
			Action:       action,
			Observation:  observation,
			Verification: verification,
		}
		if metadata := agentStrategyMetadata(strategy); metadata != nil {
			step.Provenance = mergeProvenance(step.Provenance, *metadata)
			session.Provenance = mergeProvenance(session.Provenance, *metadata)
		}
		if metadata := providerProvenanceFromModel(e.Model); metadata != nil {
			step.Provenance = mergeProvenance(step.Provenance, *metadata)
			session.Provenance = mergeProvenance(session.Provenance, *metadata)
		}
		if metadata := verifierProvenanceFromEngine(e.Verifier); metadata != nil {
			step.Provenance = mergeProvenance(step.Provenance, *metadata)
			session.Provenance = mergeProvenance(session.Provenance, *metadata)
		}
		steps = append(steps, step)

		if err := e.recordStep(ctx, session.ID, step, observation); err != nil {
			return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, steps, err)
		}

		session.UpdatedAt = now()
		outcome := e.decideIterationOutcome(protocol, action, verification)
		if outcome.terminal {
			session.Status = outcome.status
			if err := e.Sessions.SaveSession(ctx, session); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			if err := e.emitSessionCompleteEvent(session); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			return session, plan, steps, outcome.error
		}

		if stepIndex == maxSteps {
			return e.failSession(ctx, session, domain.SessionStatusBudgetExceeded, plan, steps, nil)
		}

		retries++
		if retries > maxRetries {
			return e.failSession(ctx, session, domain.SessionStatusVerificationFailed, plan, steps, errRetryBudgetExceeded)
		}

		plan, err = e.createPlan(ctx, protocol, session, now())
		if err != nil {
			return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, steps, err)
		}
		if err := e.emitStepCompleteEvent(len(steps)+1, plan.Summary); err != nil {
			return e.failSession(ctx, session, domain.SessionStatusFatalError, plan, steps, err)
		}
	}

	return e.failSession(ctx, session, domain.SessionStatusBudgetExceeded, plan, steps, nil)
}

// RunStream 异步启动运行时循环，并立即返回事件通道。
func (e Engine) RunStream(ctx context.Context, task domain.Task) (*EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
	if !e.hasRequiredDependencies() {
		return nil, domain.Session{}, domain.Plan{}, nil, fmt.Errorf("runtime engine requires all dependencies")
	}

	eventChannel := NewEventChannel(64)
	streamEngine := e
	streamEngine.EventChannel = eventChannel
	streamCtx := withStreamEventEmitter(ctx, func(event domain.StreamingEvent) error {
		return eventChannel.Emit(event)
	})

	go func() {
		defer eventChannel.Close()
		_, _, _, _ = streamEngine.Run(streamCtx, task)
	}()

	return eventChannel, domain.Session{}, domain.Plan{}, nil, nil
}

func withStreamEventEmitter(ctx context.Context, emit streamEventEmitter) context.Context {
	if emit == nil {
		return ctx
	}
	return context.WithValue(ctx, streamContextKey{}, emit)
}

func streamEmitterFromContext(ctx context.Context) streamEventEmitter {
	if ctx == nil {
		return nil
	}
	emit, _ := ctx.Value(streamContextKey{}).(streamEventEmitter)
	return emit
}

func (e Engine) createPlan(ctx context.Context, protocol ResolvedTaskProtocol, session domain.Session, createdAt time.Time) (domain.Plan, error) {
	memory := e.recallMemory(ctx, protocol.Task, session)
	strategy, err := e.resolveAgentStrategy()
	if err != nil {
		return domain.Plan{}, err
	}
	planDecision, err := strategy.CreatePlan(ctx, AgentPlanContext{Task: protocol.Task, Session: session, Memory: memory})
	if err != nil {
		return domain.Plan{}, err
	}
	plan, err := validateAgentPlanDecision(protocol.Task, planDecision)
	if err != nil {
		return domain.Plan{}, err
	}
	if plan.TaskID == "" {
		plan.TaskID = protocol.Task.ID
	}
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = createdAt
	}
	if err := e.Sessions.SavePlan(ctx, plan); err != nil {
		return domain.Plan{}, err
	}
	return plan, nil
}

func (e Engine) executeIteration(ctx context.Context, protocol ResolvedTaskProtocol, session domain.Session, plan domain.Plan, steps []domain.Step) (domain.Action, domain.Observation, domain.VerificationResult, error) {
	tools := e.listToolInfo()
	memory := e.recallMemory(ctx, protocol.Task, session)
	strategy, err := e.resolveAgentStrategy()
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}
	actionDecision, err := strategy.NextAction(ctx, AgentActionContext{Task: protocol.Task, Session: session, Plan: plan, Steps: steps, Memory: memory, AllowedTools: tools})
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}
	action, err := validateAgentActionDecision(AgentActionContext{AllowedTools: tools}, actionDecision)
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}
	stepIndex := len(steps) + 1
	if action.ToolCall != nil {
		if err := e.emitToolStartEvent(stepIndex, *action.ToolCall); err != nil {
			return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
		}
	}

	var result *domain.ToolResult
	if action.ToolCall != nil {
		executed, execErr := e.Tools.Execute(ctx, *action.ToolCall)
		if execErr != nil {
			executed.Error = execErr.Error()
		}
		result = &executed
		if err := e.emitToolEndEvent(stepIndex, executed); err != nil {
			return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
		}
	}

	observationDecision, err := strategy.Observe(ctx, AgentObservationContext{Task: protocol.Task, Session: session, Plan: plan, Action: action, ToolResult: result})
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}
	observation, err := validateAgentObservationDecision(observationDecision)
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}
	observation.ToolResult = result
	observation = e.normalizeObservationForAction(action, observation)
	if err := e.emitStepCompleteEvent(stepIndex, observation.Summary); err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}

	verification, err := e.verifyIteration(ctx, protocol, session, plan, steps, action, observation)
	if err != nil {
		return domain.Action{}, domain.Observation{}, domain.VerificationResult{}, err
	}

	return action, observation, verification, nil
}

func (e Engine) resolveTaskProtocol(task domain.Task) ResolvedTaskProtocol {
	registry := NewTaskRegistry()
	return registry.Resolve(task)
}

func (e Engine) normalizeObservationForAction(action domain.Action, observation domain.Observation) domain.Observation {
	if strings.TrimSpace(observation.Summary) == "" {
		observation.Summary = strings.TrimSpace(action.Summary)
	}
	if strings.TrimSpace(observation.FinalResponse) == "" {
		switch action.Type {
		case domain.ActionTypeRespond, domain.ActionTypeRequestInput, domain.ActionTypeComplete:
			observation.FinalResponse = strings.TrimSpace(action.Response)
		}
	}
	return observation
}

func (e Engine) verifyIteration(ctx context.Context, protocol ResolvedTaskProtocol, session domain.Session, plan domain.Plan, steps []domain.Step, action domain.Action, observation domain.Observation) (domain.VerificationResult, error) {
	switch action.Type {
	case domain.ActionTypeRequestInput:
		return domain.VerificationResult{
			Passed: false,
			Status: domain.VerificationStatusNotApplicable,
			Reason: fmt.Sprintf("%s task blocked waiting for external input", protocol.Metadata.TaskType),
		}, nil
	default:
		return e.Verifier.Verify(ctx, protocol.Task, session, plan, steps, observation)
	}
}

func (e Engine) decideIterationOutcome(protocol ResolvedTaskProtocol, action domain.Action, verification domain.VerificationResult) iterationOutcome {
	_ = protocol.Metadata
	switch action.Type {
	case domain.ActionTypeRequestInput:
		return iterationOutcome{terminal: true, status: domain.SessionStatusBlockedInput}
	}
	if verification.Passed {
		return iterationOutcome{terminal: true, status: domain.SessionStatusSuccess}
	}
	return iterationOutcome{}
}

func (e Engine) recallMemory(ctx context.Context, task domain.Task, session domain.Session) []domain.MemoryEntry {
	const recallLimit = 10

	queries := []domain.RecallQuery{
		{SessionID: session.ID, Scope: domain.MemoryScopeSession, Type: domain.MemoryTypeFact, Limit: recallLimit},
		{SessionID: session.ID, Scope: domain.MemoryScopeSession, Type: domain.MemoryTypeSummary, Limit: recallLimit},
		{SessionID: session.ID, Scope: domain.MemoryScopeProject, Type: domain.MemoryTypeFact, Limit: recallLimit},
		{SessionID: session.ID, Scope: domain.MemoryScopeProject, Type: domain.MemoryTypeSummary, Limit: recallLimit},
	}

	combined := make([]domain.MemoryEntry, 0, recallLimit)
	seen := map[string]struct{}{}
	for _, query := range queries {
		entries, err := e.Memory.Recall(ctx, query)
		if err != nil {
			log.Printf("runtime: memory recall failed: %v", err)
			return nil
		}
		for _, entry := range entries {
			key := strings.Join([]string{entry.ID, entry.SessionID, entry.Key, entry.Content, entry.Source, entry.Provenance}, "|")
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			combined = append(combined, entry)
		}
	}

	if len(combined) == 0 {
		return nil
	}

	keywords := tokenizeKeywords(task.Description)
	if len(keywords) == 0 {
		if len(combined) > recallLimit {
			return combined[:recallLimit]
		}
		return combined
	}

	type scoredEntry struct {
		entry domain.MemoryEntry
		score int
	}
	scored := make([]scoredEntry, 0, len(combined))
	for _, entry := range combined {
		haystack := strings.ToLower(strings.Join([]string{entry.Key, entry.Content, entry.Source, entry.Provenance}, " "))
		score := 0
		for _, keyword := range keywords {
			if strings.Contains(haystack, keyword) {
				score++
			}
		}
		scored = append(scored, scoredEntry{entry: entry, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	recalled := make([]domain.MemoryEntry, 0, recallLimit)
	for _, item := range scored {
		recalled = append(recalled, item.entry)
		if len(recalled) == recallLimit {
			break
		}
	}
	return recalled
}

func tokenizeKeywords(text string) []string {
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) == 0 {
		return nil
	}
	keywords := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		trimmed := strings.Trim(field, " .,;:!?()[]{}\"'`")
		if len(trimmed) < 3 {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		keywords = append(keywords, trimmed)
	}
	return keywords
}

func (e Engine) hasRequiredDependencies() bool {
	if e.Tools == nil || e.Memory == nil || e.Sessions == nil || e.Verifier == nil {
		return false
	}
	return e.Model != nil || e.AgentStrategies != nil
}

func (e Engine) resolveAgentStrategy() (AgentStrategyPlugin, error) {
	registry := e.AgentStrategies
	if registry == nil {
		registry = NewAgentStrategyRegistry()
		if err := registry.Register(BuiltinAgentStrategyHostDefault, func() (AgentStrategyPlugin, error) {
			if e.Model == nil {
				return nil, errors.New("runtime engine requires model for built-in agent strategy")
			}
			return newBuiltinModelAgentStrategy(e.Model), nil
		}); err != nil {
			return nil, err
		}
	} else {
		registry = registry.Clone()
		if _, ok := registry.Get(BuiltinAgentStrategyHostDefault); !ok {
			if err := registry.Register(BuiltinAgentStrategyHostDefault, func() (AgentStrategyPlugin, error) {
				if e.Model == nil {
					return nil, errors.New("runtime engine requires model for built-in agent strategy")
				}
				return newBuiltinModelAgentStrategy(e.Model), nil
			}); err != nil {
				return nil, err
			}
		}
	}
	return registry.Resolve(e.AgentStrategyID)
}

func (e Engine) listToolInfo() []domain.ToolInfo {
	provider, ok := e.Tools.(toolExecutorWithRegistry)
	if !ok {
		return nil
	}
	registry := provider.Registry()
	if registry == nil {
		return nil
	}
	return registry.ListToolInfo()
}

func (e Engine) recordStep(ctx context.Context, sessionID string, step domain.Step, observation domain.Observation) error {
	if err := e.Sessions.AppendStep(ctx, sessionID, step); err != nil {
		return err
	}
	if err := e.Memory.Remember(ctx, sessionID, observation); err != nil {
		return err
	}
	return nil
}

func (e Engine) failSession(ctx context.Context, session domain.Session, status domain.SessionStatus, plan domain.Plan, steps []domain.Step, cause error) (domain.Session, domain.Plan, []domain.Step, error) {
	now := time.Now
	if e.Clock != nil {
		now = e.Clock
	}
	session.Status = status
	session.UpdatedAt = now()
	if err := e.Sessions.SaveSession(ctx, session); err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}
	if cause != nil {
		if emitErr := e.emitErrorEvent(len(steps), cause.Error()); emitErr != nil {
			return domain.Session{}, domain.Plan{}, nil, emitErr
		}
	}
	if emitErr := e.emitSessionCompleteEvent(session); emitErr != nil {
		return domain.Session{}, domain.Plan{}, nil, emitErr
	}
	return session, plan, steps, cause
}

func (e Engine) emitToolStartEvent(stepIndex int, call domain.ToolCall) error {
	event, err := domain.ToolStart(stepIndex, call.Name, call.Input)
	if err != nil {
		return err
	}
	return e.emitEvent(*event)
}

func (e Engine) emitToolEndEvent(stepIndex int, result domain.ToolResult) error {
	event, err := domain.ToolEnd(stepIndex, result.ToolName, result.Output, result.Error)
	if err != nil {
		return err
	}
	return e.emitEvent(*event)
}

func (e Engine) emitStepCompleteEvent(stepIndex int, summary string) error {
	event, err := domain.StepComplete(stepIndex, summary)
	if err != nil {
		return err
	}
	return e.emitEvent(*event)
}

func (e Engine) emitErrorEvent(stepIndex int, message string) error {
	event, err := domain.Error(stepIndex, message)
	if err != nil {
		return err
	}
	return e.emitEvent(*event)
}

func (e Engine) emitSessionCompleteEvent(session domain.Session) error {
	event, err := domain.SessionComplete(session.ID, string(session.Status))
	if err != nil {
		return err
	}
	return e.emitEvent(*event)
}

func (e Engine) emitEvent(event domain.StreamingEvent) error {
	if e.EventChannel == nil {
		return nil
	}
	return e.EventChannel.Emit(event)
}

func verifierProvenanceFromEngine(verifier domain.Verifier) *domain.PluginMetadata {
	carrier, ok := verifier.(verifierProvenanceCarrier)
	if !ok || carrier == nil {
		return nil
	}
	return carrier.VerifierProvenance()
}

func providerProvenanceFromModel(model domain.Model) *domain.PluginMetadata {
	carrier, ok := model.(providerProvenanceCarrier)
	if !ok || carrier == nil {
		return nil
	}
	return carrier.ProviderProvenance()
}

func agentStrategyMetadata(strategy AgentStrategyPlugin) *domain.PluginMetadata {
	carrier, ok := strategy.(agentStrategyMetadataCarrier)
	if !ok || carrier == nil {
		return nil
	}
	metadata := carrier.Metadata().Normalize()
	if err := metadata.Validate(); err != nil {
		return nil
	}
	if metadata.Family == domain.PluginFamilyAgentStrategy && metadata.LogicalID == BuiltinAgentStrategyHostDefault && metadata.ExecutionMode == domain.PluginExecutionModeNative && metadata.SourcePath == "builtin://runtime/model" {
		return nil
	}
	return &metadata
}

func mergeProvenance(existing *domain.Provenance, metadata domain.PluginMetadata) *domain.Provenance {
	normalized := metadata.Normalize()
	if err := normalized.Validate(); err != nil {
		return existing
	}
	plugins := make([]domain.PluginMetadata, 0, 1)
	if existing != nil {
		plugins = append(plugins, existing.Normalize().Plugins...)
		for _, current := range plugins {
			if current.Family == normalized.Family && current.LogicalID == normalized.LogicalID {
				copyExisting := existing.Normalize()
				return &copyExisting
			}
		}
	}
	plugins = append(plugins, normalized)
	merged := domain.Provenance{Plugins: plugins}.Normalize()
	if err := merged.Validate(); err != nil {
		return existing
	}
	return &merged
}
