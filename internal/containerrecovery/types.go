// Package containerrecovery 提供容器自动恢复引擎功能
package containerrecovery

import (
	"sync"
	"time"
)

// ========== 健康检查策略 ==========

// HealthCheckType 健康检查类型.
type HealthCheckType string

const (
	HealthCheckHTTP         HealthCheckType = "http"          // HTTP 健康检查
	HealthCheckTCP          HealthCheckType = "tcp"           // TCP 端口检查
	HealthCheckCommand      HealthCheckType = "command"       // 命令执行检查
	HealthCheckContainer    HealthCheckType = "container"     // 容器状态检查
)

// HealthCheckConfig 健康检查配置.
type HealthCheckConfig struct {
	Type               HealthCheckType    `json:"type"`                          // 检查类型
	HTTPPath           string             `json:"http_path,omitempty"`           // HTTP 检查路径
	HTTPPort           int                `json:"http_port,omitempty"`           // HTTP 检查端口
	HTTPExpectedStatus int                `json:"http_expected_status,omitempty"` // 期望 HTTP 状态码
	TCPHost            string             `json:"tcp_host,omitempty"`            // TCP 检查主机
	TCPPort            int                `json:"tcp_port,omitempty"`            // TCP 检查端口
	Command            string             `json:"command,omitempty"`             // 检查命令
	CommandArgs        []string           `json:"command_args,omitempty"`        // 命令参数
	Interval           time.Duration      `json:"interval"`                      // 检查间隔
	Timeout            time.Duration      `json:"timeout"`                       // 检查超时
	HealthyThreshold   int                `json:"healthy_threshold"`             // 连续健康次数阈值
	UnhealthyThreshold int                `json:"unhealthy_threshold"`           // 连续不健康次数阈值
}

// HealthStatus 健康状态.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
	HealthStatusStarting  HealthStatus = "starting"
)

// ========== 恢复策略 ==========

// RecoveryAction 恢复动作类型.
type RecoveryAction string

const (
	RecoveryActionRestart   RecoveryAction = "restart"    // 重启容器
	RecoveryActionNotify    RecoveryAction = "notify"     // 发送通知
	RecoveryActionRollback  RecoveryAction = "rollback"   // 回滚到上一版本
	RecoveryActionScaleUp   RecoveryAction = "scale_up"   // 扩容
)

// RecoveryStrategy 恢复策略配置.
type RecoveryStrategy struct {
	Action            RecoveryAction `json:"action"`                        // 恢复动作
	MaxRetries        int            `json:"max_retries"`                   // 最大重试次数
	InitialBackoff    time.Duration  `json:"initial_backoff"`               // 初始退避时间
	MaxBackoff        time.Duration  `json:"max_backoff"`                   // 最大退避时间
	BackoffMultiplier float64        `json:"backoff_multiplier"`            // 退避倍数
	CooldownPeriod    time.Duration  `json:"cooldown_period"`               // 冷却期
	NotifyOnFailure   bool           `json:"notify_on_failure"`             // 失败时通知
	WebhookURL        string         `json:"webhook_url,omitempty"`         // 通知 Webhook URL
}

// DefaultRecoveryStrategy 返回默认恢复策略.
func DefaultRecoveryStrategy() RecoveryStrategy {
	return RecoveryStrategy{
		Action:            RecoveryActionRestart,
		MaxRetries:        3,
		InitialBackoff:    5 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
		CooldownPeriod:    10 * time.Minute,
		NotifyOnFailure:   true,
	}
}

// ========== 故障模式 ==========

// FailureMode 故障模式.
type FailureMode string

const (
	FailureModeOOMKilled         FailureMode = "oom_killed"           // 内存溢出被杀
	FailureModeCrashLoopBackOff  FailureMode = "crash_loop_backoff"   // 崩溃循环
	FailureModeImagePullBackOff  FailureMode = "image_pull_backoff"   // 镜像拉取失败
	FailureModeNetworkError      FailureMode = "network_error"        // 网络错误
	FailureModeDiskPressure      FailureMode = "disk_pressure"        // 磁盘压力
	FailureModeUnknown           FailureMode = "unknown"              // 未知故障
)

// FailureInfo 故障信息.
type FailureInfo struct {
	Mode      FailureMode `json:"mode"`                // 故障模式
	Container string      `json:"container"`           // 容器名称
	Message   string      `json:"message"`             // 故障描述
	Timestamp time.Time   `json:"timestamp"`           // 故障时间
	Details   string      `json:"details,omitempty"`   // 详细信息
}

// ========== 依赖管理 ==========

// ContainerDependency 容器依赖关系.
type ContainerDependency struct {
	Name         string   `json:"name"`                    // 容器名称
	Dependencies []string `json:"dependencies,omitempty"`  // 依赖的容器列表
	Priority     int      `json:"priority"`                // 恢复优先级（数值越小优先级越高）
}

// DependencyGraph 依赖关系图.
type DependencyGraph struct {
	mu          sync.RWMutex
	containers  map[string]*ContainerDependency
	dependents  map[string][]string // 被依赖关系（反向索引）
}

// ========== 恢复钩子 ==========

// HookPhase 钩子执行阶段.
type HookPhase string

const (
	HookPhasePreRecovery  HookPhase = "pre_recovery"   // 恢复前
	HookPhasePostRecovery HookPhase = "post_recovery"  // 恢复后
)

// RecoveryHook 恢复钩子配置.
type RecoveryHook struct {
	Name     string    `json:"name"`      // 钩子名称
	Phase    HookPhase `json:"phase"`     // 执行阶段
	Command  string    `json:"command"`   // 执行命令
	Args     []string  `json:"args,omitempty"` // 命令参数
	Timeout  time.Duration `json:"timeout"` // 执行超时
	ContinueOnError bool  `json:"continue_on_error"` // 失败时是否继续
}

// ========== 恢复记录 ==========

// RecoveryStatus 恢复状态.
type RecoveryStatus string

const (
	RecoveryStatusPending   RecoveryStatus = "pending"    // 等待恢复
	RecoveryStatusRunning   RecoveryStatus = "running"    // 恢复中
	RecoveryStatusSuccess   RecoveryStatus = "success"    // 恢复成功
	RecoveryStatusFailed    RecoveryStatus = "failed"     // 恢复失败
	RecoveryStatusSkipped   RecoveryStatus = "skipped"    // 跳过恢复
)

// RecoveryRecord 恢复记录.
type RecoveryRecord struct {
	ID           string         `json:"id"`                      // 记录 ID
	Container    string         `json:"container"`               // 容器名称
	Action       RecoveryAction `json:"action"`                  // 恢复动作
	Status       RecoveryStatus `json:"status"`                  // 恢复状态
	FailureMode  FailureMode    `json:"failure_mode"`            // 故障模式
	Reason       string         `json:"reason"`                  // 恢复原因
	RetryCount   int            `json:"retry_count"`             // 重试次数
	MaxRetries   int            `json:"max_retries"`             // 最大重试次数
	StartTime    time.Time      `json:"start_time"`              // 开始时间
	EndTime      *time.Time     `json:"end_time,omitempty"`      // 结束时间
	Duration     time.Duration  `json:"duration"`                // 持续时间
	ErrorMessage string         `json:"error_message,omitempty"` // 错误信息
	HooksExecuted []HookResult  `json:"hooks_executed,omitempty"` // 已执行钩子
}

// HookResult 钩子执行结果.
type HookResult struct {
	Name      string        `json:"name"`       // 钩子名称
	Phase     HookPhase     `json:"phase"`      // 执行阶段
	Success   bool          `json:"success"`    // 是否成功
	Output    string        `json:"output,omitempty"` // 输出
	Error     string        `json:"error,omitempty"`  // 错误
	Duration  time.Duration `json:"duration"`   // 执行时长
}

// ========== 恢复统计 ==========

// RecoveryStats 恢复统计.
type RecoveryStats struct {
	TotalRecoveries    int64         `json:"total_recoveries"`     // 总恢复次数
	SuccessfulCount    int64         `json:"successful_count"`     // 成功次数
	FailedCount        int64         `json:"failed_count"`         // 失败次数
	SuccessRate        float64       `json:"success_rate"`         // 成功率
	MTTR               time.Duration `json:"mttr"`                 // 平均恢复时间
	AvgRetries         float64       `json:"avg_retries"`          // 平均重试次数
	FailureFrequency   map[string]int64 `json:"failure_frequency"` // 故障频率统计
	ContainerStats     map[string]*ContainerStats `json:"container_stats"` // 容器级别统计
	LastUpdated        time.Time     `json:"last_updated"`         // 最后更新时间
}

// ContainerStats 单容器统计.
type ContainerStats struct {
	Container        string        `json:"container"`          // 容器名称
	TotalRecoveries  int64         `json:"total_recoveries"`   // 总恢复次数
	SuccessfulCount  int64         `json:"successful_count"`   // 成功次数
	FailedCount      int64         `json:"failed_count"`       // 失败次数
	LastRecovery     *time.Time    `json:"last_recovery,omitempty"` // 最后恢复时间
	LastFailureMode  FailureMode   `json:"last_failure_mode,omitempty"` // 最后故障模式
	AvgRecoveryTime  time.Duration `json:"avg_recovery_time"`  // 平均恢复时间
}

// ========== 容器配置 ==========

// ContainerConfig 容器恢复配置.
type ContainerConfig struct {
	ContainerName   string            `json:"container_name"`              // 容器名称
	Enabled         bool              `json:"enabled"`                     // 是否启用恢复
	HealthCheck     HealthCheckConfig `json:"health_check"`                // 健康检查配置
	Strategy        RecoveryStrategy  `json:"strategy"`                    // 恢复策略
	Dependencies    []string          `json:"dependencies,omitempty"`      // 依赖容器
	Priority        int               `json:"priority"`                    // 恢复优先级
	Hooks           []RecoveryHook    `json:"hooks,omitempty"`             // 恢复钩子
	Labels          map[string]string `json:"labels,omitempty"`            // 标签
}

// ========== 告警 ==========

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertLevelInfo    AlertLevel = "info"
	AlertLevelWarning AlertLevel = "warning"
	AlertLevelError   AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert 告警消息.
type Alert struct {
	Level     AlertLevel `json:"level"`              // 告警级别
	Container string     `json:"container"`          // 容器名称
	Title     string     `json:"title"`              // 告警标题
	Message   string     `json:"message"`            // 告警内容
	Timestamp time.Time  `json:"timestamp"`          // 告警时间
	Details   string     `json:"details,omitempty"`  // 详细信息
}

// ========== 引擎配置 ==========

// EngineConfig 引擎配置.
type EngineConfig struct {
	Enabled            bool          `json:"enabled"`                        // 是否启用
	Concurrency        int           `json:"concurrency"`                    // 并发恢复数
	HealthCheckInterval time.Duration `json:"health_check_interval"`         // 全局健康检查间隔
	RecoveryTimeout    time.Duration `json:"recovery_timeout"`              // 单次恢复超时
	HistoryLimit       int           `json:"history_limit"`                 // 历史记录保留数量
	WebhookURL         string        `json:"webhook_url,omitempty"`         // 全局通知 Webhook
}

// DefaultEngineConfig 返回默认引擎配置.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		Enabled:             true,
		Concurrency:         3,
		HealthCheckInterval: 30 * time.Second,
		RecoveryTimeout:     5 * time.Minute,
		HistoryLimit:        1000,
	}
}

// ========== 存储接口 ==========

// Store 持久化存储接口.
type Store interface {
	// SaveRecord 保存恢复记录.
	SaveRecord(record *RecoveryRecord) error
	// GetRecords 获取恢复记录.
	GetRecords(container string, limit int) ([]*RecoveryRecord, error)
	// GetStats 获取恢复统计.
	GetStats() (*RecoveryStats, error)
	// UpdateStats 更新统计信息.
	UpdateStats(record *RecoveryRecord) error
	// Cleanup 清理旧记录.
	Cleanup(olderThan time.Duration) error
}

// ========== 告警发送器接口 ==========

// AlertSender 告警发送器接口.
type AlertSender interface {
	// Send 发送告警.
	Send(alert *Alert) error
}

// ========== 容器操作接口 ==========

// ContainerOperator 容器操作接口.
type ContainerOperator interface {
	// Restart 重启容器.
	Rename(container string) error
	// Stop 停止容器.
	Stop(container string) error
	// Start 启动容器.
	Start(container string) error
	// GetStatus 获取容器状态.
	GetStatus(container string) (string, error)
	// GetHealthCheck 获取容器健康状态.
	GetHealthCheck(container string) (HealthStatus, error)
	// Rollback 回滚到上一版本.
	Rollback(container string) error
	// Scale 扩容.
	Scale(container string, count int) error
}
