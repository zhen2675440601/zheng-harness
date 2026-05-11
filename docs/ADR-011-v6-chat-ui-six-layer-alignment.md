# ADR-011: v6 Chat-First UI and Six-Layer Harness Alignment

**Date**: 2026-05-11
**Status**: Proposed
**Related ADRs**: ADR-010 (v5 Web UI Scope Freeze), ADR-009 (v4 API Server Productization), ADR-006 (Streaming Runtime Architecture)

## Context

v5 delivered a same-origin embedded Web UI that made the harness browsable, but the primary interaction model is task-submission centric: users navigate to a "new task" form, submit a single task, and watch execution on a stream page. This matches an operator/dashboard workflow but does not match the conversational interaction expected of a general-purpose agent.

Additionally, the current code organization has business orchestration logic embedded directly in HTTP handlers (`internal/server/api.go`), without an explicit application/service boundary. This makes the architecture harder to audit, test, and align with industry-standard Harness Engineering practices.

OpenAI's Harness Engineering model defines a six-layer code organization within business domains: `Types → Config → Repo → Service → Runtime → UI`. This ADR applies that model to zheng-harness, identifies current misalignments, and defines the target architecture for v6.

## Decision

### 1. Six-Layer Baseline

The public OpenAI Harness Engineering baseline defines these layers (from most stable to most volatile):

| Layer | Responsibility | Dependency Direction |
|-------|---------------|---------------------|
| **Types** | Domain contracts, value objects, interfaces | Depends on nothing |
| **Config** | Configuration schema, feature flags, environment wiring | Depends on Types |
| **Repo** | Persistence, storage adapters, data access | Depends on Types |
| **Service** | Application use cases, business orchestration, transactions | Depends on Types, Config, Repo |
| **Runtime** | Execution engine, tools, plugins, verification, LLM adapters | Depends on Types, Config |
| **UI** | HTTP transport, embedded web assets, SSE streaming endpoints | Depends on Service, Runtime |

### 2. Current Package Mapping

| Package | Layer | Rationale |
|---------|-------|-----------|
| `internal/domain` | **Types** | Session, Task, Step, Event, Provenance contracts |
| `internal/config` | **Config** | Provider settings, runtime/server/web config |
| `internal/store` | **Repo** | SQLite session/memory/step persistence, lifecycle tracking |
| `internal/runtime` | **Runtime** | Session manager, agent strategy, event channels, model adapter |
| `internal/runtimebuilder` | **Config/Runtime** | Builder for wiring runtime components |
| `internal/orchestration` | **Runtime** | DAG scheduler, worker pool, aggregation |
| `internal/tools` | **Runtime** | Tool implementations (bash, file, search, web, ask) |
| `internal/verify` | **Runtime** | Task-aware, command, and non-coding verifiers |
| `internal/plugin` | **Runtime** | Plugin manager, registry, native/external loaders |
| `internal/memory` | **Runtime** | Memory store and retrieval |
| `internal/llm` | **Runtime** | Provider implementations (OpenAI, Anthropic, DashScope), streaming |
| `internal/server` | **UI + Service** | HTTP handlers (UI), embedded web assets (UI), business orchestration (misplaced in UI) |

### 3. Current Misalignments

**Misalignment 1: Business logic embedded in HTTP handlers**
- **Location**: `internal/server/api.go:176-344`
- **Problem**: `HandleRun` and `HandleResume` directly construct session IDs, build runner factories, persist state, and orchestrate execution start. This mixes HTTP concerns (request parsing, response writing) with business logic (session creation, runner factory assembly, failure handling).
- **Target**: Extract orchestration into a dedicated Service layer. HTTP handlers should delegate to service methods.

**Misalignment 2: No application/service boundary layer**
- **Location**: Project-wide — no `internal/service` or equivalent package exists
- **Problem**: The `API` struct in `internal/server/api.go:29-39` directly composes `SessionStore`, `MemoryStore`, `Manager`, and `Builder` — creating a direct transport→store/runtime dependency chain without an intermediate service abstraction
- **Target**: Introduce a `Service` or `app` package that owns use-case orchestration and is callable from both HTTP handlers and CLI entrypoints.

**Misalignment 3: Domain model is task-centric, not conversation/message-centric**
- **Location**: `internal/domain/session.go:5-27` and `internal/server/api.go:43-50`
- **Problem**: The primary domain concept is `Session` with a single `Task` payload (`runRequest.task`). There is no `Message`, `Conversation`, or `Turn` concept. The data model treats each interaction as a standalone task rather than a conversation with multiple turns.
- **Target**: Introduce conversation/message domain contracts that coexist with existing session types, supporting multi-turn chat while preserving single-task session semantics for resume/inspect.

### 4. Target Dependency Direction for v6

```
UI (internal/server, embedded web)
  ↓ depends on
Service (new internal/service or app package)
  ↓ depends on
Repo (internal/store) ← Types (internal/domain)
  ↓                    ↑
  └────────────────────┘

Runtime (internal/runtime, tools, verify, plugin, memory, llm, orchestration)
  ↑ used by Service
```

**Rules**:
1. UI/transport must NOT directly invoke repo or runtime operations — all orchestration goes through Service
2. Service must be callable without HTTP (testable in isolation)
3. Runtime components remain siblings consumed by Service, not layered under Repo
4. Config flows from Config layer into both Service and Runtime, not from UI directly

### 5. V5 Constraints Preserved in v6

- Same-origin embedded Web UI served by the same chi router on the same port
- Embedded static assets via Go `embed.FS` — no separate build toolchain
- Manual JWT token entry UX with localStorage persistence
- Live SSE consumption via browser `EventSource` API
- Session history from persisted inspect data (no SSE replay)
- `GET /api/v1/sessions` endpoint for session listing
- `--web-ui-enabled` flag for optional enablement
- Existing run/resume/inspect/stream API contract backward compatibility

### 6. V6 Constraints That Change

- **Primary UX**: Chat-first conversational interface replaces task-form-first navigation as the default user experience
- **Domain model**: New message/conversation/turn contracts coexist with session types
- **Service layer**: New application/service package owns business orchestration; HTTP handlers become thin adapters
- **API surface**: Chat-oriented endpoints (conversation start, follow-up turn, transcript retrieval) added alongside existing session endpoints
- **Route navigation**: Browser default route is the chat workspace, not a dashboard/task form

### 7. Non-Goals for v6

- Independent SPA build system (Vite, Webpack, React, etc.)
- Separate frontend origin or second HTTP server
- WebSocket transport (SSE remains the streaming primitive)
- Accounts, password login, OAuth, refresh-token system
- Frontend framework migration
- Plugin marketplace or plugin auto-discovery
- v4/v5 API contract breakage on existing routes

## Consequences

### Positive
- Architecture becomes auditable and testable with clear service boundaries
- Chat-first UX aligns with general-purpose agent interaction expectations
- Service layer enables both HTTP and CLI entrypoints to share business logic
- Six-layer alignment makes architecture intent explicit and easier to maintain

### Negative
- Adds one more package boundary (Service layer) that must be maintained
- Domain model expansion (conversation/message types) increases type surface
- Transition period where both task-form and chat-flow code coexist in the repo

## References

- OpenAI Harness Engineering: https://openai.com/index/harness-engineering/
- Martin Fowler on Harness Engineering: https://martinfowler.com/articles/harness-engineering.html
- ADR-010 (v5 Web UI): docs/ADR-010-v5-web-ui.md
- Current handler implementation: internal/server/api.go:176-344
- Current domain types: internal/domain/session.go:5-27
