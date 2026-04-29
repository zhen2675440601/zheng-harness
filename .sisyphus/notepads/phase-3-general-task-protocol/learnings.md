# Phase 3: General Task Protocol - Notepad

## Inherited Wisdom from Phase 1-2

### Domain Architecture Principles
- Domain types must NOT import infrastructure (store, tools, runtime)
- Use explicit structs, avoid map[string]any
- Keep ports interfaces narrow and testable

### Runtime Patterns
- Single-agent plan-execute-verify loop
- Explicit terminal states: success, verification_failed, budget_exceeded, fatal_error, interrupted
- Budget limits enforced with tested step/timeout

### Verification Philosophy
- Evidence-based, not model-confidence-based
- Verification must be independent runtime responsibility
- CommandVerifier is coding-only implementation

### CLI/Config Backward Compatibility
- CLI commands must remain unchanged for existing users
- New flags/features must have safe defaults
- Persistence must handle old session data

## Phase 3 Key Guardrails
- Keep ToolCall.Input as string (no contract breakage)
- No plugin/dynamic loading in Phase 3
- Task-type integration must be static/compile-time
- Keep git-based continuation explicit in docs

## Current Focus
Wave 1: Domain protocol foundation
- Task 1: Add general task typing to domain
- Task 2: Expand action contract (respond, tool_call, request_input, complete)
- Task 3: Static task-type registry
- Task 4: Task-aware verification contract
- Task 5: Cross-machine continuation docs

## Task 1 Learnings
- `domain.Task` can preserve backward compatibility by normalizing additive metadata during JSON marshal/unmarshal, so older persisted payloads with missing fields still deserialize to `general`.
- Unsupported task categories should normalize deterministically to `general` at the domain boundary instead of leaking unknown values downstream.

## Task 5 Learnings (Cross-Machine Documentation)
- Portable state (.sisyphus/plans/, .sisyphus/notepads/, docs/, PROGRESS.md) should be git-tracked
- Machine-local state (.sisyphus/boulder.json, zheng.json, *.db) must remain untracked
- Safe resumption requires explicit documentation of what NOT to copy between machines
- README and PROGRESS.md should consistently describe project as "general agent harness" not "coding agent"
- Git-based continuation workflow should have numbered steps with clear do's and don'ts

## Task 3 Learnings
- A runtime-facing static registry can mirror the tools registry pattern while keeping task-type branching centralized behind a single Resolve boundary instead of scattering switches across engine/prompt code.
- Protocol compatibility defaults belong in registry metadata so older tasks can inherit stable `ProtocolHint` and `VerificationPolicy` values without dynamic manifests or plugin discovery.

## Task 2 Learnings
- Keep `domain.ActionType` additive and string-backed so new general protocol actions such as `request_input` and `complete` preserve existing `respond`/`tool_call` behavior.
- Action semantics belong in domain comments: `request_input` signals blocked external input, while `complete` signals completion intent rather than tool execution.
- Verification results can stay backward-compatible by adding a normalized Status field and deriving legacy pass/fail semantics from it during JSON marshal/unmarshal.
- A central TaskAwareVerifier keeps CLI wiring additive while moving task-type and protocol-based verification selection out of config-only assumptions.
- "Verification not applicable yet" should be modeled as a first-class verification status so runtime can distinguish pending evidence from hard failure without inventing a DSL.

## Task 6 Learnings
- Runtime protocol resolution works best when done once at session start, then threaded through plan creation and iteration helpers so task-type-specific behavior stays centralized inside the single loop.
- `request_input` and `complete` should short-circuit verifier dispatch in runtime: they are protocol-level control actions with deterministic session outcomes, not ordinary evidence-producing steps.
- Adding an additive `blocked_input` session state keeps blocked external-input flows explicit without conflating them with interruption or verification failure.

## Task 8 Learnings
- Non-coding verification benefits from a small typed evidence model attached to `domain.Observation`, letting research and file-workflow checks stay deterministic without introducing a generalized DSL.
- Task-aware verifier dispatch should keep command execution isolated to coding/command policies; research and file-workflow policies can verify completion purely from structured evidence and observed file-state results.

## Task 7 Learnings
- Non-coding verification benefits from a small typed evidence model attached to `domain.Observation`, letting research and file-workflow checks stay deterministic without introducing a generalized DSL.
- Task-aware verifier dispatch should keep command execution isolated to coding/command policies; research and file-workflow policies can verify completion purely from structured evidence and observed file-state results.

## Task 9 Learnings
- CLI task-type controls can stay backward-compatible by making task metadata flags fully optional and persisting only additive metadata, leaving default coding-oriented flows untouched when flags are omitted.
- Persisting general task metadata in sessions.config_json avoids schema churn while giving resume/inspect a deterministic source of truth; older sessions can safely fall back to plan summary plus normalized general task defaults.
- Runtime replay fixtures can prove non-coding end-to-end behavior by carrying task category plus structured evidence payloads through the same engine loop used by coding fixtures.
- A fixture-level verifier selector keeps deterministic replay coverage flexible: coding regressions can stay on fake verifier responses while research/file-workflow fixtures exercise real task-aware verifier dispatch without network access.

## Task 12 Learnings (Phase 3 Documentation Closure)
- Top-level documentation must consistently use "general-purpose agent harness" or "通用 Agent Harness" terminology, avoiding "coding agent" which incorrectly narrows project scope.
- Phase 3 completion status should be explicit in README.md and PROGRESS.md, listing all Wave 1-3 tasks as completed with checkmarks.
- Handoff guidance requires clear separation between git-tracked portable artifacts (plans, notepads, docs, PROGRESS.md) and machine-local state (boulder.json, zheng.json, *.db files).
- Evidence files (.sisyphus/evidence/) provide auditable proof of task completion with dated records of changes made and verification checklists.

## Task 11 Learnings
- Final regression-wave evidence should explicitly distinguish "test failure" from "tooling unavailable" so downstream verification can block cleanly instead of misclassifying environment setup issues as product regressions.
- When Go is missing, evidence should still capture attempted `go test ./...`, `go test -race ./...`, and `go build ./...` commands plus the discovered test-file surface, so a follow-up machine with Go installed can reproduce the full verification wave exactly.

## F4 Scope Fidelity Check Learnings
- `internal/domain/task.go` keeps task protocol metadata additive and typed (`TaskCategory`, strings for protocol fields) with no `map[string]any` escape hatch introduced into the task contract.
- `internal/domain/action.go` remains narrowly scoped to action vocabulary expansion; out-of-scope runtime capabilities such as plugin hooks, dynamic loaders, or multi-agent actions were not introduced there.
- `internal/runtime/task_registry.go` uses a compile-time `staticTaskProtocolMetadata` map plus deterministic fallback, which preserves the Phase 3 guardrail that task-type integration stays static rather than plugin-driven.
- `internal/domain/tool.go` still defines `ToolCall.Input` as `string`, so Phase 3 preserved the explicit no-contract-breakage constraint for tool invocation input.
- No internal packages or filenames indicate plugin, dynamic-loading, or vector-database additions, which aligns with the README and Phase 3 plan guardrails.

## Task 8 Interactive Tool Learnings
- Interactive tool adapters can stay testable by injecting `io.Reader`/`io.Writer` in the constructor while executor wiring supplies `os.Stdin`/`os.Stdout` at the composition boundary.
- For prompt tools with timeout semantics, a context-aware read wrapper can translate `context.Canceled` and `context.DeadlineExceeded` into a stable user-facing error (`user did not respond in time`) without coupling domain logic to terminal implementations.
- Multiple-choice CLI prompts should centralize numbered-option validation and cap retries deterministically so tests can assert both successful selection and exhaustion behavior.

## Task 9 Code Search Tool Learnings
- A dedicated code search adapter can stay aligned with the existing grep adapter by keeping regex matching line-oriented while layering language-to-extension filtering and directory/file exclusion before file reads.
- `code_search` content mode is easiest to keep deterministic by merging overlapping ±2-line context windows per file and emitting `path:line: text` records for every line in the merged window.
- Treating `max_results` as a cap on returned result units (`files_with_matches` files, `count` files, `content` context blocks) allows early walk termination without changing the existing `ToolResult` contract.

## T10 Streaming CLI Output Learnings
- CLI streaming can stay backward-compatible by adding an explicit `--stream` flag and routing through injected `runEngine`/`runStreamEngine` helpers, keeping the default `Run()` path unchanged and easy to regression-test.
- For `--stream --json`, the simplest stable contract is JSONL of raw `domain.StreamingEvent` values, while normal streamed output can remain human-readable and event-type-specific.
- Runtime `SessionComplete` events currently expose the runtime session id, so the CLI must normalize streamed completion payloads back to the persisted user-facing session id before printing or encoding JSONL.
