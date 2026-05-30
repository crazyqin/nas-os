// Package tailscale 提供 Tailscale VPN 零配置组网功能
// 管理器实现，负责业务逻辑和状态管理
package tailscale

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager Tailscale 管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	status      *TailscaleStatus
	nodes       map[string]*TailscaleNode
	aclPolicy   *ACLPolicy
	subnets     map[string]*SubnetRoute
	exitNodes   map[string]*ExitNode
	dnsConfig   *DNSConfig
	authKeys    map[string]*AuthKey
	stats       *TrafficStats
	stopCh      chan struct{}
	running     bool
	collectInterval time.Duration
}

// NewManager 创建 Tailscale 管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:          logger,
		nodes:           make(map[string]*TailscaleNode),
		subnets:         make(map[string]*SubnetRoute),
		exitNodes:       make(map[string]*ExitNode),
		authKeys:        make(map[string]*AuthKey),
		stopCh:          make(chan struct{}),
		collectInterval: 30 * time.Second,
	}

	// 初始化默认配置
	m.initDefaults()

	return m
}

// initDefaults 初始化默认配置
func (m *Manager) initDefaults() {
	// 默认状态
	m.status = &TailscaleStatus{
		Connected:   true,
		NodeID:      "node_local",
		HostName:    "nas-server",
		TailnetName: "my-tailnet.ts.net",
		Version:     "1.52.0",
		IPv4:        "100.64.0.1",
		IPv6:        "fd7a:115c:a1e0::1",
		PublicKey:   "pubkey_local_xxxxx",
		OS:          "linux",
		Online:      true,
		LastSeen:    time.Now(),
		StartedAt:   time.Now().Add(-24 * time.Hour),
	}

	// 示例节点
	m.nodes["node_001"] = &TailscaleNode{
		ID:       "node_001",
		HostName: "laptop-01",
		IPv4:     "100.64.0.2",
		IPv6:     "fd7a:115c:a1e0::2",
		OS:       "windows",
		Online:   true,
		LastSeen: time.Now(),
		Tags:     []string{"staff", "engineering"},
		Approved: true,
		ExitNode: false,
	}

	m.nodes["node_002"] = &TailscaleNode{
		ID:       "node_002",
		HostName: "phone-01",
		IPv4:     "100.64.0.3",
		IPv6:     "fd7a:115c:a1e0::3",
		OS:       "ios",
		Online:   false,
		LastSeen: time.Now().Add(-2 * time.Hour),
		Tags:     []string{"mobile"},
		Approved: true,
		ExitNode: false,
	}

	m.nodes["node_003"] = &TailscaleNode{
		ID:       "node_003",
		HostName: "pending-device",
		IPv4:     "100.64.0.4",
		IPv6:     "fd7a:115c:a1e0::4",
		OS:       "linux",
		Online:   true,
		LastSeen: time.Now(),
		Tags:     []string{},
		Approved: false,
		ExitNode: false,
	}

	// 默认 ACL 策略
	m.aclPolicy = &ACLPolicy{
		Version: 1,
		ACLs: []ACLRule{
			{
				Sources:      []string{"staff"},
				Destinations: []string{"*:*"},
				Action:       "accept",
				Description:  "允许员工访问所有资源",
			},
			{
				Sources:      []string{"mobile"},
				Destinations: []string{"tag:server:*"},
				Action:       "accept",
				Description:  "允许移动设备访问服务器",
			},
		},
		UpdatedAt: time.Now(),
	}

	// 子网路由
	m.subnets["subnet_001"] = &SubnetRoute{
		ID:         "subnet_001",
		CIDR:       "192.168.1.0/24",
		NodeID:     "node_001",
		Enabled:    true,
		Advertised: true,
	}

	// 出口节点
	m.exitNodes["node_001"] = &ExitNode{
		ID:        "node_001",
		IP:        "100.64.0.2",
		HostName:  "laptop-01",
		IsCurrent: false,
		Latency:   15,
		Online:    true,
		Country:   "CN",
	}

	// DNS 配置
	m.dnsConfig = &DNSConfig{
		MagicDNSEnabled: true,
		Domains:         []string{"home.local", "nas.local"},
		Nameservers:     []string{"100.100.100.100", "8.8.8.8"},
		SearchDomains:   []string{"my-tailnet.ts.net"},
	}

	// 流量统计
	m.stats = &TrafficStats{
		InboundBytes:  1024 * 1024 * 100,  // 100MB
		OutboundBytes: 1024 * 1024 * 50,   // 50MB
		Connections:   5,
		ActivePeers:   3,
		Latency:       20,
		PacketLoss:    0.1,
		Timestamp:     time.Now(),
	}

	// 认证密钥
	m.authKeys["key_001"] = &AuthKey{
		ID:          "key_001",
		Key:         "tskey-auth-xxxxx",
		Description: "开发环境测试密钥",
		CreatedAt:   time.Now().Add(-7 * 24 * time.Hour),
		ExpiresAt:   timePtr(time.Now().Add(30 * 24 * time.Hour)),
		Reusable:    true,
		Ephemeral:   false,
		Revoked:     false,
		UsedCount:   3,
	}
}

// timePtr 辅助函数，返回时间指针
func timePtr(t time.Time) *time.Time {
	return &t
}

// Start 启动定时采集
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go func() {
		// 立即采集一次
		m.collect()

		ticker := time.NewTicker(m.collectInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collect()
			case <-m.stopCh:
				return
			}
		}
	}()

	m.logger.Info("[Tailscale] 启动定时采集", zap.Duration("interval", m.collectInterval))
}

// Stop 停止定时采集
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopCh)
	m.logger.Info("[Tailscale] 停止定时采集")
}

// collect 采集状态和统计数据
func (m *Manager) collect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 更新最后在线时间
	if m.status != nil && m.status.Online {
		m.status.LastSeen = now
	}

	// 模拟更新统计
	if m.stats != nil {
		m.stats.InboundBytes += 1024 * 10 // 每次采集增加 10KB
		m.stats.OutboundBytes += 1024 * 5
		m.stats.Timestamp = now
		m.stats.Latency = 15 + int(now.Second()%10) // 模拟延迟波动
	}

	// 检查节点在线状态
	for _, node := range m.nodes {
		if now.Sub(node.LastSeen) > 5*time.Minute {
			node.Online = false
		}
	}
}

// ========== 状态查询 ==========

// GetStatus 获取 Tailscale 状态
func (m *Manager) GetStatus() *TailscaleStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.status == nil {
		return &TailscaleStatus{}
	}
	// 返回副本
	status := *m.status
	return &status
}

// ========== 节点管理 ==========

// GetNodes 获取所有节点列表
func (m *Manager) GetNodes() []TailscaleNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]TailscaleNode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, *node)
	}
	return nodes
}

// GetNode 获取节点详情
func (m *Manager) GetNode(id string) (*TailscaleNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("节点不存在: %s", id)
	}

	// 返回副本
	result := *node
	return &result, nil
}

// ApproveNode 批准节点
func (m *Manager) ApproveNode(id string, approved bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("节点不存在: %s", id)
	}

	node.Approved = approved
	m.logger.Info("[Tailscale] 批准节点",
		zap.String("nodeId", id),
		zap.Bool("approved", approved))

	return nil
}

// ========== ACL 策略 ==========

// GetACL 获取 ACL 策略
func (m *Manager) GetACL() *ACLPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.aclPolicy == nil {
		return &ACLPolicy{}
	}
	// 返回副本
	policy := *m.aclPolicy
	rules := make([]ACLRule, len(policy.ACLs))
	copy(rules, policy.ACLs)
	policy.ACLs = rules
	return &policy
}

// UpdateACL 更新 ACL 策略
func (m *Manager) UpdateACL(rules []ACLRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.aclPolicy == nil {
		m.aclPolicy = &ACLPolicy{}
	}

	m.aclPolicy.Version++
	m.aclPolicy.ACLs = rules
	m.aclPolicy.UpdatedAt = time.Now()

	m.logger.Info("[Tailscale] 更新 ACL 策略",
		zap.Int("version", m.aclPolicy.Version),
		zap.Int("rules", len(rules)))

	return nil
}

// ========== 子网路由 ==========

// GetSubnets 获取子网路由列表
func (m *Manager) GetSubnets() []SubnetRoute {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subnets := make([]SubnetRoute, 0, len(m.subnets))
	for _, subnet := range m.subnets {
		subnets = append(subnets, *subnet)
	}
	return subnets
}

// AddSubnet 添加子网路由
func (m *Manager) AddSubnet(cidr, nodeID string) (*SubnetRoute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证节点存在
	if _, ok := m.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}

	id := fmt.Sprintf("subnet_%d", time.Now().UnixNano())
	route := &SubnetRoute{
		ID:         id,
		CIDR:       cidr,
		NodeID:     nodeID,
		Enabled:    true,
		Advertised: true,
	}

	m.subnets[id] = route

	m.logger.Info("[Tailscale] 添加子网路由",
		zap.String("id", id),
		zap.String("cidr", cidr),
		zap.String("nodeId", nodeID))

	return route, nil
}

// ToggleSubnet 切换子网路由启用状态
func (m *Manager) ToggleSubnet(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subnet, ok := m.subnets[id]
	if !ok {
		return fmt.Errorf("子网路由不存在: %s", id)
	}

	subnet.Enabled = enabled

	m.logger.Info("[Tailscale] 切换子网路由",
		zap.String("id", id),
		zap.Bool("enabled", enabled))

	return nil
}

// ========== Exit Node ==========

// GetExitNodes 获取出口节点列表
func (m *Manager) GetExitNodes() []ExitNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]ExitNode, 0, len(m.exitNodes))
	for _, node := range m.exitNodes {
		nodes = append(nodes, *node)
	}
	return nodes
}

// SelectExitNode 选择出口节点
func (m *Manager) SelectExitNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exitNode, ok := m.exitNodes[nodeID]
	if !ok {
		return fmt.Errorf("出口节点不存在: %s", nodeID)
	}

	if !exitNode.Online {
		return fmt.Errorf("出口节点离线: %s", nodeID)
	}

	// 取消其他出口节点
	for _, node := range m.exitNodes {
		node.IsCurrent = false
	}

	// 选择当前出口节点
	exitNode.IsCurrent = true

	m.logger.Info("[Tailscale] 选择出口节点",
		zap.String("nodeId", nodeID),
		zap.String("hostName", exitNode.HostName))

	return nil
}

// DeselectExitNode 取消出口节点选择
func (m *Manager) DeselectExitNode() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, node := range m.exitNodes {
		node.IsCurrent = false
	}

	m.logger.Info("[Tailscale] 取消出口节点选择")
	return nil
}

// ========== DNS 配置 ==========

// GetDNS 获取 DNS 配置
func (m *Manager) GetDNS() *DNSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.dnsConfig == nil {
		return &DNSConfig{}
	}
	// 返回副本
	config := *m.dnsConfig
	return &config
}

// UpdateDNS 更新 DNS 配置
func (m *Manager) UpdateDNS(req *UpdateDNSRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.dnsConfig == nil {
		m.dnsConfig = &DNSConfig{}
	}

	if req.MagicDNSEnabled != nil {
		m.dnsConfig.MagicDNSEnabled = *req.MagicDNSEnabled
	}
	if req.Domains != nil {
		m.dnsConfig.Domains = req.Domains
	}
	if req.Nameservers != nil {
		m.dnsConfig.Nameservers = req.Nameservers
	}

	m.logger.Info("[Tailscale] 更新 DNS 配置",
		zap.Bool("magicDns", m.dnsConfig.MagicDNSEnabled),
		zap.Strings("domains", m.dnsConfig.Domains))

	return nil
}

// ========== 认证密钥 ==========

// GetAuthKeys 获取认证密钥列表
func (m *Manager) GetAuthKeys() []AuthKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]AuthKey, 0, len(m.authKeys))
	for _, key := range m.authKeys {
		keys = append(keys, *key)
	}
	return keys
}

// CreateAuthKey 创建认证密钥
func (m *Manager) CreateAuthKey(req *CreateAuthKeyRequest) (*AuthKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("key_%d", time.Now().UnixNano())
	key := &AuthKey{
		ID:          id,
		Key:         fmt.Sprintf("tskey-auth-%s", id),
		Description: req.Description,
		CreatedAt:   time.Now(),
		ExpiresAt:   req.ExpiresAt,
		Reusable:    req.Reusable,
		Ephemeral:   req.Ephemeral,
		Revoked:     false,
		UsedCount:   0,
	}

	m.authKeys[id] = key

	m.logger.Info("[Tailscale] 创建认证密钥",
		zap.String("id", id),
		zap.String("description", req.Description))

	return key, nil
}

// RevokeAuthKey 撤销认证密钥
func (m *Manager) RevokeAuthKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.authKeys[id]
	if !ok {
		return fmt.Errorf("认证密钥不存在: %s", id)
	}

	if key.Revoked {
		return fmt.Errorf("认证密钥已撤销: %s", id)
	}

	key.Revoked = true

	m.logger.Info("[Tailscale] 撤销认证密钥", zap.String("id", id))
	return nil
}

// ========== 流量统计 ==========

// GetStats 获取流量统计
func (m *Manager) GetStats() *TrafficStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stats == nil {
		return &TrafficStats{}
	}
	// 返回副本
	stats := *m.stats
	return &stats
}
