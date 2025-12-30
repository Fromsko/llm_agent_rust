package specialized

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
)

// CratesIOAgent crates.io 搜索 Agent
type CratesIOAgent struct {
	name       string
	model      model.Model
	httpClient *http.Client
}

func NewCratesIOAgent(m model.Model) *CratesIOAgent {
	return &CratesIOAgent{
		name:       "cratesio-agent",
		model:      m,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *CratesIOAgent) Name() string { return a.name }

const cratesIOPrompt = `你是一个 Rust 依赖管理专家。

你的职责：
1. 根据用户需求推荐合适的 crate
2. 分析 crate 的质量（下载量、更新频率、文档质量）
3. 提供 Cargo.toml 依赖配置
4. 说明 crate 的基本用法

输出格式：
## 推荐 Crate

### [crate名称]
- 版本: x.x.x
- 下载量: xxx
- 描述: xxx
- Cargo.toml: ` + "`[dependencies]\ncrate = \"x.x.x\"`" + `
- 基本用法: ...`

func (a *CratesIOAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		// 搜索 crates.io
		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 1, 3, "搜索 crates.io"))

		crates, err := a.searchCrates(ctx, input)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "SEARCH_ERROR", err.Error()))
			return
		}

		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 2, 3, "分析搜索结果"))

		// 让 LLM 分析并推荐
		cratesJSON, _ := json.MarshalIndent(crates, "", "  ")
		messages := []*model.Message{
			model.NewSystemMessage(cratesIOPrompt),
			model.NewUserMessage(fmt.Sprintf("用户需求：%s\n\n搜索结果：\n%s", input, string(cratesJSON))),
		}

		resp, err := a.model.Generate(ctx, messages)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(a.name, "LLM_ERROR", err.Error()))
			return
		}

		event.EmitEvent(ctx, eventChan, event.NewProgressEvent(a.name, 3, 3, "生成推荐"))
		event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, resp.Content))
		event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, map[string]any{"crates": crates}))
	}()

	return eventChan, nil
}

type CrateInfo struct {
	Name        string `json:"name"`
	Version     string `json:"max_version"`
	Description string `json:"description"`
	Downloads   int    `json:"downloads"`
	Repository  string `json:"repository"`
}

func (a *CratesIOAgent) searchCrates(ctx context.Context, query string) ([]CrateInfo, error) {
	url := fmt.Sprintf("https://crates.io/api/v1/crates?q=%s&per_page=10", query)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "rust-agent/1.0")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Crates []CrateInfo `json:"crates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Crates, nil
}
