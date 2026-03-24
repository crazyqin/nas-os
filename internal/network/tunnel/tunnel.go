package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Tunnel errors
var (
	ErrTunnelNotStarted    = errors.New("tunnel not started")
	ErrTunnelAlreadyExists = errors.New("tunnel already exists")
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrMaxPeersReached     = errors.New("maximum peers reached")
)

// TunnelManager manages the overall tunnel system
type TunnelManager struct {
	config     *TunnelConfig
	peerID     string
	
	// Components
	stunClient   *STUNClient
	turnClient   *TURNClient
	signaling    *SignalingClient
	peerManager  *PeerManager
	crypto       *Crypto
	
	// State
	state      *TunnelState
	stats      *TunnelStats
	startTime  time.Time
	
	// Control
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	
	// Event handlers
	eventHandlers []EventHandler
	mu           sync.RWMutex
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(config *TunnelConfig) (*TunnelManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Generate peer ID if not set
	peerID := generatePeerID()

	// Initialize crypto
	cryptoConfig := &CryptoConfig{
		CipherType: CipherChaCha20Poly1305,
	}
	crypto, err := NewCrypto(cryptoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize crypto: %w", err)
	}

	// Generate key pair
	if err := crypto.GenerateKeyPair(); err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Initialize state
	state := &TunnelState{
		Peers: make(map[string]*PeerInfo),
	}

	stats := &TunnelStats{}

	// Initialize peer manager
	peerManager := NewPeerManager(config)
	peerManager.SetCrypto(crypto)

	return &TunnelManager{
		config:      config,
		peerID:      peerID,
		crypto:      crypto,
		peerManager: peerManager,
		state:       state,
		stats:       stats,
		eventHandlers: make([]EventHandler, 0),
	}, nil
}

// Start starts the tunnel manager
func (m *TunnelManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.startTime = time.Now()

	// Initialize STUN client
	m.stunClient = NewSTUNClient(m.config)

	// Discover NAT type and public address
	discoveryCtx, cancel := context.WithTimeout(m.ctx, m.config.STUNTimeout)
	defer cancel()

	result, err := m.stunClient.Discover(discoveryCtx)
	if err != nil {
		m.emitEvent(TunnelEvent{
			Type:  "stun_failed",
			Error: err,
		})
		// Continue without STUN info
	} else {
		m.state.LocalNATType = result.NATType
		m.state.PublicIP = result.PublicIP
		m.state.PublicPort = result.PublicPort
		
		m.emitEvent(TunnelEvent{
			Type: "nat_discovered",
			Data: result,
		})
	}

	// Initialize signaling client if configured
	if m.config.SignalingURL != "" {
		m.signaling = NewSignalingClient(m.config.SignalingURL)
		m.peerManager.SetSignaling(m.signaling)

		if err := m.signaling.Connect(m.ctx, m.peerID); err != nil {
			m.emitEvent(TunnelEvent{
				Type:  "signaling_failed",
				Error: err,
			})
		} else {
			// Register with signaling server
			peerInfo := &PeerInfo{
				ID:        m.peerID,
				PublicKey: m.crypto.GetPublicKey(),
				NATType:   m.state.LocalNATType,
				Metadata: map[string]string{
					"version": "1.0",
				},
			}

			if err := m.signaling.Register(m.ctx, peerInfo); err != nil {
				m.emitEvent(TunnelEvent{
					Type:  "registration_failed",
					Error: err,
				})
			}

			// Set up message handlers
			m.setupSignalingHandlers()

			m.emitEvent(TunnelEvent{
				Type: "signaling_connected",
			})
		}
	}

	// Initialize TURN client if configured and needed
	if len(m.config.TURNServers) > 0 && needsRelay(m.state.LocalNATType) {
		if err := m.initTURNClient(); err != nil {
			m.emitEvent(TunnelEvent{
				Type:  "turn_init_failed",
				Error: err,
			})
		}
	}

	// Start keepalive routine
	m.wg.Add(1)
	go m.keepaliveLoop()

	// Start stats collection
	m.wg.Add(1)
	go m.statsLoop()

	m.state.Connected = true

	m.emitEvent(TunnelEvent{
		Type: "started",
	})

	return nil
}

// setupSignalingHandlers sets up signaling message handlers
func (m *TunnelManager) setupSignalingHandlers() {
	// Handle peer discovery
	m.signaling.OnMessage("peer_discovered", func(msg *Message) error {
		var peerInfo PeerInfo
		if err := json.Unmarshal(msg.Payload, &peerInfo); err != nil {
			return err
		}

		config := &PeerConfig{
			ID:        peerInfo.ID,
			PublicKey: peerInfo.PublicKey,
			NATType:   peerInfo.NATType,
		}

		// Convert endpoints
		for _, addr := range peerInfo.Endpoints {
			config.Endpoints = append(config.Endpoints, &addr)
		}

		_, err := m.peerManager.AddPeer(config)
		return err
	})

	// Handle offers
	m.signaling.OnMessage(MsgTypeOffer, func(msg *Message) error {
		var sdp SessionDescription
		if err := json.Unmarshal(msg.Payload, &sdp); err != nil {
			return err
		}

		peer, err := m.peerManager.GetPeer(msg.From)
		if err != nil {
			// Create new peer
			peer, err = m.peerManager.AddPeer(&PeerConfig{
				ID: msg.From,
			})
			if err != nil {
				return err
			}
		}

		return peer.HandleOffer(m.ctx, &sdp, m.config, m.signaling)
	})

	// Handle candidates
	m.signaling.OnMessage(MsgTypeCandidate, func(msg *Message) error {
		var candidate Candidate
		if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
			return err
		}

		// Add to peer's remote candidates
		return nil
	})

	// Handle disconnect
	m.signaling.OnMessage(MsgTypeDisconnect, func(msg *Message) error {
		return m.peerManager.RemovePeer(msg.From)
	})
}

// initTURNClient initializes the TURN client
func (m *TunnelManager) initTURNClient() error {
	if len(m.config.TURNServers) == 0 {
		return errors.New("no TURN servers configured")
	}

	server := m.config.TURNServers[0]
	m.turnClient = NewTURNClient(m.config, server)

	if err := m.turnClient.Connect(m.ctx, server.URL); err != nil {
		return fmt.Errorf("TURN connection failed: %w", err)
	}

	allocCtx, cancel := context.WithTimeout(m.ctx, m.config.TURNTimeout)
	defer cancel()

	_, err := m.turnClient.Allocate(allocCtx)
	if err != nil {
		return fmt.Errorf("TURN allocation failed: %w", err)
	}

	relayAddr := m.turnClient.GetRelayAddress()
	if relayAddr != nil {
		m.state.LocalCandidates = append(m.state.LocalCandidates, Candidate{
			Type: CandidateTypeRelay,
			IP:   relayAddr.IP,
			Port: relayAddr.Port,
		})
	}

	m.emitEvent(TunnelEvent{
		Type: "turn_connected",
	})

	return nil
}

// keepaliveLoop sends keepalive packets
func (m *TunnelManager) keepaliveLoop() {
	defer m.wg.Done()

	interval := m.config.Keepalive
	if interval == 0 {
		interval = ICEKeepaliveInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Send keepalive to all peers
			peers := m.peerManager.GetPeers()
			for _, peer := range peers {
				if peer.GetState() == PeerStateConnected {
					m.signaling.SendToPeer(peer.ID, MsgTypeKeepalive, nil)
				}
			}

			// Refresh TURN allocation if needed
			if m.turnClient != nil && m.turnClient.IsAllocated() {
				_ = m.turnClient.Refresh(m.ctx)
			}
		}
	}
}

// statsLoop collects statistics
func (m *TunnelManager) statsLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.collectStats()
		}
	}
}

// collectStats collects and aggregates statistics
func (m *TunnelManager) collectStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var totalSent, totalReceived uint64
	var totalPacketsSent, totalPacketsReceived uint64

	peers := m.peerManager.GetPeers()
	for _, peer := range peers {
		sent, received, packetsSent, packetsReceived := peer.GetStats()
		totalSent += sent
		totalReceived += received
		totalPacketsSent += packetsSent
		totalPacketsReceived += packetsReceived
	}

	m.stats.BytesSent = totalSent
	m.stats.BytesReceived = totalReceived
	m.stats.PacketsSent = totalPacketsSent
	m.stats.PacketsReceived = totalPacketsReceived
	m.stats.Connections = len(peers)
	m.stats.Uptime = time.Since(m.startTime)
}

// ConnectPeer connects to a specific peer
func (m *TunnelManager) ConnectPeer(ctx context.Context, peerID string, peerInfo *PeerInfo) error {
	if len(m.peerManager.GetPeers()) >= m.config.MaxPeers {
		return ErrMaxPeersReached
	}

	config := &PeerConfig{
		ID:        peerID,
		PublicKey: peerInfo.PublicKey,
		NATType:   peerInfo.NATType,
	}

	// Convert endpoints
	for _, addr := range peerInfo.Endpoints {
		config.Endpoints = append(config.Endpoints, &addr)
	}

	peer, err := m.peerManager.AddPeer(config)
	if err != nil {
		return err
	}

	return peer.Connect(ctx, m.config, m.signaling)
}

// DisconnectPeer disconnects from a peer
func (m *TunnelManager) DisconnectPeer(peerID string) error {
	if m.signaling != nil {
		m.signaling.SendToPeer(peerID, MsgTypeDisconnect, nil)
	}
	return m.peerManager.RemovePeer(peerID)
}

// Send sends data to a specific peer
func (m *TunnelManager) Send(peerID string, data []byte) error {
	peer, err := m.peerManager.GetPeer(peerID)
	if err != nil {
		return err
	}
	return peer.Send(data)
}

// Broadcast sends data to all connected peers
func (m *TunnelManager) Broadcast(data []byte) error {
	return m.peerManager.Broadcast(data)
}

// Receive returns a channel for receiving data from any peer
func (m *TunnelManager) Receive() <-chan *PeerData {
	ch := make(chan *PeerData, 100)
	
	// Start goroutine to aggregate data from all peers
	go func() {
		defer close(ch)
		peers := m.peerManager.GetPeers()
		for _, peer := range peers {
			go func(p *Peer) {
				for data := range p.Receive() {
					ch <- &PeerData{
						PeerID: p.ID,
						Data:   data,
					}
				}
			}(peer)
		}
	}()

	return ch
}

// PeerData represents data received from a peer
type PeerData struct {
	PeerID string
	Data   []byte
}

// GetState returns the current tunnel state
func (m *TunnelManager) GetState() *TunnelState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// GetStats returns tunnel statistics
func (m *TunnelManager) GetStats() *TunnelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetPeerID returns the local peer ID
func (m *TunnelManager) GetPeerID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.peerID
}

// GetPeers returns all connected peers
func (m *TunnelManager) GetPeers() []*Peer {
	return m.peerManager.GetPeers()
}

// GetPublicKey returns the local public key
func (m *TunnelManager) GetPublicKey() []byte {
	return m.crypto.GetPublicKey()
}

// Close stops the tunnel manager
func (m *TunnelManager) Close() error {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.state.Connected = false
	m.mu.Unlock()

	m.wg.Wait()

	var errs []error

	if m.stunClient != nil {
		if err := m.stunClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.turnClient != nil {
		if err := m.turnClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.signaling != nil {
		if err := m.signaling.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := m.peerManager.Close(); err != nil {
		errs = append(errs, err)
	}

	m.emitEvent(TunnelEvent{
		Type: "stopped",
	})

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}
	return nil
}

// OnEvent registers an event handler
func (m *TunnelManager) OnEvent(handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)
	m.peerManager.OnEvent(handler)
}

// emitEvent emits an event to all handlers
func (m *TunnelManager) emitEvent(event TunnelEvent) {
	event.Timestamp = time.Now()
	for _, handler := range m.eventHandlers {
		go handler(event)
	}
}

// CreateTunnel creates a new tunnel between two peers
func (m *TunnelManager) CreateTunnel(ctx context.Context, remotePeerID string, config *TunnelConfig) error {
	peer, err := m.peerManager.GetPeer(remotePeerID)
	if err != nil {
		return err
	}

	if peer.GetState() != PeerStateConnected {
		return ErrPeerNotConnected
	}

	return nil
}

// DiscoverPeers discovers peers on the local network
func (m *TunnelManager) DiscoverPeers(ctx context.Context) ([]*PeerInfo, error) {
	// Would implement mDNS or broadcast discovery
	return nil, nil
}

// GetLocalCandidates returns local ICE candidates
func (m *TunnelManager) GetLocalCandidates() []Candidate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.LocalCandidates
}

// needsRelay determines if TURN relay is needed based on NAT type
func needsRelay(natType NATType) bool {
	return natType == NATSymmetric || natType == NATSymmetricUDPFirewall
}

// TunnelServer represents a server that can accept incoming tunnel connections
type TunnelServer struct {
	manager *TunnelManager
	port    int
}

// NewTunnelServer creates a new tunnel server
func NewTunnelServer(manager *TunnelManager, port int) *TunnelServer {
	return &TunnelServer{
		manager: manager,
		port:    port,
	}
}

// Start starts listening for incoming connections
func (s *TunnelServer) Start(ctx context.Context) error {
	addr := &net.UDPAddr{Port: s.port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	go s.handleConnections(ctx, conn)
	return nil
}

// handleConnections handles incoming tunnel connections
func (s *TunnelServer) handleConnections(ctx context.Context, conn *net.UDPConn) {
	defer conn.Close()

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			// Parse incoming packet
			packet := buf[:n]
			_ = packet  // Process packet
			_ = remoteAddr // Handle peer
		}
	}
}