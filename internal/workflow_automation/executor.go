// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Executor 工作流执行器，负责工作流执行的编排和调度.
type Executor struct {
	mu         sync.RWMutex
	engine     *Engine
	logger     *zap.Logger
	executions map[string]*Execution
	maxConcurrent int
	semaphore chan struct{}
}

// NewExecutor 创建工作流执行器.
func NewExecutor(engine *Engine, logger *zap.Logger, maxConcurrent int) *Executor {
	if logger == nil {
		logger = zap.NewNop()
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &Executor{
		engine:        engine,
		logger:        logger,
		executions:    make(map[string]*Execution),
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// Submit 提交工作流执行.
func (ex *Executor) Submit(ctx context.Context, workflowID string, trigger *TriggerEvent) (*Execution, error) {
	// 获取信号量（限制并发）
	select {
	case ex.semaphore <- struct{}{}:
		defer func() { <-ex.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	exec, err := ex.engine.Execute(ctx, workflowID, trigger)
	if err != nil {
		return nil, fmt.Errorf("execute workflow: %w", err)
	}

	ex.mu.Lock()
	ex.executions[exec.ID] = exec
	ex.mu.Unlock()

	return exec, nil
}

// SubmitSync 同步提交工作流执行并等待完成.
func (ex *Executor) SubmitSync(ctx context.Context, workflowID string, trigger *TriggerEvent) (*Execution, error) {
	// 获取信号量
	select {
	case ex.semaphore <- struct{}{}:
		defer func() { <-ex.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 同步执行
	wf, err := ex.engine.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}

	if wf.Status != StatusActive {
		return nil, fmt.Errorf("workflow %q is not active", workflowID)
	}

	exec := &Execution{
		ID:         uuid.New().String(),
		WorkflowID: wf.ID,
		Version:    wf.Version,
		Status:     ExecRunning,
		Context:    make(map[string]interface{}),
		Steps:      make([]*StepExecution, 0),
		StartedAt:  time.Now(),
	}

	if trigger != nil {
		exec.TriggerID = trigger.TriggerID
		exec.TriggeredBy = string(trigger.Type)
		for k, v := range trigger.Payload {
			exec.Context[k] = v
		}
	}

	// 执行并等待结果
	engine := ex.engine
	err = engine.executeFromNode(ctx, wf, exec, "start")

	now := time.Now()
	exec.FinishedAt = &now
	exec.Duration = now.Sub(exec.StartedAt)

	if err != nil {
		exec.Status = ExecFailed
		exec.Error = err.Error()
	} else {
		exec.Status = ExecSuccess
	}

	ex.mu.Lock()
	ex.executions[exec.ID] = exec
	ex.mu.Unlock()

	return exec, err
}

// GetExecution 获取执行记录.
func (ex *Executor) GetExecution(id string) (*Execution, bool) {
	ex.mu.RLock()
	defer ex.mu.RUnlock()
	exec, ok := ex.executions[id]
	return exec, ok
}

// ListRunning 列出正在运行的执行.
func (ex *Executor) ListRunning() []*Execution {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	running := make([]*Execution, 0)
	for _, exec := range ex.executions {
		if exec.Status == ExecRunning || exec.Status == ExecPending {
			running = append(running, exec)
		}
	}
	return running
}

// Cancel 取消执行.
func (ex *Executor) Cancel(executionID string) error {
	ex.mu.RLock()
	exec, ok := ex.executions[executionID]
	ex.mu.RUnlock()

	if !ok {
		return fmt.Errorf("execution %q not found", executionID)
	}

	if exec.Status != ExecRunning && exec.Status != ExecPending {
		return fmt.Errorf("execution %q is not running (status: %s)", executionID, exec.Status)
	}

	exec.Status = ExecCancelled
	now := time.Now()
	exec.FinishedAt = &now
	exec.Duration = now.Sub(exec.StartedAt)

	ex.logger.Info("execution cancelled", zap.String("execution_id", executionID))
	return nil
}

// GetStats 获取执行统计.
func (ex *Executor) GetStats() *ExecutorStats {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	stats := &ExecutorStats{
		Total: len(ex.executions),
	}

	for _, exec := range ex.executions {
		switch exec.Status {
		case ExecRunning:
			stats.Running++
		case ExecPending:
			stats.Pending++
		case ExecSuccess:
			stats.Success++
		case ExecFailed:
			stats.Failed++
		case ExecCancelled:
			stats.Cancelled++
		}
		stats.TotalDuration += exec.Duration
	}

	if stats.Total > 0 {
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.Total)
	}

	return stats
}

// ExecutorStats 执行器统计.
type ExecutorStats struct {
	Total         int           `json:"total"`
	Running       int           `json:"running"`
	Pending       int           `json:"pending"`
	Success       int           `json:"success"`
	Failed        int           `json:"failed"`
	Cancelled     int           `json:"cancelled"`
	TotalDuration time.Duration `json:"total_duration"`
	AvgDuration   time.Duration `json:"avg_duration"`
}

// Cleanup 清理已完成的旧执行记录.
func (ex *Executor) Cleanup(maxAge time.Duration) int {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, exec := range ex.executions {
		if exec.FinishedAt != nil && exec.FinishedAt.Before(cutoff) {
			delete(ex.executions, id)
			removed++
		}
	}

	if removed > 0 {
		ex.logger.Info("cleaned up old executions", zap.Int("removed", removed))
	}

	return removed
}
