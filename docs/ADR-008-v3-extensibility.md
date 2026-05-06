# ADR-008: v3 Extensibility Boundaries and Plugin Family Contracts

**Date**: 2026-04-30
**Status**: Accepted
**Related ADRs**: ADR-003 (No Plugin System in v1), ADR-004 (No Vector Database), ADR-007 (Plugin System Architecture)

## Context

v2 implemented a dual-mode plugin system for tools only (ADR-007), explicitly deferring provider, agent, and verifier plugins to later versions. As v3 planning begins, we need clear boundaries for what extensibility means, what plugin families are in scope, and what remains out of scope.

This ADR establishes the extensibility model for v3: three distinct plugin families with separate contracts, fail-closed runtime behavior, and explicit non-goals that preserve the harness engineering philosophy.

## Decision

### Three Plugin Families

v3 defines **three distinct plugin families**, each with its own contract and loading semantics. We explicitly reject a single generic meta-plugin interface in favor of family-specific contracts.

#### 1. Provider Family

**Purpose**: Abstract LLM backend differences (model APIs, streaming protocols, token accounting).

**Contract interface**:
```go
type ProviderPlugin interface {
    Name() string
    Model() string
    StreamPlan(ctx context.Context, plan domain.Plan) (<-chan domain.TokenDelta, error)
    StreamChooseNext(ctx context.Context, state domain.State) (<-chan domain.ToolCallDelta, error)
    StreamSummarize(ctx context.Context, history []domain.Step) (<-chan domain.Observation, error)
    Close() error
}
```

**Loading semantics**: External process (JSON-RPC 2.0 over stdio) is the canonical baseline. Go plugin mode (.so loading) is permitted on Linux/macOS as an optimization.

**Built-in defaults**: dashscope, openai providers remain first-class. Provider plugins are **opt-in**. Zero plugins installed by default.

#### 2. Verifier Family

**Purpose**: Task-aware verification strategies that inspect evidence and decide completion.

**Contract interface**:
```go
type VerifierPlugin interface {
    Name() string
    TaskTypes() []domain.TaskType // Which task types this verifier handles
    Verify(ctx context.Context, session domain.Session) (domain.VerificationResult, error)
    Close() error
}
```

**Loading semantics**: External process is canonical. Go plugin mode permitted on supported platforms.

**Built-in defaults**: TaskAwareVerifier with policy dispatch (standard, strict, off) remains first-class. Verifier plugins are **opt-in**.

#### 3. Agent Strategy Family

**Purpose**: Alternative reasoning and action-selection strategies beyond the default plan-execute-verify loop.

**Contract interface**:
```go
type AgentStrategyPlugin interface {
    Name() string
    SelectNext(ctx context.Context, state domain.State) (domain.ToolCall, error)
    ShouldTerminate(ctx context.Context, history []domain.Step) (bool, domain.Observation)
    Close() error
}
```

**Loading semantics**: External process is canonical. Go plugin mode permitted on supported platforms.

**Built-in defaults**: Default plan-execute-verify loop remains first-class. Agent strategy plugins are **opt-in**.

### Fail-Closed Runtime Behavior

All plugin families follow **fail-closed** semantics:

1. **Plugin load failure**: If a plugin fails to load (path not found, contract version mismatch, protocol error), the plugin is rejected and not registered. The runtime continues with built-in defaults.

2. **Plugin execution failure**: If a plugin call times out, panics, or returns malformed data, the call fails and the error is surfaced to the verifier. The runtime does not retry automatically beyond the configured step retry limit.

3. **Graceful degradation**: Plugin failures never crash the host. The runtime logs the failure, marks the plugin as unavailable for the current session, and continues with remaining tools/providers/verifiers.

### Inspect / Readability Guarantees

**Canonical portability rule**: External-process loading (JSON-RPC 2.0 over stdio) is the semantic baseline for all plugin families. This guarantees:

- **Platform independence**: Same protocol on Linux, macOS, Windows
- **Language neutrality**: Plugins can be written in any language that speaks JSON-RPC 2.0
- **Debugging clarity**: Protocol traces are human-readable
- **No CGO hell**: No cgo dependency chains, no ABI compatibility nightmares

Go plugin mode is an **optimization** for trusted local tools on supported platforms, but external process remains the reference implementation.

### Collision Rejection Rules

The runtime enforces strict namespace collision policies:

1. **Tool/tool name collision**: If a plugin tool has the same name as a built-in tool, the plugin tool is rejected with a clear error message. Built-in tools always win.

2. **Provider collision**: If a plugin provider has the same name as a configured built-in provider, the plugin is rejected.

3. **Verifier collision**: If a plugin verifier claims the same task types as a built-in verifier, the plugin is rejected.

4. **Agent strategy collision**: If a plugin agent strategy has the same name as the default strategy, the plugin is rejected.

Rationale: Built-ins are first-class defaults. Plugins extend, never replace. This prevents accidental shadowing and maintains predictable behavior.

### Provenance Persistence Requirements

All plugin invocations must be auditable:

1. **Plugin identity**: Plugin name, version, and source path are recorded in the session history for each step that uses the plugin.

2. **Contract version**: The plugin contract version is persisted alongside each tool call for future replay compatibility.

3. **Failure attribution**: Plugin-caused failures include the plugin identity in error messages and session logs.

4. **No hidden state**: Plugins cannot persist state outside the session database. All observable effects must flow through the tool call / observation interface.

## Consequences

### Positive

- **Clear boundaries**: Three families with distinct contracts prevent the "one interface to rule them all" trap that led to ADR-003's original deferral.
- **Fail-closed safety**: Plugin misbehavior cannot brick the runtime, preserving the harness engineering principle of central safety policy.
- **Inspectability**: External-process baseline means every plugin interaction is traceable and debuggable.
- **Built-in supremacy**: First-class defaults mean the runtime is always usable without plugins.

### Negative

- **No marketplace after the fact**: We are explicitly not building a plugin marketplace, discovery service, or version resolver. Plugins are loaded by explicit configuration, not discovered.
- **No HTTP API server**: We will not expose plugin loading or execution via HTTP. The CLI and embedded library are the only supported interfaces.
- **No vector database**: Provider plugins do not change ADR-004. No embedding storage, no similarity search, no knowledge graph.
- **No recursive agents**: Agent strategy plugins cannot spawn sub-agents that themselves use agent strategy plugins. Recursion depth is bounded at 1.

### Neutral

- **Configuration burden**: Users must explicitly configure plugin paths and contracts. No "just works" auto-discovery.
- **Contract evolution**: Future contract changes (v3.1, v3.2) require stub adapters or re-compilation of plugins.

## Non-Goals (v3 Boundaries)

**Explicitly out of scope for v3**:

1. **Marketplace / discovery service**: No plugin registry, no version negotiation, no auto-update.
2. **HTTP API server**: No REST or WebSocket interface for remote plugin loading or execution.
3. **Vector database**: No embedding storage, no semantic search, no RAG infrastructure.
4. **Recursive agents**: No agent spawning sub-agents that spawn sub-agents (bounded depth = 1).
5. **Hot reload**: No dynamic plugin reloading without session restart.
6. **Backward-incompatible config changes**: Plugin configuration follows the same JSON schema stability rules as other config areas.

## Migration from v2

v2 tool plugins continue to work in v3 with no changes. Provider, verifier, and agent_strategy plugin families are additive and do not break v2 tool plugin contracts.

## References

- ADR-003: No Plugin System in v1 (original deferral rationale)
- ADR-004: No Vector Database for MVP Memory (still in effect for v3)
- ADR-007: Plugin System Architecture (v2 tool plugin baseline)
- internal/plugin/doc.go: v2 plugin contract documentation
