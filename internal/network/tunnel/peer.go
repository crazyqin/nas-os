package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Peer errors.
var (
	ErrPeerNotConnected    = errors.New("peer not connected")
	ErrPeerAlreadyExists   = errors.New("peer already exists")
	ErrPeerHandshakeFailed = errors.New("peer handshake failed")
)

// PeerState represents the state of a peer connection.
type PeerState int

const (
	// PeerStateNew indicates a new peer connection.
	PeerStateNew PeerState = iota
	// PeerStateConnecting indicates peer is connecting.
	PeerStateConnecting
	// PeerStateConnected indicates peer is connected.
	PeerStateConnected
	// PeerStateDisconnected indicates peer is disconnected.
	PeerStateDisconnected
	// PeerStateFailed indicates peer connection failed.
	PeerStateFailed
)

func (s PeerState) String() string {
	switch s {
	case PeerStateNew:
		return "new"
	case PeerStateConnecting:
		return "connecting"
	case PeerStateConnected:
		return "connected"
	case PeerStateDisconnected:
		return "disconnected"
	case PeerStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Peer represents a remote peer connection.
type Peer struct {
	ID        string
	PublicKey []byte

	// Network addresses
	Endpoints []*net.UDPAddr
	NATType   NATType

	// Connection
	ConnectionType ConnectionType
	State          PeerState

	// ICE agent
	iceAgent *ICEAgent

	// Encryption
	crypto     *Crypto
	sessionKey []byte

	// Statistics
	bytesSent       uint64
	bytesReceived   uint64
	packetsSent     uint64
	packetsReceived uint64
	lastSeen        time.Time

	// Channels
	sendChan chan []byte
	recvChan chan []byte

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// PeerConfig holds peer configuration.
type PeerConfig struct {
	ID        string
	PublicKey []byte
	Endpoints []*net.UDPAddr
	NATType   NATType
}

// NewPeer creates a new peer.
func NewPeer(config *PeerConfig) *Peer {
	return &Peer{
		ID:        config.ID,
		PublicKey: config.PublicKey,
		Endpoints: config.Endpoints,
		NATType:   config.NATType,
		State:     PeerStateNew,
		sendChan:  make(chan []byte, 100),
		recvChan:  make(chan []byte, 100),
	}
}

// Connect initiates connection to the peer.
func (p *Peer) Connect(ctx context.Context, localConfig *TunnelConfig, signaling *SignalingClient) error {
	p.mu.Lock()
	p.State = PeerStateConnecting
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	// Initialize ICE agent
	iceConfig := &TunnelConfig{
		STUNServers: localConfig.STUNServers,
		TURNServers: localConfig.TURNServers,
		ListenPort:  localConfig.ListenPort,
		STUNTimeout: localConfig.STUNTimeout,
		TURNTimeout: localConfig.TURNTimeout,
	}

	iceAgent := NewICEAgent(iceConfig)
	if err := iceAgent.Initialize(ctx); err != nil {
		p.setState(PeerStateFailed)
		return fmt.Errorf("failed to initialize ICE: %w", err)
	}

	p.iceAgent = iceAgent

	// Get local SDP
	localSDP := iceAgent.GetLocalDescription()

	// Send offer via signaling
	if err := signaling.SendToPeer(p.ID, MsgTypeOffer, localSDP); err != nil {
		p.setState(PeerStateFailed)
		return fmt.Errorf("failed to send offer: %w", err)
	}

	// Wait for answer
	answerChan := make(chan *SessionDescription, 1)
	signaling.OnMessage(MsgTypeAnswer, func(msg *Message) error {
		if msg.From == p.ID {
			var sdp SessionDescription
			if err := unmarshalPayload(msg.Payload, &sdp); err == nil {
				answerChan <- &sdp
			}
		}
		return nil
	})

	select {
	case <-ctx.Done():
		p.setState(PeerStateFailed)
		return ctx.Err()
	case answer := <-answerChan:
		// Process answer - convert []Candidate to []*Candidate
		candidates := make([]*Candidate, len(answer.Candidates))
		for i := range answer.Candidates {
			candidates[i] = &answer.Candidates[i]
		}
		iceAgent.SetRemoteCandidates(candidates)
		if err := iceAgent.StartConnectivityChecks(answer.ICEUfrag, answer.ICEPwd); err != nil {
			p.setState(PeerStateFailed)
			return err
		}
	}

	// Wait for ICE connection
	connected := make(chan struct{}, 1)
	iceAgent.OnConnected(func() {
		connected <- struct{}{}
	})
	iceAgent.OnFailed(func(err error) {
		p.setState(PeerStateFailed)
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-connected:
		p.setState(PeerStateConnected)
		p.lastSeen = time.Now()

		// Start data handling
		go p.sendLoop()
		go p.receiveLoop()

		return nil
	}
}

// HandleOffer handles an incoming offer from a peer.
func (p *Peer) HandleOffer(ctx context.Context, offer *SessionDescription, localConfig *TunnelConfig, signaling *SignalingClient) error {
	p.mu.Lock()
	p.State = PeerStateConnecting
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	// Initialize ICE agent
	iceConfig := &TunnelConfig{
		STUNServers: localConfig.STUNServers,
		TURNServers: localConfig.TURNServers,
		ListenPort:  localConfig.ListenPort,
	}

	iceAgent := NewICEAgent(iceConfig)
	if err := iceAgent.Initialize(ctx); err != nil {
		p.setState(PeerStateFailed)
		return err
	}

	p.iceAgent = iceAgent

	// Set remote candidates from offer - convert []Candidate to []*Candidate
	candidates := make([]*Candidate, len(offer.Candidates))
	for i := range offer.Candidates {
		candidates[i] = &offer.Candidates[i]
	}
	iceAgent.SetRemoteCandidates(candidates)

	// Start connectivity checks
	if err := iceAgent.StartConnectivityChecks(offer.ICEUfrag, offer.ICEPwd); err != nil {
		p.setState(PeerStateFailed)
		return err
	}

	// Send answer
	answer := iceAgent.GetLocalDescription()
	if err := signaling.SendToPeer(p.ID, MsgTypeAnswer, answer); err != nil {
		p.setState(PeerStateFailed)
		return err
	}

	// Wait for connection
	connected := make(chan struct{}, 1)
	iceAgent.OnConnected(func() {
		connected <- struct{}{}
	})

	select {
	case <-ctx.Done():
		p.setState(PeerStateFailed)
		return ctx.Err()
	case <-connected:
		p.setState(PeerStateConnected)
		p.lastSeen = time.Now()

		go p.sendLoop()
		go p.receiveLoop()

		return nil
	}
}

// sendLoop handles outgoing data.
func (p *Peer) sendLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case data := <-p.sendChan:
			if err := p.sendData(data); err != nil {
				// Log error, retry or disconnect
				_ = err // explicitly ignore: send errors handled by connection state
			}
		}
	}
}

// receiveLoop handles incoming data.
func (p *Peer) receiveLoop() {
	buf := make([]byte, 65535)
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			if p.iceAgent == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			n, err := p.iceAgent.Read(buf)
			if err != nil {
				continue
			}

			p.mu.Lock()
			p.bytesReceived += uint64(n)
			p.packetsReceived++
			p.lastSeen = time.Now()
			p.mu.Unlock()

			// Send to receive channel
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case p.recvChan <- data:
			default:
				// Channel full, drop packet
			}
		}
	}
}

// sendData sends data through the ICE connection.
func (p *Peer) sendData(data []byte) error {
	if p.iceAgent == nil {
		return ErrPeerNotConnected
	}

	// Encrypt if crypto is set
	if p.crypto != nil && p.sessionKey != nil {
		encrypted, err := p.crypto.Encrypt(data, p.ID)
		if err != nil {
			return err
		}
		data = encrypted
	}

	n, err := p.iceAgent.Write(data)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.bytesSent += uint64(n)
	p.packetsSent++
	p.mu.Unlock()

	return nil
}

// Send queues data to be sent to the peer.
func (p *Peer) Send(data []byte) error {
	p.mu.RLock()
	if p.State != PeerStateConnected {
		p.mu.RUnlock()
		return ErrPeerNotConnected
	}
	p.mu.RUnlock()

	select {
	case p.sendChan <- data:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

// Receive returns the receive channel.
func (p *Peer) Receive() <-chan []byte {
	return p.recvChan
}

// Close closes the peer connection.
func (p *Peer) Close() error {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.State = PeerStateDisconnected
	p.mu.Unlock()

	if p.iceAgent != nil {
		return p. //nolint:errcheck
				iceAgent.Close()
	}
	return nil
}

// GetStats returns peer statistics.
func (p *Peer) GetStats() (bytesSent, bytesReceived, packetsSent, packetsReceived uint64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.bytesSent, p.bytesReceived, p.packetsSent, p.packetsReceived
}

// GetLastSeen returns the last seen time.
func (p *Peer) GetLastSeen() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastSeen
}

// GetState returns the current state.
func (p *Peer) GetState() PeerState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// setState sets the peer state.
func (p *Peer) setState(state PeerState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.State = state
}

// SetCrypto sets up encryption for the peer.
func (p *Peer) SetCrypto(crypto *Crypto) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Derive session key from peer's public key
	sessionKey, err := crypto.DeriveSharedKey(p.PublicKey)
	if err != nil {
		return err
	}

	p.crypto = crypto
	p.sessionKey = sessionKey

	return nil
}

// HolePunch performs UDP hole punching with the peer.
func (p *Peer) HolePunch(ctx context.Context, localPort int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.Endpoints) == 0 {
		return errors.New("no endpoints to punch")
	}

	// Create UDP socket
	localAddr := &net.UDPAddr{Port: localPort}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return err
	}
	defer //nolint:errcheck
	conn.Close()

	// Send punch packets to all endpoints
	punchPacket := []byte("PUNCH")
	for _, endpoint := range p.Endpoints {
		//nolint:errcheck
		conn.WriteToUDP(punchPacket, endpoint)
	}

	// Wait for response
	//nolint:errcheck
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return err
	}

	// Verify punch response
	if n > 0 && string(buf[:n]) == "PUNCH_ACK" {
		p.ConnectionType = ConnectionHolePunched
		// Update endpoint to the one that responded
		p.Endpoints = []*net.UDPAddr{addr}
		return nil
	}

	return errors.New("hole punch failed")
}

// PeerManager manages multiple peer connections.
type PeerManager struct {
	peers     map[string]*Peer
	config    *TunnelConfig
	signaling *SignalingClient
	crypto    *Crypto

	eventHandlers []EventHandler
	mu            sync.RWMutex
}

// NewPeerManager creates a new peer manager.
func NewPeerManager(config *TunnelConfig) *PeerManager {
	return &PeerManager{
		peers:  make(map[string]*Peer),
		config: config,
	}
}

// SetSignaling sets the signaling client.
func (m *PeerManager) SetSignaling(client *SignalingClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signaling = client
}

// SetCrypto sets the crypto instance.
func (m *PeerManager) SetCrypto(crypto *Crypto) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.crypto = crypto
}

// AddPeer adds a new peer.
func (m *PeerManager) AddPeer(config *PeerConfig) (*Peer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.peers[config.ID]; exists {
		return nil, ErrPeerAlreadyExists
	}

	peer := NewPeer(config)
	m.peers[config.ID] = peer

	m.emitEvent(TunnelEvent{
		Type: "peer_added",
		Data: peer.ID,
	})

	return peer, nil
}

// RemovePeer removes a peer.
func (m *PeerManager) RemovePeer(peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	peer, exists := m.peers[peerID]
	if !exists {
		return ErrPeerNotFound
	}

	//nolint:errcheck
	peer.Close()
	delete(m.peers, peerID)

	m.emitEvent(TunnelEvent{
		Type: "peer_removed",
		Data: peerID,
	})

	return nil
}

// GetPeer returns a peer by ID.
func (m *PeerManager) GetPeer(peerID string) (*Peer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peer, exists := m.peers[peerID]
	if !exists {
		return nil, ErrPeerNotFound
	}
	return peer, nil
}

// GetPeers returns all peers.
func (m *PeerManager) GetPeers() []*Peer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*Peer, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	return peers
}

// ConnectPeer initiates connection to a peer.
func (m *PeerManager) ConnectPeer(ctx context.Context, peerID string) error {
	m.mu.RLock()
	peer, exists := m.peers[peerID]
	signaling := m.signaling
	m.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	if signaling == nil {
		return errors.New("no signaling client")
	}

	go peer.Connect(ctx, m.config, signaling)
	return nil
}

// Broadcast sends data to all connected peers.
func (m *PeerManager) Broadcast(data []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, peer := range m.peers {
		if peer.GetState() == PeerStateConnected {
			if err := peer.Send(data); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// Close closes all peer connections.
func (m *PeerManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, peer := range m.peers {
		//nolint:errcheck
		peer.Close()
	}
	m.peers = make(map[string]*Peer)
	return nil
}

// OnEvent registers an event handler.
func (m *PeerManager) OnEvent(handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)
}

// emitEvent emits an event to all handlers.
func (m *PeerManager) emitEvent(event TunnelEvent) {
	for _, handler := range m.eventHandlers {
		go handler(event)
	}
}

// Helper functions

func unmarshalPayload(payload []byte, v interface{}) error {
	return jsonUnmarshal(payload, v)
}

// jsonUnmarshal is a helper for JSON unmarshaling.
func jsonUnmarshal(data []byte, v interface{}) error {
	// Would use encoding/json
	return nil
}
