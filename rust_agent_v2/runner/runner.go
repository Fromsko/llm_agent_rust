package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"rust_agent_v2/event"
	"rust_agent_v2/graph"
	"rust_agent_v2/model"
)

// Runner 运行时管理器
type Runner struct {
	graph          *graph.Graph
	sessionService *SessionService
	config         *Config
}

type Config struct {
	AppName            string
	AutoSummarize      bool
	SummarizeThreshold int
	MaxMessages        int
}

type Option func(*Runner)

func WithConfig(cfg *Config) Option {
	return func(r *Runner) { r.config = cfg }
}

func WithSessionService(ss *SessionService) Option {
	return func(r *Runner) { r.sessionService = ss }
}

func New(g *graph.Graph, opts ...Option) *Runner {
	r := &Runner{
		graph: g,
		config: &Config{
			AppName:            "rust-agent",
			AutoSummarize:      true,
			SummarizeThreshold: 20,
			MaxMessages:        100,
		},
		sessionService: NewSessionService(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Request 运行请求
type Request struct {
	UserID       string
	SessionID    string
	InvocationID string
	Input        string
}

// Run 运行
func (r *Runner) Run(ctx context.Context, req *Request) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, 100)

	go func() {
		defer close(eventChan)

		// 获取或创建会话
		sess := r.sessionService.GetOrCreate(req.UserID, req.SessionID)

		// 加载历史消息
		messages := sess.GetMessages()

		// 检查是否需要摘要
		if r.config.AutoSummarize && len(messages) > r.config.SummarizeThreshold {
			messages = messages[len(messages)-10:] // 简单截断
		}

		// 构建初始状态
		initialState := graph.State{
			graph.StateKeyUserInput: req.Input,
			graph.StateKeyMessages:  messages,
		}

		// 执行图
		executor := graph.NewExecutor(r.graph)
		graphChan, err := executor.Execute(ctx, initialState)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent("runner", "EXEC_ERROR", err.Error()))
			return
		}

		// 处理事件
		for ev := range graphChan {
			// 保存到会话
			sess.AppendEvent(ev)

			// 转发事件
			event.EmitEvent(ctx, eventChan, ev)
		}
	}()

	return eventChan, nil
}

// Session 会话
type Session struct {
	ID        string
	UserID    string
	Messages  []*model.Message
	Events    []*event.Event
	CreatedAt time.Time
	UpdatedAt time.Time
	mu        sync.RWMutex
}

func NewSession(userID, sessionID string) *Session {
	return &Session{
		ID:        sessionID,
		UserID:    userID,
		Messages:  make([]*model.Message, 0),
		Events:    make([]*event.Event, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *Session) GetMessages() []*model.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]*model.Message{}, s.Messages...)
}

func (s *Session) AppendMessage(msg *model.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

func (s *Session) AppendEvent(ev *event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Events = append(s.Events, ev)
	s.UpdatedAt = time.Now()

	// 如果是响应事件，也添加到消息
	if ev.Type == event.TypeResponse && ev.Response != nil {
		s.Messages = append(s.Messages, &model.Message{
			Role:    "assistant",
			Content: ev.Response.Content,
		})
	}
}

// SessionService 会话服务
type SessionService struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionService() *SessionService {
	return &SessionService{sessions: make(map[string]*Session)}
}

func (ss *SessionService) GetOrCreate(userID, sessionID string) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, sessionID)
	if sess, ok := ss.sessions[key]; ok {
		return sess
	}

	sess := NewSession(userID, sessionID)
	ss.sessions[key] = sess
	return sess
}

func (ss *SessionService) Get(userID, sessionID string) (*Session, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", userID, sessionID)
	sess, ok := ss.sessions[key]
	return sess, ok
}

func (ss *SessionService) Delete(userID, sessionID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, sessionID)
	delete(ss.sessions, key)
}
