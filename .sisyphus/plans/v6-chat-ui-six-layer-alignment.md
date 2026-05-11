# v6 Chat-First UI + Six-Layer Harness Alignment

## TL;DR
> **Summary**: Rework v5’s same-origin embedded Web UI from task-submit/session-observe workflow into a chat-first conversational interface, while restructuring the serving/application boundaries so the project aligns more closely with the public Harness Engineering six-layer baseline `Types → Config → Repo → Service → Runtime → UI`.
> **Deliverables**:
> - Chat-first primary UX replacing task-form-first navigation
> - First-class conversation/message/service contracts that preserve streaming, resume, inspect, and history
> - Dependency/boundary refactor toward six-layer alignment without unnecessary package churn
> - Regression-safe automated verification across Go tests, Playwright, race, coverage, and evidence capture
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: 1 → 2 → 3 → 6 → 8 → F1-F4

## Context
### Original Request
- v5 虽然已经完成并加了 web ui，但前端交互不是想要的结果。
- 项目是通用智能体，期望前端是聊天界面、对话式沟通，而不是提交任务式。
- 项目目标基于 Harness Engineering，需要包含六层架构。
- 需要查看相关技术文档，对照当前项目判断是否满足需求，并给出改进计划。
- 六层基线采用公开 Harness Engineering 资料。
- 输出新增 v6 规划，不替换 vv5。
- v6 只聚焦“聊天式前端 + 六层架构对齐 + 必要验证补强”，不额外扩展新产品能力。

### Interview Summary
- 已确认当前 v5 Web UI 不满足“通用智能体对话式交互”的目标。
- 已确认评估基线采用公开 Harness Engineering 资料，而不是内部私有六层定义。
- 已确认本次只输出 v6 改进规划，且不扩展额外产品能力。
- 已确认范围应保留已有 v5 的可用能力（流式输出、会话、inspect、验证基础），但改造其主交互方式与架构边界。

### Metis Review (gaps addressed)
- 防止 scope creep：禁止顺带扩展额外产品功能，只允许为 chat-first 与六层对齐所必需的改动。
- 防止表面化改造：要求后端先建立会话/消息/流式事件的聊天原语，不能只换 UI 皮肤继续调用“单次任务提交”心智模型。
- 防止纯 package 搬家：六层对齐以依赖方向和职责清晰为目标，而非大规模目录重命名。
- 防止 chat/legacy 双轨割裂：stream、resume、inspect、history 必须直接服务于 chat 主流程，不能保留“聊天只负责提交，老页面负责关键能力”的分裂体验。

## Work Objectives
### Core Objective
让 zheng-harness 的 v6 版本以“聊天界面”作为 Web 主入口，并将当前偏 HTTP handler 直连 runtime/store 的实现收敛为更清晰的六层职责边界，使项目在用户体验和 Harness Engineering 架构表达上都更贴近通用智能体产品定位。

### Deliverables
- 新的 v6 架构 ADR/设计说明，明确 chat-first UX 与六层边界
- 后端聊天应用层/服务层抽象（conversation/session/message/stream/use-case）
- 与现有 runtime/store/server 兼容的聊天 API/流式契约
- 同源内嵌 chat-first Web UI
- 覆盖 happy path / failure path / resume / inspect / auth / stream 的自动化测试与证据

### Definition of Done (verifiable conditions with commands)
- `go test ./...` 通过，且不存在因 v6 改造引入的单元/集成回归。
- `go test -race ./...` 通过，聊天流式场景无新增并发问题。
- `go test -cover ./...` 通过，关键聊天服务与 server 集成具备新增覆盖。
- `npx playwright test --project=chromium`（在 `e2e/` 下）通过，chat-first 主流程稳定。
- 启用 Web UI 后，浏览器主入口默认进入聊天工作区，而非“新建任务”表单式页面。
- 聊天主流程内可完成：发起请求、接收流式回复、查看历史、继续会话、查看结构化详情、处理错误与鉴权失效。
- 服务端依赖边界可证明为：UI/transport 不直接承载业务编排决策；核心聊天用例可脱离 HTTP handler 被调用。

### Must Have
- 保留同源内嵌部署模式，除非某项改动对 chat-first 为绝对必要；默认不引入独立 SPA 构建系统。
- 复用现有 SSE / session / inspect / persistence 能力，而不是平行再造第二套执行引擎。
- 引入聊天领域契约：conversation/message/turn/assistant-event 或等价结构。
- 显式定义六层映射：Types、Config、Repo、Service、Runtime、UI。
- 测试与验证继续保持“零人工判断”原则。

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- 不新增与本次目标无关的产品能力（如多账户、OAuth、独立前端站点、WebSocket 重构、插件市场等）。
- 不做仅 UI 文案/样式层面的“伪聊天化”。
- 不做无依据的大规模 package rename / move，仅在职责或依赖方向必须调整时变更。
- 不破坏 v4/v5 已验证的 run/resume/inspect/stream 行为与现有 CI 基线。
- 不允许聊天流程依赖旧“任务表单页”作为关键前置步骤。

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: tests-after + existing Go test + Playwright framework
- QA policy: Every task has agent-executed scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.
> Extract shared dependencies as Wave-1 tasks for max parallelism.

Wave 1: foundation + architecture contracts + acceptance alignment (Tasks 1-4)
Wave 2: service/API/runtime integration + UI shell migration + tests (Tasks 5-7)
Wave 3: regression hardening + docs/evidence + cleanup (Tasks 8-10)

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks |
|---|---|---|
| 1 | - | 2,3,4,5,6,7,8,9,10 |
| 2 | 1 | 5,6,7,8,9,10 |
| 3 | 1 | 5,6,7,8 |
| 4 | 1 | 6,7,8,9 |
| 5 | 1,2,3 | 6,8,9 |
| 6 | 2,3,4,5 | 7,8,9 |
| 7 | 4,5,6 | 8,9 |
| 8 | 5,6,7 | 9,10 |
| 9 | 5,6,7,8 | 10 |
| 10 | 8,9 | F1-F4 |

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 4 tasks → `deep`, `ultrabrain`, `unspecified-high`, `writing`
- Wave 2 → 3 tasks → `unspecified-high`, `visual-engineering`, `deep`
- Wave 3 → 3 tasks → `unspecified-high`, `writing`, `quick`

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Freeze v6 architecture baseline and six-layer mapping

  **What to do**: Create a v6 architecture decision record that names the public Harness Engineering baseline `Types → Config → Repo → Service → Runtime → UI`, maps each current package/module into that baseline, identifies concrete misalignments, and defines the target dependency direction for chat-first work. The document must explicitly reference the current v5 web constraints from `docs/ADR-010-v5-web-ui.md`, the embedded serving mechanism in `internal/server/web.go`, and the current package set under `internal/`. It must also define which legacy v5 constraints remain in force (same-origin embedded UI, SSE, existing auth model) versus what changes in v6 (chat-first primary UX, application/service extraction, message-centric contracts).
  **Must NOT do**: Do not rename packages or move files as part of the ADR task. Do not introduce a second six-layer interpretation or leave package ownership ambiguous.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: ADR/spec production with strict architecture mapping and references
  - Skills: `[]` - No additional skill required
  - Omitted: `['/frontend-ui-ux']` - Not a UI design task

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [2,3,4,5,6,7,8,9,10] | Blocked By: []

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `docs/ADR-010-v5-web-ui.md:11-39` - v5 explicitly framed as lean same-origin Web UI over existing REST + SSE contract
  - Pattern: `internal/server/web.go:13-45` - embedded asset serving and same-origin route mount behavior
  - API/Type: `internal/config/config.go:27-68` - config and WebUI settings boundary for Config layer
  - API/Type: `internal/domain/session.go:5-27` - current session type baseline within Types layer
  - API/Type: `internal/store/session_store.go:19-65` - persistence/inspect/list lifecycle structures for Repo layer
  - API/Type: `internal/runtime/session_manager.go:36-109` - runtime session orchestration/event subscription boundary
  - External: `https://openai.com/index/harness-engineering/` - public six-layer baseline
  - External: `https://martinfowler.com/articles/harness-engineering.html` - harness categories and fitness framing

  **Acceptance Criteria** (agent-executable only):
  - [ ] New ADR/design doc exists and explicitly maps every current v6-relevant package to one of the six layers.
  - [ ] ADR identifies at least three current misalignments or ambiguity points with concrete file/package references.
  - [ ] ADR states preserved v5 constraints vs changed v6 constraints with no contradictory guidance.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Six-layer mapping completeness audit
    Tool: Bash
    Steps: Run a repo check that lists referenced packages/modules and compare against the ADR mapping table.
    Expected: Every v6-relevant package used by chat/server/runtime/store/config/domain is covered once with no unmapped package.
    Evidence: .sisyphus/evidence/task-1-architecture-baseline.txt

  Scenario: Misalignment specificity check
    Tool: Bash
    Steps: Grep the ADR for concrete file/package references and named dependency directions.
    Expected: The ADR contains explicit references and does not rely on generic statements like "improve layering" without evidence.
    Evidence: .sisyphus/evidence/task-1-architecture-baseline-error.txt
  ```

  **Commit**: YES | Message: `docs(v6): define chat-first six-layer baseline` | Files: [docs/ADR-v6-*.md or equivalent]

- [x] 2. Introduce first-class chat domain contracts in Types layer

  **What to do**: Add or extend domain-level types so chat becomes a first-class concept rather than an overloaded task form. Define conversation/session-turn/message/event primitives, user message payloads, assistant stream event envelopes, resumable conversation semantics, and inspectable chat transcript structures. Ensure these types coexist with existing `domain.Session`, `domain.Step`, task categories, and streaming concepts without breaking verified run/resume behavior. The contracts must support chat-first UI needs while still fitting the harness runtime model.
  **Must NOT do**: Do not hard-code HTTP concerns, DOM fields, or persistence schema shortcuts into domain contracts. Do not let chat types duplicate existing session identity or status semantics inconsistently.

  **Recommended Agent Profile**:
  - Category: `ultrabrain` - Reason: contract design with cross-layer impact and compatibility constraints
  - Skills: `[]` - No extra skill required
  - Omitted: `['/frontend-ui-ux']` - This is domain modeling, not visual design

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [5,6,7,8,9,10] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/domain/session.go:5-27` - existing session identity and status baseline
  - Pattern: `internal/server/api.go:43-89` - current request/response contracts centered on run/resume/list/inspect rather than conversation turns
  - Pattern: `internal/server/web/js/api.js:68-94` - current client API only knows run/resume/inspect/list
  - Pattern: `docs/ADR-006-streaming-architecture.md:7-31` - existing streaming event expectations and fallback semantics
  - Pattern: `internal/runtime/session_manager.go:102-109,349-621` - event subscription/overflow semantics to preserve
  - API/Type: `internal/store/session_store.go:19-65` - inspect/list persistence payload expectations

  **Acceptance Criteria** (agent-executable only):
  - [ ] Types layer exposes explicit chat/conversation/message contracts with tests proving serialization or usage expectations.
  - [ ] New contracts can represent: first user turn, streamed assistant response, follow-up user turn, resume/re-entry, and inspectable transcript state.
  - [ ] Existing session/status concepts remain single-source-of-truth and are referenced rather than duplicated.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Domain contract compilation and tests
    Tool: Bash
    Steps: Run `go test ./internal/domain/... ./internal/server/...` or the exact package tests covering new chat contracts.
    Expected: Contracts compile, tests pass, and no incompatible type regressions occur.
    Evidence: .sisyphus/evidence/task-2-chat-types.txt

  Scenario: Multi-turn transcript representation check
    Tool: Bash
    Steps: Execute a targeted unit test that constructs a conversation with multiple turns and streamed events.
    Expected: Test proves transcript/order/status semantics are deterministic and resumable.
    Evidence: .sisyphus/evidence/task-2-chat-types-error.txt
  ```

  **Commit**: YES | Message: `feat(domain): add chat conversation contracts` | Files: [internal/domain/**, related tests]

- [x] 3. Extract chat application/service layer from HTTP handlers

  **What to do**: Introduce a dedicated service/use-case layer that owns conversation start, follow-up turn submission, resume/continue decisions, transcript assembly, inspect shaping, and stream handoff orchestration. Move business orchestration currently embedded in `internal/server/api.go` into this layer so HTTP handlers become transport-only adapters. The service layer must compose `store`, `runtime`, config, and any builder/runtime factory concerns without leaking raw HTTP request structs into business logic.
  **Must NOT do**: Do not leave the main business path inside `HandleRun`, `HandleResume`, or equivalent handlers. Do not make the new service layer depend on DOM routes or hash-based page navigation assumptions.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: medium-to-high complexity backend refactor with dependency re-shaping
  - Skills: `[]` - No extra skill required
  - Omitted: `['/git-master']` - Not a git task

  **Parallelization**: Can Parallel: LIMITED | Wave 1 | Blocks: [5,6,7,8,9] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/api.go:176-344` - existing run/resume/inspect/list business flow embedded in handlers
  - Pattern: `internal/server/api.go:360-411,638-738` - stream and response shaping responsibilities needing clear separation
  - Pattern: `internal/runtime/session_manager.go:44-68,162-315` - session start/finalization lifecycle to be called from service layer
  - Pattern: `internal/store/session_store.go:103-410` - persistence operations to be mediated by service layer, not directly by transport
  - API/Type: `internal/config/config.go:27-68` - config boundary for service orchestration inputs
  - API/Type: `internal/server/api.go:27-39` - current `API` struct dependency bundle that should be reduced in responsibility

  **Acceptance Criteria** (agent-executable only):
  - [ ] A service/application package exists and can be tested without spinning up HTTP routes.
  - [ ] Server handlers delegate chat/run/resume/inspect logic to service methods rather than owning orchestration decisions.
  - [ ] Service tests cover success and failure paths for starting a chat turn and continuing a conversation.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Transport-free application test
    Tool: Bash
    Steps: Run targeted service-layer tests without starting an HTTP server.
    Expected: Core chat use cases pass through service APIs directly.
    Evidence: .sisyphus/evidence/task-3-chat-service.txt

  Scenario: HTTP handler thinness regression check
    Tool: Bash
    Steps: Run server package tests verifying handlers call service methods and still return correct status/JSON shapes.
    Expected: HTTP behavior preserved while business logic remains outside handler bodies.
    Evidence: .sisyphus/evidence/task-3-chat-service-error.txt
  ```

  **Commit**: YES | Message: `refactor(service): extract chat application orchestration` | Files: [internal/server/**, new service package, tests]

- [x] 4. Define chat-first UX contract and legacy capability mapping

  **What to do**: Produce a concrete v6 UX contract that specifies the browser information architecture, screen states, component responsibilities, and how every essential legacy capability maps into chat. At minimum define: default landing state, conversation pane, message composer, assistant stream area, session/history access, inspect/details access, auth bootstrap, reconnect/disconnect handling, blocked/resume semantics, and failure display. This contract must explicitly replace the task-form-first flow embodied by `index.html` + hash routes with a chat-first primary route.
  **Must NOT do**: Do not leave the UX as a vague mockup request. Do not keep "dashboard" and "new task" as equal-priority top-level entrypoints.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: interaction architecture and concrete UI contract definition
  - Skills: [] - No extra skill required
  - Omitted: `['/frontend-ui-ux']` - unavailable skill; use category instead

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [6,7,8,9] | Blocked By: [1]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/web/index.html:28-102` - current nav/dashboard/task/stream/detail page contract
  - Pattern: `internal/server/web/js/app.js:324-326,700-714,832-952,962-1162` - current hash-route page switching, dashboard loading, stream/detail rendering, run/resume behaviors
  - Pattern: `docs/ADR-010-v5-web-ui.md:19-39` - v5 scope and explicit deferrals to respect unless v6 requires change
  - Pattern: `internal/server/web/js/api.js:68-96` - current client contract limitations
  - External: `https://openai.com/index/harness-engineering/` - baseline emphasizing harnessability and operator ergonomics

  **Acceptance Criteria** (agent-executable only):
  - [ ] v6 UX contract names every top-level route/view/state and identifies chat as the default primary workflow.
  - [ ] Legacy capabilities (stream, resume, inspect, history, auth) each have an explicit place inside the chat workflow.
  - [ ] No required user journey depends on first visiting a standalone “new task” form page.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: UX contract completeness review
    Tool: Bash
    Steps: Grep the UX contract for required states/features: auth, compose, stream, history, inspect, resume, error, reconnect.
    Expected: All named states are present with explicit behavior notes.
    Evidence: .sisyphus/evidence/task-4-ux-contract.txt

  Scenario: Legacy mapping gap check
    Tool: Bash
    Steps: Compare the v5 feature list against the v6 UX contract mapping section.
    Expected: No preserved v5 capability is left unmapped or marked as "future" within v6 scope.
    Evidence: .sisyphus/evidence/task-4-ux-contract-error.txt
  ```

  **Commit**: YES | Message: `docs(ui): define chat-first browser workflow` | Files: [docs/v6-ux-*.md or equivalent]

- [x] 5. Add chat-oriented API and persistence flow without breaking existing session semantics

  **What to do**: Extend the server/service/store integration so chat-oriented operations are first-class: create conversation/start first turn, append follow-up turn to an existing conversation/session, retrieve transcript/history optimized for chat rendering, and continue streaming through the existing event pipeline. Reuse current session identity, inspect, and persistence mechanisms wherever possible, but add any necessary repo/service adapters to represent message history cleanly. Keep compatibility with current run/resume/list/inspect flows unless the v6 ADR explicitly marks them deprecated but supported.
  **Must NOT do**: Do not replace the existing verified session model with a parallel unpersisted chat-only store. Do not introduce API breakage that forces removal of v4/v5 flows during v6.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: coordinated API/service/store implementation across multiple backend packages
  - Skills: [] - No extra skill required
  - Omitted: `['/playwright']` - Not a browser task

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [6,7,8,9] | Blocked By: [1,2,3]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/api.go:43-89,176-344,638-738` - current request/response and stream URL shapes to preserve or extend carefully
  - Pattern: `internal/store/session_store.go:19-65,296-410` - inspect/list persistence behavior and session summaries
  - Pattern: `internal/runtime/session_manager.go:162-315,349-621` - event stream lifecycle and overflow behavior
  - Pattern: `internal/server/web/js/api.js:68-96` - current client-facing API contract gaps
  - API/Type: `internal/domain/session.go:19-27` - canonical session identity/status
  - API/Type: `internal/config/config.go:53-68` - server/web config boundaries that must remain stable

  **Acceptance Criteria** (agent-executable only):
  - [ ] Backend exposes a documented chat-oriented flow for first turn + follow-up turn + transcript retrieval using persisted session/conversation state.
  - [ ] Existing stream behavior remains usable for chat replies and does not regress inspect/list session APIs.
  - [ ] API/server tests cover unauthorized, invalid payload, missing conversation/session, and active-stream conflict cases.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Chat API success flow
    Tool: Bash
    Steps: Run targeted server/integration tests that create a conversation, send a follow-up turn, and inspect transcript state.
    Expected: Session/conversation persists correctly and streamed reply hooks remain valid.
    Evidence: .sisyphus/evidence/task-5-chat-api.txt

  Scenario: Chat API failure handling
    Tool: Bash
    Steps: Run targeted tests for unauthorized requests, malformed turn payloads, missing session IDs, and invalid resume/follow-up states.
    Expected: Backend returns deterministic error codes/messages without corrupting persisted state.
    Evidence: .sisyphus/evidence/task-5-chat-api-error.txt
  ```

  **Commit**: YES | Message: `feat(server): support persisted chat conversation flows` | Files: [internal/server/**, service layer, store/runtime integration, tests]

- [x] 6. Rebuild embedded Web UI around chat-first primary workflow

  **What to do**: Replace the current hash-routed dashboard/task/detail-first information architecture with a chat-first UI shell served from the same embedded asset pipeline. The default screen after auth should be a conversation workspace with transcript pane, composer, assistant streaming area, conversation/session context, and direct affordances for history, inspect, resume, and failure recovery. Preserve same-origin deployment, current auth bootstrap model, and embedded asset delivery. The UI may still expose secondary history/inspect surfaces, but the main navigation must center on conversation rather than “new task.”
  **Must NOT do**: Do not retain the existing task form as the primary interaction surface. Do not make users switch back to a legacy dashboard page for streaming or inspection of the active conversation.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: substantial UI architecture and interaction rewrite with embedded web constraints
  - Skills: [] - No extra skill required
  - Omitted: `['/refactor']` - plan references are sufficient; direct refactor skill not needed

  **Parallelization**: Can Parallel: LIMITED | Wave 2 | Blocks: [7,8,9] | Blocked By: [2,3,4,5]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/web/index.html:10-102` - current auth + dashboard + task form + stream + detail markup to replace/restructure
  - Pattern: `internal/server/web/js/app.js:19-46,324-326,700-714,832-952,962-1162` - current UI state model and page navigation behavior
  - Pattern: `internal/server/web/js/api.js:28-96` - request/auth/unauthorized handling to preserve
  - Pattern: `internal/server/web.go:16-45` - embedded same-origin serving contract
  - Pattern: `docs/ADR-010-v5-web-ui.md:21-39,41-49` - same-origin + embedded-asset guardrails still relevant
  - Test: `docs/validation-matrix.md:694-761` - existing web UI validation surfaces to preserve and evolve

  **Acceptance Criteria** (agent-executable only):
  - [ ] After auth, the browser defaults to a chat workspace rather than a task creation page.
  - [ ] Users can submit a first prompt and at least one follow-up prompt from the same conversation UI.
  - [ ] Active assistant output streams into the conversation UI without requiring navigation to a separate stream-only page.
  - [ ] Inspect/history actions are reachable from the chat workflow and correctly display persisted data.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Chat-first browser happy path
    Tool: Playwright
    Steps: Launch the server with Web UI enabled, authenticate, submit an initial user prompt, wait for assistant stream output, send a follow-up prompt, and open the persisted conversation details.
    Expected: The flow completes entirely inside the chat workspace and shows persisted conversation/session data.
    Evidence: .sisyphus/evidence/task-6-chat-ui.png

  Scenario: Chat UI error/reconnect path
    Tool: Playwright
    Steps: Trigger auth expiry or stream interruption while on the chat page, then attempt recovery according to the UI contract.
    Expected: The UI shows a deterministic error/reconnect state and does not silently lose the active conversation context.
    Evidence: .sisyphus/evidence/task-6-chat-ui-error.png
  ```

  **Commit**: YES | Message: `feat(web): make embedded ui chat-first` | Files: [internal/server/web/**, related tests]

- [x] 7. Preserve and adapt resume/inspect/history flows inside chat UX

  **What to do**: Rework existing session history, inspect view, and resume behavior so they become natural subflows of the chat experience. A user must be able to reopen a prior conversation, inspect structured execution artifacts, understand blocked/running/completed states, and continue the conversation if resumable—all from the chat-oriented navigation model. Align backend response shaping and frontend rendering so these are not “legacy pages” bolted on afterward.
  **Must NOT do**: Do not leave history/inspect/resume usable only through deprecated routes or ad hoc developer links. Do not flatten execution detail so much that harness traceability is lost.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: full-stack integration of retained harness capabilities into new UX
  - Skills: [] - No extra skill required
  - Omitted: `['/playwright']` - Playwright is for QA scenarios, not implementation guidance

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [8,9] | Blocked By: [4,5,6]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/web/index.html:53-102` - current dashboard/session-list/stream/detail separation
  - Pattern: `internal/server/web/js/app.js:704-714,832-952,1117-1162` - list loading, stream shell, detail rendering, run/resume redirect behavior
  - Pattern: `internal/store/session_store.go:19-65,296-410` - inspect/list lifecycle and summary data available
  - Pattern: `internal/server/api.go:235-344` - current resume/inspect/list server endpoints and shapes
  - Test: `docs/validation-matrix.md:683-761` - established validation surfaces for dashboard/detail/history and mount behavior

  **Acceptance Criteria** (agent-executable only):
  - [ ] Prior sessions/conversations can be found from the chat UI and reopened into a transcript-aware view.
  - [ ] Resume/continue affordances are only shown when the underlying session state supports them.
  - [ ] Structured inspect data remains accessible for debugging/audit without leaving the chat experience orphaned.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Reopen and continue prior conversation
    Tool: Playwright
    Steps: Create a conversation, navigate to history, reopen it, and continue with a new follow-up turn.
    Expected: The resumed conversation preserves prior transcript and appends the new turn correctly.
    Evidence: .sisyphus/evidence/task-7-history-resume.png

  Scenario: Non-resumable session guardrail
    Tool: Playwright
    Steps: Open a terminal/non-resumable conversation state and inspect available actions.
    Expected: Resume/continue controls are hidden or disabled with clear rationale, while inspect remains available.
    Evidence: .sisyphus/evidence/task-7-history-resume-error.png
  ```

  **Commit**: YES | Message: `feat(web): fold history and resume into chat flow` | Files: [internal/server/web/**, internal/server/**, tests]

- [x] 8. Expand automated tests for chat-first service, API, and browser flows

  **What to do**: Add or refactor automated tests so v6 behavior is proven across domain/service/server/browser layers. Cover chat conversation creation, multi-turn follow-up, streaming assistant output, auth bootstrap/expiry, history reopen, inspect availability, resume handling, conflict/error states, and non-regression of existing session APIs. Ensure CI remains the source of truth and any renamed tests still preserve the v5 guarantees that matter in v6.
  **Must NOT do**: Do not rely on manual exploratory verification. Do not add brittle browser tests that assert cosmetic details instead of behaviorally relevant selectors and states.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: broad test-surface redesign across backend and browser with regression sensitivity
  - Skills: [] - No extra skill required
  - Omitted: `['/review-work']` - final verification wave handles review separately

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: [9,10] | Blocked By: [5,6,7]

  **References** (executor has NO interview context - be exhaustive):
  - Test: `internal/server/api_test.go` - current server-level regression patterns
  - Test: `cmd/server/server_test.go` - route mount and web enablement expectations
  - Test: `internal/runtime/session_manager_test.go` - event/session lifecycle regression coverage
  - Test: `docs/validation-matrix.md:683-761` - v5 Web UI validation matrix surfaces to adapt for chat-first
  - Pattern: `.github/workflows/ci.yml:11-98` - verify + browser jobs the new suite must continue satisfying
  - Pattern: `docs/ADR-005-tdd-first.md:10-13` - testing philosophy baseline

  **Acceptance Criteria** (agent-executable only):
  - [ ] Go test suite includes explicit v6 chat-first coverage in domain/service/server layers.
  - [ ] Playwright suite proves the primary chat workflow, follow-up turn, reopen/history, and failure handling.
  - [ ] CI commands remain green: `go test ./...`, `go test -race ./...`, `go test -cover ./...`, and Playwright browser run.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Full backend regression suite
    Tool: Bash
    Steps: Run `go test ./...`, `go test -race ./...`, and `go test -cover ./...`.
    Expected: All commands pass with v6 chat additions and no regressions.
    Evidence: .sisyphus/evidence/task-8-test-suite.txt

  Scenario: Full browser regression suite
    Tool: Bash
    Steps: In `e2e/`, run `npx playwright test --project=chromium` against the updated chat-first UI.
    Expected: Browser suite passes and includes chat-first selectors/states rather than only task-submit flows.
    Evidence: .sisyphus/evidence/task-8-test-suite-error.txt
  ```

  **Commit**: YES | Message: `test(v6): cover chat-first harness flows` | Files: [internal/**/*_test.go, e2e/**, docs/validation-matrix.md if updated]

- [x] 9. Update validation matrix and operational documentation for v6

  **What to do**: Revise validation and usage documentation so it describes the v6 chat-first workflow, preserved same-origin deployment, six-layer architecture intent, and the exact commands/tests/evidence needed to verify the system. Update any v5-oriented wording that still frames the UI primarily as dashboard/new-task workflow. Ensure the docs make it obvious how chat, history, inspect, stream, and resume work together, and how the architecture now maps to the public six-layer baseline.
  **Must NOT do**: Do not leave docs partially updated with contradictory v5 vs v6 narratives. Do not document aspirational behavior that is not covered by tests.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: high-precision technical documentation aligned with implemented behavior
  - Skills: [] - No extra skill required
  - Omitted: `['/playwright']` - documentation task only

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [10] | Blocked By: [5,6,7,8]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `README.md` - current v5 summary and positioning language
  - Pattern: `docs/USAGE.md:713-847` - current v5 Web UI usage section and browser test commands
  - Pattern: `docs/validation-matrix.md:683-761` - current embedded Web UI validation section to evolve
  - Pattern: `docs/ADR-010-v5-web-ui.md:19-39` - v5 constraints to reference when documenting retained/deprecated behavior
  - External: `https://openai.com/index/harness-engineering/` - six-layer baseline wording source

  **Acceptance Criteria** (agent-executable only):
  - [ ] README/USAGE/validation docs consistently describe chat-first v6 behavior and preserved deployment constraints.
  - [ ] Validation matrix includes chat-first proof surfaces with exact commands and evidence targets.
  - [ ] No doc section still claims the primary web workflow is "new task" or dashboard-first unless explicitly marked historical.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Documentation consistency check
    Tool: Bash
    Steps: Grep README, USAGE, ADR, and validation docs for outdated v5-primary workflow language and verify updated v6 wording.
    Expected: No unqualified doc text contradicts the implemented chat-first workflow.
    Evidence: .sisyphus/evidence/task-9-docs.txt

  Scenario: Validation doc executability check
    Tool: Bash
    Steps: Cross-check every new v6 validation claim against an actual test command or evidence target in the matrix.
    Expected: Every documented proof surface maps to an executable command/test.
    Evidence: .sisyphus/evidence/task-9-docs-error.txt
  ```

  **Commit**: YES | Message: `docs(v6): align validation and usage with chat ui` | Files: [README.md, docs/USAGE.md, docs/validation-matrix.md, related docs]

- [x] 10. Run architecture regression and dependency-boundary cleanup pass

  **What to do**: After feature/test/docs work lands, perform a final cleanup pass focused on six-layer integrity and maintainability: eliminate temporary compatibility shims that violate target boundaries, remove dead legacy task-form UI paths if truly superseded, confirm service-layer ownership is stable, and verify transport/business/runtime separation is observable in code. This pass should also lock in naming consistency for conversation/chat/session terminology and ensure no new UI logic has leaked into repo/runtime packages.
  **Must NOT do**: Do not expand this pass into unrelated refactors. Do not remove compatibility code that tests or docs still depend on.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: targeted architecture hardening and cleanup across touched areas
  - Skills: [] - No extra skill required
  - Omitted: `['/ai-slop-remover']` - this is multi-file architecture cleanup, not single-file style polishing

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: [F1,F2,F3,F4] | Blocked By: [8,9]

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `internal/server/api.go:176-344` - original handler-heavy design to compare against final thin-handler state
  - Pattern: `internal/server/web/index.html:36-37,63-67` - original dashboard/new-task-first contract to remove or demote if superseded
  - Pattern: `internal/server/web/js/app.js:1078-1162` - original task submission/resume route behavior to ensure cleanup is intentional
  - Pattern: `docs/ADR-010-v5-web-ui.md:29-37` - deferred items that should still remain out of scope unless explicitly required
  - API/Type: `internal/domain/session.go`, `internal/store/session_store.go`, `internal/runtime/session_manager.go`, `internal/config/config.go` - final six-layer ownership checkpoints

  **Acceptance Criteria** (agent-executable only):
  - [ ] Final codebase demonstrates clear separation between UI transport, service orchestration, repo persistence, and runtime execution.
  - [ ] Deprecated legacy task-form-first paths are either removed or explicitly retained with documented rationale and tests.
  - [ ] Naming/ownership consistency check passes across conversation/session/message terminology.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Dependency-boundary audit
    Tool: Bash
    Steps: Run targeted grep/static checks over touched packages to verify HTTP/UI concerns do not appear inside repo/runtime/domain layers and service orchestration is not re-embedded in handlers.
    Expected: Boundary violations are absent or explicitly documented as temporary with tests.
    Evidence: .sisyphus/evidence/task-10-boundary-audit.txt

  Scenario: Legacy path cleanup check
    Tool: Bash
    Steps: Search for obsolete task-form-first selectors/routes/components and compare results against docs/tests.
    Expected: Remaining legacy paths are intentional, documented, and non-primary.
    Evidence: .sisyphus/evidence/task-10-boundary-audit-error.txt
  ```

  **Commit**: YES | Message: `refactor(v6): finalize six-layer boundary cleanup` | Files: [touched backend/web/doc files]

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Suggested commit slices:
  1. `feat(service): introduce chat conversation application layer`
  2. `feat(server): expose chat-first web and api flows`
  3. `test(web): add chat-first e2e and regression coverage`
  4. `docs(v6): document six-layer alignment and chat ux`
- If implementation lands in fewer commits, preserve separation between architecture/service work and UI/test work.

## Success Criteria
- Primary browser workflow feels and behaves like a conversation, not a task submission form.
- Six-layer mapping is explicit in code and docs, and transport/business/runtime responsibilities are easier to audit.
- Existing harness strengths (streaming, persistence, resume, inspect, verification) remain intact and are reachable from the chat workflow.
- Verification remains fully automated and CI-enforced.
