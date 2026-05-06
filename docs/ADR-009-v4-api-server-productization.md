# ADR-009: v4 API Server Productization

**Date**: 2026-05-06
**Status**: Proposed
**Related ADRs**: ADR-008 (v3 Extensibility Boundaries), ADR-006 (Streaming Runtime Architecture), ADR-003 (No Plugin System in v1)

## Context

v3 established a robust plugin system with three families (provider, verifier, agent-strategy) and fail-closed runtime semantics. However, all interactions remain CLI-only: `run`, `resume`, and `inspect` commands operate through a local terminal interface.

v4 introduces an **HTTP API server** that exposes the harness runtime as a networked service. This productization step is critical for:

1. **External integration**: Enable CI/CD pipelines, IDE extensions, and orchestration platforms to interact with the harness programmatically
2. **Session lifecycle management**: Provide RESTful endpoints for creating, monitoring, and controlling agent sessions
3. **Streaming output**: Deliver real-time token deltas and tool events via Server-Sent Events (SSE)
4. **Authentication and authorization**: Enforce JWT-based access control for all API routes
5. **Operational predictability**: Define concrete HTTP status codes, rate limits, and concurrency caps

This ADR freezes the external API contract for v4. All subsequent implementation must conform to these decisions. Deviations require a new ADR.

## Decision

### 1. v4 Scope Boundary

**In scope for v4**:
- HTTP API server with REST endpoints for session management
- JWT authentication for all `/api/v1/*` routes
- SSE streaming for real-time session events
- Health check endpoint for load balancer probes

**Explicitly deferred to v5**:
- Web UI / dashboard
- WebSocket support
- GraphQL API
- JSON-RPC gateway

**Rationale**: v4 focuses on API server productization, not core extensibility. The runtime remains unchanged from v3. We are exposing existing capabilities via HTTP, not adding new plugin families or modifying the plan-execute-verify loop.

### 2. Session Lifecycle

Sessions follow a strict state machine:

```
created -> running -> completed | failed | cancelled
                |
                v
            (resume allowed only if not completed)
```

**State definitions**:
- `created`: Session initialized, awaiting first step execution
- `running`: At least one step in progress, not yet terminal
- `completed`: Terminal state, all steps finished successfully
- `failed`: Terminal state, unrecoverable error occurred
- `cancelled`: Terminal state, user or system initiated cancellation

**Resume semantics**:
- Resume is **only permitted** for sessions in `running` state that have not reached a terminal state
- Attempting to resume a `completed` session returns `409 Conflict`
- Attempting to resume a `failed` or `cancelled` session returns `409 Conflict`

**Rationale**: Prevents duplicate work and ensures session integrity. Once a session reaches a terminal state, its history is immutable.

### 3. REST Endpoints

v4 defines the following HTTP routes:

| Method | Path                           | Description                          | Auth Required |
|--------|--------------------------------|--------------------------------------|---------------|
| POST   | `/api/v1/run`                  | Create and start a new session       | Yes           |
| POST   | `/api/v1/resume`               | Resume an existing running session   | Yes           |
| GET    | `/api/v1/sessions/{id}/inspect`| Get full session state and history   | Yes           |
| GET    | `/api/v1/sessions/{id}/stream` | SSE stream for session events        | Yes           |
| GET    | `/healthz`                     | Health check (no auth)               | No            |

**Path conventions**:
- All API routes prefixed with `/api/v1` for future versioning
- Session IDs are opaque strings (e.g., `session-1710000000000000000`)
- Health check is unauthenticated for load balancer probes

### 4. Request/Response JSON Shapes

#### POST /api/v1/run

**Request body**:
```json
{
  "task": "inspect repository and propose next step",
  "task_type": "coding",
  "provider": "dashscope",
  "model": "qwen3.6-plus",
  "max_steps": 8,
  "verify_mode": "standard"
}
```

**Response (202 Accepted)**:
```json
{
  "session_id": "session-1710000000000000000",
  "status": "created",
  "stream_url": "/api/v1/sessions/session-1710000000000000000/stream"
}
```

**Rationale**: Returns `202 Accepted` instead of `201 Created` because the session is queued for execution, not immediately complete. The `stream_url` provides a direct link to SSE events.

#### POST /api/v1/resume

**Request body**:
```json
{
  "session_id": "session-1710000000000000000"
}
```

**Response (202 Accepted)**:
```json
{
  "session_id": "session-1710000000000000000",
  "status": "running",
  "stream_url": "/api/v1/sessions/session-1710000000000000000/stream"
}
```

#### GET /api/v1/sessions/{id}/inspect

**Response (200 OK)**:
```json
{
  "session_id": "session-1710000000000000000",
  "status": "running",
  "task": "inspect repository and propose next step",
  "task_type": "coding",
  "created_at": "2026-05-06T10:30:00Z",
  "updated_at": "2026-05-06T10:35:00Z",
  "steps": [
    {
      "step_number": 1,
      "tool_name": "code_search",
      "tool_input": {"query": "func main", "lang": "go"},
      "observation": "Found 3 matches",
      "status": "completed"
    }
  ],
  "provenance": {
    "provider_plugin": null,
    "verifier_plugin": null,
    "agent_strategy_plugin": null
  }
}
```

**Rationale**: Includes full step history and plugin provenance for auditability. Matches the CLI `inspect --json` output structure.

### 5. HTTP Status Code Mapping

| Code | Meaning                    | When Returned                                              |
|------|----------------------------|------------------------------------------------------------|
| 202  | Accepted                   | Session created or resumed successfully                    |
| 200  | OK                         | `/healthz` or `/inspect` succeeded                         |
| 401  | Unauthorized               | JWT missing, expired, or invalid                           |
| 404  | Not Found                  | Session ID does not exist                                  |
| 409  | Conflict                   | Duplicate resume attempt on running session                |
| 429  | Too Many Requests          | Active session cap exceeded (default: 8)                   |
| 500  | Internal Server Error      | Unrecoverable server error                                 |

**409 Conflict semantics**:
- Returned when attempting to resume a session that is already in `running` state
- Returned when attempting to resume a session that has reached a terminal state (`completed`, `failed`, `cancelled`)

**429 Too Many Requests semantics**:
- Returned when the number of concurrent `running` sessions exceeds the configured cap (default: 8)
- Includes `Retry-After` header with seconds to wait

### 6. Authentication

**JWT mandatory** for all `/api/v1/*` routes.

**Token format**: HS256-signed JWT with the following claims:
- `sub`: User or service identifier
- `iat`: Issued-at timestamp
- `exp`: Expiration timestamp (max 24 hours)

**Configuration**:
- Secret key loaded from `API_JWT_SECRET` environment variable
- `/healthz` endpoint is **unauthenticated** for load balancer probes

**Rationale**: JWT provides stateless authentication suitable for horizontal scaling. The health check exemption allows external load balancers to probe without credentials.

### 7. SSE-Only Streaming

All real-time events are delivered via **Server-Sent Events** (SSE) over the `/api/v1/sessions/{id}/stream` endpoint.

**Event types**:
- `token_delta`: Incremental LLM token (data: `{"delta": "..."}`)
- `tool_start`: Tool execution beginning (data: `{"tool_name": "...", "input": {...}}`)
- `tool_end`: Tool execution completed (data: `{"tool_name": "...", "output": "...", "duration_ms": 123}`)
- `step_complete`: Step finished (data: `{"step_number": 1, "observation": "..."}`)
- `error`: Error event (data: `{"message": "...", "code": "..."}`)
- `session_complete`: Session finished (data: `{"status": "completed", "summary": "..."}`)

**SSE format**:
```
event: token_delta
data: {"delta": "Hello"}

event: tool_start
data: {"tool_name": "code_search", "input": {"query": "func main"}}
```

**Rationale**: SSE provides built-in reconnection logic and is simpler than WebSocket for one-way streaming. JSON payload format ensures consistency with REST responses.

### 8. No-Replay Policy

**SSE streams do not replay past events**.

If a client disconnects and reconnects:
- **Session still running**: Client receives only **future events** from the reconnection point
- **Session completed**: Stream closes immediately with `session_complete` event

**Rationale**: Replay would require buffering all events server-side, increasing memory pressure. Clients are expected to use `/inspect` to retrieve historical state. SSE is for **real-time observation only**.

### 9. Duplicate Resume Rejection

**Policy**: Attempting to resume a session that is already `running` returns `409 Conflict`.

**Example**:
```
POST /api/v1/resume
Content-Type: application/json
Authorization: Bearer <jwt>

{"session_id": "session-123"}

# Response if session-123 is already running:
HTTP/1.1 409 Conflict
Content-Type: application/json

{"error": "session already running", "session_id": "session-123"}
```

**Rationale**: Prevents accidental duplicate execution. Users must explicitly cancel a running session before resuming.

### 10. Operational Defaults

| Setting              | Default Value | Description                                    |
|----------------------|---------------|------------------------------------------------|
| Port                 | 8080          | HTTP server listen port                        |
| Active session cap   | 8             | Maximum concurrent `running` sessions          |
| Shutdown timeout     | 30s           | Graceful shutdown wait for running sessions    |
| JWT expiration max   | 24h           | Maximum token lifetime                         |
| SSE keepalive        | 15s           | Comment ping to prevent proxy timeout          |

**Graceful shutdown**:
- On SIGTERM, server stops accepting new `/run` and `/resume` requests
- Existing `running` sessions are given up to 30s to complete
- After timeout, all sessions are marked `cancelled` and server exits

### 11. Non-Goals

**Explicitly out of scope for v4**:

1. **Web UI**: No dashboard, no visual session inspector, no interactive console
2. **WebSocket**: No bidirectional communication, SSE is the only streaming protocol
3. **GraphQL**: No query language, REST is the only API style
4. **JSON-RPC**: No RPC gateway, only REST endpoints
5. **Model switching mid-session**: Once a session starts, its provider and model are fixed
6. **Plugin hot-reload**: Plugin configuration is frozen at session creation time

**Rationale**: v4 is about **productization of the API server**, not expanding the feature set. These non-goals preserve focus and prevent scope creep.

## Consequences

### Positive

- **Clear API contract**: External integrators have a stable target for v4
- **Operational predictability**: Concrete defaults for port, concurrency, and timeouts
- **Security baseline**: JWT authentication prevents unauthorized access
- **Streaming simplicity**: SSE is easier to implement and debug than WebSocket

### Negative

- **No Web UI**: Users must rely on CLI or build their own dashboard
- **No replay**: Clients cannot rejoin a stream from the beginning
- **Stateless auth**: JWT revocation requires token expiration or server-side blacklist

### Neutral

- **Migration effort**: Existing CLI users must adapt to HTTP API for automation
- **JWT management**: Users must generate and rotate tokens securely

## Migration from v3

v3 CLI commands remain fully functional. The HTTP API server is an **additional interface**, not a replacement.

**CLI to API mapping**:
- `agent run --task "..."` → `POST /api/v1/run`
- `agent resume --session <id>` → `POST /api/v1/resume`
- `agent inspect --session <id>` → `GET /api/v1/sessions/{id}/inspect`
- `agent run --stream` → `GET /api/v1/sessions/{id}/stream`

## References

- ADR-006: Streaming Runtime Architecture (SSE event types, token delta format)
- ADR-008: v3 Extensibility Boundaries (plugin families, fail-closed semantics)
- `internal/domain/session.go`: Session state machine definition
- `internal/runtime/server.go`: HTTP server implementation (to be added in v4)
