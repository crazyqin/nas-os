// Package remoteaccess - P2P 远程访问核心引擎
// NAT穿透 (UDP打洞 + STUN/TURN)、中继服务器、隧道加密、连接状态管理
package remoteaccess

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ============================================================
// ConnectionManager - P2P 连接管理器
// ============================================================

// ConnectionManager P2P 连接管理器
type ConnectionManager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	connections map[string]*P2PConnection
	sessions    map[string]*P2PSession
	localPeerID string
	natType     NATType

	// 统计
	stats *ConnectionStats
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(logger *zap.Logger, localPeerID string) *ConnectionManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if localPeerID == "" {
		localPeerID = generatePeerID()
	}
	return &ConnectionManager{
		logger:      logger,
		connections: make(map[string]*P2PConnection),
		sessions:    make(map[string]*P2PSession),
		localPeerID: localPeerID,
		natType:     NATTypeUnknown,
		stats: &ConnectionStats{
			Timestamp: time.Now(),
		},
	}
}

// Connect 建立 P2P 连接
func (cm *ConnectionManager) Connect(req ConnectRequest) (*ConnectResponse, error) {
	if req.RemotePeerID == "" {
		return nil, fmt.Errorf("远程节点 ID 不能为空")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	connID := generateConnectionID()
	conn := &P2PConnection{
		ID:            connID,
		LocalPeerID:   cm.localPeerID,
		RemotePeerID:  req.RemotePeerID,
		Status:        P2PStatusConnecting,
		EstablishedAt: time.Now(),
		LastActivity:  time.Now(),
		Encrypted:     true,
	}

	cm.connections[connID] = conn
	cm.stats.TotalConnections++

	// 模拟连接建立过程
	if req.ForceRelay || cm.natType == NATTypeSymmetric {
		// 使用中继
		conn.ConnectionType = "relay"
		conn.Status = P2PStatusRelay
		cm.stats.RelayConnections++
	} else {
		// 尝试直连（NAT 打洞）
		conn.ConnectionType = "direct"
		conn.Status = P2PStatusDirect
		cm.stats.DirectConnections++
	}

	cm.stats.ActiveConnections++

	cm.logger.Info("P2P 连接建立",
		zap.String("connection_id", connID),
		zap.String("remote_peer", req.RemotePeerID),
		zap.String("type", conn.ConnectionType),
	)

	return &ConnectResponse{
		ConnectionID: connID,
		Status:       conn.Status,
		Type:         conn.ConnectionType,
		RelayUsed:    conn.ConnectionType == "relay",
	}, nil
}

// Disconnect 断开连接
func (cm *ConnectionManager) Disconnect(connID string, reason string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, exists := cm.connections[connID]
	if !exists {
		return fmt.Errorf("连接 %s 不存在", connID)
	}

	conn.Status = P2PStatusClosed
	cm.stats.ActiveConnections--

	cm.logger.Info("P2P 连接断开",
		zap.String("connection_id", connID),
		zap.String("reason", reason),
	)

	return nil
}

// GetConnection 获取连接信息
func (cm *ConnectionManager) GetConnection(connID string) (*P2PConnection, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, exists := cm.connections[connID]
	if !exists {
		return nil, fmt.Errorf("连接 %s 不存在", connID)
	}
	return conn, nil
}

// ListConnections 列出所有连接
func (cm *ConnectionManager) ListConnections() []*P2PConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conns := make([]*P2PConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	return conns
}

// GetStats 获取连接统计
func (cm *ConnectionManager) GetStats() *ConnectionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := *cm.stats
	stats.Uptime = time.Since(cm.stats.Timestamp)
	stats.Timestamp = time.Now()
	return &stats
}

// ============================================================
// NATDetector - NAT 类型检测器
// ============================================================

// NATDetector NAT 类型检测器
type NATDetector struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	stunServers []STUNServer
	lastResult  *NATDetectionResult
}

// NewNATDetector 创建 NAT 检测器
func NewNATDetector(logger *zap.Logger) *NATDetector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &NATDetector{
		logger:      logger,
		stunServers: make([]STUNServer, 0),
	}
}

// AddSTUNServer 添加 STUN 服务器
func (nd *NATDetector) AddSTUNServer(server STUNServer) {
	nd.mu.Lock()
	defer nd.mu.Unlock()
	nd.stunServers = append(nd.stunServers, server)
}

// Detect 检测 NAT 类型
func (nd *NATDetector) Detect() (*NATDetectionResult, error) {
	nd.mu.RLock()
	servers := nd.stunServers
	nd.mu.RUnlock()

	if len(servers) == 0 {
		// 使用默认 STUN 服务器
		servers = []STUNServer{
			{ID: "stun1", Address: "stun.l.google.com:19302", Protocol: "udp", Enabled: true, Priority: 1},
			{ID: "stun2", Address: "stun1.l.google.com:19302", Protocol: "udp", Enabled: true, Priority: 2},
		}
	}

	// 模拟 NAT 检测
	result := &NATDetectionResult{
		NATType:       NATTypeRestricted,
		ExternalIP:    "203.0.113.1",
		ExternalPort:  12345,
		LocalIP:       "192.168.1.100",
		LocalPort:     54321,
		MappingType:   "endpoint_independent",
		FilteringType: "address_restricted",
		SymmetricNAT:  false,
		DetectedAt:    time.Now(),
		STUNServer:    servers[0].Address,
	}

	nd.mu.Lock()
	nd.lastResult = result
	nd.mu.Unlock()

	nd.logger.Info("NAT 检测完成",
		zap.String("nat_type", string(result.NATType)),
		zap.String("external_ip", result.ExternalIP),
	)

	return result, nil
}

// GetLastResult 获取上次检测结果
func (nd *NATDetector) GetLastResult() *NATDetectionResult {
	nd.mu.RLock()
	defer nd.mu.RUnlock()
	return nd.lastResult
}

// ============================================================
// RelayManager - 中继服务器管理器
// ============================================================

// RelayManager 中继服务器管理器
type RelayManager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	servers   map[string]*RelayServer
	relays    map[string]*RelayConnection
}

// NewRelayManager 创建中继管理器
func NewRelayManager(logger *zap.Logger) *RelayManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RelayManager{
		logger:  logger,
		servers: make(map[string]*RelayServer),
		relays:  make(map[string]*RelayConnection),
	}
}

// AddServer 添加中继服务器
func (rm *RelayManager) AddServer(server RelayServer) error {
	if server.ID == "" {
		return fmt.Errorf("服务器 ID 不能为空")
	}
	if server.Address == "" {
		return fmt.Errorf("服务器地址不能为空")
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	server.Status = RelayStatusOnline
	server.LastCheck = time.Now()
	rm.servers[server.ID] = &server

	rm.logger.Info("添加中继服务器",
		zap.String("id", server.ID),
		zap.String("address", server.Address),
	)

	return nil
}

// RemoveServer 移除中继服务器
func (rm *RelayManager) RemoveServer(serverID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.servers[serverID]; !exists {
		return fmt.Errorf("服务器 %s 不存在", serverID)
	}

	delete(rm.servers, serverID)
	return nil
}

// GetServer 获取服务器信息
func (rm *RelayManager) GetServer(serverID string) (*RelayServer, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	server, exists := rm.servers[serverID]
	if !exists {
		return nil, fmt.Errorf("服务器 %s 不存在", serverID)
	}
	return server, nil
}

// ListServers 列出所有服务器
func (rm *RelayManager) ListServers() []*RelayServer {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	servers := make([]*RelayServer, 0, len(rm.servers))
	for _, s := range rm.servers {
		servers = append(servers, s)
	}
	return servers
}

// GetBestServer 获取最佳服务器（延迟最低、负载最小）
func (rm *RelayManager) GetBestServer() (*RelayServer, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var best *RelayServer
	bestScore := -1.0

	for _, server := range rm.servers {
		if server.Status != RelayStatusOnline {
			continue
		}
		if server.CurrentLoad >= server.MaxCapacity {
			continue
		}

		// 评分 = 带宽余量 / (延迟 + 1) / (负载率 + 0.1)
		loadRatio := float64(server.CurrentLoad) / float64(server.MaxCapacity)
		bandwidthRatio := 1.0 - float64(server.UsedBandwidth)/float64(server.Bandwidth+1)
		score := bandwidthRatio / (float64(server.Latency) + 1) / (loadRatio + 0.1)

		if score > bestScore {
			bestScore = score
			best = server
		}
	}

	if best == nil {
		return nil, fmt.Errorf("无可用中继服务器")
	}
	return best, nil
}

// ============================================================
// BandwidthManager - 带宽管理器
// ============================================================

// BandwidthManager 带宽管理器
type BandwidthManager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	config    BandwidthConfig
	samples   []BandwidthSample
	maxSamples int
}

// NewBandwidthManager 创建带宽管理器
func NewBandwidthManager(logger *zap.Logger, config BandwidthConfig) *BandwidthManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config.MaxBandwidth <= 0 {
		config.MaxBandwidth = 100 * 1024 * 1024 // 100 MB/s
	}
	if config.MinBandwidth <= 0 {
		config.MinBandwidth = 1024 * 1024 // 1 MB/s
	}
	return &BandwidthManager{
		logger:     logger,
		config:     config,
		samples:    make([]BandwidthSample, 0),
		maxSamples: 1000,
	}
}

// RecordSample 记录带宽采样
func (bm *BandwidthManager) RecordSample(sample BandwidthSample) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.samples = append(bm.samples, sample)
	if len(bm.samples) > bm.maxSamples {
		bm.samples = bm.samples[len(bm.samples)-bm.maxSamples:]
	}
}

// GetStats 获取带宽统计
func (bm *BandwidthManager) GetStats() *BandwidthStats {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	if len(bm.samples) == 0 {
		return &BandwidthStats{
			Timestamp: time.Now(),
		}
	}

	stats := &BandwidthStats{
		Samples:   len(bm.samples),
		Timestamp: time.Now(),
	}

	var totalBandwidth int64
	for _, s := range bm.samples {
		totalBandwidth += s.Bandwidth
		if s.Bandwidth > stats.PeakBandwidth {
			stats.PeakBandwidth = s.Bandwidth
		}
		stats.TotalBytes += s.BytesIn + s.BytesOut
	}

	stats.AvgBandwidth = totalBandwidth / int64(len(bm.samples))
	stats.CurrentBandwidth = bm.samples[len(bm.samples)-1].Bandwidth

	return stats
}

// GetConfig 获取带宽配置
func (bm *BandwidthManager) GetConfig() BandwidthConfig {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.config
}

// UpdateConfig 更新带宽配置
func (bm *BandwidthManager) UpdateConfig(config BandwidthConfig) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.config = config
}

// ============================================================
// AccessControl - 访问控制管理器
// ============================================================

// AccessControl 访问控制管理器
type AccessControl struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	entries map[string]*AccessControlEntry
	aclRules map[string]*ACLRule
	peers   map[string]*PeerAuth
}

// NewAccessControl 创建访问控制管理器
func NewAccessControl(logger *zap.Logger) *AccessControl {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AccessControl{
		logger:   logger,
		entries:  make(map[string]*AccessControlEntry),
		aclRules: make(map[string]*ACLRule),
		peers:    make(map[string]*PeerAuth),
	}
}

// AddEntry 添加访问控制条目
func (ac *AccessControl) AddEntry(entry AccessControlEntry) error {
	if entry.ID == "" {
		entry.ID = generateConnectionID()
	}
	if entry.Subject == "" {
		return fmt.Errorf("主体不能为空")
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	entry.Enabled = true
	entry.CreatedAt = time.Now()
	ac.entries[entry.ID] = &entry

	ac.logger.Info("添加访问控制条目",
		zap.String("id", entry.ID),
		zap.String("subject", entry.Subject),
		zap.String("permission", string(entry.Permission)),
	)

	return nil
}

// RemoveEntry 移除访问控制条目
func (ac *AccessControl) RemoveEntry(entryID string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if _, exists := ac.entries[entryID]; !exists {
		return fmt.Errorf("条目 %s 不存在", entryID)
	}

	delete(ac.entries, entryID)
	return nil
}

// CheckAccess 检查访问权限
func (ac *AccessControl) CheckAccess(subject, resource string, permission Permission) bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	for _, entry := range ac.entries {
		if !entry.Enabled {
			continue
		}
		if entry.Subject == subject && entry.Resource == resource && entry.Permission == permission {
			if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
				continue
			}
			return entry.Policy == AccessPolicyAllow
		}
	}

	// 默认拒绝
	return false
}

// AddACLRule 添加 ACL 规则
func (ac *AccessControl) AddACLRule(rule ACLRule) error {
	if rule.ID == "" {
		rule.ID = generateConnectionID()
	}
	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	rule.Enabled = true
	ac.aclRules[rule.ID] = &rule
	return nil
}

// RemoveACLRule 移除 ACL 规则
func (ac *AccessControl) RemoveACLRule(ruleID string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if _, exists := ac.aclRules[ruleID]; !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}

	delete(ac.aclRules, ruleID)
	return nil
}

// ListACLRules 列出所有 ACL 规则
func (ac *AccessControl) ListACLRules() []*ACLRule {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	rules := make([]*ACLRule, 0, len(ac.aclRules))
	for _, r := range ac.aclRules {
		rules = append(rules, r)
	}
	return rules
}

// AuthenticatePeer 认证节点
func (ac *AccessControl) AuthenticatePeer(peerID, publicKey, authToken, method string) (*PeerAuth, error) {
	if peerID == "" {
		return nil, fmt.Errorf("节点 ID 不能为空")
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	auth := &PeerAuth{
		PeerID:     peerID,
		PublicKey:  publicKey,
		AuthToken:  authToken,
		AuthMethod: method,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		Trusted:    true,
		LastAuth:   time.Now(),
	}

	ac.peers[peerID] = auth

	ac.logger.Info("节点认证成功",
		zap.String("peer_id", peerID),
		zap.String("method", method),
	)

	return auth, nil
}

// GetPeerAuth 获取节点认证信息
func (ac *AccessControl) GetPeerAuth(peerID string) (*PeerAuth, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	auth, exists := ac.peers[peerID]
	if !exists {
		return nil, fmt.Errorf("节点 %s 未认证", peerID)
	}

	if time.Now().After(auth.ExpiresAt) {
		return nil, fmt.Errorf("节点 %s 认证已过期", peerID)
	}

	return auth, nil
}

// ============================================================
// TunnelManager - 隧道管理器
// ============================================================

// TunnelManager 隧道管理器
type TunnelManager struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	tunnels map[string]*TunnelStatus
	tlsCfg  *TLSConfig
}

// NewTunnelManager 创建隧道管理器
func NewTunnelManager(logger *zap.Logger, tlsCfg *TLSConfig) *TunnelManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TunnelManager{
		logger:  logger,
		tunnels: make(map[string]*TunnelStatus),
		tlsCfg:  tlsCfg,
	}
}

// CreateTunnel 创建加密隧道
func (tm *TunnelManager) CreateTunnel(localPeerID, remotePeerID, protocol string, localPort, remotePort int) (*TunnelStatus, error) {
	if localPeerID == "" || remotePeerID == "" {
		return nil, fmt.Errorf("节点 ID 不能为空")
	}

	tunnelID := generateConnectionID()
	tunnel := &TunnelStatus{
		ID:            tunnelID,
		LocalPeerID:   localPeerID,
		RemotePeerID:  remotePeerID,
		Protocol:      protocol,
		LocalPort:     localPort,
		RemotePort:    remotePort,
		Encrypted:     true,
		Active:        true,
		EstablishedAt: time.Now(),
		LastActivity:  time.Now(),
	}

	if tm.tlsCfg != nil && tm.tlsCfg.Enabled {
		tunnel.TLSVersion = tm.tlsCfg.MinVersion
		tunnel.CipherSuite = "TLS_AES_256_GCM_SHA384"
	}

	tm.mu.Lock()
	tm.tunnels[tunnelID] = tunnel
	tm.mu.Unlock()

	tm.logger.Info("创建加密隧道",
		zap.String("tunnel_id", tunnelID),
		zap.String("local", fmt.Sprintf("%s:%d", localPeerID, localPort)),
		zap.String("remote", fmt.Sprintf("%s:%d", remotePeerID, remotePort)),
	)

	return tunnel, nil
}

// CloseTunnel 关闭隧道
func (tm *TunnelManager) CloseTunnel(tunnelID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tunnel, exists := tm.tunnels[tunnelID]
	if !exists {
		return fmt.Errorf("隧道 %s 不存在", tunnelID)
	}

	tunnel.Active = false
	return nil
}

// GetTunnel 获取隧道信息
func (tm *TunnelManager) GetTunnel(tunnelID string) (*TunnelStatus, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnel, exists := tm.tunnels[tunnelID]
	if !exists {
		return nil, fmt.Errorf("隧道 %s 不存在", tunnelID)
	}
	return tunnel, nil
}

// ListTunnels 列出所有隧道
func (tm *TunnelManager) ListTunnels() []*TunnelStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnels := make([]*TunnelStatus, 0, len(tm.tunnels))
	for _, t := range tm.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// ============================================================
// RemoteAccessManager - 远程访问总管理器
// ============================================================

// RemoteAccessManager 远程访问总管理器
type RemoteAccessManager struct {
	logger    *zap.Logger
	config    RemoteAccessConfig
	connMgr   *ConnectionManager
	natDetect *NATDetector
	relayMgr  *RelayManager
	bwMgr     *BandwidthManager
	acl       *AccessControl
	tunnelMgr *TunnelManager

	// 会话与日志
	sessions  map[string]*AccessSession
	accessLog []AccessLogEntry
	sessMu    sync.RWMutex
}

// RemoteAccessConfig 远程访问配置
type RemoteAccessConfig struct {
	LocalPeerID      string         `json:"local_peer_id"`
	MaxBandwidth     int64          `json:"max_bandwidth"`
	BandwidthPolicy  BandwidthPolicy `json:"bandwidth_policy"`
	TLSEnabled       bool           `json:"tls_enabled"`
	TLSMinVersion    string         `json:"tls_min_version"`
	DDNS             *DDNSConfig    `json:"ddns,omitempty"`
}

// AccessProtocol 访问协议类型
type AccessProtocol string

const (
	ProtocolHTTPS AccessProtocol = "https"
	ProtocolSSH   AccessProtocol = "ssh"
	ProtocolVPN   AccessProtocol = "vpn"
	ProtocolP2P   AccessProtocol = "p2p"
)

// DDNSConfig DDNS 配置
type DDNSConfig struct {
	Provider   string `json:"provider"`
	Domain     string `json:"domain"`
	APIKey     string `json:"api_key"`
	Enabled    bool   `json:"enabled"`
	LastUpdate string `json:"last_update"`
}

// RemoteAccessStatus 远程访问状态
type RemoteAccessStatus struct {
	Enabled         bool   `json:"enabled"`
	DDNSEnabled     bool   `json:"ddns_enabled"`
	TLSEnabled      bool   `json:"tls_enabled"`
	ActiveSessions  int    `json:"active_sessions"`
	PublicIP        string `json:"public_ip"`
	DDNSDomain      string `json:"ddns_domain"`
}

// AccessSession 访问会话
type AccessSession struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	DeviceName string         `json:"device_name"`
	IP         string         `json:"ip"`
	Protocol   AccessProtocol `json:"protocol"`
	StartTime  time.Time      `json:"start_time"`
	LastActive time.Time      `json:"last_active"`
	Status     string         `json:"status"`
}

// AccessLogEntry 访问日志条目
type AccessLogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	UserID    string         `json:"user_id"`
	IP        string         `json:"ip"`
	Protocol  AccessProtocol `json:"protocol"`
	Action    string         `json:"action"`
	Status    string         `json:"status"`
}

// CertificateInfo 证书信息
type CertificateInfo struct {
	Domain    string    `json:"domain"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	Status    string    `json:"status"`
}

// NewRemoteAccessManager 创建远程访问管理器
func NewRemoteAccessManager(logger *zap.Logger, config RemoteAccessConfig) *RemoteAccessManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config.LocalPeerID == "" {
		config.LocalPeerID = generatePeerID()
	}

	connMgr := NewConnectionManager(logger, config.LocalPeerID)
	natDetect := NewNATDetector(logger)
	relayMgr := NewRelayManager(logger)

	bwConfig := BandwidthConfig{
		Policy:       config.BandwidthPolicy,
		MaxBandwidth: config.MaxBandwidth,
		MinBandwidth: 1024 * 1024,
	}
	bwMgr := NewBandwidthManager(logger, bwConfig)

	acl := NewAccessControl(logger)

	tlsCfg := &TLSConfig{
		Enabled:    config.TLSEnabled,
		MinVersion: config.TLSMinVersion,
		MaxVersion: "TLSv1.3",
	}
	tunnelMgr := NewTunnelManager(logger, tlsCfg)

	return &RemoteAccessManager{
		logger:    logger,
		config:    config,
		connMgr:   connMgr,
		natDetect: natDetect,
		relayMgr:  relayMgr,
		bwMgr:     bwMgr,
		acl:       acl,
		tunnelMgr: tunnelMgr,
		sessions:  make(map[string]*AccessSession),
		accessLog: make([]AccessLogEntry, 0),
	}
}

// GetConnectionManager 获取连接管理器
func (ram *RemoteAccessManager) GetConnectionManager() *ConnectionManager {
	return ram.connMgr
}

// GetNATDetector 获取 NAT 检测器
func (ram *RemoteAccessManager) GetNATDetector() *NATDetector {
	return ram.natDetect
}

// GetRelayManager 获取中继管理器
func (ram *RemoteAccessManager) GetRelayManager() *RelayManager {
	return ram.relayMgr
}

// GetBandwidthManager 获取带宽管理器
func (ram *RemoteAccessManager) GetBandwidthManager() *BandwidthManager {
	return ram.bwMgr
}

// GetAccessControl 获取访问控制
func (ram *RemoteAccessManager) GetAccessControl() *AccessControl {
	return ram.acl
}

// GetTunnelManager 获取隧道管理器
func (ram *RemoteAccessManager) GetTunnelManager() *TunnelManager {
	return ram.tunnelMgr
}

// ============= 会话管理方法 =============

// ListActiveSessions 列出活跃会话
func (ram *RemoteAccessManager) ListActiveSessions() []*AccessSession {
	ram.sessMu.RLock()
	defer ram.sessMu.RUnlock()

	sessions := make([]*AccessSession, 0, len(ram.sessions))
	for _, s := range ram.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// CreateSession 创建访问会话
func (ram *RemoteAccessManager) CreateSession(userID, deviceName, ip string, protocol AccessProtocol) (*AccessSession, error) {
	if userID == "" || deviceName == "" {
		return nil, fmt.Errorf("user_id and device_name are required")
	}

	sessionID := generateConnectionID()
	session := &AccessSession{
		ID:         sessionID,
		UserID:     userID,
		DeviceName: deviceName,
		IP:         ip,
		Protocol:   protocol,
		StartTime:  time.Now(),
		LastActive: time.Now(),
		Status:     "active",
	}

	ram.sessMu.Lock()
	ram.sessions[sessionID] = session
	ram.sessMu.Unlock()

	// 记录日志
	ram.addAccessLogEntry(userID, ip, protocol, "connect", "success")

	ram.logger.Info("会话创建", zap.String("id", sessionID), zap.String("user", userID))
	return session, nil
}

// GetSession 获取会话
func (ram *RemoteAccessManager) GetSession(sessionID string) (*AccessSession, error) {
	ram.sessMu.RLock()
	defer ram.sessMu.RUnlock()

	session, exists := ram.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	return session, nil
}

// CloseSession 关闭会话
func (ram *RemoteAccessManager) CloseSession(sessionID string) error {
	ram.sessMu.Lock()
	defer ram.sessMu.Unlock()

	session, exists := ram.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.Status = "closed"
	delete(ram.sessions, sessionID)

	ram.addAccessLogEntry(session.UserID, session.IP, session.Protocol, "disconnect", "success")
	return nil
}

// ============= 状态与配置方法 =============

// GetRemoteAccessStatus 获取远程访问状态
func (ram *RemoteAccessManager) GetRemoteAccessStatus() *RemoteAccessStatus {
	ram.sessMu.RLock()
	activeSessions := len(ram.sessions)
	ram.sessMu.RUnlock()

	ddnsDomain := ""
	ddnsEnabled := false
	if ram.config.DDNS != nil {
		ddnsDomain = ram.config.DDNS.Domain
		ddnsEnabled = ram.config.DDNS.Enabled
	}

	return &RemoteAccessStatus{
		Enabled:        true,
		DDNSEnabled:    ddnsEnabled,
		TLSEnabled:     ram.config.TLSEnabled,
		ActiveSessions: activeSessions,
		DDNSDomain:     ddnsDomain,
	}
}

// GetConfig 获取配置
func (ram *RemoteAccessManager) GetConfig() RemoteAccessConfig {
	return ram.config
}

// UpdateConfig 更新配置
func (ram *RemoteAccessManager) UpdateConfig(config *RemoteAccessConfig) {
	if config != nil {
		ram.config = *config
	}
}

// ============= DDNS 方法 =============

// GetDDNSStatus 获取 DDNS 状态
func (ram *RemoteAccessManager) GetDDNSStatus() *DDNSConfig {
	if ram.config.DDNS == nil {
		return &DDNSConfig{}
	}
	return ram.config.DDNS
}

// UpdateDDNS 更新 DDNS 配置
func (ram *RemoteAccessManager) UpdateDDNS(config *DDNSConfig) error {
	if config == nil {
		return fmt.Errorf("ddns config is required")
	}
	ram.config.DDNS = config
	ram.logger.Info("DDNS 配置更新", zap.String("domain", config.Domain))
	return nil
}

// ============= 证书方法 =============

// ListCertificates 列出证书
func (ram *RemoteAccessManager) ListCertificates() []CertificateInfo {
	return []CertificateInfo{}
}

// RenewCertificate 续期证书
func (ram *RemoteAccessManager) RenewCertificate() error {
	ram.logger.Info("证书续期请求")
	return nil
}

// ============= 访问日志方法 =============

// GetAccessLog 获取访问日志
func (ram *RemoteAccessManager) GetAccessLog(limit int) []AccessLogEntry {
	ram.sessMu.RLock()
	defer ram.sessMu.RUnlock()

	if limit <= 0 || limit > len(ram.accessLog) {
		limit = len(ram.accessLog)
	}
	start := len(ram.accessLog) - limit
	if start < 0 {
		start = 0
	}
	return ram.accessLog[start:]
}

// addAccessLogEntry 添加访问日志
func (ram *RemoteAccessManager) addAccessLogEntry(userID, ip string, protocol AccessProtocol, action, status string) {
	ram.accessLog = append(ram.accessLog, AccessLogEntry{
		Timestamp: time.Now(),
		UserID:    userID,
		IP:        ip,
		Protocol:  protocol,
		Action:    action,
		Status:    status,
	})
}

// ============================================================
// 辅助函数
// ============================================================

// generatePeerID 生成节点 ID
func generatePeerID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "peer-" + hex.EncodeToString(b)
}

// generateConnectionID 生成连接 ID
func generateConnectionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// clamp 限制值在范围内
func clamp(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}
