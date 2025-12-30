package workflow

import (
	"context"
	"os"
	"path/filepath"

	"rust_agent_v2/agent"
	"rust_agent_v2/agent/specialized"
	"rust_agent_v2/event"
	"rust_agent_v2/graph"
	"rust_agent_v2/model"
	"rust_agent_v2/tool"
)

// RustWorkflow 完整的 Rust 代码生成工作流
type RustWorkflow struct {
	model     model.Model
	tools     *tool.Registry
	workspace string
}

func NewRustWorkflow(m model.Model, tools *tool.Registry, workspace string) *RustWorkflow {
	return &RustWorkflow{model: m, tools: tools, workspace: workspace}
}

// Run 运行完整工作流
func (w *RustWorkflow) Run(ctx context.Context, input string) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		// 确保工作目录存在
		os.MkdirAll(w.workspace, 0755)

		// Phase 1: 规划
		event.EmitEvent(ctx, eventChan, &event.Event{
			Type:      event.TypeProgress,
			AgentName: "workflow",
			Progress:  &event.Progress{Current: 1, Total: 4, Message: "🎯 Phase 1: 需求分析与规划"},
		})

		planner := specialized.NewPlannerAgent(w.model)
		planChan, err := planner.Run(ctx, input)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent("workflow", "PLANNER_ERROR", err.Error()))
			return
		}

		var plan *specialized.Plan
		for ev := range planChan {
			event.EmitEvent(ctx, eventChan, ev)
			if ev.Type == event.TypeCompletion && ev.Completion != nil {
				if p, ok := ev.Completion.Result.(*specialized.Plan); ok {
					plan = p
				}
			}
		}

		if plan == nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent("workflow", "NO_PLAN", "规划失败"))
			return
		}

		// Phase 2: 执行（创建项目和代码）
		event.EmitEvent(ctx, eventChan, &event.Event{
			Type:      event.TypeProgress,
			AgentName: "workflow",
			Progress:  &event.Progress{Current: 2, Total: 4, Message: "🔨 Phase 2: 创建项目与代码"},
		})

		executor := specialized.NewExecutorAgent(w.model, w.tools, w.workspace)
		execChan, err := executor.Run(ctx, input, agent.WithState(map[string]any{"plan": plan}))
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent("workflow", "EXECUTOR_ERROR", err.Error()))
			return
		}

		var projectDir string
		var compileSuccess bool
		var compileError string

		for ev := range execChan {
			event.EmitEvent(ctx, eventChan, ev)
			if ev.Type == event.TypeCompletion && ev.Completion != nil {
				if result, ok := ev.Completion.Result.(map[string]any); ok {
					projectDir, _ = result["project_dir"].(string)
					compileSuccess, _ = result["success"].(bool)
					compileError, _ = result["compile_error"].(string)
				}
			}
		}

		// Phase 3: 修复（如果有编译错误）
		if !compileSuccess && compileError != "" {
			event.EmitEvent(ctx, eventChan, &event.Event{
				Type:      event.TypeProgress,
				AgentName: "workflow",
				Progress:  &event.Progress{Current: 3, Total: 4, Message: "🔧 Phase 3: 自动修复编译错误"},
			})

			fixer := specialized.NewFixerAgent(w.model)
			fixChan, err := fixer.Run(ctx, "", agent.WithState(map[string]any{
				"project_dir":   projectDir,
				"compile_error": compileError,
			}))
			if err != nil {
				event.EmitEvent(ctx, eventChan, event.NewErrorEvent("workflow", "FIXER_ERROR", err.Error()))
				return
			}

			for ev := range fixChan {
				event.EmitEvent(ctx, eventChan, ev)
				if ev.Type == event.TypeCompletion && ev.Completion != nil {
					if result, ok := ev.Completion.Result.(map[string]any); ok {
						compileSuccess, _ = result["success"].(bool)
					}
				}
			}
		} else {
			event.EmitEvent(ctx, eventChan, &event.Event{
				Type:      event.TypeProgress,
				AgentName: "workflow",
				Progress:  &event.Progress{Current: 3, Total: 4, Message: "✅ Phase 3: 无需修复"},
			})
		}

		// Phase 4: 完成
		event.EmitEvent(ctx, eventChan, &event.Event{
			Type:      event.TypeProgress,
			AgentName: "workflow",
			Progress:  &event.Progress{Current: 4, Total: 4, Message: "📦 Phase 4: 完成"},
		})

		// 输出最终结果
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent("workflow", formatFinalResult(projectDir, compileSuccess)))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent("workflow", map[string]any{
			"project_dir": projectDir,
			"success":     compileSuccess,
		}))
	}()

	return eventChan, nil
}

func formatFinalResult(projectDir string, success bool) string {
	status := "✅ 成功"
	if !success {
		status = "⚠️ 有错误"
	}

	return `
============================================================
🦀 Rust 项目生成完成
============================================================
📁 项目目录: ` + projectDir + `
📊 状态: ` + status + `

运行方式:
  cd ` + projectDir + `
  cargo run

============================================================`
}

// BuildGraph 构建工作流图（用于更复杂的场景）
func BuildGraph(m model.Model, tools *tool.Registry, workspace string) *graph.Graph {
	planner := specialized.NewPlannerAgent(m)
	executor := specialized.NewExecutorAgent(m, tools, workspace)
	fixer := specialized.NewFixerAgent(m)
	reviewer := specialized.NewReviewAgent(m, tools)

	return graph.NewBuilder("rust-workflow").
		AddAgentNode("planner", planner).
		AddAgentNode("executor", executor).
		AddAgentNode("fixer", fixer).
		AddAgentNode("reviewer", reviewer).
		AddNode("check_compile", func(ctx context.Context, state graph.State) (graph.State, error) {
			success, _ := state["compile_success"].(bool)
			state[graph.StateKeySuccess] = success
			return state, nil
		}).
		AddNode("end", func(ctx context.Context, state graph.State) (graph.State, error) {
			return state, nil
		}).
		AddEdge("planner", "executor").
		AddEdge("executor", "check_compile").
		AddConditionalEdge("check_compile", "reviewer", func(ctx context.Context, state graph.State) bool {
			success, _ := state[graph.StateKeySuccess].(bool)
			return success
		}).
		AddConditionalEdge("check_compile", "fixer", func(ctx context.Context, state graph.State) bool {
			success, _ := state[graph.StateKeySuccess].(bool)
			return !success
		}).
		AddEdge("fixer", "check_compile").
		AddEdge("reviewer", "end").
		SetEntryPoint("planner").
		SetEndNode("end").
		Build()
}

// ListProjects 列出工作区中的项目
func ListProjects(workspace string) ([]string, error) {
	var projects []string

	entries, err := os.ReadDir(workspace)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			cargoPath := filepath.Join(workspace, entry.Name(), "Cargo.toml")
			if _, err := os.Stat(cargoPath); err == nil {
				projects = append(projects, entry.Name())
			}
		}
	}

	return projects, nil
}
