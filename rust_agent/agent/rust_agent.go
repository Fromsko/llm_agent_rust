package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"rust-agent/config"
	"rust-agent/model"
	"rust-agent/tool"
)

// RustAgent 完整的 Rust 代码 Agent
type RustAgent struct {
	cfg *config.Config

	// 子 Agent
	codeGen  *CodeGenAgent
	errorFix *ErrorFixAgent
	review   *ReviewAgent

	// 工具
	cargo *tool.CargoTool

	// 状态
	projectDir string
}

// NewRustAgent 创建 Rust Agent
func NewRustAgent(cfg *config.Config) *RustAgent {
	m := model.New(cfg.Model)

	// 确保工作目录存在
	os.MkdirAll(cfg.Output.WorkDir, 0755)

	return &RustAgent{
		cfg:      cfg,
		codeGen:  NewCodeGenAgent(m),
		errorFix: NewErrorFixAgent(m),
		review:   NewReviewAgent(m),
		cargo:    tool.NewCargoTool(cfg.Rust, cfg.Output.WorkDir),
	}
}

// GenerateResult 生成结果
type GenerateResult struct {
	Success     bool
	Code        string
	ProjectDir  string
	BuildOutput string
	RunOutput   string
	Iterations  int
	Errors      []tool.CompileError
}

// Generate 生成代码（完整流程）
func (a *RustAgent) Generate(ctx context.Context, requirement string) (*GenerateResult, error) {
	result := &GenerateResult{}

	fmt.Println("🦀 Rust Agent 开始工作...")
	fmt.Printf("📝 需求: %s\n\n", requirement)

	// 1. 初始化项目
	fmt.Println("📦 初始化 Cargo 项目...")
	projectName := a.cfg.Output.ProjectName
	_, err := a.cargo.Init(ctx, projectName)
	if err != nil {
		return nil, fmt.Errorf("init project: %w", err)
	}
	a.projectDir = filepath.Join(a.cfg.Output.WorkDir, projectName)
	result.ProjectDir = a.projectDir

	// 2. 生成代码
	fmt.Println("🔨 生成代码...")
	code, err := a.codeGen.GenerateMainRs(ctx, requirement)
	if err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}
	result.Code = code

	// 3. 写入文件
	if err := a.cargo.WriteFile(a.projectDir, "src/main.rs", code); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	// 4. 编译检查和修复循环
	maxIterations := 5
	for i := 0; i < maxIterations; i++ {
		result.Iterations = i + 1
		fmt.Printf("\n🔍 编译检查 (第 %d 次)...\n", i+1)

		checkResult, err := a.cargo.Check(ctx, a.projectDir)
		if err != nil {
			return nil, fmt.Errorf("cargo check: %w", err)
		}

		result.BuildOutput = checkResult.Output
		result.Errors = checkResult.Errors

		if checkResult.Success {
			fmt.Println("✅ 编译通过!")
			result.Success = true
			break
		}

		fmt.Printf("❌ 发现 %d 个错误，尝试修复...\n", len(checkResult.Errors))

		// 读取当前代码
		currentCode, _ := a.cargo.ReadFile(a.projectDir, "src/main.rs")

		// 修复错误
		fixedCode, err := a.errorFix.Fix(ctx, currentCode, checkResult.Errors)
		if err != nil {
			fmt.Printf("⚠️ 修复失败: %v\n", err)
			continue
		}

		// 写入修复后的代码
		if err := a.cargo.WriteFile(a.projectDir, "src/main.rs", fixedCode); err != nil {
			return nil, fmt.Errorf("write fixed code: %w", err)
		}

		result.Code = fixedCode
	}

	// 5. 如果编译成功，尝试运行
	if result.Success {
		fmt.Println("\n🚀 运行程序...")
		runResult, err := a.cargo.Run(ctx, a.projectDir)
		if err == nil {
			result.RunOutput = runResult.Output
			fmt.Printf("📤 输出:\n%s\n", runResult.Output)
		}
	}

	return result, nil
}

// GenerateAndReview 生成并审查
func (a *RustAgent) GenerateAndReview(ctx context.Context, requirement string) (*GenerateResult, string, error) {
	result, err := a.Generate(ctx, requirement)
	if err != nil {
		return nil, "", err
	}

	if result.Success {
		fmt.Println("\n📋 代码审查...")
		review, err := a.review.Review(ctx, result.Code)
		if err != nil {
			return result, "", err
		}
		return result, review, nil
	}

	return result, "", nil
}

// FixCode 修复现有代码
func (a *RustAgent) FixCode(ctx context.Context, code string) (string, error) {
	// 创建临时项目
	projectName := "temp_fix"
	a.cargo.Init(ctx, projectName)
	projectDir := filepath.Join(a.cfg.Output.WorkDir, projectName)

	// 写入代码
	a.cargo.WriteFile(projectDir, "src/main.rs", code)

	// 检查
	checkResult, _ := a.cargo.Check(ctx, projectDir)
	if checkResult.Success {
		return code, nil // 代码已经正确
	}

	// 修复
	return a.errorFix.Fix(ctx, code, checkResult.Errors)
}

// ReviewCode 审查代码
func (a *RustAgent) ReviewCode(ctx context.Context, code string) (string, error) {
	return a.review.Review(ctx, code)
}

// ExplainCode 解释代码
func (a *RustAgent) ExplainCode(ctx context.Context, code string) (string, error) {
	m := model.New(a.cfg.Model)

	prompt := fmt.Sprintf(`请解释以下 Rust 代码：

`+"```rust"+`
%s
`+"```"+`

请解释：
1. 代码的整体功能
2. 关键数据结构和类型
3. 所有权和借用的流转
4. 重要的设计决策`, code)

	return m.Chat(ctx, "你是一个 Rust 教育专家，擅长解释 Rust 代码。", prompt)
}

// GenerateTest 为代码生成测试
func (a *RustAgent) GenerateTest(ctx context.Context, code string) (string, error) {
	return a.codeGen.GenerateTest(ctx, code)
}
