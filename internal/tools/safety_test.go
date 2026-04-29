package tools

import (
	"strings"
	"testing"
)

func TestPluginSafety(t *testing.T) {
	t.Parallel()

	t.Run("allows plugin within configured roots", func(t *testing.T) {
		t.Parallel()

		workspace := t.TempDir()
		policy := SafetyPolicy{
			WorkspaceRoot:      workspace,
			AllowedPluginPaths: []string{"plugins", "vendor/plugins"},
		}

		if err := policy.ValidatePluginPath("plugins/echo-plugin"); err != nil {
			t.Fatalf("ValidatePluginPath() error = %v, want nil", err)
		}
		if err := policy.ValidatePluginPath("vendor/plugins/inspect.so"); err != nil {
			t.Fatalf("ValidatePluginPath() secondary root error = %v, want nil", err)
		}
	})

	t.Run("rejects plugin outside configured roots", func(t *testing.T) {
		t.Parallel()

		workspace := t.TempDir()
		policy := SafetyPolicy{
			WorkspaceRoot:      workspace,
			AllowedPluginPaths: []string{"plugins"},
		}

		err := policy.ValidatePluginPath("other/echo-plugin")
		if err == nil || !strings.Contains(err.Error(), "escapes allowed roots") {
			t.Fatalf("ValidatePluginPath() error = %v, want allowlist rejection", err)
		}
	})

	t.Run("allows any plugin path when allowlist is empty", func(t *testing.T) {
		t.Parallel()

		policy := SafetyPolicy{WorkspaceRoot: t.TempDir()}
		if err := policy.ValidatePluginPath("anywhere/plugin"); err != nil {
			t.Fatalf("ValidatePluginPath() unrestricted error = %v, want nil", err)
		}
	})

	t.Run("matches declared capabilities case-insensitively", func(t *testing.T) {
		t.Parallel()

		policy := SafetyPolicy{PluginCapabilities: []string{"filesystem.read", "web.fetch"}}
		if !policy.DeclaresPluginCapability("FILESYSTEM.READ") {
			t.Fatal("DeclaresPluginCapability() = false, want true")
		}
		if policy.DeclaresPluginCapability("shell.exec") {
			t.Fatal("DeclaresPluginCapability() = true, want false")
		}
	})

	t.Run("validates plugin capability allowlist", func(t *testing.T) {
		t.Parallel()

		policy := SafetyPolicy{PluginCapabilities: []string{"filesystem.read", "web.fetch"}}
		if err := policy.ValidatePluginCapabilities([]string{"FILESYSTEM.READ"}); err != nil {
			t.Fatalf("ValidatePluginCapabilities() error = %v, want nil", err)
		}

		err := policy.ValidatePluginCapabilities([]string{"shell.exec"})
		if err == nil || !strings.Contains(err.Error(), `plugin capability "shell.exec" is not allowed`) {
			t.Fatalf("ValidatePluginCapabilities() error = %v, want capability rejection", err)
		}
	})

	t.Run("requires declared plugin capabilities when policy configured", func(t *testing.T) {
		t.Parallel()

		policy := SafetyPolicy{PluginCapabilities: []string{"filesystem.read"}}
		err := policy.ValidatePluginCapabilities(nil)
		if err == nil || !strings.Contains(err.Error(), "plugin capabilities must be declared") {
			t.Fatalf("ValidatePluginCapabilities() error = %v, want missing declaration rejection", err)
		}
	})
}
