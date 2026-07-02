// Package workflow 智能工作流引擎 - NAS任务自动化
package workflow

import (
	"errors"
	"sync"
	"time"
)

// TriggerType 触发器类型.
type TriggerType string

const (
	TriggerCron    TriggerType = "cron"    // 定时触发
	TriggerFile    TriggerType = "file"    // 文件变化触发
	TriggerAPI     TriggerType = "api"     // API调用触发
	TriggerAlert   TriggerType = "alert"   // 告警触发
	TriggerEmail   TriggerType = "email"   // 邮件触发
	TriggerWebhook TriggerType = "webhook" // Webhook触发
	TriggerManual  TriggerType = "manual"  // 手动触发
)

// NodeType 节点类型.
type NodeType string

const (
	NodeFileOp    NodeType = "file_op"   // 文件操作
	NodeScript    NodeType = "script"    // 脚本执行
	NodeHTTP      NodeType = "http"      // HTTP调用
	NodeNotify    NodeType = "notify"    // 通知
	NodeAI        NodeType = "ai"        // AI推理
	NodeCondition NodeType = "condition" // 条件判断
	NodeParallel  NodeType = "parallel"  // 并行执行
	NodeLoop      NodeType = "loop"      // 循环
	NodeDelay     NodeType = "delay"     // 延时
	NodeTransform NodeType = "transform" // 数据转换
	NodeStart     NodeType = "start"     // 开始节点
	NodeEnd       NodeType = "end"       // 结束节点
)

// ExecutionStatus 执行状态.
type ExecutionStatus string

const (
	ExecPending   ExecutionStatus = "pending"
	ExecRunning   ExecutionStatus = "running"
	ExecSuccess   ExecutionStatus = "success"
	ExecFailed    ExecutionStatus = "failed"
	ExecCancelled ExecutionStatus = "cancelled"
	ExecSkipped   ExecutionStatus = "skipped"
)

// WorkflowStatus 工作流状态.
type WorkflowStatus string

const (
	WfActive   WorkflowStatus = "active"
	WfDisabled WorkflowStatus = "disabled"
	WfDraft    WorkflowStatus = "draft"
)

// Trigger 触发器定义.
type Trigger struct {
	Type    TriggerType       `json:"type"`
	Config  map[string]string `json:"config"`
	Enabled bool              `json:"enabled"`
}

// NodePosition 节点位置（用于可视化DAG）.
type NodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Edge DAG边.
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Label     string `json:"label,omitempty"`
	Condition string `json:"condition,omitempty"` // 条件边，条件节点使用
}

// Node 工作流节点.
type Node struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       NodeType          `json:"type"`
	Config     map[string]string `json:"config"`
	Position   NodePosition      `json:"position"`
	Timeout    int               `json:"timeout"`     // 超时秒数
	RetryCount int               `json:"retry_count"` // 重试次数
	RetryDelay int               `json:"retry_delay"` // 重试间隔秒数
}

// Version 工作流版本.
type Version struct {
	Version     int       `json:"version"`
	Description string    `json:"description"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
	Triggers    []Trigger `json:"triggers"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
}

// Workflow 工作流定义.
type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      WorkflowStatus    `json:"status"`
	Version     int               `json:"version"`
	Nodes       []Node            `json:"nodes"`
	Edges       []Edge            `json:"edges"`
	Triggers    []Trigger         `json:"triggers"`
	Tags        []string          `json:"tags,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// NodeExecution 节点执行记录.
type NodeExecution struct {
	NodeID    string          `json:"node_id"`
	Status    ExecutionStatus `json:"status"`
	Input     string          `json:"input,omitempty"`
	Output    string          `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   time.Time       `json:"ended_at"`
	Duration  int64           `json:"duration"` // 毫秒
	RetryNum  int             `json:"retry_num"`
}

// Execution 工作流执行记录.
type Execution struct {
	ID          string          `json:"id"`
	WorkflowID  string          `json:"workflow_id"`
	Version     int             `json:"version"`
	Status      ExecutionStatus `json:"status"`
	TriggerType TriggerType     `json:"trigger_type"`
	Input       string          `json:"input,omitempty"`
	Output      string          `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	Nodes       []NodeExecution `json:"nodes"`
	StartedAt   time.Time       `json:"started_at"`
	EndedAt     time.Time       `json:"ended_at"`
	Duration    int64           `json:"duration"` // 毫秒
}

// Template 工作流模板.
type Template struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Workflow    Workflow `json:"workflow"`
	Downloads   int      `json:"downloads"`
	Rating      float64  `json:"rating"`
	Author      string   `json:"author"`
}

// Stats 工作流统计.
type Stats struct {
	TotalWorkflows  int     `json:"total_workflows"`
	ActiveWorkflows int     `json:"active_workflows"`
	TotalExecutions int     `json:"total_executions"`
	SuccessRate     float64 `json:"success_rate"`
	AvgDuration     float64 `json:"avg_duration"` // 毫秒
	TotalTemplates  int     `json:"total_templates"`
}

// Manager 工作流管理器.
type Manager struct {
	mu         sync.RWMutex
	workflows  map[string]*Workflow
	executions map[string]*Execution
	versions   map[string][]*Version
	templates  map[string]*Template
	config     *Config
	dataFile   string
}

// Config 管理器配置.
type Config struct {
	MaxWorkflows  int `json:"max_workflows"`
	MaxExecutions int `json:"max_executions"`
	ExecRetention int `json:"exec_retention"` // 执行记录保留天数
	MaxVersions   int `json:"max_versions"`   // 每个工作流最大版本数
}

var (
	ErrWorkflowNotFound  = errors.New("workflow not found")
	ErrWorkflowExists    = errors.New("workflow already exists")
	ErrExecutionNotFound = errors.New("execution not found")
	ErrTemplateNotFound  = errors.New("template not found")
	ErrMaxWorkflows      = errors.New("max workflows reached")
	ErrInvalidDAG        = errors.New("invalid DAG: cycle detected")
	ErrNoStartNode       = errors.New("no start node found")
	ErrNoEndNode         = errors.New("no end node found")
	ErrInvalidNodeType   = errors.New("invalid node type")
	ErrInvalidTrigger    = errors.New("invalid trigger type")
	ErrWorkflowDisabled  = errors.New("workflow is disabled")
)
