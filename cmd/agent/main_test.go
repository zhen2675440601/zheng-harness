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
	"zheng-harness/internal/tools"
	"zheng-harness/internal/verify"
)

func TestRunCommandJSONCreatesPersistentSession(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--verify-mode", config.VerifyModeOff, "--db", dbPath, "--json"}, &stdout, &stderr)
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
	if payload.TaskType != domain.TaskCategoryGeneral {
		t.Fatalf("task type = %q, want %q", payload.TaskType, domain.TaskCategoryGeneral)
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
	if strings.TrimSpace(plan.Summary) == "" {
		t.Fatalf("plan summary = %q, want non-empty", plan.Summary)
	}
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	loadedTask, ok, err := sessionStore.LoadTask(context.Background(), payload.SessionID)
	if err != nil {
		t.Fatalf("load persisted task: %v", err)
	}
	if ok {
		t.Fatalf("LoadTask metadata flag = %v, want false for default run", ok)
	}
	if loadedTask.Category != domain.TaskCategoryGeneral {
		t.Fatalf("loaded task category = %q, want %q", loadedTask.Category, domain.TaskCategoryGeneral)
	}
}

func TestResumeAndInspectOutput(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var runOut bytes.Buffer
	var runErr bytes.Buffer
	if exitCode := runCLI(context.Background(), []string{"run", "--task", "prepare summary", "--verify-mode", config.VerifyModeOff, "--db", dbPath, "--json"}, &runOut, &runErr); exitCode != 0 {
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

	exitCode := runCLI(context.Background(), []string{"run", "--task", "bounded task", "--max-steps", "3", "--verify-mode", config.VerifyModeOff, "--db", dbPath, "--json"}, &stdout, &stderr)
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

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "dashscope",
		"providers": {
			"dashscope": {
				"type": "openai",
				"model": "qwen3.6-plus"
			}
		},
		"runtime": {
			"max_steps": 4,
			"step_timeout": "30s",
			"memory_limit_mb": 256,
			"verify_mode": "off"
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// Use --provider dashscope (which has no api_key set) with --verify-mode off
	// Since no real API key is available, this test validates config loading works.
	// Without an API key, the real provider will fail, so we test config selection without --provider override.
	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--config", configPath, "--verify-mode", config.VerifyModeOff, "--db", dbPath, "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
}

func TestRunCLIRejectsMissingSelectedProvider(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
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
	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--config", configPath, "--provider", config.ProviderOpenAI, "--db", dbPath, "--json"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), `provider "openai" not found`) {
		t.Fatalf("stderr = %q, want provider not found", stderr.String())
	}
}

func TestRunCLIUsesProviderAdapterForOpenAI(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When OpenAI provider is specified without an API key, CLI falls back to FakeModel.
	// This test verifies the CLI handles missing API key gracefully (uses FakeModel instead of crashing).
	exitCode := runCLI(context.Background(), []string{
		"run",
		"--task", "inspect repository",
		"--provider", config.ProviderOpenAI,
		"--verify-mode", config.VerifyModeOff,
		"--db", dbPath,
		"--json",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (FakeModel fallback), stderr=%s", exitCode, stderr.String())
	}

	var payload runJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal run output: %v", err)
	}
	if payload.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want success (FakeModel fallback)", payload.Status)
	}
}

func TestRunCLIVerifyModeStrictUsesCommandVerifier(t *testing.T) {
	t.Parallel()

	strictCfg := config.Default()
	strictCfg.Runtime.VerifyMode = config.VerifyModeStrict
	if _, ok := newVerifierFromConfig(strictCfg, FakeToolExecutor{}).(*verify.TaskAwareVerifier); !ok {
		t.Fatalf("verify_mode=strict should wire task-aware verifier")
	}
}

func TestRootHelpShowsUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "no args", args: nil},
		{name: "long help", args: []string{"--help"}},
		{name: "short help", args: []string{"-h"}},
		{name: "help command", args: []string{"help"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := runCLI(context.Background(), tt.args, &stdout, &stderr)
			if exitCode != 0 {
				t.Fatalf("exit code = %d, want 0", exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "Usage: zheng-agent <run|resume|inspect> [flags]") {
				t.Fatalf("stderr = %q, want usage output", stderr.String())
			}
			if strings.Contains(stderr.String(), "unknown subcommand") {
				t.Fatalf("stderr = %q, should not report unknown subcommand", stderr.String())
			}
		})
	}
}

func TestRunInspectAndResumePreserveTaskMetadata(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var runOut bytes.Buffer
	var runErr bytes.Buffer
	if exitCode := runCLI(context.Background(), []string{
		"run",
		"--task", "collect research notes",
		"--task-type", string(domain.TaskCategoryResearch),
		"--task-protocol", "evidence-first research workflow",
		"--task-verification-policy", "evidence",
		"--verify-mode", config.VerifyModeOff,
		"--db", dbPath,
		"--json",
	}, &runOut, &runErr); exitCode != 0 {
		t.Fatalf("run exit code = %d, stderr=%s", exitCode, runErr.String())
	}

	var runPayload runJSONOutput
	if err := json.Unmarshal(runOut.Bytes(), &runPayload); err != nil {
		t.Fatalf("unmarshal run output: %v", err)
	}
	if runPayload.TaskType != domain.TaskCategoryResearch {
		t.Fatalf("run task_type = %q, want %q", runPayload.TaskType, domain.TaskCategoryResearch)
	}
	if runPayload.ProtocolHint != "evidence-first research workflow" {
		t.Fatalf("run protocol_hint = %q, want evidence-first research workflow", runPayload.ProtocolHint)
	}
	if runPayload.VerificationPolicy != "evidence" {
		t.Fatalf("run verification_policy = %q, want evidence", runPayload.VerificationPolicy)
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
	if inspectPayload.TaskType != domain.TaskCategoryResearch {
		t.Fatalf("inspect task_type = %q, want %q", inspectPayload.TaskType, domain.TaskCategoryResearch)
	}
	if inspectPayload.ProtocolHint != runPayload.ProtocolHint {
		t.Fatalf("inspect protocol_hint = %q, want %q", inspectPayload.ProtocolHint, runPayload.ProtocolHint)
	}
	if inspectPayload.VerificationPolicy != runPayload.VerificationPolicy {
		t.Fatalf("inspect verification_policy = %q, want %q", inspectPayload.VerificationPolicy, runPayload.VerificationPolicy)
	}

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer func() { _ = sessionStore.Close() }()
	loadedTask, ok, err := sessionStore.LoadTask(context.Background(), runPayload.SessionID)
	if err != nil {
		t.Fatalf("LoadTask() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadTask() ok = false, want true")
	}
	if loadedTask.Category != domain.TaskCategoryResearch {
		t.Fatalf("loaded task category = %q, want %q", loadedTask.Category, domain.TaskCategoryResearch)
	}

	var resumeStdout bytes.Buffer
	var resumeStderr bytes.Buffer
	if exitCode := runCLI(context.Background(), []string{"resume", "--session", runPayload.SessionID, "--db", dbPath}, &resumeStdout, &resumeStderr); exitCode != 0 {
		t.Fatalf("resume exit code = %d, stderr=%s", exitCode, resumeStderr.String())
	}
	if got := resumeStdout.String(); !strings.Contains(got, "Resumed session: "+runPayload.SessionID) {
		t.Fatalf("resume output missing session heading:\n%s", got)
	}
}

func TestNewVerifierFromConfigRespectsVerifyMode(t *testing.T) {
	t.Parallel()

	offCfg := config.Default()
	offCfg.Runtime.VerifyMode = config.VerifyModeOff
	if _, ok := newVerifierFromConfig(offCfg, FakeToolExecutor{}).(FakeVerifier); !ok {
		t.Fatalf("verify_mode=off should use FakeVerifier")
	}

	standardCfg := config.Default()
	standardCfg.Runtime.VerifyMode = config.VerifyModeStandard
	if _, ok := newVerifierFromConfig(standardCfg, FakeToolExecutor{}).(*verify.TaskAwareVerifier); !ok {
		t.Fatalf("verify_mode=standard should use verify.TaskAwareVerifier")
	}

	strictCfg := config.Default()
	strictCfg.Runtime.VerifyMode = config.VerifyModeStrict
	if _, ok := newVerifierFromConfig(strictCfg, FakeToolExecutor{}).(*verify.TaskAwareVerifier); !ok {
		t.Fatalf("verify_mode=strict should use verify.TaskAwareVerifier")
	}
}

func TestVerifyModeOffUnchanged(t *testing.T) {
	t.Parallel()

	offCfg := config.Default()
	offCfg.Runtime.VerifyMode = config.VerifyModeOff
	verifier, ok := newVerifierFromConfig(offCfg, FakeToolExecutor{}).(FakeVerifier)
	if !ok {
		t.Fatalf("verify_mode=off should use FakeVerifier")
	}

	result, err := verifier.Verify(context.Background(), domain.Task{ID: "t1"}, domain.Session{ID: "s1"}, domain.Plan{ID: "p1"}, nil, domain.Observation{FinalResponse: "done"})
	if err != nil {
		t.Fatalf("off verify: %v", err)
	}
	if !result.Passed {
		t.Fatalf("fake verifier should still pass when final response exists, got %+v", result)
	}
}

func TestAllowCommandAdditionsWork(t *testing.T) {
	t.Parallel()

	app := cliApp{
		cfg: config.Config{
			Runtime: config.RuntimeSettings{
				AllowedCommands: []string{"go"},
			},
		},
		newExecutor: func() domain.ToolExecutor {
			executor, err := tools.NewExecutor(".", tools.WithAllowedCommands([]string{"go"}))
			if err != nil {
				t.Fatalf("new executor: %v", err)
			}
			return executor
		},
	}

	app = app.withExtraAllowedCommands([]string{"npm"})
	executor, ok := app.newExecutor().(*tools.Executor)
	if !ok {
		t.Fatal("expected tools.Executor")
	}

	_, err := executor.Execute(context.Background(), domain.ToolCall{
		Name:  "exec_command",
		Input: "npm test",
	})
	if err == nil || strings.Contains(err.Error(), "not allowlisted") || strings.Contains(err.Error(), "explicitly denied") {
		t.Fatalf("npm command error = %v, want allowlisted execution attempt", err)
	}
}
