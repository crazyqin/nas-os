package vpnclient

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"
)

// WireGuardClient manages WireGuard client connections and peers.
type WireGuardClient struct {
	mu          sync.RWMutex
	configs     map[string]*WireGuardConfig
	connections map[string]*VPNConnection
	keys        map[string]*KeyPair
}

// KeyPair represents a WireGuard key pair.
type KeyPair struct {
	PublicKey  string    `json:"public_key"`
	PrivateKey string    `json:"private_key"`
	CreatedAt  time.Time `json:"created_at"`
}

// Peer represents a WireGuard peer configuration.
type Peer struct {
	PublicKey     string       `json:"public_key"`
	Endpoint      string       `json:"endpoint,omitempty"`
	AllowedIPs    []string     `json:"allowed_ips"`
	Keepalive     int          `json:"keepalive,omitempty"`
	PresharedKey  string       `json:"preshared_key,omitempty"`
	Name          string       `json:"name,omitempty"`
	Enabled       bool         `json:"enabled"`
	LastHandshake time.Time    `json:"last_handshake,omitempty"`
	Traffic       TrafficStats `json:"traffic"`
}

// NewWireGuardClient creates a new WireGuard client manager.
func NewWireGuardClient() *WireGuardClient {
	return &WireGuardClient{
		configs:     make(map[string]*WireGuardConfig),
		connections: make(map[string]*VPNConnection),
		keys:        make(map[string]*KeyPair),
	}
}

// GenerateKeyPair generates a new WireGuard key pair.
func (c *WireGuardClient) GenerateKeyPair() (*KeyPair, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	privBytes := make([]byte, 32)
	if _, err := rand.Read(privBytes); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	privateKey := base64.StdEncoding.EncodeToString(privBytes)

	pubBytes := make([]byte, 32)
	if _, err := rand.Read(pubBytes); err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKey := base64.StdEncoding.EncodeToString(pubBytes)

	kp := &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  time.Now(),
	}

	c.keys[publicKey] = kp
	return kp, nil
}

// GeneratePresharedKey generates a new preshared key.
func (c *WireGuardClient) GeneratePresharedKey() (string, error) {
	pskBytes := make([]byte, 32)
	if _, err := rand.Read(pskBytes); err != nil {
		return "", fmt.Errorf("failed to generate preshared key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pskBytes), nil
}

// ImportConfig imports a WireGuard configuration.
func (c *WireGuardClient) ImportConfig(profileID, configContent string) (*WireGuardConfig, error) {
	if configContent == "" {
		return nil, fmt.Errorf("config content is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	config, err := parseWireGuardConfig(configContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	c.configs[profileID] = config
	return config, nil
}

// ExportConfig exports a WireGuard configuration to file content.
func (c *WireGuardClient) ExportConfig(profileID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return "", fmt.Errorf("config not found for profile: %s", profileID)
	}

	return generateWireGuardConfig(config), nil
}

// GetConfig returns the WireGuard configuration for a profile.
func (c *WireGuardClient) GetConfig(profileID string) (*WireGuardConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return nil, fmt.Errorf("config not found for profile: %s", profileID)
	}

	result := *config
	return &result, nil
}

// UpdateConfig updates a WireGuard configuration.
func (c *WireGuardClient) UpdateConfig(profileID string, config *WireGuardConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.configs[profileID] = config
	return nil
}

// DeleteConfig deletes a WireGuard configuration.
func (c *WireGuardClient) DeleteConfig(profileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.configs[profileID]; !exists {
		return fmt.Errorf("config not found for profile: %s", profileID)
	}

	delete(c.configs, profileID)
	return nil
}

// Connect establishes a WireGuard connection using the specified profile.
func (c *WireGuardClient) Connect(profile *VPNProfile) (*VPNConnection, error) {
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
		config = &WireGuardConfig{
			Address:    "10.0.0.2/32",
			DNS:        []string{"1.1.1.1", "1.0.0.1"},
			MTU:        1420,
			Endpoint:   fmt.Sprintf("%s:%d", profile.ServerAddr, profile.ServerPort),
			AllowedIPs: []string{"0.0.0.0/0"},
			Keepalive:  25,
		}
		c.configs[profile.ID] = config
	}

	connID := fmt.Sprintf("wg_%s_%d", profile.ID, time.Now().UnixNano())
	conn := &VPNConnection{
		ID:          connID,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Protocol:    ProtocolWireGuard,
		Status:      StatusConnecting,
		LocalIP:     "10.0.0.2",
		RemoteIP:    profile.ServerAddr,
		Gateway:     "10.0.0.1",
		DNS:         config.DNS,
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

// Disconnect terminates a WireGuard connection.
func (c *WireGuardClient) Disconnect(connID string) error {
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

// GetConnection returns an active WireGuard connection.
func (c *WireGuardClient) GetConnection(connID string) (*VPNConnection, error) {
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

// ListConnections returns all active WireGuard connections.
func (c *WireGuardClient) ListConnections() []VPNConnection {
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

// GeneratePeerConfig generates a WireGuard client configuration for a peer.
func (c *WireGuardClient) GeneratePeerConfig(privateKey, publicKey, endpoint string, allowedIPs []string) (string, error) {
	if privateKey == "" {
		return "", fmt.Errorf("private key is required")
	}
	if publicKey == "" {
		return "", fmt.Errorf("public key is required")
	}
	if endpoint == "" {
		return "", fmt.Errorf("endpoint is required")
	}

	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", privateKey))
	sb.WriteString("DNS = 1.1.1.1, 1.0.0.1\n\n")

	sb.WriteString("[Peer]\n")
	sb.WriteString(fmt.Sprintf("PublicKey = %s\n", publicKey))
	sb.WriteString(fmt.Sprintf("Endpoint = %s\n", endpoint))

	if len(allowedIPs) > 0 {
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(allowedIPs, ", ")))
	} else {
		sb.WriteString("AllowedIPs = 0.0.0.0/0\n")
	}

	sb.WriteString("PersistentKeepalive = 25\n")

	return sb.String(), nil
}

// GetKeyPair returns a key pair by public key.
func (c *WireGuardClient) GetKeyPair(publicKey string) (*KeyPair, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	kp, exists := c.keys[publicKey]
	if !exists {
		return nil, fmt.Errorf("key pair not found: %s", publicKey)
	}

	result := *kp
	return &result, nil
}

// ListKeyPairs returns all stored key pairs.
func (c *WireGuardClient) ListKeyPairs() []KeyPair {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]KeyPair, 0, len(c.keys))
	for _, kp := range c.keys {
		keys = append(keys, *kp)
	}
	return keys
}

// establishConnection simulates establishing a WireGuard connection.
func (c *WireGuardClient) establishConnection(connID string) {
	time.Sleep(200 * time.Millisecond)

	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.connections[connID]
	if !exists {
		return
	}

	conn.Status = StatusConnected
	conn.UpdatedAt = time.Now()
}

// parseWireGuardConfig parses a WireGuard configuration file content.
func parseWireGuardConfig(content string) (*WireGuardConfig, error) {
	config := &WireGuardConfig{
		MTU:       1420,
		Keepalive: 25,
	}

	lines := strings.Split(content, "\n")
	section := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch section {
		case "interface":
			switch strings.ToLower(key) {
			case "privatekey":
				config.PrivateKey = value
			case "address":
				config.Address = value
			case "dns":
				config.DNS = strings.Split(value, ",")
				for i := range config.DNS {
					config.DNS[i] = strings.TrimSpace(config.DNS[i])
				}
			case "mtu":
				fmt.Sscanf(value, "%d", &config.MTU)
			}
		case "peer":
			switch strings.ToLower(key) {
			case "publickey":
				config.PublicKey = value
			case "endpoint":
				config.Endpoint = value
			case "allowedips":
				config.AllowedIPs = strings.Split(value, ",")
				for i := range config.AllowedIPs {
					config.AllowedIPs[i] = strings.TrimSpace(config.AllowedIPs[i])
				}
			case "persistentkeepalive":
				fmt.Sscanf(value, "%d", &config.Keepalive)
			case "presharedkey":
				config.PresharedKey = value
			}
		}
	}

	return config, nil
}

// generateWireGuardConfig generates WireGuard configuration file content.
func generateWireGuardConfig(config *WireGuardConfig) string {
	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", config.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s\n", config.Address))

	if len(config.DNS) > 0 {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", strings.Join(config.DNS, ", ")))
	}

	if config.MTU > 0 {
		sb.WriteString(fmt.Sprintf("MTU = %d\n", config.MTU))
	}

	sb.WriteString("\n[Peer]\n")
	sb.WriteString(fmt.Sprintf("PublicKey = %s\n", config.PublicKey))
	sb.WriteString(fmt.Sprintf("Endpoint = %s\n", config.Endpoint))

	if len(config.AllowedIPs) > 0 {
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(config.AllowedIPs, ", ")))
	}

	if config.Keepalive > 0 {
		sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", config.Keepalive))
	}

	if config.PresharedKey != "" {
		sb.WriteString(fmt.Sprintf("PresharedKey = %s\n", config.PresharedKey))
	}

	return sb.String()
}
