package agent

import (
	"context"
	"fmt"

	"rust-agent/model"
)

// ReviewAgent 代码审查 Agent
type ReviewAgent struct {
	model *model.Model
}

// NewReviewAgent 创建代码审查 Agent
func NewReviewAgent(m *model.Model) *ReviewAgent {
	return &ReviewAgent{model: m}
}

const reviewSystemPrompt = `你是一个 Rust 代码审查专家。你的任务是审查代码质量并提供改进建议。

## 审查维度

### 1. 正确性
- 所有权和借用是否正确
- 生命周期是否合理
- 错误处理是否完整

### 2. 安全性
- 是否有不必要的 unsafe
- 是否有潜在的 panic
- 是否有资源泄漏

### 3. 性能
- 是否有不必要的克隆
- 是否有不必要的堆分配
- 迭代器使用是否高效

### 4. 可读性
- 命名是否清晰
- 代码结构是否合理
- 注释是否充分

### 5. 惯用写法
- 是否遵循 Rust 惯例
- 是否使用了合适的标准库 API
- 是否符合 clippy 建议

## 输出格式

使用以下格式输出审查结果：

### 总体评分: X/10

### 优点
- ...

### 问题
1. [严重程度: 高/中/低] 问题描述
   建议: ...

### 改进建议
- ...`

// ReviewResult 审查结果
type ReviewResult struct {
	Score       int
	Strengths   []string
	Issues      []ReviewIssue
	Suggestions []string
	Summary     string
}

// ReviewIssue 审查问题
type ReviewIssue struct {
	Severity    string // high, medium, low
	Description string
	Suggestion  string
	Line        int
}

// Review 审查代码
func (a *ReviewAgent) Review(ctx context.Context, code string) (string, error) {
	prompt := fmt.Sprintf(`请审查以下 Rust 代码：

`+"```rust"+`
%s
`+"```"+`

请从正确性、安全性、性能、可读性、惯用写法五个维度进行审查。`, code)

	return a.model.Chat(ctx, reviewSystemPrompt, prompt)
}

// ReviewWithFocus 聚焦审查
func (a *ReviewAgent) ReviewWithFocus(ctx context.Context, code string, focus string) (string, error) {
	prompt := fmt.Sprintf(`请审查以下 Rust 代码，重点关注 %s：

`+"```rust"+`
%s
`+"```"+`

请详细分析 %s 方面的问题和改进建议。`, focus, code, focus)

	return a.model.Chat(ctx, reviewSystemPrompt, prompt)
}

// CheckOwnership 检查所有权
func (a *ReviewAgent) CheckOwnership(ctx context.Context, code string) (string, error) {
	prompt := fmt.Sprintf(`请分析以下 Rust 代码的所有权和借用：

`+"```rust"+`
%s
`+"```"+`

请分析：
1. 每个变量的所有权流转
2. 借用是否正确
3. 生命周期是否合理
4. 是否有潜在的所有权问题`, code)

	return a.model.Chat(ctx, reviewSystemPrompt, prompt)
}

// SuggestRefactor 建议重构
func (a *ReviewAgent) SuggestRefactor(ctx context.Context, code string) (string, error) {
	prompt := fmt.Sprintf(`请为以下 Rust 代码提供重构建议：

`+"```rust"+`
%s
`+"```"+`

请建议：
1. 如何提高代码可读性
2. 如何提高性能
3. 如何使代码更符合 Rust 惯例
4. 是否可以使用更好的抽象`, code)

	return a.model.Chat(ctx, reviewSystemPrompt, prompt)
}
