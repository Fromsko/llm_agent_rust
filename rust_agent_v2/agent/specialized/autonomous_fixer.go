package specialized

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// AutonomousFixerAgent 自主修复 Agent - 拥有工具调用能力，自主决策修复策略
type AutonomousFixerAgent struct {
	name  string
	model model.Model
	tools *tool.Registry
}

func NewAutonomousFixerAgent(m model.Model, tools *tool.Registry) *AutonomousFixerAgent {
	return &AutonomousFixerAgent{
		name:  "autonomous-fixer",
		model: m,
		tools: tools,
	}
}

func (a *AutonomousFixerAgent) Name() string { return a.name }

const autonomousFixerPrompt = `你是 Rust 代码修复专家。严格按照流程执行，不要跳过任何步骤。

## 工具列表
- rust_error_analyzer: 分析错误
- file_read: 读取文件
- file_write: 写入完整代码
- cargo_check: 验证编译

## 强制执行流程（必须按顺序）

步骤1: 调用 rust_error_analyzer 分析错误
步骤2: 调用 file_read 读取原始代码
步骤3: 调用 file_write 写入修复后的完整代码
步骤4: 调用 cargo_check 验证
步骤5: 检查 cargo_check 结果
  - 如果 stderr 包含 "error" → 回到步骤3继续修复
  - 如果没有 error → 输出 "FIXED"

## 绝对禁止
- 禁止跳过 cargo_check
- 禁止在 cargo_check 有 error 时说 FIXED
- 禁止删除或改变原有功能代码

## E0425 修复方法（io::read/io::write 不存在）

错误原因: tokio::io 模块没有 read/write 函数
正确做法: 使用 AsyncReadExt/AsyncWriteExt trait

修复前:
use tokio::io;
let n = io::read(&socket, &mut buf).await.unwrap();
io::write(&socket, data).await.unwrap();

修复后:
use tokio::io::{AsyncReadExt, AsyncWriteExt};
let n = socket.read(&mut buf).await.unwrap();
socket.write_all(data).await.unwrap();

关键点:
1. 导入 AsyncReadExt, AsyncWriteExt
2. 删除 use tokio::io; 或改为 use tokio::io::{AsyncReadExt, AsyncWriteExt};
3. io::read(&socket, buf) 改为 socket.read(buf)
4. io::write(&socket, data) 改为 socket.write_all(data)
5. socket 必须是 mut: let (mut socket, _) = ...

## 完整修复示例

原始错误代码:
use tokio::net::TcpListener;
use tokio::io;

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind("127.0.0.1:8080").await?;
    loop {
        let (socket, addr) = listener.accept().await?;
        tokio::spawn(async move {
            let mut buf = vec![0u8; 1024];
            let n = io::read(&socket, &mut buf).await.unwrap();
            io::write(&socket, b"Hello").await.unwrap();
        });
    }
}

修复后代码:
use tokio::net::TcpListener;
use tokio::io::{AsyncReadExt, AsyncWriteExt};

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind("127.0.0.1:8080").await?;
    loop {
        let (mut socket, addr) = listener.accept().await?;
        tokio::spawn(async move {
            let mut buf = vec![0u8; 1024];
            let n = socket.read(&mut buf).await.unwrap();
            socket.write_all(b"Hello").await.unwrap();
        });
    }
}`

func (a *AutonomousFixerAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
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

		// 构建工具定义
		toolDefs := a.buildToolDefs()

		// 初始消息
		messages := []*model.Message{
			model.NewSystemMessage(autonomousFixerPrompt),
			model.NewUserMessage(fmt.Sprintf(`项目目录: %s
主文件路径: %s

编译错误:
%s

请按照流程修复错误。注意：使用完整的文件路径（项目目录 + 相对路径）。`, projectDir, projectDir+"/src/main.rs", compileError)),
		}

		// 跟踪最后一次 cargo_check 结果
		var lastCargoCheckResult string
		var lastCargoCheckSuccess bool

		maxIterations := 15
		for i := 0; i < maxIterations; i++ {
			event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, i+1, maxIterations,
				fmt.Sprintf("自主修复中 (迭代 %d/%d)", i+1, maxIterations)))

			// 调用 LLM
			resp, err := a.model.Generate(ctx, messages, model.WithTools(toolDefs...))
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
				return
			}

			// 调试：打印 LLM 响应
			event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
				fmt.Sprintf("🤖 LLM响应: content=%d字符, tool_calls=%d个", len(resp.Content), len(resp.ToolCalls))))

			// 检查是否完成
			if strings.Contains(resp.Content, "FIXED") {
				// 严格验证：必须有 cargo_check 且成功
				if lastCargoCheckSuccess && !strings.Contains(lastCargoCheckResult, "error[") {
					event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "✅ 修复完成！cargo_check 验证通过"))
					event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{"success": true}))
					return
				} else {
					// LLM 错误地说 FIXED
					event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
						"⚠️ 检测到 LLM 错误声称 FIXED，但 cargo_check 未通过，继续修复..."))
					messages = append(messages, model.NewUserMessage(
						"错误！cargo_check 仍有 error，不能说 FIXED。请继续修复，直到 cargo_check 没有 error。"))
					continue
				}
			}

			// 如果有响应内容，显示
			if resp.Content != "" {
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, resp.Content))
			}

			// 处理工具调用
			if len(resp.ToolCalls) == 0 {
				// 没有工具调用
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "⚠️ LLM 没有调用任何工具"))
				messages = append(messages, model.NewAssistantMessage(resp.Content))
				continue
			}

			// 执行工具调用
			for _, tc := range resp.ToolCalls {
				// 解析参数用于显示
				var argsMap map[string]any
				json.Unmarshal([]byte(tc.Arguments), &argsMap)

				event.EmitEvent(ctx, eventChan, &event.Event{
					Type:      event.TypeToolCall,
					AgentName: a.name,
					ToolCall:  &event.ToolCall{Name: tc.Name, Arguments: argsMap},
				})

				t, ok := a.tools.Get(tc.Name)
				if !ok {
					errMsg := fmt.Sprintf("错误: 工具 %s 不存在", tc.Name)
					event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, errMsg))
					messages = append(messages, &model.Message{
						Role:    "tool",
						Content: errMsg,
					})
					continue
				}

				result, err := t.Run(ctx, tc.Arguments)
				if err != nil {
					errMsg := fmt.Sprintf("工具执行错误: %s", err.Error())
					event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, errMsg))
					messages = append(messages, &model.Message{
						Role:    "tool",
						Content: errMsg,
					})
					continue
				}

				// 显示工具结果
				resultPreview := result
				if len(resultPreview) > 500 {
					resultPreview = resultPreview[:500] + "...[truncated]"
				}
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
					fmt.Sprintf("📤 %s 结果:\n%s", tc.Name, resultPreview)))

				messages = append(messages, &model.Message{
					Role:    "tool",
					Content: result,
				})

				// 跟踪 cargo_check 结果
				if tc.Name == "cargo_check" {
					lastCargoCheckResult = result
					lastCargoCheckSuccess = strings.Contains(result, `"success":true`)

					// 如果 cargo_check 成功且没有 error，提示可以说 FIXED
					if lastCargoCheckSuccess && !strings.Contains(result, "error[") {
						messages = append(messages, model.NewUserMessage(
							"cargo_check 成功，没有编译错误。现在可以输出 FIXED 表示修复完成。"))
					} else if strings.Contains(result, "error[") {
						// 还有错误，提示继续修复
						messages = append(messages, model.NewUserMessage(
							"cargo_check 仍有错误，请分析错误并继续修复。"))
					}
				}

				// 如果刚执行了 file_write，强制要求调用 cargo_check
				if tc.Name == "file_write" {
					messages = append(messages, model.NewUserMessage(
						"文件已写入。现在必须调用 cargo_check 验证编译结果。"))
				}
			}

			messages = append(messages, model.NewAssistantMessage(resp.Content))
		}

		event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "MAX_ITERATIONS", "达到最大迭代次数"))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{"success": false}))
	}()

	return eventChan, nil
}

func (a *AutonomousFixerAgent) buildToolDefs() []*model.ToolDef {
	var defs []*model.ToolDef

	// 只添加修复相关的工具（移除 file_editor，强制使用 file_write）
	toolNames := []string{
		"rust_error_analyzer",
		"file_read_lines",
		"file_read",
		"code_search",
		"rust_doc_lookup",
		"file_write",
		"cargo_check",
	}

	for _, name := range toolNames {
		if t, ok := a.tools.Get(name); ok {
			defs = append(defs, &model.ToolDef{
				Type: "function",
				Function: &model.Function{
					Name:        t.Name(),
					Description: t.Description(),
					Parameters:  t.InputSchema(),
				},
			})
		}
	}

	return defs
}

// AutonomousCoderAgent 自主编码 Agent - 拥有完整的工具链
type AutonomousCoderAgent struct {
	name      string
	model     model.Model
	tools     *tool.Registry
	workspace string
}

func NewAutonomousCoderAgent(m model.Model, tools *tool.Registry, workspace string) *AutonomousCoderAgent {
	return &AutonomousCoderAgent{
		name:      "autonomous-coder",
		model:     m,
		tools:     tools,
		workspace: workspace,
	}
}

func (a *AutonomousCoderAgent) Name() string { return a.name }

const autonomousCoderPrompt = `你是一个专业的 Rust 开发专家。你必须严格遵循以下规则，否则任务将被视为失败。

## 核心原则（违反任何一条 = 任务失败）
1. ❌ 禁止写 TODO、占位符、模拟实现
2. ❌ 禁止猜测 API - 必须先查看源码
3. ❌ 禁止在 cargo_check 失败时说 DONE
4. ✅ 必须使用用户指定的库
5. ✅ 必须编译通过才能说 DONE

## 可用工具

### 本地 Rust 工具（最重要！）
- cargo_tree: 查看依赖树 {"project_dir": "目录"}
- crate_source: 读取本地 crate 源码 {"crate_name": "名称", "file": "lib.rs"}
  - 这是你了解 API 的主要方式！
  - 先 cargo_build 下载依赖，再用 crate_source 读取源码
  - 常用文件: lib.rs, macros.rs, examples/*.rs

### 网络搜索（辅助）
- crates_search: 搜索 crates.io {"query": "关键词"}
- crates_info: 获取 crate 详情 {"crate_name": "名称"}
- github_readme: 获取 README {"repo": "owner/repo"}

### 项目管理
- cargo_init: 创建项目 {"work_dir": "目录", "project_name": "名称"}
- cargo_build: 构建并下载依赖 {"project_dir": "目录"}
- cargo_check: 检查编译 {"project_dir": "目录"}

### 文件操作
- file_write: 写入完整文件 {"path": "路径", "content": "完整内容"}
- file_read: 读取文件 {"path": "路径"}

## 强制工作流程（必须按顺序执行）

### 阶段1: 准备
1. cargo_init 创建项目
2. crates_search 确认库名和版本
3. file_write 写入 Cargo.toml（包含依赖）
4. cargo_build 下载依赖

### 阶段2: 学习 API（最关键！）
5. crate_source 读取库的 lib.rs 了解导出
6. crate_source 读取库的 macros.rs（如果有宏）
7. crate_source 读取 examples/ 目录下的示例
8. 根据源码理解正确的 API 用法

### 阶段3: 实现
9. file_write 写入 main.rs（基于源码学到的 API）
10. cargo_check 验证编译

### 阶段4: 修复循环
11. 如果 cargo_check 有错误:
    - 分析错误信息
    - 用 crate_source 查看相关源码
    - file_write 修复代码
    - cargo_check 再次验证
    - 重复直到编译通过

### 阶段5: 完成
12. 只有当 cargo_check 返回 success:true 且没有 error 时
13. 输出 "DONE"

## 重要提示

- file_write 写入完整文件内容，不是增量编辑
- cargo_build 后才能用 crate_source 读取依赖源码
- crate 名称：Cargo.toml 用连字符（如 rig-core），代码中用下划线（如 rig）
- 不要猜测 API！看不懂就多读几个源文件

## 常见错误和解决方案

### 错误: "cannot find function/method"
→ 用 crate_source 读取源码，找到正确的函数名和模块路径

### 错误: "trait bound not satisfied"
→ 用 crate_source 读取 trait 定义，确认需要导入哪些 trait

### 错误: "no method named X"
→ 可能需要导入 trait，用 crate_source 查看 prelude 或 trait 定义

## 输出

只有在 cargo_check 成功（success:true 且无 error）后才能输出 "DONE"
如果无法完成，输出 "FAILED: <原因>"`

func (a *AutonomousCoderAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		// 构建工具定义
		toolDefs := a.buildAllToolDefs()

		messages := []*model.Message{
			model.NewSystemMessage(autonomousCoderPrompt),
			model.NewUserMessage(fmt.Sprintf(`工作目录: %s

任务: %s

请严格按照工作流程完成任务。记住：
1. 必须使用 crate_source 查看源码了解 API
2. 必须 cargo_check 成功才能说 DONE
3. 禁止写 TODO 或占位符`, a.workspace, input)),
		}

		// 跟踪状态
		var lastCargoCheckSuccess bool
		var lastCargoCheckResult string
		var projectDir string
		hasCalledCargoCheck := false

		maxIterations := 50
		for i := 0; i < maxIterations; i++ {
			event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, i+1, maxIterations,
				fmt.Sprintf("自主编码中 (步骤 %d)", i+1)))

			resp, err := a.model.Generate(ctx, messages, model.WithTools(toolDefs...))
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
				return
			}

			// 检查是否说 DONE
			if strings.Contains(resp.Content, "DONE") {
				// 严格验证
				if !hasCalledCargoCheck {
					event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
						"⚠️ 错误：还没有调用 cargo_check 就说 DONE！必须先验证编译。"))
					messages = append(messages, model.NewUserMessage(
						"错误！你还没有调用 cargo_check 验证编译。请先调用 cargo_check，确认编译成功后再说 DONE。"))
					continue
				}

				if !lastCargoCheckSuccess || strings.Contains(lastCargoCheckResult, "error[") {
					event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
						"⚠️ 错误：cargo_check 未通过就说 DONE！请继续修复。"))
					messages = append(messages, model.NewUserMessage(
						"错误！cargo_check 仍有错误，不能说 DONE。请分析错误并修复，直到 cargo_check 成功。"))
					continue
				}

				// 验证通过
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "✅ 任务完成！编译验证通过。"))
				event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{
					"success":     true,
					"project_dir": projectDir,
				}))
				return
			}

			// 检查是否说 FAILED
			if strings.Contains(resp.Content, "FAILED:") {
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, resp.Content))
				event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{
					"success":     false,
					"project_dir": projectDir,
				}))
				return
			}

			if resp.Content != "" {
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, resp.Content))
			}

			if len(resp.ToolCalls) == 0 {
				messages = append(messages, model.NewAssistantMessage(resp.Content))
				continue
			}

			for _, tc := range resp.ToolCalls {
				event.EmitEvent(ctx, eventChan, event.NewToolCallEvent(a.name, tc.Name, nil))

				t, ok := a.tools.Get(tc.Name)
				if !ok {
					messages = append(messages, &model.Message{Role: "tool", Content: "工具不存在: " + tc.Name})
					continue
				}

				result, err := t.Run(ctx, tc.Arguments)
				resultMsg := result
				if err != nil {
					resultMsg = "错误: " + err.Error()
				}

				// 跟踪 cargo_check 结果
				if tc.Name == "cargo_check" {
					hasCalledCargoCheck = true
					lastCargoCheckResult = result
					lastCargoCheckSuccess = strings.Contains(result, `"success":true`)

					// 提取项目目录
					var args struct {
						ProjectDir string `json:"project_dir"`
					}
					json.Unmarshal([]byte(tc.Arguments), &args)
					if args.ProjectDir != "" {
						projectDir = args.ProjectDir
					}

					// 根据结果给出指导
					if lastCargoCheckSuccess && !strings.Contains(result, "error[") {
						messages = append(messages, &model.Message{Role: "tool", Content: resultMsg})
						messages = append(messages, model.NewUserMessage(
							"✅ cargo_check 成功！编译通过，没有错误。现在可以输出 DONE 表示任务完成。"))
						continue
					} else {
						messages = append(messages, &model.Message{Role: "tool", Content: resultMsg})
						messages = append(messages, model.NewUserMessage(
							"❌ cargo_check 有错误。请分析错误信息，使用 crate_source 查看相关源码，然后修复代码。"))
						continue
					}
				}

				// 跟踪 cargo_init 结果
				if tc.Name == "cargo_init" {
					var args struct {
						WorkDir     string `json:"work_dir"`
						ProjectName string `json:"project_name"`
					}
					json.Unmarshal([]byte(tc.Arguments), &args)
					if args.WorkDir != "" && args.ProjectName != "" {
						projectDir = args.WorkDir + "/" + args.ProjectName
					}
				}

				// 简化显示
				var resultPreview string
				if len(resultMsg) > 500 {
					resultPreview = resultMsg[:500] + "..."
				} else {
					resultPreview = resultMsg
				}
				event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name,
					fmt.Sprintf("🔧 %s:\n%s", tc.Name, resultPreview)))

				messages = append(messages, &model.Message{Role: "tool", Content: resultMsg})
			}

			messages = append(messages, model.NewAssistantMessage(resp.Content))
		}

		event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "MAX_ITERATIONS", "达到最大迭代次数"))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{
			"success":     false,
			"project_dir": projectDir,
		}))
	}()

	return eventChan, nil
}

func (a *AutonomousCoderAgent) buildAllToolDefs() []*model.ToolDef {
	var defs []*model.ToolDef

	// 只添加必要的工具，移除容易出问题的 file_editor
	essentialTools := []string{
		// 项目管理
		"cargo_init", "cargo_check", "cargo_build",
		// 文件操作（只用 file_write，不用 file_editor）
		"file_write", "file_read", "file_list",
		// 本地 Rust 工具
		"cargo_tree", "crate_source",
		// 网络搜索
		"crates_search", "crates_info", "github_readme",
		// 错误分析
		"rust_error_analyzer",
	}

	for _, name := range essentialTools {
		if t, ok := a.tools.Get(name); ok {
			defs = append(defs, &model.ToolDef{
				Type: "function",
				Function: &model.Function{
					Name:        t.Name(),
					Description: t.Description(),
					Parameters:  t.InputSchema(),
				},
			})
		}
	}

	return defs
}
