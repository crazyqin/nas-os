// 备份调度器 - 避开业务高峰期
package smartlifebackup

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Scheduler 备份调度器.
type Scheduler struct {
	mu sync.RWMutex

	config *ScheduleConfig

	// 调度任务
	tasks map[string]*ScheduledTask

	// 停止信号
	stopCh  chan struct{}
	stopped bool
}

// ScheduledTask 调度任务.
type ScheduledTask struct {
	ID       string
	Schedule string
	Func     func()
	NextRun  time.Time
	LastRun  time.Time
	Enabled  bool
}

// NewScheduler 创建调度器.
func NewScheduler(config *ScheduleConfig) *Scheduler {
	if config == nil {
		config = DefaultScheduleConfig()
	}

	s := &Scheduler{
		config: config,
		tasks:  make(map[string]*ScheduledTask),
		stopCh: make(chan struct{}),
	}

	// 启动调度循环
	go s.run()

	return s
}

// Stop 停止调度器.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.stopped {
		close(s.stopCh)
		s.stopped = true
	}
}

// ScheduleTask 调度任务.
func (s *Scheduler) ScheduleTask(schedule string, taskFunc func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()

	// 解析下次运行时间
	nextRun, err := s.parseSchedule(schedule)
	if err != nil {
		return err
	}

	task := &ScheduledTask{
		ID:       id,
		Schedule: schedule,
		Func:     taskFunc,
		NextRun:  nextRun,
		Enabled:  true,
	}

	s.tasks[id] = task

	log.Printf("[Scheduler] 任务已调度：%s, 下次运行：%s", id, nextRun.Format(time.RFC3339))
	return nil
}

// UnscheduleTask 取消调度任务.
func (s *Scheduler) UnscheduleTask(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tasks, id)
	log.Printf("[Scheduler] 任务已取消：%s", id)
}

// GetConfig 获取调度配置.
func (s *Scheduler) GetConfig() *ScheduleConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig 更新调度配置.
func (s *Scheduler) UpdateConfig(config *ScheduleConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	log.Printf("[Scheduler] 配置已更新")
	return nil
}

// IsWithinAllowedWindow 检查当前时间是否在允许执行的窗口内.
func (s *Scheduler) IsWithinAllowedWindow() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	hour := now.Hour()
	weekday := int(now.Weekday())

	// 检查是否在高峰时段
	for _, peak := range s.config.PeakHours {
		if s.isInTimeRange(hour, peak.StartHour, peak.EndHour) {
			// 检查是否在指定日期
			if len(peak.Days) > 0 {
				for _, day := range peak.Days {
					if day == weekday {
						return false // 在高峰时段
					}
				}
			}
		}
	}

	// 检查是否在允许的窗口内
	if len(s.config.AllowedWindows) == 0 {
		return true // 没有配置允许窗口，默认允许
	}

	for _, window := range s.config.AllowedWindows {
		if s.isInTimeRange(hour, window.StartHour, window.EndHour) {
			return true
		}
	}

	return false
}

// GetNextAllowedWindow 获取下一个允许执行的时间窗口.
func (s *Scheduler) GetNextAllowedWindow() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()

	// 遍历未来24小时，找到第一个允许的时间点
	for i := 0; i < 24*60; i++ {
		checkTime := now.Add(time.Duration(i) * time.Minute)
		hour := checkTime.Hour()
		weekday := int(checkTime.Weekday())

		// 检查是否在高峰时段
		isPeak := false
		for _, peak := range s.config.PeakHours {
			if s.isInTimeRange(hour, peak.StartHour, peak.EndHour) {
				if len(peak.Days) > 0 {
					for _, day := range peak.Days {
						if day == weekday {
							isPeak = true
							break
						}
					}
				}
			}
		}

		if isPeak {
			continue
		}

		// 检查是否在允许窗口内
		for _, window := range s.config.AllowedWindows {
			if s.isInTimeRange(hour, window.StartHour, window.EndHour) {
				// 调整到整点或半点
				minute := checkTime.Minute()
				if minute < 30 {
					checkTime = checkTime.Add(time.Duration(30-minute) * time.Minute)
				} else {
					checkTime = checkTime.Add(time.Duration(60-minute) * time.Minute)
				}
				return checkTime
			}
		}
	}

	// 如果找不到，默认明天凌晨2点
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 2, 0, 0, 0, now.Location())
}

// isInTimeRange 检查小时是否在时间范围内（支持跨天）.
func (s *Scheduler) isInTimeRange(hour, start, end int) bool {
	if start <= end {
		return hour >= start && hour < end
	}
	// 跨天情况（如 22:00 - 06:00）
	return hour >= start || hour < end
}

// run 运行调度循环.
func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndExecuteTasks()
		}
	}
}

// checkAndExecuteTasks 检查并执行任务.
func (s *Scheduler) checkAndExecuteTasks() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()

	for _, task := range s.tasks {
		if !task.Enabled {
			continue
		}

		// 检查是否到了执行时间
		if now.After(task.NextRun) {
			// 检查是否在允许的窗口内
			if !s.IsWithinAllowedWindow() {
				log.Printf("[Scheduler] 跳过任务 %s - 当前在高峰时段", task.ID)
				// 推迟到下一个允许窗口
				task.NextRun = s.GetNextAllowedWindow()
				continue
			}

			// 执行任务
			log.Printf("[Scheduler] 执行任务：%s", task.ID)
			task.LastRun = now

			// 在新协程中执行，避免阻塞
			go func(t *ScheduledTask) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Scheduler] 任务执行 panic：%v", r)
					}
				}()
				t.Func()
			}(task)

			// 计算下次运行时间
			nextRun, err := s.parseSchedule(task.Schedule)
			if err != nil {
				log.Printf("[Scheduler] 解析调度表达式失败：%v", err)
				task.Enabled = false
			} else {
				task.NextRun = nextRun
			}
		}
	}
}

// parseSchedule 解析调度表达式
// 简化版本，支持以下格式：
// - "@hourly" - 每小时
// - "@daily" - 每天
// - "@weekly" - 每周
// - "@monthly" - 每月
// - "H:M" - 每天指定时间（如 "02:00"）.
func (s *Scheduler) parseSchedule(schedule string) (time.Time, error) {
	now := time.Now()

	switch schedule {
	case "@hourly":
		// 下一个整点
		next := now.Truncate(time.Hour).Add(time.Hour)
		return next, nil

	case "@daily":
		// 明天凌晨2点（避开高峰）
		tomorrow := now.AddDate(0, 0, 1)
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 2, 0, 0, 0, now.Location()), nil

	case "@weekly":
		// 下周一凌晨2点
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		nextMonday := now.AddDate(0, 0, daysUntilMonday)
		return time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 2, 0, 0, 0, now.Location()), nil

	case "@monthly":
		// 下个月1号凌晨2点
		nextMonth := now.AddDate(0, 1, 0)
		return time.Date(nextMonth.Year(), nextMonth.Month(), 1, 2, 0, 0, 0, now.Location()), nil

	default:
		// 尝试解析 "H:M" 格式
		var hour, minute int
		_, err := fmt.Sscanf(schedule, "%d:%d", &hour, &minute)
		if err != nil {
			return time.Time{}, fmt.Errorf("不支持的调度格式：%s", schedule)
		}

		// 计算今天的这个时间
		target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

		// 如果已经过了，明天再执行
		if target.Before(now) {
			target = target.AddDate(0, 0, 1)
		}

		return target, nil
	}
}

// GetScheduledTasks 获取所有调度任务.
func (s *Scheduler) GetScheduledTasks() map[string]*ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make(map[string]*ScheduledTask)
	for k, v := range s.tasks {
		tasks[k] = v
	}
	return tasks
}

// EnableTask 启用任务.
func (s *Scheduler) EnableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("任务不存在：%s", id)
	}

	task.Enabled = true
	return nil
}

// DisableTask 禁用任务.
func (s *Scheduler) DisableTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("任务不存在：%s", id)
	}

	task.Enabled = false
	return nil
}

// GetStats 获取调度器统计.
func (s *Scheduler) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabledCount := 0
	nextRun := time.Time{}

	for _, task := range s.tasks {
		if task.Enabled {
			enabledCount++
		}
		if !task.NextRun.IsZero() {
			if nextRun.IsZero() || task.NextRun.Before(nextRun) {
				nextRun = task.NextRun
			}
		}
	}

	return map[string]interface{}{
		"total_tasks":   len(s.tasks),
		"enabled_tasks": enabledCount,
		"next_run":      nextRun,
		"is_in_window":  s.IsWithinAllowedWindow(),
		"next_window":   s.GetNextAllowedWindow(),
	}
}
