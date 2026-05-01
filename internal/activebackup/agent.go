// Package activebackup 提供整机备份管理功能
package activebackup

import (
	"sync"
	"time"
)

// AgentManager Agent 连接管理器.
type AgentManager struct {
	mu           sync.RWMutex
	manager      *Manager
	connections  map[string]*AgentConnection // agentID -> connection
	quit         chan struct{}
	running      bool
	offlineAfter time.Duration // 超过此时间视为离线
}

// AgentConnection Agent 连接信息.
type AgentConnection struct {
	AgentID       string    `json:"agent_id"`       // Agent ID
	RemoteAddr    string    `json:"remote_addr"`    // 远程地址
	ConnectedAt   time.Time `json:"connected_at"`   // 连接时间
	LastHeartbeat time.Time `json:"last_heartbeat"` // 最后心跳
	BytesSent     uint64    `json:"bytes_sent"`     // 已发送字节数
	BytesReceived uint64    `json:"bytes_received"` // 已接收字节数
	IsActive      bool      `json:"is_active"`      // 是否活跃
}

// NewAgentManager 创建 Agent 连接管理器.
func NewAgentManager(mgr *Manager) *AgentManager {
	return &AgentManager{
		manager:      mgr,
		connections:  make(map[string]*AgentConnection),
		quit:         make(chan struct{}),
		offlineAfter: 5 * time.Minute,
	}
}

// Start 启动 Agent 管理器.
func (am *AgentManager) Start() {
	am.mu.Lock()
	if am.running {
		am.mu.Unlock()
		return
	}
	am.running = true
	am.mu.Unlock()

	go am.checkLoop()
}

// Stop 停止 Agent 管理器.
func (am *AgentManager) Stop() {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.running {
		return
	}

	am.running = false
	close(am.quit)
	am.quit = make(chan struct{})
}

// IsRunning 是否运行中.
func (am *AgentManager) IsRunning() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.running
}

// checkLoop 离线检测循环.
func (am *AgentManager) checkLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-am.quit:
			return
		case <-ticker.C:
			am.checkOfflineAgents()
		}
	}
}

// checkOfflineAgents 检查离线 Agent.
func (am *AgentManager) checkOfflineAgents() {
	am.manager.mu.Lock()
	defer am.manager.mu.Unlock()

	now := time.Now()
	for _, agent := range am.manager.agents {
		if now.Sub(agent.LastSeen) > am.offlineAfter {
			agent.Status = AgentStatusOffline
		}
	}
}

// RegisterConnection 注册连接.
func (am *AgentManager) RegisterConnection(agentID, remoteAddr string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.connections[agentID] = &AgentConnection{
		AgentID:       agentID,
		RemoteAddr:    remoteAddr,
		ConnectedAt:   time.Now(),
		LastHeartbeat: time.Now(),
		IsActive:      true,
	}
}

// UnregisterConnection 注销连接.
func (am *AgentManager) UnregisterConnection(agentID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.connections, agentID)
}

// UpdateHeartbeat 更新心跳.
func (am *AgentManager) UpdateHeartbeat(agentID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	conn, exists := am.connections[agentID]
	if exists {
		conn.LastHeartbeat = time.Now()
		conn.IsActive = true
	}
}

// GetConnection 获取连接信息.
func (am *AgentManager) GetConnection(agentID string) *AgentConnection {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return am.connections[agentID]
}

// ListConnections 列出所有连接.
func (am *AgentManager) ListConnections() []*AgentConnection {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*AgentConnection, 0, len(am.connections))
	for _, c := range am.connections {
		result = append(result, c)
	}
	return result
}

// GetOnlineCount 获取在线连接数.
func (am *AgentManager) GetOnlineCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()

	count := 0
	for _, c := range am.connections {
		if c.IsActive && time.Since(c.LastHeartbeat) < am.offlineAfter {
			count++
		}
	}
	return count
}

// AddBytesSent 记录发送字节.
func (am *AgentManager) AddBytesSent(agentID string, bytes uint64) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if conn, exists := am.connections[agentID]; exists {
		conn.BytesSent += bytes
	}
}

// AddBytesReceived 记录接收字节.
func (am *AgentManager) AddBytesReceived(agentID string, bytes uint64) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if conn, exists := am.connections[agentID]; exists {
		conn.BytesReceived += bytes
	}
}
