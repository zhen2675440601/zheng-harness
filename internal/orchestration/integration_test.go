package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	runtimepkg "runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	pluginruntime "zheng-harness/internal/plugin"
	runtimeengine "zheng-harness/internal/runtime"
	"zheng-harness/internal/tools"
)

func TestIntegrationFullFlow(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "sample.go"), []byte("package sample\n\nfunc target() string {\n\treturn \"integration\"\n}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(sample.go): %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "web integration payload")
	}))
	defer server.Close()

	pluginPath := buildIntegrationEchoPluginBinary(t)
	executor, closeExecutor := newIntegrationExecutor(t, workspace, pluginPath)
	defer closeExecutor()

	provider := newIntegrationProvider(map[string]integrationScenario{
		"full integration flow --decompose --stream --plugin": {
			planSummary: "decompose streamed task and use built-in plus plugin tools",
			actions: []integrationActionStep{
				{
					actionJSON: fmt.Sprintf(`{"type":"tool_call","summary":"fetch remote context","tool_call":{"name":"web_fetch","input":%s,"timeout":"2s"}}`, mustJSONString(t, fmt.Sprintf(`{"url":%q}`, server.URL))),
					observationJSON: `{"summary":"fetched remote context"}`,
				},
				{
					actionJSON: fmt.Sprintf(`{"type":"tool_call","summary":"search source for target","tool_call":{"name":"code_search","input":%s,"timeout":"2s"}}`, mustJSONString(t, `{"pattern":"target","language":"go","output_mode":"content","max_results":5}`)),
					observationJSON: `{"summary":"searched code successfully"}`,
				},
				{
					actionJSON: fmt.Sprintf(`{"type":"tool_call","summary":"invoke plugin echo","tool_call":{"name":"echo","input":%s,"timeout":"2s"}}`, mustJSONString(t, `hello from plugin`)),
					observationJSON: `{"summary":"plugin responded"}`,
				},
				{
					actionJSON: `{"type":"complete","summary":"integration complete","response":"streaming tools plugins all integrated"}`,
					observationJSON: `{"summary":"integration complete","final_response":"streaming tools plugins all integrated"}`,
				},
			},
		},
	})

	stores := newIntegrationStores()
	engine := runtimeengine.Engine{
		Model:          runtimeengine.NewModelAdapter(provider),
		Tools:          executor,
		Memory:         stores,
		Sessions:       stores,
		Verifier:       integrationVerifier{},
		Clock:          integrationFixedClock(),
		MaxSteps:       4,
		MaxRetries:     4,
		SessionTimeout: time.Minute,
	}

	task := domain.Task{
		ID:          "integration-full-flow",
		Description: "full integration flow --decompose --stream --plugin",
		Goal:        "verify streaming plus tools plus plugins together",
		Category:    domain.TaskCategoryResearch,
		CreatedAt:   integrationFixedClock()(),
	}

	events, _, _, _, err := engine.RunStream(context.Background(), task)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}

	received := collectIntegrationEvents(events)
	if len(received) == 0 {
		t.Fatal("received no streaming events")
	}

	assertEventTypePresent(t, received, domain.EventTokenDelta)
	assertEventTypePresent(t, received, domain.EventToolStart)
	assertEventTypePresent(t, received, domain.EventToolEnd)
	assertEventTypePresent(t, received, domain.EventStepComplete)
	assertEventTypePresent(t, received, domain.EventSessionComplete)

	toolStarts := extractToolNames(t, received, domain.EventToolStart)
	if !equalStrings(toolStarts, []string{"web_fetch", "code_search", "echo"}) {
		t.Fatalf("tool start sequence = %v, want [web_fetch code_search echo]", toolStarts)
	}
	toolEnds := extractToolNames(t, received, domain.EventToolEnd)
	if !equalStrings(toolEnds, []string{"web_fetch", "code_search", "echo"}) {
		t.Fatalf("tool end sequence = %v, want [web_fetch code_search echo]", toolEnds)
	}

	if !provider.streamUsed {
		t.Fatal("provider Stream() was not used")
	}

	storedSession := stores.lastSession(task.ID)
	if storedSession.Status != domain.SessionStatusSuccess {
		t.Fatalf("stored session status = %q, want %q", storedSession.Status, domain.SessionStatusSuccess)
	}
	storedPlan := stores.lastPlan(task.ID)
	if !strings.Contains(storedPlan.Summary, "built-in plus plugin tools") {
		t.Fatalf("stored plan summary = %q", storedPlan.Summary)
	}
	storedSteps := stores.stepsFor(task.ID)
	if len(storedSteps) != 4 {
		t.Fatalf("stored steps = %d, want 4", len(storedSteps))
	}
	last := storedSteps[len(storedSteps)-1]
	if !last.Verification.Passed {
		t.Fatalf("last verification = %+v, want passed", last.Verification)
	}
	if got := strings.TrimSpace(last.Observation.FinalResponse); got != "streaming tools plugins all integrated" {
		t.Fatalf("final response = %q, want integration completion", got)
	}
	if !strings.Contains(storedSteps[0].Observation.ToolResult.Output, "web integration payload") {
		t.Fatalf("web_fetch output = %q, want server payload", storedSteps[0].Observation.ToolResult.Output)
	}
	if !strings.Contains(storedSteps[1].Observation.ToolResult.Output, "sample.go") {
		t.Fatalf("code_search output = %q, want sample.go match", storedSteps[1].Observation.ToolResult.Output)
	}
	if strings.TrimSpace(storedSteps[2].Observation.ToolResult.Output) != "hello from plugin" {
		t.Fatalf("plugin output = %q, want echo payload", storedSteps[2].Observation.ToolResult.Output)
	}
	toolsSeen := provider.toolsForTask(task.Description)
	for _, required := range []string{"web_fetch", "code_search", "echo"} {
		if !containsString(toolsSeen, required) {
			t.Fatalf("provider tools = %v, missing %q", toolsSeen, required)
		}
	}
	if len(stores.remembered) != 4 {
		t.Fatalf("remembered observations = %d, want 4", len(stores.remembered))
	}
}

func TestIntegrationMultiAgentWithPlugins(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "worker.go"), []byte("package worker\n\nfunc workerTarget() string {\n\treturn \"ok\"\n}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(worker.go): %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "multi-agent web payload")
	}))
	defer server.Close()

	pluginPath := buildIntegrationEchoPluginBinary(t)
	executor, closeExecutor := newIntegrationExecutor(t, workspace, pluginPath)
	defer closeExecutor()

	provider := newIntegrationProvider(map[string]integrationScenario{
		"collect remote context": {
			planSummary: "worker fetches remote context",
			actions: []integrationActionStep{
				{
					actionJSON: fmt.Sprintf(`{"type":"tool_call","summary":"fetch shared context","tool_call":{"name":"web_fetch","input":%s,"timeout":"2s"}}`, mustJSONString(t, fmt.Sprintf(`{"url":%q}`, server.URL))),
					observationJSON: `{"summary":"remote context fetched"}`,
				},
				{
					actionJSON: `{"type":"complete","summary":"worker finished","response":"remote context ready"}`,
					observationJSON: `{"summary":"worker finished","final_response":"remote context ready"}`,
				},
			},
		},
		"search code and summarize via plugin": {
			planSummary: "worker searches code then uses plugin",
			actions: []integrationActionStep{
				{
					actionJSON: fmt.Sprintf(`{"type":"tool_call","summary":"search worker code","tool_call":{"name":"code_search","input":%s,"timeout":"2s"}}`, mustJSONString(t, `{"pattern":"workerTarget","language":"go","output_mode":"files_with_matches","max_results":5}`)),
					observationJSON: `{"summary":"worker code searched"}`,
				},
				{
					actionJSON: fmt.Sprintf(`{"type":"tool_call","summary":"plugin summarize","tool_call":{"name":"echo","input":%s,"timeout":"2s"}}`, mustJSONString(t, `plugin summary`)),
					observationJSON: `{"summary":"plugin summary captured"}`,
				},
				{
					actionJSON: `{"type":"complete","summary":"worker finished","response":"plugin summary ready"}`,
					observationJSON: `{"summary":"worker finished","final_response":"plugin summary ready"}`,
				},
			},
		},
	})

	baseEngine := runtimeengine.Engine{
		Model:          runtimeengine.NewModelAdapter(provider),
		Tools:          executor,
		Memory:         newIntegrationStores(),
		Sessions:       newIntegrationStores(),
		Verifier:       integrationVerifier{},
		Clock:          integrationFixedClock(),
		MaxSteps:       3,
		MaxRetries:     3,
		SessionTimeout: time.Minute,
	}

	decomposition := TaskDecomposition{
		TaskID: "integration-multi-agent",
		Subtasks: []Subtask{
			{ID: "fetch", Description: "collect remote context", ExpectedOutput: "remote context ready", Status: SubtaskStatusPending},
			{ID: "summarize", Description: "search code and summarize via plugin", ExpectedOutput: "plugin summary ready", Dependencies: []string{"fetch"}, Status: SubtaskStatusPending},
		},
	}

	resultCh := make(chan WorkerResult, len(decomposition.Subtasks))
	workerStores := map[string]*integrationStores{
		"fetch":     newIntegrationStores(),
		"summarize": newIntegrationStores(),
	}

	orch := Orchestrator{
		MaxWorkers: 2,
		ResultChannel: resultCh,
		WorkerFactory: func(subtask Subtask) Worker {
			engine := baseEngine
			engine.Memory = workerStores[subtask.ID]
			engine.Sessions = workerStores[subtask.ID]
			return NewWorkerAgent(decomposition.TaskID, subtask, engine, resultCh)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := orch.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := orch.SubmitTask(ctx, decomposition); err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}
	orch.Stop()
	if err := orch.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	results := collectResults(resultCh)
	if len(results) != 2 {
		t.Fatalf("worker results = %d, want 2", len(results))
	}
	sort.Slice(results, func(i, j int) bool { return results[i].SubtaskID < results[j].SubtaskID })
	for _, result := range results {
		if result.Status != SubtaskStatusCompleted {
			t.Fatalf("result for %s status = %q, want %q", result.SubtaskID, result.Status, SubtaskStatusCompleted)
		}
		if result.Err != nil {
			t.Fatalf("result for %s err = %v, want nil", result.SubtaskID, result.Err)
		}
		if !result.VerificationPassed {
			t.Fatalf("result for %s verification failed", result.SubtaskID)
		}
	}

	aggregator := &Aggregator{Strategy: AggregationStrategyAllSucceed}
	taskResults := make([]TaskResult, 0, len(results))
	for _, result := range results {
		status := domain.VerificationStatusPassed
		if result.Err != nil {
			status = domain.VerificationStatusFailed
		}
		taskResults = append(taskResults, TaskResult{SubtaskID: result.SubtaskID, Output: result.Output, Error: result.Err, VerificationStatus: status})
	}
	aggregated, err := aggregator.Aggregate(taskResults)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}
	if aggregated.Status != AggregationStatusSucceeded {
		t.Fatalf("aggregate status = %q, want %q", aggregated.Status, AggregationStatusSucceeded)
	}
	if got := aggregated.Results["fetch"].Output; !strings.Contains(got, "remote context ready") {
		t.Fatalf("fetch aggregated output = %q", got)
	}
	if got := aggregated.Results["summarize"].Output; !strings.Contains(got, "plugin summary ready") {
		t.Fatalf("summarize aggregated output = %q", got)
	}
	if !provider.sawTask("collect remote context") || !provider.sawTask("search code and summarize via plugin") {
		t.Fatalf("provider did not execute both subtasks; seen=%v", provider.taskNames())
	}
	if got := workerStores["fetch"].stepsFor("fetch"); len(got) != 2 {
		t.Fatalf("fetch steps = %d, want 2", len(got))
	}
	if got := workerStores["summarize"].stepsFor("summarize"); len(got) != 3 {
		t.Fatalf("summarize steps = %d, want 3", len(got))
	}
	if !strings.Contains(workerStores["summarize"].stepsFor("summarize")[0].Observation.ToolResult.Output, "worker.go") {
		t.Fatalf("code search output = %q, want worker.go", workerStores["summarize"].stepsFor("summarize")[0].Observation.ToolResult.Output)
	}
	if strings.TrimSpace(workerStores["summarize"].stepsFor("summarize")[1].Observation.ToolResult.Output) != "plugin summary" {
		t.Fatalf("plugin tool output = %q, want plugin summary", workerStores["summarize"].stepsFor("summarize")[1].Observation.ToolResult.Output)
	}
	if len(orch.Workers) != 0 {
		t.Fatalf("workers still registered = %d, want 0", len(orch.Workers))
	}
}

type integrationScenario struct {
	planSummary string
	actions     []integrationActionStep
}

type integrationActionStep struct {
	actionJSON      string
	observationJSON string
}

type integrationProvider struct {
	mu         sync.Mutex
	scenarios  map[string]integrationScenario
	indices    map[string]int
	observes   map[string]int
	toolsSeen  map[string][]string
	tasksSeen  map[string]int
	streamUsed bool
}

func newIntegrationProvider(scenarios map[string]integrationScenario) *integrationProvider {
	return &integrationProvider{
		scenarios: scenarios,
		indices:   make(map[string]int),
		observes:  make(map[string]int),
		toolsSeen: make(map[string][]string),
		tasksSeen: make(map[string]int),
	}
}

func (p *integrationProvider) Name() string  { return "integration-provider" }
func (p *integrationProvider) Model() string { return "integration-model" }

func (p *integrationProvider) Generate(_ context.Context, request llm.Request) (llm.Response, error) {
	output, err := p.responseFor(request)
	if err != nil {
		return llm.Response{}, err
	}
	return llm.Response{Model: p.Model(), Output: output, StopReason: "done"}, nil
}

func (p *integrationProvider) Stream(ctx context.Context, request llm.Request, emit func(domain.StreamingEvent) error) error {
	p.mu.Lock()
	p.streamUsed = true
	p.mu.Unlock()
	response, err := p.Generate(ctx, request)
	if err != nil {
		return err
	}
	chunks := chunkString(response.Output, 18)
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		event, err := domain.TokenDelta(0, chunk)
		if err != nil {
			return err
		}
		if err := emit(*event); err != nil {
			return err
		}
	}
	complete, err := domain.SessionComplete("", string(domain.SessionStatusSuccess))
	if err != nil {
		return err
	}
	return emit(*complete)
	}

func (p *integrationProvider) responseFor(request llm.Request) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(request.Input), &payload); err != nil {
		return "", fmt.Errorf("decode prompt payload: %w", err)
	}
	operation, _ := payload["operation"].(string)
	taskPayload, _ := payload["task"].(map[string]any)
	taskDescription, _ := taskPayload["description"].(string)
	if taskDescription == "" {
		return "", errors.New("missing task description in prompt payload")
	}
	scenario, ok := p.scenarios[taskDescription]
	if !ok {
		return "", fmt.Errorf("unexpected task description %q", taskDescription)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.tasksSeen[taskDescription]++

	switch operation {
	case "create_plan":
		return fmt.Sprintf(`{"summary":%q,"steps":["use configured tools","return completion"]}`, scenario.planSummary), nil
	case "next_action":
		index := p.indices[taskDescription]
		if index >= len(scenario.actions) {
			return "", fmt.Errorf("next_action index %d out of range for %q", index, taskDescription)
		}
		p.indices[taskDescription] = index + 1
		var actionPayload struct {
			ToolCall *struct {
				Name string `json:"name"`
			} `json:"tool_call,omitempty"`
		}
		if err := json.Unmarshal([]byte(scenario.actions[index].actionJSON), &actionPayload); err == nil && actionPayload.ToolCall != nil {
			p.toolsSeen[taskDescription] = append(p.toolsSeen[taskDescription], actionPayload.ToolCall.Name)
		}
		return scenario.actions[index].actionJSON, nil
	case "observe":
		index := p.observes[taskDescription]
		if index >= len(scenario.actions) {
			return "", fmt.Errorf("observe index %d out of range for %q", index, taskDescription)
		}
		p.observes[taskDescription] = index + 1
		return scenario.actions[index].observationJSON, nil
	default:
		return "", fmt.Errorf("unexpected operation %q", operation)
	}
}

func (p *integrationProvider) toolsForTask(task string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.toolsSeen[task]...)
}

func (p *integrationProvider) sawTask(task string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tasksSeen[task] > 0
}

func (p *integrationProvider) taskNames() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	names := make([]string, 0, len(p.tasksSeen))
	for name := range p.tasksSeen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type integrationToolExecutor struct {
	base     *tools.Executor
	registry *tools.Registry
	plugins  map[string]pluginruntime.PluginTool
	closeFn  func() error
}

func newIntegrationExecutor(t *testing.T, workspaceRoot, pluginPath string) (*integrationToolExecutor, func()) {
	t.Helper()

	base, err := tools.NewExecutor(workspaceRoot)
	if err != nil {
		t.Fatalf("NewExecutor(): %v", err)
	}
	manager := pluginruntime.NewManager(filepath.Dir(pluginPath))
	tool, err := manager.Load(context.Background(), pluginPath)
	if err != nil {
		t.Fatalf("Load(plugin) error = %v", err)
	}
	registry := tools.NewRegistry()
	for _, def := range base.Registry().List() {
		if err := registry.Register(def); err != nil {
			t.Fatalf("Register(base tool %s): %v", def.Name, err)
		}
	}
	if err := registry.Register(toIntegrationToolDefinition(tool)); err != nil {
		t.Fatalf("Register(plugin tool): %v", err)
	}
	executor := &integrationToolExecutor{
		base:     base,
		registry: registry,
		plugins:  map[string]pluginruntime.PluginTool{tool.Name(): tool},
		closeFn:  manager.CloseAll,
	}
	return executor, func() {
		_ = executor.Close()
	}
}

func (e *integrationToolExecutor) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	if pluginTool, ok := e.plugins[call.Name]; ok {
		return pluginTool.Execute(ctx, call)
	}
	return e.base.Execute(ctx, call)
}

func (e *integrationToolExecutor) Registry() *tools.Registry {
	return e.registry
}

func (e *integrationToolExecutor) Close() error {
	if e == nil || e.closeFn == nil {
		return nil
	}
	return e.closeFn()
}

func toIntegrationToolDefinition(tool pluginruntime.PluginTool) tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:           tool.Name(),
		Description:    tool.Description(),
		Schema:         tool.Schema(),
		DefaultTimeout: 30 * time.Second,
		SafetyLevel:    tool.SafetyLevel(),
		Handler:        tool.Execute,
	}
}

type integrationStores struct {
	mu         sync.Mutex
	sessions   map[string]domain.Session
	plans      map[string]domain.Plan
	steps      map[string][]domain.Step
	remembered []domain.Observation
}

func newIntegrationStores() *integrationStores {
	return &integrationStores{
		sessions: make(map[string]domain.Session),
		plans:    make(map[string]domain.Plan),
		steps:    make(map[string][]domain.Step),
	}
}

func (s *integrationStores) Remember(_ context.Context, _ string, observation domain.Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remembered = append(s.remembered, observation)
	return nil
}

func (s *integrationStores) Recall(_ context.Context, _ domain.RecallQuery) ([]domain.MemoryEntry, error) {
	return nil, nil
}

func (s *integrationStores) SaveSession(_ context.Context, session domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.TaskID] = session
	if session.ID != "" {
		s.sessions[session.ID] = session
	}
	return nil
}

func (s *integrationStores) SavePlan(_ context.Context, plan domain.Plan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plans[plan.TaskID] = plan
	return nil
}

func (s *integrationStores) AppendStep(_ context.Context, sessionID string, step domain.Step) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionID
	if session, ok := s.sessions[sessionID]; ok && session.TaskID != "" {
		key = session.TaskID
	}
	s.steps[key] = append(s.steps[key], step)
	return nil
}

func (s *integrationStores) lastSession(key string) domain.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[key]
}

func (s *integrationStores) lastPlan(taskID string) domain.Plan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plans[taskID]
}

func (s *integrationStores) stepsFor(taskID string) []domain.Step {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.Step(nil), s.steps[taskID]...)
}

type integrationVerifier struct{}

func (integrationVerifier) Verify(_ context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	if strings.TrimSpace(observation.FinalResponse) != "" {
		return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "final response recorded"}, nil
	}
	return domain.VerificationResult{Passed: false, Status: domain.VerificationStatusFailed, Reason: "awaiting final response"}, nil
}

func integrationFixedClock() func() time.Time {
	return func() time.Time {
		return time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	}
}

func collectIntegrationEvents(channel *runtimeengine.EventChannel) []domain.StreamingEvent {
	collected := make([]domain.StreamingEvent, 0)
	for event := range channel.Events() {
		collected = append(collected, event)
	}
	return collected
}

func assertEventTypePresent(t *testing.T, events []domain.StreamingEvent, want domain.StreamingEventType) {
	t.Helper()
	for _, event := range events {
		if event.Type == want {
			return
		}
	}
	t.Fatalf("event type %q not found in %v", want, integrationEventTypes(events))
}

func extractToolNames(t *testing.T, events []domain.StreamingEvent, kind domain.StreamingEventType) []string {
	t.Helper()
	names := make([]string, 0)
	for _, event := range events {
		switch kind {
		case domain.EventToolStart:
			if event.Type != kind {
				continue
			}
			var payload domain.ToolStartPayload
			if err := event.GetPayload(&payload); err != nil {
				t.Fatalf("decode tool start payload: %v", err)
			}
			names = append(names, payload.ToolName)
		case domain.EventToolEnd:
			if event.Type != kind {
				continue
			}
			var payload domain.ToolEndPayload
			if err := event.GetPayload(&payload); err != nil {
				t.Fatalf("decode tool end payload: %v", err)
			}
			names = append(names, payload.ToolName)
		}
	}
	return names
}

func integrationEventTypes(events []domain.StreamingEvent) []domain.StreamingEventType {
	types := make([]domain.StreamingEventType, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func chunkString(value string, size int) []string {
	if size <= 0 || len(value) <= size {
		return []string{value}
	}
	chunks := make([]string, 0, (len(value)+size-1)/size)
	for start := 0; start < len(value); start += size {
		end := start + size
		if end > len(value) {
			end = len(value)
		}
		chunks = append(chunks, value[start:end])
	}
	return chunks
}

func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%q): %v", value, err)
	}
	return string(data)
}

var (
	buildIntegrationEchoPluginOnce sync.Once
	buildIntegrationEchoPluginPath string
	buildIntegrationEchoPluginErr  error
)

func buildIntegrationEchoPluginBinary(t *testing.T) string {
	t.Helper()

	buildIntegrationEchoPluginOnce.Do(func() {
		repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			buildIntegrationEchoPluginErr = err
			return
		}
		outputDir, err := os.MkdirTemp("", "integration-echo-plugin-*")
		if err != nil {
			buildIntegrationEchoPluginErr = err
			return
		}
		binaryName := "echo_plugin"
		if runtimepkg.GOOS == "windows" {
			binaryName += ".exe"
		}
		buildIntegrationEchoPluginPath = filepath.Join(outputDir, binaryName)
		cmd := exec.Command("go", "build", "-o", buildIntegrationEchoPluginPath, "./testdata/plugins/echo_plugin")
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildIntegrationEchoPluginErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
	})

	if buildIntegrationEchoPluginErr != nil {
		t.Fatalf("build echo plugin: %v", buildIntegrationEchoPluginErr)
	}
	return buildIntegrationEchoPluginPath
}
