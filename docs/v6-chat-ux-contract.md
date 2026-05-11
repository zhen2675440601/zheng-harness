# v6 Chat-First UX Contract

**Status**: Draft contract for v6 implementation  
**Scope**: Embedded browser UI served by the Go API server via `embed.FS`  
**Non-goals**: Separate SPA framework, WebSocket transport, dashboard-first navigation, parallel legacy task-form UX

## 1. Purpose

v6 must change the browser experience from a task-submission console into a **chat-first workspace** for a general-purpose agent. The user should land in a conversational surface immediately after authentication, send a message with a lightweight **compose** experience, watch assistant output **stream** inline, reopen prior conversations from **history**, **inspect** structured execution details without leaving chat, and **resume** blocked or completed work from the same conversation context.

This contract preserves the proven v5 operational capabilities—auth, streaming, resume, inspect, session history, and same-origin serving—but relocates them into a single primary conversation workflow.

## 2. Product Principles

1. **Chat is the product, not a side screen.** The default post-auth state is the chat workspace.
2. **Legacy capabilities are subordinate to conversation.** History, inspect, and resume support the chat flow; they are not separate top-level products.
3. **One embedded surface.** The UI remains same-origin, served by the Go server through `embed.FS` and chi routing.
4. **Thin browser client.** Pure HTML/CSS/JS only; no React, Vue, Svelte, or client-side SPA runtime.
5. **Streaming is visible where the conversation happens.** Assistant progress and tool output appear inline inside the active transcript, not on a detached stream page.
6. **Operational transparency remains first-class.** Structured inspect data, verification evidence, and resumability must remain accessible from chat.

## 3. Information Architecture

## 3.1 Primary entry

- **Before auth**: JWT bootstrap screen.
- **After auth success**: land directly in **Chat Workspace**.
- **No dashboard as equal-priority home**: the v5 dashboard/session board is demoted into a secondary history surface.
- **No top-level new-task route**: the v5 task form is replaced by the message composer in chat.

## 3.2 Navigation model

The browser shell is a single conversation-oriented layout:

- **Center pane**: active chat workspace.
- **Left side panel or overlay**: conversation history and filters.
- **Right side panel, drawer, or modal overlay**: transcript detail / inspect.
- **Top utility strip**: auth state, reconnect status, current conversation metadata, and lightweight global actions.

The user should never need to navigate to a separate standalone dashboard page to access essential behavior.

## 3.3 Route/state expectations

Hash routing may still be used for embedded simplicity, but the route hierarchy must reflect chat primacy:

- `#/chat` or `#/` → active/default chat workspace
- `#/chat/:conversationId` → existing conversation transcript loaded into the same workspace
- `#/history` → optional history overlay open state, not a separate product home
- `#/chat/:conversationId/inspect` → inspect drawer/panel open for the selected conversation

The browser may encode panel state in hash or query fragments, but implementation must preserve this UX rule: **chat remains mounted as the anchor surface**.

## 4. Required Screen States and Views

## 4.1 Auth Bootstrap

**Purpose**: preserve the v5 manual JWT entry flow and local token persistence, but route successful authentication directly into chat.

**Required behavior**:

- Manual JWT token entry remains available.
- Optional auto-connect from persisted token may remain.
- On auth success, the shell transitions straight into the chat workspace.
- On auth failure, an explicit inline **error** state appears on the bootstrap screen.
- On token expiry during later usage, the UI returns to auth bootstrap or an auth-expired overlay with clear re-entry instructions.

## 4.2 Chat Workspace (Default)

**Purpose**: become the primary working surface for all agent interaction.

**Required regions**:

- **Conversation transcript pane**: chronological user/assistant/system messages.
- **Message composer**: the new primary input surface replacing the v5 new-task form.
- **Assistant stream area**: live streaming content rendered inline inside the active assistant bubble/turn container.
- **Conversation meta rail**: current status, resumability, verification summary, timestamps, and quick actions.

**Required behavior**:

- First input from the composer creates a conversation/session if none exists.
- Follow-up inputs append to the current conversation rather than creating a detached task page.
- The current streaming turn stays visible in context below the triggering user message.
- Stream completion converts the transient assistant bubble into persisted transcript content.
- If execution blocks, completes, or fails, the workspace shows that state inline near the latest turn.

## 4.3 Conversation History Panel

**Purpose**: provide secondary access to past conversations without replacing the chat workspace.

**Required behavior**:

- Open from a history button or shell affordance.
- Render a list of past conversations/sessions with status badges and updated timestamps.
- Support filtering by status at minimum: running, blocked/resumable, completed, failed, cancelled.
- Selecting a conversation loads that transcript into the main chat workspace.
- The currently active conversation is highlighted.
- History access must not feel like navigating back to the v5 dashboard; it is an adjunct surface.

## 4.4 Transcript Detail / Inspect

**Purpose**: expose structured execution detail for auditability without leaving conversation context.

**Required behavior**:

- Open from the active transcript or from history row actions.
- Present tool calls, plan/step structure, verification results, provenance, timing, and execution metadata.
- Allow the user to move between high-level transcript reading and low-level inspect detail without route thrash.
- Close back into the same chat workspace with transcript position preserved.

## 4.5 Error / Reconnect States

**Purpose**: make network and auth interruptions visible and recoverable.

**Required behavior**:

- **Auth expiry**: show explicit auth-expired message; block send actions until re-auth succeeds.
- **Stream interruption**: keep partial transcript content visible, mark the stream as interrupted, and expose reconnect/reload guidance.
- **Network failure**: show a non-silent error banner or inline state that explains whether the server action may still be running.
- **Reconnect**: provide a deterministic reconnect path for re-opening history, reloading transcript state, and re-attaching to a live or recently active conversation when possible.

## 4.6 Blocked / Resume State

**Purpose**: preserve resumable execution as a first-class chat affordance.

**Required behavior**:

- Conversations that stop in a resumable state must show a visible **resume** / continue control in both chat workspace and history.
- The UI must distinguish between blocked/resumable and terminal/completed states.
- A resumed run appends to the existing conversation transcript rather than spawning a disconnected execution page.
- Resume intent should be legible: e.g. “Continue from blocked step” or “Continue with new instruction”.

## 5. Component Responsibilities

The v6 UI should split responsibilities into small embedded primitives while preserving a plain HTML/CSS/JS implementation.

| Component | Responsibility | Notes |
|---|---|---|
| `AuthBootstrap` | Manual JWT entry, stored-token reconnect, auth error handling | Preserves v5 auth contract but redirects to chat on success |
| `AppShell` | Global layout, panel toggles, auth status, reconnect status | Anchors chat-first IA |
| `ChatWorkspace` | Owns active conversation state and default landing view | Main browser surface after auth |
| `TranscriptPane` | Renders ordered messages and turn grouping | Must support partial streaming turns |
| `MessageComposer` | Collects and submits first and follow-up user input | Replaces new-task form as primary input |
| `AssistantStreamBubble` | Displays incremental stream output inline | Owns pending/streaming/completed visual phases |
| `ConversationHistoryPanel` | Lists conversations, filters by status, loads selected transcript | Secondary access path only |
| `InspectPanel` | Shows structured tool calls, steps, verification, provenance | Opened contextually from transcript/history |
| `ConversationStatusBar` | Shows status, timestamps, resume availability, error/reconnect summaries | Visible within workspace |
| `ReconnectBanner` | Displays transient network/auth interruption messaging | Must not hide ongoing execution uncertainty |

## 6. Legacy Capability Mapping

Every essential v5 capability must survive in v6, but be relocated into the chat-first flow.

| Legacy capability | v5 location | v6 location | Required v6 behavior |
|---|---|---|---|
| `auth` | Standalone JWT screen before dashboard/task routes | `Auth Bootstrap` before chat workspace | Preserve manual token entry and stored-token reconnect; on success enter chat directly |
| `stream` | Dedicated stream page | Inline in chat workspace message bubbles | Assistant output streams inside the active assistant turn using SSE-backed incremental rendering |
| `resume` | Button on detail/stream-oriented views | Continue button in chat workspace and history panel | Resume appends to the same conversation and restarts inline streaming |
| `inspect` | Dedicated detail page | Expandable inspect drawer/panel from transcript or history | Show structured step-by-step execution, tool calls, verification evidence, provenance |
| `history` | Dashboard/session list page | Side panel or overlay launched from chat shell | Load prior conversations into chat without making dashboard the home screen |
| new task form | Dedicated top-level route | Message composer inside chat workspace | First message creates the conversation; no equal-priority task route remains |

## 7. Interaction Flows

## 7.1 First message flow

1. User authenticates through Auth Bootstrap.
2. UI opens the default chat workspace.
3. User types the first instruction into the message composer.
4. Browser submits the message to the existing run/chat initiation API surface.
5. Conversation/session identifier is created.
6. User message is appended to the transcript immediately.
7. Assistant reply begins to stream inline beneath that user message.
8. On completion, the assistant turn is committed as a normal transcript item and conversation status updates.

## 7.2 Follow-up message flow

1. User opens an existing conversation in chat.
2. User sends another message through the composer.
3. New user turn is appended to the active transcript.
4. Assistant stream appears below as incremental output.
5. Final state, verification summary, and resumability markers update in place.

## 7.3 History access flow

1. User opens the history panel from the chat shell.
2. User filters by status if needed.
3. User clicks a conversation row.
4. Panel closes or remains docked according to layout.
5. Main chat workspace loads the selected transcript.
6. If the conversation is still active, stream/reconnect status is surfaced inline.

## 7.4 Resume flow

1. User selects a blocked or completed conversation from chat or history.
2. UI exposes a **continue** / resume action with conversation context.
3. User clicks resume, optionally after adding a clarifying instruction.
4. Existing conversation remains in view.
5. Resumed execution appends new turns/events to that transcript.
6. Assistant output streams inline in the same workspace.

## 7.5 Inspect flow

1. User opens inspect from a transcript turn, status bar, or history item.
2. Inspect panel/drawer opens without replacing chat as the base surface.
3. Structured execution detail appears: steps, tool calls, verification results, timing, provenance.
4. User closes inspect and returns to the same transcript scroll position.

## 8. State Contract Details

## 8.1 Conversation states visible in chat

At minimum, the browser contract must represent:

- idle / empty workspace
- sending
- streaming
- blocked / resumable
- completed
- failed
- cancelled
- reconnecting
- auth expired

These states must be visually explicit in the active workspace, not only discoverable in inspect.

## 8.2 Message rendering contract

- User messages are durable transcript items.
- Assistant messages may begin as transient streaming bubbles and settle into final transcript items.
- Tool/progress events may appear as collapsible sub-events within the assistant turn rather than as detached pages.
- Verification summaries should appear inline at the end of a relevant assistant turn, with a path into deeper inspect detail.

## 8.3 Secondary panel contract

- History and inspect may be drawers, overlays, or docked side panels.
- Mobile or narrow widths may collapse them into full-screen overlays.
- Regardless of layout adaptation, chat remains the canonical destination surface.

## 9. Backend/Transport Constraints

The UX contract is constrained by existing platform decisions and must remain compatible with them:

- **Same-origin embedded delivery** through the Go server and chi router.
- **Static assets embedded with `embed.FS`**.
- **Pure HTML/CSS/JS** only; no React/Vue/Svelte/SPA framework.
- **SSE streaming transport** using browser `EventSource` semantics/API contract for live updates.
- **Manual JWT token entry preserved**.
- **No WebSocket transport**.
- **No separate frontend origin**.
- **No requirement to break existing backend routes solely for cosmetic routing changes**.

## 10. Migration Rules from v5 to v6

1. The v5 auth bootstrap survives, but successful auth no longer reveals dashboard/task navigation as the primary experience.
2. The v5 dashboard session list becomes a conversation history panel/overlay.
3. The v5 stream page is removed as a primary route; its behavior is absorbed into inline assistant streaming.
4. The v5 detail page becomes inspect behavior inside or adjacent to chat.
5. The v5 new-task form is removed as a peer page and re-expressed as the message composer.
6. Any retained hash routing must describe conversation context, not dashboard-vs-task primacy.

## 11. Implementation Acceptance Criteria

- Browser enters **chat workspace by default after auth**.
- No equal-priority top-level dashboard home or new-task route remains in the primary UX.
- `auth`, `stream`, `resume`, `inspect`, and `history` all remain available and explicitly mapped.
- Composer-driven first message and follow-up flows are documented and implemented against the embedded transport model.
- Error and reconnect behavior are visible, actionable, and non-silent.
- Resumable conversations have clear continue affordances in both chat and history contexts.
- Structured inspect detail is reachable without abandoning the chat workspace.

## 12. Out-of-Scope Guardrails

- Do not introduce React, Vue, Svelte, Next.js, Vite, or any separate SPA build system.
- Do not preserve the dashboard as an equal-priority landing page.
- Do not keep the new-task form as a top-level sibling workflow.
- Do not strand legacy capabilities on detached routes that the user must leave chat to access.
- Do not replace SSE with WebSocket for v6.

## 13. Summary

v6’s browser UX contract is a **single chat-centered embedded workspace**. Authentication still begins with manual JWT bootstrap; conversation remains the default destination; assistant output streams inline; history and inspect become secondary support surfaces; resume is an explicit conversational action; and auth, error, and reconnect states are handled without breaking the same-origin Go-served architecture.
