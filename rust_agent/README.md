# 🦀 Rust Code Agent

基于 LLM 的智能 Rust 代码生成、修复、审查工具。

## 功能特性

- **代码生成**: 根据自然语言需求生成 Rust 代码
- **自动修复**: 自动检测并修复编译错误（最多 5 次迭代）
- **代码审查**: 从正确性、安全性、性能等维度审查代码
- **代码解释**: 解释 Rust 代码的功能和设计
- **测试生成**: 为代码生成单元测试

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                      RustAgent (主控)                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ CodeGen     │  │ ErrorFix    │  │ Review      │         │
│  │ Agent       │  │ Agent       │  │ Agent       │         │
│  │ 代码生成     │  │ 错误修复    │  │ 代码审查    │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│                       Tool Layer                             │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ cargo check │  │ cargo build │  │ cargo run   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 目录结构

```
rust_agent/
├── cmd/
│   └── main.go          # 入口（CLI + 交互模式）
├── agent/
│   ├── rust_agent.go    # 主 Agent
│   ├── codegen.go       # 代码生成
│   ├── errorfix.go      # 错误修复
│   └── review.go        # 代码审查
├── config/
│   └── config.go        # 配置
├── model/
│   └── model.go         # LLM 模型接口
├── tool/
│   └── cargo.go         # Cargo 工具封装
└── rust_workspace/      # 生成的项目目录
```

## 工作流程

```
需求输入 → 代码生成 → cargo check → 编译成功?
                           ↓ No          ↓ Yes
                      错误修复 ←──────  cargo run
                           ↓
                      重新检查 (最多5次)
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

- `gen <需求>` - 生成代码
- `fix` - 修复上次生成的代码
- `review` - 审查代码
- `explain` - 解释代码
- `test` - 生成测试
- `show` - 显示当前代码
- `quit` - 退出

### 命令行模式

```bash
# 生成代码
rust-agent.exe -mode generate -req "实现一个简单的 HTTP 服务器"

# 审查代码
cat main.rs | rust-agent.exe -mode review

# 修复代码
cat main.rs | rust-agent.exe -mode fix

# 解释代码
cat main.rs | rust-agent.exe -mode explain
```

## 示例

### 生成 Hello World

```
🦀 > gen 打印 Hello World
```

### 生成 HTTP 服务器

```
🦀 > gen 使用 std::net 实现一个简单的 TCP echo 服务器
```

### 生成数据结构

```
🦀 > gen 实现一个泛型的二叉搜索树，支持插入、查找、删除操作
```

## 依赖

- Go 1.22+
- Rust 工具链 (cargo, rustc)
- 智谱 API Key

## 配置

编辑 `config/config.go` 修改:

- `ZhipuAPIKey`: API Key
- `Model`: 模型名称
- `WorkspaceDir`: 工作目录
- `CargoPath`: Cargo 路径
