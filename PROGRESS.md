# zheng-harness 项目进度记录

## 项目概述

基于 Harness Engineering 思想实现的通用 Agent Harness Go MVP。

## 当前进度

**Phase 1 ✅ 完成 | Phase 2 ✅ 完成 | Phase 3 ✅ 完成 | Phase 4 ✅ 完成**

**已完成 T1-T11 (11/11 核心任务) + Phase 3 T1-T12 + T12 文档收尾 + Phase 4 闭环验证 + v1 发布准备 + v2 Wave 2 T11-T13**

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

### 后续新增功能 (未编号)

#### 1. 阿里百炼 API 集成
- **文件**: ~~internal/llm/dashscope.go~~
- **说明**: 实现真正的 HTTP API 调用，连接阿里百炼 DashScope
- **状态**: ✅ 完成
- **测试**: 单元测试与适配链路验证通过（实网调用依赖有效 API Key）

#### 2. 配置文件支持
- **文件**: internal/config/config.go
- **说明**: 支持 JSON 配置文件，加载优先级：CLI > 环境变量 > 配置文件 > 默认值
- **状态**: ✅ 完成

#### 3. 多厂商 API 配置
- **文件**: internal/config/config.go, zheng.json
- **说明**: 支持在配置文件中配置多个 provider，可动态切换
- **配置格式**:

#### 4. Model Adapter
- **文件**: internal/runtime/model_adapter.go
- **说明**: 将 llm.Provider 适配为 domain.Model 接口，使 CLI 可使用真实 LLM
- **状态**: ✅ 完成

#### 5. 跨机器 handoff 协议
- **说明**: 基于 git 的可移植状态同步机制
- **文档**: PROGRESS.md Git-Based Continuation 章节
- **状态**: ✅ 完成
  `json
  {
    "default_provider": "dashscope",
    "providers": {
      "dashscope": { "type": "dashscope", "model": "...", "api_key": "...", "base_url": "..." },
      "openai": { "type": "openai", "model": "...", "api_key": "...", "base_url": "..." },
      "deepseek": { "type": "openai", "model": "...", "api_key": "...", "base_url": "..." }
    },
    "runtime": { "max_steps": 8, "step_timeout": "30s", ... }
  }
  `
- **CLI 使用**:
  - --provider dashscope 或 --provider openai 切换 provider
- **状态**: ✅ 完成

#### 4. Model Adapter
- **文件**: internal/runtime/model_adapter.go
- **说明**: 将 llm.Provider 适配为 domain.Model 接口，使 CLI 可使用真实 LLM
- **状态**: ✅ 完成

## 遇到的问题及解决方案

### 1. Go 环境变量问题
- **问题**: Windows 环境下 Go 不在 PATH 中
- **解决**: 使用完整路径 D:\zwlword\go\bin\go.exe

### 2. CLI 参数冲突
- **问题**: config.Load 解析了 --task 等子命令参数，导致 "flag provided but not defined" 错误
- **解决**: 添加 ilterConfigArgs 函数过滤出只保留 config 相关的参数

### 3. LLM 返回 JSON 被 Markdown 包裹
- **问题**: DashScope 返回的 JSON 被 \\\json ... \\\ 包裹，解析失败
- **解决**: 在 decodeJSONResponse 函数中添加 markdown 代码块移除逻辑

### 4. 多 provider 配置验证问题
- **问题**: CLI 指定不存在的 provider 时，系统自动创建空 provider 导致验证通过
- **解决**: 修改 upsertSelectedProvider 和 Load 函数，不再自动创建新 provider

### 5. Provider 与验证器运行时接线不一致
- **问题**: CLI 早期仅对 DashScope 使用真实 provider，且默认总是使用 FakeVerifier，导致 erify_mode 行为不完整
- **解决**:
  - cmd/agent/cli.go 改为对所有受支持 provider 统一走 llm.NewProvider + runtime.NewModelAdapter
  - 新增 
ewVerifierFromConfig，按 erify_mode 选择 verifier（off/standard/strict）
  - 为 run/resume/inspect 补齐 --verify-mode 等配置相关 flag 兼容

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

说明：openai / nthropic 当前为 stub provider（用于边界与流程验证），dashscope 为真实 HTTP 适配实现。

## Git 操作记录

### 初始化 (已在其他主机完成)
`ash
git init
git add .
git commit -m "feat: initial commit"
git remote add origin https://github.com/zhen2675440601/zheng-harness.git
git push -u origin main
`

### Phase 1-2 提交历史
`ash
# 多 provider LLM 支持与 DashScope 集成
git add -A
git commit -m "feat: add multi-provider LLM support with DashScope integration"
git push origin main
`

## 下一步执行入口

**Phase 状态**: Phase 1 ✅ 完成 | Phase 2 ✅ 完成 | Phase 3 ✅ 完成 | Phase 4 ✅ 完成 | v1 发布准备 ✅ 完成

所有 Phase 4 闭环验证已完成，v1 发布准备已完成。仓库已就绪，可由人工执行正式发布操作。

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

**最后更新**: 2026-04-27
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
