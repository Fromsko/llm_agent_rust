package config

import "time"

// Config 配置
type Config struct {
	Model  ModelConfig
	Rust   RustConfig
	Output OutputConfig
}

// ModelConfig 模型配置
type ModelConfig struct {
	APIKey      string
	BaseURL     string
	ModelName   string
	Concurrency int
	Timeout     time.Duration
}

// RustConfig Rust 工具链配置
type RustConfig struct {
	CargoPath   string // cargo 路径
	RustcPath   string // rustc 路径
	ClippyArgs  []string
	FmtArgs     []string
	TestTimeout time.Duration
}

// OutputConfig 输出配置
type OutputConfig struct {
	WorkDir     string
	ProjectName string
}

// DefaultConfig 默认配置
var DefaultConfig = &Config{
	Model: ModelConfig{
		APIKey:      "",
		BaseURL:     "",
		ModelName:   "",
		Concurrency: 50,
		Timeout:     120 * time.Second,
	},
	Rust: RustConfig{
		CargoPath:   "cargo",
		RustcPath:   "rustc",
		ClippyArgs:  []string{"-W", "clippy::all"},
		FmtArgs:     []string{},
		TestTimeout: 60 * time.Second,
	},
	Output: OutputConfig{
		WorkDir:     "./rust_workspace",
		ProjectName: "generated_project",
	},
}
