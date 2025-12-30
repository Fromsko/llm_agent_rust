package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CargoCheck cargo check 工具
type CargoCheck struct {
	cargoPath string
	timeout   time.Duration
}

func NewCargoCheck() *CargoCheck {
	return &CargoCheck{cargoPath: "cargo", timeout: 60 * time.Second}
}

func (t *CargoCheck) Name() string        { return "cargo_check" }
func (t *CargoCheck) Description() string { return "运行 cargo check 检查 Rust 代码编译错误" }
func (t *CargoCheck) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录路径"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoCheck) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.cargoPath, "check", "--message-format=short")
	cmd.Dir = args.ProjectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := map[string]any{
		"success": err == nil,
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
	}

	if err != nil {
		result["errors"] = parseCargoErrors(stderr.String())
	}

	return FormatOutput(result), nil
}

// CargoBuild cargo build 工具
type CargoBuild struct {
	cargoPath string
	timeout   time.Duration
}

func NewCargoBuild() *CargoBuild {
	return &CargoBuild{cargoPath: "cargo", timeout: 120 * time.Second}
}

func (t *CargoBuild) Name() string        { return "cargo_build" }
func (t *CargoBuild) Description() string { return "运行 cargo build 构建 Rust 项目" }
func (t *CargoBuild) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录路径"},
			"release":     map[string]any{"type": "boolean", "description": "是否 release 模式"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoBuild) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
		Release    bool   `json:"release"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmdArgs := []string{"build"}
	if args.Release {
		cmdArgs = append(cmdArgs, "--release")
	}

	cmd := exec.CommandContext(ctx, t.cargoPath, cmdArgs...)
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

// CargoRun cargo run 工具
type CargoRun struct {
	cargoPath string
	timeout   time.Duration
}

func NewCargoRun() *CargoRun {
	return &CargoRun{cargoPath: "cargo", timeout: 30 * time.Second}
}

func (t *CargoRun) Name() string        { return "cargo_run" }
func (t *CargoRun) Description() string { return "运行 cargo run 执行 Rust 程序" }
func (t *CargoRun) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录路径"},
			"args":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "程序参数"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoRun) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string   `json:"project_dir"`
		Args       []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmdArgs := []string{"run"}
	if len(args.Args) > 0 {
		cmdArgs = append(cmdArgs, "--")
		cmdArgs = append(cmdArgs, args.Args...)
	}

	cmd := exec.CommandContext(ctx, t.cargoPath, cmdArgs...)
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

// CargoTest cargo test 工具
type CargoTest struct {
	cargoPath string
	timeout   time.Duration
}

func NewCargoTest() *CargoTest {
	return &CargoTest{cargoPath: "cargo", timeout: 60 * time.Second}
}

func (t *CargoTest) Name() string        { return "cargo_test" }
func (t *CargoTest) Description() string { return "运行 cargo test 执行测试" }
func (t *CargoTest) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录路径"},
			"test_name":   map[string]any{"type": "string", "description": "指定测试名称（可选）"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoTest) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
		TestName   string `json:"test_name"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmdArgs := []string{"test"}
	if args.TestName != "" {
		cmdArgs = append(cmdArgs, args.TestName)
	}

	cmd := exec.CommandContext(ctx, t.cargoPath, cmdArgs...)
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

// CargoClippy cargo clippy 工具
type CargoClippy struct {
	cargoPath string
	timeout   time.Duration
}

func NewCargoClippy() *CargoClippy {
	return &CargoClippy{cargoPath: "cargo", timeout: 60 * time.Second}
}

func (t *CargoClippy) Name() string        { return "cargo_clippy" }
func (t *CargoClippy) Description() string { return "运行 cargo clippy 进行代码 lint 检查" }
func (t *CargoClippy) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_dir": map[string]any{"type": "string", "description": "项目目录路径"},
		},
		"required": []string{"project_dir"},
	}
}

func (t *CargoClippy) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.cargoPath, "clippy", "--", "-W", "clippy::all")
	cmd.Dir = args.ProjectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return FormatOutput(map[string]any{
		"success":  err == nil,
		"stdout":   stdout.String(),
		"stderr":   stderr.String(),
		"warnings": parseClippyWarnings(stderr.String()),
	}), nil
}

// CargoInit cargo new 工具
type CargoInit struct {
	cargoPath string
}

func NewCargoInit() *CargoInit {
	return &CargoInit{cargoPath: "cargo"}
}

func (t *CargoInit) Name() string        { return "cargo_init" }
func (t *CargoInit) Description() string { return "创建新的 Rust 项目" }
func (t *CargoInit) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"work_dir":     map[string]any{"type": "string", "description": "工作目录"},
			"project_name": map[string]any{"type": "string", "description": "项目名称"},
			"lib":          map[string]any{"type": "boolean", "description": "是否创建库项目"},
		},
		"required": []string{"work_dir", "project_name"},
	}
}

func (t *CargoInit) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		WorkDir     string `json:"work_dir"`
		ProjectName string `json:"project_name"`
		Lib         bool   `json:"lib"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 确保工作目录存在
	os.MkdirAll(args.WorkDir, 0755)

	// 删除已存在的项目
	projectDir := filepath.Join(args.WorkDir, args.ProjectName)
	os.RemoveAll(projectDir)

	cmdArgs := []string{"new", args.ProjectName}
	if args.Lib {
		cmdArgs = append(cmdArgs, "--lib")
	}

	cmd := exec.CommandContext(ctx, t.cargoPath, cmdArgs...)
	cmd.Dir = args.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return FormatOutput(map[string]any{
		"success":     err == nil,
		"project_dir": projectDir,
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
	}), nil
}

// 辅助函数
func parseCargoErrors(output string) []map[string]string {
	var errors []map[string]string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "error") {
			errors = append(errors, map[string]string{"message": line})
		}
	}
	return errors
}

func parseClippyWarnings(output string) []string {
	var warnings []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "warning:") {
			warnings = append(warnings, strings.TrimSpace(line))
		}
	}
	return warnings
}
