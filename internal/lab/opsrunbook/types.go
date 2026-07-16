// Package opsrunbook 运维手册自动化执行引擎
// 提供标准化运维流程定义、自动化执行、回滚与审计能力
// 对标 PagerDuty Runbook Automation、Rundeck 等企业级运维自动化平台
package opsrunbook

import (
	"time"
)

// RunbookStatus 运维手册状态.
type RunbookStatus string

const (
	StatusDraft     RunbookStatus = "draft"
	StatusActive    RunbookStatus = "active"
	StatusArchived  RunbookStatus = "archived"
	StatusExecuting RunbookStatus = "executing"
)

// StepType 步骤类型.
type StepType string

const (
	StepTypeCommand   StepType = "command"   // 执行命令
	StepTypeCheck     StepType = "check"     // 健康检查
	StepTypeWait      StepType = "wait"      // 等待条件
	StepTypeApproval  StepType = "approval"  // 人工审批
	StepTypeScript    StepType = "script"    // 脚本执行
	StepTypeNotify    StepType = "notify"    // 发送通知
	StepTypeRollback  StepType = "rollback"  // 回滚操作
	StepTypeCondition StepType = "condition" // 条件分支
)

// StepStatus 步骤执行状态.
type StepStatus string

const (
	StepPending  StepStatus = "pending"
	StepRunning  StepStatus = "running"
	StepSuccess  StepStatus = "success"
	StepFailed   StepStatus = "failed"
	StepSkipped  StepStatus = "skipped"
	StepRollback StepStatus = "rollback"
	StepWaiting  StepStatus = "waiting_approval"
)

// Severity 运维手册严重级别.
type Severity string

const (
	SevInfo     Severity = "info"
	SevWarning  Severity = "warning"
	SevError    Severity = "error"
	SevCritical Severity = "critical"
)

// TriggerType 触发方式.
type TriggerType string

const (
	TriggerManual   TriggerType = "manual"
	TriggerAlert    TriggerType = "alert"
	TriggerSchedule TriggerType = "schedule"
	TriggerWebhook  TriggerType = "webhook"
	TriggerIncident TriggerType = "incident"
)

// Runbook 运维手册定义.
type Runbook struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	Severity    Severity      `json:"severity"`
	Tags        []string      `json:"tags,omitempty"`
	Trigger     TriggerType   `json:"trigger"`
	Steps       []*Step       `json:"steps"`
	Variables   []*Variable   `json:"variables,omitempty"`
	Timeout     time.Duration `json:"timeout"`
	RollbackOn  string        `json:"rollback_on"` // failure, always, never
	Status      RunbookStatus `json:"status"`
	Version     int           `json:"version"`
	Author      string        `json:"author"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	RunCount    int           `json:"run_count"`
	SuccessRate float64       `json:"success_rate"`
}

// Step 运维步骤.
type Step struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        StepType          `json:"type"`
	Command     string            `json:"command,omitempty"`
	Script      string            `json:"script,omitempty"`
	Timeout     time.Duration     `json:"timeout"`
	RetryCount  int               `json:"retry_count"`
	RetryDelay  time.Duration     `json:"retry_delay"`
	Condition   string            `json:"condition,omitempty"`  // 条件表达式
	Variables   map[string]string `json:"variables,omitempty"`  // 步骤级变量
	Rollback    *Step             `json:"rollback,omitempty"`   // 回滚步骤
	DependsOn   []string          `json:"depends_on,omitempty"` // 依赖步骤ID
	AutoApprove bool              `json:"auto_approve"`         // 自动审批
	ContinueOn  string            `json:"continue_on"`          // failure, success, always
}

// Variable 运行时变量.
type Variable struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type"` // string, int, bool, secret
	DefaultValue string `json:"default_value,omitempty"`
	Required     bool   `json:"required"`
	Secret       bool   `json:"secret"`
}

// Execution 执行记录.
type Execution struct {
	ID          string            `json:"id"`
	RunbookID   string            `json:"runbook_id"`
	RunbookName string            `json:"runbook_name"`
	Status      StepStatus        `json:"status"`
	Trigger     TriggerType       `json:"trigger"`
	TriggerRef  string            `json:"trigger_ref,omitempty"` // 触发源引用(告警ID/事件ID等)
	Variables   map[string]string `json:"variables,omitempty"`
	Steps       []*StepResult     `json:"steps"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	Duration    time.Duration     `json:"duration"`
	Operator    string            `json:"operator"` // 执行人
	Error       string            `json:"error,omitempty"`
	Rollbacked  bool              `json:"rollbacked"`
}

// StepResult 步骤执行结果.
type StepResult struct {
	StepID    string        `json:"step_id"`
	StepName  string        `json:"step_name"`
	Status    StepStatus    `json:"status"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
	Retries   int           `json:"retries"`
}

// ApprovalRequest 审批请求.
type ApprovalRequest struct {
	ID          string     `json:"id"`
	ExecutionID string     `json:"execution_id"`
	StepID      string     `json:"step_id"`
	StepName    string     `json:"step_name"`
	Description string     `json:"description"`
	RequestedBy string     `json:"requested_by"`
	RequestedAt time.Time  `json:"requested_at"`
	ApprovedBy  string     `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	Rejected    bool       `json:"rejected"`
	Reason      string     `json:"reason,omitempty"`
}

// ExecutionStats 执行统计.
type ExecutionStats struct {
	TotalRuns    int           `json:"total_runs"`
	SuccessRuns  int           `json:"success_runs"`
	FailedRuns   int           `json:"failed_runs"`
	RollbackRuns int           `json:"rollback_runs"`
	AvgDuration  time.Duration `json:"avg_duration"`
	LastRunAt    *time.Time    `json:"last_run_at,omitempty"`
	SuccessRate  float64       `json:"success_rate"`
}

// RunbookFilter 运维手册过滤器.
type RunbookFilter struct {
	Category string        `json:"category,omitempty"`
	Severity Severity      `json:"severity,omitempty"`
	Status   RunbookStatus `json:"status,omitempty"`
	Tags     []string      `json:"tags,omitempty"`
	Trigger  TriggerType   `json:"trigger,omitempty"`
	Search   string        `json:"search,omitempty"`
}

// ExecutionFilter 执行记录过滤器.
type ExecutionFilter struct {
	RunbookID string      `json:"runbook_id,omitempty"`
	Status    StepStatus  `json:"status,omitempty"`
	Trigger   TriggerType `json:"trigger,omitempty"`
	Since     *time.Time  `json:"since,omitempty"`
	Until     *time.Time  `json:"until,omitempty"`
	Limit     int         `json:"limit,omitempty"`
}
