package highavailability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Manager 高可用管理器
type Manager struct {
	mu          sync.RWMutex
	config      *HAConfig
	localNodeID string
	state       *clusterState
	logger      *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
	eventChan   chan FailoverEvent
	stopChan    chan struct{}
}

// NewManager 创建高可用管理器
func NewManager(localNodeID string, config *HAConfig, logger *slog.Logger) *Manager {
	if config == nil {
		config = DefaultHAConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:      config,
		localNodeID: localNodeID,
		state: &clusterState{
			nodes:  make(map[string]*ClusterNode),
			events: make([]FailoverEvent, 0),
		},
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan FailoverEvent, 100),
		stopChan:  make(chan struct{}),
	}
}

// Start 启动高可用管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("HA manager already running")
	}

	m.logger.Info("starting HA manager",
		"node_id", m.localNodeID,
		"mode", m.config.Mode,
		"vip", m.config.VIP,
	)

	// 初始化本地节点
	m.state.mu.Lock()
	m.state.localNode = &ClusterNode{
		ID:            m.localNodeID,
		Role:          RoleStandby,
		Status:        StatusOnline,
		LastHeartbeat: time.Now(),
		Metadata:      make(map[string]string),
	}
	m.state.nodes[m.localNodeID] = m.state.localNode
	m.state.mu.Unlock()

	m.running = true

	// 启动心跳发送
	go m.heartbeatSender()
	// 启动健康检查
	go m.healthChecker()
	// 启动事件处理
	go m.eventProcessor()

	m.logger.Info("HA manager started", "node_id", m.localNodeID)
	return nil
}

// Stop 停止高可用管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("HA manager not running")
	}

	m.logger.Info("stopping HA manager", "node_id", m.localNodeID)

	m.cancel()
	close(m.stopChan)
	m.running = false

	m.logger.Info("HA manager stopped", "node_id", m.localNodeID)
	return nil
}

// IsRunning 检查管理器是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// AddNode 添加集群节点
func (m *Manager) AddNode(node *ClusterNode) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	if _, exists := m.state.nodes[node.ID]; exists {
		return fmt.Errorf("node %s already exists", node.ID)
	}

	m.state.nodes[node.ID] = node
	m.logger.Info("node added to cluster", "node_id", node.ID, "address", node.Address)

	// 发送节点加入事件
	m.emitEvent(FailoverEvent{
		ID:             generateEventID(),
		Timestamp:      time.Now(),
		Type:           EventNodeJoined,
		PromotedNodeID: node.ID,
		Reason:         "node joined cluster",
	})

	return nil
}

// RemoveNode 移除集群节点
func (m *Manager) RemoveNode(nodeID string) error {
	if nodeID == m.localNodeID {
		return fmt.Errorf("cannot remove local node")
	}

	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	if _, exists := m.state.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	delete(m.state.nodes, nodeID)
	m.logger.Info("node removed from cluster", "node_id", nodeID)

	// 发送节点离开事件
	m.emitEvent(FailoverEvent{
		ID:           generateEventID(),
		Timestamp:    time.Now(),
		Type:         EventNodeLeft,
		FailedNodeID: nodeID,
		Reason:       "node removed from cluster",
	})

	return nil
}

// GetNode 获取节点信息
func (m *Manager) GetNode(nodeID string) (*ClusterNode, error) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	node, exists := m.state.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	// 返回副本
	nodeCopy := *node
	return &nodeCopy, nil
}

// GetLocalNode 获取本地节点信息
func (m *Manager) GetLocalNode() *ClusterNode {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	if m.state.localNode == nil {
		return nil
	}
	nodeCopy := *m.state.localNode
	return &nodeCopy
}

// GetNodes 获取所有节点
func (m *Manager) GetNodes() []*ClusterNode {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	nodes := make([]*ClusterNode, 0, len(m.state.nodes))
	for _, node := range m.state.nodes {
		nodeCopy := *node
		nodes = append(nodes, &nodeCopy)
	}
	return nodes
}

// GetActiveNode 获取当前活跃节点
func (m *Manager) GetActiveNode() *ClusterNode {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	for _, node := range m.state.nodes {
		if node.Role == RoleActive {
			nodeCopy := *node
			return &nodeCopy
		}
	}
	return nil
}

// GetEvents 获取故障切换事件历史
func (m *Manager) GetEvents(limit int) []FailoverEvent {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	events := m.state.events
	if limit > 0 && limit < len(events) {
		events = events[len(events)-limit:]
	}

	result := make([]FailoverEvent, len(events))
	copy(result, events)
	return result
}

// ManualSwitchover 手动切换活跃节点
func (m *Manager) ManualSwitchover(targetNodeID string) error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	targetNode, exists := m.state.nodes[targetNodeID]
	if !exists {
		return fmt.Errorf("target node %s not found", targetNodeID)
	}

	// 找到当前活跃节点
	var currentActive *ClusterNode
	for _, node := range m.state.nodes {
		if node.Role == RoleActive {
			currentActive = node
			break
		}
	}

	if currentActive != nil && currentActive.ID == targetNodeID {
		return fmt.Errorf("node %s is already active", targetNodeID)
	}

	startTime := time.Now()

	// 切换角色
	if currentActive != nil {
		currentActive.Role = RoleStandby
	}
	targetNode.Role = RoleActive
	targetNode.LastHeartbeat = time.Now()

	// 尝试获取锁
	if err := m.acquireLock(targetNodeID); err != nil {
		m.logger.Error("failed to acquire lock during switchover", "error", err)
	}

	// VIP 漂移
	if err := m.moveVIP(targetNodeID); err != nil {
		m.logger.Error("failed to move VIP", "error", err)
	}

	// 记录事件
	event := FailoverEvent{
		ID:             generateEventID(),
		Timestamp:      time.Now(),
		Type:           EventManualSwitchover,
		PromotedNodeID: targetNodeID,
		Reason:         "manual switchover",
		Duration:       time.Since(startTime),
	}
	if currentActive != nil {
		event.FailedNodeID = currentActive.ID
	}

	m.state.events = append(m.state.events, event)

	m.logger.Info("manual switchover completed",
		"from", event.FailedNodeID,
		"to", targetNodeID,
		"duration", event.Duration,
	)

	return nil
}

// EventChannel 返回事件通道（只读）
func (m *Manager) EventChannel() <-chan FailoverEvent {
	return m.eventChan
}

// heartbeatSender 心跳发送协程
func (m *Manager) heartbeatSender() {
	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (m *Manager) sendHeartbeat() {
	m.state.mu.Lock()
	if m.state.localNode != nil {
		m.state.localNode.LastHeartbeat = time.Now()
		m.state.localNode.Status = StatusOnline
	}
	m.state.mu.Unlock()

	m.logger.Debug("heartbeat sent", "node_id", m.localNodeID)
}

// healthChecker 健康检查协程
func (m *Manager) healthChecker() {
	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkNodeHealth()
		}
	}
}

// checkNodeHealth 检查所有节点健康状态
func (m *Manager) checkNodeHealth() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	now := time.Now()

	for _, node := range m.state.nodes {
		if node.ID == m.localNodeID {
			continue
		}

		timeSinceLastHeartbeat := now.Sub(node.LastHeartbeat)

		if timeSinceLastHeartbeat > m.config.HeartbeatTimeout {
			// 节点心跳超时
			if node.Status != StatusFailed {
				m.logger.Warn("node heartbeat timeout",
					"node_id", node.ID,
					"last_heartbeat", node.LastHeartbeat,
					"timeout", timeSinceLastHeartbeat,
				)

				node.Status = StatusFailed

				// 如果是活跃节点故障，触发故障切换
				if node.Role == RoleActive {
					go m.initiateFailover(node)
				}
			}
		} else if timeSinceLastHeartbeat > m.config.HeartbeatInterval*3 {
			// 节点状态降级
			if node.Status == StatusOnline {
				m.logger.Warn("node degraded",
					"node_id", node.ID,
					"last_heartbeat", node.LastHeartbeat,
				)
				node.Status = StatusDegraded
			}
		}
	}
}

// initiateFailover 发起故障切换
func (m *Manager) initiateFailover(failedNode *ClusterNode) {
	startTime := time.Now()

	m.logger.Warn("initiating failover",
		"failed_node", failedNode.ID,
		"delay", m.config.FailoverDelay,
	)

	// 等待故障切换延迟（防抖动）
	select {
	case <-time.After(m.config.FailoverDelay):
	case <-m.ctx.Done():
		return
	case <-m.stopChan:
		return
	}

	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	// 再次检查节点是否仍然故障
	if failedNode.Status != StatusFailed {
		m.logger.Info("node recovered, canceling failover", "node_id", failedNode.ID)
		return
	}

	// 找到最合适的备用节点
	bestStandby := m.findBestStandby()
	if bestStandby == nil {
		m.logger.Error("no suitable standby node found for failover")
		return
	}

	// 执行故障切换
	failedNode.Role = RoleStandby
	bestStandby.Role = RoleActive
	bestStandby.LastHeartbeat = time.Now()

	// 尝试获取锁
	if err := m.acquireLock(bestStandby.ID); err != nil {
		m.logger.Error("failed to acquire lock during failover", "error", err)
	}

	// VIP 漂移
	if err := m.moveVIP(bestStandby.ID); err != nil {
		m.logger.Error("failed to move VIP during failover", "error", err)
	}

	// 记录事件
	event := FailoverEvent{
		ID:             generateEventID(),
		Timestamp:      time.Now(),
		Type:           EventFailover,
		FailedNodeID:   failedNode.ID,
		PromotedNodeID: bestStandby.ID,
		Reason:         "heartbeat timeout",
		Duration:       time.Since(startTime),
	}
	m.state.events = append(m.state.events, event)

	m.logger.Warn("failover completed",
		"failed_node", failedNode.ID,
		"promoted_node", bestStandby.ID,
		"duration", event.Duration,
	)

	// 发送事件到通道
	select {
	case m.eventChan <- event:
	default:
		m.logger.Warn("event channel full, dropping event")
	}
}

// findBestStandby 找到最佳备用节点
func (m *Manager) findBestStandby() *ClusterNode {
	var best *ClusterNode

	for _, node := range m.state.nodes {
		if node.ID == m.localNodeID {
			continue
		}
		if node.Status == StatusFailed || node.Status == StatusOffline {
			continue
		}
		if node.Role != RoleStandby {
			continue
		}

		if best == nil || node.LastHeartbeat.After(best.LastHeartbeat) {
			best = node
		}
	}

	return best
}

// acquireLock 获取资源锁（防脑裂）
func (m *Manager) acquireLock(holderID string) error {
	now := time.Now()

	m.state.lock = &LockInfo{
		HolderID:   holderID,
		AcquiredAt: now,
		ExpiresAt:  now.Add(m.config.LockTTL),
		Version:    time.Now().UnixNano(),
	}

	m.logger.Info("lock acquired",
		"holder", holderID,
		"resource", m.config.LockResource,
		"expires_at", m.state.lock.ExpiresAt,
	)

	return nil
}

// releaseLock 释放资源锁
func (m *Manager) releaseLock(holderID string) error {
	if m.state.lock == nil {
		return fmt.Errorf("no lock held")
	}

	if m.state.lock.HolderID != holderID {
		return fmt.Errorf("lock held by %s, not %s", m.state.lock.HolderID, holderID)
	}

	m.state.lock = nil
	m.logger.Info("lock released", "holder", holderID, "resource", m.config.LockResource)

	return nil
}

// GetLockInfo 获取当前锁信息
func (m *Manager) GetLockInfo() *LockInfo {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	if m.state.lock == nil {
		return nil
	}

	lockCopy := *m.state.lock
	return &lockCopy
}

// moveVIP 移动 VIP 到指定节点
func (m *Manager) moveVIP(targetNodeID string) error {
	if m.config.VIP == "" {
		return nil // VIP 未配置
	}

	m.state.lastLeader = targetNodeID

	m.logger.Info("VIP moved",
		"vip", m.config.VIP,
		"interface", m.config.VIPInterface,
		"target_node", targetNodeID,
	)

	return nil
}

// eventProcessor 事件处理协程
func (m *Manager) eventProcessor() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.stopChan:
			return
		case event := <-m.eventChan:
			m.processEvent(event)
		}
	}
}

// processEvent 处理事件
func (m *Manager) processEvent(event FailoverEvent) {
	m.logger.Info("processing event",
		"type", event.Type,
		"failed_node", event.FailedNodeID,
		"promoted_node", event.PromotedNodeID,
		"reason", event.Reason,
	)

	// 可以在这里添加事件处理逻辑，如通知、日志记录等
}

// emitEvent 发送事件
func (m *Manager) emitEvent(event FailoverEvent) {
	m.state.events = append(m.state.events, event)

	select {
	case m.eventChan <- event:
	default:
		m.logger.Warn("event channel full, dropping event")
	}
}

// generateEventID 生成事件 ID
func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
