package specialized

import (
	"context"
	"encoding/json"
	"fmt"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
)

// PlannerAgent 规划 Agent - 分析需求并生成执行计划
type PlannerAgent struct {
	name  string
	model model.Model
}

func NewPlannerAgent(m model.Model) *PlannerAgent {
	return &PlannerAgent{name: "planner-agent", model: m}
}

func (a *PlannerAgent) Name() string { return a.name }

const plannerPrompt = `你是一个 Rust 项目规划专家。

你的任务是分析用户需求，生成详细的执行计划。

重要提示 - 常用 crate 正确名称和版本：
- rig-core = "0.6" (不是 rig)
- ollama-rs = "0.2" (不是 ollama)
- tokio = { version = "1", features = ["full"] }
- reqwest = { version = "0.12", features = ["json"] }
- serde = { version = "1", features = ["derive"] }
- serde_json = "1"
- anyhow = "1"
- thiserror = "2"

输出必须是严格的 JSON 格式：
{
  "project_name": "项目名称（小写下划线）",
  "description": "项目描述",
  "dependencies": [
    {"name": "crate名称", "version": "版本号", "features": ["可选features"]}
  ],
  "files": [
    {
      "path": "src/main.rs 或 src/lib.rs 等",
      "purpose": "文件用途",
      "key_components": ["主要组件/函数/结构体"]
    }
  ],
  "build_steps": ["构建步骤"],
  "test_plan": ["测试计划"]
}

规则：
1. 项目名称必须是有效的 Rust 包名（小写、下划线）
2. 依赖版本要使用上面提供的正确版本
3. 文件结构要合理，遵循 Rust 项目惯例
4. 只输出 JSON，不要其他内容`

// Plan 执行计划
type Plan struct {
	ProjectName  string       `json:"project_name"`
	Description  string       `json:"description"`
	Dependencies []Dependency `json:"dependencies"`
	Files        []FileSpec   `json:"files"`
	BuildSteps   []string     `json:"build_steps"`
	TestPlan     []string     `json:"test_plan"`
}

type Dependency struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Features []string `json:"features,omitempty"`
}

type FileSpec struct {
	Path          string   `json:"path"`
	Purpose       string   `json:"purpose"`
	KeyComponents []string `json:"key_components"`
}

func (a *PlannerAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 1, 2, "分析需求，生成执行计划"))

		messages := []*model.Message{
			model.NewSystemMessage(plannerPrompt),
			model.NewUserMessage(fmt.Sprintf("请为以下需求生成执行计划：\n%s", input)),
		}

		resp, err := a.model.Generate(ctx, messages)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
			return
		}

		// 解析计划
		var plan Plan
		if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &plan); err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "PARSE_ERROR", "无法解析执行计划: "+err.Error()))
			return
		}

		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 2, 2, "计划生成完成"))
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, fmt.Sprintf("📋 项目: %s\n📝 描述: %s\n📦 依赖: %d 个\n📄 文件: %d 个",
			plan.ProjectName, plan.Description, len(plan.Dependencies), len(plan.Files))))

		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, &plan))
	}()

	return eventChan, nil
}

// extractJSON 从文本中提取 JSON
func extractJSON(text string) string {
	start := -1
	end := -1
	depth := 0

	for i, c := range text {
		if c == '{' {
			if start == -1 {
				start = i
			}
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if start != -1 && end != -1 {
		return text[start:end]
	}
	return text
}
