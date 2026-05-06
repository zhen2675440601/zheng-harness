package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/tools"
)

func TestPluginManagerDiscovery(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTempPluginFile(t, dir, "alpha.so")
	writeTempPluginFile(t, dir, "echo-plugin")
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("Mkdir(): %v", err)
	}

	manager := NewManager(dir)

	plugins, err := manager.Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(plugins) != 2 {
		t.Fatalf("Discover() returned %d plugins, want 2", len(plugins))
	}

	got := map[string]PluginType{}
	for _, plugin := range plugins {
		got[filepath.Base(plugin.Path)] = plugin.Type
	}

	if got["alpha.so"] != PluginTypeNative {
		t.Fatalf("alpha.so type = %q, want %q", got["alpha.so"], PluginTypeNative)
	}
	if got["echo-plugin"] != PluginTypeExternal {
		t.Fatalf("echo-plugin type = %q, want %q", got["echo-plugin"], PluginTypeExternal)
	}
}

func TestPluginManagerLoadExternal(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	manager.externalLoad = func(_ context.Context, _ string) (PluginTool, error) {
		return &stubManagerPlugin{name: "external-tool", contractVersion: ContractVersion, capabilities: []string{"filesystem.read"}}, nil
	}

	tool, err := manager.Load(context.Background(), filepath.Join(manager.DiscoveryPath, "echo-plugin"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if tool.Name() != "external-tool" {
		t.Fatalf("Load().Name() = %q, want %q", tool.Name(), "external-tool")
	}
	if _, ok := manager.LoadedPlugins[tool.Name()]; !ok {
		t.Fatalf("LoadedPlugins missing key %q", tool.Name())
	}
	if len(manager.LoadedPlugins) != 1 {
		t.Fatalf("LoadedPlugins len = %d, want 1", len(manager.LoadedPlugins))
	}
	if manager.LoadedPlugins[tool.Name()] != tool {
		t.Fatal("LoadedPlugins stored unexpected tool instance")
	}
}

func TestPluginManagerLoadNative(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	var called string
	manager.nativeLoad = func(_ context.Context, path string) (PluginTool, error) {
		called = path
		return &stubManagerPlugin{name: "native-tool", contractVersion: ContractVersion, capabilities: []string{"filesystem.read"}}, nil
	}

	pluginPath := filepath.Join(manager.DiscoveryPath, "native_plugin.so")
	tool, err := manager.Load(context.Background(), pluginPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if called != pluginPath {
		t.Fatalf("native loader path = %q, want %q", called, pluginPath)
	}
	if tool.Name() != "native-tool" {
		t.Fatalf("Load().Name() = %q, want %q", tool.Name(), "native-tool")
	}

	if _, ok := manager.LoadedPlugins[tool.Name()]; !ok {
		t.Fatalf("LoadedPlugins missing key %q", tool.Name())
	}
}

func TestPluginManagerCloseAll(t *testing.T) {
	t.Parallel()

	first := &stubManagerPlugin{name: "alpha", contractVersion: ContractVersion, capabilities: []string{"filesystem.read"}}
	second := &stubManagerPlugin{name: "beta", contractVersion: ContractVersion, capabilities: []string{"filesystem.read"}}

	var closed []string
	manager := NewManager(t.TempDir())
	manager.LoadedPlugins = map[string]PluginTool{
		first.Name():  first,
		second.Name(): second,
	}
	manager.CloseHandler = func(tool PluginTool) error {
		closed = append(closed, tool.Name())
		return tool.Close()
	}

	if err := manager.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}
	if !first.closed || !second.closed {
		t.Fatalf("CloseAll() did not close all plugins: first=%v second=%v", first.closed, second.closed)
	}
	if len(manager.LoadedPlugins) != 0 {
		t.Fatalf("LoadedPlugins len = %d, want 0", len(manager.LoadedPlugins))
	}
	sort.Strings(closed)
	if len(closed) != 2 || closed[0] != "alpha" || closed[1] != "beta" {
		t.Fatalf("closed plugins = %v, want [alpha beta]", closed)
	}
	if err := manager.CloseAll(); err != nil {
		t.Fatalf("second CloseAll() error = %v", err)
	}
	if first.closeCalls != 1 || second.closeCalls != 1 {
		t.Fatalf("close calls = (%d,%d), want (1,1)", first.closeCalls, second.closeCalls)
	}
}

func TestPluginVersionMismatchFailsClosed(t *testing.T) {
	t.Parallel()

	plugin := &stubManagerPlugin{name: "bad-version", contractVersion: "0.9.0"}
	manager := NewManager(t.TempDir())
	manager.externalLoad = func(_ context.Context, _ string) (PluginTool, error) {
		return plugin, nil
	}

	_, err := manager.Load(context.Background(), filepath.Join(manager.DiscoveryPath, "bad-plugin"))
	if err == nil {
		t.Fatal("expected Load() error")
	}
	if !errors.Is(err, ErrContractVersionMismatch) {
		t.Fatalf("Load() error = %v, want %v", err, ErrContractVersionMismatch)
	}
	if !plugin.closed {
		t.Fatal("Load() did not close version-mismatched plugin")
	}
	if len(manager.LoadedPlugins) != 0 {
		t.Fatalf("LoadedPlugins len = %d, want 0", len(manager.LoadedPlugins))
	}
}

func TestPluginLoadCleanupOnFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  tools.SafetyPolicy
		plugin  *stubManagerPlugin
		wantErr error
		wantMsg string
	}{
		{
			name:    "contract version mismatch",
			plugin:  &stubManagerPlugin{name: "bad-version", contractVersion: "0.9.0", capabilities: []string{"filesystem.read"}},
			wantErr: ErrContractVersionMismatch,
		},
		{
			name:    "capability rejection",
			policy:  tools.SafetyPolicy{PluginCapabilities: []string{"filesystem.read"}},
			plugin:  &stubManagerPlugin{name: "restricted", contractVersion: ContractVersion, capabilities: []string{"shell.exec"}},
			wantMsg: `plugin capability "shell.exec" is not allowed`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := NewManager(t.TempDir())
			manager.Policy = tc.policy
			healthy := &stubManagerPlugin{name: "healthy", contractVersion: ContractVersion, capabilities: []string{"filesystem.read"}}
			manager.LoadedPlugins[healthy.Name()] = healthy
			manager.externalLoad = func(_ context.Context, _ string) (PluginTool, error) {
				return tc.plugin, nil
			}

				_, err := manager.Load(context.Background(), filepath.Join(manager.DiscoveryPath, tc.plugin.Name()))
				if err == nil {
					t.Fatal("expected Load() error")
				}
				if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
					t.Fatalf("Load() error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
					t.Fatalf("Load() error = %v, want message containing %q", err, tc.wantMsg)
				}
				if !tc.plugin.closed {
					t.Fatal("Load() did not close failed plugin")
				}
				if tc.plugin.closeCalls != 1 {
					t.Fatalf("failed plugin close calls = %d, want 1", tc.plugin.closeCalls)
				}
				if len(manager.LoadedPlugins) != 1 {
					t.Fatalf("LoadedPlugins len = %d, want 1", len(manager.LoadedPlugins))
				}
				if manager.LoadedPlugins[healthy.Name()] != healthy {
					t.Fatal("healthy plugin registration changed after failed load")
				}
				if _, exists := manager.LoadedPlugins[tc.plugin.Name()]; exists {
					t.Fatalf("failed plugin %q remained registered", tc.plugin.Name())
				}
		})
	}
}

func TestPluginManagerRejectsPluginWithUndeclaredCapabilities(t *testing.T) {
	t.Parallel()

	plugin := &stubManagerPlugin{name: "restricted", contractVersion: ContractVersion, capabilities: []string{"shell.exec"}}
	manager := NewManager(t.TempDir())
	manager.Policy = tools.SafetyPolicy{PluginCapabilities: []string{"filesystem.read", "web.fetch"}}
	manager.externalLoad = func(_ context.Context, _ string) (PluginTool, error) {
		return plugin, nil
	}

	_, err := manager.Load(context.Background(), filepath.Join(manager.DiscoveryPath, "restricted-plugin"))
	if err == nil {
		t.Fatal("expected Load() error")
	}
	if !strings.Contains(err.Error(), `plugin capability "shell.exec" is not allowed`) {
		t.Fatalf("Load() error = %v, want capability rejection", err)
	}
	if !plugin.closed {
		t.Fatal("Load() did not close capability-mismatched plugin")
	}
	if len(manager.LoadedPlugins) != 0 {
		t.Fatalf("LoadedPlugins len = %d, want 0", len(manager.LoadedPlugins))
	}
}

func TestPluginManagerRejectsPluginWithoutDeclaredCapabilitiesWhenPolicyConfigured(t *testing.T) {
	t.Parallel()

	plugin := &stubManagerPlugin{name: "missing-caps", contractVersion: ContractVersion}
	manager := NewManager(t.TempDir())
	manager.Policy = tools.SafetyPolicy{PluginCapabilities: []string{"filesystem.read"}}
	manager.externalLoad = func(_ context.Context, _ string) (PluginTool, error) {
		return plugin, nil
	}

	_, err := manager.Load(context.Background(), filepath.Join(manager.DiscoveryPath, "missing-caps-plugin"))
	if err == nil {
		t.Fatal("expected Load() error")
	}
	if !strings.Contains(err.Error(), "plugin capabilities must be declared") {
		t.Fatalf("Load() error = %v, want missing capability declaration", err)
	}
	if !plugin.closed {
		t.Fatal("Load() did not close plugin missing capability declaration")
	}
	if len(manager.LoadedPlugins) != 0 {
		t.Fatalf("LoadedPlugins len = %d, want 0", len(manager.LoadedPlugins))
	}
}

func TestPluginConcurrentLoadUnload(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	var sequence atomic.Int64
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		id := sequence.Add(1)
		return &stubManagerPlugin{
			name:            fmt.Sprintf("%s-%d", filepath.Base(path), id),
			contractVersion: ContractVersion,
			capabilities:    []string{"filesystem.read"},
		}, nil
	}

	const goroutines = 8
	const iterations = 20

	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	for worker := range goroutines {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range iterations {
				path := filepath.Join(manager.DiscoveryPath, fmt.Sprintf("plugin-%d-%d", worker, i))
				if _, err := manager.Load(context.Background(), path); err != nil {
					errCh <- fmt.Errorf("load worker=%d iteration=%d: %w", worker, i, err)
					return
				}
				if i%3 == 0 {
					if err := manager.CloseAll(); err != nil {
						errCh <- fmt.Errorf("close worker=%d iteration=%d: %w", worker, i, err)
						return
					}
				}
			}
		}(worker)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := manager.CloseAll(); err != nil {
		t.Fatalf("final CloseAll() error = %v", err)
	}
	if len(manager.LoadedPlugins) != 0 {
		t.Fatalf("LoadedPlugins len = %d, want 0", len(manager.LoadedPlugins))
	}
}

type stubManagerPlugin struct {
	name            string
	contractVersion string
	capabilities    []string
	closed          bool
	closeCalls      int
}

func (p *stubManagerPlugin) Name() string { return p.name }

func (p *stubManagerPlugin) Description() string { return "stub plugin" }

func (p *stubManagerPlugin) Schema() string { return `{"type":"object"}` }

func (p *stubManagerPlugin) Capabilities() []string { return append([]string(nil), p.capabilities...) }

func (p *stubManagerPlugin) SafetyLevel() domain.SafetyLevel { return domain.SafetyLevelLow }

func (p *stubManagerPlugin) ContractVersion() string { return p.contractVersion }

func (p *stubManagerPlugin) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{ToolName: call.Name, Output: call.Input}, nil
}

func (p *stubManagerPlugin) Close() error {
	p.closed = true
	p.closeCalls++
	return nil
}

func writeTempPluginFile(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("stub"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
