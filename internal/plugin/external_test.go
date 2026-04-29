package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestExternalPluginInitialize(t *testing.T) {
	t.Parallel()

	tool := loadEchoPlugin(t, "", nil, 0)
	t.Cleanup(func() { _ = tool.Close() })

	if got := tool.Name(); got != "echo" {
		t.Fatalf("Name() = %q, want %q", got, "echo")
	}
	if got := tool.Description(); got != "echoes the provided input" {
		t.Fatalf("Description() = %q, want %q", got, "echoes the provided input")
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
	if err := ValidateContract(tool); err != nil {
		t.Fatalf("ValidateContract() error = %v", err)
	}
}

func TestExternalPluginExecute(t *testing.T) {
	t.Parallel()

	tool := loadEchoPlugin(t, "", nil, 0)
	t.Cleanup(func() { _ = tool.Close() })

	result, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "hello plugin", Timeout: time.Second})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ToolName != "echo" {
		t.Fatalf("Execute().ToolName = %q, want %q", result.ToolName, "echo")
	}
	if result.Output != "hello plugin" {
		t.Fatalf("Execute().Output = %q, want %q", result.Output, "hello plugin")
	}
	if result.Duration <= 0 {
		t.Fatalf("Execute().Duration = %v, want > 0", result.Duration)
	}
}

func TestExternalPluginCrashRecovery(t *testing.T) {
	t.Parallel()

	tool := loadEchoPlugin(t, "crash_execute", nil, 0)

	_, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "boom", Timeout: time.Second})
	if err == nil {
		t.Fatal("expected Execute() error")
	}
	if !strings.Contains(err.Error(), "decode tool.execute response") && !strings.Contains(err.Error(), "stderr=") {
		t.Fatalf("Execute() error = %v, want crash/protocol details", err)
	}
	if _, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "retry"}); !errors.Is(err, ErrExternalPluginClosed) {
		t.Fatalf("second Execute() error = %v, want %v", err, ErrExternalPluginClosed)
	}
	if err := tool.Close(); err != nil {
		t.Fatalf("Close() after crash error = %v", err)
	}
}

func TestExternalPluginMalformedResponse(t *testing.T) {
	t.Parallel()

	tool := loadEchoPlugin(t, "malformed_execute", nil, 0)

	_, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "bad", Timeout: time.Second})
	if err == nil {
		t.Fatal("expected Execute() error")
	}
	if !strings.Contains(err.Error(), ErrExternalPluginProtocol.Error()) && !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("Execute() error = %v, want protocol error", err)
	}
	if err := tool.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestExternalPluginStartupTimeout(t *testing.T) {
	t.Parallel()

	loader := newEchoPluginLoader(t, "slow_initialize", nil, 50*time.Millisecond)

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected Load() timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Load() error = %v, want timeout", err)
	}
}

func TestExternalPluginShutdown(t *testing.T) {
	t.Parallel()

	shutdownFile := filepath.Join(t.TempDir(), "shutdown.txt")
	tool := loadEchoPlugin(t, "", map[string]string{"ECHO_PLUGIN_SHUTDOWN_FILE": shutdownFile}, 0)

	if err := tool.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	contents, err := os.ReadFile(shutdownFile)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", shutdownFile, err)
	}
	if strings.TrimSpace(string(contents)) != "shutdown" {
		t.Fatalf("shutdown marker = %q, want %q", strings.TrimSpace(string(contents)), "shutdown")
	}
	if err := tool.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func loadEchoPlugin(t *testing.T, mode string, extraEnv map[string]string, startupTimeout time.Duration) *ExternalPluginTool {
	t.Helper()

	tool, err := newEchoPluginLoader(t, mode, extraEnv, startupTimeout).Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return tool
}

func newEchoPluginLoader(t *testing.T, mode string, extraEnv map[string]string, startupTimeout time.Duration) ExternalLoader {
	t.Helper()

	env := append([]string{}, os.Environ()...)
	if mode != "" {
		env = append(env, "ECHO_PLUGIN_MODE="+mode)
	}
	for key, value := range extraEnv {
		env = append(env, key+"="+value)
	}

	return ExternalLoader{
		Command:        buildEchoPluginBinary(t),
		Env:            env,
		StartupTimeout: startupTimeout,
	}
}

var (
	buildEchoPluginOnce sync.Once
	buildEchoPluginPath string
	buildEchoPluginErr  error
)

func buildEchoPluginBinary(t *testing.T) string {
	t.Helper()

	buildEchoPluginOnce.Do(func() {
		repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			buildEchoPluginErr = err
			return
		}
		outputDir, err := os.MkdirTemp("", "echo-plugin-build-*")
		if err != nil {
			buildEchoPluginErr = err
			return
		}
		binaryName := "echo_plugin"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		buildEchoPluginPath = filepath.Join(outputDir, binaryName)
		cmd := exec.Command("go", "build", "-o", buildEchoPluginPath, "./testdata/plugins/echo_plugin")
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildEchoPluginErr = errors.New(string(output))
		}
	})

	if buildEchoPluginErr != nil {
		t.Fatalf("build echo plugin: %v", buildEchoPluginErr)
	}
	return buildEchoPluginPath
}
