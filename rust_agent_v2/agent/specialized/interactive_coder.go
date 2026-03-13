package specialized

import (
	"context"
	"fmt"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// InteractiveCoderAgent 交互式编码 Agent
// 结合 ReAct 模式和 ask_user 工具，实现人机协作编码
type InteractiveCoderAgent struct {
	name      string
	loop      *agent.AgentLoop
	model     model.Model
	tools     *tool.Registry
	workspace string
}

func NewInteractiveCoderAgent(m model.Model, tools *tool.Registry, workspace string) *InteractiveCoderAgent {
	return &InteractiveCoderAgent{
		name:      "interactive-coder",
		model:     m,
		tools:     tools,
		workspace: workspace,
	}
}

func (a *InteractiveCoderAgent) Name() string { return a.name }

const interactiveCoderPrompt = `你是交互式 Rust 编码助手。你的工作流程是:

1. **分析需求**: 理解用户想要什么
2. **询问澄清**: 如果需求不明确，使用 ask_user 工具询问用户
3. **规划设计**: 规划实现方案
4. **执行编码**: 创建项目、编写代码
5. **验证修复**: 编译检查，如有错误则修复
6. **确认完成**: 向用户展示结果并确认

**重要规则**:
- 需求不明确时，必须询问用户，不要猜测
- 提供选项时使用 ask_user 工具的 options 参数
- 编码过程中保持与用户的沟通
- 编译错误必须修复，不能跳过

**可用工具**:
- ask_user: 询问用户问题（支持多选项）
- cargo_init: 创建项目
- cargo_check: 检查编译
- cargo_build: 构建项目
- file_write: 写入文件
- file_read: 读取文件
- crates_search: 搜索 crates
- crate_source: 读取 crate 源码

**ReAct 格式**:
Thought: [你的思考]
Action: [工具名称]
Input: [JSON 参数]
`

func (a *InteractiveCoderAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
			"🤖 交互式编码助手已启动\n我会与你协作完成 Rust 项目开发。"))

		// 阶段1: 需求分析和澄清
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 1, 4, "📋 分析需求..."))

		clarifiedTask, err := a.clarifyRequirements(ctx, input, eventChan)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "CLARIFY_ERROR", err.Error()))
			return
		}

		// 阶段2: 技术选型（询问用户）
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 2, 4, "🔧 技术选型..."))

		selectedCrates, err := a.selectCrates(ctx, clarifiedTask, eventChan)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "SELECT_ERROR", err.Error()))
			return
		}

		// 阶段3: 编码实现
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 3, 4, "💻 编码实现..."))

		result, err := a.implementCode(ctx, clarifiedTask, selectedCrates, eventChan)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "IMPLEMENT_ERROR", err.Error()))
			return
		}

		// 阶段4: 确认完成
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 4, 4, "✅ 确认完成..."))

		success := a.confirmCompletion(ctx, result, eventChan)

		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{
			"success":     success,
			"project_dir": result,
		}))
	}()

	return eventChan, nil
}

// clarifyRequirements 澄清需求
func (a *InteractiveCoderAgent) clarifyRequirements(ctx context.Context, input string, eventChan chan<- *event.Event) (string, error) {
	// 使用 ReAct AgentLoop 分析需求
	loop := agent.NewAgentLoop(
		agent.WithLoopName("requirement-clarifier"),
		agent.WithLoopModel(a.model),
		agent.WithLoopTools(a.tools),
		agent.WithLoopSystemPrompt(`你负责分析用户需求并澄清不明确的地方。

分析步骤:
1. 理解用户原始需求
2. 识别不明确的点（功能范围、技术偏好、项目结构等）
3. 如果需要澄清，使用 ask_user 工具询问
4. 返回澄清后的完整需求描述

注意: 如果需求已经很明确，直接返回需求描述，不需要询问。`),
		agent.WithLoopMaxIter(10),
		agent.WithReActMode(true),
	)

	event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "正在分析你的需求..."))

	clarifyInput := fmt.Sprintf(`用户需求: %s

请分析这个需求，如果有任何不明确的地方，使用 ask_user 工具询问用户。
如果需求明确，直接返回完整的项目需求描述。`, input)

	loopChan, err := loop.Run(ctx, clarifyInput)
	if err != nil {
		return "", err
	}

	var finalResult string
	for ev := range loopChan {
		event.EmitEvent(ctx, eventChan, ev)

		if ev.Type == event.TypeCompletion && ev.Completion != nil {
			if result, ok := ev.Completion.Result.(map[string]any); ok {
				if r, ok := result["result"].(string); ok {
					finalResult = r
				}
			}
		}
	}

	if finalResult == "" {
		finalResult = input
	}

	return finalResult, nil
}

// selectCrates 选择技术栈
func (a *InteractiveCoderAgent) selectCrates(ctx context.Context, task string, eventChan chan<- *event.Event) ([]string, error) {
	// 询问用户选择技术方案
	result, err := tool.InteractiveAskUser(
		"根据你的需求，推荐以下技术方案，请选择：",
		[]string{
			"tokio + axum - 异步 Web 服务器",
			"tokio + reqwest - HTTP 客户端",
			"clap - 命令行工具",
			"serde + serde_json - 数据处理",
			"自定义（我会询问具体需求）",
		},
		false,
	)
	if err != nil {
		// 使用默认
		return []string{"tokio"}, nil
	}

	// 解析选择
	switch result {
	case "tokio + axum - 异步 Web 服务器":
		return []string{"tokio", "axum"}, nil
	case "tokio + reqwest - HTTP 客户端":
		return []string{"tokio", "reqwest"}, nil
	case "clap - 命令行工具":
		return []string{"clap"}, nil
	case "serde + serde_json - 数据处理":
		return []string{"serde", "serde_json"}, nil
	default:
		// 自定义，询问具体需求
		specific, _ := tool.AskWithFreeText("请描述你需要的具体功能或库：")
		return []string{specific}, nil
	}
}

// implementCode 实现代码
func (a *InteractiveCoderAgent) implementCode(ctx context.Context, task string, crates []string, eventChan chan<- *event.Event) (string, error) {
	// 使用 ReAct AgentLoop 编码
	loop := agent.NewAgentLoop(
		agent.WithLoopName("code-implementer"),
		agent.WithLoopModel(a.model),
		agent.WithLoopTools(a.tools),
		agent.WithLoopSystemPrompt(fmt.Sprintf(`你负责实现 Rust 代码。

项目要求:
- 使用以下 crates: %v
- 代码必须编译通过
- 遵循 Rust 最佳实践

工作流程:
1. cargo_init 创建项目
2. 写入 Cargo.toml 添加依赖
3. 使用 crate_source 查看 API
4. 写入 src/main.rs 实现代码
5. cargo_check 验证编译
6. 如有错误，修复并重新验证

重要: 不要写 TODO，必须实现完整功能。`, crates)),
		agent.WithLoopMaxIter(30),
		agent.WithReActMode(true),
	)

	event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
		fmt.Sprintf("开始实现项目，使用 crates: %v", crates)))

	implementInput := fmt.Sprintf(`任务: %s
工作目录: %s
需要的 crates: %v

请按照工作流程完成项目实现。`, task, a.workspace, crates)

	loopChan, err := loop.Run(ctx, implementInput)
	if err != nil {
		return "", err
	}

	var projectDir string
	for ev := range loopChan {
		event.EmitEvent(ctx, eventChan, ev)

		if ev.Type == event.TypeCompletion && ev.Completion != nil {
			if result, ok := ev.Completion.Result.(map[string]any); ok {
				if dir, ok := result["project_dir"].(string); ok {
					projectDir = dir
				}
			}
		}
	}

	return projectDir, nil
}

// confirmCompletion 确认完成
func (a *InteractiveCoderAgent) confirmCompletion(ctx context.Context, projectDir string, eventChan chan<- *event.Event) bool {
	// 询问用户是否满意
	confirmed, err := tool.ConfirmYesNo("项目已完成，是否满意？", true)
	if err != nil {
		return true // 默认满意
	}

	if !confirmed {
		// 询问需要修改的地方
		feedback, _ := tool.AskWithFreeText("请告诉我需要修改的地方：")
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
			fmt.Sprintf("收到反馈: %s，准备修改...", feedback)))

		// 这里可以实现修改逻辑
		_ = feedback
	}

	return confirmed
}
