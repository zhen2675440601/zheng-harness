# Make Agent Actually Work (Phase 2)

## TL;DR
> **Summary**: Unlock the agent's ability to genuinely solve coding tasks by injecting tool awareness into LLM prompts, integrating memory recall, implementing real OpenAI/Anthropic adapters, fixing execution primitives (grep/shell/glob), and making the verifier actively execute verification commands.
> **Deliverables**:
> - Tool definitions injected into every LLM prompt so the model knows what tools exist
> - Memory recall integrated into the runtime loop so context persists across steps
> - Real OpenAI and Anthropic HTTP adapters replacing stubs
> - Enhanced tool layer: glob tool, regex grep, fixed shell quoting, expanded allowlist
> - Active verifier that executes test/build/lint commands and parses real results
> - Deepened test coverage for runtime, memory, prompts, and adapters
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: 1 → 2 → 3 → 5 → 6 → 7

## Context
### Original Request
项目初始阶段计划（11个任务）已全部完成，需要规划下一步。目标是让 Agent 真正能解决编码任务，而非仅演示循环机制。

### Interview Summary
- 方向选择：让 Agent 真正能用（而非深耕测试或架构扩展）
- LLM 提供商：OpenAI 和 Anthropic 都实现真实适配器
- 验证器策略：主动执行验证命令（通过 exec_command 运行 go test/go build 等）
- 工具 schema 注入方式：text-in-prompt 优先，native tool calling 后续
- ToolCall.Input 保持 string，通过结构化输入约定传递复杂参数
- TDD 继续作为测试策略

### Metis Review (gaps addressed)
- 补充了依赖排序：工具上下文管线 → 记忆循环 → 执行+验证基础 → 适配器 → 开发工具 → 覆盖率
- 增加关键 guardrail：Verifier 依赖 exec_command 正确性，必须先修 quoting 再增强验证器
- 增加接口稳定性 guardrail：冻结工具调用格式后再扩展工具数量
- 增加验证规则：验证器主动执行需要防超时、区分 "command not found" vs 测试失败
- 识别了隐藏依赖：exec_command quoting 修复必须在验证器增强之前
- 识别了 scope creep 风险：避免适配器变成平台重写、验证器变成 job runner
- 补充了端到端验收标准：agent 必须能识别工具、调用工具、使用返回数据、执行后续步骤

## Work Objectives
### Core Objective
让 Agent 能端到端完成一个真实的编码任务循环：接收任务 → 识别可用工具 → 调用工具操作代码 → 运行验证命令 → 判断成功/失败 → 自纠正。每个子系统（prompt、memory、tools、verification、adapters）都真正可用。

### Deliverables
- `internal/domain/tool.go` 新增 ToolInfo 类型
- `internal/domain/ports.go` Model 接口增加 tools/memory 参数
- `internal/config/prompts/model_adapter.go` 注入工具定义和记忆上下文
- `internal/runtime/runtime.go` 集成 Recall 调用并传递工具/记忆给 Model
- `internal/llm/openai.go` 真实 OpenAI HTTP 适配器
- `internal/llm/anthropic.go` 真实 Anthropic HTTP 适配器
- `internal/tools/adapters/search.go` regex grep + 行号 + 输出模式
- `internal/tools/adapters/shell.go` 修复 quoting + 扩展 allowlist
- `internal/tools/adapters/glob.go` 新增 glob 文件模式匹配工具
- `internal/verify/verifier.go` 主动执行验证命令
- 测试覆盖提升：runtime → 70%+, memory → 50%+, adapters/prompts → 40%+

### Definition of Done (verifiable conditions with commands)
- `go test ./...` 通过
- `go test -race ./...` 通过
- Agent 在配置 DashScope/OpenAI 后能接收任务、识别并调用至少一个工具、使用返回数据完成后续步骤
- Agent 能 recall 先前步骤的记忆并在后续推理中使用
- Verifier 能通过 exec_command 主动执行 `go test` 并区分 pass/fail
- `grep_search` 支持 regex 模式匹配并返回行号
- `glob` 工具能按 `**/*.go` 等模式匹配文件
- `exec_command` 能正确传递含空格的引号参数
- OpenAI 适配器能完成一次真实推理请求
- Anthropic 适配器能完成一次真实推理请求

### Must Have
- 工具定义必须注入 LLM prompt，否则 model 不知道能调用什么
- 记忆 Recall 必须集成到 runtime 循环，否则记忆系统形同虚设
- Shell quoting 必须修复，否则 exec_command 在真实场景不可用
- Verifier 必须主动执行验证命令，否则 agent 无法真正验证完成状态
- 接口变更必须保持向后兼容（fake/stub 测试继续通过）

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- 不实现 native tool calling (Path B) — 本阶段只做 text-in-prompt
- 不实现多代理编排
- 不实现插件系统
- 不实现 Web UI / 网关
- 不实现向量数据库 / embedding 检索
- 不把适配器实现变成平台重写 — 每个适配器聚焦 HTTP 客户端
- 不把验证器变成 job runner — 只执行固定命令集，不引入调度/沙箱/环境管理
- 不扩展 allowlist 时引入危险命令（rm -rf、格式化等）无保护
- 不在 ToolCall.Input 引入 breaking change — 通过约定格式传递结构化参数
- 不过度设计 prompt 模板系统 — 本阶段只做必要的工具/记忆注入

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: **TDD + Go testing framework**
- QA policy: Every task includes executable happy-path and failure-path scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
> 依赖排序基于 Metis 建议：工具上下文管线 → 记忆循环 → 执行+验证基础 → 适配器 → 开发工具 → 覆盖率

Wave 1: 上下文管线 + 执行基础 (`1, 2, 4, 5`)
- 1: ToolInfo 类型 + Model 接口更新
- 2: 工具定义注入 prompt
- 4: 修复 exec_command quoting
- 5: 扩展 exec_command allowlist + 可配置化

Wave 2: 记忆集成 + 验证器 + glob (`3, 6, 8`)
- 3: Memory recall 集成到 runtime
- 6: Verifier 主动执行验证命令
- 8: 新增 glob 工具

Wave 3: 适配器 + grep 增强 + 测试深化 (`7, 9, 10`)
- 7: 真实 OpenAI 适配器
- 9: 真实 Anthropic 适配器
- 10: grep regex 增强 + 测试覆盖提升

### Dependency Matrix (full, all tasks)
- 1 blocks 2, 3, 6, 7, 9
- 2 blocks 7, 9
- 3 blocks 7, 9
- 4 blocks 6
- 5 blocks 6
- 6 blocks 10
- 8 blocks nothing (独立工具)
- 7 blocks 10
- 9 blocks 10

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1 → 4 tasks → deep / unspecified-high / quick
- Wave 2 → 3 tasks → deep / unspecified-high
- Wave 3 → 3 tasks → deep / unspecified-high / writing

## TODOs

- [x] 1. Add ToolInfo type and update Model interface to accept tool definitions

  **What to do**:
  1. Create `internal/domain/tool.go` with `ToolInfo` struct: `Name string`, `Description string`, `Schema string` (JSON schema)
  2. Add `ListToolInfo() []domain.ToolInfo` method to `internal/tools/registry.go` that converts `ToolDefinition` to `ToolInfo` (strips Handler/Timeout/SafetyLevel — only prompt-relevant fields)
  3. Update `domain.Model` interface in `ports.go`: add `tools []ToolInfo` param to `NextAction()` and `memory []MemoryEntry` param to `CreatePlan()` and `NextAction()`
  4. Confirm `domain.MemoryStore` interface in `ports.go` already has `Recall()` method — if not, add it with signature matching `internal/store/memory_store.go:113` (`Recall(ctx, query RecallQuery) ([]MemoryEntry, error)`)
  5. Update ALL implementations of `domain.Model`: `FakeModel` (cmd/agent/fakes.go), `ModelAdapter` (internal/runtime/model_adapter.go), and any test fakes — they must accept and forward the new params
  5. Update `runtime.go` Engine to call `e.Tools.Registry().ListToolInfo()` and pass results to Model calls
  **Must NOT do**: Do not change ToolCall.Input type from string. Do not add native tool calling structures. Do not remove existing Model methods.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: interface changes ripple through multiple packages, requires careful dependency handling
  - Skills: `[]`
  - Omitted: [`/refactor`] - changes are targeted, not general refactoring

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [2,3,6,7,9] | Blocked By: []

  **References**:
  - Type definition: `internal/tools/definition.go:23-30` — ToolDefinition struct with Name/Description/Schema/SafetyLevel/Handler
  - Registry: `internal/tools/registry.go` — has `List() []ToolDefinition` method to convert
  - Interface: `internal/domain/ports.go:6-10` — current Model interface signatures
  - Fakes: `cmd/agent/fakes.go` — FakeModel that must be updated
  - Adapter: `internal/runtime/model_adapter.go` — ModelAdapter that must accept new params
  - Runtime: `internal/runtime/runtime.go:28-132` — Engine loop where tools are obtained and passed

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes after interface changes (all existing tests updated)
  - [ ] `ToolInfo` struct exists in `internal/domain/tool.go` with Name/Description/Schema fields
  - [ ] `Registry.ListToolInfo()` returns same count as `Registry.List()` but with only prompt-relevant fields
  - [ ] `FakeModel.NextAction()` accepts and stores `tools []ToolInfo` param (for test inspection)
  - [ ] `ModelAdapter.NextAction()` receives tools from runtime and forwards to prompt builder

  **QA Scenarios**:
  ```
  Scenario: ToolInfo conversion preserves tool metadata
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestListToolInfo`
    Expected: ListToolInfo returns entries matching List with Name/Description/Schema populated, Handler/Timeout/SafetyLevel absent
    Evidence: .sisyphus/evidence/task-1-toolinfo.txt

  Scenario: Interface changes don't break existing fakes
    Tool: Bash
    Steps: run `go test ./...` after all interface updates
    Expected: all existing tests pass without modification to test logic (only signature updates)
    Evidence: .sisyphus/evidence/task-1-toolinfo-compat.txt
  ```

  **Commit**: YES | Message: `feat(domain): add ToolInfo type and update Model interface for tool awareness` | Files: [`internal/domain/tool.go`, `internal/domain/ports.go`, `internal/tools/registry.go`, `cmd/agent/fakes.go`, `internal/runtime/model_adapter.go`, `internal/runtime/runtime.go`]

- [x] 2. Inject tool definitions and memory context into LLM prompts

  **What to do**:
  1. Update `internal/config/prompts/model_adapter.go`: `BuildNextActionInput()` accepts `tools []domain.ToolInfo` and `memory []domain.MemoryEntry` params
  2. Add `tools` field to the JSON payload: array of `{name, description, schema}` objects
  3. Add `memory` field to the JSON payload: array of `{scope, type, content, confidence, source}` objects
  4. Update `BuildCreatePlanInput()` similarly to accept and inject memory context
  5. Update `internal/runtime/model_adapter.go` to pass tools and memory to prompt builders
  6. Verify that prompts now contain tool names and descriptions so the model can reason about available tools
  **Must NOT do**: Do not implement native tool calling (Path B). Do not over-engineer prompt template system. Do not inject full tool schemas if they exceed reasonable token budgets — start with name + description + brief schema summary.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: prompt format changes affect all LLM interactions, requires careful design
  - Skills: `[]`
  - Omitted: [`/ai-slop-remover`] - new code, not cleanup

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [7,9] | Blocked By: [1]

  **References**:
  - Prompt builder: `internal/config/prompts/model_adapter.go` — current BuildNextActionInput/BuildCreatePlanInput functions
  - Model adapter: `internal/runtime/model_adapter.go` — calls prompt builders and forwards to provider
  - System prompt: `internal/config/prompts/system.go` — static system prompt to potentially update with tool usage guidance
  - ToolInfo type: `internal/domain/tool.go` (from task 1)
  - MemoryEntry type: `internal/memory/policy.go` — MemoryEntry struct with Scope/Type/Content/Confidence/Source

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] `BuildNextActionInput()` output JSON contains a `tools` array with all registered tool names
  - [ ] `BuildNextActionInput()` output JSON contains a `memory` array when memory entries exist
  - [ ] `BuildCreatePlanInput()` output JSON contains a `memory` array when memory entries exist
  - [ ] FakeModel in tests receives and can inspect tool definitions in the call

  **QA Scenarios**:
  ```
  Scenario: Tool definitions appear in generated prompt
    Tool: Bash
    Steps: run `go test ./internal/config/prompts/... -run TestToolDefinitionsInPrompt`
    Expected: BuildNextActionInput output contains "list_dir", "read_file", "write_file", "edit_file", "grep_search", "exec_command" tool names
    Evidence: .sisyphus/evidence/task-2-prompt-tools.txt

  Scenario: Memory entries appear in generated prompt
    Tool: Bash
    Steps: run `go test ./internal/config/prompts/... -run TestMemoryInPrompt`
    Expected: BuildNextActionInput with non-empty memory includes memory content in JSON output
    Evidence: .sisyphus/evidence/task-2-prompt-memory.txt

  Scenario: Empty memory does not bloat prompt
    Tool: Bash
    Steps: run `go test ./internal/config/prompts/... -run TestEmptyMemoryInPrompt`
    Expected: BuildNextActionInput with empty memory produces JSON without extraneous null/empty memory fields
    Evidence: .sisyphus/evidence/task-2-prompt-empty-memory.txt
  ```

  **Commit**: YES | Message: `feat(prompts): inject tool definitions and memory context into LLM prompts` | Files: [`internal/config/prompts/model_adapter.go`, `internal/runtime/model_adapter.go`, `*_test.go`]

- [x] 3. Integrate memory Recall into runtime loop

  **What to do**:
  1. In `internal/runtime/runtime.go`, before `createPlan()` call: invoke `e.Memory.Recall()` with query for session-scoped and project-scoped facts/summaries
  2. Pass recalled memory entries to `e.Model.CreatePlan()` (already has memory param from task 1)
  3. In `executeIteration()`, before `e.Model.NextAction()` call: invoke `e.Memory.Recall()` for relevant context
  4. Pass recalled memory to `e.Model.NextAction()` (already has memory param from task 1)
  5. Add `RecallQuery` construction logic: use task description as keywords, scope=session+project, type=fact+summary, limit=10
  6. Handle empty recall gracefully (no memory on first turn) — no prompt bloat
  **Must NOT do**: Do not add semantic search or vector retrieval. Do not recall global-scope entries (blocked by current policy). Do not inject all memory — use relevance filtering.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: runtime loop is the core execution path, memory integration affects all agent behavior
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [7,9] | Blocked By: [1]

  **References**:
  - Runtime loop: `internal/runtime/runtime.go:28-132` — createPlan at line ~68, executeIteration at line ~85
  - Memory policy: `internal/memory/policy.go` — RecallQuery struct with Scope/Type/Source/Keywords/Limit
  - Memory store: `internal/store/memory_store.go:113-179` — Recall() implementation with filtering
  - Remember call: `internal/runtime/runtime.go:183` — existing Remember() call site
  - Model interface: `internal/domain/ports.go` — CreatePlan/NextAction with memory param (from task 1)

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] Runtime calls Recall before both CreatePlan and NextAction
  - [ ] When memory entries exist, they are passed to Model calls
  - [ ] When no memory exists (first turn), runtime proceeds without error or prompt bloat
  - [ ] Recall query uses session+project scope and fact+summary type

  **QA Scenarios**:
  ```
  Scenario: Memory from step 1 is available in step 2
    Tool: Bash
    Steps: run `go test ./internal/runtime/... -run TestMemoryRecallInLoop`
    Expected: after a step that writes memory via Remember(), the next step's Model call receives non-empty memory entries
    Evidence: .sisyphus/evidence/task-3-memory-recall.txt

  Scenario: First turn with no memory works cleanly
    Tool: Bash
    Steps: run `go test ./internal/runtime/... -run TestNoMemoryFirstTurn`
    Expected: runtime proceeds normally when Recall returns empty, Model receives empty memory slice
    Evidence: .sisyphus/evidence/task-3-memory-empty.txt

  Scenario: Recall does not cause infinite loop or latency
    Tool: Bash
    Steps: run `go test ./internal/runtime/... -run TestMemoryRecallPerformance`
    Expected: Recall call completes within normal step timeout, no retry or blocking
    Evidence: .sisyphus/evidence/task-3-memory-perf.txt
  ```

  **Commit**: YES | Message: `feat(runtime): integrate memory recall into agent loop` | Files: [`internal/runtime/runtime.go`, `*_test.go`]

- [x] 4. Fix exec_command shell quoting

  **What to do**:
  1. Add `github.com/kballard/go-shellquote` dependency (or implement minimal shell-style quoting parser)
  2. Replace `strings.Fields(commandLine)` in `internal/tools/adapters/shell.go` with proper shell parsing that preserves quoted arguments
  3. Handle: arguments with spaces (`go test "./path with spaces/..."`), quoted strings (`git commit -m "hello world"`), escaped characters
  4. Update tests to verify quoted arguments are preserved correctly
  5. Verify safety policy still validates the base command (first field) against allowlist
  **Must NOT do**: Do not implement full bash/shell semantics (pipes, redirects, subshells). Do not change SafetyPolicy validation logic. Do not allow command chaining (&&, ||, ;).

  **Recommended Agent Profile**:
  - Category: `quick` - Reason: focused fix in a single adapter file
  - Skills: `[]`
  - Omitted: [`/review-work`] - narrow fix, review after full wave

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [6] | Blocked By: []

  **References**:
  - Current impl: `internal/tools/adapters/shell.go` — uses `strings.Fields()` which breaks quoted args
  - Safety: `internal/tools/safety.go` — validateCommand checks first field against allowlist
  - Tests: `internal/tools/tools_test.go` — existing tool execution tests
  - Go shellquote: `github.com/kballard/go-shellquote` — well-maintained shell parsing library

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] `exec_command` with `git commit -m "hello world"` produces command=`git`, args=`["commit", "-m", "hello world"]`
  - [ ] `exec_command` with `go test "./pkg/sub/..."` preserves the path with spaces
  - [ ] Safety policy still rejects commands not in allowlist
  - [ ] Command chaining (&&, ||, ;) is rejected by safety policy

  **QA Scenarios**:
  ```
  Scenario: Quoted arguments are preserved
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestShellQuoting`
    Expected: "git commit -m \"hello world\"" parses to ["git", "commit", "-m", "hello world"]
    Evidence: .sisyphus/evidence/task-4-shellquote.txt

  Scenario: Command chaining is blocked
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestCommandChainingBlocked`
    Expected: "go test && rm -rf /" is rejected by safety policy with explicit error
    Evidence: .sisyphus/evidence/task-4-shellchain-error.txt

  Scenario: Unquoted args still work
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestUnquotedArgs`
    Expected: "go test ./..." still works as before (backward compatibility)
    Evidence: .sisyphus/evidence/task-4-shellquote-compat.txt
  ```

  **Commit**: YES | Message: `fix(tools): use proper shell quoting for exec_command` | Files: [`internal/tools/adapters/shell.go`, `go.mod`, `go.sum`, `*_test.go`]

- [x] 5. Expand exec_command allowlist and make it configurable

  **What to do**:
  1. Expand default `AllowedCommands` in `internal/tools/executor.go` to include: `npm`, `node`, `npx`, `yarn`, `pnpm`, `python`, `python3`, `pip`, `pip3`, `uv`, `make`, `cargo`, `rustc`, `docker`, `docker-compose`, `cat`, `head`, `tail`, `echo`, `mkdir`, `cp`, `mv`, `env`, `which`, `ctest`
  2. Add `allowed_commands` field to runtime config in `internal/config/config.go` under `runtime` section
  3. When `allowed_commands` is specified in config, use it instead of defaults; otherwise use expanded defaults
  4. Add `--allow-command` CLI flag for one-off command additions
  5. Classify commands by risk: read-only (cat, head, ls, pwd) vs build (go, make, npm) vs destructive (rm, mv, docker) — destructive commands require explicit allowlist inclusion
  6. Add `rm` to explicit exclusion list (never allow by default even if in config)
  **Must NOT do**: Do not add `rm -rf`, `format`, `mkfs`, or similar destructive commands to default allowlist. Do not implement sandboxing or resource limits beyond existing timeout.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: security-sensitive changes to command execution policy
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [6] | Blocked By: []

  **References**:
  - Current allowlist: `internal/tools/executor.go:23` — `AllowedCommands: []string{"go", "git", "pwd", "ls", "dir"}`
  - Safety policy: `internal/tools/safety.go` — validateCommand checks against allowlist
  - Config: `internal/config/config.go` — runtime configuration structure
  - CLI: `cmd/agent/cli.go` — CLI flag definitions

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] Default allowlist includes at least: go, git, npm, node, python, python3, make, docker, cat, mkdir
  - [ ] `rm` is explicitly excluded from default allowlist even if somehow added to config
  - [ ] Config `allowed_commands` overrides defaults when specified
  - [ ] `--allow-command` flag adds to allowlist for the session

  **QA Scenarios**:
  ```
  Scenario: Expanded default allowlist allows npm
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestExpandedAllowlist`
    Expected: "npm test" is accepted by safety policy with default config
    Evidence: .sisyphus/evidence/task-5-allowlist.txt

  Scenario: rm is always blocked
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestRmAlwaysBlocked`
    Expected: "rm -rf /tmp/test" is rejected even when rm is added to config allowlist
    Evidence: .sisyphus/evidence/task-5-allowlist-rm.txt

  Scenario: Custom allowlist overrides defaults
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestCustomAllowlist`
    Expected: when config specifies allowed_commands=["go"], only "go" commands are allowed, "npm" is rejected
    Evidence: .sisyphus/evidence/task-5-allowlist-custom.txt
  ```

  **Commit**: YES | Message: `feat(tools): expand command allowlist and make it configurable` | Files: [`internal/tools/executor.go`, `internal/tools/safety.go`, `internal/config/config.go`, `cmd/agent/cli.go`, `*_test.go`]

- [x] 6. Enhance verifier to actively execute verification commands

  **What to do**:
  1. Add a `CommandVerifier` struct in `internal/verify/` that accepts a `ToolExecutor` dependency
  2. Implement verification strategies that actively run commands:
     - `TestVerification`: executes `go test ./...` (or equivalent) via ToolExecutor, parses exit code and output
     - `BuildVerification`: executes `go build ./...` via ToolExecutor, checks for compile errors
     - `LintVerification`: executes `go vet ./...` via ToolExecutor, checks for warnings
  3. Replace heuristic keyword matching with real command execution + exit code checking
  4. Wire `CommandVerifier` into `cmd/agent/cli.go` when `verify_mode` is `standard` or `strict`
  5. Add timeout for verification commands (separate from step timeout, e.g., 60s max)
  6. Handle failures: "command not found" (tool not available) vs "test failed" (real failure)
  **Must NOT do**: Do not turn verifier into a job runner/scheduler. Do not implement sandboxing. Do not add parallel test execution. Do not verify by running commands outside the tool executor.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: verification is the core Harness value layer, requires careful design
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI verification

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: [10] | Blocked By: [4, 5]

  **References**:
  - Current verifier: `internal/verify/verifier.go` — PolicyVerifier with check orchestration
  - Current checks: `internal/verify/checks.go` — heuristic-based EvidenceCheck/TestCheck/BuildCheck/LintCheck
  - Tool executor: `internal/tools/executor.go` — Executor implements ToolExecutor
  - CLI wiring: `cmd/agent/cli.go:116-125` — newVerifierFromConfig function
  - Domain port: `internal/domain/ports.go` — Verifier interface

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] `CommandVerifier` executes real `go test` via ToolExecutor and correctly identifies pass/fail
  - [ ] `CommandVerifier` executes real `go build` and correctly identifies success/compile error
  - [ ] Verification timeout is enforced (commands exceeding 60s are terminated)
  - [ ] "command not found" errors are handled gracefully (not treated as test failure)
  - [ ] CLI `--verify-mode standard` uses CommandVerifier with real execution
  - [ ] CLI `--verify-mode off` still uses FakeVerifier (backward compatible)

  **QA Scenarios**:
  ```
  Scenario: Verifier detects real test failure
    Tool: Bash
    Steps: run `go test ./internal/verify/... -run TestCommandVerifierDetectsFailure`
    Expected: when a test file with a deliberate failure exists, CommandVerifier reports verification_failed with test output
    Evidence: .sisyphus/evidence/task-6-verifier.txt

  Scenario: Verifier confirms real test pass
    Tool: Bash
    Steps: run `go test ./internal/verify/... -run TestCommandVerifierConfirmsPass`
    Expected: when all tests pass, CommandVerifier reports verification_passed
    Evidence: .sisyphus/evidence/task-6-verifier-pass.txt

  Scenario: Verifier handles timeout
    Tool: Bash
    Steps: run `go test ./internal/verify/... -run TestCommandVerifierTimeout`
    Expected: a long-running verification command is terminated after 60s with timeout error
    Evidence: .sisyphus/evidence/task-6-verifier-timeout.txt

  Scenario: Verify off mode unchanged
    Tool: Bash
    Steps: run `go test ./cmd/agent/... -run TestVerifyModeOff`
    Expected: FakeVerifier is used when verify_mode=off, same as before
    Evidence: .sisyphus/evidence/task-6-verifier-off.txt
  ```

  **Commit**: YES | Message: `feat(verify): add active command execution verification` | Files: [`internal/verify/command_verifier.go`, `internal/verify/verifier.go`, `cmd/agent/cli.go`, `*_test.go`]

- [x] 7. Implement real OpenAI HTTP adapter

  **What to do**:
  1. Replace stub implementation in `internal/llm/openai.go` with real HTTP client calling OpenAI Chat Completions API (`/v1/chat/completions`)
  2. Support request format: `model`, `messages` array (system + user), JSON response mode
  3. Parse response: extract `choices[0].message.content` as the model output
  4. Handle errors: auth failure (401), rate limit (429), server error (5xx), timeout
  5. Support configuration from `zheng.json`: `api_key`, `base_url` (for DeepSeek/OpenAI-compatible services), `model`
  6. Add retry logic: up to 2 retries with exponential backoff on 429/5xx
  7. Ensure `buildStubJSONOutput()` is removed from OpenAI adapter path
  **Must NOT do**: Do not implement streaming (SSE) — use non-streaming responses only. Do not implement function calling / native tool use. Do not hardcode model names — read from config.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: external API integration with error handling, retry, and config
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI

  **Parallelization**: Can Parallel: YES (with task 9) | Wave 3 | Blocks: [10] | Blocked By: [2, 3]

  **References**:
  - Current stub: `internal/llm/openai.go` — calls buildStubJSONOutput(), ~30 lines
  - Real reference: `internal/llm/dashscope.go` — 195 lines of working HTTP implementation
  - Provider interface: `internal/llm/provider.go` — Provider interface with Generate method
  - Config: `internal/config/config.go` — provider config with api_key/base_url/model
  - Stub JSON: `internal/llm/stub_json.go` — buildStubJSONOutput to remove from OpenAI path

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] OpenAI adapter makes real HTTP POST to configured base_url
  - [ ] Auth failure (401) returns explicit error message
  - [ ] Rate limit (429) triggers retry with backoff
  - [ ] Valid request returns parsed response content (not stub output)
  - [ ] `buildStubJSONOutput()` is no longer called in OpenAI path
  - [ ] Works with DeepSeek-compatible base_url (configurable)

  **QA Scenarios**:
  ```
  Scenario: OpenAI adapter calls real API
    Tool: Bash
    Steps: run `go test ./internal/llm/... -run TestOpenAIRealAPI` (requires OPENAI_API_KEY env)
    Expected: adapter returns non-stub response from real API
    Evidence: .sisyphus/evidence/task-7-openai.txt

  Scenario: OpenAI adapter handles auth error
    Tool: Bash
    Steps: run `go test ./internal/llm/... -run TestOpenAIAuthError`
    Expected: invalid API key returns explicit "authentication failed" error, not panic
    Evidence: .sisyphus/evidence/task-7-openai-auth-error.txt

  Scenario: OpenAI adapter retries on rate limit
    Tool: Bash
    Steps: run `go test ./internal/llm/... -run TestOpenAIRateLimit`
    Expected: 429 response triggers up to 2 retries with increasing delay
    Evidence: .sisyphus/evidence/task-7-openai-retry.txt

  Scenario: Works with DeepSeek-compatible endpoint
    Tool: Bash
    Steps: run `go test ./internal/llm/... -run TestOpenAICompatibleEndpoint`
    Expected: when base_url points to DeepSeek API, adapter works correctly with custom model name
    Evidence: .sisyphus/evidence/task-7-openai-deepseek.txt
  ```

  **Commit**: YES | Message: `feat(llm): implement real OpenAI HTTP adapter` | Files: [`internal/llm/openai.go`, `*_test.go`]

- [x] 8. Add glob tool for file pattern matching

  **What to do**:
  1. Create `internal/tools/adapters/glob.go` with a `GlobAdapter` struct
  2. Implement glob pattern matching using `filepath.Walk` + pattern matching (`filepath.Match` for simple patterns, or `github.com/bmatcuk/doublestar` for `**/*.go` recursive patterns)
  3. Register as `glob` tool in `internal/tools/executor.go` with: Name="glob", Description="Find files matching a glob pattern", Schema for pattern input, SafetyLevel=low
  4. Input format: glob pattern string (e.g., `**/*.go`, `src/**/*.ts`, `*.json`)
  5. Output: list of matched file paths relative to workspace root
  6. Handle edge cases: no matches (return empty list, not error), hidden files (respect .gitignore if feasible), Windows path separators
  **Must NOT do**: Do not implement file content search (that's grep). Do not implement regex in glob. Do not add .gitignore parsing in v1 (can be future enhancement).

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: new tool implementation with OS-specific considerations
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [] | Blocked By: []

  **References**:
  - Tool adapter pattern: `internal/tools/adapters/files.go` — existing adapter implementations to follow
  - Executor registration: `internal/tools/executor.go:62-116` — builtinDefinitions() where tools are registered
  - ToolDefinition: `internal/tools/definition.go:23-30` — struct for registering new tools
  - Safety: `internal/tools/safety.go` — path validation for workspace boundary

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] `glob` tool is registered in executor with Name/Description/Schema/SafetyLevel
  - [ ] Pattern `**/*.go` returns all Go files in workspace recursively
  - [ ] Pattern `*.json` returns only top-level JSON files
  - [ ] No-match pattern returns empty list (not an error)
  - [ ] Results are relative to workspace root

  **QA Scenarios**:
  ```
  Scenario: Glob finds Go files recursively
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestGlobRecursive`
    Expected: pattern "**/*.go" returns at least cmd/agent/main.go and internal/domain/ports.go
    Evidence: .sisyphus/evidence/task-8-glob.txt

  Scenario: No match returns empty list
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestGlobNoMatch`
    Expected: pattern "*.xyz" returns empty file list without error
    Evidence: .sisyphus/evidence/task-8-glob-empty.txt

  Scenario: Glob respects workspace boundary
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestGlobWorkspaceBoundary`
    Expected: pattern "**/*" does not return files outside workspace root
    Evidence: .sisyphus/evidence/task-8-glob-boundary.txt
  ```

  **Commit**: YES | Message: `feat(tools): add glob file pattern matching tool` | Files: [`internal/tools/adapters/glob.go`, `internal/tools/executor.go`, `go.mod`, `go.sum`, `*_test.go`]

- [x] 9. Implement real Anthropic HTTP adapter

  **What to do**:
  1. Replace stub implementation in `internal/llm/anthropic.go` with real HTTP client calling Anthropic Messages API (`/v1/messages`)
  2. Support request format: `model`, `system` (separate field), `messages` array, `max_tokens`
  3. Handle Anthropic-specific response format: `content[0].text` as output, `stop_reason` for completion detection
  4. Add `x-api-key` header and `anthropic-version` header (same pattern as DashScope which already uses Anthropic-compatible endpoint)
  5. Handle errors: auth (401), rate limit (429), overloaded (529), server error (5xx)
  6. Add retry logic: up to 2 retries with exponential backoff on 429/529/5xx
  7. Remove `buildStubJSONOutput()` call from Anthropic adapter path
  **Must NOT do**: Do not implement streaming. Do not implement native tool use (tool_use blocks). Do not hardcode API version — read from config or use stable default.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: external API integration, Anthropic has different request/response format from OpenAI
  - Skills: `[]`
  - Omitted: [`/playwright`] - no UI

  **Parallelization**: Can Parallel: YES (with task 7) | Wave 3 | Blocks: [10] | Blocked By: [2, 3]

  **References**:
  - Current stub: `internal/llm/anthropic.go` — calls buildStubJSONOutput(), ~30 lines
  - DashScope reference: `internal/llm/dashscope.go` — already uses Anthropic-compatible endpoint, 195 lines
  - Provider interface: `internal/llm/provider.go` — Provider interface
  - Config: `internal/config/config.go` — provider config

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] Anthropic adapter makes real HTTP POST to `/v1/messages`
  - [ ] Request includes `x-api-key` and `anthropic-version` headers
  - [ ] Response is parsed from `content[0].text` format
  - [ ] Auth failure returns explicit error
  - [ ] Rate limit triggers retry with backoff
  - [ ] `buildStubJSONOutput()` is no longer called in Anthropic path

  **QA Scenarios**:
  ```
  Scenario: Anthropic adapter calls real API
    Tool: Bash
    Steps: run `go test ./internal/llm/... -run TestAnthropicRealAPI` (requires ANTHROPIC_API_KEY env)
    Expected: adapter returns non-stub response from real API
    Evidence: .sisyphus/evidence/task-9-anthropic.txt

  Scenario: Anthropic auth error handled
    Tool: Bash
    Steps: run `go test ./internal/llm/... -run TestAnthropicAuthError`
    Expected: invalid API key returns explicit "authentication failed" error
    Evidence: .sisyphus/evidence/task-9-anthropic-auth-error.txt

  Scenario: Anthropic overloaded error triggers retry
    Tool: Bash
    Steps: run `go test ./internal/llm/... -run TestAnthropicOverloaded`
    Expected: 529 response triggers retry with backoff
    Evidence: .sisyphus/evidence/task-9-anthropic-retry.txt
  ```

  **Commit**: YES | Message: `feat(llm): implement real Anthropic HTTP adapter` | Files: [`internal/llm/anthropic.go`, `*_test.go`]

- [x] 10. Enhance grep_search and deepen test coverage

  **What to do**:
  1. Replace `strings.Contains` in `internal/tools/adapters/search.go` with `regexp.Compile` + `regexp.MatchString` for regex support
  2. Add output modes to grep: `files_with_matches` (current default, returns file paths), `content` (returns matching lines with line numbers), `count` (returns match count per file)
  3. Input format convention: first line is pattern, optional second line is flags (e.g., `i` for case-insensitive), optional third line is output_mode (`files_with_matches`/`content`/`count`), optional fourth line is include glob (e.g., `*.go`)
  4. Handle invalid regex patterns gracefully (return error, not panic)
  5. Deepen test coverage:
     - Add integration tests for runtime loop with real tool/memory/verify interactions
     - Add memory policy rule tests (currently 0% coverage)
     - Add prompt template tests (currently 0% coverage)
     - Add tool adapter edge case tests
  6. Target coverage: runtime ≥70%, memory ≥50%, adapters/prompts ≥40%
  **Must NOT do**: Do not change ToolCall.Input type. Do not add external grep binary dependency. Do not chase coverage percentage at expense of meaningful tests.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: combines feature enhancement (grep) with broad test deepening
  - Skills: `[]`
  - Omitted: [`/ai-slop-remover`] - new code and tests

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: [] | Blocked By: [6, 7, 9]

  **References**:
  - Current grep: `internal/tools/adapters/search.go` — strings.Contains, no regex/line numbers/output modes
  - ToolCall: `internal/domain/tool.go` — Input is single string (use convention format)
  - Coverage gaps: runtime 42.6%, memory 0%, adapters 0%, prompts 0%
  - Test patterns: `internal/runtime/runtime_test.go` — existing fake-based test pattern
  - Memory store: `internal/store/memory_store.go` — untested memory policy rules

  **Acceptance Criteria** (agent-executable only):
  - [ ] `go test ./...` passes
  - [ ] `grep_search` with regex pattern returns correct matches
  - [ ] `grep_search` with `content` mode returns matching lines with line numbers
  - [ ] `grep_search` with `count` mode returns match counts per file
  - [ ] Invalid regex pattern returns explicit error (not panic)
  - [ ] Case-insensitive flag works (pattern matches regardless of case)
  - [ ] Include glob filter works (only search `*.go` files)
  - [ ] Test coverage: runtime ≥70%, memory ≥50%, adapters ≥40%, prompts ≥40%

  **QA Scenarios**:
  ```
  Scenario: Regex grep finds pattern
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestGrepRegex`
    Expected: pattern "func.*Test" matches test function declarations in Go files
    Evidence: .sisyphus/evidence/task-10-grep-regex.txt

  Scenario: Content mode with line numbers
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestGrepContentMode`
    Expected: grep returns "file.go:42: matching line content" format
    Evidence: .sisyphus/evidence/task-10-grep-content.txt

  Scenario: Invalid regex handled gracefully
    Tool: Bash
    Steps: run `go test ./internal/tools/... -run TestGrepInvalidRegex`
    Expected: pattern "[invalid" returns error "invalid regex pattern" without panic
    Evidence: .sisyphus/evidence/task-10-grep-invalid.txt

  Scenario: Coverage targets met
    Tool: Bash
    Steps: run `go test ./... -cover` and check package-level percentages
    Expected: runtime ≥70%, memory ≥50%, adapters ≥40%, prompts ≥40%
    Evidence: .sisyphus/evidence/task-10-coverage.txt
  ```

  **Commit**: YES | Message: `feat(tools): enhance grep with regex and output modes; deepen test coverage` | Files: [`internal/tools/adapters/search.go`, `internal/runtime/*_test.go`, `internal/memory/*_test.go`, `internal/config/prompts/*_test.go`, `*_test.go`]

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (run agent with real LLM on a test coding task)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- One commit per numbered task when the task creates a coherent verification boundary
- Keep commit messages aligned to Phase 1 convention: `feat(core)`, `feat(tools)`, `feat(llm)`, `feat(verify)`, `fix(tools)`, `test(runtime)`

## Success Criteria
- Agent can receive a task, identify relevant tools from prompt-injected tool definitions, call at least one tool correctly, and use returned data in a follow-up step
- Agent can recall stored memory during a later turn and use it in context
- Verifier can execute an actual test/build command and distinguish pass vs fail
- OpenAI and Anthropic adapters can complete a real inference request successfully
- `exec_command` can pass quoted arguments without corruption
- `grep_search` supports regex, line numbers, and output modes
- `glob` can match file patterns used in real coding tasks
- All existing tests continue to pass (backward compatibility maintained)
- Test coverage: runtime ≥70%, memory ≥50%, adapters/prompts ≥40%
