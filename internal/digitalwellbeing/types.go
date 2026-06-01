// Package digitalwellbeing 提供数字健康功能，支持屏幕时间统计、使用模式分析和专注模式。
// 对标 Apple Screen Time，为 NAS 系统提供家庭数字健康管理体验。
package digitalwellbeing

import "time"

// ScreenTime 屏幕时间
type ScreenTime struct {
	ID          string        `json:"id"`
	UserID      string        `json:"user_id"`
	Date        string        `json:"date"`          // YYYY-MM-DD
	TotalMinutes int          `json:"total_minutes"` // 总使用分钟数
	Apps        []AppUsage    `json:"apps"`
	Categories  []CategoryUsage `json:"categories"`
	FirstPickup  time.Time    `json:"first_pickup"`
	LastPickup   time.Time    `json:"last_pickup"`
	PickupCount  int          `json:"pickup_count"` // 拿起次数
	NotificationCount int     `json:"notification_count"`
}

// AppUsage 应用使用情况
type AppUsage struct {
	AppName    string `json:"app_name"`
	AppID      string `json:"app_id"`
	Category   string `json:"category"`
	Minutes    int    `json:"minutes"`
	Percentage float64 `json:"percentage"` // 占比
}

// CategoryUsage 分类使用情况
type CategoryUsage struct {
	Category   string  `json:"category"`   // social, productivity, entertainment, etc.
	Minutes    int     `json:"minutes"`
	Percentage float64 `json:"percentage"`
}

// UsagePattern 使用模式
type UsagePattern struct {
	ID             string      `json:"id"`
	UserID         string      `json:"user_id"`
	Period         string      `json:"period"`          // daily, weekly, monthly
	AverageMinutes int         `json:"average_minutes"` // 平均每日使用分钟数
	PeakHour       int         `json:"peak_hour"`       // 使用高峰时段 (0-23)
	PeakDay        string      `json:"peak_day"`        // 使用高峰日
	Trend          string      `json:"trend"`           // increasing, decreasing, stable
	TrendPercent   float64     `json:"trend_percent"`   // 趋势变化百分比
	TopApps        []AppUsage  `json:"top_apps"`
	Insights       []Insight   `json:"insights"`
}

// Insight 洞察
type Insight struct {
	Type    string `json:"type"`    // tip, warning, achievement
	Title   string `json:"title"`
	Message string `json:"message"`
}

// FocusSession 专注会话
type FocusSession struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at,omitempty"`
	DurationMin int       `json:"duration_min"` // 计划时长（分钟）
	ActualMin   int       `json:"actual_min"`   // 实际时长（分钟）
	Status      FocusStatus `json:"status"`
	BlockedApps []string  `json:"blocked_apps,omitempty"` // 屏蔽的应用
	AllowCalls  bool      `json:"allow_calls"`
	AllowNotifs bool      `json:"allow_notifs"`
}

// FocusStatus 专注状态
type FocusStatus string

const (
	FocusStatusActive    FocusStatus = "active"
	FocusStatusPaused    FocusStatus = "paused"
	FocusStatusCompleted FocusStatus = "completed"
	FocusStatusCancelled FocusStatus = "cancelled"
)

// FamilyMember 家庭成员
type FamilyMember struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Role       string   `json:"role"`       // parent, child, teen
	Avatar     string   `json:"avatar,omitempty"`
	DeviceIDs  []string `json:"device_ids,omitempty"`
	AgeGroup   string   `json:"age_group,omitempty"` // child (0-12), teen (13-17), adult (18+)
	CreatedAt  time.Time `json:"created_at"`
}

// WellbeingReport 健康报告
type WellbeingReport struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Period      string          `json:"period"` // daily, weekly, monthly
	StartDate   string          `json:"start_date"`
	EndDate     string          `json:"end_date"`
	Summary     ReportSummary   `json:"summary"`
	DailyData   []ScreenTime    `json:"daily_data"`
	TopApps     []AppUsage      `json:"top_apps"`
	Trends      UsagePattern    `json:"trends"`
	Suggestions []string        `json:"suggestions"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalMinutes    int     `json:"total_minutes"`
	AverageDaily    int     `json:"average_daily"`
	MostUsedApp     string  `json:"most_used_app"`
	MostUsedCategory string `json:"most_used_category"`
	TotalPickups    int     `json:"total_pickups"`
	AveragePickups  int     `json:"average_pickups"`
	ComparedToLast  float64 `json:"compared_to_last"` // 与上期对比百分比
}

// DowntimeSchedule 停机时间计划
type DowntimeSchedule struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Enabled   bool     `json:"enabled"`
	StartHour int      `json:"start_hour"` // 0-23
	EndHour   int      `json:"end_hour"`   // 0-23
	Days      []string `json:"days"`       // monday, tuesday, ...
	AllowApps []string `json:"allow_apps"` // 允许使用的应用
}

// AppLimit 应用限制
type AppLimit struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	AppName   string `json:"app_name"`
	AppID     string `json:"app_id"`
	DailyMin  int    `json:"daily_min"`  // 每日使用限制（分钟）
	Enabled   bool   `json:"enabled"`
}
