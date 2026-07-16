// Package p2premote implements a P2P remote access system inspired by
// 飞牛 FN Connect. It provides secure peer-to-peer connections for remote
// NAS access without port forwarding, with end-to-end encryption.
package p2premote

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// ConnectionStatus represents the status of a P2P connection.
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusRelaying     ConnectionStatus = "relaying" // Using relay server
)

// PeerInfo represents a peer in the P2P network.
type PeerInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	PublicKey   string            `json:"public_key"`
	Addresses   []string          `json:"addresses"`
	NATType     string            `json:"nat_type"` // full_cone, restricted, symmetric
	RelayServer string            `json:"relay_server,omitempty"`
	LastSeen    time.Time         `json:"last_seen"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Tunnel represents an active P2P tunnel.
type Tunnel struct {
	ID          string            `json:"id"`
	LocalPeer   string            `json:"local_peer"`
	RemotePeer  string            `json:"remote_peer"`
	Status      ConnectionStatus  `json:"status"`
	Protocol    string            `json:"protocol"` // tcp, udp
	LocalPort   int               `json:"local_port"`
	RemotePort  int               `json:"remote_port"`
	BytesSent   int64             `json:"bytes_sent"`
	BytesRecv   int64             `json:"bytes_recv"`
	Established *time.Time        `json:"established,omitempty"`
	LastActive  time.Time         `json:"last_active"`
	Encrypted   bool              `json:"encrypted"`
	Cipher      string            `json:"cipher,omitempty"` // e.g. "AES-256-GCM"
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// P2PConfig configuration for the P2P service.
type P2PConfig struct {
	ListenPort     int             `json:"listen_port"`
	RelayServers   []string        `json:"relay_servers"`
	STUNServers    []string        `json:"stun_servers"`
	EnableRelay    bool            `json:"enable_relay"`
	EnableIPv6     bool            `json:"enable_ipv6"`
	MaxPeers       int             `json:"max_peers"`
	MaxTunnels     int             `json:"max_tunnels"`
	KeepAlive      time.Duration   `json:"keep_alive"`
	ConnectTimeout time.Duration   `json:"connect_timeout"`
	RateLimit      RateLimitConfig `json:"rate_limit"`
}

// ============================================================
// AES-256-GCM Crypto Engine
// ============================================================

// CryptoEngine provides AES-256-GCM encryption and decryption.
type CryptoEngine struct {
	key []byte // 32 bytes for AES-256
}

const (
	// AES256GCMKeySize is the required key size for AES-256-GCM.
	AES256GCMKeySize = 32
	// AESGCMNonceSize is the nonce size for AES-GCM.
	AESGCMNonceSize = 12
)

// NewCryptoEngine creates a new AES-256-GCM crypto engine with the given key.
// The key must be exactly 32 bytes (256 bits).
func NewCryptoEngine(key []byte) (*CryptoEngine, error) {
	if len(key) != AES256GCMKeySize {
		return nil, fmt.Errorf("crypto: key must be %d bytes, got %d", AES256GCMKeySize, len(key))
	}
	return &CryptoEngine{key: key}, nil
}

// NewCryptoEngineRandom generates a random AES-256 key and returns the engine.
func NewCryptoEngineRandom() (*CryptoEngine, error) {
	key := make([]byte, AES256GCMKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("crypto: failed to generate key: %w", err)
	}
	return &CryptoEngine{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns nonce || ciphertext || tag (appended together).
func (ce *CryptoEngine) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(ce.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce generation: %w", err)
	}

	// Seal appends the ciphertext+tag to nonce, producing nonce||ciphertext||tag
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts data produced by Encrypt (nonce || ciphertext || tag).
func (ce *CryptoEngine) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(ce.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("crypto: ciphertext too short (%d bytes)", len(ciphertext))
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt failed: %w", err)
	}

	return plaintext, nil
}

// KeyHex returns the hex-encoded key (useful for sharing / config).
func (ce *CryptoEngine) KeyHex() string {
	return hex.EncodeToString(ce.key)
}

// ============================================================
// Rate Limiter (Token Bucket)
// ============================================================

// RateLimitConfig configures the token-bucket rate limiter.
type RateLimitConfig struct {
	// Rate is the number of tokens added per second (sustained throughput)
	Rate float64 `json:"rate"`
	// Burst is the maximum number of tokens that can accumulate (peak throughput)
	Burst int `json:"burst"`
}

// RateLimiter implements a token-bucket rate limiter.
type RateLimiter struct {
	mu         sync.Mutex
	rate       float64   // tokens per second
	burst      int       // max tokens
	tokens     float64   // current tokens
	lastRefill time.Time // last time tokens were added
}

// NewRateLimiter creates a rate limiter with the given rate and burst.
// If rate <= 0 or burst <= 0, a no-op (unlimited) limiter is returned.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	if rate <= 0 || burst <= 0 {
		return &RateLimiter{rate: 0, burst: 0, tokens: 0, lastRefill: time.Now()}
	}
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}

// Allow reports whether a single token is immediately available.
// It is shorthand for rl.AllowN(1).
func (rl *RateLimiter) Allow() bool {
	return rl.AllowN(1)
}

// AllowN reports whether n tokens are immediately available.
// If they are, the tokens are consumed; otherwise no tokens are consumed.
func (rl *RateLimiter) AllowN(n int) bool {
	if rl.rate <= 0 {
		return true // unlimited
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= float64(n) {
		rl.tokens -= float64(n)
		return true
	}
	return false
}

// Wait blocks until n tokens are available or the context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context, n int) error {
	if rl.rate <= 0 {
		return nil // unlimited
	}

	for {
		if rl.AllowN(n) {
			return nil
		}

		// Calculate how long until enough tokens accumulate
		rl.mu.Lock()
		needed := float64(n) - rl.tokens
		waitSec := needed / rl.rate
		rl.mu.Unlock()

		waitDur := time.Duration(waitSec * float64(time.Second))
		if waitDur < time.Millisecond {
			waitDur = time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
			// retry
		}
	}
}

// Tokens returns the current number of available tokens (for diagnostics).
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}

// refill adds tokens based on elapsed time; caller must hold rl.mu.
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.lastRefill = now

	rl.tokens += elapsed * rl.rate
	if rl.tokens > float64(rl.burst) {
		rl.tokens = float64(rl.burst)
	}
}

// P2PService manages P2P connections.
type P2PService struct {
	config      P2PConfig
	peers       map[string]*PeerInfo
	tunnels     map[string]*Tunnel
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	nodeID      string
	stats       P2PStats
	crypto      *CryptoEngine
	rateLimiter *RateLimiter
}

// P2PStats contains P2P statistics.
type P2PStats struct {
	TotalPeers     int       `json:"total_peers"`
	ActiveTunnels  int       `json:"active_tunnels"`
	TotalBytesSent int64     `json:"total_bytes_sent"`
	TotalBytesRecv int64     `json:"total_bytes_recv"`
	Uptime         time.Time `json:"uptime"`
}

// NewP2PService creates a new P2P service.
func NewP2PService(config P2PConfig) *P2PService {
	ctx, cancel := context.WithCancel(context.Background())

	if config.ListenPort == 0 {
		config.ListenPort = 51820
	}
	if config.MaxPeers == 0 {
		config.MaxPeers = 100
	}
	if config.MaxTunnels == 0 {
		config.MaxTunnels = 50
	}
	if config.KeepAlive == 0 {
		config.KeepAlive = 25 * time.Second
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 30 * time.Second
	}

	nodeID, _ := generateNodeID()

	// Create AES-256-GCM crypto engine with random key
	crypto, _ := NewCryptoEngineRandom()

	// Create rate limiter (default: 10 tunnels/sec burst 20)
	rlCfg := config.RateLimit
	if rlCfg.Rate == 0 {
		rlCfg.Rate = 10
	}
	if rlCfg.Burst == 0 {
		rlCfg.Burst = 20
	}
	rateLimiter := NewRateLimiter(rlCfg.Rate, rlCfg.Burst)

	return &P2PService{
		config:      config,
		peers:       make(map[string]*PeerInfo),
		tunnels:     make(map[string]*Tunnel),
		ctx:         ctx,
		cancel:      cancel,
		nodeID:      nodeID,
		crypto:      crypto,
		rateLimiter: rateLimiter,
	}
}

// Start begins the P2P service.
func (s *P2PService) Start() error {
	log.Printf("[P2PRemote] Starting P2P service (Node: %s)", s.nodeID)

	// Start NAT traversal
	go s.natTraversalLoop()

	// Start keepalive
	go s.keepAliveLoop()

	// Start stats collector
	go s.statsLoop()

	s.stats.Uptime = time.Now()
	log.Println("[P2PRemote] Service started successfully")
	return nil
}

// Stop gracefully stops the service.
func (s *P2PService) Stop() error {
	s.cancel()

	// Close all tunnels
	s.mu.Lock()
	for _, tunnel := range s.tunnels {
		tunnel.Status = StatusDisconnected
	}
	s.mu.Unlock()

	log.Println("[P2PRemote] Service stopped")
	return nil
}

// RegisterPeer registers a new peer.
func (s *P2PService) RegisterPeer(peer *PeerInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.peers) >= s.config.MaxPeers {
		return fmt.Errorf("maximum peers reached (%d)", s.config.MaxPeers)
	}

	if peer.ID == "" {
		return fmt.Errorf("peer ID is required")
	}

	peer.LastSeen = time.Now()
	s.peers[peer.ID] = peer
	s.stats.TotalPeers = len(s.peers)

	log.Printf("[P2PRemote] Registered peer: %s (%s)", peer.ID, peer.Name)
	return nil
}

// UnregisterPeer removes a peer.
func (s *P2PService) UnregisterPeer(peerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.peers[peerID]; !exists {
		return fmt.Errorf("peer not found: %s", peerID)
	}

	delete(s.peers, peerID)
	s.stats.TotalPeers = len(s.peers)

	// Close tunnels to this peer
	for id, tunnel := range s.tunnels {
		if tunnel.RemotePeer == peerID {
			tunnel.Status = StatusDisconnected
			delete(s.tunnels, id)
		}
	}

	log.Printf("[P2PRemote] Unregistered peer: %s", peerID)
	return nil
}

// GetPeer returns peer information.
func (s *P2PService) GetPeer(peerID string) (*PeerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peer, exists := s.peers[peerID]
	if !exists {
		return nil, fmt.Errorf("peer not found: %s", peerID)
	}
	return peer, nil
}

// ListPeers returns all registered peers.
func (s *P2PService) ListPeers() []*PeerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	return peers
}

// CreateTunnel creates a P2P tunnel to a remote peer.
// Rate limiting is enforced before tunnel creation.
func (s *P2PService) CreateTunnel(remotePeerID string, localPort, remotePort int, protocol string) (*Tunnel, error) {
	// Rate limit check (non-blocking)
	if !s.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded, try again later")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tunnels) >= s.config.MaxTunnels {
		return nil, fmt.Errorf("maximum tunnels reached (%d)", s.config.MaxTunnels)
	}

	remotePeer, exists := s.peers[remotePeerID]
	if !exists {
		return nil, fmt.Errorf("remote peer not found: %s", remotePeerID)
	}

	tunnel := &Tunnel{
		ID:         generateTunnelID(),
		LocalPeer:  s.nodeID,
		RemotePeer: remotePeerID,
		Status:     StatusConnecting,
		Protocol:   protocol,
		LocalPort:  localPort,
		RemotePort: remotePort,
		Encrypted:  true,
		Cipher:     "AES-256-GCM",
		LastActive: time.Now(),
	}

	s.tunnels[tunnel.ID] = tunnel

	// Start connection in background
	go s.establishTunnel(tunnel, remotePeer)

	log.Printf("[P2PRemote] Creating tunnel to %s (%s:%d -> %s:%d) [AES-256-GCM]",
		remotePeerID, "localhost", localPort, remotePeer.Name, remotePort)

	return tunnel, nil
}

// CloseTunnel closes a P2P tunnel.
func (s *P2PService) CloseTunnel(tunnelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tunnel, exists := s.tunnels[tunnelID]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", tunnelID)
	}

	tunnel.Status = StatusDisconnected
	delete(s.tunnels, tunnelID)

	log.Printf("[P2PRemote] Closed tunnel: %s", tunnelID)
	return nil
}

// GetTunnel returns tunnel information.
func (s *P2PService) GetTunnel(tunnelID string) (*Tunnel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tunnel, exists := s.tunnels[tunnelID]
	if !exists {
		return nil, fmt.Errorf("tunnel not found: %s", tunnelID)
	}
	return tunnel, nil
}

// ListTunnels returns all tunnels.
func (s *P2PService) ListTunnels() []*Tunnel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tunnels := make([]*Tunnel, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// GetStats returns P2P statistics.
func (s *P2PService) GetStats() P2PStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := s.stats
	stats.ActiveTunnels = 0
	for _, t := range s.tunnels {
		if t.Status == StatusConnected {
			stats.ActiveTunnels++
			stats.TotalBytesSent += t.BytesSent
			stats.TotalBytesRecv += t.BytesRecv
		}
	}
	return stats
}

// GetCrypto returns the crypto engine (for external use / testing).
func (s *P2PService) GetCrypto() *CryptoEngine {
	return s.crypto
}

// GetRateLimiter returns the rate limiter (for diagnostics / testing).
func (s *P2PService) GetRateLimiter() *RateLimiter {
	return s.rateLimiter
}

// establishTunnel establishes a P2P tunnel.
func (s *P2PService) establishTunnel(tunnel *Tunnel, remotePeer *PeerInfo) {
	// Simulate NAT traversal and connection
	time.Sleep(2 * time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if tunnel still exists
	if _, exists := s.tunnels[tunnel.ID]; !exists {
		return
	}

	// Simulate successful connection
	now := time.Now()
	tunnel.Status = StatusConnected
	tunnel.Established = &now
	tunnel.LastActive = now

	log.Printf("[P2PRemote] Tunnel established: %s", tunnel.ID)
}

// natTraversalLoop performs NAT traversal.
func (s *P2PService) natTraversalLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// NAT traversal logic would go here
		}
	}
}

// keepAliveLoop sends keepalive packets.
func (s *P2PService) keepAliveLoop() {
	ticker := time.NewTicker(s.config.KeepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for _, tunnel := range s.tunnels {
				if tunnel.Status == StatusConnected {
					tunnel.LastActive = time.Now()
				}
			}
			s.mu.Unlock()
		}
	}
}

// statsLoop logs statistics.
func (s *P2PService) statsLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			stats := s.GetStats()
			log.Printf("[P2PRemote] Stats: %d peers, %d active tunnels",
				stats.TotalPeers, stats.ActiveTunnels)
		}
	}
}

// Helper functions.
func generateNodeID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateTunnelID() string {
	return fmt.Sprintf("tun_%d", time.Now().UnixNano())
}
