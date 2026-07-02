// Package containerfailover 容器 HA 故障转移模块
package containerfailover

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 容器故障转移管理器
// 管理 HA 集群中的容器注册、健康检查、故障转移和状态同步。
type Manager struct {
	mu sync.RWMutex
	// containers 容器表：ID -> *Container
	containers map[string]*Container
	// nodes 集群节点表：ID -> *ClusterNode
	nodes map[string]*ClusterNode
	// policy 故障转移策略
	policy *FailoverPolicy
	// ipManager IP 管理器
	ipManager *IPManager
	// stateSync 状态同步器
	stateSync *StateSync
	// events 故障转移事件历史
	events []FailoverEvent
	// healthStop 健康检查停止通道
	healthStop chan struct{}
	// healthRunning 健康检查是否运行中
	healthRunning bool
	// failoverActive 是否正在故障转移
	failoverActive bool
	// smbHAEnabled 是否启用 SMB HA
	smbHAEnabled bool
}

// NewManager 创建故障转移管理器
// localNodeID 本节点 ID
// backend 状态同步后端（可为 nil 表示不启用远程同步）.
func NewManager(localNodeID string, backend StateBackend) *Manager {
	m := &Manager{
		containers: make(map[string]*Container),
		nodes:      make(map[string]*ClusterNode),
		policy:     DefaultFailoverPolicy(),
		ipManager:  NewIPManager(),
		events:     make([]FailoverEvent, 0),
	}

	if backend != nil {
		m.stateSync = NewStateSync(backend, localNodeID)
	}

	// 注册本节点
	localNode := &ClusterNode{
		ID:         localNodeID,
		Name:       localNodeID,
		IP:         "",
		Status:     NodeOnline,
		Containers: []string{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.nodes[localNodeID] = localNode

	return m
}

// ========== 策略管理 ==========

// GetPolicy 获取故障转移策略.
func (m *Manager) GetPolicy() *FailoverPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.policy == nil {
		return DefaultFailoverPolicy()
	}
	policyCopy := *m.policy
	return &policyCopy
}

// SetPolicy 设置故障转移策略.
func (m *Manager) SetPolicy(p *FailoverPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.HealthCheckInterval <= 0 {
		return fmt.Errorf("健康检查间隔必须大于 0")
	}
	if p.FailoverDelay < 0 {
		return fmt.Errorf("故障转移延迟不能为负数")
	}
	m.policy = p
	m.smbHAEnabled = p.SMBHA
	return nil
}

// ========== 节点管理 ==========

// RegisterNode 注册节点到 HA 集群.
func (m *Manager) RegisterNode(n *ClusterNode) (*ClusterNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if n.ID == "" {
		return nil, fmt.Errorf("节点 ID 不能为空")
	}
	if n.Name == "" {
		return nil, fmt.Errorf("节点名称不能为空")
	}

	node := &ClusterNode{
		ID:         n.ID,
		Name:       n.Name,
		IP:         n.IP,
		Status:     NodeOnline,
		Containers: n.Containers,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.nodes[n.ID] = node

	// 同步到后端
	if m.stateSync != nil {
		_ = m.stateSync.SyncNode(node)
	}

	nodeCopy := *node
	return &nodeCopy, nil
}

// GetNode 获取节点信息.
func (m *Manager) GetNode(nodeID string) (*ClusterNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点 %s 不存在", nodeID)
	}
	nodeCopy := *node
	return &nodeCopy, nil
}

// ListNodes 列出所有节点.
func (m *Manager) ListNodes() []*ClusterNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ClusterNode, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodeCopy := *n
		result = append(result, &nodeCopy)
	}
	return result
}

// UpdateNodeStatus 更新节点状态.
func (m *Manager) UpdateNodeStatus(nodeID string, status NodeStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}
	node.Status = status
	node.UpdatedAt = time.Now()

	if m.stateSync != nil {
		_ = m.stateSync.SyncNode(node)
	}
	return nil
}

// ========== 容器管理 ==========

// RegisterContainer 注册容器到 HA 集群.
func (m *Manager) RegisterContainer(c *Container) (*Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c.ID == "" {
		return nil, fmt.Errorf("容器 ID 不能为空")
	}
	if c.Name == "" {
		return nil, fmt.Errorf("容器名称不能为空")
	}
	if c.Image == "" {
		return nil, fmt.Errorf("容器镜像不能为空")
	}

	container := &Container{
		ID:            c.ID,
		Name:          c.Name,
		Image:         c.Image,
		IP:            c.IP,
		Status:        ContainerRunning,
		Node:          c.Node,
		PreferredNode: c.PreferredNode,
		Ports:         c.Ports,
		Volumes:       c.Volumes,
		Labels:        c.Labels,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 如果指定了节点，检查节点是否存在
	if c.Node != "" {
		if _, exists := m.nodes[c.Node]; !exists {
			return nil, fmt.Errorf("节点 %s 不存在", c.Node)
		}
		// 将容器加入节点列表
		m.nodes[c.Node].Containers = append(m.nodes[c.Node].Containers, c.ID)
	}

	m.containers[c.ID] = container

	// 分配静态 IP（如果指定）
	if c.IP != "" && c.Node != "" {
		if _, err := m.ipManager.Allocate(c.IP, c.ID, c.Node, "eth0"); err != nil {
			return nil, fmt.Errorf("IP 分配失败: %w", err)
		}
	}

	// 同步到后端
	if m.stateSync != nil {
		_ = m.stateSync.SyncContainer(container)
	}

	containerCopy := *container
	return &containerCopy, nil
}

// GetContainer 获取容器信息.
func (m *Manager) GetContainer(containerID string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}
	cCopy := *c
	return &cCopy, nil
}

// ListContainers 列出所有容器.
func (m *Manager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		cCopy := *c
		result = append(result, &cCopy)
	}
	return result
}

// RemoveContainer 从集群中移除容器.
func (m *Manager) RemoveContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	// 从节点容器列表中移除
	if node, exists := m.nodes[c.Node]; exists {
		newContainers := make([]string, 0, len(node.Containers))
		for _, id := range node.Containers {
			if id != containerID {
				newContainers = append(newContainers, id)
			}
		}
		node.Containers = newContainers
	}

	// 释放 IP
	_ = m.ipManager.Release(containerID)

	// 从后端删除
	if m.stateSync != nil {
		_ = m.stateSync.DeleteContainer(containerID)
	}

	delete(m.containers, containerID)
	return nil
}

// ========== 健康检查 ==========

// StartHealthCheck 启动容器健康检查.
func (m *Manager) StartHealthCheck() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.healthRunning {
		return fmt.Errorf("健康检查已在运行")
	}

	m.healthRunning = true
	m.healthStop = make(chan struct{})
	go m.runHealthCheck()
	return nil
}

// StopHealthCheck 停止健康检查.
func (m *Manager) StopHealthCheck() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.healthRunning {
		return fmt.Errorf("健康检查未运行")
	}
	close(m.healthStop)
	m.healthRunning = false
	return nil
}

// runHealthCheck 健康检查循环.
func (m *Manager) runHealthCheck() {
	interval := m.GetPolicy().HealthCheckInterval
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	failCount := make(map[string]int) // 容器 ID -> 连续失败次数

	for {
		select {
		case <-m.healthStop:
			return
		case <-ticker.C:
			m.checkAllContainers(failCount)
		}
	}
}

// checkAllContainers 检查所有容器健康状态.
func (m *Manager) checkAllContainers(failCount map[string]int) {
	m.mu.RLock()
	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		cCopy := *c
		containers = append(containers, &cCopy)
	}
	policy := m.GetPolicy()
	m.mu.RUnlock()

	for _, c := range containers {
		healthy := m.checkContainerHealth(c)

		if !healthy {
			failCount[c.ID]++
			if failCount[c.ID] >= policy.MaxRetryAttempts {
				// 健康检查连续失败，触发故障转移
				if policy.AutoFailover {
					m.triggerAutoFailover(c.ID, fmt.Sprintf("健康检查连续失败 %d 次", failCount[c.ID]), TriggerHealthCheck)
				}
				failCount[c.ID] = 0
			}
		} else {
			failCount[c.ID] = 0
		}
	}
}

// checkContainerHealth 检查单个容器健康状态
// 此处使用模拟检查，实际环境可对接容器运行时 API.
func (m *Manager) checkContainerHealth(c *Container) bool {
	if c.Status == ContainerFailed || c.Status == ContainerStopped {
		return false
	}
	// 检查容器所在节点是否在线
	m.mu.RLock()
	defer m.mu.RUnlock()
	if node, exists := m.nodes[c.Node]; exists {
		return node.Status == NodeOnline
	}
	return false
}

// ========== 故障转移 ==========

// ManualFailover 手动触发容器故障转移.
func (m *Manager) ManualFailover(containerID, toNode, reason string) (*FailoverEvent, error) {
	m.mu.Lock()
	if m.failoverActive {
		m.mu.Unlock()
		return nil, fmt.Errorf("故障转移正在进行中")
	}
	m.failoverActive = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.failoverActive = false
		m.mu.Unlock()
	}()

	event, err := m.executeFailover(containerID, toNode, reason, TriggerManual)
	if err != nil {
		return nil, err
	}
	// 同步故障转移事件到后端
	if event != nil && m.stateSync != nil {
		_ = m.stateSync.SyncFailoverEvent(event)
	}
	return event, nil
}

// triggerAutoFailover 自动触发故障转移.
func (m *Manager) triggerAutoFailover(containerID, reason string, trigger FailoverTrigger) {
	m.mu.Lock()
	if m.failoverActive {
		m.mu.Unlock()
		return
	}
	m.failoverActive = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.failoverActive = false
		m.mu.Unlock()
	}()

	// 选择目标节点
	toNode := m.selectFailoverTarget(containerID)
	if toNode == "" {
		// 没有可用目标节点，记录失败事件
		event := FailoverEvent{
			ID:          uuid.New().String(),
			ContainerID: containerID,
			TriggeredAt: time.Now(),
			Trigger:     trigger,
			Reason:      reason,
			Success:     false,
			Error:       "没有可用的故障转移目标节点",
		}
		m.mu.Lock()
		m.events = append(m.events, event)
		m.mu.Unlock()
		return
	}

	event, _ := m.executeFailover(containerID, toNode, reason, trigger)
	if event != nil && m.stateSync != nil {
		_ = m.stateSync.SyncFailoverEvent(event)
	}
}

// executeFailover 执行故障转移.
func (m *Manager) executeFailover(containerID, toNode, reason string, trigger FailoverTrigger) (*FailoverEvent, error) {
	m.mu.Lock()

	c, exists := m.containers[containerID]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	targetNode, exists := m.nodes[toNode]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("目标节点 %s 不存在", toNode)
	}

	if targetNode.Status != NodeOnline {
		m.mu.Unlock()
		return nil, fmt.Errorf("目标节点 %s 不在线", toNode)
	}

	fromNode := c.Node

	// 标记容器为故障转移中
	c.Status = ContainerFailingOver
	c.UpdatedAt = time.Now()

	// 应用故障转移延迟
	policy := m.policy
	delay := time.Duration(policy.FailoverDelay) * time.Second
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	event := FailoverEvent{
		ID:            uuid.New().String(),
		ContainerID:   containerID,
		ContainerName: c.Name,
		TriggeredAt:   time.Now(),
		Trigger:       trigger,
		FromNode:      fromNode,
		ToNode:        toNode,
		Reason:        reason,
	}

	startTime := time.Now()

	// 1. 从源节点容器列表中移除
	if oldNode, exists := m.nodes[fromNode]; exists {
		newList := make([]string, 0, len(oldNode.Containers))
		for _, id := range oldNode.Containers {
			if id != containerID {
				newList = append(newList, id)
			}
		}
		oldNode.Containers = newList
		oldNode.UpdatedAt = time.Now()
	}

	// 2. 迁移容器到目标节点
	c.Node = toNode
	c.Status = ContainerRunning
	c.UpdatedAt = time.Now()
	targetNode.Containers = append(targetNode.Containers, containerID)
	targetNode.UpdatedAt = time.Now()

	// 3. 迁移静态 IP
	if c.IP != "" {
		_, err := m.ipManager.Migrate(c.IP, toNode)
		if err != nil {
			event.Error = fmt.Sprintf("IP 迁移失败: %v", err)
		} else {
			event.IPMigrated = true
		}
	}

	// 4. SMB HA 故障转移（如果启用）
	if m.smbHAEnabled {
		event.SMBFailover = true
		// 模拟 SMB HA 故障转移
		// 实际环境中会触发 SMB 锁定迁移和客户端重连
	}

	// 5. 同步到后端
	if m.stateSync != nil {
		_ = m.stateSync.SyncContainer(c)
		_ = m.stateSync.SyncNode(m.nodes[fromNode])
		_ = m.stateSync.SyncNode(targetNode)
	}

	event.Success = true
	now := time.Now()
	event.CompletedAt = &now
	event.Duration = time.Since(startTime).Milliseconds()

	m.events = append(m.events, event)

	return &event, nil
}

// selectFailoverTarget 选择故障转移目标节点.
func (m *Manager) selectFailoverTarget(containerID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, exists := m.containers[containerID]
	if !exists {
		return ""
	}

	// 优先使用 PreferredNode
	if c.PreferredNode != "" && c.PreferredNode != c.Node {
		if node, exists := m.nodes[c.PreferredNode]; exists && node.Status == NodeOnline {
			return c.PreferredNode
		}
	}

	// 选择任意在线且非当前节点
	for id, node := range m.nodes {
		if id != c.Node && node.Status == NodeOnline {
			return id
		}
	}
	return ""
}

// GetFailoverHistory 获取故障转移历史.
func (m *Manager) GetFailoverHistory(limit int) []FailoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := m.events
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	result := make([]FailoverEvent, len(events))
	copy(result, events)
	return result
}

// ========== 状态查询 ==========

// ClusterStatus 集群状态摘要.
type ClusterStatus struct {
	TotalNodes        int            `json:"total_nodes"`
	OnlineNodes       int            `json:"online_nodes"`
	TotalContainers   int            `json:"total_containers"`
	RunningContainers int            `json:"running_containers"`
	FailedContainers  int            `json:"failed_containers"`
	FailoverCount     int            `json:"failover_count"`
	Nodes             []*ClusterNode `json:"nodes"`
	Containers        []*Container   `json:"containers"`
	LastFailover      *FailoverEvent `json:"last_failover,omitempty"`
}

// GetClusterStatus 获取集群状态摘要.
func (m *Manager) GetClusterStatus() *ClusterStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &ClusterStatus{
		TotalNodes:      len(m.nodes),
		TotalContainers: len(m.containers),
		FailoverCount:   len(m.events),
		Nodes:           make([]*ClusterNode, 0),
		Containers:      make([]*Container, 0),
	}

	for _, n := range m.nodes {
		nc := *n
		status.Nodes = append(status.Nodes, &nc)
		if n.Status == NodeOnline {
			status.OnlineNodes++
		}
	}

	for _, c := range m.containers {
		cc := *c
		status.Containers = append(status.Containers, &cc)
		switch c.Status {
		case ContainerRunning:
			status.RunningContainers++
		case ContainerFailed:
			status.FailedContainers++
		}
	}

	if len(m.events) > 0 {
		lastEvent := m.events[len(m.events)-1]
		status.LastFailover = &lastEvent
	}

	return status
}

// ========== IP 管理接口封装 ==========

// GetIPManager 获取 IP 管理器.
func (m *Manager) GetIPManager() *IPManager {
	return m.ipManager
}

// ========== 状态同步接口封装 ==========

// GetStateSync 获取状态同步器.
func (m *Manager) GetStateSync() *StateSync {
	return m.stateSync
}

// SyncAll 将所有本地状态同步到后端.
func (m *Manager) SyncAll() error {
	if m.stateSync == nil {
		return fmt.Errorf("状态同步未配置")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 同步所有容器
	for _, c := range m.containers {
		if err := m.stateSync.SyncContainer(c); err != nil {
			return err
		}
	}
	// 同步所有节点
	for _, n := range m.nodes {
		if err := m.stateSync.SyncNode(n); err != nil {
			return err
		}
	}
	// 同步所有 IP 分配记录
	for _, alloc := range m.ipManager.ListAllocations() {
		if err := m.stateSync.SyncIPAllocation(alloc); err != nil {
			return err
		}
	}
	return nil
}
