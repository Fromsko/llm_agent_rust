package specialized

import (
	"context"
	"fmt"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// ErrorFixAgent 错误修复 Agent
type ErrorFixAgent struct {
	name  string
	model model.Model
	tools *tool.Registry
}

func NewErrorFixAgent(m model.Model, tools *tool.Registry) *ErrorFixAgent {
	return &ErrorFixAgent{name: "errorfix-agent", model: m, tools: tools}
}

func (a *ErrorFixAgent) Name() string { return a.name }

const errorFixPrompt = `你是一个专业的 Rust 错误修复专家。

你的职责：
1. 分析编译错误和运行时错误
2. 理解 Rust 的所有权、借用和生命周期规则
3. 提供准确的修复方案
4. 解释错误原因和修复原理

常见错误类型：
- E0382: 值已被移动
- E0502: 不能同时借用为可变和不可变
- E0499: 不能多次可变借用
- E0597: 借用的值生命周期不够长
- E0308: 类型不匹配

输出格式：
1. 错误分析
2. 修复方案（代码）
3. 原理解释

可用工具：
- file_read: 读取源文件
- file_write: 写入修复后的代码
- cargo_check: 验证修复`

func (a *ErrorFixAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		messages := []*model.Message{
			model.NewSystemMessage(errorFixPrompt),
			model.NewUserMessage(fmt.Sprintf("请修复以下 Rust 错误：\n%s", input)),
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
