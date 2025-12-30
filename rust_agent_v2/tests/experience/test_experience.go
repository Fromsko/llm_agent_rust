package main

import (
	"context"
	"fmt"
	"os"

	"rust_agent_v2/memory"
)

func main() {
	workspace := "./rust_workspace"
	os.MkdirAll(workspace+"/.experience", 0755)

	// 创建经验存储
	expStore, err := memory.NewExperienceStore(workspace + "/.experience")
	if err != nil {
		fmt.Printf("❌ 创建经验存储失败: %v\n", err)
		return
	}

	// 创建一个测试经验
	exp := &memory.Experience{
		Task:      "code_generation",
		CrateName: "rmcp",
		Success:   true,
		Code: `use rmcp::{ServerHandler, tool, tool_router};

#[derive(Clone)]
struct MathServer {
    tool_router: ToolRouter<Self>,
}

#[tool_router]
impl MathServer {
    #[tool(description = "Add two numbers")]
    async fn add(&self, a: f64, b: f64) -> Result<CallToolResult, McpError> {
        Ok(CallToolResult::success(vec![Content::text(format!("{}", a + b))]))
    }
}`,
		Imports: []string{
			"use rmcp::{ServerHandler, tool, tool_router, handler::server::tool::ToolRouter};",
			"use rmcp::model::{CallToolResult, Content};",
			"use rmcp::ErrorData as McpError;",
		},
		APIUsage: []string{
			"#[tool_router]",
			"#[tool(description = \"...\")]",
			"CallToolResult::success(vec![Content::text(...)])",
			".serve(stdio()).await?",
		},
		Lessons: []string{
			"使用 #[tool_router] 宏定义工具路由",
			"需要实现 ServerHandler trait",
			"使用 .serve(stdio()) 启动 stdio 服务",
			"rmcp 需要 features = [\"server\", \"macros\"]",
		},
		Tags: []string{"rmcp", "mcp", "math"},
	}

	// 保存经验
	if err := expStore.Save(context.Background(), exp); err != nil {
		fmt.Printf("❌ 保存经验失败: %v\n", err)
		return
	}

	fmt.Println("✅ 经验保存成功！")

	// 读取并显示
	experiences := expStore.FindByCrate("rmcp")
	fmt.Printf("\n📚 找到 %d 条关于 rmcp 的经验\n", len(experiences))

	for _, e := range experiences {
		fmt.Printf("\n--- 经验 ID: %s ---\n", e.ID)
		fmt.Printf("成功: %v\n", e.Success)
		fmt.Printf("导入: %v\n", e.Imports)
		fmt.Printf("教训: %v\n", e.Lessons)
	}

	// 测试 FormatForPrompt
	fmt.Println("\n--- FormatForPrompt 输出 ---")
	fmt.Println(expStore.FormatForPrompt("rmcp"))
}
