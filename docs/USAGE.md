# CLI Usage

`zheng-agent` 是一个 **通用 Agent Harness** CLI。当前版本支持 coding、research、file workflow 等多种任务类型，并支持实时 streaming 输出、resume/inspect 会话连续性，以及受约束的内置工具集。

本文档描述已验证的 CLI 契约。验证证据见 [`validation-matrix.md`](validation-matrix.md)。

## 命令概览

```bash
zheng-agent run --task "inspect repository and propose next step"
zheng-agent resume --session <id>
zheng-agent inspect --session <id>

# streaming mode
zheng-agent run --task "inspect repository and propose next step" --stream
zheng-agent resume --session <id> --stream
```

默认 SQLite 数据库文件位置为当前工作目录下的 `./agent.db`。

## 先决条件

- Go 1.26.0
- 可写的本地文件系统（用于 `agent.db`）
- 已克隆本仓库

## 构建与测试

```bash
go test ./...
go test -race ./...
go test -cover ./...
```

如果仓库配置了 Makefile，也可使用：

```bash
make test
make test-race
make test-cover
```

## `zheng-agent run --task "..."`

启动一个新会话并执行单代理 plan-execute-verify 循环。

### 基本示例

```bash
go run ./cmd/agent run --task "inspect repository and propose next step" --task-type coding
```

### 常用参数

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

参数说明：

- `--task`：必填，任务描述
- `--task-type`：可选，任务类型 (`coding`, `research`, `file_workflow`, `general`)，默认为 `general`
- `--config`：可选，JSON 配置文件路径；默认按 `./zheng.json`、`~/.zheng/config.json` 顺序查找
- `--db`：SQLite 文件路径，默认 `./agent.db`
- `--max-steps`：最大步数，必须大于 0
- `--json`：输出机器可读 JSON
- `--stream`：启用实时事件流输出；文本模式下增量打印 token/tool/step 事件，JSON 模式下输出 JSONL

### Task-Type 验证策略

不同任务类型使用不同的验证器：

- `coding`: 运行 `go test`, `go build`, `go vet` 验证代码变更
- `research`: 验证主张是否有引用的证据支持
- `file_workflow`: 检查是否产生了预期的文件产物
- `general`: 基于证据的通用验证（默认）

验证模式通过 `--verify-mode` 控制（`off`, `standard`, `strict`），详见配置章节。

### 文本输出

`run` 会输出：

- Session ID
- 当前状态
- 原始任务描述
- 计划摘要
- 已记录步数

启用 `--stream` 后，CLI 会立即消费运行时事件流：

- `token_delta`：直接增量打印模型文本
- `tool_start` / `tool_end`：打印工具生命周期
- `step_complete`：打印步骤边界
- `error`：打印到 stderr
- `session_complete`：打印最终 session 状态

### JSON 输出

启用 `--json` 后，会输出类似：

```json
{
  "command": "run",
  "session_id": "session-1710000000000000000",
  "status": "success",
  "task_input": "inspect repository and propose next step",
  "plan": "Inspect the repository and summarize the next action.",
  "steps": 3
}
```

### Streaming JSONL 输出

当 `--stream --json` 同时启用时，`run` 和 `resume` 输出为 **JSON Lines**，每行一个 `StreamingEvent`：

```json
{"type":"token_delta","step_index":1,"payload":{"content":"hello"},"timestamp":"2026-04-28T09:00:00Z"}
{"type":"tool_start","step_index":1,"payload":{"tool_name":"code_search","input":"{\"pattern\":\"RunStream\"}"},"timestamp":"2026-04-28T09:00:01Z"}
{"type":"tool_end","step_index":1,"payload":{"tool_name":"code_search","output":"internal/runtime/runtime.go","error":""},"timestamp":"2026-04-28T09:00:01Z"}
{"type":"step_complete","step_index":1,"payload":{"step_summary":"validated runtime stream path"},"timestamp":"2026-04-28T09:00:02Z"}
{"type":"session_complete","payload":{"session_id":"session-1710000000000000000","status":"success"},"timestamp":"2026-04-28T09:00:02Z"}
```

说明：

- token/tool 等中间 streaming 事件 **不会持久化到 SQLite**
- 仅最终 `Session` / `Plan` / `Step` 状态会持久化
- `resume --stream` 会基于已持久化状态继续执行，并仅 stream 剩余过程
- `inspect` 对 streaming session 的输出格式与非 streaming session 完全一致

## `zheng-agent resume --session <id>`

从 SQLite 中恢复一个已有会话。如果该会话已经结束，命令会直接打印当前状态；如果还未结束，则继续执行运行时循环。

### 基本示例

```bash
go run ./cmd/agent resume --session session-1710000000000000000
```

### 带参数示例

```bash
go run ./cmd/agent resume \
  --session session-1710000000000000000 \
  --db ./agent.db \
  --stream \
  --max-steps 8
```

参数说明：

- `--session`：必填，会话 ID
- `--db`：SQLite 文件路径，默认 `./agent.db`
- `--max-steps`：恢复后继续运行时允许的最大步数，必须大于 0
- `--stream`：对恢复后的剩余执行过程启用实时流式输出

`resume` 的文本输出会展示：

- 会话 ID
- 当前状态
- 计划摘要
- 最近的步骤历史摘要

如果恢复的 session 已经终止，`resume --stream` 不会重新播放旧的 token 事件，而是直接输出当前持久化状态。

## `zheng-agent inspect --session <id>`

只读取并展示已有会话状态，不继续执行。

### 基本示例

```bash
go run ./cmd/agent inspect --session session-1710000000000000000
```

### JSON 模式

```bash
go run ./cmd/agent inspect --session session-1710000000000000000 --json
```

参数说明：

- `--session`：必填，会话 ID
- `--db`：SQLite 文件路径，默认 `./agent.db`
- `--json`：输出机器可读 JSON

`inspect` 输出包含：

- Session ID
- 当前状态
- 终止原因
- 计划摘要
- 步数统计
- 最近步骤摘要

对于 streaming session，`inspect` 仍然只展示持久化后的最终 plan/step/session 结果，不展示中间 token delta 事件。

## 内置工具

当前 CLI runtime 默认注册以下内置工具：

- `read_file` / `write_file` / `edit_file` / `list_dir`
- `glob`
- `grep_search`
- `code_search`：带语言过滤的代码搜索
- `ask_user`：在 CLI 中向用户提问并等待输入
- `web_fetch`：执行受约束的 HTTP/HTTPS 页面抓取
- `exec_command`：执行 allowlisted 本地命令

### `code_search`

用于在工作区内搜索源码，支持语言过滤与不同输出模式。

输入 schema：

```json
{
  "pattern": "RunStream",
  "language": "go",
  "output_mode": "content",
  "max_results": 20
}
```

- `pattern`：必填，搜索模式
- `language`：可选，语言过滤
- `output_mode`：可选，`content` / `files_with_matches` / `count`
- `max_results`：可选，默认 50

### `ask_user`

用于在 CLI 运行过程中请求人工输入。

输入 schema：

```json
{
  "question": "Which config file should I update?",
  "options": ["README.md", "docs/USAGE.md", "both"]
}
```

- `question`：必填，向用户展示的问题
- `options`：可选，候选项列表

### `web_fetch`

用于抓取 HTTP/HTTPS 页面内容。

输入 schema：

```json
{
  "url": "https://example.com",
  "max_length": 4000
}
```

- `url`：必填，仅支持 `http` / `https`
- `max_length`：可选，返回内容最大字符数，默认 `10000`

### `web_fetch` 域名 allowlist

`web_fetch` 受安全策略约束，支持按域名 allowlist 限制访问范围。

- 当 `AllowedDomains` 为空时：允许任意 HTTP/HTTPS 域名
- 当 `AllowedDomains` 非空时：仅允许精确匹配的 hostname
- 不在 allowlist 内的域名会返回：`web_fetch domain "<host>" is not allowed`

当前默认执行器会将 `AllowedDomains` 初始化为空列表；如果你在自定义 runtime 装配 `tools.SafetyPolicy`，可以设置：

```go
tools.SafetyPolicy{
  AllowedDomains: []string{"example.com", "docs.example.com"},
}
```

## 环境变量配置

运行时配置支持以下优先级：**CLI flags > 环境变量 > 配置文件 > 默认值**。

## 配置文件

推荐将 LLM provider 的敏感配置放入 JSON 配置文件，而不是直接写在命令行参数中。

默认查找路径：

1. 当前工作目录下的 `./zheng.json`
2. 用户目录下的 `~/.zheng/config.json`

也可以通过 `--config <path>` 显式指定。

示例：

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

仓库示例文件：[`../zheng.example.json`](../zheng.example.json)

运行示例：

```bash
go run ./cmd/agent run \
  --task "inspect repository and propose next step" \
  --config ./zheng.json
```

如果同时提供配置文件和命令行参数，命令行参数会覆盖配置文件中的同名项。

运行时配置可通过环境变量覆盖：

- `ZHENG_MODEL`
- `ZHENG_PROVIDER`
- `ZHENG_MAX_STEPS`
- `ZHENG_STEP_TIMEOUT`
- `ZHENG_MEMORY_LIMIT_MB`
- `ZHENG_VERIFY_MODE`
- `ZHENG_API_KEY`
- `ZHENG_BASE_URL`

含义如下：

- `ZHENG_MODEL`：模型标识
- `ZHENG_PROVIDER`：Provider，当前支持 `openai`、`anthropic`、`dashscope`
- `ZHENG_MAX_STEPS`：默认最大执行步数
- `ZHENG_STEP_TIMEOUT`：单步超时，例如 `30s`
- `ZHENG_MEMORY_LIMIT_MB`：内存预算（MB）
- `ZHENG_VERIFY_MODE`：验证模式，支持 `off`、`standard`、`strict`
- `ZHENG_API_KEY`：LLM provider API key
- `ZHENG_BASE_URL`：LLM provider API base URL

示例：

```bash
export ZHENG_PROVIDER=openai
export ZHENG_MODEL=gpt-4.1-mini
export ZHENG_MAX_STEPS=8
export ZHENG_STEP_TIMEOUT=30s
export ZHENG_MEMORY_LIMIT_MB=256
export ZHENG_VERIFY_MODE=standard
export ZHENG_API_KEY=sk-sp-xxx
export ZHENG_BASE_URL=https://coding.dashscope.aliyuncs.com/apps/anthropic/v1
```

## SQLite 数据位置

- 默认位置：当前工作目录 `./agent.db`
- 可通过 `--db <path>` 指向其他 SQLite 文件

建议：

- 本地开发时把数据库放在仓库根目录，便于调试
- CI 或临时验证时使用临时路径，避免污染长期会话数据

## contributor workflow

新增文档、工具或运行时行为时，推荐按以下流程操作：

1. 先写或更新失败测试
2. 实现最小变更
3. 运行 `go test ./...`
4. 运行 `go test -race ./...`
5. 必要时运行 `go test -cover ./...`
6. 使用 `go run ./cmd/agent ...` 手动检查 CLI 行为
7. 更新 README、ADR 或使用文档，确保 v1 边界仍然清晰

## v1 明确不包含

- 多代理编排
- 插件系统或动态工具加载
- Web UI
- Slack / Telegram / Discord 等网关接入
- 向量数据库、embedding 检索、知识图谱
- 面向 v2 的平台化文档
