# CLI Usage

`zheng-agent` 是一个 **通用 Agent Harness** CLI。v1 支持 coding、research、file workflow 等多种任务类型，每种任务类型路由到不同的验证器。

本文档描述已验证的 CLI 契约。验证证据见 [`validation-matrix.md`](validation-matrix.md)。

## 命令概览

```bash
zheng-agent run --task "inspect repository and propose next step"
zheng-agent resume --session <id>
zheng-agent inspect --session <id>
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
  --json
```

参数说明：

- `--task`：必填，任务描述
- `--task-type`：可选，任务类型 (`coding`, `research`, `file_workflow`, `general`)，默认为 `general`
- `--config`：可选，JSON 配置文件路径；默认按 `./zheng.json`、`~/.zheng/config.json` 顺序查找
- `--db`：SQLite 文件路径，默认 `./agent.db`
- `--max-steps`：最大步数，必须大于 0
- `--json`：输出机器可读 JSON

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
  --max-steps 8
```

参数说明：

- `--session`：必填，会话 ID
- `--db`：SQLite 文件路径，默认 `./agent.db`
- `--max-steps`：恢复后继续运行时允许的最大步数，必须大于 0

`resume` 的文本输出会展示：

- 会话 ID
- 当前状态
- 计划摘要
- 最近的步骤历史摘要

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
