package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/store"
)

func TestRunCommandJSONCreatesPersistentSession(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--db", dbPath, "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var payload runJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal run output: %v\noutput=%s", err, stdout.String())
	}
	if payload.Command != "run" {
		t.Fatalf("command = %q, want run", payload.Command)
	}
	if !strings.HasPrefix(payload.SessionID, "session-") {
		t.Fatalf("session id = %q, want session-*", payload.SessionID)
	}
	if strings.HasSuffix(payload.SessionID, "-session") {
		t.Fatalf("session id = %q, should stay user-facing", payload.SessionID)
	}
	if payload.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want success", payload.Status)
	}
	if payload.TaskInput != "inspect repository" {
		t.Fatalf("task input = %q, want original task", payload.TaskInput)
	}

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	session, plan, steps, err := sessionStore.ResumeSession(context.Background(), payload.SessionID)
	if err != nil {
		t.Fatalf("resume persisted session: %v", err)
	}
	if session.Status != domain.SessionStatusSuccess {
		t.Fatalf("persisted status = %q, want success", session.Status)
	}
	if !strings.Contains(plan.Summary, payload.TaskInput) {
		t.Fatalf("plan summary = %q, want task input", plan.Summary)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
}

func TestResumeAndInspectOutput(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var runOut bytes.Buffer
	var runErr bytes.Buffer
	if exitCode := runCLI(context.Background(), []string{"run", "--task", "prepare summary", "--db", dbPath, "--json"}, &runOut, &runErr); exitCode != 0 {
		t.Fatalf("run exit code = %d, stderr=%s", exitCode, runErr.String())
	}

	var runPayload runJSONOutput
	if err := json.Unmarshal(runOut.Bytes(), &runPayload); err != nil {
		t.Fatalf("unmarshal run output: %v", err)
	}

	var resumeStdout bytes.Buffer
	var resumeStderr bytes.Buffer
	if exitCode := runCLI(context.Background(), []string{"resume", "--session", runPayload.SessionID, "--db", dbPath}, &resumeStdout, &resumeStderr); exitCode != 0 {
		t.Fatalf("resume exit code = %d, stderr=%s", exitCode, resumeStderr.String())
	}
	if got := resumeStdout.String(); !strings.Contains(got, "Resumed session: "+runPayload.SessionID) || !strings.Contains(got, "History:") {
		t.Fatalf("resume output missing expected fields:\n%s", got)
	}
	if strings.Contains(resumeStdout.String(), runPayload.SessionID+"-session") {
		t.Fatalf("resume output leaked runtime session id:\n%s", resumeStdout.String())
	}

	var inspectStdout bytes.Buffer
	var inspectStderr bytes.Buffer
	if exitCode := runCLI(context.Background(), []string{"inspect", "--session", runPayload.SessionID, "--db", dbPath, "--json"}, &inspectStdout, &inspectStderr); exitCode != 0 {
		t.Fatalf("inspect exit code = %d, stderr=%s", exitCode, inspectStderr.String())
	}

	var inspectPayload inspectJSONOutput
	if err := json.Unmarshal(inspectStdout.Bytes(), &inspectPayload); err != nil {
		t.Fatalf("unmarshal inspect output: %v", err)
	}
	if inspectPayload.Command != "inspect" {
		t.Fatalf("inspect command = %q, want inspect", inspectPayload.Command)
	}
	if inspectPayload.SessionID != runPayload.SessionID {
		t.Fatalf("inspect session id = %q, want %q", inspectPayload.SessionID, runPayload.SessionID)
	}
	if inspectPayload.Status != domain.SessionStatusSuccess {
		t.Fatalf("inspect status = %q, want success", inspectPayload.Status)
	}
	if inspectPayload.StepCount != 1 {
		t.Fatalf("inspect step count = %d, want 1", inspectPayload.StepCount)
	}
	if len(inspectPayload.StepSummaries) != 1 {
		t.Fatalf("inspect summaries = %d, want 1", len(inspectPayload.StepSummaries))
	}
}

func TestRunCommandInterruptPersistsInterruptedSession(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	signalCh := make(chan os.Signal, 1)
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
		newExecutor: func() domain.ToolExecutor {
			return FakeToolExecutor{}
		},
		newModel: func() domain.Model {
			return &FakeModel{Delay: 250 * time.Millisecond}
		},
		newVerifier: func() domain.Verifier { return FakeVerifier{} },
		notifySignal: func(ch chan<- os.Signal, _ ...os.Signal) {
			go func() {
				time.Sleep(25 * time.Millisecond)
				ch <- syscall.SIGINT
			}()
		},
		stopSignal: func(chan<- os.Signal) {},
		now:        time.Now,
	}

	exitCode := app.run(context.Background(), []string{"run", "--task", "slow task", "--db", dbPath, "--json"})
	_ = signalCh
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), context.Canceled.Error()) {
		t.Fatalf("stderr = %q, want context canceled", stderr.String())
	}

	var payload runJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal interrupted run output: %v", err)
	}
	if payload.Status != domain.SessionStatusInterrupted {
		t.Fatalf("run status = %q, want interrupted", payload.Status)
	}

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	session, _, _, err := sessionStore.ResumeSession(context.Background(), payload.SessionID)
	if err != nil {
		t.Fatalf("resume interrupted session: %v", err)
	}
	if session.Status != domain.SessionStatusInterrupted {
		t.Fatalf("persisted interrupted status = %q, want interrupted", session.Status)
	}
}

func TestRunCommandSupportsMaxStepsFlag(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI(context.Background(), []string{"run", "--task", "bounded task", "--max-steps", "3", "--db", dbPath, "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}

	var payload runJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal run output: %v", err)
	}
	if payload.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want success", payload.Status)
	}
}

func TestRunCLIUsesSelectedProviderFromConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "dashscope",
		"providers": {
			"dashscope": {
				"type": "openai",
				"model": "qwen3.6-plus"
			},
			"openai": {
				"type": "openai",
				"model": "gpt-4.1-mini"
			}
		},
		"runtime": {
			"max_steps": 4,
			"step_timeout": "30s",
			"memory_limit_mb": 256,
			"verify_mode": "standard"
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--config", configPath, "--provider", "openai", "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCLIRejectsMissingSelectedProvider(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "dashscope",
		"providers": {
			"dashscope": {
				"type": "dashscope",
				"model": "qwen3.6-plus",
				"api_key": "dash-key"
			}
		},
		"runtime": {
			"max_steps": 4,
			"step_timeout": "30s",
			"memory_limit_mb": 256,
			"verify_mode": "standard"
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--config", configPath, "--provider", config.ProviderOpenAI, "--json"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), `provider "openai" not found`) {
		t.Fatalf("stderr = %q, want provider not found", stderr.String())
	}
}
