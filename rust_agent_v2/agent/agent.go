package agent

import (
	"context"

	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// Agent 接口定义
type Agent interface {
	Run(ctx context.Context, input string, opts ...InvocationOption) (<-chan *event.Event, error)
	Name() string
}

// InvocationOption 调用选项
type InvocationOption func(*InvocationOptions)

type InvocationOptions struct {
	InvocationID string
	Messages     []*model.Message
	State        map[string]any
}

func WithInvocationID(id string) InvocationOption {
	return func(o *InvocationOptions) { o.InvocationID = id }
}

func WithMessages(msgs []*model.Message) InvocationOption {
	return func(o *InvocationOptions) { o.Messages = msgs }
}

func WithState(state map[string]any) InvocationOption {
	return func(o *InvocationOptions) { o.State = state }
}

func ApplyOptions(opts ...InvocationOption) *InvocationOptions {
	o := &InvocationOptions{State: make(map[string]any)}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// BaseAgent 基础 Agent 实现
type BaseAgent struct {
	name         string
	model        model.Model
	systemPrompt string
	tools        *tool.Registry
	maxIter      int
}

type BaseOption func(*BaseAgent)

func WithName(name string) BaseOption {
	return func(a *BaseAgent) { a.name = name }
}

func WithModel(m model.Model) BaseOption {
	return func(a *BaseAgent) { a.model = m }
}

func WithSystemPrompt(prompt string) BaseOption {
	return func(a *BaseAgent) { a.systemPrompt = prompt }
}

func WithTools(r *tool.Registry) BaseOption {
	return func(a *BaseAgent) { a.tools = r }
}

func WithMaxIter(n int) BaseOption {
	return func(a *BaseAgent) { a.maxIter = n }
}

func NewBaseAgent(opts ...BaseOption) *BaseAgent {
	a := &BaseAgent{name: "base-agent", maxIter: 10}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *BaseAgent) Name() string { return a.name }

func (a *BaseAgent) Run(ctx context.Context, input string, opts ...InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		invOpts := ApplyOptions(opts...)
		messages := a.buildMessages(input, invOpts)

		for i := 0; i < a.maxIter; i++ {
			select {
			case <-ctx.Done():
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "CANCELLED", ctx.Err().Error()))
				return
			default:
			}

			resp, err := a.model.Generate(ctx, messages)
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
				return
			}

			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, resp.Content))

			if len(resp.ToolCalls) == 0 {
				break
			}

			// 执行工具调用
			for _, tc := range resp.ToolCalls {
				event.EmitEvent(ctx, eventChan, event.NewToolCallEvent(a.name, tc.Name, map[string]any{"args": tc.Arguments}))

				if a.tools != nil {
					if t, ok := a.tools.Get(tc.Name); ok {
						result, err := t.Run(ctx, tc.Arguments)
						if err != nil {
							messages = append(messages, &model.Message{Role: "tool", Content: "Error: " + err.Error()})
						} else {
							messages = append(messages, &model.Message{Role: "tool", Content: result})
						}
					}
				}
			}

			messages = append(messages, &model.Message{Role: "assistant", Content: resp.Content})
		}

		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, nil))
	}()

	return eventChan, nil
}

func (a *BaseAgent) buildMessages(input string, opts *InvocationOptions) []*model.Message {
	var messages []*model.Message

	if a.systemPrompt != "" {
		messages = append(messages, model.NewSystemMessage(a.systemPrompt))
	}

	if opts.Messages != nil {
		messages = append(messages, opts.Messages...)
	}

	messages = append(messages, model.NewUserMessage(input))
	return messages
}
