package plugin

import (
	"context"
	"errors"
	"fmt"

	"zheng-harness/internal/domain"
)

// ContractVersion 标识当前工具插件契约版本。
const ContractVersion = "1.0.0"

// PluginTool 描述工具插件需要实现的最小契约。
type PluginTool interface {
	Name() string
	Description() string
	Schema() string
	Capabilities() []string
	SafetyLevel() domain.SafetyLevel
	ContractVersion() string
	Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error)
	Close() error
}

// ErrContractVersionMismatch 表示插件声明的契约版本与宿主不兼容。
var ErrContractVersionMismatch = errors.New("plugin contract version mismatch")

// ValidateContract 确认插件声明的契约版本与当前宿主一致。
func ValidateContract(tool PluginTool) error {
	if tool == nil {
		return errors.New("plugin tool is nil")
	}

	if version := tool.ContractVersion(); version != ContractVersion {
		return fmt.Errorf("%w: plugin=%q expected=%q", ErrContractVersionMismatch, version, ContractVersion)
	}

	return nil
}
