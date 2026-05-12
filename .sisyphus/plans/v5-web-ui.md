# v5 Web UI

## TL;DR
> **Summary**: v5 adds a same-origin, server-hosted Web UI on top of the completed v4 HTTP API and SSE runtime so zheng-harness becomes directly usable from the browser without changing the core harness engine semantics.
> **Deliverables**:
> - Browser-accessible Web UI served by `cmd/server`
> - Session list API plus UI pages for run/resume/inspect/stream flows
> - Embedded static assets and dedicated UI handlers/routes
> - Browser automation test infrastructure and CI coverage
> - Docs/progress truth sync for v5 UI capability
> **Effort**: XL
> **Parallel**: YES - 4 waves
> **Critical Path**: T1 contract freeze → T2 server/web mount foundation → T4 UI shell → T6/T7/T8 browser workflows → T9 verification + CI → T10 docs

## Context
### Original Request
v4已经完成，按照计划v5应该干什么；随后要求直接生成计划。

### Interview Summary
- v4 is treated as complete and remains the backend foundation for v5.
- Existing v4 plan and ADR explicitly reserve v5 for **Web UI / dashboard** work.
- The repo currently has **no frontend assets, templates, static files, embedded assets, or browser routes**.
- The existing server/router/API/SSE surface in `cmd/server` and `internal/server` is the approved attachment point.
- The plan should avoid reopening v4 scope and should not invent a separate product/auth system.

### Metis Review (gaps addressed)
- Locked v5 to a **same-origin, server-hosted UI** instead of a separate SPA deployment, preventing unnecessary toolchain and CORS complexity.
- Added a required **session list API** because the current v4 API only supports single-session inspect and cannot power a dashboard/history view alone.
- Kept auth scoped to **browser-side JWT entry/persistence** and explicitly excluded accounts, OAuth, password flows, and session-cookie auth redesign.
- Made SSE limitations explicit: **no replay is added**; completed sessions are viewed through persisted `inspect` data, not stream history.
- Added mandatory **browser automation and CI** because the repo has no existing UI verification path.

## Work Objectives
### Core Objective
Expose the existing harness runtime through a minimal but production-usable browser interface served by the current Go server, while preserving v4 API compatibility, SSE-only streaming, fail-closed auth, and CLI/server dual-entrypoint architecture.

### Deliverables
1. Same-origin Web UI mounted on the existing `cmd/server` chi router.
2. Embedded static asset delivery for the Web UI without introducing a Node/Vite/Webpack SPA toolchain.
3. UI screens for:
   - JWT token bootstrap/connect
   - New task submission
   - Active session streaming view
   - Session inspect/detail view
   - Session list/history navigation
   - Resume action for eligible sessions
4. New list endpoint: `GET /api/v1/sessions` with pagination/filter support sufficient for the UI dashboard/history view.
5. Browser automation test harness plus CI job covering happy/error paths.
6. README / usage / progress / validation matrix updates that truthfully describe v5.

### Definition of Done (verifiable conditions with commands)
```bash
go build ./...
go test ./...
go test -race ./...
go test ./cmd/server/... -run "Test(ServerStartsAndExposesHealthEndpoint|WebUI)"
go test ./internal/server/... -run "Test(ListSessions|WebUI|Stream)"
npx playwright test --grep "WebUI"
curl -sS http://127.0.0.1:8080/
curl -sS -H "Authorization: Bearer test-token" "http://127.0.0.1:8080/api/v1/sessions?limit=20"
```

### Must Have
- Web UI is served by the existing Go HTTP server on the **same origin** as `/api/v1/*`.
- Existing v4 endpoints remain behavior-compatible:
  - `POST /api/v1/run`
  - `POST /api/v1/resume`
  - `GET /api/v1/sessions/{id}/inspect`
  - `GET /api/v1/sessions/{id}/stream`
- New `GET /api/v1/sessions` exists for dashboard/history needs.
- Browser auth UX uses **manual JWT token entry** and browser persistence; no new identity system is introduced.
- UI clearly distinguishes:
  - live stream state via SSE
  - historical/completed state via persisted inspect/list data
- Embedded/static asset delivery keeps `cmd/server` deployable as a single binary.
- Browser tests run automatically in CI.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- NO separate SPA build system (`vite`, `webpack`, `next`, `react`, etc.).
- NO separate frontend origin or second HTTP server.
- NO WebSocket transport, polling replacement for live stream, or SSE replay store.
- NO accounts, password login, OAuth, refresh-token system, or auth-cookie redesign.
- NO v4 API contract breakage on existing routes.
- NO UI logic embedded into `internal/server/api.go`; keep API and UI concerns separated.
- NO manual-only verification steps; all acceptance criteria must be agent-executable.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: TDD (RED-GREEN-REFACTOR) for server/UI handlers and browser flows.
- QA policy: every task includes executable happy-path and failure/edge-case scenarios.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`
- HTTP verification policy: existing `httptest` patterns from `internal/server/api_test.go` remain the base for API/server checks.
- Browser verification policy: Playwright drives exact selectors and confirms stream/list/inspect/auth behavior.
- CI verification policy: existing Go CI remains; add a dedicated browser job instead of folding UI checks into ad-hoc local steps.

## Execution Strategy
### Parallel Execution Waves
Wave 1: Contract freeze, server mount/config foundation, and list API backend
Wave 2: UI shell/assets/auth bootstrap plus task submit/resume workflows
Wave 3: live stream/history/detail UX and browser verification infrastructure
Wave 4: docs/progress/validation truth sync

### Dependency Matrix (full, all tasks)
| Task | Blocks | Blocked By |
|------|--------|------------|
| T1 v5 ADR + UI contract freeze | T2,T3,T4,T5,T6,T7,T8,T9,T10 | - |
| T2 Add web mount + config foundation | T4,T5,T6,T7,T8,T9,T10 | T1 |
| T3 Add session list API for dashboard/history | T6,T8,T9,T10 | T1 |
| T4 Add embedded UI shell and route handlers | T5,T6,T7,T8,T9,T10 | T1,T2 |
| T5 Add browser JWT bootstrap/persistence UX | T6,T7,T8,T9,T10 | T2,T4 |
| T6 Implement task submit + resume browser flows | T9,T10 | T3,T4,T5 |
| T7 Implement live SSE session stream view | T9,T10 | T4,T5 |
| T8 Implement session list + inspect/detail pages | T9,T10 | T3,T4,T5 |
| T9 Add browser automation + CI coverage | T10 | T3,T4,T5,T6,T7,T8 |
| T10 Docs/progress/validation sync | F1,F2,F3,F4 | T2,T3,T4,T5,T6,T7,T8,T9 |

### Agent Dispatch Summary
| Wave | Task Count | Categories |
|------|------------|------------|
| Wave 1 | 3 | writing, unspecified-high, deep |
| Wave 2 | 3 | deep, unspecified-high, deep |
| Wave 3 | 3 | deep, visual-engineering, unspecified-high |
| Wave 4 | 1 | writing |

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] T1. Freeze v5 Web UI ADR and browser contract

  **What to do**:
  1. Create a v5 ADR that fixes the Web UI scope, same-origin deployment model, JWT bootstrap UX, SSE/non-replay behavior, and the rule that completed sessions render from inspect data.
  2. Define the exact page set: dashboard/home, task form, session stream, session detail, token/connect panel.
  3. Define exact browser-state transitions for created/running/completed/failed/interrupted sessions.
  4. Document that the only new backend route required for UI is `GET /api/v1/sessions`.

  **Must NOT do**: Do not reopen v4 API semantics. Do not approve separate frontend deployment, WebSocket transport, or login/account scope.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: this is a contract/ADR freeze task with downstream architectural authority.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable for ADR work.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T2,T3,T4,T5,T6,T7,T8,T9,T10 | Blocked By: -

  **References**:
  - Pattern: `docs/ADR-009-v4-api-server-productization.md:21-37` - Prior version scope freeze and explicit v5 deferment.
  - Pattern: `.sisyphus/plans/v4-api-server-productization.md:22-24` - v4/v5 boundary already established.
  - API Contract: `internal/server/api.go:43-97` - Existing JSON request/response and error-envelope shapes.

  **Acceptance Criteria**:
  - [ ] v5 ADR explicitly states same-origin UI, embedded assets, JWT token entry UX, no replay, and no separate SPA toolchain.
  - [ ] The page/screen contract is fully enumerated and matches all downstream implementation tasks.
  - [ ] The ADR defines the new `GET /api/v1/sessions` endpoint purpose and non-goals.

  **QA Scenarios**:
  ```
  Scenario: ADR captures v5 scope and non-goals
    Tool: Bash
    Steps: Run `grep -n "same-origin\|no replay\|GET /api/v1/sessions\|No separate SPA\|JWT" docs/ADR-*.md`
    Expected: v5 ADR contains each required contract phrase exactly once or more
    Evidence: .sisyphus/evidence/task-1-v5-adr-scope.txt

  Scenario: v5 ADR does not leak forbidden auth/toolchain scope
    Tool: Bash
    Steps: Run `grep -n "OAuth\|password\|webpack\|vite\|WebSocket" docs/ADR-*.md`
    Expected: v5 ADR only references these items as exclusions/non-goals, not deliverables
    Evidence: .sisyphus/evidence/task-1-v5-adr-guardrails.txt
  ```

  **Commit**: YES | Message: `docs(adr): freeze v5 web ui contract` | Files: `docs/ADR-*.md`

- [x] T2. Add same-origin web mount and server/config foundation

  **What to do**:
  1. Extend server configuration with Web UI settings needed for enable/disable and web route behavior while keeping same-origin hosting as default.
  2. Add a dedicated web mount/handler registration path in `cmd/server` without disturbing the existing `/api/v1` route block.
  3. Add dedicated UI handler files/package structure so browser-serving concerns are separate from `internal/server/api.go`.
  4. Add server tests proving `/` and related UI routes are installed while `/api/v1/*` semantics remain unchanged.

  **Must NOT do**: Do not add a second server process, separate listen port, or CORS-driven split-origin architecture.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: server wiring/config changes are broad but structurally bounded.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Browser tooling not yet needed for the foundation task.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T4,T5,T6,T7,T8,T9,T10 | Blocked By: T1

  **References**:
  - Pattern: `cmd/server/server.go:168-194` - Existing runtime assembly and router registration attachment point.
  - Pattern: `cmd/server/server_test.go:45-125` - Server startup/handler-install verification style.
  - Config: `internal/config/config.go:53-60` - Existing `ServerSettings` extension point.

  **Acceptance Criteria**:
  - [ ] Server can register Web UI routes on the same chi router that already serves `/healthz` and `/api/v1/*`.
  - [ ] Existing `/api/v1/*` route behavior remains backward-compatible.
  - [ ] Web UI serving can be tested with `httptest` without booting a real browser.
  - [ ] Config defaults preserve single-binary, same-origin deployment.

  **QA Scenarios**:
  ```
  Scenario: Server exposes root UI route and retains API mount
    Tool: Bash
    Steps: Run `go test ./cmd/server/... -run TestWebUIRoutesMountedWithoutBreakingAPI -v`
    Expected: Test passes; `/` returns UI response and `/api/v1/*` routes remain registered
    Evidence: .sisyphus/evidence/task-2-web-mount.txt

  Scenario: API mount remains auth-protected after UI wiring
    Tool: Bash
    Steps: Run `go test ./internal/server/... -run TestRunRequiresAuthentication -v`
    Expected: Test passes; unauthenticated API requests still return 401
    Evidence: .sisyphus/evidence/task-2-api-auth-intact.txt
  ```

  **Commit**: YES | Message: `feat(server): mount same-origin web ui foundation` | Files: `cmd/server/**, internal/config/**, internal/server/**`

- [x] T3. Add session list API for dashboard and history views

  **What to do**:
  1. Add `GET /api/v1/sessions` with JWT auth, stable JSON shape, and pagination/filter parameters sufficient for dashboard/history rendering.
  2. Back the endpoint from persisted store truth so active and completed sessions both render without an in-memory actor requirement.
  3. Include fields needed by the UI list: session ID, task summary, task type, status, created/updated timestamps, and resumability indicator.
  4. Add store/API tests for filtering by status and pagination boundaries.

  **Must NOT do**: Do not expose unauthenticated session history. Do not require stream replay state for list rendering.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: endpoint semantics must align with persisted lifecycle truth and UI needs.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - API-first backend task.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T6,T8,T9,T10 | Blocked By: T1

  **References**:
  - Pattern: `internal/server/api.go:56-97` - Existing response-envelope conventions.
  - Pattern: `internal/server/api_test.go:27-92` - Authenticated route and accepted-response testing style.
  - State Contract: `docs/ADR-009-v4-api-server-productization.md:39-63` - Session lifecycle truth.

  **Acceptance Criteria**:
  - [ ] `GET /api/v1/sessions` returns authenticated, paginated session summaries from persisted data.
  - [ ] Response includes enough data for UI status badges and resume affordances.
  - [ ] Filtering/pagination edge cases return deterministic results and error handling.

  **QA Scenarios**:
  ```
  Scenario: Session list returns authenticated dashboard data
    Tool: Bash
    Steps: Run `go test ./internal/server/... -run TestListSessionsReturnsPaginatedSummaries -v`
    Expected: Test passes; response contains ordered session summaries with status and timestamps
    Evidence: .sisyphus/evidence/task-3-session-list.txt

  Scenario: Unauthenticated session list is rejected
    Tool: Bash
    Steps: Run `go test ./internal/server/... -run TestListSessionsRequiresAuthentication -v`
    Expected: Test passes; route returns 401 with standard error envelope
    Evidence: .sisyphus/evidence/task-3-session-list-auth.txt
  ```

  **Commit**: YES | Message: `feat(server): add session list api for web ui` | Files: `internal/server/**, internal/store/**`

- [x] T4. Build embedded Web UI shell and browser routes

  **What to do**:
  1. Add embedded static assets and/or server-rendered HTML shell served by the Go binary using Go-native embedding.
  2. Implement browser routes for home/dashboard, session detail, and any supporting asset paths.
  3. Ensure initial page load works without requiring a separate asset server or Node build output.
  4. Keep the shell minimal and task-focused: navigation, status regions, stream panel, list/detail containers.

  **Must NOT do**: Do not introduce React/Vue/Svelte, bundlers, npm-based SPA compilation, or mixed concerns inside `api.go`.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: this sets the concrete delivery mechanism and browser route structure for all UI flows.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Verification comes later.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T5,T6,T7,T8,T9,T10 | Blocked By: T1,T2

  **References**:
  - Mount Point: `cmd/server/server.go:180-193` - Existing router where UI routes must attach.
  - Package Boundary: `internal/server/api.go:29-39` - Server package ownership and API struct context.
  - Test Pattern: `cmd/server/server_test.go:60-118` - Handler installation and route behavior tests.

  **Acceptance Criteria**:
  - [ ] `GET /` returns the Web UI shell from the server binary.
  - [ ] UI routes resolve without breaking `/api/v1/*` and `/healthz`.
  - [ ] The shell includes stable selectors for token entry, task form, session list, stream panel, and inspect/detail sections.

  **QA Scenarios**:
  ```
  Scenario: Root route serves the embedded UI shell
    Tool: Bash
    Steps: Run `go test ./cmd/server/... -run TestRootServesEmbeddedWebUI -v`
    Expected: Test passes; root response is HTML and contains stable data-test selectors
    Evidence: .sisyphus/evidence/task-4-root-shell.txt

  Scenario: Unknown asset or UI route fails deterministically
    Tool: Bash
    Steps: Run `go test ./cmd/server/... -run TestWebUIUnknownRouteReturnsNotFound -v`
    Expected: Test passes; invalid route returns deterministic 404 behavior
    Evidence: .sisyphus/evidence/task-4-ui-404.txt
  ```

  **Commit**: YES | Message: `feat(web): serve embedded web ui shell` | Files: `cmd/server/**, internal/server/**`

- [x] T5. Add browser JWT bootstrap and persistence UX

  **What to do**:
  1. Add a browser-side token entry flow that lets the user paste a JWT and persist it for later API calls.
  2. Default to browser storage that survives refreshes but does not require new server cookie handling.
  3. Add connection-state UI that proves the token works against an authenticated endpoint before enabling normal workflows.
  4. Add clear invalid-token and disconnected-state rendering.

  **Must NOT do**: Do not add login credentials, server-issued cookies, refresh flows, or hidden auth magic.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: browser UX state is small in scope but high-risk if auth semantics drift.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Browser automation comes in T9.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T6,T7,T8,T9,T10 | Blocked By: T2,T4

  **References**:
  - Auth Contract: `internal/server/api.go:29-39` - API is JWT-protected and server-owned.
  - Existing Auth Behavior: `internal/server/api_test.go:27-36` - Unauthorized request contract.
  - Server Setting Source: `internal/config/config.go:53-60` - JWT settings remain server-side only.

  **Acceptance Criteria**:
  - [ ] User can enter a JWT in the browser and the UI persists it across reloads.
  - [ ] UI does not enable run/resume/list/inspect flows until auth bootstrap succeeds or at least stores a token.
  - [ ] Invalid or missing JWT produces visible, deterministic error state.

  **QA Scenarios**:
  ```
  Scenario: Valid JWT enables connected UI state
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/'); page.getByLabel('JWT Token').fill('test-token'); page.getByRole('button', { name: 'Connect' }).click(); page.waitForSelector('[data-test="auth-status-connected"]')`
    Expected: Connected indicator appears and task form becomes enabled
    Evidence: .sisyphus/evidence/task-5-auth-connected.txt

  Scenario: Invalid JWT shows auth failure state
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/'); page.getByLabel('JWT Token').fill('bad-token'); page.getByRole('button', { name: 'Connect' }).click(); page.waitForSelector('[data-test="auth-error"]')`
    Expected: Auth error region renders and submission controls remain disabled
    Evidence: .sisyphus/evidence/task-5-auth-error.txt
  ```

  **Commit**: YES | Message: `feat(web): add jwt bootstrap and persistence ux` | Files: `cmd/server/**, internal/server/**`

- [x] T6. Implement browser task submission and resume workflows

  **What to do**:
  1. Add task form UX for `task`, `task_type`, optional provider/model/max_steps/verify_mode fields that map to the v4 API contract.
  2. On submit, call `POST /api/v1/run`, capture the returned `session_id`, and route the user into the live session view.
  3. Add resume affordances that call `POST /api/v1/resume` only for eligible sessions from detail/history views.
  4. Render deterministic validation and conflict/rate-limit errors in the UI.

  **Must NOT do**: Do not invent browser-only task semantics or bypass API validation rules.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: form/submit/resume workflows must align exactly with backend contracts and lifecycle semantics.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Full browser checks are centralized in T9.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T9,T10 | Blocked By: T3,T4,T5

  **References**:
  - Run Contract: `internal/server/api.go:43-60` - `runRequest` and accepted response shape.
  - Run/Inspect Behavior: `internal/server/api_test.go:38-92` - Request/response semantics for run.
  - Resume Lifecycle: `docs/ADR-009-v4-api-server-productization.md:57-75` - Resume rules and route definitions.

  **Acceptance Criteria**:
  - [ ] Browser task submission maps exactly to `POST /api/v1/run` and redirects/updates into the created session view.
  - [ ] Resume action is shown only where appropriate and correctly handles `202`, `404`, `409`, and `429` responses.
  - [ ] Empty/invalid form values show deterministic validation feedback.

  **QA Scenarios**:
  ```
  Scenario: Submit task from browser and enter live session view
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/'); page.getByLabel('JWT Token').fill('test-token'); page.getByRole('button', { name: 'Connect' }).click(); page.getByLabel('Task').fill('inspect repository and propose next step'); page.getByLabel('Task Type').selectOption('coding'); page.getByRole('button', { name: 'Run Task' }).click(); page.waitForSelector('[data-test="session-stream"]'); page.waitForSelector('[data-test="session-id"]')`
    Expected: Stream view opens with non-empty session ID and active session status
    Evidence: .sisyphus/evidence/task-6-run-flow.txt

  Scenario: Empty task submission shows validation error
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/'); page.getByLabel('JWT Token').fill('test-token'); page.getByRole('button', { name: 'Connect' }).click(); page.getByRole('button', { name: 'Run Task' }).click(); page.waitForSelector('[data-test="task-validation-error"]')`
    Expected: Validation error appears and no session view is entered
    Evidence: .sisyphus/evidence/task-6-validation-error.txt
  ```

  **Commit**: YES | Message: `feat(web): add browser run and resume workflows` | Files: `cmd/server/**, internal/server/**`

- [x] T7. Implement live SSE stream view and degraded-state handling

  **What to do**:
  1. Build the browser stream panel that consumes the existing SSE endpoint and renders `token_delta`, `tool_start`, `tool_end`, `step_complete`, `error`, and `session_complete` events.
  2. Show explicit UI states for connecting, streaming, overflow/error, disconnect, and completed terminal states.
  3. When a session is terminal or the browser reconnects after completion, transition to inspect/detail rendering rather than pretending replay exists.
  4. Keep stream rendering isolated from history/detail logic so no-replay semantics stay honest.

  **Must NOT do**: Do not implement WebSocket fallback, hidden polling, or fake replay of events that were never received.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: this task translates internal SSE semantics into precise browser behavior.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Browser verification is captured by T9.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T9,T10 | Blocked By: T4,T5

  **References**:
  - SSE Writer Contract: `internal/server/api.go:99-109` - SSE serialization helper boundary.
  - Streaming Flow Test: `internal/server/api_test.go:94-197` - Ordered stream behavior and disconnect semantics.
  - Scope Guardrail: `docs/ADR-009-v4-api-server-productization.md:31-35` - WebSocket explicitly deferred.

  **Acceptance Criteria**:
  - [ ] Browser renders all six SSE event types with deterministic selectors/state transitions.
  - [ ] Disconnect/error state is visible and does not imply the session was cancelled.
  - [ ] Terminal session state hands off to persisted detail rendering instead of replay assumptions.

  **QA Scenarios**:
  ```
  Scenario: Live session stream renders ordered events to the browser
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/'); page.getByLabel('JWT Token').fill('test-token'); page.getByRole('button', { name: 'Connect' }).click(); page.getByLabel('Task').fill('authenticated integration'); page.getByLabel('Task Type').selectOption('coding'); page.getByRole('button', { name: 'Run Task' }).click(); page.waitForSelector('[data-test="stream-event-token_delta"]'); page.waitForSelector('[data-test="stream-event-step_complete"]'); page.waitForSelector('[data-test="stream-event-session_complete"]')`
    Expected: Ordered event markers render and final completion state appears
    Evidence: .sisyphus/evidence/task-7-live-stream.txt

  Scenario: Stream disconnect shows degraded-state banner
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/'); /* connect and enter a live session */ ; page.context().setOffline(true); page.waitForSelector('[data-test="stream-disconnected"]')`
    Expected: Disconnect banner appears without falsely marking the session cancelled
    Evidence: .sisyphus/evidence/task-7-stream-disconnect.txt
  ```

  **Commit**: YES | Message: `feat(web): add live sse session stream view` | Files: `cmd/server/**, internal/server/**`

- [x] T8. Implement dashboard history and inspect/detail browser views

  **What to do**:
  1. Build a session list/dashboard view backed by `GET /api/v1/sessions` with status filters and pagination controls.
  2. Build a session detail/inspect view that renders persisted plan, steps, provenance, and terminal reason from `GET /api/v1/sessions/{id}/inspect`.
  3. Show resume affordance only for resumable states and completed/failure badges based on persisted truth.
  4. Ensure deep-linking into a completed session works even without an open SSE stream.

  **Must NOT do**: Do not rely on in-memory stream state to render completed history. Do not hide backend failure/termination reasons.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: this is primarily browser-facing information architecture and status presentation with concrete backend data.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Verification is consolidated later.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T9,T10 | Blocked By: T3,T4,T5

  **References**:
  - Inspect Shape: `internal/server/api.go:62-97` - Session detail JSON contract.
  - Inspect/Stream Example: `internal/server/api_test.go:168-197` - Completed session inspect assertions.
  - Session Lifecycle: `docs/ADR-009-v4-api-server-productization.md:41-63` - Status meaning and terminal behavior.

  **Acceptance Criteria**:
  - [ ] Dashboard lists persisted sessions with stable status badges and links.
  - [ ] Detail view renders inspect data for completed and non-completed sessions.
  - [ ] Deep-linking to a completed session works without requiring an active SSE connection.

  **QA Scenarios**:
  ```
  Scenario: Dashboard lists sessions and opens completed detail view
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/'); page.getByLabel('JWT Token').fill('test-token'); page.getByRole('button', { name: 'Connect' }).click(); page.waitForSelector('[data-test="session-list"]'); page.getByRole('link', { name: /session-/ }).first().click(); page.waitForSelector('[data-test="session-detail"]')`
    Expected: Session list renders and clicking a session opens detail content from inspect data
    Evidence: .sisyphus/evidence/task-8-dashboard-detail.txt

  Scenario: Completed session detail renders without live stream dependency
    Tool: Playwright
    Steps: `page.goto('http://127.0.0.1:8080/sessions/test-completed'); page.waitForSelector('[data-test="session-detail-status-completed"]'); page.waitForSelector('[data-test="inspect-plan"]')`
    Expected: Completed badge and inspect plan render without requiring stream connection
    Evidence: .sisyphus/evidence/task-8-completed-detail.txt
  ```

  **Commit**: YES | Message: `feat(web): add dashboard and inspect detail views` | Files: `cmd/server/**, internal/server/**`

- [x] T9. Add browser automation infrastructure and CI coverage for v5

  **What to do**:
  1. Add Playwright-based end-to-end/browser verification for the Web UI.
  2. Add test harness startup/teardown that boots the local Go server with deterministic JWT config and test data.
  3. Cover happy-path connect → run → stream → inspect as well as invalid JWT, validation failure, and disconnect/degraded-state cases.
  4. Extend GitHub Actions with a dedicated browser job that installs required browser tooling and executes the UI suite.

  **Must NOT do**: Do not rely on manual local browser testing. Do not replace existing Go CI; add browser verification beside it.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: this spans test harnessing, workflow wiring, and deterministic CI execution.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - The executor should still use browser automation, but the planning category stays general.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T10 | Blocked By: T3,T4,T5,T6,T7,T8

  **References**:
  - CI Baseline: `.github/workflows/ci.yml:10-52` - Existing Go-only verification job to extend.
  - Server Test Harness Pattern: `cmd/server/server_test.go:60-96` - Dependency-injected server startup shape.
  - API Integration Pattern: `internal/server/api_test.go:94-197` - Realistic run/stream/inspect flow baseline.

  **Acceptance Criteria**:
  - [ ] Browser suite can boot against the local Go server and run deterministically in CI.
  - [ ] CI contains a separate browser verification job in addition to existing Go checks.
  - [ ] Browser suite covers at least connect/auth, task submission, stream completion, detail rendering, and an error/degraded path.

  **QA Scenarios**:
  ```
  Scenario: Playwright suite passes for Web UI flows
    Tool: Bash
    Steps: Run `npx playwright test --grep "WebUI"`
    Expected: Browser suite passes with run/stream/detail/auth coverage
    Evidence: .sisyphus/evidence/task-9-playwright.txt

  Scenario: CI workflow includes browser verification job
    Tool: Bash
    Steps: Run `grep -n "playwright\|browser\|ui" .github/workflows/ci.yml`
    Expected: CI file contains a dedicated browser/UI verification job or step group
    Evidence: .sisyphus/evidence/task-9-ci-browser.txt
  ```

  **Commit**: YES | Message: `test(web): add browser automation and ci coverage` | Files: `.github/workflows/**, cmd/server/**, internal/server/**, e2e/**`

- [x] T10. Sync documentation, progress, and validation truth for v5

  **What to do**:
  1. Update README to describe the new Web UI capability, same-origin serving, and JWT bootstrap UX.
  2. Update usage docs with exact startup, browser access, and test commands.
  3. Update progress/validation matrix so v5 is represented truthfully and v4/v5 boundaries stay visible.
  4. Document known non-goals: no replay, no separate frontend app, no login/account system.

  **Must NOT do**: Do not claim WebSocket, replay, or separate SPA support exists. Do not omit the new browser test requirement.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: this is documentation truth-sync work across project-facing files.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not needed for the doc changes themselves.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: F1,F2,F3,F4 | Blocked By: T2,T3,T4,T5,T6,T7,T8,T9

  **References**:
  - Pattern: `README.md` - Existing versioned capability summary and quick-start flow.
  - Pattern: `docs/USAGE.md` - Existing CLI/API usage surface.
  - Pattern: `docs/validation-matrix.md` - Existing verification-truth file.
  - Pattern: `.sisyphus/plans/v4-api-server-productization.md:495-534` - Prior docs/progress sync expectations.

  **Acceptance Criteria**:
  - [ ] README, usage docs, progress, and validation matrix all describe the Web UI consistently.
  - [ ] Docs explicitly state no replay, no separate frontend app, and JWT bootstrap expectations.
  - [ ] Verification commands include browser automation coverage.

  **QA Scenarios**:
  ```
  Scenario: Documentation reflects v5 Web UI truth
    Tool: Bash
    Steps: Run `grep -n "Web UI\|JWT\|SSE\|no replay\|Playwright" README.md docs/USAGE.md PROGRESS.md docs/validation-matrix.md`
    Expected: All required files mention the implemented v5 capabilities and non-goals consistently
    Evidence: .sisyphus/evidence/task-10-docs-sync.txt

  Scenario: Docs do not overclaim unsupported v5 features
    Tool: Bash
    Steps: Run `grep -n "WebSocket\|OAuth\|React\|Vite\|event replay" README.md docs/USAGE.md PROGRESS.md docs/validation-matrix.md`
    Expected: Any matches are exclusions/non-goals only, not advertised features
    Evidence: .sisyphus/evidence/task-10-docs-guardrails.txt
  ```

  **Commit**: YES | Message: `docs: sync v5 web ui capability and validation` | Files: `README.md, PROGRESS.md, docs/USAGE.md, docs/validation-matrix.md`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer one commit per task where the task changes a coherent surface area.
- Keep API/backend commits separate from UI rendering commits where feasible to preserve rollback clarity.
- Do not squash T3 session-list backend work into purely UI commits; the new API surface must stay auditable.
- Keep browser test/CI work in a dedicated testing commit unless tightly coupled to selectors introduced in the immediately preceding task.

## Success Criteria
- Starting `cmd/server` exposes a usable browser UI at `/` without any separate frontend process.
- A user can enter a JWT, submit a task, watch live SSE progress, inspect history, and resume eligible sessions entirely from the browser.
- Existing v4 API routes continue to function with their documented auth/status-code behavior.
- Completed sessions remain visible through inspect/history views even though SSE replay is still unsupported.
- CI verifies both Go backend behavior and browser UI flows automatically.
