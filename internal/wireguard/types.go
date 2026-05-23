package wireguard

import (
	"time"
)

// WireGuardPeer represents a WireGuard peer configuration
type WireGuardPeer struct {
	ID                   string    `json:"id"`
	PublicKey            string    `json:"public_key"`
	AllowedIPs           string    `json:"allowed_ips"`
	Endpoint             string    `json:"endpoint,omitempty"`
	PersistentKeepalive  int       `json:"persistent_keepalive,omitempty"`
	LastHandshake        time.Time `json:"last_handshake,omitempty"`
	BytesRx              int64     `json:"bytes_rx"`
	BytesTx              int64     `json:"bytes_tx"`
	Enabled              bool      `json:"enabled"`
	CreatedAt            time.Time `json:"created_at"`
}

// WireGuardInterface represents the WireGuard interface configuration
type WireGuardInterface struct {
	Name       string           `json:"name"`
	ListenPort int              `json:"listen_port"`
	PrivateKey string           `json:"private_key,omitempty"`
	PublicKey  string           `json:"public_key"`
	Address    string           `json:"address"`
	DNS        string           `json:"dns,omitempty"`
	Peers      []WireGuardPeer  `json:"peers,omitempty"`
	Enabled    bool             `json:"enabled"`
	MTU        int              `json:"mtu,omitempty"`
}

// WireGuardConfig represents the complete WireGuard configuration
type WireGuardConfig struct {
	Interface WireGuardInterface `json:"interface"`
	Peers     []WireGuardPeer    `json:"peers"`
}

// WireGuardStats represents aggregated WireGuard statistics
type WireGuardStats struct {
	TotalPeers  int   `json:"total_peers"`
	ActivePeers int   `json:"active_peers"`
	TotalBytesRx int64 `json:"total_bytes_rx"`
	TotalBytesTx int64 `json:"total_bytes_tx"`
}

// CreatePeerRequest represents a request to create a new WireGuard peer
type CreatePeerRequest struct {
	PublicKey           string `json:"public_key" validate:"required"`
	AllowedIPs          string `json:"allowed_ips" validate:"required"`
	Endpoint            string `json:"endpoint,omitempty"`
	PersistentKeepalive int    `json:"persistent_keepalive,omitempty"`
	Enabled             *bool  `json:"enabled,omitempty"`
}

// UpdatePeerRequest represents a request to update a WireGuard peer
type UpdatePeerRequest struct {
	PublicKey           *string `json:"public_key,omitempty"`
	AllowedIPs          *string `json:"allowed_ips,omitempty"`
	Endpoint            *string `json:"endpoint,omitempty"`
	PersistentKeepalive *int    `json:"persistent_keepalive,omitempty"`
	Enabled             *bool   `json:"enabled,omitempty"`
}

// InterfaceConfigRequest represents a request to configure the WireGuard interface
type InterfaceConfigRequest struct {
	Name       *string `json:"name,omitempty"`
	ListenPort *int    `json:"listen_port,omitempty"`
	PrivateKey *string `json:"private_key,omitempty"`
	Address    *string `json:"address,omitempty"`
	DNS        *string `json:"dns,omitempty"`
	MTU        *int    `json:"mtu,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
}

// KeyPairResponse represents a generated key pair
type KeyPairResponse struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

// PeerConfigResponse represents a peer configuration file
type PeerConfigResponse struct {
	Config string `json:"config"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a generic API success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}
