# v6 Chat-UI + Six-Layer Alignment Decisions

## Task 4: Chat-first UX contract
- The v6 browser information architecture is chat-first: successful auth routes directly into the chat workspace rather than a dashboard or a new-task page.
- `auth`, `stream`, `resume`, `inspect`, and `history` remain mandatory capabilities, but each is remapped into chat-centered surfaces instead of detached top-level pages.
- Inspect and history stay secondary panels/overlays so the UI preserves operational depth without displacing conversation as the primary workflow.
## 2026-05-11 Task 6
- Replaced the dashboard/task/detail-first embedded HTML shell with a three-region chat workspace: history panel, central transcript/composer, and contextual inspect panel.
- Preserved the existing JWT bootstrap flow and `/api/v1/sessions/{id}/stream` transport, but moved streaming presentation inline into the transcript instead of a separate stream page.
- Kept routing lightweight with hash state anchored on chat (`#/chat`, `#/chat/:conversationId`, `#/chat/:conversationId/inspect`) so the chat surface stays mounted while secondary panels change.

- Task 8: API regression coverage was extended at the HTTP layer for chat-first routes with explicit 401 unauthenticated checks and 404/409 mapping assertions so service-to-API error contracts stay backward compatible.
