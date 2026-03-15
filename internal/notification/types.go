// Package notification 提供通知中心功能
// Version: v2.49.0 - 通知中心模块
package notification

import (
	"time"
)

// NotificationLevel 通知级别
type NotificationLevel string

const (
	LevelInfo     NotificationLevel = "info"
	LevelSuccess  NotificationLevel = "success"
	LevelWarning  NotificationLevel = "warning"
	LevelError    NotificationLevel = "error"
	LevelCritical NotificationLevel = "critical"
)

// ChannelType 通知渠道类型
type ChannelType string

const (
	ChannelEmail     ChannelType = "email"
	ChannelWebhook   ChannelType = "webhook"
	ChannelWebSocket ChannelType = "websocket"
	ChannelWeChat    ChannelType = "wechat"
	ChannelDingTalk  ChannelType = "dingtalk"
	ChannelTelegram  ChannelType = "telegram"
)

// NotificationStatus 通知状态
type NotificationStatus string

const (
	StatusPending   NotificationStatus = "pending"
	StatusSent      NotificationStatus = "sent"
	StatusFailed    NotificationStatus = "failed"
	StatusRetrying  NotificationStatus = "retrying"
	StatusCancelled NotificationStatus = "cancelled"
)

// RuleCondition 规则条件运算符
type RuleCondition string

const (
	ConditionEquals      RuleCondition = "equals"
	ConditionNotEquals   RuleCondition = "not_equals"
	ConditionContains    RuleCondition = "contains"
	ConditionNotContains RuleCondition = "not_contains"
	ConditionGreaterThan RuleCondition = "greater_than"
	ConditionLessThan    RuleCondition = "less_than"
	ConditionMatches     RuleCondition = "matches"
	ConditionExists      RuleCondition = "exists"
)

// LogicalOperator 逻辑运算符
type LogicalOperator string

const (
	OperatorAnd LogicalOperator = "and"
	OperatorOr  LogicalOperator = "or"
	OperatorNot LogicalOperator = "not"
)

// Notification 通知消息
type Notification struct {
	ID         string                 `json:"id"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Level      NotificationLevel      `json:"level"`
	Category   string                 `json:"category,omitempty"`
	Source     string                 `json:"source,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	TemplateID string                 `json:"templateId,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
	ExpiresAt  *time.Time             `json:"expiresAt,omitempty"`
}

// NotificationRecord 通知发送记录
type NotificationRecord struct {
	ID             string             `json:"id"`
	NotificationID string             `json:"notificationId"`
	Notification   *Notification      `json:"notification,omitempty"`
	Channel        ChannelType        `json:"channel"`
	ChannelName    string             `json:"channelName"`
	Status         NotificationStatus `json:"status"`
	Attempts       int                `json:"attempts"`
	MaxAttempts    int                `json:"maxAttempts"`
	Error          string             `json:"error,omitempty"`
	SentAt         *time.Time         `json:"sentAt,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

// ChannelConfig 通知渠道配置
type ChannelConfig struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        ChannelType            `json:"type"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
	Description string                 `json:"description,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// EmailChannelConfig 邮件渠道配置
type EmailChannelConfig struct {
	SMTPHost   string   `json:"smtpHost"`
	SMTPPort   int      `json:"smtpPort"`
	Username   string   `json:"username"`
	Password   string   `json:"password,omitempty"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	UseTLS     bool     `json:"useTLS"`
	SkipVerify bool     `json:"skipVerify"`
}

// WebhookChannelConfig Webhook 渠道配置
type WebhookChannelConfig struct {
	URL           string            `json:"url"`
	Method        string            `json:"method,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Timeout       int               `json:"timeout,omitempty"` // 秒
	RetryCount    int               `json:"retryCount,omitempty"`
	RetryInterval int               `json:"retryInterval,omitempty"` // 秒
}

// WebSocketChannelConfig WebSocket 渠道配置
type WebSocketChannelConfig struct {
	RoomIDs   []string `json:"roomIds,omitempty"`
	UserIDs   []string `json:"userIds,omitempty"`
	Broadcast bool     `json:"broadcast,omitempty"`
}

// WeChatChannelConfig 企业微信渠道配置
type WeChatChannelConfig struct {
	WebhookURL string `json:"webhookUrl"`
}

// DingTalkChannelConfig 钉钉渠道配置
type DingTalkChannelConfig struct {
	WebhookURL string `json:"webhookUrl"`
	Secret     string `json:"secret,omitempty"`
}

// TelegramChannelConfig Telegram 渠道配置
type TelegramChannelConfig struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
}

// Template 通知模板
type Template struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Category    string             `json:"category,omitempty"`
	Subject     string             `json:"subject"`
	Body        string             `json:"body"`
	Variables   []TemplateVariable `json:"variables,omitempty"`
	Channels    []ChannelType      `json:"channels,omitempty"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

// TemplateVariable 模板变量
type TemplateVariable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// Rule 通知规则
type Rule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Priority    int          `json:"priority"`
	Conditions  RuleGroup    `json:"conditions"`
	Actions     []RuleAction `json:"actions"`
	Channels    []string     `json:"channels"` // 渠道 ID 列表
	TemplateID  string       `json:"templateId,omitempty"`
	RateLimit   *RateLimit   `json:"rateLimit,omitempty"`
	QuietHours  *QuietHours  `json:"quietHours,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// RuleGroup 规则条件组
type RuleGroup struct {
	Operator LogicalOperator     `json:"operator"`
	Rules    []RuleConditionItem `json:"rules,omitempty"`
	Groups   []RuleGroup         `json:"groups,omitempty"`
}

// RuleConditionItem 规则条件项
type RuleConditionItem struct {
	Field     string        `json:"field"`
	Condition RuleCondition `json:"condition"`
	Value     interface{}   `json:"value,omitempty"`
}

// RuleAction 规则动作
type RuleAction struct {
	Type       string                 `json:"type"` // forward, transform, filter
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// RateLimit 频率限制
type RateLimit struct {
	Count    int           `json:"count"`
	Duration time.Duration `json:"duration"`
}

// QuietHours 静默时段
type QuietHours struct {
	Start string `json:"start"`          // HH:mm 格式
	End   string `json:"end"`            // HH:mm 格式
	Days  []int  `json:"days,omitempty"` // 0-6, 0=周日
}

// HistoryFilter 历史记录过滤条件
type HistoryFilter struct {
	StartTime *time.Time         `json:"startTime,omitempty"`
	EndTime   *time.Time         `json:"endTime,omitempty"`
	Level     NotificationLevel  `json:"level,omitempty"`
	Status    NotificationStatus `json:"status,omitempty"`
	Channel   ChannelType        `json:"channel,omitempty"`
	Category  string             `json:"category,omitempty"`
	Source    string             `json:"source,omitempty"`
	Search    string             `json:"search,omitempty"`
	Page      int                `json:"page,omitempty"`
	PageSize  int                `json:"pageSize,omitempty"`
}

// HistoryStats 历史统计
type HistoryStats struct {
	TotalCount      int                       `json:"totalCount"`
	SuccessCount    int                       `json:"successCount"`
	FailedCount     int                       `json:"failedCount"`
	PendingCount    int                       `json:"pendingCount"`
	ChannelStats    map[ChannelType]int       `json:"channelStats"`
	LevelStats      map[NotificationLevel]int `json:"levelStats"`
	DailyStats      []DailyStat               `json:"dailyStats,omitempty"`
	AvgDeliveryTime float64                   `json:"avgDeliveryTime,omitempty"`
}

// DailyStat 每日统计
type DailyStat struct {
	Date    string `json:"date"`
	Count   int    `json:"count"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// SendRequest 发送通知请求
type SendRequest struct {
	Notification *Notification          `json:"notification"`
	Channels     []string               `json:"channels,omitempty"` // 指定渠道 ID，为空则使用规则匹配
	TemplateID   string                 `json:"templateId,omitempty"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	Priority     int                    `json:"priority,omitempty"`
	Delay        time.Duration          `json:"delay,omitempty"`
}

// SendResponse 发送通知响应
type SendResponse struct {
	NotificationID string                `json:"notificationId"`
	Records        []*NotificationRecord `json:"records"`
	Success        bool                  `json:"success"`
	Errors         map[string]string     `json:"errors,omitempty"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	DefaultRetryCount    int           `json:"defaultRetryCount"`
	DefaultRetryInterval time.Duration `json:"defaultRetryInterval"`
	MaxHistoryDays       int           `json:"maxHistoryDays"`
	MaxConcurrent        int           `json:"maxConcurrent"`
	HistoryStorage       string        `json:"historyStorage"` // memory, file, database
	StoragePath          string        `json:"storagePath,omitempty"`
}
