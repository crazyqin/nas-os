// Package wellbeing 提供数字健康助手功能，监控使用时间、提醒休息、限制过度使用。
package wellbeing

import "time"

// SessionType 使用会话类型
type SessionType string

const (
	SessionTypeApp     SessionType = "app"
	SessionTypeScreen  SessionType = "screen"
	SessionTypeService SessionType = "service"
	SessionTypeNetwork SessionType = "network"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	StatusActive  SessionStatus = "active"
	StatusPaused  SessionStatus = "paused"
	StatusEnded   SessionStatus = "ended"
	StatusBlocked SessionStatus = "blocked"
)

// ReminderType 提醒类型
type ReminderType string

const (
	ReminderBreak     ReminderType = "break"
	ReminderLimit     ReminderType = "limit"
	ReminderPosture   ReminderType = "posture"
	ReminderEyeRest   ReminderType = "eye_rest"
	ReminderHydration ReminderType = "hydration"
	ReminderCustom    ReminderType = "custom"
)

// ReminderStatus 提醒状态
type ReminderStatus string

const (
	ReminderActive    ReminderStatus = "active"
	ReminderTriggered ReminderStatus = "triggered"
	ReminderSnoozed   ReminderStatus = "snoozed"
	ReminderDismissed ReminderStatus = "dismissed"
	ReminderCompleted ReminderStatus = "completed"
)

// InsightType 洞察类型
type InsightType string

const (
	InsightUsageIncrease InsightType = "usage_increase"
	InsightUsageDecrease InsightType = "usage_decrease"
	InsightPeakTime      InsightType = "peak_time"
	InsightSuggestion    InsightType = "suggestion"
	InsightAchievement   InsightType = "achievement"
)

// UsageSession 使用会话
type UsageSession struct {
	ID          string        `json:"id"`
	UserID      string        `json:"user_id"`
	Type        SessionType   `json:"type"`
	AppName     string        `json:"app_name,omitempty"`
	ServiceName string        `json:"service_name,omitempty"`
	Status      SessionStatus `json:"status"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     *time.Time    `json:"end_time,omitempty"`
	Duration    time.Duration `json:"duration"`
	Activity    string        `json:"activity,omitempty"`
	DeviceInfo  string        `json:"device_info,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

// BreakReminder 休息提醒
type BreakReminder struct {
	ID              string         `json:"id"`
	UserID          string         `json:"user_id"`
	Type            ReminderType   `json:"type"`
	Title           string         `json:"title" binding:"required"`
	Message         string         `json:"message" binding:"required"`
	IntervalMinutes int            `json:"interval_minutes" binding:"required,min=1"`
	DurationMinutes int            `json:"duration_minutes"`
	Status          ReminderStatus `json:"status"`
	Enabled         bool           `json:"enabled"`
	SnoozeMinutes   int            `json:"snooze_minutes"`
	LastTriggered   *time.Time     `json:"last_triggered,omitempty"`
	NextTrigger     *time.Time     `json:"next_trigger,omitempty"`
	Sound           string         `json:"sound,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// ScreenTime 屏幕时间统计
type ScreenTime struct {
	UserID       string        `json:"user_id"`
	Date         string        `json:"date"`
	TotalMinutes int           `json:"total_minutes"`
	AppUsage     []AppUsage    `json:"app_usage"`
	HourlyUsage  []HourlyUsage `json:"hourly_usage"`
	BreaksTaken  int           `json:"breaks_taken"`
	BreaksMissed int           `json:"breaks_missed"`
	FirstActive  time.Time     `json:"first_active"`
	LastActive   time.Time     `json:"last_active"`
}

// AppUsage 应用使用统计
type AppUsage struct {
	AppName     string  `json:"app_name"`
	Minutes     int     `json:"minutes"`
	Percentage  float64 `json:"percentage"`
	LaunchCount int     `json:"launch_count"`
}

// HourlyUsage 每小时使用统计
type HourlyUsage struct {
	Hour    int `json:"hour"`
	Minutes int `json:"minutes"`
}

// WellnessInsight 健康洞察
type WellnessInsight struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Type      InsightType `json:"type"`
	Title     string      `json:"title"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Priority  int         `json:"priority"`
	Read      bool        `json:"read"`
	CreatedAt time.Time   `json:"created_at"`
}

// WellnessReport 健康报告
type WellnessReport struct {
	ID              string            `json:"id"`
	UserID          string            `json:"user_id"`
	Period          string            `json:"period"`
	StartDate       time.Time         `json:"start_date"`
	EndDate         time.Time         `json:"end_date"`
	TotalScreenTime int               `json:"total_screen_time"`
	AvgDailyMinutes int               `json:"avg_daily_minutes"`
	TopApps         []AppUsage        `json:"top_apps"`
	DailyBreakdown  []DailyBreakdown  `json:"daily_breakdown"`
	Insights        []WellnessInsight `json:"insights"`
	Score           int               `json:"score"`
	ComparedToPrev  *Comparison       `json:"compared_to_previous,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// DailyBreakdown 每日明细
type DailyBreakdown struct {
	Date    string `json:"date"`
	Minutes int    `json:"minutes"`
	Breaks  int    `json:"breaks"`
	Score   int    `json:"score"`
}

// Comparison 对比数据
type Comparison struct {
	ScreenTimeChange float64 `json:"screen_time_change"`
	BreakChange      float64 `json:"break_change"`
	ScoreChange      int     `json:"score_change"`
	Trend            string  `json:"trend"`
}

// UsageLimit 使用限制
type UsageLimit struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	AppName      string    `json:"app_name" binding:"required"`
	DailyLimit   int       `json:"daily_limit_minutes" binding:"required,min=1"`
	Enabled      bool      `json:"enabled"`
	CurrentUsed  int       `json:"current_used_minutes"`
	WarningAt    int       `json:"warning_at_percent"`
	BlockAtLimit bool      `json:"block_at_limit"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateReminderRequest 创建提醒请求
type CreateReminderRequest struct {
	Type            ReminderType `json:"type" binding:"required"`
	Title           string       `json:"title" binding:"required"`
	Message         string       `json:"message" binding:"required"`
	IntervalMinutes int          `json:"interval_minutes" binding:"required,min=1"`
	DurationMinutes int          `json:"duration_minutes"`
	SnoozeMinutes   int          `json:"snooze_minutes"`
	Sound           string       `json:"sound,omitempty"`
}

// UpdateReminderRequest 更新提醒请求
type UpdateReminderRequest struct {
	Title           string `json:"title,omitempty"`
	Message         string `json:"message,omitempty"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
	SnoozeMinutes   int    `json:"snooze_minutes,omitempty"`
	Enabled         *bool  `json:"enabled,omitempty"`
	Sound           string `json:"sound,omitempty"`
}

// CreateUsageLimitRequest 创建使用限制请求
type CreateUsageLimitRequest struct {
	AppName      string `json:"app_name" binding:"required"`
	DailyLimit   int    `json:"daily_limit_minutes" binding:"required,min=1"`
	WarningAt    int    `json:"warning_at_percent"`
	BlockAtLimit bool   `json:"block_at_limit"`
}

// UpdateUsageLimitRequest 更新使用限制请求
type UpdateUsageLimitRequest struct {
	DailyLimit   *int  `json:"daily_limit_minutes,omitempty"`
	Enabled      *bool `json:"enabled,omitempty"`
	WarningAt    *int  `json:"warning_at_percent,omitempty"`
	BlockAtLimit *bool `json:"block_at_limit,omitempty"`
}

// ReportRequest 报告请求
type ReportRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Period   string `json:"period" binding:"required,oneof=daily weekly monthly"`
	FromDate string `json:"from_date,omitempty"`
	ToDate   string `json:"to_date,omitempty"`
}

// DefaultBreakReminder 默认休息提醒配置
func DefaultBreakReminder() *CreateReminderRequest {
	return &CreateReminderRequest{
		Type:            ReminderBreak,
		Title:           "休息一下",
		Message:         "您已经连续工作很久了，请起身活动一下，放松眼睛。",
		IntervalMinutes: 45,
		DurationMinutes: 5,
		SnoozeMinutes:   10,
	}
}

// EyeRestReminder 眼睛休息提醒
func EyeRestReminder() *CreateReminderRequest {
	return &CreateReminderRequest{
		Type:            ReminderEyeRest,
		Title:           "眼睛休息",
		Message:         "请遵循 20-20-20 法则：看 20 英尺外的物体 20 秒。",
		IntervalMinutes: 20,
		DurationMinutes: 1,
		SnoozeMinutes:   5,
	}
}

// HydrationReminder 饮水提醒
func HydrationReminder() *CreateReminderRequest {
	return &CreateReminderRequest{
		Type:            ReminderHydration,
		Title:           "喝水提醒",
		Message:         "记得喝水，保持充足的水分摄入有助于健康。",
		IntervalMinutes: 60,
		DurationMinutes: 0,
		SnoozeMinutes:   15,
	}
}
