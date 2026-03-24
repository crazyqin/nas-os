package tunnel

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if len(config.STUNServers) == 0 {
		t.Error("DefaultConfig should have STUN servers")
	}

	if config.ListenPort <= 0 {
		t.Error("DefaultConfig should have valid listen port")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *TunnelConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty STUN servers",
			config: &TunnelConfig{
				STUNServers: []string{},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &TunnelConfig{
				STUNServers: []string{"stun:stun.l.google.com:19302"},
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			config: &TunnelConfig{
				STUNServers: []string{"stun:stun.l.google.com:19302"},
				ListenPort:  -1,
			},
			wantErr: true,
		},
		{
			name: "port too high",
			config: &TunnelConfig{
				STUNServers: []string{"stun:stun.l.google.com:19302"},
				ListenPort:  70000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigBuilder(t *testing.T) {
	config, err := NewConfigBuilder().
		WithListenPort(51820).
		WithSTUNServers("stun:stun.l.google.com:19302").
		WithSignaling("wss://signal.example.com").
		WithKeepalive(30 * time.Second).
		WithMaxPeers(50).
		Build()

	if err != nil {
		t.Fatalf("ConfigBuilder.Build() error = %v", err)
	}

	if config.ListenPort != 51820 {
		t.Errorf("ListenPort = %d, want 51820", config.ListenPort)
	}

	if len(config.STUNServers) != 1 {
		t.Errorf("STUNServers count = %d, want 1", len(config.STUNServers))
	}

	if config.SignalingURL != "wss://signal.example.com" {
		t.Errorf("SignalingURL = %s, want wss://signal.example.com", config.SignalingURL)
	}

	if config.MaxPeers != 50 {
		t.Errorf("MaxPeers = %d, want 50", config.MaxPeers)
	}
}

func TestNATTypeString(t *testing.T) {
	tests := []struct {
		natType NATType
		want    string
	}{
		{NATUnknown, "Unknown"},
		{NATNone, "None"},
		{NATFullCone, "Full Cone"},
		{NATRestrictedCone, "Restricted Cone"},
		{NATPortRestricted, "Port Restricted"},
		{NATSymmetric, "Symmetric"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.natType.String(); got != tt.want {
				t.Errorf("NATType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionTypeString(t *testing.T) {
	tests := []struct {
		connType ConnectionType
		want     string
	}{
		{ConnectionUnknown, "Unknown"},
		{ConnectionDirect, "Direct"},
		{ConnectionRelay, "Relay"},
		{ConnectionHolePunched, "Hole Punched"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.connType.String(); got != tt.want {
				t.Errorf("ConnectionType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateKeyPair(t *testing.T) {
	pub1, priv1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if len(pub1) != 32 {
		t.Errorf("public key length = %d, want 32", len(pub1))
	}

	if len(priv1) != 32 {
		t.Errorf("private key length = %d, want 32", len(priv1))
	}

	// Generate another pair and verify they're different
	pub2, _, _ := GenerateKeyPair()
	if string(pub1) == string(pub2) {
		t.Error("GenerateKeyPair should produce different keys")
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey() error = %v", err)
	}

	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}
}

func TestComputeHMAC(t *testing.T) {
	key := []byte("test-key")
	data := []byte("test-data")

	mac := ComputeHMAC(key, data)
	if len(mac) != 32 {
		t.Errorf("HMAC length = %d, want 32", len(mac))
	}

	// Verify HMAC
	if !VerifyHMAC(key, data, mac) {
		t.Error("HMAC verification failed")
	}

	// Wrong key should fail
	if VerifyHMAC([]byte("wrong-key"), data, mac) {
		t.Error("HMAC should not verify with wrong key")
	}
}

func TestHash(t *testing.T) {
	data := []byte("test-data")
	hash := Hash(data)

	if len(hash) != 32 {
		t.Errorf("hash length = %d, want 32", len(hash))
	}

	// Same input should produce same hash
	hash2 := Hash(data)
	if string(hash) != string(hash2) {
		t.Error("Hash should be deterministic")
	}

	// Different input should produce different hash
	hash3 := Hash([]byte("different-data"))
	if string(hash) == string(hash3) {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestNeedsRelay(t *testing.T) {
	tests := []struct {
		natType NATType
		want    bool
	}{
		{NATNone, false},
		{NATFullCone, false},
		{NATSymmetric, true},
		{NATSymmetricUDPFirewall, true},
	}

	for _, tt := range tests {
		t.Run(tt.natType.String(), func(t *testing.T) {
			if got := needsRelay(tt.natType); got != tt.want {
				t.Errorf("needsRelay(%v) = %v, want %v", tt.natType, got, tt.want)
			}
		})
	}
}

func TestCalculatePriority(t *testing.T) {
	hostPriority := calculatePriority(CandidateTypeHost, 0, ICEComponentRTP)
	srflxPriority := calculatePriority(CandidateTypeSrflx, 0, ICEComponentRTP)
	relayPriority := calculatePriority(CandidateTypeRelay, 0, ICEComponentRTP)

	// Host should have highest priority
	if hostPriority <= srflxPriority {
		t.Error("Host candidate should have higher priority than srflx")
	}

	if srflxPriority <= relayPriority {
		t.Error("Srflx candidate should have higher priority than relay")
	}
}

func TestSTUNClient(t *testing.T) {
	config := DefaultConfig()
	client := NewSTUNClient(config)

	if client == nil {
		t.Fatal("NewSTUNClient returned nil")
	}

	// Close should not error
	if err := client.Close(); err != nil {
		t.Errorf("STUNClient.Close() error = %v", err)
	}
}

func TestPeerConfig(t *testing.T) {
	config := &PeerConfig{
		ID:        "test-peer",
		PublicKey: make([]byte, 32),
		Endpoints: []*net.UDPAddr{
			{IP: net.ParseIP("192.168.1.1"), Port: 51820},
		},
	}

	peer := NewPeer(config)

	if peer == nil {
		t.Fatal("NewPeer returned nil")
	}

	if peer.ID != "test-peer" {
		t.Errorf("peer.ID = %s, want test-peer", peer.ID)
	}

	if peer.GetState() != PeerStateNew {
		t.Errorf("peer state = %v, want %v", peer.GetState(), PeerStateNew)
	}
}

func TestPeerStateString(t *testing.T) {
	tests := []struct {
		state PeerState
		want  string
	}{
		{PeerStateNew, "new"},
		{PeerStateConnecting, "connecting"},
		{PeerStateConnected, "connected"},
		{PeerStateDisconnected, "disconnected"},
		{PeerStateFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("PeerState.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTunnelManager(t *testing.T) {
	config := DefaultConfig()
	manager, err := NewTunnelManager(config)

	if err != nil {
		t.Fatalf("NewTunnelManager() error = %v", err)
	}

	if manager == nil {
		t.Fatal("NewTunnelManager returned nil")
	}

	if manager.GetPeerID() == "" {
		t.Error("Manager should have a peer ID")
	}

	if manager.GetPublicKey() == nil {
		t.Error("Manager should have a public key")
	}
}

func TestPeerManager(t *testing.T) {
	config := DefaultConfig()
	manager := NewPeerManager(config)

	if manager == nil {
		t.Fatal("NewPeerManager returned nil")
	}

	// Add a peer
	peerConfig := &PeerConfig{
		ID:        "test-peer",
		PublicKey: make([]byte, 32),
	}

	peer, err := manager.AddPeer(peerConfig)
	if err != nil {
		t.Fatalf("AddPeer() error = %v", err)
	}

	if peer == nil {
		t.Fatal("AddPeer returned nil")
	}

	// Get the peer
	gotPeer, err := manager.GetPeer("test-peer")
	if err != nil {
		t.Fatalf("GetPeer() error = %v", err)
	}

	if gotPeer.ID != "test-peer" {
		t.Errorf("got peer ID = %s, want test-peer", gotPeer.ID)
	}

	// Try adding duplicate peer
	_, err = manager.AddPeer(peerConfig)
	if err != ErrPeerAlreadyExists {
		t.Errorf("duplicate AddPeer should return ErrPeerAlreadyExists, got %v", err)
	}

	// Remove peer
	if err := manager.RemovePeer("test-peer"); err != nil {
		t.Fatalf("RemovePeer() error = %v", err)
	}

	// Get non-existent peer
	_, err = manager.GetPeer("test-peer")
	if err != ErrPeerNotFound {
		t.Errorf("GetPeer should return ErrPeerNotFound, got %v", err)
	}
}

func TestCryptoBasic(t *testing.T) {
	config := &CryptoConfig{
		CipherType: CipherChaCha20Poly1305,
	}

	crypto, err := NewCrypto(config)
	if err != nil {
		t.Fatalf("NewCrypto() error = %v", err)
	}

	// Generate key pair
	if err := crypto.GenerateKeyPair(); err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	pubKey := crypto.GetPublicKey()
	if pubKey == nil {
		t.Error("GetPublicKey returned nil")
	}

	if len(pubKey) != 32 {
		t.Errorf("public key length = %d, want 32", len(pubKey))
	}
}

func TestSigner(t *testing.T) {
	signer, err := NewSigner()
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}

	data := []byte("test message")

	signature := signer.Sign(data)
	if signature == nil {
		t.Fatal("Sign returned nil")
	}

	if !signer.Verify(data, signature) {
		t.Error("signature verification failed")
	}

	// Tampered data should fail verification
	if signer.Verify([]byte("tampered"), signature) {
		t.Error("tampered data should not verify")
	}
}

func TestICECandidatePriority(t *testing.T) {
	local := &Candidate{
		Type:     CandidateTypeHost,
		IP:       net.ParseIP("192.168.1.1"),
		Port:     51820,
		Priority: calculatePriority(CandidateTypeHost, 0, ICEComponentRTP),
	}

	remote := &Candidate{
		Type:     CandidateTypeSrflx,
		IP:       net.ParseIP("1.2.3.4"),
		Port:     12345,
		Priority: calculatePriority(CandidateTypeSrflx, 0, ICEComponentRTP),
	}

	pairPriority := calculatePairPriority(local.Priority, remote.Priority, true)

	if pairPriority == 0 {
		t.Error("pair priority should not be zero")
	}

	// Local priority should be higher than remote
	controlling := calculatePairPriority(local.Priority, remote.Priority, true)
	controlled := calculatePairPriority(local.Priority, remote.Priority, false)

	if controlling == controlled {
		t.Error("controlling and controlled priorities should differ")
	}
}

func TestContextCancellation(t *testing.T) {
	config := DefaultConfig()
	manager, err := NewTunnelManager(config)
	if err != nil {
		t.Fatalf("NewTunnelManager() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// Start manager
	startErr := make(chan error, 1)
	go func() {
		startErr <- manager.Start(ctx)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Close should work even after cancellation
	if err := manager.Close(); err != nil {
		// Expected: connection already closed
		t.Logf("Close() returned expected error: %v", err)
	}
}