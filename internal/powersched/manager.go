// Package powersched 提供电源调度功能
// 定时开机/关机、周期重启、休眠唤醒、节假日调度、UPS联动
package powersched

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// TaskType 任务类型
type TaskType string

const (
	TaskTypePowerOn  TaskType = "power_on"  // 开机
	TaskTypePowerOff TaskType = "power_off" // 关机
	TaskTypeRestart  TaskType = "restart"   // 重启
	TaskTypeHibernate TaskType = "hibernate" // 休眠
	TaskTypeWakeUp   TaskType = "wake_up"   // 唤醒
)

// ScheduleRule 调度规则
type ScheduleRule struct {
	Type     string `json:"type"`     // once/daily/weekly/monthly/custom
	Weekday  int    `json:"weekday"`  // 0=周日, 1-6=周一到周六
	DayOfMonth int  `json:"dayOfMonth"` // 每月几号
	Time     string `json:"time"`     // HH:MM 格式
	Date     string `json:"date"`     // YYYY-MM-DD 一次性任务用
	CronExpr string `json:"cronExpr,omitempty"` // 自定义cron表达式
}

// ScheduleTask 调度任务
type ScheduleTask struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        TaskType     `json:"type"`
	Rule        ScheduleRule `json:"rule"`
	Enabled     bool         `json:"enabled"`
	SkipHoliday bool         `json:"skipHoliday"` // 节假日跳过
	LastRun     time.Time    `json:"lastRun,omitempty"`
	NextRun     time.Time    `json:"nextRun,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
}

// Holiday 节假日
type Holiday struct {
	Date    string `json:"date"` // YYYY-MM-DD
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// PowerEvent 电源事件
type PowerEvent struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	TaskName  string    `json:"taskName"`
	TaskType  TaskType  `json:"taskType"`
	ExecTime  time.Time `json:"execTime"`
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
}

// UPSStatus UPS状态
type UPSStatus struct {
	Battery    int   `json:"battery"`    // 电量百分比 0-100
	OnBattery  bool  `json:"onBattery"`  // 是否电池供电
	Estimated  int   `json:"estimated"`  // 预计续航（分钟）
	LoadPercent int  `json:"loadPercent"` // 负载百分比
	UpdatedAt  time.Time `json:"updatedAt"`
}

// DayTasks 日历单日任务
type DayTasks struct {
	Day     int            `json:"Day"`
	Tasks   []ScheduleTask `json:"tasks"`
	IsHoliday bool         `json:"isHoliday"`
}

// ScheduleCalendar 月度日历
type ScheduleCalendar struct {
	Year  int        `json:"year"`
	Month time.Month `json:"month"`
	Days  []DayTasks `json:"days"`
}

// ========== Manager ==========

// Manager 电源调度管理器
type Manager struct {
	mu       sync.RWMutex
	tasks    map[string]*ScheduleTask
	holidays map[string]*Holiday
	events   []PowerEvent
	ups      *UPSStatus
	nextID   int
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		tasks:    make(map[string]*ScheduleTask),
		holidays: make(map[string]*Holiday),
		ups: &UPSStatus{
			Battery:     100,
			OnBattery:   false,
			Estimated:   0,
			LoadPercent: 0,
			UpdatedAt:   time.Now(),
		},
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认配置
func (m *Manager) initDefaults() {
	// 添加一些示例节假日
	m.holidays["2026-01-01"] = &Holiday{Date: "2026-01-01", Name: "元旦", Enabled: true}
	m.holidays["2026-01-29"] = &Holiday{Date: "2026-01-29", Name: "春节", Enabled: true}
	m.holidays["2026-05-01"] = &Holiday{Date: "2026-05-01", Name: "劳动节", Enabled: true}
	m.holidays["2026-10-01"] = &Holiday{Date: "2026-10-01", Name: "国庆节", Enabled: true}
}

// generateID 生成ID
func (m *Manager) generateID(prefix string) string {
	m.nextID++
	return fmt.Sprintf("%s-%d", prefix, m.nextID)
}

// ========== 任务管理 ==========

// CreateTask 创建任务
func (m *Manager) CreateTask(task *ScheduleTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if task.Rule.Time == "" && task.Rule.CronExpr == "" {
		return fmt.Errorf("task time is required")
	}

	task.ID = m.generateID("task")
	task.CreatedAt = time.Now()
	task.NextRun = m.calculateNextRun(task)

	m.tasks[task.ID] = task
	log.Printf("[电源调度] 创建任务: %s (%s)", task.Name, task.ID)
	return nil
}

// UpdateTask 更新任务
func (m *Manager) UpdateTask(task *ScheduleTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[task.ID]; !ok {
		return fmt.Errorf("task %s not found", task.ID)
	}

	task.NextRun = m.calculateNextRun(task)
	m.tasks[task.ID] = task
	log.Printf("[电源调度] 更新任务: %s", task.ID)
	return nil
}

// DeleteTask 删除任务
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return fmt.Errorf("task %s not found", id)
	}

	delete(m.tasks, id)
	log.Printf("[电源调度] 删除任务: %s", id)
	return nil
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) *ScheduleTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil
	}
	return task
}

// ListTasks 列出所有任务
func (m *Manager) ListTasks() []ScheduleTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]ScheduleTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, *t)
	}
	return tasks
}

// EnableTask 启用任务
func (m *Manager) EnableTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}

	task.Enabled = true
	task.NextRun = m.calculateNextRun(task)
	log.Printf("[电源调度] 启用任务: %s", id)
	return nil
}

// DisableTask 禁用任务
func (m *Manager) DisableTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}

	task.Enabled = false
	log.Printf("[电源调度] 禁用任务: %s", id)
	return nil
}

// ========== 节假日管理 ==========

// AddHoliday 添加节假日
func (m *Manager) AddHoliday(h *Holiday) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h.Date == "" {
		return fmt.Errorf("holiday date is required")
	}

	h.Enabled = true
	m.holidays[h.Date] = h
	log.Printf("[电源调度] 添加节假日: %s (%s)", h.Name, h.Date)
	return nil
}

// RemoveHoliday 移除节假日
func (m *Manager) RemoveHoliday(date string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.holidays[date]; !ok {
		return fmt.Errorf("holiday %s not found", date)
	}

	delete(m.holidays, date)
	log.Printf("[电源调度] 移除节假日: %s", date)
	return nil
}

// ListHolidays 列出所有节假日
func (m *Manager) ListHolidays() []Holiday {
	m.mu.RLock()
	defer m.mu.RUnlock()

	holidays := make([]Holiday, 0, len(m.holidays))
	for _, h := range m.holidays {
		holidays = append(holidays, *h)
	}
	return holidays
}

// ========== 事件查询 ==========

// GetPowerEvents 获取电源事件
func (m *Manager) GetPowerEvents(since time.Time) []PowerEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []PowerEvent
	for _, e := range m.events {
		if e.ExecTime.After(since) || e.ExecTime.Equal(since) {
			events = append(events, e)
		}
	}
	return events
}

// ========== UPS ==========

// GetUPSStatus 获取UPS状态
func (m *Manager) GetUPSStatus() (*UPSStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.ups == nil {
		return nil, fmt.Errorf("UPS not configured")
	}

	// 模拟更新UPS状态
	ups := *m.ups
	ups.UpdatedAt = time.Now()
	return &ups, nil
}

// ========== 日历 ==========

// GetCalendar 获取月度日历
func (m *Manager) GetCalendar(year int, month time.Month) *ScheduleCalendar {
	m.mu.RLock()
	defer m.mu.RUnlock()

	calendar := &ScheduleCalendar{
		Year:  year,
		Month: month,
	}

	// 获取该月天数
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	lastDay := firstDay.AddDate(0, 1, -1)
	daysInMonth := lastDay.Day()

	calendar.Days = make([]DayTasks, daysInMonth)

	for day := 1; day <= daysInMonth; day++ {
		date := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		dateStr := date.Format("2006-01-02")

		dayTasks := DayTasks{
			Day:       day,
			IsHoliday: m.isHoliday(dateStr),
		}

		// 收集该天要执行的任务
		for _, task := range m.tasks {
			if !task.Enabled {
				continue
			}
			if m.taskRunsOnDate(task, date) {
				dayTasks.Tasks = append(dayTasks.Tasks, *task)
			}
		}

		calendar.Days[day-1] = dayTasks
	}

	return calendar
}

// ========== 冲突检测 ==========

// CheckConflicts 检测任务冲突
func (m *Manager) CheckConflicts(task *ScheduleTask) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var conflicts []string

	for _, existing := range m.tasks {
		if existing.ID == task.ID {
			continue
		}
		if !existing.Enabled {
			continue
		}

		// 同类型任务在相同时间可能冲突
		if existing.Type == task.Type && existing.Rule.Time == task.Rule.Time {
			if m.tasksOverlap(existing, task) {
				conflicts = append(conflicts, fmt.Sprintf("与任务 '%s' 时间重叠", existing.Name))
			}
		}

		// 开机后立即关机的冲突
		if (existing.Type == TaskTypePowerOn && task.Type == TaskTypePowerOff) ||
			(existing.Type == TaskTypePowerOff && task.Type == TaskTypePowerOn) {
			if existing.Rule.Time == task.Rule.Time {
				conflicts = append(conflicts, fmt.Sprintf("与任务 '%s' 开关机时间冲突", existing.Name))
			}
		}
	}

	return conflicts
}

// ========== 辅助方法 ==========

// isHoliday 检查是否是节假日
func (m *Manager) isHoliday(date string) bool {
	h, ok := m.holidays[date]
	return ok && h.Enabled
}

// taskRunsOnDate 检查任务是否在指定日期运行
func (m *Manager) taskRunsOnDate(task *ScheduleTask, date time.Time) bool {
	dateStr := date.Format("2006-01-02")

	switch task.Rule.Type {
	case "once":
		return task.Rule.Date == dateStr
	case "daily":
		if task.SkipHoliday && m.isHoliday(dateStr) {
			return false
		}
		return true
	case "weekday":
		// 工作日（周一到周五）
		weekday := date.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return false
		}
		if task.SkipHoliday && m.isHoliday(dateStr) {
			return false
		}
		return true
	case "weekly":
		return int(date.Weekday()) == task.Rule.Weekday
	case "monthly":
		return date.Day() == task.Rule.DayOfMonth
	default:
		return false
	}
}

// tasksOverlap 检查两个任务是否有重叠
func (m *Manager) tasksOverlap(a, b *ScheduleTask) bool {
	if a.Rule.Type != b.Rule.Type {
		return false
	}
	switch a.Rule.Type {
	case "once":
		return a.Rule.Date == b.Rule.Date
	case "daily":
		return true
	case "weekly":
		return a.Rule.Weekday == b.Rule.Weekday
	case "monthly":
		return a.Rule.DayOfMonth == b.Rule.DayOfMonth
	default:
		return false
	}
}

// calculateNextRun 计算下次运行时间
func (m *Manager) calculateNextRun(task *ScheduleTask) time.Time {
	now := time.Now()

	switch task.Rule.Type {
	case "once":
		t, err := time.ParseInLocation("2006-01-02 15:04", task.Rule.Date+" "+task.Rule.Time, time.Local)
		if err != nil {
			return now.Add(24 * time.Hour)
		}
		if t.Before(now) {
			return now.Add(24 * time.Hour) // 已过期
		}
		return t
	case "daily", "weekday":
		t, err := time.ParseInLocation("15:04", task.Rule.Time, time.Local)
		if err != nil {
			return now.Add(24 * time.Hour)
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next
	case "weekly":
		t, err := time.ParseInLocation("15:04", task.Rule.Time, time.Local)
		if err != nil {
			return now.Add(7 * 24 * time.Hour)
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		for int(next.Weekday()) != task.Rule.Weekday || next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next
	case "monthly":
		t, err := time.ParseInLocation("15:04", task.Rule.Time, time.Local)
		if err != nil {
			return now.AddDate(0, 1, 0)
		}
		next := time.Date(now.Year(), now.Month(), task.Rule.DayOfMonth, t.Hour(), t.Minute(), 0, 0, time.Local)
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
		return next
	default:
		return now.Add(24 * time.Hour)
	}
}

// AddPowerEvent 添加电源事件
func (m *Manager) AddPowerEvent(event *PowerEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	event.ID = fmt.Sprintf("evt-%d", len(m.events)+1)
	m.events = append(m.events, *event)
}
