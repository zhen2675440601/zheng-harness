# v7 Internal Reliability Release

## TL;DR
> **Summary**: Consolidate v6’s chat-first Web UI and six-layer backend into a version that is dependable for sustained internal use by hardening failure paths, expanding TDD-backed regression coverage, and adding minimum viable diagnostics for faster debugging.
> **Deliverables**:
> - Reliability-focused failing-test-first coverage for chat/session/stream/runtime paths
> - Hardened session lifecycle, SSE streaming, and API/service error handling
> - Minimum viable diagnostics and operator-facing troubleshooting surface for internal use
> - Expanded Go + Playwright verification with evidence capture and race/coverage gates
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: 1 → 2 → 4 → 7 → F1-F4

## Context
### Original Request
- v6 已经完成，下一步需要判断应继续扩充能力，还是先规划别的方向。
- 已确认下一阶段不以“能力扩充”为主，而是先做产品化强化。
- 已确认 v7 的目标是“内部可稳定使用”，不是对外 polished 发布，也不是大规模新能力版本。
- 已确认测试策略采用 TDD：先补失败用例，再实施稳定性改进。

### Interview Summary
- 用户接受“先稳固、后扩张”的节奏：v7 先做内部稳定使用版，v8 再考虑能力扩展。
- v7 成功标准聚焦三类：回归闭环、排障能力、交互健壮性。
- 范围内允许少量开发者/运维辅助能力，但仅限于支撑内部稳定使用，不得演化为新产品能力。
- 范围外明确排除：长期记忆升级、插件生态扩展、重型多 agent 新编排、对外演示型 UX 打磨。

### Metis Review (gaps addressed)
- 防止“稳定性”变成无限兜底清单：v7 只覆盖 chat/session/stream/error/diagnostics 这条主链，不把所有历史模块都纳入大清洗。
- 防止 scope creep 到“新能力”：调试面板、诊断接口、回放能力仅做内部排障最小闭环，不做独立产品功能。
- 防止“只加日志不加验证”：每项稳定性改造必须先有失败测试，再有实现，再补证据。
- 防止误把“内部稳定使用”理解成“外部上线”：本计划不要求多租户、配额系统、完整指标平台、复杂权限域模型。
- 防止未验证假设：以现有 `go test` / `go test -race` / `go test -cover` / Playwright / smoke-test 作为主验证轨道，不另起新测试框架。

## Work Objectives
### Core Objective
将 v6 已完成的 chat-first Web UI + six-layer backend 打磨成一个可被团队持续内部使用的稳定版本：核心会话主流程可重复验证，异常路径可快速定位，流式与会话状态边界更稳，且所有新增行为均通过 TDD 和自动化验证闭环证明。

### Deliverables
- 针对 chat start / submit / resume / stream / session lifecycle / failure paths 的 TDD 失败用例与回归测试
- 会话生命周期、SSE 传输、服务层冲突/校验/恢复语义的稳定性增强
- 统一错误响应、最小诊断信息、panic/异常恢复后的可排障证据
- 内部使用所需的最小调试入口或诊断输出（仅限现有会话/流式链路）
- README / PROGRESS / validation matrix / evidence 的 v7 结果更新

### Definition of Done (verifiable conditions with commands)
- `go test ./...` 通过，且新增的 chat/service/runtime/api 失败路径测试全部稳定通过。
- `go test -race ./...` 通过，session manager / stream path 无新增并发问题。
- `go test -cover ./...` 通过，并产出包含 `internal/service`、`internal/server`、`internal/runtime` 包覆盖输出的证据文件；计划不要求覆盖率阈值提升百分比，但要求这些包出现在覆盖输出中且新增测试文件已纳入本次变更。
- `go test -tags=smoke ./internal/llm/...` 通过，确保可靠性改动未破坏既有 smoke 轨道。
- `npx playwright test --project=chromium`（在 `e2e/` 下）通过；若环境缺少 Node 依赖或 Chromium，先执行仓库现有依赖安装/浏览器安装步骤，再运行测试，并把安装日志一并存证。
- 断流、非法输入、无效 JWT、会话冲突、会话关闭五类失败场景中至少覆盖三类，且每类都必须有对应自动化证据文件，证据中可见预期 API 错误或 UI 反馈。
- 通过代码审查与测试引用证明六层边界未倒退：业务规则改动仅发生在 `internal/service/*`、`internal/runtime/*`、`internal/store/*`；`internal/server/api.go` 只承载 transport/middleware/error mapping，不新增业务编排分支。

### Must Have
- 继续沿用 `docs/ADR-005-tdd-first.md:1-16` 的 TDD-first 策略，不允许“先改再补测试”。
- 复用现有验证命令与基础设施：`Makefile:9-19`、`e2e/playwright.config.js`、`e2e/tests/web-ui.spec.js`。
- 以 `internal/service/chat.go`、`internal/server/api.go`、`internal/runtime/session_manager.go`、`internal/runtime/runtime.go` 为主链做可靠性增强。
- 允许增加最小诊断输出，但不得引入完整可观测平台（如外部 tracing/metrics 基建）。
- 所有新增验证必须能够在 agent 环境中自动执行并留下 `.sisyphus/evidence/` 证据。
- 若 `e2e/` 运行依赖未安装，必须在首个涉及 Playwright 的任务中显式完成依赖准备并留痕；不得假设浏览器或 node_modules 已预装。

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- 不新增与内部稳定使用无关的净新产品能力。
- 不重写 UI 技术栈，不引入独立 SPA 重构，不改变同源嵌入部署模型。
- 不发起全仓库式“统一风格重构”或“顺手清理所有错误处理”。
- 不把 v7 目标扩展为生产级 SRE 平台建设、多租户权限系统或插件生态升级。
- 不接受缺少失败场景验证、只验证 happy path 的“稳定性改动”。

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: **TDD** + existing Go test / Playwright framework
- QA policy: Every task includes at least one happy path and one failure/edge path scenario
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`
- Commands baseline:
  - `go test ./...`
  - `go test -race ./...`
  - `go test -cover ./...`
  - `go test -tags=smoke ./internal/llm/...`
  - `cd e2e && npx playwright test --project=chromium`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.
> Extract shared dependencies as Wave-1 tasks for max parallelism.

Wave 1: 1-3（TDD 基线、服务层失败语义、session manager 并发/关闭边界）

Wave 2: 4-6（SSE/stream 可靠性、API 统一错误与恢复、最小诊断能力）

Wave 3: 7-9（Playwright 回归、文档/验证矩阵更新、稳定性收口验证）

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | - | 2,4,7,9 |
| 2 | 1 | 4,5,7 |
| 3 | 1 | 4,6,7 |
| 4 | 1,2,3 | 7,9 |
| 5 | 2 | 6,7,9 |
| 6 | 3,5 | 7,9 |
| 7 | 1,2,3,4,5,6 | 8,9 |
| 8 | 7 | 9 |
| 9 | 4,5,6,7,8 | F1-F4 |

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 3 tasks → `quick`, `unspecified-high`
- Wave 2 → 3 tasks → `unspecified-high`, `deep`
- Wave 3 → 3 tasks → `quick`, `writing`, `unspecified-high`

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Build TDD Reliability Baseline for Chat/Mainline Flows

  **What to do**: Audit and extend the existing reliability-focused test matrix around the current chat-first flow before any implementation changes. Add failing tests first for invalid input, session lifecycle transitions, stream subscription edge cases, and API error shape guarantees. Normalize which commands and evidence files will prove v7 stability so every downstream task extends the same baseline instead of inventing its own checks.
  **Must NOT do**: Do not change production behavior in this task beyond the minimum wiring needed to make new tests compile. Do not add a new test framework or replace Playwright/Go test.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Requires coordinated TDD setup across Go service/runtime/API and browser-level verification.
  - Skills: `[]` - Existing repo patterns and built-in tools are sufficient.
  - Omitted: `['review-work']` - Not needed inside an individual implementation task; reserved for final verification wave.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [2, 3, 4, 7, 9] | Blocked By: []

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/service/chat_test.go:16-75` - Existing TDD-style service tests for start conversation validation and persistence expectations.
  - Pattern: `internal/runtime/session_manager_test.go` - Session manager test conventions for concurrency/lifecycle assertions.
  - Pattern: `internal/server/api_test.go` - API contract tests to mirror structured error behavior.
  - Test: `e2e/tests/web-ui.spec.js:47-77` - Existing auth/composer browser regression style; extend rather than replace.
  - API/Type: `internal/service/service.go:38-78` - ValidationError / NotFoundError / ConflictError contracts to assert against.
  - External: `docs/ADR-005-tdd-first.md:1-16` - Mandatory failing-test-first workflow.
  - External: `Makefile:9-19` - Canonical verification commands already supported by the repo.

  **Acceptance Criteria** (agent-executable only):
  - [ ] New failing tests exist first for at least: invalid chat input, session conflict/closed-state handling, stream subscription error/termination behavior, and structured API error envelopes.
  - [ ] During this task’s RED phase, targeted commands for the newly added tests fail in a controlled way and the failure output is captured as evidence.
  - [ ] `cd e2e && npx playwright test --project=chromium --list` shows the new reliability-oriented browser scenarios are registered, or if Playwright dependencies are missing, the task records and resolves that prerequisite before listing tests again.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Failing test baseline is visible
    Tool: Bash
    Steps: Run targeted RED-phase commands such as `go test ./internal/service/...`, `go test ./internal/runtime/...`, and `go test ./internal/server/...` after adding new assertions but before behavior changes; capture the expected failures.
    Expected: Output shows targeted failures in the newly introduced reliability tests, proving RED phase was reached without requiring downstream tasks to be complete.
    Evidence: .sisyphus/evidence/task-1-tdd-baseline.txt

  Scenario: Browser regression cases are registered
    Tool: Bash
    Steps: In `e2e/`, ensure dependencies are available (`npm install` and Playwright browser install if required by the current repo setup), then run `npx playwright test --project=chromium --list` and confirm the newly added reliability/auth/session scenarios are present.
    Expected: The list includes new WebUI reliability scenarios without syntax/config errors, and any dependency/bootstrap output is captured.
    Evidence: .sisyphus/evidence/task-1-tdd-baseline-playwright.txt
  ```

  **Commit**: YES | Message: `test(reliability): 建立 v7 失败用例基线` | Files: `internal/**/*_test.go`, `e2e/tests/web-ui.spec.js`, `.sisyphus/evidence/task-1-*`

- [x] 2. Harden Chat Service Validation, Resume, and Conflict Semantics

  **What to do**: Strengthen `ChatService` behaviors around invalid start/submit input, unsupported verify/task options, resume eligibility, missing conversation/session links, and conflict semantics when a session is already active or no longer resumable. Implement only after task 1 has introduced the failing tests. Ensure service-returned errors map cleanly into the typed error contracts already defined in the service package.
  **Must NOT do**: Do not push HTTP-specific formatting into the service layer. Do not add ad hoc stringly-typed errors when a typed service error can express the condition.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: This is a bounded service-layer hardening task centered on existing chat contracts.
  - Skills: `[]` - No special skill needed.
  - Omitted: `['playwright']` - UI work is not required in this task.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [4, 5, 7] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/service/chat.go:107-160` - StartConversation input validation and task construction flow.
  - Pattern: `internal/service/chat.go` - Resume, submit, transcript, and list flows that must preserve chat semantics while hardening edge cases.
  - API/Type: `internal/service/service.go:18-110` - ChatService dependencies and typed error definitions.
  - Pattern: `internal/service/chat_test.go:16-120` - Existing validation and persistence assertions to extend.
  - Pattern: `internal/store/session_store.go` - Conversation/session lookup behaviors the service relies on.

  **Acceptance Criteria** (agent-executable only):
  - [ ] Service methods return typed validation/not-found/conflict errors for the new RED-path cases added in task 1.
  - [ ] `go test ./internal/service/...` passes with the new failure-path coverage.
  - [ ] Diff review confirms service-layer changes are limited to typed error semantics, resume/submit/start logic, and store/runtime interactions; no HTTP status codes or response-envelope structs are introduced under `internal/service/`.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Invalid chat input is rejected consistently
    Tool: Bash
    Steps: Run `go test ./internal/service/...` and inspect the new validation-focused tests for empty message, unsupported verify mode, or invalid max_steps cases.
    Expected: Tests pass and assert typed ValidationError behavior with stable messages.
    Evidence: .sisyphus/evidence/task-2-service-validation.txt

  Scenario: Resume/conflict edge cases stay typed
    Tool: Bash
    Steps: Run the targeted service tests covering already-active sessions, missing sessions, and non-resumable states.
    Expected: Tests pass and confirm ConflictError / NotFoundError behavior instead of generic errors.
    Evidence: .sisyphus/evidence/task-2-service-conflict.txt
  ```

  **Commit**: YES | Message: `fix(service): 加固聊天校验与恢复冲突语义` | Files: `internal/service/chat.go`, `internal/service/service.go`, `internal/service/*_test.go`, `.sisyphus/evidence/task-2-*`

- [x] 3. Harden Session Manager Lifecycle, Shutdown, and Subscription Boundaries

  **What to do**: Add TDD-backed protections for active-session cap handling, duplicate activation, shutdown races, event subscriber overflow/closure, actor finalization guarantees, and failure propagation from runtime execution into session state. Ensure the session manager remains the single lifecycle authority and that failure/closed states are observable and deterministic.
  **Must NOT do**: Do not bypass the session manager by moving lifecycle logic into handlers or the service layer. Do not mask overflow/closure problems by silently dropping all error information.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Concurrency and lifecycle hardening with race-sensitive tests requires careful coordination.
  - Skills: `[]` - Standard tools suffice.
  - Omitted: `['playwright']` - Browser execution is not part of the core implementation work here.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [4, 6, 7] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/runtime/session_manager.go:13-109` - Core limits, actor state constants, and manager structure.
  - Pattern: `internal/runtime/session_manager.go` - Subscription, actor start/finalize, shutdown, and overflow handling logic.
  - Pattern: `internal/runtime/session_manager_test.go` - Existing lifecycle/concurrency tests to extend under TDD.
  - Pattern: `internal/domain/session.go` - Session status model that lifecycle transitions must preserve.
  - Pattern: `internal/runtime/runtime.go:63-120` - Runtime execution entrypoint whose results/errors feed session finalization.

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./internal/runtime/...` passes with new coverage for shutdown, overflow, duplicate activation, and failure propagation.
  - [ ] `go test -race ./internal/runtime/...` passes without new race reports.
  - [ ] Session lifecycle tests prove deterministic final state for success, failure, cancellation, and manager shutdown paths.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Session manager race-sensitive paths stay green
    Tool: Bash
    Steps: Run `go test -race ./internal/runtime/...` after implementing the new session manager protections.
    Expected: Race detector reports no issues and targeted lifecycle tests pass.
    Evidence: .sisyphus/evidence/task-3-session-manager-race.txt

  Scenario: Subscriber overflow/closure edge paths are covered
    Tool: Bash
    Steps: Run the targeted runtime tests for subscription overflow, closed subscriptions, and shutdown while a session is active.
    Expected: Tests pass and confirm deterministic state/error propagation instead of hangs or silent drops.
    Evidence: .sisyphus/evidence/task-3-session-manager-edge.txt
  ```

  **Commit**: YES | Message: `fix(runtime): 加固会话生命周期与关闭边界` | Files: `internal/runtime/session_manager.go`, `internal/runtime/*_test.go`, `.sisyphus/evidence/task-3-*`

- [x] 4. Harden Stream Delivery and SSE Failure Handling End-to-End

  **What to do**: Tighten the runtime-to-HTTP streaming path so disconnects, malformed events, closed sessions, slow subscribers, and stream termination are handled predictably. Ensure SSE responses from `/api/v1/sessions/{id}/stream` do not leak partial/inconsistent behavior and that stream failures leave a diagnosable trail in runtime/session state and test evidence.
  **Must NOT do**: Do not replace SSE with another transport. Do not introduce best-effort silent recovery that hides broken stream state from tests or diagnostics.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Cross-layer reliability work spanning runtime event flow and HTTP SSE transport.
  - Skills: `[]` - Existing repo/test tooling is sufficient.
  - Omitted: `['frontend-ui-ux']` - This is backend transport hardening, not UI redesign.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [7, 9] | Blocked By: [1, 2, 3]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/api.go:154-157` - `sseWriter` transport wrapper.
  - Pattern: `internal/server/api.go:215-380` - Run/resume/chat/stream handlers and service-error mapping path.
  - Pattern: `internal/runtime/runtime.go:15-120` - Run path, retry budget, event emission dependencies.
  - Pattern: `internal/llm/sse.go` - Upstream SSE parsing behavior that may need edge-case tests if reused in the stream chain.
  - Test: `internal/server/api_test.go` - Existing HTTP handler contract tests to extend.
  - Test: `internal/runtime/*_test.go` - Existing runtime stream/event tests to follow.

  **Acceptance Criteria** (agent-executable only):
  - [ ] Stream-related unit/integration tests exist and pass for disconnect, terminated session, malformed/terminal event handling, and slow/closed subscriber behavior.
  - [ ] `go test ./internal/server/... ./internal/runtime/...` passes with the new stream reliability coverage.
  - [ ] Stream path behavior remains compatible with existing chat-first consumers (no contract-breaking route or payload changes unless tests/docs are updated in the same task).

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: SSE stream happy path remains valid
    Tool: Bash
    Steps: Run targeted server/runtime tests that open a stream, deliver events, and observe normal completion.
    Expected: Tests pass with expected event ordering and terminal signaling.
    Evidence: .sisyphus/evidence/task-4-stream-happy.txt

  Scenario: Disconnect and closed-session cases fail gracefully
    Tool: Bash
    Steps: Run targeted tests for client disconnect, missing/closed session stream requests, and slow subscriber edge cases.
    Expected: Tests pass and assert graceful termination or structured errors rather than panic/hang.
    Evidence: .sisyphus/evidence/task-4-stream-failure.txt
  ```

  **Commit**: YES | Message: `fix(stream): 加固 SSE 与流式失败处理` | Files: `internal/server/api.go`, `internal/runtime/runtime.go`, `internal/server/*_test.go`, `internal/runtime/*_test.go`, `.sisyphus/evidence/task-4-*`

- [x] 5. Normalize API Error Envelopes, Recovery, and Request Diagnostics

  **What to do**: Standardize API error behavior so auth failures, validation issues, not-found/conflict cases, and panic recovery all emit predictable structured envelopes with stable codes/messages and request correlation where already supported. Tighten recovery behavior so internal errors are surfaced consistently to clients while preserving enough diagnostic context for internal debugging.
  **Must NOT do**: Do not expose sensitive internal stack traces to clients. Do not duplicate service/business decisions in the HTTP layer.

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: Focused API contract hardening around existing middleware and error writer behavior.
  - Skills: `[]` - No special skill required.
  - Omitted: `['deep']` - The task is bounded to established API behavior.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [6, 7, 9] | Blocked By: [2]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/api.go:144-213` - `errorEnvelope`, `apiError`, `JSON`, `Recoverer`, `AuthMiddleware`.
  - Pattern: `internal/server/api.go:215-380` - Handler entrypoints and service-error mapping callsites.
  - Pattern: `internal/service/service.go:38-78` - Typed service errors that should map predictably to HTTP.
  - Test: `internal/server/api_test.go` - Existing API tests to extend for envelope and recovery guarantees.

  **Acceptance Criteria** (agent-executable only):
  - [ ] API tests prove unauthorized, validation, not-found, conflict, and panic recovery responses share stable envelope structure.
  - [ ] `go test ./internal/server/...` passes with new recovery/error mapping assertions.
  - [ ] Server test assertions explicitly verify response `status`, `error.code`, and `error.message` fields for each targeted error class and confirm panic recovery does not expose stack traces or raw internal error text.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Structured API errors are stable
    Tool: Bash
    Steps: Run targeted server tests covering invalid JWT, bad payload, missing session, and conflict cases.
    Expected: Tests pass and assert the same response envelope shape with correct status/code/message combinations.
    Evidence: .sisyphus/evidence/task-5-api-errors.txt

  Scenario: Panic recovery returns safe internal error
    Tool: Bash
    Steps: Run targeted middleware/recovery tests that force a panic inside a wrapped handler.
    Expected: Tests pass and confirm HTTP 500 with safe structured body rather than crash or raw stack trace.
    Evidence: .sisyphus/evidence/task-5-api-recoverer.txt
  ```

  **Commit**: YES | Message: `fix(api): 统一错误包络与恢复语义` | Files: `internal/server/api.go`, `internal/server/*_test.go`, `.sisyphus/evidence/task-5-*`

- [x] 6. Add Minimum Viable Internal Diagnostics for Session/Stream Troubleshooting

  **What to do**: Introduce the smallest useful diagnostics surface for internal operators and developers to understand why a chat/session/stream failed: request/session correlation, terminal failure reason visibility, and inspectable session/stream state through existing store/inspect patterns or minimal debug-facing output. Prefer extending existing inspect/status flows over inventing a new subsystem.
  **Must NOT do**: Do not build a full observability platform, metrics backend, or external tracing dependency. Do not expose secrets or raw provider payloads unnecessarily.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Requires careful scoping so diagnostics are useful yet do not become a large side-system.
  - Skills: `[]` - The work is repository-specific.
  - Omitted: `['playwright']` - Browser work may validate outcomes later but should not drive implementation choices here.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [7, 9] | Blocked By: [3, 5]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/store/session_store.go:19-25` and `internal/store/session_store.go:315-` - Existing `InspectState` type and `InspectSession` persistence/access pattern to extend.
  - Pattern: `internal/service/chat.go` - Transcript/inspect responses that already expose conversation/session state.
  - Pattern: `internal/server/api.go` - Existing request error envelope and session-facing handlers.
  - Test: `internal/service/chat_test.go` and `internal/server/api_test.go` - Validation points for new diagnostics visibility.
  - External: `README.md` and `docs/validation-matrix.md` - Final-facing docs/evidence references for internal troubleshooting capability.

  **Acceptance Criteria** (agent-executable only):
  - [ ] At least one existing inspect/status/API flow exposes request/session correlation plus terminal failure reason for failed internal chat/session runs, verified by automated assertions against concrete fields.
  - [ ] No new external dependency or standalone observability stack is introduced.
  - [ ] `go test ./internal/service/... ./internal/server/... ./internal/store/...` passes with diagnostic visibility assertions.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Internal diagnostics reveal terminal failure reason
    Tool: Bash
    Steps: Run targeted service/server tests that trigger a failed session and then inspect the exposed session/transcript/inspect state.
    Expected: Tests pass and confirm the failure can be correlated and understood through the supported diagnostics surface.
    Evidence: .sisyphus/evidence/task-6-diagnostics-happy.txt

  Scenario: Diagnostics stay minimal and safe
    Tool: Bash
    Steps: Run targeted tests that verify the diagnostic output does not leak raw secrets, stack traces, or unrelated provider internals.
    Expected: Tests pass and confirm only intended debug-safe fields are exposed.
    Evidence: .sisyphus/evidence/task-6-diagnostics-safe.txt
  ```

  **Commit**: YES | Message: `feat(debug): 增加最小内部诊断能力` | Files: `internal/service/chat.go`, `internal/server/api.go`, `internal/store/*`, `internal/**/*_test.go`, `.sisyphus/evidence/task-6-*`

- [x] 7. Expand Browser-Level Reliability Regression for Internal Chat Use

  **What to do**: Upgrade the current thin Playwright coverage into a reliability-oriented regression suite for the internal chat workflow: valid/invalid auth, empty input, start/submit flow, visible error feedback, stream/session continuity, and refresh/reconnect/session-history behaviors that matter for repeated internal use. Reuse the existing server harness and same-origin setup.
  **Must NOT do**: Do not redesign the UI. Do not add flaky timing-based checks that rely on arbitrary sleeps when deterministic selectors/events can be used.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Requires coordinated browser coverage across auth/chat/session edge paths with backend changes already in place.
  - Skills: `[]` - Existing Playwright setup is enough.
  - Omitted: `['frontend-ui-ux']` - The goal is regression reliability, not aesthetic improvement.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [8, 9] | Blocked By: [1, 2, 3, 4, 5, 6]

  **References** (executor has NO interview context - be exhaustive):
  - Test: `e2e/tests/web-ui.spec.js:1-77` - Existing auth and composer coverage style; extend this file or adjacent e2e tests consistently.
  - Test: `e2e/server-harness.js` - Canonical start/stop harness and JWT generation helpers.
  - External: `e2e/playwright.config.js` - Chromium-only project config, timeout, base URL semantics.
  - Pattern: `internal/server/api.go` and `internal/service/chat.go` - Backend routes/behaviors the browser flow depends on.

  **Acceptance Criteria** (agent-executable only):
  - [ ] `cd e2e && npx playwright test --project=chromium` passes with added internal reliability scenarios.
  - [ ] Browser tests cover at least: auth success/failure, empty composer validation, normal chat submission, visible failure feedback, and one continuity/reconnect/history scenario.
  - [ ] New e2e assertions avoid arbitrary sleeps; diff review confirms they use existing stable selectors, request helpers, or deterministic server-harness behavior.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Internal chat happy path is stable in browser
    Tool: Bash
    Steps: In `e2e/`, run `npx playwright test --project=chromium` after reliability scenarios are added.
    Expected: Browser suite passes and validates auth, chat workspace entry, composer use, and successful request flow.
    Evidence: .sisyphus/evidence/task-7-playwright-happy.txt

  Scenario: Browser shows graceful error feedback
    Tool: Bash
    Steps: Run the e2e cases that intentionally use invalid auth or failure-inducing submission/session states.
    Expected: The UI surfaces explicit error feedback instead of silent failure or broken layout.
    Evidence: .sisyphus/evidence/task-7-playwright-failure.txt
  ```

  **Commit**: YES | Message: `test(e2e): 扩充内部聊天稳定性回归` | Files: `e2e/tests/*.js`, `e2e/server-harness.js` (if needed), `.sisyphus/evidence/task-7-*`

- [x] 8. Update Progress, Validation Matrix, and Reliability Runbook Evidence

  **What to do**: Document v7’s reliability scope, verification commands, and evidence locations in the project-facing progress/docs artifacts. Update the validation matrix and milestone status so future work sees v7 as a hardening release with explicit verification proof, not an implicit set of code changes.
  **Must NOT do**: Do not overstate production readiness or external GA claims. Do not mark v7 complete until all implementation and verification tasks actually pass.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: This is documentation/status consolidation grounded in completed engineering evidence.
  - Skills: `[]` - Direct doc editing is enough.
  - Omitted: `['deep']` - No architecture redesign is needed.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [9] | Blocked By: [7]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `README.md` - Existing version progression and capability summary format.
  - Pattern: `PROGRESS.md` - Milestone completion tracking file.
  - Pattern: `docs/validation-matrix.md` - Verification-state reporting surface.
  - Pattern: `.sisyphus/plans/v6-chat-ui-six-layer-alignment.md` - Prior milestone summary structure for grounding v7 update language.
  - Evidence: `.sisyphus/evidence/task-1-*` through `.sisyphus/evidence/task-7-*` - Must be referenced accurately.

  **Acceptance Criteria** (agent-executable only):
  - [ ] README / PROGRESS / validation matrix reflect v7 as an internal reliability release with accurate scope.
  - [ ] Documentation references only evidence files and commands that actually exist and pass.
  - [ ] Doc diffs remain scoped to v7 status, verification, troubleshooting guidance, and any dependency/bootstrap notes required to run the verified commands.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Documentation matches executed evidence
    Tool: Bash
    Steps: Inspect the updated docs and confirm every referenced command/evidence file exists after tasks 1-7 complete.
    Expected: No stale or fabricated references remain in README, PROGRESS, or validation matrix.
    Evidence: .sisyphus/evidence/task-8-docs-proof.txt

  Scenario: v7 status is not overstated
    Tool: Bash
    Steps: Review the v7 wording in docs for unsupported claims such as external GA, multi-tenant readiness, or full observability platform completion.
    Expected: Documentation stays faithful to “internal stable usage” scope only.
    Evidence: .sisyphus/evidence/task-8-docs-scope.txt
  ```

  **Commit**: YES | Message: `docs(v7): 更新稳定性发布进度与验证矩阵` | Files: `README.md`, `PROGRESS.md`, `docs/validation-matrix.md`, `.sisyphus/evidence/task-8-*`

- [x] 9. Run Full Reliability Verification Sweep and Triage Remaining Gaps

  **What to do**: Execute the full v7 verification matrix after tasks 1-8 are complete, collect evidence, and fix any final reliability regressions revealed by unit, race, coverage, smoke, or Playwright runs. This task is the implementation-side release gate before the final four-agent verification wave.
  **Must NOT do**: Do not skip failing commands or downgrade assertions to make the suite appear green. Do not mark success until all listed commands pass and evidence is captured.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: This is a release-gate verification and triage task spanning the entire changed surface.
  - Skills: `[]` - Existing repo tooling is the gate.
  - Omitted: `['review-work']` - Final review is handled by F1-F4, not here.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: [F1, F2, F3, F4] | Blocked By: [4, 5, 6, 7, 8]

  **References** (executor has NO interview context - be exhaustive):
  - External: `Makefile:9-19` - Canonical Go verification commands.
  - External: `docs/ADR-005-tdd-first.md:1-16` - Confirms tests are first-class release evidence.
  - Test: `e2e/playwright.config.js` and `e2e/tests/web-ui.spec.js` - Browser verification track.
  - Evidence: `.sisyphus/evidence/task-1-*` through `.sisyphus/evidence/task-8-*` - Prior task outputs to cross-check.

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes.
  - [ ] `go test -race ./...` passes.
  - [ ] `go test -cover ./...` passes.
  - [ ] `go test -tags=smoke ./internal/llm/...` passes.
  - [ ] `cd e2e && npx playwright test --project=chromium` passes, with any required dependency/bootstrap commands captured in evidence.
  - [ ] Evidence files for all commands and any triaged fixes are present under `.sisyphus/evidence/`.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Full automated release gate passes
    Tool: Bash
    Steps: Run `go test ./...`, `go test -race ./...`, `go test -cover ./...`, `go test -tags=smoke ./internal/llm/...`, then `cd e2e && npx playwright test --project=chromium`.
    Expected: Every command exits successfully with captured logs/evidence.
    Evidence: .sisyphus/evidence/task-9-full-gate.txt

  Scenario: Any final gap is explicitly triaged
    Tool: Bash
    Steps: If any gate fails during the first pass, fix the defect, re-run the exact failed gate, and record both failure and pass evidence.
    Expected: No unresolved verification failure remains hidden; either the gate passes or the task remains incomplete.
    Evidence: .sisyphus/evidence/task-9-triage.txt
  ```

  **Commit**: YES | Message: `test(v7): 完成内部稳定性发布总验收` | Files: `.sisyphus/evidence/task-9-*`, plus any minimal follow-up fixes required by gate failures

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer one commit per task when the task meaningfully changes a bounded reliability slice (tests + implementation + evidence).
- Use conventional commit messages in Chinese where repository hooks require it, e.g. `test(service): 补齐聊天失败路径回归` or `fix(runtime): 加固会话流式关闭边界`.
- Avoid bundling unrelated backend, UI, and docs work into one giant commit unless the task explicitly spans them.

## Success Criteria
- 团队成员可以连续多次运行 chat-first 主流程，而不会在常见异常情况下陷入不可恢复或不可诊断状态。
- 核心失败路径具备自动化测试与浏览器级验证，不再依赖人工口头证明“应该没问题”。
- 出现 JWT、输入、会话冲突、断流、关闭、内部错误等问题时，API/UI/证据的表现一致且可定位。
- v7 完成后，v8 扩能力时不需要先回头补大面积稳定性债务。
