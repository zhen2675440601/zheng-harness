# ADR-010: v5 Web UI Scope Freeze

**Date**: 2026-05-07
**Status**: Proposed
**Related ADRs**: ADR-009 (v4 API Server Productization)

## Context

v4 established an HTTP API server with REST endpoints for session management, JWT authentication, SSE streaming, and concurrent session execution. The API surface is stable and production-ready.

However, all interactions remain programmatic: CLI commands and HTTP calls. There is no graphical interface for creating tasks, monitoring session progress, or inspecting completed results. ADR-009 explicitly deferred a Web UI / dashboard to v5 (lines 31-37), and ADR-009's Non-Goals section lists "Web UI: No dashboard, no visual session inspector, no interactive console" as out of scope for v4.

v5 introduces a **same-origin Web UI** that renders the harness as a browsable application. This ADR freezes the v5 Web UI scope and establishes the deployment model, page contract, authentication UX, streaming display rules, and testing strategy. All downstream v5 implementation tasks (T2 through T10) must conform to these decisions.

## Decision

### 1. v5 Scope Boundary

**In scope for v5**:

- Same-origin Web UI served by the same chi router on the same port as `/api/v1/*`
- Embedded static assets via Go `embed.FS` — no separate build toolchain
- Manual JWT token entry UX with localStorage persistence
- Live SSE consumption via browser `EventSource` API
- Session history rendering from `GET /api/v1/sessions/{id}/inspect` data
- `GET /api/v1/sessions` endpoint for paginated, filtered session listing
- Playwright-based E2E tests in a separate CI job

**Explicitly deferred to future versions**:

- Separate SPA build system (Vite, Webpack, Next.js, React, etc.)
- Separate frontend origin or second HTTP server
- WebSocket transport
- Polling replacement for live stream
- SSE replay or event buffering store
- Accounts, password login, OAuth, refresh-token system, or auth-cookie redesign
- v4 API contract breakage on existing routes

**Rationale**: v5 focuses on a **lean Web UI** that reuses the existing v4 API surface. No new backend infrastructure is required beyond a single list endpoint. The UI is a thin presentation layer over the existing REST + SSE contract.

### 2. Same-Origin Deployment Model

The Web UI is served by the same Go HTTP server on the same port.

**How it works**:

- All static assets (HTML, CSS, JavaScript, images) are embedded at compile time using Go's `embed.FS` directive.
- The chi router serves the UI under `/` or a dedicated prefix (e.g., `/ui/`) alongside the existing `/api/v1/*` routes.
- API routes remain at `/api/v1/*` with JWT authentication.
- No reverse proxy, no CORS configuration, no second listen port.

**Rationale**:

- Eliminates CORS entirely — browser and API share the same origin.
- Zero infrastructure overhead — single binary, single port, single deployment artifact.
- No Node.js/npm dependency in the build chain — `go build` produces everything.

**Build flow**:

```
.
├── cmd/server/        # existing main package, grows embed directive
├── web/               # NEW: static assets directory
│   ├── index.html
│   ├── css/
│   ├── js/
│   └── ...
└── internal/server/   # existing chi router, grows static file handler
```

The `cmd/server/main.go` file will include an `//go:embed` directive referencing the `web/` directory, and the `internal/server/api.go` router setup will register a file server for the embedded filesystem.

### 3. Page/Screen Contract

The Web UI consists of exactly four logical pages:

#### 3.1 Home / Dashboard (`GET /` or `/ui/`)

Displays the session list and the new-task form.

**Elements**:
- Task input form (task description, task type selector, optional provider/model/max-steps fields)
- Paginated session table with columns: ID (truncated), task description, status, created-at, actions
- "New Session" button
- Pagination controls (page number, page size)

**Data source**: `GET /api/v1/sessions?page=1&page_size=20&status=`

#### 3.2 Session Stream (`/ui/sessions/{id}/stream`)

Live view of a running session.

**Elements**:
- Real-time SSE event log
- Token delta display (accumulating text)
- Tool call visualization (tool name, input, output)
- Step progress indicator
- Status badge
- "Open in Detail" link (navigates to inspect page)

**Data source**: `EventSource` connected to `GET /api/v1/sessions/{id}/stream`

**Behavior**:
- Page loads → opens EventSource to the stream endpoint
- Events rendered as they arrive
- When `session_complete` event fires, EventSource closes and page offers a link to the detail view
- No event replay — if the session is already complete on page load, the user sees a message directing them to the detail page

#### 3.3 Session Detail (`/ui/sessions/{id}/detail`)

Read-only view of a completed or failed session's full history.

**Elements**:
- Session metadata (status, task, created/updated timestamps)
- Termination reason (if terminal)
- Full step list with tool name, input, output, observation
- Step-by-step expand/collapse
- "Copy Session ID" button
- "Back to Dashboard" link

**Data source**: `GET /api/v1/sessions/{id}/inspect`

**Rationale**: This page is always rendered from inspect data, never from SSE history. Completed sessions have no replayable stream.

#### 3.4 Token / Connect Panel

Overlay or sidebar for JWT authentication.

**Elements**:
- JWT token text input
- "Connect" button
- Connection status indicator (disconnected / connected / error)
- "Disconnect" button
- Token persistence indicator

**Behavior**:
- On first visit, shows the token entry panel
- Token is stored in `localStorage`
- On subsequent visits, the stored token is automatically applied
- All API calls include `Authorization: Bearer <token>` header
- If any API call returns 401, the connection status changes to "disconnected" and the token entry panel is shown again

**Rationale**: No server-side session, no cookies, no OAuth flow. The user obtains a JWT out of band (from server logs or admin CLI) and pastes it manually.

### 4. Session State Transitions in Browser

The UI maps each session status to a specific view and set of available actions.

| Status (domain) | API Status | UI Behaviour |
|-----------------|------------|--------------|
| `pending` | `created` | After form submission, redirect to stream page while showing "Starting session..." |
| `running` | `running` | Stream page displays live SSE events |
| `blocked_input` | `running` | Stream page displays live SSE events (same as running) |
| `success` | `completed` | Detail page renders full history from inspect; shows success badge |
| `verification_failed` | `failed` | Detail page renders full history from inspect; shows verification-failed badge |
| `budget_exceeded` | `failed` | Detail page renders full history from inspect; shows budget-exceeded badge |
| `fatal_error` | `failed` | Detail page renders full history from inspect; shows fatal-error badge |
| `interrupted` | `cancelled` | Detail page renders partial history from inspect; shows "Resume" button |

**Transition diagram**:

```
form submit → /ui/sessions/{id}/stream
  │
  ├─ session_complete received → link to detail page
  ├─ page refresh while running → reconnect EventSource (no replay)
  └─ page refresh after terminal → message: "Session complete, view detail"
```

**Resume flow**:
- On the detail page for `interrupted` sessions, a "Resume" button is shown.
- Clicking it calls `POST /api/v1/resume` with the session ID.
- On success (202), the browser navigates to the stream page.
- On error (409, 429), the error is displayed inline.

**Rationale**: The UI is reactive to the session's stored status. There is no server-pushed navigation — the browser issues API calls to determine the current state and renders accordingly. This keeps the UI stateless from the server's perspective.

### 5. Browser Auth Contract

**Authentication is entirely client-managed**.

**Flow**:

1. User obtains a JWT token (via server startup log, admin CLI, or operator distribution).
2. User opens the Web UI in a browser.
3. The Token/Connect panel prompts for the JWT.
4. User pastes the JWT and clicks "Connect".
5. The token is stored in `localStorage` under key `zheng_jwt`.
6. All subsequent `fetch()` and `EventSource` calls include the token in the `Authorization` header.
7. If a 401 response is received, `localStorage` is cleared and the token panel is shown again.

**No server-side auth state**:
- No session cookies.
- No OAuth redirects.
- No password fields.
- No refresh-token rotation.
- No CSRF tokens (same-origin + JWT header is sufficient).

**Connection-state UI**:
- A persistent indicator in the header/navbar shows the connection state.
- States: "Disconnected" (no token), "Connected" (token present, last API call succeeded), "Error" (last API call returned 401 or network error).
- The token panel is always accessible from the connection indicator.

**Rationale**: JWT manual entry mirrors the existing v4 auth model. The server already validates JWT on every request. This approach adds zero server-side auth infrastructure. localStorage is appropriate for a same-origin single-page application with no cross-origin concerns.

### 6. SSE and No-Replay Policy

**Live stream consumption**:

- The browser uses the standard `EventSource` API to connect to `GET /api/v1/sessions/{id}/stream`.
- Each incoming event is rendered immediately in the stream log.
- The UI supports the same six event types defined in ADR-009: `token_delta`, `tool_start`, `tool_end`, `step_complete`, `error`, `session_complete`.

**No replay policy** (inherited from ADR-009, section 8):

- If a client disconnects and reconnects while the session is still running, the client receives only future events.
- If a session has already completed when the stream page is loaded, the UI does not attempt to connect to the SSE endpoint. Instead, it shows a message directing the user to the detail page.

**Completed sessions render from inspect data**:

- The session detail page never uses SSE data. It always calls `GET /api/v1/sessions/{id}/inspect`.
- This is the same contract as CLI `inspect`: the inspect endpoint returns the full persisted session history.
- No event buffering, no replay store, no replay endpoint.

**Rationale**: Preserves the v4 design principle that SSE is for real-time observation only. Historical data comes from the inspect endpoint. This avoids server-side memory allocation for event buffers.

### 7. New Backend Surface

The only new backend route added for v5 is:

| Method | Path | Description | Auth Required |
|--------|------|-------------|---------------|
| GET | `/api/v1/sessions` | List sessions (paginated, filtered) | Yes |

**Query parameters**:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number (1-indexed) |
| `page_size` | int | 20 | Items per page (max 100) |
| `status` | string | "" | Filter by session status (`created`, `running`, `completed`, `failed`, `cancelled`) |

**Response (200 OK)**:

```json
{
  "sessions": [
    {
      "session_id": "session-1710000000000000000",
      "status": "completed",
      "task": "inspect repository and propose next step",
      "created_at": "2026-05-06T10:30:00Z",
      "updated_at": "2026-05-06T10:35:00Z"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 42
}
```

**No other new routes**. Specifically, no route is added for:
- Creating sessions (reuses `POST /api/v1/run`)
- Resuming sessions (reuses `POST /api/v1/resume`)
- Inspecting sessions (reuses `GET /api/v1/sessions/{id}/inspect`)
- Streaming sessions (reuses `GET /api/v1/sessions/{id}/stream`)
- Health checks (reuses `GET /healthz`)

**Rationale**: The UI is a consumer of the existing API surface. The only missing piece is a list endpoint, which the v4 API did not need (CLI users already know their session IDs from the `run` output).

### 8. Embedding Strategy

**Go `embed.FS`** is the sole mechanism for serving static assets.

**Directory layout**:

```
web/
├── index.html          # Main page (served at / or /ui/)
├── css/
│   └── app.css         # All styles (single file, no preprocessor)
├── js/
���   ├── app.js          # Main application logic
│   ├── api.js          # API client (fetch wrappers, EventSource management)
│   └── auth.js         # JWT token management (localStorage, header injection)
└── favicon.ico         # Favicon
```

**Serving**:

```go
//go:embed web/*
var webFS embed.FS

// In router setup:
r.Handle("/*", http.FileServer(http.FS(webFS)))
```

**No build toolchain**:
- No npm, no package.json, no node_modules.
- No TypeScript compilation, no JS bundling, no CSS preprocessing.
- JavaScript is vanilla ES6+ (browser-native `fetch`, `EventSource`, `class`, `async/await`).
- CSS is vanilla (no Sass, Less, Tailwind, or PostCSS).

**Rationale**: Zero build dependencies, zero compilation step. The UI code is committed as plain text files alongside Go source. `go build` produces a single binary that contains everything. This aligns with the Go philosophy of simple deployments.

### 9. Browser Testing

**Playwright-based E2E tests** in a dedicated CI job.

**Test scope**:

1. **Connect flow**: Navigate to UI, enter a valid JWT, verify connection indicator shows "Connected". Enter an invalid JWT, verify error state.
2. **Create and run session**: Fill task form, submit, verify redirect to stream page, verify SSE events appear.
3. **Stream and completion**: While session runs, verify token deltas and tool events render. After session completes, verify link to detail page.
4. **Inspect completed session**: Navigate to detail page, verify full step history renders, verify status badge is correct.
5. **Resume interrupted session**: Navigate to an interrupted session's detail page, click "Resume", verify redirect to stream page.
6. **Dashboard pagination**: Verify session list paginates correctly, filter by status works.
7. **Error paths**: Verify 401 (expired JWT) shows disconnect state, verify 429 (too many sessions) shows meaningful error.
8. **localStorage persistence**: Reload the page, verify stored token is reused.

**CI setup**:

```yaml
ui-tests:
  runs-on: ubuntu-latest
  steps:
    - name: Checkout
      uses: actions/checkout@v4
    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.26'
    - name: Build server
      run: go build ./cmd/server
    - name: Install Playwright
      run: npx playwright install chromium
    - name: Run UI tests
      run: go test ./internal/server/... -run TestUI -v
```

**Rationale**: Playwright provides reliable browser automation across Chromium, Firefox, and WebKit. A separate CI job keeps UI tests independent from the core runtime test suite. The `-run TestUI` convention limits UI test execution to that specific job.

### 10. Non-Goals

**Explicitly out of scope for v5**:

1. **No separate SPA build system**: No Vite, Webpack, Next.js, React, Vue, Svelte, or any npm-based compilation. The UI is vanilla JS served by Go `embed.FS`.
2. **No separate frontend origin**: No second HTTP server, no reverse proxy for a frontend app, no CORS configuration. Everything runs on one port.
3. **No WebSocket transport**: SSE is the only streaming transport. No bidirectional communication.
4. **No polling replacement for live stream**: The UI never polls for session progress. It uses SSE for live data and the inspect endpoint for historical data.
5. **No SSE replay store**: No server-side event buffering. Clients that miss events must use the inspect endpoint.
6. **No accounts, password login, OAuth, or refresh tokens**: Authentication remains JWT-only, identical to v4. The UI just exposes a token entry field.
7. **No auth-cookie redesign**: No server-managed sessions. No cookie-based auth.
8. **No v4 API contract breakage**: Existing routes (`POST /api/v1/run`, `POST /api/v1/resume`, `GET /api/v1/sessions/{id}/inspect`, `GET /api/v1/sessions/{id}/stream`, `GET /healthz`) remain completely unchanged in request/response shape and auth requirements.

**Rationale**: Every non-goal preserves the simplicity of the deployment model and prevents scope creep. The goal is a usable Web UI with minimal infrastructure, not a full-featured dashboard platform.

## Consequences

### Positive

- **Single binary deployment**: The entire application — API server and Web UI — ships as one binary. No static file server, no CDN, no build pipeline.
- **No CORS complexity**: Same-origin architecture eliminates all cross-origin concerns.
- **Minimal new backend surface**: Only one new endpoint (`GET /api/v1/sessions`) is needed. All other UI functionality reuses existing v4 routes.
- **Auth simplicity**: JWT manual entry requires zero server-side changes. The v4 auth middleware handles all authentication.
- **Testable with standard tools**: Playwright E2E tests validate the full user journey without mocking the backend.

### Negative

- **No session replay**: Users who navigate to a completed session's stream page cannot see the live stream — they must use the inspect detail page instead.
- **Manual JWT workflow**: Users must obtain and paste a JWT manually. This is acceptable for admin/operator use cases but not for public-facing applications.
- **Vanilla JS only**: Developers accustomed to React/Vue component models must work with vanilla JavaScript and DOM manipulation. No hot-reload development server.

### Neutral

- **Page navigation is client-side only**: The UI manages its own routing via URL hash or `history.pushState`. The server serves all routes from the same `index.html`.
- **localStorage-based token persistence**: Tokens survive page reloads but are cleared on browser data wipe. No server-side session affinity.
- **No mobile optimization**: The UI targets desktop browsers. Mobile responsiveness is not a v5 requirement.

## QA Scenarios

### E2E Scenario 1: Happy path — create, stream, inspect

1. User opens `http://localhost:8080/ui/`
2. Token panel is visible; user pastes a valid JWT and clicks "Connect"
3. Connection indicator shows "Connected"
4. User fills task form with "list directory contents" and clicks "Run"
5. Browser redirects to `/ui/sessions/{id}/stream`
6. SSE events appear: `token_delta`, `tool_start`, `tool_end`, `step_complete`
7. `session_complete` event arrives; link to detail page appears
8. User clicks the link, navigates to `/ui/sessions/{id}/detail`
9. Full step history is displayed with status badge "completed"

### E2E Scenario 2: Resume interrupted session

1. User navigates to dashboard; session list shows a session with status "cancelled"
2. User clicks the session row, navigates to detail page
3. Detail page shows partial step history and a "Resume" button
4. User clicks "Resume"; `POST /api/v1/resume` returns 202
5. Browser redirects to `/ui/sessions/{id}/stream`
6. New SSE events arrive; session completes

### E2E Scenario 3: Auth error

1. User opens UI; enters an invalid/expired JWT
2. Connection indicator shows "Error"
3. All API calls return 401; token panel remains visible
4. User enters a valid JWT and clicks "Connect"
5. Dashboard loads with session list

### E2E Scenario 4: Dashboard pagination

1. User opens dashboard with 50 stored sessions
2. Page 1 shows 20 sessions; pagination controls show pages 1, 2, 3
3. User clicks page 2; sessions 21-40 are displayed
4. User selects status filter "completed"; only completed sessions appear

### E2E Scenario 5: Session cap error

1. User submits a new task while the server has 8 running sessions
2. `POST /api/v1/run` returns 429
3. UI displays "Too many active sessions. Please wait and try again."

## References

- ADR-009: v4 API Server Productization (this ADR is the v5 successor to the deferred Web UI scope)
- ADR-006: Streaming Runtime Architecture (SSE event types inherited by v5 UI)
- `internal/server/api.go`: Existing v4 API handler implementations
- `cmd/server/server.go`: Chi router and route registration point
- `internal/domain/session.go`: Session status definitions (8 statuses, resumability rules)
- `internal/store/session_store.go`: Lifecycle classification and persistence layer
