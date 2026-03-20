// Package quota 提供存储配额管理功能
package quota

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

// 配额相关错误
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

// Type 配额类型
type Type string

// 配额类型常量
const (
	TypeUser      Type = "user"      // 用户配额
	TypeGroup     Type = "group"     // 用户组配额
	TypeDirectory Type = "directory" // 目录配额
)

// 向后兼容的常量别名
const (
	QuotaTypeUser      = TypeUser      // 用户配额
	QuotaTypeGroup     = TypeGroup     // 用户组配额
	QuotaTypeDirectory = TypeDirectory // 目录配额
)

// QuotaType 是 Type 的别名，保留用于向后兼容
type QuotaType = Type

// Quota 存储配额定义
type Quota struct {
	ID         string `json:"id"`
	Type       Type   `json:"type"`        // user 或 group
	TargetID   string    `json:"target_id"`   // 用户名或组名
	TargetName string    `json:"target_name"` // 显示名称
	VolumeName string    `json:"volume_name"` // 适用卷名（空表示全局）
	Path       string    `json:"path"`        // 限制路径
	HardLimit  uint64    `json:"hard_limit"`  // 硬限制（字节）
	SoftLimit  uint64    `json:"soft_limit"`  // 软限制（字节，超限告警）
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Usage 配额使用情况
type Usage struct {
	QuotaID      string    `json:"quota_id"`
	Type         Type      `json:"type"`
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

// QuotaUsage 是 Usage 的别名，保留用于向后兼容
type QuotaUsage = Usage

// ========== 告警类型 ==========

// AlertType 告警类型
type AlertType string

// 告警类型常量
const (
	AlertTypeSoftLimit AlertType = "soft_limit" // 软限制告警
	AlertTypeHardLimit AlertType = "hard_limit" // 硬限制告警
	AlertTypeCleanup   AlertType = "cleanup"    // 自动清理告警
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	AlertSeverityInfo      AlertSeverity = "info"      // 信息
	AlertSeverityWarning   AlertSeverity = "warning"   // 警告
	AlertSeverityCritical  AlertSeverity = "critical"  // 严重
	AlertSeverityEmergency AlertSeverity = "emergency" // 紧急
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive    AlertStatus = "active"    // 活跃
	AlertStatusResolved  AlertStatus = "resolved"  // 已解决
	AlertStatusSilenced  AlertStatus = "silenced"  // 静默
	AlertStatusEscalated AlertStatus = "escalated" // 已升级
)

// Alert 配额告警
type Alert struct {
	ID              string        `json:"id"`
	QuotaID         string        `json:"quota_id"`
	Type            AlertType     `json:"type"`
	Severity        AlertSeverity `json:"severity"` // 严重级别
	Status          AlertStatus   `json:"status"`
	TargetID        string        `json:"target_id"`
	TargetName      string        `json:"target_name"`
	VolumeName      string        `json:"volume_name"`
	Path            string        `json:"path"`
	UsedBytes       uint64        `json:"used_bytes"`
	LimitBytes      uint64        `json:"limit_bytes"`
	UsagePercent    float64       `json:"usage_percent"`
	Threshold       float64       `json:"threshold"` // 触发阈值百分比
	Message         string        `json:"message"`
	CreatedAt       time.Time     `json:"created_at"`
	ResolvedAt      *time.Time    `json:"resolved_at,omitempty"`
	EscalatedAt     *time.Time    `json:"escalated_at,omitempty"` // 升级时间
	EscalationLevel int           `json:"escalation_level"`       // 升级级别
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
	Username       string  `json:"username"`
	Quotas         []Usage `json:"quotas"`
	TotalLimit     uint64  `json:"total_limit"`
	TotalUsed      uint64  `json:"total_used"`
	TotalAvailable uint64  `json:"total_available"`
	UsagePercent   float64 `json:"usage_percent"`
}

// VolumeQuotaReport 卷配额报告详情
type VolumeQuotaReport struct {
	VolumeName   string  `json:"volume_name"`
	TotalLimit   uint64  `json:"total_limit"`
	TotalUsed    uint64  `json:"total_used"`
	TotalFree    uint64  `json:"total_free"`
	UserQuotas   []Usage `json:"user_quotas"`
	GroupQuotas  []Usage `json:"group_quotas"`
	ActiveAlerts []Alert `json:"active_alerts"`
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

// TrendStats 趋势统计
type TrendStats struct {
	QuotaID    string    `json:"quota_id"`
	TargetName string    `json:"target_name"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`

	// 使用量统计
	MinUsedBytes     uint64  `json:"min_used_bytes"`
	MaxUsedBytes     uint64  `json:"max_used_bytes"`
	AvgUsedBytes     float64 `json:"avg_used_bytes"`
	CurrentUsedBytes uint64  `json:"current_used_bytes"`

	// 百分比统计
	MinUsagePercent     float64 `json:"min_usage_percent"`
	MaxUsagePercent     float64 `json:"max_usage_percent"`
	AvgUsagePercent     float64 `json:"avg_usage_percent"`
	CurrentUsagePercent float64 `json:"current_usage_percent"`

	// 增长分析
	GrowthRate          float64    `json:"growth_rate"`                   // 字节/天
	GrowthPercent       float64    `json:"growth_percent"`                // 日增长百分比
	ProjectedDaysToFull int        `json:"projected_days_to_full"`        // 预计多少天填满
	ProjectedFullDate   *time.Time `json:"projected_full_date,omitempty"` // 预计填满日期

	// 峰值分析
	PeakTime         *time.Time `json:"peak_time,omitempty"`
	PeakUsedBytes    uint64     `json:"peak_used_bytes"`
	PeakUsagePercent float64    `json:"peak_usage_percent"`

	// 数据点数量
	DataPointCount int `json:"data_point_count"`
}

// TrendHistory 趋势历史记录（持久化用）
type TrendHistory struct {
	QuotaID    string           `json:"quota_id"`
	DataPoints []TrendDataPoint `json:"data_points"`
	LastUpdate time.Time        `json:"last_update"`
}

// TrendReportRequest 趋势报告请求
type TrendReportRequest struct {
	QuotaID     string        `json:"quota_id,omitempty"`    // 可选，不指定则返回所有
	Duration    time.Duration `json:"duration"`              // 统计周期
	Granularity time.Duration `json:"granularity,omitempty"` // 数据粒度（如每小时、每天）
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

	// 高级选项
	Recursive       bool     `json:"recursive"`        // 是否递归子目录
	ExcludePatterns []string `json:"exclude_patterns"` // 排除的文件模式
	MaxFiles        int      `json:"max_files"`        // 单次最大处理文件数
	DryRun          bool     `json:"dry_run"`          // 预览模式（不实际执行）
}

// CleanupPreview 清理预览结果
type CleanupPreview struct {
	PolicyID      string        `json:"policy_id"`
	PolicyName    string        `json:"policy_name"`
	Path          string        `json:"path"`
	TotalFiles    int           `json:"total_files"`
	TotalBytes    uint64        `json:"total_bytes"`
	Files         []CleanupFile `json:"files"`
	Warnings      []string      `json:"warnings,omitempty"`
	EstimatedTime time.Duration `json:"estimated_time"`
}

// CleanupFile 清理文件信息
type CleanupFile struct {
	Path    string     `json:"path"`
	Size    uint64     `json:"size"`
	ModTime time.Time  `json:"mod_time"`
	AccTime *time.Time `json:"acc_time,omitempty"` // 访问时间
	Reason  string     `json:"reason"`             // 匹配原因
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

	// 多级预警阈值配置
	WarningThreshold   float64 `json:"warning_threshold"`   // 警告级别阈值（默认 70%）
	CriticalThreshold  float64 `json:"critical_threshold"`  // 严重级别阈值（默认 85%）
	EmergencyThreshold float64 `json:"emergency_threshold"` // 紧急级别阈值（默认 95%）

	// 告警升级配置
	EscalationEnabled     bool          `json:"escalation_enabled"`      // 是否启用告警升级
	EscalationInterval    time.Duration `json:"escalation_interval"`     // 升级间隔（未处理多久后升级）
	MaxEscalationLevel    int           `json:"max_escalation_level"`    // 最大升级级别
	EscalationNotifyEmail bool          `json:"escalation_notify_email"` // 升级时邮件通知
	EscalationWebhookURL  string        `json:"escalation_webhook_url"`  // 升级通知 webhook
}

// AlertLevelConfig 预警级别配置
type AlertLevelConfig struct {
	Name      string        `json:"name"`      // 级别名称
	Threshold float64       `json:"threshold"` // 触发阈值（百分比）
	Severity  AlertSeverity `json:"severity"`  // 严重级别
	Message   string        `json:"message"`   // 自定义消息模板
}

// DefaultAlertLevels 默认预警级别配置
var DefaultAlertLevels = []AlertLevelConfig{
	{Name: "info", Threshold: 60, Severity: AlertSeverityInfo, Message: "存储使用已达到 %.1f%%"},
	{Name: "warning", Threshold: 70, Severity: AlertSeverityWarning, Message: "存储使用已达到 %.1f%%，请注意"},
	{Name: "critical", Threshold: 85, Severity: AlertSeverityCritical, Message: "存储使用已达到 %.1f%%，请及时处理"},
	{Name: "emergency", Threshold: 95, Severity: AlertSeverityEmergency, Message: "存储使用已达到 %.1f%%，即将超出限制"},
}
