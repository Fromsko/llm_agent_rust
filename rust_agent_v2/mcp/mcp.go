package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Client MCP 客户端接口
type Client interface {
	Call(ctx context.Context, method string, params map[string]any) (any, error)
	ListTools(ctx context.Context) ([]ToolInfo, error)
	Close() error
}

// ToolInfo MCP 工具信息
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// StdioClient 基于 stdio 的 MCP 客户端
type StdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
	reqID  int
}

// NewStdioClient 创建 stdio MCP 客户端
func NewStdioClient(command string, args ...string) (*StdioClient, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	client := &StdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}

	// 初始化
	_, err = client.Call(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "rust-agent",
			"version": "1.0.0",
		},
	})
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	return client, nil
}

// Call 调用 MCP 方法
func (c *StdioClient) Call(ctx context.Context, method string, params map[string]any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reqID++
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.reqID,
		"method":  method,
		"params":  params,
	}

	reqBytes, _ := json.Marshal(req)
	reqBytes = append(reqBytes, '\n')

	if _, err := c.stdin.Write(reqBytes); err != nil {
		return nil, err
	}

	// 读取响应
	buf := make([]byte, 65536)
	n, err := c.stdout.Read(buf)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error: %s", resp.Error.Message)
	}

	return resp.Result, nil
}

// ListTools 列出可用工具
func (c *StdioClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	result, err := c.Call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(result)
	var resp struct {
		Tools []ToolInfo `json:"tools"`
	}
	json.Unmarshal(data, &resp)

	return resp.Tools, nil
}

// CallTool 调用工具
func (c *StdioClient) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	result, err := c.Call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	data, _ := json.Marshal(result)
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(data, &resp)

	if len(resp.Content) > 0 {
		return resp.Content[0].Text, nil
	}

	return fmt.Sprintf("%v", result), nil
}

// Close 关闭客户端
func (c *StdioClient) Close() error {
	c.stdin.Close()
	return c.cmd.Process.Kill()
}

// FetchMCP fetch MCP 客户端（HTML 转 Markdown）
type FetchMCP struct {
	client *StdioClient
}

// NewFetchMCP 创建 fetch MCP 客户端
func NewFetchMCP() (*FetchMCP, error) {
	// 使用 uvx 运行 fetch MCP
	client, err := NewStdioClient("uvx", "mcp-server-fetch")
	if err != nil {
		return nil, err
	}
	return &FetchMCP{client: client}, nil
}

// Fetch 获取 URL 内容并转换为 Markdown
func (m *FetchMCP) Fetch(ctx context.Context, url string) (string, error) {
	return m.client.CallTool(ctx, "fetch", map[string]any{
		"url": url,
	})
}

// Close 关闭
func (m *FetchMCP) Close() error {
	return m.client.Close()
}

// MCPRegistry MCP 注册表
type MCPRegistry struct {
	mu      sync.RWMutex
	clients map[string]Client
}

func NewMCPRegistry() *MCPRegistry {
	return &MCPRegistry{clients: make(map[string]Client)}
}

func (r *MCPRegistry) Register(name string, client Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[name] = client
}

func (r *MCPRegistry) Get(name string) (Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clients[name]
	return c, ok
}

func (r *MCPRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.clients {
		c.Close()
	}
}

// SimpleFetch 简单的 HTTP fetch（不依赖 MCP）
func SimpleFetch(ctx context.Context, url string) (string, error) {
	// 使用 curl 获取内容
	cmd := exec.CommandContext(ctx, "curl", "-s", "-L", url)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
