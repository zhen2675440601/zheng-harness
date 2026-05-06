## Learnings
- The vendored chi shim only exposed GET initially; T5 required extending the local chi replacement with middleware support, nested routes, POST handlers, and path parameter extraction so the API server contract could be expressed without adding an external dependency.
- Persisted inspect support is best exposed from `SQLiteSessionStore` as a dedicated `InspectSession` aggregate so HTTP handlers can read session/task/plan/steps without depending on CLI formatting code.

## Decisions
- Implemented JWT auth as a small HS256 validator using the existing configured secret, enforcing `sub`, `iat`, `exp`, and the ADR-009 24 hour maximum lifetime requirement.
- Standardized API failures into a single JSON envelope with `error.code`, `error.message`, and request ID, and mapped active-session-cap pressure to `429` with `Retry-After`.

## Issues
- LSP diagnostics still report stale chi interface errors even after the vendored shim and package tests compile successfully; `go test ./cmd/server ./internal/server ./internal/store` is currently the reliable verification signal.
