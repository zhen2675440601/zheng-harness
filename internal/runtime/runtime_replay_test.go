package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/store"
	"zheng-harness/internal/tools"
	"zheng-harness/internal/verify"
)

type replayFixture struct {
	Name    string            `json:"name"`
	Clock   string            `json:"clock"`
	Task    replayTask        `json:"task"`
	Engine  replayEngine      `json:"engine"`
	Store   replayStore       `json:"store"`
	Tool    replayTool        `json:"tool"`
	Verifier replayVerifier   `json:"verifier"`
	Plans   []replayPlan      `json:"plans"`
	Actions []replayAction    `json:"actions"`
	Results []replayResult    `json:"tool_results"`
	Obs     []replayObservation `json:"observations"`
	Verify  []replayVerify    `json:"verifications"`
}

type replayTask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Goal        string `json:"goal"`
	Category    string `json:"category"`
}

type replayEngine struct {
	MaxSteps         int `json:"max_steps"`
	MaxRetries       int `json:"max_retries"`
	SessionTimeoutMS int `json:"session_timeout_ms"`
}

type replayStore struct {
	Kind string `json:"kind"`
}

type replayTool struct {
	Kind string `json:"kind"`
}

type replayVerifier struct {
	Kind string `json:"kind"`
}

type replayPlan struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

type replayAction struct {
	Type          string `json:"type"`
	Summary       string `json:"summary"`
	Response      string `json:"response"`
	ToolName      string `json:"tool_name"`
	ToolInput     string `json:"tool_input"`
	ToolTimeoutMS int    `json:"tool_timeout_ms"`
}

type replayResult struct {
	ToolName   string `json:"tool_name"`
	Output     string `json:"output"`
	Error      string `json:"error"`
	DurationMS int    `json:"duration_ms"`
}

type replayObservation struct {
	Summary       string `json:"summary"`
	FinalResponse string `json:"final_response"`
	Evidence      *replayEvidence `json:"evidence"`
}

type replayVerify struct {
	Passed bool   `json:"passed"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type replayEvidence struct {
	Research     *replayResearchEvidence     `json:"research"`
	FileWorkflow *replayFileWorkflowEvidence `json:"file_workflow"`
}

type replayResearchEvidence struct {
	Sources    []replayEvidenceSource  `json:"sources"`
	Findings   []replayEvidenceFinding `json:"findings"`
	Conclusion string                  `json:"conclusion"`
}

type replayEvidenceSource struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Locator string `json:"locator"`
	Excerpt string `json:"excerpt"`
}

type replayEvidenceFinding struct {
	Claim               string   `json:"claim"`
	SupportingSourceIDs []string `json:"supporting_source_ids"`
}

type replayFileWorkflowEvidence struct {
	Expectations []replayFileExpectation `json:"expectations"`
	Results      []replayFileResult      `json:"results"`
	Summary      string                  `json:"summary"`
}

type replayFileExpectation struct {
	Path             string   `json:"path"`
	ShouldExist      bool     `json:"should_exist"`
	RequiredContents []string `json:"required_contents"`
}

type replayFileResult struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

func TestRuntimeReplaySuccessFixture(t *testing.T) {
	t.Parallel()

	fixture := loadReplayFixture(t, "success_session.json")
	engine, sessions, memory, task := newReplayEngine(t, fixture)

	session, plan, steps, err := engine.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusSuccess)
	}
	if plan.ID != fixture.Plans[0].ID {
		t.Fatalf("plan id = %q, want %q", plan.ID, fixture.Plans[0].ID)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if got := steps[0].Observation.ToolResult.Output; got != fixture.Results[0].Output {
		t.Fatalf("tool output = %q, want fixture output", got)
	}
	if len(sessions.savedPlans) != 1 {
		t.Fatalf("saved plans = %d, want 1", len(sessions.savedPlans))
	}
	if len(memory.remembered) != 1 {
		t.Fatalf("remembered observations = %d, want 1", len(memory.remembered))
	}
	if steps[0].Verification.Reason != fixture.Verify[0].Reason {
		t.Fatalf("verification reason = %q, want %q", steps[0].Verification.Reason, fixture.Verify[0].Reason)
	}
	if got := steps[0].Verification.Status; got != domain.VerificationStatusPassed {
		t.Fatalf("verification status = %q, want %q", got, domain.VerificationStatusPassed)
	}
	if got := task.Category; got != domain.TaskCategoryCoding {
		t.Fatalf("task category = %q, want %q", got, domain.TaskCategoryCoding)
	}
	if strings.Contains(strings.ToLower(steps[0].Observation.ToolResult.Output), "not allowlisted") {
		t.Fatal("success fixture unexpectedly exercised unsafe tool rejection")
	}
}

func TestRuntimeReplayVerificationFailureFixture(t *testing.T) {
	t.Parallel()

	fixture := loadReplayFixture(t, "verification_reject.json")
	engine, sessions, _, task := newReplayEngine(t, fixture)

	session, plan, steps, err := engine.Run(context.Background(), task)
	if err == nil {
		t.Fatal("expected retry budget error")
	}
	if got := err.Error(); got != "runtime retry budget exceeded" {
		t.Fatalf("error = %q, want retry budget exceeded", got)
	}
	if session.Status != domain.SessionStatusVerificationFailed {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusVerificationFailed)
	}
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	if plan.ID != fixture.Plans[1].ID {
		t.Fatalf("final plan id = %q, want %q", plan.ID, fixture.Plans[1].ID)
	}
	if !strings.Contains(steps[len(steps)-1].Observation.FinalResponse, "done") {
		t.Fatalf("final response = %q, want erroneous completion claim", steps[len(steps)-1].Observation.FinalResponse)
	}
	if got := steps[len(steps)-1].Verification.Reason; got != fixture.Verify[1].Reason {
		t.Fatalf("verification reason = %q, want %q", got, fixture.Verify[1].Reason)
	}
	if got := steps[0].Verification.Status; got != domain.VerificationStatusFailed {
		t.Fatalf("first verification status = %q, want %q", got, domain.VerificationStatusFailed)
	}
	if got := steps[len(steps)-1].Verification.Status; got != domain.VerificationStatusFailed {
		t.Fatalf("final verification status = %q, want %q", got, domain.VerificationStatusFailed)
	}
	if got := task.Category; got != domain.TaskCategoryCoding {
		t.Fatalf("task category = %q, want %q", got, domain.TaskCategoryCoding)
	}
	if final := sessions.savedSessions[len(sessions.savedSessions)-1].Status; final != domain.SessionStatusVerificationFailed {
		t.Fatalf("final saved status = %q, want verification_failed", final)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("verification failure fixture should fail by retry budget, not timeout")
	}
}

func TestRuntimeReplayResumeFixture(t *testing.T) {
	t.Parallel()

	fixture := loadReplayFixture(t, "resume_session.json")
	engine, sessionStore, memoryStore, task := newSQLiteReplayEngine(t, fixture)

	session, plan, steps, err := engine.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusSuccess)
	}
	if got := task.Category; got != domain.TaskCategoryCoding {
		t.Fatalf("task category = %q, want %q", got, domain.TaskCategoryCoding)
	}

	resumedSession, resumedPlan, resumedSteps, err := sessionStore.ResumeSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumedSession != session {
		t.Fatalf("resumed session mismatch: got %#v want %#v", resumedSession, session)
	}
	if !reflect.DeepEqual(resumedPlan, plan) {
		t.Fatalf("resumed plan mismatch: got %#v want %#v", resumedPlan, plan)
	}
	if !reflect.DeepEqual(resumedSteps, steps) {
		t.Fatalf("resumed steps mismatch: got %#v want %#v", resumedSteps, steps)
	}
	if _, _, _, err := sessionStore.ResumeSession(context.Background(), "missing-session"); err == nil {
		t.Fatal("expected missing session resume failure")
	}
	if got := resumedSteps[0].Verification.Status; got != domain.VerificationStatusPassed {
		t.Fatalf("resumed verification status = %q, want %q", got, domain.VerificationStatusPassed)
	}
	if err := memoryStore.Remember(context.Background(), session.ID, domain.Observation{Summary: "resume check only"}); err != nil {
		t.Fatalf("Remember(): %v", err)
	}
}

func TestRuntimeReplayResearchFixture(t *testing.T) {
	t.Parallel()

	fixture := loadReplayFixture(t, "research_session.json")
	engine, sessions, memory, task := newReplayEngine(t, fixture)

	session, plan, steps, err := engine.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusSuccess)
	}
	if task.Category != domain.TaskCategoryResearch {
		t.Fatalf("task category = %q, want %q", task.Category, domain.TaskCategoryResearch)
	}
	if plan.ID != fixture.Plans[0].ID {
		t.Fatalf("plan id = %q, want %q", plan.ID, fixture.Plans[0].ID)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if steps[0].Observation.ToolResult != nil {
		t.Fatal("research fixture should not execute a tool")
	}
	if steps[0].Verification.Status != domain.VerificationStatusPassed {
		t.Fatalf("verification status = %q, want %q", steps[0].Verification.Status, domain.VerificationStatusPassed)
	}
	if steps[0].Observation.Evidence == nil || steps[0].Observation.Evidence.Research == nil {
		t.Fatal("expected research evidence in observation")
	}
	if got := steps[0].Observation.Evidence.Research.Conclusion; got != "All reviewed sources agree the protocol supports deterministic research tasks." {
		t.Fatalf("research conclusion = %q, want fixture conclusion", got)
	}
	if len(steps[0].Observation.Evidence.Research.Sources) != 2 {
		t.Fatalf("research sources = %d, want 2", len(steps[0].Observation.Evidence.Research.Sources))
	}
	if got := steps[0].Observation.Evidence.Research.Findings[1].SupportingSourceIDs[0]; got != "src-2" {
		t.Fatalf("second finding support = %q, want src-2", got)
	}
	if got := steps[0].Verification.Reason; got != fixture.Verify[0].Reason {
		t.Fatalf("verification reason = %q, want %q", got, fixture.Verify[0].Reason)
	}
	if len(sessions.savedPlans) != 1 {
		t.Fatalf("saved plans = %d, want 1", len(sessions.savedPlans))
	}
	if len(memory.remembered) != 1 {
		t.Fatalf("remembered observations = %d, want 1", len(memory.remembered))
	}
	if got := sessions.savedSessions[len(sessions.savedSessions)-1].Status; got != domain.SessionStatusSuccess {
		t.Fatalf("final saved status = %q, want success", got)
	}
	if strings.Contains(strings.ToLower(steps[0].Observation.FinalResponse), "tool") {
		t.Fatalf("final response = %q, want evidence-focused completion", steps[0].Observation.FinalResponse)
	}
}

func TestRuntimeReplayFileWorkflowFixture(t *testing.T) {
	t.Parallel()

	fixture := loadReplayFixture(t, "file_workflow_session.json")
	engine, sessions, memory, task := newReplayEngine(t, fixture)

	session, plan, steps, err := engine.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusSuccess)
	}
	if task.Category != domain.TaskCategoryFileWorkflow {
		t.Fatalf("task category = %q, want %q", task.Category, domain.TaskCategoryFileWorkflow)
	}
	if plan.ID != fixture.Plans[0].ID {
		t.Fatalf("plan id = %q, want %q", plan.ID, fixture.Plans[0].ID)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if steps[0].Observation.ToolResult != nil {
		t.Fatal("file workflow fixture should not execute a tool")
	}
	if steps[0].Verification.Status != domain.VerificationStatusPassed {
		t.Fatalf("verification status = %q, want %q", steps[0].Verification.Status, domain.VerificationStatusPassed)
	}
	if steps[0].Observation.Evidence == nil || steps[0].Observation.Evidence.FileWorkflow == nil {
		t.Fatal("expected file workflow evidence in observation")
	}
	if got := steps[0].Observation.Evidence.FileWorkflow.Results[0].Content; !strings.Contains(got, "status: complete") {
		t.Fatalf("file workflow result content = %q, want completion marker", got)
	}
	if len(steps[0].Observation.Evidence.FileWorkflow.Expectations) != 2 {
		t.Fatalf("file workflow expectations = %d, want 2", len(steps[0].Observation.Evidence.FileWorkflow.Expectations))
	}
	if got := steps[0].Observation.Evidence.FileWorkflow.Expectations[0].RequiredContents[1]; got != "owner: runtime" {
		t.Fatalf("file workflow required content = %q, want owner marker", got)
	}
	if got := steps[0].Observation.Evidence.FileWorkflow.Results[1].Exists; got {
		t.Fatal("archive result should record missing file")
	}
	if got := steps[0].Verification.Reason; got != fixture.Verify[0].Reason {
		t.Fatalf("verification reason = %q, want %q", got, fixture.Verify[0].Reason)
	}
	if len(sessions.savedPlans) != 1 {
		t.Fatalf("saved plans = %d, want 1", len(sessions.savedPlans))
	}
	if len(memory.remembered) != 1 {
		t.Fatalf("remembered observations = %d, want 1", len(memory.remembered))
	}
	if got := sessions.savedSessions[len(sessions.savedSessions)-1].Status; got != domain.SessionStatusSuccess {
		t.Fatalf("final saved status = %q, want success", got)
	}
}

func TestRuntimeReplayUnsafeToolRejectionFixture(t *testing.T) {
	t.Parallel()

	fixture := loadReplayFixture(t, "unsafe_tool_rejection.json")
	engine, sessions, _, task := newReplayEngine(t, fixture)

	session, _, steps, err := engine.Run(context.Background(), task)
	if err == nil {
		t.Fatal("expected verification failure after unsafe tool rejection")
	}
	if got := err.Error(); got != "runtime retry budget exceeded" {
		t.Fatalf("error = %q, want retry budget exceeded", got)
	}
	if session.Status != domain.SessionStatusVerificationFailed {
		t.Fatalf("status = %q, want %q", session.Status, domain.SessionStatusVerificationFailed)
	}
	if got := task.Category; got != domain.TaskCategoryCoding {
		t.Fatalf("task category = %q, want %q", got, domain.TaskCategoryCoding)
	}
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	for _, step := range steps {
		if step.Observation.ToolResult == nil {
			t.Fatal("expected tool result to capture safety rejection")
		}
		if !strings.Contains(step.Observation.ToolResult.Error, "not allowlisted") {
			t.Fatalf("tool error = %q, want allowlist rejection", step.Observation.ToolResult.Error)
		}
		if step.Verification.Status != domain.VerificationStatusFailed {
			t.Fatalf("verification status = %q, want %q", step.Verification.Status, domain.VerificationStatusFailed)
		}
	}
	if final := sessions.savedSessions[len(sessions.savedSessions)-1].Status; final != domain.SessionStatusVerificationFailed {
		t.Fatalf("final saved status = %q, want verification_failed", final)
	}
	if steps[0].Action.ToolCall == nil || steps[0].Action.ToolCall.Name != "exec_command" {
		t.Fatalf("tool call = %#v, want exec_command", steps[0].Action.ToolCall)
	}
	if !strings.Contains(steps[0].Observation.ToolResult.Error, `command "powershell" is not allowlisted`) {
		t.Fatalf("tool error = %q, want deterministic powershell rejection", steps[0].Observation.ToolResult.Error)
	}
}

func BenchmarkRuntimeReplayFixtures(b *testing.B) {
	fixtures := []string{
		"success_session.json",
		"research_session.json",
		"file_workflow_session.json",
		"verification_reject.json",
		"resume_session.json",
		"unsafe_tool_rejection.json",
	}

	for _, name := range fixtures {
		fixture := loadReplayFixtureForBenchmark(b, name)
		b.Run(fixture.Name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if fixture.Store.Kind == "sqlite" {
					engine, sessionStore, _, task := newSQLiteReplayEngineForBenchmark(b, fixture)
					session, _, _, err := engine.Run(context.Background(), task)
					if name == "resume_session.json" {
						if err != nil {
							b.Fatalf("run: %v", err)
						}
						if _, _, _, err := sessionStore.ResumeSession(context.Background(), session.ID); err != nil {
							b.Fatalf("resume: %v", err)
						}
						continue
					}
					if err == nil {
						b.Fatalf("expected non-nil error for %s", fixture.Name)
					}
					continue
				}

				engine, _, _, task := newReplayEngineForBenchmark(b, fixture)
				_, _, _, err := engine.Run(context.Background(), task)
				switch name {
				case "success_session.json", "research_session.json", "file_workflow_session.json":
					if err != nil {
						b.Fatalf("run: %v", err)
					}
				default:
					if err == nil {
						b.Fatalf("expected non-nil error for %s", fixture.Name)
					}
				}
			}
		})
	}
}

func loadReplayFixture(t testing.TB, name string) replayFixture {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "runtime", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	var fixture replayFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %q: %v", name, err)
	}
	return fixture
}

func loadReplayFixtureForBenchmark(b *testing.B, name string) replayFixture {
	b.Helper()
	path := filepath.Join("..", "..", "testdata", "runtime", name)
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("read fixture %q: %v", name, err)
	}
	var fixture replayFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		b.Fatalf("unmarshal fixture %q: %v", name, err)
	}
	return fixture
}

func newReplayEngine(t *testing.T, fixture replayFixture) (runtime.Engine, *fakeSessionStore, *fakeMemoryStore, domain.Task) {
	t.Helper()
	return buildReplayEngine(t, fixture)
}

func newReplayEngineForBenchmark(b *testing.B, fixture replayFixture) (runtime.Engine, *fakeSessionStore, *fakeMemoryStore, domain.Task) {
	b.Helper()
	return buildReplayEngine(b, fixture)
}

func buildReplayEngine(tb testing.TB, fixture replayFixture) (runtime.Engine, *fakeSessionStore, *fakeMemoryStore, domain.Task) {
	tb.Helper()
	task, now := fixtureTaskAndClock(tb, fixture)
	model := &fakeModel{
		plans:        fixturePlans(tb, fixture),
		actions:      fixtureActions(tb, fixture),
		observations: fixtureObservations(fixture),
	}
	toolsExecutor := fixtureToolExecutor(tb, fixture)
	sessions := &fakeSessionStore{}
	memory := &fakeMemoryStore{}
	verifier := fixtureVerifier(tb, fixture, toolsExecutor)

	engine := runtime.Engine{
		Model:          model,
		Tools:          toolsExecutor,
		Memory:         memory,
		Sessions:       sessions,
		Verifier:       verifier,
		Clock:          fixedClock(now),
		MaxSteps:       fixture.Engine.MaxSteps,
		MaxRetries:     fixture.Engine.MaxRetries,
		SessionTimeout: time.Duration(fixture.Engine.SessionTimeoutMS) * time.Millisecond,
	}
	return engine, sessions, memory, task
}

func newSQLiteReplayEngine(t *testing.T, fixture replayFixture) (runtime.Engine, *store.SQLiteSessionStore, *store.SQLiteMemoryStore, domain.Task) {
	t.Helper()
	return buildSQLiteReplayEngine(t, fixture)
}

func newSQLiteReplayEngineForBenchmark(b *testing.B, fixture replayFixture) (runtime.Engine, *store.SQLiteSessionStore, *store.SQLiteMemoryStore, domain.Task) {
	b.Helper()
	return buildSQLiteReplayEngine(b, fixture)
}

func buildSQLiteReplayEngine(tb testing.TB, fixture replayFixture) (runtime.Engine, *store.SQLiteSessionStore, *store.SQLiteMemoryStore, domain.Task) {
	tb.Helper()
	task, now := fixtureTaskAndClock(tb, fixture)
	dbPath := filepath.Join(tb.TempDir(), "runtime-replay.db")
	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		tb.Fatalf("NewSQLiteSessionStore(): %v", err)
	}
	tb.Cleanup(func() { _ = sessionStore.Close() })
	memoryStore, err := store.NewMemoryStore(dbPath)
	if err != nil {
		tb.Fatalf("NewMemoryStore(): %v", err)
	}
	tb.Cleanup(func() { _ = memoryStore.Close() })
	toolsExecutor := fixtureToolExecutor(tb, fixture)

	engine := runtime.Engine{
		Model:          &fakeModel{plans: fixturePlans(tb, fixture), actions: fixtureActions(tb, fixture), observations: fixtureObservations(fixture)},
		Tools:          toolsExecutor,
		Memory:         memoryStore,
		Sessions:       sessionStore,
		Verifier:       fixtureVerifier(tb, fixture, toolsExecutor),
		Clock:          fixedClock(now),
		MaxSteps:       fixture.Engine.MaxSteps,
		MaxRetries:     fixture.Engine.MaxRetries,
		SessionTimeout: time.Duration(fixture.Engine.SessionTimeoutMS) * time.Millisecond,
	}
	return engine, sessionStore, memoryStore, task
}

func fixtureTaskAndClock(tb testing.TB, fixture replayFixture) (domain.Task, time.Time) {
	tb.Helper()
	now, err := time.Parse(time.RFC3339, fixture.Clock)
	if err != nil {
		tb.Fatalf("parse fixture clock: %v", err)
	}
	return domain.Task{
		ID:          fixture.Task.ID,
		Description: fixture.Task.Description,
		Goal:        fixture.Task.Goal,
		Category:    domain.TaskCategory(fixture.Task.Category),
		CreatedAt:   now,
	}, now
}

func fixturePlans(tb testing.TB, fixture replayFixture) []domain.Plan {
	tb.Helper()
	now, err := time.Parse(time.RFC3339, fixture.Clock)
	if err != nil {
		tb.Fatalf("parse fixture clock: %v", err)
	}
	plans := make([]domain.Plan, 0, len(fixture.Plans))
	for _, plan := range fixture.Plans {
		plans = append(plans, domain.Plan{ID: plan.ID, TaskID: fixture.Task.ID, Summary: plan.Summary, CreatedAt: now})
	}
	return plans
}

func fixtureActions(tb testing.TB, fixture replayFixture) []domain.Action {
	tb.Helper()
	actions := make([]domain.Action, 0, len(fixture.Actions))
	for _, action := range fixture.Actions {
		item := domain.Action{Type: domain.ActionType(action.Type), Summary: action.Summary, Response: action.Response}
		if action.ToolName != "" {
			item.ToolCall = &domain.ToolCall{
				Name:    action.ToolName,
				Input:   action.ToolInput,
				Timeout: time.Duration(action.ToolTimeoutMS) * time.Millisecond,
			}
		}
		actions = append(actions, item)
	}
	return actions
}

func fixtureObservations(fixture replayFixture) []domain.Observation {
	observations := make([]domain.Observation, 0, len(fixture.Obs))
	for _, observation := range fixture.Obs {
		observations = append(observations, domain.Observation{
			Summary:       observation.Summary,
			FinalResponse: observation.FinalResponse,
			Evidence:      fixtureEvidence(observation.Evidence),
		})
	}
	return observations
}

func fixtureEvidence(evidence *replayEvidence) *domain.Evidence {
	if evidence == nil {
		return nil
	}
	result := &domain.Evidence{}
	if evidence.Research != nil {
		result.Research = &domain.ResearchEvidence{
			Conclusion: evidence.Research.Conclusion,
			Sources:    fixtureResearchSources(evidence.Research.Sources),
			Findings:   fixtureResearchFindings(evidence.Research.Findings),
		}
	}
	if evidence.FileWorkflow != nil {
		result.FileWorkflow = &domain.FileWorkflowEvidence{
			Summary:      evidence.FileWorkflow.Summary,
			Expectations: fixtureFileExpectations(evidence.FileWorkflow.Expectations),
			Results:      fixtureFileResults(evidence.FileWorkflow.Results),
		}
	}
	if result.Research == nil && result.FileWorkflow == nil {
		return nil
	}
	return result
}

func fixtureResearchSources(items []replayEvidenceSource) []domain.EvidenceSource {
	result := make([]domain.EvidenceSource, 0, len(items))
	for _, item := range items {
		result = append(result, domain.EvidenceSource{ID: item.ID, Kind: item.Kind, Locator: item.Locator, Excerpt: item.Excerpt})
	}
	return result
}

func fixtureResearchFindings(items []replayEvidenceFinding) []domain.EvidenceFinding {
	result := make([]domain.EvidenceFinding, 0, len(items))
	for _, item := range items {
		result = append(result, domain.EvidenceFinding{Claim: item.Claim, SupportingSourceIDs: append([]string(nil), item.SupportingSourceIDs...)})
	}
	return result
}

func fixtureFileExpectations(items []replayFileExpectation) []domain.FileExpectation {
	result := make([]domain.FileExpectation, 0, len(items))
	for _, item := range items {
		result = append(result, domain.FileExpectation{Path: item.Path, ShouldExist: item.ShouldExist, RequiredContents: append([]string(nil), item.RequiredContents...)})
	}
	return result
}

func fixtureFileResults(items []replayFileResult) []domain.FileResult {
	result := make([]domain.FileResult, 0, len(items))
	for _, item := range items {
		result = append(result, domain.FileResult{Path: item.Path, Exists: item.Exists, Content: item.Content, Error: item.Error})
	}
	return result
}

func fixtureVerifications(fixture replayFixture) []domain.VerificationResult {
	results := make([]domain.VerificationResult, 0, len(fixture.Verify))
	for _, verification := range fixture.Verify {
		status := domain.VerificationStatus(strings.TrimSpace(verification.Status))
		results = append(results, domain.VerificationResult{Passed: verification.Passed, Status: status, Reason: verification.Reason}.Normalize())
	}
	return results
}

func fixtureToolExecutor(tb testing.TB, fixture replayFixture) *fakeToolExecutor {
	tb.Helper()
	switch fixture.Tool.Kind {
	case "", "fake":
		results := make([]domain.ToolResult, 0, len(fixture.Results))
		errs := make([]error, 0, len(fixture.Results))
		for _, result := range fixture.Results {
			results = append(results, domain.ToolResult{
				ToolName: result.ToolName,
				Output:   result.Output,
				Error:    result.Error,
				Duration: time.Duration(result.DurationMS) * time.Millisecond,
			})
			if result.Error != "" {
				errs = append(errs, errors.New(result.Error))
			} else {
				errs = append(errs, nil)
			}
		}
		return &fakeToolExecutor{results: results, errs: errs}
	case "real_executor":
		workspace := tb.TempDir()
		realExecutor, err := tools.NewExecutor(workspace)
		if err != nil {
			tb.Fatalf("NewExecutor(): %v", err)
		}
		return &fakeToolExecutor{executeFn: func(ctx context.Context, call domain.ToolCall, _ int) (domain.ToolResult, error) {
			return realExecutor.Execute(ctx, call)
		}}
	default:
		tb.Fatalf("unsupported tool kind %q", fixture.Tool.Kind)
		return nil
	}
}

func fixtureVerifier(tb testing.TB, fixture replayFixture, executor domain.ToolExecutor) domain.Verifier {
	tb.Helper()
	switch fixture.Verifier.Kind {
	case "", "fake":
		return &fakeVerifier{results: fixtureVerifications(fixture)}
	case "task_aware":
		return runtimeTaskAwareVerifier(executor)
	default:
		tb.Fatalf("unsupported verifier kind %q", fixture.Verifier.Kind)
		return nil
	}
}

func runtimeTaskAwareVerifier(executor domain.ToolExecutor) domain.Verifier {
	return verify.NewTaskAwareVerifier("standard", executor)
}
