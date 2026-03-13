package event

import (
	"context"
	"time"
)

// Type 事件类型
type Type string

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
	TypeAskUser    Type = "ask_user"
	TypeUserInput  Type = "user_input"
)

// Event 事件
type Event struct {
	ID        string
	Type      Type
	Timestamp time.Time
	AgentName string
	NodeName  string

	Response   *Response
	ToolCall   *ToolCall
	ToolResult *ToolResult
	MCPCall    *MCPCall
	MCPResult  *MCPResult
	Error      *Error
	Completion *Completion
	Progress   *Progress
	State      map[string]any
	AskUser    *AskUser
	UserInput  *UserInput
}

type Response struct {
	Content      string
	ToolCalls    []*ToolCall
	FinishReason string
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type ToolResult struct {
	ToolCallID string
	Content    string
	Error      string
}

type MCPCall struct {
	Server string
	Method string
	Params map[string]any
}

type MCPResult struct {
	Content string
	Error   string
}

type Error struct {
	Code    string
	Message string
}

type Completion struct {
	Result  any
	Summary string
}

type Progress struct {
	Current int
	Total   int
	Message string
}

type AskUser struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

type UserInput struct {
	Response string `json:"response"`
}

// EmitEvent 发送事件
func EmitEvent(ctx context.Context, ch chan<- *Event, event *Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	select {
	case ch <- event:
	case <-ctx.Done():
	}
}

// 便捷构造函数
func NewResponseEvent(agent, content string) *Event {
	return &Event{Type: TypeResponse, AgentName: agent, Response: &Response{Content: content}}
}

func NewErrorEvent(agent, code, msg string) *Event {
	return &Event{Type: TypeError, AgentName: agent, Error: &Error{Code: code, Message: msg}}
}

func NewProgressEvent(agent string, current, total int, msg string) *Event {
	return &Event{Type: TypeProgress, AgentName: agent, Progress: &Progress{Current: current, Total: total, Message: msg}}
}

func NewCompletionEvent(agent string, result any) *Event {
	return &Event{Type: TypeCompletion, AgentName: agent, Completion: &Completion{Result: result}}
}

func NewToolCallEvent(agent, toolName string, args map[string]any) *Event {
	return &Event{Type: TypeToolCall, AgentName: agent, ToolCall: &ToolCall{Name: toolName, Arguments: args}}
}

func NewMCPCallEvent(agent, server, method string, params map[string]any) *Event {
	return &Event{Type: TypeMCPCall, AgentName: agent, MCPCall: &MCPCall{Server: server, Method: method, Params: params}}
}

func NewAskUserEvent(agent, question string, options []string) *Event {
	return &Event{Type: TypeAskUser, AgentName: agent, AskUser: &AskUser{Question: question, Options: options}}
}

func NewUserInputEvent(agent, response string) *Event {
	return &Event{Type: TypeUserInput, AgentName: agent, UserInput: &UserInput{Response: response}}
}
