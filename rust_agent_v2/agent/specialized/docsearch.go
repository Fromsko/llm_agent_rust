package specialized

import (
	"context"
	"fmt"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/mcp"
	"rust_agent_v2/model"
)

// DocSearchAgent 文档搜索 Agent（使用 fetch MCP）
type DocSearchAgent struct {
	name     string
	model    model.Model
	fetchMCP *mcp.FetchMCP
}

func NewDocSearchAgent(m model.Model) *DocSearchAgent {
	return &DocSearchAgent{name: "docsearch-agent", model: m}
}

func (a *DocSearchAgent) Name() string { return a.name }

// SetFetchMCP 设置 fetch MCP 客户端
func (a *DocSearchAgent) SetFetchMCP(f *mcp.FetchMCP) {
	a.fetchMCP = f
}

const docSearchPrompt = `你是一个 Rust 文档搜索专家。

你的职责：
1. 根据用户问题确定需要查询的文档
2. 使用 fetch 工具获取文档内容
3. 从文档中提取相关信息
4. 整理并返回有用的信息

常用文档源：
- https://doc.rust-lang.org/std/ - 标准库文档
- https://doc.rust-lang.org/book/ - Rust Book
- https://doc.rust-lang.org/reference/ - 语言参考
- https://docs.rs/ - crates 文档

输出格式：
1. 相关文档链接
2. 关键信息摘要
3. 代码示例（如果有）`

func (a *DocSearchAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		// 先让 LLM 决定要查询哪些文档
		planMessages := []*model.Message{
			model.NewSystemMessage("你是 Rust 文档专家。根据用户问题，列出需要查询的文档 URL（每行一个）。只输出 URL，不要其他内容。"),
			model.NewUserMessage(input),
		}

		planResp, err := a.model.Generate(ctx, planMessages)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
			return
		}

		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 1, 3, "确定文档源"))

		// 获取文档内容
		var docContent string
		if a.fetchMCP != nil {
			// 使用 MCP 获取文档
			event.EmitEvent(ctx, eventChan, event.NewMCPCallEvent(a.name, "fetch", "fetch", map[string]any{"urls": planResp.Content}))

			content, err := a.fetchMCP.Fetch(ctx, "https://doc.rust-lang.org/std/")
			if err == nil {
				docContent = content
			}
		} else {
			// 使用简单 HTTP 获取
			content, err := mcp.SimpleFetch(ctx, "https://doc.rust-lang.org/std/")
			if err == nil {
				docContent = content
			}
		}

		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 2, 3, "获取文档内容"))

		// 让 LLM 总结文档
		summaryMessages := []*model.Message{
			model.NewSystemMessage(docSearchPrompt),
			model.NewUserMessage(fmt.Sprintf("用户问题：%s\n\n文档内容：\n%s", input, truncate(docContent, 8000))),
		}

		summaryResp, err := a.model.Generate(ctx, summaryMessages)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
			return
		}

		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 3, 3, "整理文档信息"))
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, summaryResp.Content))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, nil))
	}()

	return eventChan, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
