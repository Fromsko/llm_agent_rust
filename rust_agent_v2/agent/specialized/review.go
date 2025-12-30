package specialized

import (
	"context"
	"fmt"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// ReviewAgent 代码审查 Agent
type ReviewAgent struct {
	name  string
	model model.Model
	tools *tool.Registry
}

func NewReviewAgent(m model.Model, tools *tool.Registry) *ReviewAgent {
	return &ReviewAgent{name: "review-agent", model: m, tools: tools}
}

func (a *ReviewAgent) Name() string { return a.name }

const reviewPrompt = `你是一个专业的 Rust 代码审查专家。

审查维度：
1. 代码正确性 - 逻辑是否正确
2. 内存安全 - 所有权、借用是否正确
3. 错误处理 - Result/Option 使用是否恰当
4. 性能 - 是否有不必要的克隆、分配
5. 可读性 - 命名、结构是否清晰
6. 惯用性 - 是否符合 Rust 惯用写法
7. 文档 - 注释和文档是否充分

输出格式：
## 审查结果

### 问题列表
- [严重程度] 问题描述 (行号)

### 改进建议
1. 具体建议

### 总体评分
- 正确性: X/10
- 安全性: X/10
- 性能: X/10
- 可读性: X/10`

func (a *ReviewAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		messages := []*model.Message{
			model.NewSystemMessage(reviewPrompt),
			model.NewUserMessage(fmt.Sprintf("请审查以下 Rust 代码：\n%s", input)),
		}

		var toolDefs []*model.ToolDef
		if a.tools != nil {
			for _, t := range a.tools.List() {
				toolDefs = append(toolDefs, &model.ToolDef{
					Type: "function",
					Function: &model.Function{
						Name:        t.Name(),
						Description: t.Description(),
						Parameters:  t.InputSchema(),
					},
				})
			}
		}

		resp, err := a.model.Generate(ctx, messages, model.WithTools(toolDefs...))
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
			return
		}

		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, resp.Content))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{"review": resp.Content}))
	}()

	return eventChan, nil
}
