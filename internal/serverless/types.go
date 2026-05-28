// Package serverless 提供边缘 Serverless 函数引擎功能.
package serverless

import (
	"time"
)

// Runtime 函数运行时.
type Runtime string

const (
	// RuntimeGo Go 运行时.
	RuntimeGo Runtime = "go"
	// RuntimePython Python 运行时.
	RuntimePython Runtime = "python"
	// RuntimeNode 运行时.
	RuntimeNode Runtime = "node"
	// RuntimeShell Shell 运行时.
	RuntimeShell Runtime = "shell"
)

// DeployStatus 部署状态.
type DeployStatus string

const (
	// DeployStatusDraft 草稿.
	DeployStatusDraft DeployStatus = "draft"
	// DeployStatusDeploying 部署中.
	DeployStatusDeploying DeployStatus = "deploying"
	// DeployStatusDeployed 已部署.
	DeployStatusDeployed DeployStatus = "deployed"
	// DeployStatusFailed 部署失败.
	DeployStatusFailed DeployStatus = "failed"
	// DeployStatusStopped 已停止.
	DeployStatusStopped DeployStatus = "stopped"
)

// TriggerType 触发器类型.
type TriggerType string

const (
	// TriggerHTTP HTTP 触发器.
	TriggerHTTP TriggerType = "http"
	// TriggerCron Cron 触发器.
	TriggerCron TriggerType = "cron"
	// TriggerFileWatcher 文件监控触发器.
	TriggerFileWatcher TriggerType = "filewatcher"
	// TriggerEvent 事件触发器.
	TriggerEvent TriggerType = "event"
)

// InvocationMode 调用模式.
type InvocationMode string

const (
	// InvocationSync 同步调用.
	InvocationSync InvocationMode = "sync"
	// InvocationAsync 异步调用.
	InvocationAsync InvocationMode = "async"
)

// InvocationStatus 调用状态.
type InvocationStatus string

const (
	// InvocationStatusPending 等待执行.
	InvocationStatusPending InvocationStatus = "pending"
	// InvocationStatusRunning 执行中.
	InvocationStatusRunning InvocationStatus = "running"
	// InvocationStatusSuccess 执行成功.
	InvocationStatusSuccess InvocationStatus = "success"
	// InvocationStatusFailed 执行失败.
	InvocationStatusFailed InvocationStatus = "failed"
	// InvocationStatusTimeout 执行超时.
	InvocationStatusTimeout InvocationStatus = "timeout"
)

// LogLevel 日志级别.
type LogLevel string

const (
	// LogLevelDebug 调试.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo 信息.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn 警告.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError 错误.
	LogLevelError LogLevel = "error"
)

// FunctionConfig 函数资源配置.
type FunctionConfig struct {
	// CPUMilli CPU 毫核限制.
	CPUMilli int `json:"cpu_milli"`
	// MemoryMB 内存 MB 限制.
	MemoryMB int `json:"memory_mb"`
	// TimeoutS 超时秒数.
	TimeoutS int `json:"timeout_s"`
	// MaxConcurrency 最大并发数.
	MaxConcurrency int `json:"max_concurrency"`
	// EnvVars 环境变量.
	EnvVars map[string]string `json:"env_vars,omitempty"`
}

// DefaultFunctionConfig 返回默认函数配置.
func DefaultFunctionConfig() FunctionConfig {
	return FunctionConfig{
		CPUMilli:       100,
		MemoryMB:       128,
		TimeoutS:       30,
		MaxConcurrency: 10,
		EnvVars:        make(map[string]string),
	}
}

// Function 函数定义.
type Function struct {
	// ID 函数唯一标识.
	ID string `json:"id"`
	// Name 函数名称.
	Name string `json:"name"`
	// Description 描述.
	Description string `json:"description,omitempty"`
	// Runtime 运行时.
	Runtime Runtime `json:"runtime"`
	// Handler 入口处理器.
	Handler string `json:"handler"`
	// Code 函数代码.
	Code string `json:"code"`
	// Version 当前版本.
	Version string `json:"version"`
	// DeployStatus 部署状态.
	DeployStatus DeployStatus `json:"deploy_status"`
	// Config 资源配置.
	Config FunctionConfig `json:"config"`
	// Triggers 触发器列表.
	Triggers []*Trigger `json:"triggers,omitempty"`
	// Tags 标签.
	Tags []string `json:"tags,omitempty"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// Metadata 元数据.
	Metadata map[string]string `json:"metadata,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
	// LastInvokeAt 最后调用时间.
	LastInvokeAt *time.Time `json:"last_invoke_at,omitempty"`
	// InvokeCount 累计调用次数.
	InvokeCount int64 `json:"invoke_count"`
	// ErrorCount 累计错误次数.
	ErrorCount int64 `json:"error_count"`
}

// Trigger 触发器.
type Trigger struct {
	// ID 触发器唯一标识.
	ID string `json:"id"`
	// FunctionID 关联函数 ID.
	FunctionID string `json:"function_id"`
	// Type 触发器类型.
	Type TriggerType `json:"type"`
	// Config 触发器配置.
	Config TriggerConfig `json:"config"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
}

// TriggerConfig 触发器配置.
type TriggerConfig struct {
	// HTTP 触发器配置.
	Path   string `json:"path,omitempty"`   // HTTP 路径
	Method string `json:"method,omitempty"` // HTTP 方法
	// Cron 触发器配置.
	Schedule string `json:"schedule,omitempty"` // Cron 表达式
	// FileWatcher 触发器配置.
	WatchPath  string `json:"watch_path,omitempty"`  // 监控路径
	FileFilter string `json:"file_filter,omitempty"` // 文件过滤
	// Event 触发器配置.
	EventType string `json:"event_type,omitempty"` // 事件类型
	EventSrc  string `json:"event_src,omitempty"`  // 事件源
}

// Invocation 函数调用记录.
type Invocation struct {
	// ID 调用唯一标识.
	ID string `json:"id"`
	// FunctionID 函数 ID.
	FunctionID string `json:"function_id"`
	// FunctionName 函数名称.
	FunctionName string `json:"function_name"`
	// Version 函数版本.
	Version string `json:"version"`
	// Mode 调用模式.
	Mode InvocationMode `json:"mode"`
	// Status 调用状态.
	Status InvocationStatus `json:"status"`
	// Request 请求数据.
	Request map[string]interface{} `json:"request,omitempty"`
	// Response 响应数据.
	Response map[string]interface{} `json:"response,omitempty"`
	// Error 错误信息.
	Error string `json:"error,omitempty"`
	// TriggeredBy 触发来源.
	TriggeredBy string `json:"triggered_by,omitempty"`
	// StartedAt 开始时间.
	StartedAt time.Time `json:"started_at"`
	// CompletedAt 完成时间.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// Duration 执行耗时.
	Duration time.Duration `json:"duration"`
	// MemoryUsedMB 内存使用量.
	MemoryUsedMB int `json:"memory_used_mb,omitempty"`
	// CPUUsedMilli CPU 使用量.
	CPUUsedMilli int `json:"cpu_used_milli,omitempty"`
}

// FunctionVersion 函数版本记录.
type FunctionVersion struct {
	// Version 版本号.
	Version string `json:"version"`
	// FunctionID 函数 ID.
	FunctionID string `json:"function_id"`
	// Code 代码快照.
	Code string `json:"code"`
	// Config 配置快照.
	Config FunctionConfig `json:"config"`
	// Runtime 运行时.
	Runtime Runtime `json:"runtime"`
	// Handler 入口处理器.
	Handler string `json:"handler"`
	// DeployStatus 部署状态.
	DeployStatus DeployStatus `json:"deploy_status"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// CreatedBy 创建者.
	CreatedBy string `json:"created_by,omitempty"`
	// Changelog 变更说明.
	Changelog string `json:"changelog,omitempty"`
}

// FunctionLog 函数执行日志.
type FunctionLog struct {
	// ID 日志唯一标识.
	ID string `json:"id"`
	// InvocationID 调用 ID.
	InvocationID string `json:"invocation_id"`
	// FunctionID 函数 ID.
	FunctionID string `json:"function_id"`
	// Level 日志级别.
	Level LogLevel `json:"level"`
	// Message 日志内容.
	Message string `json:"message"`
	// Timestamp 日志时间.
	Timestamp time.Time `json:"timestamp"`
	// Data 附加数据.
	Data map[string]interface{} `json:"data,omitempty"`
}

// FunctionMetrics 函数指标.
type FunctionMetrics struct {
	// FunctionID 函数 ID.
	FunctionID string `json:"function_id"`
	// TotalInvocations 总调用次数.
	TotalInvocations int64 `json:"total_invocations"`
	// SuccessCount 成功次数.
	SuccessCount int64 `json:"success_count"`
	// ErrorCount 失败次数.
	ErrorCount int64 `json:"error_count"`
	// TimeoutCount 超时次数.
	TimeoutCount int64 `json:"timeout_count"`
	// AvgDuration 平均执行耗时.
	AvgDuration time.Duration `json:"avg_duration"`
	// P50Duration P50 耗时.
	P50Duration time.Duration `json:"p50_duration"`
	// P95Duration P95 耗时.
	P95Duration time.Duration `json:"p95_duration"`
	// P99Duration P99 耗时.
	P99Duration time.Duration `json:"p99_duration"`
	// MaxMemoryUsedMB 最大内存使用.
	MaxMemoryUsedMB int `json:"max_memory_used_mb"`
	// Period 统计周期.
	Period string `json:"period"`
	// PeriodStart 周期开始时间.
	PeriodStart time.Time `json:"period_start"`
	// PeriodEnd 周期结束时间.
	PeriodEnd time.Time `json:"period_end"`
}

// FunctionFilter 函数过滤条件.
type FunctionFilter struct {
	Runtime      Runtime      `json:"runtime,omitempty"`
	DeployStatus DeployStatus `json:"deploy_status,omitempty"`
	Enabled      *bool        `json:"enabled,omitempty"`
	Tags         []string     `json:"tags,omitempty"`
	Search       string       `json:"search,omitempty"`
	Page         int          `json:"page,omitempty"`
	PageSize     int          `json:"page_size,omitempty"`
}

// InvocationFilter 调用记录过滤条件.
type InvocationFilter struct {
	FunctionID string           `json:"function_id,omitempty"`
	Status     InvocationStatus `json:"status,omitempty"`
	StartTime  *time.Time       `json:"start_time,omitempty"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	Page       int              `json:"page,omitempty"`
	PageSize   int              `json:"page_size,omitempty"`
}

// Stats 引擎统计.
type Stats struct {
	TotalFunctions    int            `json:"total_functions"`
	DeployedFunctions int            `json:"deployed_functions"`
	EnabledFunctions  int            `json:"enabled_functions"`
	TotalInvocations  int64          `json:"total_invocations"`
	TodayInvocations  int64          `json:"today_invocations"`
	SuccessCount      int64          `json:"success_count"`
	ErrorCount        int64          `json:"error_count"`
	SuccessRate       float64        `json:"success_rate"`
	RunningFunctions  int            `json:"running_functions"`
	RuntimeStats      map[string]int `json:"runtime_stats"`
}

// InvokeRequest 调用请求.
type InvokeRequest struct {
	FunctionID string                 `json:"function_id"`
	Mode       InvocationMode         `json:"mode"`
	Input      map[string]interface{} `json:"input,omitempty"`
}

// InvokeResponse 调用响应.
type InvokeResponse struct {
	InvocationID string                 `json:"invocation_id"`
	Status       InvocationStatus       `json:"status"`
	Output       map[string]interface{} `json:"output,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Duration     time.Duration          `json:"duration"`
}

// EngineConfig 引擎配置.
type EngineConfig struct {
	// MaxFunctions 最大函数数.
	MaxFunctions int `json:"max_functions"`
	// MaxConcurrentInvocations 最大并发调用数.
	MaxConcurrentInvocations int `json:"max_concurrent_invocations"`
	// DefaultTimeoutS 默认超时秒数.
	DefaultTimeoutS int `json:"default_timeout_s"`
	// DefaultMemoryMB 默认内存限制.
	DefaultMemoryMB int `json:"default_memory_mb"`
	// LogRetentionDays 日志保留天数.
	LogRetentionDays int `json:"log_retention_days"`
	// EnableMetrics 是否启用指标.
	EnableMetrics bool `json:"enable_metrics"`
}

// DefaultEngineConfig 返回默认引擎配置.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxFunctions:             100,
		MaxConcurrentInvocations: 50,
		DefaultTimeoutS:          30,
		DefaultMemoryMB:          128,
		LogRetentionDays:         7,
		EnableMetrics:            true,
	}
}
