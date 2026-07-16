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
	// ErrTaskNotFound 任务不存在.
	ErrTaskNotFound = errors.New("任务不存在")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
	// ErrEndTimeBeforeStart 结束时间早于开始时间.
	ErrEndTimeBeforeStart = errors.New("结束时间不能早于开始时间")
	// ErrCalendarHasEvents 删除日历时存在关联事件.
	ErrCalendarHasEvents = errors.New("日历中仍有事件，请先删除事件")
	// ErrCalendarHasTasks 删除日历时存在关联任务.
	ErrCalendarHasTasks = errors.New("日历中仍有任务，请先删除任务")
	// ErrShareNotFound 分享不存在.
	ErrShareNotFound = errors.New("分享不存在")
	// ErrAlreadyShared 已经分享给该用户.
	ErrAlreadyShared = errors.New("已经分享给该用户")
	// ErrNoPermission 没有操作权限.
	ErrNoPermission = errors.New("没有操作权限")
	// ErrReminderNotFound 提醒不存在.
	ErrReminderNotFound = errors.New("提醒不存在")
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

// ========== 任务状态与优先级 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusNeedsAction 待处理.
	TaskStatusNeedsAction TaskStatus = "needs-action"
	// TaskStatusInProcess 进行中.
	TaskStatusInProcess TaskStatus = "in-process"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusCancelled 已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskPriority 任务优先级 (0-9, 0=无, 1=最高, 9=最低).
type TaskPriority int

const (
	// PriorityNone 无优先级.
	PriorityNone TaskPriority = 0
	// PriorityHighest 最高优先级.
	PriorityHighest TaskPriority = 1
	// PriorityHigh 高优先级.
	PriorityHigh TaskPriority = 3
	// PriorityMedium 中等优先级.
	PriorityMedium TaskPriority = 5
	// PriorityLow 低优先级.
	PriorityLow TaskPriority = 7
	// PriorityLowest 最低优先级.
	PriorityLowest TaskPriority = 9
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
	ID      string `json:"id"`
	Minutes int    `json:"minutes"` // 事件开始前多少分钟提醒
	Method  string `json:"method"`  // notification / email
}

// ReminderNotification 提醒通知记录.
type ReminderNotification struct {
	ID          string     `json:"id"`
	ReminderID  string     `json:"reminder_id"`
	EventID     string     `json:"event_id"`
	UserID      string     `json:"user_id"`
	Message     string     `json:"message"`
	Method      string     `json:"method"` // notification / email
	Status      string     `json:"status"` // pending / sent / failed
	ScheduledAt time.Time  `json:"scheduled_at"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// 预定义提醒时间点（分钟）.
var PresetReminders = []int{5, 15, 60, 1440}

// ========== 参与者 ==========

// Attendee 事件参与者.
type Attendee struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Status  string `json:"status"` // accepted / declined / tentative / needs-action
	Comment string `json:"comment,omitempty"`
}

// ========== 日历分享与权限 ==========

// SharePermission 分享权限级别.
type SharePermission string

const (
	// PermissionRead 只读.
	PermissionRead SharePermission = "read"
	// PermissionWrite 读写.
	PermissionWrite SharePermission = "write"
	// PermissionOwner 所有者权限.
	PermissionOwner SharePermission = "owner"
)

// CalendarShare 日历分享记录.
type CalendarShare struct {
	ID         string          `json:"id"`
	CalendarID string          `json:"calendar_id"`
	UserID     string          `json:"user"` // 被分享用户
	Permission SharePermission `json:"permission"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ShareRequest 分享请求.
type ShareRequest struct {
	UserID     string          `json:"user" binding:"required"`
	Permission SharePermission `json:"permission"`
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
	Timezone    string    `json:"timezone"`
	CreatedAt   time.Time `json:"created_at"`
}

// Event 日历事件.
type Event struct {
	ID          string          `json:"id"`
	CalendarID  string          `json:"calendar_id" binding:"required"`
	Title       string          `json:"title" binding:"required"`
	Description string          `json:"description"`
	Location    string          `json:"location"`
	StartTime   time.Time       `json:"start_time" binding:"required"`
	EndTime     time.Time       `json:"end_time" binding:"required"`
	AllDay      bool            `json:"all_day"`
	Recurrence  *RecurrenceRule `json:"recurrence,omitempty"`
	Reminders   []Reminder      `json:"reminders"`
	Attendees   []Attendee      `json:"attendees"`
	Status      EventStatus     `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Task 待办任务 (VTODO).
type Task struct {
	ID          string          `json:"id"`
	CalendarID  string          `json:"calendar_id" binding:"required"`
	Title       string          `json:"title" binding:"required"`
	Description string          `json:"description"`
	Status      TaskStatus      `json:"status"`
	Priority    TaskPriority    `json:"priority"`
	DueDate     *time.Time      `json:"due_date,omitempty"`
	StartDate   *time.Time      `json:"start_date,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Progress    int             `json:"progress"` // 0-100
	Recurrence  *RecurrenceRule `json:"recurrence,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
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

// TaskQuery 任务查询参数.
type TaskQuery struct {
	CalendarID string `form:"calendar_id"`
	Status     string `form:"status"`
	Priority   int    `form:"priority"`
	Keyword    string `form:"keyword"`
	HasDueDate *bool  `form:"has_due_date"`
}

// ========== ICS 相关 ==========

// ICSData ICS 导入/导出的数据表示.
type ICSData struct {
	Content string `json:"content"` // 原始 ICS 文本
}

// ========== CalDAV 相关 ==========

// CalDAVResource CalDAV 资源.
type CalDAVResource struct {
	Path         string    `json:"path"`
	ETag         string    `json:"etag"`
	ContentType  string    `json:"content_type"`
	LastModified time.Time `json:"last_modified"`
}

// CalDAVCalendarInfo CalDAV 日历信息.
type CalDAVCalendarInfo struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	CTag        string `json:"ctag"` // 日历变更标签
}
