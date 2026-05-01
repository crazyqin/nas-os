// Package calendar 提供日历与事件管理功能，对标群晖 Calendar.
package calendar

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrCalendarNotFound 日历不存在.
	ErrCalendarNotFound = errors.New("日历不存在")
	// ErrEventNotFound 事件不存在.
	ErrEventNotFound = errors.New("事件不存在")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
	// ErrEndTimeBeforeStart 结束时间早于开始时间.
	ErrEndTimeBeforeStart = errors.New("结束时间不能早于开始时间")
	// ErrCalendarHasEvents 删除日历时存在关联事件.
	ErrCalendarHasEvents = errors.New("日历中仍有事件，请先删除事件")
)

// ========== 事件状态 ==========

// EventStatus 事件状态类型.
type EventStatus string

const (
	// StatusConfirmed 已确认.
	StatusConfirmed EventStatus = "confirmed"
	// StatusTentative 暂定.
	StatusTentative EventStatus = "tentative"
	// StatusCancelled 已取消.
	StatusCancelled EventStatus = "cancelled"
)

// ========== 重复规则 ==========

// RecurrenceFrequency 重复频率.
type RecurrenceFrequency string

const (
	// FreqDaily 每日重复.
	FreqDaily RecurrenceFrequency = "daily"
	// FreqWeekly 每周重复.
	FreqWeekly RecurrenceFrequency = "weekly"
	// FreqMonthly 每月重复.
	FreqMonthly RecurrenceFrequency = "monthly"
	// FreqYearly 每年重复.
	FreqYearly RecurrenceFrequency = "yearly"
)

// RecurrenceRule 重复规则 (简化 RRULE).
type RecurrenceRule struct {
	Frequency  RecurrenceFrequency `json:"frequency"`
	Interval   int                 `json:"interval"`   // 间隔，默认1
	Count      int                 `json:"count"`      // 重复次数，0表示无限
	Until      *time.Time          `json:"until"`      // 截止日期
	ByDay      []string            `json:"by_day"`     // 星期几，如 ["MO","WE","FR"]
	Exceptions []time.Time         `json:"exceptions"` // 例外日期
}

// ========== 提醒 ==========

// Reminder 提醒配置.
type Reminder struct {
	ID       string `json:"id"`
	Minutes  int    `json:"minutes"`  // 事件开始前多少分钟提醒
	Method   string `json:"method"`   // notification / email
}

// 预定义提醒时间点（分钟）.
var PresetReminders = []int{5, 15, 60, 1440}

// ========== 参与者 ==========

// Attendee 事件参与者.
type Attendee struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Status  string `json:"status"`  // accepted / declined / tentative / needs-action
	Comment string `json:"comment,omitempty"`
}

// ========== 核心数据结构 ==========

// Calendar 日历.
type Calendar struct {
	ID          string    `json:"id"`
	Name        string    `json:"name" binding:"required"`
	Color       string    `json:"color"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
}

// Event 日历事件.
type Event struct {
	ID            string          `json:"id"`
	CalendarID    string          `json:"calendar_id" binding:"required"`
	Title         string          `json:"title" binding:"required"`
	Description   string          `json:"description"`
	Location      string          `json:"location"`
	StartTime     time.Time       `json:"start_time" binding:"required"`
	EndTime       time.Time       `json:"end_time" binding:"required"`
	AllDay        bool            `json:"all_day"`
	Recurrence    *RecurrenceRule `json:"recurrence,omitempty"`
	Reminders     []Reminder      `json:"reminders"`
	Attendees     []Attendee      `json:"attendees"`
	Status        EventStatus     `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
}

// ========== 查询参数 ==========

// EventQuery 事件查询参数.
type EventQuery struct {
	CalendarID string    `form:"calendar_id"`
	Start      time.Time `form:"start"`
	End        time.Time `form:"end"`
	Keyword    string    `form:"keyword"`
	Status     string    `form:"status"`
}

// ========== ICS 相关 ==========

// ICSData ICS 导入/导出的数据表示.
type ICSData struct {
	Content string `json:"content"` // 原始 ICS 文本
}
