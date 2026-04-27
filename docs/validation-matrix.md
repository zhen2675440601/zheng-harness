# Phase 4 Closed-Loop Validation Matrix

**Purpose**: This document maps every required proof surface to concrete tests, replay fixtures, CLI commands, expected outcomes, and evidence targets. All validation must be agent-executable with zero human judgment.

**Last Updated**: 2026-04-27  
**Phase**: 4 - Closed-Loop Validation  
**Status**: ✅ Validated - All blockers resolved

---

## Validation Surface Inventory

### 1. CLI Command Surfaces (`run` / `resume` / `inspect`)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **run creates persistent session** | `go test ./cmd/agent -run TestRunCommandJSONCreatesPersistentSession` | N/A | Session persisted to SQLite with status=success, plan saved, 1 step recorded | `cmd/agent/main_test.go` | ✅ PASS |
| **run supports max-steps flag** | `go test ./cmd/agent -run TestRunCommandSupportsMaxStepsFlag` | N/A | Session completes within bounded steps | `cmd/agent/main_test.go` | ✅ PASS |
| **run preserves task metadata** | `go test ./cmd/agent -run TestRunInspectAndResumePreserveTaskMetadata` | N/A | task_type, protocol_hint, verification_policy survive persistence | `cmd/agent/main_test.go` | ✅ PASS |
| **resume reconstructs session** | `go test ./cmd/agent -run TestResumeAndInspectOutput` | N/A | Resume output contains session ID, plan summary, step history | `cmd/agent/main_test.go` | ✅ PASS |
| **inspect reads without mutation** | `go test ./cmd/agent -run TestResumeAndInspectOutput` (inspect sub-test) | N/A | JSON output shows session status, step count, summaries | `cmd/agent/main_test.go` | ✅ PASS |
| **run/resume/inspect preserve task-type metadata** | `go test ./cmd/agent -run TestRunInspectAndResumePreserveTaskMetadata` | N/A | TaskCategoryResearch survives all lifecycle transitions | `cmd/agent/main_test.go` | ✅ PASS |
| **interrupt persists interrupted session** | `go test ./cmd/agent -run TestRunCommandInterruptPersistsInterruptedSession` | N/A | Session status=interrupted, plan persisted, recoverable | `cmd/agent/main_test.go` | ✅ PASS (Fixed) |

**CLI Coverage Summary**:
- Happy path: ✅ `run` → `resume` → `inspect` continuity proven
- Failure path: ✅ Interrupt persistence tested and passes
- Recovery path: ✅ Resume after interrupt proven

**Fix Applied**: CLI interrupt persistence uses `context.WithoutCancel(ctx)` for session/plan/step saves so persistence completes even when runtime context is canceled.

---

### 2. Task-Type Routing (`coding` / `research` / `file_workflow`)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **coding task routes to command verifier** | `go test ./internal/verify -run TestTaskAwareVerifierUsesCommandVerifierForCodingTasks` | N/A | Command verifier executes go test/build/vet | `internal/verify/task_aware_verifier_test.go` | ✅ PASS |
| **research task routes to evidence verifier** | `go test ./internal/verify -run TestTaskAwareVerifierUsesEvidenceVerifierForResearchTasks` | N/A | Evidence verifier checks sources/findings consistency | `internal/verify/task_aware_verifier_test.go` | ✅ PASS |
| **file_workflow routes to state-output verifier** | `go test ./internal/verify -run TestTaskAwareVerifierUsesStateOutputVerifierForFileWorkflowTasks` | N/A | State-output verifier checks file existence/content | `internal/verify/task_aware_verifier_test.go` | ✅ PASS |
| **research task executes end-to-end** | `go test ./internal/runtime -run TestRuntimeReplayResearchFixture` | `testdata/runtime/research_session.json` | Session completes with research evidence attached | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **file_workflow executes end-to-end** | `go test ./internal/runtime -run TestRuntimeReplayFileWorkflowFixture` | `testdata/runtime/file_workflow_session.json` | Session completes with file evidence attached | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **unknown task type falls back safely** | `go test ./internal/verify -run TestTaskAwareVerifierFallsBackToCompatibilityPolicyWhenTaskMetadataMissing` | N/A | Falls back to command verification without panic | `internal/verify/task_aware_verifier_test.go` | ✅ PASS |

**Task-Type Coverage Summary**:
- Happy path: ✅ All three categories (coding/research/file_workflow) have routing + end-to-end tests
- Failure path: ✅ Research evidence validation fails when source reference unknown
- Recovery path: ✅ Explicit verification policy override supported

---

### 3. Verifier Dispatch (`off` / `standard` / `strict`)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **verify_mode=off uses FakeVerifier** | `go test ./cmd/agent -run TestNewVerifierFromConfigRespectsVerifyMode` (off subcase) | N/A | FakeVerifier always passes if final response exists | `cmd/agent/main_test.go` | ✅ PASS |
| **verify_mode=standard uses TaskAwareVerifier** | `go test ./cmd/agent -run TestNewVerifierFromConfigRespectsVerifyMode` (standard subcase) | N/A | Task-aware verifier dispatched | `cmd/agent/main_test.go` | ✅ PASS |
| **verify_mode=strict uses TaskAwareVerifier** | `go test ./cmd/agent -run TestRunCLIVerifyModeStrictUsesCommandVerifier` | N/A | Task-aware verifier with strict policy | `cmd/agent/main_test.go` | ✅ PASS |
| **command verifier executes test/build/lint** | `go test ./internal/verify -run TestCommandVerifierRunsTestBuildLint` | N/A | All three commands executed and pass | `internal/verify/command_verifier_test.go` | ✅ PASS |
| **non-coding verifier rejects without evidence** | `go test ./internal/verify -run TestTaskAwareVerifierRepresentsNotApplicableYet` | N/A | Returns VerificationStatusNotApplicable with reason | `internal/verify/task_aware_verifier_test.go` | ✅ PASS |

**Verifier Coverage Summary**:
- Happy path: ✅ All three verify modes dispatch correctly
- Failure path: ✅ Research evidence validation fails explicitly on missing source references
- Recovery path: ✅ Explicit verification policy can override category default

---

### 4. Runtime Replay Fixtures

| Proof Surface | Fixture File | Test | Expected Outcome | Evidence Target | Status |
|--------------|--------------|------|------------------|-----------------|--------|
| **success path** | `testdata/runtime/success_session.json` | `TestRuntimeReplaySuccessFixture` | Completes in 1 step, verification passes | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **verification rejection** | `testdata/runtime/verification_reject.json` | `TestRuntimeReplayVerificationFailureFixture` | Retry budget exceeded, status=verification_failed | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **resume persistence** | `testdata/runtime/resume_session.json` | `TestRuntimeReplayResumeFixture` | Session survives SQLite persistence and resume | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **research flow** | `testdata/runtime/research_session.json` | `TestRuntimeReplayResearchFixture` | Evidence-backed research completion | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **file_workflow flow** | `testdata/runtime/file_workflow_session.json` | `TestRuntimeReplayFileWorkflowFixture` | File-state evidence validated | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **unsafe tool rejection** | `testdata/runtime/unsafe_tool_rejection.json` | `TestRuntimeReplayUnsafeToolRejectionFixture` | Tool allowlist blocks unsafe command | `internal/runtime/runtime_replay_test.go` | ✅ PASS |

**Replay Coverage Summary**:
- Happy path: ✅ Success, research, file_workflow, resume all covered
- Failure path: ✅ Verification failure, unsafe tool rejection covered
- Recovery path: ✅ Resume tested in fixture and CLI integration proven

---

### 5. Rejection / Failure Handling

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **verification failure terminates session** | `go test ./internal/runtime -run TestRuntimeReplayVerificationFailureFixture` | `testdata/runtime/verification_reject.json` | Status=verification_failed, retry budget exceeded | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **unsafe tool is blocked** | `go test ./internal/runtime -run TestRuntimeReplayUnsafeToolRejectionFixture` | `testdata/runtime/unsafe_tool_rejection.json` | Command rejected as not allowlisted | `internal/runtime/runtime_replay_test.go` | ✅ PASS |
| **context cancellation stops runtime** | `go test ./internal/runtime -run TestRuntimeInterruptsWhenContextCancelled` | N/A | Runtime exits gracefully on context cancellation | `internal/runtime/runtime_test.go` | ✅ PASS |
| **interrupt persists session state** | `go test ./cmd/agent -run TestRunCommandInterruptPersistsInterruptedSession` | N/A | Session saved with status=interrupted, plan persisted | `cmd/agent/main_test.go` | ✅ PASS (Fixed) |

**Rejection Coverage Summary**:
- Happy path: N/A (this is failure-path coverage)
- Failure path: ✅ Verification failure, tool rejection, context cancellation all covered
- Recovery path: ✅ Interrupt persistence and resume proven

---

### 6. Configuration / Multi-Provider

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **multi-provider config loads and switches** | `go test ./internal/config -run TestLoadUsesMultiProviderConfigAndSwitchesProvider` | N/A | Provider selection succeeds, model matches config | `internal/config/config_test.go` | ✅ PASS (Fixed) |
| **provider flag validates existence** | `go test ./cmd/agent -run TestRunCLIRejectsMissingSelectedProvider` | N/A | Error if --provider references undefined provider | `cmd/agent/main_test.go` | ✅ PASS |
| **env overrides work** | `go test ./internal/config -run TestValidConfigAndProviderBoundary` | N/A | Environment variables override config file | `internal/config/config_test.go` | ✅ PASS |

**Configuration Coverage Summary**:
- Happy path: ✅ Multi-provider config loading tested and passes
- Failure path: ✅ Missing provider rejected
- Recovery path: ✅ Provider switching returns correct model

**Fix Applied**: Multi-provider config uses `fs.Visit` to track explicitly-set CLI flags, preventing stale defaults from leaking into newly selected provider.

---

## Resolved Blockers (Phase 4 Task 5)

### Blocker 1: `TestRunCommandInterruptPersistsInterruptedSession` ✅ RESOLVED

- **Location**: `cmd/agent/main_test.go`
- **Original Failure**: Resume fails after persistence - "plan for session not found: sql: no rows in result set"
- **Root Cause**: Runtime persistence calls used cancelable context; on SIGINT, session row could exist while plan/step persistence lost race under canceled context
- **Fix Applied**: `cmd/agent/cli.go` - `sessionAliasStore` uses `context.WithoutCancel(ctx)` for all persistence writes so they complete even when runtime context is canceled
- **Verification**: `go test ./cmd/agent -run TestRunCommandInterruptPersistsInterruptedSession` passes

### Blocker 2: `TestLoadUsesMultiProviderConfigAndSwitchesProvider` ✅ RESOLVED

- **Location**: `internal/config/config_test.go`
- **Original Failure**: Expected model "gpt-4.1-mini", got "qwen3.6-plus"
- **Root Cause**: CLI flag defaults initialized from current provider before parsing; when switching providers without explicit model flag, old provider's model leaked into new provider
- **Fix Applied**: `internal/config/config.go` - Uses `fs.Visit` to track visited flags; only updates provider settings when corresponding CLI flags were explicitly set
- **Verification**: `go test ./internal/config -run TestLoadUsesMultiProviderConfigAndSwitchesProvider` passes

---

## Acceptance Criteria Checklist

### Required Surfaces (All Assigned and Verified)

- [x] `run` command - Test: `TestRunCommandJSONCreatesPersistentSession` ✅ PASS
- [x] `resume` command - Test: `TestResumeAndInspectOutput` ✅ PASS
- [x] `inspect` command - Test: `TestResumeAndInspectOutput` ✅ PASS
- [x] `coding` task routing - Test: `TestTaskAwareVerifierUsesCommandVerifierForCodingTasks` ✅ PASS
- [x] `research` task routing - Test: `TestTaskAwareVerifierUsesEvidenceVerifierForResearchTasks` ✅ PASS
- [x] `file_workflow` task routing - Test: `TestTaskAwareVerifierUsesStateOutputVerifierForFileWorkflowTasks` ✅ PASS
- [x] Verifier dispatch (`off`/`standard`/`strict`) - Test: `TestNewVerifierFromConfigRespectsVerifyMode` ✅ PASS
- [x] Verification rejection handling - Test: `TestRuntimeReplayVerificationFailureFixture` ✅ PASS
- [x] Unsafe tool rejection - Test: `TestRuntimeReplayUnsafeToolRejectionFixture` ✅ PASS
- [x] Config multi-provider support - Test: `TestLoadUsesMultiProviderConfigAndSwitchesProvider` ✅ PASS

### Required Paths (All Have Evidence)

- [x] Happy path: Success fixture, Research fixture, File workflow fixture ✅ PASS
- [x] Failure path: Verification rejection, Unsafe tool rejection ✅ PASS
- [x] Recovery path: Resume after interrupt ✅ PASS

---

## Evidence File Map

Phase 4 evidence files created:

```
.sisyphus/evidence/
├── task-7-build.txt           # go build ./... result
├── task-7-test-full.txt       # go test ./... -v result
├── task-7-test-race.txt       # go test -race ./... result
├── task-7-test-cover.txt      # go test -cover ./... result
├── task-7-cli-lifecycle.txt   # CLI continuity tests
├── task-7-replay.txt          # Runtime replay tests
├── task-7-verifier.txt        # Verifier dispatch tests
└── task-7-config.txt          # Config multi-provider tests
```

---

## Final Acceptance Commands

All of the following commands pass:

```bash
go build ./...                   # Build verification
go test ./...                    # Full test suite
go test -race ./...              # Race condition check
go test -cover ./...             # Coverage check
go test ./cmd/agent/... -run TestRunCommandInterruptPersistsInterruptedSession  # Interrupt fix
go test ./internal/config/... -run TestLoadUsesMultiProviderConfigAndSwitchesProvider  # Config fix
go test ./internal/runtime/... -run TestRuntimeReplay  # All replay fixtures
go test ./internal/verify/...    # Verifier dispatch
```

---

**Notes**:
- This matrix reflects the validated Phase 4 state after all blocker fixes.
- All tests pass; evidence files exist.
- The harness now demonstrates reliable CLI continuity, task-type routing, and task-aware verification.