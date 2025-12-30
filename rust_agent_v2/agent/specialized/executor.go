package specialized

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// ExecutorAgent 执行 Agent - 根据计划实际创建项目和代码
type ExecutorAgent struct {
	name      string
	model     model.Model
	tools     *tool.Registry
	workspace string
}

func NewExecutorAgent(m model.Model, tools *tool.Registry, workspace string) *ExecutorAgent {
	return &ExecutorAgent{
		name:      "executor-agent",
		model:     m,
		tools:     tools,
		workspace: workspace,
	}
}

func (a *ExecutorAgent) Name() string { return a.name }

const codeGenSystemPrompt = `你是一个专业的 Rust 代码生成专家。

根据提供的文件规格，生成完整、可编译、可运行的 Rust 代码。

规则：
1. 代码必须完整，不能有省略
2. 必须包含所有必要的 use 语句
3. 必须正确处理错误（使用 Result/Option）
4. 添加适当的注释
5. 只输出代码，不要其他解释
6. 代码必须能直接编译通过`

func (a *ExecutorAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		// 从 state 获取计划
		invOpts := agent.ApplyOptions(opts...)
		planData, ok := invOpts.State["plan"]
		if !ok {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "NO_PLAN", "没有执行计划"))
			return
		}

		plan, ok := planData.(*Plan)
		if !ok {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "INVALID_PLAN", "计划格式无效"))
			return
		}

		projectDir := filepath.Join(a.workspace, plan.ProjectName)
		totalSteps := 3 + len(plan.Files)
		currentStep := 0

		// Step 1: 创建项目
		currentStep++
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, currentStep, totalSteps, "创建 Cargo 项目"))

		cargoInit := tool.NewCargoInit()
		result, err := cargoInit.Run(ctx, fmt.Sprintf(`{"work_dir": "%s", "project_name": "%s"}`,
			strings.ReplaceAll(a.workspace, "\\", "\\\\"),
			plan.ProjectName))
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "CARGO_INIT_ERROR", err.Error()))
			return
		}
		event.EmitEvent(ctx, eventChan, event.NewToolCallEvent(a.name, "cargo_init", map[string]any{"result": result}))

		// Step 2: 生成 Cargo.toml
		currentStep++
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, currentStep, totalSteps, "配置 Cargo.toml"))

		cargoToml := generateCargoToml(plan)
		cargoTomlPath := filepath.Join(projectDir, "Cargo.toml")
		if err := os.WriteFile(cargoTomlPath, []byte(cargoToml), 0644); err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "WRITE_ERROR", err.Error()))
			return
		}
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, fmt.Sprintf("✅ 已生成 Cargo.toml\n```toml\n%s\n```", cargoToml)))

		// Step 3+: 生成每个源文件
		for _, fileSpec := range plan.Files {
			currentStep++
			event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, currentStep, totalSteps,
				fmt.Sprintf("生成 %s", fileSpec.Path)))

			code, err := a.generateCode(ctx, plan, fileSpec)
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "CODEGEN_ERROR", err.Error()))
				return
			}

			filePath := filepath.Join(projectDir, fileSpec.Path)
			dir := filepath.Dir(filePath)
			os.MkdirAll(dir, 0755)

			if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "WRITE_ERROR", err.Error()))
				return
			}

			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
				fmt.Sprintf("✅ 已生成 %s\n```rust\n%s\n```", fileSpec.Path, code)))
		}

		// Step final: 编译检查
		currentStep++
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, currentStep, totalSteps, "编译检查"))

		cargoCheck := tool.NewCargoCheck()
		checkResult, _ := cargoCheck.Run(ctx, fmt.Sprintf(`{"project_dir": "%s"}`,
			strings.ReplaceAll(projectDir, "\\", "\\\\")))

		var checkResp struct {
			Success bool   `json:"success"`
			Stderr  string `json:"stderr"`
		}
		json.Unmarshal([]byte(checkResult), &checkResp)

		if checkResp.Success {
			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "✅ 编译检查通过！"))
		} else {
			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
				fmt.Sprintf("⚠️ 编译有错误，需要修复:\n```\n%s\n```", checkResp.Stderr)))
		}

		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{
			"project_dir":   projectDir,
			"success":       checkResp.Success,
			"compile_error": checkResp.Stderr,
		}))
	}()

	return eventChan, nil
}

func (a *ExecutorAgent) generateCode(ctx context.Context, plan *Plan, fileSpec FileSpec) (string, error) {
	// 构建依赖信息
	var depsInfo strings.Builder
	for _, dep := range plan.Dependencies {
		depsInfo.WriteString(fmt.Sprintf("- %s = \"%s\"", dep.Name, dep.Version))
		if len(dep.Features) > 0 {
			depsInfo.WriteString(fmt.Sprintf(" (features: %v)", dep.Features))
		}
		depsInfo.WriteString("\n")
	}

	prompt := fmt.Sprintf(`项目: %s
描述: %s

依赖:
%s

文件: %s
用途: %s
需要实现的组件: %v

请生成完整的 Rust 代码。`, plan.ProjectName, plan.Description, depsInfo.String(),
		fileSpec.Path, fileSpec.Purpose, fileSpec.KeyComponents)

	messages := []*model.Message{
		model.NewSystemMessage(codeGenSystemPrompt),
		model.NewUserMessage(prompt),
	}

	resp, err := a.model.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	return extractCode(resp.Content), nil
}

func generateCargoToml(plan *Plan) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`[package]
name = "%s"
version = "0.1.0"
edition = "2021"
description = "%s"

[dependencies]
`, plan.ProjectName, plan.Description))

	for _, dep := range plan.Dependencies {
		if len(dep.Features) > 0 {
			sb.WriteString(fmt.Sprintf("%s = { version = \"%s\", features = [", dep.Name, dep.Version))
			for i, f := range dep.Features {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("\"%s\"", f))
			}
			sb.WriteString("] }\n")
		} else {
			sb.WriteString(fmt.Sprintf("%s = \"%s\"\n", dep.Name, dep.Version))
		}
	}

	return sb.String()
}

func extractCode(text string) string {
	// 提取 ```rust ... ``` 之间的代码
	start := strings.Index(text, "```rust")
	if start == -1 {
		start = strings.Index(text, "```")
	}
	if start == -1 {
		return text
	}

	start = strings.Index(text[start:], "\n") + start + 1
	end := strings.LastIndex(text, "```")
	if end <= start {
		return text[start:]
	}

	return strings.TrimSpace(text[start:end])
}
