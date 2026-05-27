package smartnotify

import (
	"sync"
	"time"
)

// Priority represents notification priority levels
type Priority int

const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ParsePriority parses string to Priority
func ParsePriority(s string) Priority {
	switch s {
	case "low":
		return PriorityLow
	case "medium":
		return PriorityMedium
	case "high":
		return PriorityHigh
	case "critical":
		return PriorityCritical
	default:
		return PriorityMedium
	}
}

// Channel represents notification delivery channel
type Channel string

const (
	ChannelEmail     Channel = "email"
	ChannelWebhook   Channel = "webhook"
	ChannelTelegram  Channel = "telegram"
	ChannelDiscord   Channel = "discord"
	ChannelWeChat    Channel = "wecom"
	ChannelDingTalk  Channel = "dingtalk"
	ChannelSMS       Channel = "sms"
)

// NotificationStatus represents the delivery status
type NotificationStatus string

const (
	StatusPending   NotificationStatus = "pending"
	StatusSending   NotificationStatus = "sending"
	StatusDelivered NotificationStatus = "delivered"
	StatusFailed    NotificationStatus = "failed"
	StatusAggregated NotificationStatus = "aggregated"
)

// Notification represents a notification message
type Notification struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Content      string             `json:"content"`
	Priority     Priority           `json:"priority"`
	Source       string             `json:"source"`   // source module name
	Labels       map[string]string  `json:"labels"`
	Status       NotificationStatus `json:"status"`
	IsAggregated bool               `json:"is_aggregated,omitempty"` // true if this is an aggregated notification
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// RoutingRule defines how notifications are routed
type RoutingRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Priority    []Priority        `json:"priority"`      // match these priorities
	SourceMatch []string          `json:"source_match"`  // match these sources (glob patterns)
	Channels    []Channel         `json:"channels"`      // deliver to these channels
	Recipients  []string          `json:"recipients"`    // recipient identifiers
	TimeWindow  *TimeWindow       `json:"time_window"`   // optional time constraint
	Enabled     bool              `json:"enabled"`
}

// TimeWindow defines a time-based routing constraint
type TimeWindow struct {
	Start  string `json:"start"`  // HH:MM format
	End    string `json:"end"`    // HH:MM format
	Days   []int  `json:"days"`   // 0=Sunday, 1=Monday, ..., 6=Saturday
	Timezone string `json:"timezone"`
}

// QuietHours represents do-not-disturb configuration
type QuietHours struct {
	Enabled  bool   `json:"enabled"`
	Start    string `json:"start"`    // HH:MM format
	End      string `json:"end"`      // HH:MM format
	Timezone string `json:"timezone"`
	Override []Priority `json:"override"` // priorities that can break quiet hours
}

// AggregationConfig defines notification aggregation rules
type AggregationConfig struct {
	Enabled    bool          `json:"enabled"`
	Window     time.Duration `json:"window"`      // aggregation time window
	MaxCount   int           `json:"max_count"`    // max notifications before flush
	GroupBy    []string      `json:"group_by"`     // group by these fields (source, priority, labels)
}

// RetryConfig defines retry behavior for failed deliveries
type RetryConfig struct {
	MaxRetries  int           `json:"max_retries"`
	InitialWait time.Duration `json:"initial_wait"`
	MaxWait     time.Duration `json:"max_wait"`
	Multiplier  float64       `json:"multiplier"`
}

// DeliveryResult represents the result of a notification delivery attempt
type DeliveryResult struct {
	NotificationID string             `json:"notification_id"`
	Channel        Channel            `json:"channel"`
	Recipient      string             `json:"recipient"`
	Status         NotificationStatus `json:"status"`
	Error          string             `json:"error,omitempty"`
	SentAt         time.Time          `json:"sent_at"`
	RetryCount     int                `json:"retry_count"`
}

// UserPreference stores per-user notification preferences
type UserPreference struct {
	UserID       string            `json:"user_id"`
	Channels     []Channel         `json:"channels"`      // preferred channels
	QuietHours   *QuietHours       `json:"quiet_hours"`
	MinPriority  Priority          `json:"min_priority"`  // ignore below this priority
	Labels       map[string]string `json:"labels"`        // filter by labels
}

// DeliveryFunc is a function that delivers a notification to a channel
type DeliveryFunc func(channel Channel, recipient string, notif *Notification) error

// AggregationKey is used to group notifications for aggregation
type AggregationKey struct {
	Source   string
	Priority Priority
	LabelKey string
}

// PendingAggregation holds notifications waiting to be aggregated
type PendingAggregation struct {
	Key           AggregationKey
	Notifications []*Notification
	CreatedAt     time.Time
	Timer         *time.Timer
	mu            sync.Mutex
}
