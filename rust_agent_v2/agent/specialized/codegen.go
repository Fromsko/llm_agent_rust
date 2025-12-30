package specialized

import (
	"context"
	"fmt"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// CodeGenAgent 代码生成 Agent
type CodeGenAgent struct {
	name  string
	model model.Model
	tools *tool.Registry
}

func NewCodeGenAgent(m model.Model, tools *tool.Registry) *CodeGenAgent {
	return &CodeGenAgent{name: "codegen-agent", model: m, tools: tools}
}

func (a *CodeGenAgent) Name() string { return a.name }

const codeGenPrompt = `你是一个专业的 Rust 代码生成专家。

你的职责：
1. 根据需求生成高质量的 Rust 代码
2. 遵循 Rust 最佳实践和惯用写法
3. 使用适当的错误处理（Result/Option）
4. 添加必要的注释和文档
5. 考虑性能和内存安全

输出格式：
- 使用 markdown 代码块包裹代码
- 说明代码的关键设计决策
- 列出需要添加的依赖（如果有）

可用工具：
- file_write: 写入文件
- cargo_init: 创建新项目
- cargo_check: 检查代码`

func (a *CodeGenAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		messages := []*model.Message{
			model.NewSystemMessage(codeGenPrompt),
			model.NewUserMessage(fmt.Sprintf("请生成以下 Rust 代码：\n%s", input)),
		}

		// 构建工具定义
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

		for i := 0; i < 5; i++ {
			resp, err := a.model.Generate(ctx, messages, model.WithTools(toolDefs...))
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
				return
			}

			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, resp.Content))

			if len(resp.ToolCalls) == 0 {
				break
			}

			// 执行工具
			for _, tc := range resp.ToolCalls {
				event.EmitEvent(ctx, eventChan, event.NewToolCallEvent(a.name, tc.Name, nil))

				if t, ok := a.tools.Get(tc.Name); ok {
					result, _ := t.Run(ctx, tc.Arguments)
					messages = append(messages, &model.Message{Role: "tool", Content: result})
				}
			}

			messages = append(messages, model.NewAssistantMessage(resp.Content))
		}

		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, nil))
	}()

	return eventChan, nil
}
