package agent

import (
	"context"
	"fmt"
	"strings"

	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// ReActStep ReAct 推理步骤
type ReActStep struct {
	Thought string `json:"thought"`
	Action  string `json:"action"`
	Input   string `json:"input"`
	Result  string `json:"result,omitempty"`
}

// AgentLoop 增强的 Agent 循环，支持 ReAct 模式
type AgentLoop struct {
	name          string
	model         model.Model
	tools         *tool.Registry
	systemPrompt  string
	maxIterations int
	reactMode     bool // 是否启用 ReAct 模式
}

// AgentLoopOption AgentLoop 配置选项
type AgentLoopOption func(*AgentLoop)

func WithLoopName(name string) AgentLoopOption {
	return func(a *AgentLoop) { a.name = name }
}

func WithLoopModel(m model.Model) AgentLoopOption {
	return func(a *AgentLoop) { a.model = m }
}

func WithLoopTools(t *tool.Registry) AgentLoopOption {
	return func(a *AgentLoop) { a.tools = t }
}

func WithLoopSystemPrompt(prompt string) AgentLoopOption {
	return func(a *AgentLoop) { a.systemPrompt = prompt }
}

func WithLoopMaxIter(n int) AgentLoopOption {
	return func(a *AgentLoop) { a.maxIterations = n }
}

func WithReActMode(enabled bool) AgentLoopOption {
	return func(a *AgentLoop) { a.reactMode = enabled }
}

func NewAgentLoop(opts ...AgentLoopOption) *AgentLoop {
	loop := &AgentLoop{
		name:          "agent-loop",
		maxIterations: 20,
		reactMode:     true,
	}
	for _, opt := range opts {
		opt(loop)
	}
	return loop
}

// Run 执行 Agent 循环
func (l *AgentLoop) Run(ctx context.Context, input string, opts ...InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		invOpts := ApplyOptions(opts...)
		messages := l.buildMessages(input, invOpts)

		// ReAct 步骤历史
		var reactSteps []*ReActStep

		for i := 0; i < l.maxIterations; i++ {
			select {
			case <-ctx.Done():
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(l.name, "CANCELLED", ctx.Err().Error()))
				return
			default:
			}

			// 发送进度事件
			event.EmitEvent(ctx, eventChan, event.NewProgressEvent(l.name, i+1, l.maxIterations,
				fmt.Sprintf("思考中 (迭代 %d/%d)", i+1, l.maxIterations)))

			// 调用模型
			resp, err := l.model.Generate(ctx, messages, l.buildToolDefs())
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(l.name, "LLM_ERROR", err.Error()))
				return
			}

			// 解析 ReAct 步骤
			step := l.parseReActStep(resp.Content)
			if step != nil {
				reactSteps = append(reactSteps, step)
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(l.name,
					fmt.Sprintf("🧠 思考: %s\n🎯 行动: %s", step.Thought, step.Action)))
			} else {
				// 普通响应
				if resp.Content != "" {
					event.EmitEvent(ctx, eventChan, event.NewResponseEvent(l.name, resp.Content))
				}
			}

			// 检查是否完成
			if len(resp.ToolCalls) == 0 {
				// 没有工具调用，任务完成
				if step != nil && step.Action == "FINISH" {
					event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(l.name, map[string]any{
						"success": true,
						"result":  step.Result,
						"steps":   reactSteps,
					}))
				} else {
					event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(l.name, map[string]any{
						"success": true,
						"result":  resp.Content,
						"steps":   reactSteps,
					}))
				}
				return
			}

			// 执行工具调用
			for _, tc := range resp.ToolCalls {
				event.EmitEvent(ctx, eventChan, event.NewToolCallEvent(l.name, tc.Name,
					map[string]any{"args": tc.Arguments}))

				result, err := l.executeTool(ctx, tc.Name, tc.Arguments)
				if err != nil {
					result = fmt.Sprintf("错误: %v", err)
				}

				// 更新 ReAct 步骤结果
				if step != nil {
					step.Result = result
				}

				// 显示结果预览
				preview := result
				if len(preview) > 300 {
					preview = preview[:300] + "..."
				}
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(l.name,
					fmt.Sprintf("📤 %s 结果:\n%s", tc.Name, preview)))

				messages = append(messages, &model.Message{
					Role:    "tool",
					Content: result,
				})
			}

			messages = append(messages, &model.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
		}

		event.EmitEvent(ctx, eventChan, event.NewErrorEvent(l.name, "MAX_ITERATIONS", "达到最大迭代次数"))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(l.name, map[string]any{
			"success": false,
			"steps":   reactSteps,
		}))
	}()

	return eventChan, nil
}

// parseReActStep 解析 ReAct 格式的响应
func (l *AgentLoop) parseReActStep(content string) *ReActStep {
	if !l.reactMode {
		return nil
	}

	step := &ReActStep{}

	// 提取 Thought
	if idx := strings.Index(content, "Thought:"); idx != -1 {
		endIdx := strings.Index(content[idx:], "\n")
		if endIdx == -1 {
			endIdx = len(content) - idx
		}
		step.Thought = strings.TrimSpace(content[idx+8 : idx+endIdx])
	}

	// 提取 Action
	if idx := strings.Index(content, "Action:"); idx != -1 {
		endIdx := strings.Index(content[idx:], "\n")
		if endIdx == -1 {
			endIdx = len(content) - idx
		}
		step.Action = strings.TrimSpace(content[idx+7 : idx+endIdx])
	}

	// 提取 Input
	if idx := strings.Index(content, "Input:"); idx != -1 {
		endIdx := strings.Index(content[idx:], "\n")
		if endIdx == -1 {
			endIdx = len(content) - idx
		}
		step.Input = strings.TrimSpace(content[idx+6 : idx+endIdx])
	}

	// 如果没有提取到任何内容，返回 nil
	if step.Thought == "" && step.Action == "" {
		return nil
	}

	return step
}

// executeTool 执行工具
func (l *AgentLoop) executeTool(ctx context.Context, name string, arguments string) (string, error) {
	if l.tools == nil {
		return "", fmt.Errorf("没有可用的工具")
	}

	t, ok := l.tools.Get(name)
	if !ok {
		return "", fmt.Errorf("工具不存在: %s", name)
	}

	return t.Run(ctx, arguments)
}

// buildMessages 构建消息列表
func (l *AgentLoop) buildMessages(input string, opts *InvocationOptions) []*model.Message {
	var messages []*model.Message

	if l.systemPrompt != "" {
		// 添加 ReAct 格式的系统提示
		prompt := l.systemPrompt
		if l.reactMode {
			prompt += "\n\n" + reactPrompt
		}
		messages = append(messages, model.NewSystemMessage(prompt))
	}

	if opts.Messages != nil {
		messages = append(messages, opts.Messages...)
	}

	messages = append(messages, model.NewUserMessage(input))
	return messages
}

// buildToolDefs 构建工具定义
func (l *AgentLoop) buildToolDefs() model.Option {
	if l.tools == nil {
		return nil
	}

	var toolDefs []*model.ToolDef
	for _, t := range l.tools.List() {
		toolDefs = append(toolDefs, &model.ToolDef{
			Type: "function",
			Function: &model.Function{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.InputSchema(),
			},
		})
	}

	return model.WithTools(toolDefs...)
}

// reactPrompt ReAct 格式提示
const reactPrompt = `
你必须按照以下 ReAct 格式思考和行动：

格式:
Thought: [你的思考过程，分析当前情况]
Action: [要执行的动作，工具名称或 FINISH]
Input: [动作的输入参数]

可用动作:
- 工具名称: 调用对应的工具
- FINISH: 任务完成，输出最终结果

示例:
Thought: 用户需要一个 HTTP 服务器，我应该先创建项目
Action: cargo_init
Input: {"work_dir": "./workspace", "project_name": "http_server"}

Thought: 项目已创建，现在需要写入代码
Action: file_write
Input: {"path": "./workspace/http_server/src/main.rs", "content": "..."}

Thought: 所有任务已完成，代码编译通过
Action: FINISH
Input: 任务完成，项目已创建并编译成功

重要规则:
1. 必须严格按照 Thought/Action/Input 格式输出
2. Action 必须是工具名称或 FINISH
3. Input 必须是有效的 JSON 格式
4. 每次只执行一个 Action
5. 任务完成后必须使用 FINISH
`

// InteractiveAgentLoop 交互式 Agent 循环，支持 ask_user
type InteractiveAgentLoop struct {
	*AgentLoop
	userInputHandler UserInputHandler
}

// UserInputHandler 用户输入处理器
type UserInputHandler func(ctx context.Context, question string, options []string) (string, error)

func NewInteractiveAgentLoop(handler UserInputHandler, opts ...AgentLoopOption) *InteractiveAgentLoop {
	loop := NewAgentLoop(opts...)
	return &InteractiveAgentLoop{
		AgentLoop:        loop,
		userInputHandler: handler,
	}
}

// RunInteractive 运行交互式循环
func (l *InteractiveAgentLoop) RunInteractive(ctx context.Context, input string, opts ...InvocationOption) (<-chan *event.Event, error) {
	// 如果注册了 ask_user 工具，设置处理器
	if l.tools != nil {
		if t, ok := l.tools.Get("ask_user"); ok {
			if askTool, ok := t.(*tool.AskUserTool); ok && l.userInputHandler != nil {
				askTool.SetHandler(func(question string, options []string) (string, error) {
					return l.userInputHandler(ctx, question, options)
				})
			}
		}
	}

	return l.Run(ctx, input, opts...)
}
