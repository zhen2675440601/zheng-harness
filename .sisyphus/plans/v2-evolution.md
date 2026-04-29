# v2 Evolution: Deepen + Extend + Orchestrate

## TL;DR
> **Summary**: v2 evolves zheng-harness from a CLI-only single-agent engine into a streaming-capable, plugin-extensible, multi-agent orchestrating platform — in four progressive waves that each build on the last.
> **Deliverables**: Provider stabilization + streaming output, 3 new tools (web_fetch, ask_user, code_search), dual-mode plugin system (Go plugin + external process), orchestrator-worker multi-agent runtime
> **Effort**: XL
> **Parallel**: YES - 5 waves with intra-wave parallelism
> **Critical Path**: Wave 1 (Foundation) → Wave 2 (Single-Agent Depth) → Wave 3 (Plugin System) → Wave 4 (Multi-Agent) → Wave 5 (Integration + Validation)

## Context
### Original Request
v1收尾已经完成，后续应该进行什么任务呢，牢记项目的需求和目标去规划任务。

### Interview Summary
- **3 directions confirmed**: deepen single-agent capability, multi-agent orchestration, extensibility/plugin system
- **Biggest pain point**: tool capability insufficient → prioritize tool expansion
- **Streaming**: needed, CLI real-time LLM response
- **Tool priority**: web access > human interaction > code analysis
- **Plugin mechanism**: dual-mode (Go plugin .so on Linux/macOS + external process cross-platform fallback)
- **Multi-agent**: orchestrator-worker pattern first (not peer-to-peer)
- **Provider strategy**: stabilize existing 3 (dashscope/openai/anthropic) with real-network verification, no new providers
- **Testing**: strict TDD consistent with v1
- **Scope**: full 3-direction plan in one document, executed in waves
- **Excluded**: new providers, vector DB/embedding, Web UI, messaging gateways, git tools, peer-to-peer multi-agent

### Metis Review (gaps addressed)
- **Streaming contract underspecified**: Plan now defines explicit StreamingEvent types (TokenDelta, ToolStart, ToolEnd, StepComplete, SessionComplete, Error) with ordering guarantees and cancellation behavior
- **Provider.Stream() fallback needed**: Non-streaming providers wrap Generate() into single-event stream
- **Plugin scope creep risk**: v2 limited to tool plugins only; no provider/agent/verifier plugins
- **External process as portable baseline**: External-process protocol is the canonical portable path; native .so is optimization
- **Multi-agent termination rules**: Max workers configurable (default 4), no recursive workers, fail-fast with partial results preserved
- **Test matrix explosion**: Minimal required matrix per wave; extended coverage as nightly/optional
- **Streaming persistence**: Only final Step/Session state persisted (not intermediate token deltas); resume reconstructs from persisted steps

## Work Objectives
### Core Objective
Evolve zheng-harness from v1's CLI-only single-agent into a platform that supports: (1) real-time streaming interaction, (2) extensible tool ecosystem via dual-mode plugins, (3) multi-agent orchestration with bounded concurrency.

### Deliverables
1. **Provider stabilization**: OpenAI/Anthropic real-network smoke tests with recorded fixtures
2. **Streaming infrastructure**: Provider.Stream() interface, runtime event channel, CLI incremental output
3. **3 new tools**: web_fetch, ask_user, code_search
4. **Plugin system**: dual-mode (Go plugin .so + external process JSON-RPC over stdio)
5. **Multi-agent runtime**: orchestrator-worker with errgroup, typed channels, DAG decomposition

### Definition of Done (verifiable conditions with commands)
```bash
go build ./...                              # All code compiles
go test ./...                               # Full test suite passes
go test -race ./...                         # No race conditions
go test ./internal/llm/... -run TestStream  # Streaming interface tests pass
go test ./internal/tools/... -run TestWebFetch|TestAskUser|TestCodeSearch  # New tool tests pass
go test ./internal/plugin/...               # Plugin system tests pass (external process on all platforms)
go test ./internal/runtime/... -run TestMultiAgent  # Multi-agent tests pass
go test ./cmd/agent/... -run TestStreaming  # CLI streaming integration passes
```

### Must Have
- Provider.Stream() with callback-based chunk delivery
- StreamingEvent typed events with ordering guarantee
- Fallback for non-streaming providers (wrap Generate into single TokenDelta)
- web_fetch tool with configurable timeout and domain allowlist
- ask_user tool that prompts CLI user and returns response to agent
- code_search tool with regex + AST-aware search
- External-process plugin protocol: JSON-RPC 2.0 over stdio
- Go plugin loading on Linux/macOS with build tags
- PluginManager with version contract validation
- Orchestrator with errgroup-based worker dispatch
- Worker lifecycle: spawn → execute → report → terminate
- Bounded concurrency (configurable max workers)
- Cancellation propagation from orchestrator to all workers
- Partial result preservation on worker failure

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- NO new LLM providers (Ollama, vLLM, etc.)
- NO vector database / embedding retrieval / knowledge graph
- NO Web UI or HTTP API server
- NO Slack / Telegram / Discord gateways
- NO git operation tools
- NO peer-to-peer multi-agent
- NO provider/agent/verifier plugins (tool plugins only in v2)
- NO plugin marketplace or registry service
- NO recursive worker spawning (workers cannot spawn sub-workers)
- NO streaming persistence of intermediate token deltas (only final state)
- NO generic "plugin ecosystem" — fixed protocol, fixed capability set
- No AI slop: no over-abstracted factories, no unnecessary interfaces, no speculative generality

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: TDD (RED-GREEN-REFACTOR) consistent with v1
- QA policy: Every task has agent-executed scenarios (happy path + failure path)
- Evidence: .sisyphus/evidence/task-{N}-{slug}.{ext}
- Minimal test matrix: per-wave required tests only; extended combinations as nightly/optional

## Execution Strategy
### Parallel Execution Waves

**Wave 1: Foundation (6 tasks)** — Provider stabilization + streaming interface contracts
- Provider real-network smoke tests + fixture recording
- StreamingEvent type definitions
- Provider.Stream() interface + fallback wrapper
- Runtime event channel infrastructure
- ADR-006: Streaming Architecture Decision Record
- ADR-007: Plugin System Architecture Decision Record

**Wave 2: Single-Agent Depth (7 tasks)** — New tools + streaming CLI integration
- web_fetch tool adapter
- ask_user tool adapter (CLI interactive prompt)
- code_search tool adapter
- Streaming CLI output (token delta display + tool event display)
- Streaming runtime integration (Engine.RunStream)
- Streaming resume/inspect compatibility
- Update USAGE.md + README.md for streaming + new tools

**Wave 3: Plugin System (6 tasks)** — Dual-mode tool loading
- Plugin Tool interface + contract version
- External-process plugin loader (JSON-RPC 2.0 over stdio)
- Go plugin native loader (build-tag gated, Linux/macOS only)
- PluginManager (discovery, loading, version validation, lifecycle)
- Plugin safety policy extension (allowed plugin paths, capability declaration)
- Plugin CLI flags (--plugin-dir, --plugin, --allow-plugin)

**Wave 4: Multi-Agent Orchestration (7 tasks)** — Orchestrator-worker runtime
- Subtask + TaskDecomposition domain types
- Orchestrator struct with errgroup-based dispatch
- Worker agent (scoped plan-execute-verify loop)
- Channel-based message passing (TaskRequest → TaskResult)
- DAG dependency-aware scheduling
- Result aggregation strategies (AllSucceed, BestEffort)
- Multi-agent CLI commands (run --decompose, --max-workers)

**Wave 5: Integration + Validation (4 tasks)** — End-to-end validation + documentation
- Full integration test: streaming + tools + plugins + multi-agent
- Validation matrix update for v2
- v2 release preparation
- Final verification wave (F1-F4)

### Dependency Matrix (full, all tasks)

| Task | Blocks | Blocked By |
|------|--------|------------|
| T1 Provider smoke tests | T5 | - |
| T2 StreamingEvent types | T3, T4, T10 | - |
| T3 Provider.Stream() + fallback | T10, T11 | T2 |
| T4 Runtime event channel | T10, T11 | T2 |
| T5 ADR-006 Streaming | - | - |
| T6 ADR-007 Plugin | T14 | - |
| T7 web_fetch tool | T12 | - |
| T8 ask_user tool | T12 | - |
| T9 code_search tool | T12 | - |
| T10 Streaming CLI output | T12 | T3, T4 |
| T11 Streaming runtime (RunStream) | T12, T13 | T3, T4 |
| T12 Streaming resume/inspect compat | - | T10, T11, T7, T8, T9 |
| T13 Update docs | - | T10, T11 |
| T14 Plugin Tool interface + contract | T15, T16, T17 | T6 |
| T15 External-process plugin loader | T17 | T14 |
| T16 Go plugin native loader | T17 | T14 |
| T17 PluginManager | T18, T19 | T14, T15, T16 |
| T18 Plugin safety extension | - | T17 |
| T19 Plugin CLI flags | - | T17 |
| T20 Subtask + TaskDecomposition types | T21, T22 | - |
| T21 Orchestrator (errgroup dispatch) | T23, T25 | T20 |
| T22 Worker agent (scoped PEV loop) | T23, T24 | T20 |
| T23 Channel message passing | T24, T25 | T21, T22 |
| T24 DAG scheduling | - | T22, T23 |
| T25 Result aggregation | - | T21, T23 |
| T26 Multi-agent CLI commands | - | T25 |
| T27 Full integration test | - | T12, T19, T26 |
| T28 Validation matrix update | - | T27 |
| T29 v2 release prep | - | T28 |
| F1 Plan compliance audit | - | T27 |
| F2 Code quality review | - | T27 |
| F3 Real manual QA | - | T27 |
| F4 Scope fidelity check | - | T27 |

### Agent Dispatch Summary
| Wave | Tasks | Categories |
|------|-------|------------|
| Wave 1 | 6 | deep, unspecified-high, writing, writing |
| Wave 2 | 7 | deep, unspecified-high, unspecified-high, deep, deep, deep, writing |
| Wave 3 | 6 | deep, deep, deep, deep, unspecified-high, unspecified-high |
| Wave 4 | 7 | deep, deep, deep, deep, deep, deep, unspecified-high |
| Wave 5 | 4 | deep, writing, writing, oracle+unspecified-high×3 |

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] T1. Provider Real-Network Smoke Tests + Fixture Recording

  **What to do**:
  1. Create `internal/llm/smoke_test.go` with build tag `//go:build smoke` to avoid running in normal `go test`
  2. For each provider (dashscope, openai, anthropic), write a smoke test that makes a real HTTP call with a simple prompt (e.g., "Respond with exactly: OK")
  3. Validate: response contains non-empty Output, Model matches config, no error
  4. Record successful responses as fixtures in `testdata/llm/{provider}_smoke_response.json`
  5. Add `Makefile` target `make smoke-test` that runs `go test -tags=smoke ./internal/llm/...`
  6. Document in ADR: smoke tests are opt-in (require valid API keys), not part of CI default

  **Must NOT do**: Do not modify existing Provider interface. Do not add streaming yet. Do not add new providers.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Requires understanding existing provider implementations + test infrastructure design
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not a UI task

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T5 | Blocked By: none

  **References**:
  - Pattern: `internal/llm/dashscope.go` - Real HTTP provider implementation
  - Pattern: `internal/llm/openai.go` - Real HTTP provider implementation
  - Pattern: `internal/llm/anthropic.go` - Real HTTP provider implementation
  - Test: `internal/llm/dashscope_test.go` - Existing test patterns
  - Config: `internal/config/config.go` - Provider configuration loading
  - Fixture: `testdata/runtime/` - Existing fixture pattern

  **Acceptance Criteria**:
  - [ ] `go test -tags=smoke ./internal/llm/... -run TestDashScopeSmoke` passes (with valid API key)
  - [ ] `go test -tags=smoke ./internal/llm/... -run TestOpenAISmoke` passes (with valid API key)
  - [ ] `go test -tags=smoke ./internal/llm/... -run TestAnthropicSmoke` passes (with valid API key)
  - [ ] `make smoke-test` target exists in Makefile
  - [ ] Fixture files exist: `testdata/llm/dashscope_smoke_response.json`, `testdata/llm/openai_smoke_response.json`, `testdata/llm/anthropic_smoke_response.json`
  - [ ] `go test ./...` still passes (smoke tests excluded by build tag)

  **QA Scenarios**:
  ```
  Scenario: Provider smoke test with valid credentials
    Tool: Bash
    Steps: Set valid API keys in environment; run `go test -tags=smoke ./internal/llm/... -run TestDashScopeSmoke -v`
    Expected: Test passes; response Output non-empty; fixture file created
    Evidence: .sisyphus/evidence/task-1-smoke-pass.txt

  Scenario: Provider smoke test with invalid credentials
    Tool: Bash
    Steps: Set invalid API key; run `go test -tags=smoke ./internal/llm/... -run TestOpenAISmoke -v`
    Expected: Test fails with authentication error (not panic)
    Evidence: .sisyphus/evidence/task-1-smoke-fail.txt

  Scenario: Normal test suite unaffected
    Tool: Bash
    Steps: Run `go test ./internal/llm/...`
    Expected: All existing tests pass; smoke tests NOT included
    Evidence: .sisyphus/evidence/task-1-normal-tests.txt
  ```

  **Commit**: YES | Message: `test(llm): add real-network smoke tests with fixture recording` | Files: `internal/llm/smoke_test.go, testdata/llm/*.json, Makefile`

- [x] T2. StreamingEvent Type Definitions

  **What to do**:
  1. Create `internal/domain/events.go` defining StreamingEvent types:
     ```go
     type StreamingEventType string
     const (
         EventTokenDelta    StreamingEventType = "token_delta"    // Incremental text chunk
         EventToolStart     StreamingEventType = "tool_start"    // Tool call initiated
         EventToolEnd       StreamingEventType = "tool_end"      // Tool call completed
         EventStepComplete  StreamingEventType = "step_complete" // Step finished (with Step summary)
         EventError         StreamingEventType = "error"         // Error occurred
         EventSessionComplete StreamingEventType = "session_complete" // Session finished
     )
     type StreamingEvent struct {
         Type      StreamingEventType `json:"type"`
         StepIndex int                `json:"step_index,omitempty"`
         Payload   json.RawMessage    `json:"payload"`
         Timestamp time.Time          `json:"timestamp"`
     }
     type TokenDeltaPayload struct { Content string `json:"content"` }
     type ToolStartPayload struct { ToolName string `json:"tool_name"; Input string `json:"input"` }
     type ToolEndPayload struct { ToolName string `json:"tool_name"; Output string `json:"output"; Error string `json:"error,omitempty"` }
     type StepCompletePayload struct { StepSummary string `json:"step_summary"` }
     type ErrorPayload struct { Message string `json:"message"` }
     type SessionCompletePayload struct { SessionID string `json:"session_id"; Status string `json:"status"` }
     ```
  2. Add `EmitEvent(event StreamingEvent) error` method to domain ports or as separate Emitter interface
  3. Write TDD tests for event type marshaling/unmarshaling, ordering (timestamp monotonicity), and payload validation
  4. Define ordering guarantee: events within a step are ordered; inter-step events are ordered by StepIndex

  **Must NOT do**: Do not implement streaming in providers yet. Do not modify runtime loop yet. Do not persist intermediate events.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Core type design with ordering guarantees and interface contracts
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not a UI task

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T3, T4, T10 | Blocked By: none

  **References**:
  - Pattern: `internal/domain/ports.go:5-10` - Model interface pattern for new Emitter interface
  - Pattern: `internal/domain/tool.go` - ToolCall/ToolResult/ToolInfo type pattern
  - Pattern: `internal/domain/step.go` - Step type for StepIndex reference
  - Pattern: `internal/domain/session.go` - Session type for SessionID reference

  **Acceptance Criteria**:
  - [ ] `internal/domain/events.go` exists with StreamingEvent and all payload types
  - [ ] `go test ./internal/domain/... -run TestStreamingEvent` passes
  - [ ] Event marshaling round-trip test passes (marshal → unmarshal → equal)
  - [ ] Timestamp ordering validation test passes
  - [ ] No changes to existing domain types (backward compatible)

  **QA Scenarios**:
  ```
  Scenario: Event marshaling round-trip
    Tool: Bash
    Steps: Run `go test ./internal/domain/... -run TestStreamingEventMarshalRoundTrip -v`
    Expected: All event types marshal and unmarshal correctly; payload preserved
    Evidence: .sisyphus/evidence/task-2-event-marshal.txt

  Scenario: Ordering guarantee validation
    Tool: Bash
    Steps: Run `go test ./internal/domain/... -run TestStreamingEventOrdering -v`
    Expected: Events with ascending StepIndex pass; out-of-order events are detected
    Evidence: .sisyphus/evidence/task-2-event-order.txt

  Scenario: Backward compatibility
    Tool: Bash
    Steps: Run `go test ./...` after changes
    Expected: All existing tests still pass; no regressions
    Evidence: .sisyphus/evidence/task-2-backward.txt
  ```

  **Commit**: YES | Message: `feat(domain): add StreamingEvent types with ordering guarantees` | Files: `internal/domain/events.go, internal/domain/events_test.go`

- [x] T3. Provider.Stream() Interface + Fallback Wrapper

  **What to do**:
  1. Add `Stream(ctx context.Context, request Request, emit func(StreamingEvent) error) error` to Provider interface in `internal/llm/provider.go`
  2. Create `internal/llm/streaming.go` implementing `StreamFallback` - wraps `Generate()` into a single `TokenDelta` + `SessionComplete` event sequence for providers that don't support native streaming
  3. Update each provider (dashscope, openai, anthropic) to implement `Stream()`:
     - DashScope: implement SSE parsing with `stream: true` parameter
     - OpenAI: implement SSE parsing with `stream: true` parameter
     - Anthropic: implement SSE parsing with `stream: true` parameter
  4. Write TDD tests:
     - TestStreamFallbackEmitsTokenDeltaAndComplete: fallback wraps Generate into events
     - TestStreamCancelStopsEmission: context cancellation stops callback chain
     - TestStreamOrdering: events emitted in correct order
  5. SSE parsing helper in `internal/llm/sse.go` - generic SSE line parser for all providers

  **Must NOT do**: Do not modify runtime loop. Do not modify CLI. Do not change Generate() behavior. Do not persist streaming events.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Core interface extension + SSE implementation across 3 providers
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not a UI task

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T10, T11 | Blocked By: T2

  **References**:
  - Pattern: `internal/llm/provider.go:24-28` - Current Provider interface (add Stream method)
  - Pattern: `internal/llm/openai.go:81-177` - OpenAI Generate implementation (add SSE version)
  - Pattern: `internal/llm/anthropic.go:89-173` - Anthropic Generate implementation
  - Pattern: `internal/llm/dashscope.go` - DashScope implementation
  - Type: `internal/domain/events.go` - StreamingEvent types from T2

  **Acceptance Criteria**:
  - [ ] `Provider` interface in `internal/llm/provider.go` has `Stream()` method
  - [ ] `internal/llm/streaming.go` exists with StreamFallback
  - [ ] `internal/llm/sse.go` exists with generic SSE parser
  - [ ] `go test ./internal/llm/... -run TestStreamFallback` passes
  - [ ] `go test ./internal/llm/... -run TestStreamCancel` passes
  - [ ] `go test ./internal/llm/... -run TestSSEParser` passes
  - [ ] `go test ./...` still passes (existing Generate() unchanged)

  **QA Scenarios**:
  ```
  Scenario: StreamFallback wraps Generate into events
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestStreamFallbackEmitsTokenDeltaAndComplete -v`
    Expected: Exactly 2 events emitted: TokenDelta with full content, then SessionComplete
    Evidence: .sisyphus/evidence/task-3-fallback.txt

  Scenario: Stream cancellation stops emission
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestStreamCancelStopsEmission -v`
    Expected: After context cancel, no more events emitted; Stream returns context.Canceled
    Evidence: .sisyphus/evidence/task-3-cancel.txt

  Scenario: SSE parser handles malformed input
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestSSEParserMalformed -v`
    Expected: Malformed lines skipped; parser recovers; no panic
    Evidence: .sisyphus/evidence/task-3-sse-malformed.txt
  ```

  **Commit**: YES | Message: `feat(llm): add Provider.Stream() interface with SSE and fallback` | Files: `internal/llm/provider.go, internal/llm/streaming.go, internal/llm/sse.go, internal/llm/openai.go, internal/llm/anthropic.go, internal/llm/dashscope.go, internal/llm/streaming_test.go, internal/llm/sse_test.go`

- [x] T4. Runtime Event Channel Infrastructure

  **What to do**:
  1. Create `internal/runtime/emitter.go` implementing EventChannel:
     ```go
     type EventChannel struct {
         ch     chan domain.StreamingEvent
         closed atomic.Bool
     }
     func NewEventChannel(buffer int) *EventChannel
     func (ec *EventChannel) Emit(event domain.StreamingEvent) error  // non-blocking send, returns error if closed
     func (ec *EventChannel) Events() <-chan domain.StreamingEvent
     func (ec *EventChannel) Close()
     ```
  2. Integrate EventChannel into Engine struct (optional field, nil = no streaming)
  3. Modify runtime.go to emit events at key points:
     - After Model.CreatePlan: emit StepComplete with plan summary
     - After Model.NextAction: emit ToolStart if action is tool_call
     - After ToolExecutor.Execute: emit ToolEnd with result
     - After Model.Observe: emit StepComplete with observation summary
     - On session completion: emit SessionComplete
     - On error: emit Error
  4. Keep Engine.Run() signature unchanged (backward compatible). Add Engine.RunStream() that creates EventChannel and returns it.
  5. Write TDD tests for EventChannel (buffer, close, emit-after-close, concurrent emit)

  **Must NOT do**: Do not change Engine.Run() behavior. Do not modify CLI. Do not modify Model interface.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Core runtime infrastructure with concurrency patterns
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not a UI task

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T10, T11 | Blocked By: T2

  **References**:
  - Pattern: `internal/runtime/runtime.go:45-146` - Engine.Run() main loop
  - Pattern: `internal/runtime/runtime.go:166-194` - executeIteration() where events should be emitted
  - Type: `internal/domain/events.go` - StreamingEvent types from T2
  - Pattern: `internal/domain/ports.go` - Model interface (unchanged)

  **Acceptance Criteria**:
  - [ ] `internal/runtime/emitter.go` exists with EventChannel implementation
  - [ ] `Engine.RunStream(ctx, task) (*EventChannel, error)` exists
  - [ ] `Engine.Run()` still works identically (no events emitted, nil EventChannel)
  - [ ] `go test ./internal/runtime/... -run TestEventChannel` passes
  - [ ] `go test ./internal/runtime/... -run TestRunStreamEmitsEvents` passes
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: EventChannel concurrent emit
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestEventChannelConcurrent -race -v`
    Expected: No race conditions; all events received in order; no events lost
    Evidence: .sisyphus/evidence/task-4-channel-concurrent.txt

  Scenario: Emit after close returns error
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestEventChannelEmitAfterClose -v`
    Expected: Emit returns error; no panic; no send on closed channel
    Evidence: .sisyphus/evidence/task-4-channel-close.txt

  Scenario: RunStream emits complete lifecycle
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestRunStreamEmitsLifecycle -v`
    Expected: Events: StepComplete (plan), ToolStart, ToolEnd, StepComplete (observe), SessionComplete
    Evidence: .sisyphus/evidence/task-4-lifecycle.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add EventChannel and RunStream with event emission` | Files: `internal/runtime/emitter.go, internal/runtime/emitter_test.go, internal/runtime/runtime.go, internal/runtime/runtime_test.go`

- [x] T5. ADR-006: Streaming Architecture Decision Record

  **What to do**:
  1. Create `docs/ADR-006-streaming-architecture.md` documenting:
     - Decision: callback-based Stream() with EventChannel, not separate StreamingModel interface
     - Rationale: preserves Provider interface cohesion, allows fallback for non-streaming providers
     - Event types and ordering guarantees
     - Fallback behavior: Generate() → single TokenDelta + SessionComplete
     - Persistence decision: only final Step/Session state persisted, not intermediate deltas
     - Resume behavior: reconstructs from persisted steps, no replay of deltas
  2. Follow existing ADR format from `docs/ADR-001-single-process-agent.md`

  **Must NOT do**: Do not implement any code. Documentation only.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: Architecture documentation
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not a UI task

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: none | Blocked By: T1 (for smoke test insights)

  **References**:
  - Pattern: `docs/ADR-001-single-process-agent.md` - ADR format
  - Pattern: `docs/ADR-002-sqlite-memory.md` - ADR format
  - Source: T2, T3, T4 design decisions

  **Acceptance Criteria**:
  - [ ] `docs/ADR-006-streaming-architecture.md` exists
  - [ ] Document covers: decision, rationale, event types, ordering, fallback, persistence, resume
  - [ ] Follows existing ADR format

  **QA Scenarios**:
  ```
  Scenario: ADR completeness
    Tool: Bash
    Steps: Check file exists and contains sections: Decision, Rationale, Consequences, Event Types, Fallback, Persistence
    Expected: All sections present and non-empty
    Evidence: .sisyphus/evidence/task-5-adr006.txt
  ```

  **Commit**: YES | Message: `docs: add ADR-006 streaming architecture` | Files: `docs/ADR-006-streaming-architecture.md`

- [x] T6. ADR-007: Plugin System Architecture Decision Record

  **What to do**:
  1. Create `docs/ADR-007-plugin-system.md` documenting:
     - Decision: dual-mode plugin (Go plugin .so on Linux/macOS + external process JSON-RPC over stdio)
     - Rationale: external process as portable baseline; native plugin as performance optimization
     - v2 scope: tool plugins only (no provider/agent/verifier plugins)
     - Plugin contract: version string, Tool interface, JSON-RPC 2.0 protocol
     - Security model: plugins are trusted local extensions (no sandboxing in v2)
     - Windows strategy: external process only, native loading disabled via build tags
     - Protocol: JSON-RPC 2.0 over stdio (initialize → tool_call → shutdown)
  2. Follow existing ADR format

  **Must NOT do**: Do not implement any code. Documentation only.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: Architecture documentation
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not a UI task

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T14 | Blocked By: none

  **References**:
  - Pattern: `docs/ADR-001-single-process-agent.md` - ADR format
  - Pattern: `docs/ADR-003-no-plugin-system.md` - v1 decision to NOT have plugins (now reversed)
  - Source: Librarian research on Go plugin, HashiCorp go-plugin, MCP

  **Acceptance Criteria**:
  - [ ] `docs/ADR-007-plugin-system.md` exists
  - [ ] Document covers: decision, rationale, dual-mode, v2 scope, contract, security, Windows strategy, protocol
  - [ ] Follows existing ADR format
  - [ ] References ADR-003 (v1 exclusion) and explains reversal

  **QA Scenarios**:
  ```
  Scenario: ADR completeness
    Tool: Bash
    Steps: Check file exists and contains sections: Decision, Rationale, Dual-Mode, Scope, Contract, Security, Windows, Protocol
    Expected: All sections present; ADR-003 referenced
    Evidence: .sisyphus/evidence/task-6-adr007.txt
  ```

  **Commit**: YES | Message: `docs: add ADR-007 plugin system architecture` | Files: `docs/ADR-007-plugin-system.md`

- [x] T7. web_fetch Tool Adapter

  **What to do**:
  1. Create `internal/tools/adapters/web.go` implementing WebAdapter with methods:
     - `Fetch(ctx, call)` - HTTP GET with configurable timeout, returns page content (truncated to max size)
  2. Register `web_fetch` tool in executor.go builtinDefinitions with:
     - SafetyLevel: Medium (external network access)
     - DefaultTimeout: 15s
     - Schema: `{"url": "string (required)", "max_length": "int (optional, default 10000)"}`
  3. Add domain allowlist to SafetyPolicy: `AllowedDomains []string` - if empty, all domains allowed; if set, only allowed domains
  4. Input format: JSON with `url` and optional `max_length`
  5. Validate URL: must be http/https, reject file:// and other schemes
  6. Truncate response body to max_length characters
  7. TDD tests:
     - TestWebFetchSuccess: mock HTTP server, fetch returns content
     - TestWebFetchTimeout: slow server triggers context timeout
     - TestWebFetchBlockedDomain: domain not in allowlist → error
     - TestWebFetchInvalidURL: non-http URL → error
     - TestWebFetchTruncation: large response truncated to max_length

  **Must NOT do**: Do not add JavaScript rendering. Do not add authentication. Do not modify existing tools.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Single tool implementation with clear contract, high effort for proper testing
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T12 | Blocked By: none

  **References**:
  - Pattern: `internal/tools/adapters/shell.go` - Shell adapter implementation pattern
  - Pattern: `internal/tools/adapters/search.go` - Search adapter with input parsing
  - Pattern: `internal/tools/executor.go` - builtinDefinitions registration
  - Pattern: `internal/tools/safety.go` - SafetyPolicy validation
  - Interface: `internal/domain/tool.go` - ToolCall/ToolResult types

  **Acceptance Criteria**:
  - [ ] `internal/tools/adapters/web.go` exists with WebAdapter
  - [ ] `web_fetch` registered in builtinDefinitions
  - [ ] `go test ./internal/tools/... -run TestWebFetch` passes (all 5 sub-tests)
  - [ ] `AllowedDomains` field added to SafetyPolicy
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: Fetch valid URL
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestWebFetchSuccess -v`
    Expected: Returns page content; ToolResult.Output non-empty; Duration recorded
    Evidence: .sisyphus/evidence/task-7-web-success.txt

  Scenario: Fetch blocked domain
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestWebFetchBlockedDomain -v`
    Expected: ToolResult.Error contains "domain not allowed"; no HTTP request made
    Evidence: .sisyphus/evidence/task-7-web-blocked.txt

  Scenario: Fetch timeout
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestWebFetchTimeout -v`
    Expected: ToolResult.Error contains "context deadline exceeded"; no hang
    Evidence: .sisyphus/evidence/task-7-web-timeout.txt
  ```

  **Commit**: YES | Message: `feat(tools): add web_fetch tool with domain allowlist` | Files: `internal/tools/adapters/web.go, internal/tools/adapters/web_test.go, internal/tools/executor.go, internal/tools/safety.go`

- [x] T8. ask_user Tool Adapter (CLI Interactive Prompt)

  **What to do**:
  1. Create `internal/tools/adapters/interactive.go` implementing InteractiveAdapter with:
     - `AskUser(ctx, call)` - Prompts CLI user for input, returns response as ToolResult
  2. Register `ask_user` tool in executor.go builtinDefinitions with:
     - SafetyLevel: Low (no side effects, only reads stdin)
     - DefaultTimeout: 300s (5 minutes for user thinking time)
     - Schema: `{"question": "string (required)", "options": "[]string (optional)"}`
  3. Input format: JSON with `question` (required) and optional `options` array
  4. Behavior:
     - Print question to stdout
     - If options provided, print numbered choices and validate input
     - Read from stdin until newline
     - If context cancelled (timeout), return error "user did not respond in time"
  5. Inject stdin/stdout via adapter constructor (not os.Stdin/os.Stdout directly) for testability
  6. TDD tests:
     - TestAskUserFreeText: mock stdin provides text → ToolResult contains text
     - TestAskUserWithOptions: mock stdin provides valid choice → ToolResult contains selected option
     - TestAskUserInvalidOption: mock stdin provides invalid choice → re-prompt (max 3 attempts, then error)
     - TestAskUserTimeout: context cancelled → error

  **Must NOT do**: Do not implement GUI. Do not add persistent user profiles. Do not modify existing tools.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Single tool with I/O testing considerations
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T12 | Blocked By: none

  **References**:
  - Pattern: `internal/tools/adapters/shell.go` - Adapter with external I/O pattern
  - Pattern: `internal/tools/executor.go` - builtinDefinitions registration
  - Pattern: `internal/domain/tool.go` - ToolCall/ToolResult types

  **Acceptance Criteria**:
  - [ ] `internal/tools/adapters/interactive.go` exists with InteractiveAdapter
  - [ ] `ask_user` registered in builtinDefinitions
  - [ ] `go test ./internal/tools/... -run TestAskUser` passes (all 4 sub-tests)
  - [ ] Constructor accepts io.Reader/io.Writer for testability
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: User provides free text response
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestAskUserFreeText -v`
    Expected: ToolResult.Output contains the user's text response
    Evidence: .sisyphus/evidence/task-8-ask-freetext.txt

  Scenario: User selects from options
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestAskUserWithOptions -v`
    Expected: ToolResult.Output contains the selected option string
    Evidence: .sisyphus/evidence/task-8-ask-options.txt

  Scenario: User timeout
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestAskUserTimeout -v`
    Expected: ToolResult.Error contains "did not respond"; no indefinite hang
    Evidence: .sisyphus/evidence/task-8-ask-timeout.txt
  ```

  **Commit**: YES | Message: `feat(tools): add ask_user interactive prompt tool` | Files: `internal/tools/adapters/interactive.go, internal/tools/adapters/interactive_test.go, internal/tools/executor.go`

- [x] T9. code_search Tool Adapter

  **What to do**:
  1. Create `internal/tools/adapters/codesearch.go` implementing CodeSearchAdapter with:
     - `Search(ctx, call)` - Regex search with AST-aware file type filtering
  2. Register `code_search` tool in executor.go builtinDefinitions with:
     - SafetyLevel: Low (read-only)
     - DefaultTimeout: 10s
     - Schema: `{"pattern": "string (required)", "language": "string (optional)", "output_mode": "string (optional: content|files_with_matches|count)", "max_results": "int (optional, default 50)"}`
  3. Differentiation from existing `grep_search`:
     - Language-aware: `language` field maps to file extensions (.go, .py, .js, etc.)
     - Excludes non-code files (images, binaries, minified JS, vendor dirs)
     - Returns line numbers with context (2 lines before/after in content mode)
  4. Default exclude patterns: `vendor/`, `node_modules/`, `*.min.js`, `*.min.css`, binary files
  5. TDD tests:
     - TestCodeSearchByLanguage: search Go files only → finds .go matches, skips .py
     - TestCodeSearchContentMode: returns lines with context
     - TestCodeSearchExcludesVendor: vendor/ dir excluded
     - TestCodeSearchMaxResults: results capped at max_results

  **Must NOT do**: Do not add AST parsing (tree-sitter etc.). Do not add semantic search. Do not modify existing grep_search.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Single tool with filtering logic
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T12 | Blocked By: none

  **References**:
  - Pattern: `internal/tools/adapters/search.go` - Existing grep_search adapter (extend, don't modify)
  - Pattern: `internal/tools/executor.go` - builtinDefinitions registration
  - Pattern: `internal/domain/tool.go` - ToolCall/ToolResult types

  **Acceptance Criteria**:
  - [ ] `internal/tools/adapters/codesearch.go` exists with CodeSearchAdapter
  - [ ] `code_search` registered in builtinDefinitions
  - [ ] `go test ./internal/tools/... -run TestCodeSearch` passes (all 4 sub-tests)
  - [ ] Language-to-extension mapping covers: go, python, javascript, typescript, java, rust
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: Search by language filter
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestCodeSearchByLanguage -v`
    Expected: Only files matching the language extension returned; others excluded
    Evidence: .sisyphus/evidence/task-9-codesearch-lang.txt

  Scenario: Vendor directory excluded
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestCodeSearchExcludesVendor -v`
    Expected: No results from vendor/ or node_modules/ directories
    Evidence: .sisyphus/evidence/task-9-codesearch-exclude.txt

  Scenario: Max results capped
    Tool: Bash
    Steps: Run `go test ./internal/tools/... -run TestCodeSearchMaxResults -v`
    Expected: Results capped at max_results value; no more returned
    Evidence: .sisyphus/evidence/task-9-codesearch-max.txt
  ```

  **Commit**: YES | Message: `feat(tools): add code_search tool with language-aware filtering` | Files: `internal/tools/adapters/codesearch.go, internal/tools/adapters/codesearch_test.go, internal/tools/executor.go`

- [x] T10. Streaming CLI Output (Token Delta + Tool Event Display)

  **What to do**:
  1. Modify `cmd/agent/cli.go`:
     - When `--stream` flag is set, use `Engine.RunStream()` instead of `Engine.Run()`
     - Consume EventChannel in a goroutine, printing to stdout:
       - TokenDelta: print content incrementally (no newline between deltas)
       - ToolStart: print `[Tool: {name}]` on new line
       - ToolEnd: print `[Tool: {name}] done ({duration})` on new line
       - StepComplete: print `--- Step {n} complete ---` on new line
       - Error: print `ERROR: {message}` on stderr
       - SessionComplete: print final summary
  2. Add `--stream` flag to `run` and `resume` commands (default: false for backward compat)
  3. When `--stream --json` both set: emit each StreamingEvent as JSON line (JSONL format)
  4. Non-streaming mode (`--stream` not set): behavior identical to v1 (no changes)
  5. TDD tests:
     - TestCLIStreamFlagEnabled: --stream triggers RunStream
     - TestCLIStreamJSONL: --stream --json emits JSONL events
     - TestCLIStreamDisabled: no --stream uses Run (v1 behavior)
     - TestCLIStreamInterrupt: Ctrl-C during streaming persists session

  **Must NOT do**: Do not change default behavior (--stream defaults to false). Do not modify Engine.Run(). Do not add progress bars or spinners.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: CLI integration touching runtime, requires careful backward compatibility
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: T12 | Blocked By: T3, T4

  **References**:
  - Pattern: `cmd/agent/cli.go` - Current CLI implementation
  - Pattern: `cmd/agent/cli.go:321` - engine.Run() call site (add RunStream branch)
  - Pattern: `cmd/agent/cli.go:548-566` - emitRunResult() (extend for streaming)
  - Type: `internal/domain/events.go` - StreamingEvent types
  - Infrastructure: `internal/runtime/emitter.go` - EventChannel from T4

  **Acceptance Criteria**:
  - [ ] `--stream` flag added to `run` and `resume` commands
  - [ ] `go test ./cmd/agent/... -run TestCLIStream` passes
  - [ ] `go test ./cmd/agent/... -run TestCLIStreamJSONL` passes
  - [ ] `go test ./cmd/agent/... -run TestCLIStreamDisabled` passes (v1 behavior preserved)
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: Streaming output displays incrementally
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestCLIStreamFlagEnabled -v`
    Expected: RunStream called; events printed to stdout; final output complete
    Evidence: .sisyphus/evidence/task-10-cli-stream.txt

  Scenario: JSONL streaming output
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestCLIStreamJSONL -v`
    Expected: Each line is valid JSON; lines are StreamingEvent objects; final line is SessionComplete
    Evidence: .sisyphus/evidence/task-10-cli-jsonl.txt

  Scenario: Non-streaming mode unchanged
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestCLIStreamDisabled -v`
    Expected: Engine.Run() called (not RunStream); output format identical to v1
    Evidence: .sisyphus/evidence/task-10-cli-nostream.txt
  ```

  **Commit**: YES | Message: `feat(cli): add --stream flag with incremental event display` | Files: `cmd/agent/cli.go, cmd/agent/cli_stream_test.go`

- [x] T11. Streaming Runtime Integration (Engine.RunStream)

  **What to do**:
  1. Implement `Engine.RunStream(ctx context.Context, task domain.Task) (*EventChannel, domain.Session, domain.Plan, []domain.Step, error)` in runtime.go
  2. RunStream creates EventChannel (buffer 64), starts the plan-execute-verify loop in a goroutine, and returns the channel immediately
  3. The loop emits events via EventChannel at each step point (as designed in T4)
  4. On completion or error, the loop closes the EventChannel
  5. RunStream blocks until the loop completes BUT the caller can consume events concurrently
  6. Add ModelAdapter streaming: when Provider supports Stream(), use it for NextAction/Observe to emit TokenDelta events; when Provider only supports Generate(), use StreamFallback
  7. TDD tests:
     - TestRunStreamReturnsEventChannel: channel not nil, events readable
     - TestRunStreamTokenDeltasFromProvider: TokenDelta events contain LLM output chunks
     - TestRunStreamFallbackNonStreamingProvider: non-streaming provider emits single TokenDelta
     - TestRunStreamCancellationClosesChannel: context cancel → channel closed

  **Must NOT do**: Do not change Engine.Run() behavior. Do not modify Model interface. Do not persist streaming events.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Core runtime integration with streaming, concurrency patterns
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: T12, T13 | Blocked By: T3, T4

  **References**:
  - Pattern: `internal/runtime/runtime.go:45-146` - Engine.Run() (model for RunStream)
  - Pattern: `internal/runtime/model_adapter.go` - ModelAdapter (add streaming support)
  - Pattern: `internal/runtime/emitter.go` - EventChannel from T4
  - Pattern: `internal/llm/streaming.go` - StreamFallback from T3

  **Acceptance Criteria**:
  - [ ] `Engine.RunStream()` method exists and returns EventChannel
  - [ ] `go test ./internal/runtime/... -run TestRunStream` passes (all 4 sub-tests)
  - [ ] ModelAdapter uses Provider.Stream() when available
  - [ ] ModelAdapter falls back to StreamFallback when Provider.Stream() not supported
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: RunStream with streaming provider
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestRunStreamTokenDeltasFromProvider -v`
    Expected: Multiple TokenDelta events received; content concatenates to full response
    Evidence: .sisyphus/evidence/task-11-runstream-live.txt

  Scenario: RunStream with non-streaming provider
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestRunStreamFallbackNonStreamingProvider -v`
    Expected: Single TokenDelta with full content; then SessionComplete
    Evidence: .sisyphus/evidence/task-11-runstream-fallback.txt

  Scenario: RunStream cancellation
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestRunStreamCancellationClosesChannel -race -v`
    Expected: Channel closed after cancellation; no race conditions; no goroutine leak
    Evidence: .sisyphus/evidence/task-11-runstream-cancel.txt
  ```

  **Commit**: YES | Message: `feat(runtime): implement Engine.RunStream with provider streaming integration` | Files: `internal/runtime/runtime.go, internal/runtime/runtime_stream_test.go, internal/runtime/model_adapter.go`

- [x] T12. Streaming Resume/Inspect Compatibility

  **What to do**:
  1. Verify that `resume --stream` works: resumes a session and streams remaining steps
  2. Verify that `inspect` still works on sessions created with streaming (no format differences in persisted data)
  3. Ensure streaming events are NOT persisted - only final Step/Session state (same as v1)
  4. Write integration test: run session with --stream → interrupt → resume with --stream → inspect
  5. Fix any inconsistencies found

  **Must NOT do**: Do not add new persistence fields for streaming. Do not change Session/Step schema.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Integration testing across CLI lifecycle with streaming
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: none | Blocked By: T10, T11, T7, T8, T9

  **References**:
  - Pattern: `cmd/agent/cli.go` - CLI resume/inspect commands
  - Pattern: `internal/store/` - SQLite persistence layer
  - Pattern: `testdata/runtime/resume_session.json` - Existing resume fixture

  **Acceptance Criteria**:
  - [ ] `go test ./cmd/agent/... -run TestStreamResumeLifecycle` passes
  - [ ] `go test ./cmd/agent/... -run TestStreamInspectCompat` passes
  - [ ] Inspected streaming session has identical format to non-streaming session
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: Stream → interrupt → resume lifecycle
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestStreamResumeLifecycle -v`
    Expected: Session starts with streaming, persists on interrupt, resumes with streaming, completes successfully
    Evidence: .sisyphus/evidence/task-12-stream-resume.txt

  Scenario: Inspect streaming session format
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestStreamInspectCompat -v`
    Expected: Inspected session format identical to v1; no streaming-specific fields in persisted data
    Evidence: .sisyphus/evidence/task-12-stream-inspect.txt
  ```

  **Commit**: YES | Message: `test(cli): verify streaming resume/inspect compatibility` | Files: `cmd/agent/cli_stream_compat_test.go`

- [x] T13. Update USAGE.md + README.md for Streaming + New Tools

  **What to do**:
  1. Update `docs/USAGE.md`:
     - Add `--stream` flag documentation for run/resume
     - Add JSONL streaming output format documentation
     - Add web_fetch, ask_user, code_search tool documentation
     - Add domain allowlist configuration for web_fetch
  2. Update `README.md`:
     - Update CLI usage examples with `--stream`
     - Add new tools to feature list
     - Update project structure (new adapter files)
     - Update v1 exclusions section (remove "no plugin system" - now in v2 scope)
  3. Update `PROGRESS.md` with v2 wave progress tracking

  **Must NOT do**: Do not add documentation for plugin system or multi-agent (those come in later waves).

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: Documentation update
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: none | Blocked By: T10, T11

  **References**:
  - Pattern: `docs/USAGE.md` - Current usage documentation
  - Pattern: `README.md` - Current README
  - Pattern: `PROGRESS.md` - Current progress tracking

  **Acceptance Criteria**:
  - [ ] `docs/USAGE.md` updated with --stream, web_fetch, ask_user, code_search
  - [ ] `README.md` updated with new features and tools
  - [ ] `PROGRESS.md` has v2 wave tracking section
  - [ ] No broken links in documentation

  **QA Scenarios**:
  ```
  Scenario: Documentation completeness
    Tool: Bash
    Steps: Grep USAGE.md for "--stream", "web_fetch", "ask_user", "code_search"
    Expected: All 4 terms present with usage examples
    Evidence: .sisyphus/evidence/task-13-docs.txt
  ```

  **Commit**: YES | Message: `docs: update USAGE and README for streaming and new tools` | Files: `docs/USAGE.md, README.md, PROGRESS.md`

- [x] T14. Plugin Tool Interface + Contract Version

  **What to do**:
  1. Create `internal/plugin/contract.go` defining:
     ```go
     const ContractVersion = "1.0.0"
     
     // PluginTool is the interface that all plugins must implement.
     // Go plugins export a function: func NewPluginTool() PluginTool
     // External processes implement the JSON-RPC protocol.
     type PluginTool interface {
         Name() string
         Description() string
         Schema() string                    // JSON schema for input
         SafetyLevel() domain.SafetyLevel   // or string
         ContractVersion() string           // Must match ContractVersion
         Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error)
         Close() error                      // Cleanup (for external processes)
     }
     ```
  2. Create `internal/plugin/doc.go` with package documentation
  3. Write TDD tests for contract validation:
     - TestContractVersionMismatch: plugin with wrong version → error on load
     - TestPluginToolInterfaceCompliance: verify PluginTool implements required methods

  **Must NOT do**: Do not implement loading mechanism yet. Do not add provider/agent/verifier plugins. Do not add plugin marketplace.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Core interface design that all plugin code depends on
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T15, T16, T17 | Blocked By: T6

  **References**:
  - Pattern: `internal/tools/definition.go` - ToolDefinition struct (PluginTool mirrors this)
  - Pattern: `internal/domain/tool.go` - ToolCall/ToolResult/ToolInfo types
  - Pattern: `internal/domain/ports.go` - Interface definition pattern
  - Doc: `docs/ADR-007-plugin-system.md` - Architecture decisions from T6

  **Acceptance Criteria**:
  - [ ] `internal/plugin/contract.go` exists with PluginTool interface and ContractVersion
  - [ ] `go test ./internal/plugin/... -run TestContractVersion` passes
  - [ ] PluginTool interface matches ToolDefinition fields (Name, Description, Schema, SafetyLevel)

  **QA Scenarios**:
  ```
  Scenario: Contract version validation
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestContractVersionMismatch -v`
    Expected: Plugin with mismatched version returns error on load; matching version succeeds
    Evidence: .sisyphus/evidence/task-14-contract.txt

  Scenario: Interface compliance
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestPluginToolInterfaceCompliance -v`
    Expected: All required methods present; type assertion succeeds
    Evidence: .sisyphus/evidence/task-14-interface.txt
  ```

  **Commit**: YES | Message: `feat(plugin): add PluginTool interface with contract version` | Files: `internal/plugin/contract.go, internal/plugin/doc.go, internal/plugin/contract_test.go`

- [x] T15. External-Process Plugin Loader (JSON-RPC 2.0 over stdio)

  **What to do**:
  1. Create `internal/plugin/external.go` implementing ExternalLoader:
     - Spawns plugin as subprocess (`exec.CommandContext`)
     - Discovers plugin via stdout handshake (similar to HashiCorp go-plugin pattern)
     - Implements JSON-RPC 2.0 protocol over stdin/stdout:
       ```json
       // Host → Plugin
       {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"contract_version":"1.0.0"}}
       {"jsonrpc":"2.0","id":2,"method":"tool.info","params":{}}
       {"jsonrpc":"2.0","id":3,"method":"tool.execute","params":{"name":"...","input":"..."}}
       {"jsonrpc":"2.0","id":4,"method":"shutdown","params":{}}
       
       // Plugin → Host
       {"jsonrpc":"2.0","id":1,"result":{"name":"...","description":"...","schema":"...","safety_level":"...","contract_version":"1.0.0"}}
       {"jsonrpc":"2.0","id":3,"result":{"output":"...","error":""}}
       ```
  2. Create `internal/plugin/protocol.go` with JSON-RPC request/response types
  3. Implement `ExternalPluginTool` struct that wraps subprocess and implements PluginTool interface
  4. Handle lifecycle: spawn → initialize → info → execute → shutdown → process cleanup
  5. Handle errors: startup timeout (5s), execution timeout (from tool call), crash recovery, malformed messages
  6. Separate stdout (protocol) from stderr (plugin logs, forwarded to host stderr)
  7. TDD tests with mock plugin binary (compiled test helper in `testdata/plugins/echo_plugin/`):
     - TestExternalPluginInitialize: handshake succeeds
     - TestExternalPluginExecute: tool call → result
     - TestExternalPluginCrashRecovery: plugin crashes → error returned, no host panic
     - TestExternalPluginMalformedResponse: malformed JSON → error, no panic
     - TestExternalPluginStartupTimeout: slow plugin → timeout error
     - TestExternalPluginShutdown: clean shutdown → process exits

  **Must NOT do**: Do not implement gRPC. Do not add HTTP transport. Do not implement Go plugin loading. Do not add sandboxing.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Subprocess management + IPC protocol + lifecycle handling; most complex plugin task
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T17 | Blocked By: T14

  **References**:
  - Pattern: `internal/tools/adapters/shell.go` - Subprocess execution pattern (exec.CommandContext)
  - Interface: `internal/plugin/contract.go` - PluginTool interface from T14
  - External: HashiCorp go-plugin handshake pattern (stdout discovery)
  - External: JSON-RPC 2.0 specification

  **Acceptance Criteria**:
  - [ ] `internal/plugin/external.go` exists with ExternalLoader and ExternalPluginTool
  - [ ] `internal/plugin/protocol.go` exists with JSON-RPC types
  - [ ] Mock plugin binary exists in `testdata/plugins/echo_plugin/`
  - [ ] `go test ./internal/plugin/... -run TestExternalPlugin` passes (all 6 sub-tests)
  - [ ] ExternalPluginTool implements PluginTool interface

  **QA Scenarios**:
  ```
  Scenario: External plugin execute
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestExternalPluginExecute -v`
    Expected: Tool call sent via JSON-RPC; result received; output matches mock plugin response
    Evidence: .sisyphus/evidence/task-15-ext-execute.txt

  Scenario: External plugin crash
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestExternalPluginCrashRecovery -v`
    Expected: Error returned with "plugin process exited"; no host panic; subprocess cleaned up
    Evidence: .sisyphus/evidence/task-15-ext-crash.txt

  Scenario: Malformed response
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestExternalPluginMalformedResponse -v`
    Expected: Error returned with "invalid JSON-RPC response"; no panic; protocol recovers for next call
    Evidence: .sisyphus/evidence/task-15-ext-malformed.txt

  Scenario: Startup timeout
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestExternalPluginStartupTimeout -v`
    Expected: Error returned within 5s; plugin process killed; no indefinite hang
    Evidence: .sisyphus/evidence/task-15-ext-timeout.txt
  ```

  **Commit**: YES | Message: `feat(plugin): add external-process loader with JSON-RPC over stdio` | Files: `internal/plugin/external.go, internal/plugin/protocol.go, internal/plugin/external_test.go, testdata/plugins/echo_plugin/main.go`

- [x] T16. Go Plugin Native Loader (Build-Tag Gated, Linux/macOS Only)

  **What to do**:
  1. Create `internal/plugin/native.go` with build tag `//go:build !windows`:
     ```go
     type NativeLoader struct{}
     func (l *NativeLoader) Load(path string) (PluginTool, error) {
         p, err := plugin.Open(path)
         // Lookup("NewPluginTool") → func() PluginTool
         // Validate ContractVersion
         // Return PluginTool instance
     }
     func (l *NativeLoader) CanLoad(path string) bool {
         return strings.HasSuffix(path, ".so")
     }
     ```
  2. Create `internal/plugin/native_stub.go` with build tag `//go:build windows`:
     ```go
     type NativeLoader struct{}
     func (l *NativeLoader) Load(path string) (PluginTool, error) {
         return nil, fmt.Errorf("native plugins not supported on Windows")
     }
     func (l *NativeLoader) CanLoad(path string) bool { return false }
     ```
  3. Write test `internal/plugin/native_test.go` (also build-tagged for !windows):
     - TestNativePluginLoad: loads .so, validates contract, returns PluginTool
     - TestNativePluginVersionMismatch: wrong version → error
     - TestNativePluginSymbolNotFound: no NewPluginTool symbol → error
  4. Create example native plugin in `testdata/plugins/native_example/` (builds to .so on Linux/macOS)

  **Must NOT do**: Do not add Windows .dll support. Do not add plugin unloading (Go limitation). Do not add hot-reload.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Build-tag gated code + Go plugin mechanics
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T17 | Blocked By: T14

  **References**:
  - Interface: `internal/plugin/contract.go` - PluginTool interface from T14
  - External: Go plugin package documentation (plugin.Open, Lookup)
  - Pattern: Build tag pattern for platform-specific code

  **Acceptance Criteria**:
  - [ ] `internal/plugin/native.go` exists with `//go:build !windows` tag
  - [ ] `internal/plugin/native_stub.go` exists with `//go:build windows` tag
  - [ ] `go build ./internal/plugin/...` succeeds on all platforms
  - [ ] On Windows: `go test ./internal/plugin/... -run TestNativePlugin` returns "not supported" error gracefully
  - [ ] On Linux/macOS: test loads .so and validates contract

  **QA Scenarios**:
  ```
  Scenario: Native plugin loads on Linux/macOS
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestNativePluginLoad -v` (on Linux/macOS)
    Expected: Plugin .so loaded; ContractVersion matches; Execute works
    Evidence: .sisyphus/evidence/task-16-native-load.txt

  Scenario: Native plugin stub on Windows
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestNativePlugin -v` (on Windows)
    Expected: CanLoad returns false; Load returns "not supported" error; no panic
    Evidence: .sisyphus/evidence/task-16-native-stub.txt

  Scenario: Version mismatch
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestNativePluginVersionMismatch -v`
    Expected: Error returned with version mismatch details; plugin not loaded
    Evidence: .sisyphus/evidence/task-16-native-version.txt
  ```

  **Commit**: YES | Message: `feat(plugin): add Go plugin native loader with Windows stub` | Files: `internal/plugin/native.go, internal/plugin/native_stub.go, internal/plugin/native_test.go, testdata/plugins/native_example/main.go`

- [x] T17. PluginManager (Discovery, Loading, Version Validation, Lifecycle)

  **What to do**:
  1. Create `internal/plugin/manager.go` implementing PluginManager:
     ```go
     type Manager struct {
         loaders  []Loader       // Ordered: NativeLoader first, then ExternalLoader
         tools    map[string]PluginTool
         mu       sync.RWMutex
     }
     type Loader interface {
         Load(path string) (PluginTool, error)
         CanLoad(path string) bool
     }
     func NewManager() *Manager                          // Creates with both loaders
     func (m *Manager) LoadPlugin(path string) error      // Tries loaders in order
     func (m *Manager) LoadDir(dir string) error          // Loads all plugins in directory
     func (m *Manager) Get(name string) (PluginTool, bool)
     func (m *Manager) List() []PluginTool
     func (m *Manager) Close() error                      // Close all plugins (terminate external processes)
     ```
  2. Loading logic:
     - Try NativeLoader first (if CanLoad), then ExternalLoader
     - Validate ContractVersion on each loaded plugin
     - Reject duplicate tool names
     - Log loading progress (name, type, version)
  3. Directory scanning: `LoadDir` walks directory, loads .so files and executable files
  4. TDD tests:
     - TestManagerLoadPlugin: loads external plugin via Manager
     - TestManagerLoadDir: scans directory, loads all valid plugins
     - TestManagerRejectDuplicate: second plugin with same name → error
     - TestManagerVersionValidation: wrong version → rejected
     - TestManagerClose: terminates all external processes
     - TestManagerFallbackToExternal: .so not available on Windows → external loader used

  **Must NOT do**: Do not add plugin marketplace. Do not add auto-update. Do not add plugin dependencies.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Central plugin management with multi-loader dispatch
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T18, T19 | Blocked By: T14, T15, T16

  **References**:
  - Interface: `internal/plugin/contract.go` - PluginTool from T14
  - Pattern: `internal/plugin/external.go` - ExternalLoader from T15
  - Pattern: `internal/plugin/native.go` - NativeLoader from T16
  - Pattern: `internal/tools/registry.go` - In-memory map with RWMutex pattern

  **Acceptance Criteria**:
  - [ ] `internal/plugin/manager.go` exists with Manager struct
  - [ ] `go test ./internal/plugin/... -run TestManager` passes (all 6 sub-tests)
  - [ ] Manager tries native loader first, falls back to external
  - [ ] Duplicate tool names rejected
  - [ ] Close terminates all external plugin processes

  **QA Scenarios**:
  ```
  Scenario: Load directory of plugins
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestManagerLoadDir -v`
    Expected: All valid plugins loaded; invalid files skipped; no panic
    Evidence: .sisyphus/evidence/task-17-manager-dir.txt

  Scenario: Duplicate rejection
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestManagerRejectDuplicate -v`
    Expected: Second plugin with same name returns error; first plugin remains loaded
    Evidence: .sisyphus/evidence/task-17-manager-dup.txt

  Scenario: Close cleanup
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestManagerClose -v`
    Expected: All external processes terminated; no zombie processes; Close returns nil
    Evidence: .sisyphus/evidence/task-17-manager-close.txt
  ```

  **Commit**: YES | Message: `feat(plugin): add PluginManager with discovery and lifecycle` | Files: `internal/plugin/manager.go, internal/plugin/manager_test.go`

- [x] T18. Plugin Safety Policy Extension

  **What to do**:
  1. Extend `internal/tools/safety.go` SafetyPolicy with:
     ```go
     AllowedPluginPaths []string   // Directories where plugins can be loaded from
     AllowedPluginTools []string   // Tool names allowed from plugins (empty = all allowed)
     DeniedPluginTools  []string   // Tool names explicitly denied
     ```
  2. Add validation in PluginManager.LoadPlugin:
     - Plugin path must be within AllowedPluginPaths (if set)
     - Plugin tool name must not be in DeniedPluginTools
     - Plugin tool name must be in AllowedPluginTools (if set; empty = all allowed)
  3. Extend executor to validate plugin tool calls against SafetyPolicy
  4. TDD tests:
     - TestPluginPathAllowed: path in AllowedPluginPaths → load succeeds
     - TestPluginPathDenied: path not in AllowedPluginPaths → load rejected
     - TestPluginToolDenied: tool name in DeniedPluginTools → load rejected
     - TestPluginToolAllowed: tool name in AllowedPluginTools → load succeeds

  **Must NOT do**: Do not add sandboxing. Do not add capability-based permissions. Do not add code signing.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Security policy extension with clear boundaries
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: none | Blocked By: T17

  **References**:
  - Pattern: `internal/tools/safety.go` - Existing SafetyPolicy with AllowedCommands pattern
  - Pattern: `internal/plugin/manager.go` - Manager from T17

  **Acceptance Criteria**:
  - [ ] SafetyPolicy has AllowedPluginPaths, AllowedPluginTools, DeniedPluginTools fields
  - [ ] `go test ./internal/plugin/... -run TestPluginSafety` passes
  - [ ] `go test ./internal/tools/... -run TestSafetyPolicy` still passes
  - [ ] Config file supports plugin safety fields

  **QA Scenarios**:
  ```
  Scenario: Plugin path validation
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestPluginPathDenied -v`
    Expected: Plugin from unauthorized path rejected with clear error message
    Evidence: .sisyphus/evidence/task-18-safety-path.txt

  Scenario: Plugin tool name denied
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestPluginToolDenied -v`
    Expected: Plugin with denied tool name rejected; not registered in manager
    Evidence: .sisyphus/evidence/task-18-safety-tool.txt
  ```

  **Commit**: YES | Message: `feat(tools): extend SafetyPolicy with plugin path and tool allowlist` | Files: `internal/tools/safety.go, internal/plugin/manager.go, internal/config/config.go`

- [x] T19. Plugin CLI Flags (--plugin-dir, --plugin, --allow-plugin)

  **What to do**:
  1. Add CLI flags to `run` and `resume` commands:
     - `--plugin-dir <path>`: Directory to scan for plugins (can be specified multiple times)
     - `--plugin <path>`: Specific plugin file to load (can be specified multiple times)
     - `--allow-plugin <tool-name>`: Allow specific plugin tool name (can be specified multiple times; default: all allowed)
  2. Integrate PluginManager into CLI flow:
     - After config loading, create Manager
     - Load plugins from --plugin-dir and --plugin paths
     - Register loaded PluginTools into Executor's Registry
     - Add PluginTools to tool list provided to Model
  3. Add plugin fields to config file (`zheng.json`):
     ```json
     "plugins": {
       "dirs": ["./plugins"],
       "allowed": ["my_tool"],
       "denied": ["dangerous_tool"]
     }
     ```
  4. TDD tests:
     - TestCLIPluginDir: --plugin-dir loads plugins from directory
     - TestCLIPluginSpecific: --plugin loads specific plugin
     - TestCLIPluginAllowDeny: --allow-plugin filters loaded tools

  **Must NOT do**: Do not add plugin marketplace URL. Do not add plugin auto-download. Do not add interactive plugin management.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: CLI integration with clear flag semantics
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: none | Blocked By: T17

  **References**:
  - Pattern: `cmd/agent/cli.go` - Current CLI flag definitions
  - Pattern: `internal/config/config.go` - Config file format
  - Pattern: `internal/plugin/manager.go` - Manager from T17

  **Acceptance Criteria**:
  - [ ] `--plugin-dir`, `--plugin`, `--allow-plugin` flags added to run/resume
  - [ ] Config file supports `plugins` section
  - [ ] `go test ./cmd/agent/... -run TestCLIPlugin` passes
  - [ ] Plugin tools appear in tool list for Model
  - [ ] `go test ./...` still passes

  **QA Scenarios**:
  ```
  Scenario: Load plugins from directory
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestCLIPluginDir -v`
    Expected: Plugins from directory loaded; tool names available to agent
    Evidence: .sisyphus/evidence/task-19-cli-plugindir.txt

  Scenario: Allow/deny filter
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestCLIPluginAllowDeny -v`
    Expected: Denied plugin not loaded; allowed plugin loaded; tool list reflects filter
    Evidence: .sisyphus/evidence/task-19-cli-filter.txt
  ```

  **Commit**: YES | Message: `feat(cli): add plugin loading flags and config support` | Files: `cmd/agent/cli.go, internal/config/config.go, cmd/agent/cli_plugin_test.go`

- [x] T20. Subtask + TaskDecomposition Domain Types

  **What to do**:
  1. Add to `internal/domain/task.go`:
     ```go
     type Subtask struct {
         ID           string              `json:"id"`
         Description  string              `json:"description"`
         TaskType     TaskCategory        `json:"task_type"`
         Dependencies []string            `json:"dependencies,omitempty"`
         Tools        []string            `json:"tools,omitempty"`
         Timeout      time.Duration       `json:"timeout"`
     }
     
     type TaskDecomposition struct {
         ID         string    `json:"id"`
         Subtasks   []Subtask `json:"subtasks"`
         Strategy   string    `json:"strategy"` // "all_succeed" | "best_effort"
         CreatedAt  time.Time `json:"created_at"`
     }
     
     type WorkerResult struct {
         SubtaskID   string          `json:"subtask_id"`
         Status      string          `json:"status"` // "success"|"failed"|"timeout"|"cancelled"
         Output      string          `json:"output,omitempty"`
         Error       string          `json:"error,omitempty"`
         Steps       []Step          `json:"steps,omitempty"`
         Duration    time.Duration   `json:"duration_ms"`
     }
     ```
  2. Validate DAG: detect cycles in Dependencies
  3. TDD tests: cycle detection, topological sort, field validation

  **Must NOT do**: Do not add TaskDecomposer interface. Do not modify existing Task type.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Core domain type design with DAG validation
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: T21, T22 | Blocked By: none

  **References**:
  - Pattern: `internal/domain/task.go` - Current Task type
  - Pattern: `internal/domain/plan.go` - Plan type (similar structure)
  - Pattern: `internal/domain/step.go` - Step type (referenced in WorkerResult)

  **Acceptance Criteria**:
  - [ ] Subtask, TaskDecomposition, WorkerResult types exist in domain/task.go
  - [ ] `go test ./internal/domain/... -run TestTaskDecompositionCycleDetection` passes
  - [ ] `go test ./internal/domain/... -run TestTaskDecompositionTopoSort` passes

  **QA Scenarios**:
  ```
  Scenario: DAG cycle detection
    Tool: Bash
    Steps: Run `go test ./internal/domain/... -run TestTaskDecompositionCycleDetection -v`
    Expected: Circular dependency returns error; acyclic succeeds
    Evidence: .sisyphus/evidence/task-20-dag.txt

  Scenario: Topological sort
    Tool: Bash
    Steps: Run `go test ./internal/domain/... -run TestTaskDecompositionTopoSort -v`
    Expected: Dependencies appear before dependents in sorted order
    Evidence: .sisyphus/evidence/task-20-sort.txt
  ```

  **Commit**: YES | Message: `feat(domain): add Subtask and TaskDecomposition types` | Files: `internal/domain/task.go, internal/domain/task_decomp_test.go`

- [x] T21. Orchestrator (errgroup-based Dispatch)

  **What to do**:
  1. Create `internal/runtime/orchestrator.go` with Orchestrator struct
  2. Implement `Execute(ctx, TaskDecomposition) ([]WorkerResult, error)`:
     - Topo-sort subtasks by dependencies
     - Dispatch independent subtasks via `errgroup.WithContext` + `SetLimit(maxWorkers)`
     - Each worker calls `engine.Run(ctx, scopedTask)` in goroutine
     - AllSucceed: first error cancels group
     - BestEffort: collect all results, filter errors
  3. Add `golang.org/x/sync` to go.mod
  4. TDD: all pass, fail-fast, best-effort, concurrency limit, dependency order

  **Must NOT do**: Do not implement LLM-driven decomposition. No recursive workers. Do not modify Engine.Run().

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Core orchestration with concurrency and dependency management
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: T23, T25 | Blocked By: T20

  **References**:
  - Pattern: `internal/runtime/runtime.go` - Engine.Run() used by workers
  - Type: `internal/domain/task.go` - TaskDecomposition, WorkerResult from T20
  - External: `golang.org/x/sync/errgroup` (new dependency)

  **Acceptance Criteria**:
  - [ ] `internal/runtime/orchestrator.go` exists
  - [ ] `go test ./internal/runtime/... -run TestOrchestratorAllSucceed` passes
  - [ ] `go test ./internal/runtime/... -run TestOrchestratorFailFast` passes
  - [ ] `go test ./internal/runtime/... -run TestOrchestratorMaxConcurrency` passes
  - [ ] Max workers enforced; dependency order respected

  **QA Scenarios**:
  ```
  Scenario: All workers succeed
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestOrchestratorAllSucceed -v`
    Expected: All WorkerResults success; no errors
    Evidence: .sisyphus/evidence/task-21-orch.txt

  Scenario: Concurrency limit
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestOrchestratorMaxConcurrency -v`
    Expected: Max 3 concurrent with maxWorkers=3; all 10 complete
    Evidence: .sisyphus/evidence/task-21-concurrency.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add Orchestrator with errgroup-based dispatch` | Files: `internal/runtime/orchestrator.go, internal/runtime/orchestrator_test.go, go.mod, go.sum`

- [x] T22. Worker Agent (Scoped Plan-Execute-Verify Loop)

  **What to do**:
  1. Create `internal/runtime/worker.go` with Worker struct wrapping Engine
  2. Worker.Execute: maps Subtask→Task, calls engine.Run(), builds WorkerResult
  3. Tool scoping: if Subtask.Tools non-empty, restrict to listed tools
  4. Handle: success, verification failure, timeout, cancellation
  5. TDD: success, failure, timeout, tool scope

  **Must NOT do**: No separate Engine instances. No recursive workers.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Worker lifecycle with Engine integration
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: T23, T24 | Blocked By: T20

  **References**:
  - Pattern: `internal/runtime/runtime.go:45-146` - Engine.Run()
  - Type: `internal/domain/task.go` - Subtask, WorkerResult from T20

  **Acceptance Criteria**:
  - [ ] `internal/runtime/worker.go` exists
  - [ ] `go test ./internal/runtime/... -run TestWorkerSuccess` passes
  - [ ] `go test ./internal/runtime/... -run TestWorkerTimeout` passes
  - [ ] `go test ./internal/runtime/... -run TestWorkerToolScope` passes

  **QA Scenarios**:
  ```
  Scenario: Tool scoping works
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestWorkerToolScope -v`
    Expected: Unlisted tools rejected for this worker
    Evidence: .sisyphus/evidence/task-22-scope.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add Worker with scoped PEV loop` | Files: `internal/runtime/worker.go, internal/runtime/worker_test.go`

- [x] T23. Channel-Based Message Passing

  **What to do**:
  1. Create `internal/runtime/messaging.go` with TaskQueue (incoming/outgoing typed channels)
  2. Enqueue/Dequeue for task dispatch; Submit for result collection
  3. Goroutine-safe with buffer and close semantics
  4. TDD: round-trip, buffer full, results flow, graceful close

  **Must NOT do**: No persistence. No network transport. No routing/broadcasting.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Typed channel infrastructure
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: T24, T25 | Blocked By: T21, T22

  **References**:
  - Pattern: `internal/runtime/emitter.go` - EventChannel pattern from T4

  **Acceptance Criteria**:
  - [ ] `internal/runtime/messaging.go` exists
  - [ ] `go test ./internal/runtime/... -run TestTaskQueueEnqueueDequeue` passes
  - [ ] `go test ./internal/runtime/... -run TestTaskQueueClose` passes

  **QA Scenarios**:
  ```
  Scenario: Queue close drains existing
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestTaskQueueClose -v`
    Expected: After close, no new items accepted; existing drain safely
    Evidence: .sisyphus/evidence/task-23-close.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add typed channel message passing` | Files: `internal/runtime/messaging.go, internal/runtime/messaging_test.go`

- [x] T24. DAG Dependency-Aware Scheduling

  **What to do**:
  1. Create `internal/runtime/scheduler.go` with Scheduler
  2. Topo-sort → maintain ready set → dispatch ready to queue → on result, update ready set
  3. No worker waits for another directly; all coordination through scheduler
  4. TDD: linear chain, parallel branches, diamond pattern, max concurrency

  **Must NOT do**: No deadlock detection. No priority scheduling. No time-based scheduling.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: DAG scheduling algorithm
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: none | Blocked By: T22, T23

  **References**:
  - Type: `internal/domain/task.go` - Subtask with Dependencies from T20
  - Pattern: `internal/runtime/messaging.go` - TaskQueue from T23
  - Pattern: `internal/runtime/worker.go` - Worker from T22

  **Acceptance Criteria**:
  - [ ] `internal/runtime/scheduler.go` exists
  - [ ] `go test ./internal/runtime/... -run TestSchedulerLinearChain` passes
  - [ ] `go test ./internal/runtime/... -run TestSchedulerDiamond` passes

  **QA Scenarios**:
  ```
  Scenario: Diamond execution
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestSchedulerDiamond -v`
    Expected: A→(B\|C)→D; D starts only after both B and C finish
    Evidence: .sisyphus/evidence/task-24-diamond.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add DAG dependency-aware scheduler` | Files: `internal/runtime/scheduler.go, internal/runtime/scheduler_test.go`

- [x] T25. Result Aggregation Strategies

  **What to do**:
  1. Create `internal/runtime/aggregator.go` with Aggregator and AggregatedResult
  2. AllSucceed: error if any worker fails (but partial results included)
  3. BestEffort: no error, all results included, summary with counts
  4. Combine individual outputs into combined output
  5. TDD: all pass, all fail, mixed, empty, combined output

  **Must NOT do**: No majority voting. No confidence weighting. No LLM summarization.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Result aggregation logic
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: none | Blocked By: T21, T23

  **References**:
  - Type: `internal/domain/task.go` - WorkerResult from T20
  - Pattern: `internal/runtime/orchestrator.go` - Orchestrator from T21 (consumer)

  **Acceptance Criteria**:
  - [ ] `internal/runtime/aggregator.go` exists
  - [ ] `go test ./internal/runtime/... -run TestAggregateAllSucceedPass` passes
  - [ ] `go test ./internal/runtime/... -run TestAggregateBestEffort` passes

  **QA Scenarios**:
  ```
  Scenario: Best effort with mixed results
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestAggregateBestEffort -v`
    Expected: No error; summary shows "3/4 succeeded, 1 failed"
    Evidence: .sisyphus/evidence/task-25-best.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add result aggregation strategies` | Files: `internal/runtime/aggregator.go, internal/runtime/aggregator_test.go`

- [x] T26. Multi-Agent CLI Commands

  **What to do**:
  1. Add `--decompose <strategy>` and `--max-workers <n>` flags to `run` command
  2. When --decompose set: create Orchestrator, execute, display AggregatedResult
  3. Without --decompose: normal single-agent behavior
  4. TDD: multi-agent run, max workers enforced, no decompose flag = v1 behavior

  **Must NOT do**: No LLM-driven decomposition. No --decompose on resume.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: CLI integration with flag handling
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: none | Blocked By: T25

  **References**:
  - Pattern: `cmd/agent/cli.go` - Current CLI flags
  - Pattern: `internal/runtime/orchestrator.go` - Orchestrator from T21

  **Acceptance Criteria**:
  - [ ] `--decompose` and `--max-workers` flags added
  - [ ] `go test ./cmd/agent/... -run TestCLIMultiAgentRun` passes
  - [ ] `go test ./cmd/agent/... -run TestCLIMultiAgentMaxWorkers` passes

  **QA Scenarios**:
  ```
  Scenario: Multi-agent run
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestCLIMultiAgentRun -v`
    Expected: Orchestrator.Execute called; AggregatedResult displayed
    Evidence: .sisyphus/evidence/task-26-multi.txt
  ```

  **Commit**: YES | Message: `feat(cli): add --decompose and --max-workers for multi-agent` | Files: `cmd/agent/cli.go, cmd/agent/cli_multiagent_test.go`

- [x] T27. Full Integration Test: Streaming + Tools + Plugins + Multi-Agent

  **What to do**:
  1. Create `internal/integration/v2_integration_test.go` with build tag `//go:build integration`
  2. End-to-end scenario: run agent with --stream, --decompose, --plugin-dir combined
  3. Verify streaming events emitted, plugin tools available, multi-agent orchestration completes
  4. Gap detection: find incompatibilities between features; fix integration issues
  5. Follow existing replay fixture pattern for test structure

  **Must NOT do**: Do not make this part of normal CI. Do not add new features during integration.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Cross-feature integration testing with dependency verification
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 5 | Blocks: T28, T29 | Blocked By: T12, T19, T26

  **References**:
  - Pattern: `testdata/runtime/success_session.json` - Existing fixture pattern
  - Pattern: `internal/runtime/runtime_replay_test.go` - Existing integration test pattern

  **Acceptance Criteria**:
  - [ ] `internal/integration/v2_integration_test.go` exists
  - [ ] `go test -tags=integration ./internal/integration/...` passes
  - [ ] Streaming + plugin + multi-agent coexist without conflicts
  - [ ] `go test ./...` still passes (integration excluded by build tag)

  **QA Scenarios**:
  ```
  Scenario: All v2 features work together
    Tool: Bash
    Steps: Run `go test -tags=integration ./internal/integration/... -v`
    Expected: Streaming events flow, plugin tool executes, multi-agent completes; no errors
    Evidence: .sisyphus/evidence/task-27-integration.txt

  Scenario: Normal suite unaffected
    Tool: Bash
    Steps: Run `go test ./...`
    Expected: All existing tests pass; integration tests not included
    Evidence: .sisyphus/evidence/task-27-normal.txt
  ```

  **Commit**: YES | Message: `test(integration): add full v2 feature integration test` | Files: `internal/integration/v2_integration_test.go`

- [x] T28. Validation Matrix Update for v2

  **What to do**:
  1. Update `docs/validation-matrix.md` with v2 proof surfaces:
     - Streaming: provider interface, runtime event channel, CLI output
     - New tools: web_fetch, ask_user, code_search
     - Plugin system: external-process loader, native loader, manager, safety
     - Multi-agent: orchestrator, worker, scheduler, aggregator
     - Integration: all features combined
  2. Follow existing matrix format (proof surface, command/test, expected outcome, evidence, status)
  3. Link all evidence file paths from T1-T28

  **Must NOT do**: Do not modify v1 sections. Do not remove existing validation entries.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: Documentation finalization
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 5 | Blocks: T29 | Blocked By: T27

  **References**:
  - Pattern: `docs/validation-matrix.md` - Existing v1 matrix with full format

  **Acceptance Criteria**:
  - [ ] `docs/validation-matrix.md` updated with v2 sections
  - [ ] All v2 tasks have corresponding matrix entries
  - [ ] Evidence file paths listed for each proof surface

  **QA Scenarios**:
  ```
  Scenario: Matrix completeness
    Tool: Bash
    Steps: Grep validation-matrix.md for "streaming", "plugin", "multi-agent"
    Expected: All 3 terms present with proof surfaces and test commands
    Evidence: .sisyphus/evidence/task-28-matrix.txt
  ```

  **Commit**: YES | Message: `docs: update validation matrix for v2 features` | Files: `docs/validation-matrix.md`

- [x] T29. v2 Release Preparation

  **What to do**:
  1. Run all final acceptance commands:
     ```bash
     go build ./... && go test ./... && go test -race ./... && go test -cover ./...
     ```
  2. Update `PROGRESS.md` with v2 completion status (all waves done)
  3. Update `README.md`: remove v1 exclusions now in v2, add new features section
  4. Update ADR index in README (add ADR-006, ADR-007)
  5. Confirm all evidence files present in `.sisyphus/evidence/`
  6. Document deferred items for v3 consideration

  **Must NOT do**: Do not create git tag. Do not publish GitHub Release. Do not merge to release branch.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: Release documentation and acceptance verification
  - Skills: [] - No special skills needed
  - Omitted: [`playwright`] - Not browser automation

  **Parallelization**: Can Parallel: NO | Wave 5 | Blocks: F1-F4 | Blocked By: T28

  **References**:
  - Pattern: `PROGRESS.md` - Current progress tracking
  - Pattern: `README.md` - Current README (v1 exclusions section needs update)

  **Acceptance Criteria**:
  - [ ] `go build ./...` passes with zero errors
  - [ ] `go test ./...` passes with zero failures
  - [ ] `go test -race ./...` passes with zero race conditions
  - [ ] `PROGRESS.md` updated with v2 completion and deferred v3 items
  - [ ] `README.md` v1 exclusions updated for v2 capabilities
  - [ ] All evidence files T1-T28 present

  **QA Scenarios**:
  ```
  Scenario: Build and tests pass
    Tool: Bash
    Steps: Run `go build ./... && go test ./... && go test -race ./...`
    Expected: All exit 0; no errors, failures, or races
    Evidence: .sisyphus/evidence/task-29-build-test.txt

  Scenario: Evidence completeness
    Tool: Bash
    Steps: Check .sisyphus/evidence/ for task-1 through task-28 files
    Expected: All evidence files present and non-empty
    Evidence: .sisyphus/evidence/task-29-evidence.txt
  ```

  **Commit**: YES | Message: `docs: v2 release preparation and final documentation update` | Files: `PROGRESS.md, README.md, docs/validation-matrix.md`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Each task commits independently with message format: `type(scope): description`
- Types: feat, fix, refactor, test, docs, chore
- Scopes: llm, runtime, tools, plugin, agent, cli, config, docs
- No merge commits; rebase before merge

## Success Criteria
1. `go build ./...` compiles with zero errors
2. `go test ./...` passes with zero failures
3. `go test -race ./...` passes with zero race conditions
4. All 3 providers (dashscope, openai, anthropic) pass real-network smoke tests or fail with documented error class
5. CLI streaming displays incremental token output in real-time
6. web_fetch, ask_user, code_search tools each have passing TDD tests
7. External-process plugin loads, executes, and unloads cleanly on Windows
8. Go plugin loads on Linux/macOS with version contract validation
9. Orchestrator dispatches workers with bounded concurrency and cancellation propagation
10. v2 validation matrix documents all proof surfaces
