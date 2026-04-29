// Package domain 包含通用 agent harness 的核心业务概念与契约。
//
// 归属：该包树定义稳定的内部模型，必须与基础设施关注点保持独立。
//
// 约束：internal/domain 下的包不得导入基础设施层或接口层包，例如 internal/store、internal/tools、
// internal/runtime、cmd/agent 或未来的 transport/adapters。依赖只能向内指向，
// 以确保领域逻辑保持可移植且可测试。
package domain
