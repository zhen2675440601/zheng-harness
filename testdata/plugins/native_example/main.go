package main

import (
	"context"

	"zheng-harness/internal/domain"
	pluginpkg "zheng-harness/internal/plugin"
)

const nativeExampleSchema = `{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}`

type nativeExamplePlugin struct{}

func (nativeExamplePlugin) Name() string {
	return "native_example"
}

func (nativeExamplePlugin) Description() string {
	return "echoes the provided input via native Go plugin"
}

func (nativeExamplePlugin) Schema() string {
	return nativeExampleSchema
}

func (nativeExamplePlugin) Capabilities() []string {
	return []string{"filesystem.read"}
}

func (nativeExamplePlugin) SafetyLevel() domain.SafetyLevel {
	return domain.SafetyLevelLow
}

func (nativeExamplePlugin) ContractVersion() string {
	return pluginpkg.ContractVersion
}

func (nativeExamplePlugin) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{
		ToolName: "native_example",
		Output:   call.Input,
	}, nil
}

func (nativeExamplePlugin) Close() error {
	return nil
}

func NewPluginTool() pluginpkg.PluginTool {
	return nativeExamplePlugin{}
}
