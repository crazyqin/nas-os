// Package quota 提供存储配额管理功能
package quota

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrQuotaNotFound         = errors.New("配额不存在")
	ErrQuotaExists           = errors.New("配额已存在")
	ErrQuotaExceeded         = errors.New("超出配额限制")
	ErrUserNotFound          = errors.New("用户不存在")
	ErrGroupNotFound         = errors.New("用户组不存在")
	ErrVolumeNotFound        = errors.New("卷不存在")
	ErrInvalidLimit          = errors.New("无效的配额限制")
	ErrCleanupPolicyNotFound = errors.New("清理策略不存在")
)

// ========== 配额类型 ==========

// QuotaType 配额类型
type QuotaType string

const (
	QuotaTypeUser  QuotaType = "user"  // 用户配额
	QuotaTypeGroup QuotaType = "group" // 用户组配额
)

// Quota 存储配额定义
type Quota struct {
	ID         string    `json:"id"`
	Type       QuotaType `json:"type"`        // user 或 group
	TargetID   string    `json:"target_id"`   // 用户名或组名
	TargetName string    `json:"target_name"` // 显示名称
	VolumeName string    `json:"volume_name"` // 适用卷名（空表示全局）
	Path       string    `json:"path"`        // 限制路径
	HardLimit  uint64    `json:"hard_limit"`  // 硬限制（字节）
	SoftLimit  uint64    `json:"soft_limit"`  // 软限制（字节，超限告警）
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// QuotaUsage 配额使用情况
type QuotaUsage struct {
	QuotaID      string    `json:"quota_id"`
	Type         QuotaType `json:"type"`
	TargetID     string    `json:"target_id"`
	TargetName   string    `json:"target_name"`
	VolumeName   string    `json:"volume_name"`
	Path         string    `json:"path"`
	HardLimit    uint64    `json:"hard_limit"`
	SoftLimit    uint64    `json:"soft_limit"`
	UsedBytes    uint64    `json:"used_bytes"`
	Available    uint64    `json:"available"`
	UsagePercent float64   `json:"usage_percent"`
	IsOverSoft   bool      `json:"is_over_soft"` // 是否超过软限制
	IsOverHard   bool      `json:"is_over_hard"` // 是否超过硬限制
	LastChecked  time.Time `json:"last_checked"`
}

// ========== 告警类型 ==========

// AlertType 告警类型
type AlertType string

const (
	AlertTypeSoftLimit AlertType = "soft_limit" // 软限制告警
	AlertTypeHardLimit AlertType = "hard_limit" // 硬限制告警
	AlertTypeCleanup   AlertType = "cleanup"    // 自动清理告警
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive   AlertStatus = "active"   // 活跃
	AlertStatusResolved AlertStatus = "resolved" // 已解决
	AlertStatusSilenced AlertStatus = "silenced" // 静默
)

// Alert 配额告警
type Alert struct {
	ID           string      `json:"id"`
	QuotaID      string      `json:"quota_id"`
	Type         AlertType   `json:"type"`
	Status       AlertStatus `json:"status"`
	TargetID     string      `json:"target_id"`
	TargetName   string      `json:"target_name"`
	VolumeName   string      `json:"volume_name"`
	Path         string      `json:"path"`
	UsedBytes    uint64      `json:"used_bytes"`
	LimitBytes   uint64      `json:"limit_bytes"`
	UsagePercent float64     `json:"usage_percent"`
	Message      string      `json:"message"`
	CreatedAt    time.Time   `json:"created_at"`
	ResolvedAt   *time.Time  `json:"resolved_at,omitempty"`
}

// ========== 清理策略 ==========

// CleanupPolicyType 清理策略类型
type CleanupPolicyType string

const (
	CleanupPolicyAge     CleanupPolicyType = "age"     // 按文件年龄
	CleanupPolicySize    CleanupPolicyType = "size"    // 按文件大小
	CleanupPolicyPattern CleanupPolicyType = "pattern" // 按文件名模式
	CleanupPolicyQuota   CleanupPolicyType = "quota"   // 按配额比例
	CleanupPolicyAccess  CleanupPolicyType = "access"  // 按访问时间
)

// CleanupAction 清理动作
type CleanupAction string

const (
	CleanupActionDelete  CleanupAction = "delete"  // 删除
	CleanupActionArchive CleanupAction = "archive" // 归档
	CleanupActionMove    CleanupAction = "move"    // 移动
)

// CleanupPolicy 自动清理策略
type CleanupPolicy struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	VolumeName string            `json:"volume_name"`
	Path       string            `json:"path"`
	Type       CleanupPolicyType `json:"type"`
	Action     CleanupAction     `json:"action"`
	Enabled    bool              `json:"enabled"`

	// 策略参数
	MaxAge       int      `json:"max_age,omitempty"`        // 最大保留天数（age 类型）
	MinSize      uint64   `json:"min_size,omitempty"`       // 最小文件大小字节（size 类型）
	Patterns     []string `json:"patterns,omitempty"`       // 文件名模式（pattern 类型）
	QuotaPercent float64  `json:"quota_percent,omitempty"`  // 触发阈值（quota 类型）
	MaxAccessAge int      `json:"max_access_age,omitempty"` // 最大未访问天数（access 类型）

	// 归档/移动目标
	ArchivePath string `json:"archive_path,omitempty"` // 归档目标路径
	MovePath    string `json:"move_path,omitempty"`    // 移动目标路径

	// 执行计划
	Schedule      string `json:"schedule,omitempty"`       // cron 表达式
	RetentionDays int    `json:"retention_days,omitempty"` // 保留天数（归档后）

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CleanupTask 清理任务执行记录
type CleanupTask struct {
	ID             string            `json:"id"`
	PolicyID       string            `json:"policy_id"`
	PolicyName     string            `json:"policy_name"`
	VolumeName     string            `json:"volume_name"`
	Path           string            `json:"path"`
	Status         CleanupTaskStatus `json:"status"`
	StartedAt      time.Time         `json:"started_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	FilesProcessed int               `json:"files_processed"`
	BytesFreed     uint64            `json:"bytes_freed"`
	Errors         []string          `json:"errors,omitempty"`
}

// CleanupTaskStatus 清理任务状态
type CleanupTaskStatus string

const (
	CleanupTaskRunning   CleanupTaskStatus = "running"
	CleanupTaskCompleted CleanupTaskStatus = "completed"
	CleanupTaskFailed    CleanupTaskStatus = "failed"
	CleanupTaskCancelled CleanupTaskStatus = "cancelled"
)

// ========== 配额报告 ==========

// ReportType 报告类型
type ReportType string

const (
	ReportTypeSummary ReportType = "summary" // 汇总报告
	ReportTypeUser    ReportType = "user"    // 用户配额报告
	ReportTypeGroup   ReportType = "group"   // 用户组配额报告
	ReportTypeVolume  ReportType = "volume"  // 卷配额报告
	ReportTypeTrend   ReportType = "trend"   // 趋势报告
)

// ReportFormat 报告格式
type ReportFormat string

const (
	ReportFormatJSON ReportFormat = "json"
	ReportFormatCSV  ReportFormat = "csv"
	ReportFormatHTML ReportFormat = "html"
)

// Report 配额报告
type Report struct {
	ID          string        `json:"id"`
	Type        ReportType    `json:"type"`
	Format      ReportFormat  `json:"format"`
	GeneratedAt time.Time     `json:"generated_at"`
	Period      ReportPeriod  `json:"period"`
	Summary     ReportSummary `json:"summary"`
	Details     interface{}   `json:"details,omitempty"`
}

// ReportPeriod 报告时间范围
type ReportPeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalQuotas     int     `json:"total_quotas"`
	TotalLimitBytes uint64  `json:"total_limit_bytes"`
	TotalUsedBytes  uint64  `json:"total_used_bytes"`
	TotalFreeBytes  uint64  `json:"total_free_bytes"`
	AverageUsage    float64 `json:"average_usage"`
	OverSoftLimit   int     `json:"over_soft_limit"`
	OverHardLimit   int     `json:"over_hard_limit"`
	ActiveAlerts    int     `json:"active_alerts"`
	CleanupTasksRun int     `json:"cleanup_tasks_run"`
	BytesCleaned    uint64  `json:"bytes_cleaned"`
}

// UserQuotaReport 用户配额报告详情
type UserQuotaReport struct {
	Username       string       `json:"username"`
	Quotas         []QuotaUsage `json:"quotas"`
	TotalLimit     uint64       `json:"total_limit"`
	TotalUsed      uint64       `json:"total_used"`
	TotalAvailable uint64       `json:"total_available"`
	UsagePercent   float64      `json:"usage_percent"`
}

// VolumeQuotaReport 卷配额报告详情
type VolumeQuotaReport struct {
	VolumeName   string       `json:"volume_name"`
	TotalLimit   uint64       `json:"total_limit"`
	TotalUsed    uint64       `json:"total_used"`
	TotalFree    uint64       `json:"total_free"`
	UserQuotas   []QuotaUsage `json:"user_quotas"`
	GroupQuotas  []QuotaUsage `json:"group_quotas"`
	ActiveAlerts []Alert      `json:"active_alerts"`
}

// TrendDataPoint 趋势数据点
type TrendDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedBytes    uint64    `json:"used_bytes"`
	UsagePercent float64   `json:"usage_percent"`
}

// QuotaTrend 配额趋势
type QuotaTrend struct {
	QuotaID             string           `json:"quota_id"`
	TargetName          string           `json:"target_name"`
	DataPoints          []TrendDataPoint `json:"data_points"`
	GrowthRate          float64          `json:"growth_rate"`            // 字节/天
	ProjectedDaysToFull int              `json:"projected_days_to_full"` // 预计多少天填满
}

// ========== 输入结构 ==========

// QuotaInput 创建/更新配额输入
type QuotaInput struct {
	Type       QuotaType `json:"type" binding:"required"`
	TargetID   string    `json:"target_id" binding:"required"`
	VolumeName string    `json:"volume_name"`
	Path       string    `json:"path"`
	HardLimit  uint64    `json:"hard_limit" binding:"required"`
	SoftLimit  uint64    `json:"soft_limit"`
}

// CleanupPolicyInput 创建/更新清理策略输入
type CleanupPolicyInput struct {
	Name          string            `json:"name" binding:"required"`
	VolumeName    string            `json:"volume_name" binding:"required"`
	Path          string            `json:"path"`
	Type          CleanupPolicyType `json:"type" binding:"required"`
	Action        CleanupAction     `json:"action" binding:"required"`
	Enabled       bool              `json:"enabled"`
	MaxAge        int               `json:"max_age"`
	MinSize       uint64            `json:"min_size"`
	Patterns      []string          `json:"patterns"`
	QuotaPercent  float64           `json:"quota_percent"`
	MaxAccessAge  int               `json:"max_access_age"`
	ArchivePath   string            `json:"archive_path"`
	MovePath      string            `json:"move_path"`
	Schedule      string            `json:"schedule"`
	RetentionDays int               `json:"retention_days"`
}

// ReportRequest 报告请求
type ReportRequest struct {
	Type       ReportType   `json:"type" binding:"required"`
	Format     ReportFormat `json:"format"`
	StartTime  *time.Time   `json:"start_time"`
	EndTime    *time.Time   `json:"end_time"`
	VolumeName string       `json:"volume_name"`
	UserID     string       `json:"user_id"`
	GroupID    string       `json:"group_id"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled            bool          `json:"enabled"`
	SoftLimitThreshold float64       `json:"soft_limit_threshold"` // 软限制告警阈值（百分比）
	HardLimitThreshold float64       `json:"hard_limit_threshold"` // 硬限制告警阈值（百分比）
	CheckInterval      time.Duration `json:"check_interval"`
	NotifyEmail        bool          `json:"notify_email"`
	NotifyWebhook      bool          `json:"notify_webhook"`
	WebhookURL         string        `json:"webhook_url"`
	SilenceDuration    time.Duration `json:"silence_duration"`
}
