# Phase 3: General Task Protocol

## TL;DR
> **Summary**: Evolve the current harness from a coding-leaning agent loop into a general task protocol substrate by generalizing task classification, action contracts, verification contracts, and runtime orchestration while preserving current CLI and coding-flow behavior.
> **Deliverables**:
> - General task protocol domain model and runtime flow
> - Expanded action vocabulary and structured parsing/serialization updates
> - Task-aware verification contract replacing coding-only default assumptions
> - Static task-type registry/adapters for at least two non-coding task categories
> - Updated docs and continuation guidance for git-based multi-machine development
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: 1 → 2 → 4 → 6 → 8 → F1-F4

## Context
### Original Request
Plan the next phase after Phase 2 without drifting from the project goal. The user clarified the project is a **general agent harness inspired by Hermes/OpenClaw**, not a coding agent. The user approved prioritizing **general task abstraction / action protocol**.

### Interview Summary
- Project direction corrected from coding-agent optimization to general harness engineering.
- Phase 3 should optimize for **protocol core** rather than observability-first or verifier-first.
- Phase 3 should **expand the action surface now**, rather than keep only `respond` / `tool_call`.
- Phase 3 verification should become **task-aware**, rather than remain centered on `go test/build/vet`.
- Cross-machine continuation should be handled through **git + repo-tracked docs/artifacts**, not a dedicated migration project.

### Metis Review (gaps addressed)
- Added explicit guardrail to preserve current CLI/coding behavior while generalizing internals.
- Added static task-type registry/adapters and explicitly deferred plugin/dynamic loading work.
- Added explicit acceptance criteria for **two non-coding task types** to prevent over-generalization without proof.
- Added explicit handling for unknown task type, task-type mismatch, and verification-not-applicable flows.
- Kept `ToolCall.Input` as `string` in Phase 3 to avoid uncontrolled contract breakage; structured params remain convention-based until proven insufficient.

## Work Objectives
### Core Objective
Turn the current harness into a **general task protocol runtime** that can orchestrate more than coding tasks, while keeping the existing CLI/runtime/provider stack stable and backward-compatible for current flows.

### Deliverables
- A generalized task protocol in `internal/domain` with explicit task-type and verification-policy semantics.
- An expanded `Action` contract and parser/prompt flow that supports non-coding-oriented steps.
- A task-aware verifier boundary and initial verifier implementations for distinct task categories.
- Runtime orchestration changes so planning, acting, observing, and verifying no longer assume coding as the default task class.
- Updated CLI/docs/progress artifacts that document continuation across machines using repo-tracked files only.

### Definition of Done (verifiable conditions with commands)
- `go test ./...` passes after all protocol/domain/runtime changes.
- `go test -race ./...` passes.
- `go build ./...` passes.
- At least two non-coding task categories have deterministic end-to-end tests proving runtime + verifier compatibility.
- Existing coding-oriented regression tests continue to pass without CLI flag changes.

### Must Have
- Preserve single-process, single-agent harness architecture.
- Preserve current CLI subcommands (`run`, `resume`, `inspect`) and their current user-facing contract.
- Make general-task support concrete via at least **two** non-coding task categories.
- Keep task-type integration static/compile-time; no plugin system.
- Keep git-based continuation explicit in docs; do not depend on `.sisyphus/boulder.json` for handoff.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- Must NOT introduce a plugin system, dynamic tool loading, or multi-agent orchestration.
- Must NOT make coding-specific verification the default fallback for every task type.
- Must NOT change `ToolCall.Input` away from `string` in Phase 3.
- Must NOT rely on machine-local session state (`.sisyphus/boulder.json`) as required continuation input.
- Must NOT add vague “future extensibility” abstractions without a concrete task-type consumer in this phase.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: **tests-after** using Go testing framework already present in repo.
- QA policy: Every task includes agent-executed scenarios. Use unit/integration tests and CLI execution where appropriate.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.

Wave 1: domain protocol foundation (`1-3`) + verifier contract foundation (`4`) + docs baseline (`5`)

Wave 2: runtime/prompt/action integration (`6-8`) + CLI/persistence compatibility (`9`)

Wave 3: concrete task-type proof, regressions, and docs closure (`10-12`)

### Dependency Matrix (full, all tasks)
- 1 blocks 2, 4, 6, 9, 10
- 2 blocks 6, 7, 10
- 3 blocks 6, 7, 10
- 4 blocks 8, 10, 11
- 5 blocks 12
- 6 blocks 7, 8, 10
- 7 blocks 10
- 8 blocks 10, 11
- 9 blocks 11
- 10 blocks 11, 12
- 11 blocks F1-F4
- 12 blocks F1-F4

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 5 tasks → `unspecified-high`, `writing`
- Wave 2 → 4 tasks → `unspecified-high`
- Wave 3 → 3 tasks → `unspecified-high`, `writing`

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [ ] 1. Add explicit general task typing to the domain model

  **What to do**: Introduce domain-level task typing and task protocol metadata so runtime and verification no longer infer “coding” implicitly from the task description. Add additive types/fields in `internal/domain` for task category, protocol hints, and verification policy reference. Keep existing task creation flow backward-compatible by defaulting unspecified tasks to a safe general category rather than a coding-only assumption.
  **Must NOT do**: Do not remove current fields from `Task`; do not introduce dynamic registration or plugin loading; do not change persisted data in a way that makes existing sessions unreadable without migration logic.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: core contract evolution with compatibility constraints
  - Skills: `[]`
  - Omitted: [`/playwright`] - no browser dependency

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [2,4,6,9,10] | Blocked By: []

  **References**:
  - Pattern: `internal/domain/ports.go:5-33` - current domain contract boundaries that must remain additive-first
  - Pattern: `internal/domain/session.go` - session model/status compatibility affected by new protocol metadata
  - Pattern: `cmd/agent/cli.go:221-260` - current task/session creation path that must keep working
  - Pattern: `internal/runtime/runtime.go:39-138` - runtime currently treats all tasks through one implicit flow
  - Pattern: `internal/store/session_store.go` - persisted task/session compatibility must remain readable for older data

  **Acceptance Criteria** (agent-executable only):
  - [ ] `internal/domain` exposes explicit task type / protocol metadata without removing existing task fields
  - [ ] Existing domain tests pass and new tests cover default behavior for unspecified task type
  - [ ] Existing sessions/tasks still build and load without requiring manual migration edits
  - [ ] Persistence changes are additive-only, with optional/defaulted fields so older stored sessions deserialize successfully

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Default task classification remains backward-compatible
    Tool: Bash
    Steps: Run `go test ./internal/domain ./cmd/agent`
    Expected: Tests pass; unspecified tasks follow default classification path without compile or decode errors
    Evidence: .sisyphus/evidence/task-1-domain-task-typing.txt

  Scenario: Invalid task type rejected or normalized deterministically
    Tool: Bash
    Steps: Run targeted tests covering unsupported/unknown task-type input
    Expected: Tests prove deterministic fallback or explicit validation error per implementation choice
    Evidence: .sisyphus/evidence/task-1-domain-task-typing-error.txt
  ```

  **Commit**: YES | Message: `feat(domain): add general task typing` | Files: [`internal/domain/*`, `cmd/agent/*test*`]

- [ ] 2. Expand the action contract beyond respond and tool_call

  **What to do**: Extend `internal/domain/action.go` and dependent parsing structures so the runtime can express a minimal general-task action vocabulary. The minimum expanded surface for Phase 3 is: `respond`, `tool_call`, `request_input`, and `complete`. Define each action’s semantics clearly and update domain tests accordingly. `complete` should signal task completion intent without overloading plain response text; `request_input` should represent blocked external input needs without pretending verification passed.
  **Must NOT do**: Do not add more action kinds than needed; do not add parallel/subtask/multi-agent actions; do not make actions task-type-specific enums.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: contract design with ripple effects into prompts/runtime
  - Skills: `[]`
  - Omitted: [`/playwright`] - protocol work only

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [6,7,10] | Blocked By: [1]

  **References**:
  - Pattern: `internal/domain/action.go:3-17` - current action surface is too narrow
  - Pattern: `internal/runtime/model_adapter.go:26-37` - current JSON action body assumes only respond/tool_call
  - Pattern: `internal/config/prompts/model_adapter.go:55-87` - prompt instructions currently enforce old shape

  **Acceptance Criteria** (agent-executable only):
  - [ ] Domain action contract supports `respond`, `tool_call`, `request_input`, and `complete`
  - [ ] Parsing/serialization tests prove deterministic handling of each action type
  - [ ] Existing respond/tool_call behavior remains unchanged for current coding tests

  **QA Scenarios**:
  ```
  Scenario: Expanded action types decode correctly
    Tool: Bash
    Steps: Run targeted tests for action parsing and runtime model adapter decoding
    Expected: All four action types parse into stable domain structures
    Evidence: .sisyphus/evidence/task-2-action-contract.txt

  Scenario: Unsupported action type fails safely
    Tool: Bash
    Steps: Run targeted tests with an unknown action type payload
    Expected: Runtime/model adapter returns explicit unsupported action error
    Evidence: .sisyphus/evidence/task-2-action-contract-error.txt
  ```

  **Commit**: YES | Message: `feat(domain): expand general action protocol` | Files: [`internal/domain/action.go`, `internal/runtime/model_adapter.go`, `internal/config/prompts/model_adapter.go`, tests]

- [ ] 3. Define a static task-type registry and protocol adapter boundary

  **What to do**: Add a compile-time task-type registry/lookup layer that maps task categories to protocol behavior, without introducing runtime plugins. The registry should define which verifier policy, prompting hints, and compatibility defaults apply to each supported task type. Include at least: `coding`, `research`, and `file_workflow` as initial categories, even if only two non-coding categories are fully exercised in tests.
  **Must NOT do**: Do not add filesystem-loaded manifests or plugin discovery; do not scatter task-type switches across runtime without a central registry boundary.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: architecture-shaping but still bounded
  - Skills: `[]`
  - Omitted: [`/playwright`] - not needed

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [6,7,10] | Blocked By: [1]

  **References**:
  - Pattern: `internal/llm/provider.go:23-63` - static provider selection is precedent for compile-time dispatch
  - Pattern: `internal/tools/registry.go:11-83` - existing registry pattern to mirror for static definitions
  - Pattern: `internal/runtime/runtime.go:140-188` - protocol-dependent behavior currently inlined, needs centralization

  **Acceptance Criteria** (agent-executable only):
  - [ ] Registry resolves supported task types to explicit protocol metadata
  - [ ] Unknown task type behavior is deterministic and covered by tests
  - [ ] No plugin/dynamic loading code is introduced

  **QA Scenarios**:
  ```
  Scenario: Registry resolves known task types
    Tool: Bash
    Steps: Run targeted tests for task-type lookup and defaults
    Expected: coding, research, and file_workflow map to explicit protocol definitions
    Evidence: .sisyphus/evidence/task-3-task-registry.txt

  Scenario: Unknown task type is handled safely
    Tool: Bash
    Steps: Run targeted tests for unsupported task-type lookup
    Expected: Registry returns documented fallback or explicit validation error
    Evidence: .sisyphus/evidence/task-3-task-registry-error.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add static task protocol registry` | Files: [`internal/runtime/*`, `internal/domain/*`, tests]

- [ ] 4. Replace coding-only verifier assumptions with a task-aware verification contract

  **What to do**: Refactor the verification boundary so verification policy is selected by task type or protocol metadata rather than assuming code commands are always relevant. Keep `CommandVerifier` as the coding-task implementation, but add a generalized contract that supports at minimum: command-based verification for coding, evidence-based verification for research, and state/output verification for file workflow. Ensure “verification not applicable yet” is representable without falsely marking success. The verifier dispatch must live outside CLI-only wiring so task-aware selection is available to runtime and tests through a central contract.
  **Must NOT do**: Do not remove `CommandVerifier`; do not build a verification DSL; do not hardcode new task types directly into CLI code.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: core behavioral generalization with compatibility risk
  - Skills: `[]`
  - Omitted: [`/playwright`] - CLI/runtime verification only

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [8,10,11] | Blocked By: [1]

  **References**:
  - Pattern: `internal/verify/command_verifier.go:15-143` - current coding-only verifier that must become one implementation among several
  - Pattern: `internal/domain/ports.go:30-33` - existing verifier boundary to evolve carefully
  - Pattern: `cmd/agent/cli.go:176-187` - current verifier factory is config-driven, not task-aware

  **Acceptance Criteria** (agent-executable only):
  - [ ] Verification selection depends on task type/protocol, not only config mode
  - [ ] Coding verification still uses command-backed verifier path
  - [ ] Research and file_workflow verification paths have deterministic tests

  **QA Scenarios**:
  ```
  Scenario: Coding tasks still run command verification
    Tool: Bash
    Steps: Run targeted verifier/runtime tests for coding task classification
    Expected: Verification dispatch selects command-based verifier and preserves current behavior
    Evidence: .sisyphus/evidence/task-4-task-aware-verifier.txt

  Scenario: Non-coding task does not execute irrelevant go commands
    Tool: Bash
    Steps: Run targeted tests for research/file_workflow verification
    Expected: Verification uses task-aware policy and does not call coding-only commands by default
    Evidence: .sisyphus/evidence/task-4-task-aware-verifier-error.txt
  ```

  **Commit**: YES | Message: `feat(verify): add task-aware verification contract` | Files: [`internal/verify/*`, `internal/domain/ports.go`, `cmd/agent/cli.go`, tests]

- [ ] 5. Update repo-tracked continuation docs for git-based cross-machine work

  **What to do**: Update repo-tracked documentation so planning and execution can continue on another machine using git alone. Document what is portable (`.sisyphus/plans`, `.sisyphus/notepads`, docs, progress files), what is machine-local (`.sisyphus/boulder.json`, local config secrets), and how to resume safely without hidden state assumptions. Update `README.md`, `PROGRESS.md`, and any relevant usage docs to reflect the project’s corrected goal as a general harness, not a coding-agent-only system.
  **Must NOT do**: Do not create a separate migration subsystem; do not document `.sisyphus/boulder.json` as required shared state; do not leave “coding agent” as the primary positioning text in top-level docs.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: documentation-heavy with architectural precision needed
  - Skills: `[]`
  - Omitted: [`/playwright`] - docs only

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [12] | Blocked By: []

  **References**:
  - Pattern: `PROGRESS.md:3-6` - current project description still says “通用 Coding Agent Go MVP” and must be corrected
  - Pattern: `PROGRESS.md:179-184` - current next-step guidance is coding-leaning and must be refreshed
  - Pattern: `cmd/agent/cli.go:25-31` - repo/local-path defaults relevant to continuation docs

  **Acceptance Criteria** (agent-executable only):
  - [ ] README/PROGRESS/usage docs consistently describe the project as a general agent harness
  - [ ] Docs explicitly distinguish repo-tracked continuation artifacts from machine-local state
  - [ ] New-machine continuation steps are documented and verifiable without hidden session state

  **QA Scenarios**:
  ```
  Scenario: Continuation docs are internally consistent
    Tool: Bash
    Steps: Run `go test ./...` and inspect updated docs in changed-files review
    Expected: Code still passes and docs consistently describe portable vs machine-local artifacts
    Evidence: .sisyphus/evidence/task-5-cross-machine-docs.txt

  Scenario: Old coding-agent-only wording removed from top-level guidance
    Tool: Bash
    Steps: Review updated README and PROGRESS entries referenced by tests or grep-based checks
    Expected: No contradictory top-level positioning remains in updated files
    Evidence: .sisyphus/evidence/task-5-cross-machine-docs-error.txt
  ```

  **Commit**: YES | Message: `docs: align project positioning and continuation guidance` | Files: [`README.md`, `PROGRESS.md`, `docs/*`]

- [ ] 6. Refactor runtime planning/execution flow around protocol metadata

  **What to do**: Update `internal/runtime/runtime.go` so plan creation, action selection, observation handling, and retry/termination logic consult protocol metadata instead of assuming one task shape. Preserve the single loop, but make task-type-specific behavior explicit and centralized. Ensure task-type mismatch, blocked-input actions, and completion actions produce coherent session status transitions. Define explicit status semantics in this phase: `request_input` transitions the session into a new blocked-input status (or the project’s documented equivalent additive status), while `complete` transitions through the normal successful completion path without pretending a tool executed.
  **Must NOT do**: Do not create multiple runtimes; do not add goroutine-based parallel task execution; do not scatter if/else task-type branches across unrelated packages.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: central execution-path refactor with failure-mode sensitivity
  - Skills: `[]`
  - Omitted: [`/playwright`] - no browser work

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [7,8,10] | Blocked By: [1,2,3]

  **References**:
  - Pattern: `internal/runtime/runtime.go:39-138` - existing loop to preserve structurally while generalizing decisions
  - Pattern: `internal/runtime/runtime.go:160-188` - current iteration path assumes only action/tool/result/verify sequence
  - Pattern: `internal/runtime/runtime.go:190-258` - memory recall logic should remain task-agnostic while integrating protocol context

  **Acceptance Criteria** (agent-executable only):
  - [ ] Runtime consults protocol/task metadata during iteration and termination decisions
  - [ ] `request_input` and `complete` actions map to deterministic session/step outcomes
  - [ ] Existing coding-task runtime tests still pass after refactor

  **QA Scenarios**:
  ```
  Scenario: Runtime handles multiple task protocols deterministically
    Tool: Bash
    Steps: Run targeted runtime tests covering coding, research, and file_workflow paths
    Expected: Runtime produces correct state transitions per protocol without branching chaos
    Evidence: .sisyphus/evidence/task-6-runtime-protocol.txt

  Scenario: request_input/complete do not masquerade as successful tool execution
    Tool: Bash
    Steps: Run targeted tests for blocked-input and explicit-complete paths
    Expected: Session status and verification outcomes follow documented semantics
    Evidence: .sisyphus/evidence/task-6-runtime-protocol-error.txt
  ```

  **Commit**: YES | Message: `refactor(runtime): drive loop from task protocol metadata` | Files: [`internal/runtime/runtime.go`, tests]

- [ ] 7. Update prompt builders and model adapter parsing for the general protocol

  **What to do**: Revise `internal/config/prompts/model_adapter.go` and `internal/runtime/model_adapter.go` so provider-facing payloads include task type/protocol context and the expanded action contract. Keep JSON-only prompting, but remove wording that implies coding-first behavior. Ensure the model adapter can decode `request_input` and `complete` consistently and preserve backward compatibility for existing provider output expectations. Unsupported future action kinds must fail with an explicit deterministic error rather than silently coercing to another action.
  **Must NOT do**: Do not switch to provider-native tool calling in Phase 3; do not remove current JSON contract discipline; do not make prompt payloads depend on machine-local state.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: prompt/adapter/contract synchronization work
  - Skills: `[]`
  - Omitted: [`/playwright`] - not applicable

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [10] | Blocked By: [2,3,6]

  **References**:
  - Pattern: `internal/config/prompts/model_adapter.go:13-205` - current payload builders and action JSON instructions to generalize
  - Pattern: `internal/runtime/model_adapter.go:21-149` - parsing logic tied to old action shape
  - Pattern: `internal/runtime/model_adapter.go:178-225` - provider adapter boundary to keep stable

  **Acceptance Criteria** (agent-executable only):
  - [ ] Prompt payloads include task/protocol context where needed
  - [ ] Model adapter decodes all supported action types
  - [ ] Existing provider tests continue to pass without native tool-calling changes

  **QA Scenarios**:
  ```
  Scenario: Prompt payloads encode general protocol context
    Tool: Bash
    Steps: Run targeted prompt-builder and model-adapter tests
    Expected: Payloads include task/protocol metadata and action contract instructions without coding-only assumptions
    Evidence: .sisyphus/evidence/task-7-prompt-protocol.txt

  Scenario: Legacy respond/tool_call payloads still parse
    Tool: Bash
    Steps: Run regression tests for existing provider/model adapter fixtures
    Expected: Old fixtures continue to pass while new action types are supported
    Evidence: .sisyphus/evidence/task-7-prompt-protocol-error.txt
  ```

  **Commit**: YES | Message: `feat(runtime): align prompt adapters with general protocol` | Files: [`internal/config/prompts/model_adapter.go`, `internal/runtime/model_adapter.go`, tests]

- [ ] 8. Implement initial non-coding verifier policies and evidence models

  **What to do**: Add the first concrete non-coding verification implementations required by the task-aware contract. For `research`, verification should evaluate structured evidence completeness/consistency rather than run code commands. For `file_workflow`, verification should validate expected file-state/result conditions through existing safe tools. Keep implementations deterministic and testable. Introduce the minimal domain evidence representation required for non-coding verification so research verification is not forced to overload code-command output semantics.
  **Must NOT do**: Do not add network-dependent verification flows; do not require human approval as a verification mechanism; do not add a generalized rules DSL.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: new verifier implementations with evidence semantics
  - Skills: `[]`
  - Omitted: [`/playwright`] - no browser requirement

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [10,11] | Blocked By: [4,6]

  **References**:
  - Pattern: `internal/verify/command_verifier.go:26-60` - current verification result pattern to preserve conceptually
  - Pattern: `internal/domain/tool.go:19-25` - existing normalized tool result shape available for evidence
  - Pattern: `internal/runtime/runtime.go:177-187` - verifier consumes observation after action execution

  **Acceptance Criteria** (agent-executable only):
  - [ ] Minimal domain evidence structure exists for non-coding verifier input and has deterministic tests
  - [ ] Research verifier has deterministic tests for pass/fail evidence evaluation
  - [ ] File workflow verifier has deterministic tests for pass/fail output/file-state checks
  - [ ] No coding-only commands run for non-coding verification paths by default

  **QA Scenarios**:
  ```
  Scenario: Research verification evaluates evidence without code commands
    Tool: Bash
    Steps: Run targeted verifier tests for research tasks
    Expected: Pass/fail is based on structured observation/evidence conditions, not go command execution
    Evidence: .sisyphus/evidence/task-8-noncoding-verifiers.txt

  Scenario: File workflow verification catches missing/incorrect file outcomes
    Tool: Bash
    Steps: Run targeted tests using safe tool-result fixtures for file_workflow tasks
    Expected: Verification fails deterministically when expected state/output is absent or malformed
    Evidence: .sisyphus/evidence/task-8-noncoding-verifiers-error.txt
  ```

  **Commit**: YES | Message: `feat(verify): add non-coding verifier policies` | Files: [`internal/verify/*`, tests]

- [ ] 9. Preserve CLI and persistence compatibility while surfacing general task controls

  **What to do**: Update `cmd/agent` and storage compatibility paths so Phase 3 protocol changes remain additive for current users. Surface task-type selection or protocol hints in a backward-compatible manner, ensure session persistence can store/reload new metadata, and keep current defaults safe for existing users who do not pass new flags.
  **Must NOT do**: Do not rename existing subcommands; do not require new flags for current coding-oriented flows; do not break resume/inspect on older stored sessions.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: CLI/store compatibility work across user-facing and persistence boundaries
  - Skills: `[]`
  - Omitted: [`/playwright`] - CLI only

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [11] | Blocked By: [1]

  **References**:
  - Pattern: `cmd/agent/cli.go:120-174` - current runtime wiring and backward-compatibility expectations
  - Pattern: `cmd/agent/cli.go:221-260` - current `run` command argument handling that new controls must not break
  - Pattern: `cmd/agent/cli.go:176-187` - verifier selection path to evolve without breaking current config behavior
  - Pattern: `internal/store/session_store.go` - persistence round-trip compatibility for old/new task metadata

  **Acceptance Criteria** (agent-executable only):
  - [ ] Current CLI commands still work without any new required flags
  - [ ] New task-type/protocol metadata persists and reloads deterministically
  - [ ] Resume/inspect tests cover old-session and new-session compatibility

  **QA Scenarios**:
  ```
  Scenario: Existing CLI usage still works unchanged
    Tool: Bash
    Steps: Run `go test ./cmd/agent ./internal/store`
    Expected: Existing run/resume/inspect tests pass with no required CLI changes
    Evidence: .sisyphus/evidence/task-9-cli-compat.txt

  Scenario: Legacy CLI invocation runs without task-type flag
    Tool: Bash
    Steps: Run a targeted CLI test or fixture equivalent to `go run ./cmd/agent run --task "test task"` without any new task-type flag
    Expected: Task defaults to the documented safe category and runtime proceeds without argument or decode failure
    Evidence: .sisyphus/evidence/task-9-cli-backcompat.txt

  Scenario: New protocol metadata does not break older session loads
    Tool: Bash
    Steps: Run targeted persistence compatibility tests with old/new fixtures
    Expected: Old sessions remain readable and new metadata round-trips correctly
    Evidence: .sisyphus/evidence/task-9-cli-compat-error.txt
  ```

  **Commit**: YES | Message: `feat(cli): preserve compatibility for general task protocol` | Files: [`cmd/agent/*`, `internal/store/*`, tests]

- [ ] 10. Prove the protocol with end-to-end non-coding task flows

  **What to do**: Add deterministic end-to-end tests/fixtures that prove the harness can execute at least two non-coding task categories under the new protocol. Required categories for this phase: `research` and `file_workflow`. Use fake/model fixtures where needed so the tests are deterministic and do not rely on external network access.
  **Must NOT do**: Do not use flaky live-provider tests as the proof of correctness; do not claim generality without these end-to-end demonstrations.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: cross-cutting validation of the phase goal
  - Skills: `[]`
  - Omitted: [`/playwright`] - not needed

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: [11,12] | Blocked By: [1,2,3,4,6,7,8]

  **References**:
  - Pattern: `internal/runtime/runtime.go:39-138` - end-to-end runtime loop under test
  - Pattern: `internal/runtime/model_adapter.go:50-176` - action/observe path that fixtures must exercise
  - Pattern: `internal/verify/command_verifier.go:26-60` - existing verifier style showing how deterministic pass/fail should remain explicit
  - Test: `testdata/` - fixture location for deterministic general-task scenarios and compatibility fixtures

  **Acceptance Criteria** (agent-executable only):
  - [ ] Research-task end-to-end test passes deterministically
  - [ ] File-workflow end-to-end test passes deterministically
  - [ ] Existing coding-task end-to-end/regression coverage still passes

  **QA Scenarios**:
  ```
  Scenario: Research task completes under general protocol
    Tool: Bash
    Steps: Run targeted end-to-end tests/fixtures for research tasks
    Expected: Session reaches correct terminal state with non-coding verification path
    Evidence: .sisyphus/evidence/task-10-e2e-general-tasks.txt

  Scenario: File workflow task fails cleanly on missing expected artifact
    Tool: Bash
    Steps: Run targeted negative end-to-end tests for file_workflow tasks
    Expected: Runtime and verifier return explicit failure without invoking irrelevant coding checks
    Evidence: .sisyphus/evidence/task-10-e2e-general-tasks-error.txt
  ```

  **Commit**: YES | Message: `test(runtime): prove general task protocol end to end` | Files: [`internal/runtime/*test*`, `cmd/agent/*test*`, `testdata/*`]

- [ ] 11. Run full regression and compatibility wave

  **What to do**: Execute the full test/build/race regression suite and add any missing targeted regressions discovered during Phase 3 work. This task exists to prove that protocol generalization did not regress current coding paths or CLI/persistence behavior.
  **Must NOT do**: Do not skip race coverage; do not treat partial package tests as sufficient final evidence.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: broad regression closure across repo
  - Skills: `[]`
  - Omitted: [`/playwright`] - CLI/runtime repository only

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: [F1-F4] | Blocked By: [4,8,9,10]

  **References**:
  - Test: `cmd/agent/main_test.go` - CLI regression expectations
  - Pattern: `internal/runtime/runtime.go:39-138` - central execution logic with highest regression risk
  - Pattern: `internal/verify/command_verifier.go:15-143` - compatibility of coding verifier path must remain intact

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] `go test -race ./...` passes
  - [ ] `go build ./...` passes

  **QA Scenarios**:
  ```
  Scenario: Full regression suite passes
    Tool: Bash
    Steps: Run `go test ./...`, `go test -race ./...`, and `go build ./...`
    Expected: All commands succeed with zero failing packages
    Evidence: .sisyphus/evidence/task-11-regression-wave.txt

  Scenario: Regression suite catches compatibility break if introduced
    Tool: Bash
    Steps: Run targeted negative/fixture regression checks added during Phase 3
    Expected: Tests fail deterministically when compatibility assumptions are broken
    Evidence: .sisyphus/evidence/task-11-regression-wave-error.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: [none]

- [ ] 12. Close documentation and operator guidance for Phase 3

  **What to do**: Finalize docs, progress notes, and contributor guidance so future work starts from the correct general-harness framing. Update “next steps” sections to reflect post-Phase-3 priorities and ensure contributors know that repo-tracked plans/notepads/docs are the handoff mechanism, while machine-local files remain non-authoritative.
  **Must NOT do**: Do not leave stale “coding agent” terminology in top-level files; do not document unfinished future phases as if implemented.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: final alignment of docs and contributor workflow
  - Skills: `[]`
  - Omitted: [`/playwright`] - docs only

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [F1-F4] | Blocked By: [5,10]

  **References**:
  - Pattern: `PROGRESS.md:1-196` - current progress narrative to update after Phase 3
  - Pattern: `cmd/agent/cli.go:120-174` - CLI contract docs must remain aligned with implementation
  - External: `.sisyphus/plans/make-agent-work.md` - previous phase completion context to supersede in progress docs

  **Acceptance Criteria** (agent-executable only):
  - [ ] Contributor-facing docs reflect general-harness positioning and Phase 3 capabilities accurately
  - [ ] Handoff/continuation guidance is explicit about portable vs machine-local artifacts
  - [ ] Documentation changes are consistent with implemented behavior and tested commands

  **QA Scenarios**:
  ```
  Scenario: Documentation matches verified implementation
    Tool: Bash
    Steps: Run regression commands and compare final docs against actual supported commands/flows
    Expected: No documented command or workflow contradicts implemented behavior
    Evidence: .sisyphus/evidence/task-12-doc-closure.txt

  Scenario: Handoff guidance excludes machine-local dependency
    Tool: Bash
    Steps: Review changed docs and grep for authoritative references to `.sisyphus/boulder.json`
    Expected: Docs do not require machine-local state for continuation
    Evidence: .sisyphus/evidence/task-12-doc-closure-error.txt
  ```

  **Commit**: YES | Message: `docs: finalize phase 3 general harness guidance` | Files: [`README.md`, `PROGRESS.md`, `docs/*`, `.sisyphus/notepads/*` if needed]

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- After F1-F4 complete, create a consolidated review summary artifact in repo-tracked planning context (for example under `.sisyphus/notepads/` or equivalent review notes) that records verdicts, fixes applied, and any remaining user-facing approval items.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer one commit per numbered task when the change is cohesive and independently verifiable.
- If tasks 6-8 require tightly coupled refactors, allow a combined commit only if regression evidence is captured before moving on.
- Keep docs-only tasks separate from runtime/domain changes.
- Do not commit machine-local config or `.sisyphus/boulder.json`.

## Success Criteria
- The harness no longer treats coding as the implicit default protocol for all tasks.
- At least two non-coding task categories are proven end-to-end under the same harness loop.
- Existing CLI and coding regressions continue to pass.
- Documentation and repo-tracked artifacts are sufficient for git-based continuation on another machine.
