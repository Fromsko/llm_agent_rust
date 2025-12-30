package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"rust-agent/model"
)

// CodeGenAgent 代码生成 Agent
type CodeGenAgent struct {
	model *model.Model
}

// NewCodeGenAgent 创建代码生成 Agent
func NewCodeGenAgent(m *model.Model) *CodeGenAgent {
	return &CodeGenAgent{model: m}
}

const codeGenSystemPrompt = `你是一个 Rust 编程专家。你的任务是生成高质量的 Rust 代码。

## 核心原则

1. **所有权系统**: 正确处理 ownership, borrowing, lifetime
2. **错误处理**: 使用 Result<T, E> 和 ? 操作符
3. **惯用写法**: 遵循 Rust 社区最佳实践
4. **安全性**: 避免 unsafe，除非绝对必要
5. **性能**: 零成本抽象，避免不必要的克隆

## 输出格式

只输出代码，用 ` + "```rust" + ` 和 ` + "```" + ` 包裹。
不要输出解释，除非被要求。

## 代码规范

- 使用 4 空格缩进
- 函数和类型添加文档注释 (///)
- 公共 API 添加 #[must_use] 等属性
- 错误类型实现 std::error::Error
- 使用 clippy 推荐的写法`

// Generate 生成代码
func (a *CodeGenAgent) Generate(ctx context.Context, requirement string) (string, error) {
	prompt := fmt.Sprintf(`请根据以下需求生成 Rust 代码：

%s

要求：
1. 代码必须能通过 cargo check
2. 包含必要的 use 语句
3. 添加适当的文档注释
4. 处理所有可能的错误`, requirement)

	response, err := a.model.Chat(ctx, codeGenSystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return extractRustCode(response), nil
}

// GenerateFunction 生成函数
func (a *CodeGenAgent) GenerateFunction(ctx context.Context, signature, description string) (string, error) {
	prompt := fmt.Sprintf(`请实现以下 Rust 函数：

函数签名: %s
功能描述: %s

要求：
1. 正确处理所有权和借用
2. 使用 Result 处理可能的错误
3. 添加文档注释和示例`, signature, description)

	response, err := a.model.Chat(ctx, codeGenSystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return extractRustCode(response), nil
}

// GenerateStruct 生成结构体
func (a *CodeGenAgent) GenerateStruct(ctx context.Context, name, description string, fields []string) (string, error) {
	fieldsStr := strings.Join(fields, "\n- ")
	prompt := fmt.Sprintf(`请生成一个 Rust 结构体：

名称: %s
描述: %s
字段:
- %s

要求：
1. 实现 Debug, Clone 等常用 trait
2. 添加 new() 构造函数
3. 为每个字段添加 getter/setter（如果需要）
4. 添加文档注释`, name, description, fieldsStr)

	response, err := a.model.Chat(ctx, codeGenSystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return extractRustCode(response), nil
}

// GenerateTest 生成测试
func (a *CodeGenAgent) GenerateTest(ctx context.Context, code string) (string, error) {
	prompt := fmt.Sprintf(`请为以下 Rust 代码生成单元测试：

%s

要求：
1. 测试所有公共函数
2. 包含正常情况和边界情况
3. 使用 #[test] 属性
4. 测试错误处理路径`, code)

	response, err := a.model.Chat(ctx, codeGenSystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return extractRustCode(response), nil
}

// GenerateMainRs 生成完整的 main.rs
func (a *CodeGenAgent) GenerateMainRs(ctx context.Context, requirement string) (string, error) {
	prompt := fmt.Sprintf(`请生成一个完整的 Rust main.rs 文件：

需求: %s

要求：
1. 包含 main 函数
2. 包含所有必要的 use 语句
3. 代码结构清晰
4. 添加适当的注释
5. 能直接通过 cargo run 运行`, requirement)

	response, err := a.model.Chat(ctx, codeGenSystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return extractRustCode(response), nil
}

// extractRustCode 从响应中提取 Rust 代码
func extractRustCode(response string) string {
	// 尝试提取 ```rust ... ``` 块
	re := regexp.MustCompile("(?s)```rust\\s*\\n(.+?)\\n```")
	matches := re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// 尝试提取 ``` ... ``` 块
	re = regexp.MustCompile("(?s)```\\s*\\n(.+?)\\n```")
	matches = re.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// 如果没有代码块，返回原始响应
	return strings.TrimSpace(response)
}
