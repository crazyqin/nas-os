// Package activityfeed 实现NAS系统的统一活动流模块。
// 它聚合所有NAS服务的活动，提供智能过滤、摘要生成、
// Webhook订阅和活动导出功能。
package activityfeed

import (
	"time"
)

// Severity 表示活动的严重级别。
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// ServiceType 表示产生活动的NAS服务类型。
type ServiceType string

const (
	ServiceFileOps    ServiceType = "file_ops"
	ServiceUserAuth   ServiceType = "user_auth"
	ServiceSystem     ServiceType = "system"
	ServiceBackup     ServiceType = "backup"
	ServiceNetwork    ServiceType = "network"
	ServiceStorage    ServiceType = "storage"
	ServiceDocker     ServiceType = "docker"
	ServiceScheduled  ServiceType = "scheduled"
	ServiceSecurity   ServiceType = "security"
	ServiceOther      ServiceType = "other"
)

// ActivityActor 表示执行活动的实体（用户或系统组件）。
type ActivityActor struct {
	// ID 是执行者的唯一标识符。
	ID string `json:"id"`
	// Name 是执行者的显示名称。
	Name string `json:"name"`
	// Type 是执行者类型（user, system, service）。
	Type string `json:"type"`
	// IP 是执行者的IP地址（可选）。
	IP string `json:"ip,omitempty"`
}

// Activity 表示NAS系统中的一条活动记录。
type Activity struct {
	// ID 是活动的唯一标识符。
	ID string `json:"id"`
	// Timestamp 是活动发生的时间。
	Timestamp time.Time `json:"timestamp"`
	// Service 是产生此活动的服务类型。
	Service ServiceType `json:"service"`
	// Action 是具体的操作名称。
	Action string `json:"action"`
	// Description 是活动的可读描述。
	Description string `json:"description"`
	// Severity 是活动的严重级别。
	Severity Severity `json:"severity"`
	// Actor 是执行此活动的实体。
	Actor ActivityActor `json:"actor"`
	// Resource 是活动关联的资源路径或标识。
	Resource string `json:"resource,omitempty"`
	// Metadata 包含活动的附加键值对数据。
	Metadata map[string]string `json:"metadata,omitempty"`
	// RelatedIDs 包含关联活动的ID列表。
	RelatedIDs []string `json:"related_ids,omitempty"`
	// CreatedAt 是记录创建的时间。
	CreatedAt time.Time `json:"created_at"`
}

// ActivityFilter 定义查询活动的过滤条件。
type ActivityFilter struct {
	// Services 过滤指定服务类型的活动。
	Services []ServiceType `json:"services,omitempty"`
	// ActorIDs 过滤指定执行者的活动。
	ActorIDs []string `json:"actor_ids,omitempty"`
	// Severities 过滤指定严重级别的活动。
	Severities []Severity `json:"severities,omitempty"`
	// StartTime 过滤在此时间之后的活动。
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 过滤在此时间之前的活动。
	EndTime *time.Time `json:"end_time,omitempty"`
	// Resource 包含指定资源关键词的活动。
	Resource string `json:"resource,omitempty"`
	// Keyword 匹配描述或操作中的关键词。
	Keyword string `json:"keyword,omitempty"`
	// Limit 限制返回的活动数量（默认100）。
	Limit int `json:"limit,omitempty"`
	// Offset 用于分页的偏移量。
	Offset int `json:"offset,omitempty"`
}

// ActivitySummary 包含一段时间内活动的统计摘要。
type ActivitySummary struct {
	// Period 是摘要的时间范围描述（如 "daily", "weekly"）。
	Period string `json:"period"`
	// StartTime 是摘要范围的开始时间。
	StartTime time.Time `json:"start_time"`
	// EndTime 是摘要范围的结束时间。
	EndTime time.Time `json:"end_time"`
	// TotalActivities 是活动总数。
	TotalActivities int `json:"total_activities"`
	// ByService 按服务类型统计的活动数量。
	ByService map[ServiceType]int `json:"by_service"`
	// BySeverity 按严重级别统计的活动数量。
	BySeverity map[Severity]int `json:"by_severity"`
	// TopActors 是最活跃的执行者列表。
	TopActors []ActorStat `json:"top_actors"`
	// TopActions 是最常见的操作列表。
	TopActions []ActionStat `json:"top_actions"`
	// ErrorSummary 是错误和关键事件的摘要。
	ErrorSummary string `json:"error_summary,omitempty"`
	// GeneratedAt 是摘要生成的时间。
	GeneratedAt time.Time `json:"generated_at"`
}

// ActorStat 包含执行者的活动统计。
type ActorStat struct {
	// Actor 是执行者信息。
	Actor ActivityActor `json:"actor"`
	// Count 是活动数量。
	Count int `json:"count"`
}

// ActionStat 包含操作的统计。
type ActionStat struct {
	// Action 是操作名称。
	Action string `json:"action"`
	// Count 是操作次数。
	Count int `json:"count"`
}

// WebhookConfig 定义Webhook订阅配置。
type WebhookConfig struct {
	// ID 是订阅的唯一标识符。
	ID string `json:"id"`
	// URL 是Webhook的回调地址。
	URL string `json:"url"`
	// Secret 用于验证Webhook请求的签名。
	Secret string `json:"secret,omitempty"`
	// Filter 定义触发Webhook的活动过滤条件。
	Filter ActivityFilter `json:"filter"`
	// Enabled 表示订阅是否启用。
	Enabled bool `json:"enabled"`
	// CreatedAt 是订阅创建的时间。
	CreatedAt time.Time `json:"created_at"`
}

// FeedConfig 定义活动流的整体配置。
type FeedConfig struct {
	// MaxActivities 是内存中保留的最大活动数量。
	MaxActivities int `json:"max_activities"`
	// RetentionDays 是活动保留的天数。
	RetentionDays int `json:"retention_days"`
	// EnableWebhook 是否启用Webhook通知。
	EnableWebhook bool `json:"enable_webhook"`
	// DefaultSummarySchedule 是默认的摘要生成计划（daily/weekly）。
	DefaultSummarySchedule string `json:"default_summary_schedule"`
	// ExportFormats 是支持的导出格式列表。
	ExportFormats []string `json:"export_formats"`
}

// ExportFormat 表示导出格式类型。
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
)

// ExportData 包含导出的活动数据。
type ExportData struct {
	// Format 是导出格式。
	Format ExportFormat `json:"format"`
	// Filename 是建议的文件名。
	Filename string `json:"filename"`
	// Content 是导出的数据内容。
	Content []byte `json:"content"`
	// Count 是导出的活动数量。
	Count int `json:"count"`
	// ExportedAt 是导出时间。
	ExportedAt time.Time `json:"exported_at"`
}

// FeedEvent 表示通过订阅通道推送的活动事件。
type FeedEvent struct {
	// Activity 是触发事件的活动。
	Activity Activity `json:"activity"`
	// SubscriptionID 是触发的订阅ID。
	SubscriptionID string `json:"subscription_id"`
	// Timestamp 是事件发送的时间。
	Timestamp time.Time `json:"timestamp"`
}
