// Package clustermgr2 提供增强版集群管理器，对标群晖 Cluster Manager。
// 支持多站点联合、边缘计算、负载均衡、工作负载迁移、集中化保护。
package clustermgr2

import (
	"fmt"
	"sync"
	"time"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusDrain   NodeStatus = "drain"
	NodeStatusError   NodeStatus = "error"
)

// WorkloadType 工作负载类型
type WorkloadType string

const (
	WorkloadStorage  WorkloadType = "storage"
	WorkloadCompute  WorkloadType = "compute"
	WorkloadNetwork  WorkloadType = "network"
	WorkloadBackup   WorkloadType = "backup"
)

// ClusterNode 集群节点
type ClusterNode struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Address    string       `json:"address"`
	Status     NodeStatus   `json:"status"`
	Region     string       `json:"region"`
	CPUCores   int          `json:"cpu_cores"`
	MemoryGB   int          `json:"memory_gb"`
	StorageGB  int          `json:"storage_gb"`
	UsedCPU    float64      `json:"used_cpu"`
	UsedMemory float64      `json:"used_memory"`
	UsedStorage float64     `json:"used_storage"`
	Labels     map[string]string `json:"labels"`
	LastSeen   time.Time    `json:"last_seen"`
}

// Workload 工作负载
type Workload struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        WorkloadType `json:"type"`
	NodeID      string       `json:"node_id"`
	Status      string       `json:"status"`
	CPU         float64      `json:"cpu"`
	MemoryGB    float64      `json:"memory_gb"`
	StorageGB   float64      `json:"storage_gb"`
	Ports       []int        `json:"ports,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ClusterPolicy 集群策略
type ClusterPolicy struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Rule        string `json:"rule"`
	Action      string `json:"action"`
}

// ClusterConfig 集群配置
type ClusterConfig struct {
	ClusterName    string `json:"cluster_name"`
	Region         string `json:"region"`
	MaxNodes       int    `json:"max_nodes"`
	EnableLB       bool   `json:"enable_lb"`
	EnableEdge     bool   `json:"enable_edge"`
	EnableFederation bool `json:"enable_federation"`
	HealthInterval time.Duration `json:"health_interval"`
}

// DefaultClusterConfig 返回默认配置
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		ClusterName:    "nas-os-cluster",
		Region:         "default",
		MaxNodes:       100,
		EnableLB:       true,
		EnableEdge:     true,
		EnableFederation: false,
		HealthInterval: 30 * time.Second,
	}
}

// Manager 集群管理器
type Manager struct {
	mu        sync.RWMutex
	config    *ClusterConfig
	nodes     map[string]*ClusterNode
	workloads map[string]*Workload
	policies  map[string]*ClusterPolicy
	running   bool
	startTime time.Time
}

// NewManager 创建集群管理器
func NewManager(config *ClusterConfig) *Manager {
	if config == nil {
		config = DefaultClusterConfig()
	}
	return &Manager{
		config:    config,
		nodes:     make(map[string]*ClusterNode),
		workloads: make(map[string]*Workload),
		policies:  make(map[string]*ClusterPolicy),
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("ClusterManager2 已在运行")
	}
	m.running = true
	m.startTime = time.Now()
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// AddNode 添加节点
func (m *Manager) AddNode(node *ClusterNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return fmt.Errorf("管理器未运行")
	}
	if len(m.nodes) >= m.config.MaxNodes {
		return fmt.Errorf("已达到最大节点数: %d", m.config.MaxNodes)
	}
	node.LastSeen = time.Now()
	m.nodes[node.ID] = node
	return nil
}

// RemoveNode 移除节点
func (m *Manager) RemoveNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.nodes[id]; !exists {
		return fmt.Errorf("节点不存在: %s", id)
	}
	// 检查节点上是否有工作负载
	for _, w := range m.workloads {
		if w.NodeID == id {
			return fmt.Errorf("节点 %s 上仍有工作负载，请先迁移", id)
		}
	}
	delete(m.nodes, id)
	return nil
}

// GetNode 获取节点
func (m *Manager) GetNode(id string) (*ClusterNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	node, exists := m.nodes[id]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", id)
	}
	return node, nil
}

// ListNodes 列出节点
func (m *Manager) ListNodes(status NodeStatus) []*ClusterNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var nodes []*ClusterNode
	for _, n := range m.nodes {
		if status == "" || n.Status == status {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// DeployWorkload 部署工作负载
func (m *Manager) DeployWorkload(wl *Workload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return fmt.Errorf("管理器未运行")
	}
	node, exists := m.nodes[wl.NodeID]
	if !exists {
		return fmt.Errorf("节点不存在: %s", wl.NodeID)
	}
	if node.Status != NodeStatusOnline {
		return fmt.Errorf("节点 %s 不在线", wl.NodeID)
	}
	wl.CreatedAt = time.Now()
	wl.UpdatedAt = time.Now()
	wl.Status = "running"
	m.workloads[wl.ID] = wl
	return nil
}

// MigrateWorkload 迁移工作负载
func (m *Manager) MigrateWorkload(workloadID, targetNodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	wl, exists := m.workloads[workloadID]
	if !exists {
		return fmt.Errorf("工作负载不存在: %s", workloadID)
	}
	target, exists := m.nodes[targetNodeID]
	if !exists {
		return fmt.Errorf("目标节点不存在: %s", targetNodeID)
	}
	if target.Status != NodeStatusOnline {
		return fmt.Errorf("目标节点不在线: %s", targetNodeID)
	}
	wl.NodeID = targetNodeID
	wl.UpdatedAt = time.Now()
	return nil
}

// ListWorkloads 列出工作负载
func (m *Manager) ListWorkloads(nodeID string) []*Workload {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var wls []*Workload
	for _, w := range m.workloads {
		if nodeID == "" || w.NodeID == nodeID {
			wls = append(wls, w)
		}
	}
	return wls
}

// AddPolicy 添加策略
func (m *Manager) AddPolicy(p *ClusterPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies[p.Name] = p
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	onlineNodes := 0
	for _, n := range m.nodes {
		if n.Status == NodeStatusOnline {
			onlineNodes++
		}
	}
	return map[string]interface{}{
		"running":      m.running,
		"total_nodes":  len(m.nodes),
		"online_nodes": onlineNodes,
		"total_workloads": len(m.workloads),
		"total_policies":  len(m.policies),
		"uptime":       time.Since(m.startTime).String(),
	}
}
