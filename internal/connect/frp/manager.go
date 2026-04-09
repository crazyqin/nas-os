// Package frp provides FRP client implementation
// FRP客户端管理器 - 提供一键连接和状态监控API
package frp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ClientManager 客户端管理器
type ClientManager struct {
	clients     map[string]*Client
	nodeConfig  *FreeNodeConfig
	logger      *zap.Logger
	
	// 状态监控
	statusCache  map[string]*ClientStatusInfo
	statusMu     sync.RWMutex
	
	// 事件通道
	eventChan    chan ClientEvent
	
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// ClientStatusInfo 客户端状态信息
type ClientStatusInfo struct {
	ClientID    string         `json:"client_id"`
	NodeID      string         `json:"node_id"`
	NodeName    string         `json:"node_name"`
	Region      NodeRegion     `json:"region"`
	Status      string         `json:"status"` // connected, disconnected, connecting, error
	ConnectedAt time.Time      `json:"connected_at"`
	Uptime      string         `json:"uptime"`
	Stats       ClientStats    `json:"stats"`
	Tunnels     []TunnelStatus `json:"tunnels"`
	LastUpdate  time.Time      `json:"last_update"`
	Error       string         `json:"error,omitempty"`
}

// ClientEvent 客户端事件
type ClientEvent struct {
	Type      string    `json:"type"` // connected, disconnected, tunnel_started, tunnel_stopped, error
	ClientID  string    `json:"client_id"`
	NodeID    string     `json:"node_id"`
	TunnelID  string     `json:"tunnel_id,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
	Error     error      `json:"error,omitempty"`
}

// NewClientManager 创建客户端管理器
func NewClientManager(logger *zap.Logger) *ClientManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	mgr := &ClientManager{
		clients:     make(map[string]*Client),
		nodeConfig:  NewFreeNodeConfig(),
		logger:      logger,
		statusCache: make(map[string]*ClientStatusInfo),
		eventChan:   make(chan ClientEvent, 100),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// 启动状态监控
	mgr.wg.Add(1)
	go mgr.statusMonitorLoop()
	
	return mgr
}

// QuickConnect 一键连接（使用免费节点）
func (m *ClientManager) QuickConnect(config *QuickConnectConfig) (*QuickConnectResult, error) {
	// 选择节点
	var node *FreeNode
	if config.NodeID != "" {
		node = m.nodeConfig.GetNode(config.NodeID)
		if node == nil {
			return nil, fmt.Errorf("node not found: %s", config.NodeID)
		}
	} else {
		// 自动选择最优节点
		if config.Region != "" {
			node = m.nodeConfig.GetBestNode(config.Region)
		} else {
			node = m.nodeConfig.GetBestNode()
		}
		if node == nil {
			return nil, fmt.Errorf("no available node")
		}
	}
	
	// 创建客户端配置
	clientConfig := NodeToClientConfig(node)
	
	// 添加隧道配置
	tunnelName := config.TunnelName
	if tunnelName == "" {
		tunnelName = fmt.Sprintf("quick-%d", time.Now().Unix())
	}
	
	tunnelType := config.TunnelType
	if tunnelType == "" {
		tunnelType = TunnelTypeTCP
	}
	
	tunnel := TunnelConfig{
		ID:         generateTunnelID(),
		Name:       tunnelName,
		Type:       tunnelType,
		LocalIP:    "127.0.0.1",
		LocalPort:  config.LocalPort,
		RemotePort: config.RemotePort,
		Enabled:    true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	
	clientConfig.Tunnels = append(clientConfig.Tunnels, tunnel)
	
	// 创建客户端
	client, err := NewClient(clientConfig, m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	
	// 设置回调
	client.SetOnConnect(func() {
		m.eventChan <- ClientEvent{
			Type:      "connected",
			ClientID:  clientConfig.Common.ServerAddr,
			NodeID:    node.ID,
			Timestamp: time.Now(),
		}
	})
	
	client.SetOnDisconnect(func(err error) {
		m.eventChan <- ClientEvent{
			Type:      "disconnected",
			ClientID:  clientConfig.Common.ServerAddr,
			NodeID:    node.ID,
			Timestamp: time.Now(),
			Error:     err,
		}
	})
	
	// 启动客户端
	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("failed to start client: %w", err)
	}
	
	// 保存客户端
	clientID := node.ID
	m.mu.Lock()
	m.clients[clientID] = client
	m.mu.Unlock()
	
	// 构建结果
	result := &QuickConnectResult{
		Success:   true,
		Node:      node,
		TunnelID:  tunnel.ID,
		ConnectAt: time.Now(),
	}
	
	// 构建公网URL
	if tunnel.Type == TunnelTypeHTTP || tunnel.Type == TunnelTypeHTTPS {
		// HTTP隧道使用子域名
		result.PublicURL = fmt.Sprintf("https://%s.%s", tunnelName, node.ServerAddr)
	} else if tunnel.RemotePort > 0 {
		result.PublicURL = fmt.Sprintf("%s:%d", node.ServerAddr, tunnel.RemotePort)
	}
	
	return result, nil
}

// Disconnect 断开指定客户端
func (m *ClientManager) Disconnect(clientID string) error {
	m.mu.Lock()
	client, exists := m.clients[clientID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("client not found: %s", clientID)
	}
	delete(m.clients, clientID)
	m.mu.Unlock()
	
	if err := client.Stop(); err != nil {
		return err
	}
	
	// 清除状态缓存
	m.statusMu.Lock()
	delete(m.statusCache, clientID)
	m.statusMu.Unlock()
	
	return nil
}

// DisconnectAll 断开所有客户端
func (m *ClientManager) DisconnectAll() error {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for id, client := range m.clients {
		clients = append(clients, client)
		delete(m.clients, id)
	}
	m.mu.Unlock()
	
	var errs []error
	for _, client := range clients {
		if err := client.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	
	// 清除状态缓存
	m.statusMu.Lock()
	m.statusCache = make(map[string]*ClientStatusInfo)
	m.statusMu.Unlock()
	
	if len(errs) > 0 {
		return fmt.Errorf("errors during disconnect: %v", errs)
	}
	return nil
}

// GetClient 获取客户端
func (m *ClientManager) GetClient(clientID string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[clientID]
}

// GetAllClients 获取所有客户端
func (m *ClientManager) GetAllClients() map[string]*Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]*Client, len(m.clients))
	for id, client := range m.clients {
		result[id] = client
	}
	return result
}

// GetClientStatus 获取客户端状态
func (m *ClientManager) GetClientStatus(clientID string) *ClientStatusInfo {
	m.statusMu.RLock()
	status, exists := m.statusCache[clientID]
	m.statusMu.RUnlock()
	
	if exists {
		return status
	}
	
	// 实时获取状态
	m.mu.RLock()
	client, ok := m.clients[clientID]
	m.mu.RUnlock()
	
	if !ok {
		return nil
	}
	
	return m.buildClientStatus(clientID, client)
}

// GetAllClientStatus 获取所有客户端状态
func (m *ClientManager) GetAllClientStatus() []*ClientStatusInfo {
	m.mu.RLock()
	clients := make(map[string]*Client, len(m.clients))
	for id, client := range m.clients {
		clients[id] = client
	}
	m.mu.RUnlock()
	
	statuses := make([]*ClientStatusInfo, 0, len(clients))
	for id, client := range clients {
		status := m.buildClientStatus(id, client)
		if status != nil {
			statuses = append(statuses, status)
		}
	}
	
	return statuses
}

// buildClientStatus 构建客户端状态
func (m *ClientManager) buildClientStatus(clientID string, client *Client) *ClientStatusInfo {
	// 获取节点信息
	node := m.nodeConfig.GetNode(clientID)
	if node == nil {
		// 尝试从配置中提取服务器地址
		node = &FreeNode{
			ID:         clientID,
			ServerAddr: client.config.Common.ServerAddr,
			ServerPort: client.config.Common.ServerPort,
		}
	}
	
	status := &ClientStatusInfo{
		ClientID:   clientID,
		NodeID:     node.ID,
		NodeName:   node.Name,
		Region:     node.Region,
		Status:     client.GetStatus(),
		Stats:      client.GetStats(),
		Tunnels:    client.ListTunnelStatus(),
		LastUpdate: time.Now(),
	}
	
	if !status.Stats.ConnectedAt.IsZero() {
		status.ConnectedAt = status.Stats.ConnectedAt
		status.Uptime = status.Stats.Uptime
	}
	
	return status
}

// GetNodes 获取所有可用节点
func (m *ClientManager) GetNodes() []*FreeNode {
	return m.nodeConfig.GetAllNodes()
}

// GetNodesByRegion 获取指定区域的节点
func (m *ClientManager) GetNodesByRegion(region NodeRegion) []*FreeNode {
	return m.nodeConfig.GetNodesByRegion(region)
}

// GetBestNode 获取最优节点
func (m *ClientManager) GetBestNode(region ...NodeRegion) *FreeNode {
	if len(region) > 0 {
		return m.nodeConfig.GetBestNode(region[0])
	}
	return m.nodeConfig.GetBestNode()
}

// AddNode 添加自定义节点
func (m *ClientManager) AddNode(node *FreeNode) {
	m.nodeConfig.AddNode(node)
}

// RemoveNode 移除节点
func (m *ClientManager) RemoveNode(nodeID string) {
	m.nodeConfig.RemoveNode(nodeID)
}

// UpdateNodeStatus 更新节点状态
func (m *ClientManager) UpdateNodeStatus(nodeID string, online bool, latency int) {
	m.nodeConfig.UpdateNodeStatus(nodeID, online, latency)
}

// Events 返回事件通道
func (m *ClientManager) Events() <-chan ClientEvent {
	return m.eventChan
}

// statusMonitorLoop 状态监控循环
func (m *ClientManager) statusMonitorLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateAllStatus()
		}
	}
}

// updateAllStatus 更新所有客户端状态
func (m *ClientManager) updateAllStatus() {
	m.mu.RLock()
	clients := make(map[string]*Client, len(m.clients))
	for id, client := range m.clients {
		clients[id] = client
	}
	m.mu.RUnlock()
	
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	
	for id, client := range clients {
		status := m.buildClientStatus(id, client)
		if status != nil {
			m.statusCache[id] = status
		}
	}
}

// Close 关闭管理器
func (m *ClientManager) Close() error {
	m.cancel()
	m.wg.Wait()
	
	// 关闭所有客户端
	_ = m.DisconnectAll()
	
	close(m.eventChan)
	
	return nil
}

// HealthCheck 健康检查所有节点
func (m *ClientManager) HealthCheck(ctx context.Context) map[string]*NodeHealthResult {
	nodes := m.nodeConfig.GetAllNodes()
	results := make(map[string]*NodeHealthResult)
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for _, node := range nodes {
		wg.Add(1)
		go func(n *FreeNode) {
			defer wg.Done()
			
			result := &NodeHealthResult{
				NodeID:    n.ID,
				NodeName:  n.Name,
				Region:    n.Region,
				CheckTime: time.Now(),
			}
			
			// 执行连接测试
			start := time.Now()
			if err := m.checkNodeConnectivity(ctx, n); err != nil {
				result.Online = false
				result.Error = err.Error()
			} else {
				result.Online = true
				result.Latency = int(time.Since(start).Milliseconds())
			}
			
			mu.Lock()
			results[n.ID] = result
			mu.Unlock()
			
			// 更新节点状态
			m.nodeConfig.UpdateNodeStatus(n.ID, result.Online, result.Latency)
		}(node)
	}
	
	wg.Wait()
	return results
}

// NodeHealthResult 节点健康检查结果
type NodeHealthResult struct {
	NodeID    string     `json:"node_id"`
	NodeName  string     `json:"node_name"`
	Region    NodeRegion `json:"region"`
	Online    bool       `json:"online"`
	Latency   int        `json:"latency_ms"`
	Error     string     `json:"error,omitempty"`
	CheckTime time.Time  `json:"check_time"`
}

// checkNodeConnectivity 检查节点连通性
func (m *ClientManager) checkNodeConnectivity(ctx context.Context, node *FreeNode) error {
	// 简单的TCP连接测试
	addr := fmt.Sprintf("%s:%d", node.ServerAddr, node.ServerPort)
	
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	_ = conn.Close()
	
	return nil
}