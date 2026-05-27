// Package vpnclient implements VPN client management for NAS-OS,
// supporting OpenVPN, WireGuard, and L2TP/IPSec protocols.
package vpnclient

import (
	"fmt"
	"net"
	"time"
)

// Protocol represents the VPN protocol type.
type Protocol string

const (
	// ProtocolOpenVPN is the OpenVPN protocol.
	ProtocolOpenVPN Protocol = "openvpn"
	// ProtocolWireGuard is the WireGuard protocol.
	ProtocolWireGuard Protocol = "wireguard"
	// ProtocolL2TP is the L2TP/IPSec protocol.
	ProtocolL2TP Protocol = "l2tp"
)

// ConnectionStatus represents the status of a VPN connection.
type ConnectionStatus string

const (
	// StatusDisconnected means the connection is inactive.
	StatusDisconnected ConnectionStatus = "disconnected"
	// StatusConnecting means the connection is being established.
	StatusConnecting ConnectionStatus = "connecting"
	// StatusConnected means the connection is active.
	StatusConnected ConnectionStatus = "connected"
	// StatusReconnecting means the connection is reconnecting.
	StatusReconnecting ConnectionStatus = "reconnecting"
	// StatusError means the connection encountered an error.
	StatusError ConnectionStatus = "error"
)

// AuthType represents the authentication method.
type AuthType string

const (
	// AuthCertificate uses certificate-based authentication.
	AuthCertificate AuthType = "certificate"
	// AuthPassword uses password-based authentication.
	AuthPassword AuthType = "password"
	// AuthPSK uses pre-shared key authentication.
	AuthPSK AuthType = "psk"
	// AuthBoth uses both certificate and password.
	AuthBoth AuthType = "both"
)

// VPNProfile represents a VPN connection profile.
type VPNProfile struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Protocol    Protocol         `json:"protocol"`
	ServerAddr  string           `json:"server_addr"`
	ServerPort  int              `json:"server_port"`
	AuthType    AuthType         `json:"auth_type"`
	Username    string           `json:"username,omitempty"`
	Password    string           `json:"password,omitempty"`
	CertFile    string           `json:"cert_file,omitempty"`
	KeyFile     string           `json:"key_file,omitempty"`
	CAFile      string           `json:"ca_file,omitempty"`
	ConfigFile  string           `json:"config_file,omitempty"`
	AutoConnect bool             `json:"auto_connect"`
	Enabled     bool             `json:"enabled"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// VPNConnection represents an active VPN connection.
type VPNConnection struct {
	ID           string           `json:"id"`
	ProfileID    string           `json:"profile_id"`
	ProfileName  string           `json:"profile_name"`
	Protocol     Protocol         `json:"protocol"`
	Status       ConnectionStatus `json:"status"`
	LocalIP      string           `json:"local_ip,omitempty"`
	RemoteIP     string           `json:"remote_ip,omitempty"`
	Gateway      string           `json:"gateway,omitempty"`
	DNS          []string         `json:"dns,omitempty"`
	ConnectedAt  time.Time        `json:"connected_at,omitempty"`
	Duration     time.Duration    `json:"duration"`
	Traffic      TrafficStats     `json:"traffic"`
	ErrorMessage string           `json:"error_message,omitempty"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// TrafficStats holds traffic statistics for a connection.
type TrafficStats struct {
	RxBytes   int64     `json:"rx_bytes"`
	TxBytes   int64     `json:"tx_bytes"`
	RxPackets int64     `json:"rx_packets"`
	TxPackets int64     `json:"tx_packets"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrafficSnapshot represents a point-in-time traffic measurement.
type TrafficSnapshot struct {
	Timestamp  time.Time    `json:"timestamp"`
	RxBytes    int64        `json:"rx_bytes"`
	TxBytes    int64        `json:"tx_bytes"`
	RxRate     float64      `json:"rx_rate"` // bytes per second
	TxRate     float64      `json:"tx_rate"` // bytes per second
	Connection string       `json:"connection_id"`
}

// TrafficHistory holds historical traffic data.
type TrafficHistory struct {
	ProfileID  string            `json:"profile_id"`
	Period     string            `json:"period"` // "hour", "day", "week", "month"
	Snapshots  []TrafficSnapshot `json:"snapshots"`
	TotalRx    int64             `json:"total_rx"`
	TotalTx    int64             `json:"total_tx"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
}

// TrafficAlert represents a traffic threshold alert.
type TrafficAlert struct {
	ID          string    `json:"id"`
	ProfileID   string    `json:"profile_id"`
	Threshold   int64     `json:"threshold"`   // bytes
	Direction   string    `json:"direction"`    // "rx", "tx", "both"
	Period      string    `json:"period"`       // "hour", "day", "month"
	Enabled     bool      `json:"enabled"`
	Triggered   bool      `json:"triggered"`
	TriggeredAt time.Time `json:"triggered_at,omitempty"`
	LastValue   int64     `json:"last_value"`
	CreatedAt   time.Time `json:"created_at"`
}

// TunnelState represents the state of a VPN tunnel.
type TunnelState struct {
	ProfileID    string           `json:"profile_id"`
	Status       ConnectionStatus `json:"status"`
	LocalAddr    string           `json:"local_addr"`
	RemoteAddr   string           `json:"remote_addr"`
	MTU          int              `json:"mtu"`
	TxQueueLen   int              `json:"tx_queue_len"`
	Flags        []string         `json:"flags"`
	LastActivity time.Time        `json:"last_activity"`
}

// OpenVPNConfig represents OpenVPN client configuration.
type OpenVPNConfig struct {
	RemoteAddr   string   `json:"remote_addr"`
	RemotePort   int      `json:"remote_port"`
	Protocol     string   `json:"protocol"` // "udp" or "tcp"
	DevType      string   `json:"dev_type"` // "tun" or "tap"
	Cipher       string   `json:"cipher"`
	AuthDigest   string   `json:"auth_digest"`
	CompLZO      bool     `json:"comp_lzo"`
	KeepAlive    string   `json:"keep_alive"`
	ResolvRetry  string   `json:"resolv_retry"`
	Verb         int      `json:"verb"`
	CertContent  string   `json:"cert_content,omitempty"`
	KeyContent   string   `json:"key_content,omitempty"`
	CAContent    string   `json:"ca_content,omitempty"`
	TLSAuth      string   `json:"tls_auth,omitempty"`
	ExtraOptions []string `json:"extra_options,omitempty"`
}

// WireGuardConfig represents WireGuard client configuration.
type WireGuardConfig struct {
	PrivateKey    string   `json:"private_key"`
	Address       string   `json:"address"`
	DNS           []string `json:"dns"`
	MTU           int      `json:"mtu"`
	PublicKey      string   `json:"public_key"`
	Endpoint      string   `json:"endpoint"`
	AllowedIPs    []string `json:"allowed_ips"`
	Keepalive     int      `json:"keepalive"`
	PresharedKey  string   `json:"preshared_key,omitempty"`
}

// L2TPConfig represents L2TP/IPSec client configuration.
type L2TPConfig struct {
	ServerAddr   string `json:"server_addr"`
	ServerPort   int    `json:"server_port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	PSK          string `json:"psk"` // Pre-shared key for IPSec
	PPPAuthType  string `json:"ppp_auth_type"` // "pap", "chap", "mschap-v2"
	MTU          int    `json:"mtu"`
	MRU          int    `json:"mru"`
	IdleTimeout  int    `json:"idle_timeout"`
	DefRoute     bool   `json:"def_route"`
	IPSecProto   string `json:"ipsec_proto"` // "ikev1", "ikev2"
	IPSecIKE     string `json:"ipsec_ike"`   // IKE cipher suite
	IPSecESP     string `json:"ipsec_esp"`   // ESP cipher suite
}

// ManagerStatus represents the overall VPN client manager status.
type ManagerStatus struct {
	ActiveConnections int            `json:"active_connections"`
	TotalProfiles     int            `json:"total_profiles"`
	Connections       []VPNConnection `json:"connections"`
	DefaultProfile    string         `json:"default_profile,omitempty"`
	FailoverEnabled   bool           `json:"failover_enabled"`
	AutoReconnect     bool           `json:"auto_reconnect"`
	Uptime            time.Duration  `json:"uptime"`
}

// CreateProfileRequest is the request to create a VPN profile.
type CreateProfileRequest struct {
	Name        string            `json:"name"`
	Protocol    Protocol          `json:"protocol"`
	ServerAddr  string            `json:"server_addr"`
	ServerPort  int               `json:"server_port"`
	AuthType    AuthType          `json:"auth_type"`
	Username    string            `json:"username,omitempty"`
	Password    string            `json:"password,omitempty"`
	ConfigFile  string            `json:"config_file,omitempty"`
	AutoConnect bool              `json:"auto_connect"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UpdateProfileRequest is the request to update a VPN profile.
type UpdateProfileRequest struct {
	Name        *string           `json:"name,omitempty"`
	ServerAddr  *string           `json:"server_addr,omitempty"`
	ServerPort  *int              `json:"server_port,omitempty"`
	Username    *string           `json:"username,omitempty"`
	Password    *string           `json:"password,omitempty"`
	AutoConnect *bool             `json:"auto_connect,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ConnectRequest is the request to establish a VPN connection.
type ConnectRequest struct {
	ProfileID string `json:"profile_id"`
}

// DisconnectRequest is the request to terminate a VPN connection.
type DisconnectRequest struct {
	ConnectionID string `json:"connection_id"`
}

// Validate validates the CreateProfileRequest.
func (r *CreateProfileRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if r.Protocol == "" {
		return fmt.Errorf("protocol is required")
	}
	if r.ServerAddr == "" {
		return fmt.Errorf("server address is required")
	}
	if r.ServerPort <= 0 || r.ServerPort > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	switch r.Protocol {
	case ProtocolOpenVPN, ProtocolWireGuard, ProtocolL2TP:
		// valid
	default:
		return fmt.Errorf("unsupported protocol: %s", r.Protocol)
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

// validateIPNetwork validates an IP network string like "10.0.0.1/24".
func validateIPNetwork(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}
	return nil
}

// calculateRate calculates bytes per second between two snapshots.
func calculateRate(prev, curr int64, duration time.Duration) float64 {
	if duration.Seconds() <= 0 {
		return 0
	}
	return float64(curr-prev) / duration.Seconds()
}
