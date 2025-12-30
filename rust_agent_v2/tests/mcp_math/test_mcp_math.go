package main

import (
	"context"
	"fmt"
	"os"

	"rust_agent_v2/agent/specialized"
	"rust_agent_v2/config"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

func main() {
	// 加载配置
	cfg, err := config.Load("config.json")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	workspace := "./rust_workspace"
	os.MkdirAll(workspace, 0755)

	// 创建模型
	m := model.NewZhipuModel(cfg.API.Model,
		model.ZhipuWithAPIKey(cfg.API.ZhipuAPIKey),
		model.ZhipuWithBaseURL(cfg.API.ZhipuBaseURL),
	)

	// 创建工具注册表
	tools := tool.CreateAdvancedRegistry()

	ctx := context.Background()

	// 测试任务：使用 rmcp 创建 MCP 数学工具
	task := `使用 https://github.com/modelcontextprotocol/rust-sdk (rmcp crate) 创建一个 MCP 服务器，实现4个简单的数学工具：
1. add - 加法
2. subtract - 减法  
3. multiply - 乘法
4. divide - 除法

要求：
- 必须使用 rmcp crate（不是 mcp-attr 或其他库）
- 使用 #[tool] 宏定义工具
- 实现 ServerHandler trait
- 使用 stdio transport
- 项目名称: mcp_math_tools`

	fmt.Println("============================================================")
	fmt.Println("🧪 测试：使用 rmcp 创建 MCP 数学工具")
	fmt.Println("============================================================")
	fmt.Printf("📝 任务: %s\n", task)
	fmt.Printf("📁 工作目录: %s\n", workspace)
	fmt.Println("============================================================")

	// 使用 SupervisorAgent（带经验学习）
	supervisor := specialized.NewSupervisorAgent(m, tools, workspace)
	eventChan, err := supervisor.Run(ctx, task)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

func handleEvent(ev *event.Event) {
	switch ev.Type {
	case event.TypeResponse:
		if ev.Response != nil {
			fmt.Printf("\n%s\n", ev.Response.Content)
		}
	case event.TypeError:
		if ev.Error != nil {
			fmt.Printf("❌ [%s] %s\n", ev.Error.Code, ev.Error.Message)
		}
	case event.TypeProgress:
		if ev.Progress != nil {
			fmt.Printf("\n📊 [%d/%d] %s\n", ev.Progress.Current, ev.Progress.Total, ev.Progress.Message)
		}
	case event.TypeToolCall:
		if ev.ToolCall != nil {
			fmt.Printf("🔧 调用工具: %s\n", ev.ToolCall.Name)
		}
	case event.TypeCompletion:
		if ev.Completion != nil {
			if result, ok := ev.Completion.Result.(map[string]any); ok {
				if success, ok := result["success"].(bool); ok {
					if success {
						fmt.Println("\n✅ 测试通过！")
					} else {
						fmt.Println("\n❌ 测试失败")
					}
				}
			}
		}
	}
}
