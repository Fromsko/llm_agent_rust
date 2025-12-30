package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"rust-agent/config"
)

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Model LLM 模型
type Model struct {
	cfg        config.ModelConfig
	httpClient *http.Client
	limiter    *RateLimiter
}

// New 创建模型
func New(cfg config.ModelConfig) *Model {
	return &Model{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		limiter: NewRateLimiter(cfg.Concurrency),
	}
}

// Generate 生成
func (m *Model) Generate(ctx context.Context, messages []Message) (string, error) {
	if err := m.limiter.Acquire(ctx); err != nil {
		return "", err
	}
	defer m.limiter.Release()

	reqBody := map[string]any{
		"model":       m.cfg.ModelName,
		"messages":    messages,
		"temperature": 0.3, // 代码生成用低温度
		"max_tokens":  4096,
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", m.cfg.BaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("api error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response")
	}

	return result.Choices[0].Message.Content, nil
}

// Chat 对话
func (m *Model) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return m.Generate(ctx, messages)
}

// RateLimiter 速率限制
type RateLimiter struct {
	tokens chan struct{}
	mu     sync.Mutex
}

func NewRateLimiter(n int) *RateLimiter {
	rl := &RateLimiter{tokens: make(chan struct{}, n)}
	for i := 0; i < n; i++ {
		rl.tokens <- struct{}{}
	}
	return rl
}

func (rl *RateLimiter) Acquire(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (rl *RateLimiter) Release() {
	select {
	case rl.tokens <- struct{}{}:
	default:
	}
}
