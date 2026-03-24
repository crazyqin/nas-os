// Package tunnel implements NAT traversal and secure tunneling for remote access.
// It provides STUN/TURN/ICE protocols support for peer-to-peer connections
// behind NAT firewalls, similar to fnOS's FN Connect feature.
package tunnel

import (
	"encoding/json"
	"net"
	"time"
)

// NATType represents the type of NAT detected
type NATType int

const (
	NATUnknown NATType = iota
	NATNone               // No NAT (public IP)
	NATFullCone           // Full Cone NAT (easiest to traverse)
	NATRestrictedCone     // Restricted Cone NAT
	NATPortRestricted     // Port Restricted Cone NAT
	NATSymmetric          // Symmetric NAT (hardest to traverse)
	NATSymmetricUDPFirewall // Symmetric UDP Firewall
)

func (n NATType) String() string {
	switch n {
	case NATNone:
		return "None"
	case NATFullCone:
		return "Full Cone"
	case NATRestrictedCone:
		return "Restricted Cone"
	case NATPortRestricted:
		return "Port Restricted"
	case NATSymmetric:
		return "Symmetric"
	case NATSymmetricUDPFirewall:
		return "Symmetric UDP Firewall"
	default:
		return "Unknown"
	}
}

// ConnectionType represents how two peers are connected
type ConnectionType int

const (
	ConnectionUnknown ConnectionType = iota
	ConnectionDirect                  // Direct P2P connection
	ConnectionRelay                   // Through TURN relay
	ConnectionHolePunched             // UDP hole punched
)

func (c ConnectionType) String() string {
	switch c {
	case ConnectionDirect:
		return "Direct"
	case ConnectionRelay:
		return "Relay"
	case ConnectionHolePunched:
		return "Hole Punched"
	default:
		return "Unknown"
	}
}

// PeerInfo contains information about a remote peer
type PeerInfo struct {
	ID           string            `json:"id"`
	PublicKey    []byte            `json:"public_key"`
	Endpoints    []net.UDPAddr     `json:"endpoints"`
	NATType      NATType           `json:"nat_type"`
	Metadata     map[string]string `json:"metadata"`
	LastSeen     time.Time         `json:"last_seen"`
	ConnectionType ConnectionType  `json:"connection_type"`
}

// Candidate represents an ICE candidate
type Candidate struct {
	Type        string     `json:"type"`         // host, srflx, relay
	Network     string     `json:"network"`      // udp, tcp
	IP          net.IP     `json:"ip"`
	Port        int        `json:"port"`
	Priority    uint32     `json:"priority"`
	Foundation  string     `json:"foundation"`
	Component   int        `json:"component"`
	RelAddr     net.IP     `json:"rel_addr,omitempty"`  // Related address for srflx/relay
	RelPort     int        `json:"rel_port,omitempty"`  // Related port for srflx/relay
}

// TunnelConfig holds tunnel configuration
type TunnelConfig struct {
	// Local configuration
	ListenPort    int           `json:"listen_port"`
	PublicKey     []byte        `json:"public_key"`
	PrivateKey    []byte        `json:"-"`
	
	// STUN servers
	STUNServers   []string      `json:"stun_servers"`
	
	// TURN servers (for relay fallback)
	TURNServers   []TURNServer  `json:"turn_servers"`
	
	// Signaling server
	SignalingURL  string        `json:"signaling_url"`
	
	// Security
	EncryptionKey []byte        `json:"-"`
	
	// Timeouts
	STUNTimeout   time.Duration `json:"stun_timeout"`
	TURNTimeout   time.Duration `json:"turn_timeout"`
	ICETimeout    time.Duration `json:"ice_timeout"`
	Keepalive     time.Duration `json:"keepalive"`
	
	// Limits
	MaxPeers      int           `json:"max_peers"`
	MaxRetries    int           `json:"max_retries"`
}

// TURNServer represents a TURN server configuration
type TURNServer struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// TunnelState represents the current state of the tunnel
type TunnelState struct {
	LocalNATType    NATType         `json:"local_nat_type"`
	PublicIP        net.IP          `json:"public_ip"`
	PublicPort      int             `json:"public_port"`
	LocalCandidates []Candidate     `json:"local_candidates"`
	Peers           map[string]*PeerInfo `json:"peers"`
	Connected       bool            `json:"connected"`
	LastError       string          `json:"last_error,omitempty"`
}

// TunnelStats holds statistics for the tunnel
type TunnelStats struct {
	BytesSent       uint64        `json:"bytes_sent"`
	BytesReceived   uint64        `json:"bytes_received"`
	PacketsSent     uint64        `json:"packets_sent"`
	PacketsReceived uint64        `json:"packets_received"`
	Connections     int           `json:"connections"`
	Uptime          time.Duration `json:"uptime"`
	LastConnect     time.Time     `json:"last_connect"`
}

// Message represents a signaling message
type Message struct {
	Type      string          `json:"type"`
	From      string          `json:"from"`
	To        string          `json:"to"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// SessionDescription contains SDP for WebRTC-like session
type SessionDescription struct {
	SessionID   string      `json:"session_id"`
	Candidates  []Candidate `json:"candidates"`
	Fingerprint string      `json:"fingerprint"`
	ICEUfrag    string      `json:"ice_ufrag"`
	ICEPwd      string      `json:"ice_pwd"`
}

// TunnelEvent represents an event in the tunnel lifecycle
type TunnelEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
	Error     error       `json:"error,omitempty"`
}

// EventHandler handles tunnel events
type EventHandler func(event TunnelEvent)

// DefaultConfig returns a default tunnel configuration
func DefaultConfig() *TunnelConfig {
	return &TunnelConfig{
		ListenPort: 51820,
		STUNServers: []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
			"stun:stun2.l.google.com:19302",
			"stun:stun.cloudflare.com:3478",
		},
		STUNTimeout: 5 * time.Second,
		TURNTimeout: 10 * time.Second,
		ICETimeout:  30 * time.Second,
		Keepalive:   25 * time.Second,
		MaxPeers:    100,
		MaxRetries:  3,
	}
}

// EncryptedPacket represents an encrypted data packet
type EncryptedPacket struct {
	Nonce     []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	Tag       []byte `json:"tag"`
}

// HolePunchRequest represents a hole punch synchronization request
type HolePunchRequest struct {
	PeerID     string    `json:"peer_id"`
	LocalAddr  string    `json:"local_addr"`
	RemoteAddr string    `json:"remote_addr"`
	Timestamp  time.Time `json:"timestamp"`
}

// SignalingMessage types
const (
	MsgTypeOffer       = "offer"
	MsgTypeAnswer      = "answer"
	MsgTypeCandidate   = "candidate"
	MsgTypeHolePunch   = "hole_punch"
	MsgTypeKeepalive   = "keepalive"
	MsgTypeDisconnect  = "disconnect"
	MsgTypeError       = "error"
)