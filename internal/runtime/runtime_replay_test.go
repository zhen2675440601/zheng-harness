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
)

type replayFixture struct {
	Name    string            `json:"name"`
	Clock   string            `json:"clock"`
	Task    replayTask        `json:"task"`
	Engine  replayEngine      `json:"engine"`
	Store   replayStore       `json:"store"`
	Tool    replayTool        `json:"tool"`
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
}

type replayVerify struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
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
	if err := memoryStore.Remember(context.Background(), session.ID, domain.Observation{Summary: "resume check only"}); err != nil {
		t.Fatalf("Remember(): %v", err)
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
	}
	if final := sessions.savedSessions[len(sessions.savedSessions)-1].Status; final != domain.SessionStatusVerificationFailed {
		t.Fatalf("final saved status = %q, want verification_failed", final)
	}
	if steps[0].Action.ToolCall == nil || steps[0].Action.ToolCall.Name != "exec_command" {
		t.Fatalf("tool call = %#v, want exec_command", steps[0].Action.ToolCall)
	}
}

func BenchmarkRuntimeReplayFixtures(b *testing.B) {
	fixtures := []string{
		"success_session.json",
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
				case "success_session.json":
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
	verifier := &fakeVerifier{results: fixtureVerifications(fixture)}

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

	engine := runtime.Engine{
		Model:          &fakeModel{plans: fixturePlans(tb, fixture), actions: fixtureActions(tb, fixture), observations: fixtureObservations(fixture)},
		Tools:          fixtureToolExecutor(tb, fixture),
		Memory:         memoryStore,
		Sessions:       sessionStore,
		Verifier:       &fakeVerifier{results: fixtureVerifications(fixture)},
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
		observations = append(observations, domain.Observation{Summary: observation.Summary, FinalResponse: observation.FinalResponse})
	}
	return observations
}

func fixtureVerifications(fixture replayFixture) []domain.VerificationResult {
	results := make([]domain.VerificationResult, 0, len(fixture.Verify))
	for _, verification := range fixture.Verify {
		results = append(results, domain.VerificationResult{Passed: verification.Passed, Reason: verification.Reason})
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
