// Package smartalerttriage 提供智能告警分类系统。
// 支持AI驱动的告警分类、优先级排序、去重聚合、根因分析、
// 告警升级策略、抑制规则、知识库推荐和多通道通知。
package smartalerttriage

import (
	"time"
)

// Priority 告警优先级.
type Priority string

const (
	PriorityCritical Priority = "critical" // 紧急
	PriorityHigh     Priority = "high"     // 高
	PriorityMedium   Priority = "medium"   // 中
	PriorityLow      Priority = "low"      // 低
	PriorityInfo     Priority = "info"     // 信息
)

// PriorityWeight 优先级权重（越大越优先）.
var PriorityWeight = map[Priority]int{
	PriorityCritical: 5,
	PriorityHigh:     4,
	PriorityMedium:   3,
	PriorityLow:      2,
	PriorityInfo:     1,
}

// Category 告警分类.
type Category string

const (
	CategoryStorage   Category = "storage"   // 存储
	CategoryNetwork   Category = "network"   // 网络
	CategorySystem    Category = "system"    // 系统
	CategorySecurity  Category = "security"  // 安全
	CategoryService   Category = "service"   // 服务
	CategoryHardware  Category = "hardware"  // 硬件
	CategoryUnknown   Category = "unknown"   // 未知
)

// TriageState 告警分类处理状态.
type TriageState string

const (
	StateNew          TriageState = "new"          // 新建
	StateClassified   TriageState = "classified"   // 已分类
	StateDeduped      TriageState = "deduped"      // 已去重
	StateCorrelated   TriageState = "correlated"   // 已关联
	StateAcknowledged TriageState = "acknowledged" // 已确认
	StateEscalated    TriageState = "escalated"    // 已升级
	StateSuppressed   TriageState = "suppressed"   // 已抑制
	StateResolved     TriageState = "resolved"     // 已解决
)

// NotificationChannel 通知通道类型.
type NotificationChannel string

const (
	ChannelEmail    NotificationChannel = "email"    // 邮件
	ChannelWebhook  NotificationChannel = "webhook"  // Webhook
	ChannelSMS      NotificationChannel = "sms"      // 短信
	ChannelIM       NotificationChannel = "im"       // 即时通讯（企业微信/钉钉/Slack）
)

// Alert 智能告警条目.
type Alert struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	Priority         Priority          `json:"priority"`
	OriginalPriority Priority          `json:"original_priority"` // 原始优先级
	Category         Category          `json:"category"`
	State            TriageState       `json:"state"`
	Source           string            `json:"source"`   // 告警来源
	Resource         string            `json:"resource"` // 关联资源
	Labels           map[string]string `json:"labels,omitempty"`
	Fingerprint      string            `json:"fingerprint"` // 告警指纹（用于去重）

	// 关联信息
	GroupID      string   `json:"group_id,omitempty"`      // 聚合组ID
	RootCauseID  string   `json:"root_cause_id,omitempty"` // 根因ID
	RelatedIDs   []string `json:"related_ids,omitempty"`   // 关联告警ID

	// 知识库推荐
	RecommendedActions []RecommendedAction `json:"recommended_actions,omitempty"`

	// 生命周期
	FirstSeen      time.Time  `json:"first_seen"`
	LastSeen       time.Time  `json:"last_seen"`
	Count          int        `json:"count"` // 去重后的出现次数
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string     `json:"acknowledged_by,omitempty"`
	EscalatedAt    *time.Time `json:"escalated_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	SuppressedBy   string     `json:"suppressed_by,omitempty"` // 抑制规则ID
}

// RecommendedAction 推荐操作.
type RecommendedAction struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Automated   bool   `json:"automated"` // 是否可自动化执行
	RiskLevel   string `json:"risk_level"` // low/medium/high
}

// AlertGroup 告警聚合组.
type AlertGroup struct {
	ID          string    `json:"id"`
	Fingerprint string    `json:"fingerprint"` // 聚合指纹
	AlertIDs    []string  `json:"alert_ids"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Priority    Priority  `json:"priority"`
	Category    Category  `json:"category"`
	Source      string    `json:"source"`      // 告警来源
	Title       string    `json:"title"`
}

// RootCause 根因分析结果.
type RootCause struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	Category        Category `json:"category"`
	Confidence      float64  `json:"confidence"` // 置信度 0-1
	RelatedAlertIDs []string `json:"related_alert_ids"`
	SuggestedFix    string   `json:"suggested_fix,omitempty"`
}

// EscalationPolicy 告警升级策略.
type EscalationPolicy struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Priority     Priority      `json:"priority"`      // 适用的优先级
	UpgradeAfter time.Duration `json:"upgrade_after"`  // 未处理多久后升级
	MaxPriority  Priority      `json:"max_priority"`   // 最高升到什么级别
	NotifyOnEsc  bool          `json:"notify_on_esc"`  // 升级时是否通知
	Enabled      bool          `json:"enabled"`
}

// SuppressionRule 告警抑制规则.
type SuppressionRule struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Category    Category   `json:"category,omitempty"` // 按分类抑制
	Source      string     `json:"source,omitempty"`   // 按来源抑制
	Pattern     string     `json:"pattern,omitempty"`  // 标题匹配模式
	StartTime   time.Time  `json:"start_time"`
	EndTime     time.Time  `json:"end_time"`
	Reason      string     `json:"reason"` // 抑制原因（维护窗口/已知问题）
	CreatedBy   string     `json:"created_by"`
	Enabled     bool       `json:"enabled"`
}

// KnowledgeEntry 知识库条目.
type KnowledgeEntry struct {
	ID              string             `json:"id"`
	Keywords        []string           `json:"keywords"`
	Title           string             `json:"title"`
	Summary         string             `json:"summary"`
	Category        Category           `json:"category"`
	Priority        Priority           `json:"priority"`
	Actions         []RecommendedAction `json:"actions"`
	References      []string           `json:"references"`
	RootCauseKey    string             `json:"root_cause_key"`
}

// AlertStats 告警统计.
type AlertStats struct {
	TotalAlerts     int                `json:"total_alerts"`
	ActiveAlerts    int                `json:"active_alerts"`
	CriticalCount   int                `json:"critical_count"`
	HighCount       int                `json:"high_count"`
	MediumCount     int                `json:"medium_count"`
	LowCount        int                `json:"low_count"`
	InfoCount       int                `json:"info_count"`
	SuppressedCount int                `json:"suppressed_count"`
	ByCategory      map[Category]int   `json:"by_category"`
	BySource        map[string]int     `json:"by_source"`
	AcknowledgedPct float64            `json:"acknowledged_pct"`
	AvgResolution   time.Duration      `json:"avg_resolution"` // 平均解决时间
}

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	Count        int       `json:"count"`
	CriticalCount int      `json:"critical_count"`
	WarningCount  int      `json:"warning_count"`
}

// NotificationConfig 通知配置.
type NotificationConfig struct {
	Channel  NotificationChannel `json:"channel"`
	Enabled  bool                `json:"enabled"`
	Target   string              `json:"target"`   // 目标地址/URL
	Priority Priority            `json:"priority"` // 触发通知的最低优先级
}

// ========== API 请求/响应类型 ==========

// ClassifyRequest 分类告警请求.
type ClassifyRequest struct {
	Title       string            `json:"title" binding:"required"`
	Description string            `json:"description"`
	Source      string            `json:"source"`
	Resource    string            `json:"resource"`
	Labels      map[string]string `json:"labels"`
}

// AcknowledgeRequest 确认告警请求.
type AcknowledgeRequest struct {
	Operator string `json:"operator"`
}

// SuppressRequest 创建抑制规则请求.
type SuppressRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Source      string   `json:"source"`
	Pattern     string   `json:"pattern"`
	DurationMin int      `json:"duration_min" binding:"required,min=1"`
	Reason      string   `json:"reason" binding:"required"`
	CreatedBy   string   `json:"created_by"`
}

// ListQuery 告警列表查询参数.
type ListQuery struct {
	Category Category   `form:"category"`
	Priority Priority   `form:"priority"`
	State    TriageState `form:"state"`
	Source   string     `form:"source"`
}

// StatsQuery 统计查询参数.
type StatsQuery struct {
	Hours int `form:"hours"` // 统计时间范围（小时）
}

// TrendQuery 趋势查询参数.
type TrendQuery struct {
	Hours  int `form:"hours"`  // 时间范围（小时）
	Points int `form:"points"` // 数据点数量
}
