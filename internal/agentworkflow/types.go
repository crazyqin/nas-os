// Package agentworkflow 提供 AI 代理工作流功能
// 参考群晖 DSM Agent 2.0 的 agentic workflows，实现自然语言任务解析、
// 跨服务工作流编排、多步骤自动化、条件分支和任务状态管理
package agentworkflow

import (
	"time"
)

// ========== 任务状态定义 ==========

// TaskStatus 任务执行状态.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"   // 待执行
	TaskRunning   TaskStatus = "running"   // 执行中
	TaskCompleted TaskStatus = "completed" // 已完成
	TaskFailed    TaskStatus = "failed"    // 失败
	TaskCancelled TaskStatus = "cancelled" // 已取消
	TaskPaused    TaskStatus = "paused"    // 已暂停
)

// StepStatus 工作流步骤状态.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
	StepWaiting   StepStatus = "waiting" // 等待条件满足
)

// ========== 条件操作符 ==========

// ConditionOperator 条件操作符.
type ConditionOperator string

const (
	OpEquals      ConditionOperator = "eq"       // 等于
	OpNotEquals   ConditionOperator = "ne"       // 不等于
	OpGreaterThan ConditionOperator = "gt"       // 大于
	OpLessThan    ConditionOperator = "lt"       // 小于
	OpGreaterEq   ConditionOperator = "gte"      // 大于等于
	OpLessEq      ConditionOperator = "lte"      // 小于等于
	OpContains    ConditionOperator = "contains" // 包含
	OpIn          ConditionOperator = "in"       // 在列表中
	OpNotIn       ConditionOperator = "notIn"    // 不在列表中
)

// ========== 工作流类型 ==========

// WorkflowType 工作流类型.
type WorkflowType string

const (
	WorkflowSequential  WorkflowType = "sequential"  // 顺序执行
	WorkflowParallel    WorkflowType = "parallel"    // 并行执行
	WorkflowConditional WorkflowType = "conditional" // 条件分支
	WorkflowLoop        WorkflowType = "loop"        // 循环
	WorkflowEvent       WorkflowType = "event"       // 事件驱动
)

// ========== 核心类型 ==========

// AgentTask AI 代理任务.
type AgentTask struct {
	ID           string         `json:"id"`
	NLInput      string         `json:"nlInput"`      // 自然语言输入
	ParsedIntent string         `json:"parsedIntent"` // 解析出的意图
	WorkflowID   string         `json:"workflowId,omitempty"`
	Status       TaskStatus     `json:"status"`
	Progress     float64        `json:"progress"` // 0-100
	Priority     int            `json:"priority"` // 1-10
	CreatedAt    time.Time      `json:"createdAt"`
	StartedAt    time.Time      `json:"startedAt,omitempty"`
	FinishedAt   time.Time      `json:"finishedAt,omitempty"`
	Error        string         `json:"error,omitempty"`
	Result       map[string]any `json:"result,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
}

// Workflow 工作流定义.
type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        WorkflowType   `json:"type"`
	Steps       []WorkflowStep `json:"steps"`
	TaskID      string         `json:"taskId,omitempty"`
	Status      TaskStatus     `json:"status"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	Version     string         `json:"version,omitempty"`
}

// WorkflowStep 工作流步骤.
type WorkflowStep struct {
	ID          string         `json:"id"`
	Order       int            `json:"order"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Service     string         `json:"service"` // 目标服务名
	Action      string         `json:"action"`  // 服务动作
	Parameters  map[string]any `json:"parameters,omitempty"`
	Condition   *TaskCondition `json:"condition,omitempty"` // 执行前提条件
	OnSuccess   string         `json:"onSuccess,omitempty"` // 成功后跳转的步骤 ID
	OnFailure   string         `json:"onFailure,omitempty"` // 失败后跳转的步骤 ID
	Status      StepStatus     `json:"status"`
	StartedAt   time.Time      `json:"startedAt,omitempty"`
	FinishedAt  time.Time      `json:"finishedAt,omitempty"`
	Error       string         `json:"error,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
}

// TaskCondition 任务条件.
type TaskCondition struct {
	Field    string            `json:"field"` // 检查的字段（来自前序步骤输出）
	Operator ConditionOperator `json:"operator"`
	Value    any               `json:"value"` // 比较值
}

// ExecutionContext 执行上下文 - 在工作流步骤间传递数据.
type ExecutionContext struct {
	TaskID      string                    `json:"taskId"`
	WorkflowID  string                    `json:"workflowId"`
	Variables   map[string]any            `json:"variables"`  // 上下文变量
	StepOutput  map[string]map[string]any `json:"stepOutput"` // 各步骤的输出
	StartedAt   time.Time                 `json:"startedAt"`
	CurrentStep int                       `json:"currentStep"`
}

// WorkflowTemplate 工作流模板.
type WorkflowTemplate struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        WorkflowType   `json:"type"`
	Steps       []WorkflowStep `json:"steps"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// ========== 请求/响应类型 ==========

// ParseTaskRequest 自然语言任务解析请求.
type ParseTaskRequest struct {
	Input    string   `json:"input" binding:"required"`
	Priority int      `json:"priority,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// ParseTaskResult 任务解析结果.
type ParseTaskResult struct {
	TaskID       string    `json:"taskId"`
	NLInput      string    `json:"nlInput"`
	ParsedIntent string    `json:"parsedIntent"`
	Workflow     *Workflow `json:"workflow,omitempty"`
	Confidence   float64   `json:"confidence"` // 0-1
	Warnings     []string  `json:"warnings,omitempty"`
}

// ExecuteWorkflowRequest 执行工作流请求.
type ExecuteWorkflowRequest struct {
	TaskID    string         `json:"taskId" binding:"required"`
	DryRun    bool           `json:"dryRun"`
	Variables map[string]any `json:"variables,omitempty"`
}

// ExecuteWorkflowResult 工作流执行结果.
type ExecuteWorkflowResult struct {
	TaskID     string         `json:"taskId"`
	WorkflowID string         `json:"workflowId"`
	Status     TaskStatus     `json:"status"`
	Progress   float64        `json:"progress"`
	Steps      []WorkflowStep `json:"steps"`
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt time.Time      `json:"finishedAt,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// CancelTaskRequest 取消任务请求.
type CancelTaskRequest struct {
	TaskID string `json:"taskId" binding:"required"`
	Reason string `json:"reason,omitempty"`
}

// TaskStatusResponse 任务状态响应.
type TaskStatusResponse struct {
	TaskID    string     `json:"taskId"`
	Status    TaskStatus `json:"status"`
	Progress  float64    `json:"progress"`
	Workflow  *Workflow  `json:"workflow,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// ========== 内部模型 ==========

// taskState 内部任务状态.
type taskState struct {
	id           string
	nlInput      string
	parsedIntent string
	workflowID   string
	status       TaskStatus
	progress     float64
	priority     int
	workflow     *Workflow
	context      *ExecutionContext
	createdAt    time.Time
	startedAt    time.Time
	finishedAt   time.Time
	error        string
	result       map[string]any
	tags         []string
}

// ========== 意图到工作流的映射规则 ==========

// IntentRule 意图规则.
type IntentRule struct {
	Keywords     []string       `json:"keywords"`
	Intent       string         `json:"intent"`
	WorkflowType WorkflowType   `json:"workflowType"`
	Steps        []WorkflowStep `json:"steps"`
}
