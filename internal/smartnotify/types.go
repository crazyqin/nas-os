// Package smartnotify 提供智能通知中心功能，支持多渠道通知、规则引擎、优先级分级、免打扰等。
package smartnotify

import "time"

// NotifyChannel 通知渠道
type NotifyChannel string

const (
	ChannelEmail     NotifyChannel = "email"
	ChannelSMS       NotifyChannel = "sms"
	ChannelWeChat    NotifyChannel = "wechat"
	ChannelDingTalk  NotifyChannel = "dingtalk"
	ChannelTelegram  NotifyChannel = "telegram"
	ChannelWebhook   NotifyChannel = "webhook"
	ChannelPush      NotifyChannel = "push"
)

// NotifyPriority 通知优先级
type NotifyPriority int

const (
	PriorityLow      NotifyPriority = 0
	PriorityNormal   NotifyPriority = 1
	PriorityImportant NotifyPriority = 2
	PriorityUrgent   NotifyPriority = 3
)

// PriorityName 获取优先级名称
func PriorityName(p NotifyPriority) string {
	switch p {
	case PriorityUrgent:
		return "紧急"
	case PriorityImportant:
		return "重要"
	case PriorityNormal:
		return "普通"
	case PriorityLow:
		return "低"
	default:
		return "未知"
	}
}

// NotifyStatus 通知状态
type NotifyStatus string

const (
	StatusPending   NotifyStatus = "pending"
	StatusSending   NotifyStatus = "sending"
	StatusSent      NotifyStatus = "sent"
	StatusDelivered NotifyStatus = "delivered"
	StatusFailed    NotifyStatus = "failed"
	StatusSilenced  NotifyStatus = "silenced"
	StatusEscalated NotifyStatus = "escalated"
)

// RuleOperator 规则操作符
type RuleOperator string

const (
	OpEquals     RuleOperator = "equals"
	OpNotEquals  RuleOperator = "not_equals"
	OpContains   RuleOperator = "contains"
	OpGreaterThan RuleOperator = "gt"
	OpLessThan   RuleOperator = "lt"
	OpRegex      RuleOperator = "regex"
)

// AggregateType 聚合类型
type AggregateType string

const (
	AggregateNone  AggregateType = "none"
	AggregateCount AggregateType = "count"
	AggregateTime  AggregateType = "time"
)

// Notification 通知消息
type Notification struct {
	ID         string            `json:"id"`
	Title      string            `json:"title" binding:"required"`
	Content    string            `json:"content" binding:"required"`
	Priority   NotifyPriority    `json:"priority"`
	Channels   []NotifyChannel   `json:"channels"`
	Tags       map[string]string `json:"tags,omitempty"`
	Source     string            `json:"source,omitempty"`
	TemplateID string            `json:"template_id,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"`
	Status     NotifyStatus      `json:"status"`
	RetryCount int               `json:"retry_count"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	SentAt     *time.Time        `json:"sent_at,omitempty"`
}

// NotifyRule 通知规则
type NotifyRule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description,omitempty"`
	Enabled     bool            `json:"enabled"`
	Priority    NotifyPriority  `json:"priority"`
	Conditions  []RuleCondition `json:"conditions" binding:"required,min=1"`
	Channels    []NotifyChannel `json:"channels" binding:"required,min=1"`
	Aggregate   AggregateConfig `json:"aggregate,omitempty"`
	Silence     SilenceConfig   `json:"silence,omitempty"`
	Escalation  EscalationConfig `json:"escalation,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	Field    string       `json:"field" binding:"required"`
	Operator RuleOperator `json:"operator" binding:"required"`
	Value    string       `json:"value" binding:"required"`
}

// AggregateConfig 聚合配置
type AggregateConfig struct {
	Type     AggregateType `json:"type"`
	Window   time.Duration `json:"window,omitempty"`
	MaxCount int           `json:"max_count,omitempty"`
}

// SilenceConfig 免打扰配置
type SilenceConfig struct {
	Enabled   bool        `json:"enabled"`
	StartTime string      `json:"start_time,omitempty"` // HH:MM 格式
	EndTime   string      `json:"end_time,omitempty"`   // HH:MM 格式
	Days      []time.Weekday `json:"days,omitempty"`
}

// EscalationConfig 升级策略配置
type EscalationConfig struct {
	Enabled      bool            `json:"enabled"`
	Timeout      time.Duration   `json:"timeout,omitempty"`
	MaxLevel     int             `json:"max_level,omitempty"`
	Channels     []NotifyChannel `json:"channels,omitempty"`
}

// NotifyTemplate 通知模板
type NotifyTemplate struct {
	ID        string            `json:"id"`
	Name      string            `json:"name" binding:"required"`
	Channel   NotifyChannel     `json:"channel"`
	Title     string            `json:"title" binding:"required"`
	Content   string            `json:"content" binding:"required"`
	Variables []string          `json:"variables,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// NotifyHistory 通知历史
type NotifyHistory struct {
	ID          string          `json:"id"`
	NotifyID    string          `json:"notify_id"`
	RuleID      string          `json:"rule_id,omitempty"`
	Title       string          `json:"title"`
	Content     string          `json:"content"`
	Priority    NotifyPriority  `json:"priority"`
	Channel     NotifyChannel   `json:"channel"`
	Status      NotifyStatus    `json:"status"`
	Error       string          `json:"error,omitempty"`
	RetryCount  int             `json:"retry_count"`
	Escalated   bool            `json:"escalated"`
	CreatedAt   time.Time       `json:"created_at"`
	DeliveredAt *time.Time      `json:"delivered_at,omitempty"`
}

// NotifyStats 通知统计
type NotifyStats struct {
	TotalSent     int            `json:"total_sent"`
	TotalFailed   int            `json:"total_failed"`
	TotalSilenced int            `json:"total_silenced"`
	ByChannel     map[NotifyChannel]int `json:"by_channel"`
	ByPriority    map[string]int        `json:"by_priority"`
	AvgLatency    time.Duration  `json:"avg_latency"`
}

// ChannelConfig 渠道配置
type ChannelConfig struct {
	Channel NotifyChannel `json:"channel"`
	Enabled bool          `json:"enabled"`
	Config  interface{}   `json:"config,omitempty"`
}

// SmartNotifyConfig 智能通知配置
type SmartNotifyConfig struct {
	Enabled        bool            `json:"enabled"`
	DefaultChannels []NotifyChannel `json:"default_channels"`
	MaxRetries     int             `json:"max_retries"`
	RetryInterval  time.Duration   `json:"retry_interval"`
	MaxHistory     int             `json:"max_history"`
	Deduplication  bool            `json:"deduplication"`
	DedupWindow    time.Duration   `json:"dedup_window"`
}

// DefaultSmartNotifyConfig 默认配置
func DefaultSmartNotifyConfig() *SmartNotifyConfig {
	return &SmartNotifyConfig{
		Enabled: true,
		DefaultChannels: []NotifyChannel{
			ChannelEmail,
			ChannelPush,
		},
		MaxRetries:    3,
		RetryInterval: 5 * time.Minute,
		MaxHistory:    10000,
		Deduplication: true,
		DedupWindow:   5 * time.Minute,
	}
}

// ValidChannels 获取所有有效渠道
func ValidChannels() []NotifyChannel {
	return []NotifyChannel{
		ChannelEmail,
		ChannelSMS,
		ChannelWeChat,
		ChannelDingTalk,
		ChannelTelegram,
		ChannelWebhook,
		ChannelPush,
	}
}

// IsValidChannel 检查渠道是否有效
func IsValidChannel(ch NotifyChannel) bool {
	for _, c := range ValidChannels() {
		if c == ch {
			return true
		}
	}
	return false
}

// ChannelName 获取渠道中文名称
func ChannelName(ch NotifyChannel) string {
	names := map[NotifyChannel]string{
		ChannelEmail:    "邮件",
		ChannelSMS:      "短信",
		ChannelWeChat:   "微信",
		ChannelDingTalk: "钉钉",
		ChannelTelegram: "Telegram",
		ChannelWebhook:  "Webhook",
		ChannelPush:     "推送",
	}
	if name, ok := names[ch]; ok {
		return name
	}
	return string(ch)
}
