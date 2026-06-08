# zheng-harness

基于 Harness Engineering 思想实现的 **通用 Agent Harness Engine** Go MVP。当前版本已完成通用任务协议扩展，并新增 streaming CLI 输出、新工具能力、插件系统与多 Agent 编排支持。

**v1** 聚焦 **CLI-first、单进程、单代理、可验证、可恢复、可检查持久记忆**。  
**v2** 新增 **streaming 实时输出、3 个新工具、双模式插件系统、多 Agent 编排**。  
**v3** 新增 **三家族插件系统 (provider/verifier/agent-strategy)、fail-closed 运行时、来源可追溯**。
**v4** 新增 **HTTP API 服务器、SSE 流式输出、会话并发执行、OpenAI/Anthropic 真实 Provider**。
**v5** 新增 **同源内嵌 Web UI、Session 列表 API、浏览器 JWT Bootstrap、Dashboard 与 Live SSE 观看**。
**v6** 新增 **Chat-First 对话式 Web UI、六层 Harness 架构对齐、聊天 API 端点**。  
**v7** 新增 **内部可靠性强化：服务层校验、会话生命周期硬化工、SSE 错误路径保护、统一 API 错误包络、最小诊断、浏览器回归清单**。  
**v8** 新增 **插件生命周期管理：热重载、调用前健康检查、自动恢复（有限重试）、语义版本兼容性**。

**定位**: 通用任务执行引擎，支持 coding、research、file workflow 等多种任务类型。

## 当前进度

**Phase 1 ✅ 完成 | Phase 2 ✅ 完成 | Phase 3 ✅ 完成 | Phase 4 ✅ 完成 | v2 ✅ 完成 | v3 ✅ 完成 | v4 ✅ 完成 | v5 ✅ 完成 | v6 ✅ 完成 | v7 ✅ 完成 | v8 🔄 进行中**

核心任务 T1-T11 已全部完成，Phase 3 通用任务协议任务 (T1-T12) 已完成，Phase 4 闭环验证已完成；v2 (Wave 2) streaming runtime、新工具、插件系统、多 Agent 编排已全部完成并验证；**v3 三家族插件系统、fail-closed 语义、fail-closed 运行时已验证**；**v4 HTTP API 服务器、SSE 流式输出、会话并发执行、OpenAI/Anthropic 真实 Provider 已验证**；**v5 Web UI 同源内嵌、Session 列表 API、浏览器 JWT Bootstrap、Dashboard/Live SSE/Inspect 视图、E2E 浏览器自动化测试已验证**；**v6 聊天优先 Web UI、六层架构对齐、聊天 API 端点已验证**；**v7 内部可靠性发布：服务层校验、会话生命周期硬化工、SSE 错误保护、统一 API 错误包络、最小诊断、浏览器回归清单已验证**；**v8 插件生命周期管理：热重载、健康检查、自动恢复、语义版本兼容性 🔄 进行中**。详细进度请见 [PROGRESS.md](PROGRESS.md)。

验证状态见 [`docs/validation-matrix.md`](docs/validation-matrix.md)。

## v2 新增特性

### 1. Streaming 实时输出
- 增量 token delta 显示，实时查看 LLM 响应
- Tool lifecycle 事件：工具调用开始/结束可视化
- Step/Session completion 事件带摘要信息
- CLI `--stream` 与 `--stream --json` (JSONL) 支持
- 非 streaming provider 自动 fallback 包装

### 2. 新工具能力
- **web_fetch**: HTTP/S 网页抓取，域名白名单安全策略
- **ask_user**: CLI 交互提示，支持选项验证与重试
- **code_search**: 语言感知代码搜索，多种输出模式

### 3. 双模式插件系统
- **外部进程**: JSON-RPC 2.0 over stdio，跨平台
- **原生 Go 插件**: .so 文件加载 (Linux/macOS)
- 插件发现、版本验证、生命周期管理
- 安全策略：路径白名单、能力声明

### 4. 多 Agent 编排
- Orchestrator-worker 架构
- DAG 依赖感知调度
- 有界并发控制 (默认 4 worker)
- 结果聚合策略：AllSucceed/BestEffort
- 取消传播与部分结果保留

## v3 新增特性 (Extensibility)

### 1. 三家族插件系统

v3 定义了三个独立的插件家族，每个家族有自己的合约和加载语义：

- **Provider 家族**: 抽象 LLM 后端差异 (模型 API、流式协议、token 计数)
- **Verifier 家族**: 任务感知验证策略，检查证据并决定完成状态
- **Agent Strategy 家族**: 替代默认 plan-execute-verify 循环的推理与动作选择策略

每个插件家族支持双模式加载：
- **外部进程**: JSON-RPC 2.0 over stdio，跨平台
- **原生 Go 插件**: .so 文件加载 (Linux/macOS)

### 2. Fail-Closed 运行时语义

所有插件家族遵循 **fail-closed** 语义：
- 当用户或持久化会话**显式选择**某个插件 provider / verifier / agent-strategy，而该插件缺失、版本不兼容或校验失败时，`run` / `resume` 会**确定性失败**，不会静默 fallback 到内置实现
- 当未显式选择插件时，运行时仍默认使用内置实现；内置路径始终可用，零插件安装即可工作
- 插件执行失败时，错误会被归因到对应插件并传达给宿主验证/持久化路径；运行时不会隐式切换到其他实现来掩盖错误
- 插件崩溃不会导致宿主崩溃；失败会被记录，并保留可检查的 provenance 与历史

### 3. 来源可追溯性

所有插件调用都可审计：
- 插件身份（名称、版本、来源路径）在会话历史中记录
- 合约版本与每次工具调用一起持久化
- 插件导致的失败包含插件身份的错误消息

`inspect` 会直接展示已持久化的 provenance，即使原始插件二进制已不存在；`resume` 则会根据持久化 provenance 执行 fail-closed 校验，并在缺失/不匹配时返回确定性错误。

### 4. 内置优先原则

- 内置工具和 provider 始终为首选默认实现
- 插件扩展但从不替换内置功能
- 命名空间碰撞时，内置实现优先，插件被拒绝

## v4 新增特性 (API Server)

### 1. HTTP API 服务器

v4 新增独立的 HTTP API 服务器入口点，与 CLI 并行存在：

- **`cmd/server`**: 独立的 HTTP API 服务器二进制
- **`cmd/agent`**: 保持原有的 CLI 入口点，行为不变
- **双入口点设计**: CLI 和服务器共享底层 engine 组装逻辑

### 2. 认证 REST 端点

所有 API 端点需要 JWT 认证（`/healthz` 除外）：

- `POST /api/v1/run`: 创建新会话并异步执行
- `POST /api/v1/resume`: 恢复未完成的会话
- `GET /api/v1/sessions/{id}/inspect`: 检查会话状态（无需活跃 actor）
- `GET /api/v1/sessions/{id}/stream`: SSE 流式输出运行时事件
- `GET /healthz`: 健康检查（无需认证）

### 3. SSE 流式输出

服务器通过 SSE (Server-Sent Events) 输出 6 种运行时事件：

- `token_delta`: 增量模型输出
- `tool_start` / `tool_end`: 工具调用生命周期
- `step_complete`: 步骤完成事件
- `error`: 错误事件
- `session_complete`: 会话完成事件

**断开语义**: 客户端断开 SSE 连接不会取消会话执行；重连仅接收未来事件（v4 无事件回放）。

### 4. 会话并发执行

- **Session-Actor 架构**: 每个活跃会话拥有独立的 goroutine 和 engine 实例
- **并发限制**: 默认最多 8 个活跃会话（可配置），超出返回 429
- **重复请求处理**: 对已运行会话的重复 resume 请求返回 409 Conflict
- **优雅关闭**: 关闭时拒绝新请求，等待进行中会话完成或取消

### 5. SQLite 并发强化

- **WAL 模式**: 服务器路径显式启用 WAL 模式以支持并发写入
- **并发测试**: 验证多会话并发写入时 inspect 仍可读取一致状态
- **生命周期状态持久化**: 区分 queued/running/completed/failed/cancelled/resumable 状态

### 6. OpenAI/Anthropic Provider 实现

v4 完成了真实的 HTTP-backed provider 实现：

- **OpenAI Provider**: 支持 `Generate` 和 `Stream` 路径
- **Anthropic Provider**: 支持 `Generate` 和 `Stream` 路径
- **错误标准化**: Provider 错误规范化为确定性运行时错误
- **配置兼容性**: 保持现有的配置优先级和选择语义

### 7. 安全与 Fail-Closed

- **JWT 认证**: 所有 `/api/v1/*` 端点强制 JWT 认证
- **Fail-Closed**: 插件缺失/超时/崩溃时返回确定性错误，不静默 fallback
- **会话隔离**: 多会话并发执行时不共享可变状态

## v5 新增特性 (Web UI)

### 1. 同源内嵌 Web UI.header
v5 在 Go 二进制中内嵌了完整的 Web UI，通过 `//go:embed` 编译期嵌入，零外部依赖：

- **同源部署**: Web UI 与服务端同一端口，无需独立前端部署或 CORS 配置
- **浏览器 JWT Bootstrap**: 用户粘贴 JWT token，Connect 按钮通过 API 调试验证 token 有效性
- **Token 持久化**: token 存储于浏览器 localStorage，刷新页面后无需重新输入

### 2. Session 列表 API

- **`GET /api/v1/sessions`**: 分页查询所有会话，支持 status 过滤和多种排序模式
- 用于 Dashboard 概览和 History 视图分页加载

### 3. Dashboard 仪表板

- **状态过滤器**: 按 running/ completed/ failed/ cancelled/ resumable 筛选
- **分页浏览**: 按页加载会话列表
- **实时统计**: 展示各状态会话数量

### 4. 任务提交与 Live SSE 流

- **任务提交表单**: 浏览器内创建新会话，支持 task_type/ provider/ model/ max_steps/ verify_mode 高级选项
- **Live SSE 视图**: 打开会话后实时流式显示 token_delta/ tool_start/ tool_end/ step_complete/ error/ session_complete 事件
- **会话恢复**: 对 eligible 会话支持 resume 工作流

### 5. 检查与历史视图

- **Inspect 视图**: 展示会话详情，包含步骤列表、计划摘要、provenance 信息
- **History 视图**: 分页浏览历史所有会话，支持 loading/ empty/ error 状态

### 6. E2E 浏览器自动化测试

- **Playwright 测试套件**: `e2e/` 目录中覆盖完整 UI 流程
- **CI 集成**: GitHub Actions CI job 自动运行 E2E 测试
- 测试覆盖: 首页加载、JWT 连接、任务提交表单、Dashboard/Inspect/History 视图

### v5 非目标

以下功能 **不在** v5 范围内：
- **NO 独立 SPA 部署**: Web UI 仅作为 Go 二进制内嵌资源，不单独部署
- **NO WebSocket**: 实时流仅使用 SSE（Server-Sent Events）
- **NO 事件回放**: SSE 重连仅接收未来事件
- **NO 登录/账户系统**: JWT 认证采用手动 token 粘贴方式，无用户注册/登录

## v6 新增特性 (Chat-First UI + Six-Layer Alignment)

### 1. 聊天优先 Web UI

v6 将 v5 的任务提交式 Dashboard 改造为 **对话式聊天工作区** 作为 Web 主入口：

- **Chat 工作区主入口**: 浏览器打开后默认进入聊天界面，发起对话式请求，而不是 "New Task" 表单
- **流式对话回复**: 聊天消息实时流式渲染 SSE token_delta，工具调用以可折叠行内卡片展示
- **会话侧栏**: 左侧历史会话列表，支持切换/继续/resume 已有会话
- **结构化 Inspect**: 聊天内可查看步骤详情、计划摘要、provenance，无需离开聊天界面
- **同源内嵌**: 沿用 `//go:embed` 编译期嵌入，零外部前端依赖

### 2. 六层 Harness 架构对齐

v6 将服务端边界收敛为 Harness Engineering 六层基线，职责清晰、依赖单向：

| Layer | 职责 | 对应包 |
|-------|------|--------|
| **Types** | 领域契约、事件定义、接口声明 | `internal/domain` |
| **Config** | 配置加载、验证、多 provider 解析 | `internal/config` |
| **Repo** | 持久化适配（SQLite）、会话/计划/步骤 CRUD | `internal/persistence` |
| **Service** | 聊天用例（conversation/session/message/stream） | `internal/service` |
| **Runtime** | 引擎编排、工具执行、plan-execute-verify 循环 | `internal/runtime` |
| **UI** | HTTP transport、SSE streaming、内嵌 Web 资源 | `internal/server`, `cmd/server` |

**依赖方向**: UI → Service → Runtime → Repo → Types，Config 被各层依赖。UI/transport 不直接承载业务编排决策；核心聊天用例可脱离 HTTP handler 被调用。

### 3. 聊天 API 端点

v6 新增 chat-first API 端点，与现有 REST API 共存：

- `POST /api/v1/chat`: 创建新会话并发送首条消息，返回 session_id
- `POST /api/v1/chat/{session_id}`: 向已有会话追加消息，触发 agent 响应
- `GET /api/v1/chat/{session_id}/stream`: SSE 流式监听聊天响应事件
- `GET /api/v1/conversations`: 分页列出所有对话历史щих

### v6 非目标

- **NO 独立 SPA 构建系统**: 保持同源内嵌，默认不引入 React/Vue 构建流程
- **NO 扩展产品功能**: v6 仅改造 UX 和架构边界，不新增 tool/provider/verifier 能力
- **NO 破坏 v5 API**: 现有 `POST /api/v1/run`、`GET /api/v1/sessions` 等端点保持可用

## v7 新增特性 (Internal Reliability Release)

v7 是 **内部可靠性发布**，不扩展新能力，专注于将 v6 的 chat-first Web UI + six-layer backend 打磨为可依赖的内部稳定版本。所有改动均遵循 TDD：先补失败用例，再实施稳定性改进。

### 1. 服务层校验硬化

- **task_type 校验**: `StartConversation()` 和 `StartChat()` 拒绝不支持的 task_type，返回 `ValidationError`
- **缺失会话处理**: 提交回复到不存在的会话时返回 `NotFoundError`，消息中包含缺失资源 ID
- **运行中冲突**: 正在运行的会话不接受新的提交回复，返回 `ConflictError`
- **终止会话拒绝**: 已完成/失败的会话拒绝 resume，返回 `ConflictError`

### 2. 会话生命周期硬化

- **订阅 fail-closed**: 已终结的 actor 拒绝新订阅，防止幽灵监听
- **Nil runner 提升**: Nil runner factory 被显式提升为 actor 失败，确保运行状态与实际执行一致
- **Relay 安全拆除**: `sessionEventRelay` 拆除后新订阅被确定性拒绝，不注册到死 relay

### 3. SSE 流式错误路径保护

- **结构化错误包络**: 所有错误路径使用统一的 `writeStructuredError` 包络：`{"error":{"code":"...","message":"..."},"request_id":"..."}`
- **5 种错误码**: unauthorized (401), invalid_request (400), not_found (404), conflict (409), too_many_requests (429), internal_error (500)
- **缺失会话 SSE 包络**: 流式请求缺失会话时返回结构化 `not_found` 包络，包含缺失的 session ID
- **Panic 安全恢复**: handler panic 永远不暴露原始值到客户端，统一返回 "internal server error"

### 4. 运行时协议诊断

- **运行时协议元数据解析**: Runtime 通过静态 task registry 解析协议元数据，在 plan 创建、action 选择、observation 处理、验证/终止决策中使用
- **request_input 终端路径**: 被确定性视为阻塞输入终端：不执行工具，标记 `not_applicable` 验证状态，会话转为 `blocked_input`
- **complete 终端路径**: 被显式视为成功终端：不执行工具，标记 `passed` 验证状态，会话正常成功退出

### 5. Prompt 协议扩展

- **任务上下文注入**: 在 prompt payload 中添加 `task.type` 和 `task.protocol` 上下文
- **扩展 action 说明**: `next_action` JSON 指令覆盖 respond, tool_call, request_input, complete 四种类型
- **去除编码假设**: 移除 action 选择说明中的编码任务假设

### 6. 浏览器回归清单

- **17 个 Playwright 测试注册**: 覆盖 auth/composer/history/stream/reliability 五类场景
- **可靠性子套件**: 新增 7 个可靠性测试：JWT 手动登录/持久化、无效/过期 JWT 反馈、空 composer 阻止提交、正常聊天提交、提交失败反馈、刷新保持会话连续性

### v7 非目标

- **NO 外部发布/GA**: v7 仅面向内部稳定使用，不适用于对外 polished 发布
- **NO 多租户**: 不引入租户隔离、配额系统或复杂权限模型
- **NO 完整可观测性平台**: 仅做排障所需的最小诊断入口
- **NO 长期记忆升级**: memory 层保持 v6 范围
- **NO 插件生态扩展**: 不新增插件类型或市场机制
- **NO 事件回放/重连历史**: SSE 重连仅接收未来事件，v7 不实现回放

## v8 新增特性 (Plugin Lifecycle Management)

v8 专注于 Tool 插件的运行时可靠性，不涉及 Provider/Verifier/AgentStrategy 插件家族。

### 1. 热重载 (Hard Reload)

- **`PluginManager.ReloadTool(name)`**: 关闭现有插件实例，重新发现并加载插件二进制，重新执行 JSON-RPC 握手
- **API 端点**: `POST /api/v1/plugins/{name}/reload` — HTTP 触发重载
- **CLI 命令**: `zheng-agent tool reload <plugin-name> [--plugin-dir ./plugins]`
- **安全**: 仅手动触发，不自动重载；不影响现有插件实例直至显式触发

### 2. 调用前健康检查

- 每次 `ExternalPluginTool.Execute()` 调用前检查插件进程存活状态
- 使用可配置超时（默认 5s）
- 进程不响应时返回 `HealthCheckError`

### 3. 自动恢复 (有限重试)

- 插件崩溃后自动重启一次
- 连续第二次失败后标记插件为 "unavailable"
- 成功执行后重置失败计数
- 避免无限循环或恢复风暴

### 4. 语义版本兼容性

- 首次加载时验证插件合同版本的主版本一致性
- 主版本一致（如 1.x 兼容 1.y）则接受
- 主版本不一致（如期望 1.x 但收到 2.x）则拒绝

### v8 非目标

- **NO 扩展到其他插件家族**: Provider/Verifier/AgentStrategy 保持不变
- **NO 自动定期重载**: 仅手动触发
- **NO 复杂状态迁移或增量重载**: 仅硬重载
- **NO 插件市场或分发系统**

## 快速开始

### 1. 克隆并进入仓库

```bash
git clone https://github.com/zhen2675440601/zheng-harness.git
cd zheng-harness
```

### 2. 安装依赖环境

- Go 1.26.0
- 本地可写文件系统（用于 SQLite 数据库文件）

### 3. 运行测试

```bash
go test ./...
go test -race ./...
go test -cover ./...
```

如果使用 Makefile：

```bash
make test
make test-race
make test-cover
```

### 4. 启动 CLI

```bash
go run ./cmd/agent run --task "inspect repository and propose next step" --task-type coding

# streaming mode
go run ./cmd/agent run --task "inspect repository and propose next step" --task-type coding --stream
```

Supported `--task-type` values: `coding`, `research`, `file_workflow`, `general`. Each routes to a task-specific verifier.

### 4b. 启动 API 服务器 (v4+)

```bash
# 启动 HTTP API 服务器
go run ./cmd/server --config ./zheng.json --addr :8080

# 健康检查
curl -sS http://127.0.0.1:8080/healthz

# 运行新会话
curl -sS -X POST http://127.0.0.1:8080/api/v1/run \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"task":"inspect repository and propose next step","task_type":"coding"}'

# SSE 流式输出
curl -N -H "Authorization: Bearer YOUR_TOKEN" \
  http://127.0.0.1:8080/api/v1/sessions/<session-id>/stream

# 检查会话状态
curl -sS -H "Authorization: Bearer YOUR_TOKEN" \
  http://127.0.0.1:8080/api/v1/sessions/<session-id>/inspect
```

API 服务器需要 JWT 认证配置。详见 [`docs/USAGE.md`](docs/USAGE.md) API 服务器章节。

### 4c. 启动 Web UI (v5+, v6 chat-first)

```bash
# 启动 API 服务器并启用 Web UI (v6 默认聊天工作区)
go run ./cmd/server --config ./zheng.json --addr :8080 --web-ui-enabled

# 浏览器访问
open http://127.0.0.1:8080

# v6: 粘贴 JWT token 完成认证后，直接进入聊天工作区进行对话式交互
# v5: 粘贴 JWT token 到页面右上角输入框，点击 Connect 连接即可使用
```

v6 Web UI 提供**聊天工作区**作为主入口，支持对话式交互、流式回复、会话切换/继续、结构化 inspect 和历史浏览。v5 Dashboard 和任务提交功能作为辅助视图保留。详细使用说明见 [`docs/USAGE.md`](docs/USAGE.md) v6 Chat UI 章节。

### 5. 查看详细使用说明

- CLI 使用文档：[`docs/USAGE.md`](docs/USAGE.md)
- 架构决策记录：[`docs/`](docs)

## CLI 使用示例

### 运行新会话

```bash
go run ./cmd/agent run --task "inspect repository and propose next step" --task-type coding

# JSONL streaming output
go run ./cmd/agent run --task "inspect repository and propose next step" --task-type coding --stream --json
```

The `--task-type` flag selects the verification strategy:
- `coding`: Runs `go test`, `go build`, `go vet` to validate changes
- `research`: Validates that claims are backed by cited evidence
- `file_workflow`: Checks that expected file artifacts were produced
- `general`: Falls back to evidence-based verification

The `--verify-mode` flag controls verification strictness:
- `off`: Uses FakeVerifier, always passes if final response exists
- `standard`: Uses TaskAwareVerifier dispatched by task type (default)
- `strict`: Uses TaskAwareVerifier with strict policy

### 指定数据库和 JSON 输出

```bash
go run ./cmd/agent run \
  --task "inspect repository and propose next step" \
  --task-type coding \
  --config ./zheng.json \
  --db ./agent.db \
  --max-steps 8 \
  --stream \
  --json
```

### 使用配置文件管理 Provider 凭据

推荐将 API key、base URL 等敏感配置写入 JSON 配置文件，而不是直接放在命令行参数中。

`zheng-agent` 默认会按以下顺序查找配置文件：

1. `./zheng.json`
2. `~/.zheng/config.json`

也支持通过 `--config <path>` 显式指定：

```json
{
  "default_provider": "dashscope",
  "providers": {
    "dashscope": {
      "type": "dashscope",
      "model": "qwen3.6-plus",
      "api_key": "sk-sp-xxx",
      "base_url": "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1"
    },
    "openai": {
      "type": "openai",
      "model": "gpt-4.1-mini",
      "api_key": "sk-xxx",
      "base_url": "https://api.openai.com/v1"
    }
  },
  "runtime": {
    "max_steps": 8,
    "step_timeout": "30s",
    "memory_limit_mb": 256,
    "verify_mode": "standard"
  }
}
```

仓库提供了可复制修改的示例文件：[`zheng.example.json`](zheng.example.json)。

配置优先级为：**CLI flags > 环境变量 > 配置文件 > 默认值**。

对于 provider 选择：

- `--provider <built-in-id>` 用于选择内置 provider
- `--plugin-provider <plugin-id>` 用于选择插件 provider
- 若配置文件中的 `plugin_provider` 与 CLI 的 `--provider` / `--plugin-provider` 冲突，CLI 会直接报错，不会隐式兜底

### 恢复会话

```bash
go run ./cmd/agent resume --session session-1710000000000000000

# resume remaining work with streaming
go run ./cmd/agent resume --session session-1710000000000000000 --stream
```

### 检查会话状态

```bash
go run ./cmd/agent inspect --session session-1710000000000000000 --json
```

若该会话使用过插件，`inspect` 输出会包含持久化的 `provenance`；这一步不依赖实时加载插件，因此历史始终可读。

默认 SQLite 数据文件位置是当前工作目录下的 `./agent.db`。

Streaming mode uses runtime events for token deltas, tool lifecycle updates, step boundaries, and final session completion. Intermediate streaming events are not persisted; only final session/plan/step state is stored for `resume` and `inspect` continuity.

## 当前架构概览

项目采用明确的 **Domain / Runtime / Infrastructure / Interface** 分层。

### 核心边界

- `internal/domain`：核心类型与端口接口
- `internal/runtime`：单代理 plan-execute-verify 循环
- `internal/tools`：工具注册表、schema、超时与安全级别、执行器
- `internal/verify`：验证与自纠正策略
- `internal/store`：SQLite 持久化（session、steps、memory 等）
- `internal/memory`：受限记忆策略与规则
- `internal/config`：配置加载与环境变量覆盖
- `internal/llm`：模型 Provider 边界
- `cmd/agent`：CLI 入口与 `run` / `resume` / `inspect` 契约

## 内置工具能力

当前运行时默认提供以下内置工具：

- 文件工具：`list_dir`, `read_file`, `write_file`, `edit_file`
- 搜索工具：`glob`, `grep_search`, `code_search`
- 交互工具：`ask_user`
- Web 工具：`web_fetch`
- 本地命令工具：`exec_command`

其中：

- `code_search` 支持语言过滤与多种输出模式
- `ask_user` 支持 CLI 中断点式人工输入
- `web_fetch` 支持 HTTP/HTTPS 抓取，并可通过安全策略配置域名 allowlist

### 核心端口

`internal/domain/ports.go` 中定义了运行时依赖的核心接口：

- `Model`：负责计划生成、下一步动作选择、观察总结
- `ToolExecutor`：执行已批准的工具调用
- `MemoryStore`：持久化可检查观察结果
- `SessionStore`：持久化 session、plan、step 历史
- `Verifier`：基于证据判断任务是否完成

这意味着运行时依赖接口，而不是依赖具体基础设施实现。

## Harness Engineering 核心原则

1. **Constrain**：通过工具注册表、安全级别、allowlist 约束代理行为
2. **Inform**：通过结构化 task / plan / step / observation 提供明确上下文
3. **Verify**：通过独立验证器与证据检查确认完成状态
4. **Correct**：验证失败时给出有边界的纠正路径，而不是无限重试

## 技术栈

- **语言**：Go 1.26.0
- **持久化**：SQLite（`modernc.org/sqlite`，纯 Go）
- **测试**：Go testing framework + TDD
- **CI**：GitHub Actions

## 项目结构

```text
zheng-harness/
├── cmd/agent/          # CLI 入口
├── internal/
│   ├── domain/         # 核心域类型与端口接口
│   ├── runtime/        # Agent 运行时循环
│   ├── tools/          # 工具注册表与执行器
│   ├── verify/         # 验证与自纠正系统
│   ├── config/         # 配置系统
│   ├── llm/            # LLM Provider 适配器边界
│   ├── store/          # SQLite 持久化存储
│   └── memory/         # 受限记忆系统
├── docs/               # ADR 与 CLI 文档
├── testdata/           # 回放 fixtures / 测试数据
├── .github/workflows/  # CI 配置
└── Makefile            # 开发便捷命令
```

## contributor workflow

建议新贡献者遵循以下顺序：

1. 阅读本 README 和 [`docs/USAGE.md`](docs/USAGE.md)
2. 阅读 `internal/domain/ports.go` 理解核心边界
3. 先写失败测试，再实现功能
4. 运行 `go test ./...`
5. 运行 `go test -race ./...`
6. 必要时运行 `go test -cover ./...`
7. 用 `go run ./cmd/agent ...` 手动验证 CLI 行为
8. 如果改动影响架构边界或使用方式，更新 `README.md` / `docs/` / ADR

## 当前仍不包含

- Slack / Telegram / Discord 等网关
- 向量数据库、embedding 检索、知识图谱
- 插件市场 / 发现服务
- WebSocket 传输 (v5 仅支持 SSE)
- 递归 Agent（深度=1 为上限）

**v2 已实现**: 多代理编排 (orchestrator-worker)、插件系统 (双模式：外部进程 + 原生 Go 插件)、streaming 输出、新工具 (web_fetch, ask_user, code_search)。

**v3 已实现**: 三家族插件系统 (provider/verifier/agent-strategy)、fail-closed 运行时、来源可追溯性、内置优先原则。

**v4 已实现**: HTTP API 服务器、SSE 流式输出、会话并发执行、OpenAI/Anthropic 真实 Provider、SQLite WAL 模式、JWT 认证。

**vNominator 已实现**: 同源内嵌 Web UI、Session 列表 API、浏览器 JWT Bootstrap、Dashboard/Live SSE/Inspect/History 视图、E2E 浏览器自动化测试。

**vNominate 已实现**: Same-origin embedded Web UI, session list API (GET /api/v1/sessions), browser JWT bootstrap UX, task submission/resume from browser, live SSE stream view, dashboard/inspect history views, Playwright browser automation tests.

## ADR 索引

- [ADR-001: Single-Process Single-Agent Runtime](docs/ADR-001-single-process-agent.md)
- [ADR-002: SQLite Persistence and Constrained Memory](docs/ADR-002-sqlite-memory.md)
- [ADR-003: No Plugin System in v1](docs/ADR-003-no-plugin-system.md)
- [ADR-004: No Vector Database for MVP Memory](docs/ADR-004-no-vector-db.md)
- [ADR-005: Test-Driven Development First](docs/ADR-005-tdd-first.md)
- [ADR-006: Streaming Runtime Architecture](docs/ADR-006-streaming-architecture.md)
- [ADR-007: Plugin System Architecture](docs/ADR-007-plugin-system.md)
- [ADR-008: v3 Extensibility Boundaries and Plugin Family Contracts](docs/ADR-008-v3-extensibility.md)
- [ADR-009: v4 API Server Productization](docs/ADR-009-v4-api-server-productization.md)
- [ADR-010: v5 Same-Origin Embedded Web UI](docs/ADR-010-v5-web-ui.md)

## 许可证

MIT
