package cloudsync

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler 调度器
type Scheduler struct {
	mu       sync.RWMutex
	cron     *cron.Cron
	tasks    map[string]cron.EntryID
	handlers map[string]func()
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		tasks:    make(map[string]cron.EntryID),
		handlers: make(map[string]func()),
	}
}

// Run 启动调度器
func (s *Scheduler) Run() {
	s.cron.Start()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// AddCronTask 添加 Cron 任务
func (s *Scheduler) AddCronTask(taskID, expr string, handler func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已存在，先删除
	if entryID, ok := s.tasks[taskID]; ok {
		s.cron.Remove(entryID)
	}

	entryID, err := s.cron.AddFunc(expr, handler)
	if err != nil {
		return err
	}

	s.tasks[taskID] = entryID
	s.handlers[taskID] = handler

	return nil
}

// AddIntervalTask 添加间隔任务
func (s *Scheduler) AddIntervalTask(taskID, interval string, handler func()) error {
	// 解析间隔，支持格式: "1h", "30m", "1h30m" 等
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return err
	}

	// 转换为 cron 表达式
	// 例如: 1h -> "0 0 * * * *" (每小时整点)
	//      30m -> "0 */30 * * * *" (每30分钟)
	// 注意: cron 最小粒度为秒

	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已存在，先删除
	if entryID, ok := s.tasks[taskID]; ok {
		s.cron.Remove(entryID)
	}

	// 使用定时器实现
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		for range ticker.C {
			handler()
		}
	}()

	s.handlers[taskID] = handler

	return nil
}

// RemoveTask 移除任务
func (s *Scheduler) RemoveTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.tasks[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.tasks, taskID)
	}

	delete(s.handlers, taskID)
}

// GetNextRun 获取下次运行时间
func (s *Scheduler) GetNextRun(taskID string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if entryID, ok := s.tasks[taskID]; ok {
		entry := s.cron.Entry(entryID)
		return entry.Next
	}

	return time.Time{}
}

// ListTasks 列出所有调度任务
func (s *Scheduler) ListTasks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []string
	for taskID := range s.tasks {
		list = append(list, taskID)
	}
	return list
}
