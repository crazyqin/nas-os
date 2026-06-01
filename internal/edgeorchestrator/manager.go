package edgeorchestrator

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ========== 常量 ==========

const (
	DefaultMaxTasksPerNode     = 50
	DefaultHeartbeatTimeout    = 30 * time.Second
	DefaultHealthCheckInterval = 10 * time.Second
)

// ========== 错误类型 ==========

// NotFoundError 资源未找到
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

// InsufficientResourceError 资源不足
type InsufficientResourceError struct {
	Resource string
	Required float64
	Available float64
}

func (e *InsufficientResourceError) Error() string {
	return fmt.Sprintf("insufficient %s: required %.2f, available %.2f", e.Resource, e.Required, e.Available)
}

// ========== 节点管理 ==========

// RegisterNode 注册边缘节点
func (m *Manager) RegisterNode(req *RegisterNodeRequest) (*EdgeNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeID := uuid.New().String()
	now := time.Now()

	maxTasks := req.MaxTasks
	if maxTasks <= 0 {
		maxTasks = DefaultMaxTasksPerNode
	}

	node := &EdgeNode{
		ID:              nodeID,
		Name:            req.Name,
		IPAddress:       req.IPAddress,
		Region:          req.Region,
		Zone:            req.Zone,
		Status:          NodeStatusOnline,
		CPUCores:        req.CPUCores,
		MemoryMB:        req.MemoryMB,
		DiskGB:          req.DiskGB,
		GPUCount:        req.GPUCount,
		GPUModel:        req.GPUModel,
		Labels:          req.Labels,
		Capabilities:    req.Capabilities,
		CurrentCPUUsage: 0,
		CurrentMemUsage: 0,
		RunningTasks:    0,
		MaxTasks:        maxTasks,
		LastHeartbeat:   now,
		RegisteredAt:    now,
		EndpointURL:     req.EndpointURL,
		Architecture:    "amd64",
	}

	m.nodes[nodeID] = node
	m.syncStatuses[nodeID] = &SyncStatus{
		NodeID:       nodeID,
		LastSyncTime: now,
	}

	log.Printf("Node registered: %s (%s) at %s", node.Name, nodeID, node.IPAddress)
	return node, nil
}

// UnregisterNode 注销边缘节点
func (m *Manager) UnregisterNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return &NotFoundError{Resource: "node", ID: nodeID}
	}

	// 检查是否有运行中的任务
	for _, task := range m.tasks {
		if task.AssignedNodeID == nodeID && task.Status == TaskStatusRunning {
			return fmt.Errorf("cannot unregister node %s: has running tasks", nodeID)
		}
	}

	delete(m.nodes, nodeID)
	delete(m.syncStatuses, nodeID)
	log.Printf("Node unregistered: %s (%s)", node.Name, nodeID)
	return nil
}

// GetNode 获取节点信息
func (m *Manager) GetNode(nodeID string) (*EdgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return nil, &NotFoundError{Resource: "node", ID: nodeID}
	}
	return node, nil
}

// ListNodes 列出所有节点
func (m *Manager) ListNodes(status string) []*EdgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*EdgeNode, 0, len(m.nodes))
	for _, node := range m.nodes {
		if status == "" || string(node.Status) == status {
			nodes = append(nodes, node)
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].RegisteredAt.Before(nodes[j].RegisteredAt)
	})
	return nodes
}

// UpdateNodeStatus 更新节点状态
func (m *Manager) UpdateNodeStatus(nodeID string, status EdgeNodeStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return &NotFoundError{Resource: "node", ID: nodeID}
	}

	node.Status = status
	log.Printf("Node %s status updated to %s", nodeID, status)
	return nil
}

// UpdateNodeLabels 更新节点标签
func (m *Manager) UpdateNodeLabels(nodeID string, labels map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return &NotFoundError{Resource: "node", ID: nodeID}
	}

	node.Labels = labels
	return nil
}

// Heartbeat 更新节点心跳
func (m *Manager) Heartbeat(nodeID string, cpuUsage, memUsage float64, runningTasks int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return &NotFoundError{Resource: "node", ID: nodeID}
	}

	node.LastHeartbeat = time.Now()
	node.CurrentCPUUsage = cpuUsage
	node.CurrentMemUsage = memUsage
	node.RunningTasks = runningTasks
	return nil
}

// ========== 任务调度 ==========

// SubmitTask 提交边缘任务
func (m *Manager) SubmitTask(req *SubmitTaskRequest) (*EdgeTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskID := uuid.New().String()
	now := time.Now()

	timeout := time.Duration(req.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}

	maxRetries := req.MaxRetries
	if maxRetries < 0 {
		maxRetries = 3
	}

	taskType := req.Type
	if taskType == "" {
		taskType = TaskTypeGeneral
	}

	task := &EdgeTask{
		ID:              taskID,
		Name:            req.Name,
		Description:     req.Description,
		Type:            taskType,
		Status:          TaskStatusPending,
		Priority:        req.Priority,
		Image:           req.Image,
		Command:         req.Command,
		Args:            req.Args,
		Env:             req.Env,
		CPURequest:      req.CPURequest,
		MemoryRequestMB: req.MemoryRequestMB,
		GPURequest:      req.GPURequest,
		Timeout:         timeout,
		MaxRetries:      maxRetries,
		NodeSelector:    req.NodeSelector,
		Affinity:        req.Affinity,
		CreatedAt:       now,
		Labels:          req.Labels,
	}

	m.tasks[taskID] = task
	log.Printf("Task submitted: %s (%s)", task.Name, taskID)
	return task, nil
}

// ScheduleTask 调度任务到合适的节点
func (m *Manager) ScheduleTask(taskID string) (*EdgeTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, &NotFoundError{Resource: "task", ID: taskID}
	}

	if task.Status != TaskStatusPending {
		return nil, fmt.Errorf("task %s is not in pending status", taskID)
	}

	// 选择最佳节点
	node, err := m.selectBestNode(task)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	task.Status = TaskStatusScheduled
	task.AssignedNodeID = node.ID
	task.ScheduledAt = &now

	log.Printf("Task %s scheduled to node %s", taskID, node.ID)
	return task, nil
}

// selectBestNode 根据调度策略选择最佳节点
func (m *Manager) selectBestNode(task *EdgeTask) (*EdgeNode, error) {
	candidates := m.filterCandidateNodes(task)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available nodes for task %s", task.ID)
	}

	switch m.schedulerConfig.Strategy {
	case StrategyRoundRobin:
		return m.selectRoundRobin(candidates), nil
	case StrategyLeastLoad:
		return m.selectLeastLoad(candidates), nil
	case StrategyRandom:
		return m.selectRandom(candidates), nil
	case StrategyBinPack:
		return m.selectBinPack(candidates), nil
	case StrategySpread:
		return m.selectSpread(candidates), nil
	default:
		return m.selectLeastLoad(candidates), nil
	}
}

// filterCandidateNodes 过滤候选节点
func (m *Manager) filterCandidateNodes(task *EdgeTask) []*EdgeNode {
	candidates := make([]*EdgeNode, 0)

	for _, node := range m.nodes {
		// 检查节点状态
		if node.Status != NodeStatusOnline {
			continue
		}

		// 检查任务容量
		if node.RunningTasks >= node.MaxTasks {
			continue
		}

		// 检查资源需求
		if !m.checkResourceFit(node, task) {
			continue
		}

		// 检查节点选择器
		if !m.checkNodeSelector(node, task.NodeSelector) {
			continue
		}

		// 检查亲和性
		if m.schedulerConfig.EnableAffinity && task.Affinity != nil {
			if !m.checkAffinity(node, task.Affinity) {
				continue
			}
		}

		// 检查污点
		if m.schedulerConfig.EnableTaints && !m.checkTaints(node, task) {
			continue
		}

		// 检查GPU需求
		if task.GPURequest > 0 && node.GPUCount < task.GPURequest {
			continue
		}

		candidates = append(candidates, node)
	}

	return candidates
}

// checkResourceFit 检查资源是否满足
func (m *Manager) checkResourceFit(node *EdgeNode, task *EdgeTask) bool {
	availableCPU := float64(node.CPUCores) * (1 - node.CurrentCPUUsage/100)
	availableMem := float64(node.MemoryMB) * (1 - node.CurrentMemUsage/100)

	return availableCPU >= task.CPURequest && availableMem >= float64(task.MemoryRequestMB)
}

// checkNodeSelector 检查节点选择器
func (m *Manager) checkNodeSelector(node *EdgeNode, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}

	for key, value := range selector {
		if node.Labels[key] != value {
			return false
		}
	}
	return true
}

// checkAffinity 检查亲和性规则
func (m *Manager) checkAffinity(node *EdgeNode, affinity *AffinityRule) bool {
	// 检查首选节点
	if len(affinity.PreferredNodes) > 0 {
		found := false
		for _, id := range affinity.PreferredNodes {
			if node.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查必需标签
	for key, value := range affinity.RequiredLabels {
		if node.Labels[key] != value {
			return false
		}
	}

	// 检查必需区域
	if len(affinity.RequiredZones) > 0 {
		found := false
		for _, zone := range affinity.RequiredZones {
			if node.Zone == zone {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// checkTaints 检查污点
func (m *Manager) checkTaints(node *EdgeNode, task *EdgeTask) bool {
	for _, taint := range node.Taints {
		if taint.Effect == "NoSchedule" {
			// 检查任务是否有对应的容忍
			tolerated := false
			if task.Labels != nil {
				if val, ok := task.Labels["tolerate/"+taint.Key]; ok && val == taint.Value {
					tolerated = true
				}
			}
			if !tolerated {
				return false
			}
		}
	}
	return true
}

// selectRoundRobin 轮询选择
func (m *Manager) selectRoundRobin(nodes []*EdgeNode) *EdgeNode {
	// 按注册时间排序，选择最早注册的
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].RegisteredAt.Before(nodes[j].RegisteredAt)
	})
	return nodes[0]
}

// selectLeastLoad 最小负载选择
func (m *Manager) selectLeastLoad(nodes []*EdgeNode) *EdgeNode {
	sort.Slice(nodes, func(i, j int) bool {
		loadI := nodes[i].CurrentCPUUsage + nodes[i].CurrentMemUsage
		loadJ := nodes[j].CurrentCPUUsage + nodes[j].CurrentMemUsage
		return loadI < loadJ
	})
	return nodes[0]
}

// selectRandom 随机选择
func (m *Manager) selectRandom(nodes []*EdgeNode) *EdgeNode {
	return nodes[rand.Intn(len(nodes))]
}

// selectBinPack 装箱策略
func (m *Manager) selectBinPack(nodes []*EdgeNode) *EdgeNode {
	// 选择负载最高的节点（尽可能填满）
	sort.Slice(nodes, func(i, j int) bool {
		loadI := nodes[i].CurrentCPUUsage + nodes[i].CurrentMemUsage
		loadJ := nodes[j].CurrentCPUUsage + nodes[j].CurrentMemUsage
		return loadI > loadJ
	})
	return nodes[0]
}

// selectSpread 分散策略
func (m *Manager) selectSpread(nodes []*EdgeNode) *EdgeNode {
	// 选择运行任务最少的节点
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].RunningTasks < nodes[j].RunningTasks
	})
	return nodes[0]
}

// StartTask 启动任务
func (m *Manager) StartTask(taskID string) (*EdgeTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, &NotFoundError{Resource: "task", ID: taskID}
	}

	if task.Status != TaskStatusScheduled {
		return nil, fmt.Errorf("task %s is not in scheduled status", taskID)
	}

	node, ok := m.nodes[task.AssignedNodeID]
	if !ok {
		return nil, &NotFoundError{Resource: "node", ID: task.AssignedNodeID}
	}

	now := time.Now()
	task.Status = TaskStatusRunning
	task.StartedAt = &now
	node.RunningTasks++

	log.Printf("Task %s started on node %s", taskID, node.ID)
	return task, nil
}

// CompleteTask 完成任务
func (m *Manager) CompleteTask(taskID string, result *TaskResult) (*EdgeTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, &NotFoundError{Resource: "task", ID: taskID}
	}

	if task.Status != TaskStatusRunning {
		return nil, fmt.Errorf("task %s is not running", taskID)
	}

	if node, ok := m.nodes[task.AssignedNodeID]; ok {
		node.RunningTasks--
	}

	now := time.Now()
	task.Status = TaskStatusCompleted
	task.CompletedAt = &now
	task.Result = result

	log.Printf("Task %s completed", taskID)
	return task, nil
}

// FailTask 标记任务失败
func (m *Manager) FailTask(taskID string, errMsg string) (*EdgeTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, &NotFoundError{Resource: "task", ID: taskID}
	}

	if node, ok := m.nodes[task.AssignedNodeID]; ok {
		if task.Status == TaskStatusRunning {
			node.RunningTasks--
		}
	}

	task.RetryCount++
	if task.RetryCount < task.MaxRetries {
		// 重新调度
		task.Status = TaskStatusPending
		task.AssignedNodeID = ""
		task.ScheduledAt = nil
		task.StartedAt = nil
		log.Printf("Task %s failed, retrying (%d/%d)", taskID, task.RetryCount, task.MaxRetries)
	} else {
		now := time.Now()
		task.Status = TaskStatusFailed
		task.CompletedAt = &now
		task.Result = &TaskResult{
			ExitCode:    -1,
			Error:       errMsg,
			CompletedAt: now,
		}
		log.Printf("Task %s failed permanently: %s", taskID, errMsg)
	}

	return task, nil
}

// CancelTask 取消任务
func (m *Manager) CancelTask(taskID string) (*EdgeTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, &NotFoundError{Resource: "task", ID: taskID}
	}

	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled {
		return nil, fmt.Errorf("task %s is already in terminal status", taskID)
	}

	if node, ok := m.nodes[task.AssignedNodeID]; ok {
		if task.Status == TaskStatusRunning {
			node.RunningTasks--
		}
	}

	now := time.Now()
	task.Status = TaskStatusCancelled
	task.CompletedAt = &now

	log.Printf("Task %s cancelled", taskID)
	return task, nil
}

// GetTask 获取任务详情
func (m *Manager) GetTask(taskID string) (*EdgeTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, &NotFoundError{Resource: "task", ID: taskID}
	}
	return task, nil
}

// ListTasks 列出任务
func (m *Manager) ListTasks(filter *ListTasksRequest) []*EdgeTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*EdgeTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		if filter.Status != "" && string(task.Status) != filter.Status {
			continue
		}
		if filter.Type != "" && string(task.Type) != filter.Type {
			continue
		}
		if filter.NodeID != "" && task.AssignedNodeID != filter.NodeID {
			continue
		}
		if filter.Priority > 0 && int(task.Priority) != filter.Priority {
			continue
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	// 应用分页
	if filter.Offset > 0 && filter.Offset < len(tasks) {
		tasks = tasks[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(tasks) {
		tasks = tasks[:filter.Limit]
	}

	return tasks
}

// ========== AI 推理 ==========

// SubmitInference 提交AI推理任务
func (m *Manager) SubmitInference(req *SubmitInferenceRequest) (*AIInferenceTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskID := uuid.New().String()

	maxLatency := time.Duration(req.MaxLatencyMs) * time.Millisecond
	if maxLatency <= 0 {
		maxLatency = 5 * time.Second
	}

	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	task := &AIInferenceTask{
		TaskID:       taskID,
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Framework:    req.Framework,
		InputType:    req.InputType,
		InputURL:     req.InputURL,
		Parameters:   req.Parameters,
		BatchSize:    batchSize,
		Priority:     req.Priority,
		MaxLatency:   maxLatency,
		ResultChan:   make(chan *InferenceResult, 1),
	}

	m.inferenceTasks[taskID] = task

	log.Printf("Inference task submitted: %s (model: %s)", taskID, req.ModelName)
	return task, nil
}

// GetInferenceResult 获取推理结果
func (m *Manager) GetInferenceResult(taskID string) (*InferenceResult, error) {
	m.mu.RLock()
	task, ok := m.inferenceTasks[taskID]
	m.mu.RUnlock()

	if !ok {
		return nil, &NotFoundError{Resource: "inference_task", ID: taskID}
	}

	// 等待结果或超时
	select {
	case result := <-task.ResultChan:
		return result, nil
	case <-time.After(task.MaxLatency):
		return nil, fmt.Errorf("inference timeout for task %s", taskID)
	}
}

// CompleteInference 完成推理任务
func (m *Manager) CompleteInference(taskID string, result *InferenceResult) error {
	m.mu.RLock()
	task, ok := m.inferenceTasks[taskID]
	m.mu.RUnlock()

	if !ok {
		return &NotFoundError{Resource: "inference_task", ID: taskID}
	}

	select {
	case task.ResultChan <- result:
		return nil
	default:
		return fmt.Errorf("result channel full for task %s", taskID)
	}
}

// ========== 同步和监控 ==========

// SyncNodeStatus 同步节点状态
func (m *Manager) SyncNodeStatus(ctx context.Context) error {
	m.mu.RLock()
	nodes := make([]*EdgeNode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	m.mu.RUnlock()

	for _, node := range nodes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		m.mu.Lock()
		syncStatus, ok := m.syncStatuses[node.ID]
		if !ok {
			syncStatus = &SyncStatus{NodeID: node.ID}
			m.syncStatuses[node.ID] = syncStatus
		}
		syncStatus.LastSyncTime = time.Now()
		m.mu.Unlock()
	}

	return nil
}

// CheckNodeHealth 检查节点健康状态
func (m *Manager) CheckNodeHealth(nodeID string) (*NodeHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return nil, &NotFoundError{Resource: "node", ID: nodeID}
	}

	healthy := true
	warnings := make([]string, 0)

	// 检查心跳超时
	if time.Since(node.LastHeartbeat) > m.schedulerConfig.HeartbeatTimeout {
		healthy = false
		warnings = append(warnings, "heartbeat timeout")
	}

	// 检查资源使用率
	if node.CurrentCPUUsage > 90 {
		warnings = append(warnings, "high CPU usage")
	}
	if node.CurrentMemUsage > 90 {
		warnings = append(warnings, "high memory usage")
	}

	return &NodeHealth{
		NodeID:        nodeID,
		Healthy:       healthy,
		CPUPercent:    node.CurrentCPUUsage,
		MemoryPercent: node.CurrentMemUsage,
		Warnings:      warnings,
		LastCheck:     time.Now(),
	}, nil
}

// GetClusterMetrics 获取集群指标
func (m *Manager) GetClusterMetrics() *ClusterMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := &ClusterMetrics{
		Timestamp: time.Now(),
	}

	for _, node := range m.nodes {
		metrics.TotalNodes++
		switch node.Status {
		case NodeStatusOnline:
			metrics.OnlineNodes++
		case NodeStatusOffline:
			metrics.OfflineNodes++
		}

		metrics.TotalCPUCores += node.CPUCores
		metrics.UsedCPUCores += float64(node.CPUCores) * node.CurrentCPUUsage / 100
		metrics.TotalMemoryMB += node.MemoryMB
		metrics.UsedMemoryMB += int64(float64(node.MemoryMB) * node.CurrentMemUsage / 100)
		metrics.TotalGPUs += node.GPUCount
	}

	for _, task := range m.tasks {
		metrics.TotalTasks++
		switch task.Status {
		case TaskStatusRunning:
			metrics.RunningTasks++
		case TaskStatusPending, TaskStatusScheduled:
			metrics.PendingTasks++
		case TaskStatusCompleted:
			metrics.CompletedTasks++
		case TaskStatusFailed:
			metrics.FailedTasks++
		}
	}

	return metrics
}

// GetSyncStatus 获取同步状态
func (m *Manager) GetSyncStatus() []*SyncStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]*SyncStatus, 0, len(m.syncStatuses))
	for _, s := range m.syncStatuses {
		statuses = append(statuses, s)
	}
	return statuses
}

// ========== 自动调度循环 ==========

// StartScheduler 启动自动调度器
func (m *Manager) StartScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.schedulerConfig.HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.schedulePendingTasks()
				m.checkHeartbeats()
			}
		}
	}()
}

// StopScheduler 停止调度器
func (m *Manager) StopScheduler() {
	close(m.stopCh)
}

// schedulePendingTasks 调度待处理任务
func (m *Manager) schedulePendingTasks() {
	m.mu.RLock()
	pendingTasks := make([]*EdgeTask, 0)
	for _, task := range m.tasks {
		if task.Status == TaskStatusPending {
			pendingTasks = append(pendingTasks, task)
		}
	}
	m.mu.RUnlock()

	// 按优先级排序
	sort.Slice(pendingTasks, func(i, j int) bool {
		return pendingTasks[i].Priority > pendingTasks[j].Priority
	})

	for _, task := range pendingTasks {
		_, err := m.ScheduleTask(task.ID)
		if err != nil {
			log.Printf("Failed to schedule task %s: %v", task.ID, err)
			continue
		}

		// 自动启动已调度的任务
		_, err = m.StartTask(task.ID)
		if err != nil {
			log.Printf("Failed to start task %s: %v", task.ID, err)
		}
	}
}

// checkHeartbeats 检查节点心跳
func (m *Manager) checkHeartbeats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, node := range m.nodes {
		if node.Status == NodeStatusOnline && time.Since(node.LastHeartbeat) > m.schedulerConfig.HeartbeatTimeout {
			node.Status = NodeStatusOffline
			log.Printf("Node %s marked offline due to heartbeat timeout", node.ID)
		}
	}
}
