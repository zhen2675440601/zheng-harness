package runtime

import (
	"context"
	"testing"

	"zheng-harness/internal/domain"
)

func TestListToolInfoReturnsRegistryData(t *testing.T) {
	t.Parallel()

	engine := Engine{
		Tools: toolExecutorWithRegistryStub{
			infos: []domain.ToolInfo{{Name: "grep_search", Description: "search", Schema: "{}"}},
		},
	}

	tools := engine.listToolInfo()
	if len(tools) != 1 {
		t.Fatalf("listToolInfo len = %d, want 1", len(tools))
	}
	if got := tools[0].Name; got != "grep_search" {
		t.Fatalf("tool name = %q, want grep_search", got)
	}
}

func TestListToolInfoHandlesMissingRegistry(t *testing.T) {
	t.Parallel()

	if got := (Engine{}).listToolInfo(); got != nil {
		t.Fatalf("listToolInfo = %#v, want nil without registry", got)
	}

	engine := Engine{Tools: toolExecutorWithRegistryStub{}}
	if got := engine.listToolInfo(); got != nil {
		t.Fatalf("listToolInfo = %#v, want nil with nil registry", got)
	}
}

type toolExecutorWithRegistryStub struct{ infos []domain.ToolInfo }

func (t toolExecutorWithRegistryStub) Execute(_ context.Context, _ domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{}, nil
}

func (t toolExecutorWithRegistryStub) Registry() toolInfoLister {
	if t.infos == nil {
		return nil
	}
	return toolInfoListerStub{infos: t.infos}
}

type toolInfoListerStub struct{ infos []domain.ToolInfo }

func (t toolInfoListerStub) ListToolInfo() []domain.ToolInfo {
	return append([]domain.ToolInfo(nil), t.infos...)
}
