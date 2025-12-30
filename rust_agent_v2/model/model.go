package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Model 模型接口
type Model interface {
	Generate(ctx context.Context, messages []*Message, opts ...Option) (*Response, error)
	GenerateStream(ctx context.Context, messages []*Message, opts ...Option) (<-chan *Response, error)
	Name() string
}

// Message 消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response 响应
type Response struct {
	ID           string
	Content      string
	ToolCalls    []*ToolCall
	FinishReason string
	Usage        *Usage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Option 选项
type Option func(*options)

type options struct {
	Temperature *float64
	MaxTokens   *int
	Tools       []*ToolDef
}

type ToolDef struct {
	Type     string    `json:"type"`
	Function *Function `json:"function"`
}

type Function struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func WithTemperature(t float64) Option {
	return func(o *options) { o.Temperature = &t }
}

func WithMaxTokens(n int) Option {
	return func(o *options) { o.MaxTokens = &n }
}

func WithTools(tools ...*ToolDef) Option {
	return func(o *options) { o.Tools = tools }
}

// ZhipuModel 智谱模型实现
type ZhipuModel struct {
	name       string
	apiKey     string
	baseURL    string
	httpClient *http.Client
	limiter    *RateLimiter
}

type ZhipuOption func(*ZhipuModel)

func ZhipuWithAPIKey(key string) ZhipuOption {
	return func(m *ZhipuModel) { m.apiKey = key }
}

func ZhipuWithBaseURL(url string) ZhipuOption {
	return func(m *ZhipuModel) { m.baseURL = url }
}

func ZhipuWithConcurrency(n int) ZhipuOption {
	return func(m *ZhipuModel) { m.limiter = NewRateLimiter(n) }
}

func NewZhipuModel(name string, opts ...ZhipuOption) *ZhipuModel {
	m := &ZhipuModel{
		name:       name,
		baseURL:    "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		httpClient: &http.Client{Timeout: 120 * time.Second},
		limiter:    NewRateLimiter(50),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *ZhipuModel) Name() string { return m.name }

func (m *ZhipuModel) Generate(ctx context.Context, messages []*Message, opts ...Option) (*Response, error) {
	if err := m.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer m.limiter.Release()

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	reqBody := map[string]any{
		"model":    m.name,
		"messages": messages,
	}
	if o.Temperature != nil {
		reqBody["temperature"] = *o.Temperature
	} else {
		reqBody["temperature"] = 0.3
	}
	if o.MaxTokens != nil {
		reqBody["max_tokens"] = *o.MaxTokens
	} else {
		reqBody["max_tokens"] = 4096
	}
	if len(o.Tools) > 0 {
		reqBody["tools"] = o.Tools
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("api error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response")
	}

	choice := result.Choices[0]
	response := &Response{
		ID:           result.ID,
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
	}

	for _, tc := range choice.Message.ToolCalls {
		response.ToolCalls = append(response.ToolCalls, &ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	if result.Usage != nil {
		response.Usage = &Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		}
	}

	return response, nil
}

func (m *ZhipuModel) GenerateStream(ctx context.Context, messages []*Message, opts ...Option) (<-chan *Response, error) {
	ch := make(chan *Response, 1)
	go func() {
		defer close(ch)
		resp, err := m.Generate(ctx, messages, opts...)
		if err == nil {
			ch <- resp
		}
	}()
	return ch, nil
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

// 便捷函数
func NewSystemMessage(content string) *Message {
	return &Message{Role: "system", Content: content}
}

func NewUserMessage(content string) *Message {
	return &Message{Role: "user", Content: content}
}

func NewAssistantMessage(content string) *Message {
	return &Message{Role: "assistant", Content: content}
}