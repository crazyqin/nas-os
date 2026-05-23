package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages WireGuard interface and peers
type Manager struct {
	mu        sync.RWMutex
	iface     WireGuardInterface
	peers     map[string]WireGuardPeer
	privateKey string
}

// NewManager creates a new WireGuard manager with mock data
func NewManager() *Manager {
	m := &Manager{
		peers: make(map[string]WireGuardPeer),
		iface: WireGuardInterface{
			Name:       "wg0",
			ListenPort: 51820,
			Address:    "10.0.0.1/24",
			DNS:        "1.1.1.1",
			Enabled:    true,
			MTU:        1420,
		},
	}
	
	// Generate mock keys
	pub, priv, _ := m.GenerateKeyPair()
	m.iface.PublicKey = pub
	m.privateKey = priv
	
	// Add some mock peers
	m.addMockPeers()
	
	return m
}

func (m *Manager) addMockPeers() {
	mockPeers := []struct {
		name      string
		allowedIPs string
	}{
		{"phone", "10.0.0.2/32"},
		{"laptop", "10.0.0.3/32"},
		{"tablet", "10.0.0.4/32"},
	}
	
	for _, mp := range mockPeers {
		pub, _, _ := m.GenerateKeyPair()
		peer := WireGuardPeer{
			ID:                  uuid.New().String(),
			PublicKey:           pub,
			AllowedIPs:          mp.allowedIPs,
			PersistentKeepalive: 25,
			Enabled:             true,
			CreatedAt:           time.Now(),
			BytesRx:             int64(1024 * (time.Now().Unix() % 100)),
			BytesTx:             int64(512 * (time.Now().Unix() % 100)),
			LastHandshake:       time.Now().Add(-time.Duration(time.Now().Unix()%60) * time.Minute),
		}
		m.peers[peer.ID] = peer
	}
}

// GetInterface returns the current WireGuard interface configuration
func (m *Manager) GetInterface() (*WireGuardInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	iface := m.iface
	iface.PrivateKey = m.privateKey
	
	// Include peers
	iface.Peers = make([]WireGuardPeer, 0, len(m.peers))
	for _, p := range m.peers {
		iface.Peers = append(iface.Peers, p)
	}
	
	return &iface, nil
}

// ConfigureInterface updates the WireGuard interface configuration
func (m *Manager) ConfigureInterface(cfg InterfaceConfigRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if cfg.Name != nil {
		m.iface.Name = *cfg.Name
	}
	if cfg.ListenPort != nil {
		if *cfg.ListenPort < 1 || *cfg.ListenPort > 65535 {
			return fmt.Errorf("invalid listen port: %d", *cfg.ListenPort)
		}
		m.iface.ListenPort = *cfg.ListenPort
	}
	if cfg.PrivateKey != nil {
		m.privateKey = *cfg.PrivateKey
		// Generate public key from private (mock: just use as is for demo)
		m.iface.PublicKey = *cfg.PrivateKey
	}
	if cfg.Address != nil {
		m.iface.Address = *cfg.Address
	}
	if cfg.DNS != nil {
		m.iface.DNS = *cfg.DNS
	}
	if cfg.MTU != nil {
		if *cfg.MTU < 1 || *cfg.MTU > 65535 {
			return fmt.Errorf("invalid MTU: %d", *cfg.MTU)
		}
		m.iface.MTU = *cfg.MTU
	}
	if cfg.Enabled != nil {
		m.iface.Enabled = *cfg.Enabled
	}
	
	return nil
}

// ListPeers returns all configured peers
func (m *Manager) ListPeers() ([]WireGuardPeer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	peers := make([]WireGuardPeer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	return peers, nil
}

// GetPeer returns a specific peer by ID
func (m *Manager) GetPeer(id string) (*WireGuardPeer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	peer, ok := m.peers[id]
	if !ok {
		return nil, fmt.Errorf("peer not found: %s", id)
	}
	return &peer, nil
}

// CreatePeer creates a new WireGuard peer
func (m *Manager) CreatePeer(req CreatePeerRequest) (*WireGuardPeer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check for duplicate public key
	for _, p := range m.peers {
		if p.PublicKey == req.PublicKey {
			return nil, fmt.Errorf("peer with public key already exists")
		}
	}
	
	peer := WireGuardPeer{
		ID:                  uuid.New().String(),
		PublicKey:           req.PublicKey,
		AllowedIPs:          req.AllowedIPs,
		Endpoint:            req.Endpoint,
		PersistentKeepalive: req.PersistentKeepalive,
		Enabled:             true,
		CreatedAt:           time.Now(),
	}
	
	if req.Enabled != nil {
		peer.Enabled = *req.Enabled
	}
	
	m.peers[peer.ID] = peer
	return &peer, nil
}

// UpdatePeer updates an existing WireGuard peer
func (m *Manager) UpdatePeer(id string, req UpdatePeerRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	peer, ok := m.peers[id]
	if !ok {
		return fmt.Errorf("peer not found: %s", id)
	}
	
	if req.PublicKey != nil {
		// Check for duplicate
		for pid, p := range m.peers {
			if pid != id && p.PublicKey == *req.PublicKey {
				return fmt.Errorf("peer with public key already exists")
			}
		}
		peer.PublicKey = *req.PublicKey
	}
	if req.AllowedIPs != nil {
		peer.AllowedIPs = *req.AllowedIPs
	}
	if req.Endpoint != nil {
		peer.Endpoint = *req.Endpoint
	}
	if req.PersistentKeepalive != nil {
		peer.PersistentKeepalive = *req.PersistentKeepalive
	}
	if req.Enabled != nil {
		peer.Enabled = *req.Enabled
	}
	
	m.peers[id] = peer
	return nil
}

// DeletePeer removes a WireGuard peer
func (m *Manager) DeletePeer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.peers[id]; !ok {
		return fmt.Errorf("peer not found: %s", id)
	}
	
	delete(m.peers, id)
	return nil
}

// GetStats returns aggregated WireGuard statistics
func (m *Manager) GetStats() (*WireGuardStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := &WireGuardStats{
		TotalPeers: len(m.peers),
	}
	
	for _, p := range m.peers {
		stats.TotalBytesRx += p.BytesRx
		stats.TotalBytesTx += p.BytesTx
		if p.LastHandshake.After(time.Now().Add(-3 * time.Minute)) {
			stats.ActivePeers++
		}
	}
	
	return stats, nil
}

// EnableInterface enables the WireGuard interface
func (m *Manager) EnableInterface() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.iface.Enabled = true
	return nil
}

// DisableInterface disables the WireGuard interface
func (m *Manager) DisableInterface() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.iface.Enabled = false
	return nil
}

// GenerateKeyPair generates a new WireGuard key pair (mock implementation)
func (m *Manager) GenerateKeyPair() (publicKey, privateKey string, err error) {
	// Mock: generate random base64 strings that look like WireGuard keys
	privBytes := make([]byte, 32)
	if _, err := rand.Read(privBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}
	privateKey = base64.StdEncoding.EncodeToString(privBytes)
	
	pubBytes := make([]byte, 32)
	if _, err := rand.Read(pubBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKey = base64.StdEncoding.EncodeToString(pubBytes)
	
	return publicKey, privateKey, nil
}

// GeneratePeerConfig generates a WireGuard client configuration for a peer
func (m *Manager) GeneratePeerConfig(peer *WireGuardPeer) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if peer == nil {
		return "", fmt.Errorf("peer cannot be nil")
	}
	
	config := fmt.Sprintf(`[Interface]
PrivateKey = <PRIVATE_KEY>
Address = %s
DNS = %s

[Peer]
PublicKey = %s
Endpoint = <SERVER_ADDRESS>:%d
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = %d
`, peer.AllowedIPs, m.iface.DNS, m.iface.PublicKey, m.iface.ListenPort, peer.PersistentKeepalive)
	
	return config, nil
}
