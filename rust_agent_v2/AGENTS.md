# AGENTS.md - Rust Agent v2

This file documents essential information for AI agents working in this codebase.

## Project Overview

**Rust Agent v2** is a multi-agent collaborative system written in Go for generating Rust code. It uses a supervisor pattern with specialized agents to plan, generate, compile, and fix Rust projects automatically.

- **Language**: Go 1.21+
- **Module**: `rust_agent_v2`
- **Entry Point**: `cmd/main.go`

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Runner Layer                         │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   RustRunner                         │   │
│  │  (Session management, event handling, state tracking) │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                         Agent Layer                          │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                SupervisorAgent                       │   │
│  │  Analyze → Retrieve experience → Execute → Validate  │   │
│  │  → Reflect → Save experience                         │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐  │
│  │ Autonomous│ │ Autonomous│ │ CratesIO  │ │ Planner   │  │
│  │ Coder     │ │ Fixer     │ │ Agent     │ │ Agent     │  │
│  └───────────┘ └───────────┘ └───────────┘ └───────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                         Tool Layer                           │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐  │
│  │cargo_check│ │cargo_build│ │cargo_run  │ │cargo_test │  │
│  └───────────┘ └───────────┘ └───────────┘ └───────────┘  │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐  │
│  │cargo_clippy│ │cargo_fmt │ │file_read  │ │file_write │  │
│  └───────────┘ └───────────┘ └───────────┘ └───────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
rust_agent_v2/
├── cmd/
│   └── main.go              # CLI entry point (interactive + command modes)
├── agent/
│   ├── agent.go             # Agent interface and BaseAgent implementation
│   └── specialized/         # Specialized agents
│       ├── supervisor.go    # SupervisorAgent - coordinates all agents
│       ├── codegen.go       # AutonomousCoderAgent - generates code
│       ├── fixer.go         # FixerAgent - fixes compilation errors
│       ├── autonomous_fixer.go  # AutonomousFixerAgent - autonomous fixing
│       ├── cratesio.go      # CratesIOAgent - searches crates.io
│       ├── planner.go       # PlannerAgent - task planning
│       ├── executor.go      # ExecutorAgent - executes plans
│       ├── review.go        # ReviewAgent - code review
│       ├── docsearch.go     # DocSearchAgent - documentation search
│       └── errorfix.go      # ErrorFixAgent - error fixing
├── config/
│   └── config.go            # Configuration management
├── event/
│   └── event.go             # Event system for agent communication
├── graph/
│   └── graph.go             # Graph orchestration engine for workflows
├── mcp/
│   └── mcp.go               # MCP (Model Context Protocol) client
├── memory/
│   ├── memory.go            # Memory storage (InMemoryStore, FileStore)
│   └── experience.go        # Experience storage and knowledge base
├── model/
│   └── model.go             # LLM model interface (ZhipuModel implementation)
├── runner/
│   └── runner.go            # Runner with session management
├── tool/
│   ├── tool.go              # Tool interface and Registry
│   ├── cargo.go             # Cargo tools (check, build, run, test, clippy, init)
│   ├── file.go              # File tools (read, write, list, delete)
│   ├── web_tools.go         # Web tools
│   ├── rust_tools.go        # Rust-specific tools
│   └── rust_local_tools.go  # Local Rust tools
├── workflow/
│   └── workflow.go          # Workflow definitions
├── tests/                   # Test cases
│   ├── mcp_math/
│   ├── fixer/
│   ├── experience/
│   └── coder/
└── rust_workspace/          # Generated Rust projects directory
    └── .experience/         # Experience database
```

## Essential Commands

### Build

```bash
# Build the executable
go build -o rust-agent.exe ./cmd

# Or use the provided batch file (Windows)
run.bat
```

### Run

```bash
# Interactive mode
rust-agent.exe -i

# Command line mode - create project
rust-agent.exe -create "description"

# Search crates
rust-agent.exe -search "keywords"

# Specify workspace
rust-agent.exe -workspace "./my_workspace" -create "..."
```

### Interactive Mode Commands

Once in interactive mode (`-i`):

- `/create <description>` - Create project with supervisor (with experience learning)
- `/interactive <description>` - Interactive coding with ReAct + ask_user (multi-choice)
- `/direct <description>` - Direct coding mode (without supervisor)
- `/fix <project_name>` - Autonomous fix project
- `/search <keywords>` - Search crates.io
- `/list` - List created projects
- `/run <project_name>` - Run project
- `/exp` - View experience database
- `/quit` - Exit

### Test

```bash
# Run Go tests
go test ./...

# Run specific test
go test ./tests/...
```

## Configuration

Create `config.json` in the project root:

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

## Code Patterns

### Agent Pattern

All agents implement the `Agent` interface:

```go
type Agent interface {
    Run(ctx context.Context, input string, opts ...InvocationOption) (<-chan *event.Event, error)
    Name() string
}
```

### Creating a New Agent

```go
type MyAgent struct {
    name  string
    model model.Model
    tools *tool.Registry
}

func NewMyAgent(m model.Model, tools *tool.Registry) *MyAgent {
    return &MyAgent{
        name:  "my-agent",
        model: m,
        tools: tools,
    }
}

func (a *MyAgent) Name() string { return a.name }

func (a *MyAgent) Run(ctx context.Context, input string, opts ...agent.InvocationOption) (<-chan *event.Event, error) {
    eventChan := make(chan *event.Event, 100)
    
    go func() {
        defer close(eventChan)
        // Agent logic here
        event.EmitEvent(ctx, eventChan, event.NewResponseEvent(a.name, "result"))
        event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(a.name, result))
    }()
    
    return eventChan, nil
}
```

### Tool Pattern

Tools implement the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Run(ctx context.Context, input string) (string, error)
}
```

### Event System

Events are used for communication between agents and the runner:

```go
// Event types
const (
    TypeResponse   Type = "response"
    TypeError      Type = "error"
    TypeToolCall   Type = "tool_call"
    TypeToolResult Type = "tool_result"
    TypeMCPCall    Type = "mcp_call"
    TypeMCPResult  Type = "mcp_result"
    TypeCompletion Type = "completion"
    TypeProgress   Type = "progress"
    TypeState      Type = "state"
)

// Emit event
event.EmitEvent(ctx, eventChan, event.NewResponseEvent(agentName, content))
```

### Graph Workflow

Workflows are built using the graph builder:

```go
g := graph.NewBuilder("workflow-name").
    AddAgentNode("planner", plannerAgent).
    AddAgentNode("executor", executorAgent).
    AddNode("check", checkFunc).
    AddEdge("planner", "executor").
    AddConditionalEdge("check", "success", conditionFunc).
    SetEntryPoint("planner").
    SetEndNode("end").
    Build()
```

## Naming Conventions

- **Packages**: lowercase, no underscores (e.g., `agent`, `tool`, `memory`)
- **Files**: snake_case (e.g., `cargo.go`, `experience.go`)
- **Types**: PascalCase (e.g., `SupervisorAgent`, `CargoCheck`)
- **Interfaces**: PascalCase ending with "er" (e.g., `Agent`, `Tool`, `Model`)
- **Functions**: PascalCase for exported, camelCase for internal
- **Variables**: camelCase
- **Constants**: PascalCase or ALL_CAPS for exported

## Key Components

### SupervisorAgent (`agent/specialized/supervisor.go`)

The main orchestrator that:
1. Analyzes tasks to detect required crates
2. Retrieves relevant experience from knowledge base
3. Executes coding tasks via AutonomousCoderAgent
4. Validates results with cargo check
5. Attempts auto-fix if compilation fails
6. Saves experience for future use

### ExperienceStore (`memory/experience.go`)

Stores successful/failed experiences:
- Task descriptions
- Correct imports
- API usage patterns
- Errors encountered
- Solutions and lessons learned

### KnowledgeBase (`memory/experience.go`)

Pre-configured knowledge for common crates:
- rig-core
- rmcp
- tokio
- reqwest
- serde

### ZhipuModel (`model/model.go`)

LLM client for Zhipu AI API:
- Supports tool calling
- Rate limiting with concurrency control
- Streaming support (simulated)

### AgentLoop with ReAct (`agent/loop.go`)

Enhanced agent loop supporting ReAct (Reasoning + Acting) pattern:

```go
loop := agent.NewAgentLoop(
    agent.WithLoopName("my-loop"),
    agent.WithLoopModel(m),
    agent.WithLoopTools(tools),
    agent.WithLoopSystemPrompt(prompt),
    agent.WithLoopMaxIter(10),
    agent.WithLoopReActMode(true),
)

result, err := loop.Run(ctx, "task description")
```

ReAct format:
```
Thought: [reasoning about what to do]
Action: [tool name]
Input: [JSON parameters]
```

### AskUser Tool (`tool/ask_user.go`)

Interactive user input tool with multi-choice support:

```go
// Simple question
result, _ := tool.InteractiveAskUser("What is your name?", nil, true)

// Multi-choice selection
choice, _ := tool.InteractiveAskUser(
    "Select framework:",
    []string{"tokio", "async-std", "smol"},
    false,
)

// Yes/No confirmation
confirmed, _ := tool.ConfirmYesNo("Continue?", true)

// Free text input
text, _ := tool.AskWithFreeText("Describe your requirements:")
```

Tool parameters:
- `question`: The question to ask
- `options`: Optional list of choices
- `allow_free_text`: Allow custom input when options provided
- `default`: Default value if user presses Enter
- `context`: Additional context for the question

### InteractiveCoderAgent (`agent/specialized/interactive_coder.go`)

Interactive coding agent combining ReAct + ask_user:

Workflow:
1. **Clarify requirements** - Ask user for clarification if needed
2. **Select crates** - Present technology options, let user choose
3. **Implement code** - Use ReAct loop to generate code
4. **Confirm completion** - Show results and ask for confirmation

Usage:
```bash
# Interactive mode
rust-agent -i
> /interactive "create an HTTP server"

# Or direct command
rust-agent -interactive "create a CLI tool"
```

## Important Gotchas

1. **Event Channel Buffering**: Always create event channels with sufficient buffer (`make(chan *event.Event, 100)`) to prevent blocking.

2. **Context Cancellation**: Check `ctx.Done()` in long-running agent loops to handle cancellation properly.

3. **JSON Parsing**: Tool inputs/outputs are JSON strings. Use `tool.ParseInput()` and `tool.FormatOutput()` for consistency.

4. **Path Handling**: On Windows, paths need escaping when passed as JSON. Use `strings.ReplaceAll(path, "\\", "\\")`.

5. **Experience Storage**: Experience is stored in `{workspace}/.experience/` as JSON files. The store auto-loads on creation.

6. **Cargo Tool Timeouts**: Each cargo tool has specific timeouts:
   - `cargo_check`: 60s
   - `cargo_build`: 120s
   - `cargo_run`: 30s
   - `cargo_test`: 60s
   - `cargo_clippy`: 60s

7. **Model Rate Limiting**: The ZhipuModel has built-in rate limiting. Configure concurrency via `ZhipuWithConcurrency(n)`.

8. **Graph State**: Use `graph.State` (map[string]any) for passing data between nodes. Clone state when needed to avoid mutations.

9. **Tool Registry**: Use `tool.CreateAdvancedRegistry()` or `tool.CreateDefaultRegistry()` to get pre-configured tool sets.

10. **MCP Client**: MCP clients use stdio transport. Ensure `uvx` and `mcp-server-fetch` are available for fetch functionality.

## Testing

Test files are in the `tests/` directory:

- `tests/mcp_math/test_mcp_math.go` - Tests MCP math tools with rmcp
- `tests/fixer/test_fix.go` - Tests fixer agent
- `tests/experience/test_experience.go` - Tests experience storage
- `tests/coder/test_coder.go` - Tests coder agent

Run tests with:
```bash
cd tests/mcp_math && go run test_mcp_math.go
```

## Dependencies

External requirements:
- Go 1.21+
- Rust toolchain (cargo, rustc)
- Zhipu API Key
- uvx (for MCP fetch server)

Go module dependencies (see `go.mod`):
- Standard library only (no external dependencies)

## Adding New Features

### Adding a New Tool

1. Create tool struct in appropriate file (e.g., `tool/cargo.go`)
2. Implement `Tool` interface
3. Register in `CreateDefaultRegistry()` or `CreateAdvancedRegistry()`

### Adding a New Agent

1. Create file in `agent/specialized/`
2. Implement `Agent` interface
3. Use `BaseAgent` for common functionality if needed
4. Emit proper events for progress, responses, and completion

### Adding Knowledge for New Crate

Add to `NewKnowledgeBase()` in `memory/experience.go`:

```go
kb.AddKnowledge(&CrateKnowledge{
    Name:      "crate-name",
    CargoName: "crate-name",
    CodeName:  "crate_name",
    RequiredTraits: []string{"trait1", "trait2"},
    CommonImports: []string{"use crate_name::..."},
    ExampleCode: `...`,
    Gotchas: []string{"..."},
})
```
