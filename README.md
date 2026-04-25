# zheng-harness

基于 Harness Engineering 思想实现的通用 Coding Agent Go MVP。

v1 聚焦 **CLI-first、单进程、单代理、可验证、可恢复、可检查持久记忆**，避免过早平台化。

## 当前进度

**已完成 11/11 个核心任务 (100%)**

| 状态 | 任务 | 说明 |
|------|------|------|
| ✅ | T1: Bootstrap Go 项目骨架 | Go 模块、目录结构、架构边界 |
| ✅ | T2: 定义核心域契约 | 域类型、端口接口、fake 适配器 |
| ✅ | T3: 实现 plan-execute-verify 循环 | 多步迭代、预算限制、终止状态、自纠正 |
| ✅ | T4: 构建工具注册表 | 工具定义、注册表、执行器、安全策略 |
| ✅ | T5: 建立 TDD/CI 基线 | GitHub Actions、Makefile |
| ✅ | T6: 配置与模型适配器边界 | Config、Provider 接口、Prompt 版本化 |
| ✅ | T7: 实现验证与自纠正系统 | 验证检查、失败分类、纠正指令 |
| ✅ | T8: SQLite 持久化 | Session/Event/Memory 存储 |
| ✅ | T9: CLI 命令 | run / resume / inspect |
| ✅ | T10: 基准测试与回放 | 回归测试套件 |
| ✅ | T11: 文档与 ADR | 架构决策记录、使用文档、contributor workflow |

## 快速开始

### 1. 克隆并进入仓库

```bash
git clone https://github.com/zhen2675440601/zheng-harness.git
cd zheng-harness
```

### 2. 安装依赖环境

- Go 1.22+
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
go run ./cmd/agent run --task "inspect repository and propose next step"
```

### 5. 查看详细使用说明

- CLI 使用文档：[`docs/USAGE.md`](docs/USAGE.md)
- 架构决策记录：[`docs/`](docs)

## CLI 使用示例

### 运行新会话

```bash
go run ./cmd/agent run --task "inspect repository and propose next step"
```

### 指定数据库和 JSON 输出

```bash
go run ./cmd/agent run \
  --task "inspect repository and propose next step" \
  --config ./zheng.json \
  --db ./agent.db \
  --max-steps 8 \
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
  "provider": "dashscope",
  "model": "qwen3.6-plus",
  "api_key": "sk-sp-xxx",
  "base_url": "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1",
  "max_steps": 8,
  "step_timeout": "30s",
  "memory_limit_mb": 256,
  "verify_mode": "standard"
}
```

仓库提供了可复制修改的示例文件：[`zheng.example.json`](zheng.example.json)。

配置优先级为：**CLI flags > 配置文件 > 环境变量 > 默认值**。

### 恢复会话

```bash
go run ./cmd/agent resume --session session-1710000000000000000
```

### 检查会话状态

```bash
go run ./cmd/agent inspect --session session-1710000000000000000 --json
```

默认 SQLite 数据文件位置是当前工作目录下的 `./agent.db`。

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

- **语言**：Go 1.22+
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

## v1 明确不包含

- 多代理编排
- 插件系统或动态加载工具
- Web UI
- Slack / Telegram / Discord 等网关
- 向量数据库、embedding 检索、知识图谱
- 面向 v2 的平台化设计文档

这些边界是有意为之，用于确保 MVP 保持小范围、可测试、可验证。

## ADR 索引

- [ADR-001: Single-Process Single-Agent Runtime](docs/ADR-001-single-process-agent.md)
- [ADR-002: SQLite Persistence and Constrained Memory](docs/ADR-002-sqlite-memory.md)
- [ADR-003: No Plugin System in v1](docs/ADR-003-no-plugin-system.md)
- [ADR-004: No Vector Database for MVP Memory](docs/ADR-004-no-vector-db.md)
- [ADR-005: Test-Driven Development First](docs/ADR-005-tdd-first.md)

## 许可证

MIT
