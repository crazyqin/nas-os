package remoteaccess

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages remote access functionality including P2P, relay, DDNS, and port mapping.
type Manager struct {
	mu            sync.RWMutex
	config        Config
	logger        *zap.Logger
	state         *ManagerState
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	stunClient    *STUNClient
	relayClient   *RelayClient
	ddnsClient    *DDNSClient
	portMapper    *PortMapper
	authManager   *AuthManager
	encryptor     *Encryptor
}

// NewManager creates a new remote access manager.
func NewManager(config Config, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config: config,
		logger: logger,
		state: &ManagerState{
			config:       config,
			connections:  make(map[string]*ConnectionInfo),
			sessions:     make(map[string]*Session),
			portMappings: make(map[string]*PortMapping),
			p2pSessions:  make(map[string]*P2PSession),
			relaySessions: make(map[string]*RelaySession),
			startTime:    time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize sub-components
	m.stunClient = NewSTUNClient(config.STUN, logger)
	m.relayClient = NewRelayClient(config.Relay, logger)
	m.ddnsClient = NewDDNSClient(config.DDNS, logger)
	m.portMapper = NewPortMapper(config.UPnP, config.NATPMP, logger)
	m.authManager = NewAuthManager(config.Auth, logger)
	m.encryptor = NewEncryptor(config.Encryption, logger)

	return m
}

// Start starts the remote access manager and all its components.
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting remote access manager")

	// Start STUN client for NAT detection
	if m.config.STUN.Enabled {
		m.wg.Add(1)
		go m.runSTUNDetection()
	}

	// Start DDNS updater
	if m.config.DDNS.Enabled {
		m.wg.Add(1)
		go m.runDDNSUpdater()
	}

	// Start port mapping manager
	if m.config.UPnP.Enabled || m.config.NATPMP.Enabled {
		m.wg.Add(1)
		go m.runPortMapper()
	}

	// Start connection monitor
	m.wg.Add(1)
	go m.runConnectionMonitor()

	// Start session cleaner
	m.wg.Add(1)
	go m.runSessionCleaner()

	m.logger.Info("Remote access manager started")
	return nil
}

// Stop stops the remote access manager and all its components.
func (m *Manager) Stop() error {
	m.logger.Info("Stopping remote access manager")

	// Cancel context to stop all goroutines
	m.cancel()

	// Wait for all goroutines to finish
	m.wg.Wait()

	// Clean up port mappings
	if err := m.portMapper.Cleanup(); err != nil {
		m.logger.Error("Failed to cleanup port mappings", zap.Error(err))
	}

	// Close relay connections
	if err := m.relayClient.Close(); err != nil {
		m.logger.Error("Failed to close relay connections", zap.Error(err))
	}

	m.logger.Info("Remote access manager stopped")
	return nil
}

// GetConfig returns the current configuration.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates the configuration.
func (m *Manager) UpdateConfig(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate config
	if err := m.validateConfig(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	m.config = config
	m.state.config = config

	// Update sub-components
	m.stunClient.UpdateConfig(config.STUN)
	m.relayClient.UpdateConfig(config.Relay)
	m.ddnsClient.UpdateConfig(config.DDNS)
	m.portMapper.UpdateConfig(config.UPnP, config.NATPMP)
	m.authManager.UpdateConfig(config.Auth)
	m.encryptor.UpdateConfig(config.Encryption)

	m.logger.Info("Configuration updated")
	return nil
}

// ============================================================
// NAT 穿透
// ============================================================

// DetectNAT detects the NAT type using STUN.
func (m *Manager) DetectNAT(ctx context.Context) (*STUNResult, error) {
	result, err := m.stunClient.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("NAT detection failed: %w", err)
	}

	m.state.mu.Lock()
	m.state.natType = result.NATType
	m.state.publicIP = result.PublicIP
	m.state.mu.Unlock()

	m.logger.Info("NAT detection completed",
		zap.String("nat_type", string(result.NATType)),
		zap.String("public_ip", result.PublicIP),
	)

	return result, nil
}

// CreateP2PSession creates a new P2P session with a peer.
func (m *Manager) CreateP2PSession(ctx context.Context, peerID string) (*P2PSession, error) {
	// Detect NAT type if not already detected
	natResult, err := m.stunClient.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("NAT detection failed: %w", err)
	}

	// Check if P2P is possible based on NAT type
	if !m.isP2PPossible(natResult.NATType) {
		return nil, fmt.Errorf("P2P not possible with NAT type: %s", natResult.NATType)
	}

	session := &P2PSession{
		ID:         generateID(),
		PeerID:     peerID,
		LocalAddr:  fmt.Sprintf("%s:%d", natResult.LocalIP, natResult.LocalPort),
		RemoteAddr: fmt.Sprintf("%s:%d", natResult.PublicIP, natResult.PublicPort),
		NATType:    natResult.NATType,
		Status:     "connecting",
		CreatedAt:  time.Now(),
	}

	m.state.mu.Lock()
	m.state.p2pSessions[session.ID] = session
	m.state.mu.Unlock()

	// Start P2P connection establishment in background
	m.wg.Add(1)
	go m.establishP2PConnection(session)

	return session, nil
}

// GetP2PSession returns a P2P session by ID.
func (m *Manager) GetP2PSession(id string) (*P2PSession, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	session, exists := m.state.p2pSessions[id]
	return session, exists
}

// ListP2PSessions returns all P2P sessions.
func (m *Manager) ListP2PSessions() []*P2PSession {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	sessions := make([]*P2PSession, 0, len(m.state.p2pSessions))
	for _, session := range m.state.p2pSessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// CloseP2PSession closes a P2P session.
func (m *Manager) CloseP2PSession(id string) error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	session, exists := m.state.p2pSessions[id]
	if !exists {
		return fmt.Errorf("P2P session not found: %s", id)
	}

	if session.Status == "closed" {
		return nil
	}

	now := time.Now()
	session.Status = "closed"
	session.ClosedAt = &now

	m.logger.Info("P2P session closed",
		zap.String("session_id", id),
		zap.String("peer_id", session.PeerID),
	)

	return nil
}

// ============================================================
// 中继服务
// ============================================================

// CreateRelaySession creates a new relay session.
func (m *Manager) CreateRelaySession(ctx context.Context, clientID string) (*RelaySession, error) {
	if !m.config.Relay.Enabled {
		return nil, fmt.Errorf("relay service is not enabled")
	}

	// Check max sessions limit
	m.state.mu.RLock()
	activeRelaySessions := 0
	for _, session := range m.state.relaySessions {
		if session.Status == "active" {
			activeRelaySessions++
		}
	}
	m.state.mu.RUnlock()

	if activeRelaySessions >= m.config.Relay.MaxSessions {
		return nil, fmt.Errorf("maximum relay sessions reached: %d", m.config.Relay.MaxSessions)
	}

	session := &RelaySession{
		ID:         generateID(),
		ClientID:   clientID,
		ServerAddr: fmt.Sprintf("%s:%d", m.config.Relay.Server, m.config.Relay.Port),
		Status:     "active",
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	m.state.mu.Lock()
	m.state.relaySessions[session.ID] = session
	m.state.mu.Unlock()

	m.logger.Info("Relay session created",
		zap.String("session_id", session.ID),
		zap.String("client_id", clientID),
	)

	return session, nil
}

// GetRelaySession returns a relay session by ID.
func (m *Manager) GetRelaySession(id string) (*RelaySession, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	session, exists := m.state.relaySessions[id]
	return session, exists
}

// ListRelaySessions returns all relay sessions.
func (m *Manager) ListRelaySessions() []*RelaySession {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	sessions := make([]*RelaySession, 0, len(m.state.relaySessions))
	for _, session := range m.state.relaySessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// CloseRelaySession closes a relay session.
func (m *Manager) CloseRelaySession(id string) error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	session, exists := m.state.relaySessions[id]
	if !exists {
		return fmt.Errorf("relay session not found: %s", id)
	}

	if session.Status == "closed" {
		return nil
	}

	now := time.Now()
	session.Status = "closed"
	session.ClosedAt = &now

	m.logger.Info("Relay session closed",
		zap.String("session_id", id),
		zap.String("client_id", session.ClientID),
	)

	return nil
}

// ============================================================
// 动态域名
// ============================================================

// UpdateDDNS forces a DDNS update.
func (m *Manager) UpdateDDNS(ctx context.Context) error {
	if !m.config.DDNS.Enabled {
		return fmt.Errorf("DDNS is not enabled")
	}

	status, err := m.ddnsClient.Update(ctx)
	if err != nil {
		return fmt.Errorf("DDNS update failed: %w", err)
	}

	m.state.mu.Lock()
	m.state.ddnsStatus = status
	m.state.mu.Unlock()

	m.logger.Info("DDNS updated",
		zap.String("domain", status.Domain),
		zap.String("current_ip", status.CurrentIP),
		zap.String("dns_ip", status.DNSIP),
	)

	return nil
}

// GetDDNSStatus returns the current DDNS status.
func (m *Manager) GetDDNSStatus() *DDNSStatus {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return m.state.ddnsStatus
}

// ============================================================
// 端口映射
// ============================================================

// CreatePortMapping creates a new port mapping.
func (m *Manager) CreatePortMapping(ctx context.Context, mapping *PortMapping) error {
	if !m.config.UPnP.Enabled && !m.config.NATPMP.Enabled {
		return fmt.Errorf("port mapping is not enabled")
	}

	mapping.ID = generateID()
	mapping.CreatedAt = time.Now()

	if err := m.portMapper.AddMapping(ctx, mapping); err != nil {
		return fmt.Errorf("failed to create port mapping: %w", err)
	}

	m.state.mu.Lock()
	m.state.portMappings[mapping.ID] = mapping
	m.state.mu.Unlock()

	m.logger.Info("Port mapping created",
		zap.String("id", mapping.ID),
		zap.String("protocol", string(mapping.Protocol)),
		zap.Int("external_port", mapping.ExternalPort),
		zap.Int("internal_port", mapping.InternalPort),
	)

	return nil
}

// GetPortMapping returns a port mapping by ID.
func (m *Manager) GetPortMapping(id string) (*PortMapping, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	mapping, exists := m.state.portMappings[id]
	return mapping, exists
}

// ListPortMappings returns all port mappings.
func (m *Manager) ListPortMappings() []*PortMapping {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	mappings := make([]*PortMapping, 0, len(m.state.portMappings))
	for _, mapping := range m.state.portMappings {
		mappings = append(mappings, mapping)
	}
	return mappings
}

// DeletePortMapping deletes a port mapping.
func (m *Manager) DeletePortMapping(id string) error {
	m.state.mu.Lock()
	mapping, exists := m.state.portMappings[id]
	if !exists {
		m.state.mu.Unlock()
		return fmt.Errorf("port mapping not found: %s", id)
	}
	delete(m.state.portMappings, id)
	m.state.mu.Unlock()

	if err := m.portMapper.RemoveMapping(context.Background(), mapping); err != nil {
		m.logger.Error("Failed to remove port mapping from gateway",
			zap.String("id", id),
			zap.Error(err),
		)
	}

	m.logger.Info("Port mapping deleted",
		zap.String("id", id),
		zap.Int("external_port", mapping.ExternalPort),
	)

	return nil
}

// RefreshPortMappings refreshes all port mappings.
func (m *Manager) RefreshPortMappings(ctx context.Context) error {
	m.state.mu.RLock()
	mappings := make([]*PortMapping, 0, len(m.state.portMappings))
	for _, mapping := range m.state.portMappings {
		mappings = append(mappings, mapping)
	}
	m.state.mu.RUnlock()

	var lastErr error
	for _, mapping := range mappings {
		if err := m.portMapper.AddMapping(ctx, mapping); err != nil {
			m.logger.Error("Failed to refresh port mapping",
				zap.String("id", mapping.ID),
				zap.Error(err),
			)
			lastErr = err
		}
	}

	return lastErr
}

// ============================================================
// 连接管理
// ============================================================

// GetConnection returns a connection by ID.
func (m *Manager) GetConnection(id string) (*ConnectionInfo, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	conn, exists := m.state.connections[id]
	return conn, exists
}

// ListConnections returns all connections.
func (m *Manager) ListConnections() []*ConnectionInfo {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	conns := make([]*ConnectionInfo, 0, len(m.state.connections))
	for _, conn := range m.state.connections {
		conns = append(conns, conn)
	}
	return conns
}

// GetConnectionStats returns connection statistics.
func (m *Manager) GetConnectionStats() *ConnectionStats {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	stats := &ConnectionStats{
		TotalConnections: len(m.state.connections),
		ByMode:           make(map[ConnectionMode]int),
	}

	var totalLatency float64
	var latencyCount int

	for _, conn := range m.state.connections {
		stats.TotalBytesSent += conn.BytesSent
		stats.TotalBytesRecv += conn.BytesRecv

		switch conn.Status {
		case ConnectionStatusConnected:
			stats.ActiveConnections++
		}

		stats.ByMode[conn.Mode]++

		if conn.Mode == ConnectionModeP2P {
			stats.P2PConnections++
		} else if conn.Mode == ConnectionModeRelay {
			stats.RelayConnections++
		} else if conn.Mode == ConnectionModeDirect {
			stats.DirectConnections++
		}

		if conn.LatencyMs > 0 {
			totalLatency += float64(conn.LatencyMs)
			latencyCount++
		}
	}

	if latencyCount > 0 {
		stats.AverageLatency = totalLatency / float64(latencyCount)
	}

	stats.Uptime = time.Since(m.state.startTime)
	stats.TotalBandwidth = stats.TotalBytesSent + stats.TotalBytesRecv

	return stats
}

// CloseConnection closes a connection by ID.
func (m *Manager) CloseConnection(id string) error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	conn, exists := m.state.connections[id]
	if !exists {
		return fmt.Errorf("connection not found: %s", id)
	}

	conn.Status = ConnectionStatusDisconnected

	m.logger.Info("Connection closed",
		zap.String("id", id),
		zap.String("mode", string(conn.Mode)),
	)

	return nil
}

// ============================================================
// 会话管理
// ============================================================

// CreateSession creates a new user session.
func (m *Manager) CreateSession(userID, deviceID, deviceName, ip, userAgent string) (*Session, error) {
	if !m.config.Auth.Enabled {
		return nil, fmt.Errorf("authentication is not enabled")
	}

	// Check for existing sessions from same device
	m.state.mu.RLock()
	for _, session := range m.state.sessions {
		if session.DeviceID == deviceID && session.IsActive {
			m.state.mu.RUnlock()
			return nil, fmt.Errorf("device already has an active session")
		}
	}
	m.state.mu.RUnlock()

	now := time.Now()
	session := &Session{
		ID:         generateID(),
		UserID:     userID,
		DeviceID:   deviceID,
		DeviceName: deviceName,
		IP:         ip,
		UserAgent:  userAgent,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(m.config.Auth.TokenExpirySec) * time.Second),
		LastActive: now,
		IsActive:   true,
	}

	m.state.mu.Lock()
	m.state.sessions[session.ID] = session
	m.state.mu.Unlock()

	m.logger.Info("Session created",
		zap.String("session_id", session.ID),
		zap.String("user_id", userID),
		zap.String("device_id", deviceID),
	)

	return session, nil
}

// GetSession returns a session by ID.
func (m *Manager) GetSession(id string) (*Session, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	session, exists := m.state.sessions[id]
	return session, exists
}

// ListSessions returns all sessions.
func (m *Manager) ListSessions() []*Session {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	sessions := make([]*Session, 0, len(m.state.sessions))
	for _, session := range m.state.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// RefreshSession refreshes a session's expiry.
func (m *Manager) RefreshSession(id string) error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	session, exists := m.state.sessions[id]
	if !exists {
		return fmt.Errorf("session not found: %s", id)
	}

	if !session.IsActive {
		return fmt.Errorf("session is not active: %s", id)
	}

	now := time.Now()
	session.LastActive = now
	session.ExpiresAt = now.Add(time.Duration(m.config.Auth.TokenExpirySec) * time.Second)

	return nil
}

// InvalidateSession invalidates a session.
func (m *Manager) InvalidateSession(id string) error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	session, exists := m.state.sessions[id]
	if !exists {
		return fmt.Errorf("session not found: %s", id)
	}

	session.IsActive = false

	m.logger.Info("Session invalidated",
		zap.String("session_id", id),
		zap.String("user_id", session.UserID),
	)

	return nil
}

// InvalidateAllUserSessions invalidates all sessions for a user.
func (m *Manager) InvalidateAllUserSessions(userID string) int {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	count := 0
	for _, session := range m.state.sessions {
		if session.UserID == userID && session.IsActive {
			session.IsActive = false
			count++
		}
	}

	m.logger.Info("All user sessions invalidated",
		zap.String("user_id", userID),
		zap.Int("count", count),
	)

	return count
}

// ============================================================
// 安全认证
// ============================================================

// Authenticate authenticates a user and returns a token.
func (m *Manager) Authenticate(ctx context.Context, username, password string) (*Token, error) {
	if !m.config.Auth.Enabled {
		return nil, fmt.Errorf("authentication is not enabled")
	}

	// Authenticate user
	userID, err := m.authManager.Authenticate(ctx, username, password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Generate token
	token, err := m.authManager.GenerateToken(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	m.logger.Info("User authenticated",
		zap.String("user_id", userID),
	)

	return token, nil
}

// ValidateToken validates an authentication token.
func (m *Manager) ValidateToken(tokenString string) (*TokenClaims, error) {
	if !m.config.Auth.Enabled {
		return nil, fmt.Errorf("authentication is not enabled")
	}

	claims, err := m.authManager.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	return claims, nil
}

// RefreshToken refreshes an authentication token.
func (m *Manager) RefreshToken(refreshToken string) (*Token, error) {
	if !m.config.Auth.Enabled {
		return nil, fmt.Errorf("authentication is not enabled")
	}

	token, err := m.authManager.RefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	return token, nil
}

// RevokeToken revokes an authentication token.
func (m *Manager) RevokeToken(tokenString string) error {
	if !m.config.Auth.Enabled {
		return fmt.Errorf("authentication is not enabled")
	}

	return m.authManager.RevokeToken(tokenString)
}

// ============================================================
// 辅助方法
// ============================================================

func (m *Manager) validateConfig(config Config) error {
	if config.STUN.Enabled && len(config.STUN.Servers) == 0 {
		return fmt.Errorf("STUN enabled but no servers configured")
	}

	if config.Relay.Enabled && config.Relay.Server == "" {
		return fmt.Errorf("relay enabled but no server configured")
	}

	if config.DDNS.Enabled {
		if config.DDNS.Domain == "" {
			return fmt.Errorf("DDNS enabled but no domain configured")
		}
		if config.DDNS.Provider == "" {
			return fmt.Errorf("DDNS enabled but no provider configured")
		}
	}

	if config.Auth.Enabled && config.Auth.TokenSecret == "" {
		return fmt.Errorf("auth enabled but no token secret configured")
	}

	return nil
}

func (m *Manager) isP2PPossible(natType NATType) bool {
	switch natType {
	case NATTypeNone, NATTypeFull:
		return true
	case NATTypeRestricted, NATTypePortRestricted:
		return true // Possible with STUN
	case NATTypeSymmetric:
		return false // Requires TURN relay
	default:
		return false
	}
}

func (m *Manager) establishP2PConnection(session *P2PSession) {
	defer m.wg.Done()

	// Simulate P2P connection establishment
	time.Sleep(100 * time.Millisecond)

	m.state.mu.Lock()
	if session.Status == "connecting" {
		now := time.Now()
		session.Status = "connected"
		session.ConnectedAt = &now
	}
	m.state.mu.Unlock()

	m.logger.Info("P2P connection established",
		zap.String("session_id", session.ID),
		zap.String("peer_id", session.PeerID),
	)
}

// ============================================================
// 后台任务
// ============================================================

func (m *Manager) runSTUNDetection() {
	defer m.wg.Done()

	// Initial detection
	if _, err := m.DetectNAT(m.ctx); err != nil {
		m.logger.Error("Initial NAT detection failed", zap.Error(err))
	}

	// Periodic re-detection
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if _, err := m.DetectNAT(m.ctx); err != nil {
				m.logger.Error("NAT re-detection failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runDDNSUpdater() {
	defer m.wg.Done()

	// Initial update
	if err := m.UpdateDDNS(m.ctx); err != nil {
		m.logger.Error("Initial DDNS update failed", zap.Error(err))
	}

	// Periodic updates
	interval := time.Duration(m.config.DDNS.IntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.UpdateDDNS(m.ctx); err != nil {
				m.logger.Error("DDNS update failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runPortMapper() {
	defer m.wg.Done()

	// Initial port mapping setup
	if err := m.RefreshPortMappings(m.ctx); err != nil {
		m.logger.Error("Initial port mapping refresh failed", zap.Error(err))
	}

	// Periodic refresh
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.RefreshPortMappings(m.ctx); err != nil {
				m.logger.Error("Port mapping refresh failed", zap.Error(err))
			}
		}
	}
}

func (m *Manager) runConnectionMonitor() {
	defer m.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.monitorConnections()
		}
	}
}

func (m *Manager) monitorConnections() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	now := time.Now()
	idleTimeout := 5 * time.Minute

	for id, conn := range m.state.connections {
		if conn.Status == ConnectionStatusConnected {
			if now.Sub(conn.LastActive) > idleTimeout {
				conn.Status = ConnectionStatusDisconnected
				m.logger.Info("Connection timed out",
					zap.String("id", id),
					zap.Duration("idle", now.Sub(conn.LastActive)),
				)
			}
		}
	}
}

func (m *Manager) runSessionCleaner() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanExpiredSessions()
		}
	}
}

func (m *Manager) cleanExpiredSessions() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for _, session := range m.state.sessions {
		if session.IsActive && now.After(session.ExpiresAt) {
			session.IsActive = false
			cleaned++
		}
	}

	if cleaned > 0 {
		m.logger.Info("Cleaned expired sessions", zap.Int("count", cleaned))
	}
}

// generateID generates a random hex ID.
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// STUNClient is a stub for STUN client functionality.
type STUNClient struct {
	config STUNConfig
	logger *zap.Logger
}

// NewSTUNClient creates a new STUN client.
func NewSTUNClient(config STUNConfig, logger *zap.Logger) *STUNClient {
	return &STUNClient{config: config, logger: logger}
}

// UpdateConfig updates the STUN client configuration.
func (c *STUNClient) UpdateConfig(config STUNConfig) {
	c.config = config
}

// Detect performs NAT detection using STUN.
func (c *STUNClient) Detect(ctx context.Context) (*STUNResult, error) {
	// Simulate STUN detection
	// In production, this would use actual STUN protocol
	localIP, err := getLocalIP()
	if err != nil {
		return nil, err
	}

	return &STUNResult{
		NATType:    NATTypeUnknown,
		PublicIP:   "0.0.0.0",
		PublicPort: 0,
		LocalIP:    localIP,
		LocalPort:  0,
		Server:     c.config.Servers[0],
		Timestamp:  time.Now(),
	}, nil
}

// RelayClient is a stub for relay client functionality.
type RelayClient struct {
	config RelayConfig
	logger *zap.Logger
}

// NewRelayClient creates a new relay client.
func NewRelayClient(config RelayConfig, logger *zap.Logger) *RelayClient {
	return &RelayClient{config: config, logger: logger}
}

// UpdateConfig updates the relay client configuration.
func (c *RelayClient) UpdateConfig(config RelayConfig) {
	c.config = config
}

// Close closes the relay client.
func (c *RelayClient) Close() error {
	return nil
}

// DDNSClient is a stub for DDNS client functionality.
type DDNSClient struct {
	config DDNSConfig
	logger *zap.Logger
}

// NewDDNSClient creates a new DDNS client.
func NewDDNSClient(config DDNSConfig, logger *zap.Logger) *DDNSClient {
	return &DDNSClient{config: config, logger: logger}
}

// UpdateConfig updates the DDNS client configuration.
func (c *DDNSClient) UpdateConfig(config DDNSConfig) {
	c.config = config
}

// Update performs a DDNS update.
func (c *DDNSClient) Update(ctx context.Context) (*DDNSStatus, error) {
	// Simulate DDNS update
	// In production, this would call actual DDNS provider APIs
	return &DDNSStatus{
		Provider:   c.config.Provider,
		Domain:     c.config.Domain,
		CurrentIP:  "0.0.0.0",
		DNSIP:      "0.0.0.0",
		LastUpdate: time.Now(),
		NextUpdate: time.Now().Add(time.Duration(c.config.IntervalSec) * time.Second),
		Status:     "pending",
	}, nil
}

// PortMapper is a stub for port mapping functionality.
type PortMapper struct {
	upnpConfig   UPnPConfig
	natpmpConfig NATPMPConfig
	logger       *zap.Logger
}

// NewPortMapper creates a new port mapper.
func NewPortMapper(upnpConfig UPnPConfig, natpmpConfig NATPMPConfig, logger *zap.Logger) *PortMapper {
	return &PortMapper{upnpConfig: upnpConfig, natpmpConfig: natpmpConfig, logger: logger}
}

// UpdateConfig updates the port mapper configuration.
func (p *PortMapper) UpdateConfig(upnpConfig UPnPConfig, natpmpConfig NATPMPConfig) {
	p.upnpConfig = upnpConfig
	p.natpmpConfig = natpmpConfig
}

// AddMapping adds a port mapping.
func (p *PortMapper) AddMapping(ctx context.Context, mapping *PortMapping) error {
	// Simulate port mapping
	// In production, this would use UPnP/NAT-PMP protocols
	p.logger.Info("Port mapping added",
		zap.Int("external_port", mapping.ExternalPort),
		zap.Int("internal_port", mapping.InternalPort),
	)
	return nil
}

// RemoveMapping removes a port mapping.
func (p *PortMapper) RemoveMapping(ctx context.Context, mapping *PortMapping) error {
	// Simulate port mapping removal
	p.logger.Info("Port mapping removed",
		zap.Int("external_port", mapping.ExternalPort),
	)
	return nil
}

// Cleanup cleans up all port mappings.
func (p *PortMapper) Cleanup() error {
	return nil
}

// AuthManager is a stub for authentication functionality.
type AuthManager struct {
	config AuthConfig
	logger *zap.Logger
}

// NewAuthManager creates a new auth manager.
func NewAuthManager(config AuthConfig, logger *zap.Logger) *AuthManager {
	return &AuthManager{config: config, logger: logger}
}

// UpdateConfig updates the auth manager configuration.
func (a *AuthManager) UpdateConfig(config AuthConfig) {
	a.config = config
}

// Authenticate authenticates a user.
func (a *AuthManager) Authenticate(ctx context.Context, username, password string) (string, error) {
	// Simulate authentication
	// In production, this would validate against a user database
	return "user-id", nil
}

// GenerateToken generates an authentication token.
func (a *AuthManager) GenerateToken(userID string) (*Token, error) {
	// Simulate token generation
	// In production, this would use JWT or similar
	return &Token{
		AccessToken:  generateID(),
		RefreshToken: generateID(),
		TokenType:    "Bearer",
		ExpiresIn:    a.config.TokenExpirySec,
		ExpiresAt:    time.Now().Add(time.Duration(a.config.TokenExpirySec) * time.Second),
	}, nil
}

// ValidateToken validates an authentication token.
func (a *AuthManager) ValidateToken(tokenString string) (*TokenClaims, error) {
	// Simulate token validation
	// In production, this would verify JWT signature
	return &TokenClaims{
		UserID:    "user-id",
		DeviceID:  "device-id",
		SessionID: "session-id",
		IP:        "0.0.0.0",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}, nil
}

// RefreshToken refreshes an authentication token.
func (a *AuthManager) RefreshToken(refreshToken string) (*Token, error) {
	// Simulate token refresh
	return a.GenerateToken("user-id")
}

// RevokeToken revokes an authentication token.
func (a *AuthManager) RevokeToken(tokenString string) error {
	// Simulate token revocation
	return nil
}

// Encryptor is a stub for encryption functionality.
type Encryptor struct {
	config EncryptionConfig
	logger *zap.Logger
}

// NewEncryptor creates a new encryptor.
func NewEncryptor(config EncryptionConfig, logger *zap.Logger) *Encryptor {
	return &Encryptor{config: config, logger: logger}
}

// UpdateConfig updates the encryptor configuration.
func (e *Encryptor) UpdateConfig(config EncryptionConfig) {
	e.config = config
}

// getLocalIP returns the local IP address.
func getLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
