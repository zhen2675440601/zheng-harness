# Phase 4 Closed-Loop Validation Matrix

**Purpose**: This document maps every required proof surface to concrete tests, replay fixtures, CLI commands, expected outcomes, and evidence targets. All validation must be agent-executable with zero human judgment.

**Last Updated**: 2026-05-06  
**Phase**: 4 - Closed-Loop Validation  
**Status**: ✅ Validated - All blockers resolved  
**v4 Status**: ✅ Validated - API server, SSE, concurrency, providers complete

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
| **multi-provider config loads and switches** | `go test ./internal/config -run TestLoadUsesMultiProviderConfigAndSwitchesProvider` | N/A | Provider selection succeeds, model matches config | `internal/config/config_test.go` | ✅ PASS |
| **provider flag validates existence** | `go test ./cmd/agent -run TestRunCLIRejectsMissingSelectedProvider` | N/A | Error if --provider references undefined provider | `cmd/agent/main_test.go` | ✅ PASS |
| **env overrides work** | `go test ./internal/config -run TestValidConfigAndProviderBoundary` | N/A | Environment variables override config file | `internal/config/config_test.go` | ✅ PASS |

**Configuration Coverage Summary**:
- Happy path: ✅ Multi-provider config loading tested and passes
- Failure path: ✅ Missing provider rejected
- Recovery path: ✅ Provider switching returns correct model

---

## v2 Feature Validation (Wave 2)

### 7. Streaming Runtime (Token Deltas, Tool Events, Step/Session Completion)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **streaming emits token deltas** | `go test ./internal/runtime -run TestRuntimeStreamEmitsTokenDeltas` | N/A | TokenDelta events emitted for each chunk | `internal/runtime/streaming_test.go` | ✅ PASS |
| **streaming emits tool start/end events** | `go test ./internal/runtime -run TestRuntimeStreamEmitsToolLifecycleEvents` | N/A | ToolStart and ToolEnd events with correct payloads | `internal/runtime/streaming_test.go` | ✅ PASS |
| **streaming emits step complete events** | `go test ./internal/runtime -run TestRuntimeStreamEmitsStepCompleteEvents` | N/A | StepComplete event after each step with summary | `internal/runtime/streaming_test.go` | ✅ PASS |
| **streaming emits session complete event** | `go test ./internal/runtime -run TestRuntimeStreamEmitsSessionCompleteEvent` | N/A | SessionComplete event with final status | `internal/runtime/streaming_test.go` | ✅ PASS |
| **event ordering within step is preserved** | `go test ./internal/domain -run TestStreamingEventOrdering` | N/A | Events ordered: TokenDelta* → ToolStart → ToolEnd → StepComplete | `internal/domain/events_test.go` | ✅ PASS |
| **event ordering across steps is preserved** | `go test ./internal/runtime -run TestRuntimeStreamStepOrdering` | N/A | Step N completes before step N+1 starts | `internal/runtime/streaming_test.go` | ✅ PASS |
| **non-streaming providers wrap to streaming facade** | `go test ./internal/llm -run TestNonStreamingProviderWrapsToStreaming` | N/A | Generate() output wrapped to TokenDelta + SessionComplete | `internal/llm/streaming.go` | ✅ PASS |
| **streaming integration test full flow** | `go test ./internal/orchestration -run TestIntegrationFullFlow` | N/A | All event types present, tool sequence correct, session persisted | `internal/orchestration/integration_test.go` | ✅ PASS |

**Streaming Coverage Summary**:
- Happy path: ✅ Token deltas, tool lifecycle, step/session completion all covered
- Failure path: ✅ Error events emitted on failures
- Recovery path: ✅ Non-streaming providers auto-wrapped to streaming facade

**New in v2**: Streaming architecture with callback-based `EventChannel` (~[ADR-006](docs/ADR-006-streaming-architecture.md)). Events NOT persisted; only final session/plan/step state stored for `resume`/`inspect` continuity.

---

### 8. New Tools (web_fetch, ask_user, code_search)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **web_fetch executes HTTP GET with domain validation** | `go test ./internal/tools/adapters -run TestWebAdapterFetchWithAllowedDomains` | N/A | Fetch succeeds for allowed domains, rejected for others | `internal/tools/adapters/web_test.go` | ✅ PASS |
| **web_fetch validates URL scheme (HTTP/HTTPS only)** | `go test ./internal/tools/adapters -run TestWebFetchURLValidation` | N/A | Non-HTTP(S) URLs rejected | `internal/tools/adapters/web_test.go` | ✅ PASS |
| **web_fetch truncates output to max_length** | `go test ./internal/tools/adapters -run TestWebFetchTruncatesToMaxLength` | N/A | Output truncated to configured limit | `internal/tools/adapters/web_test.go` | ✅ PASS |
| **ask_user prompts with question and options** | `go test ./internal/tools/adapters -run TestInteractiveAdapterAskUserWithOptions` | N/A | Options displayed, selection validated | `internal/tools/adapters/interactive_test.go` | ✅ PASS |
| **ask_user retries on invalid option selection** | `go test ./internal/tools/adapters -run TestInteractiveAdapterRetriesOnInvalidOption` | N/A | Up to 3 attempts before failure | `internal/tools/adapters/interactive_test.go` | ✅ PASS |
| **ask_user times out on context cancellation** | `go test ./internal/tools/adapters -run TestInteractiveAdapterTimeoutOnContextCancellation` | N/A | Returns timeout error if user doesn't respond | `internal/tools/adapters/interactive_test.go` | ✅ PASS |
| **code_search executes regex against source files** | `go test ./internal/tools/adapters -run TestCodeSearchAdapterExecutesRegexSearch` | N/A | Matches found with context lines | `internal/tools/adapters/codesearch_test.go` | ✅ PASS |
| **code_search supports language filtering** | `go test ./internal/tools/adapters -run TestCodeSearchLanguageFiltering` | N/A | Only files with matching extensions searched | `internal/tools/adapters/codesearch_test.go` | ✅ PASS |
| **code_search output modes (files_with_matches, content, count)** | `go test ./internal/tools/adapters -run TestCodeSearchOutputModes` | N/A | Correct output format per mode | `internal/tools/adapters/codesearch_test.go` | ✅ PASS |
| **code_search respects max_results limit** | `go test ./internal/tools/adapters -run TestCodeSearchRespectsMaxResults` | N/A | Stops after max results reached | `internal/tools/adapters/codesearch_test.go` | ✅ PASS |
| **new tools integration test** | `go test ./internal/orchestration -run TestIntegrationFullFlow` | N/A | web_fetch, code_search, echo plugin all invoked in sequence | `internal/orchestration/integration_test.go` | ✅ PASS |

**New Tools Coverage Summary**:
- Happy path: ✅ All three new tools tested with valid inputs
- Failure path: ✅ Invalid URLs, timeout, language mismatches all handled
- Recovery path: ✅ ask_user retries, code_search truncates gracefully

**New in v2**: Three new built-in tools added:
- `web_fetch`: HTTP/HTTPS fetching with domain allowlist safety
- `ask_user`: Interactive CLI prompts with option validation
- `code_search`: Language-aware regex search with multiple output modes

---

### 9. Plugin System (External Process, Native Loader, Safety Policy)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **plugin manager discovers external and native plugins** | `go test ./internal/plugin -run TestPluginManagerDiscovery` | N/A | External process (.exe/.bin) and native (.so) plugins discovered | `internal/plugin/manager_test.go` | ✅ PASS |
| **plugin manager loads external process plugin** | `go test ./internal/plugin -run TestPluginManagerLoadExternal` | N/A | JSON-RPC 2.0 stdio communication established | `internal/plugin/manager_test.go` | ✅ PASS |
| **plugin manager loads native Go plugin** | `go test ./internal/plugin -run TestPluginManagerLoadNative` | N/A | .so file loaded via Go plugin package | `internal/plugin/manager_test.go` | ✅ PASS |
| **plugin manager validates contract version** | `go test ./internal/plugin -run TestPluginManagerVersionValidation` | N/A | Contract version mismatch rejected with error | `internal/plugin/manager_test.go` | ✅ PASS |
| **plugin manager CloseAll closes all loaded plugins** | `go test ./internal/plugin -run TestPluginManagerCloseAll` | N/A | All plugins closed, registry cleared | `internal/plugin/manager_test.go` | ✅ PASS |
| **plugin tools registered in tool registry** | `go test ./internal/orchestration -run TestIntegrationFullFlow` (plugin sub-test) | N/A | Plugin tool "echo" invoked alongside built-in tools | `internal/orchestration/integration_test.go` | ✅ PASS |
| **plugin safety policy enforced** | `go test ./internal/config -run TestPluginSecurityPolicyValidation` | N/A | Plugin paths and tool names validated against policy | `internal/config/config_test.go` | ✅ PASS |
| **plugin disabled on Windows for native mode** | Build tag verification | N/A | Native plugin loading disabled via `//go:build !windows` | `internal/plugin/loader_windows.go` (stub) | ✅ PASS |

**Plugin System Coverage Summary**:
- Happy path: ✅ External process and native plugins discovered, loaded, executed
- Failure path: ✅ Contract version mismatch rejected, CloseAll cleanup proven
- Recovery path: ✅ Plugins gracefully closed on shutdown

**New in v2**: Dual-mode plugin system (~[ADR-007](docs/ADR-007-plugin-system.md)):
- **External Process Mode**: JSON-RPC 2.0 over stdio, cross-platform
- **Native Go Plugin Mode**: .so files loaded at runtime (Linux/macOS only)
- **Security Model**: Trusted local extensions, no sandboxing in v2

---

### 10. Multi-Agent Orchestration (Orchestrator, Worker, DAG Scheduling, Result Aggregation)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **orchestrator starts and accepts decompositions** | `go test ./internal/orchestration -run TestOrchestratorStartAndSubmit` | N/A | TaskChannel accepts decomposition, worker loop starts | `internal/orchestration/orchestrator_test.go` | ✅ PASS |
| **orchestrator respects max workers semaphore** | `go test ./internal/orchestration -run TestOrchestratorMaxWorkersSemaphore` | N/A | No more than MaxWorkers concurrent | `internal/orchestration/orchestrator_test.go` | ✅ PASS |
| **worker executes plan/execute/verify lifecycle** | `go test ./internal/orchestration -run TestWorkerLifecycle` | N/A | Plan → Execute → Verify sequence completed | `internal/orchestration/worker_test.go` | ✅ PASS |
| **DAG scheduling respects dependencies** | `go test ./internal/orchestration -run TestDAGSchedulingRespectsDependencies` | N/A | Subtask waits for dependencies before launch | `internal/orchestration/scheduler_test.go` | ✅ PASS |
| **parallel edges in DAG are concurrent** | `go test ./internal/orchestration -run TestParallelEdgesExecuteConcurrently` | N/A | DependencyTypeParallelWith subtasks run in parallel | `internal/orchestration/scheduler_test.go` | ✅ PASS |
| **result aggregation collects worker outputs** | `go test ./internal/orchestration -run TestResultAggregation` | N/A | All worker results received via ResultChannel | `internal/orchestration/aggregation_test.go` | ✅ PASS |
| **aggregator supports all_succeed strategy** | `go test ./internal/orchestration -run TestAggregatorAllSucceedStrategy` | N/A | Aggregation succeeds only if all workers pass | `internal/orchestration/aggregation_test.go` | ✅ PASS |
| **multi-agent integration test with plugins** | `go test ./internal/orchestration -run TestIntegrationMultiAgentWithPlugins` | N/A | Two workers execute with DAG, results aggregated, plugins used | `internal/orchestration/integration_test.go` | ✅ PASS |
| **orchestrator cancellation propagates to workers** | `go test ./internal/orchestration -run TestOrchestratorCancelPropagatesToWorkers` | N/A | Workers terminated on orchestrator.Cancel() | `internal/orchestration/orchestrator_test.go` | ✅ PASS |

**Multi-Agent Coverage Summary**:
- Happy path: ✅ Orchestrator, workers, DAG scheduling, aggregation all tested
- Failure path: ✅ Worker errors reported, verification failures handled
- Recovery path: ✅ Context cancellation propagates, workers terminate gracefully

**New in v2**: Bounded concurrent multi-agent execution with:
- **Orchestrator**: Coordinates worker lifecycle, manages semaphore-bounded concurrency
- **Worker**: Executes plan-execute-verify loop for one subtask
- **DAG Scheduling**: Dependency-aware launch with parallel edge support
- **Result Aggregation**: Collects and merges worker outputs with configurable strategy

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

## v2 Acceptance Criteria Checklist

### v2 Streaming Features (New in v2)

- [x] Token delta streaming - Test: `TestRuntimeStreamEmitsTokenDeltas` ✅ PASS
- [x] Tool lifecycle events - Test: `TestRuntimeStreamEmitsToolLifecycleEvents` ✅ PASS
- [x] Step completion events - Test: `TestRuntimeStreamEmitsStepCompleteEvents` ✅ PASS
- [x] Session completion event - Test: `TestRuntimeStreamEmitsSessionCompleteEvent` ✅ PASS
- [x] Event ordering preserved - Test: `TestStreamingEventOrdering` ✅ PASS
- [x] Non-streaming provider facade - Test: `TestNonStreamingProviderWrapsToStreaming` ✅ PASS
- [x] Streaming integration flow - Test: `TestIntegrationFullFlow` ✅ PASS

### v2 New Tools (New in v2)

- [x] web_fetch with domain validation - Test: `TestWebAdapterFetchWithAllowedDomains` ✅ PASS
- [x] web_fetch URL scheme validation - Test: `TestWebFetchURLValidation` ✅ PASS
- [x] ask_user with options - Test: `TestInteractiveAdapterAskUserWithOptions` ✅ PASS
- [x] ask_user retry logic - Test: `TestInteractiveAdapterRetriesOnInvalidOption` ✅ PASS
- [x] code_search with language filtering - Test: `TestCodeSearchAdapterExecutesRegexSearch` ✅ PASS
- [x] code_search output modes - Test: `TestCodeSearchOutputModes` ✅ PASS

### v2 Plugin System (New in v2)

- [x] Plugin discovery (external + native) - Test: `TestPluginManagerDiscovery` ✅ PASS
- [x] External process loading - Test: `TestPluginManagerLoadExternal` ✅ PASS
- [x] Native Go plugin loading - Test: `TestPluginManagerLoadNative` ✅ PASS
- [x] Contract version validation - Test: `TestPluginManagerVersionValidation` ✅ PASS
- [x] Plugin cleanup on shutdown - Test: `TestPluginManagerCloseAll` ✅ PASS
- [x] Plugin integration with tools - Test: `TestIntegrationFullFlow` (echo plugin) ✅ PASS

### v2 Multi-Agent Orchestration (New in v2)

- [x] Orchestrator lifecycle - Test: `TestOrchestratorStartAndSubmit` ✅ PASS
- [x] Worker semaphore bounded concurrency - Test: `TestOrchestratorMaxWorkersSemaphore` ✅ PASS
- [x] Worker lifecycle (plan/execute/verify) - Test: `TestWorkerLifecycle` ✅ PASS
- [x] DAG dependency scheduling - Test: `TestDAGSchedulingRespectsDependencies` ✅ PASS
- [x] Parallel edge execution - Test: `TestParallelEdgesExecuteConcurrently` ✅ PASS
- [x] Result aggregation - Test: `TestResultAggregation` ✅ PASS
- [x] Multi-agent integration with plugins - Test: `TestIntegrationMultiAgentWithPlugins` ✅ PASS

### v2 Required Paths

- [x] Happy path: Streaming events, plugin execution, multi-agent coordination ✅ PASS
- [x] Failure path: Plugin version mismatch, DAG dependency failures ✅ PASS
- [x] Recovery path: Orchestrator cancellation, worker cleanup ✅ PASS

---

## Evidence File Map

Phase 4 evidence files created:

```
.sisyphus/evidence/
├── task-7-build.txt           # go build ./... result
├── task-7-test-full.txt       # go test ./... result
├── task-7-test-race.txt       # go test -race ./... result
├── task-7-test-cover.txt      # go test -cover ./... result
├── task-7-cli-lifecycle.txt   # CLI continuity tests
├── task-7-replay.txt          # Runtime replay tests
├── task-7-verifier.txt        # Verifier dispatch tests
└── task-7-config.txt          # Config multi-provider tests
```

Wave 2 (v2) evidence files:

```
.sisyphus/evidence/
├── task-28-streaming.txt      # Streaming event tests (token deltas, tool events, step/session completion)
├── task-28-tools.txt          # New tools tests (web_fetch, ask_user, code_search)
├── task-28-plugins.txt        # Plugin system tests (discovery, external/native loading, version validation)
├── task-28-orchestration.txt  # Multi-agent orchestration tests (orchestrator, worker, DAG, aggregation)
└── task-28-integration.txt    # Full integration tests (streaming + tools + plugins + multi-agent)
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

### v2 (Wave 2) Acceptance Commands

```bash
go test ./internal/runtime/... -run TestRuntimeStream  # Streaming events
go test ./internal/tools/adapters/...                  # New tools (web_fetch, ask_user, code_search)
go test ./internal/plugin/...                          # Plugin system
go test ./internal/orchestration/... -run TestOrchestrator  # Multi-agent orchestration
go test ./internal/orchestration/... -run TestIntegration  # Full integration (streaming + tools + plugins + multi-agent)
go test -race ./internal/orchestration/...             # Multi-agent race detection
```

---

**Notes**:
- This matrix reflects the validated Phase 4 state after all blocker fixes.
- All tests pass; evidence files exist.
- The harness now demonstrates reliable CLI continuity, task-type routing, and task-aware verification.
- **v2 (Wave 2) additions**: Streaming runtime, new tools (web_fetch, ask_user, code_search), dual-mode plugin system, and multi-agent orchestration with DAG scheduling. All v2 features tested and integrated.
- **Test Coverage**: v2 features covered in `internal/runtime/streaming_test.go`, `internal/tools/adapters/*_test.go`, `internal/plugin/manager_test.go`, and `internal/orchestration/integration_test.go`.

---

## v3 Extensibility Validation (Wave 3)

### 11. Provider Plugin Family

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **provider registry resolves built-in providers** | `go test ./internal/llm -run TestBuiltinProviderRegistry` | N/A | Built-in providers (openai, anthropic, dashscope) resolved | `internal/llm/provider_registry_test.go` | ✅ PASS |
| **provider plugin adapter wraps built-in provider** | `go test ./internal/llm -run TestAdaptProviderPlugin` | N/A | Built-in provider wrapped with plugin metadata | `internal/llm/provider.go` | ✅ PASS |
| **provider plugin contract validates metadata** | `go test ./internal/llm -run TestProviderPluginMetadata` | N/A | Metadata includes family, logical ID, contract version | `internal/llm/provider.go` | ✅ PASS |
| **built-in provider path still routes through registry seam** | `go test ./internal/llm/... -run TestProviderPluginBuiltinsUseRegistry` | N/A | Built-in provider selection remains compatible while using registry seam | `internal/llm/provider_registry_test.go` | ✅ PASS |
| **unknown provider selection fails deterministically** | `go test ./internal/llm/... -run TestProviderPluginUnknownProviderFailsDeterministically` | N/A | Unknown provider rejected with stable unsupported-provider error | `internal/llm/provider_registry_test.go` | ✅ PASS |

### 12. Verifier Plugin Family

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **verifier registry resolves built-in verifiers** | `go test ./internal/verify -run TestVerifierRegistryBuiltinsAvailableWithoutPlugins` | N/A | Built-in verifiers (command, evidence, state-output) resolved | `internal/verify/registry_test.go` | ✅ PASS |
| **verifier registry rejects unknown policy** | `go test ./internal/verify -run TestVerifierRegistryRejectsUnknownPolicy` | N/A | Unknown policy rejected with error | `internal/verify/registry_test.go` | ✅ PASS |
| **verifier plugin rejected outside predeclared policies** | `go test ./internal/verify -run TestVerifierRegistryRejectsPluginBindingOutsidePredeclaredPolicies` | N/A | Plugin binding outside declared policies rejected | `internal/verify/registry_test.go` | ✅ PASS |
| **verifier plugin timeout fails closed** | `go test ./internal/verify -run TestVerifierPluginTimeoutFailsClosed` | N/A | Timeout results in fail-closed failed status | `internal/verify/registry_test.go` | ✅ PASS |
| **verifier plugin crash fails closed** | `go test ./internal/verify -run TestVerifierPluginCrashFailsClosed` | N/A | Crash results in fail-closed failed status with provenance | `internal/verify/registry_test.go` | ✅ PASS |
| **verifier plugin malformed result detected** | `go test ./internal/verify -run TestVerifierPluginRejectsMalformedResult` | N/A | Malformed result detected and attributed to plugin | `internal/verify/registry_test.go` | ✅ PASS |
| **plugin-backed verifier output remains bounded by host result validation** | `go test ./internal/verify/... -run TestVerifierPluginRejectsMalformedResult` | N/A | Contradictory plugin result converted to fail-closed failed status | `internal/verify/registry_test.go` | ✅ PASS |

### 13. Agent Strategy Family (Built-in Runtime)

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **built-in runtime strategy executes** | `go test ./internal/e2e -run TestE2E_AgentStrategyBuiltIn` | N/A | Built-in strategy resolves through seam | `internal/e2e/e2e_validation_test.go` | ✅ PASS |
| **strategy persistence works** | `go test ./internal/e2e -run TestE2E_ProvenancePersistence` | N/A | Strategy execution provenance persisted | `internal/e2e/e2e_validation_test.go` | ✅ PASS |
| **strategy plugin cancellation propagates from host context** | `go test ./internal/runtime/... -run TestAgentStrategyPluginCancellationPropagation` | N/A | Plugin-backed strategy stops on host cancellation without persisted partial step | `internal/runtime/agent_strategy_test.go` | ✅ PASS |
| **invalid strategy response is rejected deterministically** | `go test ./internal/runtime/... -run TestAgentStrategyRejectsInvalidResponse` | N/A | Recursive/unmanaged strategy action rejected before persistence corruption | `internal/runtime/agent_strategy_test.go` | ✅ PASS |

### 14. Fail-Closed Runtime Behavior

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **plugin load failure rejected** | `go test ./internal/e2e -run TestE2E_FailClosedPluginSelection` | N/A | Missing plugin rejected with deterministic error | `internal/e2e/e2e_validation_test.go` | ✅ PASS |
| **plugin execution timeout fails closed** | `go test ./internal/verify -run TestVerifierPluginTimeoutFailsClosed` | N/A | Timeout attribution recorded with plugin identity | `internal/verify/registry_test.go` | ✅ PASS |
| **plugin crash contained** | `go test ./internal/verify -run TestVerifierPluginCrashFailsClosed` | N/A | Crash does not propagate, attributed to plugin | `internal/verify/registry_test.go` | ✅ PASS |
| **resume fails closed when persisted provider plugin is unavailable** | `go test ./cmd/agent/... -run TestResumeFailsClosedWhenPersistedProviderPluginUnavailable` | N/A | Resume aborts with deterministic provider plugin error and preserved history | `cmd/agent/main_test.go` | ✅ PASS |

### 15. Provenance Persistence

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **session persisted with provenance** | `go test ./internal/e2e -run TestE2E_ProvenancePersistence` | N/A | Session persisted with plugin metadata | `internal/e2e/e2e_validation_test.go` | ✅ PASS |
| **verifier provenance recorded** | `go test ./internal/verify -run TestVerifierPluginRejectsMalformedResult` | N/A | Verifier provenance accessible via `VerifierProvenance()` | `internal/verify/registry_test.go` | ✅ PASS |
| **persisted provenance remains readable without live plugin artifact** | `go test ./cmd/agent/... -run TestInspectDisplaysPersistedPluginProvenanceWithoutLivePlugin` | N/A | Inspect JSON includes persisted provenance with no live plugin load requirement | `cmd/agent/main_test.go` | ✅ PASS |

### 16. Built-in Only Compatibility

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **built-in only config loads** | `go test ./internal/e2e -run TestE2E_BuiltInOnlyCompatibility` | N/A | Config loads with zero plugins | `internal/e2e/e2e_validation_test.go` | ✅ PASS |
| **built-in tools work without plugins** | `go test ./internal/tools -run TestBuiltinTools` | N/A | Built-in tools (read_file, write_file, etc.) functional | `internal/tools/builtin_test.go` | ✅ PASS |
| **built-in providers work without plugins** | `go test ./internal/llm -run TestBuiltinProviders` | N/A | Built-in providers (openai, anthropic, dashscope) functional | `internal/llm/provider_registry_test.go` | ✅ PASS |
| **CLI built-in provider config remains compatible** | `go test ./cmd/agent/... -run TestPluginSelectionBuiltInProviderCompatibility` | N/A | Existing built-in provider config runs unchanged with zero plugins | `cmd/agent/main_test.go` | ✅ PASS |
| **CLI additive plugin provider selection works** | `go test ./cmd/agent/... -run TestPluginSelectionSupportsExplicitPluginProvider` | N/A | Plugin provider can be selected explicitly without renaming existing config keys | `cmd/agent/main_test.go` | ✅ PASS |
| **CLI rejects conflicting provider selection sources** | `go test ./cmd/agent/... -run TestPluginSelectionRejectsConflictingSources` | N/A | Config and CLI conflicts produce deterministic user-facing error | `cmd/agent/main_test.go` | ✅ PASS |

---

## v3 Acceptance Criteria Checklist

### v3 Provider Family (New in v3)

- [x] Provider registry resolves built-in providers - Test: `TestBuiltinProviderRegistry` ✅ PASS
- [x] Provider plugin adapter wraps built-in - Test: `TestAdaptProviderPlugin` ✅ PASS
- [x] Provider plugin metadata recorded - Test: `TestProviderPluginMetadata` ✅ PASS
- [x] Registry seam preserves built-in compatibility - Test: `TestProviderPluginBuiltinsUseRegistry` ✅ PASS
- [x] Unknown provider rejection is deterministic - Test: `TestProviderPluginUnknownProviderFailsDeterministically` ✅ PASS

### v3 Verifier Family (New in v3)

- [x] Verifier registry resolves built-ins - Test: `TestVerifierRegistryBuiltinsAvailableWithoutPlugins` ✅ PASS
- [x] Unknown policy rejected - Test: `TestVerifierRegistryRejectsUnknownPolicy` ✅ PASS
- [x] Plugin binding outside policies rejected - Test: `TestVerifierRegistryRejectsPluginBindingOutsidePredeclaredPolicies` ✅ PASS
- [x] Timeout fails closed - Test: `TestVerifierPluginTimeoutFailsClosed` ✅ PASS
- [x] Crash fails closed - Test: `TestVerifierPluginCrashFailsClosed` ✅ PASS
- [x] Malformed result detected - Test: `TestVerifierPluginRejectsMalformedResult` ✅ PASS

### v3 Agent Strategy Family (New in v3)

- [x] Built-in strategy executes - Test: `TestE2E_AgentStrategyBuiltIn` ✅ PASS
- [x] Strategy persistence works - Test: `TestE2E_ProvenancePersistence` ✅ PASS
- [x] Strategy cancellation propagates safely - Test: `TestAgentStrategyPluginCancellationPropagation` ✅ PASS
- [x] Invalid strategy responses are rejected - Test: `TestAgentStrategyRejectsInvalidResponse` ✅ PASS

### v3 Fail-Closed Semantics (New in v3)

- [x] Plugin load failure rejected - Test: `TestE2E_FailClosedPluginSelection` ✅ PASS
- [x] Plugin timeout fails closed - Test: `TestVerifierPluginTimeoutFailsClosed` ✅ PASS
- [x] Plugin crash contained - Test: `TestVerifierPluginCrashFailsClosed` ✅ PASS
- [x] Resume fails closed on missing provider plugin - Test: `TestResumeFailsClosedWhenPersistedProviderPluginUnavailable` ✅ PASS

### v3 Provenance Persistence (New in v3)

- [x] Session persisted with provenance - Test: `TestE2E_ProvenancePersistence` ✅ PASS
- [x] Verifier provenance recorded - Test: `TestVerifierPluginRejectsMalformedResult` ✅ PASS
- [x] Inspect remains readable without live plugin artifact - Test: `TestInspectDisplaysPersistedPluginProvenanceWithoutLivePlugin` ✅ PASS

### v3 Built-in Compatibility (New in v3)

- [x] Built-in only config loads - Test: `TestE2E_BuiltInOnlyCompatibility` ✅ PASS
- [x] Built-in tools work - Test: `TestBuiltinTools` ✅ PASS
- [x] Built-in providers work - Test: `TestBuiltinProviders` ✅ PASS
- [x] CLI built-in provider config remains compatible - Test: `TestPluginSelectionBuiltInProviderCompatibility` ✅ PASS
- [x] CLI additive plugin provider selection works - Test: `TestPluginSelectionSupportsExplicitPluginProvider` ✅ PASS
- [x] CLI provider-selection conflicts error deterministically - Test: `TestPluginSelectionRejectsConflictingSources` ✅ PASS

---

## v3 Evidence File Map

Wave 3 (v3) evidence files:

```
.sisyphus/evidence/
├── task-14-provider-family.txt     # Provider plugin tests (registry, adapter, metadata)
├── task-14-verifier-family.txt     # Verifier plugin tests (registry, fail-closed, provenance)
├── task-14-agent-strategy.txt      # Agent strategy tests (built-in runtime, persistence)
├── task-14-fail-closed.txt         # Fail-closed behavior tests (timeout, crash, load failure)
├── task-14-provenance.txt          # Provenance persistence tests
└── task-14-builtin-compat.txt      # Built-in compatibility tests
```

---

## v3 Final Acceptance Commands

All of the following commands pass:

```bash
go test ./internal/llm/...              # Provider plugin family
go test ./internal/verify/...           # Verifier plugin family
go test ./internal/e2e/... -run TestE2E # End-to-end extensibility validation
go test ./internal/plugin/...           # Plugin system (v2 + v3)
go test ./internal/orchestration/...    # Multi-agent orchestration (v2 + v3)
```

### v3 Evidence Commands

```bash
go test ./internal/verify/... -run TestVerifierRegistry # Verifier registry
go test ./internal/verify/... -run TestVerifierPlugin   # Verifier plugin fail-closed
go test ./internal/e2e/... -run TestE2E                 # End-to-end validation
```

---

## v4 API Server Validation (Wave 4)

### 17. HTTP API Endpoints

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **POST /api/v1/run creates session** | `go test ./internal/server -run TestServerRunCreatesSession` | N/A | 202 Accepted, session_id returned, session persisted | `internal/server/handlers_test.go` | ✅ PASS |
| **POST /api/v1/resume recovers session** | `go test ./internal/server -run TestServerResumeRecoversSession` | N/A | 202 Accepted, actor dispatched | `internal/server/handlers_test.go` | ✅ PASS |
| **POST /api/v1/resume rejects completed** | `go test ./internal/server -run TestServerResumeRejectsCompleted` | N/A | 409 Conflict, error message explains ineligibility | `internal/server/handlers_test.go` | ✅ PASS |
| **GET /api/v1/sessions/{id}/inspect reads state** | `go test ./internal/server -run TestServerInspectReadsState` | N/A | 200 OK, JSON contains session/plan/step state | `internal/server/handlers_test.go` | ✅ PASS |
| **GET /healthz returns healthy** | `go test ./internal/server -run TestServerHealthz` | N/A | 200 OK, machine-readable status | `internal/server/handlers_test.go` | ✅ PASS |
| **Unauthenticated requests rejected** | `go test ./internal/server -run TestServerAuthRejected` | N/A | 401 Unauthorized, standardized JSON error | `internal/server/handlers_test.go` | ✅ PASS |
| **Invalid JSON returns 400** | `go test ./internal/server -run TestServerInvalidJSONRejected` | N/A | 400 Bad Request, validation error message | `internal/server/handlers_test.go` | ✅ PASS |

### 18. SSE Streaming Endpoint

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **SSE emits ordered events** | `go test ./internal/server -run TestSSEEmitsOrderedEvents` | N/A | Events ordered: token_delta* → tool_start → tool_end → step_complete → session_complete | `internal/server/sse_test.go` | ✅ PASS |
| **SSE requires auth** | `go test ./internal/server -run TestSSERequiresAuth` | N/A | 401 Unauthorized without JWT token | `internal/server/sse_test.go` | ✅ PASS |
| **Client disconnect doesn't cancel session** | `go test ./internal/server -run TestSSEDisconnectDoesntCancelSession` | N/A | Session continues after client disconnects, inspect shows completion | `internal/server/sse_test.go` | ✅ PASS |
| **Slow subscriber overflow is explicit** | `go test ./internal/server -run TestSSESlowSubscriberOverflow` | N/A | Buffer overflow emits error event and closes only that connection | `internal/server/sse_test.go` | ✅ PASS |
| **Heartbeat sent every 15s** | `go test ./internal/server -run TestSSEHeartbeat` | N/A | Heartbeat comments observed in stream | `internal/server/sse_test.go` | ✅ PASS |

### 19. Session Concurrency

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **Session manager enforces limit** | `go test ./internal/server -run TestSessionManagerEnforcesLimit` | N/A | Active sessions capped at configured limit | `internal/server/session_manager_test.go` | ✅ PASS |
| **Duplicate resume returns 409** | `go test ./internal/server -run TestSessionManagerDuplicateResume` | N/A | Running/completed session resume rejected with 409 | `internal/server/session_manager_test.go` | ✅ PASS |
| **Missing session returns 404** | `go test ./internal/server -run TestSessionManagerMissingSession` | N/A | Non-existent session returns 404 | `internal/server/session_manager_test.go` | ✅ PASS |
| **Graceful shutdown drains sessions** | `go test ./internal/server -run TestSessionManagerGracefulShutdown` | N/A | Shutdown waits for active sessions or times out | `internal/server/session_manager_test.go` | ✅ PASS |
| **Multi-session concurrency** | `go test ./internal/server -run TestSessionManagerMultiSession` | N/A | Multiple sessions run concurrently without interference | `internal/server/session_manager_test.go` | ✅ PASS |

### 20. SQLite WAL Mode

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **WAL mode enabled in server path** | `go test ./internal/store -run TestSQLiteWALModeEnabled` | N/A | SQLite database opened in WAL mode | `internal/store/wal_test.go` | ✅ PASS |
| **Concurrent writes don't corrupt inspect** | `go test ./internal/store -run TestSQLiteConcurrentWritesInspectReadable` | N/A | Inspect reads consistent state during concurrent writes | `internal/store/wal_test.go` | ✅ PASS |
| **Resume eligibility persisted** | `go test ./internal/store -run TestSQLiteResumeEligibilityPersisted` | N/A | Completed/running sessions not incorrectly resumable | `internal/store/wal_test.go` | ✅ PASS |

### 21. OpenAI/Anthropic Providers

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **OpenAI Generate works** | `go test ./internal/llm -run TestOpenAIProviderGenerate` | N/A | Real HTTP generate request succeeds | `internal/llm/openai_test.go` | ✅ PASS |
| **OpenAI Stream works** | `go test ./internal/llm -run TestOpenAIProviderStream` | N/A | Streaming emits token deltas | `internal/llm/openai_test.go` | ✅ PASS |
| **Anthropic Generate works** | `go test ./internal/llm -run TestAnthropicProviderGenerate` | N/A | Real HTTP generate request succeeds | `internal/llm/anthropic_test.go` | ✅ PASS |
| **Anthropic Stream works** | `go test ./internal/llm -run TestAnthropicProviderStream` | N/A | Streaming emits token deltas | `internal/llm/anthropic_test.go` | ✅ PASS |
| **Provider errors normalized** | `go test ./internal/llm -run TestProviderErrorNormalization` | N/A | Auth failure, transport failure, malformed response all normalized | `internal/llm/provider_error_test.go` | ✅ PASS |

### 22. CLI Non-Regression

| Proof Surface | Command/Test | Fixture | Expected Outcome | Evidence Target | Status |
|--------------|--------------|---------|------------------|-----------------|--------|
| **CLI unaffected by server additions** | `go test ./cmd/agent -run TestCLIUnaffectedByServer` | N/A | CLI run/resume/inspect behavior unchanged | `cmd/agent/main_test.go` | ✅ PASS |

---

## v4 Acceptance Criteria Checklist

### v4 API Endpoints (New in v4)

- [x] POST /api/v1/run creates session - Test: `TestServerRunCreatesSession` ✅ PASS
- [x] POST /api/v1/resume recovers session - Test: `TestServerResumeRecoversSession` ✅ PASS
- [x] POST /api/v1/resume rejects completed - Test: `TestServerResumeRejectsCompleted` ✅ PASS
- [x] GET /api/v1/sessions/{id}/inspect reads state - Test: `TestServerInspectReadsState` ✅ PASS
- [x] GET /api/v1/sessions/{id}/stream SSE streams - Test: `TestSSEEmitsOrderedEvents` ✅ PASS
- [x] GET /healthz returns healthy - Test: `TestServerHealthz` ✅ PASS
- [x] Unauthenticated requests rejected - Test: `TestServerAuthRejected` ✅ PASS

### v4 SSE Streaming (New in v4)

- [x] SSE emits ordered events - Test: `TestSSEEmitsOrderedEvents` ✅ PASS
- [x] SSE requires auth - Test: `TestSSERequiresAuth` ✅ PASS
- [x] Client disconnect doesn't cancel session - Test: `TestSSEDisconnectDoesntCancelSession` ✅ PASS
- [x] Slow subscriber overflow explicit - Test: `TestSSESlowSubscriberOverflow` ✅ PASS
- [x] Heartbeat sent every 15s - Test: `TestSSEHeartbeat` ✅ PASS

### v4 Session Concurrency (New in v4)

- [x] Session manager enforces limit - Test: `TestSessionManagerEnforcesLimit` ✅ PASS
- [x] Duplicate resume returns 409 - Test: `TestSessionManagerDuplicateResume` ✅ PASS
- [x] Missing session returns 404 - Test: `TestSessionManagerMissingSession` ✅ PASS
- [x] Graceful shutdown drains sessions - Test: `TestSessionManagerGracefulShutdown` ✅ PASS
- [x] Multi-session concurrency works - Test: `TestSessionManagerMultiSession` ✅ PASS

### v4 SQLite WAL (New in v4)

- [x] WAL mode enabled - Test: `TestSQLiteWALModeEnabled` ✅ PASS
- [x] Concurrent writes don't corrupt inspect - Test: `TestSQLiteConcurrentWritesInspectReadable` ✅ PASS
- [x] Resume eligibility persisted - Test: `TestSQLiteResumeEligibilityPersisted` ✅ PASS

### v4 Provider Completion (New in v4)

- [x] OpenAI Generate works - Test: `TestOpenAIProviderGenerate` ✅ PASS
- [x] OpenAI Stream works - Test: `TestOpenAIProviderStream` ✅ PASS
- [x] Anthropic Generate works - Test: `TestAnthropicProviderGenerate` ✅ PASS
- [x] Anthropic Stream works - Test: `TestAnthropicProviderStream` ✅ PASS
- [x] Provider errors normalized - Test: `TestProviderErrorNormalization` ✅ PASS

### v4 CLI Non-Regression (New in v4)

- [x] CLI unaffected by server additions - Test: `TestCLIUnaffectedByServer` ✅ PASS

---

## v4 Evidence File Map

Wave 4 (v4) evidence files:

```
.sisyphus/evidence/
├── task-5-api-endpoints.txt       # API endpoint tests (run, resume, inspect, healthz)
├── task-5-sse-streaming.txt       # SSE streaming tests (ordered events, auth, disconnect)
├── task-5-session-concurrency.txt # Session concurrency tests (limit, duplicate, shutdown)
├── task-5-sqlite-wal.txt          # SQLite WAL tests (concurrent writes, resume eligibility)
├── task-8-openai-provider.txt     # OpenAI provider tests (generate, stream, errors)
├── task-9-anthropic-provider.txt  # Anthropic provider tests (generate, stream, errors)
└── task-10-cli-non-regression.txt # CLI non-regression tests
```

---

## v4 Final Acceptance Commands

All of the following commands pass:

```bash
go build ./...
go test ./...
go test ./cmd/server/...
go test ./internal/server/...
go test ./internal/runtime/... -run TestSessionManager
go test ./internal/llm/... -run "Test(OpenAI|Anthropic)"
go test ./cmd/agent/... -run TestCLIUnaffectedByServer

# Manual verification
curl -sS http://127.0.0.1:8080/healthz
curl -sS -X POST http://127.0.0.1:8080/api/v1/run \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  --data '{"task":"ping","task_type":"general"}'
curl -N http://127.0.0.1:8080/api/v1/sessions/test-session/stream \
  -H "Authorization: Bearer test-token"
```
