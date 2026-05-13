## 2026-05-12T02:20:00Z Task: bootstrap
- v7 task 1 should anchor on existing tests in `internal/service/chat_test.go`, `internal/runtime/session_manager_test.go`, `internal/server/api_test.go`, and `e2e/tests/web-ui.spec.js`.
- Playwright prerequisites are explicit in `e2e/package.json`: `npm install` for `@playwright/test`, plus `npm run install:browsers` / `playwright install --with-deps chromium` when Chromium is unavailable.
- Existing docs also mention Playwright setup and commands in `docs/USAGE.md`, `docs/validation-matrix.md`, and `docs/ADR-010-v5-web-ui.md`.
## 2026-05-12T02:55:00Z Task: v7-task-1-red-baseline
- RED-phase reliability coverage can be added by extending existing Go unit test files and the serial `WebUI` Playwright suite without changing harness structure.
- `npx playwright test --project=chromium --list` is sufficient to prove newly registered browser auth/session reliability scenarios before production fixes exist.
## 2026-05-12T10:52:00Z Task: v7-task-2-service-validation
- Added `isSupportedTaskType()` helper function in `internal/service/chat.go` to validate task_type values (general, coding, research, file_workflow)
- Validation added to both `StartConversation()` and `StartChat()` methods - returns ValidationError for unsupported task_type
- Used simple string comparison instead of domain.TaskCategory.Normalize() to match test expectations (Normalize() converts unknown values to "general" silently)
- Service layer properly uses typed errors (ValidationError, NotFoundError, ConflictError) without HTTP-specific logic
## 2026-05-12T11:20:00Z Task: v7-task-3-session-manager-lifecycle
- `SessionActor.Subscribe()` must fail closed once the actor is finalized; returning an already-closed subscription preserves session-manager lifecycle authority and makes subscribe-after-completion deterministic.
- `sessionEventRelay` needs its own closed flag so late subscribers after relay teardown are deterministically rejected instead of being registered against a dead relay.
- Nil runner factories should be promoted into explicit actor failures so runtime final state and persisted session status remain aligned with the actual execution fault.
## 2026-05-12T12:30:00Z Task: v7-task-5-api-error-envelopes
- NotFoundError now properly surfaces resource ID in API error messages: 'session "missing-session" not found'
- Fix: Updated mapServiceError in api.go to extract Kind and ID from NotFoundError instead of using generic message
- All error paths now use consistent writeStructuredError envelope: { error: { code, message }, request_id }
- Panic recovery safe semantics verified: raw panic values NOT exposed, generic "internal server error" returned
- Task 5 is independent of Task 3's environment blockers (windows/386 race, missing replay fixtures)
## 2026-05-12T13:10:00Z Task: v7-task-6-diagnostics
- Minimal internal diagnostics fit cleanly into existing session inspect/transcript flows by extending persisted session metadata with request_id, failure_reason, and finalized state rather than adding new storage tables or observability subsystems.
- Capturing request correlation at chat/session creation via chi RequestID context keeps API and service layers aligned while terminal failure reasons are best finalized in the existing PersistFinal callback.


## 2026-05-12T13:10:00Z Task: v7-task-4-stream-hardening
- sseWriter.writeEvent should treat JSON-marshal failures as malformed stream events and let HandleStream skip them while continuing delivery to later terminal events.
- Session relay overflow is more diagnosable when a dropped terminal session_complete event is preserved as the subscriber overflow signal instead of replacing it with a generic error event.
- Targeted go test ./internal/runtime -run Stream|Event|SSE in this workspace required restoring shared replay-test fakes inside internal/runtime test package compilation, even though full runtime fixture suites remain out of scope.


## 2026-05-12T13:40:00Z Task: v7-task-7-playwright-reliability
- Expanded e2e/tests/web-ui.spec.js reliability coverage around manual JWT auth success/failure, empty-composer blocking, happy-path submit, visible submit failure feedback, and reload-based transcript continuity.
- Added connectManually() and submitMessage() helpers so browser assertions wait on deterministic UI/network signals (waitForResponse, web-first locator assertions) instead of arbitrary sleeps.
## 2026-05-12T15:30:00Z Task: v7-task-8-documentation-update
- README.md, PROGRESS.md, and docs/validation-matrix.md all follow the same version-doc pattern: header summary line -> progress status -> detailed feature sections for each version
- v7 evidence files span task-1 through task-7, each with specific sub-task recordings
- Task 8 evidence files follow naming convention task-8-docs-{purpose}.txt
- Key pattern: each major version adds a line in README.md version summary, a bullet in the progress status line, a detailed section in PROGRESS.md, a validation table in docs/validation-matrix.md, and evidence files in .sisyphus/evidence/
- v7 must be clearly labeled as "internal reliability release" with explicit non-goals to prevent scope creep misinterpretation
- No fabricated evidence references: all referenced .sisyphus/evidence/task-{1-7}-* files must actually exist on disk
- The validation-matrix.md has a long-established pattern of a header block with Last Updated, Phase, Status, and per-version status lines


- SubmitReply should resolve transcripts first so either conversation_id or root session_id can continue the same chat, and prior turns should be reconstructed from transcript messages rather than raw task descriptions alone.
