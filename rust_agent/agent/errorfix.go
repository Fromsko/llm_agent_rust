package agent

import (
	"context"
	"fmt"
	"strings"

	"rust-agent/model"
	"rust-agent/tool"
)

// ErrorFixAgent 错误修复 Agent
type ErrorFixAgent struct {
	model *model.Model
}

// NewErrorFixAgent 创建错误修复 Agent
func NewErrorFixAgent(m *model.Model) *ErrorFixAgent {
	return &ErrorFixAgent{model: m}
}

const errorFixSystemPrompt = `你是一个 Rust 编译错误修复专家。你的任务是分析编译错误并修复代码。

## Rust 常见错误类型

### 所有权错误
- E0382: borrow of moved value - 值已被移动后再次使用
- E0502: cannot borrow as mutable because it is also borrowed as immutable
- E0499: cannot borrow as mutable more than once at a time

### 生命周期错误
- E0106: missing lifetime specifier
- E0597: borrowed value does not live long enough

### 类型错误
- E0308: mismatched types
- E0277: trait bound not satisfied

### 其他常见错误
- E0425: cannot find value in this scope
- E0433: failed to resolve: use of undeclared type or module

## 修复策略

1. **所有权问题**: 考虑使用 clone(), 引用 &, 或重构代码结构
2. **生命周期问题**: 添加生命周期标注，或使用 'static, Rc, Arc
3. **类型问题**: 检查类型转换，实现必要的 trait
4. **作用域问题**: 检查 use 语句，模块路径

## 输出格式

只输出修复后的完整代码，用 ` + "```rust" + ` 包裹。
在代码前简要说明修复了什么问题。`

// Fix 修复编译错误
func (a *ErrorFixAgent) Fix(ctx context.Context, code string, errors []tool.CompileError) (string, error) {
	errorsStr := tool.FormatErrors(errors)

	prompt := fmt.Sprintf(`请修复以下 Rust 代码中的编译错误：

## 原始代码
`+"```rust"+`
%s
`+"```"+`

## 编译错误
%s

请分析错误原因并提供修复后的完整代码。`, code, errorsStr)

	response, err := a.model.Chat(ctx, errorFixSystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return extractRustCode(response), nil
}

// FixWithContext 带上下文的修复
func (a *ErrorFixAgent) FixWithContext(ctx context.Context, code string, errors []tool.CompileError, context string) (string, error) {
	errorsStr := tool.FormatErrors(errors)

	prompt := fmt.Sprintf(`请修复以下 Rust 代码中的编译错误：

## 项目上下文
%s

## 原始代码
`+"```rust"+`
%s
`+"```"+`

## 编译错误
%s

请分析错误原因并提供修复后的完整代码。`, context, code, errorsStr)

	response, err := a.model.Chat(ctx, errorFixSystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return extractRustCode(response), nil
}

// ExplainError 解释错误
func (a *ErrorFixAgent) ExplainError(ctx context.Context, errors []tool.CompileError) (string, error) {
	errorsStr := tool.FormatErrors(errors)

	prompt := fmt.Sprintf(`请解释以下 Rust 编译错误：

%s

请用中文解释：
1. 每个错误的含义
2. 为什么会发生这个错误
3. 常见的修复方法`, errorsStr)

	return a.model.Chat(ctx, errorFixSystemPrompt, prompt)
}

// SuggestFix 建议修复方案（不直接修改代码）
func (a *ErrorFixAgent) SuggestFix(ctx context.Context, code string, errors []tool.CompileError) ([]string, error) {
	errorsStr := tool.FormatErrors(errors)

	prompt := fmt.Sprintf(`请为以下 Rust 代码的编译错误提供修复建议：

## 代码
`+"```rust"+`
%s
`+"```"+`

## 错误
%s

请列出具体的修复步骤，每个步骤一行，格式：
1. [修复内容]
2. [修复内容]
...`, code, errorsStr)

	response, err := a.model.Chat(ctx, errorFixSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	// 解析建议
	var suggestions []string
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && (strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") ||
			strings.HasPrefix(line, "3.") || strings.HasPrefix(line, "4.") ||
			strings.HasPrefix(line, "5.") || strings.HasPrefix(line, "-")) {
			suggestions = append(suggestions, line)
		}
	}

	return suggestions, nil
}
