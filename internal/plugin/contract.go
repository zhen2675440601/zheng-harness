package plugin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

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

// ParseVersion 解析 MAJOR.MINOR.PATCH 版本字符串。
func ParseVersion(version string) (major, minor, patch int, err error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid semantic version %q", version)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid semantic version %q: %w", version, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid semantic version %q: %w", version, err)
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid semantic version %q: %w", version, err)
	}

	return major, minor, patch, nil
}

// CheckVersionCompat 检查插件与宿主契约版本是否兼容。
func CheckVersionCompat(pluginVersion, hostVersion string) (compatible bool, err error) {
	pluginMajor, _, _, err := ParseVersion(pluginVersion)
	if err != nil {
		return false, err
	}
	hostMajor, _, _, err := ParseVersion(hostVersion)
	if err != nil {
		return false, err
	}

	return pluginMajor == hostMajor, nil
}

// ValidateContract 确认插件声明的契约版本与当前宿主兼容。
func ValidateContract(tool PluginTool) error {
	if tool == nil {
		return errors.New("plugin tool is nil")
	}

	version := tool.ContractVersion()
	compatible, err := CheckVersionCompat(version, ContractVersion)
	if err != nil {
		return err
	}
	if !compatible {
		return fmt.Errorf("%w: plugin=%q expected=%q", ErrContractVersionMismatch, version, ContractVersion)
	}

	return nil
}
