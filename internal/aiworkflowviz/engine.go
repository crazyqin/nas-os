package aiworkflowviz

import (
	"sync"
	"time"
)

// NodeType 节点类型。
type NodeType string

const (
	NodeTypeInput    NodeType = "input"
	NodeTypeOutput   NodeType = "output"
	NodeTypeProcess  NodeType = "process"
	NodeTypeAI       NodeType = "ai_model"
	NodeTypeFilter   NodeType = "filter"
	NodeTypeMerge    NodeType = "merge"
	NodeTypeSplit    NodeType = "split"
)

// Node 工作流节点。
type Node struct {
	ID        string            `json:"id"`
	Type      NodeType          `json:"type"`
	Name      string            `json:"name"`
	Config    map[string]interface{} `json:"config"`
	Position  Position          `json:"position"`
	Status    NodeStatus        `json:"status"`
}

// Position 节点位置。
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeStatus 节点状态。
type NodeStatus string

const (
	StatusIdle     NodeStatus = "idle"
	StatusRunning  NodeStatus = "running"
	StatusSuccess  NodeStatus = "success"
	StatusError    NodeStatus = "error"
)

// Edge 连接边。
type Edge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Label    string `json:"label,omitempty"`
}

// Workflow 可视化工作流。
type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Nodes       map[string]*Node  `json:"nodes"`
	Edges       map[string]*Edge  `json:"edges"`
	Status      WorkflowStatus    `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// WorkflowStatus 工作流状态。
type WorkflowStatus string

const (
	WfStatusDraft    WorkflowStatus = "draft"
	WfStatusRunning  WorkflowStatus = "running"
	WfStatusPaused   WorkflowStatus = "paused"
	WfStatusComplete WorkflowStatus = "complete"
	WfStatusError    WorkflowStatus = "error"
)

// Engine 工作流可视化引擎。
type Engine struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow
}

// NewEngine 创建新的引擎。
func NewEngine() *Engine {
	return &Engine{
		workflows: make(map[string]*Workflow),
	}
}

// CreateWorkflow 创建工作流。
func (e *Engine) CreateWorkflow(name, desc string) *Workflow {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf := &Workflow{
		ID:          generateID(),
		Name:        name,
		Description: desc,
		Nodes:       make(map[string]*Node),
		Edges:       make(map[string]*Edge),
		Status:      WfStatusDraft,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.workflows[wf.ID] = wf
	return wf
}

// GetWorkflow 获取工作流。
func (e *Engine) GetWorkflow(id string) (*Workflow, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	wf, exists := e.workflows[id]
	return wf, exists
}

// ListWorkflows 列出所有工作流。
func (e *Engine) ListWorkflows() []*Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Workflow, 0, len(e.workflows))
	for _, wf := range e.workflows {
		result = append(result, wf)
	}
	return result
}

// AddNode 添加节点。
func (e *Engine) AddNode(wfID string, node *Node) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[wfID]
	if !exists {
		return ErrWorkflowNotFound
	}
	wf.Nodes[node.ID] = node
	wf.UpdatedAt = time.Now()
	return nil
}

// AddEdge 添加连接。
func (e *Engine) AddEdge(wfID string, edge *Edge) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wf, exists := e.workflows[wfID]
	if !exists {
		return ErrWorkflowNotFound
	}
	wf.Edges[edge.ID] = edge
	wf.UpdatedAt = time.Now()
	return nil
}

// DeleteWorkflow 删除工作流。
func (e *Engine) DeleteWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.workflows[id]; !exists {
		return ErrWorkflowNotFound
	}
	delete(e.workflows, id)
	return nil
}

func generateID() string {
	return time.Now().Format("20060102150405") + randomHex(4)
}

func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
		time.Sleep(1)
	}
	return string(b)
}

// 错误定义。
var (
	ErrWorkflowNotFound = &WorkflowError{"workflow not found"}
)

type WorkflowError struct {
	msg string
}

func (e *WorkflowError) Error() string {
	return e.msg
}
