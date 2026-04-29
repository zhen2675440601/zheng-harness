package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/store"
)

func TestCLIStreamFlagEnabled(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var runCalled int
	var runStreamCalled int

	app := newTestCLIApp(t, &stdout, &stderr, dbPath)
	app.runEngine = func(context.Context, runtime.Engine, domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
		runCalled++
		return domain.Session{Status: domain.SessionStatusSuccess}, domain.Plan{Summary: "non-stream plan"}, nil, nil
	}
	app.runStreamEngine = func(ctx context.Context, engine runtime.Engine, task domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
		runStreamCalled++
		return emitTestStream(ctx, engine, task, nil, domain.SessionStatusSuccess), domain.Session{}, domain.Plan{}, nil, nil
	}

	if exitCode := app.run(context.Background(), []string{"run", "--task", "stream task", "--db", dbPath, "--stream"}); exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if runCalled != 0 {
		t.Fatalf("Run called %d times, want 0", runCalled)
	}
	if runStreamCalled != 1 {
		t.Fatalf("RunStream called %d times, want 1", runStreamCalled)
	}
	if !strings.Contains(stdout.String(), "Session: session-") {
		t.Fatalf("stdout = %q, want streamed session summary", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestCLIStreamJSONL(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	app := newTestCLIApp(t, &stdout, &stderr, dbPath)
	app.runStreamEngine = func(ctx context.Context, engine runtime.Engine, task domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
		tokenEvent, err := domain.TokenDelta(1, "hello")
		if err != nil {
			return nil, domain.Session{}, domain.Plan{}, nil, err
		}
		toolStartEvent, err := domain.ToolStart(1, "grep", "needle")
		if err != nil {
			return nil, domain.Session{}, domain.Plan{}, nil, err
		}
		toolEndEvent, err := domain.ToolEnd(1, "grep", "found", "")
		if err != nil {
			return nil, domain.Session{}, domain.Plan{}, nil, err
		}
		stepCompleteEvent, err := domain.StepComplete(1, "done")
		if err != nil {
			return nil, domain.Session{}, domain.Plan{}, nil, err
		}
		errorEvent, err := domain.Error(1, "soft warning")
		if err != nil {
			return nil, domain.Session{}, domain.Plan{}, nil, err
		}
		events := []domain.StreamingEvent{
			*tokenEvent,
			*toolStartEvent,
			*toolEndEvent,
			*stepCompleteEvent,
			*errorEvent,
		}
		return emitTestStream(ctx, engine, task, events, domain.SessionStatusSuccess), domain.Session{}, domain.Plan{}, nil, nil
	}

	if exitCode := app.run(context.Background(), []string{"run", "--task", "stream json", "--db", dbPath, "--stream", "--json"}); exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in jsonl mode", stderr.String())
	}

	lines := nonEmptyLines(stdout.String())
	if len(lines) != 6 {
		t.Fatalf("jsonl lines = %d, want 6\noutput=%s", len(lines), stdout.String())
	}

	var events []domain.StreamingEvent
	for i, line := range lines {
		var event domain.StreamingEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("unmarshal line %d: %v\nline=%s", i, err, line)
		}
		events = append(events, event)
	}

	wantTypes := []domain.StreamingEventType{
		domain.EventTokenDelta,
		domain.EventToolStart,
		domain.EventToolEnd,
		domain.EventStepComplete,
		domain.EventError,
		domain.EventSessionComplete,
	}
	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Fatalf("event[%d].type = %q, want %q", i, events[i].Type, want)
		}
	}

	var complete domain.SessionCompletePayload
	if err := events[len(events)-1].GetPayload(&complete); err != nil {
		t.Fatalf("decode session complete payload: %v", err)
	}
	if !strings.HasPrefix(complete.SessionID, "session-") {
		t.Fatalf("session complete session id = %q, want user-facing session-*", complete.SessionID)
	}
	if complete.Status != string(domain.SessionStatusSuccess) {
		t.Fatalf("session complete status = %q, want %q", complete.Status, domain.SessionStatusSuccess)
	}
}

func TestCLIStreamDisabled(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var runCalled int
	var runStreamCalled int

	app := newTestCLIApp(t, &stdout, &stderr, dbPath)
	app.runEngine = func(context.Context, runtime.Engine, domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
		runCalled++
		return domain.Session{Status: domain.SessionStatusSuccess}, domain.Plan{Summary: "legacy plan"}, []domain.Step{{Index: 1}}, nil
	}
	app.runStreamEngine = func(context.Context, runtime.Engine, domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
		runStreamCalled++
		return runtime.NewEventChannel(1), domain.Session{}, domain.Plan{}, nil, nil
	}

	if exitCode := app.run(context.Background(), []string{"run", "--task", "legacy task", "--db", dbPath, "--json"}); exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if runCalled != 1 {
		t.Fatalf("Run called %d times, want 1", runCalled)
	}
	if runStreamCalled != 0 {
		t.Fatalf("RunStream called %d times, want 0", runStreamCalled)
	}

	var payload runJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal run output: %v\noutput=%s", err, stdout.String())
	}
	if payload.Command != "run" || payload.Status != domain.SessionStatusSuccess || payload.Plan != "legacy plan" || payload.Steps != 1 {
		t.Fatalf("payload = %+v, want legacy run JSON output", payload)
	}
}

func TestCLIStreamInterrupt(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	app := cliApp{
		stdout: &stdout,
		stderr: &stderr,
		newSession: func(dbPath string) (*store.SQLiteSessionStore, error) {
			return store.NewSQLiteSessionStore(dbPath)
		},
		newMemory: func(dbPath string) (*store.SQLiteMemoryStore, error) {
			return store.NewMemoryStore(dbPath)
		},
		newExecutor: func() domain.ToolExecutor { return FakeToolExecutor{} },
		newModel:    func() domain.Model { return &FakeModel{Delay: 250 * time.Millisecond} },
		newVerifier: func(domain.ToolExecutor) domain.Verifier { return FakeVerifier{} },
		notifySignal: func(ch chan<- os.Signal, _ ...os.Signal) {
			go func() {
				time.Sleep(25 * time.Millisecond)
				ch <- syscall.SIGINT
			}()
		},
		stopSignal: func(chan<- os.Signal) {},
		now:        time.Now,
	}

	exitCode := app.run(context.Background(), []string{"run", "--task", "slow stream task", "--db", dbPath, "--stream"})
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "ERROR: context canceled") {
		t.Fatalf("stderr = %q, want streamed error", stderr.String())
	}

	sessionID := parseSessionIDFromOutput(t, stdout.String())
	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	session, plan, steps, err := sessionStore.ResumeSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("resume interrupted session: %v", err)
	}
	if session.Status != domain.SessionStatusInterrupted {
		t.Fatalf("persisted status = %q, want interrupted", session.Status)
	}
	if strings.TrimSpace(plan.Summary) == "" {
		t.Fatal("persisted plan summary = empty, want continuity")
	}
	if len(steps) != 0 {
		t.Fatalf("persisted steps = %d, want 0", len(steps))
	}
}

func TestCLIStreamResumeAndInspectInterruptedSession(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var runStdout bytes.Buffer
	var runStderr bytes.Buffer

	runApp := cliApp{
		stdout: &runStdout,
		stderr: &runStderr,
		newSession: func(dbPath string) (*store.SQLiteSessionStore, error) {
			return store.NewSQLiteSessionStore(dbPath)
		},
		newMemory: func(dbPath string) (*store.SQLiteMemoryStore, error) {
			return store.NewMemoryStore(dbPath)
		},
		newExecutor: func() domain.ToolExecutor { return FakeToolExecutor{} },
		newModel:    func() domain.Model { return &FakeModel{Delay: 250 * time.Millisecond} },
		newVerifier: func(domain.ToolExecutor) domain.Verifier { return FakeVerifier{} },
		notifySignal: func(ch chan<- os.Signal, _ ...os.Signal) {
			go func() {
				time.Sleep(25 * time.Millisecond)
				ch <- syscall.SIGINT
			}()
		},
		stopSignal: func(chan<- os.Signal) {},
		now:        time.Now,
	}

	exitCode := runApp.run(context.Background(), []string{"run", "--task", "slow stream task", "--db", dbPath, "--stream"})
	if exitCode != 1 {
		t.Fatalf("run exit code = %d, want 1", exitCode)
	}
	sessionID := parseSessionIDFromOutput(t, runStdout.String())

	var resumeStdout bytes.Buffer
	var resumeStderr bytes.Buffer
	resumeApp := newTestCLIApp(t, &resumeStdout, &resumeStderr, dbPath)
	resumeApp.runStreamEngine = func(ctx context.Context, engine runtime.Engine, task domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
		return emitResumeStream(ctx, engine, task, sessionID, domain.SessionStatusSuccess), domain.Session{}, domain.Plan{}, nil, nil
	}

	exitCode = resumeApp.run(context.Background(), []string{"resume", "--session", sessionID, "--db", dbPath, "--stream", "--json"})
	if exitCode != 0 {
		t.Fatalf("resume exit code = %d, want 0, stderr=%s", exitCode, resumeStderr.String())
	}

	lines := nonEmptyLines(resumeStdout.String())
	if len(lines) != 3 {
		t.Fatalf("resume jsonl lines = %d, want 3\noutput=%s", len(lines), resumeStdout.String())
	}

	var streamed []domain.StreamingEvent
	for i, line := range lines {
		var event domain.StreamingEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("unmarshal resume line %d: %v", i, err)
		}
		streamed = append(streamed, event)
	}
	if streamed[0].Type != domain.EventTokenDelta || streamed[1].Type != domain.EventStepComplete || streamed[2].Type != domain.EventSessionComplete {
		t.Fatalf("resume streamed event types = [%q %q %q], want [token_delta step_complete session_complete]", streamed[0].Type, streamed[1].Type, streamed[2].Type)
	}

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	session, plan, steps, err := sessionStore.ResumeSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("resume session after streamed continuation: %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("persisted status = %q, want success", session.Status)
	}
	if len(steps) != 1 {
		t.Fatalf("persisted steps = %d, want 1", len(steps))
	}
	if strings.TrimSpace(plan.Summary) == "" {
		t.Fatal("persisted plan summary = empty, want preserved plan")
	}

	var inspectStdout bytes.Buffer
	var inspectStderr bytes.Buffer
	inspectApp := newTestCLIApp(t, &inspectStdout, &inspectStderr, dbPath)
	exitCode = inspectApp.run(context.Background(), []string{"inspect", "--session", sessionID, "--db", dbPath, "--json"})
	if exitCode != 0 {
		t.Fatalf("inspect exit code = %d, want 0, stderr=%s", exitCode, inspectStderr.String())
	}

	var payload inspectJSONOutput
	if err := json.Unmarshal(inspectStdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal inspect payload: %v\noutput=%s", err, inspectStdout.String())
	}
	if payload.SessionID != sessionID {
		t.Fatalf("inspect session id = %q, want %q", payload.SessionID, sessionID)
	}
	if payload.Status != domain.SessionStatusSuccess {
		t.Fatalf("inspect status = %q, want success", payload.Status)
	}
	if payload.StepCount != 1 {
		t.Fatalf("inspect step count = %d, want 1", payload.StepCount)
	}
	if len(payload.StepSummaries) != 1 || !strings.Contains(payload.StepSummaries[0], "step 1: resumed summary") {
		t.Fatalf("inspect summaries = %#v, want resumed step summary", payload.StepSummaries)
	}
	if strings.Contains(inspectStdout.String(), "token_delta") {
		t.Fatalf("inspect output should not persist streaming events: %s", inspectStdout.String())
	}
}

func newTestCLIApp(t *testing.T, stdout, stderr *bytes.Buffer, _ string) cliApp {
	t.Helper()
	return cliApp{
		stdout: stdout,
		stderr: stderr,
		newSession: func(path string) (*store.SQLiteSessionStore, error) {
			return store.NewSQLiteSessionStore(path)
		},
		newMemory: func(path string) (*store.SQLiteMemoryStore, error) {
			return store.NewMemoryStore(path)
		},
		newExecutor: func() domain.ToolExecutor { return FakeToolExecutor{} },
		newModel:    func() domain.Model { return &FakeModel{} },
		newVerifier: func(domain.ToolExecutor) domain.Verifier { return FakeVerifier{} },
		runEngine: func(ctx context.Context, engine runtime.Engine, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
			return engine.Run(ctx, task)
		},
		runStreamEngine: func(ctx context.Context, engine runtime.Engine, task domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
			return engine.RunStream(ctx, task)
		},
		notifySignal: func(chan<- os.Signal, ...os.Signal) {},
		stopSignal:   func(chan<- os.Signal) {},
		now:          time.Now,
	}
}

func emitTestStream(ctx context.Context, engine runtime.Engine, task domain.Task, events []domain.StreamingEvent, status domain.SessionStatus) *runtime.EventChannel {
	ec := runtime.NewEventChannel(16)
	go func() {
		defer ec.Close()
		_ = engine.Sessions.SaveSession(ctx, domain.Session{TaskID: task.ID, Status: status, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		_ = engine.Sessions.SavePlan(ctx, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: "stream plan", CreatedAt: time.Now()})
		for _, event := range events {
			_ = ec.Emit(event)
		}
		completeEvent, err := domain.SessionComplete(task.ID+"-session", string(status))
		if err == nil {
			_ = ec.Emit(*completeEvent)
		}
	}()
	return ec
}

func emitResumeStream(ctx context.Context, engine runtime.Engine, task domain.Task, sessionID string, status domain.SessionStatus) *runtime.EventChannel {
	ec := runtime.NewEventChannel(16)
	go func() {
		defer ec.Close()
		step := domain.Step{
			Index: 1,
			Action: domain.Action{
				Type:     domain.ActionTypeRespond,
				Summary:  "resumed summary",
				Response: "resumed final response",
			},
			Observation: domain.Observation{
				Summary:       "resumed summary",
				FinalResponse: "resumed final response",
			},
			Verification: domain.VerificationResult{
				Passed: true,
				Status: domain.VerificationStatusPassed,
				Reason: "resumed final response",
			},
		}
		_ = engine.Sessions.SaveSession(ctx, domain.Session{ID: sessionID, TaskID: task.ID, Status: status, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		_ = engine.Sessions.SavePlan(ctx, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: "stream plan", CreatedAt: time.Now()})
		_ = engine.Sessions.AppendStep(ctx, sessionID, step)

		tokenEvent, err := domain.TokenDelta(1, "resumed final response")
		if err == nil {
			_ = ec.Emit(*tokenEvent)
		}
		stepEvent, err := domain.StepComplete(1, "resumed summary")
		if err == nil {
			_ = ec.Emit(*stepEvent)
		}
		completeEvent, err := domain.SessionComplete(sessionID, string(status))
		if err == nil {
			_ = ec.Emit(*completeEvent)
		}
	}()
	return ec
}

func nonEmptyLines(output string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseSessionIDFromOutput(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Session: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Session: "))
		}
	}
	t.Fatalf("session id not found in output:\n%s", output)
	return ""
}

func formatEventTypes(events []domain.StreamingEvent) string {
	parts := make([]string, 0, len(events))
	for _, event := range events {
		parts = append(parts, string(event.Type))
	}
	return fmt.Sprintf("%v", parts)
}
