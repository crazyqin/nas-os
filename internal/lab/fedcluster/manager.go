// Package fedcluster 提供联邦集群管理功能.
// 支持多台NAS设备统一管理、跨节点数据同步、负载均衡等.
package fedcluster

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NodeStatus 节点状态.
type NodeStatus string

const (
	NodeOnline      NodeStatus = "online"
	NodeOffline     NodeStatus = "offline"
	NodeSyncing     NodeStatus = "syncing"
	NodeMaintenance NodeStatus = "maintenance"
	NodeDegraded    NodeStatus = "degraded"
)

// ClusterRole 集群角色.
type ClusterRole string

const (
	RoleMaster  ClusterRole = "master"
	RoleWorker  ClusterRole = "worker"
	RoleWitness ClusterRole = "witness"
)

// SyncPolicy 同步策略.
type SyncPolicy string

const (
	SyncAll       SyncPolicy = "all"
	SyncSelective SyncPolicy = "selective"
	SyncOnDemand  SyncPolicy = "on_demand"
	SyncDisabled  SyncPolicy = "disabled"
)

// ClusterNode 集群节点.
type ClusterNode struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Hostname      string            `json:"hostname"`
	Port          int               `json:"port"`
	Role          ClusterRole       `json:"role"`
	Status        NodeStatus        `json:"status"`
	IPAddress     string            `json:"ip_address"`
	APIEndpoint   string            `json:"api_endpoint"`
	CPUCores      int               `json:"cpu_cores"`
	MemoryGB      int               `json:"memory_gb"`
	StorageTB     float64           `json:"storage_tb"`
	UsedStorageTB float64           `json:"used_storage_tb"`
	LastSeen      time.Time         `json:"last_seen"`
	JoinedAt      time.Time         `json:"joined_at"`
	Tags          []string          `json:"tags"`
	Metadata      map[string]string `json:"metadata"`
}

// Cluster 集群配置.
type Cluster struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	SyncPolicy    SyncPolicy              `json:"sync_policy"`
	AutoHeal      bool                    `json:"auto_heal"`
	LoadBalance   bool                    `json:"load_balance"`
	EncryptionKey string                  `json:"encryption_key,omitempty"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	Nodes         map[string]*ClusterNode `json:"nodes"`
	VirtualIP     string                  `json:"virtual_ip,omitempty"`
	Domain        string                  `json:"domain,omitempty"`
}

// SyncJob 同步任务.
type SyncJob struct {
	ID          string     `json:"id"`
	SourceNode  string     `json:"source_node"`
	TargetNode  string     `json:"target_node"`
	SourcePath  string     `json:"source_path"`
	TargetPath  string     `json:"target_path"`
	Status      string     `json:"status"`
	Progress    float64    `json:"progress"`
	BytesTotal  int64      `json:"bytes_total"`
	BytesSynced int64      `json:"bytes_synced"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// LoadBalancerConfig 负载均衡配置.
type LoadBalancerConfig struct {
	Strategy      string         `json:"strategy"` // round_robin, least_connections, weighted, latency_based
	HealthCheck   bool           `json:"health_check"`
	CheckInterval int            `json:"check_interval_seconds"`
	FailoverDelay int            `json:"failover_delay_seconds"`
	MaxRetries    int            `json:"max_retries"`
	StickySession bool           `json:"sticky_session"`
	Weights       map[string]int `json:"weights,omitempty"`
}

// Manager 集群管理器.
type Manager struct {
	mu          sync.RWMutex
	clusters    map[string]*Cluster
	syncJobs    map[string]*SyncJob
	lbConfig    *LoadBalancerConfig
	eventLog    []*ClusterEvent
	nodeMetrics map[string]*NodeMetrics
}

// ClusterEvent 集群事件.
type ClusterEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	NodeID    string    `json:"node_id,omitempty"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

// NodeMetrics 节点指标.
type NodeMetrics struct {
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	StorageUsage float64   `json:"storage_usage"`
	NetworkIn    int64     `json:"network_in"`
	NetworkOut   int64     `json:"network_out"`
	IOPS         int       `json:"iops"`
	Latency      float64   `json:"latency_ms"`
	Timestamp    time.Time `json:"timestamp"`
}

// NewManager 创建新的集群管理器.
func NewManager() *Manager {
	return &Manager{
		clusters:    make(map[string]*Cluster),
		syncJobs:    make(map[string]*SyncJob),
		eventLog:    make([]*ClusterEvent, 0),
		nodeMetrics: make(map[string]*NodeMetrics),
		lbConfig: &LoadBalancerConfig{
			Strategy:      "round_robin",
			HealthCheck:   true,
			CheckInterval: 30,
			FailoverDelay: 10,
			MaxRetries:    3,
		},
	}
}

// CreateCluster 创建集群.
func (m *Manager) CreateCluster(name, description string, syncPolicy SyncPolicy) (*Cluster, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在同名集群
	for _, c := range m.clusters {
		if c.Name == name {
			return nil, fmt.Errorf("集群 %s 已存在", name)
		}
	}

	cluster := &Cluster{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		SyncPolicy:  syncPolicy,
		AutoHeal:    true,
		LoadBalance: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Nodes:       make(map[string]*ClusterNode),
	}

	m.clusters[cluster.ID] = cluster
	m.addEvent("cluster_created", "", fmt.Sprintf("集群 %s 已创建", name), "info")

	return cluster, nil
}

// JoinNode 节点加入集群.
func (m *Manager) JoinNode(clusterID string, node *ClusterNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("集群 %s 不存在", clusterID)
	}

	// 检查节点是否已在集群中
	for _, n := range cluster.Nodes {
		if n.Hostname == node.Hostname && n.Port == node.Port {
			return fmt.Errorf("节点 %s:%d 已在集群中", node.Hostname, node.Port)
		}
	}

	node.ID = uuid.New().String()
	node.Status = NodeOnline
	node.JoinedAt = time.Now()
	node.LastSeen = time.Now()

	cluster.Nodes[node.ID] = node
	cluster.UpdatedAt = time.Now()

	m.addEvent("node_joined", node.ID, fmt.Sprintf("节点 %s 加入集群", node.Name), "info")

	return nil
}

// RemoveNode 移除节点.
func (m *Manager) RemoveNode(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("集群 %s 不存在", clusterID)
	}

	node, ok := cluster.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不在集群中", nodeID)
	}

	// 如果是master节点，需要先转移角色
	if node.Role == RoleMaster {
		return fmt.Errorf("不能直接移除master节点，请先转移角色")
	}

	delete(cluster.Nodes, nodeID)
	cluster.UpdatedAt = time.Now()

	m.addEvent("node_removed", nodeID, fmt.Sprintf("节点 %s 已从集群移除", node.Name), "warning")

	return nil
}

// PromoteNode 提升节点为master.
func (m *Manager) PromoteNode(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("集群 %s 不存在", clusterID)
	}

	targetNode, ok := cluster.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不在集群中", nodeID)
	}

	// 将当前master降级
	for _, node := range cluster.Nodes {
		if node.Role == RoleMaster {
			node.Role = RoleWorker
		}
	}

	// 提升目标节点
	targetNode.Role = RoleMaster
	cluster.UpdatedAt = time.Now()

	m.addEvent("master_promoted", nodeID, fmt.Sprintf("节点 %s 已提升为master", targetNode.Name), "info")

	return nil
}

// StartSync 启动数据同步.
func (m *Manager) StartSync(clusterID, sourceNode, targetNode, sourcePath, targetPath string) (*SyncJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("集群 %s 不存在", clusterID)
	}

	if _, ok := cluster.Nodes[sourceNode]; !ok {
		return nil, fmt.Errorf("源节点 %s 不在集群中", sourceNode)
	}

	if _, ok := cluster.Nodes[targetNode]; !ok {
		return nil, fmt.Errorf("目标节点 %s 不在集群中", targetNode)
	}

	job := &SyncJob{
		ID:         uuid.New().String(),
		SourceNode: sourceNode,
		TargetNode: targetNode,
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Status:     "pending",
		StartedAt:  time.Now(),
	}

	m.syncJobs[job.ID] = job
	m.addEvent("sync_started", sourceNode, fmt.Sprintf("同步任务 %s 已启动", job.ID), "info")

	return job, nil
}

// UpdateSyncProgress 更新同步进度.
func (m *Manager) UpdateSyncProgress(jobID string, progress float64, bytesSynced int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.syncJobs[jobID]
	if !ok {
		return fmt.Errorf("同步任务 %s 不存在", jobID)
	}

	job.Progress = progress
	job.BytesSynced = bytesSynced
	job.Status = "syncing"

	if progress >= 100 {
		job.Status = "completed"
		now := time.Now()
		job.CompletedAt = &now
		m.addEvent("sync_completed", job.SourceNode, fmt.Sprintf("同步任务 %s 已完成", jobID), "info")
	}

	return nil
}

// GetCluster 获取集群信息.
func (m *Manager) GetCluster(clusterID string) (*Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("集群 %s 不存在", clusterID)
	}

	return cluster, nil
}

// ListClusters 列出所有集群.
func (m *Manager) ListClusters() []*Cluster {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]*Cluster, 0, len(m.clusters))
	for _, c := range m.clusters {
		clusters = append(clusters, c)
	}
	return clusters
}

// GetSyncJob 获取同步任务.
func (m *Manager) GetSyncJob(jobID string) (*SyncJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.syncJobs[jobID]
	if !ok {
		return nil, fmt.Errorf("同步任务 %s 不存在", jobID)
	}

	return job, nil
}

// ListSyncJobs 列出所有同步任务.
func (m *Manager) ListSyncJobs(clusterID string) []*SyncJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*SyncJob, 0)
	for _, j := range m.syncJobs {
		// 如果指定了集群，只返回该集群的任务
		if clusterID != "" {
			// 这里简化处理，实际应该检查节点是否属于集群
			jobs = append(jobs, j)
		} else {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// UpdateNodeMetrics 更新节点指标.
func (m *Manager) UpdateNodeMetrics(nodeID string, metrics *NodeMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics.Timestamp = time.Now()
	m.nodeMetrics[nodeID] = metrics
}

// GetNodeMetrics 获取节点指标.
func (m *Manager) GetNodeMetrics(nodeID string) *NodeMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.nodeMetrics[nodeID]
}

// SelectNodeForRequest 根据负载均衡策略选择节点.
func (m *Manager) SelectNodeForRequest(clusterID string) (*ClusterNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("集群 %s 不存在", clusterID)
	}

	// 收集在线节点
	onlineNodes := make([]*ClusterNode, 0)
	for _, node := range cluster.Nodes {
		if node.Status == NodeOnline {
			onlineNodes = append(onlineNodes, node)
		}
	}

	if len(onlineNodes) == 0 {
		return nil, fmt.Errorf("没有可用的在线节点")
	}

	// 根据策略选择节点
	switch m.lbConfig.Strategy {
	case "least_connections":
		return m.selectLeastConnections(onlineNodes), nil
	case "weighted":
		return m.selectWeighted(onlineNodes), nil
	case "latency_based":
		return m.selectLatencyBased(onlineNodes), nil
	default: // round_robin
		return m.selectRoundRobin(onlineNodes), nil
	}
}

// selectRoundRobin 轮询选择.
func (m *Manager) selectRoundRobin(nodes []*ClusterNode) *ClusterNode {
	// 简化实现，实际应该维护一个计数器
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

// selectLeastConnections 最少连接选择.
func (m *Manager) selectLeastConnections(nodes []*ClusterNode) *ClusterNode {
	if len(nodes) == 0 {
		return nil
	}

	// 基于存储使用率选择（模拟最少连接）
	bestNode := nodes[0]
	bestUsage := bestNode.UsedStorageTB / bestNode.StorageTB

	for _, node := range nodes[1:] {
		usage := node.UsedStorageTB / node.StorageTB
		if usage < bestUsage {
			bestNode = node
			bestUsage = usage
		}
	}

	return bestNode
}

// selectWeighted 加权选择.
func (m *Manager) selectWeighted(nodes []*ClusterNode) *ClusterNode {
	if len(nodes) == 0 {
		return nil
	}

	// 使用权重配置
	bestNode := nodes[0]
	bestWeight := 0

	for _, node := range nodes {
		weight, ok := m.lbConfig.Weights[node.ID]
		if ok && weight > bestWeight {
			bestNode = node
			bestWeight = weight
		}
	}

	// 如果没有配置权重，使用默认
	if bestWeight == 0 {
		return nodes[0]
	}

	return bestNode
}

// selectLatencyBased 基于延迟选择.
func (m *Manager) selectLatencyBased(nodes []*ClusterNode) *ClusterNode {
	if len(nodes) == 0 {
		return nil
	}

	bestNode := nodes[0]
	bestLatency := float64(1<<63 - 1) // Max float64

	for _, node := range nodes {
		metrics := m.nodeMetrics[node.ID]
		if metrics != nil && metrics.Latency < bestLatency {
			bestNode = node
			bestLatency = metrics.Latency
		}
	}

	return bestNode
}

// GetClusterStats 获取集群统计.
func (m *Manager) GetClusterStats(clusterID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("集群 %s 不存在", clusterID)
	}

	totalStorage := 0.0
	usedStorage := 0.0
	onlineNodes := 0

	for _, node := range cluster.Nodes {
		totalStorage += node.StorageTB
		usedStorage += node.UsedStorageTB
		if node.Status == NodeOnline {
			onlineNodes++
		}
	}

	stats := map[string]interface{}{
		"cluster_id":    cluster.ID,
		"cluster_name":  cluster.Name,
		"total_nodes":   len(cluster.Nodes),
		"online_nodes":  onlineNodes,
		"total_storage": totalStorage,
		"used_storage":  usedStorage,
		"free_storage":  totalStorage - usedStorage,
		"usage_percent": (usedStorage / totalStorage) * 100,
		"sync_jobs":     len(m.syncJobs),
	}

	return stats, nil
}

// GetEventLog 获取事件日志.
func (m *Manager) GetEventLog(limit int) []*ClusterEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.eventLog) {
		limit = len(m.eventLog)
	}

	// 返回最新的事件
	start := len(m.eventLog) - limit
	if start < 0 {
		start = 0
	}

	return m.eventLog[start:]
}

// addEvent 添加事件.
func (m *Manager) addEvent(eventType, nodeID, message, severity string) {
	event := &ClusterEvent{
		ID:        uuid.New().String(),
		Type:      eventType,
		NodeID:    nodeID,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	}
	m.eventLog = append(m.eventLog, event)

	// 限制日志数量
	if len(m.eventLog) > 10000 {
		m.eventLog = m.eventLog[1000:]
	}
}

// HealthCheck 节点健康检查.
func (m *Manager) HealthCheck(clusterID string) map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return nil
	}

	results := make(map[string]bool)
	for id, node := range cluster.Nodes {
		// 检查节点最后心跳时间
		if time.Since(node.LastSeen) > 5*time.Minute {
			results[id] = false
			node.Status = NodeOffline
		} else {
			results[id] = true
		}
	}

	return results
}

// UpdateNodeHeartbeat 更新节点心跳.
func (m *Manager) UpdateNodeHeartbeat(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("集群 %s 不存在", clusterID)
	}

	node, ok := cluster.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不在集群中", nodeID)
	}

	node.LastSeen = time.Now()
	if node.Status == NodeOffline {
		node.Status = NodeOnline
		m.addEvent("node_online", nodeID, fmt.Sprintf("节点 %s 已恢复在线", node.Name), "info")
	}

	return nil
}

// SetMaintenanceMode 设置维护模式.
func (m *Manager) SetMaintenanceMode(clusterID, nodeID string, enable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("集群 %s 不存在", clusterID)
	}

	node, ok := cluster.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不在集群中", nodeID)
	}

	if enable {
		node.Status = NodeMaintenance
		m.addEvent("node_maintenance", nodeID, fmt.Sprintf("节点 %s 进入维护模式", node.Name), "warning")
	} else {
		node.Status = NodeOnline
		m.addEvent("node_online", nodeID, fmt.Sprintf("节点 %s 退出维护模式", node.Name), "info")
	}

	return nil
}
