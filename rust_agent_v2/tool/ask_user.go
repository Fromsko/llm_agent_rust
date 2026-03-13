package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// UserInputHandler 用户输入处理器类型
type UserInputHandler func(question string, options []string) (string, error)

// AskUserTool 询问用户工具
type AskUserTool struct {
	mu      sync.RWMutex
	handler UserInputHandler
}

// AskUserResult 询问用户的结果
type AskUserResult struct {
	Success  bool     `json:"success"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	Response string   `json:"response"`
	Error    string   `json:"error,omitempty"`
}

func NewAskUserTool() *AskUserTool {
	return &AskUserTool{}
}

// SetHandler 设置用户输入处理器（由 Runner 或 main 设置）
func (t *AskUserTool) SetHandler(handler UserInputHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
}

func (t *AskUserTool) Name() string        { return "ask_user" }
func (t *AskUserTool) Description() string { return "向用户询问问题并等待回答。支持开放式问题和多选项选择。" }

func (t *AskUserTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "要问用户的问题",
			},
			"options": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "可选的选项列表。如果提供，用户需要选择其中一个（输入数字1-N或选项文本）",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "问题的上下文信息，帮助用户理解为什么问这个问题",
			},
			"allow_free_text": map[string]any{
				"type":        "boolean",
				"description": "当提供选项时，是否允许用户输入自定义答案（默认false）",
			},
			"default": map[string]any{
				"type":        "string",
				"description": "默认值，如果用户直接回车则使用此值",
			},
		},
		"required": []string{"question"},
	}
}

// Run 执行询问用户
func (t *AskUserTool) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Question      string   `json:"question"`
		Options       []string `json:"options,omitempty"`
		Context       string   `json:"context,omitempty"`
		AllowFreeText bool     `json:"allow_free_text,omitempty"`
		Default       string   `json:"default,omitempty"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return FormatOutput(AskUserResult{
			Success: false,
			Error:   fmt.Sprintf("解析参数失败: %v", err),
		}), nil
	}

	// 构建完整问题
	fullQuestion := args.Question
	if args.Context != "" {
		fullQuestion = fmt.Sprintf("【%s】\n%s", args.Context, args.Question)
	}

	// 检查是否有处理器
	t.mu.RLock()
	handler := t.handler
	t.mu.RUnlock()

	if handler == nil {
		// 使用默认的交互式输入
		response, err := t.defaultAsk(fullQuestion, args.Options, args.AllowFreeText, args.Default)
		if err != nil {
			return FormatOutput(AskUserResult{
				Success:  false,
				Question: fullQuestion,
				Options:  args.Options,
				Error:    err.Error(),
			}), nil
		}
		return FormatOutput(AskUserResult{
			Success:  true,
			Question: fullQuestion,
			Options:  args.Options,
			Response: response,
		}), nil
	}

	// 调用自定义处理器
	response, err := handler(fullQuestion, args.Options)
	if err != nil {
		return FormatOutput(AskUserResult{
			Success:  false,
			Question: fullQuestion,
			Options:  args.Options,
			Error:    fmt.Sprintf("获取用户输入失败: %v", err),
		}), nil
	}

	return FormatOutput(AskUserResult{
		Success:  true,
		Question: fullQuestion,
		Options:  args.Options,
		Response: response,
	}), nil
}

// defaultAsk 默认的交互式询问实现
func (t *AskUserTool) defaultAsk(question string, options []string, allowFreeText bool, defaultValue string) (string, error) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🤔 " + question)
	fmt.Println(strings.Repeat("=", 60))

	if len(options) > 0 {
		fmt.Println("\n选项:")
		for i, opt := range options {
			fmt.Printf("  [%d] %s\n", i+1, opt)
		}
		if allowFreeText {
			fmt.Println("  [0] 其他（自定义输入）")
		}
	}

	if defaultValue != "" {
		fmt.Printf("\n[默认: %s] ", defaultValue)
	} else {
		fmt.Print("\n请输入: ")
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	input = strings.TrimSpace(input)

	// 使用默认值
	if input == "" && defaultValue != "" {
		return defaultValue, nil
	}

	// 处理选项选择
	if len(options) > 0 {
		// 尝试解析为数字
		if num, err := strconv.Atoi(input); err == nil {
			if num >= 1 && num <= len(options) {
				return options[num-1], nil
			}
			if num == 0 && allowFreeText {
				fmt.Print("请输入自定义答案: ")
				custom, _ := reader.ReadString('\n')
				return strings.TrimSpace(custom), nil
			}
		}

		// 检查是否直接输入了选项文本
		for _, opt := range options {
			if strings.EqualFold(input, opt) {
				return opt, nil
			}
		}

		// 如果不允许自由文本，提示重新输入
		if !allowFreeText {
			return "", fmt.Errorf("无效选择，请输入 1-%d 的数字", len(options))
		}
	}

	return input, nil
}

// AskUserSimple 简单询问（无选项）
func AskUserSimple(question string) map[string]any {
	return map[string]any{
		"question": question,
	}
}

// AskUserWithOptions 带选项的询问
func AskUserWithOptions(question string, options []string) map[string]any {
	return map[string]any{
		"question": question,
		"options":  options,
	}
}

// AskUserWithContext 带上下文的询问
func AskUserWithContext(question, context string) map[string]any {
	return map[string]any{
		"question": question,
		"context":  context,
	}
}

// AskUserMultiChoice 多选项询问（不允许自由文本）
func AskUserMultiChoice(question string, options []string) map[string]any {
	return map[string]any{
		"question":        question,
		"options":         options,
		"allow_free_text": false,
	}
}

// AskUserWithDefault 带默认值的询问
func AskUserWithDefault(question, defaultValue string) map[string]any {
	return map[string]any{
		"question": question,
		"default":  defaultValue,
	}
}

// InteractiveAskUser 交互式询问用户（用于代码中直接调用）
// 返回用户的选择和可能的错误
func InteractiveAskUser(question string, options []string, allowFreeText bool) (string, error) {
	tool := NewAskUserTool()
	args := map[string]any{
		"question":        question,
		"options":         options,
		"allow_free_text": allowFreeText,
	}

	input, _ := json.Marshal(args)
	result, err := tool.Run(context.Background(), string(input))
	if err != nil {
		return "", err
	}

	var askResult AskUserResult
	if err := json.Unmarshal([]byte(result), &askResult); err != nil {
		return "", err
	}

	if !askResult.Success {
		return "", fmt.Errorf(askResult.Error)
	}

	return askResult.Response, nil
}

// ConfirmYesNo 确认是/否
func ConfirmYesNo(question string, defaultYes bool) (bool, error) {
	options := []string{"y", "n"}
	defaultVal := "n"
	if defaultYes {
		defaultVal = "y"
	}

	tool := NewAskUserTool()
	args := map[string]any{
		"question": question + " (y/n)",
		"options":  options,
		"default":  defaultVal,
	}
	input, _ := json.Marshal(args)
	result, err := tool.Run(context.Background(), string(input))
	if err != nil {
		return defaultYes, err
	}

	var askResult AskUserResult
	if err := json.Unmarshal([]byte(result), &askResult); err != nil {
		return defaultYes, err
	}

	return strings.EqualFold(askResult.Response, "y") || strings.EqualFold(askResult.Response, "yes"), nil
}

// SelectFromList 从列表中选择一项
func SelectFromList(question string, options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("选项列表为空")
	}
	return InteractiveAskUser(question, options, false)
}

// AskWithFreeText 允许自由文本的询问
func AskWithFreeText(question string) (string, error) {
	tool := NewAskUserTool()
	args := AskUserSimple(question)
	input, _ := json.Marshal(args)
	result, err := tool.Run(context.Background(), string(input))
	if err != nil {
		return "", err
	}

	var askResult AskUserResult
	if err := json.Unmarshal([]byte(result), &askResult); err != nil {
		return "", err
	}

	return askResult.Response, nil
}
