## Change Log (General Agent Harness GO)

This file is the single place for change explanations referenced by notepad entries.

## New Entry Checklist (MUST)

Before considering any change "documented", verify all items below:

- [ ] Create a new change ID using format `CHG-YYYYMMDD-NNN` (append-only; do not reuse IDs).
- [ ] Add a change-log entry with fixed fields in order:
  1. `Time`
  2. `Scope`
  3. `Summary`
  4. `Why`
  5. `Files`
- [ ] Update at least one notepad entry (`decisions.md` / `issues.md` / `problems.md`) with `Change Ref: CHG-...`.
- [ ] Ensure notepad entry IDs follow append-only convention:
  - `H-###` for Historical
  - `C-###` for Current
  - `V-###` for Verification
- [ ] Add/refresh verification evidence (`Command` + `Result` + `Evidence`) in the corresponding notepad entry.
- [ ] Run minimal verification after code-affecting changes:
  - `go test ./...`
  - `go build ./...`
- [ ] If behavior/docs changed, sync `README.md` / `docs/USAGE.md` / `PROGRESS.md` as needed.

## Entry Template

Use this exact skeleton for each new change:

```md
### CHG-YYYYMMDD-NNN
- **Time**: YYYY-MM-DD (Asia/Shanghai)
- **Scope**: <module / area>
- **Summary**: <what changed>
- **Why**: <why this was necessary>
- **Files**:
  - `<path-1>`
  - `<path-2>`
```

### CHG-20260426-001
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: Runtime wiring / provider path
- **Summary**: Unified CLI runtime wiring so all supported providers (`openai` / `anthropic` / `dashscope`) go through `llm.NewProvider + runtime.NewModelAdapter`.
- **Why**: Remove provider behavior drift and make config selection effective.
- **Files**:
  - `cmd/agent/cli.go`
  - `internal/llm/openai.go`
  - `internal/llm/anthropic.go`
  - `internal/llm/stub_json.go`
  - `cmd/agent/main_test.go`

### CHG-20260426-002
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: Verifier mode wiring
- **Summary**: Bound CLI verifier initialization to `verify_mode` (`off` / `standard` / `strict`) via `newVerifierFromConfig`.
- **Why**: Ensure documented config has actual runtime effect.
- **Files**:
  - `cmd/agent/cli.go`
  - `cmd/agent/main_test.go`

### CHG-20260426-003
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: Documentation consistency
- **Summary**: Unified Go version and config precedence/docs examples.
- **Why**: Keep docs/progress/go.mod aligned with executable behavior.
- **Files**:
  - `go.mod`
  - `README.md`
  - `docs/USAGE.md`
  - `PROGRESS.md`

### CHG-20260426-004
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: Notepads governance
- **Summary**: Introduced fixed template sections (`Historical` / `Current` / `Verification`) and standardized entry fields.
- **Why**: Make updates auditable and mechanically maintainable.
- **Files**:
  - `.sisyphus/notepads/general-agent-harness-go/decisions.md`
  - `.sisyphus/notepads/general-agent-harness-go/issues.md`
  - `.sisyphus/notepads/general-agent-harness-go/problems.md`

### CHG-20260426-005
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: Notepads pre-commit governance automation
- **Summary**: Added a PowerShell governance check script to enforce required sections, unique entry IDs, and valid `Change Ref` linkage to `change-log.md`; wired as `make notecheck`.
- **Why**: Prevent undocumented or untraceable notepad updates and catch violations before commit.
- **Files**:
  - `scripts/check-notepads.ps1`
  - `Makefile`

### CHG-20260426-006
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: CI governance enforcement
- **Summary**: Added notepad governance check step into GitHub Actions CI workflow.
- **Why**: Enforce documentation traceability in PR/branch validation instead of relying on local discipline only.
- **Files**:
  - `.github/workflows/ci.yml`

### CHG-20260426-007
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: CI runtime optimization for governance checks
- **Summary**: Added path-based change detection so notepad governance check runs only when notepads governance-relevant files change.
- **Why**: Reduce CI time for PRs unrelated to notepad governance while preserving enforcement where relevant.
- **Files**:
  - `.github/workflows/ci.yml`

### CHG-20260426-008
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: edit_file protocol enhancement
- **Summary**: Upgraded `edit_file` input to an unambiguous block protocol with multiline old/new text support using `<<<OLD` and `<<<NEW` markers while preserving path-on-first-line safety checks.
- **Why**: Enable robust incremental edits for multiline code blocks and eliminate protocol ambiguity.
- **Files**:
  - `internal/tools/adapters/files.go`
  - `internal/tools/executor.go`
  - `internal/tools/safety.go`
  - `internal/tools/tools_test.go`

### CHG-20260426-009
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: strict verifier evidence hardening
- **Summary**: Added structured command evidence (`COMMAND`, `EXIT_CODE`, output envelope) and updated verify checks to prefer structured pass/fail signals with compatibility fallback.
- **Why**: Reduce false positives in strict verification and make evidence assessment deterministic.
- **Files**:
  - `internal/tools/adapters/shell.go`
  - `internal/verify/checks.go`
  - `internal/verify/verify_test.go`

### CHG-20260426-010
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: DashScope adapter boundary tests
- **Summary**: Added HTTP-level edge-case tests for timeout, non-2xx error handling, empty content rejection, and success path contract checks.
- **Why**: Increase confidence in provider adapter behavior under real failure modes.
- **Files**:
  - `internal/llm/dashscope_test.go`

### CHG-20260426-011
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Scope**: Final Verification Wave closure artifacts
- **Summary**: Added a consolidated F1-F4 final verification report and synced governance references for closeout.
- **Why**: Provide a single auditable acceptance record before final user sign-off.
- **Files**:
  - `.sisyphus/notepads/general-agent-harness-go/final-verification-report.md`
  - `.sisyphus/notepads/general-agent-harness-go/decisions.md`
