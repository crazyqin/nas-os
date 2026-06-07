// Package fileactivity 提供文件活动监控功能
// events.go - 文件活动事件定义
// 对标群晖 Active Insight 文件活动监控
package fileactivity

import (
	"time"
)

// EventType 文件活动事件类型
type EventType string

const (
	// EventCreate 文件创建
	EventCreate EventType = "create"
	// EventModify 文件修改
	EventModify EventType = "modify"
	// EventDelete 文件删除
	EventDelete EventType = "delete"
	// EventAccess 文件访问（读取）
	EventAccess EventType = "access"
	// EventRename 文件重命名
	EventRename EventType = "rename"
	// EventMove 文件移动
	EventMove EventType = "move"
	// EventCopy 文件复制
	EventCopy EventType = "copy"
	// EventPermissionChange 权限变更
	EventPermissionChange EventType = "permission_change"
)

// Severity 事件严重程度
type Severity string

const (
	// SeverityInfo 信息级别
	SeverityInfo Severity = "info"
	// SeverityWarning 警告级别
	SeverityWarning Severity = "warning"
	// SeverityCritical 严重级别
	SeverityCritical Severity = "critical"
)

// AccessType 访问类型
type AccessType string

const (
	// AccessTypeNormal 正常访问
	AccessTypeNormal AccessType = "normal"
	// AccessTypeAnomaly 异常访问
	AccessTypeAnomaly AccessType = "anomaly"
	// AccessTypeSuspicious 可疑访问
	AccessTypeSuspicious AccessType = "suspicious"
	// AccessTypeUnauthorized 未授权访问
	AccessTypeUnauthorized AccessType = "unauthorized"
)

// FileActivityEvent 文件活动事件
type FileActivityEvent struct {
	// ID 事件唯一标识
	ID string `json:"id"`

	// Type 事件类型
	Type EventType `json:"type"`

	// Timestamp 事件发生时间
	Timestamp time.Time `json:"timestamp"`

	// Path 文件路径
	Path string `json:"path"`

	// OldPath 原路径（重命名/移动事件）
	OldPath string `json:"oldPath,omitempty"`

	// FileName 文件名
	FileName string `json:"fileName"`

	// Extension 文件扩展名
	Extension string `json:"extension"`

	// Size 文件大小（字节）
	Size int64 `json:"size"`

	// IsDirectory 是否为目录
	IsDirectory bool `json:"isDirectory"`

	// UserID 用户ID
	UserID string `json:"userId"`

	// UserName 用户名
	UserName string `json:"userName"`

	// ProcessName 操作进程名
	ProcessName string `json:"processName"`

	// ProcessID 进程ID
	ProcessID int `json:"processId"`

	// ClientIP 客户端IP（远程访问）
	ClientIP string `json:"clientIp,omitempty"`

	// Protocol 访问协议（SMB/NFS/WebDAV/FTP/本地）
	Protocol string `json:"protocol,omitempty"`

	// ShareName 共享名称
	ShareName string `json:"shareName,omitempty"`

	// Severity 严重程度
	Severity Severity `json:"severity"`

	// AccessType 访问类型
	AccessType AccessType `json:"accessType"`

	// AnomalyScore 异常评分（0-100）
	AnomalyScore int `json:"anomalyScore"`

	// AnomalyReasons 异常原因列表
	AnomalyReasons []string `json:"anomalyReasons,omitempty"`

	// Metadata 附加元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// FileActivityFilter 文件活动过滤条件
type FileActivityFilter struct {
	// UserID 按用户过滤
	UserID string `json:"userId,omitempty"`

	// UserName 按用户名过滤
	UserName string `json:"userName,omitempty"`

	// Path 按路径过滤（前缀匹配）
	Path string `json:"path,omitempty"`

	// FileName 按文件名过滤（精确匹配）
	FileName string `json:"fileName,omitempty"`

	// Extension 按扩展名过滤
	Extension string `json:"extension,omitempty"`

	// EventTypes 按事件类型过滤
	EventTypes []EventType `json:"eventTypes,omitempty"`

	// Severity 按严重程度过滤
	Severity Severity `json:"severity,omitempty"`

	// AccessType 按访问类型过滤
	AccessType AccessType `json:"accessType,omitempty"`

	// StartTime 开始时间
	StartTime *time.Time `json:"startTime,omitempty"`

	// EndTime 结束时间
	EndTime *time.Time `json:"endTime,omitempty"`

	// ProcessName 按进程名过滤
	ProcessName string `json:"processName,omitempty"`

	// Protocol 按协议过滤
	Protocol string `json:"protocol,omitempty"`

	// ShareName 按共享名过滤
	ShareName string `json:"shareName,omitempty"`

	// MinSize 最小文件大小
	MinSize int64 `json:"minSize,omitempty"`

	// MaxSize 最大文件大小
	MaxSize int64 `json:"maxSize,omitempty"`

	// Limit 返回数量限制
	Limit int `json:"limit,omitempty"`

	// Offset 偏移量
	Offset int `json:"offset,omitempty"`
}

// SecurityAlert 安全告警
type SecurityAlert struct {
	// ID 告警ID
	ID string `json:"id"`

	// Timestamp 告警时间
	Timestamp time.Time `json:"timestamp"`

	// AlertType 告警类型
	AlertType string `json:"alertType"`

	// Severity 告警严重程度
	Severity Severity `json:"severity"`

	// Title 告警标题
	Title string `json:"title"`

	// Description 告警描述
	Description string `json:"description"`

	// RelatedEvents 关联事件ID列表
	RelatedEvents []string `json:"relatedEvents"`

	// UserID 相关用户ID
	UserID string `json:"userId"`

	// UserName 相关用户名
	UserName string `json:"userName"`

	// AffectedPath 受影响路径
	AffectedPath string `json:"affectedPath"`

	// RiskScore 风险评分（0-100）
	RiskScore int `json:"riskScore"`

	// IsHandled 是否已处理
	IsHandled bool `json:"isHandled"`

	// HandledBy 处理人
	HandledBy string `json:"handledBy,omitempty"`

	// HandledAt 处理时间
	HandledAt *time.Time `json:"handledAt,omitempty"`

	// Recommendation 处理建议
	Recommendation string `json:"recommendation,omitempty"`

	// Actions 建议的响应动作
	Actions []AlertAction `json:"actions,omitempty"`
}

// AlertAction 告警响应动作
type AlertAction struct {
	// Type 动作类型
	Type string `json:"type"`

	// Description 动作描述
	Description string `json:"description"`

	// AutoExecutable 是否可自动执行
	AutoExecutable bool `json:"autoExecutable"`

	// Priority 执行优先级
	Priority int `json:"priority"`
}

// AnomalyDetectionRule 异常检测规则
type AnomalyDetectionRule struct {
	// ID 规则ID
	ID string `json:"id"`

	// Name 规则名称
	Name string `json:"name"`

	// Description 规则描述
	Description string `json:"description"`

	// Type 规则类型
	Type string `json:"type"`

	// Threshold 阈值
	Threshold int `json:"threshold"`

	// TimeWindow 时间窗口（秒）
	TimeWindow int `json:"timeWindow"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// Severity 触发的严重程度
	Severity Severity `json:"severity"`

	// Actions 触发的响应动作
	Actions []string `json:"actions"`

	// Conditions 触发条件
	Conditions []RuleCondition `json:"conditions"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	// Field 字段名
	Field string `json:"field"`

	// Operator 操作符（eq, ne, gt, lt, gte, lte, contains, regex）
	Operator string `json:"operator"`

	// Value 条件值
	Value interface{} `json:"value"`
}

// MonitorStatus 监控器状态
type MonitorStatus struct {
	// IsRunning 是否运行中
	IsRunning bool `json:"isRunning"`

	// StartTime 启动时间
	StartTime time.Time `json:"startTime"`

	// Uptime 运行时长（秒）
	Uptime int64 `json:"uptime"`

	// TotalEvents 总事件数
	TotalEvents int64 `json:"totalEvents"`

	// EventsToday 今日事件数
	EventsToday int64 `json:"eventsToday"`

	// AlertsToday 今日告警数
	AlertsToday int64 `json:"alertsToday"`

	// ActiveRules 活跃规则数
	ActiveRules int `json:"activeRules"`

	// WatchPaths 监控路径列表
	WatchPaths []string `json:"watchPaths"`

	// ExcludePaths 排除路径列表
	ExcludePaths []string `json:"excludePaths"`

	// LastEventTime 最后事件时间
	LastEventTime *time.Time `json:"lastEventTime,omitempty"`

	// LastAlertTime 最后告警时间
	LastAlertTime *time.Time `json:"lastAlertTime,omitempty"`

	// Statistics 详细统计
	Statistics EventStatistics `json:"statistics"`
}

// EventStatistics 事件统计
type EventStatistics struct {
	// ByType 按类型统计
	ByType map[EventType]int64 `json:"byType"`

	// ByUser 按用户统计
	ByUser map[string]int64 `json:"byUser"`

	// ByPath 按路径统计
	ByPath map[string]int64 `json:"byPath"`

	// BySeverity 按严重程度统计
	BySeverity map[Severity]int64 `json:"bySeverity"`

	// ByAccessType 按访问类型统计
	ByAccessType map[AccessType]int64 `json:"byAccessType"`

	// ByProtocol 按协议统计
	ByProtocol map[string]int64 `json:"byProtocol"`

	// AnomalyEvents 异常事件数
	AnomalyEvents int64 `json:"anomalyEvents"`

	// SuspiciousFiles 可疑文件数
	SuspiciousFiles int64 `json:"suspiciousFiles"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	// Enabled 是否启用监控
	Enabled bool `json:"enabled"`

	// WatchPaths 监控路径列表
	WatchPaths []string `json:"watchPaths"`

	// ExcludePaths 排除路径列表（不监控）
	ExcludePaths []string `json:"excludePaths"`

	// MaxEventsBuffer 最大事件缓冲数量
	MaxEventsBuffer int `json:"maxEventsBuffer"`

	// EventRetention 事件保留时间（小时）
	EventRetention int `json:"eventRetention"`

	// AnomalyDetectionEnabled 是否启用异常检测
	AnomalyDetectionEnabled bool `json:"anomalyDetectionEnabled"`

	// AutoAlertEnabled 是否自动生成告警
	AutoAlertEnabled bool `json:"autoAlertEnabled"`

	// LogAllAccess 是否记录所有访问事件
	LogAllAccess bool `json:"logAllAccess"`

	// SamplingRate 采样率（1-100，仅记录指定比例事件）
	SamplingRate int `json:"samplingRate"`

	// NotificationChannels 通知渠道
	NotificationChannels []string `json:"notificationChannels"`

	// QuotaWarningThreshold 配额告警阈值（百分比）
	QuotaWarningThreshold int `json:"quotaWarningThreshold"`

	// Rules 异常检测规则列表
	Rules []AnomalyDetectionRule `json:"rules"`
}

// MonitorConfigRequest 配置更新请求
type MonitorConfigRequest struct {
	// Enabled 是否启用监控
	Enabled bool `json:"enabled"`

	// WatchPaths 监控路径列表
	WatchPaths []string `json:"watchPaths"`

	// ExcludePaths 排除路径列表
	ExcludePaths []string `json:"excludePaths"`

	// AnomalyDetectionEnabled 是否启用异常检测
	AnomalyDetectionEnabled bool `json:"anomalyDetectionEnabled"`

	// AutoAlertEnabled 是否自动生成告警
	AutoAlertEnabled bool `json:"autoAlertEnabled"`

	// LogAllAccess 是否记录所有访问事件
	LogAllAccess bool `json:"logAllAccess"`

	// SamplingRate 采样率
	SamplingRate int `json:"samplingRate"`

	// NotificationChannels 通知渠道
	NotificationChannels []string `json:"notificationChannels"`
}

// AlertQueryRequest 告警查询请求
type AlertQueryRequest struct {
	// Severity 严重程度过滤
	Severity Severity `json:"severity,omitempty"`

	// AlertType 告警类型过滤
	AlertType string `json:"alertType,omitempty"`

	// UserID 用户ID过滤
	UserID string `json:"userId,omitempty"`

	// Path 路径过滤
	Path string `json:"path,omitempty"`

	// IsHandled 是否已处理
	IsHandled *bool `json:"isHandled,omitempty"`

	// StartTime 开始时间
	StartTime *time.Time `json:"startTime,omitempty"`

	// EndTime 结束时间
	EndTime *time.Time `json:"endTime,omitempty"`

	// Limit 返回数量限制
	Limit int `json:"limit,omitempty"`

	// Offset 偏移量
	Offset int `json:"offset,omitempty"`
}

// AlertHandleRequest 告警处理请求
type AlertHandleRequest struct {
	// HandledBy 处理人
	HandledBy string `json:"handledBy"`

	// Action 执行的动作
	Action string `json:"action"`

	// Notes 处理备注
	Notes string `json:"notes"`
}

// RuleCreateRequest 规则创建请求
type RuleCreateRequest struct {
	// Name 规则名称
	Name string `json:"name" binding:"required"`

	// Description 规则描述
	Description string `json:"description"`

	// Type 规则类型
	Type string `json:"type" binding:"required"`

	// Threshold 阈值
	Threshold int `json:"threshold"`

	// TimeWindow 时间窗口（秒）
	TimeWindow int `json:"timeWindow"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// Severity 严重程度
	Severity Severity `json:"severity"`

	// Actions 响应动作
	Actions []string `json:"actions"`

	// Conditions 触发条件
	Conditions []RuleCondition `json:"conditions"`
}
