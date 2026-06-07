// Package distScheduler 分布式任务调度引擎核心
package distScheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Engine 调度引擎
type Engine struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *Config
	nodes    map[string]*Node
	tasks    map[string]*Task
	cronList map[string]*CronEntry
	graph    *TaskGraph
	alloc    *Allocator
	balancer *Balancer
	recovery *Recovery
	stopCh   chan struct{}
	running  bool
}

// NewEngine 创建调度引擎
func NewEngine(logger *zap.Logger, config *Config) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}

	e := &Engine{
		logger:   logger,
		config:   config,
		nodes:    make(map[string]*Node),
		tasks:    make(map[string]*Task),
		cronList: make(map[string]*CronEntry),
		graph:    &TaskGraph{Tasks: make(map[string]*Task), Edges: make(map[string][]string)},
		stopCh:   make(chan struct{}),
	}

	e.alloc = NewAllocator(logger, config)
	e.balancer = NewBalancer(logger, config)
	e.recovery = NewRecovery(logger, e)

	return e
}

// Start 启动调度引擎
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.running = true
	e.mu.Unlock()

	e.logger.Info("starting distributed scheduler",
		zap.String("strategy", string(e.config.Strategy)),
		zap.Int("workers", e.config.SchedulerWorkers),
	)

	// 启动心跳检查
	go e.heartbeatChecker(ctx)
	// 启动 Cron 调度
	go e.cronScheduler(ctx)

	return nil
}

// Stop 停止调度引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.running = false
	close(e.stopCh)
	e.logger.Info("distributed scheduler stopped")
}

// IsRunning 是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// RegisterNode 注册节点
func (e *Engine) RegisterNode(node *Node) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if node.ID == "" {
		node.ID = generateID()
	}
	node.Status = NodeStatusOnline
	node.LastHB = time.Now()
	node.CreatedAt = time.Now()
	if node.Tags == nil {
		node.Tags = make(map[string]string)
	}

	e.nodes[node.ID] = node
	e.logger.Info("node registered", zap.String("id", node.ID), zap.String("name", node.Name))
	return nil
}

// UnregisterNode 注销节点
func (e *Engine) UnregisterNode(nodeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	delete(e.nodes, nodeID)
	e.logger.Info("node unregistered", zap.String("id", nodeID))
	return nil
}

// GetNode 获取节点
func (e *Engine) GetNode(nodeID string) (*Node, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	node, exists := e.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	return node, nil
}

// ListNodes 列出所有节点
func (e *Engine) ListNodes(status NodeStatus) []*Node {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Node, 0)
	for _, node := range e.nodes {
		if status == "" || node.Status == status {
			result = append(result, node)
		}
	}
	return result
}

// Heartbeat 更新节点心跳
func (e *Engine) Heartbeat(nodeID string, resources *NodeResources) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, exists := e.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.LastHB = time.Now()
	if resources != nil {
		node.Resources = resources
	}
	return nil
}

// SubmitTask 提交任务
func (e *Engine) SubmitTask(task *Task) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if task.ID == "" {
		task.ID = generateID()
	}
	task.Status = TaskStatusPending
	if task.MaxAttempts <= 0 {
		task.MaxAttempts = e.config.MaxRetries
	}
	if task.Timeout <= 0 {
		task.Timeout = e.config.TaskTimeout
	}
	task.CreatedAt = time.Now()

	// 加入任务图
	e.tasks[task.ID] = task
	e.graph.Tasks[task.ID] = task
	if task.Dependencies != nil {
		e.graph.Edges[task.ID] = task.Dependencies
	}

	// Cron 任务
	if task.CronExpr != "" {
		nextRun, err := parseCronNext(task.CronExpr)
		if err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
		e.cronList[task.ID] = &CronEntry{
			TaskID:   task.ID,
			CronExpr: task.CronExpr,
			NextRun:  nextRun,
			Enabled:  true,
		}
		task.Status = TaskStatusScheduled
		task.ScheduledAt = &nextRun
	}

	e.logger.Info("task submitted",
		zap.String("id", task.ID),
		zap.String("name", task.Name),
		zap.Int("priority", task.Priority),
	)
	return nil
}

// CancelTask 取消任务
func (e *Engine) CancelTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
		return fmt.Errorf("task %s already finished", taskID)
	}

	task.Status = TaskStatusCancelled
	now := time.Now()
	task.FinishedAt = &now

	// 禁用 Cron
	if entry, ok := e.cronList[taskID]; ok {
		entry.Enabled = false
	}

	return nil
}

// GetTask 获取任务
func (e *Engine) GetTask(taskID string) (*Task, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// ListTasks 列出任务
func (e *Engine) ListTasks(status TaskStatus) []*Task {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range e.tasks {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}
	return result
}

// Schedule 调度任务到节点
func (e *Engine) Schedule(ctx context.Context) ([]*ScheduleResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var results []*ScheduleResult

	for _, task := range e.tasks {
		if task.Status != TaskStatusPending {
			continue
		}

		// 检查依赖是否满足
		if !e.checkDependencies(task.ID) {
			continue
		}

		// 选择节点
		nodeID, err := e.balancer.SelectNode(e.nodes, task, e.config.Strategy)
		if err != nil {
			e.logger.Warn("no available node", zap.String("task_id", task.ID), zap.Error(err))
			results = append(results, &ScheduleResult{
				TaskID:  task.ID,
				Success: false,
				Error:   err.Error(),
			})
			continue
		}

		// 分配任务
		task.NodeID = nodeID
		task.Status = TaskStatusRunning
		now := time.Now()
		task.StartedAt = &now
		task.Attempts++

		node := e.nodes[nodeID]
		node.TaskCount++

		results = append(results, &ScheduleResult{
			TaskID:  task.ID,
			NodeID:  nodeID,
			Success: true,
		})

		e.logger.Info("task scheduled",
			zap.String("task_id", task.ID),
			zap.String("node_id", nodeID),
		)
	}

	return results, nil
}

// CompleteTask 完成任务
func (e *Engine) CompleteTask(taskID string, result interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = TaskStatusCompleted
	task.Result = result
	now := time.Now()
	task.FinishedAt = &now

	// 更新节点计数
	if node, ok := e.nodes[task.NodeID]; ok {
		node.TaskCount--
		node.TotalTasks++
	}

	return nil
}

// FailTask 标记任务失败
func (e *Engine) FailTask(taskID string, err error) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Error = err.Error()

	// 更新节点计数
	if node, ok := e.nodes[task.NodeID]; ok {
		node.TaskCount--
	}

	return e.recovery.handleFailure(task)
}

// checkDependencies 检查任务依赖是否满足
func (e *Engine) checkDependencies(taskID string) bool {
	deps, exists := e.graph.Edges[taskID]
	if !exists {
		return true
	}

	for _, depID := range deps {
		dep, ok := e.tasks[depID]
		if !ok || dep.Status != TaskStatusCompleted {
			return false
		}
	}
	return true
}

// heartbeatChecker 心跳检查
func (e *Engine) heartbeatChecker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkHeartbeats()
		}
	}
}

// checkHeartbeats 检查所有节点心跳
func (e *Engine) checkHeartbeats() {
	e.mu.Lock()
	defer e.mu.Unlock()

	timeout := time.Duration(e.config.HeartbeatTimeout) * time.Second
	for _, node := range e.nodes {
		if node.Status == NodeStatusOffline {
			continue
		}
		if time.Since(node.LastHB) > timeout {
			e.logger.Warn("node heartbeat timeout",
				zap.String("node_id", node.ID),
				zap.Duration("since", time.Since(node.LastHB)),
			)
			node.Status = NodeStatusOffline
			e.recovery.handleNodeFailure(node.ID)
		}
	}
}

// cronScheduler Cron 调度
func (e *Engine) cronScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkCronTasks()
		}
	}
}

// checkCronTasks 检查 Cron 任务
func (e *Engine) checkCronTasks() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for _, entry := range e.cronList {
		if !entry.Enabled {
			continue
		}
		if now.Before(entry.NextRun) {
			continue
		}

		// 创建新的任务实例
		original, exists := e.tasks[entry.TaskID]
		if !exists {
			continue
		}

		newTask := &Task{
			ID:           generateID(),
			Name:         original.Name,
			Type:         original.Type,
			Status:       TaskStatusPending,
			Priority:     original.Priority,
			Requirements: original.Requirements,
			Payload:      original.Payload,
			MaxAttempts:  original.MaxAttempts,
			Tags:         original.Tags,
			Timeout:      original.Timeout,
			CreatedAt:    now,
		}
		e.tasks[newTask.ID] = newTask

		entry.LastRun = &now
		nextRun, _ := parseCronNext(entry.CronExpr)
		entry.NextRun = nextRun

		e.logger.Debug("cron task triggered",
			zap.String("original_id", entry.TaskID),
			zap.String("new_id", newTask.ID),
		)
	}
}

// GetStats 获取引擎统计
func (e *Engine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	taskStats := map[string]int{
		"total":     len(e.tasks),
		"pending":   0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"cancelled": 0,
	}
	for _, task := range e.tasks {
		switch task.Status {
		case TaskStatusPending, TaskStatusScheduled:
			taskStats["pending"]++
		case TaskStatusRunning, TaskStatusRetrying:
			taskStats["running"]++
		case TaskStatusCompleted:
			taskStats["completed"]++
		case TaskStatusFailed:
			taskStats["failed"]++
		case TaskStatusCancelled:
			taskStats["cancelled"]++
		}
	}

	nodeStats := map[string]int{
		"total":   len(e.nodes),
		"online":  0,
		"offline": 0,
		"busy":    0,
	}
	for _, node := range e.nodes {
		switch node.Status {
		case NodeStatusOnline:
			nodeStats["online"]++
		case NodeStatusOffline:
			nodeStats["offline"]++
		case NodeStatusBusy:
			nodeStats["busy"]++
		}
	}

	return map[string]interface{}{
		"tasks": taskStats,
		"nodes": nodeStats,
		"cron":  len(e.cronList),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// parseCronNext 解析 Cron 表达式的下次执行时间（简化实现）
func parseCronNext(expr string) (time.Time, error) {
	// 简化实现：支持 "* * * * *" 格式
	// 实际应使用 cron 库解析
	if expr == "" {
		return time.Time{}, fmt.Errorf("empty cron expression")
	}
	// 默认5分钟后
	return time.Now().Add(5 * time.Minute), nil
}
