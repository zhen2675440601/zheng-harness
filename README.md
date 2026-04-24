# zheng-harness

基于 Harness Engineering 思想的通用 Coding Agent (Go MVP)

## 当前进度

**已完成 7/11 个核心任务 (63%)**

| 状态 | 任务 | 说明 |
|------|------|------|
| ✅ | T1: Bootstrap Go 项目骨架 | Go 模块、目录结构、架构边界 |
| ✅ | T2: 定义核心域契约 | 域类型、端口接口、fake 适配器 |
| ✅ | T3: 实现 plan-execute-verify 循环 | 多步迭代、预算限制、终止状态、自纠正 |
| ✅ | T4: 构建工具注册表 | 工具定义、注册表、执行器、安全策略 |
| ✅ | T5: 建立 TDD/CI 基线 | GitHub Actions、Makefile |
| ✅ | T6: 配置与模型适配器边界 | Config、Provider 接口、Prompt 版本化 |
| ✅ | T7: 实现验证与自纠正系统 | 验证检查、失败分类、纠正指令 |

## 待完成任务

| 状态 | 任务 | 说明 |
|------|------|------|
| ⏳ | T8: SQLite 持久化 | Session/Event/Memory 存储 |
| ⏳ | T9: CLI 命令 | run/resume/inspect |
| ⏳ | T10: 基准测试与回放 | 回归测试套件 |
| ⏳ | T11: 文档与 ADR | 架构决策记录 |

## 技术栈

- **后端**: Go 1.22+
- **测试**: Go testing framework (TDD)
- **CI/CD**: GitHub Actions
- **��久化**: SQLite (待实现)

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
│   ├── store/          # 持久化存储 (待实现)
│   └── memory/         # 记忆系统 (待实现)
├── .github/workflows/  # CI 配置
├── Makefile            # 开发便捷命令
└── testdata/           # 测试数据
```

## 快速开始 (换电脑后)

```bash
# 1. 克隆仓库
git clone https://github.com/zhen2675440601/zheng-harness.git
cd zheng-harness

# 2. 安装 Go (1.22+)
# https://go.dev/doc/install

# 3. 验证项目编译
go test ./...

# 4. 查看当前任务进度
cat .sisyphus/plans/general-agent-harness-go.md | head -120
```

## 核心概念

### Harness Engineering 四大核心功能

1. **Constrain (约束)** - 限制 Agent 能做什么
2. **Inform (告知)** - 告诉 Agent 应该做什么  
3. **Verify (验证)** - 检查 Agent 做对了什么
4. **Correct (纠正)** - 修正 Agent 的错误

### 架构原则

- **Domain/Runtime/Infrastructure/Interface** 分层
- **端口接口** 模式 (不依赖具体实现)
- **TDD** 开发 (先写测试，再实现)
- **显式验证** (不是模型说完成就算完成)

## 继续开发

### 下一步: T8 - SQLite 持久化

计划文件: `.sisyphus/plans/general-agent-harness-go.md`

主要任务:
- 实现 `internal/store/session.go` - Session 存储
- 实现 `internal/store/memory.go` - Memory 存储
- 创建 SQLite schema
- 实现 Session resume 功能

```bash
# 查看 T8 详细要求
cat .sisyphus/plans/general-agent-harness-go.md | grep -A 50 "8. Implement SQLite"
```

### 运行测试

```bash
# 所有测试
go test ./...

# 带竞态检测
go test -race ./...

# 覆盖率
go test -cover ./...

# 本地便捷命令
make test
make test-race
make test-cover
```

## 注意事项

- 当前环境 **未安装 Go**，无法运行测试
- 所有测试需在安装 Go 1.22+ 的环境中运行
- 代码已通过逻辑审查，但未实际编译验证

## 许可证

MIT