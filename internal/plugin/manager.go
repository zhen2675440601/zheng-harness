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

	mu            sync.RWMutex
	LoadedPlugins map[string]PluginTool
	CloseHandler  func(PluginTool) error

	externalLoad func(context.Context, string) (PluginTool, error)
	nativeLoad   func(context.Context, string) (PluginTool, error)
}

// NewManager constructs a PluginManager with default loaders.
func NewManager(discoveryPath string) *PluginManager {
	return &PluginManager{
		DiscoveryPath: discoveryPath,
		LoadedPlugins: make(map[string]PluginTool),
	}
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

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.LoadedPlugins == nil {
		m.LoadedPlugins = make(map[string]PluginTool)
	}
	m.LoadedPlugins[tool.Name()] = tool
	return tool, nil
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
