package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestAgentStrategyBuiltInUsesRegistry(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	model := &strategyAwareModelStub{
		plan:        domain.Plan{ID: "plan-1", Summary: "builtin plan"},
		action:      domain.Action{Type: domain.ActionTypeRespond, Summary: "done", Response: "done"},
		observation: domain.Observation{Summary: "complete", FinalResponse: "done"},
	}
	sessions := &agentStrategySessionStoreStub{}
	engine := Engine{
		Model:          model,
		Tools:          agentStrategyToolExecutorStub{},
		Memory:         agentStrategyMemoryStoreStub{},
		Sessions:       sessions,
		Verifier:       agentStrategyVerifierStub{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}},
		AgentStrategies: NewAgentStrategyRegistry(),
		Clock:          func() time.Time { return fixedTime },
		MaxSteps:       1,
		SessionTimeout: time.Minute,
	}

	session, _, steps, err := engine.Run(context.Background(), domain.Task{ID: "task-1", Description: "inspect", Goal: "done", CreatedAt: fixedTime})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("session status = %q, want %q", session.Status, domain.SessionStatusSuccess)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if model.createPlanCalls != 1 || model.nextActionCalls != 1 || model.observeCalls != 1 {
		t.Fatalf("builtin strategy did not route through model seam: %+v", model)
	}
	if got := sessions.savedPlans[0].TaskID; got != "task-1" {
		t.Fatalf("saved plan task id = %q, want task-1", got)
	}
}

func TestAgentStrategyAllowsValidResponses(t *testing.T) {
	t.Parallel()

	registry := NewAgentStrategyRegistry()
	if err := registry.Register(BuiltinAgentStrategyHostDefault, func() (AgentStrategyPlugin, error) {
		return strategyPluginStub{
			metadata: validStrategyMetadata(BuiltinAgentStrategyHostDefault),
			plan:     AgentPlanDecision{Plan: domain.Plan{ID: "plan-1", Summary: "use read_file"}},
			action:   AgentActionDecision{Action: domain.Action{Type: domain.ActionTypeToolCall, Summary: "read file", ToolCall: &domain.ToolCall{Name: "read_file", Input: "README.md", Timeout: time.Second}}},
			observe:  AgentObservationDecision{Observation: domain.Observation{Summary: "observed output", FinalResponse: "done"}},
		}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	engine := Engine{
		AgentStrategyID: BuiltinAgentStrategyHostDefault,
		Tools: agentStrategyToolExecutorStub{infos: []domain.ToolInfo{{Name: "read_file"}}, result: domain.ToolResult{ToolName: "read_file", Output: "ok"}},
		Memory: agentStrategyMemoryStoreStub{},
		Sessions: &agentStrategySessionStoreStub{},
		Verifier: agentStrategyVerifierStub{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}},
		AgentStrategies: registry,
		Clock: func() time.Time { return time.Date(2026, 4, 30, 12, 1, 0, 0, time.UTC) },
		MaxSteps: 1,
		SessionTimeout: time.Minute,
	}

	_, _, steps, err := engine.Run(context.Background(), domain.Task{ID: "task-valid", Description: "inspect", Goal: "done"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if got := steps[0].Observation.ToolResult.Output; got != "ok" {
		t.Fatalf("tool output = %q, want ok", got)
	}
}

func TestAgentStrategyRejectsInvalidResponse(t *testing.T) {
	t.Parallel()

	registry := mustRegisterStrategy(t, strategyPluginStub{
		metadata: validStrategyMetadata(BuiltinAgentStrategyHostDefault),
		plan:     AgentPlanDecision{Plan: domain.Plan{ID: "plan-1", Summary: "attempt recursive spawn"}},
		action:   AgentActionDecision{Action: domain.Action{Type: domain.ActionTypeToolCall, Summary: "spawn child", ToolCall: &domain.ToolCall{Name: "spawn_agent", Input: "{}", Timeout: time.Second}}},
	})
	sessions := &agentStrategySessionStoreStub{}
	engine := Engine{
		AgentStrategyID: BuiltinAgentStrategyHostDefault,
		Tools:           agentStrategyToolExecutorStub{infos: []domain.ToolInfo{{Name: "read_file"}}},
		Memory:          agentStrategyMemoryStoreStub{},
		Sessions:        sessions,
		Verifier:        agentStrategyVerifierStub{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}},
		AgentStrategies: registry,
		Clock:           func() time.Time { return time.Date(2026, 4, 30, 12, 2, 0, 0, time.UTC) },
		MaxSteps:        1,
		SessionTimeout:  time.Minute,
	}

	session, _, steps, err := engine.Run(context.Background(), domain.Task{ID: "task-invalid", Description: "inspect", Goal: "done"})
	if !errors.Is(err, ErrInvalidAgentStrategyResponse) {
		t.Fatalf("Run() error = %v, want %v", err, ErrInvalidAgentStrategyResponse)
	}
	if session.Status != domain.SessionStatusFatalError {
		t.Fatalf("session status = %q, want %q", session.Status, domain.SessionStatusFatalError)
	}
	if len(steps) != 0 {
		t.Fatalf("steps = %d, want 0 after rejected strategy decision", len(steps))
	}
	if len(sessions.steps) != 0 {
		t.Fatalf("persisted steps = %d, want 0 after rejected strategy decision", len(sessions.steps))
	}
}

func TestAgentStrategyRejectsForgedToolResultObservation(t *testing.T) {
	t.Parallel()

	registry := mustRegisterStrategy(t, strategyPluginStub{
		metadata: validStrategyMetadata(BuiltinAgentStrategyHostDefault),
		plan:     AgentPlanDecision{Plan: domain.Plan{ID: "plan-1", Summary: "valid tool use"}},
		action:   AgentActionDecision{Action: domain.Action{Type: domain.ActionTypeToolCall, Summary: "read file", ToolCall: &domain.ToolCall{Name: "read_file", Input: "README.md", Timeout: time.Second}}},
		observe:  AgentObservationDecision{Observation: domain.Observation{Summary: "forged", ToolResult: &domain.ToolResult{ToolName: "spawn_agent", Output: "forged"}}},
	})
	sessions := &agentStrategySessionStoreStub{}
	engine := Engine{
		AgentStrategyID: BuiltinAgentStrategyHostDefault,
		Tools:           agentStrategyToolExecutorStub{infos: []domain.ToolInfo{{Name: "read_file"}}, result: domain.ToolResult{ToolName: "read_file", Output: "ok"}},
		Memory:          agentStrategyMemoryStoreStub{},
		Sessions:        sessions,
		Verifier:        agentStrategyVerifierStub{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}},
		AgentStrategies: registry,
		Clock:           func() time.Time { return time.Date(2026, 4, 30, 12, 3, 0, 0, time.UTC) },
		MaxSteps:        1,
		SessionTimeout:  time.Minute,
	}

	session, _, steps, err := engine.Run(context.Background(), domain.Task{ID: "task-forged", Description: "inspect", Goal: "done"})
	if !errors.Is(err, ErrInvalidAgentStrategyResponse) {
		t.Fatalf("Run() error = %v, want %v", err, ErrInvalidAgentStrategyResponse)
	}
	if session.Status != domain.SessionStatusFatalError {
		t.Fatalf("session status = %q, want %q", session.Status, domain.SessionStatusFatalError)
	}
	if len(steps) != 0 {
		t.Fatalf("steps = %d, want 0 after forged observation rejection", len(steps))
	}
}

func TestAgentStrategyRegistryRejectsContractMismatch(t *testing.T) {
	t.Parallel()

	registry := NewAgentStrategyRegistry()
	if err := registry.Register(BuiltinAgentStrategyHostDefault, func() (AgentStrategyPlugin, error) {
		return strategyPluginStub{metadata: domain.PluginMetadata{
			Family:          domain.PluginFamilyAgentStrategy,
			LogicalID:       BuiltinAgentStrategyHostDefault,
			DisplayName:     "Broken Strategy",
			ContractVersion: "0.9.0",
			ExecutionMode:   domain.PluginExecutionModeExternal,
			SourcePath:      "/plugins/broken-strategy",
		}}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := registry.Resolve(BuiltinAgentStrategyHostDefault)
	if !errors.Is(err, ErrAgentStrategyContractVersionMismatch) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrAgentStrategyContractVersionMismatch)
	}
}

func TestAgentStrategyPluginCancellationPropagation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	registry := mustRegisterStrategy(t, strategyPluginStub{
		metadata: validStrategyMetadata(BuiltinAgentStrategyHostDefault),
		plan:     AgentPlanDecision{Plan: domain.Plan{ID: "plan-1", Summary: "wait for cancel"}},
		actionFn: func(ctx context.Context, input AgentActionContext) (AgentActionDecision, error) {
			close(started)
			<-ctx.Done()
			return AgentActionDecision{}, ctx.Err()
		},
	})
	sessions := &agentStrategySessionStoreStub{}
	engine := Engine{
		Tools:           agentStrategyToolExecutorStub{infos: []domain.ToolInfo{{Name: "read_file"}}},
		Memory:          agentStrategyMemoryStoreStub{},
		Sessions:        sessions,
		Verifier:        agentStrategyVerifierStub{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}},
		AgentStrategies: registry,
		Clock:           func() time.Time { return time.Date(2026, 4, 30, 12, 4, 0, 0, time.UTC) },
		MaxSteps:        1,
		SessionTimeout:  time.Minute,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		_, _, _, err := engine.Run(ctx, domain.Task{ID: "task-cancel-plugin", Description: "inspect", Goal: "done"})
		errCh <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("strategy action did not start")
	}
	cancel()
	err := <-errCh
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context canceled", err)
	}
	if len(sessions.steps) != 0 {
		t.Fatalf("persisted steps = %d, want 0", len(sessions.steps))
	}
}

func TestAgentStrategyPluginFailureDoesNotCorruptSessionPersistence(t *testing.T) {
	t.Parallel()

	boom := errors.New("strategy observe failed")
	registry := mustRegisterStrategy(t, strategyPluginStub{
		metadata: validStrategyMetadata(BuiltinAgentStrategyHostDefault),
		plan:     AgentPlanDecision{Plan: domain.Plan{ID: "plan-1", Summary: "run once"}},
		action:   AgentActionDecision{Action: domain.Action{Type: domain.ActionTypeRespond, Summary: "respond", Response: "done"}},
		observeFn: func(context.Context, AgentObservationContext) (AgentObservationDecision, error) {
			return AgentObservationDecision{}, boom
		},
	})
	sessions := &agentStrategySessionStoreStub{}
	engine := Engine{
		Tools:           agentStrategyToolExecutorStub{},
		Memory:          agentStrategyMemoryStoreStub{},
		Sessions:        sessions,
		Verifier:        agentStrategyVerifierStub{result: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed}},
		AgentStrategies: registry,
		Clock:           func() time.Time { return time.Date(2026, 4, 30, 12, 5, 0, 0, time.UTC) },
		MaxSteps:        1,
		SessionTimeout:  time.Minute,
	}
	session, _, steps, err := engine.Run(context.Background(), domain.Task{ID: "task-persist", Description: "inspect", Goal: "done"})
	if !errors.Is(err, boom) {
		t.Fatalf("Run() error = %v, want %v", err, boom)
	}
	if session.Status != domain.SessionStatusFatalError {
		t.Fatalf("session status = %q, want %q", session.Status, domain.SessionStatusFatalError)
	}
	if len(steps) != 0 {
		t.Fatalf("steps = %d, want 0", len(steps))
	}
	if len(sessions.savedSessions) < 2 {
		t.Fatalf("saved sessions = %d, want >= 2", len(sessions.savedSessions))
	}
	if sessions.savedSessions[0].Status != domain.SessionStatusRunning {
		t.Fatalf("initial saved status = %q, want %q", sessions.savedSessions[0].Status, domain.SessionStatusRunning)
	}
	if sessions.savedSessions[len(sessions.savedSessions)-1].Status != domain.SessionStatusFatalError {
		t.Fatalf("final saved status = %q, want %q", sessions.savedSessions[len(sessions.savedSessions)-1].Status, domain.SessionStatusFatalError)
	}
	if len(sessions.steps) != 0 {
		t.Fatalf("persisted steps = %d, want 0", len(sessions.steps))
	}
	if sessions.savedSessions[len(sessions.savedSessions)-1].Provenance == nil || len(sessions.savedSessions[len(sessions.savedSessions)-1].Provenance.Plugins) == 0 {
		t.Fatal("expected strategy provenance to remain persisted on failure")
	}
}

func mustRegisterStrategy(t *testing.T, strategy strategyPluginStub) *AgentStrategyRegistry {
	t.Helper()
	registry := NewAgentStrategyRegistry()
	if err := registry.Register(BuiltinAgentStrategyHostDefault, func() (AgentStrategyPlugin, error) {
		return strategy, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return registry
}

func validStrategyMetadata(id string) domain.PluginMetadata {
	return domain.PluginMetadata{
		Family:                domain.PluginFamilyAgentStrategy,
		LogicalID:             id,
		DisplayName:           "Strategy Plugin",
		ContractVersion:       AgentStrategyPluginContractVersion,
		ImplementationVersion: "1.0.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "/plugins/strategy-plugin",
	}
}

type strategyPluginStub struct {
	metadata domain.PluginMetadata
	plan     AgentPlanDecision
	action   AgentActionDecision
	observe  AgentObservationDecision
	actionFn  func(context.Context, AgentActionContext) (AgentActionDecision, error)
	observeFn func(context.Context, AgentObservationContext) (AgentObservationDecision, error)
}

func (s strategyPluginStub) Metadata() domain.PluginMetadata { return s.metadata }
func (s strategyPluginStub) CreatePlan(context.Context, AgentPlanContext) (AgentPlanDecision, error) {
	return s.plan, nil
}
func (s strategyPluginStub) NextAction(ctx context.Context, input AgentActionContext) (AgentActionDecision, error) {
	if s.actionFn != nil {
		return s.actionFn(ctx, input)
	}
	return s.action, nil
}
func (s strategyPluginStub) Observe(ctx context.Context, input AgentObservationContext) (AgentObservationDecision, error) {
	if s.observeFn != nil {
		return s.observeFn(ctx, input)
	}
	return s.observe, nil
}

type strategyAwareModelStub struct {
	plan            domain.Plan
	action          domain.Action
	observation     domain.Observation
	createPlanCalls int
	nextActionCalls int
	observeCalls    int
}

func (s *strategyAwareModelStub) CreatePlan(context.Context, domain.Task, domain.Session, []domain.MemoryEntry) (domain.Plan, error) {
	s.createPlanCalls++
	return s.plan, nil
}
func (s *strategyAwareModelStub) NextAction(context.Context, domain.Task, domain.Session, domain.Plan, []domain.Step, []domain.MemoryEntry, []domain.ToolInfo) (domain.Action, error) {
	s.nextActionCalls++
	return s.action, nil
}
func (s *strategyAwareModelStub) Observe(context.Context, domain.Task, domain.Session, domain.Plan, domain.Action, *domain.ToolResult) (domain.Observation, error) {
	s.observeCalls++
	return s.observation, nil
}

type agentStrategyToolExecutorStub struct {
	infos  []domain.ToolInfo
	result domain.ToolResult
}

func (s agentStrategyToolExecutorStub) Execute(context.Context, domain.ToolCall) (domain.ToolResult, error) {
	return s.result, nil
}
func (s agentStrategyToolExecutorStub) Registry() toolInfoLister {
	if s.infos == nil {
		return nil
	}
	return toolInfoListerStub{infos: s.infos}
}

type agentStrategyMemoryStoreStub struct{}

func (agentStrategyMemoryStoreStub) Remember(context.Context, string, domain.Observation) error { return nil }
func (agentStrategyMemoryStoreStub) Recall(context.Context, domain.RecallQuery) ([]domain.MemoryEntry, error) {
	return nil, nil
}

type agentStrategySessionStoreStub struct {
	savedSessions []domain.Session
	savedPlans    []domain.Plan
	steps         []domain.Step
}

func (s *agentStrategySessionStoreStub) SaveSession(_ context.Context, session domain.Session) error {
	s.savedSessions = append(s.savedSessions, session)
	return nil
}
func (s *agentStrategySessionStoreStub) SavePlan(_ context.Context, plan domain.Plan) error {
	s.savedPlans = append(s.savedPlans, plan)
	return nil
}
func (s *agentStrategySessionStoreStub) AppendStep(_ context.Context, _ string, step domain.Step) error {
	s.steps = append(s.steps, step)
	return nil
}

type agentStrategyVerifierStub struct{ result domain.VerificationResult }

func (s agentStrategyVerifierStub) Verify(context.Context, domain.Task, domain.Session, domain.Plan, []domain.Step, domain.Observation) (domain.VerificationResult, error) {
	return s.result, nil
}
