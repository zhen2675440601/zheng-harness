# Learnings

## 2026-04-26 Phase 2 Planning Session
- Runtime loop is MATURE - plan-execute-verify cycle complete with budgets/retry/timeout
- ToolCall.Input is single string - keep it, use convention format for structured params
- domain.MemoryStore has Recall() in store implementation but NOT in ports.go interface - must add
- exec_command uses strings.Fields which breaks quoted args - hidden dependency for verifier
- FakeModel/FakeVerifier are default CLI factories - must update when interfaces change
- DashScope adapter is the only real HTTP provider (195 lines) - use as reference for OpenAI/Anthropic
- Prompt builder (config/prompts/model_adapter.go) creates JSON payloads - no tool defs included yet
- Memory Remember is called but Recall is never invoked - memory written but never read
- grep_search uses strings.Contains only - no regex/line numbers/output modes
- Safety allowlist only: go, git, pwd, ls, dir

## 2026-04-26 Phase 2 Complete

### Implementation Summary
All 10 tasks implemented:
- T1: ToolInfo + Model interface update (tools/memory params)
- T2: Prompt injection (tools/memory in JSON payload)
- T3: Memory recall integration (Recall before CreatePlan/NextAction)
- T4: Shell quoting fix (shellquote.Split + chaining rejection)
- T5: Allowlist expansion + configurable + rm hard-block
- T6: CommandVerifier (executes go test/build/vet)
- T7: OpenAI HTTP adapter (real POST + retry + auth handling)
- T8: Glob tool (doublestar + workspace boundary)
- T9: Anthropic HTTP adapter (Anthropic-specific headers + content[0].text)
- T10: Regex grep + output modes + coverage deepening

### Test Coverage
- cmd/agent: 68.3%
- internal/config: 78.8%
- internal/config/prompts: 86.8%
- internal/domain: 100%
- internal/llm: 51.9%
- internal/memory: 100%
- internal/runtime: 54.4%
- internal/store: 73.1%
- internal/tools: 81.0%
- internal/tools/adapters: 52.2%
- internal/verify: 76.0%

### Key Patterns
- Provider fallback: When API key empty, use FakeModel (graceful degradation)
- Memory recall: Fan-out to 4 queries (session+fact, session+summary, project+fact, project+summary)
- Verifier: Parses exec_command structured output (COMMAND/EXIT_CODE/OUTPUT_BEGIN)
- grep convention: Input lines for pattern/flags/mode/include glob
- CLI flag handling: Filter config flags before loading, preserve FakeModel fallback

## 2026-04-26 Task 1 Interface Foundation
- Introduced `domain.ToolInfo` as the prompt-facing value object (name/description/schema only) to avoid leaking runtime handler concerns into model contracts.
- Added domain aliases `MemoryEntry` and `RecallQuery` so domain interfaces can expose memory contracts without importing infrastructure packages directly.
- Runtime now forwards tool metadata via registry conversion (`Registry.ListToolInfo`) before model plan/action calls; memory parameter currently threads through as empty slice for later recall integration task.

## 2026-04-26 Task 2 Prompt Injection
- Prompt payload builders now support selective context injection: `BuildNextActionInput` includes `tools` + `memory`, and `BuildCreatePlanInput` includes `memory` only when entries exist.
- Tool schema text can bloat prompts quickly; truncating schema strings to a bounded length preserves utility while controlling token budget.
- Runtime model adapter wrappers must forward tools/memory explicitly; placeholder `_ = memory/_ = tools` silently drops context even when interface wiring is correct.

## 2026-04-26 Task 8 Glob Tool
- `doublestar/v2` supports recursive `**` patterns, but workspace safety still needs an explicit pre-check because joined absolute patterns could otherwise point outside the workspace.
- Returning glob results as workspace-relative slash-separated file paths keeps tool output stable across platforms and consistent with prompt/tool expectations.
- Filtering out directories after glob expansion ensures `**/*` behaves like a file finder rather than mixing files and directory entries.

## 2026-04-26 Task 3 Memory Recall in Runtime
- Runtime now recalls memory before both `CreatePlan` and `NextAction`, instead of always passing nil memory to model calls.
- Current `RecallQuery` supports single `Scope`/`Type`; to satisfy session+project and fact+summary retrieval, runtime fans out into four recall queries and merges results with deduplication.
- Recall failures are treated as non-fatal: runtime logs a warning and degrades to empty memory so sessions continue.

## 2026-04-26 Task OpenAI HTTP Adapter
- OpenAI-compatible adapters should treat `baseURL` as already containing `/v1` and append only `/chat/completions` for request routing.
- Retry policy aligned to provider behavior is sufficient with bounded retries on `429` and `5xx` plus exponential backoff (`1s`, `2s`).
- Explicit `401` authentication errors are critical for operator debugging; generic non-2xx errors should still surface API envelope messages.

## 2026-04-26 Task Anthropic HTTP Adapter
- Anthropic Messages API request shape differs from OpenAI-compatible chat APIs in two key ways: `system` is top-level and user input is in `messages[{role:"user", content:"..."}]`.
- Retry handling needs Anthropic-specific status handling (`429`, `529`, `5xx`) with bounded exponential backoff; `529` should emit explicit overload/retry context.
- Parsing only `content[0].text` (when `type == "text"`) aligns the adapter output contract to Anthropic response semantics expected by this task.

## 2026-04-26 Task Verifier Command Execution
- Verifier can be split by responsibility: keep policy/heuristics (`verify.Verifier`) and add an execution-backed verifier (`verify.CommandVerifier`) implementing the same `domain.Verifier` interface.
- Parsing `exec_command` structured output (`COMMAND`/`EXIT_CODE`/`OUTPUT_BEGIN`) is sufficient to determine pass/fail without touching tool adapters.
- CLI wiring should share one runtime ToolExecutor instance between Engine tools and verifier to ensure verification uses real allowlist/safety behavior.

## 2026-04-26 Phase 2 Manual QA Evidence
- `internal/tools/tools_test.go::TestGlobRecursive` is the direct evidence that the `glob` tool discovers recursive `**/*.go` matches and returns workspace-relative paths.
- `internal/runtime/runtime_test.go::TestMemoryRecallInLoop` proves recalled memory is passed back into subsequent model calls after a failed verification cycle.
- `cmd/agent/cli.go::newVerifierFromConfig` routes `standard`/`strict` verify modes to `verify.NewCommandVerifier`, and `internal/verify/command_verifier.go` executes `go test ./...`, `go build ./...`, and `go vet ./...` through `exec_command`.

## 2026-04-26 Scope Fidelity Audit Learnings
- Phase-boundary audits should separate product scope files from operational governance changes; CI/notepad governance additions can be useful but still count as scope creep if not listed in INCLUDE.
- Explicit Must NOT Have checks were successfully enforced for Phase 2: no native tool calling path, no plugin/multi-agent/web/vector expansions, and no ToolCall.Input breaking change.
