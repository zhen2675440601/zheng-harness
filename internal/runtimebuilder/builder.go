package runtimebuilder

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	pluginruntime "zheng-harness/internal/plugin"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/store"
	"zheng-harness/internal/tools"
	"zheng-harness/internal/verify"
)

type Options struct {
	WorkspaceRoot         string
	NewPluginManager      func(string) *pluginruntime.PluginManager
	PluginExecutorFactory func(domain.ToolExecutor, PluginOptions) (domain.ToolExecutor, error)
}

type StoreOptions struct {
	EnableWAL bool
}

type PluginOptions struct {
	DiscoveryDir string
	Names        []string
	AllowedPaths []string
}

type ExecutorOptions struct {
	ExtraAllowedCommands []string
	Plugins              PluginOptions
}

type EngineOptions struct {
	Model        domain.Model
	Tools        domain.ToolExecutor
	Memory       domain.MemoryStore
	Sessions     domain.SessionStore
	Verifier     domain.Verifier
	MaxSteps     int
	EventChannel *runtime.EventChannel
	Clock        func() time.Time
	SessionAlias string
	PersistentCtx context.Context
}

type ServerConfig struct {
	ListenAddress    string
	JWTSecret        string
	JWTSecretFile    string
	ActiveSessionCap int
	ShutdownTimeout  time.Duration
	EnableWAL        bool
}

type Builder struct {
	cfg                   config.Config
	workspaceRoot         string
	newPluginManager      func(string) *pluginruntime.PluginManager
	pluginExecutorFactory func(domain.ToolExecutor, PluginOptions) (domain.ToolExecutor, error)
	modelFactory          func() domain.Model
}

func New(cfg config.Config, opts Options) (*Builder, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = "."
	}

	modelFactory := func() domain.Model {
		return nil
	}
	providerType := strings.TrimSpace(cfg.GetProviderType())
	apiKey := strings.TrimSpace(cfg.GetAPIKey())
	shouldInitProvider := providerType == config.ProviderPlugin || (providerType != "" && apiKey != "")
	if shouldInitProvider {
		provider, err := llm.NewProvider(cfg)
		if err != nil {
			return nil, err
		}
		modelFactory = func() domain.Model {
			return runtime.NewModelAdapter(provider)
		}
	}

	b := &Builder{
		cfg:                   cfg,
		workspaceRoot:         workspaceRoot,
		newPluginManager:      opts.NewPluginManager,
		pluginExecutorFactory: opts.PluginExecutorFactory,
		modelFactory:          modelFactory,
	}
	if b.newPluginManager == nil {
		b.newPluginManager = pluginruntime.NewManager
	}
	if b.pluginExecutorFactory == nil {
		b.pluginExecutorFactory = b.buildPluginExecutor
	}
	return b, nil
}

func (b *Builder) Config() config.Config {
	if b == nil {
		return config.Config{}
	}
	return b.cfg
}

func (b *Builder) NewSessionStore(dbPath string, opts StoreOptions) (*store.SQLiteSessionStore, error) {
	return store.NewSQLiteSessionStoreWithOptions(dbPath, store.SQLiteOptions{EnableWAL: opts.EnableWAL})
}

func (b *Builder) NewMemoryStore(dbPath string, opts StoreOptions) (*store.SQLiteMemoryStore, error) {
	return store.NewMemoryStoreWithOptions(dbPath, store.SQLiteOptions{EnableWAL: opts.EnableWAL})
}

func (b *Builder) NewModel() domain.Model {
	if b == nil || b.modelFactory == nil {
		return nil
	}
	return b.modelFactory()
}

func (b *Builder) NewVerifier(executor domain.ToolExecutor) domain.Verifier {
	if b == nil {
		return nil
	}
	return NewVerifierFromConfig(b.cfg, executor)
}

func NewVerifierFromConfig(cfg config.Config, executor domain.ToolExecutor) domain.Verifier {
	switch cfg.Runtime.VerifyMode {
	case config.VerifyModeOff:
		return nil
	case config.VerifyModeStandard:
		return verify.NewTaskAwareVerifier(cfg.Runtime.VerifyMode, executor)
	case config.VerifyModeStrict:
		return verify.NewTaskAwareVerifier(cfg.Runtime.VerifyMode, executor)
	default:
		return verify.NewTaskAwareVerifier(cfg.Runtime.VerifyMode, executor)
	}
}

func (b *Builder) NewExecutor(opts ExecutorOptions) (domain.ToolExecutor, error) {
	if b == nil {
		return nil, errors.New("runtime builder is nil")
	}
	base, err := tools.NewExecutor(b.workspaceRoot,
		tools.WithAllowedCommands(b.cfg.Runtime.AllowedCommands),
		tools.WithExtraAllowedCommands(opts.ExtraAllowedCommands),
	)
	if err != nil {
		return nil, err
	}
	pluginOptions := NormalizePluginOptions(opts.Plugins)
	if len(pluginOptions.Names) == 0 {
		return base, nil
	}
	return b.WrapExecutorWithPlugins(base, pluginOptions)
}

func (b *Builder) WrapExecutorWithPlugins(base domain.ToolExecutor, options PluginOptions) (domain.ToolExecutor, error) {
	if b == nil {
		return nil, errors.New("runtime builder is nil")
	}
	pluginOptions := NormalizePluginOptions(options)
	if len(pluginOptions.Names) == 0 {
		return base, nil
	}
	return b.pluginExecutorFactory(base, pluginOptions)
}

func (b *Builder) BuildEngine(opts EngineOptions) runtime.Engine {
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = b.DefaultMaxSteps()
	}
	model := opts.Model
	if model == nil {
		model = b.NewModel()
	}
	verifier := opts.Verifier
	if verifier == nil {
		verifier = b.NewVerifier(opts.Tools)
	}
	sessions := opts.Sessions
	if strings.TrimSpace(opts.SessionAlias) != "" {
		sessions = NewSessionAliasStore(opts.Sessions, opts.PersistentCtx, opts.SessionAlias)
	}
	return runtime.Engine{
		Model:          model,
		Tools:          opts.Tools,
		Memory:         opts.Memory,
		Sessions:       sessions,
		Verifier:       verifier,
		EventChannel:   opts.EventChannel,
		Clock:          opts.Clock,
		MaxSteps:       opts.MaxSteps,
		MaxRetries:     opts.MaxSteps,
		SessionTimeout: b.SessionTimeout(opts.MaxSteps),
	}
}

func (b *Builder) DefaultMaxSteps() int {
	if b != nil && b.cfg.Runtime.MaxSteps > 0 {
		return b.cfg.Runtime.MaxSteps
	}
	return 8
}

func (b *Builder) StepTimeout() time.Duration {
	if b != nil && b.cfg.Runtime.StepTimeout > 0 {
		return b.cfg.Runtime.StepTimeout
	}
	return 30 * time.Second
}

func (b *Builder) SessionTimeout(maxSteps int) time.Duration {
	if maxSteps <= 0 {
		maxSteps = b.DefaultMaxSteps()
	}
	return time.Duration(maxSteps) * b.StepTimeout()
}

func (b *Builder) buildPluginExecutor(base domain.ToolExecutor, options PluginOptions) (domain.ToolExecutor, error) {
	manager := b.newPluginManager(options.DiscoveryDir)
	if manager == nil {
		return nil, errors.New("plugin manager factory returned nil")
	}
	manager.Policy = tools.SafetyPolicy{
		WorkspaceRoot:      b.workspaceRoot,
		AllowedPluginPaths: append(append([]string(nil), b.cfg.Runtime.AllowedPluginPaths...), options.AllowedPaths...),
		PluginCapabilities: append([]string(nil), b.cfg.Runtime.PluginCapabilities...),
	}
	registry := cloneExecutorRegistry(base)
	loaded := make(map[string]pluginruntime.PluginTool, len(options.Names))
	for _, path := range ResolvePluginTargets(options) {
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

func NormalizePluginOptions(options PluginOptions) PluginOptions {
	options.DiscoveryDir = strings.TrimSpace(options.DiscoveryDir)
	if options.DiscoveryDir == "" {
		options.DiscoveryDir = "./plugins"
	}
	options.Names = normalizeStringValues(options.Names)
	options.AllowedPaths = normalizeStringValues(options.AllowedPaths)
	return options
}

func ResolvePluginTargets(options PluginOptions) []string {
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

func CloneExecutorRegistry(base domain.ToolExecutor) *tools.Registry {
	return cloneExecutorRegistry(base)
}

func ToToolDefinition(tool pluginruntime.PluginTool) tools.ToolDefinition {
	return toToolDefinition(tool)
}

type pluginExecutor struct {
	base     domain.ToolExecutor
	registry *tools.Registry
	plugins  map[string]pluginruntime.PluginTool
	manager  *pluginruntime.PluginManager
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

func (c ServerConfig) Normalize() ServerConfig {
	c.ListenAddress = strings.TrimSpace(c.ListenAddress)
	if c.ListenAddress == "" {
		c.ListenAddress = ":8080"
	}
	c.JWTSecret = strings.TrimSpace(c.JWTSecret)
	c.JWTSecretFile = strings.TrimSpace(c.JWTSecretFile)
	if c.ActiveSessionCap <= 0 {
		c.ActiveSessionCap = 8
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 30 * time.Second
	}
	return c
}

func (c ServerConfig) Validate() error {
	normalized := c.Normalize()
	if normalized.ListenAddress == "" {
		return errors.New("server listen address must not be empty")
	}
	if normalized.JWTSecret != "" && normalized.JWTSecretFile != "" {
		return errors.New("server JWT secret source must be exactly one of inline secret or secret file")
	}
	if normalized.JWTSecret == "" && normalized.JWTSecretFile == "" {
		return errors.New("server JWT secret source must be configured")
	}
	if normalized.ActiveSessionCap <= 0 {
		return errors.New("server active session cap must be greater than zero")
	}
	if normalized.ShutdownTimeout <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	return nil
}

type SessionAliasStore struct {
	inner            domain.SessionStore
	persistentCtx    context.Context
	desiredSessionID string
}

func NewSessionAliasStore(inner domain.SessionStore, persistentCtx context.Context, desiredSessionID string) SessionAliasStore {
	if persistentCtx == nil {
		persistentCtx = context.Background()
	}
	return SessionAliasStore{inner: inner, persistentCtx: persistentCtx, desiredSessionID: desiredSessionID}
}

func (s SessionAliasStore) SaveSession(ctx context.Context, session domain.Session) error {
	ctx = s.contextOrFallback(ctx)
	session.ID = s.desiredSessionID
	return s.inner.SaveSession(ctx, session)
}

func (s SessionAliasStore) SavePlan(ctx context.Context, plan domain.Plan) error {
	return s.inner.SavePlan(s.contextOrFallback(ctx), plan)
}

func (s SessionAliasStore) AppendStep(ctx context.Context, _ string, step domain.Step) error {
	return s.inner.AppendStep(s.contextOrFallback(ctx), s.desiredSessionID, step)
}

func (s SessionAliasStore) contextOrFallback(ctx context.Context) context.Context {
	if ctx == nil {
		return s.persistentCtx
	}
	return context.WithoutCancel(ctx)
}
