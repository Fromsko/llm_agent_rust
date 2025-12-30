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

// FixerAgent 修复 Agent - 自动修复编译错误
type FixerAgent struct {
	name  string
	model model.Model
}

func NewFixerAgent(m model.Model) *FixerAgent {
	return &FixerAgent{name: "fixer-agent", model: m}
}

func (a *FixerAgent) Name() string { return a.name }

const fixerPrompt = `你是一个 Rust 编译错误修复专家。

分析编译错误，提供修复后的完整代码。

输出必须是 JSON 格式：
{
  "analysis": "错误分析",
  "fixes": [
    {
      "file": "文件路径（如 Cargo.toml 或 src/main.rs）",
      "code": "修复后的完整代码"
    }
  ]
}

常见错误修复：
1. 依赖版本错误 - 修改 Cargo.toml 中的版本号
2. 依赖不存在 - 使用正确的 crate 名称和版本
3. 语法错误 - 修复 Rust 代码语法
4. 类型错误 - 修复类型不匹配

重要提示：
- rig-core 是正确的 crate 名称，不是 rig
- ollama-rs 是 Ollama 的 Rust 客户端
- 使用 crates.io 上实际存在的版本

规则：
1. 代码必须完整，不能省略
2. 必须修复所有错误
3. 只输出 JSON，不要其他内容`

type FixResult struct {
	Analysis string `json:"analysis"`
	Fixes    []struct {
		File string `json:"file"`
		Code string `json:"code"`
	} `json:"fixes"`
}

func (a *FixerAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		invOpts := agent.ApplyOptions(opts...)
		projectDir, _ := invOpts.State["project_dir"].(string)
		compileError, _ := invOpts.State["compile_error"].(string)

		if projectDir == "" || compileError == "" {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "NO_ERROR", "没有需要修复的错误"))
			return
		}

		maxAttempts := 5
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, attempt, maxAttempts,
				fmt.Sprintf("修复尝试 %d/%d", attempt, maxAttempts)))

			// 读取当前代码
			currentCode := readProjectFiles(projectDir)

			// 让 LLM 分析并修复
			messages := []*model.Message{
				model.NewSystemMessage(fixerPrompt),
				model.NewUserMessage(fmt.Sprintf("编译错误:\n```\n%s\n```\n\n当前代码:\n%s\n\n请修复所有错误。",
					compileError, currentCode)),
			}

			resp, err := a.model.Generate(ctx, messages)
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
				return
			}

			// 解析修复结果
			var fixResult FixResult
			if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &fixResult); err != nil {
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "⚠️ 无法解析修复结果，重试..."))
				continue
			}

			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
				fmt.Sprintf("🔍 分析: %s", fixResult.Analysis)))

			// 应用修复
			for _, fix := range fixResult.Fixes {
				filePath := filepath.Join(projectDir, fix.File)
				if err := os.WriteFile(filePath, []byte(fix.Code), 0644); err != nil {
					event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "WRITE_ERROR", err.Error()))
					return
				}
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
					fmt.Sprintf("✏️ 已修复 %s", fix.File)))
			}

			// 重新编译检查
			cargoCheck := tool.NewCargoCheck()
			checkResult, _ := cargoCheck.Run(ctx, fmt.Sprintf(`{"project_dir": "%s"}`,
				strings.ReplaceAll(projectDir, "\\", "\\\\")))

			var checkResp struct {
				Success bool   `json:"success"`
				Stderr  string `json:"stderr"`
			}
			json.Unmarshal([]byte(checkResult), &checkResp)

			if checkResp.Success {
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "✅ 编译通过！"))
				event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{
					"success":  true,
					"attempts": attempt,
				}))
				return
			}

			compileError = checkResp.Stderr
			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
				fmt.Sprintf("⚠️ 仍有错误，继续修复...\n```\n%s\n```", truncateError(compileError))))
		}

		event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "MAX_ATTEMPTS", "达到最大修复尝试次数"))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{"success": false}))
	}()

	return eventChan, nil
}

func readProjectFiles(projectDir string) string {
	var sb strings.Builder

	// 读取 Cargo.toml
	cargoPath := filepath.Join(projectDir, "Cargo.toml")
	if content, err := os.ReadFile(cargoPath); err == nil {
		sb.WriteString(fmt.Sprintf("// === Cargo.toml ===\n```toml\n%s\n```\n\n", string(content)))
	}

	// 读取源文件
	filepath.Walk(filepath.Join(projectDir, "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".rs") {
			relPath, _ := filepath.Rel(projectDir, path)
			content, _ := os.ReadFile(path)
			sb.WriteString(fmt.Sprintf("// === %s ===\n```rust\n%s\n```\n\n", relPath, string(content)))
		}
		return nil
	})

	return sb.String()
}

func truncateError(s string) string {
	if len(s) > 2000 {
		return s[:2000] + "\n...[truncated]"
	}
	return s
}
