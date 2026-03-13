package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"rust_agent_v2/agent"
	"rust_agent_v2/agent/specialized"
	"rust_agent_v2/config"
	"rust_agent_v2/event"
	"rust_agent_v2/memory"
	"rust_agent_v2/model"
	"rust_agent_v2/runner"
	"rust_agent_v2/tool"
	"rust_agent_v2/workflow"
)

func main() {
	// 命令行参数
	interactive := flag.Bool("i", false, "交互模式")
	createCmd := flag.String("create", "", "创建完整项目（自主规划+生成+编译+修复）")
	searchCmd := flag.String("search", "", "搜索 crates")
	workspaceDir := flag.String("workspace", "./rust_workspace", "工作目录")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load("config.json")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 使用命令行参数覆盖配置
	if *workspaceDir != "" {
		cfg.Workspace.RootDir = *workspaceDir
	}

	// 确保工作目录存在
	os.MkdirAll(cfg.Workspace.RootDir, 0755)

	// 创建模型
	modelOpts := []model.ZhipuOption{
		model.ZhipuWithAPIKey(cfg.API.ZhipuAPIKey),
		model.ZhipuWithConcurrency(cfg.API.Concurrency),
	}
	// 只有当 baseURL 不为空时才设置，避免覆盖默认值
	if cfg.API.ZhipuBaseURL != "" {
		modelOpts = append(modelOpts, model.ZhipuWithBaseURL(cfg.API.ZhipuBaseURL))
	}
	m := model.NewZhipuModel(cfg.API.Model, modelOpts...)

	// 创建高级工具注册表
	tools := tool.CreateAdvancedRegistry()

	ctx := context.Background()

	switch {
	case *createCmd != "":
		runAutonomousCoder(ctx, m, tools, cfg.Workspace.RootDir, *createCmd)
	case *searchCmd != "":
		runCratesSearch(ctx, m, *searchCmd)
	case *interactive:
		runInteractive(ctx, cfg, m, tools)
	default:
		printUsage()
	}
}

func runAutonomousCoder(ctx context.Context, m model.Model, tools *tool.Registry, workspace, input string) {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Agent v2 - 监督者模式（带经验学习）")
	fmt.Println("============================================================")
	fmt.Printf("📝 需求: %s\n", input)
	fmt.Printf("📁 工作目录: %s\n", workspace)
	fmt.Println("============================================================")
	fmt.Println("🤖 SupervisorAgent 将协调：")
	fmt.Println("   1. 分析任务 → 2. 检索经验 → 3. 执行编码")
	fmt.Println("   4. 验证结果 → 5. 反思学习 → 6. 保存经验")
	fmt.Println("============================================================")

	// 使用 SupervisorAgent 替代直接调用 AutonomousCoderAgent
	supervisor := specialized.NewSupervisorAgent(m, tools, workspace)
	eventChan, err := supervisor.Run(ctx, input)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

// runWithRunner 使用 Runner 模块运行（带会话管理）
func runWithRunner(ctx context.Context, m model.Model, tools *tool.Registry, workspace, input, sessionID string) {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Agent v2 - Runner 模式（带会话管理）")
	fmt.Println("============================================================")
	fmt.Printf("📝 需求: %s\n", input)
	fmt.Printf("📁 工作目录: %s\n", workspace)
	fmt.Printf("🔑 会话ID: %s\n", sessionID)
	fmt.Println("============================================================")

	// 构建工作流图
	g := workflow.BuildGraph(m, tools, workspace)

	// 创建 Runner
	r := runner.New(g, runner.WithConfig(&runner.Config{
		AppName:            "rust-agent-v2",
		AutoSummarize:      true,
		SummarizeThreshold: 20,
		MaxMessages:        100,
	}))

	// 运行
	eventChan, err := r.Run(ctx, &runner.Request{
		UserID:    "default",
		SessionID: sessionID,
		Input:     input,
	})
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

// showExperience 显示经验库内容
func showExperience(workspace string) {
	expStore, err := memory.NewExperienceStore(workspace + "/.experience")
	if err != nil {
		fmt.Printf("❌ 无法加载经验库: %v\n", err)
		return
	}

	fmt.Println("============================================================")
	fmt.Println("📚 经验库内容")
	fmt.Println("============================================================")

	// 显示所有经验
	experiences := expStore.FindByCrate("")
	if len(experiences) == 0 {
		fmt.Println("📭 经验库为空")
		return
	}

	for i, exp := range experiences {
		status := "❌"
		if exp.Success {
			status = "✅"
		}
		fmt.Printf("\n%d. %s [%s] %s\n", i+1, status, exp.CrateName, exp.Task)
		fmt.Printf("   创建时间: %s\n", exp.CreatedAt.Format("2006-01-02 15:04:05"))

		if len(exp.Imports) > 0 {
			fmt.Println("   📦 正确导入:")
			for _, imp := range exp.Imports {
				fmt.Printf("      %s\n", imp)
			}
		}

		if len(exp.APIUsage) > 0 {
			fmt.Println("   🔧 API 用法:")
			for _, usage := range exp.APIUsage[:min(5, len(exp.APIUsage))] {
				fmt.Printf("      %s\n", usage)
			}
			if len(exp.APIUsage) > 5 {
				fmt.Printf("      ... 还有 %d 条\n", len(exp.APIUsage)-5)
			}
		}

		if len(exp.Lessons) > 0 {
			fmt.Println("   💡 教训:")
			for _, lesson := range exp.Lessons {
				fmt.Printf("      - %s\n", lesson)
			}
		}

		if len(exp.Errors) > 0 {
			fmt.Println("   ⚠️ 遇到的错误:")
			for _, err := range exp.Errors[:min(3, len(exp.Errors))] {
				// 只显示错误的前100个字符
				errStr := err
				if len(errStr) > 100 {
					errStr = errStr[:100] + "..."
				}
				fmt.Printf("      - %s\n", errStr)
			}
		}

		if exp.Code != "" {
			lines := strings.Split(exp.Code, "\n")
			fmt.Printf("   📄 代码: %d 行\n", len(lines))
		}
	}

	fmt.Println("\n============================================================")
	fmt.Printf("📊 总计: %d 条经验\n", len(experiences))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runFullWorkflow(ctx context.Context, m model.Model, tools *tool.Registry, workspace, input string) {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Agent v2 - 完整自动化工作流")
	fmt.Println("============================================================")
	fmt.Printf("📝 需求: %s\n", input)
	fmt.Printf("📁 工作目录: %s\n", workspace)
	fmt.Println("============================================================")

	wf := workflow.NewRustWorkflow(m, tools, workspace)
	eventChan, err := wf.Run(ctx, input)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

func runCratesSearch(ctx context.Context, m model.Model, query string) {
	fmt.Printf("🔍 搜索 crates: %s\n\n", query)

	cratesAgent := specialized.NewCratesIOAgent(m)
	eventChan, err := cratesAgent.Run(ctx, query)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

func runInteractive(ctx context.Context, cfg *config.Config, m model.Model, tools *tool.Registry) {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Agent v2 - 交互模式 (监督者 + 经验学习)")
	fmt.Println("============================================================")
	fmt.Println("命令:")
	fmt.Println("  /create <描述>      - 监督者模式创建项目（带经验学习）")
	fmt.Println("  /interactive <描述> - 交互式编码（ReAct + 多轮对话）")
	fmt.Println("  /direct <描述>      - 直接编码模式（不带监督）")
	fmt.Println("  /fix <项目名>       - 自主修复项目")
	fmt.Println("  /search <关键词>    - 搜索 crates.io")
	fmt.Println("  /list               - 列出已创建的项目")
	fmt.Println("  /run <项目名>       - 运行项目")
	fmt.Println("  /exp                - 查看经验库")
	fmt.Println("  /quit               - 退出")
	fmt.Println("============================================================")
	fmt.Printf("📁 工作目录: %s\n", cfg.Workspace.RootDir)
	fmt.Println("============================================================")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n🦀 > ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {
		case input == "/quit" || input == "/exit":
			fmt.Println("👋 再见!")
			return

		case strings.HasPrefix(input, "/create "):
			query := strings.TrimPrefix(input, "/create ")
			runAutonomousCoder(ctx, m, tools, cfg.Workspace.RootDir, query)

		case strings.HasPrefix(input, "/interactive "):
			query := strings.TrimPrefix(input, "/interactive ")
			runInteractiveCoder(ctx, m, tools, cfg.Workspace.RootDir, query)

		case strings.HasPrefix(input, "/direct "):
			query := strings.TrimPrefix(input, "/direct ")
			runDirectCoder(ctx, m, tools, cfg.Workspace.RootDir, query)

		case strings.HasPrefix(input, "/search "):
			query := strings.TrimPrefix(input, "/search ")
			runCratesSearch(ctx, m, query)

		case input == "/list":
			listProjects(cfg.Workspace.RootDir)

		case input == "/exp":
			showExperience(cfg.Workspace.RootDir)

		case strings.HasPrefix(input, "/run "):
			projectName := strings.TrimPrefix(input, "/run ")
			runProject(ctx, cfg.Workspace.RootDir, projectName)

		case strings.HasPrefix(input, "/fix "):
			projectName := strings.TrimPrefix(input, "/fix ")
			fixProjectAutonomous(ctx, m, tools, cfg.Workspace.RootDir, projectName)

		default:
			// 默认使用监督者模式
			fmt.Println("💡 提示: 使用监督者模式创建项目（带经验学习）")
			runAutonomousCoder(ctx, m, tools, cfg.Workspace.RootDir, input)
		}
	}
}

// runInteractiveCoder 交互式编码模式（ReAct + 多轮对话）
func runInteractiveCoder(ctx context.Context, m model.Model, tools *tool.Registry, workspace, input string) {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Agent v2 - 交互式编码模式（ReAct + ask_user）")
	fmt.Println("============================================================")
	fmt.Printf("📝 需求: %s\n", input)
	fmt.Printf("📁 工作目录: %s\n", workspace)
	fmt.Println("============================================================")
	fmt.Println("🤖 工作流程:")
	fmt.Println("   1. 澄清需求 → 2. 选择技术栈 → 3. 实现代码 → 4. 确认完成")
	fmt.Println("============================================================")

	// 设置用户输入处理器到 ask_user 工具
	if t, ok := tools.Get("ask_user"); ok {
		if askTool, ok := t.(*tool.AskUserTool); ok {
			askTool.SetHandler(func(question string, options []string) (string, error) {
				fmt.Println()
				fmt.Printf("🤔 %s\n", question)

				if len(options) > 0 {
					// 检测是否是 y/n 类型的问题
					isYesNo := len(options) == 2 &&
						((options[0] == "y" && options[1] == "n") ||
							(options[0] == "yes" && options[1] == "no") ||
							(options[0] == "Y" && options[1] == "N"))

					if isYesNo {
						fmt.Print("\n请输入 y/n: ")
					} else {
						fmt.Println("选项:")
						for i, opt := range options {
							fmt.Printf("   %d. %s\n", i+1, opt)
						}
						fmt.Print("\n请输入选项编号或直接输入选项: ")
					}
				} else {
					fmt.Print("\n请输入: ")
				}

				reader := bufio.NewReader(os.Stdin)
				answer, err := reader.ReadString('\n')
				if err != nil {
					return "", err
				}
				answer = strings.TrimSpace(answer)

				// 如果输入是数字，转换为选项
				if idx, err := strconv.Atoi(answer); err == nil && idx > 0 && idx <= len(options) {
					return options[idx-1], nil
				}

				// 返回原始输入（支持直接输入 y/n 或选项文本）
				return answer, nil
			})
		}
	}

	// 创建交互式编码 Agent
	interactiveCoder := specialized.NewInteractiveCoderAgent(m, tools, workspace)
	eventChan, err := interactiveCoder.Run(ctx, input)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

// runDirectCoder 直接编码模式（不带监督）
func runDirectCoder(ctx context.Context, m model.Model, tools *tool.Registry, workspace, input string) {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Agent v2 - 直接编码模式（无监督）")
	fmt.Println("============================================================")
	fmt.Printf("📝 需求: %s\n", input)
	fmt.Printf("📁 工作目录: %s\n", workspace)
	fmt.Println("============================================================")

	coder := specialized.NewAutonomousCoderAgent(m, tools, workspace)
	eventChan, err := coder.Run(ctx, input)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

func listProjects(workspace string) {
	projects, err := workflow.ListProjects(workspace)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	if len(projects) == 0 {
		fmt.Println("📭 暂无项目")
		return
	}

	fmt.Println("📦 已创建的项目:")
	for _, p := range projects {
		fmt.Printf("  - %s\n", p)
	}
}

func runProject(ctx context.Context, workspace, projectName string) {
	projectDir := filepath.Join(workspace, projectName)
	if _, err := os.Stat(filepath.Join(projectDir, "Cargo.toml")); os.IsNotExist(err) {
		fmt.Printf("❌ 项目不存在: %s\n", projectName)
		return
	}

	fmt.Printf("🚀 运行项目: %s\n", projectName)

	cargoRun := tool.NewCargoRun()
	result, err := cargoRun.Run(ctx, fmt.Sprintf(`{"project_dir": "%s"}`,
		strings.ReplaceAll(projectDir, "\\", "\\\\")))
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	fmt.Println(result)
}

func fixProjectAutonomous(ctx context.Context, m model.Model, tools *tool.Registry, workspace, projectName string) {
	projectDir := filepath.Join(workspace, projectName)
	if _, err := os.Stat(filepath.Join(projectDir, "Cargo.toml")); os.IsNotExist(err) {
		fmt.Printf("❌ 项目不存在: %s\n", projectName)
		return
	}

	fmt.Printf("🔧 自主检查并修复项目: %s\n", projectName)

	// 先检查编译
	cargoCheck := tool.NewCargoCheck()
	result, _ := cargoCheck.Run(ctx, fmt.Sprintf(`{"project_dir": "%s"}`,
		strings.ReplaceAll(projectDir, "\\", "\\\\")))

	var checkResp struct {
		Success bool   `json:"success"`
		Stderr  string `json:"stderr"`
	}
	if err := parseJSON(result, &checkResp); err != nil {
		fmt.Printf("❌ 解析错误: %v\n", err)
		return
	}

	if checkResp.Success {
		fmt.Println("✅ 项目编译正常，无需修复")
		return
	}

	// 使用自主修复 Agent
	fixer := specialized.NewAutonomousFixerAgent(m, tools)
	eventChan, err := fixer.Run(ctx, "", agent.WithState(map[string]any{
		"project_dir":   projectDir,
		"compile_error": checkResp.Stderr,
	}))
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	for ev := range eventChan {
		handleEvent(ev)
	}
}

func fixProject(ctx context.Context, m model.Model, workspace, projectName string) {
	projectDir := filepath.Join(workspace, projectName)
	if _, err := os.Stat(filepath.Join(projectDir, "Cargo.toml")); os.IsNotExist(err) {
		fmt.Printf("❌ 项目不存在: %s\n", projectName)
		return
	}

	fmt.Printf("🔧 检查并修复项目: %s\n", projectName)

	// 先检查编译
	cargoCheck := tool.NewCargoCheck()
	result, _ := cargoCheck.Run(ctx, fmt.Sprintf(`{"project_dir": "%s"}`,
		strings.ReplaceAll(projectDir, "\\", "\\\\")))

	var checkResp struct {
		Success bool   `json:"success"`
		Stderr  string `json:"stderr"`
	}
	if err := parseJSON(result, &checkResp); err != nil {
		fmt.Printf("❌ 解析错误: %v\n", err)
		return
	}

	if checkResp.Success {
		fmt.Println("✅ 项目编译正常，无需修复")
		return
	}

	// 运行修复
	fixer := specialized.NewFixerAgent(m)
	eventChan, err := fixer.Run(ctx, "", agent.WithState(map[string]any{
		"project_dir":   projectDir,
		"compile_error": checkResp.Stderr,
	}))
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
	case event.TypeMCPCall:
		if ev.MCPCall != nil {
			fmt.Printf("🌐 MCP 调用: %s.%s\n", ev.MCPCall.Server, ev.MCPCall.Method)
		}
	case event.TypeCompletion:
		// 完成事件不打印
	case event.TypeState:
		if ev.NodeName != "" {
			fmt.Printf("📍 节点: %s\n", ev.NodeName)
		}
	}
}

func parseJSON(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

func printUsage() {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Agent v2 - 基于 trpc-agent-go 架构")
	fmt.Println("============================================================")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  rust-agent -i                        交互模式")
	fmt.Println("  rust-agent -create \"描述\"            创建完整项目")
	fmt.Println("  rust-agent -search \"关键词\"          搜索 crates")
	fmt.Println("  rust-agent -workspace \"目录\"         指定工作目录")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  rust-agent -create \"实现一个简单的 HTTP 服务器\"")
	fmt.Println("  rust-agent -create \"使用 rig 框架实现 Ollama 本地模型交互\"")
	fmt.Println("  rust-agent -search \"async http\"")
	fmt.Println("  rust-agent -i")
	fmt.Println()
	fmt.Println("完整工作流:")
	fmt.Println("  1. 📋 需求分析与规划 (Planner Agent)")
	fmt.Println("  2. 🔨 创建项目与代码 (Executor Agent)")
	fmt.Println("  3. 🔧 自动修复错误   (Fixer Agent)")
	fmt.Println("  4. 📦 输出结果")
	fmt.Println()
}
