package runtimebuilder

import (
	"context"
	"testing"

	"zheng-harness/internal/config"
	"zheng-harness/internal/store"
)

func TestBuildServerDependencies(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	builder, err := New(cfg, Options{WorkspaceRoot: "."})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	serverCfg := ServerConfig{JWTSecret: "test-token", EnableWAL: true}
	if err := serverCfg.Validate(); err != nil {
		t.Fatalf("ServerConfig.Validate() error = %v", err)
	}

	dbPath := t.TempDir() + "/server.db"
	sessionStore, err := builder.NewSessionStore(dbPath, StoreOptions{EnableWAL: serverCfg.EnableWAL})
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}
	defer func() { _ = sessionStore.Close() }()
	memoryStore, err := builder.NewMemoryStore(dbPath, StoreOptions{EnableWAL: serverCfg.EnableWAL})
	if err != nil {
		t.Fatalf("NewMemoryStore() error = %v", err)
	}
	defer func() { _ = memoryStore.Close() }()
	executor, err := builder.NewExecutor(ExecutorOptions{})
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}
	engine := builder.BuildEngine(EngineOptions{
		Tools:         executor,
		Memory:        memoryStore,
		Sessions:      sessionStore,
		SessionAlias:  "server-session",
		MaxSteps:      3,
		PersistentCtx: context.Background(),
	})
	if engine.Tools == nil || engine.Memory == nil || engine.Sessions == nil {
		t.Fatalf("engine dependencies not assembled: %+v", engine)
	}
	if engine.MaxSteps != 3 || engine.MaxRetries != 3 {
		t.Fatalf("engine step limits = (%d,%d), want (3,3)", engine.MaxSteps, engine.MaxRetries)
	}
	if engine.EventChannel != nil {
		t.Fatalf("server builder should not require CLI stream coupling")
	}
	if _, ok := engine.Sessions.(SessionAliasStore); !ok {
		t.Fatalf("engine sessions = %T, want shared alias store", engine.Sessions)
	}
	if builder.NewModel() == nil {
		// default config intentionally supports fake-model/no-provider startup for host-owned assembly.
	} else {
		t.Fatalf("default builder model should stay nil without provider initialization")
	}
	if builder.NewVerifier(executor) != nil {
		// standard mode still provides a verifier.
	} else {
		t.Fatalf("shared builder verifier must be assembled")
	}
	if got := journalMode(t, sessionStore); got != "wal" {
		t.Fatalf("journal_mode = %q, want wal", got)
	}
}

func TestLoadCLIConfigPreservesRunResumePrecedenceScope(t *testing.T) {
	t.Parallel()

	cfg, err := LoadCLIConfig("run", []string{"--verify-mode", config.VerifyModeOff, "--task", "ignored"})
	if err != nil {
		t.Fatalf("LoadCLIConfig() error = %v", err)
	}
	if cfg.Runtime.VerifyMode != config.VerifyModeOff {
		t.Fatalf("verify mode = %q, want %q", cfg.Runtime.VerifyMode, config.VerifyModeOff)
	}
	if FilterConfigArgs([]string{"--task", "x", "--provider", "openai", "--db", "agent.db"})[0] != "--provider" {
		t.Fatalf("FilterConfigArgs should drop non-config flags")
	}
}

func journalMode(t *testing.T, sessionStore *store.SQLiteSessionStore) string {
	t.Helper()
	var mode string
	if err := sessionStore.RawDB().QueryRow(`PRAGMA journal_mode;`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode scan error = %v", err)
	}
	return mode
}
