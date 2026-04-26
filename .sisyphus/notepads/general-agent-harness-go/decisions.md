## Template

Each entry uses a fixed field order:
1. Time
2. Decision
3. Why
4. Scope
5. Command
6. Result
7. Evidence
8. Follow-up
9. Change Ref (required): `CHG-YYYYMMDD-NNN` in `change-log.md`

Entry ID convention:
- `H-###` for Historical
- `C-###` for Current
- `V-###` for Verification
- IDs are append-only and must not be reused.

---

## Historical

### Entry H-001
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Keep `internal/domain` as the strict inward-facing core with import guardrails.
- **Why**: Preserve clean architecture boundaries and testability.
- **Scope**: `internal/domain`, guardrail tests.
- **Command**: Code/test review in domain package.
- **Result**: Boundary contract established and retained.
- **Evidence**: Domain-specific types/ports and guardrail rationale documented.
- **Follow-up**: Continue blocking infra leakage into domain.
- **Change Ref**: CHG-20260426-004

### Entry H-002
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Use bounded runtime statuses and verification taxonomy.
- **Why**: Ensure deterministic termination and auditable failure categories.
- **Scope**: `internal/runtime`, `internal/verify`, `internal/domain`.
- **Command**: Runtime/verify test and code review.
- **Result**: Canonical statuses and taxonomy kept as shared contract.
- **Evidence**: `success`, `verification_failed`, `budget_exceeded`, `fatal_error`, `interrupted`; verification categories retained.
- **Follow-up**: Tighten strict-mode heuristics incrementally.
- **Change Ref**: CHG-20260426-004

### Entry H-003
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Preserve fixture-driven replay/benchmark strategy for reliability.
- **Why**: Keep regression coverage deterministic and low-overhead.
- **Scope**: `internal/runtime` tests and fixtures.
- **Command**: Runtime replay tests/benchmark path verification.
- **Result**: Approach remains in place.
- **Evidence**: Replay fixtures and benchmark entry retained.
- **Follow-up**: Add more scenario fixtures when behavior surface expands.
- **Change Ref**: CHG-20260426-004

## Current

### Entry C-001
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Use provider adapter wiring for all supported provider types in CLI run/resume path.
- **Why**: Remove behavior mismatch where non-dashscope providers fell back to fake model.
- **Scope**: `cmd/agent/cli.go`, `internal/llm/*`, `internal/runtime/model_adapter.go`.
- **Command**: `go test ./cmd/agent ./internal/config ./internal/runtime`
- **Result**: `openai` / `anthropic` / `dashscope` now all use `llm.NewProvider + runtime.NewModelAdapter`; openai/anthropic are stub adapters, dashscope is real HTTP adapter.
- **Evidence**: Updated CLI/provider wiring and provider-path tests.
- **Follow-up**: Replace stub providers with real SDK-backed implementations when needed.
- **Change Ref**: CHG-20260426-001

### Entry C-002
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Bind verifier selection to `verify_mode` in CLI startup.
- **Why**: Ensure config field has real runtime effect and reduce doc-code drift.
- **Scope**: `cmd/agent/cli.go`, `internal/verify/*`, related tests.
- **Command**: `go test ./cmd/agent ./internal/verify`
- **Result**: `off/standard/strict` selection path is wired and tested.
- **Evidence**: `newVerifierFromConfig` and mode-specific tests.
- **Follow-up**: Expand strict evidence checks with less heuristic matching.
- **Change Ref**: CHG-20260426-002

### Entry C-003
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Add automated notepad governance check and expose it via `make notecheck`.
- **Why**: Catch missing sections, duplicate entry IDs, and invalid/missing `Change Ref` before commit.
- **Scope**: `scripts/check-notepads.ps1`, `Makefile`, notepads governance flow.
- **Command**: `powershell -ExecutionPolicy Bypass -File ./scripts/check-notepads.ps1`
- **Result**: Governance checks are machine-enforced and repeatable.
- **Evidence**: New script validates section presence, `Entry` uniqueness, and `Change Ref` linkage to `change-log.md`.
- **Follow-up**: Add this check into CI if notepads become mandatory for every PR.
- **Change Ref**: CHG-20260426-005

### Entry C-004
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Enforce notepad governance check in CI workflow.
- **Why**: Make governance violations fail fast in PR validation, not only in local runs.
- **Scope**: `.github/workflows/ci.yml`, governance lifecycle.
- **Command**: CI step `Run notepad governance check` executes `./scripts/check-notepads.ps1` under `pwsh`.
- **Result**: Notepad governance became part of mandatory CI checks.
- **Evidence**: CI workflow now includes dedicated governance step before `go vet` and test stages.
- **Follow-up**: Keep script runtime cross-platform-compatible for hosted runners.
- **Change Ref**: CHG-20260426-006

### Entry C-005
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Run CI governance check conditionally based on path changes.
- **Why**: Avoid unnecessary governance step execution on PRs with no notepad/governance file changes.
- **Scope**: `.github/workflows/ci.yml`.
- **Command**: Use `dorny/paths-filter@v3` and gate governance step with `if: steps.note_changes.outputs.notepads == 'true'`.
- **Result**: Governance enforcement stays accurate while CI runtime improves.
- **Evidence**: New `Detect notepad changes` step with filters for `.sisyphus/notepads/**` and `scripts/check-notepads.ps1`.
- **Follow-up**: Expand filter list if governance dependencies grow.
- **Change Ref**: CHG-20260426-007

### Entry C-006
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Promote `edit_file` to multiline-safe block protocol with explicit section markers.
- **Why**: Single-line old-text framing was insufficient for real code block edits and created protocol ambiguity.
- **Scope**: `internal/tools/adapters/files.go`, `internal/tools/executor.go`, `internal/tools/safety.go`, `internal/tools/tools_test.go`.
- **Command**: `go test ./internal/tools`
- **Result**: `edit_file` now supports multiline old/new text with deterministic parse and replacement behavior.
- **Evidence**: New parser requires `<<<OLD` + `<<<NEW` blocks; tests now cover multiline old/new success, malformed payload, ambiguity, not-found, and traversal rejection.
- **Follow-up**: If needed later, add delimiter-escaping strategy for rare marker-collision content.
- **Change Ref**: CHG-20260426-008

### Entry C-007
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Harden strict verification using structured command evidence and exit-code semantics.
- **Why**: Heuristic-only checks (`contains go test/go build/go vet`) were vulnerable to false positives.
- **Scope**: `internal/tools/adapters/shell.go`, `internal/verify/checks.go`, `internal/verify/verify_test.go`.
- **Command**: `go test ./internal/verify ./internal/tools`
- **Result**: Verifier prefers structured command records (`COMMAND` + `EXIT_CODE`) and still supports legacy text evidence fallback.
- **Evidence**: Added structured success/failure tests and command output envelope generation.
- **Follow-up**: Extend verifier to use command timestamps/step correlation if stronger provenance is required.
- **Change Ref**: CHG-20260426-009

### Entry C-008
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Add adapter-level DashScope boundary tests for production-like failure modes.
- **Why**: Provider adapter had no direct tests for timeout/non-2xx/empty-content edge cases.
- **Scope**: `internal/llm/dashscope_test.go`.
- **Command**: `go test ./internal/llm`
- **Result**: Timeout, 429 envelope handling, empty-content rejection, and success response contract are covered.
- **Evidence**: New `httptest`-backed tests assert headers, status/error propagation, and output parsing expectations.
- **Follow-up**: Add retry/backoff policy tests if retry logic is introduced later.
- **Change Ref**: CHG-20260426-010

### Entry C-009
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Produce a single Final Verification Wave closeout report for F1-F4.
- **Why**: Keep final acceptance evidence centralized and auditable before requesting explicit user sign-off.
- **Scope**: `.sisyphus/notepads/general-agent-harness-go/final-verification-report.md`.
- **Command**: consolidate outputs from F1/F2/F3/F4 audits and post-remediation verification commands.
- **Result**: Final verification record now provides one-stop pass/fail summary and closure conditions.
- **Evidence**: Report captures F1-F4 statuses and references the post-fix verification state.
- **Follow-up**: After user approval, update plan checklist F1-F4 to checked.
- **Change Ref**: CHG-20260426-011

## Verification

### Entry V-001
- **Time**: 2026-04-26 (Asia/Shanghai)
- **Decision**: Validate current decision set against executable evidence.
- **Why**: Prevent decision notes from diverging from code reality.
- **Scope**: Whole repository.
- **Command**:
  - `go test ./...`
  - `go build ./...`
- **Result**: Passed.
- **Evidence**: Latest verification outputs are green across packages.
- **Follow-up**: Keep this decision-note template for subsequent updates.
- **Change Ref**: CHG-20260426-004
