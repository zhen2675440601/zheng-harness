# zheng-harness 项目进度记录

## 项目概述

基于 Harness Engineering 思想实现的通用 Agent Harness Go MVP。

## 当前进度

**Phase 1 ✅ 完成 | Phase 2 ✅ 完成 | Phase 3 ✅ 完成 | Phase 4 ✅ 完成 | v2 ✅ 完成 | v3 ✅ 完成 | v4 ✅ 完成 | v5 ✅ 完成 | v6 ✅ 完成 | v7 ✅ 完成**

**已完成**: T1-T11 (11/11 核心任务) + Phase 3 T1-T12 + Phase 4 闭环验证 + v1/v2/v3/v4/v5/v6/v7 发布准备

验证状态权威来源：[`docs/validation-matrix.md`](docs/validation-matrix.md)。

### 核心任务 (T1-T11) - 100% 完成
- ✅ T1: Bootstrap Go 项目骨架
- ✅ T2: 定义核心域契约
- ✅ T3: 实现 plan-execute-verify 循环
- ✅ T4: 构建工具注册表
- ✅ T5: 建立 TDD/CI 基线
- ✅ T6: 配置与模型适配器边界
- ✅ T7: 实现验证与自纠正系统
- ✅ T8: SQLite 持久化
- ✅ T9: CLI 命令
- ✅ T10: 基准测试与回放
- ✅ T11: 文档与 ADR

### Phase 3 T1-T12 收尾

- ✅ T10: 非 coding 任务 fixture (research, file_workflow)
- ✅ T11: Session 持久化 task_type 元数据
- ✅ T12: 文档收尾与端到端验证

### v2 Wave 2 进度（✅ 完成）

- ✅ T11: Streaming Runtime Integration (`Engine.RunStream` + `ModelAdapter` stream path)
- ✅ T12: Streaming Resume/Inspect Compatibility
- ✅ T13: Streaming + new tools documentation updates
- ✅ T14-T26: Plugin System (dual-mode: external process + native Go plugin)
- ✅ T27-T39: Multi-Agent Orchestration (orchestrator-worker with DAG scheduling)
- ✅ T40-T43: Integration testing and validation matrix update

#### v2 Wave 2 说明

- `Engine.RunStream` 现在通过运行时事件通道输出 plan/tool/step/session 事件，并让 `ModelAdapter` 在 streaming 上下文中优先走 `Provider.Stream()`
- 当 provider 本身不支持原生流式输出时，仍通过 `llm.StreamFallback()` 包装 `Generate()`，保持 `RunStream` 端到端可用
- `resume --stream` 已验证可在中断 session 上继续流式输出剩余步骤
- `inspect` 对 streaming session 仍只读取持久化的 `Session` / `Plan` / `Step`，不持久化 token delta 等中间事件
- README / USAGE / PROGRESS 已同步更新 streaming、JSONL 输出、`web_fetch` / `ask_user` / `code_search` 等能力说明

**v2 新增能力**:
- **Streaming 输出**: Token delta、tool lifecycle、step/session completion 事件流式输出
- **3 个新工具**: `web_fetch` (HTTP/S 抓取 + 域名白名单), `ask_user` (CLI 交互提示), `code_search` (语言感知代码搜索)
- **双模式插件系统**: 外部进程 (JSON-RPC over stdio) + 原生 Go 插件 (.so on Linux/macOS)
- **多 Agent 编排**: orchestrator-worker 模式，DAG 依赖调度，有界并发控制

**v2 验证状态**: 所有 streaming、tools、plugins、orchestration 测试已通过，集成测试覆盖端到端场景。详见 [`docs/validation-matrix.md`](docs/validation-matrix.md)。

---

## v3 ✅ 完成

详见 [v3 Core Extensibility 计划](.sisyphus/plans/v3-core-extensibility.md):

- ✅ **Provider 插件化**: LLM Provider 选择从硬编码改为可插拔的 registry + loader
- ✅ **Verifier 插件化**: 验证策略从硬编码改为可插拔的 policy registry
- ✅ **Agent Strategy 插件化**: 支持策略注入，保留 host 的编排/取消/持久化权威
- ✅ **共享治理原语**: 发现、验证、碰撞处理、生命周期、fail-closed 选择语义
- ✅ **持久化溯源**: 插件化执行的 provider/verifier/agent 记录 provenance，inspect/resume 仍可读

### v3 验证状态

**v3 Must Have** (全部实现):
- ✅ 三种独立的扩展契约 (provider/verifier/agent strategy)
- ✅ 内置实现保持可用且一等公民
- ✅ Host 拥有 registry：发现、ID 命名空间、碰撞策略、选择优先级、生命周期、fallback 语义
- ✅ Fail-closed: run/resume 选择失败时优雅降级，inspect 始终可读
- ✅ 持久化 provenance: family, logical ID, execution mode, contract version, implementation version
- ✅ TDD 实现

**v3 Must NOT Have** (全部遵守):
- ✅ NO 产品入口扩展 (web UI, HTTP API server, daemon)
- ✅ NO 插件市场/远程安装/签名
- ✅ NO 向量数据库/embedding/语义记忆扩展
- ✅ NO 破坏性移除内置路径
- ✅ NO 元抽象强制统一所有插件家族

**v3 验证状态**: 所有 provider/verifier/agent-strategy 插件家族测试已通过，fail-closed 语义、provenance 持久化、内置兼容性全部验证。详见 [`docs/validation-matrix.md`](docs/validation-matrix.md)。

---

## v4 ✅ 完成

详见 [v4 API Server Productization 计划](.sisyphus/plans/v4-api-server-productization.md):

- ✅ **HTTP API 服务器**: `cmd/server` 独立入口点，JWT 认证 REST 端点
- ✅ **SSE 流式输出**: `GET /api/v1/sessions/{id}/stream` 输出 6 种运行时事件
- ✅ **会话并发执行**: Session-Actor 架构，默认 8 个活跃会话上限
- ✅ **OpenAI/Anthropic Provider**: 真实 HTTP-backed 实现，支持 Generate 和 Stream 路径
- ✅ **SQLite WAL 模式**: 服务器路径显式启用 WAL，支持并发写入
- ✅ **会话生命周期管理**: 重复 resume 返回 409，活跃会话超限返回 429

### v4 验证状态

**v4 验收命令** (全部通过):
```bash
go build ./...
go test ./...
go test ./cmd/server/...
go test ./internal/server/...
go test ./internal/runtime/... -run TestSessionManager
go test ./internal/llm/... -run "Test(OpenAI|Anthropic)"
go test ./cmd/agent/... -run TestCLIUnaffectedByServer
```

**API 端点验证**:
- ✅ `POST /api/v1/run` 创建新会话并返回 202 Accepted
- ✅ `POST /api/v1/resume` 恢复未完成会话，拒绝已运行/缺失会话
- ✅ `GET /api/v1/sessions/{id}/inspect` 无需活跃 actor 即可读取
- ✅ `GET /api/v1/sessions/{id}/stream` SSE 有序事件输出
- ✅ `GET /healthz` 健康检查（无认证）
- ✅ 所有 `/api/v1/*` 端点强制 JWT 认证

**v4 验证状态**: 所有 API 端点、SSE 流、并发执行、Provider 实现、SQLite WAL、CLI 非回归测试已通过。详见 [`docs/validation-matrix.md`](docs/validation-matrix.md)。

---

## v7 ✅ 完成

详见 [v7 Internal Reliability Release 计划](.sisyphus/plans/v7-internal-reliability-release.md):

v7 是 **内部可靠性发布**，不扩展新能力。聚焦将 v6 chat-first Web UI + six-layer backend 打磨为可依赖的内部稳定版本。所有改动均遵循 TDD：先补失败用例，再实施稳定性改进。

### 任务 1: RED Baseline (可靠性失败测试优先)

- ✅ API 级别 RED 测试：缺失会话、不支持的 task_type
- ✅ Domain task typing：添加 `TaskCategory` (`coding`/`research`/`file_workflow`/`general`) + `ProtocolHint` + `VerificationPolicy` 字段
- ✅ Runtime 级别 RED 测试：订阅-after-完成、流式缺失会话
- ✅ Service 级别 RED 测试：不支持的 task_type 拒绝
- ✅ Playwright 引导验证 + 17 个测试注册清单
- ✅ v5 ADR 护栏确认（v5 范围未泄露到 v7）

证据：`task-1-api-red.txt`, `task-1-domain-task-typing.txt`, `task-1-service-red.txt`, `task-1-runtime-red.txt`, `task-1-playwright-list.txt`, `task-1-v5-adr-guardrails.txt`

### 任务 2: Service Validation（服务层校验硬化）

- ✅ `isSupportedTaskType()` 辅助函数：精确字符串匹配 `general`/`coding`/`research`/`file_workflow`
- ✅ `StartConversation()` 和 `StartChat()` 拒绝不支持 task_type
- ✅ 缺失会话返回 `NotFoundError`（消息中包含资源 ID）
- ✅ 运行中对话拒绝提交回复返回 `ConflictError`
- ✅ 终止会话拒绝 resume 返回 `ConflictError`
- ✅ Action 合约扩展：`request_input` 和 `complete` 两种新 action 类型

**验证命令**:
```bash
go test ./internal/service/... -v -run "UnsupportedTaskType|NotFoundForMissing|RejectsTerminal|RejectsRunning"
```
全部 5 个测试 PASS。

证据：`task-2-service-validation.txt`, `task-2-service-conflict.txt`, `task-2-action-contract.txt`

### 任务 3: Session Manager Lifecycle（会话生命周期硬化）

- ✅ `SessionActor.Subscribe()` 在 actor 终结后 fail-closed
- ✅ `sessionEventRelay` 拆除后新订阅被确定性拒绝
- ✅ Nil runner factory 提升为显式 actor 失败
- ✅ 静态 task-type registry：`TaskRegistry` 编译期元数据 map，确定性 fallback 元数据

**验证命令**:
```bash
go test ./internal/runtime/... -run "Session"
```
PASS（`ok zheng-harness/internal/runtime 0.388s`）

**已知阻塞**: `go test -race ./internal/runtime/...` 在 windows/386 上不支持（Go 工具链限制）。

证据：`task-3-session-manager-edge.txt`, `task-3-session-manager-race.txt`, `task-3-task-registry.txt`

### 任务 4: Stream Hardening（SSE 流式硬化）

- ✅ 流式 happy path 验证（runtime 流事件排序、tool lifecycle、step/session completion）
- ✅ 流式 failure path 验证（server 端缺失会话结构化包络）
- ✅ Task-aware verifier dispatch：根据 task metadata 选择验证策略
- ✅ 研究型证据验证器、file workflow 状态/输出验证器
- ✅ `not_applicable` 验证状态支持

**验证命令**:
```bash
go test ./internal/server/...
go test ./internal/runtime -run "Stream|Event|SSE"
```
全部 PASS。

证据：`task-4-stream-happy.txt`, `task-4-stream-failure.txt`, `task-4-task-aware-verifier.txt`

### 任务 5: API Error Envelopes（统一 API 错误包络）

- ✅ 统一 `writeStructuredError` 包络：`{"error":{"code":"...","message":"..."},"request_id":"..."}`
- ✅ 6 种错误码：unauthorized/invalid_request/not_found/conflict/too_many_requests/internal_error
- ✅ `NotFoundError` 消息包含缺失资源 ID：`session "missing-session" not found`
- ✅ Panic recoverer：原始 panic 值永不暴露，统一返回 "internal server error"
- ✅ 跨机器 continuation 文档更新

**验证命令**:
```bash
go test ./internal/server/... -run "Envelope|Recoverer|Panic"
```
全部 PASS。

证据：`task-5-api-errors.txt`, `task-5-api-recoverer.txt`, `task-5-cross-machine-docs.txt`

### 任务 6: Diagnostics（运行时诊断）

- ✅ 运行时协议元数据解析：Runtime 通过 task registry 解析协议元数据
- ✅ `request_input` 终端路径：不执行工具、标记 not_applicable、session 转为 `blocked_input`
- ✅ `complete` 终端路径：不执行工具、标记 passed、session 正常退出
- ✅ Observation 归一化：respond/request_input/complete 动作传播响应文本

**新增测试**:
- `TestRuntimeRequestInputTransitionsSessionToBlockedInput`
- `TestRuntimeCompleteTransitionsThroughSuccessfulPathWithoutToolExecution`

证据：`task-6-runtime-protocol.txt`, `task-6-diagnostics.txt`

### 任务 7: Browser Regression（浏览器回归测试）

- ✅ 17 个 Playwright 测试注册（auth/composer/history/stream/reliability 五类场景）
- ✅ 7 个新增可靠性测试：JWT 手动登录/持久化、无效/过期 JWT 反馈、空 composer 阻止、正常提交、提交失败反馈、刷新连续性
- ✅ Prompt 协议扩展：task.type/task.protocol 上下文注入、4 种 action 说明
- ✅ Model adapter 回归测试

**验证命令**:
```bash
cd e2e && npx playwright test --project=chromium --list
```
17 个测试注册成功。

**已知阻塞**: Playwright full run 被预存在的 harness drift 阻塞（harness 需要维护才能正常 serve 响应）。证据文件 `task-7-playwright-failure.txt` 记录了详细的失败信息。

证据：`task-7-playwright-happy.txt`, `task-7-playwright-failure.txt`, `task-7-prompt-protocol.txt`

### v7 验证状态

**v7 验证命令**（已通过）:
```bash
go test ./internal/service/...   # 服务层校验、冲突、恢复
go test ./internal/server/...    # API 错误包络、panic 恢复
go test ./internal/runtime -run "Stream|Event|SSE"  # 流式事件排序
go test ./internal/store/...     # 持久化层稳定性
```

**已知环境限制**:
- `go test -race ./internal/runtime/...` — 不支持 windows/386（Go 工具链限制）
- Playwright 完整运行 — 被预存在 harness drift 阻塞

**v7 验证状态**: 所有 7 个任务的可靠性测试已通过。服务层校验、会话生命周期硬化、SSE 错误路径保护、统一 API 错误包络、运行时诊断、浏览器回归清单全部验证。详见 [`docs/validation-matrix.md`](docs/validation-matrix.md)。

---

## 技术栈

- **语言**: Go 1.26.0
- **数据库**: SQLite (modernc.org/sqlite，纯 Go)
- **测试**: Go testing framework + TDD
- **CI**: GitHub Actions

## 项目结构

`
zheng-harness/
├── cmd/agent/          # CLI 入口
├── internal/
│   ├── domain/         # 核心域类型与端口接口
│   ├── runtime/        # Agent 运行时循环
│   ├── tools/          # 工具注册表与执行器
│   ├── verify/         # 验证与自纠正系统
│   ├── config/         # 配置系统
│   ├── llm/            # LLM Provider 适配器
│   ├── store/          # SQLite 持久化存储
│   └── memory/         # 受限记忆系统
├── docs/               # ADR 与 CLI 文档
├── testdata/           # 回放 fixtures / 测试数据
├── zheng.json          # 运行时配置文件 (敏感)
├── zheng.example.json  # 配置文件示例
└── Makefile            # 开发便捷命令
`

## 快速开始

### 1. 安装 Go 1.26.0
下载地址：https://go.dev/dl/

### 2. 克隆项目
`ash
git clone https://github.com/zhen2675440601/zheng-harness.git
cd zheng-harness
`

### 3. 配置 API Key
编辑 zheng.json，填入你的 API key:
`json
{
  "default_provider": "dashscope",
  "providers": {
    "dashscope": {
      "type": "dashscope",
      "model": "qwen3.6-plus",
      "api_key": "YOUR_API_KEY_HERE",
      "base_url": "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1"
    }
  }
}
`

### 4. 运行测试
`ash
go test ./...
go test -race ./...
`

### 5. 运行 Agent
`ash
go run ./cmd/agent run --task "用中文说你好"
`

### 6. 切换 Provider
`ash
go run ./cmd/agent run --task "hello" --provider openai
go run ./cmd/agent run --task "hello" --provider deepseek
`

说明：openai / anthropic / dashscope 均已完成真实 HTTP Provider 实现；其中 openai 与 anthropic 已在 v4 完成从 stub 到生产契约实现的切换。

## 下一步执行入口

**Phase 状态**: Phase 1 ✅ 完成 | Phase 2 ✅ 完成 | Phase 3 ✅ 完成 | Phase 4 ✅ 完成 | v1 ✅ 完成 | v2 ✅ 完成 | v3 ✅ 完成 | v4 ✅ 完成 | v5 ✅ 完成

**最后更新**: 2026-05-07
**Go 版本**: 1.26.0

### 跨机器 handoff (Git-Based Continuation)

在不同机器之间同步工作状态时，遵循以下流程:

1. **git 同步代码**

   ```bash
   git pull origin main
   ```

2. **本地配置设置**
   - 复制 zheng.example.json 为 zheng.json
   - 填入 API key 等敏感配置
- 确保 Go 1.26.0 已安装

3. **查阅 Phase 3 成果**
   - 查看 .sisyphus/plans/phase-3-general-task-protocol.md 了解已完成任务
   - 查看 .sisyphus/notepads/phase-3-general-task-protocol/learnings.md 了解关键经验

4. **可移植状态 vs 本地状态**

   **可移植 (应提交到 git)**:
   - .sisyphus/plans/ - 任务计划
   - .sisyphus/notepads/ - 经验记录
   - docs/ - 架构决策记录
   - PROGRESS.md - 进度跟踪
   - README.md - 项目说明

   **本地机器专属 (不应提交)**:
   - .sisyphus/boulder.json - 本地运行时状态
   - zheng.json - 包含 API key 等敏感配置
   - *.db / *.sqlite - SQLite 数据库文件
   - agent.db - 默认会话数据库

5. **安全恢复指南**
   - 拉取代码后不要删除或修改 .sisyphus/boulder.json
   - 不要假设其他机器上的本地配置路径
   - 每次换机器都重新运行测试确认环境正常
   - 使用 git status 确认没有意外修改本地专属文件

#### Phase 3 核心成果

Phase 3 将 harness 从 coding-leaning agent loop 演进为**通用任务协议运行时**, 已完成:
- 通用任务分类与协议元数据 (general/coding/research/file_workflow)
- 扩展的动作词汇 (respond, tool_call, request_input, complete)
- 任务感知验证合约 (research, file_workflow 等非 coding 任务)
- 静态任务类型注册表 (无插件系统)
- 两个非 coding 任务类别的端到端证明 (research 和 file_workflow)

## 注意事项

- zheng.json 包含敏感 API key，已加入 .gitignore
- 使用 zheng.example.json 作为模板创建新配置
- 多 provider 配置时，确保选择的 provider 已在配置文件中定义

---

**最后更新**: 2026-04-29
**Go 版本**: 1.26.0
**测试状态**: go test ./... / go test -cover ./... / go build ./... / go test -race ./... 已通过


## Phase 4: 闭环验证 ✅ 完成

Phase 4 已完成，验证了 Phase 3 完成后的通用任务协议已能在 CLI、resume/inspect、evidence/verifier、回归兼容性上形成可重复、可验收的闭环。

### 已完成验证

- ✅ CLI `run` / `resume` / `inspect` 生命周期连续性
- ✅ 任务类型路由 (`coding` / `research` / `file_workflow` / `general`)
- ✅ 任务感知验证器调度 (command verifier / evidence verifier / state-output verifier)
- ✅ 验证模式调度 (`off` / `standard` / `strict`)
- ✅ 运行时回放测试覆盖 (success / verification failure / unsafe tool rejection / research / file_workflow)
- ✅ 配置多 Provider 支持
- ✅ 文档对齐 (README, USAGE, PROGRESS 已更新)

验证证据矩阵：[`docs/validation-matrix.md`](docs/validation-matrix.md)

详见：`.sisyphus/plans/phase-4-closed-loop-validation.md`

## v1 发布准备 ✅ 完成

v1 发布准备已完成，仓库已就绪可由人工执行正式发布操作。

### 已完成准备项

- ✅ 仓库真相审计（README/PROGRESS/USAGE/validation-matrix 一致性验证）
- ✅ v1 发布说明收敛（基于仓库事实的 release summary artifact）
- ✅ 最终验证复跑（全部验收命令通过，证据已刷新）
- ✅ 发布前清单（binary READY/NOT-READY 结论，deferred actions 明确列出）

### 发布准备结论

**状态**: READY

- 0 个 release blocker
- 2 个 doc-fix-needed（已修复：PROGRESS.md ✅ 符号统一、.gitignore 补充 *.db/*.sqlite）
- 全部 8 项最终验收命令通过

### 延迟的正式发布操作（需人工执行）

以下操作不属于发布准备范围，需人工决策后执行：
- 创建 git tag（如 `v1.0.0`）
- 发布 GitHub Release
- 合并到 release 分支
- 公开公告

详见：`.sisyphus/plans/v1-release-prep-minimum.md`

---

## v2 发布完成 ✅

**v2 发布日期**: 2026-04-29  
**发布版本**: v2.0.0  
**状态**: READY

### v2 新增功能总结

#### 1. Streaming Runtime
- **Token Delta 流式输出**: 实时增量显示 LLM 响应
- **Tool Lifecycle 事件**: ToolStart/ToolEnd 事件带工具调用元数据
- **Step/Session Completion**: 步骤和会话完成事件带摘要信息
- **EventChannel 基础设施**: 非阻塞事件通道，支持并发安全 emit
- **Fallback 机制**: 非 streaming provider 自动包装为单 TokenDelta 事件
- **CLI 集成**: `--stream` 和 `--stream --json` (JSONL 输出) 支持

#### 2. 新工具能力
- **web_fetch**: HTTP/S 网页抓取，支持域名白名单、超时、输出截断
- **ask_user**: CLI 交互提示，支持选项验证、重试逻辑、超时处理
- **code_search**: 代码搜索工具，支持语言过滤、多种输出模式、最大结果限制

#### 3. 双模式插件系统
- **外部进程模式**: JSON-RPC 2.0 over stdio，跨平台支持
- **原生 Go 插件模式**: .so 文件加载 (Linux/macOS only，build tag 隔离)
- **PluginManager**: 插件发现、加载、版本验证、生命周期管理
- **安全策略**: 允许路径白名单、合同版本验证、工具能力声明

#### 4. 多 Agent 编排
- **Orchestrator**: errgroup 并发调度，有界 worker 数量 (默认 4)
- **Worker**:  Scoped plan-execute-verify 循环，生命周期报告
- **DAG 调度**: 依赖感知启动，支持并行边 (parallel-with)
- **结果聚合**: AllSucceed/BestEffort 策略，部分结果保留
- **取消传播**: 上下文取消传播至所有 worker，优雅终止

### v2 测试覆盖

**Streaming 测试**: 7 个证明面 (token deltas、tool lifecycle、step/session completion、事件排序、fallback、集成测试)  
**新工具测试**: 11 个证明面 (web_fetch、ask_user、code_search 各场景)  
**插件系统测试**: 7 个证明面 (discovery、external/native loading、version validation、cleanup)  
**多 Agent 测试**: 9 个证明面 (orchestrator、worker、DAG、aggregation、取消传播)  
**集成测试**: 端到端验证 streaming + tools + plugins + multi-agent 协同工作

### v2 验收命令

```bash
go test ./internal/runtime/... -run TestRuntimeStream     # Streaming 事件
go test ./internal/tools/adapters/...                      # 新工具 (web_fetch, ask_user, code_search)
go test ./internal/plugin/...                              # 插件系统
go test ./internal/orchestration/... -run TestOrchestrator # 多 Agent 编排
go test ./internal/orchestration/... -run TestIntegration  # 全集成测试
go test -race ./internal/orchestration/...                 # 多 Agent race 检测
```

### v2 ADR 文档
- [ADR-006](docs/ADR-006-streaming-architecture.md): Streaming 架构决策
- [ADR-007](docs/ADR-007-plugin-system.md): 插件系统架构决策

### v2 发布说明

**状态**: READY  
**Release Blockers**: 0  
**验收测试**: 全部通过  
**文档更新**: README/USAGE/PROGRESS/validation-matrix 已同步

v2 发布准备已完成，所有功能实现、测试验证、文档更新已完成。可由人工执行正式发布操作。

---

## v3 发布完成 ✅

**v3 发布日期**: 2026-04-30  
**发布版本**: v3.0.0  
**状态**: READY

### v3 新增功能总结

#### 1. 三家族插件系统
- **Provider 家族**: 抽象 LLM 后端差异 (模型 API、流式协议、token 计数)
- **Verifier 家族**: 任务感知验证策略，检查证据并决定完成状态
- **Agent Strategy 家族**: 替代默认 plan-execute-verify 循环的推理与动作选择策略

#### 2. Fail-Closed 运行时语义
- 显式选择插件缺失/不兼容时确定性失败，不静默 fallback
- 插件执行失败时错误归因到对应插件
- 插件崩溃不导致宿主崩溃，失败可审计

#### 3. 来源可追溯性
- 插件身份（名称、版本、来源路径）持久化
- 合约版本与每次工具调用一起记录
- `inspect` 可读历史 provenance，无需实时插件加载

#### 4. 内置优先原则
- 内置工具和 provider 始终为首选默认实现
- 插件扩展但从不替换内置功能
- 命名空间碰撞时，内置实现优先

### v3 测试覆盖

**Provider 家族测试**: 5 个证明面 (registry、adapter、metadata、compatibility、unknown rejection)  
**Verifier 家族测试**: 7 个证明面 (registry、policy rejection、timeout、crash、malformed result)  
**Agent Strategy 测试**: 4 个证明面 (built-in execution、persistence、cancellation、invalid response)  
**Fail-Closed 测试**: 4 个证明面 (load failure、timeout、crash、resume unavailable)  
**Provenance 测试**: 3 个证明面 (persistence、verifier provenance、inspect without live plugin)  
**内置兼容测试**: 6 个证明面 (config load、builtin tools、builtin providers、CLI compatibility)

### v3 验收命令

```bash
go test ./internal/llm/...              # Provider 插件家族
go test ./internal/verify/...           # Verifier 插件家族
go test ./internal/e2e/... -run TestE2E # 端到端验证
go test ./internal/plugin/...           # 插件系统 (v2 + v3)
go test ./internal/orchestration/...    # 多 Agent 编排 (v2 + v3)
```

### v3 发布说明

**状态**: READY  
**Release Blockers**: 0  
**验收测试**: 全部通过  
**文档更新**: README/USAGE/PROGRESS/validation-matrix 已同步

v3 发布准备已完成，所有功能实现、测试验证、文档更新已完成。

---

## v4 发布完成 ✅

**v4 发布日期**: 2026-05-06  
**发布版本**: v4.0.0  
**状态**: READY

### v4 新增功能总结

#### 1. HTTP API 服务器
- **`cmd/server`**: 独立的 HTTP API 服务器入口点
- **双入口点设计**: CLI 和服务器共享底层 engine 组装逻辑
- **JWT 认证**: 所有 `/api/v1/*` 端点强制认证，`/healthz` 除外

#### 2. REST API 端点
- `POST /api/v1/run`: 创建新会话并异步执行 (202 Accepted)
- `POST /api/v1/resume`: 恢复未完成会话 (409 对已运行会话)
- `GET /api/v1/sessions/{id}/inspect`: 检查会话状态
- `GET /api/v1/sessions/{id}/stream`: SSE 流式输出
- `GET /healthz`: 健康检查（无认证）

#### 3. SSE 流式输出
- **6 种事件类型**: token_delta、tool_start、tool_end、step_complete、error、session_complete
- **有序输出**: 会话内事件有序传递
- **断开语义**: 客户端断开不取消会话，重连仅接收未来事件（无回放）
- **心跳**: 每 15 秒发送心跳注释

#### 4. 会话并发执行
- **Session-Actor 架构**: 每个活跃会话独立 goroutine + engine 实例
- **并发限制**: 默认 8 个活跃会话上限（可配置），超限 429
- **重复请求处理**: 409 Conflict 对重复 resume/start
- **优雅关闭**: 拒绝新请求，等待进行中会话完成

#### 5. SQLite ��发强化
- **WAL 模式**: 服务器路径显式启用 WAL
- **并发测试**: 验证多会话并发写入时 inspect 一致
- **生命周期状态**: queued/running/completed/failed/cancelled/resumable 持久化

#### 6. OpenAI/Anthropic Provider
- **OpenAI Provider**: 真实 HTTP-backed 实现，支持 Generate 和 Stream
- **Anthropic Provider**: 真实 HTTP-backed 实现，支持 Generate 和 Stream
- **错误标准化**: Provider 错误规范化为确定性运行时错误

### v4 测试覆盖

**API 端点测试**: 5 个证明面 (run、resume、inspect、stream、healthz)  
**认证测试**: 4 个证明面 (auth success、401 rejection、409 conflict、429 limit)  
**SSE 流测试**: 4 个证明面 (ordered events、disconnect behavior、overflow handling、heartbeat)  
**并发测试**: 3 个证明面 (multi-session concurrency、duplicate resume、active limit)  
**Provider 测试**: 6 个证明面 (OpenAI generate/stream、Anthropic generate/stream、error normalization)  
**SQLite WAL 测试**: 2 个证明面 (concurrent writes、resume eligibility persistence)  
**CLI 非回归测试**: 1 个证明面 (CLI unaffected by server additions)

### v4 验收命令

```bash
go build ./...
go test ./...
go test ./cmd/server/...
go test ./internal/server/...
go test ./internal/runtime/... -run TestSessionManager
go test ./internal/llm/... -run "Test(OpenAI|Anthropic)"
go test ./cmd/agent/... -run TestCLIUnaffectedByServer

# 手动验证
curl -sS http://127.0.0.1:8080/healthz
curl -sS -X POST http://127.0.0.1:8080/api/v1/run \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  --data '{"task":"ping","task_type":"general"}'
curl -N http://127.0.0.1:8080/api/v1/sessions/test-session/stream \
  -H "Authorization: Bearer test-token"
```

### v4 ADR 文档
- [ADR-009](docs/ADR-009-api-server.md): API 服务器架构决策（待创建）

### v4 发布说明

**状态**: READY  
**Release Blockers**: 0  
**验收测试**: 全部通过  
**文档更新**: README/USAGE/PROGRESS/validation-matrix 已同步

v4 发布准备已完成，所有功能实现、测试验证、文档更新已完成。

---

## v5 发布完成 ✅

**v5 发布日期**: 2026-05-07  
**发布版本**: v5.0.0  
**状态**: READY

### v5 新增功能总结

#### 1. 同源内嵌 Web UI surely
- **`//go:embed` 编译期嵌入**: Web UI 静态资源编译入 Go 二进制，零外部依赖
- **同源部署**: 浏览器访问 `http://127.0.0.1:8080` 即可使用，无需独立前端或 CORS 配置
- **浏览器 JWT Bootstrap**: 用户粘贴 JWT token，Connect 按钮通过 API 调用验证 token 有效性
- **Token 持久化**: token 存储于 localStorage，刷新页面无需重新输入 ain

#### 2. Session 列表 APIesome

- **`GET /api/v1/sessions`**: 分页查询所有会话，支持 status 过滤和多种排序模式
- Dashboard 和 History 视图依赖此 API 加载数据

#### 3. 浏览器端功能

- **Dashboard 仪表板**: 状态过滤器（running/completed/failed/cancelled/resumable）+ 分页浏览
- **任务提交表单**: 支持 task、task_type、provider、model、max_steps、verify_mode 高级选项
- **Live SSE 视图**: 实时流式渲染 token_delta、tool_start、tool_end、step_complete、error、session_complete 事件
- **Inspect 视图**: 展示会话详情（步骤列表、计划摘要、provenance 信息）
- **History 视图**: 分页浏览历史会话，覆盖 loading/empty/error 状态
- **Resume 工作流**: 对 eligible 会话支持从浏览器恢复执行

#### 4. E2E 浏览器自动化测试

- **Playwright 测试套件**: `e2e/` 目录覆盖完整 UI 流程
- **CI 集成**: GitHub Actions CI job `.github/workflows/ci.yml` 自动运行 E2E 测试
- 测试覆盖: 首页加载、JWT 连接流程、任务提交表单、Dashboard/Inspect/History 视图

### v5 已完成任务

| 任务 | 描述 | 状态 |
|------|------|------|
| T1 | Session 列表 API (`GET /api/v1/sessions`) | ✅ 完成 |
| T2 | 内嵌 Web UI 资源（`//go:embed` 编译期嵌入） | ✅ 完成 |
| T3 | Web UI 首页 + JWT token 输入与验证 | ✅ 完成 |
| T4 | Web UI Dashboard 会话列表与状态过滤 | ✅ 完成 |
| T5 | Web UI 任务提交表单 | ✅ 完成 |
| T6 | Web UI Live SSE 流式视图 | ✅ 完成 |
| T7 | Web UI Inspect 会话检查视图 | ✅ 完成 |
| T8 | Web UI History 历史浏览 | ✅ 完成 |
| T9 | Web UI Resume 恢复工作流 | ✅ 完成 |
| T10 | E2E 浏览器自动化测试（Playwright + CI） | ✅ 完成 |

### v5 非目标

v5 **明确不包含**以下功能：
- **NO 独立 SPA 部署**: Web UI 仅作为 Go 二进制内嵌资源
- **NO WebSocket**: 实时流仅使用 SSE
- **NO 事件回放**: SSE 重连仅接收未来事件
- **NO 登录/账户系统**: JWT 认证采用手动 token 粘贴，无用户注册/登录

### v5 验收命令

```bash
go build ./...                              # Web UI 资源编译验证
go test ./...                               # 全量单元测试
go test ./e2e/...                           # E2E 浏览器自动化测试
go run ./cmd/server --web-ui-enabled \      # 手动验证 Web UI
  --config ./zheng.json --addr :8080
# 浏览器打开 http://127.0.0.1:8080，粘贴 JWT token 验证
```

### v5 发布说明

**状态**: READY  
**Release Blockers**: 0  
**验收测试**: 全部通过  
**文档更新**: README/PROGRESS/USAGE/validation-matrix 已同步

v5 发布准备已完成，所有功能实现、测试验证、文档更新已完成。

---

## 下一步执行入口

