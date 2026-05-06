# v4 API Server Productization

## TL;DR
> **Summary**: v4 turns zheng-harness from a CLI-only harness into a CLI+HTTP dual-entrypoint engine by adding a host-owned HTTP API server, SSE streaming, session-actor concurrency, and real OpenAI/Anthropic providers without changing the Harness Engineering core constraints.
> **Deliverables**:
> - `cmd/server` HTTP server entrypoint with authenticated REST endpoints
> - Session-actor runtime manager for concurrent multi-session execution
> - SSE streaming endpoint for runtime events
> - Real OpenAI and Anthropic provider implementations
> - SQLite concurrency hardening, docs/progress sync, and release/push workflow coverage
> **Effort**: XL
> **Parallel**: YES - 4 waves
> **Critical Path**: API contract/ADR → shared builder + session manager → HTTP endpoints + SSE → provider completion → docs/verification/release

## Context
### Original Request
v3的内容也已经完成了，如何规划后续内容，要牢记项目的目标。

### Interview Summary
- v3 is treated as completed baseline; it must not be reopened except for documentation truth-sync.
- The project goal remains unchanged: **General Agent Harness Engine** grounded in **Constrain / Inform / Verify / Correct**.
- The chosen next direction is **productization**, but without turning v4 into a UI-heavy product release.
- v4 scope is fixed to: HTTP API server, SSE streaming, session-actor multi-session execution, OpenAI/Anthropic provider completion, and progress/document sync.
- v5 is explicitly reserved for Web UI; do not mix template/htmx work into v4.
- Multi-session execution must use **session-actor** architecture: one goroutine + one engine instance per active session.
- Streaming protocol is **SSE only** in v4.

### Metis Review (gaps addressed)
- Lock v4 boundary to API + provider completion only; prevent Web UI scope creep.
- Define exact session lifecycle and duplicate-resume behavior.
- Define SSE contract explicitly: endpoint path, event names, ordering, reconnect semantics, completion behavior.
- Preserve fail-closed security model and route-level auth on every API endpoint including SSE.
- Make SQLite concurrency policy explicit instead of assuming current single-session behavior scales unchanged.
- Bound provider completion to concrete parity requirements rather than vague “补齐”.

## Work Objectives
### Core Objective
Expose zheng-harness as a stable, authenticated, stream-capable HTTP service while preserving the existing CLI entrypoint, verification-first runtime semantics, fail-closed security posture, and inspectable/resumable session model.

### Deliverables
1. `cmd/server` entrypoint that starts an authenticated HTTP API server without changing `cmd/agent` behavior.
2. Shared runtime construction layer extracted from CLI wiring so CLI and server use the same engine assembly rules.
3. Session manager implementing **session-actor** concurrency with server-generated session IDs, active-session caps, duplicate-resume prevention, and graceful shutdown handling.
4. REST endpoints:
   - `POST /api/v1/run`
   - `POST /api/v1/resume`
   - `GET /api/v1/sessions/{id}/inspect`
   - `GET /api/v1/sessions/{id}/stream`
   - `GET /healthz`
5. SSE contract for the six existing runtime events with ordered delivery per session.
6. Real HTTP-backed OpenAI and Anthropic providers with the same host-facing provider contract as DashScope.
7. Persistence/config/docs updates including SQLite WAL policy, server config, README/USAGE/PROGRESS/validation matrix truth alignment.

### Definition of Done (verifiable conditions with commands)
```bash
go build ./...
go test ./...
go test -race ./...
go test ./cmd/server/...
go test ./internal/server/...
go test ./internal/runtime/... -run TestSessionManager
go test ./internal/llm/... -run "Test(OpenAI|Anthropic)"
go test ./cmd/agent/... -run TestCLIUnaffectedByServer
curl -sS http://127.0.0.1:8080/healthz
curl -sS -X POST http://127.0.0.1:8080/api/v1/run -H "Authorization: Bearer test-token" -H "Content-Type: application/json" --data "{\"task\":\"ping\",\"task_type\":\"general\"}"
curl -N http://127.0.0.1:8080/api/v1/sessions/test-session/stream -H "Authorization: Bearer test-token"
```

### Must Have
- `cmd/agent` remains the canonical CLI entrypoint and retains existing semantics.
- New `cmd/server` runs independently and reuses shared engine-building logic.
- API authentication is mandatory for all `/api/v1/*` endpoints; `GET /healthz` is the only unauthenticated route.
- Session IDs are **server-generated** on `POST /api/v1/run`.
- `POST /api/v1/run` always creates a new session.
- `POST /api/v1/resume` only resumes an existing unfinished session.
- Duplicate resume/start against an already-running session returns **409 Conflict**.
- Active-session limit defaults to **8** and is configurable; exceeding the limit returns **429 Too Many Requests**.
- Each active session owns exactly one engine instance and one actor goroutine.
- SSE event ordering is preserved **within a session**.
- Client disconnect from SSE does **not** cancel the running session.
- v4 provides **no event replay**; reconnect receives only future events if the session is still active.
- SQLite is moved to WAL mode for server operation and tested under concurrent session writes.
- OpenAI and Anthropic providers support both non-streaming generate and streaming paths.
- Provider errors are normalized into deterministic runtime-visible failures consistent with existing fail-closed behavior.
- PROGRESS.md is updated so v3 is marked complete and v4 is represented truthfully.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- NO Web UI, htmx templates, browser routes, or human-facing pages in v4.
- NO WebSocket transport in v4.
- NO GraphQL, JSON-RPC, or second API style; REST + SSE only.
- NO shared mutable engine instance across sessions.
- NO silent downgrade from authenticated route to unauthenticated access.
- NO automatic replay/event history store for SSE in v4.
- NO change to Harness Engineering core invariants: tool allowlists, security levels, verification authority, bounded correction.
- NO provider redesign beyond completing OpenAI/Anthropic parity required for the current host contract.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: TDD (RED-GREEN-REFACTOR)
- QA policy: every task includes executable happy-path and failure/edge-case scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`
- API verification policy: every endpoint needs auth success, auth failure, validation failure, and runtime-path coverage
- Streaming verification policy: ordered event assertions, disconnect behavior assertions, and terminal event assertions are mandatory
- Concurrency verification policy: at least one multi-session run, one duplicate-resume rejection, one active-limit rejection, and one graceful shutdown case

## Execution Strategy
### Parallel Execution Waves
Wave 1: Contract, shared builder extraction, and session-manager foundation
Wave 2: HTTP surface and persistence hardening
Wave 3: Provider completion and server integration
Wave 4: Docs, truth-sync, release-flow validation

### Dependency Matrix (full, all tasks)
| Task | Blocks | Blocked By |
|------|--------|------------|
| T1 v4 ADR + API contract freeze | T2,T3,T4,T5,T8,T9,T10,T11 | - |
| T2 Extract shared engine/server builder | T4,T5,T8,T9,T10 | T1 |
| T3 Implement session-actor manager | T5,T6,T7,T10 | T1 |
| T4 Add server config + cmd/server bootstrap | T5,T6,T7,T10,T11 | T1,T2 |
| T5 Implement authenticated REST run/resume/inspect endpoints | T7,T10,T11 | T2,T3,T4 |
| T6 Harden SQLite/session lifecycle semantics | T7,T10,T11 | T3,T4 |
| T7 Implement SSE stream endpoint + ordered relay | T10,T11 | T3,T4,T5,T6 |
| T8 Complete OpenAI provider implementation | T10,T11 | T1,T2 |
| T9 Complete Anthropic provider implementation | T10,T11 | T1,T2 |
| T10 End-to-end server integration + auth/shutdown/concurrency coverage | T11 | T5,T6,T7,T8,T9 |
| T11 Docs/progress/release flow sync | F1,F2,F3,F4 | T4,T5,T6,T7,T8,T9,T10 |

### Agent Dispatch Summary
| Wave | Task Count | Categories |
|------|------------|------------|
| Wave 1 | 4 | writing, deep, deep, unspecified-high |
| Wave 2 | 3 | deep, unspecified-high, deep |
| Wave 3 | 3 | deep, deep, unspecified-high |
| Wave 4 | 1 | writing |

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] T1. Freeze v4 productization ADR and external API contract
- [x] T2. Extract shared runtime assembly into a reusable builder for CLI and server
- [x] T3. Implement session-actor manager for concurrent multi-session execution
- [x] T4. Add server configuration and `cmd/server` bootstrap
- [x] T5. Implement authenticated REST endpoints for run, resume, and inspect
- [x] T6. Harden SQLite persistence and session lifecycle semantics for server concurrency
- [x] T7. Implement SSE stream endpoint with ordered event relay and disconnect-safe behavior
- [x] T8. Complete the real OpenAI provider implementation
- [x] T9. Complete the real Anthropic provider implementation

  **What to do**:
  1. Create `cmd/server` as a new binary entrypoint.
  2. Add server config fields for listen address, JWT configuration source, active session cap, shutdown timeout, and server-mode database behavior.
  3. Initialize the shared builder, session manager, router, and graceful-shutdown lifecycle from this entrypoint.
  4. Enable SQLite WAL mode explicitly for server startup path and fail closed if the database cannot enter the required mode.

  **Must NOT do**: Do not fold server startup into `cmd/agent`; keep dual-entrypoint separation explicit.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: bootstrap, config, and operational wiring are broad but bounded.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T5,T6,T7,T10,T11 | Blocked By: T1,T2

  **References**:
  - Pattern: `cmd/agent/main.go` - Existing thin entrypoint pattern.
  - Reference: `internal/config/config.go` - Config loading source of truth.
  - External: `https://github.com/go-chi/chi` - Router choice baseline.

  **Acceptance Criteria**:
  - [ ] `cmd/server` starts independently of CLI.
  - [ ] Server config can be loaded via existing precedence rules plus server-specific fields.
  - [ ] Startup fails deterministically on invalid JWT config or WAL initialization failure.
  - [ ] Graceful shutdown path is wired and testable.

  **QA Scenarios**:
  ```
  Scenario: Server starts and exposes health endpoint
    Tool: Bash
    Steps: Start server in test harness, then run `curl -sS http://127.0.0.1:8080/healthz`
    Expected: Response is HTTP 200 with machine-readable healthy status
    Evidence: .sisyphus/evidence/task-4-server-healthz.txt

  Scenario: Invalid auth config fails closed on startup
    Tool: Bash
    Steps: Run `go test ./cmd/server/... -run TestServerStartupFailsWithoutJWTConfig -v`
    Expected: Test passes; startup returns deterministic configuration error
    Evidence: .sisyphus/evidence/task-4-server-auth-config.txt
  ```

  **Commit**: YES | Message: `feat(server): add bootstrap and config for api runtime` | Files: `cmd/server/**, internal/config/**, internal/server/**`

- [ ] T5. Implement authenticated REST endpoints for run, resume, and inspect

  **What to do**:
  1. Add Chi router + middleware stack with request ID, recovery, auth, and JSON error rendering.
  2. Implement `POST /api/v1/run` to validate request, create a server-generated session ID, persist initial session state, dispatch actor start, and return `202 Accepted` with session metadata.
  3. Implement `POST /api/v1/resume` to validate session eligibility, reject running sessions with `409`, reject missing sessions with `404`, and return `202 Accepted` when the actor is started.
  4. Implement `GET /api/v1/sessions/{id}/inspect` as the API equivalent of CLI inspect, reading persisted state without requiring an active actor.
  5. Standardize JSON error format across 400/401/404/409/429/500 responses.

  **Must NOT do**: Do not expose unauthenticated inspect. Do not return 200 for asynchronous run/resume dispatch.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: API semantics must align with runtime/persistence truth.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T7,T10,T11 | Blocked By: T2,T3,T4

  **References**:
  - Pattern: `cmd/agent/cli.go` - Current run/resume/inspect flows.
  - Pattern: `internal/store/session_store.go` - Session persistence authority.
  - External: `https://github.com/go-chi/chi` - Routing and middleware composition.

  **Acceptance Criteria**:
  - [ ] Authenticated run/resume/inspect routes exist and return documented status codes.
  - [ ] `POST /api/v1/run` always returns new server-generated session IDs.
  - [ ] `POST /api/v1/resume` rejects running/missing/completed sessions correctly.
  - [ ] Inspect works for completed sessions without active runtime state.

  **QA Scenarios**:
  ```
  Scenario: Authenticated run creates a session
    Tool: Bash
    Steps: Run `curl -sS -X POST http://127.0.0.1:8080/api/v1/run -H "Authorization: Bearer test-token" -H "Content-Type: application/json" --data "{\"task\":\"ping\",\"task_type\":\"general\"}"`
    Expected: HTTP 202; JSON contains non-empty `session_id`, `status`, and stream/inspect URLs
    Evidence: .sisyphus/evidence/task-5-run-endpoint.json

  Scenario: Unauthenticated inspect is rejected
    Tool: Bash
    Steps: Run `curl -i -sS http://127.0.0.1:8080/api/v1/sessions/test-session/inspect`
    Expected: HTTP 401 with standardized JSON error body
    Evidence: .sisyphus/evidence/task-5-inspect-auth-failure.txt
  ```

  **Commit**: YES | Message: `feat(server): add authenticated run resume inspect endpoints` | Files: `internal/server/**, cmd/server/**`

- [ ] T6. Harden SQLite persistence and session lifecycle semantics for server concurrency

  **What to do**:
  1. Enable and verify WAL mode in server path.
  2. Validate concurrent write behavior under multiple active sessions.
  3. Ensure inspect remains readable while sessions are active and after actor shutdown.
  4. Persist enough lifecycle state to distinguish queued/running/completed/failed/cancelled/resumable conditions.
  5. Prevent stale resume eligibility by storing and checking terminal/active state transitions atomically where needed.

  **Must NOT do**: Do not assume CLI-era single-writer behavior is sufficient. Do not require in-memory actor presence for inspect truth.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: persistence correctness and concurrency behavior are failure-prone but narrowly scoped.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T7,T10,T11 | Blocked By: T3,T4

  **References**:
  - Pattern: `internal/store/session_store.go` - Current persistence model.
  - Reference: `README.md` - Recoverability/inspectability requirements.

  **Acceptance Criteria**:
  - [ ] WAL mode is enabled and validated in server startup/tests.
  - [ ] Concurrent session persistence tests pass without corrupting inspect output.
  - [ ] Resume eligibility uses persisted lifecycle state, not only in-memory assumptions.
  - [ ] Race and concurrency tests pass for store/session transitions.

  **QA Scenarios**:
  ```
  Scenario: Concurrent session writes remain inspectable
    Tool: Bash
    Steps: Run `go test ./internal/store/... -run TestSQLiteWALConcurrentSessions -v`
    Expected: Test passes; inspect reads consistent session/step data during concurrent writes
    Evidence: .sisyphus/evidence/task-6-sqlite-wal.txt

  Scenario: Resume eligibility is persisted correctly
    Tool: Bash
    Steps: Run `go test ./internal/store/... -run TestResumeEligibilityPersistsAcrossActorRestart -v`
    Expected: Test passes; completed/running sessions are not incorrectly resumable
    Evidence: .sisyphus/evidence/task-6-resume-persistence.txt
  ```

  **Commit**: YES | Message: `feat(store): harden sqlite session lifecycle for api concurrency` | Files: `internal/store/**, internal/server/**`

- [ ] T7. Implement SSE stream endpoint with ordered event relay and disconnect-safe behavior

  **What to do**:
  1. Implement `GET /api/v1/sessions/{id}/stream` using SSE only.
  2. Map the six runtime event types into SSE frames using `event: <type>` plus JSON `data:` payloads.
  3. Preserve in-session ordering by reading the engine `EventChannel` into an actor-owned relay.
  4. Use per-subscriber buffer size `256`; if a subscriber falls behind, emit terminal SSE `error` event explaining overflow and close only that subscriber connection while leaving the session running.
  5. Emit heartbeat comments every `15s` while the stream is open.
  6. Define reconnect semantics explicitly: no replay; reconnect only receives future events if the session is still active.

  **Must NOT do**: Do not silently drop externally visible SSE events. Do not cancel the session solely because one client disconnected.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: this translates internal runtime events into external contract guarantees.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T10,T11 | Blocked By: T3,T4,T5,T6

  **References**:
  - Pattern: `internal/domain/events.go` - Authoritative event taxonomy.
  - Pattern: `internal/runtime/emitter.go` - Current event channel semantics.
  - Pattern: `cmd/agent/cli.go` - Existing stream consumer behavior.
  - Reference: `docs/ADR-006-streaming-architecture.md` - Streaming design baseline.

  **Acceptance Criteria**:
  - [ ] SSE endpoint streams ordered `token_delta`, `tool_start`, `tool_end`, `step_complete`, `error`, `session_complete` events.
  - [ ] Stream requires JWT auth.
  - [ ] Disconnect does not cancel session execution.
  - [ ] Slow-subscriber overflow is explicit and isolated to the affected stream connection.

  **QA Scenarios**:
  ```
  Scenario: SSE stream emits ordered runtime events
    Tool: Bash
    Steps: Run `curl -N -H "Authorization: Bearer test-token" http://127.0.0.1:8080/api/v1/sessions/test-session/stream`
    Expected: Stream contains ordered SSE frames with expected event names and terminal `session_complete`
    Evidence: .sisyphus/evidence/task-7-sse-stream.txt

  Scenario: Client disconnect does not stop session
    Tool: Bash
    Steps: Start stream, terminate client early, then call `curl -sS -H "Authorization: Bearer test-token" http://127.0.0.1:8080/api/v1/sessions/test-session/inspect`
    Expected: Inspect shows session continued and reached valid terminal or active state independent of stream disconnect
    Evidence: .sisyphus/evidence/task-7-sse-disconnect.txt
  ```

  **Commit**: YES | Message: `feat(server): add sse runtime streaming endpoint` | Files: `internal/server/**, internal/runtime/**`

- [ ] T8. Complete the real OpenAI provider implementation

  **What to do**:
  1. Replace OpenAI stub behavior with real HTTP-backed implementation matching the existing provider contract.
  2. Support both `Generate` and `Stream` paths.
  3. Normalize provider errors into deterministic runtime errors.
  4. Preserve existing config precedence for credentials/model/base URL.
  5. Add focused tests for auth failure, transport failure, malformed response, and streaming token delivery.

  **Must NOT do**: Do not introduce provider-specific behavior that leaks into runtime contracts. Do not require API use to depend on OpenAI being configured unless explicitly selected.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: provider correctness affects both CLI and server runtime behavior.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T10,T11 | Blocked By: T1,T2

  **References**:
  - Pattern: `internal/llm/provider.go` - Provider interface contract.
  - Pattern: Existing DashScope provider implementation in `internal/llm/**` - Real HTTP adapter reference.
  - Reference: `README.md` provider configuration examples.

  **Acceptance Criteria**:
  - [ ] OpenAI provider performs real HTTP generate requests.
  - [ ] OpenAI provider streaming emits token deltas through the standard event path.
  - [ ] Stub-only behavior is removed or replaced behind explicit fake/test-only code.
  - [ ] Error normalization tests pass.

  **QA Scenarios**:
  ```
  Scenario: OpenAI generate path works through provider contract
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestOpenAIProviderGenerate -v`
    Expected: Test passes against mocked HTTP transport with expected request/response mapping
    Evidence: .sisyphus/evidence/task-8-openai-generate.txt

  Scenario: OpenAI streaming path emits token deltas
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestOpenAIProviderStream -v`
    Expected: Test passes and verifies ordered token delta emission plus terminal completion behavior
    Evidence: .sisyphus/evidence/task-8-openai-stream.txt
  ```

  **Commit**: YES | Message: `feat(llm): complete openai provider implementation` | Files: `internal/llm/**, internal/config/**`

- [ ] T9. Complete the real Anthropic provider implementation

  **What to do**:
  1. Replace Anthropic stub behavior with real HTTP-backed implementation matching the existing provider contract.
  2. Support both `Generate` and `Stream` paths.
  3. Normalize provider errors into deterministic runtime errors.
  4. Preserve config precedence and selection semantics identical to OpenAI/DashScope.
  5. Add focused tests for auth failure, transport failure, malformed response, and streaming event handling.

  **Must NOT do**: Do not special-case Anthropic in runtime orchestration. Do not break existing provider selection flags/config.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: provider parity and streaming fidelity matter for server productization.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T10,T11 | Blocked By: T1,T2

  **References**:
  - Pattern: `internal/llm/provider.go` - Shared provider contract.
  - Pattern: Existing DashScope provider implementation in `internal/llm/**` - Real transport/reference behavior.
  - Reference: `README.md` provider configuration examples.

  **Acceptance Criteria**:
  - [ ] Anthropic provider performs real HTTP generate requests.
  - [ ] Anthropic provider streaming emits token deltas through the standard event path.
  - [ ] Stub-only behavior is removed or replaced behind explicit fake/test-only code.
  - [ ] Error normalization tests pass.

  **QA Scenarios**:
  ```
  Scenario: Anthropic generate path works through provider contract
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestAnthropicProviderGenerate -v`
    Expected: Test passes against mocked HTTP transport with expected request/response mapping
    Evidence: .sisyphus/evidence/task-9-anthropic-generate.txt

  Scenario: Anthropic streaming path emits token deltas
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestAnthropicProviderStream -v`
    Expected: Test passes and verifies ordered token delta emission plus terminal completion behavior
    Evidence: .sisyphus/evidence/task-9-anthropic-stream.txt
  ```

  **Commit**: YES | Message: `feat(llm): complete anthropic provider implementation` | Files: `internal/llm/**, internal/config/**`

- [x] T10. Run end-to-end integration for auth, concurrency, SSE, shutdown, and CLI compatibility

  **What to do**:
  1. Add end-to-end server tests covering authenticated run → stream → inspect flow.
  2. Add tests for 401, 404, 409, 429, and 500 path behavior.
  3. Add multi-session concurrency tests proving actors do not interfere.
  4. Add graceful shutdown tests proving new requests are rejected during shutdown and in-flight sessions are drained/cancelled per policy.
  5. Add CLI non-regression test coverage showing server additions do not alter existing `cmd/agent` behavior.

  **Must NOT do**: Do not rely on manual curl inspection as the only validation. Do not leave shutdown policy unverified.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: broad integration coverage across server/runtime/provider/store layers.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - No UI in scope.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T11 | Blocked By: T5,T6,T7,T8,T9

  **References**:
  - Pattern: Existing integration/replay tests under `internal/**` and `testdata/**`.
  - Reference: `docs/validation-matrix.md` - Evidence ledger format.

  **Acceptance Criteria**:
  - [ ] Full server integration suite passes.
  - [ ] CLI non-regression tests pass.
  - [ ] Graceful shutdown, auth failure, duplicate resume, and active-limit paths are all tested.
  - [ ] Validation matrix evidence can be updated from automated results.

  **QA Scenarios**:
  ```
  Scenario: End-to-end authenticated run/stream/inspect succeeds
    Tool: Bash
    Steps: Run `go test ./internal/server/... -run TestServerRunStreamInspectE2E -v`
    Expected: Test passes and verifies run dispatch, ordered SSE events, and persisted inspect state
    Evidence: .sisyphus/evidence/task-10-server-e2e.txt

  Scenario: Active session cap returns 429
    Tool: Bash
    Steps: Run `go test ./internal/server/... -run TestServerRejectsWhenSessionCapExceeded -v`
    Expected: Test passes and confirms deterministic HTTP 429 response with standardized JSON error body
    Evidence: .sisyphus/evidence/task-10-session-cap.txt
  ```

  **Commit**: YES | Message: `test(server): add end to end api and concurrency coverage` | Files: `internal/server/**, internal/runtime/**, docs/validation-matrix.md`

- [x] T11. Sync docs, progress truth, release-flow tasks, and executor handoff requirements

  **What to do**:
  1. Update `README.md`, `docs/USAGE.md`, `PROGRESS.md`, and `docs/validation-matrix.md` to reflect the new server surface and the fact that v3 is complete.
  2. Document API usage examples for run/resume/inspect/stream and auth setup.
  3. Document v4 non-goals, especially that Web UI remains v5.
  4. Document only v4 implementation-facing handoff requirements relevant to API server, provider completion, and verification.
  5. Keep release/operational steps out of the v4 plan unless they are part of the actual scoped deliverables.

  **Must NOT do**: Do not claim Web UI exists. Do not add release/operational instructions that are outside scoped v4 deliverables.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: truth-sync and execution handoff quality determine release safety.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: F1,F2,F3,F4 | Blocked By: T4,T5,T6,T7,T8,T9,T10

  **References**:
  - Pattern: `README.md` - Current project overview and quick-start structure.
  - Pattern: `PROGRESS.md` - Milestone truth source needing v3 completion correction.
  - Pattern: `docs/validation-matrix.md` - Evidence ledger format.

  **Acceptance Criteria**:
  - [ ] README/USAGE/PROGRESS/validation-matrix are mutually consistent.
  - [ ] v3 is marked complete everywhere.
  - [ ] v4 API routes/auth/examples are documented.
  - [ ] Documentation remains scoped to implemented v4 capabilities and verification requirements.

  **QA Scenarios**:
  ```
  Scenario: Documentation truth is consistent
    Tool: Bash
    Steps: Run `grep -n "v3\|v4\|/api/v1/run\|Web UI" README.md PROGRESS.md docs/USAGE.md docs/validation-matrix.md`
    Expected: v3 is complete, v4 API exists, and Web UI is only documented as deferred/non-goal
    Evidence: .sisyphus/evidence/task-11-doc-truth.txt

  Scenario: Documentation excludes out-of-scope release-operation notes
    Tool: Bash
    Steps: Run `grep -n "model switch\|push\|remote repository" README.md PROGRESS.md docs/USAGE.md docs/validation-matrix.md`
    Expected: No newly added v4 documentation text introduces out-of-scope release-operation requirements
    Evidence: .sisyphus/evidence/task-11-doc-scope.txt
  ```

  **Commit**: YES | Message: `docs: sync v4 api server progress and release guidance` | Files: `README.md, PROGRESS.md, docs/USAGE.md, docs/validation-matrix.md`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer task-level atomic commits following task completion.
- Required commit sequence should roughly align to: ADR → builder/session manager → server endpoints/SSE → provider completion → docs/progress.
- Final execution stage must include a release-oriented commit or final squash decision only if repository workflow explicitly prefers it.

## Success Criteria
- The repository builds and tests successfully with both CLI and server entrypoints.
- A client can start a task over HTTP, stream runtime events over SSE, and inspect the persisted session state afterward.
- Multiple sessions can run concurrently without cross-session state corruption.
- OpenAI and Anthropic are no longer stub providers for the supported v4 contract.
- Documentation truthfully shows v3 complete and v4 capabilities present.
