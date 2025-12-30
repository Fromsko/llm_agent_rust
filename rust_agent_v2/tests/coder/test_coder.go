package main

import (
	"context"
	"encoding/json"
	"fmt"

	"rust_agent_v2/agent"
	"rust_agent_v2/agent/specialized"
	"rust_agent_v2/config"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

func main() {
	cfg, _ := config.Load("config.json")

	m := model.NewZhipuModel(cfg.API.Model,
		model.ZhipuWithAPIKey(cfg.API.ZhipuAPIKey),
		model.ZhipuWithBaseURL(cfg.API.ZhipuBaseURL),
		model.ZhipuWithConcurrency(cfg.API.Concurrency),
	)

	tools := tool.CreateAdvancedRegistry()

	workspace := `C:\Users\Administrator\Desktop\model_extract\rust_agent_v2\rust_workspace`

	fmt.Println("🔧 测试自主编码 Agent")
	fmt.Println("============================================================")
	fmt.Printf("📁 工作目录: %s\n", workspace)
	fmt.Println("============================================================")

	coder := specialized.NewAutonomousCoderAgent(m, tools, workspace)
	ctx := context.Background()

	// 测试任务：使用 MCP Rust SDK 创建加减乘除工具
	task := `使用 https://github.com/modelcontextprotocol/rust-sdk 创建一个 MCP 服务器项目，项目名为 mcp_math_tools。
要求：
1. 实现4个简单的数学工具：add（加法）、subtract（减法）、multiply（乘法）、divide（除法）
2. 每个工具接收两个数字参数 a 和 b，返回计算结果
3. divide 需要处理除零错误
4. 必须查看 rust-sdk 的源码或文档，了解正确的 API 用法
5. 写出真正可编译运行的代码`

	fmt.Printf("\n📝 任务: %s\n", task)
	fmt.Println("============================================================")

	eventChan, err := coder.Run(ctx, task, agent.WithState(map[string]any{}))
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		switch ev.Type {
		case event.TypeResponse:
			if ev.Response != nil && ev.Response.Content != "" {
				content := ev.Response.Content
				if len(content) > 800 {
					content = content[:800] + "..."
				}
				fmt.Printf("\n💬 %s\n", content)
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
				argsJSON, _ := json.MarshalIndent(ev.ToolCall.Arguments, "", "  ")
				argsStr := string(argsJSON)
				if len(argsStr) > 300 {
					argsStr = argsStr[:300] + "..."
				}
				fmt.Printf("\n🔧 调用工具: %s\n   参数: %s\n", ev.ToolCall.Name, argsStr)
			}
		case event.TypeCompletion:
			fmt.Println("\n✅ 完成")
		}
	}
}
