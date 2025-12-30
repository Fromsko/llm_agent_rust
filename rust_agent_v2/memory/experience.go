package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Experience 经验记录
type Experience struct {
	ID        string            `json:"id"`
	Task      string            `json:"task"`       // 任务描述
	CrateName string            `json:"crate_name"` // 使用的 crate
	Success   bool              `json:"success"`    // 是否成功
	Code      string            `json:"code"`       // 最终代码
	Imports   []string          `json:"imports"`    // 正确的导入
	APIUsage  []string          `json:"api_usage"`  // API 用法示例
	Errors    []string          `json:"errors"`     // 遇到的错误
	Solutions []string          `json:"solutions"`  // 解决方案
	Lessons   []string          `json:"lessons"`    // 学到的教训
	CreatedAt time.Time         `json:"created_at"`
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
}

// ExperienceStore 经验存储
type ExperienceStore struct {
	dir         string
	experiences map[string]*Experience
	mu          sync.RWMutex
}

func NewExperienceStore(dir string) (*ExperienceStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	store := &ExperienceStore{
		dir:         dir,
		experiences: make(map[string]*Experience),
	}

	// 加载已有经验
	store.loadAll()

	return store, nil
}

func (s *ExperienceStore) loadAll() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}

		var exp Experience
		if err := json.Unmarshal(data, &exp); err != nil {
			continue
		}

		s.experiences[exp.ID] = &exp
	}
}

// Save 保存经验
func (s *ExperienceStore) Save(ctx context.Context, exp *Experience) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if exp.ID == "" {
		exp.ID = fmt.Sprintf("%s_%d", exp.CrateName, time.Now().UnixNano())
	}
	exp.CreatedAt = time.Now()

	s.experiences[exp.ID] = exp

	data, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.dir, exp.ID+".json"), data, 0644)
}

// FindByCrate 根据 crate 名称查找经验
func (s *ExperienceStore) FindByCrate(crateName string) []*Experience {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Experience
	for _, exp := range s.experiences {
		// 如果 crateName 为空，返回所有经验
		if crateName == "" {
			results = append(results, exp)
			continue
		}
		if exp.CrateName == crateName || strings.Contains(exp.CrateName, crateName) {
			results = append(results, exp)
		}
	}
	return results
}

// FindSuccessful 查找成功的经验
func (s *ExperienceStore) FindSuccessful(crateName string) []*Experience {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Experience
	for _, exp := range s.experiences {
		if exp.Success && (crateName == "" || exp.CrateName == crateName) {
			results = append(results, exp)
		}
	}
	return results
}

// FindByTag 根据标签查找
func (s *ExperienceStore) FindByTag(tag string) []*Experience {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Experience
	for _, exp := range s.experiences {
		for _, t := range exp.Tags {
			if t == tag {
				results = append(results, exp)
				break
			}
		}
	}
	return results
}

// GetLesson 获取关于某个 crate 的教训
func (s *ExperienceStore) GetLessons(crateName string) []string {
	experiences := s.FindByCrate(crateName)
	var lessons []string
	seen := make(map[string]bool)

	for _, exp := range experiences {
		for _, lesson := range exp.Lessons {
			if !seen[lesson] {
				lessons = append(lessons, lesson)
				seen[lesson] = true
			}
		}
	}
	return lessons
}

// GetCorrectImports 获取正确的导入语句
func (s *ExperienceStore) GetCorrectImports(crateName string) []string {
	experiences := s.FindSuccessful(crateName)
	var imports []string
	seen := make(map[string]bool)

	for _, exp := range experiences {
		for _, imp := range exp.Imports {
			if !seen[imp] {
				imports = append(imports, imp)
				seen[imp] = true
			}
		}
	}
	return imports
}

// GetAPIUsage 获取 API 用法示例
func (s *ExperienceStore) GetAPIUsage(crateName string) []string {
	experiences := s.FindSuccessful(crateName)
	var usage []string

	for _, exp := range experiences {
		usage = append(usage, exp.APIUsage...)
	}
	return usage
}

// FormatForPrompt 格式化为 prompt 可用的文本
func (s *ExperienceStore) FormatForPrompt(crateName string) string {
	experiences := s.FindByCrate(crateName)
	if len(experiences) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 关于 %s 的历史经验\n\n", crateName))

	// 成功经验
	successful := s.FindSuccessful(crateName)
	if len(successful) > 0 {
		sb.WriteString("### 成功案例\n")
		for _, exp := range successful {
			if len(exp.Imports) > 0 {
				sb.WriteString("正确的导入:\n```rust\n")
				for _, imp := range exp.Imports {
					sb.WriteString(imp + "\n")
				}
				sb.WriteString("```\n")
			}
			if len(exp.APIUsage) > 0 {
				sb.WriteString("API 用法:\n```rust\n")
				for _, usage := range exp.APIUsage[:min(3, len(exp.APIUsage))] {
					sb.WriteString(usage + "\n")
				}
				sb.WriteString("```\n")
			}
		}
	}

	// 教训
	lessons := s.GetLessons(crateName)
	if len(lessons) > 0 {
		sb.WriteString("\n### 重要教训\n")
		for _, lesson := range lessons {
			sb.WriteString("- " + lesson + "\n")
		}
	}

	return sb.String()
}

// CrateKnowledge crate 知识库
type CrateKnowledge struct {
	Name           string   `json:"name"`
	CargoName      string   `json:"cargo_name"`      // Cargo.toml 中的名称
	CodeName       string   `json:"code_name"`       // 代码中的名称
	RequiredTraits []string `json:"required_traits"` // 需要导入的 trait
	CommonImports  []string `json:"common_imports"`  // 常用导入
	ExampleCode    string   `json:"example_code"`    // 示例代码
	Gotchas        []string `json:"gotchas"`         // 容易出错的地方
}

// KnowledgeBase 知识库
type KnowledgeBase struct {
	crates map[string]*CrateKnowledge
	mu     sync.RWMutex
}

func NewKnowledgeBase() *KnowledgeBase {
	kb := &KnowledgeBase{
		crates: make(map[string]*CrateKnowledge),
	}

	// 预置一些常用 crate 的知识
	kb.AddKnowledge(&CrateKnowledge{
		Name:      "rig-core",
		CargoName: "rig-core",
		CodeName:  "rig", // extern crate self as rig
		RequiredTraits: []string{
			"rig::client::ProviderClient",
			"rig::client::CompletionClient",
			"rig::completion::Prompt",
		},
		CommonImports: []string{
			"use rig::{client::{CompletionClient, ProviderClient}, completion::Prompt, providers::openai};",
		},
		ExampleCode: `let client = openai::Client::from_env();
let agent = client.agent("gpt-4").build();
let response = agent.prompt("Hello").await?;`,
		Gotchas: []string{
			"crate 名称是 rig 不是 rig_core",
			"需要导入 ProviderClient trait 才能使用 from_env()",
			"需要导入 CompletionClient trait 才能使用 agent()",
		},
	})

	kb.AddKnowledge(&CrateKnowledge{
		Name:      "rmcp",
		CargoName: "rmcp",
		CodeName:  "rmcp",
		RequiredTraits: []string{
			"rmcp::handler::server::ServerHandler",
		},
		CommonImports: []string{
			"use rmcp::{ServerHandler, ServiceExt, tool, transport::stdio};",
			"use rmcp::handler::server::tool::ToolCallContext;",
			"use rmcp::model::{CallToolResult, Content, TextContent};",
		},
		ExampleCode: `// 定义服务结构
#[derive(Debug, Clone, Default)]
struct MathService;

// 实现工具方法
#[tool(description = "Add two numbers")]
fn add(&self, #[tool(param)] a: f64, #[tool(param)] b: f64) -> f64 {
    a + b
}

// 实现 ServerHandler trait
impl ServerHandler for MathService {
    fn get_info(&self) -> ServerInfo {
        ServerInfo {
            name: "math-server".into(),
            version: "1.0.0".into(),
            ..Default::default()
        }
    }
}

// 主函数
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let service = MathService::default().serve(stdio()).await?;
    service.waiting().await?;
    Ok(())
}`,
		Gotchas: []string{
			"使用 #[tool] 宏定义工具方法",
			"需要 features = [\"server\", \"macros\"] 来启用服务器和宏功能",
			"工具参数需要用 #[tool(param)] 标注",
			"必须实现 ServerHandler trait",
			"使用 .serve(stdio()) 启动服务",
			"rmcp 来自 https://github.com/modelcontextprotocol/rust-sdk",
		},
	})

	kb.AddKnowledge(&CrateKnowledge{
		Name:      "tokio",
		CargoName: "tokio",
		CodeName:  "tokio",
		RequiredTraits: []string{
			"tokio::io::AsyncReadExt",
			"tokio::io::AsyncWriteExt",
		},
		CommonImports: []string{
			"use tokio::io::{AsyncReadExt, AsyncWriteExt};",
			"use tokio::net::{TcpListener, TcpStream};",
		},
		ExampleCode: `#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind("127.0.0.1:8080").await?;
    loop {
        let (mut socket, _) = listener.accept().await?;
        tokio::spawn(async move {
            let mut buf = vec![0u8; 1024];
            let n = socket.read(&mut buf).await.unwrap();
            socket.write_all(&buf[..n]).await.unwrap();
        });
    }
}`,
		Gotchas: []string{
			"tokio::io 没有 read/write 函数，要用 AsyncReadExt/AsyncWriteExt trait",
			"socket.read() 需要 socket 是 mut",
			"需要 features = [\"full\"] 或具体的 feature",
		},
	})

	return kb
}

func (kb *KnowledgeBase) AddKnowledge(k *CrateKnowledge) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.crates[k.Name] = k
	kb.crates[k.CargoName] = k
	kb.crates[k.CodeName] = k
}

func (kb *KnowledgeBase) Get(name string) *CrateKnowledge {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	return kb.crates[name]
}

func (kb *KnowledgeBase) FormatForPrompt(name string) string {
	k := kb.Get(name)
	if k == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s 使用指南\n\n", k.Name))
	sb.WriteString(fmt.Sprintf("- Cargo.toml: `%s = \"...\"`\n", k.CargoName))
	sb.WriteString(fmt.Sprintf("- 代码中使用: `use %s::...`\n\n", k.CodeName))

	if len(k.CommonImports) > 0 {
		sb.WriteString("### 常用导入\n```rust\n")
		for _, imp := range k.CommonImports {
			sb.WriteString(imp + "\n")
		}
		sb.WriteString("```\n\n")
	}

	if k.ExampleCode != "" {
		sb.WriteString("### 示例代码\n```rust\n")
		sb.WriteString(k.ExampleCode)
		sb.WriteString("\n```\n\n")
	}

	if len(k.Gotchas) > 0 {
		sb.WriteString("### 注意事项\n")
		for _, g := range k.Gotchas {
			sb.WriteString("⚠️ " + g + "\n")
		}
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
