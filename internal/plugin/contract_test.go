package plugin

import (
	"context"
	"errors"
	"testing"

	"zheng-harness/internal/domain"
)

func TestContractVersionMismatch(t *testing.T) {
	t.Parallel()

	tool := stubPluginTool{version: "0.9.0"}

	err := ValidateContract(tool)
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
	if !errors.Is(err, ErrContractVersionMismatch) {
		t.Fatalf("expected ErrContractVersionMismatch, got %v", err)
	}
}

func TestPluginToolInterfaceCompliance(t *testing.T) {
	t.Parallel()

	var tool PluginTool = stubPluginTool{}

	if err := ValidateContract(tool); err != nil {
		t.Fatalf("ValidateContract() error = %v", err)
	}
	if got := tool.Name(); got != "stub" {
		t.Fatalf("Name() = %q, want %q", got, "stub")
	}
	if got := tool.Description(); got != "stub plugin tool" {
		t.Fatalf("Description() = %q, want %q", got, "stub plugin tool")
	}
	if got := tool.Schema(); got != `{"type":"object"}` {
		t.Fatalf("Schema() = %q, want %q", got, `{"type":"object"}`)
	}
	if got := tool.Capabilities(); len(got) != 1 || got[0] != "filesystem.read" {
		t.Fatalf("Capabilities() = %v, want [filesystem.read]", got)
	}
	if got := tool.SafetyLevel(); got != domain.SafetyLevelLow {
		t.Fatalf("SafetyLevel() = %q, want %q", got, domain.SafetyLevelLow)
	}
	if got := tool.ContractVersion(); got != ContractVersion {
		t.Fatalf("ContractVersion() = %q, want %q", got, ContractVersion)
	}

	result, err := tool.Execute(context.Background(), domain.ToolCall{Name: tool.Name(), Input: "payload"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ToolName != "stub" {
		t.Fatalf("Execute().ToolName = %q, want %q", result.ToolName, "stub")
	}
	if result.Output != "payload" {
		t.Fatalf("Execute().Output = %q, want %q", result.Output, "payload")
	}
	if err := tool.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

type stubPluginTool struct {
	version string
}

func (s stubPluginTool) Name() string {
	return "stub"
}

func (s stubPluginTool) Description() string {
	return "stub plugin tool"
}

func (s stubPluginTool) Schema() string {
	return `{"type":"object"}`
}

func (s stubPluginTool) Capabilities() []string {
	return []string{"filesystem.read"}
}

func (s stubPluginTool) SafetyLevel() domain.SafetyLevel {
	return domain.SafetyLevelLow
}

func (s stubPluginTool) ContractVersion() string {
	if s.version != "" {
		return s.version
	}
	return ContractVersion
}

func (s stubPluginTool) Execute(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{
		ToolName: call.Name,
		Output:   call.Input,
	}, nil
}

func (s stubPluginTool) Close() error {
	return nil
}
