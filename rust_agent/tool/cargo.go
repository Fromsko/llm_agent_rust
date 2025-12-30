package tool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"rust-agent/config"
)

// CargoTool Cargo 工具集
type CargoTool struct {
	cfg     config.RustConfig
	workDir string
}

// NewCargoTool 创建 Cargo 工具
func NewCargoTool(cfg config.RustConfig, workDir string) *CargoTool {
	return &CargoTool{cfg: cfg, workDir: workDir}
}

// CargoResult Cargo 命令结果
type CargoResult struct {
	Success  bool
	Output   string
	Errors   []CompileError
	Warnings []string
	Duration time.Duration
}

// CompileError 编译错误
type CompileError struct {
	Level   string // error, warning
	Code    string // E0382, etc.
	Message string
	File    string
	Line    int
	Column  int
	Snippet string
	Help    string
}

// Init 初始化项目
func (t *CargoTool) Init(ctx context.Context, name string) (*CargoResult, error) {
	projectDir := filepath.Join(t.workDir, name)

	// 如果目录已存在，先删除
	os.RemoveAll(projectDir)

	cmd := exec.CommandContext(ctx, t.cfg.CargoPath, "new", name)
	cmd.Dir = t.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &CargoResult{
		Success:  err == nil,
		Output:   stdout.String() + stderr.String(),
		Duration: duration,
	}

	return result, nil
}

// Check 编译检查
func (t *CargoTool) Check(ctx context.Context, projectDir string) (*CargoResult, error) {
	cmd := exec.CommandContext(ctx, t.cfg.CargoPath, "check", "--message-format=short")
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &CargoResult{
		Success:  err == nil,
		Output:   stderr.String(),
		Duration: duration,
	}

	// 解析错误
	result.Errors = t.parseErrors(stderr.String())

	return result, nil
}

// Build 构建
func (t *CargoTool) Build(ctx context.Context, projectDir string) (*CargoResult, error) {
	cmd := exec.CommandContext(ctx, t.cfg.CargoPath, "build")
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &CargoResult{
		Success:  err == nil,
		Output:   stdout.String() + stderr.String(),
		Duration: duration,
	}

	result.Errors = t.parseErrors(stderr.String())

	return result, nil
}

// Run 运行
func (t *CargoTool) Run(ctx context.Context, projectDir string) (*CargoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, t.cfg.TestTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.cfg.CargoPath, "run")
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &CargoResult{
		Success:  err == nil,
		Output:   stdout.String(),
		Duration: duration,
	}

	if stderr.Len() > 0 {
		result.Output += "\n--- stderr ---\n" + stderr.String()
	}

	return result, nil
}

// Test 测试
func (t *CargoTool) Test(ctx context.Context, projectDir string) (*CargoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, t.cfg.TestTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.cfg.CargoPath, "test")
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &CargoResult{
		Success:  err == nil,
		Output:   stdout.String() + stderr.String(),
		Duration: duration,
	}

	return result, nil
}

// Clippy Lint 检查
func (t *CargoTool) Clippy(ctx context.Context, projectDir string) (*CargoResult, error) {
	args := append([]string{"clippy"}, t.cfg.ClippyArgs...)
	cmd := exec.CommandContext(ctx, t.cfg.CargoPath, args...)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &CargoResult{
		Success:  err == nil,
		Output:   stderr.String(),
		Duration: duration,
	}

	// 解析警告
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.Contains(line, "warning:") {
			result.Warnings = append(result.Warnings, line)
		}
	}

	return result, nil
}

// Fmt 格式化
func (t *CargoTool) Fmt(ctx context.Context, projectDir string) (*CargoResult, error) {
	cmd := exec.CommandContext(ctx, t.cfg.CargoPath, "fmt")
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	return &CargoResult{
		Success:  err == nil,
		Output:   stdout.String() + stderr.String(),
		Duration: duration,
	}, nil
}

// parseErrors 解析编译错误
func (t *CargoTool) parseErrors(output string) []CompileError {
	var errors []CompileError

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 简单解析: error[E0382]: borrow of moved value
		if strings.HasPrefix(line, "error") {
			err := CompileError{Level: "error"}

			// 提取错误码
			if idx := strings.Index(line, "["); idx != -1 {
				if end := strings.Index(line[idx:], "]"); end != -1 {
					err.Code = line[idx+1 : idx+end]
				}
			}

			// 提取消息
			if idx := strings.Index(line, ":"); idx != -1 {
				err.Message = strings.TrimSpace(line[idx+1:])
			} else {
				err.Message = line
			}

			errors = append(errors, err)
		}
	}

	return errors
}

// WriteFile 写入文件
func (t *CargoTool) WriteFile(projectDir, relativePath, content string) error {
	fullPath := filepath.Join(projectDir, relativePath)

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// ReadFile 读取文件
func (t *CargoTool) ReadFile(projectDir, relativePath string) (string, error) {
	fullPath := filepath.Join(projectDir, relativePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatError 格式化错误信息（用于发送给 LLM）
func FormatErrors(errors []CompileError) string {
	if len(errors) == 0 {
		return "No errors"
	}

	var sb strings.Builder
	for i, err := range errors {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, err.Code, err.Message))
		if err.File != "" {
			sb.WriteString(fmt.Sprintf("   Location: %s:%d:%d\n", err.File, err.Line, err.Column))
		}
		if err.Help != "" {
			sb.WriteString(fmt.Sprintf("   Help: %s\n", err.Help))
		}
	}
	return sb.String()
}
