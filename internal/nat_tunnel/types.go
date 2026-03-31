// Package nat_tunnel provides NAT traversal and remote access functionality
// 对标飞牛fnOS FN Connect免费内网穿透服务
package nat_tunnel

// TunnelType defines the type of NAT traversal tunnel
type TunnelType string

const (
	TunnelTypeFRP   TunnelType = "frp"   // Fast Reverse Proxy
	TunnelTypeNPS   TunnelType = "nps"   // Nps proxy
	TunnelTypeCloud TunnelType = "cloud" // Cloudflare Tunnel
)

// TunnelStatus defines tunnel connection status
type TunnelStatus string

const (
	StatusConnected    TunnelStatus = "connected"
	StatusDisconnected TunnelStatus = "disconnected"
	StatusConnecting   TunnelStatus = "connecting"
	StatusError        TunnelStatus = "error"
)

// TunnelConfig represents tunnel configuration
type TunnelConfig struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Type         TunnelType  `json:"type"`
	Status       TunnelStatus `json:"status"`
	LocalPort    int         `json:"local_port"`
	RemotePort   int         `json:"remote_port,omitempty"`
	RemoteAddr   string      `json:"remote_addr,omitempty"`
	PublicURL    string      `json:"public_url,omitempty"`
	ServerAddr   string      `json:"server_addr,omitempty"`
	ServerPort   int         `json:"server_port,omitempty"`
	Token        string      `json:"token,omitempty"` // Authentication token
	EnableHTTPS  bool        `json:"enable_https"`
	CreatedAt    int64       `json:"created_at"`
	UpdatedAt    int64       `json:"updated_at"`
}

// TunnelService defines tunnel service interface
type TunnelService interface {
	// Create creates a new tunnel
	Create(config TunnelConfig) error
	// Start starts the tunnel connection
	Start(id string) error
	// Stop stops the tunnel connection
	Stop(id string) error
	// Delete deletes a tunnel
	Delete(id string) error
	// GetStatus gets tunnel status
	GetStatus(id string) (TunnelStatus, error)
	// List lists all tunnels
	List() ([]TunnelConfig, error)
	// GetPublicURL gets the public access URL
	GetPublicURL(id string) (string, error)
}

// TunnelManager manages multiple tunnel services
type TunnelManager struct {
	tunnels map[string]*TunnelConfig
	service TunnelService
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(service TunnelService) *TunnelManager {
	return &TunnelManager{
		tunnels: make(map[string]*TunnelConfig),
		service: service,
	}
}

// CreateTunnel creates a new tunnel
func (m *TunnelManager) CreateTunnel(config TunnelConfig) error {
	config.CreatedAt = currentTime()
	config.UpdatedAt = config.CreatedAt
	config.Status = StatusDisconnected
	
	if err := m.service.Create(config); err != nil {
		return err
	}
	
	m.tunnels[config.ID] = &config
	return nil
}

// StartTunnel starts a tunnel
func (m *TunnelManager) StartTunnel(id string) error {
	if tunnel, ok := m.tunnels[id]; ok {
		if err := m.service.Start(id); err != nil {
			tunnel.Status = StatusError
			return err
		}
		tunnel.Status = StatusConnected
		tunnel.UpdatedAt = currentTime()
	}
	return nil
}

// GetTunnel gets tunnel by ID
func (m *TunnelManager) GetTunnel(id string) *TunnelConfig {
	return m.tunnels[id]
}

// currentTime returns current unix timestamp
func currentTime() int64 {
	return 0 // Will be replaced with time.Now().Unix()
}