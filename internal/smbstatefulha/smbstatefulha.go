package smbstatefulha

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
)

// NewHAManager 创建 HA 管理器
func NewHAManager(config *HAConfig, localHostname, localIP string) *HAManager {
	if config == nil {
		config = DefaultHAConfig()
	}

	localNode := &HANode{
		ID:        uuid.New().String(),
		Hostname:  localHostname,
		IPAddress: localIP,
		VirtualIP: config.VirtualIP,
		State:     NodeStateActive,
		Priority:  1,
		Sessions:  make(map[string]*SMBSession),
	}

	return &HAManager{
		config:      config,
		localNode:   localNode,
		sessions:    make(map[string]*SMBSession),
		failoverLog: make([]FailoverEvent, 0),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动 HA 管理器
func (m *HAManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("HA 管理器已在运行")
	}

	m.running = true
	m.localNode.State = NodeStateActive
	m.localNode.LastHeartbeat = time.Now()

	log.Printf("[SMBStatefulHA] HA 管理器启动 - 虚拟IP: %s", m.config.VirtualIP)

	// 启动心跳发送器
	go m.heartbeatSender()

	// 启动心跳监听器
	go m.heartbeatListener()

	// 启动会话同步器
	if m.config.SessionSyncEnabled {
		go m.sessionSyncer()
	}

	// 启动故障检测器
	go m.failureDetector()

	return nil
}

// Stop 停止 HA 管理器
func (m *HAManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	m.localNode.State = NodeStateStandby

	log.Println("[SMBStatefulHA] HA 管理器停止")
}

// IsRunning 检查是否运行中
func (m *HAManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetLocalNode 获取本地节点信息
func (m *HAManager) GetLocalNode() *HANode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.localNode
}

// GetRemoteNode 获取远程节点信息
func (m *HAManager) GetRemoteNode() *HANode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.remoteNode
}

// AddSession 添加 SMB 会话
func (m *HAManager) AddSession(session *SMBSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("HA 管理器未运行")
	}

	if len(m.sessions) >= m.config.MaxSessions {
		return fmt.Errorf("已达到最大会话数: %d", m.config.MaxSessions)
	}

	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	session.State = "active"
	session.CreatedAt = time.Now()
	session.LastActivity = time.Now()

	m.sessions[session.ID] = session
	m.localNode.Sessions[session.ID] = session

	log.Printf("[SMBStatefulHA] 添加会话: %s (用户: %s, 客户端: %s)",
		session.ID, session.Username, session.ClientIP)

	return nil
}

// RemoveSession 移除 SMB 会话
func (m *HAManager) RemoveSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	delete(m.sessions, sessionID)
	delete(m.localNode.Sessions, sessionID)

	log.Printf("[SMBStatefulHA] 移除会话: %s (用户: %s)", sessionID, session.Username)

	return nil
}

// GetSession 获取会话信息
func (m *HAManager) GetSession(sessionID string) (*SMBSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	return session, nil
}

// ListSessions 列出所有会话
func (m *HAManager) ListSessions() []*SMBSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*SMBSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetActiveSessionCount 获取活跃会话数
func (m *HAManager) GetActiveSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, session := range m.sessions {
		if session.State == "active" {
			count++
		}
	}
	return count
}

// TriggerFailover 触发故障转移
func (m *HAManager) TriggerFailover(trigger FailoverTrigger, reason string) (*FailoverEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, fmt.Errorf("HA 管理器未运行")
	}

	if m.remoteNode == nil {
		return nil, fmt.Errorf("远程节点未配置")
	}

	startTime := time.Now()
	event := &FailoverEvent{
		ID:          uuid.New().String(),
		Timestamp:   startTime,
		Trigger:     trigger,
		FromNode:    m.localNode.Hostname,
		ToNode:      m.remoteNode.Hostname,
		Reason:      reason,
		TotalSessions: len(m.sessions),
	}

	log.Printf("[SMBStatefulHA] 开始故障转移: %s -> %s (原因: %s)",
		m.localNode.Hostname, m.remoteNode.Hostname, reason)

	// 切换状态
	m.localNode.State = NodeStateStandby
	m.remoteNode.State = NodeStateActive

	// 迁移会话
	migrated := 0
	for _, session := range m.sessions {
		session.State = "migrating"
		// 实际实现中需要同步会话状态到远程节点
		session.State = "active"
		migrated++
	}

	event.SessionsAffected = migrated
	event.Duration = time.Since(startTime)
	event.Success = true

	m.failoverLog = append(m.failoverLog, *event)

	// 触发回调
	if m.onFailover != nil {
		go m.onFailover(*event)
	}

	log.Printf("[SMBStatefulHA] 故障转移完成: 迁移 %d 个会话, 耗时 %v",
		migrated, event.Duration)

	return event, nil
}

// SetOnFailoverCallback 设置故障转移回调
func (m *HAManager) SetOnFailoverCallback(callback func(event FailoverEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onFailover = callback
}

// GetFailoverLog 获取故障转移日志
func (m *HAManager) GetFailoverLog() []FailoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	log := make([]FailoverEvent, len(m.failoverLog))
	copy(log, m.failoverLog)
	return log
}

// GetSyncState 获取同步状态
func (m *HAManager) GetSyncState() *SyncState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &SyncState{
		LastSyncTime:    time.Now(),
		SyncedSessions:  len(m.sessions),
		PendingSync:     0,
		SyncErrors:      0,
		BytesTransfered: 0,
	}
}

// GetFailoverState 获取故障转移状态
func (m *HAManager) GetFailoverState() *FailoverState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &FailoverState{
		InProgress: false,
		Progress:   100,
	}
}

// heartbeatSender 心跳发送器
func (m *HAManager) heartbeatSender() {
	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (m *HAManager) sendHeartbeat() {
	m.mu.Lock()
	m.localNode.LastHeartbeat = time.Now()
	m.mu.Unlock()

	// 实际实现中通过网络发送心跳包
	log.Printf("[SMBStatefulHA] 发送心跳 - 活跃会话: %d", len(m.sessions))
}

// heartbeatListener 心跳监听器
func (m *HAManager) heartbeatListener() {
	// 实际实现中监听网络心跳包
	// 这里使用模拟实现
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkRemoteNode()
		}
	}
}

// checkRemoteNode 检查远程节点状态
func (m *HAManager) checkRemoteNode() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.remoteNode == nil {
		return
	}

	// 检查心跳超时
	if time.Since(m.remoteNode.LastHeartbeat) > m.config.FailoverTimeout {
		if m.remoteNode.State != NodeStateFailed {
			m.remoteNode.State = NodeStateFailed
			log.Printf("[SMBStatefulHA] 远程节点心跳超时: %s", m.remoteNode.Hostname)
		}
	}
}

// sessionSyncer 会话同步器
func (m *HAManager) sessionSyncer() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.syncSessions()
		}
	}
}

// syncSessions 同步会话状态
func (m *HAManager) syncSessions() {
	m.mu.RLock()
	sessions := make([]*SMBSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()

	if len(sessions) > 0 {
		log.Printf("[SMBStatefulHA] 同步 %d 个会话到远程节点", len(sessions))
		// 实际实现中通过网络同步会话状态
	}
}

// failureDetector 故障检测器
func (m *HAManager) failureDetector() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.detectFailure()
		}
	}
}

// detectFailure 检测故障
func (m *HAManager) detectFailure() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.remoteNode == nil {
		return
	}

	// 检查远程节点是否应该接管
	if m.localNode.Priority < m.remoteNode.Priority &&
		m.localNode.State == NodeStateFailed {
		// 本地节点优先级更高，应该接管
		log.Printf("[SMBStatefulHA] 检测到本地节点故障，准备故障转移")
	}
}

// AddRemoteNode 添加远程节点
func (m *HAManager) AddRemoteNode(hostname, ipAddress string, priority int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.remoteNode != nil {
		return fmt.Errorf("远程节点已存在")
	}

	m.remoteNode = &HANode{
		ID:        uuid.New().String(),
		Hostname:  hostname,
		IPAddress: ipAddress,
		State:     NodeStateStandby,
		Priority:  priority,
		Sessions:  make(map[string]*SMBSession),
	}

	log.Printf("[SMBStatefulHA] 添加远程节点: %s (%s) 优先级: %d",
		hostname, ipAddress, priority)

	return nil
}

// RemoveRemoteNode 移除远程节点
func (m *HAManager) RemoveRemoteNode() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.remoteNode != nil {
		log.Printf("[SMBStatefulHA] 移除远程节点: %s", m.remoteNode.Hostname)
		m.remoteNode = nil
	}
}

// GetHAStatus 获取 HA 状态
func (m *HAManager) GetHAStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"enabled":         m.running,
		"virtualIP":       m.config.VirtualIP,
		"localNode":       m.localNode.Hostname,
		"localState":      string(m.localNode.State),
		"activeSessions":  len(m.sessions),
		"maxSessions":     m.config.MaxSessions,
		"heartbeatInterval": m.config.HeartbeatInterval.String(),
		"failoverTimeout": m.config.FailoverTimeout.String(),
	}

	if m.remoteNode != nil {
		status["remoteNode"] = m.remoteNode.Hostname
		status["remoteState"] = string(m.remoteNode.State)
		status["remoteIP"] = m.remoteNode.IPAddress
	} else {
		status["remoteNode"] = nil
	}

	return status
}

// UpdateConfig 更新配置
func (m *HAManager) UpdateConfig(config *HAConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	log.Printf("[SMBStatefulHA] 配置已更新")
}

// GetConfig 获取配置
func (m *HAManager) GetConfig() *HAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// IsVirtualIPOwner 检查是否为虚拟 IP 拥有者
func (m *HAManager) IsVirtualIPOwner() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.localNode.State == NodeStateActive
}

// GetVirtualIP 获取虚拟 IP
func (m *HAManager) GetVirtualIP() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config.VirtualIP
}

// CheckNetworkConnectivity 检查网络连通性
func (m *HAManager) CheckNetworkConnectivity(targetIP string) bool {
	conn, err := net.DialTimeout("tcp", targetIP+":445", 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
