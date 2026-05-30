// Package notifyrouter 提供智能通知路由功能，根据紧急程度和用户偏好自动选择最佳通知渠道。
// 提供通知路由规则管理、多渠道投递状态跟踪、渠道优化建议等能力。
package notifyrouter

import "time"

// Channel 通知渠道
type Channel string

const (
	ChannelEmail    Channel = "email"
	ChannelSMS      Channel = "sms"
	ChannelPush     Channel = "push"
	ChannelSlack    Channel = "slack"
	ChannelWeChat   Channel = "wechat"
	ChannelDingTalk Channel = "dingtalk"
	ChannelWebhook  Channel = "webhook"
	ChannelVoice    Channel = "voice"
)

// NotifyPriority 通知优先级
type NotifyPriority string

const (
	PriorityUrgent   NotifyPriority = "urgent"
	PriorityHigh     NotifyPriority = "high"
	PriorityNormal   NotifyPriority = "normal"
	PriorityLow      NotifyPriority = "low"
)

// DeliveryStatus 投递状态
type DeliveryStatus string

const (
	StatusPending   DeliveryStatus = "pending"
	StatusQueued    DeliveryStatus = "queued"
	StatusSent      DeliveryStatus = "sent"
	StatusDelivered DeliveryStatus = "delivered"
	StatusFailed    DeliveryStatus = "failed"
	StatusRetrying  DeliveryStatus = "retrying"
	StatusExpired   DeliveryStatus = "expired"
)

// RuleType 规则类型
type RuleType string

const (
	RuleTypePriority RuleType = "priority"
	RuleTypeTime     RuleType = "time"
	RuleTypeContent  RuleType = "content"
	RuleTypeUser     RuleType = "user"
	RuleTypeGroup    RuleType = "group"
)

// NotifyRule 通知路由规则
type NotifyRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Type        RuleType       `json:"type"`
	IsActive    bool           `json:"is_active"`
	Priority    NotifyPriority `json:"priority"`
	Channels    []Channel      `json:"channels" binding:"required,min=1"`
	Conditions  *Conditions    `json:"conditions,omitempty"`
	Fallback    *Channel       `json:"fallback,omitempty"`
	Throttle    *Throttle      `json:"throttle,omitempty"`
	Priority_   int            `json:"priority_order"` // 规则优先级顺序
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Conditions 触发条件
type Conditions struct {
	Priorities    []NotifyPriority `json:"priorities,omitempty"`
	UserIDs       []string         `json:"user_ids,omitempty"`
	GroupIDs      []string         `json:"group_ids,omitempty"`
	Tags          []string         `json:"tags,omitempty"`
	SourceSystems []string         `json:"source_systems,omitempty"`
	TimeWindow    *TimeWindow      `json:"time_window,omitempty"`
	Keywords      []string         `json:"keywords,omitempty"`
}

// TimeWindow 时间窗口
type TimeWindow struct {
	StartHour int `json:"start_hour"` // 0-23
	EndHour   int `json:"end_hour"`   // 0-23
	DaysOfWeek []int `json:"days_of_week,omitempty"` // 0=Sunday, 6=Saturday
}

// Throttle 限流配置
type Throttle struct {
	MaxPerMinute  int `json:"max_per_minute"`
	MaxPerHour    int `json:"max_per_hour"`
	MaxPerDay     int `json:"max_per_day"`
	CooldownSecs  int `json:"cooldown_secs"`
}

// ChannelConfig 渠道配置
type ChannelConfig struct {
	Channel     Channel `json:"channel"`
	IsEnabled   bool    `json:"is_enabled"`
	Endpoint    string  `json:"endpoint,omitempty"`
	APIKey      string  `json:"api_key,omitempty"`
	MaxRetries  int     `json:"max_retries"`
	TimeoutSecs int     `json:"timeout_secs"`
	Extra       map[string]string `json:"extra,omitempty"`
}

// UserPreference 用户通知偏好
type UserPreference struct {
	UserID           string             `json:"user_id"`
	PreferredChannels []Channel         `json:"preferred_channels"`
	QuietHours       *TimeWindow        `json:"quiet_hours,omitempty"`
	MinPriority      NotifyPriority     `json:"min_priority"`
	Language         string             `json:"language,omitempty"`
	Timezone         string             `json:"timezone,omitempty"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// Delivery 投递记录
type Delivery struct {
	ID         string         `json:"id"`
	NotifyID   string         `json:"notify_id"`
	Channel    Channel        `json:"channel"`
	UserID     string         `json:"user_id"`
	Status     DeliveryStatus `json:"status"`
	RetryCount int            `json:"retry_count"`
	LastError  string         `json:"last_error,omitempty"`
	SentAt     *time.Time     `json:"sent_at,omitempty"`
	DeliveredAt *time.Time    `json:"delivered_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// Notification 通知消息
type Notification struct {
	ID        string         `json:"id"`
	Title     string         `json:"title" binding:"required"`
	Body      string         `json:"body" binding:"required"`
	Priority  NotifyPriority `json:"priority"`
	Tags      []string       `json:"tags,omitempty"`
	Source    string         `json:"source,omitempty"`
	UserID    string         `json:"user_id" binding:"required"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// RouteResult 路由结果
type RouteResult struct {
	NotifyID       string       `json:"notify_id"`
	SelectedChannel Channel    `json:"selected_channel"`
	AllChannels    []Channel    `json:"all_channels"`
	Deliveries     []*Delivery  `json:"deliveries"`
	Reason         string       `json:"reason"`
	RoutedAt       time.Time    `json:"routed_at"`
}

// ChannelStats 渠道统计
type ChannelStats struct {
	Channel         Channel `json:"channel"`
	TotalSent       int     `json:"total_sent"`
	TotalDelivered  int     `json:"total_delivered"`
	TotalFailed     int     `json:"total_failed"`
	DeliveryRate    float64 `json:"delivery_rate"` // 百分比 0-100
	AvgDeliveryTime time.Duration `json:"avg_delivery_time"`
	LastUsed        time.Time `json:"last_used"`
}

// OptimizeResult 优化建议
type OptimizeResult struct {
	CurrentConfig    map[Channel]*ChannelStats `json:"current_config"`
	Recommendations  []Recommendation         `json:"recommendations"`
	GeneratedAt      time.Time                `json:"generated_at"`
}

// Recommendation 优化建议
type Recommendation struct {
	Channel   Channel `json:"channel"`
	Action    string  `json:"action"`
	Reason    string  `json:"reason"`
	Impact    string  `json:"impact"`
	Priority  int     `json:"priority"`
}

// RouteNotificationRequest 路由通知请求
type RouteNotificationRequest struct {
	Notification *Notification `json:"notification" binding:"required"`
	ForceChannel *Channel      `json:"force_channel,omitempty"`
}

// SetRuleRequest 设置规则请求
type SetRuleRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Type        RuleType       `json:"type"`
	Priority    NotifyPriority `json:"priority"`
	Channels    []Channel      `json:"channels" binding:"required,min=1"`
	Conditions  *Conditions    `json:"conditions,omitempty"`
	Fallback    *Channel       `json:"fallback,omitempty"`
	Throttle    *Throttle      `json:"throttle,omitempty"`
	Priority_   int            `json:"priority_order"`
}

// GetDeliveryStatusRequest 获取投递状态请求
type GetDeliveryStatusRequest struct {
	NotifyID string `json:"notify_id"`
	DeliveryID string `json:"delivery_id"`
}
