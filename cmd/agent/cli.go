package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	"zheng-harness/internal/orchestration"
	pluginruntime "zheng-harness/internal/plugin"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
	"zheng-harness/internal/store"
	"zheng-harness/internal/tools"
)

const defaultDBPath = "./agent.db"

const (
	defaultMultiAgentMaxWorkers = 4
	aggregationFlagAllSucceed   = "all-succeed"
	aggregationFlagBestEffort   = "best-effort"
)

type cliApp struct {
	stdout       io.Writer
	stderr       io.Writer
	cfg          config.Config
	builder      *runtimebuilder.Builder
	newSession   func(string) (*store.SQLiteSessionStore, error)
	newMemory    func(string) (*store.SQLiteMemoryStore, error)
	newExecutor  func() domain.ToolExecutor
	newPluginManager func(string) *pluginruntime.PluginManager
	newToolPluginReloader func(string) toolPluginReloader
	pluginExecutorFactory func(domain.ToolExecutor, pluginCLIOptions) (domain.ToolExecutor, error)
	newModel     func() domain.Model
	newVerifier  func(domain.ToolExecutor) domain.Verifier
	runEngine     func(context.Context, runtime.Engine, domain.Task) (domain.Session, domain.Plan, []domain.Step, error)
	runStreamEngine func(context.Context, runtime.Engine, domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error)
	runMultiAgent func(context.Context, runtime.Engine, domain.Task, multiAgentOptions) (domain.Session, domain.Plan, []domain.Step, error)
	notifySignal func(chan<- os.Signal, ...os.Signal)
	stopSignal   func(chan<- os.Signal)
	now          func() time.Time
}

type streamResult struct {
	session domain.Session
	plan    domain.Plan
	steps   []domain.Step
	err     error
}

type streamConsumer struct {
	stdout         io.Writer
	stderr         io.Writer
	jsonMode       bool
	sessionID      string
	toolStartedAt  map[string]time.Time
	hasInlineToken bool
}

type runJSONOutput struct {
	Command            string               `json:"command"`
	SessionID          string               `json:"session_id"`
	Status             domain.SessionStatus `json:"status"`
	TaskInput          string               `json:"task_input"`
	Plan               string               `json:"plan"`
	Steps              int                  `json:"steps"`
	TaskType           domain.TaskCategory  `json:"task_type,omitempty"`
	ProtocolHint       string               `json:"protocol_hint,omitempty"`
	VerificationPolicy string               `json:"verification_policy,omitempty"`
}

type inspectJSONOutput struct {
	Command            string               `json:"command"`
	SessionID          string               `json:"session_id"`
	Status             domain.SessionStatus `json:"status"`
	TerminatedReason   string               `json:"terminated_reason,omitempty"`
	Plan               string               `json:"plan"`
	StepCount          int                  `json:"step_count"`
	StepSummaries      []string             `json:"step_summaries"`
	Provenance         *domain.Provenance   `json:"provenance,omitempty"`
	TaskType           domain.TaskCategory  `json:"task_type,omitempty"`
	ProtocolHint       string               `json:"protocol_hint,omitempty"`
	VerificationPolicy string               `json:"verification_policy,omitempty"`
}

type taskMetadataFlags struct {
	TaskType           domain.TaskCategory
	ProtocolHint       string
	VerificationPolicy string
}

type pluginCLIOptions struct {
	DiscoveryDir string
	Names        []string
	AllowedPaths []string
}

type multiAgentOptions struct {
	Decompose  bool
	MaxWorkers int
	Aggregation string
}

type toolPluginReloader interface {
	ReloadTool(name string) error
}

type providerMetadataResolver interface {
	Get(id string) (pluginruntime.ProviderDescriptor, bool)
}

type verifierMetadataResolver interface {
	GetRegistration(id string) (pluginruntime.VerifierRegistration, bool)
}

type agentStrategyMetadataResolver interface {
	Get(id string) (pluginruntime.AgentStrategyDescriptor, bool)
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("allow-command must not be empty")
	}
	*s = append(*s, trimmed)
	return nil
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg := config.Default()
	if len(args) > 0 {
		loaded, err := runtimebuilder.LoadCLIConfig(args[0], args[1:])
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		cfg = loaded
	}
	builder, err := runtimebuilder.New(cfg, runtimebuilder.Options{WorkspaceRoot: ".", NewPluginManager: pluginruntime.NewManager})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	app := cliApp{
		stdout: stdout,
		stderr: stderr,
		cfg:    cfg,
		builder: builder,
		newSession: func(dbPath string) (*store.SQLiteSessionStore, error) {
			return builder.NewSessionStore(dbPath, runtimebuilder.StoreOptions{})
		},
		newMemory: func(dbPath string) (*store.SQLiteMemoryStore, error) {
			return builder.NewMemoryStore(dbPath, runtimebuilder.StoreOptions{})
		},
		newExecutor: func() domain.ToolExecutor {
			executor, err := builder.NewExecutor(runtimebuilder.ExecutorOptions{})
			if err != nil {
				return FakeToolExecutor{}
			}
			return executor
		},
		newPluginManager: pluginruntime.NewManager,
		newToolPluginReloader: func(path string) toolPluginReloader {
			return pluginruntime.NewManager(path)
		},
		newModel: func() domain.Model {
			if model := builder.NewModel(); model != nil {
				return model
			}
			return &FakeModel{}
		},
		newVerifier: func(executor domain.ToolExecutor) domain.Verifier {
			return newVerifierFromConfig(cfg, executor)
		},
		runEngine: func(ctx context.Context, engine runtime.Engine, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
			return engine.Run(ctx, task)
		},
		runStreamEngine: func(ctx context.Context, engine runtime.Engine, task domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
			return engine.RunStream(ctx, task)
		},
		notifySignal: signal.Notify,
		stopSignal:   signal.Stop,
		now:          time.Now,
	}
	return app.run(ctx, args)
}

func newVerifierFromConfig(cfg config.Config, executor domain.ToolExecutor) domain.Verifier {
	verifier := runtimebuilder.NewVerifierFromConfig(cfg, executor)
	if verifier == nil {
		return FakeVerifier{}
	}
	return verifier
}

func filterConfigArgs(args []string) []string {
	return runtimebuilder.FilterConfigArgs(args)
}

func (a cliApp) ensureBuilder() *runtimebuilder.Builder {
	if a.builder != nil {
		return a.builder
	}
	builder, err := runtimebuilder.New(a.cfg, runtimebuilder.Options{WorkspaceRoot: ".", NewPluginManager: a.newPluginManager})
	if err != nil {
		return nil
	}
	return builder
}

func (a cliApp) run(ctx context.Context, args []string) int {
	if len(args) == 0 || isRootHelpArg(args[0]) {
		a.printUsage()
		return 0
	}

	switch args[0] {
	case "run":
		if err := a.runCommand(ctx, args[1:]); err != nil {
			_, _ = fmt.Fprintln(a.stderr, err)
			return 1
		}
		return 0
	case "resume":
		if err := a.resumeCommand(ctx, args[1:]); err != nil {
			_, _ = fmt.Fprintln(a.stderr, err)
			return 1
		}
		return 0
	case "inspect":
		if err := a.inspectCommand(ctx, args[1:]); err != nil {
			_, _ = fmt.Fprintln(a.stderr, err)
			return 1
		}
		return 0
	case "tool":
		if err := a.toolCommand(ctx, args[1:]); err != nil {
			_, _ = fmt.Fprintln(a.stderr, err)
			return 1
		}
		return 0
	default:
		a.printUsage()
		_, _ = fmt.Fprintf(a.stderr, "unknown subcommand %q\n", args[0])
		return 1
	}
}

func isRootHelpArg(arg string) bool {
	trimmed := strings.TrimSpace(arg)
	return trimmed == "-h" || trimmed == "--help" || trimmed == "help"
}

func (a cliApp) toolCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || isRootHelpArg(args[0]) {
		return errors.New("tool requires subcommand")
	}

	switch args[0] {
	case "reload":
		return a.toolReloadCommand(ctx, args[1:])
	default:
		return fmt.Errorf("unknown tool subcommand %q", args[0])
	}
}

func (a cliApp) toolReloadCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tool reload", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	pluginDir := fs.String("plugin-dir", "./plugins", "plugin discovery directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("tool reload requires <plugin-name>")
	}

	name := strings.TrimSpace(fs.Arg(0))
	if name == "" {
		return errors.New("tool reload requires <plugin-name>")
	}

	factory := a.newToolPluginReloader
	if factory == nil {
		factory = func(path string) toolPluginReloader {
			if a.newPluginManager != nil {
				return a.newPluginManager(path)
			}
			return pluginruntime.NewManager(path)
		}
	}
	reloader := factory(*pluginDir)
	if reloader == nil {
		return fmt.Errorf("Failed to reload plugin %s: plugin manager is not configured", name)
	}
	if manager, ok := reloader.(*pluginruntime.PluginManager); ok {
		if err := primeReloadTarget(ctx, manager, name); err != nil {
			return fmt.Errorf("Failed to reload plugin %s: %w", name, err)
		}
	}
	if err := reloader.ReloadTool(name); err != nil {
		return fmt.Errorf("Failed to reload plugin %s: %w", name, err)
	}
	_, _ = fmt.Fprintf(a.stdout, "Plugin %s reloaded successfully\n", name)
	return nil
}

func (a cliApp) runCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	// 这里定义占位的配置标志，使 CLI 能无报错地接收它们。
	// 实际的配置加载会在调用此函数之前于 runCLI 中完成。
	_ = fs.String("config", "", "config file path (unused in subcommand)")
	_ = fs.String("provider", "", "built-in provider id (unused in subcommand)")
	_ = fs.String("plugin-provider", "", "plugin-backed provider id (unused in subcommand)")
	_ = fs.String("model", "", "model (unused in subcommand)")
	_ = fs.String("api-key", "", "API key (unused in subcommand)")
	_ = fs.String("base-url", "", "base URL (unused in subcommand)")
	_ = fs.String("step-timeout", "", "step timeout (unused in subcommand)")
	_ = fs.Int("memory-limit-mb", 0, "memory limit mb (unused in subcommand)")
	_ = fs.String("verify-mode", "", "verify mode (unused in subcommand)")
	var allowCommands stringSliceFlag
	fs.Var(&allowCommands, "allow-command", "additional allowed command (repeatable)")
	var plugins stringSliceFlag
	var allowedPlugins stringSliceFlag
	pluginDir := fs.String("plugin-dir", "./plugins", "plugin discovery directory")
	fs.Var(&plugins, "plugin", "plugin name or path to load (repeatable)")
	fs.Var(&allowedPlugins, "allow-plugin", "explicitly allowed plugin path (repeatable)")
	taskInput := fs.String("task", "", "task description")
	taskType := fs.String("task-type", "", "optional general task type (coding, research, file_workflow, general)")
	protocolHint := fs.String("task-protocol", "", "optional task protocol hint")
	verificationPolicy := fs.String("task-verification-policy", "", "optional task verification policy")
	dbPath := fs.String("db", defaultDBPath, "sqlite database path")
	maxSteps := fs.Int("max-steps", a.defaultMaxSteps(), "maximum runtime steps")
	jsonMode := fs.Bool("json", false, "emit machine-readable JSON")
	streamMode := fs.Bool("stream", false, "stream runtime events to stdout")
	decompose := fs.Bool("decompose", false, "enable multi-agent task decomposition")
	maxWorkers := fs.Int("max-workers", defaultMultiAgentMaxWorkers, "maximum concurrent multi-agent workers")
	aggregation := fs.String("aggregation", aggregationFlagAllSucceed, "multi-agent aggregation strategy (all-succeed|best-effort)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*taskInput) == "" {
		return errors.New("run requires --task")
	}
	if *maxSteps <= 0 {
		return errors.New("run requires --max-steps > 0")
	}
	multiAgent, err := normalizeMultiAgentOptions(*decompose, *maxWorkers, *aggregation)
	if err != nil {
		return err
	}
	if len(allowCommands) > 0 {
		a = a.withExtraAllowedCommands([]string(allowCommands))
	}
	if len(plugins) > 0 {
		updatedApp, err := a.withPluginOptions(pluginCLIOptions{
			DiscoveryDir: strings.TrimSpace(*pluginDir),
			Names:        []string(plugins),
			AllowedPaths: []string(allowedPlugins),
		})
		if err != nil {
			return err
		}
		a = updatedApp
	}

	now := a.now()
	sessionID := fmt.Sprintf("session-%d", now.UnixNano())
	sessionStore, memoryStore, cleanup, err := a.openRuntimeDeps(*dbPath)
	if err != nil {
		return err
	}
	defer cleanup()
	aliasStore := newSessionAliasStore(sessionStore, ctx, sessionID)

	if err := aliasStore.SaveSession(ctx, domain.Session{
		ID:        sessionID,
		TaskID:    sessionID,
		Status:    domain.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("save initial session: %w", err)
	}

	executor := a.newExecutor()
	if closer, ok := executor.(interface{ Close() error }); ok {
		defer func() { _ = closer.Close() }()
	}
	builder := a.ensureBuilder()
	engine := runtime.Engine{}
	if builder != nil {
		engine = builder.BuildEngine(runtimebuilder.EngineOptions{
			Model:         a.newModel(),
			Tools:         executor,
			Memory:        memoryStore,
			Sessions:      aliasStore,
			Verifier:      a.newVerifier(executor),
			MaxSteps:      *maxSteps,
			PersistentCtx: ctx,
		})
	} else {
		engine = runtime.Engine{Model: a.newModel(), Tools: executor, Memory: memoryStore, Sessions: aliasStore, Verifier: a.newVerifier(executor), MaxSteps: *maxSteps, MaxRetries: *maxSteps, SessionTimeout: a.sessionTimeout(*maxSteps)}
	}

	runCtx, stop := a.withSignalCancellation(ctx)
	defer stop()

	task := domain.Task{
		ID:          sessionID,
		Description: strings.TrimSpace(*taskInput),
		Goal:        strings.TrimSpace(*taskInput),
		Category:    domain.TaskCategory(strings.TrimSpace(*taskType)),
		ProtocolHint: strings.TrimSpace(*protocolHint),
		VerificationPolicy: strings.TrimSpace(*verificationPolicy),
		CreatedAt:   now,
	}
	task = normalizeTaskMetadata(task)
	if err := sessionStore.SaveTask(ctx, sessionID, task); err != nil {
		return fmt.Errorf("save task metadata: %w", err)
	}

	if *streamMode {
		result, err := a.runStreamingCommand(runCtx, engine, task, *jsonMode, sessionStore, sessionID)
		if err != nil {
			return err
		}
		_ = result
		return nil
	}

	session, plan, steps, runErr := a.executeTask(runCtx, engine, task, multiAgent)
	session.ID = sessionID
	if ctxErr := runCtx.Err(); ctxErr != nil && session.Status != domain.SessionStatusSuccess {
		session.Status = domain.SessionStatusInterrupted
		if session.TaskID == "" {
			session.TaskID = sessionID
		}
		_ = aliasStore.SaveSession(ctx, session)
	} else if session.Status == "" || session.Status == domain.SessionStatusRunning {
		if runErr != nil {
			session.Status = domain.SessionStatusFatalError
			if session.TaskID == "" {
				session.TaskID = sessionID
			}
			_ = aliasStore.SaveSession(ctx, session)
		}
	}

	a.emitRunResult(*jsonMode, task, session, plan, steps)
	if runErr != nil {
		return runErr
	}
	return nil
}

func (a cliApp) resumeCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	// 这里定义占位的 --config 和 --provider 标志，使 CLI 能无报错地接收它们。
	// 实际的配置加载会在调用此函数之前于 runCLI 中完成。
	_ = fs.String("config", "", "config file path (unused in subcommand)")
	_ = fs.String("provider", "", "provider (unused in subcommand)")
	_ = fs.String("model", "", "model (unused in subcommand)")
	_ = fs.String("api-key", "", "API key (unused in subcommand)")
	_ = fs.String("base-url", "", "base URL (unused in subcommand)")
	_ = fs.String("step-timeout", "", "step timeout (unused in subcommand)")
	_ = fs.Int("memory-limit-mb", 0, "memory limit mb (unused in subcommand)")
	_ = fs.String("verify-mode", "", "verify mode (unused in subcommand)")
	var allowCommands stringSliceFlag
	fs.Var(&allowCommands, "allow-command", "additional allowed command (repeatable)")
	var plugins stringSliceFlag
	var allowedPlugins stringSliceFlag
	pluginDir := fs.String("plugin-dir", "./plugins", "plugin discovery directory")
	fs.Var(&plugins, "plugin", "plugin name or path to load (repeatable)")
	fs.Var(&allowedPlugins, "allow-plugin", "explicitly allowed plugin path (repeatable)")
	sessionID := fs.String("session", "", "session identifier")
	dbPath := fs.String("db", defaultDBPath, "sqlite database path")
	maxSteps := fs.Int("max-steps", a.defaultMaxSteps(), "maximum runtime steps")
	jsonMode := fs.Bool("json", false, "emit machine-readable JSON")
	streamMode := fs.Bool("stream", false, "stream runtime events to stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" {
		return errors.New("resume requires --session")
	}
	if len(allowCommands) > 0 {
		a = a.withExtraAllowedCommands([]string(allowCommands))
	}
	if len(plugins) > 0 {
		updatedApp, err := a.withPluginOptions(pluginCLIOptions{
			DiscoveryDir: strings.TrimSpace(*pluginDir),
			Names:        []string(plugins),
			AllowedPaths: []string(allowedPlugins),
		})
		if err != nil {
			return err
		}
		a = updatedApp
	}
	if *maxSteps <= 0 {
		return errors.New("resume requires --max-steps > 0")
	}

	sessionStore, memoryStore, cleanup, err := a.openRuntimeDeps(*dbPath)
	if err != nil {
		return err
	}
	defer cleanup()

	session, plan, steps, err := sessionStore.ResumeSession(ctx, *sessionID)
	if err != nil {
		return err
	}

	if !*streamMode {
		a.emitResumeResult(session, plan, steps, false)
	}
	if isTerminalStatus(session.Status) {
		if *streamMode {
			a.emitResumeResult(session, plan, steps, false)
		}
		return nil
	}

	executor := a.newExecutor()
	if closer, ok := executor.(interface{ Close() error }); ok {
		defer func() { _ = closer.Close() }()
	}
	if err := validateResumePluginRequirements(session, a.cfg, executor); err != nil {
		return err
	}
	builder := a.ensureBuilder()
	engine := runtime.Engine{}
	if builder != nil {
		engine = builder.BuildEngine(runtimebuilder.EngineOptions{
			Model:         a.newModel(),
			Tools:         executor,
			Memory:        memoryStore,
			Sessions:      newSessionAliasStore(sessionStore, ctx, *sessionID),
			Verifier:      a.newVerifier(executor),
			MaxSteps:      *maxSteps,
			PersistentCtx: ctx,
		})
	} else {
		engine = runtime.Engine{Model: a.newModel(), Tools: executor, Memory: memoryStore, Sessions: newSessionAliasStore(sessionStore, ctx, *sessionID), Verifier: a.newVerifier(executor), MaxSteps: *maxSteps, MaxRetries: *maxSteps, SessionTimeout: a.sessionTimeout(*maxSteps)}
	}

	runCtx, stop := a.withSignalCancellation(ctx)
	defer stop()

	continuedTask, _, taskErr := sessionStore.LoadTask(ctx, *sessionID)
	if taskErr != nil {
		return taskErr
	}

	if *streamMode {
		_, err := a.runStreamingCommand(runCtx, engine, continuedTask, *jsonMode, sessionStore, *sessionID)
		return err
	}

	session, plan, steps, err = a.executeRun(runCtx, engine, continuedTask)
	aliasStore := newSessionAliasStore(sessionStore, ctx, *sessionID)
	session.ID = *sessionID
	if ctxErr := runCtx.Err(); ctxErr != nil && session.Status != domain.SessionStatusSuccess {
		session.Status = domain.SessionStatusInterrupted
		if session.TaskID == "" {
			session.TaskID = continuedTask.ID
		}
		_ = aliasStore.SaveSession(ctx, session)
	} else if session.Status == "" || session.Status == domain.SessionStatusRunning {
		if err != nil {
			session.Status = domain.SessionStatusFatalError
			if session.TaskID == "" {
				session.TaskID = continuedTask.ID
			}
			_ = aliasStore.SaveSession(ctx, session)
		}
	}
	a.emitResumeResult(session, plan, steps, true)
	if err != nil {
		return err
	}
	return nil
}

func (a cliApp) inspectCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	// 这里定义占位的 --config 和 --provider 标志，使 CLI 能无报错地接收它们。
	// 实际的配置加载会在调用此函数之前于 runCLI 中完成。
	_ = fs.String("config", "", "config file path (unused in subcommand)")
	_ = fs.String("provider", "", "provider (unused in subcommand)")
	_ = fs.String("model", "", "model (unused in subcommand)")
	_ = fs.String("api-key", "", "API key (unused in subcommand)")
	_ = fs.String("base-url", "", "base URL (unused in subcommand)")
	_ = fs.String("step-timeout", "", "step timeout (unused in subcommand)")
	_ = fs.Int("memory-limit-mb", 0, "memory limit mb (unused in subcommand)")
	_ = fs.String("verify-mode", "", "verify mode (unused in subcommand)")
	sessionID := fs.String("session", "", "session identifier")
	dbPath := fs.String("db", defaultDBPath, "sqlite database path")
	jsonMode := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" {
		return errors.New("inspect requires --session")
	}

	sessionStore, err := a.newSession(*dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = sessionStore.Close() }()

	session, plan, steps, err := sessionStore.ResumeSession(ctx, *sessionID)
	if err != nil {
		return err
	}
	task, _, taskErr := sessionStore.LoadTask(ctx, *sessionID)
	if taskErr != nil {
		return taskErr
	}
	a.emitInspectResult(*jsonMode, task, session, plan, steps)
	return nil
}

func primeReloadTarget(ctx context.Context, manager *pluginruntime.PluginManager, name string) error {
	if manager == nil {
		return errors.New("plugin manager is nil")
	}
	if existing, ok := manager.LoadedPlugins[name]; ok && existing != nil {
		return nil
	}
	plugins, err := manager.Discover()
	if err != nil {
		return err
	}
	for _, plugin := range plugins {
		if filepath.Base(plugin.Path) != name {
			continue
		}
		_, err := manager.Load(ctx, plugin.Path)
		return err
	}
	return errors.New("plugin not found")
}

func (a cliApp) openRuntimeDeps(dbPath string) (*store.SQLiteSessionStore, *store.SQLiteMemoryStore, func(), error) {
	sessionStore, err := a.newSession(dbPath)
	if err != nil {
		return nil, nil, nil, err
	}
	memoryStore, err := a.newMemory(dbPath)
	if err != nil {
		_ = sessionStore.Close()
		return nil, nil, nil, err
	}
	cleanup := func() {
		_ = sessionStore.Close()
		_ = memoryStore.Close()
	}
	return sessionStore, memoryStore, cleanup, nil
}

func (a cliApp) withExtraAllowedCommands(commands []string) cliApp {
	a.newExecutor = func() domain.ToolExecutor {
		builder := a.ensureBuilder()
		if builder == nil {
			executor, err := tools.NewExecutor(".", tools.WithAllowedCommands(a.cfg.Runtime.AllowedCommands), tools.WithExtraAllowedCommands(commands))
			if err != nil {
				return FakeToolExecutor{}
			}
			return executor
		}
		executor, err := builder.NewExecutor(runtimebuilder.ExecutorOptions{ExtraAllowedCommands: commands})
		if err != nil {
			return FakeToolExecutor{}
		}
		return executor
	}
	return a
}

func (a cliApp) withPluginOptions(options pluginCLIOptions) (cliApp, error) {
	previous := a.newExecutor
	pluginOptions := normalizePluginCLIOptions(options)
	if len(pluginOptions.Names) == 0 {
		return a, nil
	}
	a.newExecutor = func() domain.ToolExecutor {
		base := previous()
		factory := a.pluginExecutorFactory
		if factory == nil {
			factory = a.buildPluginExecutor
		}
		executor, err := factory(base, pluginOptions)
		if err != nil {
			return pluginInitializationErrorExecutor{err: err}
		}
		return executor
	}
	return a, nil
}

func (a cliApp) buildPluginExecutor(base domain.ToolExecutor, options pluginCLIOptions) (domain.ToolExecutor, error) {
	builder := a.ensureBuilder()
	if builder == nil {
		return nil, errors.New("runtime builder is not initialized")
	}
	return builder.WrapExecutorWithPlugins(base, runtimebuilder.PluginOptions(options))
}

func resolvePluginTargets(options pluginCLIOptions) []string {
	return runtimebuilder.ResolvePluginTargets(runtimebuilder.PluginOptions(options))
}

func normalizePluginCLIOptions(options pluginCLIOptions) pluginCLIOptions {
	return pluginCLIOptions(runtimebuilder.NormalizePluginOptions(runtimebuilder.PluginOptions(options)))
}

type pluginInitializationErrorExecutor struct{ err error }

func (e pluginInitializationErrorExecutor) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{ToolName: call.Name}, e.err
}

func (e pluginInitializationErrorExecutor) Registry() *tools.Registry {
	return tools.NewRegistry()
}

func (a cliApp) defaultMaxSteps() int {
	if a.builder != nil {
		return a.builder.DefaultMaxSteps()
	}
	if builder := a.ensureBuilder(); builder != nil {
		return builder.DefaultMaxSteps()
	}
	if a.cfg.Runtime.MaxSteps > 0 {
		return a.cfg.Runtime.MaxSteps
	}
	return 8
}

func (a cliApp) stepTimeout() time.Duration {
	if a.builder != nil {
		return a.builder.StepTimeout()
	}
	if builder := a.ensureBuilder(); builder != nil {
		return builder.StepTimeout()
	}
	if a.cfg.Runtime.StepTimeout > 0 {
		return a.cfg.Runtime.StepTimeout
	}
	return 30 * time.Second
}

func (a cliApp) sessionTimeout(maxSteps int) time.Duration {
	if a.builder != nil {
		return a.builder.SessionTimeout(maxSteps)
	}
	if builder := a.ensureBuilder(); builder != nil {
		return builder.SessionTimeout(maxSteps)
	}
	if maxSteps <= 0 {
		maxSteps = a.defaultMaxSteps()
	}
	return time.Duration(maxSteps) * a.stepTimeout()
}

func (a cliApp) withSignalCancellation(ctx context.Context) (context.Context, func()) {
	runCtx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	a.notifySignal(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-runCtx.Done():
		case <-sigCh:
			cancel()
		}
	}()
	return runCtx, func() {
		a.stopSignal(sigCh)
		cancel()
	}
}

func (a cliApp) emitRunResult(jsonMode bool, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step) {
	metadata := summarizeTaskMetadata(task)
	if jsonMode {
		payload := runJSONOutput{
			Command:            "run",
			SessionID:          session.ID,
			Status:             session.Status,
			TaskInput:          task.Description,
			Plan:               plan.Summary,
			Steps:              len(steps),
			TaskType:           metadata.TaskType,
			ProtocolHint:       metadata.ProtocolHint,
			VerificationPolicy: metadata.VerificationPolicy,
		}
		writeJSON(a.stdout, payload)
		return
	}
	_, _ = fmt.Fprintf(a.stdout, "Session: %s\nStatus: %s\nTask: %s\nPlan: %s\nSteps: %d\n", session.ID, session.Status, task.Description, plan.Summary, len(steps))
}

func (a cliApp) emitResumeResult(session domain.Session, plan domain.Plan, steps []domain.Step, continued bool) {
	heading := "Resumed"
	if continued {
		heading = "Continued"
	}
	_, _ = fmt.Fprintf(a.stdout, "%s session: %s\nStatus: %s\nPlan: %s\nHistory:\n", heading, session.ID, session.Status, plan.Summary)
	for _, line := range summarizeProvenance(session.Provenance) {
		_, _ = fmt.Fprintf(a.stdout, "- %s\n", line)
	}
	for _, line := range summarizeSteps(steps) {
		_, _ = fmt.Fprintf(a.stdout, "- %s\n", line)
	}
	if len(steps) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "- no steps recorded")
	}
}

func (a cliApp) emitInspectResult(jsonMode bool, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step) {
	terminatedReason := deriveTerminationReason(session, steps)
	metadata := summarizeTaskMetadata(task)
	if jsonMode {
		payload := inspectJSONOutput{
			Command:            "inspect",
			SessionID:          session.ID,
			Status:             session.Status,
			TerminatedReason:   terminatedReason,
			Plan:               plan.Summary,
			StepCount:          len(steps),
			StepSummaries:      summarizeSteps(steps),
			Provenance:         session.Provenance,
			TaskType:           metadata.TaskType,
			ProtocolHint:       metadata.ProtocolHint,
			VerificationPolicy: metadata.VerificationPolicy,
		}
		writeJSON(a.stdout, payload)
		return
	}
	_, _ = fmt.Fprintf(a.stdout, "Session: %s\nStatus: %s\nTermination: %s\nPlan: %s\nSummary:\n", session.ID, session.Status, terminatedReason, plan.Summary)
	for _, line := range summarizeProvenance(session.Provenance) {
		_, _ = fmt.Fprintf(a.stdout, "- %s\n", line)
	}
	for _, line := range summarizeSteps(steps) {
		_, _ = fmt.Fprintf(a.stdout, "- %s\n", line)
	}
	if len(steps) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "- no steps recorded")
	}
}

func (a cliApp) printUsage() {
	_, _ = fmt.Fprintln(a.stderr, "Usage: zheng-agent <run|resume|inspect|tool> [flags]")
	_, _ = fmt.Fprintln(a.stderr, "  run --task \"task description\" [--provider <built-in-id> | --plugin-provider <plugin-id>] [--db ./agent.db] [--json] [--stream] [--decompose] [--max-workers 4] [--aggregation all-succeed]")
	_, _ = fmt.Fprintln(a.stderr, "  resume --session <id> [--db ./agent.db] [--json] [--stream]")
	_, _ = fmt.Fprintln(a.stderr, "  inspect --session <id> [--db ./agent.db] [--json]")
	_, _ = fmt.Fprintln(a.stderr, "  tool reload <plugin-name> [--plugin-dir ./plugins]")
	_, _ = fmt.Fprintln(a.stderr, "  provider ids: built-in ids use --provider; plugin-backed ids use --plugin-provider")
	_, _ = fmt.Fprintln(a.stderr, "  --help, -h, help  Show this help")
}

func normalizeMultiAgentOptions(decompose bool, maxWorkers int, aggregation string) (multiAgentOptions, error) {
	normalized := multiAgentOptions{
		Decompose:  decompose,
		MaxWorkers: maxWorkers,
		Aggregation: strings.TrimSpace(aggregation),
	}
	if normalized.MaxWorkers <= 0 {
		return multiAgentOptions{}, errors.New("run requires --max-workers > 0")
	}
	switch normalized.Aggregation {
	case "", aggregationFlagAllSucceed:
		normalized.Aggregation = aggregationFlagAllSucceed
	case aggregationFlagBestEffort:
	default:
		return multiAgentOptions{}, fmt.Errorf("run requires --aggregation to be one of %q or %q", aggregationFlagAllSucceed, aggregationFlagBestEffort)
	}
	return normalized, nil
}

func (a cliApp) executeTask(ctx context.Context, engine runtime.Engine, task domain.Task, options multiAgentOptions) (domain.Session, domain.Plan, []domain.Step, error) {
	if options.Decompose {
		return a.executeMultiAgentRun(ctx, engine, task, options)
	}
	return a.executeRun(ctx, engine, task)
}

func (a cliApp) executeRun(ctx context.Context, engine runtime.Engine, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	if a.runEngine != nil {
		return a.runEngine(ctx, engine, task)
	}
	return engine.Run(ctx, task)
}

func (a cliApp) executeMultiAgentRun(ctx context.Context, engine runtime.Engine, task domain.Task, options multiAgentOptions) (domain.Session, domain.Plan, []domain.Step, error) {
	if a.runMultiAgent != nil {
		return a.runMultiAgent(ctx, engine, task, options)
	}

	decomposition := orchestration.TaskDecomposition{
		TaskID: task.ID,
		Subtasks: []orchestration.Subtask{{
			ID:             task.ID,
			Description:    task.Description,
			ExpectedOutput: task.Goal,
			Status:         orchestration.SubtaskStatusPending,
		}},
	}
	if _, err := orchestration.NewDAGScheduler(decomposition); err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}

	var (
		mu      sync.Mutex
		session domain.Session
		plan    domain.Plan
		steps   []domain.Step
	)
	orch := orchestration.Orchestrator{
		MaxWorkers: options.MaxWorkers,
		WorkerFactory: func(subtask orchestration.Subtask) orchestration.Worker {
			return orchestration.NewWorker(func(workerCtx context.Context, _ orchestration.Subtask, _ orchestration.TaskDecomposition) error {
				runSession, runPlan, runSteps, err := a.executeRun(workerCtx, engine, task)
				mu.Lock()
				session = runSession
				plan = runPlan
				steps = append([]domain.Step(nil), runSteps...)
				mu.Unlock()
				return err
			})
		},
	}
	if err := orch.Start(ctx); err != nil {
		return domain.Session{}, domain.Plan{}, nil, err
	}
	if err := orch.SubmitTask(ctx, decomposition); err != nil {
		orch.Stop()
		_ = orch.Wait()
		return domain.Session{}, domain.Plan{}, nil, err
	}
	orch.Stop()
	waitErr := orch.Wait()

	results := collectWorkerResults(orch.ResultChannel)
	taskResults := make([]orchestration.TaskResult, 0, len(results))
	for _, result := range results {
		verificationStatus := domain.VerificationStatusPassed
		if result.Err != nil || result.Status == orchestration.SubtaskStatusFailed {
			verificationStatus = domain.VerificationStatusFailed
		}
		taskResults = append(taskResults, orchestration.TaskResult{
			SubtaskID:          result.SubtaskID,
			Output:             result.Output,
			Error:              result.Err,
			VerificationStatus: verificationStatus,
		})
	}
	aggregator := &orchestration.Aggregator{Strategy: toAggregationStrategy(options.Aggregation)}
	_, aggregationErr := aggregator.Aggregate(taskResults)
	if waitErr != nil {
		return session, plan, steps, waitErr
	}
	if aggregationErr != nil {
		return session, plan, steps, aggregationErr
	}
	return session, plan, steps, nil
}

func toAggregationStrategy(flagValue string) orchestration.AggregationStrategy {
	switch strings.TrimSpace(flagValue) {
	case aggregationFlagBestEffort:
		return orchestration.AggregationStrategyBestEffort
	default:
		return orchestration.AggregationStrategyAllSucceed
	}
}

func collectWorkerResults(results <-chan orchestration.WorkerResult) []orchestration.WorkerResult {
	collected := make([]orchestration.WorkerResult, 0)
	for result := range results {
		collected = append(collected, result)
	}
	return collected
}

func (a cliApp) executeRunStream(ctx context.Context, engine runtime.Engine, task domain.Task) (*runtime.EventChannel, domain.Session, domain.Plan, []domain.Step, error) {
	if a.runStreamEngine != nil {
		return a.runStreamEngine(ctx, engine, task)
	}
	return engine.RunStream(ctx, task)
}

func (a cliApp) runStreamingCommand(ctx context.Context, engine runtime.Engine, task domain.Task, jsonMode bool, sessionStore *store.SQLiteSessionStore, sessionID string) (streamResult, error) {
	events, _, _, _, err := a.executeRunStream(ctx, engine, task)
	if err != nil {
		return streamResult{}, err
	}
	if events == nil {
		return streamResult{}, errors.New("streaming runtime returned nil event channel")
	}

	consumer := streamConsumer{
		stdout:        a.stdout,
		stderr:        a.stderr,
		jsonMode:      jsonMode,
		sessionID:     sessionID,
		toolStartedAt: make(map[string]time.Time),
	}
	var (
		consumeErr   error
		consumeErrMu sync.Mutex
	)
	setConsumeErr := func(err error) {
		if err == nil {
			return
		}
		consumeErrMu.Lock()
		defer consumeErrMu.Unlock()
		if consumeErr == nil {
			consumeErr = err
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range events.Events() {
			if err := consumer.consume(event); err != nil {
				setConsumeErr(err)
				return
			}
		}
	}()
	wg.Wait()
	if consumeErr != nil {
		return streamResult{}, consumeErr
	}

	result, err := a.loadStreamResult(context.WithoutCancel(ctx), sessionStore, sessionID)
	if err != nil {
		return streamResult{}, err
	}
	if ctxErr := ctx.Err(); ctxErr != nil && result.session.Status != domain.SessionStatusSuccess {
		return result, ctxErr
	}
	return result, result.err
}

func (a cliApp) loadStreamResult(ctx context.Context, sessionStore *store.SQLiteSessionStore, sessionID string) (streamResult, error) {
	session, plan, steps, err := sessionStore.ResumeSession(ctx, sessionID)
	if err != nil {
		return streamResult{}, err
	}
	var resultErr error
	return streamResult{session: session, plan: plan, steps: steps, err: resultErr}, nil
}

func (c *streamConsumer) consume(event domain.StreamingEvent) error {
	if event.Type == domain.EventSessionComplete {
		c.normalizeSessionCompleteEvent(&event)
	}
	if c.jsonMode {
		encoder := json.NewEncoder(c.stdout)
		return encoder.Encode(event)
	}

	switch event.Type {
	case domain.EventTokenDelta:
		var payload domain.TokenDeltaPayload
		if err := event.GetPayload(&payload); err != nil {
			return err
		}
		if _, err := io.WriteString(c.stdout, payload.Content); err != nil {
			return err
		}
		c.hasInlineToken = true
		return nil
	case domain.EventToolStart:
		var payload domain.ToolStartPayload
		if err := event.GetPayload(&payload); err != nil {
			return err
		}
		c.ensureLineBreak()
		c.toolStartedAt[c.toolKey(event.StepIndex, payload.ToolName)] = event.Timestamp
		_, err := fmt.Fprintf(c.stdout, "[Tool: %s]\n", payload.ToolName)
		return err
	case domain.EventToolEnd:
		var payload domain.ToolEndPayload
		if err := event.GetPayload(&payload); err != nil {
			return err
		}
		c.ensureLineBreak()
		duration := c.formatDuration(event.StepIndex, payload.ToolName, event.Timestamp)
		_, err := fmt.Fprintf(c.stdout, "[Tool: %s] done (%s)\n", payload.ToolName, duration)
		return err
	case domain.EventStepComplete:
		var payload domain.StepCompletePayload
		if err := event.GetPayload(&payload); err != nil {
			return err
		}
		c.ensureLineBreak()
		_, err := fmt.Fprintf(c.stdout, "--- Step %d complete ---\n", event.StepIndex)
		return err
	case domain.EventError:
		var payload domain.ErrorPayload
		if err := event.GetPayload(&payload); err != nil {
			return err
		}
		_, err := fmt.Fprintf(c.stderr, "ERROR: %s\n", payload.Message)
		return err
	case domain.EventSessionComplete:
		var payload domain.SessionCompletePayload
		if err := event.GetPayload(&payload); err != nil {
			return err
		}
		c.ensureLineBreak()
		_, err := fmt.Fprintf(c.stdout, "Session: %s\nStatus: %s\n", payload.SessionID, payload.Status)
		return err
	default:
		return nil
	}
}

func (c *streamConsumer) toolKey(stepIndex int, toolName string) string {
	return fmt.Sprintf("%d:%s", stepIndex, toolName)
}

func (c *streamConsumer) formatDuration(stepIndex int, toolName string, endedAt time.Time) string {
	startedAt, ok := c.toolStartedAt[c.toolKey(stepIndex, toolName)]
	if !ok || startedAt.IsZero() || endedAt.Before(startedAt) {
		return "unknown"
	}
	delete(c.toolStartedAt, c.toolKey(stepIndex, toolName))
	return endedAt.Sub(startedAt).String()
}

func (c *streamConsumer) ensureLineBreak() {
	if !c.hasInlineToken {
		return
	}
	_, _ = fmt.Fprintln(c.stdout)
	c.hasInlineToken = false
}

func (c *streamConsumer) normalizeSessionCompleteEvent(event *domain.StreamingEvent) {
	if event == nil || strings.TrimSpace(c.sessionID) == "" {
		return
	}
	var payload domain.SessionCompletePayload
	if err := event.GetPayload(&payload); err != nil {
		return
	}
	payload.SessionID = c.sessionID
	normalized, err := domain.NewStreamingEvent(event.Type, event.StepIndex, payload)
	if err != nil {
		return
	}
	normalized.Timestamp = event.Timestamp
	*event = *normalized
}

func writeJSON(w io.Writer, payload any) {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func summarizeSteps(steps []domain.Step) []string {
	if len(steps) == 0 {
		return nil
	}
	start := 0
	if len(steps) > 3 {
		start = len(steps) - 3
	}
	summaries := make([]string, 0, len(steps)-start)
	for _, step := range steps[start:] {
		summary := step.Action.Summary
		if strings.TrimSpace(summary) == "" {
			summary = step.Observation.Summary
		}
		if strings.TrimSpace(summary) == "" {
			summary = step.Verification.Reason
		}
		summaries = append(summaries, fmt.Sprintf("step %d: %s", step.Index, strings.TrimSpace(summary)))
	}
	return summaries
}

func summarizeProvenance(provenance *domain.Provenance) []string {
	if provenance == nil || len(provenance.Normalize().Plugins) == 0 {
		return nil
	}
	plugins := provenance.Normalize().Plugins
	summaries := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		summaries = append(summaries, fmt.Sprintf("plugin %s/%s via %s (%s)", plugin.Family, plugin.LogicalID, plugin.ExecutionMode, plugin.SourcePath))
	}
	return summaries
}

func normalizeTaskMetadata(task domain.Task) domain.Task {
	task = task.Normalize()
	task.ProtocolHint = strings.TrimSpace(task.ProtocolHint)
	task.VerificationPolicy = strings.TrimSpace(task.VerificationPolicy)
	return task
}

func summarizeTaskMetadata(task domain.Task) taskMetadataFlags {
	normalized := normalizeTaskMetadata(task)
	return taskMetadataFlags{
		TaskType:           normalized.Category,
		ProtocolHint:       normalized.ProtocolHint,
		VerificationPolicy: normalized.VerificationPolicy,
	}
}

func deriveTerminationReason(session domain.Session, steps []domain.Step) string {
	if len(steps) > 0 {
		last := steps[len(steps)-1]
		if strings.TrimSpace(last.Verification.Reason) != "" {
			return last.Verification.Reason
		}
		if strings.TrimSpace(last.Observation.FinalResponse) != "" {
			return last.Observation.FinalResponse
		}
		if strings.TrimSpace(last.Observation.Summary) != "" {
			return last.Observation.Summary
		}
	}
	return string(session.Status)
}

func validateResumePluginRequirements(session domain.Session, cfg config.Config, executor domain.ToolExecutor) error {
	if session.Provenance == nil {
		return nil
	}
	for _, metadata := range session.Provenance.Normalize().Plugins {
		switch metadata.Family {
		case domain.PluginFamilyProvider:
			if err := validateProviderRequirement(metadata, cfg); err != nil {
				return err
			}
		case domain.PluginFamilyVerifier:
			if err := validateVerifierRequirement(metadata, cfg, executor); err != nil {
				return err
			}
		case domain.PluginFamilyAgentStrategy:
			if err := validateAgentStrategyRequirement(metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateProviderRequirement(metadata domain.PluginMetadata, cfg config.Config) error {
	selected := strings.TrimSpace(cfg.PluginProvider)
	if selected == "" {
		return fmt.Errorf("resume fail-closed: provider plugin %q required by session provenance is unavailable; configure --plugin-provider %q", metadata.LogicalID, metadata.LogicalID)
	}
	if selected != metadata.LogicalID {
		return fmt.Errorf("resume fail-closed: provider plugin mismatch, session requires %q but active selection is %q", metadata.LogicalID, selected)
	}
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("resume fail-closed: provider plugin %q required by session provenance is unavailable: %w", metadata.LogicalID, err)
	}
	contract, ok := provider.(llm.ProviderPluginContract)
	if !ok {
		return fmt.Errorf("resume fail-closed: provider plugin mismatch, session requires plugin provider %q but active provider is not plugin-backed", metadata.LogicalID)
	}
	activeMetadata := contract.Metadata().Normalize()
	if activeMetadata.LogicalID != metadata.LogicalID {
		return fmt.Errorf("resume fail-closed: provider plugin mismatch, session requires %q but active provider is %q", metadata.LogicalID, activeMetadata.LogicalID)
	}
	if strings.TrimSpace(metadata.ContractVersion) != "" && activeMetadata.ContractVersion != metadata.ContractVersion {
		return fmt.Errorf("resume fail-closed: provider plugin mismatch, session requires %q@%s but active provider is %q@%s", metadata.LogicalID, metadata.ContractVersion, activeMetadata.LogicalID, activeMetadata.ContractVersion)
	}
	return nil
}

func validateVerifierRequirement(metadata domain.PluginMetadata, cfg config.Config, executor domain.ToolExecutor) error {
	policy := strings.TrimSpace(cfg.Runtime.VerifyMode)
	_ = policy
	carrier, ok := newVerifierFromConfig(cfg, executor).(interface{ VerifierProvenance() *domain.PluginMetadata })
	if !ok || carrier == nil || carrier.VerifierProvenance() == nil {
		return fmt.Errorf("resume fail-closed: verifier plugin %q required by session provenance is unavailable", metadata.LogicalID)
	}
	active := carrier.VerifierProvenance().Normalize()
	if active.LogicalID != metadata.LogicalID || active.ContractVersion != metadata.ContractVersion {
		return fmt.Errorf("resume fail-closed: verifier plugin mismatch, session requires %q@%s but active verifier is %q@%s", metadata.LogicalID, metadata.ContractVersion, active.LogicalID, active.ContractVersion)
	}
	return nil
}

func validateAgentStrategyRequirement(metadata domain.PluginMetadata) error {
	if metadata.LogicalID == runtime.BuiltinAgentStrategyHostDefault {
		return nil
	}
	return fmt.Errorf("resume fail-closed: agent strategy plugin %q required by session provenance is unavailable", metadata.LogicalID)
}

func isTerminalStatus(status domain.SessionStatus) bool {
	switch status {
	case domain.SessionStatusSuccess, domain.SessionStatusVerificationFailed, domain.SessionStatusBudgetExceeded, domain.SessionStatusFatalError:
		return true
	default:
		return false
	}
}

type sessionAliasStore = runtimebuilder.SessionAliasStore

func newSessionAliasStore(inner *store.SQLiteSessionStore, persistentCtx context.Context, desiredSessionID string) sessionAliasStore {
	return runtimebuilder.NewSessionAliasStore(inner, persistentCtx, desiredSessionID)
}
