package plugin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	"zheng-harness/internal/tools"
)

// PluginType identifies which loader should be used.
type PluginType string

const (
	PluginTypeNative   PluginType = "native"
	PluginTypeExternal PluginType = "external"
)

// DiscoveredPlugin describes a plugin artifact found on disk.
type DiscoveredPlugin struct {
	Path string
	Type PluginType
}

// PluginManager discovers, loads, validates, and shuts down tool plugins.
type PluginManager struct {
	DiscoveryPath string
	Policy        tools.SafetyPolicy
	Providers     *ProviderRegistry
	Verifiers     *VerifierRegistry
	AgentStrategies *AgentStrategyRegistry

	mu               sync.RWMutex
	LoadedPlugins    map[string]PluginTool
	loadedPaths      map[string]string
	failureCounts    map[string]int
	unavailable      map[string]bool
	CloseHandler     func(PluginTool) error

	externalLoad func(context.Context, string) (PluginTool, error)
	nativeLoad   func(context.Context, string) (PluginTool, error)
}

var ErrPluginUnavailable = errors.New("plugin unavailable")

type pluginHealthChecker interface {
	HealthCheck(time.Duration) error
}

type managedPlugin struct {
	manager *PluginManager
	tool    PluginTool
}

func (p *managedPlugin) Name() string { return p.tool.Name() }

func (p *managedPlugin) Description() string { return p.tool.Description() }

func (p *managedPlugin) Schema() string { return p.tool.Schema() }

func (p *managedPlugin) Capabilities() []string { return p.tool.Capabilities() }

func (p *managedPlugin) SafetyLevel() domain.SafetyLevel { return p.tool.SafetyLevel() }

func (p *managedPlugin) ContractVersion() string { return p.tool.ContractVersion() }

func (p *managedPlugin) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return p.manager.executeWithRecovery(ctx, p.Name(), call)
}

func (p *managedPlugin) Close() error { return p.tool.Close() }

func (p *managedPlugin) unwrap() PluginTool { return p.tool }

// NewManager constructs a PluginManager with default loaders.
func NewManager(discoveryPath string) *PluginManager {
	return &PluginManager{
		DiscoveryPath: discoveryPath,
		LoadedPlugins: make(map[string]PluginTool),
		loadedPaths:   make(map[string]string),
		failureCounts: make(map[string]int),
		unavailable:   make(map[string]bool),
		Providers:     NewProviderRegistry(),
		Verifiers:     NewVerifierRegistry(),
		AgentStrategies: NewAgentStrategyRegistry(),
	}
}

func init() {
	llm.SetDefaultProviderResolver(defaultLLMProviderResolver{})
}

// ResolveProvider applies host-owned provider selection rules.
func (m *PluginManager) ResolveProvider(id string, cfg llm.ProviderConfig) (llm.Provider, error) {
	if m == nil || m.Providers == nil {
		return nil, fmt.Errorf("%w: provider registry is nil", ErrRegistryEntryNotFound)
	}
	return m.Providers.Resolve(id, cfg)
}

type defaultLLMProviderResolver struct{}

func (defaultLLMProviderResolver) Resolve(id string, cfg llm.ProviderConfig) (llm.Provider, error) {
	return NewManager("").ResolveProvider(id, cfg)
}

// ResolveVerifier applies host-owned verifier selection rules.
func (m *PluginManager) ResolveVerifier(id, mode string, executor domain.ToolExecutor) (domain.Verifier, error) {
	if m == nil || m.Verifiers == nil {
		return nil, fmt.Errorf("%w: verifier registry is nil", ErrRegistryEntryNotFound)
	}
	return m.Verifiers.Resolve(id, mode, executor)
}

// ResolveAgentStrategy applies host-owned agent strategy selection rules.
func (m *PluginManager) ResolveAgentStrategy(id string) (AgentStrategy, error) {
	if m == nil || m.AgentStrategies == nil {
		return nil, fmt.Errorf("%w: agent strategy registry is nil", ErrRegistryEntryNotFound)
	}
	return m.AgentStrategies.Resolve(id)
}

// Discover scans the configured plugin directory and classifies plugin artifacts.
func (m *PluginManager) Discover() ([]DiscoveredPlugin, error) {
	if m == nil {
		return nil, errors.New("plugin manager is nil")
	}
	if strings.TrimSpace(m.DiscoveryPath) == "" {
		return nil, errors.New("plugin discovery path must not be empty")
	}

	entries, err := os.ReadDir(m.DiscoveryPath)
	if err != nil {
		return nil, fmt.Errorf("read plugin directory %q: %w", m.DiscoveryPath, err)
	}

	plugins := make([]DiscoveredPlugin, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(m.DiscoveryPath, entry.Name())
		pluginType := PluginTypeExternal
		if isNativePluginPath(path) {
			pluginType = PluginTypeNative
		}

		plugins = append(plugins, DiscoveredPlugin{Path: path, Type: pluginType})
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Path < plugins[j].Path
	})

	return plugins, nil
}

// Load routes the artifact to the correct loader, validates the contract, and tracks the instance.
func (m *PluginManager) Load(ctx context.Context, path string) (PluginTool, error) {
	if m == nil {
		return nil, errors.New("plugin manager is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("plugin path must not be empty")
	}

	tool, err := m.loadPlugin(ctx, path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.LoadedPlugins == nil {
		m.LoadedPlugins = make(map[string]PluginTool)
	}
	if m.loadedPaths == nil {
		m.loadedPaths = make(map[string]string)
	}
	wrapped := &managedPlugin{manager: m, tool: tool}
	m.LoadedPlugins[tool.Name()] = wrapped
	m.loadedPaths[tool.Name()] = path
	m.failureCounts[tool.Name()] = 0
	delete(m.unavailable, tool.Name())
	return wrapped, nil
}

// ReloadTool closes and hard-reloads a tracked plugin tool from the discovery path.
func (m *PluginManager) ReloadTool(name string) error {
	if m == nil {
		return errors.New("plugin manager is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.LoadedPlugins == nil {
		return errors.New("plugin not found")
	}

	existing, ok := m.LoadedPlugins[name]
	if !ok || existing == nil {
		return errors.New("plugin not found")
	}

	// Preserve failure count across reload to ensure auto-recovery logic works correctly.
	// After reload, if the plugin crashes again, it will be marked unavailable
	// only after enough consecutive failures.
	previousFailureCount := 0
	if m.failureCounts != nil {
		previousFailureCount = m.failureCounts[name]
	}

	path, err := m.reloadPath(name)
	if err != nil {
		return err
	}

	// Load the new plugin BEFORE closing the old one to ensure atomicity.
	// If loadPlugin fails, the old plugin remains intact.
	tool, err := m.loadPlugin(context.Background(), path)
	if err != nil {
		return fmt.Errorf("load new plugin %q: %w", name, err)
	}

	// Now safe to close the old plugin
	if err := m.close(existing); err != nil {
		// Close the newly loaded plugin to avoid leaks
		_ = m.close(tool)
		return fmt.Errorf("close plugin %q: %w", name, err)
	}

	// Clean up old tracking, then add new
	delete(m.LoadedPlugins, name)
	if m.loadedPaths != nil {
		delete(m.loadedPaths, name)
	}
	if m.failureCounts != nil {
		delete(m.failureCounts, name)
	}
	if m.unavailable != nil {
		delete(m.unavailable, name)
	}

	if m.LoadedPlugins == nil {
		m.LoadedPlugins = make(map[string]PluginTool)
	}
	if m.loadedPaths == nil {
		m.loadedPaths = make(map[string]string)
	}
	if m.failureCounts == nil {
		m.failureCounts = make(map[string]int)
	}
	if m.unavailable == nil {
		m.unavailable = make(map[string]bool)
	}
	m.LoadedPlugins[tool.Name()] = &managedPlugin{manager: m, tool: tool}
	m.loadedPaths[tool.Name()] = path
	m.failureCounts[tool.Name()] = previousFailureCount // Preserve failure count across reload
	// Reset unavailable flag on reload - give the plugin a fresh start
	delete(m.unavailable, tool.Name())
	return nil
}

func (m *PluginManager) executeWithRecovery(ctx context.Context, name string, call domain.ToolCall) (domain.ToolResult, error) {
	tool, unavailable := m.lookupManagedPlugin(name)
	if unavailable {
		return domain.ToolResult{ToolName: call.Name}, fmt.Errorf("%w: %s", ErrPluginUnavailable, name)
	}
	if tool == nil {
		return domain.ToolResult{ToolName: call.Name}, errors.New("plugin not found")
	}

	result, err := tool.Execute(ctx, call)
	if err == nil {
		m.resetFailureCount(name)
		return result, nil
	}

	failures := m.incrementFailureCount(name)
	if failures >= 2 {
		m.markUnavailable(name)
		return result, fmt.Errorf("%w: %s", ErrPluginUnavailable, name)
	}

	healthErr := m.healthCheck(tool)
	if healthErr == nil {
		return result, err
	}
	if reloadErr := m.ReloadTool(name); reloadErr != nil {
		return result, fmt.Errorf("%w; reload failed: %v", err, reloadErr)
	}

	reloaded, unavailable := m.lookupManagedPlugin(name)
	if unavailable {
		return result, fmt.Errorf("%w: %s", ErrPluginUnavailable, name)
	}
	if reloaded == nil {
		return result, fmt.Errorf("%w: plugin missing after reload", err)
	}

	retryResult, retryErr := reloaded.Execute(ctx, call)
	if retryErr == nil {
		m.resetFailureCount(name)
		return retryResult, nil
	}

	failures = m.incrementFailureCount(name)
	if failures >= 2 {
		m.markUnavailable(name)
		return retryResult, fmt.Errorf("%w: %s", ErrPluginUnavailable, name)
	}
	return retryResult, retryErr
}

func (m *PluginManager) lookupManagedPlugin(name string) (PluginTool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.unavailable != nil && m.unavailable[name] {
		return nil, true
	}
	tool := m.LoadedPlugins[name]
	if wrapped, ok := tool.(*managedPlugin); ok {
		return wrapped.unwrap(), false
	}
	return tool, false
}

func (m *PluginManager) healthCheck(tool PluginTool) error {
	healthChecker, ok := tool.(pluginHealthChecker)
	if !ok {
		return nil
	}
	return healthChecker.HealthCheck(0)
}

func (m *PluginManager) incrementFailureCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failureCounts == nil {
		m.failureCounts = make(map[string]int)
	}
	m.failureCounts[name]++
	return m.failureCounts[name]
}

func (m *PluginManager) resetFailureCount(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failureCounts == nil {
		m.failureCounts = make(map[string]int)
	}
	m.failureCounts[name] = 0
	if m.unavailable != nil {
		delete(m.unavailable, name)
	}
}

func (m *PluginManager) markUnavailable(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unavailable == nil {
		m.unavailable = make(map[string]bool)
	}
	m.unavailable[name] = true
}

func (m *PluginManager) failureCount(name string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.failureCounts == nil {
		return 0
	}
	return m.failureCounts[name]
}

func (m *PluginManager) isUnavailable(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.unavailable == nil {
		return false
	}
	return m.unavailable[name]
}

func (m *PluginManager) loadPlugin(ctx context.Context, path string) (PluginTool, error) {

	loader := m.externalLoader()
	loaderType := PluginTypeExternal
	if isNativePluginPath(path) {
		loader = m.nativeLoader()
		loaderType = PluginTypeNative
	}

	tool, err := loader(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("load %s plugin %q: %w", loaderType, path, err)
	}

	if err := ValidateContract(tool); err != nil {
		_ = m.close(tool)
		return nil, err
	}
	if err := m.Policy.ValidatePluginCapabilities(tool.Capabilities()); err != nil {
		_ = m.close(tool)
		return nil, fmt.Errorf("plugin %q: %w", tool.Name(), err)
	}

	return tool, nil
}

func (m *PluginManager) reloadPath(name string) (string, error) {
	plugins, err := m.Discover()
	if err != nil {
		return "", err
	}

	storedPath := ""
	if m.loadedPaths != nil {
		storedPath = m.loadedPaths[name]
	}
	if storedPath == "" {
		return "", errors.New("plugin not found")
	}

	storedBase := filepath.Base(storedPath)
	for _, plugin := range plugins {
		if filepath.Base(plugin.Path) == storedBase {
			return plugin.Path, nil
		}
	}

	return "", errors.New("plugin not found")
}

// CloseAll shuts down all tracked plugins and clears the registry.
func (m *PluginManager) CloseAll() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	loaded := make(map[string]PluginTool, len(m.LoadedPlugins))
	maps.Copy(loaded, m.LoadedPlugins)
	m.LoadedPlugins = make(map[string]PluginTool)
	m.loadedPaths = make(map[string]string)
	m.failureCounts = make(map[string]int)
	m.unavailable = make(map[string]bool)
	m.mu.Unlock()

	var errs []error
	for _, name := range sortedPluginNames(loaded) {
		if err := m.close(loaded[name]); err != nil {
			errs = append(errs, fmt.Errorf("close plugin %q: %w", name, err))
		}
	}

	return errors.Join(errs...)
}

func (m *PluginManager) close(tool PluginTool) error {
	if tool == nil {
		return nil
	}
	if wrapped, ok := tool.(*managedPlugin); ok {
		tool = wrapped.unwrap()
	}
	if m != nil && m.CloseHandler != nil {
		return m.CloseHandler(tool)
	}
	return tool.Close()
}

func (m *PluginManager) externalLoader() func(context.Context, string) (PluginTool, error) {
	if m != nil && m.externalLoad != nil {
		return m.externalLoad
	}
	return func(ctx context.Context, path string) (PluginTool, error) {
		return ExternalLoader{Command: path}.Load(ctx)
	}
}

func (m *PluginManager) nativeLoader() func(context.Context, string) (PluginTool, error) {
	if m != nil && m.nativeLoad != nil {
		return m.nativeLoad
	}
	return func(ctx context.Context, path string) (PluginTool, error) {
		return NativeLoader{Path: path}.Load(ctx)
	}
}

func isNativePluginPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".so")
}

func sortedPluginNames(loaded map[string]PluginTool) []string {
	names := make([]string, 0, len(loaded))
	for name := range loaded {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
