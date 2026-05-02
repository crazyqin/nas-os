// Package vpnserver implements VPN server management for NAS-OS,
// supporting both WireGuard and OpenVPN protocols.
package vpnserver

import (
	"fmt"
	"net"
	"time"
)

// Protocol represents the VPN protocol type.
type Protocol string

const (
	// ProtocolWireGuard is the WireGuard protocol.
	ProtocolWireGuard Protocol = "wireguard"
	// ProtocolOpenVPN is the OpenVPN protocol.
	ProtocolOpenVPN Protocol = "openvpn"
)

// InterfaceStatus represents the status of a VPN interface.
type InterfaceStatus string

const (
	// StatusRunning means the interface is active.
	StatusRunning InterfaceStatus = "running"
	// StatusStopped means the interface is stopped.
	StatusStopped InterfaceStatus = "stopped"
	// StatusError means the interface encountered an error.
	StatusError InterfaceStatus = "error"
)

// Permission represents the access level for VPN users.
type Permission string

const (
	// PermissionAllow grants VPN access.
	PermissionAllow Permission = "allow"
	// PermissionDeny denies VPN access.
	PermissionDeny Permission = "deny"
)

// WireGuardInterface represents a WireGuard network interface.
type WireGuardInterface struct {
	Name         string          `json:"name"`
	ListenPort   int             `json:"listen_port"`
	PrivateKey   string          `json:"private_key,omitempty"`
	PublicKey    string          `json:"public_key"`
	Address      string          `json:"address"`       // e.g. "10.0.0.1/24"
	DNS          []string        `json:"dns"`
	Status       InterfaceStatus `json:"status"`
	Peers        []WireGuardPeer `json:"peers"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	TrafficStats TrafficStats    `json:"traffic_stats"`
}

// WireGuardPeer represents a WireGuard peer (client).
type WireGuardPeer struct {
	PublicKey           string    `json:"public_key"`
	AllowedIPs          []string  `json:"allowed_ips"`
	Endpoint            string    `json:"endpoint,omitempty"`
	PersistentKeepalive int       `json:"persistent_keepalive,omitempty"`
	Name                string    `json:"name"`
	Enabled             bool      `json:"enabled"`
	LastHandshake       time.Time `json:"last_handshake,omitempty"`
	TrafficStats        TrafficStats `json:"traffic_stats"`
}

// TrafficStats holds traffic statistics for a connection.
type TrafficStats struct {
	RxBytes   int64     `json:"rx_bytes"`
	TxBytes   int64     `json:"tx_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OpenVPNConfig holds OpenVPN server configuration.
type OpenVPNConfig struct {
	Enabled      bool     `json:"enabled"`
	Port         int      `json:"port"`
	Protocol     string   `json:"protocol"` // "udp" or "tcp"
	Subnet       string   `json:"subnet"`   // e.g. "10.8.0.0"
	Netmask      string   `json:"netmask"`  // e.g. "255.255.255.0"
	DNS          []string `json:"dns"`
	MaxClients   int      `json:"max_clients"`
	KeepAlive    bool     `json:"keep_alive"`
	Compression  string   `json:"compression"`
	Cipher       string   `json:"cipher"`
	AuthType     string   `json:"auth_type"` // "certificate", "password", "both"
	Status       InterfaceStatus `json:"status"`
	ConnectedUsers int    `json:"connected_users"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// OpenVPNClient represents an OpenVPN client certificate/credential.
type OpenVPNClient struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CN          string    `json:"cn"` // Common Name in certificate
	Certificate string    `json:"certificate,omitempty"`
	PrivateKey  string    `json:"private_key,omitempty"`
	Enabled     bool      `json:"enabled"`
	Connected   bool      `json:"connected"`
	RemoteIP    string    `json:"remote_ip,omitempty"`
	ConnectedAt time.Time `json:"connected_at,omitempty"`
	TrafficStats TrafficStats `json:"traffic_stats"`
	CreatedAt   time.Time `json:"created_at"`
}

// VPNUser represents a user authorized for VPN access.
type VPNUser struct {
	ID          string            `json:"id"`
	Username    string            `json:"username"`
	Permission  Permission        `json:"permission"`
	Protocols   []Protocol        `json:"protocols"` // allowed protocols
	MaxDevices  int               `json:"max_devices"`
	Devices     []VPNDevice       `json:"devices"`
	TrafficLimit int64            `json:"traffic_limit"` // bytes, 0 = unlimited
	TrafficUsed int64             `json:"traffic_used"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// VPNDevice represents an authorized VPN device.
type VPNDevice struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	UserID       string    `json:"user_id"`
	Protocol     Protocol  `json:"protocol"`
	PublicKey    string    `json:"public_key,omitempty"` // for WireGuard
	Certificate  string    `json:"certificate,omitempty"` // for OpenVPN
	AssignedIP   string    `json:"assigned_ip"`
	Enabled      bool      `json:"enabled"`
	Connected    bool      `json:"connected"`
	ConnectedAt  time.Time `json:"connected_at,omitempty"`
	TrafficStats TrafficStats `json:"traffic_stats"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ConnectionSession represents an active VPN connection.
type ConnectionSession struct {
	ID          string        `json:"id"`
	UserID      string        `json:"username"`
	DeviceID    string        `json:"device_id"`
	Protocol    Protocol      `json:"protocol"`
	RemoteAddr  string        `json:"remote_addr"`
	AssignedIP  string        `json:"assigned_ip"`
	ConnectedAt time.Time     `json:"connected_at"`
	Duration    time.Duration `json:"duration"`
	TrafficStats TrafficStats `json:"traffic_stats"`
}

// DNSConfig holds DNS server configuration for VPN.
type DNSConfig struct {
	PrimaryDNS   string   `json:"primary_dns"`
	SecondaryDNS string   `json:"secondary_dns"`
	Domains      []string `json:"domains,omitempty"`
}

// NATConfig holds NAT/masquerade configuration for VPN.
type NATConfig struct {
	Enabled     bool     `json:"enabled"`
	Interface   string   `json:"interface"`    // e.g. "eth0"
	Subnets     []string `json:"subnets"`      // VPN subnets to masquerade
	Masquerade  bool     `json:"masquerade"`
	PortForward []PortForward `json:"port_forward,omitempty"`
}

// PortForward represents a port forwarding rule.
type PortForward struct {
	Protocol     string `json:"protocol"` // "tcp" or "udp"
	ExternalPort int    `json:"external_port"`
	InternalIP   string `json:"internal_ip"`
	InternalPort int    `json:"internal_port"`
	Description  string `json:"description,omitempty"`
}

// ServerStatus holds overall VPN server status.
type ServerStatus struct {
	WireGuard    *WireGuardInterface `json:"wireguard,omitempty"`
	OpenVPN      *OpenVPNConfig      `json:"openvpn,omitempty"`
	ActiveConns  int                 `json:"active_connections"`
	TotalUsers   int                 `json:"total_users"`
	TotalTraffic TrafficStats        `json:"total_traffic"`
	Uptime       time.Duration       `json:"uptime"`
}

// CreateWGInterfaceRequest is the request to create a WireGuard interface.
type CreateWGInterfaceRequest struct {
	Name       string   `json:"name"`
	ListenPort int      `json:"listen_port"`
	Address    string   `json:"address"`
	DNS        []string `json:"dns,omitempty"`
}

// AddWGPeerRequest is the request to add a WireGuard peer.
type AddWGPeerRequest struct {
	PublicKey           string   `json:"public_key"`
	Name                string   `json:"name"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive int      `json:"persistent_keepalive,omitempty"`
}

// UpdateOpenVPNRequest is the request to update OpenVPN configuration.
type UpdateOpenVPNRequest struct {
	Enabled     bool     `json:"enabled"`
	Port        int      `json:"port"`
	Protocol    string   `json:"protocol"`
	Subnet      string   `json:"subnet"`
	Netmask     string   `json:"netmask"`
	DNS         []string `json:"dns"`
	MaxClients  int      `json:"max_clients"`
	KeepAlive   bool     `json:"keep_alive"`
	Compression string   `json:"compression"`
	Cipher      string   `json:"cipher"`
	AuthType    string   `json:"auth_type"`
}

// CreateVPNUserRequest is the request to create a VPN user.
type CreateVPNUserRequest struct {
	Username     string            `json:"username"`
	Permission   Permission        `json:"permission"`
	Protocols    []Protocol        `json:"protocols"`
	MaxDevices   int               `json:"max_devices"`
	TrafficLimit int64             `json:"traffic_limit"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// AddDeviceRequest is the request to add a VPN device.
type AddDeviceRequest struct {
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
}

// UpdateNATRequest is the request to update NAT configuration.
type UpdateNATRequest struct {
	Enabled     bool           `json:"enabled"`
	Interface   string         `json:"interface"`
	Subnets     []string       `json:"subnets"`
	Masquerade  bool           `json:"masquerade"`
	PortForward []PortForward  `json:"port_forward,omitempty"`
}

// UpdateDNSRequest is the request to update DNS configuration.
type UpdateDNSRequest struct {
	PrimaryDNS   string   `json:"primary_dns"`
	SecondaryDNS string   `json:"secondary_dns"`
	Domains      []string `json:"domains,omitempty"`
}

// APIResponse is a standard API response envelope.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// generateKeyPlaceholder generates a placeholder key string for demo purposes.
func generateKeyPlaceholder(prefix string) string {
	return fmt.Sprintf("%s_key_placeholder", prefix)
}

// validateIPNetwork validates an IP network string like "10.0.0.1/24".
func validateIPNetwork(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}
	return nil
}

// validateIP validates an IP address string.
func validateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}
