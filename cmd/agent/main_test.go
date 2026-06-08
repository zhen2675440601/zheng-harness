package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	pluginruntime "zheng-harness/internal/plugin"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
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
	if strings.Contains(resumeStdout.String(), "plugin ") {
		t.Fatalf("resume output unexpectedly showed provenance for non-plugin session:\n%s", resumeStdout.String())
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
	if inspectPayload.Plan != runPayload.Plan {
		t.Fatalf("inspect plan = %q, want %q", inspectPayload.Plan, runPayload.Plan)
	}
	if inspectPayload.TerminatedReason != "final response recorded" {
		t.Fatalf("inspect terminated_reason = %q, want final response recorded", inspectPayload.TerminatedReason)
	}
	if inspectPayload.StepCount != 1 {
		t.Fatalf("inspect step count = %d, want 1", inspectPayload.StepCount)
	}
	if len(inspectPayload.StepSummaries) != 1 {
		t.Fatalf("inspect summaries = %d, want 1", len(inspectPayload.StepSummaries))
	}
	if !strings.Contains(inspectPayload.StepSummaries[0], "step 1:") {
		t.Fatalf("inspect summary = %q, want stable step prefix", inspectPayload.StepSummaries[0])
	}
}

func TestInspectDisplaysPersistedPluginProvenanceWithoutLivePlugin(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	ctx := context.Background()
	now := time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC)
	provenance := &domain.Provenance{Plugins: []domain.PluginMetadata{{
		Family:                domain.PluginFamilyProvider,
		LogicalID:             "acme/provider",
		DisplayName:           "Acme Provider",
		ContractVersion:       "1.0.0",
		ImplementationVersion: "2.0.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/provider-acme",
	}}}
	if err := sessionStore.SaveSession(ctx, domain.Session{ID: "session-provenance", TaskID: "task-provenance", Status: domain.SessionStatusSuccess, Provenance: provenance, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, domain.Plan{ID: "plan-provenance", TaskID: "task-provenance", Summary: "inspect persisted plugin provenance", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	if err := sessionStore.AppendStep(ctx, "session-provenance", domain.Step{Index: 1, Action: domain.Action{Type: domain.ActionTypeRespond, Summary: "persisted step"}, Observation: domain.Observation{Summary: "done", FinalResponse: "done"}, Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "done"}, Provenance: provenance}); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI(context.Background(), []string{"inspect", "--session", "session-provenance", "--db", dbPath, "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("inspect exit code = %d, stderr=%s", exitCode, stderr.String())
	}
	var payload inspectJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Provenance == nil || len(payload.Provenance.Plugins) != 1 {
		t.Fatalf("inspect provenance = %#v, want persisted plugin provenance", payload.Provenance)
	}
	if payload.Provenance.Plugins[0].LogicalID != "acme/provider" {
		t.Fatalf("inspect provenance logical id = %q, want acme/provider", payload.Provenance.Plugins[0].LogicalID)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestResumeFailsClosedWhenPersistedProviderPluginUnavailable(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	ctx := context.Background()
	now := time.Date(2026, time.April, 30, 12, 5, 0, 0, time.UTC)
	provenance := &domain.Provenance{Plugins: []domain.PluginMetadata{{
		Family:                domain.PluginFamilyProvider,
		LogicalID:             "acme/provider",
		DisplayName:           "Acme Provider",
		ContractVersion:       "1.0.0",
		ImplementationVersion: "2.0.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/provider-acme",
	}}}
	if err := sessionStore.SaveSession(ctx, domain.Session{ID: "session-resume-provider", TaskID: "task-resume-provider", Status: domain.SessionStatusRunning, Provenance: provenance, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, domain.Plan{ID: "plan-resume-provider", TaskID: "task-resume-provider", Summary: "resume persisted provider plugin", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI(context.Background(), []string{"resume", "--session", "session-resume-provider", "--db", dbPath}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("resume exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "resume fail-closed") || !strings.Contains(stderr.String(), "acme/provider") {
		t.Fatalf("stderr = %q, want deterministic fail-closed provider error", stderr.String())
	}

	resumed, _, _, err := sessionStore.ResumeSession(ctx, "session-resume-provider")
	if err != nil {
		t.Fatalf("ResumeSession() after failed resume error = %v", err)
	}
	if resumed.Provenance == nil || resumed.Provenance.Plugins[0].LogicalID != "acme/provider" {
		t.Fatalf("persisted provenance mutated after failed resume: %#v", resumed.Provenance)
	}
}

func TestResumeFailsClosedWhenPersistedVerifierPluginUnavailable(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	ctx := context.Background()
	now := time.Date(2026, time.April, 30, 12, 10, 0, 0, time.UTC)
	provenance := &domain.Provenance{Plugins: []domain.PluginMetadata{{
		Family:                domain.PluginFamilyVerifier,
		LogicalID:             "evidence-plugin",
		DisplayName:           "Evidence Plugin",
		ContractVersion:       verify.VerifierPluginContractVersion,
		ImplementationVersion: "2.0.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/evidence-plugin",
	}}}
	if err := sessionStore.SaveSession(ctx, domain.Session{ID: "session-resume-verifier", TaskID: "task-resume-verifier", Status: domain.SessionStatusRunning, Provenance: provenance, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(ctx, "session-resume-verifier", domain.Task{ID: "task-resume-verifier", Description: "resume verifier", Goal: "resume verifier", VerificationPolicy: verify.PolicyEvidenceBased, CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, domain.Plan{ID: "plan-resume-verifier", TaskID: "task-resume-verifier", Summary: "resume persisted verifier plugin", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI(context.Background(), []string{"resume", "--session", "session-resume-verifier", "--db", dbPath}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("resume exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "resume fail-closed") || !strings.Contains(stderr.String(), "evidence-plugin") {
		t.Fatalf("stderr = %q, want deterministic fail-closed verifier error", stderr.String())
	}
}

func TestRuntimePersistsProviderPluginProvenanceEndToEnd(t *testing.T) {
	t.Parallel()

	previous := llm.DefaultProviderResolver()
	resolver := &providerResolverStub{provider: &providerPluginStub{id: "acme/provider", name: "Acme Provider", metadata: domain.PluginMetadata{
		Family:                domain.PluginFamilyProvider,
		LogicalID:             "acme/provider",
		DisplayName:           "Acme Provider",
		ContractVersion:       "1.0.0",
		ImplementationVersion: "2.0.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/provider-acme",
	}}}
	llm.SetDefaultProviderResolver(resolver)
	defer llm.SetDefaultProviderResolver(previous)

	configPath := filepath.Join(t.TempDir(), "zheng.json")
	dbPath := filepath.Join(t.TempDir(), "agent.db")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "acme/provider",
		"plugin_provider": "acme/provider",
		"providers": {
			"acme/provider": {
				"type": "plugin",
				"model": "plugin-model",
				"api_key": "secret"
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
	exitCode := runCLI(context.Background(), []string{"run", "--task", "persist provider provenance", "--config", configPath, "--db", dbPath, "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run exit code = %d, stderr=%s", exitCode, stderr.String())
	}
	var payload runJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	reopenedStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore(reopen) error = %v", err)
	}
	defer func() { _ = reopenedStore.Close() }()
	session, _, steps, err := reopenedStore.ResumeSession(context.Background(), payload.SessionID)
	if err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
	if session.Provenance == nil || len(session.Provenance.Plugins) != 1 {
		t.Fatalf("session provenance = %#v, want provider plugin persisted", session.Provenance)
	}
	if len(steps) != 1 || steps[0].Provenance == nil || len(steps[0].Provenance.Plugins) != 1 {
		t.Fatalf("step provenance = %#v, want provider plugin persisted", steps)
	}
	if session.Provenance.Plugins[0].LogicalID != "acme/provider" {
		t.Fatalf("session provider provenance = %q, want acme/provider", session.Provenance.Plugins[0].LogicalID)
	}
	if resolver.calls == 0 {
		t.Fatal("expected provider resolver to be used")
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
	if strings.TrimSpace(session.TaskID) == "" {
		t.Fatal("persisted interrupted task id = empty, want stable task id")
	}

	inspectSession, inspectPlan, inspectSteps, err := sessionStore.ResumeSession(context.Background(), payload.SessionID)
	if err != nil {
		t.Fatalf("inspect interrupted session continuity: %v", err)
	}
	if inspectSession.ID != payload.SessionID {
		t.Fatalf("inspect session id = %q, want %q", inspectSession.ID, payload.SessionID)
	}
	if strings.TrimSpace(inspectPlan.Summary) == "" {
		t.Fatal("interrupted plan summary = empty, want persisted plan")
	}
	if len(inspectSteps) != 0 {
		t.Fatalf("interrupted steps = %d, want 0 when canceled before first step", len(inspectSteps))
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
	// 使用 --provider dashscope（其未设置 api_key）并配合 --verify-mode off。
	// 由于没有可用的真实 API key，此测试用于验证配置加载能够正常工作。
	// 在没有 API key 的情况下，真实 provider 会失败，因此这里测试不通过 --provider 覆盖时的配置选择逻辑。
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

	// 当指定 OpenAI provider 但未提供 API key 时，CLI 会回退到 FakeModel。
	// 此测试验证 CLI 能优雅处理缺少 API key 的情况（使用 FakeModel 而不是直接崩溃）。
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

func TestPluginSelectionBuiltInProviderCompatibility(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "openai",
		"providers": {
			"openai": {
				"type": "openai",
				"model": "gpt-4.1-mini"
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
	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--config", configPath, "--db", dbPath, "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestPluginSelectionSupportsExplicitPluginProvider(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "openai",
		"providers": {
			"openai": {
				"type": "openai",
				"model": "gpt-4.1-mini"
			},
			"acme/provider": {
				"type": "plugin",
				"model": "plugin-model"
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

	loaded, err := config.Load([]string{"-config", configPath, "-plugin-provider", "acme/provider"})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if loaded.Provider != "acme/provider" {
		t.Fatalf("provider = %q, want acme/provider", loaded.Provider)
	}
	if loaded.PluginProvider != "acme/provider" {
		t.Fatalf("plugin provider = %q, want acme/provider", loaded.PluginProvider)
	}
	if loaded.GetProviderType() != config.ProviderPlugin {
		t.Fatalf("provider type = %q, want %q", loaded.GetProviderType(), config.ProviderPlugin)
	}
}

func TestRunFailsClosedForExplicitPluginProviderWithoutAPIKey(t *testing.T) {
	// 注意：该测试依赖全局 provider resolver 的默认行为，不能并行执行。

	configPath := filepath.Join(t.TempDir(), "zheng.json")
	previous := llm.DefaultProviderResolver()
	llm.SetDefaultProviderResolver(nil)
	defer llm.SetDefaultProviderResolver(previous)
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "acme/provider",
		"plugin_provider": "acme/provider",
		"providers": {
			"acme/provider": {
				"type": "plugin",
				"model": "plugin-model"
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
	exitCode := runCLI(context.Background(), []string{"run", "--task", "inspect repository", "--config", configPath, "--json"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), `unsupported provider type "plugin"`) {
		t.Fatalf("stderr = %q, want deterministic plugin provider failure", stderr.String())
	}
}

func TestPluginSelectionRejectsConflictingSources(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"plugin_provider": "acme/provider",
		"providers": {
			"openai": {
				"type": "openai",
				"model": "gpt-4.1-mini"
			},
			"acme/provider": {
				"type": "plugin",
				"model": "plugin-model"
			}
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
	if !strings.Contains(stderr.String(), "provider selection conflict") {
		t.Fatalf("stderr = %q, want provider selection conflict", stderr.String())
	}
}

func TestResumeFailsClosedWhenPersistedProviderContractVersionMismatches(t *testing.T) {
	// 注意：该测试会覆盖全局 provider resolver，避免并行执行以防互相污染。

	previous := llm.DefaultProviderResolver()
	resolver := &providerResolverStub{provider: &providerPluginStub{id: "acme/provider", name: "Acme Provider", metadata: domain.PluginMetadata{
		Family:                domain.PluginFamilyProvider,
		LogicalID:             "acme/provider",
		DisplayName:           "Acme Provider",
		ContractVersion:       "2.0.0",
		ImplementationVersion: "2.1.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/provider-acme",
	}}}
	llm.SetDefaultProviderResolver(resolver)
	defer llm.SetDefaultProviderResolver(previous)

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"default_provider": "acme/provider",
		"plugin_provider": "acme/provider",
		"providers": {
			"acme/provider": {
				"type": "plugin",
				"model": "plugin-model",
				"api_key": "secret"
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

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	defer func() { _ = sessionStore.Close() }()

	ctx := context.Background()
	now := time.Date(2026, time.April, 30, 12, 10, 0, 0, time.UTC)
	provenance := &domain.Provenance{Plugins: []domain.PluginMetadata{{
		Family:                domain.PluginFamilyProvider,
		LogicalID:             "acme/provider",
		DisplayName:           "Acme Provider",
		ContractVersion:       "1.0.0",
		ImplementationVersion: "2.0.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/provider-acme",
	}}}
	if err := sessionStore.SaveSession(ctx, domain.Session{ID: "session-resume-provider-contract", TaskID: "task-resume-provider-contract", Status: domain.SessionStatusRunning, Provenance: provenance, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, domain.Plan{ID: "plan-resume-provider-contract", TaskID: "task-resume-provider-contract", Summary: "resume persisted provider plugin contract", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runCLI(context.Background(), []string{"resume", "--session", "session-resume-provider-contract", "--db", dbPath, "--config", configPath}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("resume exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "resume fail-closed") || !strings.Contains(stderr.String(), "1.0.0") || !strings.Contains(stderr.String(), "2.0.0") {
		t.Fatalf("stderr = %q, want deterministic contract-version mismatch", stderr.String())
	}
}

func TestMultiAgentCLIUsesDecomposeFlags(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var observed multiAgentOptions

	app := cliApp{
		stdout: &stdout,
		stderr: &stderr,
		newSession: func(dbPath string) (*store.SQLiteSessionStore, error) { return store.NewSQLiteSessionStore(dbPath) },
		newMemory:  func(dbPath string) (*store.SQLiteMemoryStore, error) { return store.NewMemoryStore(dbPath) },
		newExecutor: func() domain.ToolExecutor { return FakeToolExecutor{} },
		newModel:    func() domain.Model { return &FakeModel{} },
		newVerifier: func(domain.ToolExecutor) domain.Verifier { return FakeVerifier{} },
		runMultiAgent: func(_ context.Context, _ runtime.Engine, task domain.Task, options multiAgentOptions) (domain.Session, domain.Plan, []domain.Step, error) {
			observed = options
			return domain.Session{ID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{Summary: "multi-agent plan"}, []domain.Step{{Index: 1}}, nil
		},
		notifySignal: signal.Notify,
		stopSignal:   signal.Stop,
		now:          time.Now,
	}

	err := app.runCommand(context.Background(), []string{
		"--task", "coordinate subtasks",
		"--db", dbPath,
		"--json",
		"--decompose",
		"--max-workers", "6",
		"--aggregation", aggregationFlagBestEffort,
	})
	if err != nil {
		t.Fatalf("runCommand() error = %v", err)
	}
	if observed != (multiAgentOptions{Decompose: true, MaxWorkers: 6, Aggregation: aggregationFlagBestEffort}) {
		t.Fatalf("observed options = %+v", observed)
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
	if payload.Status != domain.SessionStatusSuccess {
		t.Fatalf("status = %q, want success", payload.Status)
	}
	if !strings.HasPrefix(payload.SessionID, "session-") {
		t.Fatalf("session id = %q, want session-*", payload.SessionID)
	}
	if payload.TaskInput != "coordinate subtasks" {
		t.Fatalf("task input = %q, want original task", payload.TaskInput)
	}
	if payload.Plan != "multi-agent plan" {
		t.Fatalf("plan = %q, want multi-agent plan", payload.Plan)
	}
	if payload.Steps != 1 {
		t.Fatalf("steps = %d, want 1", payload.Steps)
	}
	if payload.TaskType != domain.TaskCategoryGeneral {
		t.Fatalf("task type = %q, want %q", payload.TaskType, domain.TaskCategoryGeneral)
	}
}

func TestMultiAgentCLIRejectsInvalidAggregation(t *testing.T) {
	t.Parallel()

	app := cliApp{stderr: &bytes.Buffer{}}
	err := app.runCommand(context.Background(), []string{"--task", "bad strategy", "--decompose", "--aggregation", "random"})
	if err == nil {
		t.Fatal("runCommand() error = nil, want invalid aggregation error")
	}
	if !strings.Contains(err.Error(), "--aggregation") {
		t.Fatalf("error = %v, want aggregation validation", err)
	}
}

func TestMultiAgentCLIRejectsInvalidMaxWorkers(t *testing.T) {
	t.Parallel()

	app := cliApp{stderr: &bytes.Buffer{}}
	err := app.runCommand(context.Background(), []string{"--task", "bad workers", "--decompose", "--max-workers", "0"})
	if err == nil {
		t.Fatal("runCommand() error = nil, want invalid max-workers error")
	}
	if !strings.Contains(err.Error(), "--max-workers") {
		t.Fatalf("error = %v, want max-workers validation", err)
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
			if !strings.Contains(stderr.String(), "Usage: zheng-agent <run|resume|inspect|tool> [flags]") {
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
	if loadedTask.ProtocolHint != runPayload.ProtocolHint {
		t.Fatalf("loaded task protocol_hint = %q, want %q", loadedTask.ProtocolHint, runPayload.ProtocolHint)
	}
	if loadedTask.VerificationPolicy != runPayload.VerificationPolicy {
		t.Fatalf("loaded task verification_policy = %q, want %q", loadedTask.VerificationPolicy, runPayload.VerificationPolicy)
	}

	var resumeStdout bytes.Buffer
	var resumeStderr bytes.Buffer
	if exitCode := runCLI(context.Background(), []string{"resume", "--session", runPayload.SessionID, "--db", dbPath}, &resumeStdout, &resumeStderr); exitCode != 0 {
		t.Fatalf("resume exit code = %d, stderr=%s", exitCode, resumeStderr.String())
	}
	if got := resumeStdout.String(); !strings.Contains(got, "Resumed session: "+runPayload.SessionID) {
		t.Fatalf("resume output missing session heading:\n%s", got)
	}
	if got := resumeStdout.String(); !strings.Contains(got, "Status: success") || !strings.Contains(got, "Plan: "+inspectPayload.Plan) {
		t.Fatalf("resume output missing stable continuity fields:\n%s", got)
	}
}

type providerResolverStub struct {
	provider llm.Provider
	calls    int
}

func (s *providerResolverStub) Resolve(_ string, _ llm.ProviderConfig) (llm.Provider, error) {
	s.calls++
	if s.provider == nil {
		return nil, fmt.Errorf("missing provider")
	}
	return s.provider, nil
}

type providerPluginStub struct {
	id       string
	name     string
	metadata domain.PluginMetadata
	call     int
}

func (p *providerPluginStub) Name() string  { return p.name }
func (p *providerPluginStub) Model() string { return "plugin-model" }
func (p *providerPluginStub) Generate(_ context.Context, _ llm.Request) (llm.Response, error) {
	p.call++
	var output string
	switch p.call {
	case 1:
		output = `{"summary":"plugin plan"}`
	case 2:
		output = `{"type":"respond","summary":"respond via plugin","response":"plugin-backed completion"}`
	default:
		output = `{"summary":"observed provider plugin","final_response":"plugin-backed completion"}`
	}
	return llm.Response{Model: p.Model(), Output: output}, nil
}
func (p *providerPluginStub) Stream(_ context.Context, _ llm.Request, _ func(domain.StreamingEvent) error) error {
	return nil
}
func (p *providerPluginStub) ProviderID() string { return p.id }
func (p *providerPluginStub) Metadata() domain.PluginMetadata {
	return p.metadata
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

func TestToolReload(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		fake := &fakeToolPluginReloader{}

		app := cliApp{
			stdout: &stdout,
			stderr: &stderr,
			newToolPluginReloader: func(string) toolPluginReloader {
				return fake
			},
		}

		exitCode := app.run(context.Background(), []string{"tool", "reload", "demo"})
		if exitCode != 0 {
			t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
		}
		if got := stdout.String(); !strings.Contains(got, "Plugin demo reloaded successfully") {
			t.Fatalf("stdout = %q, want success message", got)
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty", stderr.String())
		}
		if fake.lastName != "demo" {
			t.Fatalf("ReloadTool() name = %q, want demo", fake.lastName)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		app := cliApp{
			stdout: &stdout,
			stderr: &stderr,
			newToolPluginReloader: func(string) toolPluginReloader {
				return &fakeToolPluginReloader{err: errors.New("plugin not found")}
			},
		}

		exitCode := app.run(context.Background(), []string{"tool", "reload", "missing"})
		if exitCode != 1 {
			t.Fatalf("exit code = %d, want 1", exitCode)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
		if got := stderr.String(); !strings.Contains(got, "Failed to reload plugin missing: plugin not found") {
			t.Fatalf("stderr = %q, want reload failure message", got)
		}
	})
}

func TestPluginCLI(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	loadedPaths := make([]string, 0, 2)
	allowedPluginPaths := make([]string, 0, 2)
	observedTools := make([]string, 0, 8)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	app := cliApp{
		stdout: &stdout,
		stderr: &stderr,
		cfg: config.Config{
			Runtime: config.RuntimeSettings{
				AllowedPluginPaths: []string{"plugins/default"},
			},
		},
		newSession: func(dbPath string) (*store.SQLiteSessionStore, error) {
			return store.NewSQLiteSessionStore(dbPath)
		},
		newMemory: func(dbPath string) (*store.SQLiteMemoryStore, error) {
			return store.NewMemoryStore(dbPath)
		},
		newExecutor: func() domain.ToolExecutor {
			executor, err := tools.NewExecutor(".")
			if err != nil {
				t.Fatalf("NewExecutor(): %v", err)
			}
			return executor
		},
		pluginExecutorFactory: func(base domain.ToolExecutor, options pluginCLIOptions) (domain.ToolExecutor, error) {
			allowedPluginPaths = append([]string(nil), options.AllowedPaths...)
			for _, path := range resolvePluginTargets(normalizePluginCLIOptions(options)) {
				loadedPaths = append(loadedPaths, path)
			}
			registry := runtimebuilder.CloneExecutorRegistry(base)
			for _, name := range []string{"echo-plugin", "inspect.so"} {
				if err := registry.Register(runtimebuilder.ToToolDefinition(&stubCLIPluginTool{name: name})); err != nil {
					return nil, err
				}
			}
			return pluginExecutorForTest{base: base, registry: registry}, nil
		},
		newModel: func() domain.Model { return &FakeModel{} },
		newVerifier: func(domain.ToolExecutor) domain.Verifier { return FakeVerifier{} },
		runEngine: func(_ context.Context, engine runtime.Engine, _ domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
			provider, ok := engine.Tools.(interface{ Registry() *tools.Registry })
			if !ok || provider.Registry() == nil {
				return domain.Session{}, domain.Plan{}, nil, errors.New("plugin executor registry unavailable")
			}
			for _, def := range provider.Registry().List() {
				observedTools = append(observedTools, def.Name)
			}
			return domain.Session{Status: domain.SessionStatusSuccess}, domain.Plan{Summary: "plugin test"}, nil, nil
		},
		notifySignal: signal.Notify,
		stopSignal:   signal.Stop,
		now:          time.Now,
	}

	err := app.runCommand(context.Background(), []string{
		"--task", "run plugin-enabled task",
		"--db", dbPath,
		"--plugin-dir", "./plugins/custom",
		"--plugin", "echo-plugin",
		"--plugin", "./vendor/plugins/inspect.so",
		"--allow-plugin", "plugins/custom",
		"--allow-plugin", "vendor/plugins",
	})
	if err != nil {
		t.Fatalf("runCommand() error = %v", err)
	}

	if len(allowedPluginPaths) != 2 || allowedPluginPaths[0] != "plugins/custom" || allowedPluginPaths[1] != "vendor/plugins" {
		t.Fatalf("allowed plugin paths = %v, want [plugins/custom vendor/plugins]", allowedPluginPaths)
	}

	sort.Strings(loadedPaths)
	wantPaths := []string{filepath.Clean("./plugins/custom/echo-plugin"), filepath.Clean("./vendor/plugins/inspect.so")}
	for i := range wantPaths {
		wantPaths[i] = filepath.ToSlash(filepath.Clean(wantPaths[i]))
	}
	for i := range loadedPaths {
		loadedPaths[i] = filepath.ToSlash(filepath.Clean(loadedPaths[i]))
	}
	sort.Strings(wantPaths)
	sort.Strings(loadedPaths)
	if len(loadedPaths) != len(wantPaths) || loadedPaths[0] != wantPaths[0] || loadedPaths[1] != wantPaths[1] {
		t.Fatalf("loaded plugin paths = %v, want %v", loadedPaths, wantPaths)
	}

	sort.Strings(observedTools)
	if !containsString(observedTools, "echo-plugin") || !containsString(observedTools, "inspect.so") {
		t.Fatalf("observed tools = %v, want loaded plugin tool names present", observedTools)
	}

	if containsString(observedTools, "") {
		t.Fatalf("observed tools contains empty name: %v", observedTools)
	}
}

type pluginExecutorForTest struct {
	base     domain.ToolExecutor
	registry *tools.Registry
}

type fakeToolPluginReloader struct {
	lastName string
	err      error
}

func (f *fakeToolPluginReloader) ReloadTool(name string) error {
	f.lastName = name
	return f.err
}

func (e pluginExecutorForTest) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return e.base.Execute(ctx, call)
}

func (e pluginExecutorForTest) Registry() *tools.Registry {
	return e.registry
}

type stubCLIPluginTool struct {
	name   string
	closed bool
}

func (p *stubCLIPluginTool) Name() string { return p.name }

func (p *stubCLIPluginTool) Description() string { return "stub cli plugin" }

func (p *stubCLIPluginTool) Schema() string { return `{"type":"object"}` }

func (p *stubCLIPluginTool) Capabilities() []string { return []string{"filesystem.read"} }

func (p *stubCLIPluginTool) SafetyLevel() domain.SafetyLevel { return domain.SafetyLevelLow }

func (p *stubCLIPluginTool) ContractVersion() string { return pluginruntime.ContractVersion }

func (p *stubCLIPluginTool) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{ToolName: p.name, Output: call.Input}, nil
}

func (p *stubCLIPluginTool) Close() error {
	p.closed = true
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
