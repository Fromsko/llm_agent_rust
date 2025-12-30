# 🦀 LLM Agent Rust

[English](./README.md) | [中文](./README_zh.md)

LLM-powered intelligent Rust code development assistant with multi-agent collaboration.

## Introduction

This project provides AI-powered tools for Rust development, including code generation, automatic error fixing, code review, and experience learning. Built with Go and powered by LLM APIs.

## 📁 Project Structure

```
llm_agent_rust/
├── rust_agent/        # v1 - Basic Rust code agent
└── rust_agent_v2/     # v2 - Multi-agent collaboration system
```

## 🛠️ Versions

### rust_agent (v1)

Basic LLM-powered Rust code generation and fixing tool.

- Code generation from natural language
- Auto error fixing (up to 5 iterations)
- Code review (correctness, security, performance)
- Code explanation and test generation

### rust_agent_v2 (v2)

Advanced multi-agent collaboration system with experience learning.

- Supervisor mode: coordinates multiple specialized agents
- Experience learning: records success/failure patterns for reuse
- Autonomous coding: from requirements to working code
- MCP integration: fetch, filesystem services
- Rich toolset: full cargo commands, file ops, web search

## 🚀 Quick Start

### v1 - Basic Agent

```bash
cd rust_agent
go build -o rust-agent.exe ./cmd
rust-agent.exe -i
```

Commands:

- `gen <requirement>` - Generate code
- `fix` - Fix last generated code
- `review` - Review code
- `explain` - Explain code
- `test` - Generate tests

### v2 - Multi-Agent System

```bash
cd rust_agent_v2
go build -o rust-agent.exe ./cmd
rust-agent.exe -i
```

Commands:

- `/create <desc>` - Create project with supervisor mode
- `/direct <desc>` - Direct coding mode
- `/fix <project>` - Auto-fix project
- `/search <keyword>` - Search crates.io
- `/exp` - View experience store

## 🧠 Experience Learning (v2)

The agent automatically records experiences from each task:

- **Success**: correct imports, API usage, code patterns
- **Failure**: errors encountered, solutions, lessons learned

These experiences are retrieved for similar future tasks.

## 🔧 Requirements

- Go 1.22+
- Rust toolchain (cargo, rustc)
- Zhipu API Key

## ⚙️ Configuration

### v1

Edit `config/config.go`:

- `ZhipuAPIKey`: API Key
- `Model`: Model name
- `WorkspaceDir`: Workspace directory

### v2

Create `config.json`:

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
