package edgenodemanager

import (
	"fmt"
	sortlib "sort"
	"sync"
	"time"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeOnline      NodeStatus = "online"
	NodeOffline     NodeStatus = "offline"
	NodeMaintenance NodeStatus = "maintenance"
	NodeDegraded    NodeStatus = "degraded"
)

// NodeRole 节点角色
type NodeRole string

const (
	RoleWorker  NodeRole = "worker"
	RoleGateway NodeRole = "gateway"
	RoleCompute NodeRole = "compute"
	RoleStorage NodeRole = "storage"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy string

const (
	StrategyRoundRobin    LoadBalanceStrategy = "round_robin"
	StrategyLeastLoad     LoadBalanceStrategy = "least_load"
	StrategyResourceBased LoadBalanceStrategy = "resource_based"
	StrategyLatencyBased  LoadBalanceStrategy = "latency_based"
)

// NodeMetrics 节点指标
type NodeMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIn   int64   `json:"network_in_bytes"`
	NetworkOut  int64   `json:"network_out_bytes"`
	LoadAverage float64 `json:"load_average"`
	Timestamp   time.Time `json:"timestamp"`
}

// EdgeNode 边缘节点
type EdgeNode struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	IPAddress   string       `json:"ip_address"`
	Port        int          `json:"port"`
	Role        NodeRole     `json:"role"`
	Status      NodeStatus   `json:"status"`
	Labels      map[string]string `json:"labels,omitempty"`
	Metrics     *NodeMetrics `json:"metrics,omitempty"`
	Version     string       `json:"version"`
	Region      string       `json:"region"`
	Zone        string       `json:"zone"`
	LastSeen    time.Time    `json:"last_seen"`
	RegisteredAt time.Time   `json:"registered_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ComputeTask 计算任务
type ComputeTask struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Payload     []byte     `json:"payload,omitempty"`
	AssignedTo  string     `json:"assigned_to,omitempty"`
	Status      TaskStatus `json:"status"`
	Priority    int        `json:"priority"`
	MaxRetries  int        `json:"max_retries"`
	RetryCount  int        `json:"retry_count"`
	Result      []byte     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// DeployRequest 部署请求
type DeployRequest struct {
	ID          string    `json:"id"`
	TargetNodes []string  `json:"target_nodes"`
	Image       string    `json:"image"`
	Version     string    `json:"version"`
	Config      map[string]string `json:"config,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// DataSync 数据同步记录
type DataSync struct {
	ID          string    `json:"id"`
	SourceNode  string    `json:"source_node"`
	TargetNodes []string  `json:"target_nodes"`
	SyncKey     string    `json:"sync_key"`
	SyncType    string    `json:"sync_type"`
	Status      string    `json:"status"`
	BytesSynced int64     `json:"bytes_synced"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ClusterStats 集群统计
type ClusterStats struct {
	TotalNodes      int            `json:"total_nodes"`
	OnlineNodes     int            `json:"online_nodes"`
	OfflineNodes    int            `json:"offline_nodes"`
	TotalTasks      int            `json:"total_tasks"`
	RunningTasks    int            `json:"running_tasks"`
	PendingTasks    int            `json:"pending_tasks"`
	CompletedTasks  int            `json:"completed_tasks"`
	FailedTasks     int            `json:"failed_tasks"`
	AvgCPUUsage     float64        `json:"avg_cpu_usage"`
	AvgMemoryUsage  float64        `json:"avg_memory_usage"`
	TasksByNode     map[string]int `json:"tasks_by_node"`
}

// EdgeNodeManager 边缘节点管理器
type EdgeNodeManager struct {
	mu           sync.RWMutex
	nodes        map[string]*EdgeNode
	tasks        map[string]*ComputeTask
	deploys      map[string]*DeployRequest
	syncs        map[string]*DataSync
	strategy     LoadBalanceStrategy
	roundRobinIdx int
}

// NewEdgeNodeManager 创建边缘节点管理器
func NewEdgeNodeManager(strategy LoadBalanceStrategy) *EdgeNodeManager {
	return &EdgeNodeManager{
		nodes:    make(map[string]*EdgeNode),
		tasks:    make(map[string]*ComputeTask),
		deploys:  make(map[string]*DeployRequest),
		syncs:    make(map[string]*DataSync),
		strategy: strategy,
	}
}

// RegisterNode 注册节点
func (m *EdgeNodeManager) RegisterNode(node *EdgeNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已存在", node.ID)
	}

	node.Status = NodeOnline
	node.RegisteredAt = time.Now()
	node.UpdatedAt = time.Now()
	node.LastSeen = time.Now()

	m.nodes[node.ID] = node
	return nil
}

// UnregisterNode 注销节点
func (m *EdgeNodeManager) UnregisterNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[nodeID]; !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	delete(m.nodes, nodeID)
	return nil
}

// GetNode 获取节点信息
func (m *EdgeNodeManager) GetNode(nodeID string) (*EdgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点 %s 不存在", nodeID)
	}
	return node, nil
}

// ListNodes 列出所有节点
func (m *EdgeNodeManager) ListNodes(status NodeStatus, role NodeRole, region string) []*EdgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*EdgeNode, 0)
	for _, node := range m.nodes {
		if status != "" && node.Status != status {
			continue
		}
		if role != "" && node.Role != role {
			continue
		}
		if region != "" && node.Region != region {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// UpdateNodeMetrics 更新节点指标
func (m *EdgeNodeManager) UpdateNodeMetrics(nodeID string, metrics *NodeMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	metrics.Timestamp = time.Now()
	node.Metrics = metrics
	node.LastSeen = time.Now()
	node.UpdatedAt = time.Now()

	return nil
}

// UpdateNodeStatus 更新节点状态
func (m *EdgeNodeManager) UpdateNodeStatus(nodeID string, status NodeStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	node.Status = status
	node.UpdatedAt = time.Now()

	return nil
}

// DiscoverNodes 发现节点
func (m *EdgeNodeManager) DiscoverNodes(region string, labels map[string]string) []*EdgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*EdgeNode, 0)
	for _, node := range m.nodes {
		if node.Status != NodeOnline {
			continue
		}
		if region != "" && node.Region != region {
			continue
		}
		if len(labels) > 0 && !matchLabels(node.Labels, labels) {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// SubmitTask 提交计算任务
func (m *EdgeNodeManager) SubmitTask(task *ComputeTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}

	task.Status = TaskPending
	task.CreatedAt = time.Now()

	m.tasks[task.ID] = task
	return nil
}

// ScheduleTask 调度任务到节点
func (m *EdgeNodeManager) ScheduleTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status != TaskPending {
		return fmt.Errorf("任务 %s 不在待处理状态", taskID)
	}

	// 根据策略选择节点
	node := m.selectNode()
	if node == nil {
		return fmt.Errorf("没有可用的在线节点")
	}

	now := time.Now()
	task.AssignedTo = node.ID
	task.Status = TaskRunning
	task.StartedAt = &now

	return nil
}

// selectNode 根据负载均衡策略选择节点
func (m *EdgeNodeManager) selectNode() *EdgeNode {
	onlineNodes := make([]*EdgeNode, 0)
	for _, node := range m.nodes {
		if node.Status == NodeOnline {
			onlineNodes = append(onlineNodes, node)
		}
	}

	if len(onlineNodes) == 0 {
		return nil
	}

	// 按ID排序保证确定性
	sortlib.Slice(onlineNodes, func(i, j int) bool {
		return onlineNodes[i].ID < onlineNodes[j].ID
	})

	switch m.strategy {
	case StrategyRoundRobin:
		node := onlineNodes[m.roundRobinIdx%len(onlineNodes)]
		m.roundRobinIdx++
		return node
	case StrategyLeastLoad:
		return selectLeastLoad(onlineNodes)
	case StrategyResourceBased:
		return selectResourceBased(onlineNodes)
	default:
		return onlineNodes[0]
	}
}

// selectLeastLoad 选择负载最低的节点
func selectLeastLoad(nodes []*EdgeNode) *EdgeNode {
	var selected *EdgeNode
	minLoad := float64(100)

	for _, node := range nodes {
		if node.Metrics == nil {
			return node
		}
		load := (node.Metrics.CPUUsage + node.Metrics.MemoryUsage) / 2
		if load < minLoad {
			minLoad = load
			selected = node
		}
	}

	if selected == nil {
		return nodes[0]
	}
	return selected
}

// selectResourceBased 基于资源选择节点
func selectResourceBased(nodes []*EdgeNode) *EdgeNode {
	var selected *EdgeNode
	bestScore := float64(-1)

	for _, node := range nodes {
		if node.Metrics == nil {
			return node
		}
		// 计算资源得分 (100 - usage) = available
		score := (100 - node.Metrics.CPUUsage) * 0.4 + (100 - node.Metrics.MemoryUsage) * 0.4 + (100 - node.Metrics.DiskUsage) * 0.2
		if score > bestScore {
			bestScore = score
			selected = node
		}
	}

	if selected == nil {
		return nodes[0]
	}
	return selected
}

// CompleteTask 完成任务
func (m *EdgeNodeManager) CompleteTask(taskID string, result []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	now := time.Now()
	task.Status = TaskCompleted
	task.Result = result
	task.CompletedAt = &now

	return nil
}

// FailTask 标记任务失败
func (m *EdgeNodeManager) FailTask(taskID string, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	task.Error = errMsg
	task.RetryCount++

	if task.RetryCount < task.MaxRetries {
		task.Status = TaskPending
		task.AssignedTo = ""
		task.StartedAt = nil
	} else {
		task.Status = TaskFailed
		now := time.Now()
		task.CompletedAt = &now
	}

	return nil
}

// GetTask 获取任务
func (m *EdgeNodeManager) GetTask(taskID string) (*ComputeTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListTasks 列出任务
func (m *EdgeNodeManager) ListTasks(nodeID string, status TaskStatus) []*ComputeTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ComputeTask, 0)
	for _, task := range m.tasks {
		if nodeID != "" && task.AssignedTo != nodeID {
			continue
		}
		if status != "" && task.Status != status {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

// Deploy 部署到节点
func (m *EdgeNodeManager) Deploy(req *DeployRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证目标节点存在
	for _, nodeID := range req.TargetNodes {
		if _, exists := m.nodes[nodeID]; !exists {
			return fmt.Errorf("目标节点 %s 不存在", nodeID)
		}
	}

	req.Status = "pending"
	req.CreatedAt = time.Now()

	m.deploys[req.ID] = req
	return nil
}

// GetDeploy 获取部署状态
func (m *EdgeNodeManager) GetDeploy(deployID string) (*DeployRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	deploy, exists := m.deploys[deployID]
	if !exists {
		return nil, fmt.Errorf("部署 %s 不存在", deployID)
	}
	return deploy, nil
}

// CompleteDeploy 完成部署
func (m *EdgeNodeManager) CompleteDeploy(deployID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	deploy, exists := m.deploys[deployID]
	if !exists {
		return fmt.Errorf("部署 %s 不存在", deployID)
	}

	now := time.Now()
	deploy.Status = "completed"
	deploy.CompletedAt = &now

	return nil
}

// StartDataSync 启动数据同步
func (m *EdgeNodeManager) StartDataSync(sync *DataSync) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[sync.SourceNode]; !exists {
		return fmt.Errorf("源节点 %s 不存在", sync.SourceNode)
	}

	for _, nodeID := range sync.TargetNodes {
		if _, exists := m.nodes[nodeID]; !exists {
			return fmt.Errorf("目标节点 %s 不存在", nodeID)
		}
	}

	sync.Status = "syncing"
	sync.StartedAt = time.Now()

	m.syncs[sync.ID] = sync
	return nil
}

// CompleteDataSync 完成数据同步
func (m *EdgeNodeManager) CompleteDataSync(syncID string, bytesSynced int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sync, exists := m.syncs[syncID]
	if !exists {
		return fmt.Errorf("同步 %s 不存在", syncID)
	}

	now := time.Now()
	sync.Status = "completed"
	sync.BytesSynced = bytesSynced
	sync.CompletedAt = &now

	return nil
}

// GetDataSync 获取同步状态
func (m *EdgeNodeManager) GetDataSync(syncID string) (*DataSync, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sync, exists := m.syncs[syncID]
	if !exists {
		return nil, fmt.Errorf("同步 %s 不存在", syncID)
	}
	return sync, nil
}

// GetClusterStats 获取集群统计
func (m *EdgeNodeManager) GetClusterStats() *ClusterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ClusterStats{
		TasksByNode: make(map[string]int),
	}

	totalCPU := float64(0)
	totalMem := float64(0)
	onlineWithMetrics := 0

	for _, node := range m.nodes {
		stats.TotalNodes++
		switch node.Status {
		case NodeOnline:
			stats.OnlineNodes++
		case NodeOffline:
			stats.OfflineNodes++
		}

		if node.Metrics != nil {
			totalCPU += node.Metrics.CPUUsage
			totalMem += node.Metrics.MemoryUsage
			onlineWithMetrics++
		}
	}

	if onlineWithMetrics > 0 {
		stats.AvgCPUUsage = totalCPU / float64(onlineWithMetrics)
		stats.AvgMemoryUsage = totalMem / float64(onlineWithMetrics)
	}

	for _, task := range m.tasks {
		stats.TotalTasks++
		switch task.Status {
		case TaskRunning:
			stats.RunningTasks++
		case TaskPending:
			stats.PendingTasks++
		case TaskCompleted:
			stats.CompletedTasks++
		case TaskFailed:
			stats.FailedTasks++
		}
		if task.AssignedTo != "" {
			stats.TasksByNode[task.AssignedTo]++
		}
	}

	return stats
}

// CheckNodeHealth 检查节点健康状态，标记超时节点为离线
func (m *EdgeNodeManager) CheckNodeHealth(timeout time.Duration) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	offlineNodes := make([]string, 0)
	cutoff := time.Now().Add(-timeout)

	for _, node := range m.nodes {
		if node.Status == NodeOnline && node.LastSeen.Before(cutoff) {
			node.Status = NodeOffline
			node.UpdatedAt = time.Now()
			offlineNodes = append(offlineNodes, node.ID)
		}
	}

	return offlineNodes
}

// matchLabels 检查标签是否匹配
func matchLabels(nodeLabels, required map[string]string) bool {
	for key, val := range required {
		nodeVal, exists := nodeLabels[key]
		if !exists || nodeVal != val {
			return false
		}
	}
	return true
}
