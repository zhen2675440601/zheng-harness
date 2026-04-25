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
	"strings"
	"syscall"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/store"
)

const defaultDBPath = "./agent.db"

type cliApp struct {
	stdout       io.Writer
	stderr       io.Writer
	cfg          config.Config
	newSession   func(string) (*store.SQLiteSessionStore, error)
	newMemory    func(string) (*store.SQLiteMemoryStore, error)
	newExecutor  func() domain.ToolExecutor
	newModel     func() domain.Model
	newVerifier  func() domain.Verifier
	notifySignal func(chan<- os.Signal, ...os.Signal)
	stopSignal   func(chan<- os.Signal)
	now          func() time.Time
}

type runJSONOutput struct {
	Command   string               `json:"command"`
	SessionID string               `json:"session_id"`
	Status    domain.SessionStatus `json:"status"`
	TaskInput string               `json:"task_input"`
	Plan      string               `json:"plan"`
	Steps     int                  `json:"steps"`
}

type inspectJSONOutput struct {
	Command          string               `json:"command"`
	SessionID        string               `json:"session_id"`
	Status           domain.SessionStatus `json:"status"`
	TerminatedReason string               `json:"terminated_reason,omitempty"`
	Plan             string               `json:"plan"`
	StepCount        int                  `json:"step_count"`
	StepSummaries    []string             `json:"step_summaries"`
}

// configFlagNames contains the flag names that config.Load recognizes.
// These are the only flags that should be passed to config.Load.
// Other flags like --task, --session, --db, --json are run/resume/inspect specific.
var configFlagNames = map[string]bool{
	"config":         true,
	"model":          true,
	"provider":       true,
	"max-steps":      true,
	"step-timeout":   true,
	"memory-limit-mb": true,
	"verify-mode":    true,
	"api-key":        true,
	"base-url":       true,
}

// filterConfigArgs extracts only the config-related flags from args.
// This prevents "flag provided but not defined" errors when passing
// run/resume/inspect subcommand arguments to config.Load.
func filterConfigArgs(args []string) []string {
	var filtered []string
	i := 0
	for i < len(args) {
		arg := args[i]
		// Check if this arg is a config flag (handles both -flag and --flag forms)
		flagName := strings.TrimLeft(arg, "-")
		if equalsIndex := strings.Index(flagName, "="); equalsIndex != -1 {
			flagName = flagName[:equalsIndex]
		}
		if configFlagNames[flagName] {
			filtered = append(filtered, arg)
			// All config flags take a value. If using -flag=value form, value is already included.
			// If using -flag value form, need to include the next arg as the value.
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
	if cfg.GetProviderType() == config.ProviderDashScope {
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
			return FakeToolExecutor{}
		},
		newModel: modelFactory,
		newVerifier: func() domain.Verifier {
			return FakeVerifier{}
		},
		notifySignal: signal.Notify,
		stopSignal:   signal.Stop,
		now:          time.Now,
	}
	return app.run(ctx, args)
}

func (a cliApp) run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		a.printUsage()
		return 1
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

func (a cliApp) runCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	// Dummy --config and --provider flags to allow CLI to accept them without error.
	// The actual config loading happens in runCLI before this is called.
	_ = fs.String("config", "", "config file path (unused in subcommand)")
	_ = fs.String("provider", "", "provider (unused in subcommand)")
	_ = fs.String("model", "", "model (unused in subcommand)")
	_ = fs.String("api-key", "", "API key (unused in subcommand)")
	_ = fs.String("base-url", "", "base URL (unused in subcommand)")
	taskInput := fs.String("task", "", "task description")
	dbPath := fs.String("db", defaultDBPath, "sqlite database path")
	maxSteps := fs.Int("max-steps", a.defaultMaxSteps(), "maximum runtime steps")
	jsonMode := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*taskInput) == "" {
		return errors.New("run requires --task")
	}
	if *maxSteps <= 0 {
		return errors.New("run requires --max-steps > 0")
	}

	now := a.now()
	sessionID := fmt.Sprintf("session-%d", now.UnixNano())
	sessionStore, memoryStore, cleanup, err := a.openRuntimeDeps(*dbPath)
	if err != nil {
		return err
	}
	defer cleanup()
	aliasStore := newSessionAliasStore(sessionStore, sessionID, "")

	if err := aliasStore.SaveSession(ctx, domain.Session{
		ID:        sessionID,
		TaskID:    sessionID,
		Status:    domain.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("save initial session: %w", err)
	}

	engine := runtime.Engine{
		Model:          a.newModel(),
		Tools:          a.newExecutor(),
		Memory:         memoryStore,
		Sessions:       aliasStore,
		Verifier:       a.newVerifier(),
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
		CreatedAt:   now,
	}

	session, plan, steps, runErr := engine.Run(runCtx, task)
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

	a.emitRunResult(*jsonMode, task.Description, session, plan, steps)
	if runErr != nil {
		return runErr
	}
	return nil
}

func (a cliApp) resumeCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	// Dummy --config and --provider flags to allow CLI to accept them without error.
	// The actual config loading happens in runCLI before this is called.
	_ = fs.String("config", "", "config file path (unused in subcommand)")
	_ = fs.String("provider", "", "provider (unused in subcommand)")
	_ = fs.String("model", "", "model (unused in subcommand)")
	_ = fs.String("api-key", "", "API key (unused in subcommand)")
	_ = fs.String("base-url", "", "base URL (unused in subcommand)")
	sessionID := fs.String("session", "", "session identifier")
	dbPath := fs.String("db", defaultDBPath, "sqlite database path")
	maxSteps := fs.Int("max-steps", a.defaultMaxSteps(), "maximum runtime steps")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" {
		return errors.New("resume requires --session")
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

	a.emitResumeResult(session, plan, steps, false)
	if isTerminalStatus(session.Status) {
		return nil
	}

	engine := runtime.Engine{
		Model:          a.newModel(),
		Tools:          a.newExecutor(),
		Memory:         memoryStore,
		Sessions:       newSessionAliasStore(sessionStore, *sessionID, ""),
		Verifier:       a.newVerifier(),
		MaxSteps:       *maxSteps,
		MaxRetries:     *maxSteps,
		SessionTimeout: a.sessionTimeout(*maxSteps),
	}

	runCtx, stop := a.withSignalCancellation(ctx)
	defer stop()

	continuedTask := domain.Task{
		ID:          session.TaskID,
		Description: plan.Summary,
		Goal:        plan.Summary,
		CreatedAt:   session.CreatedAt,
	}

	session, plan, steps, err = engine.Run(runCtx, continuedTask)
	aliasStore := newSessionAliasStore(sessionStore, *sessionID, "")
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
	// Dummy --config and --provider flags to allow CLI to accept them without error.
	// The actual config loading happens in runCLI before this is called.
	_ = fs.String("config", "", "config file path (unused in subcommand)")
	_ = fs.String("provider", "", "provider (unused in subcommand)")
	_ = fs.String("model", "", "model (unused in subcommand)")
	_ = fs.String("api-key", "", "API key (unused in subcommand)")
	_ = fs.String("base-url", "", "base URL (unused in subcommand)")
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
	a.emitInspectResult(*jsonMode, session, plan, steps)
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

func (a cliApp) emitRunResult(jsonMode bool, taskInput string, session domain.Session, plan domain.Plan, steps []domain.Step) {
	if jsonMode {
		payload := runJSONOutput{
			Command:   "run",
			SessionID: session.ID,
			Status:    session.Status,
			TaskInput: taskInput,
			Plan:      plan.Summary,
			Steps:     len(steps),
		}
		writeJSON(a.stdout, payload)
		return
	}
	_, _ = fmt.Fprintf(a.stdout, "Session: %s\nStatus: %s\nTask: %s\nPlan: %s\nSteps: %d\n", session.ID, session.Status, taskInput, plan.Summary, len(steps))
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

func (a cliApp) emitInspectResult(jsonMode bool, session domain.Session, plan domain.Plan, steps []domain.Step) {
	terminatedReason := deriveTerminationReason(session, steps)
	if jsonMode {
		payload := inspectJSONOutput{
			Command:          "inspect",
			SessionID:        session.ID,
			Status:           session.Status,
			TerminatedReason: terminatedReason,
			Plan:             plan.Summary,
			StepCount:        len(steps),
			StepSummaries:    summarizeSteps(steps),
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
	_, _ = fmt.Fprintln(a.stderr, "  run --task \"task description\" [--db ./agent.db] [--json]")
	_, _ = fmt.Fprintln(a.stderr, "  resume --session <id> [--db ./agent.db]")
	_, _ = fmt.Fprintln(a.stderr, "  inspect --session <id> [--db ./agent.db] [--json]")
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
	inner              *store.SQLiteSessionStore
	desiredSessionID   string
	generatedSessionID string
}

func newSessionAliasStore(inner *store.SQLiteSessionStore, desiredSessionID, generatedSessionID string) sessionAliasStore {
	return sessionAliasStore{inner: inner, desiredSessionID: desiredSessionID, generatedSessionID: generatedSessionID}
}

func (s sessionAliasStore) SaveSession(ctx context.Context, session domain.Session) error {
	session.ID = s.desiredSessionID
	return s.inner.SaveSession(ctx, session)
}

func (s sessionAliasStore) SavePlan(ctx context.Context, plan domain.Plan) error {
	return s.inner.SavePlan(ctx, plan)
}

func (s sessionAliasStore) AppendStep(ctx context.Context, _ string, step domain.Step) error {
	return s.inner.AppendStep(ctx, s.desiredSessionID, step)
}
