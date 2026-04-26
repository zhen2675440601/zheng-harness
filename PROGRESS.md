# zheng-harness 项目进度记录

## 项目概述

基于 Harness Engineering 思想实现的通用 Agent Harness Go MVP。

## 当前进度

**Phase 1 ✅ 完成 | Phase 2 ✅ 完成 | Phase 3 📋 计划已就绪**

**已完成 T1-T11 (11/11 核心任务) + 后续功能**

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

### 后续新增功能 (未编号)

#### 1. 阿里百炼 API 集成
- **文件**: `internal/llm/dashscope.go`
- **说明**: 实现真正的 HTTP API 调用，连接阿里百炼 DashScope
- **状态**: ✅ 完成
- **测试**: 单元测试与适配链路验证通过（实网调用依赖有效 API Key）

#### 2. 配置文件支持
- **文件**: `internal/config/config.go`
- **说明**: 支持 JSON 配置文件，加载优先级：CLI > 环境变量 > 配置文件 > 默认值
- **状态**: ✅ 完成

#### 3. 多厂商 API 配置
- **文件**: `internal/config/config.go`, `zheng.json`
- **说明**: 支持在配置文件中配置多个 provider，可动态切换
- **配置格式**:
  ```json
  {
    "default_provider": "dashscope",
    "providers": {
      "dashscope": { "type": "dashscope", "model": "...", "api_key": "...", "base_url": "..." },
      "openai": { "type": "openai", "model": "...", "api_key": "...", "base_url": "..." },
      "deepseek": { "type": "openai", "model": "...", "api_key": "...", "base_url": "..." }
    },
    "runtime": { "max_steps": 8, "step_timeout": "30s", ... }
  }
  ```
- **CLI 使用**:
  - `--provider dashscope` 或 `--provider openai` 切换 provider
- **状态**: ✅ 完成

#### 4. Model Adapter
- **文件**: `internal/runtime/model_adapter.go`
- **说明**: 将 llm.Provider 适配为 domain.Model 接口，使 CLI 可使用真实 LLM
- **状态**: ✅ 完成

## 遇到的问题及解决方案

### 1. Go 环境变量问题
- **问题**: Windows 环境下 Go 不在 PATH 中
- **解决**: 使用完整路径 `D:\zwlword\go\bin\go.exe`

### 2. CLI 参数冲突
- **问题**: config.Load 解析了 --task 等子命令参数，导致 "flag provided but not defined" 错误
- **解决**: 添加 `filterConfigArgs` 函数过滤出只保留 config 相关的参数

### 3. LLM 返回 JSON 被 Markdown 包裹
- **问题**: DashScope 返回的 JSON 被 ```json ... ``` 包裹，解析失败
- **解决**: 在 `decodeJSONResponse` 函数中添加 markdown 代码块移除逻辑

### 4. 多 provider 配置验证问题
- **问题**: CLI 指定不存在的 provider 时，系统自动创建空 provider 导致验证通过
- **解决**: 修改 `upsertSelectedProvider` 和 Load 函数，不再自动创建新 provider

### 5. Provider 与验证器运行时接线不一致
- **问题**: CLI 早期仅对 DashScope 使用真实 provider，且默认总是使用 `FakeVerifier`，导致 `verify_mode` 行为不完整
- **解决**:
  - `cmd/agent/cli.go` 改为对所有受支持 provider 统一走 `llm.NewProvider + runtime.NewModelAdapter`
  - 新增 `newVerifierFromConfig`，按 `verify_mode` 选择 verifier（off/standard/strict）
  - 为 run/resume/inspect 补齐 `--verify-mode` 等配置相关 flag 兼容

## 技术栈

- **语言**: Go 1.26.0
- **数据库**: SQLite (modernc.org/sqlite，纯 Go)
- **测试**: Go testing framework + TDD
- **CI**: GitHub Actions

## 项目结构

```
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
```

## 快速开始

### 1. 安装 Go 1.26.0
下载地址: https://go.dev/dl/

### 2. 克隆项目
```bash
git clone https://github.com/zhen2675440601/zheng-harness.git
cd zheng-harness
```

### 3. 配置 API Key
编辑 `zheng.json`，填入你的 API key：
```json
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
```

### 4. 运行测试
```bash
go test ./...
go test -race ./...
```

### 5. 运行 Agent
```bash
go run ./cmd/agent run --task "用中文说你好"
```

### 6. 切换 Provider
```bash
go run ./cmd/agent run --task "hello" --provider openai
go run ./cmd/agent run --task "hello" --provider deepseek
```

说明：`openai` / `anthropic` 当前为 stub provider（用于边界与流程验证），`dashscope` 为真实 HTTP 适配实现。

## Git 操作记录

### 初始化 (已在其他主机完成)
```bash
git init
git add .
git commit -m "feat: initial commit"
git remote add origin https://github.com/zhen2675440601/zheng-harness.git
git push -u origin main
```

### Phase 1-2 提交历史
```bash
# 多 provider LLM 支持与 DashScope 集成
git add -A
git commit -m "feat: add multi-provider LLM support with DashScope integration"
git push origin main
```

## 下一步执行入口

**Phase 状态**: Phase 1 ✅ 完成 | Phase 2 ✅ 完成 | Phase 3 📋 计划已就绪

Phase 3 计划文件：`.sisyphus/plans/phase-3-general-task-protocol.md`

### 跨机器继续开发

在其他机器上继续 Phase 3 实施：

1. **git 同步代码**
   ```bash
   git pull origin main
   ```

2. **本地配置设置**
   - 复制 `zheng.example.json` 为 `zheng.json`
   - 填入 API key 等敏感配置
   - 确保 Go 1.26.0 已安装

3. **查阅 Phase 3 计划**
   - 打开 `.sisyphus/plans/phase-3-general-task-protocol.md`
   - 从 Wave 1 Task 1 开始实施

### Phase 3 核心目标

将当前 harness 从 coding-leaning agent loop 演进为**通用任务协议运行时**，支持：
- 通用任务分类与协议元数据
- 扩展的动作词汇（request_input, complete 等）
- 任务感知验证合约（research, file_workflow 等非 coding 任务）
- 静态任务类型注册表（无插件系统）

**至少两个非 coding 任务类别的端到端证明**：research 和 file_workflow。

## 注意事项

- `zheng.json` 包含敏感 API key，已加入 `.gitignore`
- 使用 `zheng.example.json` 作为模板创建新配置
- 多 provider 配置时，确保选择的 provider 已在配置文件中定义

---

**最后更新**: 2026-04-26
**Go 版本**: 1.26.0
**测试状态**: `go test ./...` / `go test -cover ./...` / `go build ./...` / `go test -race ./...` 已通过
