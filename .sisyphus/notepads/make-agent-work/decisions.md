# Decisions

## 2026-04-26 Phase 2 Planning
- Tool schema injection: Path A (text-in-prompt) first, Path B (native tool calling) deferred
- Verifier strategy: actively execute verification commands via exec_command
- ToolCall.Input stays string - structured params via convention format
- rm hard-blocked in all variants (agent uses git clean instead)
- OpenAI + Anthropic both implemented as real adapters
- Dependency order: tools/memory interfaces → prompt injection → memory recall → exec fix → allowlist → verifier → adapters → glob → grep+coverage

## 2026-04-26 Task 1 Implementation Decisions
- Added `MemoryStore.Recall` to `domain.MemoryStore` with signature `Recall(ctx, query RecallQuery) ([]MemoryEntry, error)` to match SQLite memory store capability and unblock runtime recall work.
- Kept prompt-builder package unchanged per scope; `ModelAdapter` now accepts tools/memory in method signatures and forwards through thin wrappers for compatibility.
- Engine retrieves tools through executor registry (`ListToolInfo`) via runtime-local interface assertion, preserving existing `domain.ToolExecutor` boundary and avoiding broad interface churn.

## 2026-04-26 Task 2 Implementation Decisions
- Memory serialization in prompts uses `{scope,type,content,confidence,source}` mapped from memory entry fields, with `content` sourced from `Entry.Value` to keep prompt payload aligned with expected model-facing schema.
- Empty memory is omitted from prompt payloads to avoid unnecessary prompt bloat; tools follow the same omit-when-empty behavior for consistency.
- System prompt policy now explicitly instructs tool usage via `tool_call` actions constrained to tools listed in prompt input.

## 2026-04-26 Task 8 Implementation Decisions
- Added a dedicated `GlobAdapter` instead of extending file/search adapters to keep glob matching isolated and aligned with the existing one-tool-per-handler adapter pattern.
- Registered `glob` as a low-safety builtin with string input schema because the runtime already models single-string tool inputs and v1 should avoid structured input churn.
- Rejected parent-traversal patterns up front and re-checked matched paths against `workspaceRoot` so recursive globbing cannot escape the workspace boundary.

## 2026-04-26 OpenAI Adapter Decisions
- Replaced OpenAI stub output path with real HTTP chat-completions calls and removed `buildStubJSONOutput` usage from OpenAI provider.
- Updated provider construction to pass `api_key` and `base_url` from config into `NewOpenAIProvider`, with default base URL fallback to `https://api.openai.com/v1`.
- Kept non-streaming response handling only (`choices[0].message.content`) and preserved provider identity contract (`Name() == "openai"`).

## 2026-04-26 Anthropic Adapter Decisions
- Replaced Anthropic stub path with real HTTP Messages API integration and removed all `buildStubJSONOutput` usage from Anthropic provider code path.
- Introduced Anthropic provider constructor shape `NewAnthropicProvider(apiKey, baseURL, model)` and updated provider wiring to pass config credentials with default base URL fallback to `https://api.anthropic.com/v1`.
- Implemented bounded retry policy (`maxRetries=2`) for `429`, `529`, and `5xx` with exponential backoff, while surfacing explicit authentication failure messaging for `401`.

## 2026-04-26 Verifier Command Execution Decisions
- Added `verify.CommandVerifier` as a separate verifier implementation instead of replacing `verify.Verifier`; heuristic checks remain in `checks.go` unchanged.
- `newVerifierFromConfig` now takes a `domain.ToolExecutor` and maps `verify_mode=standard|strict` to `CommandVerifier`, while `off` remains `FakeVerifier`.
- Command unavailability (not allowlisted/not found/not registered) is normalized to a non-passing verification reason `verification command not available` rather than surfacing as test failure text.
