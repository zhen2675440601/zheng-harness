# v6 Chat-UI + Six-Layer Alignment Learnings

## Task 1: Architecture ADR
- Attempted 2x via writing category - subagent read files but never wrote ADR
- Third attempt via unspecified-low category with full content inline (background task bg_ee855d3b)
- Reverted unauthorized code changes from first attempt (runtime.go, api.go, api_test.go, app.js, session_store.go)

## Task 2: Chat Domain Contracts
- Domain style in `internal/domain` favors compact value objects, explicit string enums, and helper methods for derived projections instead of embedding HTTP/store concerns.
- Added chat-first primitives in `internal/domain/conversation.go`: roles, conversation lifecycle status, user turn submission payloads, resumable checkpoint/cursor semantics, chat stream envelope wrapping `StreamingEvent`, and inspectable transcript projections.
- Backward compatibility held by introducing new types only; existing `Session` / `Task` / `Step` / `StreamingEvent` contracts were not modified.
- Added focused tests in `internal/domain/conversation_test.go` validating chronological message ordering, session linkage, and transcript turn assembly from mixed message roles/events.

## Task 2: Chat domain contracts (Types layer)
- Added `internal/domain/conversation.go` with chat-first primitives: `ChatMessage`, `Conversation`, `TurnSubmission`, `UserMessagePayload`, `ChatTranscript`, and resumability via `ConversationCheckpoint` + `ResumeCursor`.
- Kept compatibility by extending (not altering) existing streaming model through `ChatStreamEvent` wrapper that carries `StreamingEvent` plus conversation/session/turn context.
- Matched existing domain style: plain structs, string enums, no persistence tags, and helper constructors/assemblers limited to deterministic ordering (`MessagesInOrder`) and transcript composition (`BuildChatTranscript`).

## Task 3: Service-layer chat orchestration
- Added `internal/service` as the application orchestration layer between store/runtimebuilder and server handlers, keeping HTTP response shaping in `internal/server/api.go` while moving run/resume/list/transcript decisions into `ChatService`.
- `ChatService` now owns session creation, runtime start/resume handoff, transcript assembly from persisted `InspectState`, and stream-session validation so service tests can execute without an HTTP server.
- Preserved backward compatibility by keeping session IDs as conversation IDs for now and mapping service errors back to existing API status codes/response shapes in the transport layer.

## Task 3: Service layer extraction
- Introduced internal/service as the chat orchestration layer so run/resume/inspect/list concerns are callable without HTTP while preserving existing API response shapes.
- Kept SSE transport in internal/server/api.go, but moved session validation and lifecycle decisions behind ChatService, which now centralizes runtime start/resume and transcript assembly from persisted inspect state.
- Added focused service tests against sqlite-backed stores and session manager fakes to validate orchestration without spinning up an HTTP server.

## Task 4: v6 chat UX contract
- The current v5 browser shell is explicitly page-based: auth gates access to dashboard/task navigation, while stream and detail live on separate views. The v6 UX contract must reverse that hierarchy so chat is the default post-auth workspace.
- Legacy capabilities only remain coherent if they are embedded into the conversation workflow: stream inside assistant bubbles, history in a side panel, inspect in a contextual drawer/panel, and resume as a continue affordance on an existing transcript.
- The UX contract must preserve same-origin Go `embed.FS` delivery, manual JWT entry, and SSE streaming while explicitly rejecting dashboard-first IA and SPA framework drift.

## 2026-05-11 Task 6
- Embedded web UI now treats `#/chat` as the primary surface and keeps history/inspect as subordinate panels instead of standalone pages.
- Inline streaming works best by optimistically appending the user turn locally, buffering `token_delta` content in a transient assistant bubble, then refreshing the transcript from `/api/v1/chat/{id}/transcript` on stream completion.
- The chat conversation list API currently returns session-shaped items, so the browser derives active conversation identity from `conversation_id || session_id` and session continuity from `stream_url`.
- Scope-fidelity audit: changed code remains concentrated in chat-first UI assets, `internal/service`, `internal/domain/conversation`, and server/store wiring needed for six-layer alignment; no unrelated product surfaces were introduced.
- Verification note: `lsp_diagnostics` is clean for changed Go packages; `go build ./...` passes; `go test ./...` currently fails because pre-existing plugin/runtime fixtures under `testdata/plugins/echo_plugin` and `testdata/runtime` are missing, so full green test verification is blocked by repository fixture state rather than the v6 diff itself.

- Task 8: chat service regression tests now cover StartConversation validation/fatal-provider persistence, SubmitReply missing/running conflicts, GetTranscript missing plus standalone empty projection, ListConversations pagination/status filtering/empty results, and ResumeConversation resumable vs terminal paths.
