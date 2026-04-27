# v1 Release Preparation Minimum

## TL;DR
> **Summary**: Prepare the repository for a clean v1 release handoff by auditing repo truth, reconciling externally visible documentation, re-running the validated acceptance commands, and producing a publish-ready checklist. This plan explicitly stops before any real release action.
> **Deliverables**:
> - Release-prep consistency audit notes
> - Consolidated v1 release notes / release summary artifact
> - Fresh verification evidence for final acceptance commands
> - Publish-ready checklist with pass/fail status and blockers
> **Effort**: Short
> **Parallel**: YES - 2 waves
> **Critical Path**: 1 → 2 → 3 → 4

## Context
### Original Request
四阶段已完成。为当前仓库规划 v1 收尾发布的最小下一步。

### Interview Summary
- User confirmed Phase 4 is complete.
- User chose **v1收尾发布** as the next planning direction.
- User chose **最小下一步** as the desired planning granularity.
- User explicitly chose **仅发布准备** as the scope boundary.
- Actual release actions are excluded: no tag, no GitHub Release publishing, no merge execution, no public announcement.

### Metis Review (gaps addressed)
- Added explicit anti-scope-creep guardrails so release prep cannot drift into v2 planning or feature work.
- Narrowed the minimum task set to four concrete tasks: consistency audit, release-note consolidation, verification rerun, publish-ready checklist.
- Added executable acceptance criteria and evidence targets for every task.
- Incorporated edge-case handling for test regressions, doc/behavior mismatches, and sensitive local files.

## Work Objectives
### Core Objective
Create a minimal but decision-complete release-preparation pass that proves the repository is internally consistent, externally documented, freshly validated, and ready for a human to perform the actual v1 release later.

### Deliverables
- A repo-truth audit covering README, PROGRESS, USAGE, validation matrix, and release boundary statements.
- A concise v1 release summary artifact derived from validated repository facts.
- A fresh final verification evidence set for the documented acceptance commands.
- A publish-ready checklist enumerating pass/fail status, blockers, and explicitly deferred real release actions.

### Definition of Done (verifiable conditions with commands)
- Required release-prep artifacts exist and contain current Phase 4-complete status.
- Final acceptance commands complete successfully and evidence files are captured.
- Documentation claims about supported task types, verify modes, and current phase status align with repo truth.
- Publish-ready checklist explicitly states that real release execution is out of scope.

### Must Have
- Strictly preparation-only scope
- Fresh evidence in `.sisyphus/evidence/`
- No new product scope, no v2 roadmap work
- Clear blocker reporting if any verification or consistency check fails

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- Must NOT create git tags, GitHub releases, or merge branches
- Must NOT add features, refactors, or opportunistic cleanup unrelated to release preparation
- Must NOT change runtime behavior unless a release-blocking inconsistency is explicitly discovered and separately approved
- Must NOT commit sensitive local-only files such as `zheng.json`, `agent.db`, `*.db`, `*.sqlite`, or `.sisyphus/boulder.json`
- Must NOT introduce vague checklist items that require human interpretation to verify

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: tests-after + Go testing framework
- QA policy: Every task has agent-executed scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. <3 per wave (except final) = under-splitting.
> Extract shared dependencies as Wave-1 tasks for max parallelism.

Wave 1: Task 1 repo truth audit, Task 2 release-note consolidation
Wave 2: Task 3 final verification rerun, Task 4 publish-ready checklist

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks |
|---|---|---|
| 1. Repo truth audit | None | 2, 4 |
| 2. Release summary consolidation | 1 | 4 |
| 3. Final verification rerun | None | 4 |
| 4. Publish-ready checklist | 1, 2, 3 | Final verification wave |

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 2 tasks → writing, unspecified-low
- Wave 2 → 2 tasks → unspecified-low, writing
- Final Verification Wave → 4 tasks → oracle, unspecified-high, deep

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Audit repository truth for v1 release-prep scope

  **What to do**: Compare externally visible repository status statements across `README.md`, `PROGRESS.md`, `docs/USAGE.md`, and `docs/validation-matrix.md`. Confirm that Phase 1-4 complete status, supported task types (`coding`, `research`, `file_workflow`, `general`), validation claims, and sensitive local-file boundaries are mutually consistent. Record mismatches and classify each as either documentation defect or release blocker.
  **Must NOT do**: Must NOT expand into code refactoring, feature additions, or v2 roadmap analysis.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: This is repo-truth reconciliation across user-facing docs with precise wording requirements.
  - Skills: `[]` - No extra skill required.
  - Omitted: `["/playwright"]` - No browser interaction is needed.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 2, 4 | Blocked By: none

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `README.md:9-15` - Current project-wide phase completion statement and validation-matrix reference.
  - Pattern: `README.md:53-72` - Supported `--task-type` values and verifier behavior.
  - Pattern: `README.md:203-214` - Contributor verification workflow.
  - Pattern: `PROGRESS.md:7-13` - Progress summary and validation matrix as authority source.
  - Pattern: `PROGRESS.md:227-244` - Portable vs local-only state and sensitive-file boundaries.
  - Pattern: `docs/USAGE.md:3-6` - CLI contract and validation matrix reference.
  - Pattern: `docs/USAGE.md:70-79` - Task-type verification strategy and verify-mode description.
  - Pattern: `docs/validation-matrix.md:1-8` - Phase 4 validated status.
  - Pattern: `docs/validation-matrix.md:181-194` - Final acceptance commands to be reused in Task 3.

  **Acceptance Criteria** (agent-executable only):
  - [ ] A written audit artifact exists that enumerates each checked document and states whether it is consistent or mismatched.
  - [ ] Every mismatch is labeled either `doc-fix-needed` or `release-blocker`.
  - [ ] The audit explicitly confirms that actual release actions remain out of scope.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Happy path repo-truth audit completed
    Tool: Bash
    Steps: Generate an audit artifact summarizing checks for README.md, PROGRESS.md, docs/USAGE.md, docs/validation-matrix.md and save it to .sisyphus/evidence/task-1-repo-truth-audit.txt
    Expected: Evidence file exists and contains one section per checked document plus an overall consistency conclusion
    Evidence: .sisyphus/evidence/task-1-repo-truth-audit.txt

  Scenario: Failure path documentation mismatch found
    Tool: Bash
    Steps: Ensure the audit artifact records any mismatch with an explicit label `doc-fix-needed` or `release-blocker`
    Expected: Mismatch entries are concrete, file-scoped, and binary classified rather than vague prose
    Evidence: .sisyphus/evidence/task-1-repo-truth-audit-error.txt
  ```

  **Commit**: NO | Message: `docs(release): reconcile repo truth for v1 prep` | Files: `README.md`, `PROGRESS.md`, `docs/USAGE.md`, `docs/validation-matrix.md`

- [x] 2. Consolidate a minimal v1 release summary artifact

  **What to do**: Create a concise release-summary artifact derived only from validated repository facts: project purpose, supported task types, architectural boundaries, completed phases, key reliability proofs from Phase 4, and explicit non-goals for v1. Use the audit from Task 1 to avoid restating known inconsistencies. The artifact should be suitable as the source text for future release notes, but must stop before any actual publishing step.
  **Must NOT do**: Must NOT invent roadmap claims, performance claims, or usage guarantees not supported by repository sources.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: This is a synthesis task turning validated repo truth into externally consumable release-summary language.
  - Skills: `[]` - No extra skill required.
  - Omitted: `["/review-work"]` - Final review is already handled in the plan’s verification wave.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 4 | Blocked By: 1

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `README.md:3-7` - Product description and positioning.
  - Pattern: `README.md:11-15` - Phase completion status.
  - Pattern: `README.md:141-181` - Architecture overview and technology stack.
  - Pattern: `README.md:216-220` - Explicit v1 exclusions.
  - Pattern: `PROGRESS.md:246-254` - Phase 3 core成果 and generalized task protocol summary.
  - Pattern: `docs/validation-matrix.md:11-118` - Validated proof surfaces across CLI, verifier routing, replay, rejection handling, and config.
  - Pattern: `docs/validation-matrix.md:198-201` - Final validated-state summary.

  **Acceptance Criteria** (agent-executable only):
  - [ ] A release-summary artifact exists with sections for product scope, completed phases, validated capabilities, and explicit v1 non-goals.
  - [ ] Every substantive claim in the artifact is traceable to at least one repository source listed in the task references.
  - [ ] The artifact explicitly states that tag/release publication is deferred.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Happy path release summary generated
    Tool: Bash
    Steps: Save the consolidated release summary to .sisyphus/evidence/task-2-v1-release-summary.md
    Expected: File exists and includes sections for scope, validated capabilities, and non-goals
    Evidence: .sisyphus/evidence/task-2-v1-release-summary.md

  Scenario: Failure path unsupported claim check
    Tool: Bash
    Steps: Review the release summary against the listed references and mark any unsupported sentence in a companion note file
    Expected: Companion note either states `no unsupported claims found` or lists exact unsupported lines
    Evidence: .sisyphus/evidence/task-2-v1-release-summary-error.txt
  ```

  **Commit**: NO | Message: `docs(release): draft minimal v1 release summary` | Files: `README.md`, `PROGRESS.md`, `docs/validation-matrix.md`

- [x] 3. Re-run final acceptance commands and refresh evidence set

  **What to do**: Execute the documented final acceptance commands from `docs/validation-matrix.md`, capture fresh outputs into `.sisyphus/evidence/`, and compare high-level outcomes against the claims made in README and USAGE. If any command fails, stop the release-prep flow and mark the failure as a release blocker with exact command output reference.
  **Must NOT do**: Must NOT silently skip slow commands, relax test coverage, or replace failing commands with partial substitutes.

  **Recommended Agent Profile**:
  - Category: `unspecified-low` - Reason: This is a bounded command-execution and evidence-capture task with no design ambiguity.
  - Skills: `[]` - No extra skill required.
  - Omitted: `["/frontend-ui-ux"]` - No UI/design work is involved.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 4 | Blocked By: none

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `docs/validation-matrix.md:163-177` - Prior evidence-file mapping.
  - Pattern: `docs/validation-matrix.md:181-194` - Canonical final acceptance commands.
  - Pattern: `README.md:31-45` - Publicly documented test commands.
  - Pattern: `docs/USAGE.md:23-37` - Publicly documented test commands in CLI docs.

  **Acceptance Criteria** (agent-executable only):
  - [ ] Fresh evidence files exist for build, full test, race test, coverage test, and the focused regression commands listed in the validation matrix.
  - [ ] A summary evidence artifact lists each command with PASS/FAIL status and output file path.
  - [ ] Any failure is surfaced as `release-blocker` with the exact failing command.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Happy path final acceptance rerun passes
    Tool: Bash
    Steps: Run the validation-matrix final acceptance commands and save outputs to .sisyphus/evidence/task-3-build.txt, task-3-test-full.txt, task-3-test-race.txt, task-3-test-cover.txt, task-3-focused-regressions.txt, plus a summary file .sisyphus/evidence/task-3-verification-summary.txt
    Expected: Summary file marks every command PASS and points to all output artifacts
    Evidence: .sisyphus/evidence/task-3-verification-summary.txt

  Scenario: Failure path command regression detected
    Tool: Bash
    Steps: If any command fails, append the command, exit status, and output file path to .sisyphus/evidence/task-3-verification-failure.txt and stop downstream release-prep tasks
    Expected: Failure artifact exists and clearly marks the issue as a release blocker
    Evidence: .sisyphus/evidence/task-3-verification-failure.txt
  ```

  **Commit**: NO | Message: `chore(release): refresh final verification evidence` | Files: `.sisyphus/evidence/*`

- [x] 4. Produce publish-ready checklist for deferred human release

  **What to do**: Build a final checklist that consumes outputs from Tasks 1-3 and answers one question only: “Is the repository ready for a human to perform v1 release actions later?” Include checklist rows for documentation consistency, final verification pass, sensitive-file hygiene, release summary availability, and unresolved blockers. End with explicit deferred steps that are intentionally out of scope: tagging, publishing GitHub Release, merging, and announcement.
  **Must NOT do**: Must NOT execute any deferred step or imply that release publication already happened.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: This is structured synthesis of audit and verification outputs into a go/no-go preparation checklist.
  - Skills: `[]` - No extra skill required.
  - Omitted: `["/git-master"]` - No git execution is allowed in this release-prep-only boundary.

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: Final verification wave | Blocked By: 1, 2, 3

  **References** (executor has NO interview context - be exhaustive):
  - Pattern: `.sisyphus/evidence/task-1-repo-truth-audit.txt` - Source for documentation consistency status.
  - Pattern: `.sisyphus/evidence/task-2-v1-release-summary.md` - Source for the release summary artifact.
  - Pattern: `.sisyphus/evidence/task-3-verification-summary.txt` - Source for command pass/fail status.
  - Pattern: `PROGRESS.md:234-244` - Sensitive local-only file boundaries and safety guidance.
  - Pattern: `README.md:216-220` - v1 explicit non-goals to preserve in checklist messaging.

  **Acceptance Criteria** (agent-executable only):
  - [ ] Checklist artifact exists and gives a binary ready/not-ready conclusion.
  - [ ] Every checklist row cites at least one source artifact or file.
  - [ ] Checklist contains an explicit “Deferred Real Release Actions” section listing tag, GitHub Release, merge, and announcement as out of scope.

  **QA Scenarios** (MANDATORY - task incomplete without these):
  ```
  Scenario: Happy path checklist generated
    Tool: Bash
    Steps: Save the publish-ready checklist to .sisyphus/evidence/task-4-publish-ready-checklist.md using Tasks 1-3 evidence as inputs
    Expected: Checklist ends with a binary conclusion and a deferred-actions section
    Evidence: .sisyphus/evidence/task-4-publish-ready-checklist.md

  Scenario: Failure path blocker propagation
    Tool: Bash
    Steps: If any upstream artifact marks `release-blocker`, include a blocker table in the checklist and mark overall status NOT READY
    Expected: Checklist clearly propagates upstream blockers without masking them
    Evidence: .sisyphus/evidence/task-4-publish-ready-checklist-error.md
  ```

  **Commit**: NO | Message: `docs(release): add publish-ready checklist` | Files: `.sisyphus/evidence/*`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer a single release-prep documentation/evidence commit after Tasks 1-4 pass.
- If documentation fixes and evidence refresh must be separated for traceability, use two commits maximum:
  1. `docs(release): reconcile v1 release-prep documentation`
  2. `chore(release): refresh final verification evidence and checklist`
- Never include local-only sensitive files in any commit.

## Success Criteria
- A future executor can determine release readiness without rereading the entire repository.
- All release-prep decisions are captured with explicit evidence and no hidden judgment calls.
- Any remaining issue is surfaced as a named blocker rather than buried in prose.
- The plan remains strictly within release-preparation scope and stops before public release execution.
