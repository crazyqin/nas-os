// Package notificationcenter 提供系统通知中心功能
// 包括多渠道通知、规则引擎、通知聚合、静默时段、模板系统
package notificationcenter

import (
	"errors"
	"time"
)

// ========== 通知优先级 ==========

// Priority 通知优先级
type Priority string

const (
	PriorityCritical Priority = "critical" // 紧急：立即推送，不可静默
	PriorityHigh     Priority = "high"     // 高：重要告警
	PriorityMedium   Priority = "medium"   // 中：一般通知
	PriorityLow      Priority = "low"      // 低：信息性通知
)

// PriorityWeight 返回优先级权重（用于排序）
func PriorityWeight(p Priority) int {
	switch p {
	case PriorityCritical:
		return 4
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}

// ========== 通知渠道 ==========

// Channel 通知渠道类型
type Channel string

const (
	ChannelWebSocket Channel = "websocket" // WebSocket 实时推送
	ChannelEmail     Channel = "email"     // 邮件
	ChannelWebhook   Channel = "webhook"   // Webhook 回调
	ChannelTelegram  Channel = "telegram"  // Telegram Bot
	ChannelWeChat    Channel = "wechat"    // 企业微信
)

// ========== 通知状态 ==========

// NotificationStatus 通知状态
type NotificationStatus string

const (
	StatusUnread   NotificationStatus = "unread"
	StatusRead     NotificationStatus = "read"
	StatusArchived NotificationStatus = "archived"
)

// ========== 错误定义 ==========

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrTemplateNotFound     = errors.New("template not found")
	ErrRuleNotFound         = errors.New("rule not found")
	ErrChannelDisabled      = errors.New("channel is disabled")
	ErrInvalidSilentPeriod  = errors.New("invalid silent period")
	ErrDuplicateRuleName    = errors.New("rule name already exists")
)

// ========== 通知实体 ==========

// Notification 通知
type Notification struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Priority  Priority               `json:"priority"`
	Category  string                 `json:"category"`
	Status    NotificationStatus     `json:"status"`
	Channels  []Channel              `json:"channels"`
	Source    string                 `json:"source,omitempty"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	AggKey    string                 `json:"agg_key,omitempty"`
	AggCount  int                    `json:"agg_count"`
	CreatedAt time.Time              `json:"created_at"`
	ReadAt    *time.Time             `json:"read_at,omitempty"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
}

// ========== 通知模板 ==========

// NotificationTemplate 通知模板
type NotificationTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Subject     string    `json:"subject"`             // 标题模板，支持 Go template
	Body        string    `json:"body"`                // 内容模板，支持 Go template
	Channel     Channel   `json:"channel"`             // 适用渠道
	Priority    Priority  `json:"priority"`            // 默认优先级
	Variables   []string  `json:"variables,omitempty"` // 可用变量列表
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ========== 通知规则 ==========

// RuleCondition 规则条件
type RuleCondition struct {
	Field    string      `json:"field"`    // 字段名
	Operator string      `json:"operator"` // ==, !=, >, <, >=, <=, contains, regex
	Value    interface{} `json:"value"`    // 比较值
}

// NotificationRule 通知规则
type NotificationRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Enabled     bool              `json:"enabled"`
	Priority    Priority          `json:"priority"`
	Channels    []Channel         `json:"channels"`
	TemplateID  string            `json:"template_id,omitempty"`
	Conditions  []RuleCondition   `json:"conditions"`
	Logic       string            `json:"logic"`              // "and" or "or"
	Throttle    int               `json:"throttle,omitempty"` // 节流：N 秒内最多触发一次
	Category    string            `json:"category,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CreatedBy   string            `json:"created_by,omitempty"`
	// 内部状态
	lastFiredAt time.Time
}

// ========== 通知聚合 ==========

// AggregationEntry 聚合条目
type AggregationEntry struct {
	AggKey    string        `json:"agg_key"`
	Count     int           `json:"count"`
	FirstAt   time.Time     `json:"first_at"`
	LastAt    time.Time     `json:"last_at"`
	LastNotif *Notification `json:"last_notification,omitempty"`
	Window    time.Duration `json:"window"`
}

// ========== 静默时段 ==========

// SilentPeriod 静默时段
type SilentPeriod struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	StartHour   int               `json:"start_hour"`   // 0-23
	StartMinute int               `json:"start_minute"` // 0-59
	EndHour     int               `json:"end_hour"`     // 0-23
	EndMinute   int               `json:"end_minute"`   // 0-59
	Weekdays    []int             `json:"weekdays"`     // 0=Sunday, 1=Monday, ...
	Priority    []Priority        `json:"priority"`     // 静默的优先级列表（critical 不可静默）
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// IsActive 检查当前是否在静默时段内
func (sp *SilentPeriod) IsActive(now time.Time) bool {
	if !sp.Enabled {
		return false
	}

	weekday := int(now.Weekday())
	inWeekday := false
	for _, wd := range sp.Weekdays {
		if wd == weekday {
			inWeekday = true
			break
		}
	}
	if !inWeekday {
		return false
	}

	current := now.Hour()*60 + now.Minute()
	start := sp.StartHour*60 + sp.StartMinute
	end := sp.EndHour*60 + sp.EndMinute

	if start <= end {
		return current >= start && current < end
	}
	// 跨午夜
	return current >= start || current < end
}

// IsPrioritySilent 检查某优先级是否被静默
func (sp *SilentPeriod) IsPrioritySilent(p Priority) bool {
	if p == PriorityCritical {
		return false // 紧急通知不可静默
	}
	for _, pp := range sp.Priority {
		if pp == p {
			return true
		}
	}
	return false
}

// ========== 用户通知偏好 ==========

// UserPreference 用户通知偏好
type UserPreference struct {
	UserID      string                  `json:"user_id"`
	Channels    map[Channel]ChannelPref `json:"channels"`
	MinPriority Priority                `json:"min_priority"` // 最低接收优先级
	Language    string                  `json:"language,omitempty"`
	Timezone    string                  `json:"timezone,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// ChannelPref 单渠道偏好
type ChannelPref struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address,omitempty"` // 邮箱、webhook URL、chat ID 等
}

// DefaultUserPreference 默认用户偏好
func DefaultUserPreference(userID string) *UserPreference {
	now := time.Now()
	return &UserPreference{
		UserID: userID,
		Channels: map[Channel]ChannelPref{
			ChannelWebSocket: {Enabled: true},
			ChannelEmail:     {Enabled: false},
			ChannelWebhook:   {Enabled: false},
			ChannelTelegram:  {Enabled: false},
			ChannelWeChat:    {Enabled: false},
		},
		MinPriority: PriorityLow,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ========== 汇总统计 ==========

// NotificationSummary 通知摘要
type NotificationSummary struct {
	TotalUnread int              `json:"total_unread"`
	TotalRead   int              `json:"total_read"`
	ByPriority  map[Priority]int `json:"by_priority"`
	ByCategory  map[string]int   `json:"by_category"`
}
