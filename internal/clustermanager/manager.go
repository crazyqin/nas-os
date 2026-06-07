// Package clustermanager 提供集群多节点管理功能
package clustermanager

import (
	"fmt"
	"sync"
	"time"
)

// ClusterManager 集群管理器
type ClusterManager struct {
	mu       sync.RWMutex
	nodes    map[string]*ClusterNode
	groups   map[string]*NodeGroup
	alerts   map[string]*ClusterAlert
	tasks    map[string]*ClusterTask
	topology *ClusterTopology
	stats    *ClusterStats
	config   *ClusterConfig
}

// NewClusterManager 创建集群管理器
func NewClusterManager(config *ClusterConfig) *ClusterManager {
	if config == nil {
		cfg := DefaultClusterConfig()
		config = &cfg
	}
	return &ClusterManager{
		nodes:    make(map[string]*ClusterNode),
		groups:   make(map[string]*NodeGroup),
		alerts:   make(map[string]*ClusterAlert),
		tasks:    make(map[string]*ClusterTask),
		topology: &ClusterTopology{},
		stats:    &ClusterStats{StartTime: time.Now()},
		config:   config,
	}
}

// RegisterNode 注册节点
func (m *ClusterManager) RegisterNode(req *AddNodeRequest) (*ClusterNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("节点名称不能为空")
	}
	if req.IPAddress == "" {
		return nil, fmt.Errorf("节点地址不能为空")
	}

	node := &ClusterNode{
		ID:           fmt.Sprintf("node_%d", time.Now().UnixNano()),
		Name:         req.Name,
		Hostname:     req.Hostname,
		IPAddress:    req.IPAddress,
		Port:         req.Port,
		Type:         req.Type,
		Status:       NodeStatusOnline,
		GroupID:      req.GroupID,
		Tags:         req.Tags,
		Location:     req.Location,
		Metadata:     req.Metadata,
		RegisteredAt: time.Now(),
		LastSeenAt:   time.Now(),
		UpdatedAt:    time.Now(),
		Health:       &NodeHealth{},
	}

	if node.Port == 0 {
		node.Port = 8080
	}

	m.nodes[node.ID] = node
	return node, nil
}

// UnregisterNode 注销节点
func (m *ClusterManager) UnregisterNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[id]; !exists {
		return fmt.Errorf("节点不存在: %s", id)
	}

	delete(m.nodes, id)
	return nil
}

// GetNode 获取节点
func (m *ClusterManager) GetNode(id string) (*ClusterNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[id]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", id)
	}
	return node, nil
}

// ListNodes 列出所有节点
func (m *ClusterManager) ListNodes() []*ClusterNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*ClusterNode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// UpdateNodeStatus 更新节点状态
func (m *ClusterManager) UpdateNodeStatus(id string, status NodeStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[id]
	if !exists {
		return fmt.Errorf("节点不存在: %s", id)
	}

	node.UpdateStatus(status)
	return nil
}

// Heartbeat 心跳
func (m *ClusterManager) Heartbeat(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[id]
	if !exists {
		return fmt.Errorf("节点不存在: %s", id)
	}

	node.LastSeenAt = time.Now()
	if node.Status == NodeStatusOffline {
		node.Status = NodeStatusOnline
	}
	return nil
}

// GetOnlineNodes 获取在线节点
func (m *ClusterManager) GetOnlineNodes() []*ClusterNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var nodes []*ClusterNode
	for _, node := range m.nodes {
		if node.IsOnline() {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// CreateGroup 创建分组
func (m *ClusterManager) CreateGroup(req *CreateGroupRequest) (*NodeGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("分组名称不能为空")
	}

	group := &NodeGroup{
		ID:          fmt.Sprintf("group_%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Tags:        req.Tags,
		Priority:    req.Priority,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.groups[group.ID] = group
	return group, nil
}

// GetGroup 获取分组
func (m *ClusterManager) GetGroup(id string) (*NodeGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("分组不存在: %s", id)
	}
	return group, nil
}

// ListGroups 列出所有分组
func (m *ClusterManager) ListGroups() []*NodeGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*NodeGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// CreateTask 创建集群任务
func (m *ClusterManager) CreateTask(req *CreateTaskRequest) (*ClusterTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("任务名称不能为空")
	}

	task := &ClusterTask{
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Name:      req.Name,
		Type:      req.Type,
		Status:    TaskStatusPending,
		Priority:  req.Priority,
		NodeIDs:   req.TargetNodeIDs,
		Payload:   req.Payload,
		CreatedAt: time.Now(),
		Timeout:   time.Duration(req.Timeout) * time.Second,
	}

	m.tasks[task.ID] = task
	return task, nil
}

// GetTask 获取任务
func (m *ClusterManager) GetTask(id string) (*ClusterTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}
	return task, nil
}

// ListTasks 列出所有任务
func (m *ClusterManager) ListTasks() []*ClusterTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ClusterTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// GetClusterStats 获取集群统计
func (m *ClusterManager) GetClusterStats() *ClusterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*ClusterNode, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, n)
	}
	m.stats.Update(nodes)
	return m.stats.GetSnapshot()
}

// GetTopology 获取集群拓扑
func (m *ClusterManager) GetTopology() *ClusterTopology {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topo := &ClusterTopology{UpdatedAt: time.Now()}
	for _, node := range m.nodes {
		topo.Nodes = append(topo.Nodes, &TopologyNode{
			ID:      node.ID,
			Name:    node.Name,
			Type:    node.Type,
			Status:  node.Status,
			GroupID: node.GroupID,
		})
	}
	return topo
}

// CreateAlert 创建告警
func (m *ClusterManager) CreateAlert(alertType AlertType, level AlertLevel, nodeID, title, message string) *ClusterAlert {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert := &ClusterAlert{
		ID:          fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		Type:        alertType,
		Level:       level,
		NodeID:      nodeID,
		Title:       title,
		Message:     message,
		Active:      true,
		TriggeredAt: time.Now(),
	}

	m.alerts[alert.ID] = alert
	return alert
}

// ListAlerts 列出告警
func (m *ClusterManager) ListAlerts(activeOnly bool) []*ClusterAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*ClusterAlert, 0)
	for _, a := range m.alerts {
		if !activeOnly || a.Active {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// CreateCluster 创建集群
func (m *ClusterManager) CreateCluster(name string) (*ClusterNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("集群名称不能为空")
	}

	// 创建一个虚拟的主节点作为集群入口
	node := &ClusterNode{
		ID:           fmt.Sprintf("cluster_%d", time.Now().UnixNano()),
		Name:         name,
		Type:         NodeTypeHybrid,
		Status:       NodeStatusOnline,
		IPAddress:    "localhost",
		Tags:         []string{"master"},
		Metadata:     map[string]string{"cluster": name, "role": "master"},
		LastSeenAt:   time.Now(),
		RegisteredAt: time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.nodes[node.ID] = node
	return node, nil
}

// ListClusters 列出所有集群（返回带 cluster 标签的节点作为集群列表）
func (m *ClusterManager) ListClusters() []*ClusterNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var clusters []*ClusterNode
	for _, node := range m.nodes {
		if role, ok := node.Metadata["role"]; ok && role == "master" {
			clusters = append(clusters, node)
		}
	}
	return clusters
}
