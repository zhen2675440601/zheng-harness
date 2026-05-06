## 2026-05-06

- Added `internal/runtime/session_manager.go` as a session-actor layer above the existing single-session engine: one actor owns one goroutine, one event channel, and one runner/engine instance.
- Manager defaults align with ADR/plan requirements: active session cap `8`, shutdown timeout `30s`, duplicate active session rejection via `ErrSessionAlreadyActive`, and cap rejection via `ErrActiveSessionLimit`.
- Actor lifecycle is normalized as `pending -> running -> completed|failed|cancelled`; cancellation maps from context cancellation/deadline and interrupted runtime sessions.
- Shutdown semantics stop new admissions first, wait for active actors, then cancel remaining actors and rely on finalizer persistence to record terminal interrupted/cancelled outcomes.
- Added targeted tests for concurrent isolation, duplicate resume/start rejection, active-cap enforcement, and graceful shutdown cancellation.
