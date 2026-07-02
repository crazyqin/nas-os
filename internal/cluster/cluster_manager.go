// Package cluster 集群管理器模块
// 支持多节点统一管理、负载均衡、故障转移
// 参考: 群晖 Cluster Manager 功能
package cluster

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// NodeStatus 节点状态.
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusSyncing NodeStatus = "syncing"
	NodeStatusError   NodeStatus = "error"
)

// NodeType 节点类型.
type NodeType string

const (
	NodeTypePrimary   NodeType = "primary"
	NodeTypeSecondary NodeType = "secondary"
	NodeTypeWorker    NodeType = "worker"
)

// ClusterNode 集群节点.
type ClusterNode struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname"`
	IP        string            `json:"ip"`
	Port      int               `json:"port"`
	Type      NodeType          `json:"type"`
	Status    NodeStatus        `json:"status"`
	CPUCores  int               `json:"cpu_cores"`
	MemoryGB  int               `json:"memory_gb"`
	StorageGB int               `json:"storage_gb"`
	UsedGB    int               `json:"used_gb"`
	Version   string            `json:"version"`
	LastSeen  time.Time         `json:"last_seen"`
	JoinedAt  time.Time         `json:"joined_at"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ClusterConfig 集群配置.
type ClusterConfig struct {
	ClusterName       string        `json:"cluster_name"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`
	MaxNodes          int           `json:"max_nodes"`
	AutoFailover      bool          `json:"auto_failover"`
	FailoverTimeout   time.Duration `json:"failover_timeout"`
	LoadBalancePolicy string        `json:"load_balance_policy"` // round-robin, least-connections, resource-based
	SyncInterval      time.Duration `json:"sync_interval"`
	EnableHA          bool          `json:"enable_ha"` // 高可用
}

// DefaultClusterConfig 默认集群配置.
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		ClusterName:       "nas-os-cluster",
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  30 * time.Second,
		MaxNodes:          32,
		AutoFailover:      true,
		FailoverTimeout:   60 * time.Second,
		LoadBalancePolicy: "resource-based",
		SyncInterval:      5 * time.Minute,
		EnableHA:          true,
	}
}

// ClusterManager 集群管理器.
type ClusterManager struct {
	mu            sync.RWMutex
	config        *ClusterConfig
	nodes         map[string]*ClusterNode
	primaryNode   *ClusterNode
	isRunning     bool
	ctx           context.Context
	cancel        context.CancelFunc
	eventHandlers []EventHandler
	taskQueue     *TaskQueue
	metrics       *ClusterMetrics
}

// ClusterMetrics 集群指标.
type ClusterMetrics struct {
	TotalNodes     int       `json:"total_nodes"`
	OnlineNodes    int       `json:"online_nodes"`
	OfflineNodes   int       `json:"offline_nodes"`
	TotalCPU       int       `json:"total_cpu"`
	TotalMemoryGB  int       `json:"total_memory_gb"`
	TotalStorageGB int       `json:"total_storage_gb"`
	UsedStorageGB  int       `json:"used_storage_gb"`
	LastUpdated    time.Time `json:"last_updated"`
}

// EventHandler 事件处理器.
type EventHandler func(event ClusterEvent)

// ClusterEvent 集群事件.
type ClusterEvent struct {
	Type      string      `json:"type"`
	NodeID    string      `json:"node_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// TaskQueue 任务队列.
type TaskQueue struct {
	mu       sync.Mutex
	tasks    []ClusterTask
	priority map[string]int
}

// ClusterTask 集群任务.
type ClusterTask struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	TargetNode  string      `json:"target_node"`
	Payload     interface{} `json:"payload"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// NewClusterManager 创建集群管理器.
func NewClusterManager(config *ClusterConfig) *ClusterManager {
	if config == nil {
		config = DefaultClusterConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ClusterManager{
		config:    config,
		nodes:     make(map[string]*ClusterNode),
		ctx:       ctx,
		cancel:    cancel,
		taskQueue: &TaskQueue{tasks: make([]ClusterTask, 0), priority: make(map[string]int)},
		metrics:   &ClusterMetrics{},
	}
}

// Start 启动集群管理器.
func (cm *ClusterManager) Start() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.isRunning {
		return fmt.Errorf("cluster manager is already running")
	}

	cm.isRunning = true
	log.Printf("集群管理器启动: %s", cm.config.ClusterName)

	// 启动心跳检测
	go cm.heartbeatLoop()

	// 启动指标收集
	go cm.metricsCollector()

	// 启动任务调度器
	go cm.taskScheduler()

	return nil
}

// Stop 停止集群管理器.
func (cm *ClusterManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.isRunning {
		return
	}

	cm.cancel()
	cm.isRunning = false
	log.Printf("集群管理器已停止")
}

// AddNode 添加节点.
func (cm *ClusterManager) AddNode(node *ClusterNode) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.nodes) >= cm.config.MaxNodes {
		return fmt.Errorf("maximum nodes reached: %d", cm.config.MaxNodes)
	}

	if _, exists := cm.nodes[node.ID]; exists {
		return fmt.Errorf("node already exists: %s", node.ID)
	}

	node.Status = NodeStatusOnline
	node.LastSeen = time.Now()
	node.JoinedAt = time.Now()

	cm.nodes[node.ID] = node

	// 如果是第一个节点，设为主节点
	if len(cm.nodes) == 1 {
		node.Type = NodeTypePrimary
		cm.primaryNode = node
	}

	cm.emitEvent(ClusterEvent{
		Type:      "node_added",
		NodeID:    node.ID,
		Timestamp: time.Now(),
		Data:      node,
	})

	log.Printf("节点已添加: %s (%s)", node.Name, node.IP)
	return nil
}

// RemoveNode 移除节点.
func (cm *ClusterManager) RemoveNode(nodeID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// 不能移除主节点（除非集群只剩一个节点）
	if node.Type == NodeTypePrimary && len(cm.nodes) > 1 {
		return fmt.Errorf("cannot remove primary node, promote another node first")
	}

	delete(cm.nodes, nodeID)

	// 如果移除的是主节点，选择新主节点
	if node.Type == NodeTypePrimary {
		cm.electNewPrimary()
	}

	cm.emitEvent(ClusterEvent{
		Type:      "node_removed",
		NodeID:    nodeID,
		Timestamp: time.Now(),
	})

	log.Printf("节点已移除: %s", nodeID)
	return nil
}

// GetNode 获取节点.
func (cm *ClusterManager) GetNode(nodeID string) (*ClusterNode, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return node, nil
}

// ListNodes 列出所有节点.
func (cm *ClusterManager) ListNodes() []*ClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	nodes := make([]*ClusterNode, 0, len(cm.nodes))
	for _, node := range cm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetOnlineNodes 获取在线节点.
func (cm *ClusterManager) GetOnlineNodes() []*ClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var online []*ClusterNode
	for _, node := range cm.nodes {
		if node.Status == NodeStatusOnline {
			online = append(online, node)
		}
	}
	return online
}

// GetPrimaryNode 获取主节点.
func (cm *ClusterManager) GetPrimaryNode() *ClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.primaryNode
}

// PromoteNode 提升节点为主节点.
func (cm *ClusterManager) PromoteNode(nodeID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	if node.Status != NodeStatusOnline {
		return fmt.Errorf("cannot promote offline node")
	}

	// 降级当前主节点
	if cm.primaryNode != nil {
		cm.primaryNode.Type = NodeTypeSecondary
	}

	// 提升新节点
	node.Type = NodeTypePrimary
	cm.primaryNode = node

	cm.emitEvent(ClusterEvent{
		Type:      "primary_changed",
		NodeID:    nodeID,
		Timestamp: time.Now(),
	})

	log.Printf("新主节点: %s", node.Name)
	return nil
}

// SelectNodeForTask 为任务选择最佳节点.
func (cm *ClusterManager) SelectNodeForTask(requirements *TaskRequirements) (*ClusterNode, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var candidates []*ClusterNode

	// 过滤符合条件的节点
	for _, node := range cm.nodes {
		if node.Status != NodeStatusOnline {
			continue
		}
		if requirements != nil {
			if requirements.CPU > 0 && node.CPUCores < requirements.CPU {
				continue
			}
			if requirements.Memory > 0 && node.MemoryGB < int(requirements.Memory/1024) {
				continue
			}
		}
		candidates = append(candidates, node)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable nodes available")
	}

	// 根据负载均衡策略选择节点
	return cm.selectByPolicy(candidates), nil
}

// selectByPolicy 根据策略选择节点.
func (cm *ClusterManager) selectByPolicy(candidates []*ClusterNode) *ClusterNode {
	switch cm.config.LoadBalancePolicy {
	case "round-robin":
		return cm.selectRoundRobin(candidates)
	case "least-connections":
		return cm.selectLeastConnections(candidates)
	case "resource-based":
		return cm.selectResourceBased(candidates)
	default:
		return candidates[0]
	}
}

// selectRoundRobin 轮询选择.
func (cm *ClusterManager) selectRoundRobin(candidates []*ClusterNode) *ClusterNode {
	// 简单轮询
	idx := time.Now().UnixNano() % int64(len(candidates))
	return candidates[idx]
}

// selectLeastConnections 最少连接选择.
func (cm *ClusterManager) selectLeastConnections(candidates []*ClusterNode) *ClusterNode {
	// 选择使用率最低的节点
	var best *ClusterNode
	bestUsage := 100.0

	for _, node := range candidates {
		usage := float64(node.UsedGB) / float64(node.StorageGB) * 100
		if usage < bestUsage {
			bestUsage = usage
			best = node
		}
	}

	if best == nil {
		return candidates[0]
	}
	return best
}

// selectResourceBased 基于资源选择.
func (cm *ClusterManager) selectResourceBased(candidates []*ClusterNode) *ClusterNode {
	// 综合考虑CPU、内存、存储使用率
	var best *ClusterNode
	bestScore := -1.0

	for _, node := range candidates {
		// 计算综合得分（越高越好）
		cpuScore := float64(node.CPUCores)
		memScore := float64(node.MemoryGB)
		storageScore := float64(node.StorageGB-node.UsedGB) / float64(node.StorageGB) * 100

		score := cpuScore*0.3 + memScore*0.3 + storageScore*0.4

		if score > bestScore {
			bestScore = score
			best = node
		}
	}

	if best == nil {
		return candidates[0]
	}
	return best
}

// SubmitTask 提交任务.
func (cm *ClusterManager) SubmitTask(task ClusterTask) error {
	cm.taskQueue.mu.Lock()
	defer cm.taskQueue.mu.Unlock()

	task.Status = "pending"
	task.CreatedAt = time.Now()

	cm.taskQueue.tasks = append(cm.taskQueue.tasks, task)

	cm.emitEvent(ClusterEvent{
		Type:      "task_submitted",
		NodeID:    task.TargetNode,
		Timestamp: time.Now(),
		Data:      task,
	})

	return nil
}

// GetTasks 获取任务列表.
func (cm *ClusterManager) GetTasks() []ClusterTask {
	cm.taskQueue.mu.Lock()
	defer cm.taskQueue.mu.Unlock()

	tasks := make([]ClusterTask, len(cm.taskQueue.tasks))
	copy(tasks, cm.taskQueue.tasks)
	return tasks
}

// GetMetrics 获取集群指标.
func (cm *ClusterManager) GetMetrics() *ClusterMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.metrics
}

// AddEventHandler 添加事件处理器.
func (cm *ClusterManager) AddEventHandler(handler EventHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.eventHandlers = append(cm.eventHandlers, handler)
}

// heartbeatLoop 心跳检测循环.
func (cm *ClusterManager) heartbeatLoop() {
	ticker := time.NewTicker(cm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.checkNodesHealth()
		}
	}
}

// checkNodesHealth 检查节点健康状态.
func (cm *ClusterManager) checkNodesHealth() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()

	for _, node := range cm.nodes {
		if now.Sub(node.LastSeen) > cm.config.HeartbeatTimeout {
			if node.Status == NodeStatusOnline {
				node.Status = NodeStatusOffline
				log.Printf("节点离线: %s", node.Name)

				cm.emitEvent(ClusterEvent{
					Type:      "node_offline",
					NodeID:    node.ID,
					Timestamp: now,
				})

				// 如果是主节点离线，触发故障转移
				if node.Type == NodeTypePrimary && cm.config.AutoFailover {
					cm.electNewPrimary()
				}
			}
		}
	}
}

// electNewPrimary 选举新主节点.
func (cm *ClusterManager) electNewPrimary() {
	var bestCandidate *ClusterNode

	for _, node := range cm.nodes {
		if node.Status == NodeStatusOnline && node.Type != NodeTypePrimary {
			if bestCandidate == nil || node.JoinedAt.Before(bestCandidate.JoinedAt) {
				bestCandidate = node
			}
		}
	}

	if bestCandidate != nil {
		bestCandidate.Type = NodeTypePrimary
		cm.primaryNode = bestCandidate
		log.Printf("故障转移完成，新主节点: %s", bestCandidate.Name)

		cm.emitEvent(ClusterEvent{
			Type:      "failover",
			NodeID:    bestCandidate.ID,
			Timestamp: time.Now(),
			Data:      map[string]string{"new_primary": bestCandidate.Name},
		})
	}
}

// metricsCollector 指标收集器.
func (cm *ClusterManager) metricsCollector() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.updateMetrics()
		}
	}
}

// updateMetrics 更新集群指标.
func (cm *ClusterManager) updateMetrics() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	metrics := &ClusterMetrics{
		TotalNodes:  len(cm.nodes),
		LastUpdated: time.Now(),
	}

	for _, node := range cm.nodes {
		metrics.TotalCPU += node.CPUCores
		metrics.TotalMemoryGB += node.MemoryGB
		metrics.TotalStorageGB += node.StorageGB
		metrics.UsedStorageGB += node.UsedGB

		if node.Status == NodeStatusOnline {
			metrics.OnlineNodes++
		} else {
			metrics.OfflineNodes++
		}
	}

	cm.metrics = metrics
}

// taskScheduler 任务调度器.
func (cm *ClusterManager) taskScheduler() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.processTasks()
		}
	}
}

// processTasks 处理任务队列.
func (cm *ClusterManager) processTasks() {
	cm.taskQueue.mu.Lock()
	defer cm.taskQueue.mu.Unlock()

	for i := range cm.taskQueue.tasks {
		task := &cm.taskQueue.tasks[i]
		if task.Status != "pending" {
			continue
		}

		// 选择目标节点
		node, err := cm.SelectNodeForTask(nil)
		if err != nil {
			task.Status = "failed"
			task.Error = err.Error()
			continue
		}

		task.TargetNode = node.ID
		task.Status = "running"
		now := time.Now()
		task.StartedAt = &now

		// 异步执行任务
		go cm.executeTask(task)
	}
}

// executeTask 执行任务.
func (cm *ClusterManager) executeTask(task *ClusterTask) {
	// 模拟任务执行
	time.Sleep(1 * time.Second)

	task.Status = "completed"
	now := time.Now()
	task.CompletedAt = &now

	cm.emitEvent(ClusterEvent{
		Type:      "task_completed",
		NodeID:    task.TargetNode,
		Timestamp: now,
		Data:      task,
	})
}

// emitEvent 发送事件.
func (cm *ClusterManager) emitEvent(event ClusterEvent) {
	for _, handler := range cm.eventHandlers {
		go handler(event)
	}
}

// GetClusterStatus 获取集群状态.
func (cm *ClusterManager) GetClusterStatus() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return map[string]interface{}{
		"cluster_name":  cm.config.ClusterName,
		"is_running":    cm.isRunning,
		"total_nodes":   len(cm.nodes),
		"primary_node":  cm.primaryNode,
		"load_balance":  cm.config.LoadBalancePolicy,
		"ha_enabled":    cm.config.EnableHA,
		"auto_failover": cm.config.AutoFailover,
		"metrics":       cm.metrics,
	}
}

// UpdateNodeStatus 更新节点状态.
func (cm *ClusterManager) UpdateNodeStatus(nodeID string, status NodeStatus) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	oldStatus := node.Status
	node.Status = status
	node.LastSeen = time.Now()

	if oldStatus != status {
		cm.emitEvent(ClusterEvent{
			Type:      "status_changed",
			NodeID:    nodeID,
			Timestamp: time.Now(),
			Data:      map[string]string{"old": string(oldStatus), "new": string(status)},
		})
	}

	return nil
}

// UpdateNodeMetrics 更新节点指标.
func (cm *ClusterManager) UpdateNodeMetrics(nodeID string, usedGB int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.UsedGB = usedGB
	node.LastSeen = time.Now()

	return nil
}
