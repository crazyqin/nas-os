// Package clustermgr 提供分布式集群管理功能
package clustermgr

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager 集群管理器.
type Manager struct {
	mu        sync.RWMutex
	cluster   *Cluster
	discovery *ServiceDiscovery
	balancer  LoadBalancer
	stats     ClusterConfig
	cancel    context.CancelFunc
	startTime time.Time

	// 节点管理
	nodes    map[string]*Node
	leaderID string

	// 配置
	config ClusterConfig

	// 回调函数
	onNodeJoin     func(node *Node)
	onNodeLeave    func(node *Node)
	onNodeFail     func(node *Node)
	onLeaderChange func(leaderID string)
}

// NewManager 创建集群管理器.
func NewManager(config ClusterConfig) *Manager {
	return &Manager{
		cluster: &Cluster{
			ID:        generateClusterID(),
			Name:      "nas-os-cluster",
			Status:    ClusterStatusUnknown,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Nodes:     make(map[string]*Node),
		},
		discovery: &ServiceDiscovery{
			Services: make(map[string]*ServiceInfo),
		},
		nodes:     make(map[string]*Node),
		config:    config,
		startTime: time.Now(),
	}
}

// Start 启动集群管理器.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建可取消的上下文
	ctx, m.cancel = context.WithCancel(ctx)

	// 初始化负载均衡器
	m.balancer = NewLoadBalancer(m.config.LoadBalanceStrategy)

	// 启动心跳检测
	go m.heartbeatChecker(ctx)

	// 启动服务发现清理
	go m.serviceCleanup(ctx)

	// 启动故障检测
	go m.failureDetector(ctx)

	// 启动统计更新
	go m.statsUpdater(ctx)

	log.Printf("[集群管理器] 启动成功，集群ID: %s", m.cluster.ID)
	return nil
}

// Stop 停止集群管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	log.Printf("[集群管理器] 已停止")
}

// GetCluster 获取集群信息.
func (m *Manager) GetCluster() *Cluster {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cluster
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() ClusterConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config ClusterConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if config.HeartbeatInterval <= 0 {
		return fmt.Errorf("心跳间隔必须大于0")
	}
	if config.HeartbeatTimeout <= 0 {
		return fmt.Errorf("心跳超时必须大于0")
	}
	if config.MaxNodes <= 0 {
		return fmt.Errorf("最大节点数必须大于0")
	}

	m.config = config
	m.cluster.Config = config
	log.Printf("[集群管理器] 配置已更新")
	return nil
}

// Join 节点加入集群.
func (m *Manager) Join(req *JoinRequest) (*JoinResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查节点数限制
	if len(m.nodes) >= m.config.MaxNodes {
		return nil, fmt.Errorf("集群节点数已达上限: %d", m.config.MaxNodes)
	}

	// 检查节点是否已存在
	if _, exists := m.nodes[req.NodeID]; exists {
		return nil, fmt.Errorf("节点已存在: %s", req.NodeID)
	}

	// 创建节点
	node := &Node{
		ID:            req.NodeID,
		Name:          req.Name,
		Address:       req.Address,
		Role:          RoleFollower,
		Status:        NodeStatusActive,
		Weight:        req.Weight,
		Zone:          req.Zone,
		Tags:          req.Tags,
		MaxConns:      req.MaxConns,
		Metadata:      req.Metadata,
		LastHeartbeat: time.Now(),
		JoinTime:      time.Now(),
	}

	// 添加到集群
	m.nodes[req.NodeID] = node
	m.cluster.AddNode(node)

	// 如果是第一个节点，设为领导者
	if len(m.nodes) == 1 {
		node.Role = RoleLeader
		m.leaderID = req.NodeID
		m.cluster.LeaderID = req.NodeID
	}

	// 更新负载均衡器
	m.balancer.UpdateNodes(m.getActiveNodes())

	// 调用回调
	if m.onNodeJoin != nil {
		go m.onNodeJoin(node)
	}

	log.Printf("[集群管理器] 节点加入: %s (%s)", req.NodeID, req.Address)

	// 构建响应
	response := &JoinResponse{
		Success:   true,
		ClusterID: m.cluster.ID,
		LeaderID:  m.leaderID,
		Nodes:     m.getAllNodes(),
		Config:    m.config,
		Message:   "节点加入成功",
	}

	return response, nil
}

// Leave 节点离开集群.
func (m *Manager) Leave(req *LeaveRequest) (*LeaveResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[req.NodeID]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", req.NodeID)
	}

	// 更新节点状态
	node.Status = NodeStatusLeaving
	node.LeaveTime = time.Now()

	// 从集群移除
	delete(m.nodes, req.NodeID)
	m.cluster.RemoveNode(req.NodeID)

	// 如果离开的是领导者，需要选举新领导者
	if req.NodeID == m.leaderID {
		m.electLeader()
	}

	// 更新负载均衡器
	m.balancer.UpdateNodes(m.getActiveNodes())

	// 调用回调
	if m.onNodeLeave != nil {
		go m.onNodeLeave(node)
	}

	log.Printf("[集群管理器] 节点离开: %s (原因: %s)", req.NodeID, req.Reason)

	return &LeaveResponse{
		Success: true,
		Message: "节点离开成功",
	}, nil
}

// Heartbeat 处理心跳.
func (m *Manager) Heartbeat(req *HeartbeatRequest) (*HeartbeatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[req.NodeID]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", req.NodeID)
	}

	// 更新节点状态
	node.UpdateHeartbeat()
	node.UpdateMetrics(req.CPUUsage, req.MemoryUsage, req.DiskUsage, req.LoadAvg)
	node.Connections = req.Connections

	return &HeartbeatResponse{
		Success:  true,
		LeaderID: m.leaderID,
		Version:  m.cluster.Version,
		Message:  "心跳成功",
	}, nil
}

// GetNode 获取节点信息.
func (m *Manager) GetNode(id string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	node, ok := m.nodes[id]
	return node, ok
}

// ListNodes 列出所有节点.
func (m *Manager) ListNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetActiveNodes 获取活跃节点.
func (m *Manager) GetActiveNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getActiveNodes()
}

// getActiveNodes 获取活跃节点（内部方法，需要锁）.
func (m *Manager) getActiveNodes() []*Node {
	var nodes []*Node
	for _, node := range m.nodes {
		if node.Status == NodeStatusActive {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// getAllNodes 获取所有节点（内部方法，需要锁）.
func (m *Manager) getAllNodes() []*Node {
	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// electLeader 选举新领导者（内部方法，需要锁）.
func (m *Manager) electLeader() {
	// 简单选举：选择第一个活跃节点
	for _, node := range m.nodes {
		if node.Status == NodeStatusActive {
			node.Role = RoleLeader
			m.leaderID = node.ID
			m.cluster.LeaderID = node.ID

			// 调用回调
			if m.onLeaderChange != nil {
				go m.onLeaderChange(node.ID)
			}

			log.Printf("[集群管理器] 新领导者选举: %s", node.ID)
			return
		}
	}

	// 没有活跃节点
	m.leaderID = ""
	m.cluster.LeaderID = ""
	log.Printf("[集群管理器] 无可用领导者")
}

// heartbeatChecker 心跳检测器.
func (m *Manager) heartbeatChecker(ctx context.Context) {
	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkHeartbeats()
		}
	}
}

// checkHeartbeats 检查所有节点心跳.
func (m *Manager) checkHeartbeats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, node := range m.nodes {
		if node.Status == NodeStatusActive {
			// 检查心跳是否超时
			if now.Sub(node.LastHeartbeat) > m.config.HeartbeatTimeout {
				node.Status = NodeStatusInactive
				log.Printf("[集群管理器] 节点心跳超时: %s", node.ID)
			}
		}
	}
}

// failureDetector 故障检测器.
func (m *Manager) failureDetector(ctx context.Context) {
	ticker := time.NewTicker(m.config.HeartbeatTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.detectFailures()
		}
	}
}

// detectFailures 检测故障节点.
func (m *Manager) detectFailures() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, node := range m.nodes {
		if node.Status == NodeStatusInactive {
			// 检查是否超过故障超时
			if now.Sub(node.LastHeartbeat) > m.config.HeartbeatTimeout*2 {
				node.Status = NodeStatusFailed
				log.Printf("[集群管理器] 节点故障: %s", node.ID)

				// 调用回调
				if m.onNodeFail != nil {
					go m.onNodeFail(node)
				}

				// 自动移除故障节点
				if m.config.AutoRemoveFailed {
					if now.Sub(node.LastHeartbeat) > m.config.FailedNodeTimeout {
						delete(m.nodes, node.ID)
						m.cluster.RemoveNode(node.ID)
						log.Printf("[集群管理器] 自动移除故障节点: %s", node.ID)
					}
				}
			}
		}
	}

	// 如果领导者故障，重新选举
	if m.leaderID != "" {
		if leader, exists := m.nodes[m.leaderID]; exists {
			if leader.Status == NodeStatusFailed {
				m.electLeader()
			}
		} else {
			m.electLeader()
		}
	}

	// 更新负载均衡器
	m.balancer.UpdateNodes(m.getActiveNodes())

	// 更新集群状态
	m.cluster.UpdateStatus()
}

// serviceCleanup 服务清理器.
func (m *Manager) serviceCleanup(ctx context.Context) {
	ticker := time.NewTicker(m.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanupExpiredServices()
		}
	}
}

// cleanupExpiredServices 清理过期服务.
func (m *Manager) cleanupExpiredServices() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, service := range m.discovery.Services {
		if service.IsExpired() {
			delete(m.discovery.Services, id)
			log.Printf("[集群管理器] 清理过期服务: %s", id)
		}
	}
}

// statsUpdater 统计更新器.
func (m *Manager) statsUpdater(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.updateStats()
		}
	}
}

// updateStats 更新统计信息.
func (m *Manager) updateStats() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeNodes := 0
	failedNodes := 0
	totalConns := 0

	for _, node := range m.nodes {
		switch node.Status {
		case NodeStatusActive:
			activeNodes++
			totalConns += node.Connections
		case NodeStatusFailed:
			failedNodes++
		}
	}

	// 更新集群统计
	// 注意：这里简化处理，实际应该更新 ClusterStats
	_ = activeNodes
	_ = failedNodes
	_ = totalConns
}

// RegisterService 注册服务.
func (m *Manager) RegisterService(service *ServiceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置过期时间（默认1小时）
	if service.ExpiresAt.IsZero() {
		service.ExpiresAt = time.Now().Add(time.Hour)
	}

	m.discovery.RegisterService(service)
	log.Printf("[集群管理器] 服务注册: %s (%s:%d)", service.Name, service.Address, service.Port)
	return nil
}

// DeregisterService 注销服务.
func (m *Manager) DeregisterService(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.discovery.DeregisterService(id) {
		log.Printf("[集群管理器] 服务注销: %s", id)
		return nil
	}
	return fmt.Errorf("服务不存在: %s", id)
}

// GetService 获取服务.
func (m *Manager) GetService(id string) (*ServiceInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.discovery.GetService(id)
}

// ListServices 列出所有服务.
func (m *Manager) ListServices() []*ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	services := make([]*ServiceInfo, 0, len(m.discovery.Services))
	for _, service := range m.discovery.Services {
		services = append(services, service)
	}
	return services
}

// GetHealthyServices 获取健康的服务.
func (m *Manager) GetHealthyServices() []*ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.discovery.GetHealthyServices()
}

// GetServicesByName 按名称获取服务.
func (m *Manager) GetServicesByName(name string) []*ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.discovery.GetServicesByName(name)
}

// SelectNode 选择节点（负载均衡）.
func (m *Manager) SelectNode(strategy LoadBalanceStrategy, key string) (*Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.balancer == nil {
		return nil, fmt.Errorf("负载均衡器未初始化")
	}

	nodes := m.getActiveNodes()
	if len(nodes) == 0 {
		return nil, fmt.Errorf("无可用节点")
	}

	return m.balancer.Select(nodes, key)
}

// GetStats 获取集群统计.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeNodes := 0
	failedNodes := 0
	totalConns := 0

	for _, node := range m.nodes {
		switch node.Status {
		case NodeStatusActive:
			activeNodes++
			totalConns += node.Connections
		case NodeStatusFailed:
			failedNodes++
		}
	}

	return map[string]interface{}{
		"clusterId":     m.cluster.ID,
		"clusterName":   m.cluster.Name,
		"clusterStatus": m.cluster.Status,
		"leaderId":      m.leaderID,
		"totalNodes":    len(m.nodes),
		"activeNodes":   activeNodes,
		"failedNodes":   failedNodes,
		"totalConns":    totalConns,
		"totalServices": len(m.discovery.Services),
		"uptime":        time.Since(m.startTime).String(),
		"startTime":     m.startTime,
	}
}

// SetOnNodeJoin 设置节点加入回调.
func (m *Manager) SetOnNodeJoin(callback func(node *Node)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNodeJoin = callback
}

// SetOnNodeLeave 设置节点离开回调.
func (m *Manager) SetOnNodeLeave(callback func(node *Node)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNodeLeave = callback
}

// SetOnNodeFail 设置节点故障回调.
func (m *Manager) SetOnNodeFail(callback func(node *Node)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNodeFail = callback
}

// SetOnLeaderChange 设置领导者变更回调.
func (m *Manager) SetOnLeaderChange(callback func(leaderID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onLeaderChange = callback
}

// generateClusterID 生成集群ID.
func generateClusterID() string {
	return fmt.Sprintf("cluster-%d", time.Now().UnixNano())
}
