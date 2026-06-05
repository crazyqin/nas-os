// Package websharepro - 批量操作模块
// 提供批量文件操作、并行执行、进度跟踪、回滚支持
package websharepro

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// BatchOperationType 批量操作类型
type BatchOperationType string

const (
	BatchOpCopy      BatchOperationType = "copy"
	BatchOpMove      BatchOperationType = "move"
	BatchOpDelete    BatchOperationType = "delete"
	BatchOpMkdir     BatchOperationType = "mkdir"
	BatchOpChmod     BatchOperationType = "chmod"
	BatchOpChown     BatchOperationType = "chown"
	BatchOpCompress  BatchOperationType = "compress"
	BatchOpExtract   BatchOperationType = "extract"
	BatchOpRename    BatchOperationType = "rename"
	BatchOpShare     BatchOperationType = "share"
)

// BatchTaskStatus 任务状态
type BatchTaskStatus string

const (
	TaskPending    BatchTaskStatus = "pending"
	TaskRunning    BatchTaskStatus = "running"
	TaskCompleted  BatchTaskStatus = "completed"
	TaskFailed     BatchTaskStatus = "failed"
	TaskCancelled  BatchTaskStatus = "cancelled"
	TaskRolledBack BatchTaskStatus = "rolledback"
)

// BatchOperation 批量操作项
type BatchOperation struct {
	ID          string             `json:"id"`
	Source      string             `json:"source"`
	Destination string             `json:"destination,omitempty"`
	Type        BatchOperationType `json:"type"`
	Options     map[string]any     `json:"options,omitempty"`
}

// BatchTask 批量任务
type BatchTask struct {
	ID             string             `json:"id"`
	Operations     []*BatchOperation  `json:"operations"`
	Status         BatchTaskStatus    `json:"status"`
	TotalOps       int                `json:"totalOps"`
	CompletedOps   int                `json:"completedOps"`
	FailedOps      int                `json:"failedOps"`
	SkippedOps     int                `json:"skippedOps"`
	Progress       float64            `json:"progress"`
	StartTime      time.Time          `json:"startTime"`
	EndTime        *time.Time         `json:"endTime,omitempty"`
	ElapsedTime    time.Duration      `json:"elapsedTime"`
	EstRemaining   time.Duration      `json:"estRemaining"`
	Errors         []BatchError       `json:"errors,omitempty"`
	Results        []BatchResult      `json:"results,omitempty"`
	Concurrency    int                `json:"concurrency"`
	Author         string             `json:"author"`
	CreatedAt      time.Time          `json:"createdAt"`
	cancel         context.CancelFunc `json:"-"`
	cancelMu       sync.Mutex         `json:"-"`
}

// BatchError 批量操作错误
type BatchError struct {
	OperationID string `json:"operationId"`
	Source      string `json:"source"`
	Error       string `json:"error"`
	Timestamp   time.Time `json:"timestamp"`
}

// BatchResult 批量操作结果
type BatchResult struct {
	OperationID string    `json:"operationId"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
}

// BatchProgressCallback 进度回调
type BatchProgressCallback func(task *BatchTask)

// BatchConfig 批量操作配置
type BatchConfig struct {
	DefaultConcurrency int              `json:"defaultConcurrency"` // 默认并发数
	MaxConcurrency     int              `json:"maxConcurrency"`     // 最大并发数
	RetryAttempts      int              `json:"retryAttempts"`      // 重试次数
	RetryDelay         time.Duration    `json:"retryDelay"`         // 重试延迟
	StopOnError        bool             `json:"stopOnError"`        // 遇错停止
	EnableRollback     bool             `json:"enableRollback"`     // 启用回滚
	ProgressInterval   time.Duration    `json:"progressInterval"`   // 进度回调间隔
}

// BatchExecutor 批量执行器
type BatchExecutor struct {
	mu        sync.RWMutex
	tasks     map[string]*BatchTask
	config    *BatchConfig
	executors map[BatchOperationType]OperationExecutor
	progress  []BatchProgressCallback
}

// OperationExecutor 操作执行器接口
type OperationExecutor interface {
	Execute(ctx context.Context, op *BatchOperation) error
	Rollback(ctx context.Context, op *BatchOperation) error
}

// NewBatchExecutor 创建批量执行器
func NewBatchExecutor(config *BatchConfig) *BatchExecutor {
	if config == nil {
		config = &BatchConfig{
			DefaultConcurrency: 4,
			MaxConcurrency:     16,
			RetryAttempts:      3,
			RetryDelay:         time.Second,
			StopOnError:        false,
			EnableRollback:     true,
			ProgressInterval:   100 * time.Millisecond,
		}
	}

	executor := &BatchExecutor{
		tasks:     make(map[string]*BatchTask),
		config:    config,
		executors: make(map[BatchOperationType]OperationExecutor),
		progress:  make([]BatchProgressCallback, 0),
	}

	// 注册默认执行器
	executor.executors[BatchOpCopy] = &CopyExecutor{}
	executor.executors[BatchOpMove] = &MoveExecutor{}
	executor.executors[BatchOpDelete] = &DeleteExecutor{}
	executor.executors[BatchOpMkdir] = &MkdirExecutor{}
	executor.executors[BatchOpCompress] = &CompressExecutor{}
	executor.executors[BatchOpShare] = &ShareExecutor{}

	return executor
}

// RegisterExecutor 注册操作执行器
func (e *BatchExecutor) RegisterExecutor(opType BatchOperationType, executor OperationExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executors[opType] = executor
}

// OnProgress 注册进度回调
func (e *BatchExecutor) OnProgress(callback BatchProgressCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.progress = append(e.progress, callback)
}

// Submit 提交批量任务
func (e *BatchExecutor) Submit(operations []*BatchOperation, author string) (*BatchTask, error) {
	if len(operations) == 0 {
		return nil, errors.New("no operations provided")
	}

	concurrency := e.config.DefaultConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > e.config.MaxConcurrency {
		concurrency = e.config.MaxConcurrency
	}

	taskID := fmt.Sprintf("batch-%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())

	task := &BatchTask{
		ID:          taskID,
		Operations:  operations,
		Status:      TaskPending,
		TotalOps:    len(operations),
		Concurrency: concurrency,
		Author:      author,
		CreatedAt:   time.Now(),
		cancel:      cancel,
	}

	// 为每个操作生成 ID
	for i, op := range operations {
		if op.ID == "" {
			op.ID = fmt.Sprintf("%s-op-%d", taskID, i)
		}
	}

	e.mu.Lock()
	e.tasks[taskID] = task
	e.mu.Unlock()

	// 异步执行
	go e.executeTask(ctx, task)

	return task, nil
}

// Cancel 取消任务
func (e *BatchExecutor) Cancel(taskID string) error {
	e.mu.RLock()
	task, exists := e.tasks[taskID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == TaskCompleted || task.Status == TaskCancelled {
		return fmt.Errorf("task already %s", task.Status)
	}

	task.cancel()
	task.Status = TaskCancelled

	now := time.Now()
	task.EndTime = &now
	task.ElapsedTime = now.Sub(task.StartTime)

	return nil
}

// GetTask 获取任务状态
func (e *BatchExecutor) GetTask(taskID string) (*BatchTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// 返回副本（不拷贝锁）
	taskCopy := BatchTask{
		ID:           task.ID,
		Operations:   make([]*BatchOperation, len(task.Operations)),
		Status:       task.Status,
		TotalOps:     task.TotalOps,
		CompletedOps: task.CompletedOps,
		FailedOps:    task.FailedOps,
		SkippedOps:   task.SkippedOps,
		Progress:     task.Progress,
		StartTime:    task.StartTime,
		EndTime:      task.EndTime,
		ElapsedTime:  task.ElapsedTime,
		EstRemaining: task.EstRemaining,
		Errors:       make([]BatchError, len(task.Errors)),
		Results:      make([]BatchResult, len(task.Results)),
		Concurrency:  task.Concurrency,
		Author:       task.Author,
		CreatedAt:    task.CreatedAt,
	}
	copy(taskCopy.Operations, task.Operations)
	copy(taskCopy.Results, task.Results)
	copy(taskCopy.Errors, task.Errors)

	return &taskCopy, nil
}

// ListTasks 列出所有任务
func (e *BatchExecutor) ListTasks() []*BatchTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]*BatchTask, 0, len(e.tasks))
	for _, task := range e.tasks {
		taskCopy := BatchTask{
			ID:           task.ID,
			Operations:   task.Operations,
			Status:       task.Status,
			TotalOps:     task.TotalOps,
			CompletedOps: task.CompletedOps,
			FailedOps:    task.FailedOps,
			SkippedOps:   task.SkippedOps,
			Progress:     task.Progress,
			StartTime:    task.StartTime,
			EndTime:      task.EndTime,
			ElapsedTime:  task.ElapsedTime,
			EstRemaining: task.EstRemaining,
			Errors:       task.Errors,
			Results:      task.Results,
			Concurrency:  task.Concurrency,
			Author:       task.Author,
			CreatedAt:    task.CreatedAt,
		}
		tasks = append(tasks, &taskCopy)
	}
	return tasks
}

// RollbackTask 回滚任务
func (e *BatchExecutor) RollbackTask(taskID string) error {
	e.mu.RLock()
	task, exists := e.tasks[taskID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if !e.config.EnableRollback {
		return errors.New("rollback not enabled")
	}

	if task.Status != TaskFailed && task.Status != TaskCancelled {
		return fmt.Errorf("cannot rollback task with status: %s", task.Status)
	}

	// 回滚已完成的操作（逆序）
	for i := len(task.Results) - 1; i >= 0; i-- {
		result := task.Results[i]
		if result.Status != "completed" {
			continue
		}

		// 找到对应的操作
		for _, op := range task.Operations {
			if op.ID == result.OperationID {
				executor, exists := e.executors[op.Type]
				if !exists {
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := executor.Rollback(ctx, op); err != nil {
					task.Errors = append(task.Errors, BatchError{
						OperationID: op.ID,
						Source:      op.Source,
						Error:       fmt.Sprintf("rollback failed: %v", err),
						Timestamp:   time.Now(),
					})
				}
				cancel()
				break
			}
		}
	}

	task.Status = TaskRolledBack
	return nil
}

// executeTask 执行批量任务
func (e *BatchExecutor) executeTask(ctx context.Context, task *BatchTask) {
	task.StartTime = time.Now()
	task.Status = TaskRunning

	// 创建工作池
	sem := make(chan struct{}, task.Concurrency)
	var wg sync.WaitGroup

	var completed atomic.Int64
	var failed atomic.Int64

	// 进度通知协程
	doneCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(e.config.ProgressInterval)
		defer ticker.Stop()
		for {
			select {
			case <-doneCh:
				return
			case <-ticker.C:
				task.mu().Lock()
				task.CompletedOps = int(completed.Load())
				task.FailedOps = int(failed.Load())
				if task.TotalOps > 0 {
					task.Progress = float64(task.CompletedOps) / float64(task.TotalOps) * 100
				}
				elapsed := time.Since(task.StartTime)
				task.ElapsedTime = elapsed
				if task.CompletedOps > 0 && task.Progress < 100 {
					perOp := elapsed / time.Duration(task.CompletedOps)
					remaining := task.TotalOps - task.CompletedOps
					task.EstRemaining = perOp * time.Duration(remaining)
				}
				task.mu().Unlock()

				e.notifyProgress(task)
			}
		}
	}()

	for _, op := range task.Operations {
		select {
		case <-ctx.Done():
			break
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(operation *BatchOperation) {
			defer wg.Done()
			defer func() { <-sem }()

			result := e.executeOperation(ctx, operation)

			task.mu().Lock()
			task.Results = append(task.Results, *result)
			task.mu().Unlock()

			if result.Status == "completed" {
				completed.Add(1)
			} else {
				failed.Add(1)
				if e.config.StopOnError {
					task.cancel()
				}
			}
		}(op)
	}

	wg.Wait()
	close(doneCh)

	// 更新最终状态
	now := time.Now()
	task.EndTime = &now
	task.ElapsedTime = now.Sub(task.StartTime)
	task.CompletedOps = int(completed.Load())
	task.FailedOps = int(failed.Load())
	task.Progress = 100

	if task.FailedOps > 0 {
		task.Status = TaskFailed
	} else {
		task.Status = TaskCompleted
	}

	e.notifyProgress(task)
}

// executeOperation 执行单个操作
func (e *BatchExecutor) executeOperation(ctx context.Context, op *BatchOperation) *BatchResult {
	start := time.Now()

	executor, exists := e.executors[op.Type]
	if !exists {
		return &BatchResult{
			OperationID: op.ID,
			Status:      "failed",
			Message:     fmt.Sprintf("unsupported operation type: %s", op.Type),
			Timestamp:   time.Now(),
			Duration:    time.Since(start),
		}
	}

	// 重试逻辑
	var lastErr error
	for attempt := 0; attempt <= e.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(e.config.RetryDelay)
		}

		err := executor.Execute(ctx, op)
		if err == nil {
			return &BatchResult{
				OperationID: op.ID,
				Status:      "completed",
				Timestamp:   time.Now(),
				Duration:    time.Since(start),
			}
		}
		lastErr = err
	}

	return &BatchResult{
		OperationID: op.ID,
		Status:      "failed",
		Message:     lastErr.Error(),
		Timestamp:   time.Now(),
		Duration:    time.Since(start),
	}
}

// notifyProgress 通知进度
func (e *BatchExecutor) notifyProgress(task *BatchTask) {
	e.mu.RLock()
	callbacks := e.progress
	e.mu.RUnlock()

	for _, cb := range callbacks {
		cb(task)
	}
}

// mu 获取任务锁
func (t *BatchTask) mu() *sync.Mutex {
	return &t.cancelMu
}

// ---- 操作执行器实现 ----

// CopyExecutor 复制操作执行器
type CopyExecutor struct{}

func (e *CopyExecutor) Execute(_ context.Context, _ *BatchOperation) error {
	// 实际实现会调用文件系统
	return errors.New("copy: not implemented")
}

func (e *CopyExecutor) Rollback(_ context.Context, _ *BatchOperation) error {
	return errors.New("copy rollback: not implemented")
}

// MoveExecutor 移动操作执行器
type MoveExecutor struct{}

func (e *MoveExecutor) Execute(_ context.Context, _ *BatchOperation) error {
	return errors.New("move: not implemented")
}

func (e *MoveExecutor) Rollback(_ context.Context, _ *BatchOperation) error {
	return errors.New("move rollback: not implemented")
}

// DeleteExecutor 删除操作执行器
type DeleteExecutor struct{}

func (e *DeleteExecutor) Execute(_ context.Context, _ *BatchOperation) error {
	return errors.New("delete: not implemented")
}

func (e *DeleteExecutor) Rollback(_ context.Context, _ *BatchOperation) error {
	return errors.New("delete rollback: not implemented")
}

// MkdirExecutor 创建目录执行器
type MkdirExecutor struct{}

func (e *MkdirExecutor) Execute(_ context.Context, _ *BatchOperation) error {
	return errors.New("mkdir: not implemented")
}

func (e *MkdirExecutor) Rollback(_ context.Context, _ *BatchOperation) error {
	return errors.New("mkdir rollback: not implemented")
}

// CompressExecutor 压缩操作执行器
type CompressExecutor struct{}

func (e *CompressExecutor) Execute(_ context.Context, _ *BatchOperation) error {
	return errors.New("compress: not implemented")
}

func (e *CompressExecutor) Rollback(_ context.Context, _ *BatchOperation) error {
	return errors.New("compress rollback: not implemented")
}

// ShareExecutor 分享操作执行器
type ShareExecutor struct{}

func (e *ShareExecutor) Execute(_ context.Context, _ *BatchOperation) error {
	return errors.New("share: not implemented")
}

func (e *ShareExecutor) Rollback(_ context.Context, _ *BatchOperation) error {
	return errors.New("share rollback: not implemented")
}

// GetTaskProgress 获取任务进度
func (e *BatchExecutor) GetTaskProgress(taskID string) (float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return 0, fmt.Errorf("task not found: %s", taskID)
	}

	return task.Progress, nil
}

// CleanCompleted 清理已完成的任务
func (e *BatchExecutor) CleanCompleted() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	count := 0
	for id, task := range e.tasks {
		if task.Status == TaskCompleted || task.Status == TaskCancelled || task.Status == TaskRolledBack {
			delete(e.tasks, id)
			count++
		}
	}
	return count
}

// GetStats 获取执行器统计
func (e *BatchExecutor) GetStats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := map[string]any{
		"totalTasks": len(e.tasks),
	}

	statusCounts := make(map[BatchTaskStatus]int)
	for _, task := range e.tasks {
		statusCounts[task.Status]++
	}
	stats["statusCounts"] = statusCounts

	return stats
}
