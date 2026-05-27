// Package workflowengine 提供工作流引擎功能
// 支持 DAG 工作流编排、条件触发、事件驱动自动化
package workflowengine

import (
	"time"
)

// WorkflowStatus 工作流状态
type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusDisabled WorkflowStatus = "disabled"
	WorkflowStatusArchived WorkflowStatus = "archived"
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusSuccess   ExecutionStatus = "success"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusSkipped   ExecutionStatus = "skipped"
)

// NodeStatus 节点执行状态
type NodeStatus string

const (
	NodeStatusPending   NodeStatus = "pending"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusSuccess   NodeStatus = "success"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
	NodeStatusCancelled NodeStatus = "cancelled"
)

// TriggerType 触发器类型
type TriggerType string

const (
	TriggerTypeManual    TriggerType = "manual"
	TriggerTypeEvent     TriggerType = "event"
	TriggerTypeSchedule  TriggerType = "schedule"
	TriggerTypeWebhook   TriggerType = "webhook"
	TriggerTypeThreshold TriggerType = "threshold"
)

// ConditionOperator 条件操作符
type ConditionOperator string

const (
	ConditionOpEquals    ConditionOperator = "equals"
	ConditionOpNotEquals ConditionOperator = "not_equals"
	ConditionOpGreater   ConditionOperator = "greater_than"
	ConditionOpLess      ConditionOperator = "less_than"
	ConditionOpContains  ConditionOperator = "contains"
	ConditionOpIn        ConditionOperator = "in"
	ConditionOpRegex     ConditionOperator = "regex"
)

// Workflow 工作流定义
type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      WorkflowStatus `json:"status"`
	Version     int            `json:"version"`

	// DAG 节点
	Nodes []WorkflowNode `json:"nodes"`

	// 触发器
	Triggers []Trigger `json:"triggers,omitempty"`

	// 全局变量
	Variables map[string]interface{} `json:"variables,omitempty"`

	// 标签
	Tags []string `json:"tags,omitempty"`

	// 元数据
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	CreatedBy string     `json:"createdBy,omitempty"`
	UpdatedBy string     `json:"updatedBy,omitempty"`

	// 统计
	ExecutionCount int        `json:"executionCount"`
	LastExecutedAt *time.Time `json:"lastExecutedAt,omitempty"`
	SuccessRate    float64    `json:"successRate"`
}

// WorkflowNode 工作流节点
type WorkflowNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"` // task, condition, parallel, subworkflow

	// 依赖关系（上游节点 ID 列表）
	Dependencies []string `json:"dependencies,omitempty"`

	// 任务配置
	TaskType   string                 `json:"taskType,omitempty"`   // shell, http, script, builtin
	TaskConfig map[string]interface{} `json:"taskConfig,omitempty"` // 任务参数

	// 条件配置
	Condition *Condition `json:"condition,omitempty"`

	// 重试配置
	MaxRetries    int    `json:"maxRetries,omitempty"`
	RetryInterval string `json:"retryInterval,omitempty"` // 例如 "30s", "1m"

	// 超时配置
	Timeout string `json:"timeout,omitempty"` // 例如 "5m", "1h"

	// 位置信息（用于可视化）
	Position *NodePosition `json:"position,omitempty"`
}

// NodePosition 节点位置（用于前端可视化）
type NodePosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Condition 条件定义
type Condition struct {
	Field    string           `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Value    interface{}      `json:"value"`
	Logic    string           `json:"logic,omitempty"` // and, or
	Children []Condition      `json:"children,omitempty"`
}

// Trigger 触发器定义
type Trigger struct {
	ID       string      `json:"id"`
	Type     TriggerType `json:"type"`
	Config   TriggerConfig `json:"config"`
	Enabled  bool        `json:"enabled"`
}

// TriggerConfig 触发器配置
type TriggerConfig struct {
	// 事件触发
	EventType  string            `json:"eventType,omitempty"`
	EventFilter map[string]string `json:"eventFilter,omitempty"`

	// 定时触发
	CronExpression string `json:"cronExpression,omitempty"`
	Timezone       string `json:"timezone,omitempty"`

	// 阈值触发
	ThresholdMetric string  `json:"thresholdMetric,omitempty"`
	ThresholdValue  float64 `json:"thresholdValue,omitempty"`
	ThresholdOp     string  `json:"thresholdOp,omitempty"` // gt, lt, gte, lte, eq

	// Webhook 触发
	WebhookPath   string `json:"webhookPath,omitempty"`
	WebhookSecret string `json:"webhookSecret,omitempty"`
}

// Execution 工作流执行记录
type Execution struct {
	ID         string          `json:"id"`
	WorkflowID string          `json:"workflowId"`
	Status     ExecutionStatus `json:"status"`

	// 触发信息
	TriggerType string `json:"triggerType"`
	TriggerID   string `json:"triggerId,omitempty"`
	TriggeredBy string `json:"triggeredBy,omitempty"` // user ID or system

	// 执行参数
	Input  map[string]interface{} `json:"input,omitempty"`
	Output map[string]interface{} `json:"output,omitempty"`

	// 节点执行状态
	NodeStates map[string]NodeExecutionState `json:"nodeStates"`

	// 时间信息
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Duration    string     `json:"duration,omitempty"`

	// 错误信息
	Error string `json:"error,omitempty"`

	// 版本快照
	WorkflowVersion int `json:"workflowVersion"`
}

// NodeExecutionState 节点执行状态
type NodeExecutionState struct {
	NodeID      string                 `json:"nodeId"`
	Status      NodeStatus             `json:"status"`
	StartedAt   *time.Time             `json:"startedAt,omitempty"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retryCount"`
}

// Event 系统事件
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// WorkflowTemplate 工作流模板
type WorkflowTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category,omitempty"`
	Workflow    Workflow `json:"workflow"`
	Tags        []string `json:"tags,omitempty"`
	IsBuiltin   bool     `json:"isBuiltin"`
	CreatedAt   time.Time `json:"createdAt"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID         string                 `json:"id"`
	EntityType string                 `json:"entityType"` // workflow, execution
	EntityID   string                 `json:"entityId"`
	Action     string                 `json:"action"` // create, update, delete, execute, cancel
	UserID     string                 `json:"userId,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// CreateWorkflowRequest 创建工作流请求
type CreateWorkflowRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Nodes       []WorkflowNode `json:"nodes" binding:"required"`
	Triggers    []Trigger      `json:"triggers,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
}

// UpdateWorkflowRequest 更新工作流请求
type UpdateWorkflowRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Nodes       []WorkflowNode  `json:"nodes,omitempty"`
	Triggers    []Trigger       `json:"triggers,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
}

// ExecuteWorkflowRequest 执行工作流请求
type ExecuteWorkflowRequest struct {
	Input       map[string]interface{} `json:"input,omitempty"`
	TriggeredBy string                 `json:"triggeredBy,omitempty"`
}

// WorkflowFilter 工作流过滤条件
type WorkflowFilter struct {
	Status   WorkflowStatus `json:"status,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Keyword  string         `json:"keyword,omitempty"`
	Page     int            `json:"page,omitempty"`
	PageSize int            `json:"pageSize,omitempty"`
}

// ExecutionFilter 执行记录过滤条件
type ExecutionFilter struct {
	WorkflowID string          `json:"workflowId,omitempty"`
	Status     ExecutionStatus `json:"status,omitempty"`
	StartTime  *time.Time      `json:"startTime,omitempty"`
	EndTime    *time.Time      `json:"endTime,omitempty"`
	Page       int             `json:"page,omitempty"`
	PageSize   int             `json:"pageSize,omitempty"`
}

// AuditLogFilter 审计日志过滤条件
type AuditLogFilter struct {
	EntityType string     `json:"entityType,omitempty"`
	EntityID   string     `json:"entityId,omitempty"`
	Action     string     `json:"action,omitempty"`
	StartTime  *time.Time `json:"startTime,omitempty"`
	EndTime    *time.Time `json:"endTime,omitempty"`
	Page       int        `json:"page,omitempty"`
	PageSize   int        `json:"pageSize,omitempty"`
}

// WorkflowStats 工作流统计
type WorkflowStats struct {
	TotalWorkflows   int     `json:"totalWorkflows"`
	ActiveWorkflows  int     `json:"activeWorkflows"`
	TotalExecutions  int     `json:"totalExecutions"`
	RunningExecs     int     `json:"runningExecutions"`
	SuccessExecs     int     `json:"successExecutions"`
	FailedExecs      int     `json:"failedExecutions"`
	AvgExecutionTime string  `json:"avgExecutionTime"`
	SuccessRate      float64 `json:"successRate"`
	TopWorkflows     []WorkflowUsage `json:"topWorkflows,omitempty"`
}

// WorkflowUsage 工作流使用统计
type WorkflowUsage struct {
	WorkflowID   string  `json:"workflowId"`
	WorkflowName string  `json:"workflowName"`
	ExecCount    int     `json:"executionCount"`
	SuccessRate  float64 `json:"successRate"`
}
