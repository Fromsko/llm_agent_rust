# 🦀 Rust Agent v2 - 多 Agent 协作系统

基于 trpc-agent-go 架构的 Rust 代码开发助手，支持多 Agent 协作、MCP 集成、经验学习。

## 核心特性

- **监督者模式**: SupervisorAgent 协调多个专业 Agent 完成复杂任务
- **经验学习**: 自动记录成功/失败经验，下次任务可复用
- **自主编码**: 从需求分析到代码生成、编译、修复全自动化
- **MCP 集成**: 支持 fetch、filesystem 等 MCP 服务
- **丰富工具**: cargo 全套命令、文件操作、Web 搜索

## 架构设计

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Runner Layer                                    │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     RustRunner                                       │   │
│  │  (Session管理, 事件处理, 状态追踪, 自动重试)                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Agent Layer                                     │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                  SupervisorAgent (监督者)                            │   │
│  │  分析任务 → 检索经验 → 执行编码 → 验证结果 → 反思学习 → 保存经验      │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ Autonomous  │  │ Autonomous  │  │ CratesIO    │  │ Planner     │        │
│  │ Coder       │  │ Fixer       │  │ Agent       │  │ Agent       │        │
│  │ 自主编码     │  │ 自主修复    │  │ 依赖搜索    │  │ 任务规划    │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                              Tool Layer                                      │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ cargo_check │  │ cargo_build │  │ cargo_run   │  │ cargo_test  │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ cargo_clippy│  │ cargo_fmt   │  │ file_read   │  │ file_write  │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                             Memory Layer                                     │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                         │
│  │ Experience  │  │ Project     │  │ Error       │                         │
│  │ Store       │  │ Context     │  │ History     │                         │
│  │ 经验库      │  │ 项目上下文   │  │ 错误历史    │                         │
│  └─────────────┘  └─────────────┘  └─────────────┘                         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 目录结构

```
rust_agent_v2/
├── cmd/
│   └── main.go              # 入口（CLI + 交互模式）
├── agent/
│   ├── agent.go             # Agent 接口
│   └── specialized/         # 专业 Agent
│       ├── supervisor.go    # 监督者 Agent
│       ├── coder.go         # 自主编码 Agent
│       ├── fixer.go         # 自主修复 Agent
│       └── cratesio.go      # Crates 搜索 Agent
├── config/
│   └── config.go            # 配置
├── event/
│   └── event.go             # 事件系统
├── graph/
│   └── graph.go             # 图编排引擎
├── mcp/
│   └── mcp.go               # MCP 客户端
├── memory/
│   ├── memory.go            # 记忆管理
│   └── experience.go        # 经验存储
├── model/
│   └── model.go             # LLM 模型接口
├── runner/
│   └── runner.go            # 运行时
├── tool/
│   ├── tool.go              # 工具接口
│   ├── cargo.go             # Cargo 工具
│   ├── file.go              # 文件工具
│   └── web_tools.go         # Web 工具
├── workflow/
│   └── workflow.go          # 工作流定义
├── rust_workspace/          # 生成的项目目录
│   └── .experience/         # 经验库存储
└── tests/                   # 测试用例
```

## 使用方法

### 编译

```bash
go build -o rust-agent.exe ./cmd
```

### 交互模式

```bash
rust-agent.exe -i
```

命令:

- `/create <描述>` - 监督者模式创建项目（带经验学习）
- `/direct <描述>` - 直接编码模式（不带监督）
- `/fix <项目名>` - 自主修复项目
- `/search <关键词>` - 搜索 crates.io
- `/list` - 列出已创建的项目
- `/run <项目名>` - 运行项目
- `/exp` - 查看经验库
- `/quit` - 退出

### 命令行模式

```bash
# 创建项目（监督者模式）
rust-agent.exe -create "实现一个简单的 HTTP 服务器"

# 搜索 crates
rust-agent.exe -search "async http"

# 指定工作目录
rust-agent.exe -workspace "./my_workspace" -create "..."
```

## 经验学习

Agent 会自动记录每次任务的经验:

- **成功经验**: 正确的 import、API 用法、代码模式
- **失败经验**: 遇到的错误、解决方案、教训

下次遇到类似任务时，会自动检索相关经验辅助编码。

```
🦀 > /exp

📚 经验库内容
============================================================
1. ✅ [reqwest] 使用 reqwest 发送 HTTP 请求
   📦 正确导入:
      use reqwest::Client;
   🔧 API 用法:
      Client::new().get(url).send().await
   💡 教训:
      - 需要启用 tokio runtime
```

## 示例

### 创建 HTTP 服务器

```
🦀 > /create 使用 axum 实现一个简单的 REST API 服务器
```

### 创建 CLI 工具

```
🦀 > /create 使用 clap 实现一个文件搜索命令行工具
```

### 使用 rig 框架

```
🦀 > /create 使用 rig 框架实现 Ollama 本地模型交互
```

## 依赖

- Go 1.22+
- Rust 工具链 (cargo, rustc)
- 智谱 API Key

## 配置

创建 `config.json`:

```json
{
  "api": {
    "zhipu_api_key": "your-api-key",
    "zhipu_base_url": "https://open.bigmodel.cn/api/paas/v4",
    "model": "glm-4-flash",
    "concurrency": 5
  },
  "workspace": {
    "root_dir": "./rust_workspace"
  }
}
```
