// Package workflowengine 提供工作流引擎功能
// 支持 DAG 工作流编排、条件触发、事件驱动自动化
package workflowengine

import (
	"time"
)

// WorkflowStatus 工作流状态
type WorkflowStatus string

const (
	// WorkflowStatusDraft 草稿状态，工作流正在编辑中
	WorkflowStatusDraft    WorkflowStatus = "draft"
	// WorkflowStatusActive 激活状态，工作流可以被执行
	WorkflowStatusActive   WorkflowStatus = "active"
	// WorkflowStatusDisabled 停用状态，工作流暂时不可执行
	WorkflowStatusDisabled WorkflowStatus = "disabled"
	// WorkflowStatusArchived 归档状态，工作流已归档
	WorkflowStatusArchived WorkflowStatus = "archived"
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	// ExecutionStatusPending 等待执行
	ExecutionStatusPending   ExecutionStatus = "pending"
	// ExecutionStatusRunning 正在执行
	ExecutionStatusRunning   ExecutionStatus = "running"
	// ExecutionStatusSuccess 执行成功
	ExecutionStatusSuccess   ExecutionStatus = "success"
	// ExecutionStatusFailed 执行失败
	ExecutionStatusFailed    ExecutionStatus = "failed"
	// ExecutionStatusCancelled 执行已取消
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	// ExecutionStatusSkipped 执行已跳过
	ExecutionStatusSkipped   ExecutionStatus = "skipped"
)

// NodeStatus 节点执行状态
type NodeStatus string

const (
	// NodeStatusPending 等待执行
	NodeStatusPending   NodeStatus = "pending"
	// NodeStatusRunning 正在执行
	NodeStatusRunning   NodeStatus = "running"
	// NodeStatusSuccess 执行成功
	NodeStatusSuccess   NodeStatus = "success"
	// NodeStatusFailed 执行失败
	NodeStatusFailed    NodeStatus = "failed"
	// NodeStatusSkipped 已跳过
	NodeStatusSkipped   NodeStatus = "skipped"
	// NodeStatusCancelled 已取消
	NodeStatusCancelled NodeStatus = "cancelled"
)

// TriggerType 触发器类型
type TriggerType string

const (
	// TriggerTypeManual 手动触发
	TriggerTypeManual    TriggerType = "manual"
	// TriggerTypeEvent 事件触发
	TriggerTypeEvent     TriggerType = "event"
	// TriggerTypeSchedule 定时触发
	TriggerTypeSchedule  TriggerType = "schedule"
	// TriggerTypeWebhook Webhook 触发
	TriggerTypeWebhook   TriggerType = "webhook"
	// TriggerTypeThreshold 阈值触发
	TriggerTypeThreshold TriggerType = "threshold"
)

// ConditionOperator 条件操作符
type ConditionOperator string

const (
	// ConditionOpEquals 等于
	ConditionOpEquals    ConditionOperator = "equals"
	// ConditionOpNotEquals 不等于
	ConditionOpNotEquals ConditionOperator = "not_equals"
	// ConditionOpGreater 大于
	ConditionOpGreater   ConditionOperator = "greater_than"
	// ConditionOpLess 小于
	ConditionOpLess      ConditionOperator = "less_than"
	// ConditionOpContains 包含
	ConditionOpContains  ConditionOperator = "contains"
	// ConditionOpIn 在列表中
	ConditionOpIn        ConditionOperator = "in"
	// ConditionOpRegex 正则匹配
	ConditionOpRegex     ConditionOperator = "regex"
)

// Workflow 工作流定义
// 表示一个完整的工作流配置，包含节点、触发器、变量等
// 工作流由 DAG（有向无环图）节点组成，支持条件触发和事件驱动自动化
type Workflow struct {
	// ID 工作流唯一标识符
	ID string `json:"id"`
	// Name 工作流名称
	Name string `json:"name"`
	// Description 工作流描述
	Description string `json:"description,omitempty"`
	// Status 工作流状态（draft/active/disabled/archived）
	Status WorkflowStatus `json:"status"`
	// Version 工作流版本号，每次更新自增
	Version int `json:"version"`

	// Nodes DAG 节点列表
	Nodes []WorkflowNode `json:"nodes"`

	// Triggers 触发器列表
	Triggers []Trigger `json:"triggers,omitempty"`

	// Variables 全局变量
	Variables map[string]interface{} `json:"variables,omitempty"`

	// Tags 标签
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
// 表示工作流中的一个执行单元，支持多种节点类型：
//   - task: 普通任务节点，执行具体的任务逻辑
//   - condition: 条件节点，根据条件判断执行路径
//   - parallel: 并行节点，并行执行多个子节点
//   - subworkflow: 子工作流节点，调用其他工作流
type WorkflowNode struct {
	// ID 节点唯一标识符
	ID string `json:"id"`
	// Name 节点名称
	Name string `json:"name"`
	// Description 节点描述
	Description string `json:"description,omitempty"`
	// Type 节点类型
	Type string `json:"type"`

	// Dependencies 依赖关系（上游节点 ID 列表）
	Dependencies []string `json:"dependencies,omitempty"`

	// 任务配置
	TaskType   string                 `json:"taskType,omitempty"`   // shell, http, script, builtin
	TaskConfig map[string]interface{} `json:"taskConfig,omitempty"`

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
// 记录工作流的一次执行，包括状态、节点执行状态、输入输出等
type Execution struct {
	// ID 执行记录唯一标识符
	ID string `json:"id"`
	// WorkflowID 关联的工作流 ID
	WorkflowID string `json:"workflowId"`
	// Status 执行状态
	Status ExecutionStatus `json:"status"`

	// 触发信息
	TriggerType string `json:"triggerType"`
	TriggerID   string `json:"triggerId,omitempty"`
	TriggeredBy string `json:"triggeredBy,omitempty"` // user ID or system

	// 执行参数
	Input  map[string]interface{} `json:"input,omitempty"`
	Output map[string]interface{} `json:"output,omitempty"`

	// NodeStates 节点执行状态
	NodeStates map[string]NodeExecutionState `json:"nodeStates"`

	// 时间信息
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Duration    string     `json:"duration,omitempty"`

	// Error 错误信息
	Error string `json:"error,omitempty"`

	// WorkflowVersion 工作流版本快照
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
// 模板可以快速创建预定义的工作流
type WorkflowTemplate struct {
	// ID 模板唯一标识符
	ID string `json:"id"`
	// Name 模板名称
	Name string `json:"name"`
	// Description 模板描述
	Description string `json:"description,omitempty"`
	// Category 模板分类（如 backup, monitoring, deployment 等）
	Category string `json:"category,omitempty"`
	// Workflow 模板包含的工作流定义
	Workflow Workflow `json:"workflow"`
	// Tags 标签
	Tags []string `json:"tags,omitempty"`
	// IsBuiltin 是否为内置模板
	IsBuiltin bool `json:"isBuiltin"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
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
	// TotalWorkflows 工作流总数
	TotalWorkflows   int     `json:"totalWorkflows"`
	// ActiveWorkflows 激活的工作流数
	ActiveWorkflows  int     `json:"activeWorkflows"`
	// TotalExecutions 执行总数
	TotalExecutions  int     `json:"totalExecutions"`
	// RunningExecs 正在执行的执行数
	RunningExecs     int     `json:"runningExecutions"`
	// SuccessExecs 成功的执行数
	SuccessExecs     int     `json:"successExecutions"`
	// FailedExecs 失败的执行数
	FailedExecs      int     `json:"failedExecutions"`
	// AvgExecutionTime 平均执行时间
	AvgExecutionTime string  `json:"avgExecutionTime"`
	// SuccessRate 成功率（百分比）
	SuccessRate      float64 `json:"successRate"`
	// TopWorkflows 使用最多的工作流
	TopWorkflows     []WorkflowUsage `json:"topWorkflows,omitempty"`
}

// WorkflowUsage 工作流使用统计
type WorkflowUsage struct {
	WorkflowID   string  `json:"workflowId"`
	WorkflowName string  `json:"workflowName"`
	ExecCount    int     `json:"executionCount"`
	SuccessRate  float64 `json:"successRate"`
}
