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
	"zheng-harness/internal/store"
	"zheng-harness/internal/tools"
	"zheng-harness/internal/verify"
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
	newSession   func(string) (*store.SQLiteSessionStore, error)
	newMemory    func(string) (*store.SQLiteMemoryStore, error)
	newExecutor  func() domain.ToolExecutor
	newPluginManager func(string) *pluginruntime.PluginManager
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

type pluginExecutor struct {
	base     domain.ToolExecutor
	registry *tools.Registry
	plugins  map[string]pluginruntime.PluginTool
	manager  *pluginruntime.PluginManager
}

// configFlagNames 包含 config.Load 可识别的标志名称。
// 这些是唯一应传递给 config.Load 的标志。
// 其他诸如 --task、--session、--db、--json 的标志仅用于 run/resume/inspect。
var configFlagNames = map[string]bool{
	"config":          true,
	"model":           true,
	"provider":        true,
	"max-steps":       true,
	"step-timeout":    true,
	"memory-limit-mb": true,
	"verify-mode":     true,
	"api-key":         true,
	"base-url":        true,
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

// filterConfigArgs 仅从参数中提取与配置相关的标志。
// 这样可避免在传递
// run/resume/inspect 子命令参数给 config.Load 时出现“flag provided but not defined”错误。
func filterConfigArgs(args []string) []string {
	var filtered []string
	i := 0
	for i < len(args) {
		arg := args[i]
		// 检查该参数是否为配置标志（同时处理 -flag 与 --flag 两种形式）。
		flagName := strings.TrimLeft(arg, "-")
		if equalsIndex := strings.Index(flagName, "="); equalsIndex != -1 {
			flagName = flagName[:equalsIndex]
		}
		if configFlagNames[flagName] {
			filtered = append(filtered, arg)
			// 所有配置标志都带有值；如果使用 -flag=value 形式，则值已包含在参数中。
			// 如果使用 -flag value 形式，则需要将下一个参数一并作为值。
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				filtered = append(filtered, args[i])
			}
		}
		i++
	}
	return filtered
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg := config.Default()
	if len(args) > 0 && (args[0] == "run" || args[0] == "resume") {
		filteredArgs := filterConfigArgs(args[1:])
		loaded, err := config.Load(filteredArgs)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		cfg = loaded
	}

	modelFactory := func() domain.Model {
		return &FakeModel{}
	}
	if cfg.GetProviderType() != "" && cfg.GetAPIKey() != "" {
		provider, err := llm.NewProvider(cfg)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		modelFactory = func() domain.Model {
			return runtime.NewModelAdapter(provider)
		}
	}

	app := cliApp{
		stdout: stdout,
		stderr: stderr,
		cfg:    cfg,
		newSession: func(dbPath string) (*store.SQLiteSessionStore, error) {
			return store.NewSQLiteSessionStore(dbPath)
		},
		newMemory: func(dbPath string) (*store.SQLiteMemoryStore, error) {
			return store.NewMemoryStore(dbPath)
		},
		newExecutor: func() domain.ToolExecutor {
			executor, err := tools.NewExecutor(".",
				tools.WithAllowedCommands(cfg.Runtime.AllowedCommands),
			)
			if err != nil {
				return FakeToolExecutor{}
			}
			return executor
		},
		newPluginManager: pluginruntime.NewManager,
		newModel: modelFactory,
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
	switch cfg.Runtime.VerifyMode {
	case config.VerifyModeOff:
		return FakeVerifier{}
	case config.VerifyModeStandard:
		return verify.NewTaskAwareVerifier(cfg.Runtime.VerifyMode, executor)
	case config.VerifyModeStrict:
		return verify.NewTaskAwareVerifier(cfg.Runtime.VerifyMode, executor)
	default:
		return verify.NewTaskAwareVerifier(cfg.Runtime.VerifyMode, executor)
	}
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

func (a cliApp) runCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
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
	engine := runtime.Engine{
		Model:          a.newModel(),
		Tools:          executor,
		Memory:         memoryStore,
		Sessions:       aliasStore,
		Verifier:       a.newVerifier(executor),
		MaxSteps:       *maxSteps,
		MaxRetries:     *maxSteps,
		SessionTimeout: a.sessionTimeout(*maxSteps),
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
	engine := runtime.Engine{
		Model:          a.newModel(),
		Tools:          executor,
		Memory:         memoryStore,
		Sessions:       newSessionAliasStore(sessionStore, ctx, *sessionID),
		Verifier:       a.newVerifier(executor),
		MaxSteps:       *maxSteps,
		MaxRetries:     *maxSteps,
		SessionTimeout: a.sessionTimeout(*maxSteps),
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
		executor, err := tools.NewExecutor(".",
			tools.WithAllowedCommands(a.cfg.Runtime.AllowedCommands),
			tools.WithExtraAllowedCommands(commands),
		)
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
	managerFactory := a.newPluginManager
	if managerFactory == nil {
		managerFactory = pluginruntime.NewManager
	}
	manager := managerFactory(options.DiscoveryDir)
	if manager == nil {
		return nil, errors.New("plugin manager factory returned nil")
	}
	manager.Policy = tools.SafetyPolicy{
		WorkspaceRoot:      ".",
		AllowedPluginPaths: append(append([]string(nil), a.cfg.Runtime.AllowedPluginPaths...), options.AllowedPaths...),
		PluginCapabilities: append([]string(nil), a.cfg.Runtime.PluginCapabilities...),
	}
	registry := cloneExecutorRegistry(base)
	loaded := make(map[string]pluginruntime.PluginTool, len(options.Names))
	for _, path := range resolvePluginTargets(options) {
		if err := manager.Policy.ValidatePluginPath(path); err != nil {
			_ = manager.CloseAll()
			return nil, err
		}
		tool, err := manager.Load(context.Background(), path)
		if err != nil {
			_ = manager.CloseAll()
			return nil, err
		}
		if err := registry.Register(toToolDefinition(tool)); err != nil {
			_ = manager.CloseAll()
			return nil, err
		}
		loaded[tool.Name()] = tool
	}
	return &pluginExecutor{base: base, registry: registry, plugins: loaded, manager: manager}, nil
}

func resolvePluginTargets(options pluginCLIOptions) []string {
	targets := make([]string, 0, len(options.Names))
	for _, name := range options.Names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if isPluginPathReference(trimmed) {
			targets = append(targets, trimmed)
			continue
		}
		targets = append(targets, filepath.Join(options.DiscoveryDir, trimmed))
	}
	return targets
}

func normalizePluginCLIOptions(options pluginCLIOptions) pluginCLIOptions {
	options.DiscoveryDir = strings.TrimSpace(options.DiscoveryDir)
	if options.DiscoveryDir == "" {
		options.DiscoveryDir = "./plugins"
	}
	options.Names = normalizeStringValues(options.Names)
	options.AllowedPaths = normalizeStringValues(options.AllowedPaths)
	return options
}

func normalizeStringValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func isPluginPathReference(value string) bool {
	return strings.Contains(value, "/") || strings.Contains(value, `\\`) || filepath.Ext(value) != ""
}

func cloneExecutorRegistry(base domain.ToolExecutor) *tools.Registry {
	registry := tools.NewRegistry()
	provider, ok := base.(interface{ Registry() *tools.Registry })
	if !ok || provider.Registry() == nil {
		return registry
	}
	for _, def := range provider.Registry().List() {
		_ = registry.Register(def)
	}
	return registry
}

func toToolDefinition(tool pluginruntime.PluginTool) tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:           tool.Name(),
		Description:    tool.Description(),
		Schema:         tool.Schema(),
		DefaultTimeout: 30 * time.Second,
		SafetyLevel:    tool.SafetyLevel(),
		Handler:        tool.Execute,
	}
}

func (e *pluginExecutor) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	if tool, ok := e.plugins[call.Name]; ok {
		return tool.Execute(ctx, call)
	}
	return e.base.Execute(ctx, call)
}

func (e *pluginExecutor) Registry() *tools.Registry {
	if e == nil {
		return nil
	}
	return e.registry
}

func (e *pluginExecutor) Close() error {
	if e == nil || e.manager == nil {
		return nil
	}
	return e.manager.CloseAll()
}

type pluginInitializationErrorExecutor struct{ err error }

func (e pluginInitializationErrorExecutor) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{ToolName: call.Name}, e.err
}

func (e pluginInitializationErrorExecutor) Registry() *tools.Registry {
	return tools.NewRegistry()
}

func (a cliApp) defaultMaxSteps() int {
	if a.cfg.Runtime.MaxSteps > 0 {
		return a.cfg.Runtime.MaxSteps
	}
	return 8
}

func (a cliApp) stepTimeout() time.Duration {
	if a.cfg.Runtime.StepTimeout > 0 {
		return a.cfg.Runtime.StepTimeout
	}
	return 30 * time.Second
}

func (a cliApp) sessionTimeout(maxSteps int) time.Duration {
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
			TaskType:           metadata.TaskType,
			ProtocolHint:       metadata.ProtocolHint,
			VerificationPolicy: metadata.VerificationPolicy,
		}
		writeJSON(a.stdout, payload)
		return
	}
	_, _ = fmt.Fprintf(a.stdout, "Session: %s\nStatus: %s\nTermination: %s\nPlan: %s\nSummary:\n", session.ID, session.Status, terminatedReason, plan.Summary)
	for _, line := range summarizeSteps(steps) {
		_, _ = fmt.Fprintf(a.stdout, "- %s\n", line)
	}
	if len(steps) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "- no steps recorded")
	}
}

func (a cliApp) printUsage() {
	_, _ = fmt.Fprintln(a.stderr, "Usage: zheng-agent <run|resume|inspect> [flags]")
	_, _ = fmt.Fprintln(a.stderr, "  run --task \"task description\" [--db ./agent.db] [--json] [--stream] [--decompose] [--max-workers 4] [--aggregation all-succeed]")
	_, _ = fmt.Fprintln(a.stderr, "  resume --session <id> [--db ./agent.db] [--json] [--stream]")
	_, _ = fmt.Fprintln(a.stderr, "  inspect --session <id> [--db ./agent.db] [--json]")
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

func isTerminalStatus(status domain.SessionStatus) bool {
	switch status {
	case domain.SessionStatusSuccess, domain.SessionStatusVerificationFailed, domain.SessionStatusBudgetExceeded, domain.SessionStatusFatalError:
		return true
	default:
		return false
	}
}

type sessionAliasStore struct {
	inner            *store.SQLiteSessionStore
	persistentCtx    context.Context
	desiredSessionID string
}

func newSessionAliasStore(inner *store.SQLiteSessionStore, persistentCtx context.Context, desiredSessionID string) sessionAliasStore {
	if persistentCtx == nil {
		persistentCtx = context.Background()
	}
	return sessionAliasStore{inner: inner, persistentCtx: persistentCtx, desiredSessionID: desiredSessionID}
}

func (s sessionAliasStore) SaveSession(ctx context.Context, session domain.Session) error {
	ctx = s.contextOrFallback(ctx)
	session.ID = s.desiredSessionID
	return s.inner.SaveSession(ctx, session)
}

func (s sessionAliasStore) SavePlan(ctx context.Context, plan domain.Plan) error {
	ctx = s.contextOrFallback(ctx)
	return s.inner.SavePlan(ctx, plan)
}

func (s sessionAliasStore) AppendStep(ctx context.Context, _ string, step domain.Step) error {
	ctx = s.contextOrFallback(ctx)
	return s.inner.AppendStep(ctx, s.desiredSessionID, step)
}

func (s sessionAliasStore) contextOrFallback(ctx context.Context) context.Context {
	if ctx == nil {
		return s.persistentCtx
	}
	return context.WithoutCancel(ctx)
}
