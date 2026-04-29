# v2 Evolution - Decisions

## Decisions

- Implemented provider-native streaming in OpenAI, Anthropic, and DashScope adapters using `stream: true` plus SSE parsing instead of wrapping all providers through fallback only.
- Kept provider stream completion semantics simple for T3 by emitting provider token deltas as `domain.TokenDelta(0, chunk)` and a terminal `domain.SessionComplete("", "success")` event after SSE completion.
- Introduced `EventPayload` in `internal/domain/events.go` as a typed raw JSON carrier to preserve structured payload decoding while staying inside domain-package guardrails.

- 2026-04-28: Chose context-based stream emitter plumbing inside internal/runtime to keep domain.Model unchanged while enabling end-to-end RunStream token delivery.
- 2026-04-29: Extended `PluginTool` with `Capabilities() []string` and enforced `Runtime.PluginCapabilities` through `PluginManager.Policy` so capability policy is checked at load time rather than deferred to tool execution.
