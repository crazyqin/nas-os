package vpnclient

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// L2TPClient manages L2TP/IPSec client connections and configurations.
type L2TPClient struct {
	mu          sync.RWMutex
	configs     map[string]*L2TPConfig
	connections map[string]*VPNConnection
}

// NewL2TPClient creates a new L2TP/IPSec client manager.
func NewL2TPClient() *L2TPClient {
	return &L2TPClient{
		configs:     make(map[string]*L2TPConfig),
		connections: make(map[string]*VPNConnection),
	}
}

// ImportConfig imports an L2TP/IPSec configuration.
func (c *L2TPClient) ImportConfig(profileID string, config *L2TPConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.configs[profileID] = config
	return nil
}

// ExportConfig exports an L2TP/IPSec configuration.
func (c *L2TPClient) ExportConfig(profileID string) (*L2TPConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return nil, fmt.Errorf("config not found for profile: %s", profileID)
	}

	result := *config
	return &result, nil
}

// GetConfig returns the L2TP/IPSec configuration for a profile.
func (c *L2TPClient) GetConfig(profileID string) (*L2TPConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return nil, fmt.Errorf("config not found for profile: %s", profileID)
	}

	result := *config
	return &result, nil
}

// UpdateConfig updates an L2TP/IPSec configuration.
func (c *L2TPClient) UpdateConfig(profileID string, config *L2TPConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.configs[profileID] = config
	return nil
}

// DeleteConfig deletes an L2TP/IPSec configuration.
func (c *L2TPClient) DeleteConfig(profileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.configs[profileID]; !exists {
		return fmt.Errorf("config not found for profile: %s", profileID)
	}

	delete(c.configs, profileID)
	return nil
}

// Connect establishes an L2TP/IPSec connection using the specified profile.
func (c *L2TPClient) Connect(profile *VPNProfile) (*VPNConnection, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already connected
	for _, conn := range c.connections {
		if conn.ProfileID == profile.ID && conn.Status == StatusConnected {
			return nil, fmt.Errorf("already connected to profile: %s", profile.ID)
		}
	}

	// Get or create config
	config, exists := c.configs[profile.ID]
	if !exists {
		config = &L2TPConfig{
			ServerAddr:  profile.ServerAddr,
			ServerPort:  profile.ServerPort,
			Username:    profile.Username,
			Password:    profile.Password,
			PPPAuthType: "mschap-v2",
			MTU:         1400,
			MRU:         1400,
			IdleTimeout: 0,
			DefRoute:    true,
			IPSecProto:  "ikev2",
			IPSecIKE:    "aes256-sha2_256-modp2048!",
			IPSecESP:    "aes256-sha2_256!",
		}
		c.configs[profile.ID] = config
	}

	connID := fmt.Sprintf("l2tp_%s_%d", profile.ID, time.Now().UnixNano())
	conn := &VPNConnection{
		ID:          connID,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Protocol:    ProtocolL2TP,
		Status:      StatusConnecting,
		LocalIP:     fmt.Sprintf("192.168.1.%d", len(c.connections)+100),
		RemoteIP:    profile.ServerAddr,
		Gateway:     "192.168.1.1",
		DNS:         []string{"8.8.8.8", "8.8.4.4"},
		ConnectedAt: time.Now(),
		UpdatedAt:   time.Now(),
		Traffic:     TrafficStats{UpdatedAt: time.Now()},
	}

	c.connections[connID] = conn

	// Simulate connection establishment
	go c.establishConnection(connID)

	result := *conn
	return &result, nil
}

// Disconnect terminates an L2TP/IPSec connection.
func (c *L2TPClient) Disconnect(connID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.connections[connID]
	if !exists {
		return fmt.Errorf("connection not found: %s", connID)
	}

	if conn.Status != StatusConnected && conn.Status != StatusConnecting {
		return fmt.Errorf("connection is not active: %s", conn.Status)
	}

	conn.Status = StatusDisconnected
	conn.UpdatedAt = time.Now()
	delete(c.connections, connID)
	return nil
}

// GetConnection returns an active L2TP/IPSec connection.
func (c *L2TPClient) GetConnection(connID string) (*VPNConnection, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conn, exists := c.connections[connID]
	if !exists {
		return nil, fmt.Errorf("connection not found: %s", connID)
	}

	result := *conn
	result.Duration = time.Since(conn.ConnectedAt)
	return &result, nil
}

// ListConnections returns all active L2TP/IPSec connections.
func (c *L2TPClient) ListConnections() []VPNConnection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conns := make([]VPNConnection, 0, len(c.connections))
	for _, conn := range c.connections {
		result := *conn
		result.Duration = time.Since(conn.ConnectedAt)
		conns = append(conns, result)
	}
	return conns
}

// SetPSK sets the pre-shared key for IPSec authentication.
func (c *L2TPClient) SetPSK(profileID, psk string) error {
	if psk == "" {
		return fmt.Errorf("PSK cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	config, exists := c.configs[profileID]
	if !exists {
		config = &L2TPConfig{
			ServerAddr:  "0.0.0.0",
			ServerPort:  1701,
			PPPAuthType: "mschap-v2",
			MTU:         1400,
			MRU:         1400,
			IPSecProto:  "ikev2",
		}
		c.configs[profileID] = config
	}

	config.PSK = psk
	return nil
}

// GenerateConfig generates L2TP/IPSec configuration files.
func (c *L2TPClient) GenerateConfig(profileID string) (l2tpConfig, ipsecConfig string, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return "", "", fmt.Errorf("config not found for profile: %s", profileID)
	}

	l2tpConfig = generateL2TPConfig(config)
	ipsecConfig = generateIPSecConfig(config)
	return l2tpConfig, ipsecConfig, nil
}

// ValidateConfig validates an L2TP/IPSec configuration.
func (c *L2TPClient) ValidateConfig(config *L2TPConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.ServerAddr == "" {
		return fmt.Errorf("server address is required")
	}

	if config.Username == "" {
		return fmt.Errorf("username is required")
	}

	if config.Password == "" {
		return fmt.Errorf("password is required")
	}

	if config.PSK == "" {
		return fmt.Errorf("pre-shared key is required")
	}

	// Validate PPP auth type
	validAuthTypes := map[string]bool{
		"pap":       true,
		"chap":      true,
		"mschap-v2": true,
	}
	if !validAuthTypes[config.PPPAuthType] {
		return fmt.Errorf("invalid PPP auth type: %s", config.PPPAuthType)
	}

	// Validate IPSec protocol
	validIPSecProtos := map[string]bool{
		"ikev1": true,
		"ikev2": true,
	}
	if !validIPSecProtos[config.IPSecProto] {
		return fmt.Errorf("invalid IPSec protocol: %s", config.IPSecProto)
	}

	return nil
}

// establishConnection simulates establishing an L2TP/IPSec connection.
func (c *L2TPClient) establishConnection(connID string) {
	time.Sleep(1 * time.Second)

	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.connections[connID]
	if !exists {
		return
	}

	conn.Status = StatusConnected
	conn.UpdatedAt = time.Now()
}

// generateL2TPConfig generates xl2tpd configuration content.
func generateL2TPConfig(config *L2TPConfig) string {
	var sb strings.Builder

	sb.WriteString("[lac vpn-connection]\n")
	sb.WriteString(fmt.Sprintf("lns = %s\n", config.ServerAddr))
	sb.WriteString("ppp debug = yes\n")
	sb.WriteString(fmt.Sprintf("pppoptfile = /etc/ppp/options.l2tpd.client\n"))
	sb.WriteString("length bit = yes\n")

	if config.MTU > 0 {
		sb.WriteString(fmt.Sprintf("mtu = %d\n", config.MTU))
	}
	if config.MRU > 0 {
		sb.WriteString(fmt.Sprintf("mru = %d\n", config.MRU))
	}

	return sb.String()
}

// generateIPSecConfig generates strongSwan IPSec configuration content.
func generateIPSecConfig(config *L2TPConfig) string {
	var sb strings.Builder

	sb.WriteString("config setup\n")
	sb.WriteString("    uniqueids=no\n\n")

	sb.WriteString("conn %default\n")
	sb.WriteString(fmt.Sprintf("    keyexchange=%s\n", config.IPSecProto))
	sb.WriteString("    authby=secret\n")
	sb.WriteString("    type=transport\n\n")

	sb.WriteString("conn vpn-connection\n")
	sb.WriteString(fmt.Sprintf("    left=%%defaultroute\n"))
	sb.WriteString(fmt.Sprintf("    leftprotoport=17/1701\n"))
	sb.WriteString(fmt.Sprintf("    right=%s\n", config.ServerAddr))
	sb.WriteString(fmt.Sprintf("    rightprotoport=17/1701\n"))
	sb.WriteString("    auto=add\n")

	if config.IPSecIKE != "" {
		sb.WriteString(fmt.Sprintf("    ike=%s\n", config.IPSecIKE))
	}
	if config.IPSecESP != "" {
		sb.WriteString(fmt.Sprintf("    esp=%s\n", config.IPSecESP))
	}

	return sb.String()
}

// GenerateOptionsFile generates PPP options file content.
func (c *L2TPClient) GenerateOptionsFile(profileID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return "", fmt.Errorf("config not found for profile: %s", profileID)
	}

	var sb strings.Builder

	sb.WriteString("ipcp-accept-local\n")
	sb.WriteString("ipcp-accept-remote\n")
	sb.WriteString(fmt.Sprintf("name %s\n", config.Username))
	sb.WriteString("refuse-eap\n")
	sb.WriteString("require-mschap-v2\n")
	sb.WriteString("noccp\n")
	sb.WriteString("noauth\n")
	sb.WriteString("idle 1800\n")
	sb.WriteString("mtu 1410\n")
	sb.WriteString("mru 1410\n")
	sb.WriteString("nodefaultroute\n")
	sb.WriteString("usepeerdns\n")
	sb.WriteString("connect-delay 5000\n")

	if strings.Contains(config.PPPAuthType, "mschap") {
		sb.WriteString("require-mschap-v2\n")
	}

	return sb.String(), nil
}
