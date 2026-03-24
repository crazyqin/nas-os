package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Signaling errors
var (
	ErrNotConnected      = errors.New("not connected to signaling server")
	ErrPeerNotFound      = errors.New("peer not found")
	ErrRegistrationFailed = errors.New("registration failed")
	ErrInvalidMessage    = errors.New("invalid message format")
)

// SignalingClient handles signaling server communication
type SignalingClient struct {
	url      string
	conn     *websocket.Conn
	peerID   string
	
	// Message handling
	handlers map[string]MessageHandler
	msgChan  chan *Message
	
	// State
	connected bool
	
	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// MessageHandler handles incoming signaling messages
type MessageHandler func(msg *Message) error

// SignalingServer represents a simple signaling server
type SignalingServer struct {
	port     int
	server   *http.Server
	upgrader websocket.Upgrader
	
	// Connected peers
	peers map[string]*peerConnection
	
	// Message routing
	broadcast chan *Message
	
	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

type peerConnection struct {
	id       string
	conn     *websocket.Conn
	sendChan chan *Message
}

// NewSignalingClient creates a new signaling client
func NewSignalingClient(url string) *SignalingClient {
	return &SignalingClient{
		url:      url,
		handlers: make(map[string]MessageHandler),
		msgChan:  make(chan *Message, 100),
	}
}

// Connect connects to the signaling server
func (c *SignalingClient) Connect(ctx context.Context, peerID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.peerID = peerID

	// Connect via WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := http.Header{}
	headers.Set("X-Peer-ID", peerID)

	conn, _, err := dialer.Dial(c.url, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to signaling server: %w", err)
	}

	c.conn = conn
	c.connected = true

	// Start message handling
	c.wg.Add(1)
	go c.readLoop()

	c.wg.Add(1)
	go c.processLoop()

	return nil
}

// Register registers the peer with the signaling server
func (c *SignalingClient) Register(ctx context.Context, info *PeerInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	msg := &Message{
		Type:      "register",
		From:      c.peerID,
		Timestamp: time.Now(),
	}

	payload, _ := json.Marshal(info)
	msg.Payload = payload

	return c.Send(msg)
}

// Send sends a message through the signaling server
func (c *SignalingClient) Send(msg *Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return ErrNotConnected
	}

	return c.conn.WriteJSON(msg)
}

// SendToPeer sends a message to a specific peer
func (c *SignalingClient) SendToPeer(peerID string, msgType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := &Message{
		Type:      msgType,
		From:      c.peerID,
		To:        peerID,
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}

	return c.Send(msg)
}

// readLoop reads messages from the WebSocket
func (c *SignalingClient) readLoop() {
	defer c.wg.Done()
	defer close(c.msgChan)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if c.conn == nil {
				return
			}

			var msg Message
			if err := c.conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					// Connection closed
					c.connected = false
					return
				}
				continue
			}

			c.msgChan <- &msg
		}
	}
}

// processLoop processes incoming messages
func (c *SignalingClient) processLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.msgChan:
			if !ok {
				return
			}

			handler, exists := c.handlers[msg.Type]
			if exists {
				_ = handler(msg)
			}
		}
	}
}

// OnMessage registers a handler for a message type
func (c *SignalingClient) OnMessage(msgType string, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

// Close closes the signaling client
func (c *SignalingClient) Close() error {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.connected = false
	c.mu.Unlock()

	c.wg.Wait()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected returns whether the client is connected
func (c *SignalingClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetPeerID returns the local peer ID
func (c *SignalingClient) GetPeerID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.peerID
}

// NewSignalingServer creates a new signaling server
func NewSignalingServer(port int) *SignalingServer {
	return &SignalingServer{
		port: port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
		peers:     make(map[string]*peerConnection),
		broadcast: make(chan *Message, 1000),
	}
}

// Start starts the signaling server
func (s *SignalingServer) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	// Start broadcast loop
	s.wg.Add(1)
	go s.broadcastLoop()

	// Start server
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// handleWebSocket handles WebSocket connections
func (s *SignalingServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Get peer ID from header
	peerID := r.Header.Get("X-Peer-ID")
	if peerID == "" {
		peerID = generatePeerID()
	}

	// Register peer
	peer := &peerConnection{
		id:       peerID,
		conn:     conn,
		sendChan: make(chan *Message, 100),
	}

	s.mu.Lock()
	s.peers[peerID] = peer
	s.mu.Unlock()

	// Cleanup on exit
	defer func() {
		s.mu.Lock()
		delete(s.peers, peerID)
		s.mu.Unlock()
	}()

	// Send registration confirmation
	peer.sendChan <- &Message{
		Type:      "registered",
		To:        peerID,
		Timestamp: time.Now(),
	}

	// Read messages from peer
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		msg.From = peerID

		// Route message
		if msg.To != "" {
			// Direct message to peer
			s.routeMessage(&msg)
		} else {
			// Broadcast
			s.broadcast <- &msg
		}
	}
}

// handleHealth handles health check requests
func (s *SignalingServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// routeMessage routes a message to a specific peer
func (s *SignalingServer) routeMessage(msg *Message) {
	s.mu.RLock()
	peer, exists := s.peers[msg.To]
	s.mu.RUnlock()

	if exists {
		select {
		case peer.sendChan <- msg:
		default:
			// Channel full, drop message
		}
	}
}

// broadcastLoop handles message broadcasting
func (s *SignalingServer) broadcastLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.broadcast:
			s.mu.RLock()
			for _, peer := range s.peers {
				if peer.id != msg.From {
					select {
					case peer.sendChan <- msg:
					default:
					}
				}
			}
			s.mu.RUnlock()
		}
	}
}

// Stop stops the signaling server
func (s *SignalingServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}

	s.wg.Wait()

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// GetPeers returns connected peer IDs
func (s *SignalingServer) GetPeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]string, 0, len(s.peers))
	for id := range s.peers {
		peers = append(peers, id)
	}
	return peers
}

// generatePeerID generates a unique peer ID
func generatePeerID() string {
	return fmt.Sprintf("peer-%d", time.Now().UnixNano())
}

// PeerDiscoverer discovers peers on the local network via mDNS
type PeerDiscoverer struct {
	serviceName string
	port        int
	peers       map[string]*PeerInfo
	mu          sync.RWMutex
}

// NewPeerDiscoverer creates a new peer discoverer
func NewPeerDiscoverer(serviceName string, port int) *PeerDiscoverer {
	return &PeerDiscoverer{
		serviceName: serviceName,
		port:        port,
		peers:       make(map[string]*PeerInfo),
	}
}

// Start starts peer discovery via mDNS
func (d *PeerDiscoverer) Start(ctx context.Context) error {
	// mDNS discovery would be implemented here
	// Using zeroconf library for mDNS service discovery
	return nil
}

// GetPeers returns discovered peers
func (d *PeerDiscoverer) GetPeers() []*PeerInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(d.peers))
	for _, peer := range d.peers {
		peers = append(peers, peer)
	}
	return peers
}

// OnPeerFound handles peer discovery
func (d *PeerDiscoverer) OnPeerFound(peer *PeerInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.peers[peer.ID] = peer
}

// OnPeerLost handles peer loss
func (d *PeerDiscoverer) OnPeerLost(peerID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.peers, peerID)
}

// BroadcastDiscovery broadcasts peer discovery on local network
func BroadcastDiscovery(ctx context.Context, port int, info *PeerInfo) error {
	// Broadcast discovery packet
	addr, err := net.ResolveUDPAddr("udp", "224.0.0.1:1900")
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

// ListenDiscovery listens for peer discovery broadcasts
func ListenDiscovery(ctx context.Context, port int, handler func(*PeerInfo)) error {
	addr, err := net.ResolveUDPAddr("udp", "224.0.0.1:1900")
	if err != nil {
		return err
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			var peer PeerInfo
			if err := json.Unmarshal(buf[:n], &peer); err != nil {
				continue
			}

			handler(&peer)
		}
	}
}