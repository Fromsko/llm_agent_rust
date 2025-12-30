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
	"runtime"
	"strings"
)

// CargoDoc 生成并读取本地文档
type CargoDoc struct{}

func NewCargoDoc() *CargoDoc { return &CargoDoc{} }

func (t *CargoDoc) Name() string { return "cargo_doc" }
func (t *CargoDoc) Description() string {
	return "生成项目依赖的本地文档，并提取指定 crate 的 API 信息"
}
func (t *CargoDoc) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录"},
			"crate_name":  map[string]any{"type": "string", "description": "要查看文档的 crate 名称（可选）"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoDoc) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
		CrateName  string `json:"crate_name"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 先运行 cargo doc 生成文档
	cmd := exec.CommandContext(ctx, "cargo", "doc", "--no-deps")
	cmd.Dir = args.ProjectDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Run() // 忽略错误，可能部分成功

	// 查找生成的文档目录
	docDir := filepath.Join(args.ProjectDir, "target", "doc")
	if _, err := os.Stat(docDir); os.IsNotExist(err) {
		return FormatOutput(map[string]any{
			"success": false,
			"error":   "文档生成失败: " + stderr.String(),
		}), nil
	}

	// 如果指定了 crate，读取其文档
	if args.CrateName != "" {
		crateName := strings.ReplaceAll(args.CrateName, "-", "_")
		crateDocDir := filepath.Join(docDir, crateName)

		// 读取 index.html 或 all.html
		indexPath := filepath.Join(crateDocDir, "index.html")
		if content, err := os.ReadFile(indexPath); err == nil {
			doc := extractRustDoc(string(content))
			return FormatOutput(map[string]any{
				"success":  true,
				"crate":    args.CrateName,
				"doc_path": crateDocDir,
				"content":  doc,
			}), nil
		}
	}

	// 列出所有可用的文档
	var crates []string
	entries, _ := os.ReadDir(docDir)
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			crates = append(crates, entry.Name())
		}
	}

	return FormatOutput(map[string]any{
		"success":          true,
		"doc_dir":          docDir,
		"available_crates": crates,
		"hint":             "使用 crate_name 参数查看特定 crate 的文档",
	}), nil
}

// extractRustDoc 从 rustdoc HTML 中提取文档内容
func extractRustDoc(html string) map[string]any {
	doc := make(map[string]any)

	// 提取 crate 描述
	descRe := regexp.MustCompile(`<section id="main-content"[^>]*>([\s\S]*?)</section>`)
	if matches := descRe.FindStringSubmatch(html); len(matches) > 1 {
		doc["description"] = cleanHTML(matches[1])
	}

	// 提取模块列表
	moduleRe := regexp.MustCompile(`<a[^>]*href="([^"]+)/index\.html"[^>]*>([^<]+)</a>`)
	var modules []string
	for _, match := range moduleRe.FindAllStringSubmatch(html, 20) {
		if len(match) > 2 {
			modules = append(modules, match[2])
		}
	}
	if len(modules) > 0 {
		doc["modules"] = modules
	}

	// 提取结构体列表
	structRe := regexp.MustCompile(`<a[^>]*href="struct\.([^"]+)\.html"[^>]*>`)
	var structs []string
	for _, match := range structRe.FindAllStringSubmatch(html, 30) {
		if len(match) > 1 {
			structs = append(structs, match[1])
		}
	}
	if len(structs) > 0 {
		doc["structs"] = structs
	}

	// 提取 trait 列表
	traitRe := regexp.MustCompile(`<a[^>]*href="trait\.([^"]+)\.html"[^>]*>`)
	var traits []string
	for _, match := range traitRe.FindAllStringSubmatch(html, 30) {
		if len(match) > 1 {
			traits = append(traits, match[1])
		}
	}
	if len(traits) > 0 {
		doc["traits"] = traits
	}

	// 提取函数列表
	fnRe := regexp.MustCompile(`<a[^>]*href="fn\.([^"]+)\.html"[^>]*>`)
	var functions []string
	for _, match := range fnRe.FindAllStringSubmatch(html, 30) {
		if len(match) > 1 {
			functions = append(functions, match[1])
		}
	}
	if len(functions) > 0 {
		doc["functions"] = functions
	}

	return doc
}

// CrateSourceReader 读取本地 crate 源码
type CrateSourceReader struct{}

func NewCrateSourceReader() *CrateSourceReader { return &CrateSourceReader{} }

func (t *CrateSourceReader) Name() string { return "crate_source" }
func (t *CrateSourceReader) Description() string {
	return "读取本地已下载的 crate 源码，查看 API 定义和示例"
}
func (t *CrateSourceReader) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"crate_name": map[string]any{"type": "string", "description": "crate 名称"},
			"version":    map[string]any{"type": "string", "description": "版本号（可选，默认最新）"},
			"file":       map[string]any{"type": "string", "description": "要读取的文件（如 lib.rs, client.rs）"},
		},
		"required": []string{"crate_name"},
	}
}

func (t *CrateSourceReader) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		CrateName string `json:"crate_name"`
		Version   string `json:"version"`
		File      string `json:"file"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 获取 cargo registry 目录
	homeDir, _ := os.UserHomeDir()
	registryDir := filepath.Join(homeDir, ".cargo", "registry", "src")

	// 查找 crate 目录
	var crateDir string
	var foundVersion string

	// 遍历 registry 源
	filepath.Walk(registryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}

		name := info.Name()
		// 匹配 crate-name-version 格式
		if strings.HasPrefix(name, args.CrateName+"-") {
			version := strings.TrimPrefix(name, args.CrateName+"-")
			if args.Version == "" || version == args.Version {
				if foundVersion == "" || version > foundVersion {
					crateDir = path
					foundVersion = version
				}
			}
		}
		return nil
	})

	if crateDir == "" {
		return FormatOutput(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("未找到 crate %s，请先运行 cargo build 下载依赖", args.CrateName),
		}), nil
	}

	// 如果指定了文件，读取该文件
	if args.File != "" {
		filePath := filepath.Join(crateDir, "src", args.File)
		if !strings.HasSuffix(args.File, ".rs") {
			filePath = filepath.Join(crateDir, "src", args.File+".rs")
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			// 尝试直接在 crate 根目录
			filePath = filepath.Join(crateDir, args.File)
			content, err = os.ReadFile(filePath)
			if err != nil {
				return FormatOutput(map[string]any{
					"success": false,
					"error":   "文件不存在: " + args.File,
				}), nil
			}
		}

		return FormatOutput(map[string]any{
			"success": true,
			"crate":   args.CrateName,
			"version": foundVersion,
			"file":    args.File,
			"content": string(content),
		}), nil
	}

	// 列出 crate 的文件结构
	var files []string
	srcDir := filepath.Join(crateDir, "src")
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".rs") {
			relPath, _ := filepath.Rel(srcDir, path)
			files = append(files, relPath)
		}
		return nil
	})

	// 读取 lib.rs 的文档注释
	libContent, _ := os.ReadFile(filepath.Join(srcDir, "lib.rs"))
	docComments := extractDocComments(string(libContent))

	return FormatOutput(map[string]any{
		"success":      true,
		"crate":        args.CrateName,
		"version":      foundVersion,
		"path":         crateDir,
		"files":        files,
		"doc_comments": docComments,
		"hint":         "使用 file 参数读取具体文件内容",
	}), nil
}

// extractDocComments 提取 Rust 文档注释
func extractDocComments(content string) []string {
	var comments []string

	// 匹配 //! 和 /// 注释
	lines := strings.Split(content, "\n")
	var currentComment strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//!") || strings.HasPrefix(trimmed, "///") {
			comment := strings.TrimPrefix(trimmed, "//!")
			comment = strings.TrimPrefix(comment, "///")
			comment = strings.TrimSpace(comment)
			if comment != "" {
				currentComment.WriteString(comment)
				currentComment.WriteString(" ")
			}
		} else if currentComment.Len() > 0 {
			comments = append(comments, strings.TrimSpace(currentComment.String()))
			currentComment.Reset()
		}
	}

	if currentComment.Len() > 0 {
		comments = append(comments, strings.TrimSpace(currentComment.String()))
	}

	// 只返回前 10 个有意义的注释
	var result []string
	for _, c := range comments {
		if len(c) > 20 {
			result = append(result, c)
			if len(result) >= 10 {
				break
			}
		}
	}

	return result
}

// RustAnalyzer 使用 rust-analyzer 获取类型信息
type RustAnalyzer struct{}

func NewRustAnalyzer() *RustAnalyzer { return &RustAnalyzer{} }

func (t *RustAnalyzer) Name() string { return "rust_analyzer" }
func (t *RustAnalyzer) Description() string {
	return "使用 rust-analyzer 分析代码，获取类型信息、补全建议等"
}
func (t *RustAnalyzer) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录"},
			"query":       map[string]any{"type": "string", "description": "查询内容（如 rig::providers::openai）"},
		},
		"required": []string{"project_dir", "query"},
	}
}

func (t *RustAnalyzer) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
		Query      string `json:"query"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 检查 rust-analyzer 是否可用
	if _, err := exec.LookPath("rust-analyzer"); err != nil {
		return FormatOutput(map[string]any{
			"success": false,
			"error":   "rust-analyzer 未安装，请运行: rustup component add rust-analyzer",
		}), nil
	}

	// 使用 cargo check 获取更多信息
	cmd := exec.CommandContext(ctx, "cargo", "check", "--message-format=json")
	cmd.Dir = args.ProjectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run()

	// 解析输出
	var messages []map[string]any
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			if reason, ok := msg["reason"].(string); ok && reason == "compiler-message" {
				messages = append(messages, msg)
			}
		}
	}

	return FormatOutput(map[string]any{
		"success":  true,
		"query":    args.Query,
		"messages": messages,
		"stderr":   stderr.String(),
	}), nil
}

// CargoTree 查看依赖树
type CargoTree struct{}

func NewCargoTree() *CargoTree { return &CargoTree{} }

func (t *CargoTree) Name() string { return "cargo_tree" }
func (t *CargoTree) Description() string {
	return "查看项目的依赖树，了解 crate 的实际名称和版本"
}
func (t *CargoTree) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录"},
			"crate_name":  map[string]any{"type": "string", "description": "要查看的 crate（可选）"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoTree) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
		CrateName  string `json:"crate_name"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	cmdArgs := []string{"tree", "--depth", "2"}
	if args.CrateName != "" {
		cmdArgs = append(cmdArgs, "-i", args.CrateName)
	}

	cmd := exec.CommandContext(ctx, "cargo", cmdArgs...)
	cmd.Dir = args.ProjectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	return FormatOutput(map[string]any{
		"success": err == nil,
		"tree":    stdout.String(),
		"stderr":  stderr.String(),
	}), nil
}

// CargoMetadata 获取项目元数据
type CargoMetadata struct{}

func NewCargoMetadata() *CargoMetadata { return &CargoMetadata{} }

func (t *CargoMetadata) Name() string { return "cargo_metadata" }
func (t *CargoMetadata) Description() string {
	return "获取项目的 Cargo 元数据，包括依赖的实际 crate 名称、版本、features 等"
}
func (t *CargoMetadata) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoMetadata) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "cargo", "metadata", "--format-version", "1", "--no-deps")
	cmd.Dir = args.ProjectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	var metadata map[string]any
	json.Unmarshal(stdout.Bytes(), &metadata)

	// 提取关键信息
	var deps []map[string]any
	if packages, ok := metadata["packages"].([]any); ok {
		for _, pkg := range packages {
			if p, ok := pkg.(map[string]any); ok {
				deps = append(deps, map[string]any{
					"name":     p["name"],
					"version":  p["version"],
					"features": p["features"],
				})
			}
		}
	}

	return FormatOutput(map[string]any{
		"success":  true,
		"packages": deps,
	}), nil
}

// GetCargoHome 获取 cargo home 目录
func GetCargoHome() string {
	if home := os.Getenv("CARGO_HOME"); home != "" {
		return home
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), ".cargo")
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cargo")
}
