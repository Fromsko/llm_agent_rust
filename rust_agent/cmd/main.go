package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"rust-agent/agent"
	"rust-agent/config"
)

func main() {
	// 命令行参数
	var (
		mode        string
		requirement string
		interactive bool
	)

	flag.StringVar(&mode, "mode", "generate", "模式: generate, fix, review, explain, interactive")
	flag.StringVar(&requirement, "req", "", "需求描述")
	flag.BoolVar(&interactive, "i", false, "交互模式")
	flag.Parse()

	// 创建 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️ 收到中断信号，正在退出...")
		cancel()
	}()

	// 创建 Agent
	rustAgent := agent.NewRustAgent(config.DefaultConfig)

	// 打印欢迎信息
	printWelcome()

	if interactive || mode == "interactive" {
		runInteractive(ctx, rustAgent)
	} else {
		runCommand(ctx, rustAgent, mode, requirement)
	}
}

func printWelcome() {
	fmt.Println("============================================================")
	fmt.Println("🦀 Rust Code Agent")
	fmt.Println("   智能 Rust 代码生成、修复、审查")
	fmt.Println("============================================================")
}

func runCommand(ctx context.Context, rustAgent *agent.RustAgent, mode, requirement string) {
	switch mode {
	case "generate":
		if requirement == "" {
			fmt.Println("❌ 请使用 -req 指定需求")
			fmt.Println("示例: rust-agent -mode generate -req \"实现一个简单的 HTTP 服务器\"")
			return
		}

		result, err := rustAgent.Generate(ctx, requirement)
		if err != nil {
			fmt.Printf("❌ 生成失败: %v\n", err)
			return
		}

		printResult(result)

	case "review":
		code := readStdin()
		if code == "" {
			fmt.Println("❌ 请通过 stdin 输入代码")
			fmt.Println("示例: cat main.rs | rust-agent -mode review")
			return
		}

		review, err := rustAgent.ReviewCode(ctx, code)
		if err != nil {
			fmt.Printf("❌ 审查失败: %v\n", err)
			return
		}

		fmt.Println("\n📋 代码审查结果:")
		fmt.Println(review)

	case "fix":
		code := readStdin()
		if code == "" {
			fmt.Println("❌ 请通过 stdin 输入代码")
			return
		}

		fixed, err := rustAgent.FixCode(ctx, code)
		if err != nil {
			fmt.Printf("❌ 修复失败: %v\n", err)
			return
		}

		fmt.Println("\n✅ 修复后的代码:")
		fmt.Println(fixed)

	case "explain":
		code := readStdin()
		if code == "" {
			fmt.Println("❌ 请通过 stdin 输入代码")
			return
		}

		explanation, err := rustAgent.ExplainCode(ctx, code)
		if err != nil {
			fmt.Printf("❌ 解释失败: %v\n", err)
			return
		}

		fmt.Println("\n📖 代码解释:")
		fmt.Println(explanation)

	default:
		fmt.Printf("❌ 未知模式: %s\n", mode)
		fmt.Println("可用模式: generate, fix, review, explain, interactive")
	}
}

func runInteractive(ctx context.Context, rustAgent *agent.RustAgent) {
	fmt.Println("\n🎮 交互模式")
	fmt.Println("命令:")
	fmt.Println("  gen <需求>  - 生成代码")
	fmt.Println("  fix        - 修复上次生成的代码")
	fmt.Println("  review     - 审查上次生成的代码")
	fmt.Println("  explain    - 解释上次生成的代码")
	fmt.Println("  test       - 为上次生成的代码生成测试")
	fmt.Println("  quit       - 退出")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	var lastCode string

	for {
		fmt.Print("🦀 > ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.SplitN(input, " ", 2)
		cmd := parts[0]

		switch cmd {
		case "gen", "generate":
			if len(parts) < 2 {
				fmt.Println("❌ 请指定需求")
				continue
			}
			requirement := parts[1]

			result, err := rustAgent.Generate(ctx, requirement)
			if err != nil {
				fmt.Printf("❌ 生成失败: %v\n", err)
				continue
			}

			lastCode = result.Code
			printResult(result)

		case "fix":
			if lastCode == "" {
				fmt.Println("❌ 没有可修复的代码，请先使用 gen 生成代码")
				continue
			}

			fixed, err := rustAgent.FixCode(ctx, lastCode)
			if err != nil {
				fmt.Printf("❌ 修复失败: %v\n", err)
				continue
			}

			lastCode = fixed
			fmt.Println("\n✅ 修复后的代码:")
			fmt.Println(fixed)

		case "review":
			if lastCode == "" {
				fmt.Println("❌ 没有可审查的代码")
				continue
			}

			review, err := rustAgent.ReviewCode(ctx, lastCode)
			if err != nil {
				fmt.Printf("❌ 审查失败: %v\n", err)
				continue
			}

			fmt.Println("\n📋 代码审查:")
			fmt.Println(review)

		case "explain":
			if lastCode == "" {
				fmt.Println("❌ 没有可解释的代码")
				continue
			}

			explanation, err := rustAgent.ExplainCode(ctx, lastCode)
			if err != nil {
				fmt.Printf("❌ 解释失败: %v\n", err)
				continue
			}

			fmt.Println("\n📖 代码解释:")
			fmt.Println(explanation)

		case "test":
			if lastCode == "" {
				fmt.Println("❌ 没有可测试的代码")
				continue
			}

			tests, err := rustAgent.GenerateTest(ctx, lastCode)
			if err != nil {
				fmt.Printf("❌ 生成测试失败: %v\n", err)
				continue
			}

			fmt.Println("\n🧪 生成的测试:")
			fmt.Println(tests)

		case "show":
			if lastCode == "" {
				fmt.Println("❌ 没有代码")
				continue
			}
			fmt.Println("\n📄 当前代码:")
			fmt.Println(lastCode)

		case "quit", "exit", "q":
			fmt.Println("👋 再见!")
			return

		case "help", "?":
			fmt.Println("命令:")
			fmt.Println("  gen <需求>  - 生成代码")
			fmt.Println("  fix        - 修复代码")
			fmt.Println("  review     - 审查代码")
			fmt.Println("  explain    - 解释代码")
			fmt.Println("  test       - 生成测试")
			fmt.Println("  show       - 显示当前代码")
			fmt.Println("  quit       - 退出")

		default:
			// 如果不是命令，当作生成需求
			result, err := rustAgent.Generate(ctx, input)
			if err != nil {
				fmt.Printf("❌ 生成失败: %v\n", err)
				continue
			}

			lastCode = result.Code
			printResult(result)
		}
	}
}

func printResult(result *agent.GenerateResult) {
	fmt.Println("\n============================================================")
	if result.Success {
		fmt.Println("✅ 生成成功!")
	} else {
		fmt.Println("❌ 生成失败")
	}
	fmt.Printf("📁 项目目录: %s\n", result.ProjectDir)
	fmt.Printf("🔄 迭代次数: %d\n", result.Iterations)

	if len(result.Errors) > 0 {
		fmt.Printf("⚠️ 剩余错误: %d\n", len(result.Errors))
	}

	fmt.Println("\n📄 生成的代码:")
	fmt.Println("------------------------------------------------------------")
	fmt.Println(result.Code)
	fmt.Println("------------------------------------------------------------")

	if result.RunOutput != "" {
		fmt.Println("\n📤 运行输出:")
		fmt.Println(result.RunOutput)
	}
	fmt.Println("============================================================")
}

func readStdin() string {
	// 检查是否有 stdin 输入
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return ""
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}
