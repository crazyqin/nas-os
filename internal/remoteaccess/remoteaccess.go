// Package remoteaccess 提供远程访问管理功能
// 支持多种协议的远程连接、P2P穿透、DDNS、SSL证书管理等
package remoteaccess

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

// AccessProtocol 远程访问协议
type AccessProtocol string

const (
	ProtocolHTTPS AccessProtocol = "https"
	ProtocolSSH   AccessProtocol = "ssh"
	ProtocolVNC   AccessProtocol = "vnc"
	ProtocolRDP   AccessProtocol = "rdp"
	ProtocolWebDAV AccessProtocol = "webdav"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	StatusActive       SessionStatus = "active"
	StatusIdle         SessionStatus = "idle"
	StatusDisconnected SessionStatus = "disconnected"
	StatusExpired      SessionStatus = "expired"
)

// RemoteSession 远程会话
type RemoteSession struct {
	ID         string        `json:"id"`
	UserID     string        `json:"user_id"`
	DeviceName string        `json:"device_name"`
	Protocol   AccessProtocol `json:"protocol"`
	Status     SessionStatus `json:"status"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    *time.Time    `json:"end_time,omitempty"`
	ClientIP   string        `json:"client_ip"`
	Bandwidth  int64         `json:"bandwidth"` // bytes per second
	BytesSent  int64         `json:"bytes_sent"`
	BytesRecv  int64         `json:"bytes_recv"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// RemoteAccessConfig 远程访问配置
type RemoteAccessConfig struct {
	Enabled           bool              `json:"enabled"`
	MaxSessions       int               `json:"max_sessions"`
	SessionTimeout    time.Duration     `json:"session_timeout"`
	IdleTimeout       time.Duration     `json:"idle_timeout"`
	AllowedProtocols  []AccessProtocol  `json:"allowed_protocols"`
	DDNSConfig        *DDNSConfig       `json:"ddns_config,omitempty"`
	CertConfig        *CertConfig       `json:"cert_config,omitempty"`
	P2PConfig         *P2PConfig        `json:"p2p_config,omitempty"`
	RateLimit         *RateLimitConfig  `json:"rate_limit,omitempty"`
	GeoFilter         *GeoFilterConfig  `json:"geo_filter,omitempty"`
	PortMappings      []PortMapping     `json:"port_mappings,omitempty"`
}

// DDNSConfig 动态DNS配置
type DDNSConfig struct {
	Provider     string        `json:"provider"`
	Domain       string        `json:"domain"`
	Username     string        `json:"username"`
	Password     string        `json:"password"`
	UpdateInterval time.Duration `json:"update_interval"`
	LastUpdate   time.Time     `json:"last_update"`
	CurrentIP    string        `json:"current_ip"`
	Enabled      bool          `json:"enabled"`
}

// CertConfig SSL证书配置
type CertConfig struct {
	Provider      string    `json:"provider"` // letsencrypt, custom
	Domain        string    `json:"domain"`
	Email         string    `json:"email"`
	CertPath      string    `json:"cert_path"`
	KeyPath       string    `json:"key_path"`
	AutoRenew     bool      `json:"auto_renew"`
	RenewBefore   int       `json:"renew_before_days"`
	ExpiryDate    time.Time `json:"expiry_date"`
	LastRenewal   time.Time `json:"last_renewal"`
}

// P2PConfig P2P穿透配置
type P2PConfig struct {
	Enabled       bool          `json:"enabled"`
	STUNServers   []string      `json:"stun_servers"`
	TURNServer    string        `json:"turn_server,omitempty"`
	TURNUsername   string        `json:"turn_username,omitempty"`
	TURNPassword   string        `json:"turn_password,omitempty"`
	RelayFallback bool          `json:"relay_fallback"`
	RelayServer   string        `json:"relay_server,omitempty"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled           bool  `json:"enabled"`
	MaxConnectionsPerIP int `json:"max_connections_per_ip"`
	ConnectionsPerMinute int `json:"connections_per_minute"`
	BandwidthLimit    int64 `json:"bandwidth_limit"` // bytes per second
}

// GeoFilterConfig 地理位置过滤配置
type GeoFilterConfig struct {
	Enabled       bool     `json:"enabled"`
	Mode          string   `json:"mode"` // allowlist, denylist
	AllowedCountries []string `json:"allowed_countries,omitempty"`
	DeniedCountries  []string `json:"denied_countries,omitempty"`
}

// PortMapping 端口映射
type PortMapping struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Protocol    AccessProtocol `json:"protocol"`
	ExternalPort int          `json:"external_port"`
	InternalPort int          `json:"internal_port"`
	InternalHost string       `json:"internal_host"`
	Enabled      bool         `json:"enabled"`
}

// AccessLogEntry 访问日志条目
type AccessLogEntry struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	UserID    string        `json:"user_id"`
	ClientIP  string        `json:"client_ip"`
	Protocol  AccessProtocol `json:"protocol"`
	Action    string        `json:"action"` // connect, disconnect, denied
	Resource  string        `json:"resource,omitempty"`
	Status    int           `json:"status"`
	UserAgent string        `json:"user_agent,omitempty"`
	Country   string        `json:"country,omitempty"`
	Details   string        `json:"details,omitempty"`
}

// DDNSStatus DDNS状态
type DDNSStatus struct {
	Enabled       bool      `json:"enabled"`
	Domain        string    `json:"domain"`
	CurrentIP     string    `json:"current_ip"`
	LastUpdate    time.Time `json:"last_update"`
	NextUpdate    time.Time `json:"next_update"`
	Status        string    `json:"status"` // ok, error, updating
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// CertificateInfo 证书信息
type CertificateInfo struct {
	Domain     string    `json:"domain"`
	Issuer     string    `json:"issuer"`
	ExpiryDate time.Time `json:"expiry_date"`
	DaysLeft   int       `json:"days_left"`
	Status     string    `json:"status"` // valid, expiring, expired
	AutoRenew  bool      `json:"auto_renew"`
}

// RemoteAccessManager 远程访问管理器
type RemoteAccessManager struct {
	mu            sync.RWMutex
	config        *RemoteAccessConfig
	sessions      map[string]*RemoteSession
	accessLog     []*AccessLogEntry
	ipConnections map[string]int
	portMappings  map[string]*PortMapping
	stopChan      chan struct{}
	running       bool
}

// NewRemoteAccessManager 创建远程访问管理器
func NewRemoteAccessManager(config *RemoteAccessConfig) *RemoteAccessManager {
	if config == nil {
		config = DefaultRemoteAccessConfig()
	}

	m := &RemoteAccessManager{
		config:        config,
		sessions:      make(map[string]*RemoteSession),
		accessLog:     make([]*AccessLogEntry, 0),
		ipConnections: make(map[string]int),
		portMappings:  make(map[string]*PortMapping),
		stopChan:      make(chan struct{}),
	}

	// 初始化端口映射
	for _, pm := range config.PortMappings {
		m.portMappings[pm.ID] = &pm
	}

	return m
}

// generateID 生成唯一ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// DefaultRemoteAccessConfig 默认配置
func DefaultRemoteAccessConfig() *RemoteAccessConfig {
	return &RemoteAccessConfig{
		Enabled:        true,
		MaxSessions:    10,
		SessionTimeout: 24 * time.Hour,
		IdleTimeout:    30 * time.Minute,
		AllowedProtocols: []AccessProtocol{
			ProtocolHTTPS,
			ProtocolSSH,
			ProtocolVNC,
			ProtocolRDP,
			ProtocolWebDAV,
		},
		DDNSConfig: &DDNSConfig{
			Provider:       "cloudflare",
			UpdateInterval: 5 * time.Minute,
			Enabled:        false,
		},
		CertConfig: &CertConfig{
			Provider:    "letsencrypt",
			AutoRenew:   true,
			RenewBefore: 30,
		},
		P2PConfig: &P2PConfig{
			Enabled:       true,
			STUNServers:   []string{"stun.l.google.com:19302", "stun1.l.google.com:19302"},
			RelayFallback: true,
		},
		RateLimit: &RateLimitConfig{
			Enabled:              true,
			MaxConnectionsPerIP:  5,
			ConnectionsPerMinute: 30,
			BandwidthLimit:       10 * 1024 * 1024, // 10 MB/s
		},
		GeoFilter: &GeoFilterConfig{
			Enabled: false,
			Mode:    "allowlist",
		},
	}
}

// Start 启动远程访问管理器
func (m *RemoteAccessManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("remote access manager already running")
	}

	m.running = true

	// 启动会话清理协程
	go m.sessionCleanupLoop()

	// 启动DDNS更新协程
	if m.config.DDNSConfig != nil && m.config.DDNSConfig.Enabled {
		go m.ddnsUpdateLoop()
	}

	// 启动证书续期检查协程
	if m.config.CertConfig != nil && m.config.CertConfig.AutoRenew {
		go m.certRenewalLoop()
	}

	m.addAccessLog(&AccessLogEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    "system_start",
		Status:    200,
		Details:   "Remote access manager started",
	})

	return nil
}

// Stop 停止远程访问管理器
func (m *RemoteAccessManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)

	// 断开所有会话
	for _, session := range m.sessions {
		if session.Status == StatusActive || session.Status == StatusIdle {
			now := time.Now()
			session.Status = StatusDisconnected
			session.EndTime = &now
		}
	}

	m.addAccessLog(&AccessLogEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    "system_stop",
		Status:    200,
		Details:   "Remote access manager stopped",
	})
}

// CreateSession 创建远程会话
func (m *RemoteAccessManager) CreateSession(userID, deviceName, clientIP string, protocol AccessProtocol) (*RemoteSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("remote access is disabled")
	}

	// 检查协议是否允许
	if !m.isProtocolAllowed(protocol) {
		m.addAccessLog(&AccessLogEntry{
			ID:        generateID(),
			Timestamp: time.Now(),
			UserID:    userID,
			ClientIP:  clientIP,
			Protocol:  protocol,
			Action:    "denied",
			Status:    403,
			Details:   "Protocol not allowed",
		})
		return nil, fmt.Errorf("protocol %s is not allowed", protocol)
	}

	// 检查地理过滤
	if m.config.GeoFilter != nil && m.config.GeoFilter.Enabled {
		if !m.isGeoAllowed(clientIP) {
			m.addAccessLog(&AccessLogEntry{
				ID:        generateID(),
				Timestamp: time.Now(),
				UserID:    userID,
				ClientIP:  clientIP,
				Protocol:  protocol,
				Action:    "denied",
				Status:    403,
				Details:   "Geo filter blocked",
			})
			return nil, fmt.Errorf("access denied by geo filter")
		}
	}

	// 检查速率限制
	if m.config.RateLimit != nil && m.config.RateLimit.Enabled {
		if !m.checkRateLimit(clientIP) {
			m.addAccessLog(&AccessLogEntry{
				ID:        generateID(),
				Timestamp: time.Now(),
				UserID:    userID,
				ClientIP:  clientIP,
				Protocol:  protocol,
				Action:    "denied",
				Status:    429,
				Details:   "Rate limit exceeded",
			})
			return nil, fmt.Errorf("rate limit exceeded for IP %s", clientIP)
		}
	}

	// 检查最大会话数
	activeCount := m.getActiveSessionCount()
	if activeCount >= m.config.MaxSessions {
		return nil, fmt.Errorf("maximum sessions reached (%d)", m.config.MaxSessions)
	}

	session := &RemoteSession{
		ID:         generateID(),
		UserID:     userID,
		DeviceName: deviceName,
		Protocol:   protocol,
		Status:     StatusActive,
		StartTime:  time.Now(),
		ClientIP:   clientIP,
		Bandwidth:  0,
		Metadata:   make(map[string]string),
	}

	m.sessions[session.ID] = session
	m.ipConnections[clientIP]++

	m.addAccessLog(&AccessLogEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		UserID:    userID,
		ClientIP:  clientIP,
		Protocol:  protocol,
		Action:    "connect",
		Status:    200,
		Resource:  session.ID,
		Details:   fmt.Sprintf("Session created for device %s", deviceName),
	})

	return session, nil
}

// CloseSession 关闭会话
func (m *RemoteAccessManager) CloseSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status == StatusDisconnected || session.Status == StatusExpired {
		return fmt.Errorf("session already closed: %s", sessionID)
	}

	now := time.Now()
	session.Status = StatusDisconnected
	session.EndTime = &now

	// 减少IP连接计数
	if m.ipConnections[session.ClientIP] > 0 {
		m.ipConnections[session.ClientIP]--
	}

	m.addAccessLog(&AccessLogEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		UserID:    session.UserID,
		ClientIP:  session.ClientIP,
		Protocol:  session.Protocol,
		Action:    "disconnect",
		Status:    200,
		Resource:  sessionID,
		Details:   "Session closed",
	})

	return nil
}

// GetSession 获取会话
func (m *RemoteAccessManager) GetSession(sessionID string) (*RemoteSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ListActiveSessions 列出活跃会话
func (m *RemoteAccessManager) ListActiveSessions() []*RemoteSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*RemoteSession, 0)
	for _, s := range m.sessions {
		if s.Status == StatusActive || s.Status == StatusIdle {
			sessions = append(sessions, s)
		}
	}

	return sessions
}

// ListAllSessions 列出所有会话
func (m *RemoteAccessManager) ListAllSessions() []*RemoteSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*RemoteSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}

	return sessions
}

// UpdateSessionBandwidth 更新会话带宽
func (m *RemoteAccessManager) UpdateSessionBandwidth(sessionID string, bytesSent, bytesRecv int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.BytesSent += bytesSent
	session.BytesRecv += bytesRecv

	// 计算带宽（简化计算）
	duration := time.Since(session.StartTime).Seconds()
	if duration > 0 {
		session.Bandwidth = int64(float64(session.BytesSent+session.BytesRecv) / duration)
	}

	return nil
}

// GetRemoteAccessStatus 获取远程访问状态
func (m *RemoteAccessManager) GetRemoteAccessStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeCount := 0
	idleCount := 0
	for _, s := range m.sessions {
		switch s.Status {
		case StatusActive:
			activeCount++
		case StatusIdle:
			idleCount++
		}
	}

	return map[string]interface{}{
		"enabled":         m.config.Enabled,
		"running":         m.running,
		"active_sessions": activeCount,
		"idle_sessions":   idleCount,
		"total_sessions":  len(m.sessions),
		"max_sessions":    m.config.MaxSessions,
		"protocols":       m.config.AllowedProtocols,
		"ddns_enabled":    m.config.DDNSConfig != nil && m.config.DDNSConfig.Enabled,
		"p2p_enabled":     m.config.P2PConfig != nil && m.config.P2PConfig.Enabled,
		"rate_limit_enabled": m.config.RateLimit != nil && m.config.RateLimit.Enabled,
		"geo_filter_enabled": m.config.GeoFilter != nil && m.config.GeoFilter.Enabled,
	}
}

// UpdateConfig 更新配置
func (m *RemoteAccessManager) UpdateConfig(config *RemoteAccessConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置
func (m *RemoteAccessManager) GetConfig() *RemoteAccessConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateDDNS 更新DDNS
func (m *RemoteAccessManager) UpdateDDNS(config *DDNSConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config == nil {
		return fmt.Errorf("ddns config cannot be nil")
	}

	m.config.DDNSConfig = config

	// 如果启用，立即触发更新
	if config.Enabled {
		go m.updateDDNS()
	}

	return nil
}

// GetDDNSStatus 获取DDNS状态
func (m *RemoteAccessManager) GetDDNSStatus() *DDNSStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.DDNSConfig == nil {
		return &DDNSStatus{
			Enabled: false,
			Status:  "not_configured",
		}
	}

	cfg := m.config.DDNSConfig
	nextUpdate := cfg.LastUpdate.Add(cfg.UpdateInterval)

	return &DDNSStatus{
		Enabled:    cfg.Enabled,
		Domain:     cfg.Domain,
		CurrentIP:  cfg.CurrentIP,
		LastUpdate: cfg.LastUpdate,
		NextUpdate: nextUpdate,
		Status:     "ok",
	}
}

// ListCertificates 列出证书
func (m *RemoteAccessManager) ListCertificates() []*CertificateInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	certs := make([]*CertificateInfo, 0)

	if m.config.CertConfig != nil {
		daysLeft := int(time.Until(m.config.CertConfig.ExpiryDate).Hours() / 24)
		status := "valid"
		if daysLeft < 0 {
			status = "expired"
		} else if daysLeft < m.config.CertConfig.RenewBefore {
			status = "expiring"
		}

		certs = append(certs, &CertificateInfo{
			Domain:     m.config.CertConfig.Domain,
			Issuer:     m.config.CertConfig.Provider,
			ExpiryDate: m.config.CertConfig.ExpiryDate,
			DaysLeft:   daysLeft,
			Status:     status,
			AutoRenew:  m.config.CertConfig.AutoRenew,
		})
	}

	return certs
}

// RenewCertificate 续期证书
func (m *RemoteAccessManager) RenewCertificate() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.CertConfig == nil {
		return fmt.Errorf("certificate not configured")
	}

	// 模拟证书续期
	m.config.CertConfig.LastRenewal = time.Now()
	m.config.CertConfig.ExpiryDate = time.Now().Add(90 * 24 * time.Hour) // 90 days

	m.addAccessLog(&AccessLogEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    "cert_renewal",
		Status:    200,
		Details:   fmt.Sprintf("Certificate renewed for %s", m.config.CertConfig.Domain),
	})

	return nil
}

// AddPortMapping 添加端口映射
func (m *RemoteAccessManager) AddPortMapping(pm *PortMapping) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pm.ID == "" {
		pm.ID = generateID()
	}

	m.portMappings[pm.ID] = pm
}

// RemovePortMapping 移除端口映射
func (m *RemoteAccessManager) RemovePortMapping(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.portMappings[id]; !ok {
		return fmt.Errorf("port mapping not found: %s", id)
	}

	delete(m.portMappings, id)
	return nil
}

// ListPortMappings 列出端口映射
func (m *RemoteAccessManager) ListPortMappings() []*PortMapping {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mappings := make([]*PortMapping, 0, len(m.portMappings))
	for _, pm := range m.portMappings {
		mappings = append(mappings, pm)
	}

	return mappings
}

// GetAccessLog 获取访问日志
func (m *RemoteAccessManager) GetAccessLog(limit int) []*AccessLogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.accessLog) {
		limit = len(m.accessLog)
	}

	start := len(m.accessLog) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*AccessLogEntry, limit)
	copy(result, m.accessLog[start:])
	return result
}

// addAccessLog 添加访问日志（内部方法，调用时需持有锁）
func (m *RemoteAccessManager) addAccessLog(entry *AccessLogEntry) {
	m.accessLog = append(m.accessLog, entry)

	// 限制日志大小
	maxLogSize := 10000
	if len(m.accessLog) > maxLogSize {
		m.accessLog = m.accessLog[len(m.accessLog)-maxLogSize:]
	}
}

// isProtocolAllowed 检查协议是否允许
func (m *RemoteAccessManager) isProtocolAllowed(protocol AccessProtocol) bool {
	for _, p := range m.config.AllowedProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// isGeoAllowed 检查地理位置是否允许
func (m *RemoteAccessManager) isGeoAllowed(ip string) bool {
	if m.config.GeoFilter == nil || !m.config.GeoFilter.Enabled {
		return true
	}

	// 简化的地理位置检查（实际实现需要GeoIP数据库）
	// 这里假设所有IP都允许
	return true
}

// checkRateLimit 检查速率限制
func (m *RemoteAccessManager) checkRateLimit(ip string) bool {
	if m.config.RateLimit == nil || !m.config.RateLimit.Enabled {
		return true
	}

	currentConns := m.ipConnections[ip]
	return currentConns < m.config.RateLimit.MaxConnectionsPerIP
}

// getActiveSessionCount 获取活跃会话数
func (m *RemoteAccessManager) getActiveSessionCount() int {
	count := 0
	for _, s := range m.sessions {
		if s.Status == StatusActive || s.Status == StatusIdle {
			count++
		}
	}
	return count
}

// sessionCleanupLoop 会话清理循环
func (m *RemoteAccessManager) sessionCleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions 清理过期会话
func (m *RemoteAccessManager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, session := range m.sessions {
		if session.Status == StatusActive || session.Status == StatusIdle {
			// 检查会话超时
			if now.Sub(session.StartTime) > m.config.SessionTimeout {
				session.Status = StatusExpired
				endTime := now
				session.EndTime = &endTime
				if m.ipConnections[session.ClientIP] > 0 {
					m.ipConnections[session.ClientIP]--
				}
			}
		}
	}
}

// ddnsUpdateLoop DDNS更新循环
func (m *RemoteAccessManager) ddnsUpdateLoop() {
	m.mu.RLock()
	interval := m.config.DDNSConfig.UpdateInterval
	m.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.updateDDNS()
		}
	}
}

// updateDDNS 更新DDNS
func (m *RemoteAccessManager) updateDDNS() {
	// 获取当前公网IP
	currentIP := m.getPublicIP()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.DDNSConfig == nil || !m.config.DDNSConfig.Enabled {
		return
	}

	// 如果IP变化，更新DDNS
	if currentIP != m.config.DDNSConfig.CurrentIP {
		m.config.DDNSConfig.CurrentIP = currentIP
		m.config.DDNSConfig.LastUpdate = time.Now()

		m.addAccessLog(&AccessLogEntry{
			ID:        generateID(),
			Timestamp: time.Now(),
			Action:    "ddns_update",
			Status:    200,
			Details:   fmt.Sprintf("DDNS updated: IP changed to %s", currentIP),
		})
	}
}

// getPublicIP 获取公网IP
func (m *RemoteAccessManager) getPublicIP() string {
	// 简化实现，实际应该调用外部服务
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "0.0.0.0"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// certRenewalLoop 证书续期检查循环
func (m *RemoteAccessManager) certRenewalLoop() {
	ticker := time.NewTicker(24 * time.Hour) // 每天检查一次
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkCertRenewal()
		}
	}
}

// checkCertRenewal 检查证书续期
func (m *RemoteAccessManager) checkCertRenewal() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.CertConfig == nil || !m.config.CertConfig.AutoRenew {
		return
	}

	daysLeft := int(time.Until(m.config.CertConfig.ExpiryDate).Hours() / 24)
	if daysLeft < m.config.CertConfig.RenewBefore {
		// 触发证书续期
		m.config.CertConfig.LastRenewal = time.Now()
		m.config.CertConfig.ExpiryDate = time.Now().Add(90 * 24 * time.Hour)

		m.addAccessLog(&AccessLogEntry{
			ID:        generateID(),
			Timestamp: time.Now(),
			Action:    "cert_auto_renewal",
			Status:    200,
			Details:   fmt.Sprintf("Certificate auto-renewed for %s", m.config.CertConfig.Domain),
		})
	}
}
