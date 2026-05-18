package cloudsyncmgr

import (
	"sync"
	"time"
)

// TaskScheduler 同步任务调度器.
type TaskScheduler struct {
	mu       sync.RWMutex
	timers   map[string]*time.Ticker
	stopChs  map[string]chan struct{}
	handlers map[string]func()
	running  bool
}

// NewTaskScheduler 创建调度器.
func NewTaskScheduler() *TaskScheduler {
	return &TaskScheduler{
		timers:   make(map[string]*time.Ticker),
		stopChs:  make(map[string]chan struct{}),
		handlers: make(map[string]func()),
		running:  true,
	}
}

// AddIntervalTask 添加间隔执行任务.
func (ts *TaskScheduler) AddIntervalTask(taskID string, interval time.Duration, handler func()) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// 如果已存在，先移除
	ts.removeLocked(taskID)

	ticker := time.NewTicker(interval)
	stopCh := make(chan struct{})
	ts.timers[taskID] = ticker
	ts.stopChs[taskID] = stopCh
	ts.handlers[taskID] = handler

	go func() {
		for {
			select {
			case <-ticker.C:
				handler()
			case <-stopCh:
				return
			}
		}
	}()
}

// AddDelayedTask 添加延迟执行任务（只执行一次）.
func (ts *TaskScheduler) AddDelayedTask(taskID string, delay time.Duration, handler func()) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.removeLocked(taskID)

	stopCh := make(chan struct{})
	ts.stopChs[taskID] = stopCh

	go func() {
		select {
		case <-time.After(delay):
			handler()
		case <-stopCh:
			return
		}
	}()
}

// Remove 移除任务.
func (ts *TaskScheduler) Remove(taskID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.removeLocked(taskID)
}

func (ts *TaskScheduler) removeLocked(taskID string) {
	if ticker, ok := ts.timers[taskID]; ok {
		ticker.Stop()
		delete(ts.timers, taskID)
	}
	if ch, ok := ts.stopChs[taskID]; ok {
		close(ch)
		delete(ts.stopChs, taskID)
	}
	delete(ts.handlers, taskID)
}

// HasTask 检查任务是否存在.
func (ts *TaskScheduler) HasTask(taskID string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	_, ok := ts.handlers[taskID]
	return ok
}

// ActiveTasks 返回活跃任务数量.
func (ts *TaskScheduler) ActiveTasks() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.handlers)
}

// Stop 停止所有调度任务.
func (ts *TaskScheduler) Stop() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for taskID := range ts.handlers {
		ts.removeLocked(taskID)
	}
	ts.running = false
}

// IsRunning 返回调度器是否在运行.
func (ts *TaskScheduler) IsRunning() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.running
}
