# ADR-007: Plugin System Architecture

**Date**: 2026-04-28
**Status**: Accepted
**Related ADRs**: ADR-003 (No Plugin System in v1), ADR-006 (Streaming Architecture)

## Context

In v1, the decision was made to NOT include a plugin system (ADR-003) to keep the MVP scope small and focused. As v2 evolves, the need for extensible tool ecosystem has become a priority. This ADR documents the decision to implement a dual-mode plugin system.

## Decision

We will implement a **dual-mode plugin system**:

1. **External Process Mode (Portable Baseline)**
   - Tool plugins run as separate OS processes
   - Communication via JSON-RPC 2.0 over stdio
   - Works on all platforms (Linux, macOS, Windows)
   - Protocol: initialize → tool_call → shutdown

2. **Go Plugin Mode (Performance Optimization)**
   - Native Go plugins (.so files) loaded at runtime
   - Available only on Linux and macOS (Go plugin limitation)
   - Disabled on Windows via build tags
   - Provides lower latency for trusted local tools

**v2 Scope Limitation**: Only tool plugins are in scope. Provider plugins, agent plugins, and verifier plugins are explicitly out of scope for v2.

## Technical Details

### Plugin Contract

All plugins must implement the PluginTool interface:

```go
type PluginTool interface {
    Name() string
    Description() string
    Schema() string // JSON schema for input validation
    SafetyLevel() domain.SafetyLevel
    ContractVersion() string // Must match host contract version
    Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error)
    Close() error // Cleanup for external processes
}
```

### Protocol: JSON-RPC 2.0 over stdio

```
// Host → Plugin
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"contract_version":"1.0.0"}}
{"jsonrpc":"2.0","id":2,"method":"tool.info","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tool.execute","params":{"name":"...","input":"..."}}
{"jsonrpc":"2.0","id":4,"method":"shutdown","params":{}}

// Plugin → Host
{"jsonrpc":"2.0","id":1,"result":{"name":"...","description":"...","schema":"...","safety_level":"medium","contract_version":"1.0.0"}}
{"jsonrpc":"2.0","id":3,"result":{"output":"...","error":""}}
```

### Security Model

Plugins are treated as **trusted local extensions** in v2. No sandboxing or capability-based permissions are implemented. This aligns with the assumption that plugins are developed by the same team or from trusted sources.

Security policy extensions:
- `AllowedPluginPaths`: Directories where plugins can be loaded from
- `AllowedPluginTools`: Tool names allowed from plugins (empty = all allowed)
- `DeniedPluginTools`: Tool names explicitly denied

### Windows Strategy

On Windows, native Go plugin loading is disabled via build tags (`//go:build !windows`). Only external process mode is available. This ensures cross-platform compatibility while maintaining performance on supported platforms.

## Consequences

### Positive
- Extensible tool ecosystem without recompiling the core application
- Cross-platform support via external process mode
- Lower latency for local tools via Go plugin mode on supported platforms
- Clear contract version enables future migrations

### Negative
- Increased complexity in tool loading and lifecycle management
- External process mode adds IPC overhead
- Security model requires trust (no sandboxing)

## References

- ADR-003: No Plugin System in v1 (reversed by this ADR)
- ADR-006: Streaming Architecture (uses similar event-based patterns)
- Go plugin package: https://pkg.go.dev/plugin
- HashiCorp go-plugin: https://github.com/hashicorp/go-plugin (reference pattern)