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
	"time"

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
	if got := manager.LoadedPlugins[tool.Name()]; got == nil {
		t.Fatal("LoadedPlugins stored nil tool instance")
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

func TestReloadTool_ReloadsPlugin(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	pluginPath := filepath.Join(manager.DiscoveryPath, "echo-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "echo-plugin")

	oldPlugin := &stubManagerPlugin{name: "reloadable-tool", contractVersion: ContractVersion, capabilities: []string{"filesystem.read"}}
	manager.LoadedPlugins[oldPlugin.Name()] = oldPlugin
	manager.loadedPaths[oldPlugin.Name()] = pluginPath

	var loadedPath string
	newPlugin := &stubManagerPlugin{name: oldPlugin.Name(), contractVersion: ContractVersion, capabilities: []string{"filesystem.read"}}
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		loadedPath = path
		return newPlugin, nil
	}

	if err := manager.ReloadTool(oldPlugin.Name()); err != nil {
		t.Fatalf("ReloadTool() error = %v", err)
	}

	if !oldPlugin.closed {
		t.Fatal("ReloadTool() did not close old plugin")
	}
	if oldPlugin.closeCalls != 1 {
		t.Fatalf("old plugin close calls = %d, want 1", oldPlugin.closeCalls)
	}
	if loadedPath != pluginPath {
		t.Fatalf("reload path = %q, want %q", loadedPath, pluginPath)
	}
	if got := manager.LoadedPlugins[oldPlugin.Name()]; got == nil || got.Name() != newPlugin.Name() {
		t.Fatalf("LoadedPlugins[%q] = %v, want reloaded plugin named %q", oldPlugin.Name(), got, newPlugin.Name())
	}
	if len(manager.LoadedPlugins) != 1 {
		t.Fatalf("LoadedPlugins len = %d, want 1", len(manager.LoadedPlugins))
	}
}

func TestReloadTool_NotFound(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	err := manager.ReloadTool("missing-tool")
	if err == nil {
		t.Fatal("ReloadTool() error = nil, want error")
	}
	if err.Error() != "plugin not found" {
		t.Fatalf("ReloadTool() error = %q, want %q", err.Error(), "plugin not found")
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
	if got := manager.LoadedPlugins[healthy.Name()]; got == nil || got.Name() != healthy.Name() {
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

func TestFullLifecycle_Reload(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	pluginPath := filepath.Join(manager.DiscoveryPath, "echo-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "echo-plugin")

	first := &stubLifecyclePlugin{
		name:            "lifecycle-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		outputs:         []string{"first-run"},
	}
	second := &stubLifecyclePlugin{
		name:            "lifecycle-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		outputs:         []string{"second-run"},
	}

	loadCalls := 0
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		if path != pluginPath {
			t.Fatalf("external load path = %q, want %q", path, pluginPath)
		}
		loadCalls++
		switch loadCalls {
		case 1:
			return first, nil
		case 2:
			return second, nil
		default:
			t.Fatalf("unexpected load call %d", loadCalls)
			return nil, nil
		}
	}

	tool, err := manager.Load(context.Background(), pluginPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	firstResult, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "before-reload"})
	if err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	if firstResult.Output != "first-run" {
		t.Fatalf("first Execute().Output = %q, want %q", firstResult.Output, "first-run")
	}
	loadedFirst, unavailable := manager.lookupManagedPlugin(tool.Name())
	if unavailable {
		t.Fatalf("plugin %q unexpectedly unavailable before reload", tool.Name())
	}
	if err := manager.healthCheck(loadedFirst); err != nil {
		t.Fatalf("healthCheck() before reload error = %v", err)
	}
	if first.healthCalls != 1 {
		t.Fatalf("first plugin health calls = %d, want 1", first.healthCalls)
	}

	if err := manager.ReloadTool(tool.Name()); err != nil {
		t.Fatalf("ReloadTool() error = %v", err)
	}
	if !first.closed {
		t.Fatal("ReloadTool() did not close first plugin")
	}

	reloaded := manager.LoadedPlugins[tool.Name()]
	if reloaded == nil {
		t.Fatalf("LoadedPlugins[%q] = nil after reload", tool.Name())
	}

	secondResult, err := reloaded.Execute(context.Background(), domain.ToolCall{Name: reloaded.Name(), Input: "after-reload"})
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if secondResult.Output != "second-run" {
		t.Fatalf("second Execute().Output = %q, want %q", secondResult.Output, "second-run")
	}
	loadedSecond, unavailable := manager.lookupManagedPlugin(reloaded.Name())
	if unavailable {
		t.Fatalf("plugin %q unexpectedly unavailable after reload", reloaded.Name())
	}
	if err := manager.healthCheck(loadedSecond); err != nil {
		t.Fatalf("healthCheck() after reload error = %v", err)
	}
	if second.healthCalls != 1 {
		t.Fatalf("second plugin health calls = %d, want 1", second.healthCalls)
	}
	if loadCalls != 2 {
		t.Fatalf("external load calls = %d, want 2", loadCalls)
	}
	if manager.failureCounts[tool.Name()] != 0 {
		t.Fatalf("failureCounts[%q] = %d, want 0", tool.Name(), manager.failureCounts[tool.Name()])
	}
	if manager.unavailable[tool.Name()] {
		t.Fatalf("plugin %q unexpectedly marked unavailable", tool.Name())
	}
}

func TestFullLifecycle_Recovery(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	pluginPath := filepath.Join(manager.DiscoveryPath, "crashy-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "crashy-plugin")

	crashed := &stubLifecyclePlugin{
		name:            "recoverable-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		healthErr:       errors.New("plugin process exited"),
		executeErrs:     []error{errors.New("plugin crashed")},
	}
	recovered := &stubLifecyclePlugin{
		name:            "recoverable-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		outputs:         []string{"recovered-output"},
	}

	loadCalls := 0
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		if path != pluginPath {
			t.Fatalf("external load path = %q, want %q", path, pluginPath)
		}
		loadCalls++
		if loadCalls == 1 {
			return crashed, nil
		}
		if loadCalls == 2 {
			return recovered, nil
		}
		t.Fatalf("unexpected load call %d", loadCalls)
		return nil, nil
	}

	tool, err := manager.Load(context.Background(), pluginPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "recover-me"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "recovered-output" {
		t.Fatalf("Execute().Output = %q, want %q", result.Output, "recovered-output")
	}
	if crashed.executeCalls != 1 {
		t.Fatalf("crashed plugin execute calls = %d, want 1", crashed.executeCalls)
	}
	if crashed.healthCalls != 1 {
		t.Fatalf("crashed plugin health calls = %d, want 1", crashed.healthCalls)
	}
	if !crashed.closed {
		t.Fatal("crashed plugin was not closed during recovery")
	}
	if recovered.executeCalls != 1 {
		t.Fatalf("recovered plugin execute calls = %d, want 1", recovered.executeCalls)
	}
	if loadCalls != 2 {
		t.Fatalf("external load calls = %d, want 2", loadCalls)
	}
	if manager.failureCounts[tool.Name()] != 0 {
		t.Fatalf("failureCounts[%q] = %d, want 0", tool.Name(), manager.failureCounts[tool.Name()])
	}
	if manager.unavailable[tool.Name()] {
		t.Fatalf("plugin %q unexpectedly marked unavailable", tool.Name())
	}
}

func TestFullLifecycle_MultiplePlugins(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	alphaPath := filepath.Join(manager.DiscoveryPath, "alpha-plugin")
	betaPath := filepath.Join(manager.DiscoveryPath, "beta-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "alpha-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "beta-plugin")

	alphaFirst := &stubLifecyclePlugin{
		name:            "alpha-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		outputs:         []string{"alpha-initial"},
	}
	alphaReloaded := &stubLifecyclePlugin{
		name:            "alpha-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		outputs:         []string{"alpha-reloaded"},
	}
	betaFirst := &stubLifecyclePlugin{
		name:            "beta-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		healthErr:       errors.New("beta exited"),
		executeErrs:     []error{errors.New("beta crash")},
	}
	betaRecovered := &stubLifecyclePlugin{
		name:            "beta-tool",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		outputs:         []string{"beta-recovered"},
	}

	alphaLoads := 0
	betaLoads := 0
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		switch path {
		case alphaPath:
			alphaLoads++
			if alphaLoads == 1 {
				return alphaFirst, nil
			}
			if alphaLoads == 2 {
				return alphaReloaded, nil
			}
			t.Fatalf("unexpected alpha load call %d", alphaLoads)
		case betaPath:
			betaLoads++
			if betaLoads == 1 {
				return betaFirst, nil
			}
			if betaLoads == 2 {
				return betaRecovered, nil
			}
			t.Fatalf("unexpected beta load call %d", betaLoads)
		default:
			t.Fatalf("unexpected load path %q", path)
		}
		return nil, nil
	}

	alphaTool, err := manager.Load(context.Background(), alphaPath)
	if err != nil {
		t.Fatalf("Load(alpha) error = %v", err)
	}
	betaTool, err := manager.Load(context.Background(), betaPath)
	if err != nil {
		t.Fatalf("Load(beta) error = %v", err)
	}

	alphaResult, err := alphaTool.Execute(context.Background(), domain.ToolCall{Name: alphaTool.Name(), Input: "alpha-before"})
	if err != nil {
		t.Fatalf("alpha Execute() before reload error = %v", err)
	}
	if alphaResult.Output != "alpha-initial" {
		t.Fatalf("alpha initial output = %q, want %q", alphaResult.Output, "alpha-initial")
	}
	alphaLoaded, unavailable := manager.lookupManagedPlugin(alphaTool.Name())
	if unavailable {
		t.Fatalf("alpha plugin %q unexpectedly unavailable before reload", alphaTool.Name())
	}
	if err := manager.healthCheck(alphaLoaded); err != nil {
		t.Fatalf("alpha healthCheck() before reload error = %v", err)
	}

	betaResult, err := betaTool.Execute(context.Background(), domain.ToolCall{Name: betaTool.Name(), Input: "beta-before"})
	if err != nil {
		t.Fatalf("beta Execute() recovery error = %v", err)
	}
	if betaResult.Output != "beta-recovered" {
		t.Fatalf("beta recovered output = %q, want %q", betaResult.Output, "beta-recovered")
	}

	if err := manager.ReloadTool(alphaTool.Name()); err != nil {
		t.Fatalf("ReloadTool(alpha) error = %v", err)
	}
	alphaReloadedTool := manager.LoadedPlugins[alphaTool.Name()]
	if alphaReloadedTool == nil {
		t.Fatalf("LoadedPlugins[%q] = nil after alpha reload", alphaTool.Name())
	}

	alphaReloadedResult, err := alphaReloadedTool.Execute(context.Background(), domain.ToolCall{Name: alphaReloadedTool.Name(), Input: "alpha-after"})
	if err != nil {
		t.Fatalf("alpha Execute() after reload error = %v", err)
	}
	if alphaReloadedResult.Output != "alpha-reloaded" {
		t.Fatalf("alpha reloaded output = %q, want %q", alphaReloadedResult.Output, "alpha-reloaded")
	}
	alphaLoadedAfterReload, unavailable := manager.lookupManagedPlugin(alphaReloadedTool.Name())
	if unavailable {
		t.Fatalf("alpha plugin %q unexpectedly unavailable after reload", alphaReloadedTool.Name())
	}
	if err := manager.healthCheck(alphaLoadedAfterReload); err != nil {
		t.Fatalf("alpha healthCheck() after reload error = %v", err)
	}

	if !alphaFirst.closed {
		t.Fatal("alpha first plugin was not closed on reload")
	}
	if !betaFirst.closed {
		t.Fatal("beta first plugin was not closed on recovery")
	}
	if alphaLoads != 2 || betaLoads != 2 {
		t.Fatalf("load counts alpha=%d beta=%d, want 2 each", alphaLoads, betaLoads)
	}
	if manager.failureCounts[alphaTool.Name()] != 0 {
		t.Fatalf("failureCounts[%q] = %d, want 0", alphaTool.Name(), manager.failureCounts[alphaTool.Name()])
	}
	if manager.failureCounts[betaTool.Name()] != 0 {
		t.Fatalf("failureCounts[%q] = %d, want 0", betaTool.Name(), manager.failureCounts[betaTool.Name()])
	}
	if len(manager.LoadedPlugins) != 2 {
		t.Fatalf("LoadedPlugins len = %d, want 2", len(manager.LoadedPlugins))
	}
	if manager.unavailable[alphaTool.Name()] || manager.unavailable[betaTool.Name()] {
		t.Fatalf("plugins unexpectedly marked unavailable: alpha=%v beta=%v", manager.unavailable[alphaTool.Name()], manager.unavailable[betaTool.Name()])
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

func TestAutoRecoveryFirstCrashRetriesOnce(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	pluginPath := filepath.Join(manager.DiscoveryPath, "echo-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "echo-plugin")

	var loadCount atomic.Int32
	crashy := &stubRecoveringPlugin{
		name:            "echo",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		executeErrs:     []error{errors.New("plugin crashed during execute")},
		healthErr:       errors.New("process exited"),
	}
	healthy := &stubRecoveringPlugin{
		name:            "echo",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
	}
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		if path != pluginPath {
			return nil, fmt.Errorf("unexpected load path %q", path)
		}
		if loadCount.Add(1) == 1 {
			return crashy, nil
		}
		return healthy, nil
	}

	tool, err := manager.Load(context.Background(), pluginPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "hello", Timeout: time.Second})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "hello" {
		t.Fatalf("Execute().Output = %q, want %q", result.Output, "hello")
	}
	if got := loadCount.Load(); got != 2 {
		t.Fatalf("load count = %d, want 2", got)
	}
	if crashy.healthCalls != 1 {
		t.Fatalf("health calls = %d, want 1", crashy.healthCalls)
	}
	if !crashy.closed {
		t.Fatal("expected crashed plugin to be closed during reload")
	}
	if got := manager.failureCount(tool.Name()); got != 0 {
		t.Fatalf("failure count = %d, want 0", got)
	}
	if manager.isUnavailable(tool.Name()) {
		t.Fatal("plugin unexpectedly marked unavailable")
	}
}

func TestAutoRecoverySecondFailureMarksUnavailable(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	pluginPath := filepath.Join(manager.DiscoveryPath, "echo-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "echo-plugin")

	var loadCount atomic.Int32
	first := &stubRecoveringPlugin{
		name:            "echo",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		executeErrs:     []error{errors.New("first crash")},
		healthErr:       errors.New("process exited"),
	}
	second := &stubRecoveringPlugin{
		name:            "echo",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		executeErrs:     []error{errors.New("second crash")},
		healthErr:       errors.New("process exited again"),
	}
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		if path != pluginPath {
			return nil, fmt.Errorf("unexpected load path %q", path)
		}
		if loadCount.Add(1) == 1 {
			return first, nil
		}
		return second, nil
	}

	tool, err := manager.Load(context.Background(), pluginPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if _, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "one", Timeout: time.Second}); err == nil {
		t.Fatal("first Execute() error = nil, want error")
	}
	if _, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "two", Timeout: time.Second}); err == nil {
		t.Fatal("second Execute() error = nil, want error")
	} else if !strings.Contains(err.Error(), "plugin unavailable") {
		t.Fatalf("second Execute() error = %v, want unavailable", err)
	}
	if got := loadCount.Load(); got != 2 {
		t.Fatalf("load count = %d, want 2", got)
	}
	if got := manager.failureCount(tool.Name()); got != 2 {
		t.Fatalf("failure count = %d, want 2", got)
	}
	if !manager.isUnavailable(tool.Name()) {
		t.Fatal("plugin should be marked unavailable")
	}
	if second.healthCalls != 0 {
		t.Fatalf("second plugin health calls = %d, want 0", second.healthCalls)
	}
}

func TestAutoRecoverySuccessfulExecutionResetsFailureCount(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	pluginPath := filepath.Join(manager.DiscoveryPath, "echo-plugin")
	writeTempPluginFile(t, manager.DiscoveryPath, "echo-plugin")

	var loadCount atomic.Int32
	first := &stubRecoveringPlugin{
		name:            "echo",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
		executeErrs:     []error{errors.New("first crash")},
		healthErr:       errors.New("process exited"),
	}
	second := &stubRecoveringPlugin{
		name:            "echo",
		contractVersion: ContractVersion,
		capabilities:    []string{"filesystem.read"},
	}
	manager.externalLoad = func(_ context.Context, path string) (PluginTool, error) {
		if path != pluginPath {
			return nil, fmt.Errorf("unexpected load path %q", path)
		}
		if loadCount.Add(1) == 1 {
			return first, nil
		}
		return second, nil
	}

	tool, err := manager.Load(context.Background(), pluginPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "ok", Timeout: time.Second})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "ok" {
		t.Fatalf("Execute().Output = %q, want %q", result.Output, "ok")
	}
	if got := manager.failureCount(tool.Name()); got != 0 {
		t.Fatalf("failure count after recovery = %d, want 0", got)
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

type stubRecoveringPlugin struct {
	name            string
	contractVersion string
	capabilities    []string
	executeErrs     []error
	executeCalls    int
	healthErr       error
	healthCalls     int
	closed          bool
	closeCalls      int
}

func (p *stubRecoveringPlugin) Name() string { return p.name }

func (p *stubRecoveringPlugin) Description() string { return "stub recovering plugin" }

func (p *stubRecoveringPlugin) Schema() string { return `{"type":"object"}` }

func (p *stubRecoveringPlugin) Capabilities() []string { return append([]string(nil), p.capabilities...) }

func (p *stubRecoveringPlugin) SafetyLevel() domain.SafetyLevel { return domain.SafetyLevelLow }

func (p *stubRecoveringPlugin) ContractVersion() string { return p.contractVersion }

func (p *stubRecoveringPlugin) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	p.executeCalls++
	if len(p.executeErrs) >= p.executeCalls {
		return domain.ToolResult{ToolName: call.Name}, p.executeErrs[p.executeCalls-1]
	}
	return domain.ToolResult{ToolName: call.Name, Output: call.Input}, nil
}

func (p *stubRecoveringPlugin) Close() error {
	p.closed = true
	p.closeCalls++
	return nil
}

func (p *stubRecoveringPlugin) HealthCheck(time.Duration) error {
	p.healthCalls++
	return p.healthErr
}

type stubLifecyclePlugin struct {
	name            string
	contractVersion string
	capabilities    []string
	outputs         []string
	executeErrs     []error
	executeCalls    int
	healthErr       error
	healthCalls     int
	closed          bool
	closeCalls      int
}

func (p *stubLifecyclePlugin) Name() string { return p.name }

func (p *stubLifecyclePlugin) Description() string { return "stub lifecycle plugin" }

func (p *stubLifecyclePlugin) Schema() string { return `{"type":"object"}` }

func (p *stubLifecyclePlugin) Capabilities() []string { return append([]string(nil), p.capabilities...) }

func (p *stubLifecyclePlugin) SafetyLevel() domain.SafetyLevel { return domain.SafetyLevelLow }

func (p *stubLifecyclePlugin) ContractVersion() string { return p.contractVersion }

func (p *stubLifecyclePlugin) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	p.executeCalls++
	idx := p.executeCalls - 1
	if idx < len(p.executeErrs) && p.executeErrs[idx] != nil {
		return domain.ToolResult{ToolName: call.Name}, p.executeErrs[idx]
	}
	output := call.Input
	if idx < len(p.outputs) {
		output = p.outputs[idx]
	}
	return domain.ToolResult{ToolName: call.Name, Output: output}, nil
}

func (p *stubLifecyclePlugin) Close() error {
	p.closed = true
	p.closeCalls++
	return nil
}

func (p *stubLifecyclePlugin) HealthCheck(time.Duration) error {
	p.healthCalls++
	return p.healthErr
}

func writeTempPluginFile(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("stub"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
