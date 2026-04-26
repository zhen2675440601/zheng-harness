# Pre-Commit Doc Consolidation

## TL;DR
> **Summary**: Normalize repository-tracked documentation before pushing so another machine can continue work from git alone, with one clear project positioning and one clear progress authority.
> **Deliverables**:
> - Unified project wording: `通用 Agent Harness`
> - README reduced to project entrypoint + concise status summary
> - PROGRESS.md promoted to single project progress authority
> - docs/USAGE.md constrained to CLI/operator usage only
> - Commit/push notes that explain cross-machine continuation boundaries
> **Effort**: Short
> **Parallel**: NO
> **Critical Path**: 1 → 2 → 3 → 4 → 5

## Context
### Original Request
Prepare the repository for a push before implementation moves to another machine. The user wants the docs and progress artifacts cleaned up so continuation after `git pull` is straightforward.

### Interview Summary
- The project should be described as a **通用 Agent Harness**, not a coding agent.
- README should keep only a concise current-status summary and point to `PROGRESS.md` instead of duplicating detailed progress tables.
- `PROGRESS.md` should become the single project progress authority.
- `PROGRESS.md` “下一步建议” should be replaced with a pointer to the Phase 3 plan.
- Cross-machine continuation should rely on repo-tracked artifacts and local environment setup, not machine-local state.

### Metis Review (gaps addressed)
- Added exact canonical positioning phrase: **通用 Agent Harness**.
- Confirmed `.gitignore` already excludes `.sisyphus/boulder.json`, so machine-local state is not intended for git handoff.
- Added terminology-scan acceptance criteria to ensure no stale “Coding Agent” wording remains in updated files.
- Kept scope limited to docs/plan artifacts only; no code/runtime changes.

## Work Objectives
### Core Objective
Make the repository self-explanatory for the next machine by clarifying which file is the entrypoint, which file is the source of truth for progress, and which artifacts are portable versus local-only.

### Deliverables
- Updated `README.md`
- Updated `PROGRESS.md`
- Updated `docs/USAGE.md`
- Cross-machine continuation notes in repo-tracked docs
- Push/commit summary text prepared from the consolidated state

### Definition of Done (verifiable conditions with commands)
- `README.md`, `PROGRESS.md`, and `docs/USAGE.md` all describe the project as **通用 Agent Harness**.
- README no longer contains the detailed progress ledger that belongs in `PROGRESS.md`.
- `PROGRESS.md` clearly states Phase 1 complete, Phase 2 complete, and Phase 3 plan ready at `.sisyphus/plans/phase-3-general-task-protocol.md`.
- Updated docs explain which artifacts are repo-tracked and which are machine-local.
- A terminology scan on updated docs shows no stale `Coding Agent`/`coding agent` phrasing in those normalized files.

### Must Have
- Keep Chinese as the primary prose language in README/PROGRESS/USAGE for consistency with current docs.
- Use one identical positioning phrase across the updated top-level docs: **通用 Agent Harness**.
- Keep `.sisyphus/plans/phase-3-general-task-protocol.md` referenced as the next execution plan.
- Preserve quick-start and CLI usage clarity.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- Must NOT rewrite ADR files in this consolidation pass.
- Must NOT change runtime/code behavior.
- Must NOT create a separate migration subsystem or new cross-machine state file.
- Must NOT leave README and PROGRESS with overlapping full progress ledgers.
- Must NOT document `.sisyphus/boulder.json` as a shared continuation input.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: docs-only verification using file review + grep/scan + optional regression command if any command examples changed materially.
- QA policy: each task includes a deterministic doc-state validation step.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
Wave 1: normalize top-level entry docs (`1-3`)

Wave 2: add continuation guidance and pre-push validation (`4-5`)

### Dependency Matrix (full, all tasks)
- 1 blocks 2, 4, 5
- 2 blocks 4, 5
- 3 blocks 4, 5
- 4 blocks 5

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 3 tasks → `writing`
- Wave 2 → 2 tasks → `writing`, `unspecified-low`

## TODOs

- [x] 1. Normalize README into project entrypoint plus concise status summary

  **What to do**: Rewrite `README.md` so it remains the project entrypoint: project positioning, quick start, architecture overview, contributor workflow, and a short current-status summary only. Replace the detailed “当前进度” table with a concise phase summary that points readers to `PROGRESS.md` for authoritative progress tracking.
  **Must NOT do**: Do not keep the full T1-T11 progress table in README; do not leave any `Coding Agent` positioning phrase; do not remove quick-start essentials.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: documentation restructuring and wording alignment
  - Skills: `[]`
  - Omitted: [`/playwright`] - docs only

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [2,4,5] | Blocked By: []

  **References**:
  - Pattern: `README.md:1-24` - current duplicated positioning and progress table to simplify
  - Pattern: `README.md:25-236` - sections to preserve as entrypoint material
  - Pattern: `PROGRESS.md:1-196` - destination authority for detailed phase/progress information

  **Acceptance Criteria** (agent-executable only):
  - [ ] README uses the exact phrase `通用 Agent Harness`
  - [ ] README current-status section is a concise summary, not a detailed ledger
  - [ ] README links readers to `PROGRESS.md` for full progress state

  **QA Scenarios**:
  ```
  Scenario: README becomes concise entrypoint
    Tool: Bash
    Steps: Review changed README and verify the detailed progress ledger is removed while quick-start sections remain
    Expected: README is shorter in progress detail and clearly points to PROGRESS.md
    Evidence: .sisyphus/evidence/task-1-readme-normalization.txt

  Scenario: README no longer uses stale positioning
    Tool: Bash
    Steps: Scan README for `Coding Agent` or conflicting positioning terms
    Expected: No stale positioning remains in README
    Evidence: .sisyphus/evidence/task-1-readme-normalization-error.txt
  ```

  **Commit**: YES | Message: `docs: normalize readme entrypoint` | Files: [`README.md`]

- [x] 2. Promote PROGRESS.md to the single project progress authority

  **What to do**: Update `PROGRESS.md` so it becomes the authoritative project status document. Keep the chronological/progress nature, but simplify duplicated setup narrative where it belongs elsewhere. Explicitly record: Phase 1 complete, Phase 2 complete, and Phase 3 plan ready at `.sisyphus/plans/phase-3-general-task-protocol.md`. Replace “下一步建议” with a “下一步执行入口” section that points to the Phase 3 plan and explains that implementation can continue on another machine after git sync plus local config setup.
  **Must NOT do**: Do not leave outdated “后续提交（本次）” examples as if they are still the next action; do not leave coding-agent wording; do not turn PROGRESS into a CLI manual.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: progress authority restructuring
  - Skills: `[]`
  - Omitted: [`/playwright`] - docs only

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [4,5] | Blocked By: [1]

  **References**:
  - Pattern: `PROGRESS.md:1-60` - current duplicated positioning and progress framing to correct
  - Pattern: `PROGRESS.md:160-177` - current git-operation history to simplify or reframe
  - Pattern: `PROGRESS.md:179-184` - next-step section to replace with Phase 3 execution entry
  - External: `.sisyphus/plans/phase-3-general-task-protocol.md` - canonical next-phase plan to reference

  **Acceptance Criteria** (agent-executable only):
  - [ ] PROGRESS uses the exact phrase `通用 Agent Harness`
  - [ ] PROGRESS explicitly records Phase 1 done, Phase 2 done, Phase 3 plan ready
  - [ ] PROGRESS contains a clear next-step pointer to `.sisyphus/plans/phase-3-general-task-protocol.md`

  **QA Scenarios**:
  ```
  Scenario: PROGRESS becomes single progress authority
    Tool: Bash
    Steps: Review updated PROGRESS structure and verify phase status plus next-step pointer are explicit
    Expected: A contributor can identify current phase state and next execution entry from PROGRESS alone
    Evidence: .sisyphus/evidence/task-2-progress-authority.txt

  Scenario: PROGRESS no longer behaves like duplicate README/usage guide
    Tool: Bash
    Steps: Compare updated PROGRESS with README and USAGE responsibilities
    Expected: PROGRESS tracks project status rather than duplicating onboarding or CLI manual detail
    Evidence: .sisyphus/evidence/task-2-progress-authority-error.txt
  ```

  **Commit**: YES | Message: `docs: promote progress as source of truth` | Files: [`PROGRESS.md`]

- [x] 3. Constrain docs/USAGE.md to operator and CLI usage only

  **What to do**: Update `docs/USAGE.md` so it describes the CLI as part of a **通用 Agent Harness** and remove wording that frames the entire project as a coding agent. Keep command examples, flags, config order, and SQLite usage. Add a brief note that this file is for CLI operation, while project status and next-phase planning live in `PROGRESS.md` and `.sisyphus/plans/` respectively.
  **Must NOT do**: Do not expand USAGE into project progress; do not add phase implementation detail here; do not leave conflicting project positioning.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: CLI docs role clarification
  - Skills: `[]`
  - Omitted: [`/playwright`] - docs only

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [4,5] | Blocked By: [1]

  **References**:
  - Pattern: `docs/USAGE.md:1-5` - current coding-agent wording to normalize
  - Pattern: `docs/USAGE.md:37-253` - valid CLI content to preserve
  - Pattern: `README.md` - top-level project entrypoint that USAGE should complement, not duplicate

  **Acceptance Criteria** (agent-executable only):
  - [ ] USAGE uses the exact phrase `通用 Agent Harness` or an equivalent sentence anchored on that phrase
  - [ ] USAGE remains CLI/operator-focused without becoming a project-progress file
  - [ ] USAGE references where to find project status and next-phase plan

  **QA Scenarios**:
  ```
  Scenario: USAGE stays operator-focused
    Tool: Bash
    Steps: Review updated docs/USAGE.md for command/flag/config coverage
    Expected: USAGE remains a clear CLI manual with no duplicate project progress ledger
    Evidence: .sisyphus/evidence/task-3-usage-role.txt

  Scenario: USAGE no longer presents the project as a coding agent
    Tool: Bash
    Steps: Scan docs/USAGE.md for stale wording
    Expected: No coding-agent positioning remains in the updated file
    Evidence: .sisyphus/evidence/task-3-usage-role-error.txt
  ```

  **Commit**: YES | Message: `docs: clarify cli usage role` | Files: [`docs/USAGE.md`]

- [x] 4. Add explicit cross-machine continuation notes to repo-tracked docs

  **What to do**: Add concise continuation guidance in the normalized docs so another machine can continue work after `git pull`. The guidance must distinguish repo-tracked artifacts (`README.md`, `PROGRESS.md`, `docs/`, `.sisyphus/plans/`, `.sisyphus/notepads/`) from machine-local artifacts (`zheng.json` secrets, `agent.db`, `.sisyphus/boulder.json`). Confirm `.gitignore` policy remains aligned with this guidance.
  **Must NOT do**: Do not document machine-local files as shared project state; do not introduce a new handoff subsystem.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: concise operational guidance with repo-state boundaries
  - Skills: `[]`
  - Omitted: [`/playwright`] - docs only

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [5] | Blocked By: [1,2,3]

  **References**:
  - Pattern: `.gitignore:27-29` - `.sisyphus/boulder.json` already treated as local-only state
  - Pattern: `README.md`, `PROGRESS.md`, `docs/USAGE.md` - insertion points for continuation notes
  - External: `.sisyphus/plans/phase-3-general-task-protocol.md` - next-machine execution target

  **Acceptance Criteria** (agent-executable only):
  - [ ] Updated docs explicitly separate repo-tracked continuation artifacts from machine-local state
  - [ ] `.sisyphus/boulder.json` is not presented as a git-synced prerequisite
  - [ ] A new machine can identify what to copy/configure locally versus what comes from git

  **QA Scenarios**:
  ```
  Scenario: Cross-machine continuation guidance is explicit
    Tool: Bash
    Steps: Review updated docs and compare against `.gitignore` machine-local policy
    Expected: Portable vs local-only artifacts are unambiguous and consistent
    Evidence: .sisyphus/evidence/task-4-cross-machine-notes.txt

  Scenario: Local-only state is not misdocumented
    Tool: Bash
    Steps: Scan updated docs for authoritative continuation instructions involving `.sisyphus/boulder.json`
    Expected: No updated doc requires `.sisyphus/boulder.json` for cross-machine continuation
    Evidence: .sisyphus/evidence/task-4-cross-machine-notes-error.txt
  ```

  **Commit**: YES | Message: `docs: add cross-machine continuation guidance` | Files: [`README.md`, `PROGRESS.md`, `docs/USAGE.md`, `.gitignore` only if needed]

- [x] 5. Prepare final pre-push validation and commit explanation

  **What to do**: Run a final documentation consistency pass and prepare the exact commit message plus short push explanation. Verify the normalized files all use the chosen positioning phrase, verify the Phase 3 plan reference is present, and verify stale `Coding Agent` wording is removed from the updated files. Prepare the final commit summary so the remote history explains why this push matters for the machine handoff.
  **Must NOT do**: Do not create a vague commit message like `update docs`; do not push before the doc-role normalization is internally consistent.

  **Recommended Agent Profile**:
  - Category: `unspecified-low` - Reason: final validation and concise release note style summary
  - Skills: `[]`
  - Omitted: [`/playwright`] - not needed

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [] | Blocked By: [1,2,3,4]

  **References**:
  - Pattern: `README.md`, `PROGRESS.md`, `docs/USAGE.md` - final consistency targets
  - External: `.sisyphus/plans/phase-3-general-task-protocol.md` - required next-phase reference
  - Pattern: `.gitignore:27-29` - local-state exclusion reminder

  **Acceptance Criteria** (agent-executable only):
  - [ ] Final validation confirms updated docs consistently use `通用 Agent Harness`
  - [ ] Final validation confirms Phase 3 plan reference exists where intended
  - [ ] Commit message and push explanation are ready and specific to the handoff purpose

  **QA Scenarios**:
  ```
  Scenario: Terminology and linkage validation pass
    Tool: Bash
    Steps: Run grep/scan checks across updated files for `Coding Agent`, `coding agent`, and `phase-3-general-task-protocol`
    Expected: No stale wording remains in updated docs and the Phase 3 plan reference is present
    Evidence: .sisyphus/evidence/task-5-prepush-validation.txt

  Scenario: Commit explanation is specific and handoff-ready
    Tool: Bash
    Steps: Review prepared commit title/body and push notes against updated repository state
    Expected: Message explains doc consolidation, positioning alignment, and next-machine continuation purpose
    Evidence: .sisyphus/evidence/task-5-prepush-validation-error.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: [none]

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Create one documentation-focused commit after all doc-role normalization is complete.
- Recommended commit message: `docs(project): consolidate docs for cross-machine continuation`
- Recommended commit body:
  - `align project positioning to 通用 Agent Harness`
  - `reduce README to entrypoint summary and move authoritative progress to PROGRESS.md`
  - `document Phase 3 handoff path for implementation on another machine`

## Success Criteria
- Another machine can `git pull` and determine current state, next plan, and local setup needs from repo-tracked docs alone.
- README, PROGRESS, and USAGE no longer fight over the same responsibilities.
- The repository history clearly records why this push was made before the machine switch.
