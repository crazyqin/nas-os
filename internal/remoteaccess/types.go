// Package remoteaccess provides remote access functionality similar to Synology QuickConnect.
// It includes P2P traversal via STUN/TURN, relay services, dynamic DNS, port mapping,
// connection management, and secure authentication.
package remoteaccess

import (
	"sync"
	"time"
)

// ============================================================
// 连接模式
// ============================================================

// ConnectionMode represents the connection mode for remote access.
type ConnectionMode string

const (
	// ConnectionModeP2P uses direct P2P connection via STUN/TURN.
	ConnectionModeP2P ConnectionMode = "p2p"
	// ConnectionModeRelay uses relay server when P2P is unavailable.
	ConnectionModeRelay ConnectionMode = "relay"
	// ConnectionModeDirect uses direct connection with port mapping.
	ConnectionModeDirect ConnectionMode = "direct"
)

// ============================================================
// NAT 穿透类型
// ============================================================

// STUNConfig represents the STUN server configuration.
type STUNConfig struct {
	Enabled    bool     `json:"enabled"`
	Servers    []string `json:"servers"`     // STUN server addresses
	TimeoutSec int      `json:"timeout_sec"` // Connection timeout in seconds
}

// DefaultSTUNConfig returns default STUN configuration.
func DefaultSTUNConfig() STUNConfig {
	return STUNConfig{
		Enabled: true,
		Servers: []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
		},
		TimeoutSec: 5,
	}
}

// TURNConfig represents the TURN relay server configuration.
type TURNConfig struct {
	Enabled    bool   `json:"enabled"`
	Server     string `json:"server"`      // TURN server address
	Username   string `json:"username"`
	Password   string `json:"password"`
	Realm      string `json:"realm"`
	Transport  string `json:"transport"`   // "udp" or "tcp"
	TimeoutSec int    `json:"timeout_sec"`
}

// DefaultTURNConfig returns default TURN configuration.
func DefaultTURNConfig() TURNConfig {
	return TURNConfig{
		Enabled:    false,
		Transport:  "udp",
		TimeoutSec: 10,
	}
}

// NATType represents the type of NAT detected.
type NATType string

const (
	NATTypeNone         NATType = "none"         // No NAT (public IP)
	NATTypeFull         NATType = "full"         // Full Cone NAT
	NATTypeRestricted   NATType = "restricted"   // Restricted Cone NAT
	NATTypePortRestricted NATType = "port_restricted" // Port Restricted Cone NAT
	NATTypeSymmetric    NATType = "symmetric"    // Symmetric NAT
	NATTypeUnknown      NATType = "unknown"
)

// STUNResult represents the result of a STUN probe.
type STUNResult struct {
	NATType    NATType   `json:"nat_type"`
	PublicIP   string    `json:"public_ip"`
	PublicPort int       `json:"public_port"`
	LocalIP    string    `json:"local_ip"`
	LocalPort  int       `json:"local_port"`
	Server     string    `json:"server"`
	Timestamp  time.Time `json:"timestamp"`
}

// P2PSession represents a P2P connection session.
type P2PSession struct {
	ID          string      `json:"id"`
	PeerID      string      `json:"peer_id"`
	LocalAddr   string      `json:"local_addr"`
	RemoteAddr  string      `json:"remote_addr"`
	NATType     NATType     `json:"nat_type"`
	Status      string      `json:"status"` // "connecting", "connected", "failed", "closed"
	CreatedAt   time.Time   `json:"created_at"`
	ConnectedAt *time.Time  `json:"connected_at,omitempty"`
	ClosedAt    *time.Time  `json:"closed_at,omitempty"`
	BytesSent   int64       `json:"bytes_sent"`
	BytesRecv   int64       `json:"bytes_recv"`
}

// ============================================================
// 中继服务类型
// ============================================================

// RelayConfig represents the relay server configuration.
type RelayConfig struct {
	Enabled       bool   `json:"enabled"`
	Server        string `json:"server"`
	Port          int    `json:"port"`
	AuthKey       string `json:"auth_key"`
	MaxBandwidth  int64  `json:"max_bandwidth"`  // Max bandwidth in bytes/sec
	MaxSessions   int    `json:"max_sessions"`   // Max concurrent sessions
	IdleTimeoutSec int   `json:"idle_timeout_sec"`
}

// DefaultRelayConfig returns default relay configuration.
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		Enabled:        false,
		Port:           443,
		MaxBandwidth:   10 * 1024 * 1024, // 10 MB/s
		MaxSessions:    100,
		IdleTimeoutSec: 300,
	}
}

// RelaySession represents a relay connection session.
type RelaySession struct {
	ID          string     `json:"id"`
	ClientID    string     `json:"client_id"`
	ServerAddr  string     `json:"server_addr"`
	Status      string     `json:"status"` // "active", "idle", "closed"
	Bandwidth   int64      `json:"bandwidth"` // Current bandwidth usage
	BytesSent   int64      `json:"bytes_sent"`
	BytesRecv   int64      `json:"bytes_recv"`
	CreatedAt   time.Time  `json:"created_at"`
	LastActive  time.Time  `json:"last_active"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

// ============================================================
// 动态域名类型
// ============================================================

// DDNSProvider represents a supported DDNS provider.
type DDNSProvider string

const (
	DDNSProviderCloudflare DDNSProvider = "cloudflare"
	DDNSProviderAliyun     DDNSProvider = "aliyun"
	DDNSProviderDnspod     DDNSProvider = "dnspod"
	DDNSProviderNoIP       DDNSProvider = "noip"
	DDNSProviderDynDNS     DDNSProvider = "dyndns"
	DDNSProviderCustom     DDNSProvider = "custom"
)

// DDNSConfig represents the DDNS service configuration.
type DDNSConfig struct {
	Enabled      bool         `json:"enabled"`
	Provider     DDNSProvider `json:"provider"`
	Domain       string       `json:"domain"`
	Subdomain    string       `json:"subdomain"`
	APIKey       string       `json:"api_key"`
	APISecret    string       `json:"api_secret"`
	ZoneID       string       `json:"zone_id"`       // For Cloudflare
	UpdateURL    string       `json:"update_url"`    // For custom provider
	IntervalSec  int          `json:"interval_sec"`  // Update interval
	ForceUpdate  bool         `json:"force_update"`  // Force update even if IP unchanged
}

// DefaultDDNSConfig returns default DDNS configuration.
func DefaultDDNSConfig() DDNSConfig {
	return DDNSConfig{
		Enabled:     false,
		Provider:    DDNSProviderCloudflare,
		IntervalSec: 300, // 5 minutes
	}
}

// DNSRecord represents a DNS record.
type DNSRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // A, AAAA, CNAME
	Content   string    `json:"content"`
	TTL       int       `json:"ttl"`
	Proxied   bool      `json:"proxied"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DDNSStatus represents the current DDNS status.
type DDNSStatus struct {
	Provider    DDNSProvider `json:"provider"`
	Domain      string       `json:"domain"`
	CurrentIP   string       `json:"current_ip"`
	DNSIP       string       `json:"dns_ip"`
	LastUpdate  time.Time    `json:"last_update"`
	NextUpdate  time.Time    `json:"next_update"`
	Status      string       `json:"status"` // "synced", "pending", "error"
	Error       string       `json:"error,omitempty"`
	UpdateCount int          `json:"update_count"`
}

// ============================================================
// 端口映射类型
// ============================================================

// PortMappingProtocol represents the port mapping protocol.
type PortMappingProtocol string

const (
	PortMappingTCP PortMappingProtocol = "tcp"
	PortMappingUDP PortMappingProtocol = "udp"
)

// PortMappingMethod represents the port mapping method.
type PortMappingMethod string

const (
	PortMappingUPnP    PortMappingMethod = "upnp"
	PortMappingNATPMP  PortMappingMethod = "natpmp"
)

// PortMapping represents a port mapping entry.
type PortMapping struct {
	ID          string              `json:"id"`
	Protocol    PortMappingProtocol `json:"protocol"`
	ExternalPort int               `json:"external_port"`
	InternalPort int               `json:"internal_port"`
	InternalIP   string            `json:"internal_ip"`
	Description  string            `json:"description"`
	Enabled      bool              `json:"enabled"`
	Method       PortMappingMethod  `json:"method"`
	LeaseTime    int               `json:"lease_time"` // Lease time in seconds
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
}

// UPnPConfig represents the UPnP configuration.
type UPnPConfig struct {
	Enabled     bool   `json:"enabled"`
	DeviceName  string `json:"device_name"`
	ListenPort int    `json:"listen_port"`
	ExternalPort int  `json:"external_port"`
}

// DefaultUPnPConfig returns default UPnP configuration.
func DefaultUPnPConfig() UPnPConfig {
	return UPnPConfig{
		Enabled:     true,
		DeviceName:  "NAS-OS",
		ListenPort:  5000,
		ExternalPort: 5000,
	}
}

// NATPMPConfig represents the NAT-PMP configuration.
type NATPMPConfig struct {
	Enabled     bool   `json:"enabled"`
	Gateway     string `json:"gateway"`
	ExternalPort int   `json:"external_port"`
	InternalPort int   `json:"internal_port"`
	Protocol     string `json:"protocol"` // "tcp" or "udp"
	Lifetime     int    `json:"lifetime"` // Mapping lifetime in seconds
}

// DefaultNATPMPConfig returns default NAT-PMP configuration.
func DefaultNATPMPConfig() NATPMPConfig {
	return NATPMPConfig{
		Enabled:  true,
		Protocol: "tcp",
		Lifetime: 3600,
	}
}

// ============================================================
// 连接管理类型
// ============================================================

// ConnectionStatus represents the overall connection status.
type ConnectionStatus string

const (
	ConnectionStatusConnected    ConnectionStatus = "connected"
	ConnectionStatusConnecting   ConnectionStatus = "connecting"
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
	ConnectionStatusError        ConnectionStatus = "error"
)

// ConnectionInfo represents detailed connection information.
type ConnectionInfo struct {
	ID            string           `json:"id"`
	Mode          ConnectionMode   `json:"mode"`
	Status        ConnectionStatus `json:"status"`
	ClientID      string           `json:"client_id"`
	RemoteAddr    string           `json:"remote_addr"`
	LocalAddr     string           `json:"local_addr"`
	NATType       NATType          `json:"nat_type"`
	RelayServer   string           `json:"relay_server,omitempty"`
	LatencyMs     int              `json:"latency_ms"`
	Bandwidth     int64            `json:"bandwidth"`      // Current bandwidth in bytes/sec
	BytesSent     int64            `json:"bytes_sent"`
	BytesRecv     int64            `json:"bytes_recv"`
	ConnectedAt   time.Time        `json:"connected_at"`
	LastActive    time.Time        `json:"last_active"`
	Error         string           `json:"error,omitempty"`
}

// Session represents a user session.
type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	IP           string    `json:"ip"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastActive   time.Time `json:"last_active"`
	IsActive     bool      `json:"is_active"`
}

// ConnectionStats represents connection statistics.
type ConnectionStats struct {
	TotalConnections  int            `json:"total_connections"`
	ActiveConnections int            `json:"active_connections"`
	P2PConnections    int            `json:"p2p_connections"`
	RelayConnections  int            `json:"relay_connections"`
	DirectConnections int            `json:"direct_connections"`
	TotalBandwidth    int64          `json:"total_bandwidth"`
	TotalBytesSent    int64          `json:"total_bytes_sent"`
	TotalBytesRecv    int64          `json:"total_bytes_recv"`
	AverageLatency    float64        `json:"average_latency_ms"`
	Uptime            time.Duration  `json:"uptime"`
	ByMode            map[ConnectionMode]int `json:"by_mode"`
}

// ============================================================
// 安全认证类型
// ============================================================

// AuthConfig represents the authentication configuration.
type AuthConfig struct {
	Enabled          bool   `json:"enabled"`
	TokenSecret      string `json:"token_secret"`
	TokenExpirySec   int    `json:"token_expiry_sec"`
	RefreshTokenSec  int    `json:"refresh_token_sec"`
	MaxLoginAttempts int    `json:"max_login_attempts"`
	LockoutDurationSec int `json:"lockout_duration_sec"`
	RequireMFA       bool   `json:"require_mfa"`
}

// DefaultAuthConfig returns default authentication configuration.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Enabled:            true,
		TokenExpirySec:     3600,       // 1 hour
		RefreshTokenSec:    86400 * 7,  // 7 days
		MaxLoginAttempts:   5,
		LockoutDurationSec: 900,        // 15 minutes
		RequireMFA:         false,
	}
}

// Token represents an authentication token.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
}

// TokenClaims represents the claims in an authentication token.
type TokenClaims struct {
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	SessionID string    `json:"session_id"`
	IP        string    `json:"ip"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
}

// EncryptionConfig represents end-to-end encryption configuration.
type EncryptionConfig struct {
	Enabled     bool   `json:"enabled"`
	Algorithm   string `json:"algorithm"`   // "aes-256-gcm", "chacha20-poly1305"
	KeySize     int    `json:"key_size"`
	CertFile    string `json:"cert_file"`
	KeyFile     string `json:"key_file"`
	MinVersion  string `json:"min_version"` // "tls12", "tls13"
}

// DefaultEncryptionConfig returns default encryption configuration.
func DefaultEncryptionConfig() EncryptionConfig {
	return EncryptionConfig{
		Enabled:    true,
		Algorithm:  "aes-256-gcm",
		KeySize:    256,
		MinVersion: "tls13",
	}
}

// ============================================================
// 管理器配置
// ============================================================

// Config represents the complete remote access configuration.
type Config struct {
	Enabled      bool            `json:"enabled"`
	STUN         STUNConfig      `json:"stun"`
	TURN         TURNConfig      `json:"turn"`
	Relay        RelayConfig     `json:"relay"`
	DDNS         DDNSConfig      `json:"ddns"`
	UPnP         UPnPConfig      `json:"upnp"`
	NATPMP       NATPMPConfig    `json:"natpmp"`
	Auth         AuthConfig      `json:"auth"`
	Encryption   EncryptionConfig `json:"encryption"`
	DebugMode    bool            `json:"debug_mode"`
}

// DefaultConfig returns a default remote access configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:    true,
		STUN:       DefaultSTUNConfig(),
		TURN:       DefaultTURNConfig(),
		Relay:      DefaultRelayConfig(),
		DDNS:       DefaultDDNSConfig(),
		UPnP:       DefaultUPnPConfig(),
		NATPMP:     DefaultNATPMPConfig(),
		Auth:       DefaultAuthConfig(),
		Encryption: DefaultEncryptionConfig(),
		DebugMode:  false,
	}
}

// ============================================================
// 内部状态类型
// ============================================================

// ManagerState holds the internal state of the remote access manager.
type ManagerState struct {
	mu              sync.RWMutex
	config          Config
	connections     map[string]*ConnectionInfo
	sessions        map[string]*Session
	portMappings    map[string]*PortMapping
	p2pSessions     map[string]*P2PSession
	relaySessions   map[string]*RelaySession
	ddnsStatus      *DDNSStatus
	natType         NATType
	publicIP        string
	startTime       time.Time
	totalBytesSent  int64
	totalBytesRecv  int64
	connectionCount int64
}
