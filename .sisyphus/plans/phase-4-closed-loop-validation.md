# Phase 4: Closed-Loop Validation

## TL;DR
> **Summary**: Prove that the completed Phase 3 general task protocol behaves as a trustworthy end-to-end harness by validating run/resume/inspect continuity, task-type routing, task-aware verification, evidence production, and regression stability. Permit only tightly scoped fixes that directly unblock or stabilize the validated loop.
> **Deliverables**:
> - Executable validation matrix covering CLI, runtime, verifier, persistence, and task-type protocol flows
> - Deterministic regression coverage for coding, research, and file_workflow paths
> - Evidence artifacts and commands that prove run/resume/inspect continuity and failure handling
> - Tightly scoped bug fixes required to make the validation loop pass
> - README / USAGE / PROGRESS alignment with the proven behavior and operator workflow
> **Effort**: Large
> **Parallel**: YES - 2 waves
> **Critical Path**: 1 → 2 → 5 → 6 → F1-F4

## Context
### Original Request
Phase 3 planning and implementation are complete. The user asked for the next step to be planned without drifting from the project goal. After reviewing repo goals and existing implementation, the selected direction is **closed-loop validation**. The user explicitly approved **validation + small fixes** rather than validation-only.

### Interview Summary
- The project remains a **general agent harness**, not a coding-only agent.
- The next plan must stay within the v1 mission: **CLI-first, single-process, single-agent, verifiable, recoverable, inspectable persistent-memory harness**.
- The next plan must not drift into plugins, multi-agent orchestration, web UI, vector DB, or broad platform work.
- The correct next move is to prove the current Phase 3 system forms a reliable closed loop before any new capabilities are added.
- Small fixes are allowed only when they directly unblock or stabilize validation.

### Metis Review (gaps addressed)
- Added an explicit **proof campaign** framing so this phase validates trustworthiness instead of expanding features.
- Added a hard **small-fix boundary**: only defects that block validation, regress proven flows, or make docs materially inaccurate may be fixed.
- Added explicit regression surfaces to protect: CLI entrypoints, runtime loop, task registry, verifier dispatch, persistence/resume, replay fixtures, and usage docs.
- Added failure-path and recovery-path requirements so the plan cannot overfit to a single happy path.
- Added an evidence-first acceptance model so all outcomes are agent-executable with zero human judgment.

## Work Objectives
### Core Objective
Demonstrate that the current harness can reliably execute and validate representative general-task workflows end-to-end — including `run`, `resume`, and `inspect` — while preserving Phase 3 protocol behavior and existing coding regressions.

### Deliverables
- A deterministic validation matrix for coding, research, and file_workflow task categories.
- Regression tests and replay fixtures that prove task-type routing and task-aware verification behavior.
- CLI-level proof that `run`, `resume`, and `inspect` preserve session continuity and inspectable state.
- Evidence artifacts for happy path, failure path, and recovery path scenarios.
- Minimal bug fixes required to make the closed loop pass.
- Documentation updated to match the validated operator workflow and verified behavior.

### Definition of Done (verifiable conditions with commands)
- `go test ./...` passes.
- `go test -race ./...` passes.
- `go test -cover ./...` passes.
- `go build ./...` passes.
- Targeted runtime replay and CLI integration tests prove successful `run`, resumed continuation, inspect-only reads, and failure/rejection handling.
- README / USAGE / PROGRESS describe the validated run/resume/inspect workflow and do not promise behavior disproven by tests.

### Must Have
- Preserve the existing single-process, single-agent CLI architecture.
- Reuse existing Go test, replay-fixture, and verifier infrastructure wherever possible.
- Validate all declared v1 task categories already present in the repo: `coding`, `research`, `file_workflow`.
- Include at least one happy path, one failure path, and one recovery/resume path with evidence.
- Treat small fixes as in-scope only when directly required to make validation pass or to align materially incorrect docs with proven behavior.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- Must NOT add new product capabilities, new task categories, plugin loading, multi-agent orchestration, web UI, or vector DB work.
- Must NOT perform broad refactors or protocol redesign unless a validation blocker proves the current implementation is broken.
- Must NOT introduce a second validation framework when existing Go tests, replay fixtures, and verifier/evidence paths are sufficient.
- Must NOT treat coding-only command verification as a universal fallback for non-coding tasks.
- Must NOT include cleanup-only edits that do not improve validation trustworthiness.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: **tests-after** using the existing Go testing, replay, CLI integration, and CI command set.
- QA policy: Every task includes agent-executed happy-path and failure/edge-path scenarios with explicit commands and evidence targets.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.

Wave 1: validation foundation and regression proof (`1-4`)

Wave 2: blocker fixes, documentation alignment, and acceptance closure (`5-7`)

### Dependency Matrix (full, all tasks)
- 1 blocks 2, 3, 4, 5, 6, 7
- 2 blocks 5, 7
- 3 blocks 5, 7
- 4 blocks 5, 6, 7
- 5 blocks 6, 7, F1-F4
- 6 blocks F1-F4
- 7 blocks F1-F4

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 4 tasks → `unspecified-high`, `writing`
- Wave 2 → 3 tasks → `unspecified-high`, `writing`

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Establish the closed-loop validation matrix and scenario inventory

  **What to do**: Create a single authoritative validation matrix that maps each required proof surface to concrete tests, replay fixtures, CLI commands, expected outcomes, and evidence files. The matrix must cover: `run`, `resume`, `inspect`, task-type routing (`coding`, `research`, `file_workflow`), verifier dispatch, rejection/failure handling, and documentation alignment checkpoints. Reuse existing test and fixture surfaces first; identify gaps explicitly before any fix work begins.
  **Must NOT do**: Do not add new task categories; do not invent a second evidence format; do not mark scenarios “manual” or leave assertions vague.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: validation inventory and acceptance-surface definition are documentation-heavy but must remain technically precise
  - Skills: `[]`
  - Omitted: [`/playwright`] - no browser surface exists in this repo

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [2,3,4,5,6,7] | Blocked By: []

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `README.md:29-49` - canonical basic test commands and CLI invocation examples
  - Pattern: `README.md:116-128` - expected `resume` and `inspect` invocation surface
  - Pattern: `README.md:192-203` - contributor workflow already expects test + CLI validation
  - Pattern: `PROGRESS.md:26-42` - Phase 3 delivered task-type and CLI continuity surfaces that must now be proven
  - Pattern: `docs/USAGE.md` - operator-facing contract that must match the matrix
  - Test: `internal/runtime/runtime_replay_test.go` - replay-driven validation baseline
  - Test: `cmd/agent/main_test.go` - CLI integration proof surface for persistent sessions

  **Acceptance Criteria** (agent-executable only):
  - [ ] A single validation matrix exists and maps every required proof surface to a command/test/fixture/evidence target
  - [ ] The matrix includes at least one happy path, one failure/rejection path, and one recovery/resume path
  - [ ] No required v1 task category or CLI command surface is left unassigned to a validation scenario

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Validation matrix covers all required proof surfaces
    Tool: Bash
    Steps: Run targeted tests/docs checks referenced by the matrix and confirm each listed surface has a concrete command or fixture backing it
    Expected: Matrix references only existing or explicitly planned validation surfaces with no unassigned gaps
    Evidence: .sisyphus/evidence/task-1-validation-matrix.txt

  Scenario: Missing validation surface is detected explicitly
    Tool: Bash
    Steps: Execute the targeted inventory checks and compare against required surfaces (run/resume/inspect, task-type routing, verifier dispatch, rejection path)
    Expected: Any missing proof surface is recorded as a concrete blocker rather than silently omitted
    Evidence: .sisyphus/evidence/task-1-validation-matrix-error.txt
  ```

  **Commit**: YES | Message: `docs(plan): define closed-loop validation matrix` | Files: [`docs/*`, `testdata/*`, test files as needed for matrix support]

- [x] 2. Prove CLI continuity across run, resume, and inspect

  **What to do**: Extend or tighten CLI integration coverage so `run`, `resume`, and `inspect` are proven to preserve session continuity, persistence, and inspectable state without relying on undocumented behavior. Validate both text/JSON contract expectations where already supported, and ensure session metadata required by Phase 3 task typing remains visible and stable across lifecycle transitions.
  **Must NOT do**: Do not redesign CLI UX; do not add new subcommands; do not weaken persistence assertions just to make tests pass.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: touches CLI contract, persistence expectations, and regression-sensitive session lifecycle behavior
  - Skills: `[]`
  - Omitted: [`/playwright`] - command-line only

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [5,7] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `cmd/agent/cli.go` - source of truth for `run`, `resume`, and `inspect` command behavior
  - Pattern: `internal/store/session.go` - persistence/resume path used by CLI lifecycle
  - Pattern: `README.md:45-49` - `run` example users will follow
  - Pattern: `README.md:116-126` - `resume` / `inspect` examples promised in docs
  - Test: `cmd/agent/main_test.go` - existing CLI integration tests to extend rather than replace
  - Test: `internal/store/sqlite_session_test.go` - persistence assertions to mirror for CLI-level behavior

  **Acceptance Criteria** (agent-executable only):
  - [ ] CLI integration tests prove a persisted session is created by `run`, reused by `resume`, and readable by `inspect`
  - [ ] Tests assert stable output or JSON fields for session continuity, not just zero exit codes
  - [ ] Task-type/session metadata relevant to Phase 3 survives persistence and lifecycle transitions

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: CLI run/resume/inspect lifecycle passes end-to-end
    Tool: Bash
    Steps: Run `go test ./cmd/agent -run "TestRunCommandJSONCreatesPersistentSession|TestResumeAndInspectOutput|TestRunCommandInterruptPersistsInterruptedSession" -count=1`
    Expected: All CLI lifecycle tests pass and prove persistent session continuity
    Evidence: .sisyphus/evidence/task-2-cli-lifecycle.txt

  Scenario: Inspect-only path does not mutate or silently heal invalid state
    Tool: Bash
    Steps: Run targeted CLI/store tests that exercise interrupted or resumed sessions and inspect behavior
    Expected: Inspect reads persisted state deterministically; lifecycle failures produce explicit assertions instead of silent success
    Evidence: .sisyphus/evidence/task-2-cli-lifecycle-error.txt
  ```

  **Commit**: YES | Message: `test(cli): prove run resume inspect continuity` | Files: [`cmd/agent/*`, `internal/store/*`, tests]

- [x] 3. Prove task-type routing and task-aware verifier dispatch

  **What to do**: Strengthen regression coverage so the harness proves correct protocol routing and verification behavior for `coding`, `research`, and `file_workflow`. This includes explicit assertions that coding uses command-backed verification, while non-coding task types use their intended policies without leaking into coding-only fallback behavior. Cover unknown/rejected task-type behavior only if already supported by the current registry contract.
  **Must NOT do**: Do not add a new verification DSL; do not collapse task-specific assertions into one generic smoke test; do not silently accept coding fallback for non-coding paths.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: protocol and verifier dispatch are Phase 3’s most critical regression surfaces
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [5,7] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/runtime/task_registry.go` - task-type registry contract to preserve
  - Pattern: `internal/verify/task_aware_verifier.go` - verification policy dispatch logic
  - Pattern: `internal/verify/verifier.go` - evidence/test/build/lint policy semantics
  - Pattern: `README.md:3-7` - publicly claimed support for coding/research/file-workflow
  - Test: `internal/runtime/task_registry_test.go` - existing task-category routing patterns
  - Test: `internal/verify/verify_test.go` - base verifier expectations
  - Test: `internal/verify/command_verifier_test.go` - command-backed verification proof surface

  **Acceptance Criteria** (agent-executable only):
  - [ ] Regression tests prove `coding`, `research`, and `file_workflow` resolve to explicit protocol/verification behavior
  - [ ] Non-coding paths are proven not to depend on coding-only command verification by default
  - [ ] Any supported unknown-category behavior is deterministic and covered by tests

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Task-type routing and verifier dispatch pass for all supported categories
    Tool: Bash
    Steps: Run `go test ./internal/runtime ./internal/verify -run "TestTaskRegistry|TestTaskAware|TestCommandVerifier|TestVerify" -count=1`
    Expected: Supported categories map to explicit verification behavior with passing regression assertions
    Evidence: .sisyphus/evidence/task-3-task-routing.txt

  Scenario: Non-coding task does not regress into coding-only fallback
    Tool: Bash
    Steps: Run targeted verifier/runtime tests covering research and file_workflow scenarios
    Expected: Tests fail if non-coding flows attempt irrelevant command-based fallback or unsupported implicit behavior
    Evidence: .sisyphus/evidence/task-3-task-routing-error.txt
  ```

  **Commit**: YES | Message: `test(verify): lock task-aware routing behavior` | Files: [`internal/runtime/*`, `internal/verify/*`, tests]

- [x] 4. Expand replay coverage for happy path, rejection path, and resume recovery

  **What to do**: Use the existing JSON replay fixture system to prove representative closed-loop outcomes across success, verification rejection, unsafe tool rejection, resumed continuation, research flow, and file workflow. Add or refine fixtures only when current fixtures do not assert the required behavior strongly enough. Ensure replay assertions produce deterministic expectations that can be rerun in CI.
  **Must NOT do**: Do not replace replay fixtures with ad hoc mocks where fixture coverage already exists; do not add flaky time- or environment-dependent expectations.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: fixture-driven runtime validation is central to closed-loop proof and regression protection
  - Skills: `[]`
  - Omitted: [`/playwright`] - replay and CLI only

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [5,6,7] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Test: `internal/runtime/runtime_replay_test.go` - replay harness to extend
  - Testdata: `testdata/runtime/success_session.json` - success baseline fixture
  - Testdata: `testdata/runtime/resume_session.json` - resumed continuation fixture
  - Testdata: `testdata/runtime/verification_reject.json` - failed verification path
  - Testdata: `testdata/runtime/research_session.json` - research task fixture
  - Testdata: `testdata/runtime/file_workflow_session.json` - file workflow fixture
  - Testdata: `testdata/runtime/unsafe_tool_rejection.json` - safety rejection fixture
  - Pattern: `internal/runtime/runtime.go` - runtime outcomes fixture assertions must reflect

  **Acceptance Criteria** (agent-executable only):
  - [ ] Replay coverage includes at least one success, one rejection/failure, and one resume-recovery path
  - [ ] Replay fixtures assert deterministic outcomes for research and file_workflow, not only coding-like flows
  - [ ] Replay tests are stable under standard CI commands and do not require manual setup beyond repo fixtures

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Replay fixtures prove representative closed-loop outcomes
    Tool: Bash
    Steps: Run `go test ./internal/runtime -run TestRuntimeReplay -count=1`
    Expected: Replay tests pass for success, rejection, resume, research, file workflow, and unsafe tool rejection fixtures
    Evidence: .sisyphus/evidence/task-4-replay-coverage.txt

  Scenario: Replay harness fails deterministically on broken expectations
    Tool: Bash
    Steps: Run targeted replay tests that cover verification rejection and unsafe tool rejection fixtures
    Expected: Negative-path assertions remain explicit and do not collapse into generic success
    Evidence: .sisyphus/evidence/task-4-replay-coverage-error.txt
  ```

  **Commit**: YES | Message: `test(runtime): expand replay validation coverage` | Files: [`internal/runtime/*`, `testdata/runtime/*`]

- [x] 5. Apply tightly scoped blocker fixes discovered by validation and re-prove the loop

  **What to do**: Implement only the minimum code or fixture changes required to fix validation blockers uncovered by Tasks 1-4. Each fix must be tied to a specific failing scenario, maintain current architecture boundaries, and add or update regression assertions proving the issue is resolved. Re-run the exact failing checks after every fix and keep changes localized to the affected path.
  **Must NOT do**: Do not batch unrelated cleanups; do not redesign protocol boundaries; do not “improve” unaffected areas; do not expand the fix set beyond direct blockers.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: fix work is implementation-heavy and may span runtime, verifier, store, or CLI, but must remain tightly bounded by evidence
  - Skills: `[]`
  - Omitted: [`/playwright`] - no browser work

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [6,7,F1-F4] | Blocked By: [1,2,3,4]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `cmd/agent/cli.go` - CLI blocker fixes must preserve command contracts
  - Pattern: `internal/runtime/runtime.go` - runtime lifecycle blocker fixes must preserve bounded execution semantics
  - Pattern: `internal/runtime/task_registry.go` - task-type fixes must not reintroduce protocol drift
  - Pattern: `internal/verify/task_aware_verifier.go` - verifier fixes must remain task-aware
  - Pattern: `internal/store/session.go` - persistence fixes must preserve resume/inspect semantics
  - Test: `cmd/agent/main_test.go` - CLI regressions to update and rerun
  - Test: `internal/runtime/runtime_replay_test.go` - replay regressions to update and rerun
  - Test: `internal/runtime/runtime_test.go` - unit-level lifecycle guardrails
  - Test: `internal/store/sqlite_session_test.go` - persistence guardrails

  **Acceptance Criteria** (agent-executable only):
  - [ ] Every fix maps to a previously failing validation scenario and is covered by a regression assertion
  - [ ] No fix introduces scope expansion beyond the directly affected validation blocker
  - [ ] All previously failing targeted checks pass after the associated fix is applied

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Blocker fix resolves the exact failing validation path
    Tool: Bash
    Steps: Re-run the previously failing targeted tests and CLI/replay checks identified in Tasks 2-4 after each blocker fix
    Expected: Each fixed blocker now passes with regression coverage proving the resolution
    Evidence: .sisyphus/evidence/task-5-blocker-fixes.txt

  Scenario: Fix does not cause unrelated regression
    Tool: Bash
    Steps: Run the nearest-surface regression suite for each fix plus `go test ./...`
    Expected: The blocker path is fixed without creating unrelated failures in adjacent runtime/CLI/verifier/store surfaces
    Evidence: .sisyphus/evidence/task-5-blocker-fixes-error.txt
  ```

  **Commit**: YES | Message: `fix(validation): resolve closed-loop blockers` | Files: [affected runtime/verify/store/cli/test paths only]

- [x] 6. Align operator documentation with the validated workflow

  **What to do**: Update README, USAGE, and PROGRESS only where validation proves current text is incomplete, inaccurate, or missing essential operator guidance. The updated docs must describe the validated `run`, `resume`, `inspect`, test, and evidence workflow in a way that matches actual behavior and avoids overpromising unsupported paths. Include any newly required validation commands or caveats uncovered during blocker fixing.
  **Must NOT do**: Do not rewrite docs for style only; do not document speculative future phases; do not claim capabilities that are not covered by validation evidence.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: documentation alignment is the primary deliverable, but must be grounded in executed validation results
  - Skills: `[]`
  - Omitted: [`/playwright`] - documentation-only surface

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [F1-F4] | Blocked By: [1,4,5]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `README.md` - top-level operator promise and contributor workflow
  - Pattern: `docs/USAGE.md` - CLI usage contract and examples
  - Pattern: `PROGRESS.md` - project status and continuation guidance
  - Pattern: `.github/workflows/ci.yml` - CI command baseline docs must not contradict
  - Test/Evidence: outputs from Tasks 2-5 - docs must reflect validated behavior only

  **Acceptance Criteria** (agent-executable only):
  - [ ] README / USAGE / PROGRESS no longer contradict validated CLI and verification behavior
  - [ ] Docs describe the actual validated commands and lifecycle expectations users should follow
  - [ ] No doc text promises unsupported task types, platform features, or unvalidated workflows

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Documentation matches validated workflow
    Tool: Bash
    Steps: Compare documented commands and lifecycle descriptions against the passing validation/test surfaces from Tasks 2-5
    Expected: Docs reference only proven commands, expected flows, and supported task categories
    Evidence: .sisyphus/evidence/task-6-doc-alignment.txt

  Scenario: Documentation drift is caught explicitly
    Tool: Bash
    Steps: Check README, USAGE, and PROGRESS for stale claims about CLI, verification, or task-type behavior after validation changes
    Expected: Any stale or unsupported claim is removed or corrected; no contradiction remains between docs and tests
    Evidence: .sisyphus/evidence/task-6-doc-alignment-error.txt
  ```

  **Commit**: YES | Message: `docs: align workflow with validated behavior` | Files: [`README.md`, `docs/USAGE.md`, `PROGRESS.md`]

- [x] 7. Run full acceptance sweep and publish validation evidence bundle

  **What to do**: Execute the full acceptance sweep using the repo’s standard CI-equivalent commands plus the targeted runtime/CLI validation commands from this phase. Produce and organize evidence artifacts so a future executor can trace each proven surface back to a command, fixture, and expected result. Ensure the final evidence set distinguishes baseline suite success from targeted closed-loop proof scenarios.
  **Must NOT do**: Do not skip targeted commands because `go test ./...` passes; do not leave evidence implicit in terminal history only; do not treat partial acceptance as complete.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: final validation requires broad execution discipline across test, build, and evidence surfaces
  - Skills: `[]`
  - Omitted: [`/playwright`] - no browser surface exists

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [F1-F4] | Blocked By: [1,2,3,4,5]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `Makefile:3-16` - standard local validation command set
  - Pattern: `.github/workflows/ci.yml:33-52` - CI-equivalent acceptance baseline
  - Test: `cmd/agent/main_test.go` - targeted CLI lifecycle proof
  - Test: `internal/runtime/runtime_replay_test.go` - targeted replay proof
  - Test: `internal/runtime/runtime_test.go` - runtime guardrails
  - Test: `internal/verify/verify_test.go` and `internal/verify/command_verifier_test.go` - verifier proof surface
  - Evidence: `.sisyphus/evidence/task-{1..7}-*.txt` - expected output archive pattern for this phase

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go build ./...`, `go test ./...`, `go test -race ./...`, and `go test -cover ./...` all pass
  - [ ] Targeted CLI and replay validation commands from this phase pass and produce evidence artifacts
  - [ ] Evidence files are sufficient to trace every major proof surface back to a command and expected outcome

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Full acceptance sweep passes
    Tool: Bash
    Steps: Run `go build ./...`; run `go test ./...`; run `go test -race ./...`; run `go test -cover ./...`; run targeted CLI/runtime/verifier commands defined in Tasks 2-4
    Expected: All baseline and targeted acceptance commands pass without manual intervention
    Evidence: .sisyphus/evidence/task-7-full-acceptance.txt

  Scenario: Acceptance sweep catches incomplete validation
    Tool: Bash
    Steps: Verify evidence set includes baseline suite results plus targeted closed-loop scenario outputs
    Expected: Missing targeted proof or missing evidence is treated as failure, even if the broad test suite passes
    Evidence: .sisyphus/evidence/task-7-full-acceptance-error.txt
  ```

  **Commit**: YES | Message: `test(validation): finalize closed-loop acceptance sweep` | Files: [tests, fixtures, docs, and evidence-related tracked paths as applicable]

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle (validation-matrix.md updated to validated state)
- [x] F2. Code Quality Review — unspecified-high (all quality checks passed)
- [x] F3. Real Manual QA — unspecified-high (all tests pass; --help exit 1 is standard Go behavior)
- [x] F4. Scope Fidelity Check — deep (plan file modification is orchestrator responsibility)

## Commit Strategy
- Prefer one commit per major validated surface when changes are substantial and logically separable.
- If validation reveals only a few small blockers, combine the blocker fix with its associated regression additions in a single commit.
- Do not commit pure evidence artifacts unless the repo already tracks them intentionally.

## Success Criteria
- The harness can be shown, via reproducible commands and tests, to preserve inspectable state across `run`, `resume`, and `inspect`.
- Task-type routing and verifier dispatch are proven for `coding`, `research`, and `file_workflow` without coding-only fallback leakage.
- Failure and rejection scenarios produce deterministic, inspectable outcomes rather than silent success.
- Any fixes introduced are minimal, justified by validation evidence, and protected by regression coverage.
- Documentation matches the actual validated behavior and operator workflow.
