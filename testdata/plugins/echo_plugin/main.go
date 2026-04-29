package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"zheng-harness/internal/domain"
	pluginpkg "zheng-harness/internal/plugin"
)

const echoSchema = `{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}`

type executeParams struct {
	Name      string `json:"name"`
	Input     string `json:"input"`
	TimeoutMS int64  `json:"timeout_ms,omitempty"`
}

func main() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	mode := os.Getenv("ECHO_PLUGIN_MODE")

	for {
		var request pluginpkg.JSONRPCRequest
		if err := decoder.Decode(&request); err != nil {
			if err.Error() != "EOF" {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			return
		}

		switch request.Method {
		case "initialize":
			if mode == "slow_initialize" {
				time.Sleep(durationFromEnv("ECHO_PLUGIN_SLEEP_MS", 250*time.Millisecond))
			}
			mustEncode(encoder, pluginpkg.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Result:  mustRaw(map[string]any{"contract_version": pluginpkg.ContractVersion}),
			})
		case "tool.info":
			if mode == "malformed_info" {
				_, _ = fmt.Fprintln(os.Stdout, "{malformed")
				return
			}
			capabilities := []string{"filesystem.read"}
			if mode == "disallowed_capability" {
				capabilities = []string{"shell.exec"}
			}
			if mode == "missing_capabilities" {
				capabilities = nil
			}
			mustEncode(encoder, pluginpkg.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Result: mustRaw(map[string]any{
					"name":             "echo",
					"description":      "echoes the provided input",
					"schema":           echoSchema,
					"capabilities":     capabilities,
					"safety_level":     domain.SafetyLevelLow,
					"contract_version": pluginpkg.ContractVersion,
				}),
			})
		case "tool.execute":
			switch mode {
			case "crash_execute":
				fmt.Fprintln(os.Stderr, "crashing during execute")
				os.Exit(2)
			case "malformed_execute":
				_, _ = fmt.Fprintln(os.Stdout, "{malformed")
				return
			}

			var params executeParams
			if len(request.Params) > 0 {
				if err := json.Unmarshal(request.Params, &params); err != nil {
					mustEncode(encoder, pluginpkg.JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      request.ID,
						Error: &pluginpkg.JSONRPCError{Code: -32602, Message: err.Error()},
					})
					continue
				}
			}
			mustEncode(encoder, pluginpkg.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Result: mustRaw(map[string]any{
					"tool_name": "echo",
					"output":    params.Input,
				}),
			})
		case "shutdown":
			if shutdownFile := os.Getenv("ECHO_PLUGIN_SHUTDOWN_FILE"); shutdownFile != "" {
				_ = os.WriteFile(shutdownFile, []byte("shutdown\n"), 0o644)
			}
			mustEncode(encoder, pluginpkg.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Result:  mustRaw(true),
			})
			return
		default:
			mustEncode(encoder, pluginpkg.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error:   &pluginpkg.JSONRPCError{Code: -32601, Message: "method not found"},
			})
		}
	}
}

func mustEncode(encoder *json.Encoder, response pluginpkg.JSONRPCResponse) {
	if err := encoder.Encode(response); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func mustRaw(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	ms, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}
