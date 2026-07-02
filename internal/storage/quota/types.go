// Package quota 提供存储配额管理和告警功能
// 参考 Synology DSM 存储配额管理设计
package quota

import (
	"time"
)

// ========== 配额规则类型 ==========

// TargetType 配额目标类型.
type TargetType string

const (
	// TargetTypeUser 用户配额.
	TargetTypeUser TargetType = "user"
	// TargetTypeGroup 用户组配额.
	TargetTypeGroup TargetType = "group"
	// TargetTypeVolume 卷配额.
	TargetTypeVolume TargetType = "volume"
)

// ActionType 配额超限动作.
type ActionType string

const (
	// ActionNotify 仅通知.
	ActionNotify ActionType = "notify"
	// ActionBlock 阻止写入.
	ActionBlock ActionType = "block"
)

// QuotaRule 配额规则.
type QuotaRule struct {
	ID          string    `json:"id"`
	TargetType  string    `json:"target_type"` // user, group, volume
	TargetID    string    `json:"target_id"`
	MaxBytes    int64     `json:"max_bytes"`
	WarnPercent int       `json:"warn_percent"` // 告警百分比（默认80）
	Action      string    `json:"action"`       // notify, block
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// QuotaUsage 配额使用情况.
type QuotaUsage struct {
	RuleID    string  `json:"rule_id"`
	TargetID  string  `json:"target_id"`
	UsedBytes int64   `json:"used_bytes"`
	MaxBytes  int64   `json:"max_bytes"`
	Percent   float64 `json:"percent"`
	Status    string  `json:"status"` // normal, warning, exceeded
}

// Alert 告警信息.
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // quota_warning, quota_exceeded
	Target    string    `json:"target"`
	Percent   float64   `json:"percent"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved,omitempty"`
}

// UsageStatus 使用状态.
type UsageStatus string

const (
	// StatusNormal 正常状态.
	StatusNormal UsageStatus = "normal"
	// StatusWarning 警告状态（超过警告阈值）.
	StatusWarning UsageStatus = "warning"
	// StatusExceeded 超限状态（超过最大限制）.
	StatusExceeded UsageStatus = "exceeded"
)

// AlertType 告警类型.
type AlertType string

const (
	// AlertTypeWarning 配额警告.
	AlertTypeWarning AlertType = "quota_warning"
	// AlertTypeExceeded 配额超限.
	AlertTypeExceeded AlertType = "quota_exceeded"
)

// ========== 输入结构 ==========

// QuotaRuleInput 创建/更新配额规则输入.
type QuotaRuleInput struct {
	TargetType  string `json:"target_type" binding:"required"`
	TargetID    string `json:"target_id" binding:"required"`
	MaxBytes    int64  `json:"max_bytes" binding:"required"`
	WarnPercent int    `json:"warn_percent"` // 默认80%
	Action      string `json:"action"`       // 默认notify
	Enabled     bool   `json:"enabled"`
}

// NotificationConfig 通知配置.
type NotificationConfig struct {
	Enabled     bool     `json:"enabled"`
	Channels    []string `json:"channels"` // email, webhook, slack, etc.
	EmailList   []string `json:"email_list,omitempty"`
	WebhookURL  string   `json:"webhook_url,omitempty"`
	CoolDownMin int      `json:"cool_down_min"` // 冷却时间（分钟）
}

// DefaultNotificationConfig 默认通知配置.
func DefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		Enabled:     true,
		Channels:    []string{"email"},
		CoolDownMin: 30,
	}
}

// ========== 容量预测相关类型 ==========

// UsageHistory 使用历史记录.
type UsageHistory struct {
	Timestamp time.Time `json:"timestamp"`
	UsedBytes int64     `json:"used_bytes"`
}

// PredictionResult 容量预测结果.
type PredictionResult struct {
	RuleID            string  `json:"rule_id"`
	TargetID          string  `json:"target_id"`
	CurrentUsage      int64   `json:"current_usage"`
	MaxBytes          int64   `json:"max_bytes"`
	CurrentPercent    float64 `json:"current_percent"`
	DaysUntilFull     float64 `json:"days_until_full"`     // 预计多少天满
	EstimatedFullDate string  `json:"estimated_full_date"` // 预计满额日期
	DailyGrowthRate   float64 `json:"daily_growth_rate"`   // 每日增长率(bytes/day)
	Trend             string  `json:"trend"`               // growing, stable, declining
	Confidence        float64 `json:"confidence"`          // 预测置信度 0-1
	WarningLevel      string  `json:"warning_level"`       // low, medium, high, critical
}

// AlertRule 告警规则配置.
type AlertRule struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	TargetType    string    `json:"target_type"` // user, group, volume, or "*" for all
	TargetID      string    `json:"target_id"`   // specific target or "*"
	Thresholds    []int     `json:"thresholds"`  // 多个阈值，如 [60, 80, 90, 95]
	Channels      []string  `json:"channels"`    // 通知渠道
	Enabled       bool      `json:"enabled"`
	ScheduleStart string    `json:"schedule_start"` // 告警生效开始时间 HH:MM
	ScheduleEnd   string    `json:"schedule_end"`   // 告警生效结束时间 HH:MM
	RepeatEnabled bool      `json:"repeat_enabled"` // 是否重复告警
	RepeatHours   int       `json:"repeat_hours"`   // 重复告警间隔（小时）
	CreatedAt     time.Time `json:"created_at"`
}

// AlertRuleInput 告警规则输入.
type AlertRuleInput struct {
	Name          string   `json:"name" binding:"required"`
	TargetType    string   `json:"target_type"`
	TargetID      string   `json:"target_id"`
	Thresholds    []int    `json:"thresholds" binding:"required,min=1"`
	Channels      []string `json:"channels"`
	Enabled       bool     `json:"enabled"`
	ScheduleStart string   `json:"schedule_start"`
	ScheduleEnd   string   `json:"schedule_end"`
	RepeatEnabled bool     `json:"repeat_enabled"`
	RepeatHours   int      `json:"repeat_hours"`
}

// ForecastConfig 预测配置.
type ForecastConfig struct {
	HistoryDays   int     `json:"history_days"`    // 使用的历史数据天数
	Method        string  `json:"method"`          // linear, exponential, moving_average
	Sensitivity   float64 `json:"sensitivity"`     // 灵敏度 0-1
	Seasonality   bool    `json:"seasonality"`     // 是否考虑季节性
	MinDataPoints int     `json:"min_data_points"` // 最少数据点
}

// DefaultForecastConfig 默认预测配置.
func DefaultForecastConfig() ForecastConfig {
	return ForecastConfig{
		HistoryDays:   30,
		Method:        "linear",
		Sensitivity:   0.7,
		Seasonality:   false,
		MinDataPoints: 7,
	}
}

// ========== 告警规则相关常量 ==========

const (
	// ThresholdLow 低阈值.
	ThresholdLow = 60
	// ThresholdMedium 中阈值.
	ThresholdMedium = 80
	// ThresholdHigh 高阈值.
	ThresholdHigh = 90
	// ThresholdCritical 紧急阈值.
	ThresholdCritical = 95

	// WarningLevelLow 低风险.
	WarningLevelLow = "low"
	// WarningLevelMedium 中风险.
	WarningLevelMedium = "medium"
	// WarningLevelHigh 高风险.
	WarningLevelHigh = "high"
	// WarningLevelCritical 紧急风险.
	WarningLevelCritical = "critical"

	// TrendGrowing 增长趋势.
	TrendGrowing = "growing"
	// TrendStable 稳定趋势.
	TrendStable = "stable"
	// TrendDeclining 下降趋势.
	TrendDeclining = "declining"
)

// GetWarningLevel 根据百分比获取警告级别.
func GetWarningLevel(percent float64) string {
	switch {
	case percent >= ThresholdCritical:
		return WarningLevelCritical
	case percent >= ThresholdHigh:
		return WarningLevelHigh
	case percent >= ThresholdMedium:
		return WarningLevelMedium
	case percent >= ThresholdLow:
		return WarningLevelLow
	default:
		return WarningLevelLow
	}
}
