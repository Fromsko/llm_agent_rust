package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// RustErrorAnalyzer Rust 错误分析工具
type RustErrorAnalyzer struct{}

func NewRustErrorAnalyzer() *RustErrorAnalyzer { return &RustErrorAnalyzer{} }

func (t *RustErrorAnalyzer) Name() string { return "rust_error_analyzer" }
func (t *RustErrorAnalyzer) Description() string {
	return "分析 Rust 编译错误，提取错误位置、类型和建议"
}
func (t *RustErrorAnalyzer) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"error_output": map[string]any{"type": "string", "description": "cargo check/build 的错误输出"},
		},
		"required": []string{"error_output"},
	}
}

type RustError struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	ErrorCode  string `json:"error_code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

func (t *RustErrorAnalyzer) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ErrorOutput string `json:"error_output"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	errors := parseRustErrors(args.ErrorOutput)
	return FormatOutput(map[string]any{
		"errors": errors,
		"count":  len(errors),
	}), nil
}

func parseRustErrors(output string) []RustError {
	var errors []RustError

	// 匹配 error[E0277]: message
	errorPattern := regexp.MustCompile(`error\[([A-Z]\d+)\]:\s*(.+)`)
	// 匹配 file:line:col
	locationPattern := regexp.MustCompile(`([^:\s]+\.rs):(\d+):(\d+)`)
	// 匹配 help: suggestion
	helpPattern := regexp.MustCompile(`help:\s*(.+)`)

	lines := strings.Split(output, "\n")
	var currentError *RustError

	for _, line := range lines {
		if matches := errorPattern.FindStringSubmatch(line); len(matches) > 0 {
			if currentError != nil {
				errors = append(errors, *currentError)
			}
			currentError = &RustError{
				ErrorCode: matches[1],
				Message:   matches[2],
			}
		}

		if currentError != nil {
			if matches := locationPattern.FindStringSubmatch(line); len(matches) > 0 {
				currentError.File = matches[1]
				fmt.Sscanf(matches[2], "%d", &currentError.Line)
				fmt.Sscanf(matches[3], "%d", &currentError.Column)
			}

			if matches := helpPattern.FindStringSubmatch(line); len(matches) > 0 {
				currentError.Suggestion = matches[1]
			}
		}
	}

	if currentError != nil {
		errors = append(errors, *currentError)
	}

	return errors
}

// FileEditor 文件编辑工具（支持 diff 式修改）
type FileEditor struct{}

func NewFileEditor() *FileEditor { return &FileEditor{} }

func (t *FileEditor) Name() string { return "file_editor" }
func (t *FileEditor) Description() string {
	return "编辑文件的指定行，支持替换、插入、删除操作"
}
func (t *FileEditor) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":       map[string]any{"type": "string", "description": "文件路径"},
			"operation":  map[string]any{"type": "string", "enum": []string{"replace", "insert", "delete"}, "description": "操作类型"},
			"start_line": map[string]any{"type": "integer", "description": "起始行号（从1开始）"},
			"end_line":   map[string]any{"type": "integer", "description": "结束行号（replace/delete时使用）"},
			"content":    map[string]any{"type": "string", "description": "新内容（replace/insert时使用）"},
		},
		"required": []string{"path", "operation", "start_line"},
	}
}

func (t *FileEditor) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Path      string `json:"path"`
		Operation string `json:"operation"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Content   string `json:"content"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	lines := strings.Split(string(content), "\n")
	startIdx := args.StartLine - 1
	endIdx := args.EndLine - 1

	if startIdx < 0 || startIdx >= len(lines) {
		return FormatOutput(map[string]any{"success": false, "error": "行号超出范围"}), nil
	}

	var newLines []string
	switch args.Operation {
	case "replace":
		if endIdx < startIdx {
			endIdx = startIdx
		}
		newLines = append(newLines, lines[:startIdx]...)
		newLines = append(newLines, strings.Split(args.Content, "\n")...)
		if endIdx+1 < len(lines) {
			newLines = append(newLines, lines[endIdx+1:]...)
		}

	case "insert":
		newLines = append(newLines, lines[:startIdx]...)
		newLines = append(newLines, strings.Split(args.Content, "\n")...)
		newLines = append(newLines, lines[startIdx:]...)

	case "delete":
		if endIdx < startIdx {
			endIdx = startIdx
		}
		newLines = append(newLines, lines[:startIdx]...)
		if endIdx+1 < len(lines) {
			newLines = append(newLines, lines[endIdx+1:]...)
		}
	}

	if err := os.WriteFile(args.Path, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	return FormatOutput(map[string]any{
		"success":      true,
		"lines_before": len(lines),
		"lines_after":  len(newLines),
	}), nil
}

// CodeSearch 代码搜索工具
type CodeSearch struct{}

func NewCodeSearch() *CodeSearch { return &CodeSearch{} }

func (t *CodeSearch) Name() string        { return "code_search" }
func (t *CodeSearch) Description() string { return "在项目中搜索代码，支持正则表达式" }
func (t *CodeSearch) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir":  map[string]any{"type": "string", "description": "项目目录"},
			"pattern":      map[string]any{"type": "string", "description": "搜索模式（正则表达式）"},
			"file_pattern": map[string]any{"type": "string", "description": "文件名模式（如 *.rs）"},
		},
		"required": []string{"project_dir", "pattern"},
	}
}

type SearchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
	Context string `json:"context"`
}

func (t *CodeSearch) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir  string `json:"project_dir"`
		Pattern     string `json:"pattern"`
		FilePattern string `json:"file_pattern"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	if args.FilePattern == "" {
		args.FilePattern = "*.rs"
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": "无效的正则表达式: " + err.Error()}), nil
	}

	var results []SearchResult

	filepath.Walk(args.ProjectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		matched, _ := filepath.Match(args.FilePattern, info.Name())
		if !matched {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				// 获取上下文（前后各2行）
				start := i - 2
				if start < 0 {
					start = 0
				}
				end := i + 3
				if end > len(lines) {
					end = len(lines)
				}
				context := strings.Join(lines[start:end], "\n")

				relPath, _ := filepath.Rel(args.ProjectDir, path)
				results = append(results, SearchResult{
					File:    relPath,
					Line:    i + 1,
					Content: strings.TrimSpace(line),
					Context: context,
				})
			}
		}
		return nil
	})

	return FormatOutput(map[string]any{
		"success": true,
		"results": results,
		"count":   len(results),
	}), nil
}

// RustDocLookup Rust 文档查询工具
type RustDocLookup struct{}

func NewRustDocLookup() *RustDocLookup { return &RustDocLookup{} }

func (t *RustDocLookup) Name() string        { return "rust_doc_lookup" }
func (t *RustDocLookup) Description() string { return "查询 Rust 标准库或 crate 的文档" }
func (t *RustDocLookup) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "查询内容（如 std::net::TcpListener）"},
			"crate": map[string]any{"type": "string", "description": "crate 名称（可选，默认 std）"},
		},
		"required": []string{"query"},
	}
}

func (t *RustDocLookup) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Query string `json:"query"`
		Crate string `json:"crate"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 内置常用 API 文档
	docs := map[string]string{
		"TcpListener": `std::net::TcpListener - TCP 监听器（同步版本）
用法: let listener = TcpListener::bind("127.0.0.1:8080")?;
注意: bind() 返回 Result<TcpListener, io::Error>，不是 Future，不能 .await
异步版本: 使用 tokio::net::TcpListener`,

		"tokio::net::TcpListener": `tokio::net::TcpListener - 异步 TCP 监听器
用法: let listener = TcpListener::bind("127.0.0.1:8080").await?;
方法:
  - bind(addr).await -> Result<TcpListener>
  - accept().await -> Result<(TcpStream, SocketAddr)>`,

		"tokio::net::TcpStream": `tokio::net::TcpStream - 异步 TCP 流
需要导入: use tokio::io::{AsyncReadExt, AsyncWriteExt};
方法:
  - stream.read(&mut buf).await -> Result<usize>
  - stream.write_all(data).await -> Result<()>
  - stream.shutdown().await -> Result<()>

示例:
use tokio::io::{AsyncReadExt, AsyncWriteExt};
let mut buf = [0u8; 1024];
let n = stream.read(&mut buf).await?;
stream.write_all(b"Hello").await?;`,

		"AsyncReadExt": `tokio::io::AsyncReadExt - 异步读取扩展 trait
需要导入: use tokio::io::AsyncReadExt;
方法:
  - read(&mut buf).await -> Result<usize>
  - read_exact(&mut buf).await -> Result<()>
  - read_to_end(&mut vec).await -> Result<usize>
  - read_to_string(&mut string).await -> Result<usize>`,

		"AsyncWriteExt": `tokio::io::AsyncWriteExt - 异步写入扩展 trait
需要导入: use tokio::io::AsyncWriteExt;
方法:
  - write(&buf).await -> Result<usize>
  - write_all(&buf).await -> Result<()>
  - flush().await -> Result<()>
  - shutdown().await -> Result<()>`,

		"io::read": `错误: tokio::io 模块没有 read 函数
正确用法: 使用 AsyncReadExt trait
示例:
use tokio::io::AsyncReadExt;
let mut buf = [0u8; 1024];
let n = stream.read(&mut buf).await?;`,

		"io::write": `错误: tokio::io 模块没有 write 函数
正确用法: 使用 AsyncWriteExt trait
示例:
use tokio::io::AsyncWriteExt;
stream.write_all(b"Hello World").await?;`,

		"Result": `std::result::Result<T, E> - 错误处理类型
用法: 
  - Ok(value) 表示成功
  - Err(error) 表示失败
  - ? 操作符自动传播错误
  - .await 用于 Future，不能用于 Result`,

		"Future": `std::future::Future - 异步计算
特点:
  - 需要 .await 来执行
  - 只有 async fn 或 async {} 返回 Future
  - std::net::TcpListener::bind() 不是 Future
  - tokio::net::TcpListener::bind() 是 Future`,

		"HTTP": `简单 HTTP 服务器示例:
use tokio::net::TcpListener;
use tokio::io::{AsyncReadExt, AsyncWriteExt};

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind("127.0.0.1:8080").await?;
    
    loop {
        let (mut socket, _) = listener.accept().await?;
        
        tokio::spawn(async move {
            let mut buf = [0u8; 1024];
            let _ = socket.read(&mut buf).await;
            
            let response = "HTTP/1.1 200 OK\r\nContent-Length: 11\r\n\r\nHello World";
            let _ = socket.write_all(response.as_bytes()).await;
        });
    }
}`,
	}

	// 查找匹配的文档
	var result string
	query := strings.ToLower(args.Query)
	for key, doc := range docs {
		if strings.Contains(strings.ToLower(key), query) ||
			strings.Contains(query, strings.ToLower(key)) {
			result += doc + "\n\n---\n\n"
		}
	}

	if result == "" {
		result = fmt.Sprintf("未找到 '%s' 的文档。建议查看 https://doc.rust-lang.org/std/ 或 https://docs.rs/", args.Query)
	}

	return FormatOutput(map[string]any{
		"success": true,
		"query":   args.Query,
		"doc":     result,
	}), nil
}

// CargoAdd 添加依赖工具
type CargoAdd struct{}

func NewCargoAdd() *CargoAdd { return &CargoAdd{} }

func (t *CargoAdd) Name() string        { return "cargo_add" }
func (t *CargoAdd) Description() string { return "添加 crate 依赖到项目" }
func (t *CargoAdd) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录"},
			"crate_name":  map[string]any{"type": "string", "description": "crate 名称"},
			"version":     map[string]any{"type": "string", "description": "版本（可选）"},
			"features":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "features（可选）"},
		},
		"required": []string{"project_dir", "crate_name"},
	}
}

func (t *CargoAdd) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string   `json:"project_dir"`
		CrateName  string   `json:"crate_name"`
		Version    string   `json:"version"`
		Features   []string `json:"features"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	cmdArgs := []string{"add", args.CrateName}
	if args.Version != "" {
		cmdArgs = append(cmdArgs, "--vers", args.Version)
	}
	if len(args.Features) > 0 {
		cmdArgs = append(cmdArgs, "--features", strings.Join(args.Features, ","))
	}

	cmd := exec.CommandContext(ctx, "cargo", cmdArgs...)
	cmd.Dir = args.ProjectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return FormatOutput(map[string]any{
		"success": err == nil,
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
	}), nil
}

// FileReadLines 读取文件指定行
type FileReadLines struct{}

func NewFileReadLines() *FileReadLines { return &FileReadLines{} }

func (t *FileReadLines) Name() string        { return "file_read_lines" }
func (t *FileReadLines) Description() string { return "读取文件的指定行范围" }
func (t *FileReadLines) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":       map[string]any{"type": "string", "description": "文件路径"},
			"start_line": map[string]any{"type": "integer", "description": "起始行号（从1开始）"},
			"end_line":   map[string]any{"type": "integer", "description": "结束行号"},
		},
		"required": []string{"path"},
	}
}

func (t *FileReadLines) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	lines := strings.Split(string(content), "\n")

	if args.StartLine <= 0 {
		args.StartLine = 1
	}
	if args.EndLine <= 0 || args.EndLine > len(lines) {
		args.EndLine = len(lines)
	}

	startIdx := args.StartLine - 1
	endIdx := args.EndLine

	if startIdx >= len(lines) {
		return FormatOutput(map[string]any{"success": false, "error": "起始行超出范围"}), nil
	}

	selectedLines := lines[startIdx:endIdx]

	// 添加行号
	var numberedLines []string
	for i, line := range selectedLines {
		numberedLines = append(numberedLines, fmt.Sprintf("%4d | %s", startIdx+i+1, line))
	}

	return FormatOutput(map[string]any{
		"success":     true,
		"content":     strings.Join(numberedLines, "\n"),
		"total_lines": len(lines),
		"start_line":  args.StartLine,
		"end_line":    args.EndLine,
	}), nil
}

// CreateAdvancedRegistry 创建高级工具注册表
func CreateAdvancedRegistry() *Registry {
	r := CreateDefaultRegistry()

	// 添加高级工具
	r.Register(NewRustErrorAnalyzer())
	r.Register(NewFileEditor())
	r.Register(NewCodeSearch())
	r.Register(NewRustDocLookup())
	r.Register(NewCargoAdd())
	r.Register(NewFileReadLines())

	// 添加网络搜索工具
	r.Register(NewCratesIOSearch())
	r.Register(NewCratesIOInfo())
	r.Register(NewDocsRSFetch())
	r.Register(NewWebSearch())
	r.Register(NewWebFetch())
	r.Register(NewGitHubReadme())

	// 添加本地 Rust 工具（利用 Rust 工具链）
	r.Register(NewCargoDoc())
	r.Register(NewCrateSourceReader())
	r.Register(NewCargoTree())
	r.Register(NewCargoMetadata())

	// 添加 ask_user 工具
	r.Register(NewAskUserTool())

	return r
}
