package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"zheng-harness/internal/config"
)

func TestCLIUnaffectedByServerConfiguration(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "agent.db")
	configPath := filepath.Join(t.TempDir(), "zheng.json")
	if err := os.WriteFile(configPath, []byte(`{
		"server": {
			"listen_address": "127.0.0.1:9090",
			"jwt_secret": "server-only-secret",
			"active_session_cap": 3,
			"shutdown_timeout": "2s"
		},
		"runtime": {
			"verify_mode": "off"
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	var runStdout bytes.Buffer
	var runStderr bytes.Buffer
	exitCode := runCLI(context.Background(), []string{"run", "--task", "cli still works", "--config", configPath, "--db", dbPath, "--json"}, &runStdout, &runStderr)
	if exitCode != 0 {
		t.Fatalf("run exit code = %d, want 0, stderr=%s", exitCode, runStderr.String())
	}
	if runStderr.Len() != 0 {
		t.Fatalf("run stderr = %q, want empty", runStderr.String())
	}
	var runPayload runJSONOutput
	if err := json.Unmarshal(runStdout.Bytes(), &runPayload); err != nil {
		t.Fatalf("unmarshal run payload: %v\noutput=%s", err, runStdout.String())
	}
	if runPayload.Command != "run" {
		t.Fatalf("run command = %q, want run", runPayload.Command)
	}

	var inspectStdout bytes.Buffer
	var inspectStderr bytes.Buffer
	exitCode = runCLI(context.Background(), []string{"inspect", "--session", runPayload.SessionID, "--config", configPath, "--db", dbPath, "--json"}, &inspectStdout, &inspectStderr)
	if exitCode != 0 {
		t.Fatalf("inspect exit code = %d, want 0, stderr=%s", exitCode, inspectStderr.String())
	}
	if inspectStderr.Len() != 0 {
		t.Fatalf("inspect stderr = %q, want empty", inspectStderr.String())
	}
	var inspectPayload inspectJSONOutput
	if err := json.Unmarshal(inspectStdout.Bytes(), &inspectPayload); err != nil {
		t.Fatalf("unmarshal inspect payload: %v\noutput=%s", err, inspectStdout.String())
	}
	if inspectPayload.Command != "inspect" {
		t.Fatalf("inspect command = %q, want inspect", inspectPayload.Command)
	}
	if bytes.Contains(runStdout.Bytes(), []byte("server-only-secret")) || bytes.Contains(inspectStdout.Bytes(), []byte("server-only-secret")) {
		t.Fatal("CLI output leaked server JWT configuration")
	}
}

func TestServerFlagsDoNotChangeCLIVerifyDefaults(t *testing.T) {
	t.Parallel()

	loaded, err := config.Load([]string{"--jwt-secret", "server-secret", "--listen-address", "127.0.0.1:8089"})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if loaded.Runtime.VerifyMode != config.VerifyModeStandard {
		t.Fatalf("runtime verify mode = %q, want %q", loaded.Runtime.VerifyMode, config.VerifyModeStandard)
	}
	if loaded.Server.JWTSecret != "server-secret" {
		t.Fatalf("server jwt secret = %q, want server-secret", loaded.Server.JWTSecret)
	}
}
