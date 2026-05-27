package vpnclient

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// OpenVPNClient manages OpenVPN client connections and configurations.
type OpenVPNClient struct {
	mu       sync.RWMutex
	configs  map[string]*OpenVPNConfig
	connections map[string]*VPNConnection
	profiles map[string]*VPNProfile
}

// NewOpenVPNClient creates a new OpenVPN client manager.
func NewOpenVPNClient() *OpenVPNClient {
	return &OpenVPNClient{
		configs:     make(map[string]*OpenVPNConfig),
		connections: make(map[string]*VPNConnection),
		profiles:    make(map[string]*VPNProfile),
	}
}

// ImportConfig imports an OpenVPN configuration file content.
func (c *OpenVPNClient) ImportConfig(profileID, configContent string) (*OpenVPNConfig, error) {
	if configContent == "" {
		return nil, fmt.Errorf("config content is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	config, err := parseOpenVPNConfig(configContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	c.configs[profileID] = config
	return config, nil
}

// ExportConfig exports an OpenVPN configuration to file content.
func (c *OpenVPNClient) ExportConfig(profileID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return "", fmt.Errorf("config not found for profile: %s", profileID)
	}

	return generateOpenVPNConfig(config), nil
}

// GetConfig returns the OpenVPN configuration for a profile.
func (c *OpenVPNClient) GetConfig(profileID string) (*OpenVPNConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profileID]
	if !exists {
		return nil, fmt.Errorf("config not found for profile: %s", profileID)
	}

	result := *config
	return &result, nil
}

// UpdateConfig updates an OpenVPN configuration.
func (c *OpenVPNClient) UpdateConfig(profileID string, config *OpenVPNConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.configs[profileID] = config
	return nil
}

// DeleteConfig deletes an OpenVPN configuration.
func (c *OpenVPNClient) DeleteConfig(profileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.configs[profileID]; !exists {
		return fmt.Errorf("config not found for profile: %s", profileID)
	}

	delete(c.configs, profileID)
	return nil
}

// Connect establishes an OpenVPN connection using the specified profile.
func (c *OpenVPNClient) Connect(profile *VPNProfile) (*VPNConnection, error) {
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
		config = &OpenVPNConfig{
			RemoteAddr:  profile.ServerAddr,
			RemotePort:  profile.ServerPort,
			Protocol:    "udp",
			DevType:     "tun",
			Cipher:      "AES-256-GCM",
			AuthDigest:  "SHA256",
			KeepAlive:   "10 120",
			ResolvRetry: "infinite",
			Verb:        3,
		}
		c.configs[profile.ID] = config
	}

	connID := fmt.Sprintf("ovpn_%s_%d", profile.ID, time.Now().UnixNano())
	conn := &VPNConnection{
		ID:          connID,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Protocol:    ProtocolOpenVPN,
		Status:      StatusConnecting,
		LocalIP:     fmt.Sprintf("10.8.0.%d", len(c.connections)+2),
		RemoteIP:    profile.ServerAddr,
		Gateway:     "10.8.0.1",
		DNS:         []string{"8.8.8.8", "8.8.4.4"},
		ConnectedAt: time.Now(),
		UpdatedAt:   time.Now(),
		Traffic:     TrafficStats{UpdatedAt: time.Now()},
	}

	c.connections[connID] = conn
	c.profiles[profile.ID] = profile

	// Simulate connection establishment
	go c.establishConnection(connID)

	result := *conn
	return &result, nil
}

// Disconnect terminates an OpenVPN connection.
func (c *OpenVPNClient) Disconnect(connID string) error {
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

// GetConnection returns an active OpenVPN connection.
func (c *OpenVPNClient) GetConnection(connID string) (*VPNConnection, error) {
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

// ListConnections returns all active OpenVPN connections.
func (c *OpenVPNClient) ListConnections() []VPNConnection {
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

// SetCertificate sets the certificate content for a profile.
func (c *OpenVPNClient) SetCertificate(profileID, certContent, keyContent, caContent string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	config, exists := c.configs[profileID]
	if !exists {
		config = &OpenVPNConfig{
			RemoteAddr: "0.0.0.0",
			RemotePort: 1194,
			Protocol:   "udp",
			DevType:    "tun",
			Cipher:     "AES-256-GCM",
		}
		c.configs[profileID] = config
	}

	config.CertContent = certContent
	config.KeyContent = keyContent
	config.CAContent = caContent
	return nil
}

// GenerateClientConfig generates a complete OpenVPN client configuration.
func (c *OpenVPNClient) GenerateClientConfig(profile *VPNProfile) (string, error) {
	if profile == nil {
		return "", fmt.Errorf("profile cannot be nil")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[profile.ID]
	if !exists {
		return "", fmt.Errorf("config not found for profile: %s", profile.ID)
	}

	return generateOpenVPNConfig(config), nil
}

// establishConnection simulates establishing an OpenVPN connection.
func (c *OpenVPNClient) establishConnection(connID string) {
	time.Sleep(500 * time.Millisecond)

	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.connections[connID]
	if !exists {
		return
	}

	conn.Status = StatusConnected
	conn.UpdatedAt = time.Now()
}

// parseOpenVPNConfig parses an OpenVPN configuration file content.
func parseOpenVPNConfig(content string) (*OpenVPNConfig, error) {
	config := &OpenVPNConfig{
		Protocol:    "udp",
		DevType:     "tun",
		Cipher:      "AES-256-GCM",
		AuthDigest:  "SHA256",
		KeepAlive:   "10 120",
		ResolvRetry: "infinite",
		Verb:        3,
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		key := strings.ToLower(parts[0])
		if len(parts) == 1 {
			// Single value options like "comp-lzo", "nobind"
			switch key {
			case "comp-lzo":
				config.CompLZO = true
			}
			continue
		}

		value := parts[1]

		switch key {
		case "remote":
			config.RemoteAddr = value
			if len(parts) > 2 {
				fmt.Sscanf(parts[2], "%d", &config.RemotePort)
			}
		case "proto":
			config.Protocol = value
		case "dev":
			config.DevType = value
		case "cipher":
			config.Cipher = value
		case "auth":
			config.AuthDigest = value
		case "keepalive":
			config.KeepAlive = strings.Join(parts[1:], " ")
		case "resolv-retry":
			config.ResolvRetry = value
		case "verb":
			fmt.Sscanf(value, "%d", &config.Verb)
		case "comp-lzo":
			config.CompLZO = true
		}
	}

	return config, nil
}

// generateOpenVPNConfig generates OpenVPN configuration file content.
func generateOpenVPNConfig(config *OpenVPNConfig) string {
	var sb strings.Builder

	sb.WriteString("client\n")
	sb.WriteString(fmt.Sprintf("dev %s\n", config.DevType))
	sb.WriteString(fmt.Sprintf("proto %s\n", config.Protocol))
	sb.WriteString(fmt.Sprintf("remote %s %d\n", config.RemoteAddr, config.RemotePort))
	sb.WriteString("resolv-retry infinite\n")
	sb.WriteString("nobind\n")
	sb.WriteString("persist-key\n")
	sb.WriteString("persist-tun\n")
	sb.WriteString(fmt.Sprintf("cipher %s\n", config.Cipher))
	sb.WriteString(fmt.Sprintf("auth %s\n", config.AuthDigest))
	sb.WriteString(fmt.Sprintf("verb %d\n", config.Verb))

	if config.CompLZO {
		sb.WriteString("comp-lzo\n")
	}

	if config.KeepAlive != "" {
		sb.WriteString(fmt.Sprintf("keepalive %s\n", config.KeepAlive))
	}

	for _, opt := range config.ExtraOptions {
		sb.WriteString(fmt.Sprintf("%s\n", opt))
	}

	if config.CAContent != "" {
		sb.WriteString("\n<ca>\n")
		sb.WriteString(config.CAContent)
		sb.WriteString("\n</ca>\n")
	}

	if config.CertContent != "" {
		sb.WriteString("\n<cert>\n")
		sb.WriteString(config.CertContent)
		sb.WriteString("\n</cert>\n")
	}

	if config.KeyContent != "" {
		sb.WriteString("\n<key>\n")
		sb.WriteString(config.KeyContent)
		sb.WriteString("\n</key>\n")
	}

	if config.TLSAuth != "" {
		sb.WriteString("\n<tls-auth>\n")
		sb.WriteString(config.TLSAuth)
		sb.WriteString("\n</tls-auth>\n")
	}

	return sb.String()
}
