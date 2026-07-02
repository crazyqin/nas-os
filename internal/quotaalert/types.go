// Package quotaalert 提供存储配额智能预警功能，支持配额设置、使用量追踪、告警生成、趋势预测和清理建议。
package quotaalert

import "time"

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
	AlertExceeded AlertLevel = "exceeded"
)

// TrendDirection 趋势方向.
type TrendDirection string

const (
	TrendGrowing   TrendDirection = "growing"
	TrendStable    TrendDirection = "stable"
	TrendShrinking TrendDirection = "shrinking"
)

// SuggestionType 清理建议类型.
type SuggestionType string

const (
	SuggestionTemp       SuggestionType = "temp"
	SuggestionDuplicates SuggestionType = "duplicates"
	SuggestionOldBackups SuggestionType = "old_backups"
	SuggestionLargeFiles SuggestionType = "large_files"
)

// QuotaRule 配额规则定义.
type QuotaRule struct {
	ID                string    `json:"id"`
	Path              string    `json:"path"`
	UserID            string    `json:"user_id"`
	MaxBytes          int64     `json:"max_bytes"`
	MaxFiles          int64     `json:"max_files"`
	WarnThreshold     float64   `json:"warn_threshold"`     // 告警阈值，如 0.8 表示 80%
	CriticalThreshold float64   `json:"critical_threshold"` // 严重告警阈值，如 0.95 表示 95%
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
}

// QuotaUsage 配额使用量.
type QuotaUsage struct {
	UserID       string    `json:"user_id"`
	Path         string    `json:"path"`
	UsedBytes    int64     `json:"used_bytes"`
	UsedFiles    int64     `json:"used_files"`
	TotalBytes   int64     `json:"total_bytes"`
	TotalFiles   int64     `json:"total_files"`
	UsagePercent float64   `json:"usage_percent"`
	LastUpdated  time.Time `json:"last_updated"`
}

// QuotaAlert 配额告警.
type QuotaAlert struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Path         string     `json:"path"`
	Level        AlertLevel `json:"level"`
	Message      string     `json:"message"`
	CurrentUsage int64      `json:"current_usage"`
	Threshold    float64    `json:"threshold"`
	Acknowledged bool       `json:"acknowledged"`
	CreatedAt    time.Time  `json:"created_at"`
}

// UsageTrend 使用量趋势.
type UsageTrend struct {
	UserID            string         `json:"user_id"`
	Path              string         `json:"path"`
	DailyGrowth       int64          `json:"daily_growth"`
	WeeklyGrowth      int64          `json:"weekly_growth"`
	MonthlyGrowth     int64          `json:"monthly_growth"`
	PredictedFullDate *time.Time     `json:"predicted_full_date"`
	TrendDirection    TrendDirection `json:"trend_direction"`
}

// CleanupSuggestion 清理建议.
type CleanupSuggestion struct {
	ID                    string         `json:"id"`
	UserID                string         `json:"user_id"`
	Path                  string         `json:"path"`
	SuggestionType        SuggestionType `json:"suggestion_type"`
	EstimatedReclaimBytes int64          `json:"estimated_reclaim_bytes"`
	Priority              int            `json:"priority"`
	Files                 []string       `json:"files"`
}

// QuotaReport 全局配额报告.
type QuotaReport struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Users       []UserQuotaSummary `json:"users"`
}

// UserQuotaSummary 用户配额摘要.
type UserQuotaSummary struct {
	UserID       string              `json:"user_id"`
	UserName     string              `json:"user_name"`
	TotalQuota   int64               `json:"total_quota"`
	UsedQuota    int64               `json:"used_quota"`
	UsagePercent float64             `json:"usage_percent"`
	Trend        *UsageTrend         `json:"trend"`
	Alerts       []QuotaAlert        `json:"alerts"`
	Suggestions  []CleanupSuggestion `json:"suggestions"`
}
