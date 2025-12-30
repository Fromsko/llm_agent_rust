# 🦀 LLM Agent Rust

[English](./README.md) | [中文](./README_zh.md)

基于 LLM 的智能 Rust 代码开发助手，支持多 Agent 协作。

## 项目简介

本项目提供 AI 驱动的 Rust 开发工具，包括代码生成、自动错误修复、代码审查和经验学习。使用 Go 构建，由 LLM API 驱动。

## 📁 项目结构

```
llm_agent_rust/
├── rust_agent/        # v1 - 基础 Rust 代码 Agent
└── rust_agent_v2/     # v2 - 多 Agent 协作系统
```

## 🛠️ 版本说明

### rust_agent (v1)

基础的 LLM 驱动 Rust 代码生成和修复工具。

- 根据自然语言需求生成代码
- 自动修复编译错误（最多 5 次迭代）
- 代码审查（正确性、安全性、性能）
- 代码解释和测试生成

### rust_agent_v2 (v2)

高级多 Agent 协作系统，支持经验学习。

- 监督者模式：协调多个专业 Agent 完成复杂任务
- 经验学习：记录成功/失败模式供复用
- 自主编码：从需求到可运行代码全自动化
- MCP 集成：fetch、filesystem 等服务
- 丰富工具：完整 cargo 命令、文件操作、Web 搜索

## 🚀 快速开始

### v1 - 基础 Agent

```bash
cd rust_agent
go build -o rust-agent.exe ./cmd
rust-agent.exe -i
```

命令：

- `gen <需求>` - 生成代码
- `fix` - 修复上次生成的代码
- `review` - 审查代码
- `explain` - 解释代码
- `test` - 生成测试

### v2 - 多 Agent 系统

```bash
cd rust_agent_v2
go build -o rust-agent.exe ./cmd
rust-agent.exe -i
```

命令：

- `/create <描述>` - 监督者模式创建项目
- `/direct <描述>` - 直接编码模式
- `/fix <项目名>` - 自主修复项目
- `/search <关键词>` - 搜索 crates.io
- `/exp` - 查看经验库

## 🧠 经验学习 (v2)

Agent 会自动记录每次任务的经验：

- **成功经验**：正确的 import、API 用法、代码模式
- **失败经验**：遇到的错误、解决方案、教训

下次遇到类似任务时，会自动检索相关经验辅助编码。

## 🔧 依赖

- Go 1.22+
- Rust 工具链 (cargo, rustc)
- 智谱 API Key

## ⚙️ 配置

### v1

编辑 `config/config.go`：

- `ZhipuAPIKey`: API Key
- `Model`: 模型名称
- `WorkspaceDir`: 工作目录

### v2

创建 `config.json`：

```json
{
  "api": {
    "zhipu_api_key": "your-api-key",
    "model": "glm-4-flash"
  },
  "workspace": {
    "root_dir": "./rust_workspace"
  }
}
```

## 📄 License

[MIT](./LICENSE)
