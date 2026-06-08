// Package workflow_automation 提供工作流自动化引擎
// 对标群晖 DSM Agent，实现智能工作流自动化
package workflow_automation

import (
	"fmt"
	"time"
)

// ========== 工作流定义 ==========

// WorkflowStatus 工作流状态.
type WorkflowStatus string

const (
	StatusDraft    WorkflowStatus = "draft"
	StatusActive   WorkflowStatus = "active"
	StatusPaused   WorkflowStatus = "paused"
	StatusDisabled WorkflowStatus = "disabled"
	StatusArchived WorkflowStatus = "archived"
)

// NodeType 工作流节点类型.
type NodeType string

const (
	NodeTypeTrigger   NodeType = "trigger"
	NodeTypeAction    NodeType = "action"
	NodeTypeCondition NodeType = "condition"
	NodeTypeLoop      NodeType = "loop"
	NodeTypeStart     NodeType = "start"
	NodeTypeEnd       NodeType = "end"
)

// Workflow 完整工作流定义.
type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     int               `json:"version"`
	Status      WorkflowStatus    `json:"status"`
	Nodes       map[string]*Node  `json:"nodes"`
	Edges       []*Edge           `json:"edges"`
	Variables   map[string]string `json:"variables,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CreatedBy   string            `json:"created_by,omitempty"`
}

// Node 工作流节点.
type Node struct {
	ID          string            `json:"id"`
	Type        NodeType          `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	Position    *Position         `json:"position,omitempty"`
	ActionType  ActionType        `json:"action_type,omitempty"`
	TriggerType TriggerType       `json:"trigger_type,omitempty"`
	Condition   *ConditionExpr    `json:"condition,omitempty"`
	LoopConfig  *LoopConfig       `json:"loop_config,omitempty"`
	Enabled     bool              `json:"enabled"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
}

// Edge 工作流边（节点连线）.
type Edge struct {
	ID        string `json:"id"`
	From      string `json:"from"` // 源节点 ID
	To        string `json:"to"`   // 目标节点 ID
	Label     string `json:"label,omitempty"`
	Condition string `json:"condition,omitempty"` // 条件表达式（条件分支时使用）
}

// Position 节点在设计器中的位置.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// RetryPolicy 重试策略.
type RetryPolicy struct {
	MaxRetries  int           `json:"max_retries"`
	BackoffType string        `json:"backoff_type"` // "fixed" | "exponential"
	Interval    time.Duration `json:"interval"`
}

// LoopConfig 循环配置.
type LoopConfig struct {
	MaxIterations  int    `json:"max_iterations"`
	CollectionKey  string `json:"collection_key"`            // 遍历的变量名
	BreakCondition string `json:"break_condition,omitempty"` // 中断条件
}

// ========== 执行相关 ==========

// ExecutionStatus 执行状态.
type ExecutionStatus string

const (
	ExecPending   ExecutionStatus = "pending"
	ExecRunning   ExecutionStatus = "running"
	ExecSuccess   ExecutionStatus = "success"
	ExecFailed    ExecutionStatus = "failed"
	ExecCancelled ExecutionStatus = "cancelled"
	ExecSkipped   ExecutionStatus = "skipped"
	ExecTimeout   ExecutionStatus = "timeout"
)

// Execution 工作流执行实例.
type Execution struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Version     int                    `json:"version"`
	Status      ExecutionStatus        `json:"status"`
	TriggerID   string                 `json:"trigger_id,omitempty"`
	TriggeredBy string                 `json:"triggered_by,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Steps       []*StepExecution       `json:"steps"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	FinishedAt  *time.Time             `json:"finished_at,omitempty"`
	Duration    time.Duration          `json:"duration"`
}

// StepExecution 单步执行记录.
type StepExecution struct {
	NodeID     string                 `json:"node_id"`
	Status     ExecutionStatus        `json:"status"`
	Input      map[string]interface{} `json:"input,omitempty"`
	Output     map[string]interface{} `json:"output,omitempty"`
	Error      string                 `json:"error,omitempty"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt *time.Time             `json:"finished_at,omitempty"`
	Duration   time.Duration          `json:"duration"`
	RetryCount int                    `json:"retry_count"`
}

// ========== 触发器相关 ==========

// TriggerType 触发器类型.
type TriggerType string

const (
	TriggerCron    TriggerType = "cron"    // 定时触发
	TriggerOnEvent TriggerType = "event"   // 事件触发
	TriggerWebhook TriggerType = "webhook" // Webhook 触发
	TriggerFile    TriggerType = "file"    // 文件变化触发
	TriggerManual  TriggerType = "manual"  // 手动触发
)

// Trigger 触发器定义.
type Trigger struct {
	ID          string            `json:"id"`
	WorkflowID  string            `json:"workflow_id"`
	Type        TriggerType       `json:"type"`
	Name        string            `json:"name"`
	Config      map[string]string `json:"config"`
	Enabled     bool              `json:"enabled"`
	LastFiredAt *time.Time        `json:"last_fired_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// TriggerEvent 触发事件.
type TriggerEvent struct {
	TriggerID  string                 `json:"trigger_id"`
	WorkflowID string                 `json:"workflow_id"`
	Type       TriggerType            `json:"type"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// ========== 动作相关 ==========

// ActionType 动作类型.
type ActionType string

const (
	ActionFileOps      ActionType = "file_ops"     // 文件操作
	ActionNotification ActionType = "notification" // 通知
	ActionAPICall      ActionType = "api_call"     // API 调用
	ActionContainer    ActionType = "container"    // 容器操作
	ActionShell        ActionType = "shell"        // Shell 命令
	ActionTransform    ActionType = "transform"    // 数据转换
)

// ActionHandler 动作处理器接口.
type ActionHandler interface {
	// Type 返回动作类型.
	Type() ActionType
	// Name 返回动作名称.
	Name() string
	// Description 返回动作描述.
	Description() string
	// Execute 执行动作.
	Execute(ctx *ActionContext) (*ActionResult, error)
	// Validate 验证动作配置.
	Validate(config map[string]string) error
}

// ActionContext 动作执行上下文.
type ActionContext struct {
	Config    map[string]string      `json:"config"`
	Input     map[string]interface{} `json:"input"`
	Variables map[string]interface{} `json:"variables"`
	Logger    ExecutionLogger        `json:"-"`
	Timeout   time.Duration          `json:"timeout"`
}

// ActionResult 动作执行结果.
type ActionResult struct {
	Success bool                   `json:"success"`
	Output  map[string]interface{} `json:"output,omitempty"`
	Message string                 `json:"message,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// ========== 条件相关 ==========

// ConditionOp 条件操作符.
type ConditionOp string

const (
	OpEquals       ConditionOp = "eq"
	OpNotEquals    ConditionOp = "ne"
	OpGreaterThan  ConditionOp = "gt"
	OpLessThan     ConditionOp = "lt"
	OpGreaterEqual ConditionOp = "gte"
	OpLessEqual    ConditionOp = "lte"
	OpContains     ConditionOp = "contains"
	OpStartsWith   ConditionOp = "starts_with"
	OpEndsWith     ConditionOp = "ends_with"
	OpMatches      ConditionOp = "matches" // 正则
	OpIn           ConditionOp = "in"
	OpExists       ConditionOp = "exists"
)

// ConditionLogic 条件逻辑.
type ConditionLogic string

const (
	LogicAnd ConditionLogic = "and"
	LogicOr  ConditionLogic = "or"
	LogicNot ConditionLogic = "not"
)

// ConditionExpr 条件表达式.
type ConditionExpr struct {
	Logic    ConditionLogic   `json:"logic,omitempty"`
	Op       ConditionOp      `json:"op,omitempty"`
	Field    string           `json:"field,omitempty"`
	Value    interface{}      `json:"value,omitempty"`
	Children []*ConditionExpr `json:"children,omitempty"`
}

// ConditionResult 条件评估结果.
type ConditionResult struct {
	Matched bool                   `json:"matched"`
	Detail  string                 `json:"detail,omitempty"`
	Values  map[string]interface{} `json:"values,omitempty"`
}

// ========== 版本管理 ==========

// WorkflowVersion 工作流版本快照.
type WorkflowVersion struct {
	WorkflowID string    `json:"workflow_id"`
	Version    int       `json:"version"`
	Snapshot   *Workflow `json:"snapshot"` // 完整工作流快照
	Comment    string    `json:"comment,omitempty"`
	CreatedBy  string    `json:"created_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ========== 日志相关 ==========

// ExecutionLogger 执行日志接口.
type ExecutionLogger interface {
	Log(level LogLevel, executionID, nodeID, message string, fields map[string]interface{})
	LogStart(executionID, workflowID string)
	LogEnd(executionID string, status ExecutionStatus, err error)
	LogStep(executionID, nodeID string, status ExecutionStatus, input, output map[string]interface{}, err error)
	GetLogs(executionID string, limit int) ([]*LogEntry, error)
}

// LogLevel 日志级别.
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// LogEntry 日志条目.
type LogEntry struct {
	ID          string                 `json:"id"`
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id,omitempty"`
	Level       LogLevel               `json:"level"`
	Message     string                 `json:"message"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ========== 存储接口 ==========

// Store 工作流存储接口.
type Store interface {
	// 工作流 CRUD
	SaveWorkflow(wf *Workflow) error
	GetWorkflow(id string) (*Workflow, error)
	ListWorkflows() ([]*Workflow, error)
	DeleteWorkflow(id string) error

	// 版本管理
	SaveVersion(v *WorkflowVersion) error
	GetVersions(workflowID string) ([]*WorkflowVersion, error)
	GetVersion(workflowID string, version int) (*WorkflowVersion, error)

	// 执行记录
	SaveExecution(exec *Execution) error
	GetExecution(id string) (*Execution, error)
	ListExecutions(workflowID string, limit int) ([]*Execution, error)

	// 触发器
	SaveTrigger(t *Trigger) error
	GetTrigger(id string) (*Trigger, error)
	ListTriggers(workflowID string) ([]*Trigger, error)
	DeleteTrigger(id string) error

	// 日志
	SaveLog(entry *LogEntry) error
	GetLogs(executionID string, limit int) ([]*LogEntry, error)
}

// ========== 错误定义 ==========

// ErrWorkflowNotFound 工作流未找到.
var ErrWorkflowNotFound = fmt.Errorf("workflow not found")

// ErrTriggerNotFound 触发器未找到.
var ErrTriggerNotFound = fmt.Errorf("trigger not found")

// ErrActionNotFound 动作处理器未找到.
var ErrActionNotFound = fmt.Errorf("action handler not found")

// ErrInvalidConfig 配置无效.
var ErrInvalidConfig = fmt.Errorf("invalid configuration")

// ErrExecutionTimeout 执行超时.
var ErrExecutionTimeout = fmt.Errorf("execution timeout")

// ErrMaxRetriesExceeded 超过最大重试次数.
var ErrMaxRetriesExceeded = fmt.Errorf("max retries exceeded")
