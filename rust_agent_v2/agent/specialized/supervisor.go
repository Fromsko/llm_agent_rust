package specialized

import (
	"context"
	"fmt"
	"strings"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/memory"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// SupervisorAgent 监督者 Agent - 负责全局协调、验证和反思
type SupervisorAgent struct {
	name            string
	model           model.Model
	tools           *tool.Registry
	workspace       string
	experienceStore *memory.ExperienceStore
	knowledgeBase   *memory.KnowledgeBase
	coderAgent      *AutonomousCoderAgent
}

func NewSupervisorAgent(m model.Model, tools *tool.Registry, workspace string) *SupervisorAgent {
	expStore, _ := memory.NewExperienceStore(workspace + "/.experience")

	return &SupervisorAgent{
		name:            "supervisor",
		model:           m,
		tools:           tools,
		workspace:       workspace,
		experienceStore: expStore,
		knowledgeBase:   memory.NewKnowledgeBase(),
		coderAgent:      NewAutonomousCoderAgent(m, tools, workspace),
	}
}

func (a *SupervisorAgent) Name() string { return a.name }

// TaskAnalysis 任务分析结果
type TaskAnalysis struct {
	RequiredCrates  []string // 需要的 crate
	TaskType        string   // 任务类型
	Constraints     []string // 约束条件
	SuccessCriteria []string // 成功标准
}

// Run 运行监督者
func (a *SupervisorAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		// 1. 分析任务
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 1, 6, "📋 分析任务..."))
		analysis := a.analyzeTask(input)

		// 2. 检索相关经验
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 2, 6, "📚 检索历史经验..."))
		experienceContext := a.gatherExperience(analysis.RequiredCrates)

		// 3. 获取知识库信息
		knowledgeContext := a.gatherKnowledge(analysis.RequiredCrates)

		// 4. 构建增强的任务描述
		enhancedTask := a.buildEnhancedTask(input, analysis, experienceContext, knowledgeContext)
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
			fmt.Sprintf("📋 任务分析完成\n- 需要的库: %v\n- 任务类型: %s\n- 已加载知识: %d 条\n- 已加载经验: %d 条",
				analysis.RequiredCrates, analysis.TaskType,
				len(analysis.RequiredCrates), len(a.experienceStore.FindByCrate("")))))

		// 5. 执行编码任务
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 3, 6, "🔨 执行编码任务..."))
		result := a.executeWithValidation(ctx, enhancedTask, analysis, eventChan)

		// 6. 验证和反思
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 4, 6, "🔍 验证结果..."))
		a.validateAndReflect(ctx, result, analysis, eventChan)

		// 7. 如果失败，尝试自动修复
		if !result.Success && result.ProjectDir != "" {
			event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 5, 6, "🔧 尝试自动修复..."))
			result = a.attemptAutoFix(ctx, result, analysis, eventChan)
		}

		// 8. 保存经验
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 6, 6, "💾 保存经验..."))
		a.saveExperience(result, analysis)

		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{
			"success": result.Success,
			"project": result.ProjectDir,
		}))
	}()

	return eventChan, nil
}

// gatherKnowledge 收集知识库信息
func (a *SupervisorAgent) gatherKnowledge(crates []string) string {
	var sb strings.Builder

	for _, crate := range crates {
		if knowledge := a.knowledgeBase.FormatForPrompt(crate); knowledge != "" {
			sb.WriteString(knowledge)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// attemptAutoFix 尝试自动修复
func (a *SupervisorAgent) attemptAutoFix(ctx context.Context, result *ExecutionResult, analysis *TaskAnalysis, eventChan chan<- *event.Event) *ExecutionResult {
	if result.ProjectDir == "" {
		return result
	}

	// 获取编译错误
	checkResult, _ := tool.NewCargoCheck().Run(ctx, fmt.Sprintf(`{"project_dir":"%s"}`, result.ProjectDir))
	if !strings.Contains(checkResult, "error[") {
		result.Success = true
		return result
	}

	event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "🔧 检测到编译错误，启动自动修复..."))

	// 使用 AutonomousFixerAgent 修复
	fixer := NewAutonomousFixerAgent(a.model, a.tools)
	fixerChan, err := fixer.Run(ctx, "", agent.WithState(map[string]any{
		"project_dir":   result.ProjectDir,
		"compile_error": checkResult,
	}))
	if err != nil {
		return result
	}

	for ev := range fixerChan {
		event.EmitEvent(ctx, eventChan, ev)
		if ev.Type == event.TypeCompletion && ev.Completion != nil {
			if success, ok := ev.Completion.Result.(map[string]any)["success"].(bool); ok {
				result.Success = success
			}
		}
	}

	return result
}

// analyzeTask 分析任务
func (a *SupervisorAgent) analyzeTask(input string) *TaskAnalysis {
	analysis := &TaskAnalysis{
		TaskType: "code_generation",
	}

	// 提取 crate 名称
	input = strings.ToLower(input)

	// 常见 crate 检测
	cratePatterns := map[string][]string{
		"rmcp":     {"rmcp", "rust-sdk", "modelcontextprotocol", "mcp"},
		"rig-core": {"rig-core", "rig"},
		"tokio":    {"tokio", "async", "异步"},
		"reqwest":  {"reqwest", "http client", "http 客户端"},
		"serde":    {"serde", "json", "序列化"},
	}

	for crate, patterns := range cratePatterns {
		for _, pattern := range patterns {
			if strings.Contains(input, pattern) {
				analysis.RequiredCrates = append(analysis.RequiredCrates, crate)
				break
			}
		}
	}

	// 提取约束条件
	if strings.Contains(input, "必须") || strings.Contains(input, "要求") {
		analysis.Constraints = append(analysis.Constraints, "用户有明确要求")
	}

	// 成功标准
	analysis.SuccessCriteria = []string{
		"cargo check 编译通过",
		"使用了指定的库",
		"实现了所有要求的功能",
	}

	return analysis
}

// gatherExperience 收集相关经验
func (a *SupervisorAgent) gatherExperience(crates []string) string {
	var sb strings.Builder

	for _, crate := range crates {
		// 从知识库获取
		if knowledge := a.knowledgeBase.FormatForPrompt(crate); knowledge != "" {
			sb.WriteString(knowledge)
			sb.WriteString("\n")
		}

		// 从经验库获取
		if experience := a.experienceStore.FormatForPrompt(crate); experience != "" {
			sb.WriteString(experience)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// buildEnhancedTask 构建增强的任务描述
func (a *SupervisorAgent) buildEnhancedTask(originalTask string, analysis *TaskAnalysis, experience string, knowledge string) string {
	var sb strings.Builder

	sb.WriteString(originalTask)
	sb.WriteString("\n\n")

	// 添加知识库信息（优先级最高）
	if knowledge != "" {
		sb.WriteString("---\n")
		sb.WriteString("## 📚 知识库信息（重要！请务必参考）\n\n")
		sb.WriteString(knowledge)
	}

	// 添加历史经验
	if experience != "" {
		sb.WriteString("\n---\n")
		sb.WriteString("## 📖 历史经验\n\n")
		sb.WriteString(experience)
	}

	if len(analysis.Constraints) > 0 {
		sb.WriteString("\n---\n## ⚠️ 约束条件\n")
		for _, c := range analysis.Constraints {
			sb.WriteString("- " + c + "\n")
		}
	}

	sb.WriteString("\n---\n## ✅ 成功标准\n")
	for _, c := range analysis.SuccessCriteria {
		sb.WriteString("- " + c + "\n")
	}

	return sb.String()
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Success    bool
	ProjectDir string
	FinalCode  string
	Imports    []string
	Errors     []string
	Iterations int
}

// executeWithValidation 带验证的执行
func (a *SupervisorAgent) executeWithValidation(ctx context.Context, task string, analysis *TaskAnalysis, eventChan chan<- *event.Event) *ExecutionResult {
	result := &ExecutionResult{}

	// 执行编码 Agent
	coderChan, err := a.coderAgent.Run(ctx, task)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	// 收集事件
	for ev := range coderChan {
		// 转发事件
		event.EmitEvent(ctx, eventChan, ev)

		// 分析结果
		if ev.Type == event.TypeCompletion && ev.Completion != nil {
			if resultMap, ok := ev.Completion.Result.(map[string]any); ok {
				if success, ok := resultMap["success"].(bool); ok {
					result.Success = success
				}
				if projectDir, ok := resultMap["project_dir"].(string); ok {
					result.ProjectDir = projectDir
				}
			}
		}
	}

	// 如果没有从 completion 获取到项目目录，尝试查找
	if result.ProjectDir == "" {
		possibleDirs := []string{}

		// 根据任务分析的 crate 名称构建可能的目录
		for _, crate := range analysis.RequiredCrates {
			possibleDirs = append(possibleDirs,
				a.workspace+"/"+strings.ReplaceAll(crate, "-", "_")+"_project",
				a.workspace+"/"+crate+"_project",
				a.workspace+"/mcp_math_tools",
				a.workspace+"/"+strings.ReplaceAll(crate, "-", "_"),
			)
		}

		// 检查哪个目录存在且有 Cargo.toml
		for _, dir := range possibleDirs {
			checkResult, _ := tool.NewCargoCheck().Run(ctx, fmt.Sprintf(`{"project_dir":"%s"}`, strings.ReplaceAll(dir, "\\", "\\\\")))
			if checkResult != "" {
				result.ProjectDir = dir
				result.Success = strings.Contains(checkResult, `"success":true`) && !strings.Contains(checkResult, "error[")
				break
			}
		}
	}

	// 最终验证
	if result.ProjectDir != "" && !result.Success {
		checkResult, _ := tool.NewCargoCheck().Run(ctx, fmt.Sprintf(`{"project_dir":"%s"}`, strings.ReplaceAll(result.ProjectDir, "\\", "\\\\")))
		result.Success = strings.Contains(checkResult, `"success":true`) && !strings.Contains(checkResult, "error[")

		// 记录错误
		if !result.Success && strings.Contains(checkResult, "error[") {
			result.Errors = append(result.Errors, checkResult)
		}
	}

	return result
}

// validateAndReflect 验证和反思
func (a *SupervisorAgent) validateAndReflect(ctx context.Context, result *ExecutionResult, analysis *TaskAnalysis, eventChan chan<- *event.Event) {
	if result.Success {
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "✅ 验证通过！项目编译成功"))
		return
	}

	// 反思失败原因
	event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "❌ 验证失败，开始反思..."))

	reflections := []string{}

	// 检查是否使用了正确的库
	for _, crate := range analysis.RequiredCrates {
		if knowledge := a.knowledgeBase.Get(crate); knowledge != nil {
			if knowledge.CodeName != knowledge.CargoName {
				reflections = append(reflections,
					fmt.Sprintf("注意：%s 在代码中应该用 %s 而不是 %s",
						crate, knowledge.CodeName, knowledge.CargoName))
			}
		}
	}

	// 输出反思
	if len(reflections) > 0 {
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
			"💡 反思结果：\n"+strings.Join(reflections, "\n")))
	}
}

// saveExperience 保存经验 - 事无巨细的分析
func (a *SupervisorAgent) saveExperience(result *ExecutionResult, analysis *TaskAnalysis) {
	if len(analysis.RequiredCrates) == 0 {
		return
	}

	ctx := context.Background()

	// 尝试找到项目目录
	projectDir := result.ProjectDir
	if projectDir == "" {
		// 尝试常见的项目目录命名
		for _, crate := range analysis.RequiredCrates {
			possibleDirs := []string{
				a.workspace + "/" + strings.ReplaceAll(crate, "-", "_") + "_project",
				a.workspace + "/" + crate + "_project",
				a.workspace + "/mcp_math_tools",
			}
			for _, dir := range possibleDirs {
				mainRs := dir + "/src/main.rs"
				if content, err := a.readFile(mainRs); err == nil && content != "" {
					projectDir = dir
					break
				}
			}
			if projectDir != "" {
				break
			}
		}
	}

	// 读取并分析代码
	var code string
	var imports []string
	var apiUsage []string
	var cargoToml string

	if projectDir != "" {
		// 读取 main.rs
		if content, err := a.readFile(projectDir + "/src/main.rs"); err == nil {
			code = content
			imports, apiUsage = a.analyzeRustCode(content)
		}

		// 读取 Cargo.toml
		if content, err := a.readFile(projectDir + "/Cargo.toml"); err == nil {
			cargoToml = content
		}
	}

	// 构建经验记录
	exp := &memory.Experience{
		Task:      analysis.TaskType,
		CrateName: analysis.RequiredCrates[0],
		Success:   result.Success,
		Code:      code,
		Imports:   imports,
		APIUsage:  apiUsage,
		Errors:    result.Errors,
		Tags:      analysis.RequiredCrates,
		Metadata:  make(map[string]string),
	}

	// 保存 Cargo.toml 信息
	if cargoToml != "" {
		exp.Metadata["cargo_toml"] = cargoToml
	}

	// 分析并添加教训
	exp.Lessons = a.extractLessons(ctx, result, analysis, code)

	// 保存经验
	if err := a.experienceStore.Save(ctx, exp); err == nil {
		fmt.Printf("💾 经验已保存: %s (成功: %v)\n", exp.CrateName, exp.Success)
	}
}

// readFile 读取文件内容
func (a *SupervisorAgent) readFile(path string) (string, error) {
	fileRead := tool.NewFileRead()
	result, err := fileRead.Run(context.Background(), fmt.Sprintf(`{"path":"%s"}`, strings.ReplaceAll(path, "\\", "\\\\")))
	if err != nil {
		return "", err
	}

	// 解析结果
	if strings.Contains(result, `"success":true`) {
		// 提取 content 字段
		start := strings.Index(result, `"content":"`)
		if start != -1 {
			start += len(`"content":"`)
			end := strings.LastIndex(result, `","success"`)
			if end > start {
				content := result[start:end]
				// 处理转义字符
				content = strings.ReplaceAll(content, `\n`, "\n")
				content = strings.ReplaceAll(content, `\"`, `"`)
				content = strings.ReplaceAll(content, `\\`, `\`)
				return content, nil
			}
		}
	}
	return "", fmt.Errorf("failed to read file")
}

// analyzeRustCode 分析 Rust 代码，提取 imports 和 API 用法
func (a *SupervisorAgent) analyzeRustCode(code string) (imports []string, apiUsage []string) {
	lines := strings.Split(code, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 提取 use 语句
		if strings.HasPrefix(line, "use ") {
			imports = append(imports, line)
		}

		// 提取宏使用
		if strings.Contains(line, "#[") && !strings.HasPrefix(line, "//") {
			apiUsage = append(apiUsage, line)
		}

		// 提取重要的 API 调用模式
		patterns := []string{
			".serve(",
			".waiting(",
			"::new(",
			"::default(",
			"impl ServerHandler",
			"impl ClientHandler",
			"#[tool(",
			"#[tool_router]",
			"CallToolResult::",
			"Content::text(",
		}
		for _, pattern := range patterns {
			if strings.Contains(line, pattern) {
				apiUsage = append(apiUsage, line)
				break
			}
		}
	}

	// 去重
	imports = uniqueStrings(imports)
	apiUsage = uniqueStrings(apiUsage)

	return
}

// extractLessons 提取教训
func (a *SupervisorAgent) extractLessons(ctx context.Context, result *ExecutionResult, analysis *TaskAnalysis, code string) []string {
	var lessons []string

	if !result.Success {
		lessons = append(lessons, "任务未能成功完成，需要更多调试")

		// 分析错误原因
		for _, err := range result.Errors {
			if strings.Contains(err, "not found") {
				lessons = append(lessons, "遇到找不到模块/函数的错误，需要检查 use 语句")
			}
			if strings.Contains(err, "trait") {
				lessons = append(lessons, "遇到 trait 相关错误，需要确认正确导入 trait")
			}
		}
	} else {
		// 成功的经验
		if code != "" {
			// 分析成功的代码模式
			if strings.Contains(code, "#[tool_router]") {
				lessons = append(lessons, "使用 #[tool_router] 宏定义工具路由")
			}
			if strings.Contains(code, "impl ServerHandler") {
				lessons = append(lessons, "需要实现 ServerHandler trait")
			}
			if strings.Contains(code, ".serve(stdio())") {
				lessons = append(lessons, "使用 .serve(stdio()) 启动 stdio 服务")
			}
			if strings.Contains(code, "CallToolResult::success") {
				lessons = append(lessons, "使用 CallToolResult::success 返回成功结果")
			}
		}
	}

	return lessons
}

// uniqueStrings 去重字符串切片
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
