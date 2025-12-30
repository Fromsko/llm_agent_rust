package graph

import (
	"context"
	"sync"

	"rust_agent_v2/agent"
	"rust_agent_v2/event"
	"rust_agent_v2/model"
)

// State 图状态
type State map[string]any

const (
	StateKeyMessages    = "messages"
	StateKeyUserInput   = "user_input"
	StateKeyAgentOutput = "agent_output"
	StateKeyCurrentNode = "current_node"
	StateKeyError       = "error"
	StateKeySuccess     = "success"
)

func (s State) Clone() State {
	clone := make(State)
	for k, v := range s {
		clone[k] = v
	}
	return clone
}

func (s State) GetMessages() []*model.Message {
	if msgs, ok := s[StateKeyMessages].([]*model.Message); ok {
		return msgs
	}
	return nil
}

func (s State) SetMessages(msgs []*model.Message) {
	s[StateKeyMessages] = msgs
}

// ConditionFunc 条件函数
type ConditionFunc func(ctx context.Context, state State) bool

// NodeType 节点类型
type NodeType int

const (
	NodeTypeHandler NodeType = iota
	NodeTypeAgent
	NodeTypeSubGraph
)

// NodeHandler 节点处理函数
type NodeHandler func(ctx context.Context, state State) (State, error)

// Node 图节点
type Node struct {
	name     string
	nodeType NodeType
	handler  NodeHandler
	agent    agent.Agent
	subGraph *Graph
}

// Edge 图边
type Edge struct {
	From      string
	To        string
	Condition ConditionFunc
}

// Graph 图结构
type Graph struct {
	name       string
	nodes      map[string]*Node
	edges      map[string][]Edge
	entryPoint string
	endNodes   map[string]bool
}

// Builder 图构建器
type Builder struct {
	graph *Graph
}

func NewBuilder(name string) *Builder {
	return &Builder{
		graph: &Graph{
			name:     name,
			nodes:    make(map[string]*Node),
			edges:    make(map[string][]Edge),
			endNodes: make(map[string]bool),
		},
	}
}

func (b *Builder) AddNode(name string, handler NodeHandler) *Builder {
	b.graph.nodes[name] = &Node{name: name, nodeType: NodeTypeHandler, handler: handler}
	return b
}

func (b *Builder) AddAgentNode(name string, ag agent.Agent) *Builder {
	b.graph.nodes[name] = &Node{name: name, nodeType: NodeTypeAgent, agent: ag}
	return b
}

func (b *Builder) AddSubGraph(name string, subGraph *Graph) *Builder {
	b.graph.nodes[name] = &Node{name: name, nodeType: NodeTypeSubGraph, subGraph: subGraph}
	return b
}

func (b *Builder) AddEdge(from, to string) *Builder {
	b.graph.edges[from] = append(b.graph.edges[from], Edge{From: from, To: to})
	return b
}

func (b *Builder) AddConditionalEdge(from, to string, condition ConditionFunc) *Builder {
	b.graph.edges[from] = append(b.graph.edges[from], Edge{From: from, To: to, Condition: condition})
	return b
}

func (b *Builder) SetEntryPoint(name string) *Builder {
	b.graph.entryPoint = name
	return b
}

func (b *Builder) SetEndNode(name string) *Builder {
	b.graph.endNodes[name] = true
	return b
}

func (b *Builder) Build() *Graph {
	return b.graph
}

// Executor 图执行器
type Executor struct {
	graph           *Graph
	checkpointer    Checkpointer
	maxIterations   int
	eventBufferSize int
}

type ExecutorOption func(*Executor)

func WithCheckpointer(cp Checkpointer) ExecutorOption {
	return func(e *Executor) { e.checkpointer = cp }
}

func WithMaxIterations(n int) ExecutorOption {
	return func(e *Executor) { e.maxIterations = n }
}

func NewExecutor(graph *Graph, opts ...ExecutorOption) *Executor {
	e := &Executor{
		graph:           graph,
		maxIterations:   100,
		eventBufferSize: 100,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Executor) Execute(ctx context.Context, initialState State) (<-chan *event.Event, error) {
	eventChan := make(chan *event.Event, e.eventBufferSize)

	go func() {
		defer close(eventChan)
		e.executeGraph(ctx, e.graph, initialState, eventChan)
	}()

	return eventChan, nil
}

func (e *Executor) executeGraph(ctx context.Context, g *Graph, state State, eventChan chan<- *event.Event) {
	currentNode := g.entryPoint
	iterations := 0

	for iterations < e.maxIterations {
		iterations++

		select {
		case <-ctx.Done():
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(g.name, "CANCELLED", ctx.Err().Error()))
			return
		default:
		}

		if g.endNodes[currentNode] {
			break
		}

		node, ok := g.nodes[currentNode]
		if !ok {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(g.name, "NODE_NOT_FOUND", currentNode))
			return
		}

		state[StateKeyCurrentNode] = currentNode
		event.EmitEvent(ctx, eventChan, &event.Event{Type: event.TypeState, AgentName: g.name, NodeName: currentNode, State: state.Clone()})

		var err error
		state, err = e.executeNode(ctx, node, state, eventChan)
		if err != nil {
			event.EmitEvent(ctx, eventChan, event.NewErrorEvent(g.name, "NODE_ERROR", err.Error()))
			return
		}

		if e.checkpointer != nil {
			e.checkpointer.Save(ctx, g.name, state)
		}

		nextNode := e.getNextNode(ctx, g, currentNode, state)
		if nextNode == "" {
			break
		}
		currentNode = nextNode
	}

	event.EmitEvent(ctx, eventChan, event.NewCompletionEvent(g.name, state))
}

func (e *Executor) executeNode(ctx context.Context, node *Node, state State, eventChan chan<- *event.Event) (State, error) {
	switch node.nodeType {
	case NodeTypeHandler:
		return node.handler(ctx, state)
	case NodeTypeAgent:
		return e.executeAgentNode(ctx, node, state, eventChan)
	case NodeTypeSubGraph:
		return e.executeSubGraph(ctx, node.subGraph, state, eventChan)
	}
	return state, nil
}

func (e *Executor) executeAgentNode(ctx context.Context, node *Node, state State, eventChan chan<- *event.Event) (State, error) {
	input, _ := state[StateKeyUserInput].(string)

	agentChan, err := node.agent.Run(ctx, input, agent.WithMessages(state.GetMessages()))
	if err != nil {
		return state, err
	}

	var lastContent string
	for ev := range agentChan {
		ev.NodeName = node.name
		event.EmitEvent(ctx, eventChan, ev)

		if ev.Type == event.TypeResponse && ev.Response != nil {
			lastContent = ev.Response.Content
		}
	}

	state[StateKeyAgentOutput] = lastContent
	return state, nil
}

func (e *Executor) executeSubGraph(ctx context.Context, subGraph *Graph, state State, eventChan chan<- *event.Event) (State, error) {
	subExecutor := NewExecutor(subGraph, WithMaxIterations(e.maxIterations))
	subChan, _ := subExecutor.Execute(ctx, state)

	for ev := range subChan {
		event.EmitEvent(ctx, eventChan, ev)
		if ev.Type == event.TypeCompletion && ev.Completion != nil {
			if result, ok := ev.Completion.Result.(State); ok {
				return result, nil
			}
		}
	}
	return state, nil
}

func (e *Executor) getNextNode(ctx context.Context, g *Graph, current string, state State) string {
	edges := g.edges[current]
	for _, edge := range edges {
		if edge.Condition == nil || edge.Condition(ctx, state) {
			return edge.To
		}
	}
	return ""
}

// Checkpointer 检查点接口
type Checkpointer interface {
	Save(ctx context.Context, graphID string, state State) error
	Load(ctx context.Context, graphID string) (State, error)
	Delete(ctx context.Context, graphID string) error
}

// InMemoryCheckpointer 内存检查点
type InMemoryCheckpointer struct {
	mu     sync.RWMutex
	states map[string]State
}

func NewInMemoryCheckpointer() *InMemoryCheckpointer {
	return &InMemoryCheckpointer{states: make(map[string]State)}
}

func (c *InMemoryCheckpointer) Save(ctx context.Context, graphID string, state State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[graphID] = state.Clone()
	return nil
}

func (c *InMemoryCheckpointer) Load(ctx context.Context, graphID string) (State, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if state, ok := c.states[graphID]; ok {
		return state.Clone(), nil
	}
	return nil, nil
}

func (c *InMemoryCheckpointer) Delete(ctx context.Context, graphID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.states, graphID)
	return nil
}
