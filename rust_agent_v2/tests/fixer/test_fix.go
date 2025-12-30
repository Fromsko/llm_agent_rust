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

	// 使用绝对路径
	projectDir := `C:\Users\Administrator\Desktop\model_extract\rust_agent_v2\rust_workspace\async_http_server`
	compileError := `error[E0425]: cannot find function 'read' in module 'io'
  --> src\main.rs:16:25
   |
16 |             let n = io::read(&socket, &mut buf).await.unwrap();
   |                         ^^^^ not found in 'io'

error[E0425]: cannot find function 'write' in module 'io'
  --> src\main.rs:22:17
   |
22 |             io::write(&socket, &response).await.unwrap();
   |                 ^^^^^ not found in 'io'`

	fmt.Println("🔧 测试自主修复 Agent")
	fmt.Println("============================================================")
	fmt.Printf("📁 项目目录: %s\n", projectDir)
	fmt.Println("============================================================")

	fixer := specialized.NewAutonomousFixerAgent(m, tools)
	ctx := context.Background()

	eventChan, err := fixer.Run(ctx, "", agent.WithState(map[string]any{
		"project_dir":   projectDir,
		"compile_error": compileError,
	}))
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		switch ev.Type {
		case event.TypeResponse:
			if ev.Response != nil && ev.Response.Content != "" {
				content := ev.Response.Content
				if len(content) > 500 {
					content = content[:500] + "..."
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
				// 打印完整的工具调用信息，包括参数
				argsJSON, _ := json.MarshalIndent(ev.ToolCall.Arguments, "", "  ")
				fmt.Printf("\n🔧 调用工具: %s\n", ev.ToolCall.Name)
				fmt.Printf("   参数: %s\n", string(argsJSON))
			}
		case event.TypeCompletion:
			fmt.Println("\n✅ 完成")
		}
	}
}
