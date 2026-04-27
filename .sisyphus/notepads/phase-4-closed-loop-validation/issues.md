# Issues

## 2026-04-27 Pre-Phase-4 Test Failures
- `cmd/agent::TestRunCommandInterruptPersistsInterruptedSession`: resume of interrupted session fails because plan for session is not found in SQLite (sql: no rows in result set)
- `internal/config::TestLoadUsesMultiProviderConfigAndSwitchesProvider`: multi-provider config loading returns wrong model ("qwen3.6-plus" instead of expected "gpt-4.1-mini")

## 2026-04-27 Phase 4 Task 4 Tooling Note
- LSP diagnostics for changed Go test file are clean, but JSON fixture diagnostics could not run because the configured `biome` server is not installed in this environment.

## 2026-04-27 Resolved in Task 2
- Interrupted CLI runs could persist session status but lose the plan row because plan/session/step writes inherited a canceled context. The fix is to persist alias-store writes with cancellation stripped while keeping runtime control flow cancelable.
## 2026-04-27 Phase 4 Task 3
- No new blockers after fix/test run. Unknown task categories still deterministically fall back to command-backed compatibility verification in `TaskAwareVerifier`; this behavior is now covered by regression tests.

## 2026-04-27 Phase 4 F3 Manual QA
- CLI help invocation `go run ./cmd/agent run --help` produced correct help text but terminated with `flag: help requested` and process exit status 1. If manual QA requires zero exit status for help flows, the command handler should intercept help requests and return success after printing usage.
