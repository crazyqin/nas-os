package migration

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Executor 迁移执行器.
// 负责执行迁移计划，支持增量迁移、断点续传、进度追踪、回滚.
type Executor struct {
	mu          sync.RWMutex
	planner     *Planner
	activeTasks map[string]*executionState
	// transferFn 数据传输函数（可替换为 mock）
	transferFn DataTransferFunc
	// rollbackFn 回滚函数
	rollbackFn RollbackFunc
	// checkpointStore 检查点存储
	checkpointStore map[string]*Checkpoint
}

// executionState 执行状态.
type executionState struct {
	task       *MigrationTask
	plan       *MigrationPlan
	ctx        context.Context
	cancel     context.CancelFunc
	result     *MigrationResult
	checkpoint *Checkpoint
}

// DataTransferFunc 数据传输函数签名.
type DataTransferFunc func(ctx context.Context, mapping DataMapping, progress func(bytes int64)) error

// RollbackFunc 回滚函数签名.
type RollbackFunc func(ctx context.Context, taskID string, checkpoint *Checkpoint) error

// NewExecutor 创建迁移执行器.
func NewExecutor(planner *Planner) *Executor {
	e := &Executor{
		planner:         planner,
		activeTasks:     make(map[string]*executionState),
		checkpointStore: make(map[string]*Checkpoint),
	}
	e.transferFn = e.defaultTransfer
	e.rollbackFn = e.defaultRollback
	return e
}

// SetTransferFunc 设置数据传输函数.
func (e *Executor) SetTransferFunc(fn DataTransferFunc) {
	e.transferFn = fn
}

// SetRollbackFunc 设置回滚函数.
func (e *Executor) SetRollbackFunc(fn RollbackFunc) {
	e.rollbackFn = fn
}

// Execute 执行迁移任务.
func (e *Executor) Execute(ctx context.Context, task *MigrationTask, plan *MigrationPlan) (*MigrationResult, error) {
	e.mu.Lock()

	// 检查是否有正在运行的任务
	if _, exists := e.activeTasks[task.ID]; exists {
		e.mu.Unlock()
		return nil, fmt.Errorf("任务 %s 已在运行中", task.ID)
	}

	// 检查任务是否已完成，不允许重复执行
	if task.Status == MigrationStatusCompleted {
		e.mu.Unlock()
		return nil, fmt.Errorf("任务 %s 已完成，无法重复执行", task.ID)
	}

	// 创建可取消的上下文
	taskCtx, cancel := context.WithCancel(ctx)

	state := &executionState{
		task:   task,
		plan:   plan,
		ctx:    taskCtx,
		cancel: cancel,
		result: &MigrationResult{
			TaskID:          task.ID,
			Status:          MigrationStatusRunning,
			CategoryResults: make([]CategoryResult, 0),
			Errors:          make([]MigrationErrorDetail, 0),
			Warnings:        make([]string, 0),
		},
	}

	// 恢复检查点（如果有）
	if cp, ok := e.checkpointStore[task.ID]; ok {
		state.checkpoint = cp
		slog.Info("从检查点恢复", "taskId", task.ID, "category", cp.CategoryIndex, "item", cp.ItemIndex)
	}

	e.activeTasks[task.ID] = state
	e.mu.Unlock()

	task.Status = MigrationStatusRunning
	task.StartedAt = time.Now()
	task.Progress = &ProgressInfo{
		CategoryProgress: make(map[string]int),
	}

	// 异步执行迁移
	go e.runMigration(state)

	return state.result, nil
}

// runMigration 运行迁移.
func (e *Executor) runMigration(state *executionState) {
	task := state.task
	plan := state.plan
	result := state.result

	defer func() {
		e.mu.Lock()
		delete(e.activeTasks, task.ID)
		delete(e.checkpointStore, task.ID)
		e.mu.Unlock()

		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(task.StartedAt)
		task.FinishedAt = result.CompletedAt
	}()

	// 确定起始位置（断点续传）
	startCategory := 0
	startItem := 0
	if state.checkpoint != nil {
		startCategory = state.checkpoint.CategoryIndex
		startItem = state.checkpoint.ItemIndex
		result.BytesMigrated = state.checkpoint.BytesTransferred
	}

	// 按顺序执行每个类别
	for i := startCategory; i < len(plan.Mappings); i++ {
		mapping := plan.Mappings[i]
		if !mapping.Selected {
			continue
		}

		select {
		case <-state.ctx.Done():
			result.Status = MigrationStatusCancelled
			task.Status = MigrationStatusCancelled
			task.Error = "迁移已取消"
			return
		default:
		}

		task.Progress.CurrentCategory = mapping.Category
		task.Progress.Phase = fmt.Sprintf("正在迁移: %s", mapping.Category)

		catResult := e.executeCategory(state, mapping, i, startItem)
		result.CategoryResults = append(result.CategoryResults, catResult)

		switch catResult.Status {
		case "success":
			result.TotalMigrated += catResult.Migrated
		case "partial":
			result.TotalMigrated += catResult.Migrated
			result.TotalFailed += catResult.Failed
		default:
			result.TotalFailed += catResult.Failed
		}
		result.TotalSkipped += catResult.Skipped
		result.BytesMigrated += catResult.SizeBytes

		// 重置起始项（仅第一个类别可能从断点恢复）
		startItem = 0

		// 保存检查点
		e.saveCheckpoint(task.ID, i+1, 0, result.BytesMigrated)

		// 更新总体进度
		e.updateOverallProgress(task, plan, i+1)
	}

	// 判断最终状态
	if result.TotalFailed > 0 {
		if result.TotalMigrated > 0 {
			result.Status = MigrationStatusCompleted
			task.Status = MigrationStatusCompleted
			result.Warnings = append(result.Warnings, fmt.Sprintf("部分数据迁移失败: %d 项", result.TotalFailed))
		} else {
			result.Status = MigrationStatusFailed
			task.Status = MigrationStatusFailed
		}
	} else {
		result.Status = MigrationStatusCompleted
		task.Status = MigrationStatusCompleted
		task.Progress.OverallPercent = 100
		task.Progress.Phase = "迁移完成"
	}

	// 生成回滚 ID
	result.RollbackID = uuid.New().String()

	slog.Info("迁移执行完成",
		"taskId", task.ID,
		"status", result.Status,
		"migrated", result.TotalMigrated,
		"failed", result.TotalFailed,
		"duration", result.Duration,
	)
}

// executeCategory 执行单个类别的迁移.
func (e *Executor) executeCategory(state *executionState, mapping DataMapping, categoryIndex, startItem int) CategoryResult {
	startTime := time.Now()
	result := CategoryResult{
		Category: mapping.Category,
		Status:   "success",
	}

	for item := startItem; item < mapping.ItemCount; item++ {
		select {
		case <-state.ctx.Done():
			result.Status = "partial"
			result.Duration = time.Since(startTime)
			return result
		default:
		}

		// 更新类别进度
		state.task.Progress.CategoryPercent = (item + 1) * 100 / mapping.ItemCount
		state.task.Progress.TransferredItems++

		// 执行传输
		err := e.transferFn(state.ctx, mapping, func(bytes int64) {
			state.task.Progress.TransferredBytes += bytes
			state.task.Progress.Speed = bytes // 简化：实际应计算平均速度
		})

		if err != nil {
			slog.Error("传输失败",
				"category", mapping.Category,
				"item", item,
				"error", err,
			)
			result.Failed++
			state.result.Errors = append(state.result.Errors, MigrationErrorDetail{
				Category: mapping.Category,
				Item:     fmt.Sprintf("item-%d", item),
				Error:    err.Error(),
			})
		} else {
			result.Migrated++
			result.SizeBytes += mapping.TotalSize / int64(mapping.ItemCount)
		}

		// 保存断点
		e.saveCheckpoint(state.task.ID, categoryIndex, item+1, state.result.BytesMigrated)
	}

	result.Duration = time.Since(startTime)
	if result.Failed > 0 && result.Migrated > 0 {
		result.Status = "partial"
	} else if result.Failed > 0 {
		result.Status = "failed"
	}

	return result
}

// updateOverallProgress 更新总体进度.
func (e *Executor) updateOverallProgress(task *MigrationTask, plan *MigrationPlan, completedCategories int) {
	if len(plan.Mappings) > 0 {
		task.Progress.OverallPercent = completedCategories * 100 / len(plan.Mappings)
	}

	// 更新剩余时间估算
	if task.Progress.Speed > 0 && task.Progress.TotalBytes > task.Progress.TransferredBytes {
		remaining := task.Progress.TotalBytes - task.Progress.TransferredBytes
		task.Progress.RemainingSec = remaining / task.Progress.Speed
	}

	// 更新类别进度映射
	task.Progress.CategoryProgress[string(task.Progress.CurrentCategory)] = task.Progress.CategoryPercent
}

// saveCheckpoint 保存断点.
func (e *Executor) saveCheckpoint(taskID string, categoryIndex, itemIndex int, bytesTransferred int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.checkpointStore[taskID] = &Checkpoint{
		TaskID:           taskID,
		CategoryIndex:    categoryIndex,
		ItemIndex:        itemIndex,
		BytesTransferred: bytesTransferred,
		Timestamp:        time.Now(),
	}
}

// Pause 暂停迁移任务.
func (e *Executor) Pause(taskID string) error {
	e.mu.RLock()
	state, ok := e.activeTasks[taskID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务 %s 未在运行", taskID)
	}

	state.cancel()
	state.task.Status = MigrationStatusPaused
	state.task.Progress.Phase = "已暂停"

	return nil
}

// Resume 断点续传.
func (e *Executor) Resume(ctx context.Context, taskID string) (*MigrationResult, error) {
	e.mu.RLock()
	cp, ok := e.checkpointStore[taskID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("无可用检查点，无法续传: %s", taskID)
	}

	// 获取原任务和计划
	// 注意：实际实现中应从持久化存储中恢复
	task := &MigrationTask{
		ID:     taskID,
		Status: MigrationStatusRunning,
	}

	slog.Info("恢复迁移任务",
		"taskId", taskID,
		"fromCategory", cp.CategoryIndex,
		"fromItem", cp.ItemIndex,
	)

	// 重新生成计划（实际应从存储中恢复）
	plan, err := e.planner.GeneratePlan(ctx, task, &SourceSystemInfo{})
	if err != nil {
		return nil, fmt.Errorf("恢复计划失败: %w", err)
	}

	return e.Execute(ctx, task, plan)
}

// Rollback 回滚迁移.
func (e *Executor) Rollback(ctx context.Context, taskID string) error {
	e.mu.RLock()
	cp := e.checkpointStore[taskID]
	e.mu.RUnlock()

	slog.Info("开始回滚迁移", "taskId", taskID)

	err := e.rollbackFn(ctx, taskID, cp)
	if err != nil {
		return fmt.Errorf("回滚失败: %w", err)
	}

	// 清除检查点
	e.mu.Lock()
	delete(e.checkpointStore, taskID)
	e.mu.Unlock()

	slog.Info("回滚完成", "taskId", taskID)
	return nil
}

// GetProgress 获取任务进度.
func (e *Executor) GetProgress(taskID string) (*ProgressInfo, error) {
	e.mu.RLock()
	state, ok := e.activeTasks[taskID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("任务 %s 未在运行", taskID)
	}

	return state.task.Progress, nil
}

// GetResult 获取迁移结果.
func (e *Executor) GetResult(taskID string) (*MigrationResult, error) {
	e.mu.RLock()
	state, ok := e.activeTasks[taskID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("任务 %s 未找到", taskID)
	}

	return state.result, nil
}

// defaultTransfer 默认数据传输实现.
func (e *Executor) defaultTransfer(ctx context.Context, mapping DataMapping, progress func(bytes int64)) error {
	// 模拟传输
	chunkSize := mapping.TotalSize / int64(mapping.ItemCount)
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		time.Sleep(10 * time.Millisecond)
		progress(chunkSize / 10)
	}
	return nil
}

// defaultRollback 默认回滚实现.
func (e *Executor) defaultRollback(ctx context.Context, taskID string, checkpoint *Checkpoint) error {
	// 模拟回滚操作
	slog.Info("执行默认回滚", "taskId", taskID)
	return nil
}
