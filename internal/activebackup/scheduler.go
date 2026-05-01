// Package activebackup 提供整机备份管理功能
package activebackup

import (
	"sync"
	"time"
)

// Scheduler 备份调度引擎.
type Scheduler struct {
	mu      sync.RWMutex
	manager *Manager
	quit    chan struct{}
	running bool
}

// NewScheduler 创建备份调度引擎.
func NewScheduler(mgr *Manager) *Scheduler {
	return &Scheduler{
		manager: mgr,
		quit:    make(chan struct{}),
	}
}

// Start 启动调度器.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.run()
}

// Stop 停止调度器.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.quit)
	s.quit = make(chan struct{})
}

// IsRunning 调度器是否运行中.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// run 调度主循环.
func (s *Scheduler) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			s.checkScheduledTasks()
		}
	}
}

// checkScheduledTasks 检查定时任务.
func (s *Scheduler) checkScheduledTasks() {
	tasks := s.manager.ListTasks()
	now := time.Now()

	for _, task := range tasks {
		if !task.Enabled {
			continue
		}
		if task.ScheduleType != ScheduleTypeScheduled {
			continue
		}
		if task.Status == TaskStatusRunning {
			continue
		}

		// 检查是否到达执行时间
		if task.NextRunAt != nil && now.After(*task.NextRunAt) {
			_, _ = s.manager.RunTask(task.ID)
		}
	}
}

// CalculateNextRun 计算下次运行时间.
func (s *Scheduler) CalculateNextRun(schedule string, from time.Time) *time.Time {
	// 简化实现：根据 cron 表达式解析
	// 实际生产中应使用 cron 解析库（如 robfig/cron）
	if schedule == "" {
		return nil
	}

	// 默认每天凌晨2点
	next := time.Date(from.Year(), from.Month(), from.Day()+1, 2, 0, 0, 0, from.Location())
	return &next
}

// UpdateNextRun 更新任务的下次运行时间.
func (s *Scheduler) UpdateNextRun(taskID string) {
	s.manager.mu.Lock()
	defer s.manager.mu.Unlock()

	task, exists := s.manager.tasks[taskID]
	if !exists {
		return
	}

	if task.ScheduleType != ScheduleTypeScheduled || task.Schedule == "" {
		return
	}

	nextRun := s.CalculateNextRun(task.Schedule, time.Now())
	task.NextRunAt = nextRun
}

// ScheduleStats 调度器统计.
type ScheduleStats struct {
	Running        bool `json:"running"`          // 调度器是否运行中
	ScheduledTasks int  `json:"scheduled_tasks"`  // 定时任务数量
	ManualTasks    int  `json:"manual_tasks"`     // 手动任务数量
	EventTasks     int  `json:"event_tasks"`      // 事件任务数量
	NextRunAt      *time.Time `json:"next_run_at,omitempty"` // 最近一次执行时间
}

// GetStats 获取调度统计.
func (s *Scheduler) GetStats() ScheduleStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ScheduleStats{
		Running: s.running,
	}

	tasks := s.manager.ListTasks()
	for _, t := range tasks {
		switch t.ScheduleType {
		case ScheduleTypeScheduled:
			stats.ScheduledTasks++
		case ScheduleTypeManual:
			stats.ManualTasks++
		case ScheduleTypeEvent:
			stats.EventTasks++
		}

		if t.NextRunAt != nil {
			if stats.NextRunAt == nil || t.NextRunAt.Before(*stats.NextRunAt) {
				stats.NextRunAt = t.NextRunAt
			}
		}
	}

	return stats
}
