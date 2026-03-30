// Package quota 提供存储配额管理和告警功能
// 参考 Synology DSM 存储配额管理设计
package quota

import (
	"time"
)

// ========== 配额规则类型 ==========

// TargetType 配额目标类型
type TargetType string

const (
	// TargetTypeUser 用户配额
	TargetTypeUser TargetType = "user"
	// TargetTypeGroup 用户组配额
	TargetTypeGroup TargetType = "group"
	// TargetTypeVolume 卷配额
	TargetTypeVolume TargetType = "volume"
)

// ActionType 配额超限动作
type ActionType string

const (
	// ActionNotify 仅通知
	ActionNotify ActionType = "notify"
	// ActionBlock 阻止写入
	ActionBlock ActionType = "block"
)

// QuotaRule 配额规则
type QuotaRule struct {
	ID          string    `json:"id"`
	TargetType  string    `json:"target_type"` // user, group, volume
	TargetID    string    `json:"target_id"`
	MaxBytes    int64     `json:"max_bytes"`
	WarnPercent int       `json:"warn_percent"` // 告警百分比（默认80）
	Action      string    `json:"action"` // notify, block
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// QuotaUsage 配额使用情况
type QuotaUsage struct {
	RuleID    string  `json:"rule_id"`
	TargetID  string  `json:"target_id"`
	UsedBytes int64   `json:"used_bytes"`
	MaxBytes  int64   `json:"max_bytes"`
	Percent   float64 `json:"percent"`
	Status    string  `json:"status"` // normal, warning, exceeded
}

// Alert 告警信息
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // quota_warning, quota_exceeded
	Target    string    `json:"target"`
	Percent   float64   `json:"percent"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved,omitempty"`
}

// UsageStatus 使用状态
type UsageStatus string

const (
	// StatusNormal 正常状态
	StatusNormal UsageStatus = "normal"
	// StatusWarning 警告状态（超过警告阈值）
	StatusWarning UsageStatus = "warning"
	// StatusExceeded 超限状态（超过最大限制）
	StatusExceeded UsageStatus = "exceeded"
)

// AlertType 告警类型
type AlertType string

const (
	// AlertTypeWarning 配额警告
	AlertTypeWarning AlertType = "quota_warning"
	// AlertTypeExceeded 配额超限
	AlertTypeExceeded AlertType = "quota_exceeded"
)

// ========== 输入结构 ==========

// QuotaRuleInput 创建/更新配额规则输入
type QuotaRuleInput struct {
	TargetType  string `json:"target_type" binding:"required"`
	TargetID    string `json:"target_id" binding:"required"`
	MaxBytes    int64  `json:"max_bytes" binding:"required"`
	WarnPercent int    `json:"warn_percent"` // 默认80%
	Action      string `json:"action"`       // 默认notify
	Enabled     bool   `json:"enabled"`
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled     bool     `json:"enabled"`
	Channels    []string `json:"channels"` // email, webhook, slack, etc.
	EmailList   []string `json:"email_list,omitempty"`
	WebhookURL  string   `json:"webhook_url,omitempty"`
	CoolDownMin int      `json:"cool_down_min"` // 冷却时间（分钟）
}

// DefaultNotificationConfig 默认通知配置
func DefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		Enabled:     true,
		Channels:    []string{"email"},
		CoolDownMin: 30,
	}
}