# General-Purpose Coding Agent with Harness Engineering (Go MVP)

## TL;DR
> **Summary**: Build a CLI-first, single-process, general-purpose coding agent in Go that applies Harness Engineering through explicit context assembly, tool control, verification/self-correction, and inspectable persistent memory.
> **Deliverables**:
> - Go project skeleton with clean domain/runtime/infrastructure/interface boundaries
> - Single-agent plan-execute-verify runtime for coding tasks
> - Built-in tool layer, safety constraints, SQLite-backed session/memory persistence
> - TDD test suite, replayable traces, and agent-executed verification workflow
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: 1 → 2 → 3 → 4 → 7

## Context
### Original Request
基于 Harness Engineering 思想，做一套合理的开发方案，目标是实现一个通用 Agent（类 Hermes）。

### Interview Summary
- 项目当前处于 0→1 探索阶段
- 团队规模：1-2 人
- 技术诉求：技术深度优先
- 目标形态：通用 Agent 的首个版本聚焦为 **Coding Agent**
- 首期入口：**CLI Only**
- 技术栈：**后端 Go**，前端可在后续阶段考虑 TypeScript
- 测试策略：**TDD**
- MVP 必备能力：**工具调用、计划-执行循环、验证/自纠正、持久记忆**

### Metis Review (gaps addressed)
- 增加了明确的 v1 禁止项，防止过早平台化
- 为终止条件、重试预算、CLI 契约、SQLite 持久化、工具边界补充了明确约束
- 将“持久记忆”限定为可检查、可追溯、可压缩的 SQLite 事实/摘要存储，避免过早引入向量库与知识图谱
- 为每个任务补充了 agent 可执行的验收标准与失败场景

## Work Objectives
### Core Objective
交付一个 **可运行、可测试、可恢复、可验证** 的 Go 版通用 Coding Agent MVP，使其体现 Harness Engineering 的核心价值：**Constrain、Inform、Verify、Correct**，同时避免一次性复刻完整 Hermes。

### Deliverables
- `go.mod` 与 Go 项目目录骨架
- `cmd/agent` CLI 入口
- `internal/domain` 核心契约与端口接口
- `internal/runtime` 单代理计划-执行-验证循环
- `internal/tools` 工具注册与执行边界
- `internal/verify` 验证与自纠正机制
- `internal/store` SQLite session / event / memory / artifact 存储
- `internal/memory` 受限持久记忆策略
- 测试套件、回放样例、基准任务、CI 基线

### Definition of Done (verifiable conditions with commands)
- `go test ./...` 成功
- `go test -race ./...` 成功
- `go test ./... -cover` 生成覆盖率结果
- `go run ./cmd/agent run --task "inspect repository and propose next step"` 能完成一次完整会话
- `go run ./cmd/agent resume --session <id>` 能恢复先前会话
- CLI 在任务失败时输出明确错误与下一步原因，而非静默失败
- SQLite 中存在可检查的 session、event、memory 记录

### Must Have
- 单进程、单代理、CLI-first
- 明确的 domain / runtime / infrastructure / interface 分层
- 工具调用必须有注册表、schema、超时、风险级别
- 验证必须是独立运行时职责，不能只藏在 prompt 中
- 持久记忆必须可追溯（带 scope/type/source/confidence）
- TDD：先写失败测试，再实现

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- 不实现多代理编排
- 不实现插件系统
- 不实现前端 Web UI
- 不实现多平台网关（Slack/Telegram/Discord 等）
- 不实现向量数据库、知识图谱、自动泛化学习系统
- 不把策略逻辑硬编码进 tool adapter
- 不用 `map[string]any` 作为核心内部数据模型
- 不在 v1 中为“通用性”过早抽象为复杂平台

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: **TDD + Go testing framework**
- QA policy: Every task includes executable happy-path and failure-path scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.

Wave 1: foundation/contracts/testing (`1, 2, 5, 6`)
Wave 2: runtime/tooling/persistence (`3, 4, 8, 9`)
Wave 3: reliability/benchmarks/CLI polish (`7, 10, 11`)

### Dependency Matrix (full, all tasks)
- 1 blocks 2, 3, 4, 5, 6, 8, 9, 10, 11
- 2 blocks 3, 7, 10
- 3 blocks 4, 7, 10
- 4 blocks 7, 10
- 5 blocks 3, 4, 8
- 6 blocks 3, 4, 8, 9
- 8 blocks 9, 10
- 9 blocks 10
- 10 blocks 11

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 4 tasks → quick / unspecified-high
- Wave 2 → 4 tasks → unspecified-high / deep
- Wave 3 → 3 tasks → unspecified-high / deep / writing

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Bootstrap Go project skeleton and architectural boundaries

  **What to do**: Initialize the Go module, create the top-level directory layout (`cmd/agent`, `internal/domain`, `internal/runtime`, `internal/tools`, `internal/verify`, `internal/store`, `internal/memory`, `internal/config`, `testdata/`), and add architecture guardrails that prevent domain → infrastructure coupling. Define package ownership in docs/comments close to the package roots.
  **Must NOT do**: Do not add frontend code, plugin interfaces, multi-agent packages, or remote gateway code.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: greenfield multi-package setup with architecture boundaries
  - Skills: `[]` - no special skill required
  - Omitted: [`/frontend-ui-ux`, `/playwright`] - not relevant for CLI Go skeleton

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [2,3,4,5,6,8,9,10,11] | Blocked By: []

  **References**:
  - Local: `.sisyphus/drafts/general-agent-harness.md` - confirmed scope, stack, and MVP decisions
  - External: `https://hermes-agent.nousresearch.com/docs/developer-guide/architecture` - inspiration for separating runtime concerns without copying full Hermes scope
  - External: `https://martinfowler.com/articles/harness-engineering.html` - harness concepts to preserve in architecture

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` runs without package-cycle errors after skeleton creation
  - [ ] No package under `internal/domain` imports infrastructure-specific dependencies
  - [ ] Directory layout exists exactly as specified

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Project skeleton compiles
    Tool: Bash
    Steps: run `go test ./...`
    Expected: command succeeds and discovers module/packages without cycle/import errors
    Evidence: .sisyphus/evidence/task-1-bootstrap.txt

  Scenario: Forbidden early scope absent
    Tool: Bash
    Steps: inspect created directories; verify no `web`, `frontend`, `plugin`, `gateway`, or `multiagent` packages were added
    Expected: none of the forbidden v1 packages exist
    Evidence: .sisyphus/evidence/task-1-bootstrap-error.txt
  ```

  **Commit**: YES | Message: `feat(core): bootstrap go agent project skeleton` | Files: [`go.mod`, `cmd/agent/**`, `internal/**`, `testdata/**`]

- [x] 2. Define core domain contracts and deterministic runtime test harness

  **What to do**: Create strongly typed domain models for `Task`, `Session`, `Plan`, `Step`, `Action`, `ToolCall`, `ToolResult`, `Observation`, `VerificationResult`, and port interfaces for `Model`, `ToolExecutor`, `MemoryStore`, `SessionStore`, and `Verifier`. Add fake adapters and deterministic fixtures to support TDD for the agent loop.
  **Must NOT do**: Do not leak JSON blobs or raw SDK response types into domain packages.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: contract design determines long-term maintainability
  - Skills: `[]` - no special skill required
  - Omitted: [`/refactor`] - premature before baseline exists

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [3,7,10] | Blocked By: [1]

  **References**:
  - Local: `.sisyphus/drafts/general-agent-harness.md` - MVP capability decisions
  - External: `https://hermes-agent.nousresearch.com/docs/` - general agent capability envelope

  **Acceptance Criteria** (agent-executable only):
  - [ ] Unit tests cover fake-model + fake-tool deterministic loop inputs/outputs
  - [ ] Internal core types use explicit structs rather than `map[string]any`
  - [ ] Runtime packages compile against interfaces, not concrete adapters

  **QA Scenarios**:
  ```
  Scenario: Deterministic fake runtime test passes
    Tool: Bash
    Steps: run `go test ./... -run TestRuntimeWithFakes`
    Expected: fake model/tool/session interactions pass deterministically
    Evidence: .sisyphus/evidence/task-2-contracts.txt

  Scenario: Raw provider types blocked from domain
    Tool: Bash
    Steps: run package-level tests/static checks added for domain imports
    Expected: tests fail if SDK/provider types leak into domain packages
    Evidence: .sisyphus/evidence/task-2-contracts-error.txt
  ```

  **Commit**: YES | Message: `feat(domain): define agent contracts and fake adapters` | Files: [`internal/domain/**`, `internal/runtime/**`, `*_test.go`]

- [x] 3. Implement the single-agent plan-execute-verify loop

  **What to do**: Build the runtime loop for one session: ingest task, create/refresh plan, choose next action, execute tool or respond, normalize observation, run verification, self-correct on failure, stop on success/budget exhaustion. Add step budgets, max iterations, session timeout, and explicit termination reasons.
  **Must NOT do**: Do not add concurrent multi-agent delegation, graph orchestration, or hidden retry loops with no counters.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: core runtime logic with failure/retry semantics
  - Skills: `[]`
  - Omitted: [`/git-master`] - no git work needed for implementation design

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [4,7,10] | Blocked By: [1,2,5,6]

  **References**:
  - Local: `.sisyphus/drafts/general-agent-harness.md` - agreed MVP loop and stack
  - External: `https://www.anthropic.com/engineering/building-effective-agents` - practical loop patterns
  - External: `https://martinfowler.com/articles/harness-engineering.html` - verify/correct separation principles

  **Acceptance Criteria** (agent-executable only):
  - [ ] Runtime exits with explicit terminal states: success, verification_failed, budget_exceeded, fatal_error, interrupted
  - [ ] Every iteration records plan/action/observation/verification outcome
  - [ ] Infinite loop protection is enforced by tested step/timeout limits

  **QA Scenarios**:
  ```
  Scenario: Runtime completes a successful task
    Tool: Bash
    Steps: run `go test ./... -run TestRuntimeCompletesSuccessfulSession`
    Expected: session reaches success with recorded steps and verification
    Evidence: .sisyphus/evidence/task-3-runtime.txt

  Scenario: Runtime stops on exhausted budget
    Tool: Bash
    Steps: run `go test ./... -run TestRuntimeStopsOnBudgetExceeded`
    Expected: session stops with `budget_exceeded` and no unbounded retry
    Evidence: .sisyphus/evidence/task-3-runtime-error.txt
  ```

  **Commit**: YES | Message: `feat(runtime): add single-agent execution loop` | Files: [`internal/runtime/**`, `*_test.go`]

- [x] 4. Build the tool registry, safety metadata, and execution adapters

  **What to do**: Implement a built-in tool registry with typed tool definitions, JSON schema or equivalent argument contracts, default timeouts, safety levels, and execution adapters for the minimum coding-agent toolset: directory listing, file read, file write/edit, search, and command execution. Enforce allowlists and command/file-scope limits appropriate for local CLI use.
  **Must NOT do**: Do not introduce dynamic plugin loading, network-heavy tool suites, or unrestricted shell execution.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: combines API design, safety policy, and local execution
  - Skills: `[]`
  - Omitted: [`/playwright`] - CLI-only MVP

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [7,10] | Blocked By: [1,3,5,6]

  **References**:
  - External: `https://hermes-agent.nousresearch.com/docs/developer-guide/architecture` - registry/runtime separation inspiration
  - External: `https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents` - tool and harness safety patterns

  **Acceptance Criteria** (agent-executable only):
  - [ ] Registry lists tool name, description, schema, timeout, safety level, and executor binding
  - [ ] Unsafe command/file operations are rejected with explicit reasons
  - [ ] Tool execution results are normalized for runtime consumption

  **QA Scenarios**:
  ```
  Scenario: Allowed tool executes successfully
    Tool: Bash
    Steps: run `go test ./... -run TestAllowedToolExecution`
    Expected: a permitted tool call succeeds and returns normalized output
    Evidence: .sisyphus/evidence/task-4-tools.txt

  Scenario: Forbidden shell or path is blocked
    Tool: Bash
    Steps: run `go test ./... -run TestForbiddenToolExecution`
    Expected: command/path is denied with an explicit policy error
    Evidence: .sisyphus/evidence/task-4-tools-error.txt
  ```

  **Commit**: YES | Message: `feat(tools): add registry and safe execution adapters` | Files: [`internal/tools/**`, `*_test.go`]

- [x] 5. Establish TDD, lint, and CI baseline for a greenfield Go project

  **What to do**: Set up project-wide test commands, formatting/linting rules, race checks, coverage generation, and CI workflow so that every later task can rely on automated verification. Define repository conventions for naming, package ownership, and test placement.
  **Must NOT do**: Do not add heavyweight release automation or deployment workflows in MVP.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: tooling baseline with limited architectural ambiguity
  - Skills: `[]`
  - Omitted: [`/review-work`] - final verification already handled in plan

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [3,4,8] | Blocked By: [1]

  **References**:
  - Local: `.sisyphus/drafts/general-agent-harness.md` - confirmed TDD decision

  **Acceptance Criteria** (agent-executable only):
  - [ ] CI runs format/lint/test/race/coverage commands successfully
  - [ ] Local contributor flow is documented by executable commands in repo scripts or README-equivalent
  - [ ] Failing tests cause CI failure

  **QA Scenarios**:
  ```
  Scenario: CI-equivalent checks pass locally
    Tool: Bash
    Steps: run the same local commands used by CI for fmt/lint/test/race/coverage
    Expected: all checks succeed on clean branch state
    Evidence: .sisyphus/evidence/task-5-ci.txt

  Scenario: Deliberate failing test breaks pipeline
    Tool: Bash
    Steps: add or simulate a failing test in a controlled branch state and run CI-equivalent command
    Expected: pipeline exits non-zero and reports failing test
    Evidence: .sisyphus/evidence/task-5-ci-error.txt
  ```

  **Commit**: YES | Message: `chore(ci): establish tdd and verification baseline` | Files: [`.github/workflows/**`, `Makefile` or equivalents, config files]

- [x] 6. Add configuration, model adapter boundary, and prompt-policy versioning

  **What to do**: Create the configuration system for model/provider selection, step budgets, timeouts, memory limits, verification mode, and CLI defaults. Implement a provider boundary package so model SDK details stay outside domain/runtime. Version the prompt/policy assets or embedded templates explicitly.
  **Must NOT do**: Do not spread prompt strings across arbitrary files or let provider SDK types leak across packages.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: config and provider boundaries affect every subsystem
  - Skills: `[]`
  - Omitted: [`/refactor`] - initial shape must be deliberate from the start

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [3,4,8,9] | Blocked By: [1]

  **References**:
  - External: `https://www.anthropic.com/engineering/building-effective-agents` - policy/context assembly guidance

  **Acceptance Criteria** (agent-executable only):
  - [ ] Runtime can switch provider/model through config without domain changes
  - [ ] Prompt/policy templates are versioned and loaded from a single controlled path
  - [ ] Invalid config fails fast with actionable errors

  **QA Scenarios**:
  ```
  Scenario: Valid config loads and drives runtime
    Tool: Bash
    Steps: run `go test ./... -run TestValidConfigAndProviderBoundary`
    Expected: configuration loads, provider adapter is selected, and runtime starts cleanly
    Evidence: .sisyphus/evidence/task-6-config.txt

  Scenario: Invalid config is rejected early
    Tool: Bash
    Steps: run `go test ./... -run TestInvalidConfigFailsFast`
    Expected: startup fails with explicit validation errors before runtime execution
    Evidence: .sisyphus/evidence/task-6-config-error.txt
  ```

  **Commit**: YES | Message: `feat(config): add provider boundary and policy versioning` | Files: [`internal/config/**`, `internal/llm/**`, policy assets, `*_test.go`]

- [x] 7. Implement explicit verification and self-correction policy

  **What to do**: Build a verifier subsystem that inspects claims of completion, chooses verification actions (tests/build/lint/structural checks), compares evidence to the task goal, and returns correction instructions when verification fails. Include retry budgets, failure taxonomy, and stop conditions.
  **Must NOT do**: Do not let the model declare success without evidence or trigger unlimited re-verification loops.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: this is the core Harness value layer
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI verification needed in v1

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [11] | Blocked By: [2,3,4]

  **References**:
  - External: `https://martinfowler.com/articles/harness-engineering.html` - verify/correct concepts
  - External: `https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents` - long-running verification patterns

  **Acceptance Criteria** (agent-executable only):
  - [ ] Claimed success requires verification evidence
  - [ ] Verification failures produce bounded corrective actions and retry counts
  - [ ] Contradictions between agent claim and tool evidence are surfaced explicitly

  **QA Scenarios**:
  ```
  Scenario: Verification confirms correct completion
    Tool: Bash
    Steps: run `go test ./... -run TestVerifierAcceptsProvenSuccess`
    Expected: verified session transitions to success with recorded evidence
    Evidence: .sisyphus/evidence/task-7-verify.txt

  Scenario: Verification rejects false completion and triggers correction
    Tool: Bash
    Steps: run `go test ./... -run TestVerifierRejectsFalseSuccess`
    Expected: verifier marks failure, emits corrective next step, and enforces retry budget
    Evidence: .sisyphus/evidence/task-7-verify-error.txt
  ```

  **Commit**: YES | Message: `feat(verify): add evidence-based self-correction` | Files: [`internal/verify/**`, `internal/runtime/**`, `*_test.go`]

- [x] 8. Implement SQLite-backed session persistence and constrained memory

  **What to do**: Add SQLite repositories for `sessions`, `events/steps`, `artifacts`, and `memory_entries`. Implement memory scopes (`session`, `project`, `global`) and memory types (`preference`, `fact`, `summary`) with provenance, confidence, timestamps, and controlled write/read rules.
  **Must NOT do**: Do not add vector search, autonomous memory writing from every observation, or opaque serialized blobs with no provenance.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: durable state and memory policy shape product behavior
  - Skills: `[]`
  - Omitted: [`/git-master`] - persistence work does not require git specialization

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [9,10] | Blocked By: [1,5,6]

  **References**:
  - External: `https://hermes-agent.nousresearch.com/docs/` - persistent memory inspiration, but MVP deliberately reduced in scope

  **Acceptance Criteria** (agent-executable only):
  - [ ] Session resume works from SQLite-backed persisted state
  - [ ] Memory entries are inspectable with scope/type/source/confidence
  - [ ] Storage behavior is covered by tests for create/read/update/resume flows

  **QA Scenarios**:
  ```
  Scenario: Session persists and resumes correctly
    Tool: Bash
    Steps: run `go test ./... -run TestSessionPersistenceAndResume`
    Expected: session state and steps are restored from SQLite correctly
    Evidence: .sisyphus/evidence/task-8-memory.txt

  Scenario: Memory write policy blocks invalid entries
    Tool: Bash
    Steps: run `go test ./... -run TestMemoryPolicyRejectsInvalidEntry`
    Expected: invalid memory entry or missing provenance is rejected with explicit error
    Evidence: .sisyphus/evidence/task-8-memory-error.txt
  ```

  **Commit**: YES | Message: `feat(store): add sqlite session and memory persistence` | Files: [`internal/store/**`, `internal/memory/**`, schema files, `*_test.go`]

- [ ] 9. Add CLI commands for run, resume, inspect, and interrupt-safe persistence

  **What to do**: Implement the CLI contract for `run`, `resume`, and `inspect`, including clear task input, machine-readable and human-readable output modes, interrupt handling (`Ctrl+C` / signal capture), and persistence before shutdown when safe.
  **Must NOT do**: Do not add web server modes, chat UI, or channel integrations.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: interface layer on top of established runtime/store
  - Skills: `[]`
  - Omitted: [`/frontend-ui-ux`, `/dev-browser`] - not applicable

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [10] | Blocked By: [6,8]

  **References**:
  - Local: `.sisyphus/drafts/general-agent-harness.md` - CLI-only decision

  **Acceptance Criteria** (agent-executable only):
  - [ ] `run` starts a task and returns a session identifier
  - [ ] `resume` resumes a persisted session successfully
  - [ ] `inspect` shows session status, terminal state, and key memory/step summaries
  - [ ] Interrupt handling persists recoverable state before exit

  **QA Scenarios**:
  ```
  Scenario: CLI run and resume work end-to-end
    Tool: Bash
    Steps: run CLI commands to start a session, capture session id, then resume it
    Expected: CLI starts/resumes session and outputs consistent state
    Evidence: .sisyphus/evidence/task-9-cli.txt

  Scenario: Interrupt causes safe persistence
    Tool: Bash
    Steps: start a long-running controlled session, send interrupt, then inspect persisted state
    Expected: CLI exits gracefully and recoverable session state remains stored
    Evidence: .sisyphus/evidence/task-9-cli-error.txt
  ```

  **Commit**: YES | Message: `feat(cli): add run resume inspect commands` | Files: [`cmd/agent/**`, `*_test.go`]

- [ ] 10. Create benchmark tasks, replay fixtures, and reliability regression suite

  **What to do**: Define a small but representative benchmark pack for coding-agent tasks, create replay fixtures from trace/session outputs, and add regression tests that guard against infinite loops, false completion, broken resume, and unsafe tool usage.
  **Must NOT do**: Do not chase large-scale benchmarking infrastructure or external leaderboard integration.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: combines evaluation design, replay, and regression quality gates
  - Skills: `[]`
  - Omitted: [`/playwright`] - CLI-only system

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [11] | Blocked By: [2,3,4,8,9]

  **References**:
  - External: `https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents` - reliability and harness evaluation mindset

  **Acceptance Criteria** (agent-executable only):
  - [ ] At least one benchmark exists for success path, verification failure, resume path, and unsafe-tool rejection
  - [ ] Replay fixtures can be executed in CI
  - [ ] Regressions fail deterministically when reliability guarantees break

  **QA Scenarios**:
  ```
  Scenario: Regression suite passes on stable implementation
    Tool: Bash
    Steps: run benchmark/replay/regression commands defined for the project
    Expected: all baseline reliability scenarios pass deterministically
    Evidence: .sisyphus/evidence/task-10-reliability.txt

  Scenario: Broken reliability guarantee is caught
    Tool: Bash
    Steps: simulate or inject a known regression (e.g. false success acceptance) and run regression suite
    Expected: suite fails with a clear signal showing which guarantee broke
    Evidence: .sisyphus/evidence/task-10-reliability-error.txt
  ```

  **Commit**: YES | Message: `test(runtime): add replay benchmarks and reliability regressions` | Files: [`testdata/**`, replay fixtures, `*_test.go`]

- [ ] 11. Document operating model, ADRs, and contributor workflow for v1

  **What to do**: Write implementation-facing docs covering system boundaries, CLI usage, architecture decision records (single-process, single-agent, SQLite memory, no plugin system, no vector DB), testing workflow, and how to extend tools in v1 without introducing a plugin architecture.
  **Must NOT do**: Do not write speculative v2 platform docs or marketing-oriented claims.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: documentation and ADR clarity for a greenfield project
  - Skills: `[]`
  - Omitted: [`/frontend-ui-ux`] - no UI docs needed

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [] | Blocked By: [7,10]

  **References**:
  - Local: `.sisyphus/drafts/general-agent-harness.md` - confirmed scope and decisions
  - External: `https://martinfowler.com/articles/harness-engineering.html` - conceptual framing

  **Acceptance Criteria** (agent-executable only):
  - [ ] ADRs exist for the major MVP decisions
  - [ ] A new contributor can follow documented commands to run tests and start the CLI agent
  - [ ] Documentation explicitly states what is out of scope for v1

  **QA Scenarios**:
  ```
  Scenario: Fresh contributor flow is executable
    Tool: Bash
    Steps: follow the documented setup/test/run commands on a clean environment
    Expected: commands work as documented without missing steps
    Evidence: .sisyphus/evidence/task-11-docs.txt

  Scenario: Scope boundaries are documented clearly
    Tool: Bash
    Steps: inspect ADRs/docs for explicit v1 exclusions and extension rules
    Expected: docs clearly prohibit frontend, plugins, multi-agent, vector DB, and gateway scope in v1
    Evidence: .sisyphus/evidence/task-11-docs-error.txt
  ```

  **Commit**: YES | Message: `docs(architecture): record v1 operating model and adrs` | Files: [`README*`, `docs/**`, `adr/**`]

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer one commit per numbered task when the task creates a coherent verification boundary
- Do not combine foundational architecture changes with later reliability/benchmark tasks in a single commit
- Keep commit messages aligned to intent: `feat(core)`, `feat(runtime)`, `feat(store)`, `feat(cli)`, `test(runtime)`, `docs(architecture)`, `chore(ci)`

## Success Criteria
- The repository contains a runnable Go-based CLI coding agent with explicit harness layers
- The MVP demonstrates Constrain, Inform, Verify, and Correct in code and tests
- Sessions can be started, interrupted, resumed, and inspected
- Completion claims are evidence-based, not model-confidence-based
- Persistent memory exists but remains constrained, inspectable, and provenance-backed
- The codebase stays intentionally small-scope and avoids premature platformization
