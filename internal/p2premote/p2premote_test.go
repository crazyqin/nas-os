package p2premote

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// Original tests (preserved)
// ============================================================

func TestNewP2PService(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	assert.NotNil(t, service)
}

func TestStartStop(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)

	err = service.Stop()
	assert.NoError(t, err)
}

func TestRegisterPeer(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	peer := &PeerInfo{
		ID:        "peer1",
		Name:      "My NAS",
		PublicKey: "abc123",
		Addresses: []string{"192.168.1.100:51820"},
		NATType:   "full_cone",
	}

	err = service.RegisterPeer(peer)
	assert.NoError(t, err)

	stats := service.GetStats()
	assert.Equal(t, 1, stats.TotalPeers)
}

func TestRegisterPeer_MaxPeers(t *testing.T) {
	service := NewP2PService(P2PConfig{MaxPeers: 2})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	for i := 0; i < 2; i++ {
		peer := &PeerInfo{
			ID:   fmt.Sprintf("peer%d", i),
			Name: fmt.Sprintf("Peer %d", i),
		}
		err := service.RegisterPeer(peer)
		require.NoError(t, err)
	}

	peer := &PeerInfo{ID: "peer3", Name: "Peer 3"}
	err = service.RegisterPeer(peer)
	assert.Error(t, err)
}

func TestUnregisterPeer(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	peer := &PeerInfo{
		ID:   "peer1",
		Name: "My NAS",
	}

	err = service.RegisterPeer(peer)
	require.NoError(t, err)

	err = service.UnregisterPeer("peer1")
	assert.NoError(t, err)

	stats := service.GetStats()
	assert.Equal(t, 0, stats.TotalPeers)
}

func TestUnregisterPeer_NotFound(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	err = service.UnregisterPeer("nonexistent")
	assert.Error(t, err)
}

func TestGetPeer(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	peer := &PeerInfo{
		ID:   "peer1",
		Name: "My NAS",
	}

	err = service.RegisterPeer(peer)
	require.NoError(t, err)

	retrieved, err := service.GetPeer("peer1")
	require.NoError(t, err)
	assert.Equal(t, "My NAS", retrieved.Name)
}

func TestGetPeer_NotFound(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	_, err = service.GetPeer("nonexistent")
	assert.Error(t, err)
}

func TestListPeers(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	for i := 0; i < 3; i++ {
		peer := &PeerInfo{
			ID:   fmt.Sprintf("peer%d", i),
			Name: fmt.Sprintf("Peer %d", i),
		}
		err := service.RegisterPeer(peer)
		require.NoError(t, err)
	}

	peers := service.ListPeers()
	assert.Len(t, peers, 3)
}

func TestCreateTunnel(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	peer := &PeerInfo{
		ID:   "peer1",
		Name: "Remote NAS",
	}

	err = service.RegisterPeer(peer)
	require.NoError(t, err)

	tunnel, err := service.CreateTunnel("peer1", 8080, 80, "tcp")
	require.NoError(t, err)
	assert.NotEmpty(t, tunnel.ID)
	assert.Equal(t, StatusConnecting, tunnel.Status)
	assert.Equal(t, "AES-256-GCM", tunnel.Cipher)

	// Wait for connection
	time.Sleep(3 * time.Second)

	updated, err := service.GetTunnel(tunnel.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusConnected, updated.Status)
}

func TestCreateTunnel_PeerNotFound(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	_, err = service.CreateTunnel("nonexistent", 8080, 80, "tcp")
	assert.Error(t, err)
}

func TestCloseTunnel(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	peer := &PeerInfo{
		ID:   "peer1",
		Name: "Remote NAS",
	}

	err = service.RegisterPeer(peer)
	require.NoError(t, err)

	tunnel, err := service.CreateTunnel("peer1", 8080, 80, "tcp")
	require.NoError(t, err)

	err = service.CloseTunnel(tunnel.ID)
	assert.NoError(t, err)

	_, err = service.GetTunnel(tunnel.ID)
	assert.Error(t, err)
}

func TestCloseTunnel_NotFound(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	err = service.CloseTunnel("nonexistent")
	assert.Error(t, err)
}

func TestListTunnels(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	peer := &PeerInfo{
		ID:   "peer1",
		Name: "Remote NAS",
	}

	err = service.RegisterPeer(peer)
	require.NoError(t, err)

	_, err = service.CreateTunnel("peer1", 8080, 80, "tcp")
	require.NoError(t, err)

	_, err = service.CreateTunnel("peer1", 8081, 443, "tcp")
	require.NoError(t, err)

	tunnels := service.ListTunnels()
	assert.Len(t, tunnels, 2)
}

func TestGetStats(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	err := service.Start()
	require.NoError(t, err)
	defer service.Stop()

	peer := &PeerInfo{
		ID:   "peer1",
		Name: "Remote NAS",
	}

	err = service.RegisterPeer(peer)
	require.NoError(t, err)

	_, err = service.CreateTunnel("peer1", 8080, 80, "tcp")
	require.NoError(t, err)

	stats := service.GetStats()
	assert.Equal(t, 1, stats.TotalPeers)
	assert.False(t, stats.Uptime.IsZero())
}

// ============================================================
// AES-256-GCM Crypto Engine Tests
// ============================================================

func TestCryptoEngine_RoundTrip(t *testing.T) {
	ce, err := NewCryptoEngineRandom()
	require.NoError(t, err)

	plaintext := []byte("hello, P2P encrypted world!")
	ct, err := ce.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ct)

	recovered, err := ce.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, plaintext, recovered)
}

func TestCryptoEngine_EncryptProducesDifferentCiphertexts(t *testing.T) {
	ce, err := NewCryptoEngineRandom()
	require.NoError(t, err)

	msg := []byte("same plaintext")
	ct1, err := ce.Encrypt(msg)
	require.NoError(t, err)
	ct2, err := ce.Encrypt(msg)
	require.NoError(t, err)

	// Different nonces → different ciphertexts
	assert.NotEqual(t, ct1, ct2)

	// Both decrypt to same plaintext
	p1, _ := ce.Decrypt(ct1)
	p2, _ := ce.Decrypt(ct2)
	assert.Equal(t, p1, p2)
	assert.Equal(t, msg, p1)
}

func TestCryptoEngine_WrongKeyFails(t *testing.T) {
	ce1, _ := NewCryptoEngineRandom()
	ce2, _ := NewCryptoEngineRandom()

	ct, err := ce1.Encrypt([]byte("secret"))
	require.NoError(t, err)

	_, err = ce2.Decrypt(ct)
	assert.Error(t, err, "decryption with wrong key should fail")
}

func TestCryptoEngine_InvalidKeySize(t *testing.T) {
	_, err := NewCryptoEngine([]byte("too-short"))
	assert.Error(t, err)

	_, err = NewCryptoEngine(make([]byte, 16)) // AES-128 size
	assert.Error(t, err)
}

func TestCryptoEngine_CorruptedCiphertext(t *testing.T) {
	ce, _ := NewCryptoEngineRandom()
	ct, err := ce.Encrypt([]byte("data"))
	require.NoError(t, err)

	// Flip a byte in the ciphertext portion
	ct[len(ct)-1] ^= 0xFF
	_, err = ce.Decrypt(ct)
	assert.Error(t, err)
}

func TestCryptoEngine_EmptyPlaintext(t *testing.T) {
	ce, _ := NewCryptoEngineRandom()
	ct, err := ce.Encrypt([]byte{})
	require.NoError(t, err)

	pt, err := ce.Decrypt(ct)
	require.NoError(t, err)
	assert.Empty(t, pt)
}

func TestCryptoEngine_LargeData(t *testing.T) {
	ce, _ := NewCryptoEngineRandom()
	big := make([]byte, 1024*1024) // 1 MB
	for i := range big {
		big[i] = byte(i % 251)
	}

	ct, err := ce.Encrypt(big)
	require.NoError(t, err)

	pt, err := ce.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, big, pt)
}

func TestCryptoEngine_KeyHex(t *testing.T) {
	ce, _ := NewCryptoEngineRandom()
	hex := ce.KeyHex()
	assert.Len(t, hex, 64) // 32 bytes → 64 hex chars
}

func TestP2PService_TunnelCipherField(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	require.NoError(t, service.Start())
	defer service.Stop()

	_ = service.RegisterPeer(&PeerInfo{ID: "p1", Name: "P1"})
	tun, err := service.CreateTunnel("p1", 9000, 9001, "tcp")
	require.NoError(t, err)
	assert.Equal(t, "AES-256-GCM", tun.Cipher)
	assert.True(t, tun.Encrypted)
}

func TestP2PService_GetCrypto(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	ce := service.GetCrypto()
	require.NotNil(t, ce)

	ct, err := ce.Encrypt([]byte("test"))
	require.NoError(t, err)
	pt, err := ce.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, []byte("test"), pt)
}

// ============================================================
// Rate Limiter Tests
// ============================================================

func TestRateLimiter_BasicAllow(t *testing.T) {
	rl := NewRateLimiter(10, 10) // 10/sec, burst 10

	// Should allow up to burst immediately
	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow(), "token %d should be allowed", i)
	}

	// 11th should be denied (burst exhausted)
	assert.False(t, rl.Allow(), "should be rate limited after burst")
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(100, 10) // 100/sec, burst 10

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		rl.Allow()
	}
	assert.False(t, rl.Allow())

	// Wait for refill (~50ms → 5 tokens at 100/sec)
	time.Sleep(60 * time.Millisecond)
	assert.True(t, rl.Allow())
}

func TestRateLimiter_AllowN(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	assert.True(t, rl.AllowN(5))
	assert.True(t, rl.AllowN(5))
	assert.False(t, rl.AllowN(1)) // exhausted
}

func TestRateLimiter_AllowN_ExceedsBurst(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	assert.False(t, rl.AllowN(6), "cannot consume more than burst")
	assert.True(t, rl.AllowN(5))
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(1000, 1) // fast refill, small burst

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Drain the single burst token
	rl.Allow()

	// Wait should succeed quickly (~1ms refill at 1000/sec)
	err := rl.Wait(ctx, 1)
	assert.NoError(t, err)
}

func TestRateLimiter_WaitContextCancelled(t *testing.T) {
	rl := NewRateLimiter(0.1, 1) // very slow: 1 token per 10 sec

	rl.Allow() // drain

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx, 1)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestRateLimiter_UnlimitedWhenZero(t *testing.T) {
	// rate=0 → unlimited
	rl := NewRateLimiter(0, 0)

	for i := 0; i < 1000; i++ {
		assert.True(t, rl.Allow())
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(100, 50)

	var wg sync.WaitGroup
	allowed := int32(0)
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// At most burst (50) should succeed initially
	assert.LessOrEqual(t, allowed, int32(50))
	assert.Greater(t, allowed, int32(0))
}

func TestRateLimiter_TokensDiagnostics(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	tokens := rl.Tokens()
	assert.InDelta(t, 10.0, tokens, 0.5)

	rl.AllowN(3)
	tokens = rl.Tokens()
	assert.InDelta(t, 7.0, tokens, 0.5)
}

// ============================================================
// Rate Limiter Integration with P2PService
// ============================================================

func TestP2PService_RateLimitOnCreateTunnel(t *testing.T) {
	service := NewP2PService(P2PConfig{
		RateLimit: RateLimitConfig{Rate: 2, Burst: 2},
	})
	require.NoError(t, service.Start())
	defer service.Stop()

	_ = service.RegisterPeer(&PeerInfo{ID: "p1", Name: "P1"})

	// First two should succeed (burst=2)
	_, err := service.CreateTunnel("p1", 8001, 80, "tcp")
	assert.NoError(t, err)
	_, err = service.CreateTunnel("p1", 8002, 80, "tcp")
	assert.NoError(t, err)

	// Third should be rate limited
	_, err = service.CreateTunnel("p1", 8003, 80, "tcp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestP2PService_RateLimiterAccess(t *testing.T) {
	service := NewP2PService(P2PConfig{
		RateLimit: RateLimitConfig{Rate: 5, Burst: 5},
	})

	rl := service.GetRateLimiter()
	assert.NotNil(t, rl)

	// Should allow up to burst
	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.Allow() {
			allowed++
		}
	}
	assert.Equal(t, 5, allowed)
}

func TestP2PService_DefaultRateLimit(t *testing.T) {
	service := NewP2PService(P2PConfig{})
	rl := service.GetRateLimiter()
	require.NotNil(t, rl)

	// Default: 10/sec, burst 20
	allowed := 0
	for i := 0; i < 30; i++ {
		if rl.Allow() {
			allowed++
		}
	}
	assert.Equal(t, 20, allowed)
}
