// Package cluster 集群管理器核心类型定义
// 此文件定义了集群管理器的基础类型，供其他模块使用
package cluster

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 集群角色常量
const (
	RoleMaster = "master"
	RoleWorker = "worker"
	RoleSlave  = "slave"
)

// 集群状态常量
const (
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusDegraded = "degraded"
	StatusSyncing  = "syncing"
	StatusError    = "error"
)

// SimpleClusterConfig 简化集群配置
type SimpleClusterConfig struct {
	Name              string `json:"name"`
	NodeID            string `json:"node_id"`
	DiscoveryPort     int    `json:"discovery_port"`
	HeartbeatInterval int    `json:"heartbeat_interval"`
	HeartbeatTimeout  int    `json:"heartbeat_timeout"`
	DataDir           string `json:"data_dir"`
}

// Member 集群成员
type Member struct {
	ID        string            `json:"id"`
	Hostname  string            `json:"hostname"`
	IP        string            `json:"ip"`
	Port      int               `json:"port"`
	Role      string            `json:"role"`
	Status    string            `json:"status"`
	Heartbeat time.Time         `json:"heartbeat"`
	JoinTime  time.Time         `json:"join_time"`
	Metrics   NodeMetrics       `json:"metrics"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// NodeMetrics 节点指标
type NodeMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIn   int64   `json:"network_in"`
	NetworkOut  int64   `json:"network_out"`
	Temperature float64 `json:"temperature"`
	LoadAvg     float64 `json:"load_avg"`
}

// Callbacks 集群回调
type Callbacks struct {
	OnNodeJoin   func(node *Member)
	OnNodeLeave  func(node *Member)
	OnNodeUpdate func(node *Member)
	OnMasterChange func(oldMaster, newMaster string)
}

// Manager 集群管理器
type Manager struct {
	config      SimpleClusterConfig
	nodes       map[string]*Member
	nodesMutex  sync.RWMutex
	masterID    string
	masterMutex sync.RWMutex
	logger      *zap.Logger
	callbacks   Callbacks
	ctx         interface{}
	cancel      interface{}
}

// NewManager 创建集群管理器
func NewManager(config SimpleClusterConfig, logger *zap.Logger) (*Manager, error) {
	if config.NodeID == "" {
		config.NodeID = "node-1"
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/cluster"
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 10
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 30
	}

	m := &Manager{
		config:   config,
		nodes:    make(map[string]*Member),
		masterID: config.NodeID,
		logger:   logger,
	}

	// 添加自身为节点
	self := &Member{
		ID:        config.NodeID,
		Hostname:  "localhost",
		IP:        "127.0.0.1",
		Port:      8080,
		Role:      "master",
		Status:    "online",
		Heartbeat: time.Now(),
		JoinTime:  time.Now(),
	}
	m.nodes[config.NodeID] = self

	return m, nil
}

// GetNodes 获取所有节点
func (m *Manager) GetNodes() []*Member {
	m.nodesMutex.RLock()
	defer m.nodesMutex.RUnlock()

	nodes := make([]*Member, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetOnlineNodes 获取在线节点
func (m *Manager) GetOnlineNodes() []*Member {
	m.nodesMutex.RLock()
	defer m.nodesMutex.RUnlock()

	online := make([]*Member, 0)
	for _, node := range m.nodes {
		if node.Status == StatusOnline {
			online = append(online, node)
		}
	}
	return online
}

// GetNode 获取指定节点
func (m *Manager) GetNode(nodeID string) (*Member, bool) {
	m.nodesMutex.RLock()
	defer m.nodesMutex.RUnlock()

	node, exists := m.nodes[nodeID]
	return node, exists
}

// RemoveNode 移除节点
func (m *Manager) RemoveNode(nodeID string) error {
	m.nodesMutex.Lock()
	defer m.nodesMutex.Unlock()

	if _, exists := m.nodes[nodeID]; !exists {
		return fmt.Errorf("节点不存在：%s", nodeID)
	}

	delete(m.nodes, nodeID)

	if m.callbacks.OnNodeLeave != nil {
		node := &Member{ID: nodeID}
		go m.callbacks.OnNodeLeave(node)
	}

	return nil
}

// UpdateNodeMetrics 更新节点指标
func (m *Manager) UpdateNodeMetrics(nodeID string, metrics NodeMetrics) error {
	m.nodesMutex.Lock()
	defer m.nodesMutex.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil
	}

	node.Metrics = metrics
	node.Heartbeat = time.Now()

	return nil
}

// IsMaster 是否为主节点
func (m *Manager) IsMaster() bool {
	m.masterMutex.RLock()
	defer m.masterMutex.RUnlock()

	return m.masterID == m.config.NodeID
}

// GetMasterID 获取主节点 ID
func (m *Manager) GetMasterID() string {
	m.masterMutex.RLock()
	defer m.masterMutex.RUnlock()

	return m.masterID
}

// SetCallbacks 设置回调
func (m *Manager) SetCallbacks(callbacks Callbacks) {
	m.callbacks = callbacks
}

// Shutdown 关闭管理器
func (m *Manager) Shutdown() error {
	return nil
}

// Initialize 初始化管理器
func (m *Manager) Initialize() error {
	m.logger.Info("集群管理器已初始化", zap.String("node_id", m.config.NodeID))
	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() SimpleClusterConfig {
	return m.config
}
