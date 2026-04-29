//go:build !windows

package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestNativePluginLoad(t *testing.T) {
	t.Parallel()

	tool := loadNativePlugin(t, buildNativePlugin(t, filepath.Join("testdata", "plugins", "native_example")))
	t.Cleanup(func() { _ = tool.Close() })

	if got := tool.Name(); got != "native_example" {
		t.Fatalf("Name() = %q, want %q", got, "native_example")
	}
	if got := tool.Description(); got != "echoes the provided input via native Go plugin" {
		t.Fatalf("Description() = %q", got)
	}
	if got := tool.Schema(); got != `{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}` {
		t.Fatalf("Schema() = %q", got)
	}
	if got := tool.SafetyLevel(); got != domain.SafetyLevelLow {
		t.Fatalf("SafetyLevel() = %q, want %q", got, domain.SafetyLevelLow)
	}
	if got := tool.ContractVersion(); got != ContractVersion {
		t.Fatalf("ContractVersion() = %q, want %q", got, ContractVersion)
	}

	result, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "hello native", Timeout: time.Second})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ToolName != "native_example" {
		t.Fatalf("Execute().ToolName = %q, want %q", result.ToolName, "native_example")
	}
	if result.Output != "hello native" {
		t.Fatalf("Execute().Output = %q, want %q", result.Output, "hello native")
	}
}

func TestNativePluginVersionMismatch(t *testing.T) {
	t.Parallel()

	pluginPath := buildPluginFromSource(t, nativePluginSource(`0.9.0`, true))

	_, err := NativeLoader{Path: pluginPath}.Load(context.Background())
	if err == nil {
		t.Fatal("expected Load() error")
	}
	if !errors.Is(err, ErrContractVersionMismatch) {
		t.Fatalf("Load() error = %v, want %v", err, ErrContractVersionMismatch)
	}
}

func TestNativePluginSymbolNotFound(t *testing.T) {
	t.Parallel()

	pluginPath := buildPluginFromSource(t, nativePluginSource(ContractVersion, false))

	_, err := NativeLoader{Path: pluginPath}.Load(context.Background())
	if err == nil {
		t.Fatal("expected Load() error")
	}
	if !errors.Is(err, ErrNativePluginSymbolNotFound) {
		t.Fatalf("Load() error = %v, want %v", err, ErrNativePluginSymbolNotFound)
	}
	if !strings.Contains(err.Error(), "NewPluginTool") {
		t.Fatalf("Load() error = %v, want missing symbol details", err)
	}
}

func loadNativePlugin(t *testing.T, pluginPath string) PluginTool {
	t.Helper()

	loader := NativeLoader{Path: pluginPath}
	if !loader.CanLoad("") {
		t.Fatalf("CanLoad() = false, want true for %q", pluginPath)
	}

	tool, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return tool
}

func buildNativePlugin(t *testing.T, relativeDir string) string {
	t.Helper()

	repoRoot := repoRoot(t)
	return buildPluginAtDir(t, filepath.Join(repoRoot, relativeDir))
}

func buildPluginFromSource(t *testing.T, source string) string {
	t.Helper()

	repoRoot := repoRoot(t)
	tempDir, err := os.MkdirTemp(repoRoot, ".native-plugin-src-*")
	if err != nil {
		t.Fatalf("MkdirTemp(%q): %v", repoRoot, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	mainPath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", mainPath, err)
	}
	return buildPluginAtDir(t, tempDir)
}

func buildPluginAtDir(t *testing.T, pluginDir string) string {
	t.Helper()

	outputPath := filepath.Join(t.TempDir(), "plugin.so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outputPath, ".")
	cmd.Dir = pluginDir
	cmd.Env = append([]string{}, os.Environ()...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build native plugin in %q: %v\n%s", pluginDir, err, string(output))
	}
	return outputPath
}

func repoRoot(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return repoRoot
}

func nativePluginSource(version string, exportFactory bool) string {
	factory := "func MissingPluginTool() pluginpkg.PluginTool { return stubNativePlugin{} }"
	if exportFactory {
		factory = "func NewPluginTool() pluginpkg.PluginTool { return stubNativePlugin{} }"
	}

	return fmt.Sprintf(`package main

import (
	"context"

	"zheng-harness/internal/domain"
	pluginpkg "zheng-harness/internal/plugin"
)

type stubNativePlugin struct{}

func (stubNativePlugin) Name() string { return "stub-native" }
func (stubNativePlugin) Description() string { return "stub native plugin" }
func (stubNativePlugin) Schema() string { return "{\"type\":\"object\"}" }
func (stubNativePlugin) Capabilities() []string { return []string{"filesystem.read"} }
func (stubNativePlugin) SafetyLevel() domain.SafetyLevel { return domain.SafetyLevelLow }
func (stubNativePlugin) ContractVersion() string { return %q }
func (stubNativePlugin) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{ToolName: call.Name, Output: call.Input}, nil
}
func (stubNativePlugin) Close() error { return nil }

%s
`, version, factory)
}

func TestNativeLoaderCanLoad(t *testing.T) {
	t.Parallel()

	loader := NativeLoader{Path: filepath.Join("tmp", "plugin.so")}
	if !loader.CanLoad("") {
		t.Fatal("CanLoad() = false, want true")
	}
	if loader.CanLoad(filepath.Join("tmp", "plugin.exe")) {
		t.Fatal("CanLoad() = true, want false for .exe")
	}
}
