# Issues

## 2026-04-26 Task 2 Verification Blocker
- `go test ./...` currently fails at module resolution due to missing dependency `github.com/bmatcuk/doublestar/v2` required by `internal/tools/adapters/glob.go`.
- This blocker is repository-level and unrelated to prompt-injection changes; tests for changed prompt package pass.

## 2026-04-26 Anthropic Adapter Verification Note
- Repository-wide `go test ./...` currently fails in `cmd/agent` because OpenAI CLI tests run without an API key and now receive `openai API key must not be empty`; this is outside Anthropic adapter changes.
- Scoped verification target for this task (`go test ./internal/llm/...`) passes after Anthropic implementation and tests were added.

## 2026-04-26 Verifier Task Verification Environment Issue
- Current environment Go toolchain reports standard library import failure during full test run: `package syscall is not in std (D:\zwlword\go\src\syscall)` from `internal/syscall/windows/registry/key.go`.
- This blocks repository-wide `go test ./...` verification regardless of changed package scope.

## 2026-04-26 Phase 2 Manual QA
- CLI manual QA command `go run ./cmd/agent run --task "inspect repository" --verify-mode off --json` failed with an outbound OpenAI network error instead of using an offline FakeModel path, so FakeModel/no-key behavior is not approved from hands-on execution.
- CLI missing-key QA command `go run ./cmd/agent run --task "hello" --provider openai --verify-mode off --json` also failed with the same OpenAI connection error, so graceful missing-key handling was not observed in this environment.
- Repository verification commands `go test ./... -cover` and `go test -race ./...` both passed during QA.

## 2026-04-26 Scope Fidelity Audit (Phase 2)
- Scope creep detected beyond Phase 2 INCLUDE list:
  - `edit_file` tool added (`internal/tools/adapters/files.go`, `internal/tools/executor.go`) even though Phase 2 INCLUDE only requested glob/grep/shell/allowlist and did not include new file-editing capability.
  - Notepad governance pipeline added (`.github/workflows/ci.yml`, `scripts/check-notepads.ps1`, `Makefile`) which is outside Phase 2 runtime/tooling objectives.
- EXCLUDE guardrails remained respected for native tool calling, multi-agent orchestration, plugin system, Web UI/gateway, vector DB retrieval, and ToolCall.Input interface stability.
