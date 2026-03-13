package config

import (
	"encoding/json"
	"os"
)

// Config 配置
type Config struct {
	API       APIConfig       `json:"api"`
	Agent     AgentConfig     `json:"agent"`
	MCP       MCPConfig       `json:"mcp"`
	Workspace WorkspaceConfig `json:"workspace"`
}

type APIConfig struct {
	ZhipuAPIKey  string `json:"zhipu_api_key"`
	ZhipuBaseURL string `json:"zhipu_base_url"`
	Model        string `json:"model"`
	Concurrency  int    `json:"concurrency"`
}

type AgentConfig struct {
	MaxIterations int  `json:"max_iterations"`
	AutoFix       bool `json:"auto_fix"`
	AutoTest      bool `json:"auto_test"`
}

type MCPConfig struct {
	FetchEnabled bool   `json:"fetch_enabled"`
	FetchCommand string `json:"fetch_command"`
}

type WorkspaceConfig struct {
	RootDir    string `json:"root_dir"`
	MemoryDir  string `json:"memory_dir"`
	ProjectDir string `json:"project_dir"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		API: APIConfig{
			ZhipuAPIKey:  "",
			ZhipuBaseURL: "https://open.bigmodel.cn/api/paas/v4",
			Model:        "glm-4-flash",
			Concurrency:  5,
		},
		Agent: AgentConfig{
			MaxIterations: 10,
			AutoFix:       true,
			AutoTest:      false,
		},
		MCP: MCPConfig{
			FetchEnabled: true,
			FetchCommand: "uvx mcp-server-fetch",
		},
		Workspace: WorkspaceConfig{
			RootDir:    "./rust_workspace",
			MemoryDir:  "./.memory",
			ProjectDir: "./rust_workspace/projects",
		},
	}
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
