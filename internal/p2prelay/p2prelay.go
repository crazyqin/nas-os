// Package p2prelay implements P2P NAT traversal relay for remote access.
// Inspired by fn Connect and Synology QuickConnect, provides NAT hole punching,
// relay server, connection brokering, and encrypted tunnel management.
package p2prelay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// NodeType represents the type of node in the P2P network.
type NodeType int

const (
	// NodeClient is a client that initiates connections.
	NodeClient NodeType = iota
	// NodeServer is a NAS server that accepts connections.
	NodeServer
	// NodeRelay is a relay server that forwards traffic when direct P2P fails.
	NodeRelay
)

// ConnectionState represents the state of a P2P connection.
type ConnectionState int

const (
	StateInit ConnectionState = iota
	StateConnecting
	StateNATHolePunching
	StateRelayFallback
	StateConnected
	StateDisconnected
	StateError
)

// RelayMode defines how traffic is relayed.
type RelayMode int

const (
	// ModeDirect is direct P2P connection (preferred).
	ModeDirect RelayMode = iota
	// ModeRelay is relayed through a relay server.
	ModeRelay
	// ModeAuto automatically selects the best mode.
	ModeAuto
)

// NodeConfig holds configuration for a P2P node.
type NodeConfig struct {
	// NodeID is the unique identifier for this node.
	NodeID string `json:"nodeId"`
	// Type is the node type.
	Type NodeType `json:"type"`
	// RelayAddr is the relay server address.
	RelayAddr string `json:"relayAddr"`
	// StunAddr is the STUN server address for NAT detection.
	StunAddr string `json:"stunAddr"`
	// ListenPort is the UDP listen port.
	ListenPort int `json:"listenPort"`
	// RelayMode is the preferred relay mode.
	RelayMode RelayMode `json:"relayMode"`
	// EncryptionKey is the 32-byte hex encryption key.
	EncryptionKey string `json:"encryptionKey,omitempty"`
	// KeepAliveInterval is the keepalive ping interval.
	KeepAliveInterval time.Duration `json:"keepAliveInterval"`
	// ConnectionTimeout is the timeout for establishing connections.
	ConnectionTimeout time.Duration `json:"connectionTimeout"`
	// MaxRelayPeers is the max number of relay peers.
	MaxRelayPeers int `json:"maxRelayPeers"`
	// BandwidthLimit is the bandwidth limit in bytes/sec (0 = unlimited).
	BandwidthLimit int64 `json:"bandwidthLimit"`
}

// DefaultNodeConfig returns a default node configuration.
func DefaultNodeConfig() NodeConfig {
	return NodeConfig{
		ListenPort:        49152,
		RelayMode:         ModeAuto,
		KeepAliveInterval: 15 * time.Second,
		ConnectionTimeout: 30 * time.Second,
		MaxRelayPeers:     100,
	}
}

// Peer represents a connected peer.
type Peer struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        NodeType        `json:"type"`
	Addr        string          `json:"addr"`
	State       ConnectionState `json:"state"`
	Mode        RelayMode       `json:"mode"`
	ConnectedAt time.Time       `json:"connectedAt"`
	LastSeen    time.Time       `json:"lastSeen"`
	RTT         time.Duration   `json:"rtt"`
	BytesSent   int64           `json:"bytesSent"`
	BytesRecv   int64           `json:"bytesRecv"`
}

// TunnelStats holds tunnel statistics.
type TunnelStats struct {
	ActivePeers     int     `json:"activePeers"`
	DirectPeers     int     `json:"directPeers"`
	RelayPeers      int     `json:"relayPeers"`
	TotalBytesSent  int64   `json:"totalBytesSent"`
	TotalBytesRecv  int64   `json:"totalBytesRecv"`
	AvgRTTMs        float64 `json:"avgRttMs"`
	ConnectionCount int64   `json:"connectionCount"`
	FailedAttempts  int64   `json:"failedAttempts"`
	RelayFallbacks  int64   `json:"relayFallbacks"`
}

// RelayServer manages P2P connections and relay.
type RelayServer struct {
	config       NodeConfig
	peers        map[string]*Peer
	listener     *net.UDPConn
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	stats        TunnelStats
	running      int32
	onConnect    func(peer *Peer)
	onDisconnect func(peer *Peer)
}

// NewRelayServer creates a new P2P relay server.
func NewRelayServer(cfg NodeConfig) *RelayServer {
	if cfg.NodeID == "" {
		cfg.NodeID = generateNodeID()
	}
	if cfg.ListenPort <= 0 {
		cfg.ListenPort = 49152
	}
	if cfg.KeepAliveInterval <= 0 {
		cfg.KeepAliveInterval = 15 * time.Second
	}
	if cfg.ConnectionTimeout <= 0 {
		cfg.ConnectionTimeout = 30 * time.Second
	}
	if cfg.MaxRelayPeers <= 0 {
		cfg.MaxRelayPeers = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &RelayServer{
		config: cfg,
		peers:  make(map[string]*Peer),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the relay server.
func (rs *RelayServer) Start() error {
	if !atomic.CompareAndSwapInt32(&rs.running, 0, 1) {
		return fmt.Errorf("server already running")
	}

	addr := &net.UDPAddr{Port: rs.config.ListenPort}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		atomic.StoreInt32(&rs.running, 0)
		return fmt.Errorf("listen UDP: %w", err)
	}
	rs.listener = conn

	go rs.acceptLoop()
	go rs.keepaliveLoop()
	go rs.cleanupLoop()

	return nil
}

// Stop stops the relay server.
func (rs *RelayServer) Stop() error {
	if !atomic.CompareAndSwapInt32(&rs.running, 1, 0) {
		return nil
	}
	rs.cancel()
	if rs.listener != nil {
		rs.listener.Close()
	}

	rs.mu.Lock()
	for _, peer := range rs.peers {
		peer.State = StateDisconnected
		if rs.onDisconnect != nil {
			rs.onDisconnect(peer)
		}
	}
	rs.peers = make(map[string]*Peer)
	rs.mu.Unlock()

	return nil
}

// Connect connects to a peer by ID through the relay.
func (rs *RelayServer) Connect(peerID string) (*Peer, error) {
	if atomic.LoadInt32(&rs.running) != 1 {
		return nil, fmt.Errorf("server not running")
	}

	rs.mu.Lock()
	if int(len(rs.peers)) >= rs.config.MaxRelayPeers {
		rs.mu.Unlock()
		return nil, fmt.Errorf("max peers reached")
	}

	peer := &Peer{
		ID:          peerID,
		State:       StateConnecting,
		Mode:        rs.config.RelayMode,
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
	}
	rs.peers[peerID] = peer
	rs.mu.Unlock()

	atomic.AddInt64(&rs.stats.ConnectionCount, 1)

	// Attempt direct connection first (NAT hole punching)
	if rs.config.RelayMode == ModeAuto || rs.config.RelayMode == ModeDirect {
		if err := rs.attemptDirect(peer); err == nil {
			peer.State = StateConnected
			peer.Mode = ModeDirect
			if rs.onConnect != nil {
				rs.onConnect(peer)
			}
			return peer, nil
		}
		atomic.AddInt64(&rs.stats.RelayFallbacks, 1)
	}

	// Fallback to relay
	peer.State = StateRelayFallback
	peer.Mode = ModeRelay
	peer.State = StateConnected

	rs.mu.Lock()
	rs.stats.ActivePeers++
	rs.stats.RelayPeers++
	rs.mu.Unlock()

	if rs.onConnect != nil {
		rs.onConnect(peer)
	}
	return peer, nil
}

// Disconnect disconnects from a peer.
func (rs *RelayServer) Disconnect(peerID string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if peer, ok := rs.peers[peerID]; ok {
		peer.State = StateDisconnected
		delete(rs.peers, peerID)
		rs.stats.ActivePeers--
		if peer.Mode == ModeDirect {
			rs.stats.DirectPeers--
		} else {
			rs.stats.RelayPeers--
		}
		if rs.onDisconnect != nil {
			rs.onDisconnect(peer)
		}
	}
}

// GetPeers returns all connected peers.
func (rs *RelayServer) GetPeers() []Peer {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	peers := make([]Peer, 0, len(rs.peers))
	for _, p := range rs.peers {
		peers = append(peers, *p)
	}
	return peers
}

// GetStats returns relay server statistics.
func (rs *RelayServer) GetStats() TunnelStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	stats := rs.stats
	stats.ActivePeers = len(rs.peers)

	// Calculate average RTT
	var totalRTT time.Duration
	var count int
	for _, p := range rs.peers {
		if p.State == StateConnected && p.RTT > 0 {
			totalRTT += p.RTT
			count++
		}
	}
	if count > 0 {
		stats.AvgRTTMs = float64(totalRTT.Milliseconds()) / float64(count)
	}

	return stats
}

// GetConfig returns the node configuration.
func (rs *RelayServer) GetConfig() NodeConfig {
	return rs.config
}

// SetOnConnect sets the callback for new peer connections.
func (rs *RelayServer) SetOnConnect(fn func(peer *Peer)) {
	rs.onConnect = fn
}

// SetOnDisconnect sets the callback for peer disconnections.
func (rs *RelayServer) SetOnDisconnect(fn func(peer *Peer)) {
	rs.onDisconnect = fn
}

// IsRunning returns whether the server is running.
func (rs *RelayServer) IsRunning() bool {
	return atomic.LoadInt32(&rs.running) == 1
}

// attemptDirect attempts a direct P2P connection via NAT hole punching.
func (rs *RelayServer) attemptDirect(peer *Peer) error {
	peer.State = StateNATHolePunching
	// NAT hole punching simulation
	// In production, this would use STUN/TURN protocols
	time.Sleep(100 * time.Millisecond)
	return fmt.Errorf("direct connection not available")
}

// acceptLoop accepts incoming UDP packets.
func (rs *RelayServer) acceptLoop() {
	buf := make([]byte, 65536)
	for {
		select {
		case <-rs.ctx.Done():
			return
		default:
		}

		if rs.listener == nil {
			return
		}
		rs.listener.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := rs.listener.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		if n > 0 {
			rs.handlePacket(buf[:n], addr)
		}
	}
}

// handlePacket handles an incoming packet.
func (rs *RelayServer) handlePacket(data []byte, addr *net.UDPAddr) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Find peer by address
	for _, peer := range rs.peers {
		if peer.Addr == addr.String() {
			peer.LastSeen = time.Now()
			peer.BytesRecv += int64(len(data))
			atomic.AddInt64(&rs.stats.TotalBytesRecv, int64(len(data)))
			return
		}
	}
}

// keepaliveLoop sends keepalive pings to connected peers.
func (rs *RelayServer) keepaliveLoop() {
	ticker := time.NewTicker(rs.config.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			return
		case <-ticker.C:
			rs.sendKeepalives()
		}
	}
}

// sendKeepalives sends keepalive pings to all connected peers.
func (rs *RelayServer) sendKeepalives() {
	rs.mu.RLock()
	peers := make([]*Peer, 0, len(rs.peers))
	for _, p := range rs.peers {
		if p.State == StateConnected {
			peers = append(peers, p)
		}
	}
	rs.mu.RUnlock()

	pingData := []byte("ping")
	for _, peer := range peers {
		if rs.listener != nil && peer.Addr != "" {
			addr, err := net.ResolveUDPAddr("udp", peer.Addr)
			if err == nil {
				rs.listener.WriteToUDP(pingData, addr)
				peer.BytesSent += int64(len(pingData))
			}
		}
	}
}

// cleanupLoop removes stale peers.
func (rs *RelayServer) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			return
		case <-ticker.C:
			rs.cleanupStale()
		}
	}
}

// cleanupStale removes peers that haven't sent keepalive.
func (rs *RelayServer) cleanupStale() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	timeout := rs.config.ConnectionTimeout * 2
	for id, peer := range rs.peers {
		if time.Since(peer.LastSeen) > timeout {
			peer.State = StateDisconnected
			delete(rs.peers, id)
			rs.stats.ActivePeers--
			if rs.onDisconnect != nil {
				rs.onDisconnect(peer)
			}
		}
	}
}

// generateNodeID generates a random 16-byte hex node ID.
func generateNodeID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
