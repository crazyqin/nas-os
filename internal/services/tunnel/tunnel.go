// Package tunnel implements NAT traversal service for remote access
package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TunnelType defines tunnel connection types
type TunnelType string

const (
	TypeFRP        TunnelType = "frp"        // FRP穿透
	TypeCloudflare TunnelType = "cloudflare" // Cloudflare Tunnel
	TypeWireGuard  TunnelType = "wireguard"  // WireGuard VPN
	TypeCustom     TunnelType = "custom"     // 自建服务
)

// TunnelStatus defines tunnel status
type TunnelStatus string

const (
	StatusConnected    TunnelStatus = "connected"
	StatusDisconnected TunnelStatus = "disconnected"
	StatusConnecting   TunnelStatus = "connecting"
	StatusError        TunnelStatus = "error"
)

// TunnelConfig defines tunnel configuration
type TunnelConfig struct {
	ID            string     `json:"id"`
	Type          TunnelType `json:"type"`
	Name          string     `json:"name"`
	ServerAddr    string     `json:"server_addr"` // 服务器地址
	ServerPort    int        `json:"server_port"` // 服务器端口
	LocalPort     int        `json:"local_port"`  // 本地端口
	Token         string     `json:"token"`       // 认证Token
	Enabled       bool       `json:"enabled"`
	AutoReconnect bool       `json:"auto_reconnect"` // 自动重连
	Timeout       int        `json:"timeout"`        // 超时秒数
}

// TunnelConnection represents an active tunnel
type TunnelConnection struct {
	Config      *TunnelConfig
	Status      TunnelStatus
	PublicURL   string        `json:"public_url"` // 公网访问地址
	Uptime      time.Duration `json:"uptime"`     // 连接时长
	BytesIn     int64         `json:"bytes_in"`   // 入流量
	BytesOut    int64         `json:"bytes_out"`  // 出流量
	LastError   string        `json:"last_error"`
	ConnectedAt time.Time     `json:"connected_at"`
}

// Service manages tunnel connections
type Service struct {
	mu        sync.RWMutex
	tunnels   map[string]*TunnelConnection
	frpClient *FRPClient
	cfClient  *CloudflareClient
	wgClient  *WireGuardClient
}

// NewService creates a new tunnel service
func NewService() *Service {
	return &Service{
		tunnels: make(map[string]*TunnelConnection),
	}
}

// CreateTunnel creates a new tunnel
func (s *Service) CreateTunnel(ctx context.Context, config *TunnelConfig) (*TunnelConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config.ID == "" {
		config.ID = fmt.Sprintf("tunnel-%d", time.Now().UnixNano())
	}

	conn := &TunnelConnection{
		Config: config,
		Status: StatusConnecting,
	}

	switch config.Type {
	case TypeFRP:
		conn.PublicURL = fmt.Sprintf("https://%s:%d", config.ServerAddr, config.ServerPort)
	case TypeCloudflare:
		conn.PublicURL = fmt.Sprintf("https://%s.trycloudflare.com", config.Name)
	case TypeWireGuard:
		conn.PublicURL = fmt.Sprintf("wg://%s:%d", config.ServerAddr, config.ServerPort)
	}

	s.tunnels[config.ID] = conn
	return conn, nil
}

// GetTunnel retrieves a tunnel by ID
func (s *Service) GetTunnel(id string) (*TunnelConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, exists := s.tunnels[id]
	if !exists {
		return nil, fmt.Errorf("tunnel not found: %s", id)
	}
	return conn, nil
}

// Connect establishes tunnel connection
func (s *Service) Connect(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.tunnels[id]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	conn.Status = StatusConnected
	conn.ConnectedAt = time.Now()
	return nil
}

// Disconnect closes tunnel connection
func (s *Service) Disconnect(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.tunnels[id]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	conn.Status = StatusDisconnected
	return nil
}

// ListTunnels returns all tunnels
func (s *Service) ListTunnels() []*TunnelConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TunnelConnection, 0, len(s.tunnels))
	for _, conn := range s.tunnels {
		result = append(result, conn)
	}
	return result
}

// FRPClient for FRP protocol
type FRPClient struct{}

// CloudflareClient for Cloudflare Tunnel
type CloudflareClient struct{}

// WireGuardClient for WireGuard VPN
type WireGuardClient struct{}
