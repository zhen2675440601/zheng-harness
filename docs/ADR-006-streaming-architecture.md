# ADR-006: Streaming Architecture with Callback-Based EventChannel

## Status
Accepted

## Context
The agent runtime needs to stream token deltas, tool lifecycle events, and step boundaries back to the UI/CLI in real-time. Early discussions considered a separate `StreamingModel` interface mirroring the non-streaming `Model` interface, but this would duplicate interfaces, fragment the Provider boundary, and complicate fallback behavior for providers that do not support native streaming.

## Decision
We use a **callback-based `Stream()` method with `EventChannel`** across all providers, rather than a separate `StreamingModel` interface. The `Model` interface exposes both `Generate()` for batch responses and `Stream()` for event-driven delivery. Non-streaming providers wrap their `Generate()` output into a single `TokenDelta` event followed by `SessionComplete`.

### Event Types
The `EventChannel` carries a discriminated union of event types:
- **TokenDelta**: Incremental text token from LLM (contains `delta`, `stepIndex`, `timestamp`)
- **ToolStart**: Tool execution begins (contains `toolName`, `toolCallId`, `arguments`, `stepIndex`)
- **ToolEnd**: Tool execution completes (contains `toolCallId`, `output`, `error`, `stepIndex`)
- **StepComplete**: Step finalized (contains `stepIndex`, `planUpdate`, `nextAction`)
- **Error**: Recoverable or terminal error (contains `message`, `severity`, `stepIndex`)
- **SessionComplete**: Session finalized (contains `finalResponse`, `totalSteps`, `status`)

### Ordering Guarantees
- Events **within a step** are strictly ordered by emission time
- Events **across steps** are ordered by `StepIndex` (step N completes before step N+1 starts)
- The runtime processes events sequentially per step; inter-step ordering is guaranteed by the step loop

## Consequences
- The Provider interface remains cohesive; implementers add streaming without duplicating non-streaming logic.
- Non-streaming providers automatically gain a streaming facade for UI consistency.
- Persistence layer stores only final Step and Session state; intermediate token deltas are NOT persisted.
- Resume behavior reconstructs from persisted steps; deltas are not replayed.
- UI/CLI subscribers receive real-time feedback without polling.
- Testing can assert on event sequences using in-memory `EventChannel` captures.

## References
- ADR-001: Single-Process Single-Agent Runtime
- ADR-002: SQLite Persistence and Constrained Memory
- Internal: `internal/domain/events.go`, `internal/llm/provider.go`
