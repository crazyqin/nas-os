// Package distScheduler 故障恢复，支持任务重试和节点故障转移
package distScheduler

import (
	"time"

	"go.uber.org/zap"
)

// Recovery 故障恢复器.
type Recovery struct {
	mu     struct{}
	logger *zap.Logger
	engine *Engine
}

// NewRecovery 创建故障恢复器.
func NewRecovery(logger *zap.Logger, engine *Engine) *Recovery {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Recovery{
		logger: logger,
		engine: engine,
	}
}

// handleFailure 处理任务失败.
func (r *Recovery) handleFailure(task *Task) error {
	if task.Attempts < task.MaxAttempts {
		// 重试
		task.Status = TaskStatusRetrying
		task.NodeID = "" // 清除节点分配
		task.StartedAt = nil

		r.logger.Info("task will retry",
			zap.String("task_id", task.ID),
			zap.Int("attempt", task.Attempts),
			zap.Int("max_attempts", task.MaxAttempts),
		)

		// 延迟后重新变为 pending
		go func() {
			time.Sleep(time.Duration(r.engine.config.RetryDelay) * time.Second)
			r.engine.mu.Lock()
			if task.Status == TaskStatusRetrying {
				task.Status = TaskStatusPending
			}
			r.engine.mu.Unlock()
		}()

		return nil
	}

	// 超过最大重试次数
	task.Status = TaskStatusFailed
	now := time.Now()
	task.FinishedAt = &now

	r.logger.Error("task failed permanently",
		zap.String("task_id", task.ID),
		zap.Int("attempts", task.Attempts),
		zap.String("error", task.Error),
	)
	return nil
}

// handleNodeFailure 处理节点故障.
func (r *Recovery) handleNodeFailure(nodeID string) {
	r.logger.Warn("handling node failure", zap.String("node_id", nodeID))

	// 标记节点为离线
	if node, exists := r.engine.nodes[nodeID]; exists {
		node.Status = NodeStatusOffline
	}

	// 找到该节点上所有运行中的任务
	for _, task := range r.engine.tasks {
		if task.NodeID != nodeID {
			continue
		}
		if task.Status != TaskStatusRunning {
			continue
		}

		r.logger.Info("reassigning task from failed node",
			zap.String("task_id", task.ID),
			zap.String("failed_node", nodeID),
		)

		// 重置任务状态
		task.NodeID = ""
		task.StartedAt = nil
		task.Status = TaskStatusPending
		task.Error = "node failure: " + nodeID
		task.Attempts++ // 算一次尝试

		// 尝试恢复
		r.handleFailure(task)
	}
}

// RecoverNode 恢复节点.
func (r *Recovery) RecoverNode(nodeID string) error {
	r.engine.mu.Lock()
	defer r.engine.mu.Unlock()

	node, exists := r.engine.nodes[nodeID]
	if !exists {
		return nil
	}

	node.Status = NodeStatusOnline
	node.LastHB = time.Now()

	r.logger.Info("node recovered", zap.String("node_id", nodeID))
	return nil
}

// GetFailedTasks 获取失败任务列表.
func (r *Recovery) GetFailedTasks() []*Task {
	r.engine.mu.RLock()
	defer r.engine.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range r.engine.tasks {
		if task.Status == TaskStatusFailed {
			result = append(result, task)
		}
	}
	return result
}

// GetRetryTasks 获取等待重试的任务列表.
func (r *Recovery) GetRetryTasks() []*Task {
	r.engine.mu.RLock()
	defer r.engine.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range r.engine.tasks {
		if task.Status == TaskStatusRetrying {
			result = append(result, task)
		}
	}
	return result
}

// RetryFailedTask 手动重试失败任务.
func (r *Recovery) RetryFailedTask(taskID string) error {
	r.engine.mu.Lock()
	defer r.engine.mu.Unlock()

	task, exists := r.engine.tasks[taskID]
	if !exists {
		return nil
	}

	if task.Status != TaskStatusFailed {
		return nil
	}

	task.Status = TaskStatusPending
	task.NodeID = ""
	task.StartedAt = nil
	task.Attempts = 0
	task.Error = ""

	r.logger.Info("task manually retried", zap.String("task_id", taskID))
	return nil
}

// GetRecoveryStats 获取恢复统计.
func (r *Recovery) GetRecoveryStats() map[string]int {
	r.engine.mu.RLock()
	defer r.engine.mu.RUnlock()

	stats := map[string]int{
		"failed_tasks":  0,
		"retrying":      0,
		"offline_nodes": 0,
	}

	for _, task := range r.engine.tasks {
		switch task.Status {
		case TaskStatusFailed:
			stats["failed_tasks"]++
		case TaskStatusRetrying:
			stats["retrying"]++
		}
	}

	for _, node := range r.engine.nodes {
		if node.Status == NodeStatusOffline {
			stats["offline_nodes"]++
		}
	}

	return stats
}
