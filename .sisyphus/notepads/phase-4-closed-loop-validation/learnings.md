# Learnings

## 2026-04-27 Phase 4 Start
- Pulled latest code from remote via SSH (HTTPS was blocked by network)
- Phase 3 implementation is complete (T1-T12 checked), but Final Wave (F1-F4) is unchecked
- Two test failures exist after pull:
  1. `cmd/agent::TestRunCommandInterruptPersistsInterruptedSession` - "plan for session not found: sql: no rows in result set"
  2. `internal/config::TestLoadUsesMultiProviderConfigAndSwitchesProvider` - got model "qwen3.6-plus", want "gpt-4.1-mini"
- These failures are validation blockers that Phase 4 Task 5 is designed to address
- Phase 4 is a closed-loop validation phase: prove the system works end-to-end, fix blockers, align docs

## 2026-04-27 Validation Matrix Complete

### Proof Surfaces Inventoried
- **CLI continuity**: 7 test surfaces mapped (run/resume/inspect, task metadata persistence, interrupt handling)
- **Task-type routing**: 6 test surfaces mapped (coding/research/file_workflow routing and end-to-end fixtures)
- **Verifier dispatch**: 5 test surfaces mapped (off/standard/strict modes, non-coding policies)
- **Replay fixtures**: 6 fixtures proven (success, verification rejection, resume, research, file_workflow, unsafe tool rejection)
- **Configuration**: 3 test surfaces mapped (multi-provider loading, validation, env overrides)

### Coverage Assessment
- ✅ Happy path: All three task categories (coding/research/file_workflow) have dedicated fixtures + routing tests
- ✅ Failure path: Verification rejection, unsafe tool rejection, context cancellation all covered
- ⚠️ Recovery path: Resume fixture exists but CLI integration test is broken (Blocker 1)

### Known Gaps
1. **Blocker 1**: CLI resume after interrupt fails - persistence layer issue (plan not saved)
2. **Blocker 2**: Provider switching returns wrong model - config selection logic issue
3. **Gap**: No explicit CLI-level test for `resume --session <interrupted-session-id>` recovery

### Test Inventory Summary
**Total test surfaces mapped**: 27 distinct validation points across 5 categories
- CLI integration: 9 tests in `cmd/agent/main_test.go`
- Runtime replay: 6 fixtures in `testdata/runtime/` + 6 corresponding tests
- Verifier dispatch: 8 tests across `internal/verify/*.go`
- Configuration: 6 tests in `internal/config/config_test.go`

### Evidence Targets (for Phase 4 completion)
- `.sisyphus/evidence/task-2-cli-lifecycle.txt` - CLI continuity proof
- `.sisyphus/evidence/task-3-task-routing.txt` - Task-type routing proof
- `.sisyphus/evidence/task-4-replay-coverage.txt` - Replay fixture outcomes
- `.sisyphus/evidence/task-5-blocker-fixes.txt` - Resolved blocker tests
- `.sisyphus/evidence/task-7-full-acceptance.txt` - Final acceptance sweep

### Action Items
- Task 2 executor: Must fix or document CLI resume limitation if Blocker 1 persists
- Task 3 executor: Verify coding fallback doesn't leak into non-coding paths
- Task 5 executor: Prioritize Blocker 1 (interrupt resume) and Blocker 2 (provider switch)
- Task 6 executor: Document current resume limitations until blockers are fixed

## 2026-04-27 Phase 4 Task 4 Replay Coverage
- Replay fixtures are more stable when JSON explicitly carries additive Phase 3 metadata (`task.category`, `verification.status`) instead of relying on normalization defaults hidden inside domain unmarshalling.
- Research and file_workflow replay tests should assert structured evidence payload details, not just terminal session success, to prove task-aware verifier dispatch remains deterministic for non-coding flows.
- Unsafe tool rejection replay is deterministic with the real executor because `exec_command` rejects the non-allowlisted `powershell` executable before shell execution, producing a stable `command "powershell" is not allowlisted` error.

## 2026-04-27 Phase 4 Task 2
- CLI interrupt persistence was flaky because runtime/store writes used the canceled `runCtx`; `SaveSession` sometimes succeeded before cancellation, but `SavePlan`/step writes could be dropped once the signal fired.
- Wrapping CLI session-store writes with a session alias store that strips cancellation (`context.WithoutCancel`) preserves the user-facing session ID while still allowing the runtime loop to stop promptly on SIGINT.
- Continuity coverage is stronger when tests assert persisted plan summary, inspect `terminated_reason`, stable `step 1:` summary formatting, and persisted task metadata across `run` -> `inspect` -> `resume`, not only exit codes.
## 2026-04-27 Phase 4 Task 3
- Multi-provider CLI provider switching was overwriting the newly selected provider's model/base_url/api_key with stale flag defaults from the previously selected provider; recording visited CLI flags before applying overrides keeps provider selection deterministic and fixes `TestLoadUsesMultiProviderConfigAndSwitchesProvider`.
- Task routing regression coverage now proves `coding`, `research`, and `file_workflow` resolve to explicit verifier policies in the task registry, and task-aware verifier tests now assert research/file_workflow dispatch does not call command-backed verification while coding and unknown compatibility fallback still do.

## 2026-04-27 Phase 4 Task 7 Acceptance Sweep
- Final acceptance sweep passed with `CGO_ENABLED=1` for all required CI-equivalent commands: `go build ./...`, `go test ./... -v`, `go test -race ./...`, and `go test -cover ./...`.
- Targeted proof commands also passed and were captured separately for traceability: CLI lifecycle (`cmd/agent` persistent session + interrupt persistence), runtime replay fixtures, verifier suite, and config suite.
- Evidence bundle was written under `.sisyphus/evidence/` with one file per command surface so a future executor can trace each proof back to the exact invocation and observed PASS result.
- Two runtime log lines appeared during otherwise passing tests (`memory recall failed: context canceled` and `context deadline exceeded`) inside interruption/replay scenarios; these are expected test-path artifacts, not acceptance blockers, because the owning tests still passed.

## 2026-04-27 Phase 4 F3 Manual QA
- Targeted manual QA commands passed cleanly for the two repaired regressions: `TestRunCommandInterruptPersistsInterruptedSession` and `TestLoadUsesMultiProviderConfigAndSwitchesProvider` both returned PASS with no follow-on failures.
- Full suite and race detector both passed across all packages; the only notable runtime noise was the already-known `memory recall failed: context canceled` log line during interrupt-path coverage, which did not change package PASS results.
- `go run ./cmd/agent run --help` printed the expected flag set, including `-provider`, `-task-type`, `-task-verification-policy`, and `-max-steps`, but `go run` surfaced `flag: help requested` and exited with status 1 instead of 0.
