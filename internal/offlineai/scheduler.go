// Package offlineai 任务调度器，支持后台任务、定时任务和优先级队列
package offlineai

import (
	"container/heap"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler 任务调度器
type Scheduler struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	tasks   map[string]*Task
	queue   *taskHeap
	workers int
	stopCh  chan struct{}
	running bool
	taskCh  chan *Task
	handler TaskHandler
}

// TaskHandler 任务处理函数
type TaskHandler func(ctx context.Context, task *Task) (interface{}, error)

// taskHeap 优先级队列（最小堆，priority 值越大优先级越高）
type taskHeap []*Task

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	// 优先级高的排前面（反转最小堆）
	return h[i].Priority > h[j].Priority
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *taskHeap) Push(x interface{}) {
	*h = append(*h, x.(*Task))
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// NewScheduler 创建任务调度器
func NewScheduler(logger *zap.Logger, workers int, handler TaskHandler) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	if workers <= 0 {
		workers = 4
	}

	h := &taskHeap{}
	heap.Init(h)

	return &Scheduler{
		logger:  logger,
		tasks:   make(map[string]*Task),
		queue:   h,
		workers: workers,
		stopCh:  make(chan struct{}),
		taskCh:  make(chan *Task, 100),
		handler: handler,
	}
}

// Start 启动调度器
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("starting task scheduler", zap.Int("workers", s.workers))

	// 启动工作线程
	for i := 0; i < s.workers; i++ {
		go s.worker(ctx, i)
	}

	// 启动定时任务检查
	go s.scheduleChecker(ctx)

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopCh)
	s.logger.Info("task scheduler stopped")
}

// Submit 提交任务
func (s *Scheduler) Submit(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler not running")
	}

	if task.ID == "" {
		task.ID = generateTaskID()
	}
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.MaxAttempts <= 0 {
		task.MaxAttempts = 3
	}
	task.CreatedAt = time.Now()

	s.tasks[task.ID] = task
	heap.Push(s.queue, task)

	s.logger.Debug("task submitted",
		zap.String("id", task.ID),
		zap.String("name", task.Name),
		zap.Int("priority", int(task.Priority)),
	)

	// 通知工作线程（定时任务不立即通知，由 scheduleChecker 处理）
	if task.ScheduledAt == nil {
		select {
		case s.taskCh <- task:
		default:
		}
	}

	return nil
}

// Cancel 取消任务
func (s *Scheduler) Cancel(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == TaskStatusRunning {
		return fmt.Errorf("cannot cancel running task %s", taskID)
	}

	task.Status = TaskStatusCancelled
	now := time.Now()
	task.FinishedAt = &now

	return nil
}

// GetTask 获取任务信息
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// ListTasks 列出所有任务
func (s *Scheduler) ListTasks(status TaskStatus) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range s.tasks {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}
	return result
}

// worker 工作线程
func (s *Scheduler) worker(ctx context.Context, id int) {
	s.logger.Debug("worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-s.taskCh:
			s.processNext(ctx, id)
		}
	}
}

// processNext 处理队列中的下一个任务
func (s *Scheduler) processNext(ctx context.Context, workerID int) {
	s.mu.Lock()
	if s.queue.Len() == 0 {
		s.mu.Unlock()
		return
	}

	task := heap.Pop(s.queue).(*Task)
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	s.mu.Unlock()

	s.logger.Info("processing task",
		zap.String("task_id", task.ID),
		zap.Int("worker_id", workerID),
	)

	// 执行任务
	var result interface{}
	var err error

	if s.handler != nil {
		result, err = s.handler(ctx, task)
	}

	s.mu.Lock()
	finishTime := time.Now()
	task.FinishedAt = &finishTime

	if err != nil {
		task.Attempts++
		task.Error = err.Error()

		if task.Attempts < task.MaxAttempts {
			// 重试
			task.Status = TaskStatusPending
			task.StartedAt = nil
			heap.Push(s.queue, task)
			s.logger.Warn("task failed, retrying",
				zap.String("task_id", task.ID),
				zap.Int("attempt", task.Attempts),
				zap.Error(err),
			)
		} else {
			task.Status = TaskStatusFailed
			s.logger.Error("task failed permanently",
				zap.String("task_id", task.ID),
				zap.Error(err),
			)
		}
	} else {
		task.Status = TaskStatusCompleted
		task.Result = result
		s.logger.Info("task completed",
			zap.String("task_id", task.ID),
			zap.Duration("duration", finishTime.Sub(*task.StartedAt)),
		)
	}
	s.mu.Unlock()
}

// scheduleChecker 定时任务检查
func (s *Scheduler) scheduleChecker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkScheduledTasks()
		}
	}
}

// checkScheduledTasks 检查定时任务
func (s *Scheduler) checkScheduledTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, task := range s.tasks {
		if task.Status != TaskStatusPending {
			continue
		}
		if task.ScheduledAt != nil && task.ScheduledAt.After(now) {
			continue
		}
		// 定时任务到时，加入队列
		if task.ScheduledAt != nil {
			task.ScheduledAt = nil
			heap.Push(s.queue, task)
		}
	}
}

// GetStats 获取调度器统计
func (s *Scheduler) GetStats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]int{
		"total":      len(s.tasks),
		"pending":    0,
		"running":    0,
		"completed":  0,
		"failed":     0,
		"cancelled":  0,
		"queue_size": s.queue.Len(),
	}

	for _, task := range s.tasks {
		switch task.Status {
		case TaskStatusPending:
			stats["pending"]++
		case TaskStatusRunning:
			stats["running"]++
		case TaskStatusCompleted:
			stats["completed"]++
		case TaskStatusFailed:
			stats["failed"]++
		case TaskStatusCancelled:
			stats["cancelled"]++
		}
	}

	return stats
}

// generateTaskID 生成任务 ID
func generateTaskID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "task-" + hex.EncodeToString(b)
}
