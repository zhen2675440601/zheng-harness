package tools

import (
	"fmt"
	"sort"
	"sync"

	"zheng-harness/internal/domain"
)

// Registry 存储内置工具定义。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]ToolDefinition
}

// NewRegistry 构造一个空的工具注册表。
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]ToolDefinition)}
}

// Register 添加工具定义，并拒绝重复或不完整的条目。
func (r *Registry) Register(def ToolDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("tool name must not be empty")
	}
	if def.Description == "" {
		return fmt.Errorf("tool %q description must not be empty", def.Name)
	}
	if def.Schema == "" {
		return fmt.Errorf("tool %q schema must not be empty", def.Name)
	}
	if def.DefaultTimeout <= 0 {
		return fmt.Errorf("tool %q timeout must be greater than zero", def.Name)
	}
	if def.Handler == nil {
		return fmt.Errorf("tool %q handler must not be nil", def.Name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tool %q already registered", def.Name)
	}
	r.tools[def.Name] = def
	return nil
}

// Get 按名称查找工具定义。
func (r *Registry) Get(name string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.tools[name]
	return def, ok
}

// List 按稳定名称顺序返回工具定义。
func (r *Registry) List() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]ToolDefinition, 0, len(r.tools))
	for _, def := range r.tools {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

// ListToolInfo 按稳定名称顺序返回面向提示词的工具定义。
func (r *Registry) ListToolInfo() []domain.ToolInfo {
	defs := r.List()
	infos := make([]domain.ToolInfo, 0, len(defs))
	for _, def := range defs {
		infos = append(infos, domain.ToolInfo{
			Name:        def.Name,
			Description: def.Description,
			Schema:      def.Schema,
		})
	}
	return infos
}
