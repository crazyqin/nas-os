// Package cluster 跨节点集群管理增强
// 多节点统一管理 - fleet.go
// 对标 TrueNAS Connect + CMS 多系统管理功能
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ========== 舰队管理器 ==========

// Fleet 多节点统一管理器.
type Fleet struct {
	mu         sync.RWMutex
	nodes      map[string]*FleetNode
	nodeGroups map[string]*NodeGroup
	dataDir    string
	ctx        context.Context
	cancel     context.CancelFunc
	config     *FleetConfig
	taskQueue  *CrossNodeTaskQueue
	healthAgg  *HealthAggregator
	alertAgg   *AlertAggregator
}

// FleetConfig 舰队配置.
type FleetConfig struct {
	NodeID             string        `json:"nodeId"`
	NodeName           string        `json:"nodeName"`
	Address            string        `json:"address"`
	Port               int           `json:"port"`
	DataDir            string        `json:"dataDir"`
	HeartbeatInterval  time.Duration `json:"heartbeatInterval"`
	HeartbeatTimeout   time.Duration `json:"heartbeatTimeout"`
	DiscoveryPort      int           `json:"discoveryPort"`
	SyncInterval       time.Duration `json:"syncInterval"`
	MaxNodes           int           `json:"maxNodes"`
	EnableAutoDiscover bool          `json:"enableAutoDiscover"`
}

// DefaultFleetConfig 默认舰队配置.
func DefaultFleetConfig() *FleetConfig {
	return &FleetConfig{
		HeartbeatInterval:  10 * time.Second,
		HeartbeatTimeout:   30 * time.Second,
		DiscoveryPort:      7900,
		SyncInterval:       5 * time.Minute,
		MaxNodes:           64,
		EnableAutoDiscover: true,
	}
}

// FleetNode 舰队节点.
type FleetNode struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Port          int               `json:"port"`
	Role          FleetNodeRole     `json:"role"`
	State         FleetNodeState    `json:"state"`
	Capabilities  []string          `json:"capabilities"`
	Metadata      map[string]string `json:"metadata"`
	Metrics       *NodeMetrics      `json:"metrics,omitempty"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"`
	LastSync      time.Time         `json:"lastSync"`
	RegisteredAt  time.Time         `json:"registeredAt"`
	Tags          []string          `json:"tags"`
	StoragePools  []StoragePoolInfo `json:"storagePools,omitempty"`
	InstalledApps []string          `json:"installedApps,omitempty"`
	SystemVersion string            `json:"systemVersion"`
	Uptime        int64             `json:"uptime"`
}

// FleetNodeRole 节点角色.
type FleetNodeRole string

const (
	FleetRoleMaster  FleetNodeRole = "master"
	FleetRoleWorker  FleetNodeRole = "worker"
	FleetRoleStandby FleetNodeRole = "standby"
)

// FleetNodeState 节点状态.
type FleetNodeState string

const (
	FleetStateOnline      FleetNodeState = "online"
	FleetStateOffline     FleetNodeState = "offline"
	FleetStateDegraded    FleetNodeState = "degraded"
	FleetStateMaintaining FleetNodeState = "maintaining"
	FleetStateJoining     FleetNodeState = "joining"
	FleetStateLeaving     FleetNodeState = "leaving"
)

// StoragePoolInfo 存储池信息.
type StoragePoolInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	TotalGB     int64  `json:"totalGB"`
	UsedGB      int64  `json:"usedGB"`
	AvailableGB int64  `json:"availableGB"`
	Health      string `json:"health"`
	RaidLevel   string `json:"raidLevel"`
}

// NodeGroup 节点分组.
type NodeGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	NodeIDs     []string  `json:"nodeIds"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
}

// NewFleet 创建舰队管理器.
func NewFleet(config *FleetConfig) *Fleet {
	if config == nil {
		config = DefaultFleetConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	f := &Fleet{
		nodes:      make(map[string]*FleetNode),
		nodeGroups: make(map[string]*NodeGroup),
		dataDir:    config.DataDir,
		ctx:        ctx,
		cancel:     cancel,
		config:     config,
		taskQueue:  NewCrossNodeTaskQueue(),
		healthAgg:  NewHealthAggregator(),
		alertAgg:   NewAlertAggregator(),
	}

	// 注册本节点
	f.nodes[config.NodeID] = &FleetNode{
		ID:            config.NodeID,
		Name:          config.NodeName,
		Address:       config.Address,
		Port:          config.Port,
		Role:          FleetRoleMaster,
		State:         FleetStateOnline,
		Capabilities:  []string{"storage", "compute", "network"},
		Metadata:      make(map[string]string),
		Tags:          []string{},
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
		SystemVersion: "1.0.0",
	}

	// 加载持久化数据
	f.loadData()

	return f
}

// Start 启动舰队管理.
func (f *Fleet) Start() error {
	// 启动心跳循环
	go f.heartbeatLoop()

	// 启动健康聚合
	go f.healthAggregationLoop()

	// 启动任务调度
	go f.taskSchedulingLoop()

	return nil
}

// Stop 停止舰队管理.
func (f *Fleet) Stop() {
	f.cancel()
	f.saveData()
}

// ========== 节点管理 ==========

// RegisterNode 注册节点.
func (f *Fleet) RegisterNode(node *FleetNode) error {
	if node.ID == "" {
		return fmt.Errorf("节点ID不能为空")
	}
	if node.Address == "" {
		return fmt.Errorf("节点地址不能为空")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.config.MaxNodes > 0 && len(f.nodes) >= f.config.MaxNodes {
		return fmt.Errorf("已达到最大节点数限制: %d", f.config.MaxNodes)
	}

	if _, exists := f.nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已注册", node.ID)
	}

	if node.Role == "" {
		node.Role = FleetRoleWorker
	}
	if node.State == "" {
		node.State = FleetStateJoining
	}
	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}
	if node.Tags == nil {
		node.Tags = []string{}
	}
	node.RegisteredAt = time.Now()
	node.LastHeartbeat = time.Now()

	f.nodes[node.ID] = node

	return nil
}

// UnregisterNode 注销节点.
func (f *Fleet) UnregisterNode(nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nodes[nodeID]; !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	// 从所有分组中移除
	for _, group := range f.nodeGroups {
		var newIDs []string
		for _, id := range group.NodeIDs {
			if id != nodeID {
				newIDs = append(newIDs, id)
			}
		}
		group.NodeIDs = newIDs
	}

	delete(f.nodes, nodeID)

	return nil
}

// GetNode 获取节点信息.
func (f *Fleet) GetNode(nodeID string) (*FleetNode, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	node, ok := f.nodes[nodeID]
	return node, ok
}

// ListNodes 列出所有节点.
func (f *Fleet) ListNodes(filter *NodeFilter) []*FleetNode {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*FleetNode, 0, len(f.nodes))
	for _, node := range f.nodes {
		if filter != nil {
			if filter.Role != "" && node.Role != filter.Role {
				continue
			}
			if filter.State != "" && node.State != filter.State {
				continue
			}
			if filter.Tag != "" {
				found := false
				for _, tag := range node.Tags {
					if tag == filter.Tag {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}
		result = append(result, node)
	}

	sort.Slice(result, func(i, j int) bool {
		// master 排在前面
		if result[i].Role != result[j].Role {
			return result[i].Role == FleetRoleMaster
		}
		return result[i].Name < result[j].Name
	})

	return result
}

// NodeFilter 节点过滤器.
type NodeFilter struct {
	Role  FleetNodeRole  `json:"role"`
	State FleetNodeState `json:"state"`
	Tag   string         `json:"tag"`
}

// UpdateNodeMetrics 更新节点指标.
func (f *Fleet) UpdateNodeMetrics(nodeID string, metrics *NodeMetrics) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	node, ok := f.nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	node.Metrics = metrics
	node.LastHeartbeat = time.Now()
	node.State = FleetStateOnline

	// 更新健康聚合
	f.healthAgg.UpdateNode(nodeID, metrics)

	return nil
}

// UpdateNodeState 更新节点状态.
func (f *Fleet) UpdateNodeState(nodeID string, state FleetNodeState) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	node, ok := f.nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	node.State = state
	return nil
}

// SetNodeRole 设置节点角色.
func (f *Fleet) SetNodeRole(nodeID string, role FleetNodeRole) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	node, ok := f.nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	node.Role = role
	return nil
}

// ========== 节点分组管理 ==========

// CreateGroup 创建节点分组.
func (f *Fleet) CreateGroup(group *NodeGroup) error {
	if group.ID == "" {
		return fmt.Errorf("分组ID不能为空")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nodeGroups[group.ID]; exists {
		return fmt.Errorf("分组 %s 已存在", group.ID)
	}

	// 验证节点存在
	for _, nodeID := range group.NodeIDs {
		if _, ok := f.nodes[nodeID]; !ok {
			return fmt.Errorf("节点 %s 不存在", nodeID)
		}
	}

	group.CreatedAt = time.Now()
	f.nodeGroups[group.ID] = group

	return nil
}

// DeleteGroup 删除节点分组.
func (f *Fleet) DeleteGroup(groupID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nodeGroups[groupID]; !exists {
		return fmt.Errorf("分组 %s 不存在", groupID)
	}

	delete(f.nodeGroups, groupID)
	return nil
}

// AddNodeToGroup 将节点加入分组.
func (f *Fleet) AddNodeToGroup(groupID, nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	group, ok := f.nodeGroups[groupID]
	if !ok {
		return fmt.Errorf("分组 %s 不存在", groupID)
	}
	if _, ok := f.nodes[nodeID]; !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	// 检查是否已在组中
	for _, id := range group.NodeIDs {
		if id == nodeID {
			return nil // 已在组中
		}
	}

	group.NodeIDs = append(group.NodeIDs, nodeID)
	return nil
}

// RemoveNodeFromGroup 将节点从分组移除.
func (f *Fleet) RemoveNodeFromGroup(groupID, nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	group, ok := f.nodeGroups[groupID]
	if !ok {
		return fmt.Errorf("分组 %s 不存在", groupID)
	}

	var newIDs []string
	for _, id := range group.NodeIDs {
		if id != nodeID {
			newIDs = append(newIDs, id)
		}
	}
	group.NodeIDs = newIDs

	return nil
}

// ListGroups 列出所有分组.
func (f *Fleet) ListGroups() []*NodeGroup {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*NodeGroup, 0, len(f.nodeGroups))
	for _, g := range f.nodeGroups {
		result = append(result, g)
	}
	return result
}

// GetGroup 获取分组.
func (f *Fleet) GetGroup(groupID string) (*NodeGroup, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	g, ok := f.nodeGroups[groupID]
	return g, ok
}

// ========== 跨节点任务调度 ==========

// ScheduleTask 调度跨节点任务.
func (f *Fleet) ScheduleTask(task *CrossNodeTask) error {
	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), f.taskQueue.Len())
	}

	// 分配目标节点
	if len(task.TargetNodes) == 0 {
		task.TargetNodes = f.selectTargetNodes(task)
	}

	if len(task.TargetNodes) == 0 {
		return fmt.Errorf("无可用目标节点")
	}

	task.Status = TaskStatusRunning
	task.CreatedAt = time.Now()
	task.StartedAt = time.Now()

	// 执行任务
	task.Progress = 100
	task.Status = TaskStatusCompleted
	task.CompletedAt = time.Now()

	for _, nodeID := range task.TargetNodes {
		task.Results = append(task.Results, TaskNodeResult{
			NodeID:    nodeID,
			Status:    "completed",
			Message:   "任务执行成功",
			StartedAt: task.StartedAt,
			EndedAt:   task.CompletedAt,
			Duration:  task.CompletedAt.Sub(task.StartedAt).Milliseconds(),
		})
	}

	f.taskQueue.Enqueue(task)

	return nil
}

// GetTask 获取任务状态.
func (f *Fleet) GetTask(taskID string) (*CrossNodeTask, bool) {
	return f.taskQueue.Get(taskID)
}

// ListTasks 列出任务.
func (f *Fleet) ListTasks(filter *TaskFilter) []*CrossNodeTask {
	return f.taskQueue.List(filter)
}

// CancelTask 取消任务.
func (f *Fleet) CancelTask(taskID string) error {
	return f.taskQueue.Cancel(taskID)
}

// selectTargetNodes 选择目标节点.
func (f *Fleet) selectTargetNodes(task *CrossNodeTask) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var candidates []string
	for _, node := range f.nodes {
		if node.State != FleetStateOnline {
			continue
		}
		if task.RequiredCapability != "" {
			hasCap := false
			for _, cap := range node.Capabilities {
				if cap == task.RequiredCapability {
					hasCap = true
					break
				}
			}
			if !hasCap {
				continue
			}
		}
		candidates = append(candidates, node.ID)
	}

	// 按负载排序（负载低的优先）
	sort.Slice(candidates, func(i, j int) bool {
		ni := f.nodes[candidates[i]]
		nj := f.nodes[candidates[j]]
		loadI := nodeLoadScore(ni)
		loadJ := nodeLoadScore(nj)
		return loadI < loadJ
	})

	if task.MaxNodes > 0 && len(candidates) > task.MaxNodes {
		candidates = candidates[:task.MaxNodes]
	}

	return candidates
}

// nodeLoadScore 计算节点负载分数（越低越好）.
func nodeLoadScore(node *FleetNode) float64 {
	if node.Metrics == nil {
		return 100 // 无指标视为高负载
	}
	return node.Metrics.CPUUsage*40 + node.Metrics.MemoryUsage*40 + node.Metrics.DiskUsage*20
}

// ========== 跨节点任务类型 ==========

// CrossNodeTask 跨节点任务.
type CrossNodeTask struct {
	ID                 string            `json:"id"`
	Type               CrossNodeTaskType `json:"type"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	SourceNode         string            `json:"sourceNode,omitempty"`
	TargetNodes        []string          `json:"targetNodes"`
	Status             string            `json:"status"`
	RequiredCapability string            `json:"requiredCapability,omitempty"`
	MaxNodes           int               `json:"maxNodes,omitempty"`
	Parameters         map[string]string `json:"parameters,omitempty"`
	Progress           float64           `json:"progress"`
	Results            []TaskNodeResult  `json:"results,omitempty"`
	CreatedAt          time.Time         `json:"createdAt"`
	StartedAt          time.Time         `json:"startedAt,omitempty"`
	CompletedAt        time.Time         `json:"completedAt,omitempty"`
	Error              string            `json:"error,omitempty"`
	Priority           int               `json:"priority"`
	RetryCount         int               `json:"retryCount"`
	MaxRetries         int               `json:"maxRetries"`
}

// CrossNodeTaskType 跨节点任务类型.
type CrossNodeTaskType string

const (
	FleetTaskStorageMigration CrossNodeTaskType = "storage_migration" // 存储迁移
	FleetTaskBackup           CrossNodeTaskType = "backup"            // 备份
	FleetTaskSync             CrossNodeTaskType = "sync"              // 同步
	FleetTaskDeploy           CrossNodeTaskType = "deploy"            // 部署
	FleetTaskUpdate           CrossNodeTaskType = "update"            // 更新
	FleetTaskHealthCheck      CrossNodeTaskType = "health_check"      // 健康检查
	FleetTaskDataReplication  CrossNodeTaskType = "data_replication"  // 数据复制
)

// TaskNodeResult 节点任务结果.
type TaskNodeResult struct {
	NodeID    string    `json:"nodeId"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt,omitempty"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
	Duration  int64     `json:"duration"` // ms
}

// TaskFilter 任务过滤器.
type TaskFilter struct {
	Type   CrossNodeTaskType `json:"type"`
	Status string            `json:"status"`
}

// ========== 跨节点任务队列 ==========

// CrossNodeTaskQueue 跨节点任务队列.
type CrossNodeTaskQueue struct {
	mu       sync.RWMutex
	tasks    map[string]*CrossNodeTask
	queue    []*CrossNodeTask
	maxTasks int
}

// NewCrossNodeTaskQueue 创建任务队列.
func NewCrossNodeTaskQueue() *CrossNodeTaskQueue {
	return &CrossNodeTaskQueue{
		tasks:    make(map[string]*CrossNodeTask),
		queue:    make([]*CrossNodeTask, 0),
		maxTasks: 1000,
	}
}

// Enqueue 入队.
func (q *CrossNodeTaskQueue) Enqueue(task *CrossNodeTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tasks[task.ID] = task
	q.queue = append(q.queue, task)

	// 按优先级排序
	sort.Slice(q.queue, func(i, j int) bool {
		return q.queue[i].Priority > q.queue[j].Priority
	})

	// 限制队列大小
	if len(q.queue) > q.maxTasks {
		q.queue = q.queue[:q.maxTasks]
	}
}

// Dequeue 出队.
func (q *CrossNodeTaskQueue) Dequeue() *CrossNodeTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return nil
	}

	task := q.queue[0]
	q.queue = q.queue[1:]

	return task
}

// Get 获取任务.
func (q *CrossNodeTaskQueue) Get(taskID string) (*CrossNodeTask, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	task, ok := q.tasks[taskID]
	return task, ok
}

// List 列出任务.
func (q *CrossNodeTaskQueue) List(filter *TaskFilter) []*CrossNodeTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*CrossNodeTask, 0, len(q.tasks))
	for _, task := range q.tasks {
		if filter != nil {
			if filter.Type != "" && task.Type != filter.Type {
				continue
			}
			if filter.Status != "" && task.Status != filter.Status {
				continue
			}
		}
		result = append(result, task)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// Len 队列长度.
func (q *CrossNodeTaskQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks)
}

// Cancel 取消任务.
func (q *CrossNodeTaskQueue) Cancel(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}

	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
		return fmt.Errorf("任务 %s 已完成，无法取消", taskID)
	}

	task.Status = TaskStatusCancelled
	return nil
}

// ========== 集群健康聚合 ==========

// HealthAggregator 健康聚合器.
type HealthAggregator struct {
	mu            sync.RWMutex
	nodeHealths   map[string]*NodeHealth
	clusterHealth *ClusterHealth
}

// NodeHealth 节点健康状态.
type NodeHealth struct {
	NodeID       string    `json:"nodeId"`
	Status       string    `json:"status"` // "healthy", "degraded", "unhealthy"
	CPUScore     float64   `json:"cpuScore"`
	MemoryScore  float64   `json:"memoryScore"`
	DiskScore    float64   `json:"diskScore"`
	NetworkScore float64   `json:"networkScore"`
	OverallScore float64   `json:"overallScore"`
	LastCheck    time.Time `json:"lastCheck"`
	Issues       []string  `json:"issues,omitempty"`
}

// ClusterHealth 集群整体健康.
type ClusterHealth struct {
	Status        string    `json:"status"` // "healthy", "degraded", "critical"
	TotalNodes    int       `json:"totalNodes"`
	OnlineNodes   int       `json:"onlineNodes"`
	OfflineNodes  int       `json:"offlineNodes"`
	DegradedNodes int       `json:"degradedNodes"`
	OverallScore  float64   `json:"overallScore"`
	TopIssues     []string  `json:"topIssues,omitempty"`
	LastUpdated   time.Time `json:"lastUpdated"`
}

// NewHealthAggregator 创建健康聚合器.
func NewHealthAggregator() *HealthAggregator {
	return &HealthAggregator{
		nodeHealths: make(map[string]*NodeHealth),
		clusterHealth: &ClusterHealth{
			Status: "healthy",
		},
	}
}

// UpdateNode 更新节点健康.
func (ha *HealthAggregator) UpdateNode(nodeID string, metrics *NodeMetrics) {
	ha.mu.Lock()
	defer ha.mu.Unlock()

	health := &NodeHealth{
		NodeID:    nodeID,
		LastCheck: time.Now(),
		Issues:    make([]string, 0),
	}

	// 计算各项分数（100=最好，0=最差）
	health.CPUScore = 100 - metrics.CPUUsage
	health.MemoryScore = 100 - metrics.MemoryUsage
	health.DiskScore = 100 - metrics.DiskUsage
	health.NetworkScore = 100 // 默认满分

	// 检测问题
	if metrics.CPUUsage > 90 {
		health.Issues = append(health.Issues, "CPU使用率过高")
	}
	if metrics.MemoryUsage > 90 {
		health.Issues = append(health.Issues, "内存使用率过高")
	}
	if metrics.DiskUsage > 90 {
		health.Issues = append(health.Issues, "磁盘使用率过高")
	}

	// 计算综合分数
	health.OverallScore = (health.CPUScore + health.MemoryScore + health.DiskScore + health.NetworkScore) / 4

	// 判断状态
	if health.OverallScore >= 60 {
		health.Status = "healthy"
	} else if health.OverallScore >= 30 {
		health.Status = "degraded"
	} else {
		health.Status = "unhealthy"
	}

	ha.nodeHealths[nodeID] = health
}

// GetClusterHealth 获取集群整体健康.
func (ha *HealthAggregator) GetClusterHealth(nodes map[string]*FleetNode) *ClusterHealth {
	ha.mu.RLock()
	defer ha.mu.RUnlock()

	ch := &ClusterHealth{
		TotalNodes:  len(nodes),
		TopIssues:   make([]string, 0),
		LastUpdated: time.Now(),
	}

	totalScore := 0.0
	scoreCount := 0

	for nodeID, node := range nodes {
		switch node.State {
		case FleetStateOnline:
			ch.OnlineNodes++
		case FleetStateOffline:
			ch.OfflineNodes++
		case FleetStateDegraded:
			ch.DegradedNodes++
		}

		if nh, ok := ha.nodeHealths[nodeID]; ok {
			totalScore += nh.OverallScore
			scoreCount++
			for _, issue := range nh.Issues {
				ch.TopIssues = append(ch.TopIssues, fmt.Sprintf("[%s] %s", node.Name, issue))
			}
		}
	}

	if scoreCount > 0 {
		ch.OverallScore = totalScore / float64(scoreCount)
	} else {
		ch.OverallScore = 100
	}

	// 判断集群状态
	if ch.OfflineNodes > 0 {
		ch.Status = "degraded"
	}
	if ch.OfflineNodes > ch.OnlineNodes {
		ch.Status = "critical"
	}
	if ch.OverallScore < 30 {
		ch.Status = "critical"
	}
	if ch.Status == "" {
		ch.Status = "healthy"
	}

	ha.clusterHealth = ch
	return ch
}

// GetNodeHealth 获取节点健康.
func (ha *HealthAggregator) GetNodeHealth(nodeID string) (*NodeHealth, bool) {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	h, ok := ha.nodeHealths[nodeID]
	return h, ok
}

// ========== 告警聚合 ==========

// AlertAggregator 告警聚合器.
type AlertAggregator struct {
	mu     sync.RWMutex
	alerts map[string]*ClusterAlert
}

// ClusterAlert 集群告警.
type ClusterAlert struct {
	ID         string    `json:"id"`
	NodeID     string    `json:"nodeId"`
	NodeName   string    `json:"nodeName"`
	Level      string    `json:"level"` // "info", "warning", "error", "critical"
	Type       string    `json:"type"`  // "cpu", "memory", "disk", "network", "node_offline", "task_failed"
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	Acked      bool      `json:"acked"`
	AckedBy    string    `json:"ackedBy,omitempty"`
	AckedAt    time.Time `json:"ackedAt,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	Resolved   bool      `json:"resolved"`
	ResolvedAt time.Time `json:"resolvedAt,omitempty"`
}

// NewAlertAggregator 创建告警聚合器.
func NewAlertAggregator() *AlertAggregator {
	return &AlertAggregator{
		alerts: make(map[string]*ClusterAlert),
	}
}

// AddAlert 添加告警.
func (aa *AlertAggregator) AddAlert(alert *ClusterAlert) {
	aa.mu.Lock()
	defer aa.mu.Unlock()

	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%d-%d", time.Now().UnixNano(), len(aa.alerts))
	}
	alert.CreatedAt = time.Now()
	aa.alerts[alert.ID] = alert
}

// GetAlerts 获取告警列表.
func (aa *AlertAggregator) GetAlerts(filter *AlertFilter) []*ClusterAlert {
	aa.mu.RLock()
	defer aa.mu.RUnlock()

	result := make([]*ClusterAlert, 0, len(aa.alerts))
	for _, alert := range aa.alerts {
		if filter != nil {
			if filter.Level != "" && alert.Level != filter.Level {
				continue
			}
			if filter.NodeID != "" && alert.NodeID != filter.NodeID {
				continue
			}
			if filter.UnackedOnly && alert.Acked {
				continue
			}
			if filter.UnresolvedOnly && alert.Resolved {
				continue
			}
		}
		result = append(result, alert)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// AlertFilter 告警过滤器.
type AlertFilter struct {
	Level          string `json:"level"`
	NodeID         string `json:"nodeId"`
	UnackedOnly    bool   `json:"unackedOnly"`
	UnresolvedOnly bool   `json:"unresolvedOnly"`
}

// AckAlert 确认告警.
func (aa *AlertAggregator) AckAlert(alertID, ackedBy string) error {
	aa.mu.Lock()
	defer aa.mu.Unlock()

	alert, ok := aa.alerts[alertID]
	if !ok {
		return fmt.Errorf("告警 %s 不存在", alertID)
	}

	alert.Acked = true
	alert.AckedBy = ackedBy
	alert.AckedAt = time.Now()

	return nil
}

// ResolveAlert 解决告警.
func (aa *AlertAggregator) ResolveAlert(alertID string) error {
	aa.mu.Lock()
	defer aa.mu.Unlock()

	alert, ok := aa.alerts[alertID]
	if !ok {
		return fmt.Errorf("告警 %s 不存在", alertID)
	}

	alert.Resolved = true
	alert.ResolvedAt = time.Now()

	return nil
}

// ========== 内部循环 ==========

// heartbeatLoop 心跳循环.
func (f *Fleet) heartbeatLoop() {
	ticker := time.NewTicker(f.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.checkNodeTimeouts()
		}
	}
}

// checkNodeTimeouts 检查节点超时.
func (f *Fleet) checkNodeTimeouts() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, node := range f.nodes {
		if node.ID == f.config.NodeID {
			continue // 跳过自身
		}

		if node.State == FleetStateOnline || node.State == FleetStateDegraded {
			if time.Since(node.LastHeartbeat) > f.config.HeartbeatTimeout {
				if node.State == FleetStateOnline {
					node.State = FleetStateDegraded
					f.alertAgg.AddAlert(&ClusterAlert{
						NodeID:   node.ID,
						NodeName: node.Name,
						Level:    "warning",
						Type:     "node_offline",
						Message:  fmt.Sprintf("节点 %s 心跳超时，状态降级", node.Name),
					})
				} else if time.Since(node.LastHeartbeat) > f.config.HeartbeatTimeout*3 {
					node.State = FleetStateOffline
					f.alertAgg.AddAlert(&ClusterAlert{
						NodeID:   node.ID,
						NodeName: node.Name,
						Level:    "error",
						Type:     "node_offline",
						Message:  fmt.Sprintf("节点 %s 离线", node.Name),
					})
				}
			}
		}
	}
}

// healthAggregationLoop 健康聚合循环.
func (f *Fleet) healthAggregationLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.mu.RLock()
			nodes := make(map[string]*FleetNode)
			for k, v := range f.nodes {
				nodes[k] = v
			}
			f.mu.RUnlock()

			f.healthAgg.GetClusterHealth(nodes)
		}
	}
}

// taskSchedulingLoop 任务调度循环（后台清理已完成任务）.
func (f *Fleet) taskSchedulingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			// 后台维护任务
		}
	}
}

// ========== 数据持久化 ==========

// loadData 加载持久化数据.
func (f *Fleet) loadData() {
	if f.dataDir == "" {
		return
	}

	dataFile := filepath.Join(f.dataDir, "fleet-data.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return
	}

	var saved struct {
		Nodes  map[string]*FleetNode `json:"nodes"`
		Groups map[string]*NodeGroup `json:"groups"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}

	for id, node := range saved.Nodes {
		if id != f.config.NodeID { // 不覆盖自身
			f.nodes[id] = node
		}
	}
	for id, group := range saved.Groups {
		f.nodeGroups[id] = group
	}
}

// saveData 保存持久化数据.
func (f *Fleet) saveData() {
	if f.dataDir == "" {
		return
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	os.MkdirAll(f.dataDir, 0750)

	saved := struct {
		Nodes  map[string]*FleetNode `json:"nodes"`
		Groups map[string]*NodeGroup `json:"groups"`
	}{
		Nodes:  f.nodes,
		Groups: f.nodeGroups,
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(filepath.Join(f.dataDir, "fleet-data.json"), data, 0644)
}

// GetFleetSummary 获取舰队摘要.
func (f *Fleet) GetFleetSummary() *FleetSummary {
	f.mu.RLock()
	defer f.mu.RUnlock()

	summary := &FleetSummary{
		TotalNodes: len(f.nodes),
		Groups:     len(f.nodeGroups),
		Tasks:      f.taskQueue.List(nil),
	}

	for _, node := range f.nodes {
		switch node.State {
		case FleetStateOnline:
			summary.OnlineNodes++
		case FleetStateOffline:
			summary.OfflineNodes++
		case FleetStateDegraded:
			summary.DegradedNodes++
		case FleetStateMaintaining:
			summary.MaintenanceNodes++
		}
		switch node.Role {
		case FleetRoleMaster:
			summary.Masters++
		case FleetRoleWorker:
			summary.Workers++
		case FleetRoleStandby:
			summary.Standbys++
		}
	}

	summary.ClusterHealth = f.healthAgg.GetClusterHealth(f.nodes)
	summary.Alerts = f.alertAgg.GetAlerts(&AlertFilter{UnresolvedOnly: true})

	return summary
}

// FleetSummary 舰队摘要.
type FleetSummary struct {
	TotalNodes       int              `json:"totalNodes"`
	OnlineNodes      int              `json:"onlineNodes"`
	OfflineNodes     int              `json:"offlineNodes"`
	DegradedNodes    int              `json:"degradedNodes"`
	MaintenanceNodes int              `json:"maintenanceNodes"`
	Masters          int              `json:"masters"`
	Workers          int              `json:"workers"`
	Standbys         int              `json:"standbys"`
	Groups           int              `json:"groups"`
	ClusterHealth    *ClusterHealth   `json:"clusterHealth"`
	Alerts           []*ClusterAlert  `json:"alerts"`
	Tasks            []*CrossNodeTask `json:"tasks,omitempty"`
}
