// Package nasclusterfed 提供NAS联邦集群管理功能
package nasclusterfed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 联邦集群管理器
type Manager struct {
	mu              sync.RWMutex
	clusters        map[string]*Cluster
	syncTasks       map[string]*SyncTask
	syncPolicies    map[string]*SyncPolicy
	metrics         map[string]*ClusterMetrics
	events          []FederationEvent
	dataDir         string
	syncInterval    time.Duration
	stopChan        chan struct{}
	running         bool
	subscribers     []chan *FederationEvent
	loadBalancer    *LoadBalancerConfig
	discoveryMethod DiscoveryMethod
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DataDir         string
	SyncInterval    time.Duration
	DiscoveryMethod DiscoveryMethod
	LoadBalancer    *LoadBalancerConfig
}

// NewManager 创建联邦集群管理器
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = 30 * time.Second
	}
	if cfg.DiscoveryMethod == "" {
		cfg.DiscoveryMethod = DiscoveryStatic
	}

	m := &Manager{
		clusters:        make(map[string]*Cluster),
		syncTasks:       make(map[string]*SyncTask),
		syncPolicies:    make(map[string]*SyncPolicy),
		metrics:         make(map[string]*ClusterMetrics),
		events:          make([]FederationEvent, 0),
		dataDir:         cfg.DataDir,
		syncInterval:    cfg.SyncInterval,
		stopChan:        make(chan struct{}),
		subscribers:     make([]chan *FederationEvent, 0),
		loadBalancer:    cfg.LoadBalancer,
		discoveryMethod: cfg.DiscoveryMethod,
	}

	if m.loadBalancer == nil {
		m.loadBalancer = &LoadBalancerConfig{
			Strategy:            "round-robin",
			HealthCheckPath:     "/health",
			HealthCheckInterval: 10 * time.Second,
			MaxRetries:          3,
		}
	}

	// 确保数据目录存在
	if m.dataDir != "" {
		if err := os.MkdirAll(m.dataDir, 0750); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
		// 加载已保存的配置
		if err := m.loadConfig(); err != nil {
			fmt.Printf("加载配置失败: %v\n", err)
		}
	}

	return m, nil
}

// Start 启动管理器
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.syncLoop()
	go m.discoveryLoop()
	go m.metricsLoop()
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		close(m.stopChan)
		m.running = false
		_ = m.saveConfig()
	}
}

// RegisterCluster 注册集群
func (m *Manager) RegisterCluster(cluster *Cluster) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cluster.ID == "" {
		return fmt.Errorf("集群ID不能为空")
	}

	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()
	if cluster.Status == "" {
		cluster.Status = ClusterStatusOnline
	}
	if cluster.Nodes == nil {
		cluster.Nodes = make([]*ClusterNode, 0)
	}

	m.clusters[cluster.ID] = cluster
	m.addEvent("cluster_registered", cluster.ID, "集群已注册", "info")

	return nil
}

// UnregisterCluster 注销集群
func (m *Manager) UnregisterCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[clusterID]; !exists {
		return fmt.Errorf("集群不存在: %s", clusterID)
	}

	delete(m.clusters, clusterID)
	m.addEvent("cluster_unregistered", clusterID, "集群已注销", "info")

	return nil
}

// GetCluster 获取集群信息
func (m *Manager) GetCluster(clusterID string) (*Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, exists := m.clusters[clusterID]
	if !exists {
		return nil, fmt.Errorf("集群不存在: %s", clusterID)
	}

	return cluster, nil
}

// ListClusters 列出所有集群
func (m *Manager) ListClusters() []*Cluster {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]*Cluster, 0, len(m.clusters))
	for _, c := range m.clusters {
		clusters = append(clusters, c)
	}
	return clusters
}

// AddNodeToCluster 添加节点到集群
func (m *Manager) AddNodeToCluster(clusterID string, node *ClusterNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.clusters[clusterID]
	if !exists {
		return fmt.Errorf("集群不存在: %s", clusterID)
	}

	node.ConnectedAt = time.Now()
	node.LastSeen = time.Now()
	if node.Status == "" {
		node.Status = ClusterStatusOnline
	}

	cluster.Nodes = append(cluster.Nodes, node)
	cluster.UpdatedAt = time.Now()

	m.addEvent("node_added", clusterID, fmt.Sprintf("节点 %s 已加入", node.Hostname), "info")

	return nil
}

// RemoveNodeFromCluster 从集群移除节点
func (m *Manager) RemoveNodeFromCluster(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.clusters[clusterID]
	if !exists {
		return fmt.Errorf("集群不存在: %s", clusterID)
	}

	for i, node := range cluster.Nodes {
		if node.ID == nodeID {
			cluster.Nodes = append(cluster.Nodes[:i], cluster.Nodes[i+1:]...)
			cluster.UpdatedAt = time.Now()
			m.addEvent("node_removed", clusterID, fmt.Sprintf("节点 %s 已移除", node.Hostname), "info")
			return nil
		}
	}

	return fmt.Errorf("节点不存在: %s", nodeID)
}

// CreateSyncTask 创建同步任务
func (m *Manager) CreateSyncTask(sourceID, targetID string, mode SyncMode) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[sourceID]; !exists {
		return nil, fmt.Errorf("源集群不存在: %s", sourceID)
	}
	if _, exists := m.clusters[targetID]; !exists {
		return nil, fmt.Errorf("目标集群不存在: %s", targetID)
	}

	task := &SyncTask{
		ID:              fmt.Sprintf("sync-%d", time.Now().UnixNano()),
		SourceClusterID: sourceID,
		TargetClusterID: targetID,
		Mode:            mode,
		Status:          "pending",
		StartedAt:       time.Now(),
	}

	m.syncTasks[task.ID] = task
	m.addEvent("sync_task_created", sourceID, "同步任务已创建", "info")

	return task, nil
}

// GetSyncTask 获取同步任务状态
func (m *Manager) GetSyncTask(taskID string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.syncTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("同步任务不存在: %s", taskID)
	}

	return task, nil
}

// ListSyncTasks 列出所有同步任务
func (m *Manager) ListSyncTasks() []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*SyncTask, 0, len(m.syncTasks))
	for _, t := range m.syncTasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// GetClusterMetrics 获取集群指标
func (m *Manager) GetClusterMetrics(clusterID string) (*ClusterMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[clusterID]
	if !exists {
		return nil, fmt.Errorf("集群指标不存在: %s", clusterID)
	}

	return metrics, nil
}

// GetFederationStatus 获取联邦状态概览
func (m *Manager) GetFederationStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalClusters := len(m.clusters)
	onlineClusters := 0
	totalNodes := 0

	for _, c := range m.clusters {
		if c.Status == ClusterStatusOnline {
			onlineClusters++
		}
		totalNodes += len(c.Nodes)
	}

	return map[string]interface{}{
		"totalClusters":   totalClusters,
		"onlineClusters":  onlineClusters,
		"totalNodes":      totalNodes,
		"activeSyncs":     len(m.syncTasks),
		"discoveryMethod": m.discoveryMethod,
	}
}

// GetEvents 获取联邦事件
func (m *Manager) GetEvents(limit int) []FederationEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}

	return m.events[start:]
}

// Subscribe 订阅联邦事件
func (m *Manager) Subscribe() <-chan *FederationEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *FederationEvent, 100)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

// syncLoop 同步循环
func (m *Manager) syncLoop() {
	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.processSyncTasks()
		}
	}
}

// discoveryLoop 集群发现循环
func (m *Manager) discoveryLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.discoverClusters()
		}
	}
}

// metricsLoop 指标收集循环
func (m *Manager) metricsLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectMetrics()
		}
	}
}

// processSyncTasks 处理同步任务
func (m *Manager) processSyncTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.syncTasks {
		if task.Status == "pending" {
			task.Status = "running"
			m.addEvent("sync_started", task.SourceClusterID, "同步任务开始", "info")
		}
	}
}

// discoverClusters 发现集群
func (m *Manager) discoverClusters() {
	// 实现集群发现逻辑
	switch m.discoveryMethod {
	case DiscoveryMulticast:
		// 组播发现
	case DiscoveryDNS:
		// DNS发现
	case DiscoveryConsul:
		// Consul发现
	default:
		// 静态配置，无需发现
	}
}

// collectMetrics 收集指标
func (m *Manager) collectMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for clusterID, cluster := range m.clusters {
		if cluster.Status != ClusterStatusOnline {
			continue
		}

		metrics := &ClusterMetrics{
			ClusterID:   clusterID,
			Timestamp:   now,
			HealthScore: 100.0,
		}

		// 计算平均指标
		for _, node := range cluster.Nodes {
			metrics.CPUCores += node.CPUCores
			metrics.MemoryGB += node.MemoryGB
			metrics.StorageTB += node.StorageTB
		}

		m.metrics[clusterID] = metrics
	}
}

// addEvent 添加事件
func (m *Manager) addEvent(eventType, clusterID, message, severity string) {
	event := FederationEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Type:      eventType,
		ClusterID: clusterID,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	}

	m.events = append(m.events, event)

	// 保持最近1000条事件
	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}

	// 通知订阅者
	for _, sub := range m.subscribers {
		select {
		case sub <- &event:
		default:
			// 队列满则跳过
		}
	}
}

// saveConfig 保存配置
func (m *Manager) saveConfig() error {
	if m.dataDir == "" {
		return nil
	}

	data := map[string]interface{}{
		"clusters":     m.clusters,
		"syncPolicies": m.syncPolicies,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	configPath := filepath.Join(m.dataDir, "federation.json")
	return os.WriteFile(configPath, jsonData, 0640)
}

// loadConfig 加载配置
func (m *Manager) loadConfig() error {
	if m.dataDir == "" {
		return nil
	}

	configPath := filepath.Join(m.dataDir, "federation.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取配置失败: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	return nil
}
