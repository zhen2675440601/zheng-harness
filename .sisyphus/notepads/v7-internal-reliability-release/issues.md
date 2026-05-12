## 2026-05-12T02:20:00Z Task: bootstrap
- The notepad directory did not exist at session start and had to be initialized manually.
- Tooling note: `glob` over the new notepad directory failed because `rg`/spawn support was unavailable in that call path, so direct file reads/creation should be preferred if the issue recurs.
## 2026-05-12T02:55:00Z Task: v7-task-1-red-baseline
- New RED tests currently fail because service chat APIs accept unsupported `task_type`, stream missing-session envelopes omit the requested session id, and runtime subscriptions remain open when attached after actor completion.
- `npm install` in `e2e/` was already up to date; bootstrap output was still captured as evidence because Playwright verification was required.
## 2026-05-12T10:52:00Z Task: v7-task-2-service-validation
- Initial fix attempt used `domain.TaskCategory(strings.TrimSpace(value)).Normalize()` but this silently converts unknown values to "general", causing tests to fail
- Solution: Use direct string comparison in `isSupportedTaskType()` checking for exact matches: "general", "coding", "research", "file_workflow"
## 2026-05-12T11:20:00Z Task: v7-task-3-session-manager-lifecycle
- Full `go test ./internal/runtime/...` remains blocked in this workspace by missing `testdata/runtime/*.json` replay fixtures unrelated to session-manager lifecycle changes.
- `go test -race ./internal/runtime/...` cannot execute in the current environment because `go env GOARCH` is `386` on Windows, and the Go toolchain reports `-race is not supported on windows/386`.
## 2026-05-12T12:30:00Z Task: v7-task-5-api-error-envelopes
- Initial implementation of NotFoundError mapping in mapServiceError used generic message "session not found" without naming the missing resource
- Test `TestStreamReturnsStructuredNotFoundEnvelopeForMissingSession` failed until fix was applied to extract Kind and ID from NotFoundError
## 2026-05-12T13:10:00Z Task: v7-task-6-diagnostics
- Switching loadSessionRecord() from returning *storedTask to *storedSessionMetadata required follow-up fixes in LoadTask() and InspectSession() to dereference nested metadata.Task fields consistently.


## 2026-05-12T13:10:00Z Task: v7-task-4-stream-hardening
- Existing internal/runtime targeted test execution was blocked by missing replay-test fake helpers/types (akeModel, akeSessionStore, ixedClock, etc.); added shared test support to restore package compilation before stream-specific verification could run.
- go test ./internal/server/... passed cleanly for evidence capture, but targeted runtime verification initially failed until the replay support scaffolding was reinstated.


## 2026-05-12T13:10:00Z Task: v7-task-4-stream-hardening
- Existing internal/runtime targeted test execution was blocked by missing replay-test fake helpers/types (fakeModel, fakeSessionStore, fixedClock, etc.); added shared test support to restore package compilation before stream-specific verification could run.
- go test ./internal/server/... passed cleanly for evidence capture, but targeted runtime verification initially failed until the replay support scaffolding was reinstated.


## 2026-05-12T13:40:00Z Task: v7-task-7-playwright-reliability
- Full Chromium Playwright execution is currently blocked by harness startup drift: existing 	estdata/e2e-server.exe serves an older dashboard UI, while go run ./cmd/server fails alidateWebUIAssets() because the current pp.js no longer contains the legacy handleConnect sentinel checked by e2e/server-harness.js.
- Evidence captured in .sisyphus/evidence/task-7-playwright-failure.txt and .sisyphus/evidence/task-7-playwright-happy.txt; task-7 specs are listed, but end-to-end pass remains blocked until harness validation/prebuilt binary are refreshed.

