package vpnclient

import (
	"fmt"
	"sync"
	"time"
)

// Manager is the unified VPN client manager that coordinates all VPN protocols.
type Manager struct {
	mu              sync.RWMutex
	profiles        map[string]*VPNProfile
	connections     map[string]*VPNConnection
	openvpn         *OpenVPNClient
	wireguard       *WireGuardClient
	l2tp            *L2TPClient
	traffic         *TrafficMonitor
	defaultProfile  string
	failoverEnabled bool
	autoReconnect   bool
	startTime       time.Time
	nextID          int
}

// NewManager creates a new VPN client manager.
func NewManager() *Manager {
	return &Manager{
		profiles:      make(map[string]*VPNProfile),
		connections:   make(map[string]*VPNConnection),
		openvpn:       NewOpenVPNClient(),
		wireguard:     NewWireGuardClient(),
		l2tp:          NewL2TPClient(),
		traffic:       NewTrafficMonitor(),
		autoReconnect: true,
		startTime:     time.Now(),
		nextID:        1,
	}
}

// generateID generates a sequential unique ID with the given prefix.
func (m *Manager) generateID(prefix string) string {
	id := fmt.Sprintf("%s_%d", prefix, m.nextID)
	m.nextID++
	return id
}

// ==================== Profile Management ====================

// CreateProfile creates a new VPN profile.
func (m *Manager) CreateProfile(req CreateProfileRequest) (*VPNProfile, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate name
	for _, p := range m.profiles {
		if p.Name == req.Name {
			return nil, fmt.Errorf("profile with name %s already exists", req.Name)
		}
	}

	id := m.generateID("profile")
	profile := &VPNProfile{
		ID:          id,
		Name:        req.Name,
		Protocol:    req.Protocol,
		ServerAddr:  req.ServerAddr,
		ServerPort:  req.ServerPort,
		AuthType:    req.AuthType,
		Username:    req.Username,
		Password:    req.Password,
		ConfigFile:  req.ConfigFile,
		AutoConnect: req.AutoConnect,
		Enabled:     true,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.profiles[id] = profile

	// Set as default if it's the first profile
	if len(m.profiles) == 1 {
		m.defaultProfile = id
	}

	result := *profile
	return &result, nil
}

// GetProfile returns a VPN profile by ID.
func (m *Manager) GetProfile(profileID string) (*VPNProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, exists := m.profiles[profileID]
	if !exists {
		return nil, fmt.Errorf("profile not found: %s", profileID)
	}

	result := *profile
	return &result, nil
}

// ListProfiles returns all VPN profiles.
func (m *Manager) ListProfiles() []VPNProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]VPNProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, *p)
	}
	return profiles
}

// UpdateProfile updates a VPN profile.
func (m *Manager) UpdateProfile(profileID string, req UpdateProfileRequest) (*VPNProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.profiles[profileID]
	if !exists {
		return nil, fmt.Errorf("profile not found: %s", profileID)
	}

	if req.Name != nil {
		profile.Name = *req.Name
	}
	if req.ServerAddr != nil {
		profile.ServerAddr = *req.ServerAddr
	}
	if req.ServerPort != nil {
		profile.ServerPort = *req.ServerPort
	}
	if req.Username != nil {
		profile.Username = *req.Username
	}
	if req.Password != nil {
		profile.Password = *req.Password
	}
	if req.AutoConnect != nil {
		profile.AutoConnect = *req.AutoConnect
	}
	if req.Enabled != nil {
		profile.Enabled = *req.Enabled
	}
	if req.Metadata != nil {
		profile.Metadata = req.Metadata
	}
	profile.UpdatedAt = time.Now()

	result := *profile
	return &result, nil
}

// DeleteProfile deletes a VPN profile.
func (m *Manager) DeleteProfile(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.profiles[profileID]; !exists {
		return fmt.Errorf("profile not found: %s", profileID)
	}

	// Check if connected
	for _, conn := range m.connections {
		if conn.ProfileID == profileID && conn.Status == StatusConnected {
			return fmt.Errorf("cannot delete profile with active connection")
		}
	}

	delete(m.profiles, profileID)

	// Clear default if it was the deleted profile
	if m.defaultProfile == profileID {
		m.defaultProfile = ""
	}

	return nil
}

// SetDefaultProfile sets the default VPN profile.
func (m *Manager) SetDefaultProfile(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.profiles[profileID]; !exists {
		return fmt.Errorf("profile not found: %s", profileID)
	}

	m.defaultProfile = profileID
	return nil
}

// GetDefaultProfile returns the default VPN profile.
func (m *Manager) GetDefaultProfile() (*VPNProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.defaultProfile == "" {
		return nil, fmt.Errorf("no default profile set")
	}

	profile, exists := m.profiles[m.defaultProfile]
	if !exists {
		return nil, fmt.Errorf("default profile not found")
	}

	result := *profile
	return &result, nil
}

// ==================== Connection Management ====================

// Connect establishes a VPN connection using the specified profile.
func (m *Manager) Connect(req ConnectRequest) (*VPNConnection, error) {
	m.mu.RLock()
	profile, exists := m.profiles[req.ProfileID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("profile not found: %s", req.ProfileID)
	}

	if !profile.Enabled {
		m.mu.RUnlock()
		return nil, fmt.Errorf("profile is disabled")
	}

	// Copy profile to avoid holding lock during connect
	profileCopy := *profile
	m.mu.RUnlock()

	var conn *VPNConnection
	var err error

	switch profileCopy.Protocol {
	case ProtocolOpenVPN:
		conn, err = m.openvpn.Connect(&profileCopy)
	case ProtocolWireGuard:
		conn, err = m.wireguard.Connect(&profileCopy)
	case ProtocolL2TP:
		conn, err = m.l2tp.Connect(&profileCopy)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", profileCopy.Protocol)
	}

	if err != nil {
		return nil, err
	}

	// Store connection reference
	m.mu.Lock()
	m.connections[conn.ID] = conn
	m.mu.Unlock()

	return conn, nil
}

// Disconnect terminates a VPN connection.
func (m *Manager) Disconnect(req DisconnectRequest) error {
	m.mu.RLock()
	conn, exists := m.connections[req.ConnectionID]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("connection not found: %s", req.ConnectionID)
	}
	protocol := conn.Protocol
	m.mu.RUnlock()

	var err error
	switch protocol {
	case ProtocolOpenVPN:
		err = m.openvpn.Disconnect(req.ConnectionID)
	case ProtocolWireGuard:
		err = m.wireguard.Disconnect(req.ConnectionID)
	case ProtocolL2TP:
		err = m.l2tp.Disconnect(req.ConnectionID)
	}

	if err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.connections, req.ConnectionID)
	m.mu.Unlock()

	return nil
}

// GetConnection returns an active VPN connection.
func (m *Manager) GetConnection(connID string) (*VPNConnection, error) {
	m.mu.RLock()
	conn, exists := m.connections[connID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("connection not found: %s", connID)
	}
	protocol := conn.Protocol
	profileID := conn.ProfileID
	m.mu.RUnlock()

	// Get live status from protocol client
	var liveConn *VPNConnection
	var err error
	switch protocol {
	case ProtocolOpenVPN:
		for _, c := range m.openvpn.ListConnections() {
			if c.ProfileID == profileID {
				lc := c
				liveConn = &lc
				break
			}
		}
	case ProtocolWireGuard:
		for _, c := range m.wireguard.ListConnections() {
			if c.ProfileID == profileID {
				lc := c
				liveConn = &lc
				break
			}
		}
	case ProtocolL2TP:
		for _, c := range m.l2tp.ListConnections() {
			if c.ProfileID == profileID {
				lc := c
				liveConn = &lc
				break
			}
		}
	}

	if liveConn != nil {
		// Update our cached connection
		m.mu.Lock()
		if cached, ok := m.connections[connID]; ok {
			cached.Status = liveConn.Status
			cached.LocalIP = liveConn.LocalIP
			cached.ErrorMessage = liveConn.ErrorMessage
			cached.UpdatedAt = liveConn.UpdatedAt
		}
		m.mu.Unlock()
		return liveConn, nil
	}

	_ = err
	m.mu.RLock()
	result := *m.connections[connID]
	result.Duration = time.Since(m.connections[connID].ConnectedAt)
	m.mu.RUnlock()
	return &result, nil
}

// ListConnections returns all active VPN connections.
func (m *Manager) ListConnections() []VPNConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conns := make([]VPNConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		result := *conn
		result.Duration = time.Since(conn.ConnectedAt)
		conns = append(conns, result)
	}
	return conns
}

// ==================== Auto-reconnect ====================

// SetAutoReconnect enables or disables auto-reconnect.
func (m *Manager) SetAutoReconnect(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoReconnect = enabled
}

// IsAutoReconnectEnabled returns whether auto-reconnect is enabled.
func (m *Manager) IsAutoReconnectEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.autoReconnect
}

// ==================== Failover ====================

// SetFailoverEnabled enables or disables failover.
func (m *Manager) SetFailoverEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failoverEnabled = enabled
}

// IsFailoverEnabled returns whether failover is enabled.
func (m *Manager) IsFailoverEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failoverEnabled
}

// GetFailoverProfile returns the failover profile for a given profile.
func (m *Manager) GetFailoverProfile(profileID string) (*VPNProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find a different profile with the same protocol as failover
	profile, exists := m.profiles[profileID]
	if !exists {
		return nil, fmt.Errorf("profile not found: %s", profileID)
	}

	for _, p := range m.profiles {
		if p.ID != profileID && p.Protocol == profile.Protocol && p.Enabled {
			result := *p
			return &result, nil
		}
	}

	return nil, fmt.Errorf("no failover profile available")
}

// ==================== Traffic ====================

// GetTrafficMonitor returns the traffic monitor.
func (m *Manager) GetTrafficMonitor() *TrafficMonitor {
	return m.traffic
}

// GetTrafficStats returns traffic stats for a connection.
func (m *Manager) GetTrafficStats(connID string) (*TrafficStats, error) {
	return m.traffic.GetTrafficStats(connID)
}

// GetTotalTraffic returns total traffic across all connections.
func (m *Manager) GetTotalTraffic() TrafficStats {
	return m.traffic.GetTotalTraffic()
}

// ==================== Status ====================

// GetStatus returns the overall VPN client manager status.
func (m *Manager) GetStatus() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conns := make([]VPNConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		result := *conn
		result.Duration = time.Since(conn.ConnectedAt)
		conns = append(conns, result)
	}

	return ManagerStatus{
		ActiveConnections: len(m.connections),
		TotalProfiles:     len(m.profiles),
		Connections:       conns,
		DefaultProfile:    m.defaultProfile,
		FailoverEnabled:   m.failoverEnabled,
		AutoReconnect:     m.autoReconnect,
		Uptime:            time.Since(m.startTime),
	}
}

// ==================== Protocol Clients ====================

// GetOpenVPNClient returns the OpenVPN client manager.
func (m *Manager) GetOpenVPNClient() *OpenVPNClient {
	return m.openvpn
}

// GetWireGuardClient returns the WireGuard client manager.
func (m *Manager) GetWireGuardClient() *WireGuardClient {
	return m.wireguard
}

// GetL2TPClient returns the L2TP/IPSec client manager.
func (m *Manager) GetL2TPClient() *L2TPClient {
	return m.l2tp
}

// ==================== Connection Attempt ====================

// attemptReconnection attempts to reconnect a failed connection.
func (m *Manager) attemptReconnection(connID string) {
	if !m.autoReconnect {
		return
	}

	m.mu.RLock()
	conn, exists := m.connections[connID]
	if !exists {
		m.mu.RUnlock()
		return
	}
	profileID := conn.ProfileID
	m.mu.RUnlock()

	// Update status to reconnecting
	m.mu.Lock()
	if conn, ok := m.connections[connID]; ok {
		conn.Status = StatusReconnecting
		conn.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	// Wait before reconnecting
	time.Sleep(5 * time.Second)

	// Try to reconnect
	_, err := m.Connect(ConnectRequest{ProfileID: profileID})
	if err != nil {
		m.mu.Lock()
		if conn, ok := m.connections[connID]; ok {
			conn.Status = StatusError
			conn.ErrorMessage = fmt.Sprintf("reconnection failed: %v", err)
			conn.UpdatedAt = time.Now()
		}
		m.mu.Unlock()
	}
}
