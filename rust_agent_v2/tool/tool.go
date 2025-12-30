package tool

import (
	"context"
	"encoding/json"
	"sync"
)

// Tool 工具接口
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Run(ctx context.Context, input string) (string, error)
}

// Registry 工具注册表
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// ToModelTools 转换为模型工具定义
func (r *Registry) ToModelTools() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []map[string]any
	for _, t := range r.tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  t.InputSchema(),
			},
		})
	}
	return tools
}

// ParseInput 解析输入
func ParseInput(input string) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return nil, err
	}
	return args, nil
}

// FormatOutput 格式化输出
func FormatOutput(data any) string {
	bytes, _ := json.Marshal(data)
	return string(bytes)
}
