# Learnings - v1 Release Prep Minimum

## Task 3: Final Acceptance Commands Re-run (2026-04-27)

### Evidence Files Created
All evidence files created successfully in `.sisyphus/evidence/`:
- `task-3-build.txt` — Build verification output
- `task-3-test-full.txt` — Full test suite output
- `task-3-test-race.txt` — Race detector output
- `task-3-test-cover.txt` — Coverage report output
- `task-3-focused-regressions.txt` — Focused regression tests output
- `task-3-verification-summary.txt` — Summary with PASS/FAIL status

### Key Findings
1. **All acceptance commands passed** — No release blockers detected
2. **Race detector clean** — No race conditions found
3. **Coverage summary**:
   - Highest: internal/memory (93.1%), internal/config/prompts (88.7%)
   - Lowest: internal/domain (44.8%), internal/tools/adapters (52.2%)
4. **Focused regression tests**: All 4 tests passed
   - Interrupt fix test
   - Config provider switching test
   - Runtime replay test
   - Verifier dispatch tests

### PowerShell Output Capture Notes
- Used `2>&1` to capture both stdout and stderr
- Used `Out-File -Encoding utf8` for file output
- Files may appear as binary to some readers due to UTF-16 encoding from PowerShell
- EXIT_CODE captured via `$LASTEXITCODE` for pass/fail determination

### Canonical Command List
Source: `docs/validation-matrix.md` lines 181-194
```bash
go build ./...
go test ./...
go test -race ./...
go test -cover ./...
go test ./cmd/agent/... -run TestRunCommandInterruptPersistsInterruptedSession
go test ./internal/config/... -run TestLoadUsesMultiProviderConfigAndSwitchesProvider
go test ./internal/runtime/... -run TestRuntimeReplay
go test ./internal/verify/...
```
## Task 2: v1 Release Summary Artifact (2026-04-27)

### Files Created

1. **.sisyphus/evidence/task-2-v1-release-summary.md** — Main release summary artifact
   - Product scope with positioning and purpose
   - Completed phases summary (Phase 1-4)
   - Validated capabilities: CLI surfaces, task-type routing, verify modes, runtime replay, rejection handling, config support
   - Architecture boundaries: Domain/Runtime/Infrastructure/Interface layers, core ports
   - v1 explicit non-goals (6 items)
   - Deferred release actions statement
   - All claims annotated with [source: filename:lines] references

2. **.sisyphus/evidence/task-2-v1-release-summary-error.txt** — Companion error-check file
   - Result: "no unsupported claims found"
   - All substantive claims verified against source documents
   - Methodology documented

### Key Learnings

1. **Source traceability is critical** — Every factual claim must have at least one repository source reference
2. **Separation of concerns** — Release summary artifact is distinct from actual release actions (tagging, publishing)
3. **Audit integration** — Task 1 audit findings (2 doc-fix-needed, no blockers) incorporated into validation evidence
4. **Deferred publication model** — This artifact provides foundation for future release notes without performing actual release

### Document Structure Pattern

The release summary follows a consistent pattern:
- Product Scope → Completed Phases → Validated Capabilities → Architecture Boundaries → Non-Goals → Deferred Actions → Validation Evidence
- Each section includes source annotations for verification
- Explicit statement that tag/release publication is deferred

### Validation Sources Used

- README.md: Product description, architecture, phase status, non-goals
- docs/validation-matrix.md: Validated proof surfaces, test results, acceptance commands
- PROGRESS.md: Phase 3 achievements, generalized task protocol
- .sisyphus/evidence/task-1-repo-truth-audit.txt: Consistency verification

### Artifacts Ready for Future Use

When release actions become appropriate, the summary provides:
- Ready-to-use release notes foundation
- Pre-validated capability statements
- Clear boundary documentation (what v1 includes/excludes)
- Traceable evidence chain

## Task 4: Publish-Ready Checklist (2026-04-27)

### Checklist Created

File: `.sisyphus/evidence/task-4-publish-ready-checklist.md`

### Checklist Structure

The checklist answers one question: "Is the repository ready for a human to perform v1 release actions later?"

**Conclusion: READY** (with 2 doc-fix-needed items noted, not blocking)

### Sections Included

1. **Documentation Consistency** — 8 rows covering phase status, task types, verify-mode, validation authority, sensitive-file boundaries, CLI commands, v1 non-goals
   - 2 mismatches identified (both doc-fix-needed, cosmetic/minor)
   - Source: `task-1-repo-truth-audit.txt` and `task-1-repo-truth-audit-error.txt`

2. **Final Verification Pass** — 8 rows covering all acceptance commands
   - All commands PASS (build, test, race, cover, 4 focused regressions)
   - Source: `task-3-verification-summary.txt`

3. **Sensitive-File Hygiene** — 9 rows documenting portable vs local-only files
   - Local-only: boulder.json, zheng.json, *.db, agent.db
   - Portable: plans/, notepads/, docs/, PROGRESS.md, README.md
   - Source: `PROGRESS.md:227-244`

4. **Release Summary Availability** — 4 rows confirming artifact existence and verification
   - Release summary exists (211 lines)
   - All claims verified against sources
   - Source: `task-2-v1-release-summary.md` and `task-2-v1-release-summary-error.txt`

5. **Unresolved Blockers** — 2 rows (both doc-fix-needed, no release-blocker)
   - Mismatch 1: PROGRESS.md uses ? instead of ✅ (cosmetic)
   - Mismatch 2: README.md missing explicit verify-mode documentation (minor gap)

6. **Overall Conclusion** — Binary READY status with rationale

7. **Deferred Real Release Actions** — Explicit list of out-of-scope actions:
   - Git tag creation
   - GitHub Release publication
   - Branch merge
   - Public announcement

### Key Pattern

Every checklist row cites at least one source artifact or file. This ensures traceability and allows future reviewers to verify the checklist conclusions.

### Release Readiness Criteria Met

- [x] All documentation mutually consistent on core factual claims
- [x] All acceptance commands pass
- [x] Release summary artifact exists and is claim-verified
- [x] No release blockers exist
- [x] Sensitive-file boundaries explicitly documented
- [x] Deferred actions clearly listed

### Findings

1. **Zero release blockers** — All mismatches are doc-fix-needed (cosmetic/minor)
2. **All tests pass** — Zero failures across build, test, race, cover, focused regressions
3. **Coverage healthy** — 44.8%-93.1% across 11 packages
4. **Artifacts complete** — All Tasks 1-4 evidence files present and verified
5. **Deferred model works** — Release prep separated from actual release actions

### Repository Status

The repository is READY for a human operator to perform v1 release actions (tag, release, merge, announce) at their discretion. The 2 doc-fix-needed items are noted but do not block release.

