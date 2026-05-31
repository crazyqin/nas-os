// Package aiagentorch 实现智能AI代理编排器
// 支持多种代理类型：存储优化、备份、安全监控、文件整理
// 提供代理任务调度、执行日志、消息传递协作和权限控制
package aiagentorch

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrAgentNotFound 代理不存在.
	ErrAgentNotFound = errors.New("agent not found")
	// ErrAgentNameExists 代理名称已存在.
	ErrAgentNameExists = errors.New("agent name already exists")
	// ErrInvalidConfig 配置无效.
	ErrInvalidConfig = errors.New("invalid agent configuration")
	// ErrAgentNotActive 代理未激活.
	ErrAgentNotActive = errors.New("agent is not active")
	// ErrTaskNotFound 任务不存在.
	ErrTaskNotFound = errors.New("task not found")
	// ErrPermissionDenied 权限不足.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrMessageNotFound 消息不存在.
	ErrMessageNotFound = errors.New("message not found")
)

// ========== 代理类型 ==========

// AgentType 代理类型.
type AgentType string

const (
	// AgentTypeStorageOptim 存储优化代理.
	AgentTypeStorageOptim AgentType = "storage_optimization"
	// AgentTypeBackup 备份代理.
	AgentTypeBackup AgentType = "backup"
	// AgentTypeSecurity 安全监控代理.
	AgentTypeSecurity AgentType = "security_monitor"
	// AgentTypeFileOrganizer 文件整理代理.
	AgentTypeFileOrganizer AgentType = "file_organizer"
)

// ========== 代理状态 ==========

// AgentStatus 代理状态.
type AgentStatus string

const (
	// AgentStatusActive 活跃.
	AgentStatusActive AgentStatus = "active"
	// AgentStatusInactive 停用.
	AgentStatusInactive AgentStatus = "inactive"
	// AgentStatusRunning 运行中.
	AgentStatusRunning AgentStatus = "running"
	// AgentStatusError 错误.
	AgentStatusError AgentStatus = "error"
)

// ========== 触发类型 ==========

// TriggerType 触发类型.
type TriggerType string

const (
	// TriggerManual 手动触发.
	TriggerManual TriggerType = "manual"
	// TriggerScheduled 定时触发.
	TriggerScheduled TriggerType = "scheduled"
	// TriggerEvent 事件触发.
	TriggerEvent TriggerType = "event"
)

// ========== 执行状态 ==========

// ExecutionStatus 执行状态.
type ExecutionStatus string

const (
	// ExecPending 等待中.
	ExecPending ExecutionStatus = "pending"
	// ExecRunning 运行中.
	ExecRunning ExecutionStatus = "running"
	// ExecSuccess 成功.
	ExecSuccess ExecutionStatus = "success"
	// ExecFailed 失败.
	ExecFailed ExecutionStatus = "failed"
	// ExecCancelled 已取消.
	ExecCancelled ExecutionStatus = "cancelled"
)

// ========== 消息优先级 ==========

// MessagePriority 消息优先级.
type MessagePriority string

const (
	// PriorityLow 低优先级.
	PriorityLow MessagePriority = "low"
	// PriorityNormal 普通优先级.
	PriorityNormal MessagePriority = "normal"
	// PriorityHigh 高优先级.
	PriorityHigh MessagePriority = "high"
	// PriorityUrgent 紧急.
	PriorityUrgent MessagePriority = "urgent"
)

// ========== 事件触发条件 ==========

// EventTrigger 事件触发条件.
type EventTrigger struct {
	// EventType 事件类型.
	EventType string `json:"eventType"`
	// EventPattern 事件匹配模式.
	EventPattern string `json:"eventPattern"`
	// DebounceSec 去抖时间（秒）.
	DebounceSec int `json:"debounceSec"`
}

// ========== 代理权限 ==========

// AgentPermission 代理权限配置.
type AgentPermission struct {
	// AllowedPaths 允许访问的目录列表.
	AllowedPaths []string `json:"allowedPaths"`
	// DeniedPaths 禁止访问的目录列表.
	DeniedPaths []string `json:"deniedPaths"`
	// ReadOnly 是否只读.
	ReadOnly bool `json:"readOnly"`
	// MaxConcurrent 最大并发任务数.
	MaxConcurrent int `json:"maxConcurrent"`
	// RateLimitPerMin 每分钟最大操作次数.
	RateLimitPerMin int `json:"rateLimitPerMin"`
}

// ========== 代理配置 ==========

// AgentConfig 代理配置.
type AgentConfig struct {
	// ModelName AI模型名称.
	ModelName string `json:"modelName"`
	// ModelEndpoint 模型API端点.
	ModelEndpoint string `json:"modelEndpoint"`
	// Temperature 生成温度.
	Temperature float64 `json:"temperature"`
	// MaxTokens 最大token数.
	MaxTokens int `json:"maxTokens"`
	// SystemPrompt 系统提示词.
	SystemPrompt string `json:"systemPrompt"`
	// Parameters 自定义参数.
	Parameters map[string]string `json:"parameters"`
}

// ========== 代理定义 ==========

// Agent AI代理定义.
type Agent struct {
	// ID 代理ID.
	ID string `json:"id"`
	// Name 代理名称.
	Name string `json:"name"`
	// Type 代理类型.
	Type AgentType `json:"type"`
	// Description 描述.
	Description string `json:"description"`
	// Status 状态.
	Status AgentStatus `json:"status"`
	// Config 代理AI配置.
	Config AgentConfig `json:"config"`
	// Permission 权限配置.
	Permission AgentPermission `json:"permission"`
	// TriggerType 默认触发类型.
	TriggerType TriggerType `json:"triggerType"`
	// CronExpr 定时表达式（scheduled触发时使用）.
	CronExpr string `json:"cronExpr"`
	// EventTriggers 事件触发条件列表.
	EventTriggers []EventTrigger `json:"eventTriggers"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// LastRun 上次运行时间.
	LastRun *time.Time `json:"lastRun,omitempty"`
	// NextRun 下次运行时间.
	NextRun *time.Time `json:"nextRun,omitempty"`
	// RunCount 总运行次数.
	RunCount int `json:"runCount"`
	// ErrorCount 错误次数.
	ErrorCount int `json:"errorCount"`
	// LastError 最后错误信息.
	LastError string `json:"lastError,omitempty"`
	// Tags 标签.
	Tags []string `json:"tags"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ========== 代理任务 ==========

// AgentTask 代理任务.
type AgentTask struct {
	// ID 任务ID.
	ID string `json:"id"`
	// AgentID 所属代理ID.
	AgentID string `json:"agentId"`
	// Name 任务名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description"`
	// TriggerType 触发类型.
	TriggerType TriggerType `json:"triggerType"`
	// CronExpr 定时表达式.
	CronExpr string `json:"cronExpr"`
	// EventTrigger 事件触发条件.
	EventTrigger *EventTrigger `json:"eventTrigger,omitempty"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// LastRun 上次运行时间.
	LastRun *time.Time `json:"lastRun,omitempty"`
	// NextRun 下次运行时间.
	NextRun *time.Time `json:"nextRun,omitempty"`
	// RunCount 运行次数.
	RunCount int `json:"runCount"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 执行日志 ==========

// ExecutionLog 执行日志.
type ExecutionLog struct {
	// ID 日志ID.
	ID string `json:"id"`
	// AgentID 代理ID.
	AgentID string `json:"agentId"`
	// AgentName 代理名称.
	AgentName string `json:"agentName"`
	// TaskID 关联任务ID（可空）.
	TaskID string `json:"taskId,omitempty"`
	// TriggerType 触发类型.
	TriggerType TriggerType `json:"triggerType"`
	// Status 执行状态.
	Status ExecutionStatus `json:"status"`
	// StartTime 开始时间.
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间.
	EndTime *time.Time `json:"endTime,omitempty"`
	// DurationMs 执行时长（毫秒）.
	DurationMs int64 `json:"durationMs"`
	// Result 执行结果摘要.
	Result string `json:"result,omitempty"`
	// Error 错误信息.
	Error string `json:"error,omitempty"`
	// Details 详细信息.
	Details map[string]string `json:"details,omitempty"`
}

// ========== 代理间消息 ==========

// AgentMessage 代理间消息.
type AgentMessage struct {
	// ID 消息ID.
	ID string `json:"id"`
	// FromAgentID 发送方代理ID.
	FromAgentID string `json:"fromAgentId"`
	// ToAgentID 接收方代理ID.
	ToAgentID string `json:"toAgentId"`
	// MessageType 消息类型.
	MessageType string `json:"messageType"`
	// Content 消息内容.
	Content string `json:"content"`
	// Priority 优先级.
	Priority MessagePriority `json:"priority"`
	// Read 是否已读.
	Read bool `json:"read"`
	// CreatedAt 发送时间.
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 仪表板统计 ==========

// OrchStats 编排器统计.
type OrchStats struct {
	// TotalAgents 代理总数.
	TotalAgents int `json:"totalAgents"`
	// ActiveAgents 活跃代理数.
	ActiveAgents int `json:"activeAgents"`
	// RunningAgents 运行中代理数.
	RunningAgents int `json:"runningAgents"`
	// ErrorAgents 错误代理数.
	ErrorAgents int `json:"errorAgents"`
	// TotalTasks 任务总数.
	TotalTasks int `json:"totalTasks"`
	// TotalExecutions 总执行次数.
	TotalExecutions int `json:"totalExecutions"`
	// SuccessExecutions 成功执行数.
	SuccessExecutions int `json:"successExecutions"`
	// FailedExecutions 失败执行数.
	FailedExecutions int `json:"failedExecutions"`
	// UnreadMessages 未读消息数.
	UnreadMessages int `json:"unreadMessages"`
	// AgentsByType 按类型统计.
	AgentsByType map[AgentType]int `json:"agentsByType"`
}
