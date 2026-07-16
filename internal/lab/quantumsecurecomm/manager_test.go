package quantumsecurecomm

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("Expected non-nil manager")
	}

	if mgr.defaultAlgo != AlgorithmKyber {
		t.Errorf("Expected default algorithm Kyber, got %s", mgr.defaultAlgo)
	}
	if mgr.defaultSecurity != SecurityLevel1 {
		t.Errorf("Expected default security level1, got %s", mgr.defaultSecurity)
	}
}

func TestNewManager_WithConfig(t *testing.T) {
	cfg := &ManagerConfig{
		DefaultAlgorithm: AlgorithmDilithium,
		DefaultSecurity:  SecurityLevel3,
		MaxChannels:      500,
		MaxHandshakes:    50,
		AuditLogSize:     5000,
	}

	mgr := NewManager(cfg)
	if mgr.defaultAlgo != AlgorithmDilithium {
		t.Errorf("Expected Dilithium, got %s", mgr.defaultAlgo)
	}
	if mgr.defaultSecurity != SecurityLevel3 {
		t.Errorf("Expected level3, got %s", mgr.defaultSecurity)
	}
}

func TestManager_StartStop(t *testing.T) {
	mgr := NewManager(nil)

	mgr.Start()
	if !mgr.running {
		t.Error("Expected manager to be running")
	}

	// 重复启动应无操作
	mgr.Start()

	mgr.Stop()
	if mgr.running {
		t.Error("Expected manager to be stopped")
	}
}

func TestManager_GenerateKeyPair(t *testing.T) {
	mgr := NewManager(nil)

	kp, err := mgr.GenerateKeyPair(AlgorithmKyber, SecurityLevel1)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	if kp == nil {
		t.Fatal("Expected non-nil key pair")
	}
	if len(kp.PublicKey) == 0 {
		t.Error("Expected non-empty public key")
	}
	if len(kp.PrivateKey) == 0 {
		t.Error("Expected non-empty private key")
	}
	if kp.Algorithm != AlgorithmKyber {
		t.Errorf("Expected Kyber, got %s", kp.Algorithm)
	}
	if kp.SecurityLevel != SecurityLevel1 {
		t.Errorf("Expected level1, got %s", kp.SecurityLevel)
	}
}

func TestManager_GenerateKeyPair_DefaultAlgorithm(t *testing.T) {
	mgr := NewManager(&ManagerConfig{
		DefaultAlgorithm: AlgorithmDilithium,
	})

	kp, err := mgr.GenerateKeyPair("", SecurityLevel1)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	if kp.Algorithm != AlgorithmDilithium {
		t.Errorf("Expected Dilithium, got %s", kp.Algorithm)
	}
}

func TestManager_InitiateHandshake(t *testing.T) {
	mgr := NewManager(nil)

	session, err := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)
	if err != nil {
		t.Fatalf("Failed to initiate handshake: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}
	if session.State != HandshakeStateInit {
		t.Errorf("Expected state init, got %s", session.State)
	}
	if session.ID == "" {
		t.Error("Expected non-empty session ID")
	}
}

func TestManager_CompleteHandshake(t *testing.T) {
	mgr := NewManager(nil)

	// 发起握手
	session, _ := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)

	// 完成握手
	remotePubKey := make([]byte, 800)
	channel, err := mgr.CompleteHandshake(session.ID, remotePubKey)
	if err != nil {
		t.Fatalf("Failed to complete handshake: %v", err)
	}

	if channel == nil {
		t.Fatal("Expected non-nil channel")
	}
	if channel.State != ChannelStateEstablished {
		t.Errorf("Expected state established, got %s", channel.State)
	}
}

func TestManager_CompleteHandshake_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.CompleteHandshake("non-existent", []byte{})
	if err == nil {
		t.Error("Expected error for non-existent handshake")
	}
}

func TestManager_EncryptDecryptMessage(t *testing.T) {
	mgr := NewManager(nil)

	// 建立通道
	session, _ := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)
	channel, _ := mgr.CompleteHandshake(session.ID, make([]byte, 800))

	// 加密
	plaintext := []byte("Hello, Quantum World!")
	encrypted, err := mgr.EncryptMessage(channel.ID, plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if encrypted.ChannelID != channel.ID {
		t.Errorf("Channel ID mismatch: got %s, want %s", encrypted.ChannelID, channel.ID)
	}
	if len(encrypted.Ciphertext) == 0 {
		t.Error("Expected non-empty ciphertext")
	}

	// 解密
	decrypted, err := mgr.DecryptMessage(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text mismatch: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestManager_EncryptMessage_ChannelNotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.EncryptMessage("non-existent", []byte("test"))
	if err == nil {
		t.Error("Expected error for non-existent channel")
	}
}

func TestManager_SignAndVerify(t *testing.T) {
	mgr := NewManager(nil)

	// 生成密钥对
	_, err := mgr.GenerateKeyPair(AlgorithmDilithium, SecurityLevel1)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// 获取实际生成的 key ID
	var keyID string
	mgr.mu.RLock()
	for id := range mgr.keyPairs {
		keyID = id
		break
	}
	mgr.mu.RUnlock()

	// 签名
	message := []byte("Test message to sign")
	sig, err := mgr.SignMessage(keyID, message)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	if sig == nil {
		t.Fatal("Expected non-nil signature")
	}
	if len(sig.Signature) == 0 {
		t.Error("Expected non-empty signature")
	}

	// 验证
	valid, err := mgr.VerifySignature(sig)
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}
	if !valid {
		t.Error("Expected signature to be valid")
	}
}

func TestManager_SignMessage_KeyNotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.SignMessage("non-existent", []byte("test"))
	if err == nil {
		t.Error("Expected error for non-existent key")
	}
}

func TestManager_VerifySignature_Invalid(t *testing.T) {
	mgr := NewManager(nil)

	// 空签名
	valid, err := mgr.VerifySignature(&DigitalSignature{
		Algorithm: AlgorithmDilithium,
	})
	if err == nil {
		t.Error("Expected error for empty signature")
	}
	if valid {
		t.Error("Expected invalid for empty signature")
	}
}

func TestManager_CloseChannel(t *testing.T) {
	mgr := NewManager(nil)

	// 建立通道
	session, _ := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)
	channel, _ := mgr.CompleteHandshake(session.ID, make([]byte, 800))

	// 关闭通道
	err := mgr.CloseChannel(channel.ID)
	if err != nil {
		t.Fatalf("Failed to close channel: %v", err)
	}

	// 验证状态
	ch, _ := mgr.GetChannel(channel.ID)
	if ch.State != ChannelStateClosed {
		t.Errorf("Expected state closed, got %s", ch.State)
	}
}

func TestManager_CloseChannel_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.CloseChannel("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent channel")
	}
}

func TestManager_GetChannel(t *testing.T) {
	mgr := NewManager(nil)

	session, _ := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)
	channel, _ := mgr.CompleteHandshake(session.ID, make([]byte, 800))

	retrieved, err := mgr.GetChannel(channel.ID)
	if err != nil {
		t.Fatalf("Failed to get channel: %v", err)
	}
	if retrieved.ID != channel.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, channel.ID)
	}
}

func TestManager_GetChannel_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.GetChannel("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent channel")
	}
}

func TestManager_ListChannels(t *testing.T) {
	mgr := NewManager(nil)

	// 初始应为空
	list := mgr.ListChannels()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d", len(list))
	}

	// 创建通道
	session1, _ := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)
	mgr.CompleteHandshake(session1.ID, make([]byte, 800))

	session2, _ := mgr.InitiateHandshake(AlgorithmDilithium, SecurityLevel1)
	mgr.CompleteHandshake(session2.ID, make([]byte, 1312))

	list = mgr.ListChannels()
	if len(list) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(list))
	}
}

func TestManager_GetHandshake(t *testing.T) {
	mgr := NewManager(nil)

	session, _ := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)

	retrieved, err := mgr.GetHandshake(session.ID)
	if err != nil {
		t.Fatalf("Failed to get handshake: %v", err)
	}
	if retrieved.ID != session.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, session.ID)
	}
}

func TestManager_GetHandshake_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.GetHandshake("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent handshake")
	}
}

func TestManager_GetSupportedAlgorithms(t *testing.T) {
	mgr := NewManager(nil)

	algos := mgr.GetSupportedAlgorithms()
	if len(algos) < 4 {
		t.Errorf("Expected at least 4 algorithms, got %d", len(algos))
	}

	// 验证包含 Kyber
	found := false
	for _, a := range algos {
		if a.Type == AlgorithmKyber {
			found = true
			if !a.IsNISTStandard {
				t.Error("Expected Kyber to be NIST standard")
			}
		}
	}
	if !found {
		t.Error("Expected Kyber in supported algorithms")
	}
}

func TestManager_GetState(t *testing.T) {
	mgr := NewManager(nil)
	mgr.Start()

	state := mgr.GetState()
	if state == nil {
		t.Fatal("Expected non-nil state")
	}
	if state.ActiveChannels != 0 {
		t.Errorf("Expected 0 active channels, got %d", state.ActiveChannels)
	}

	// 创建通道后再次检查
	session, _ := mgr.InitiateHandshake(AlgorithmKyber, SecurityLevel1)
	mgr.CompleteHandshake(session.ID, make([]byte, 800))

	state = mgr.GetState()
	if state.ActiveChannels != 1 {
		t.Errorf("Expected 1 active channel, got %d", state.ActiveChannels)
	}

	mgr.Stop()
}

func TestManager_GetAuditLog(t *testing.T) {
	mgr := NewManager(nil)

	// 生成密钥对会触发审计日志
	_, _ = mgr.GenerateKeyPair(AlgorithmKyber, SecurityLevel1)
	_, _ = mgr.GenerateKeyPair(AlgorithmDilithium, SecurityLevel1)

	log := mgr.GetAuditLog(10)
	if len(log) < 2 {
		t.Errorf("Expected at least 2 audit entries, got %d", len(log))
	}

	// 测试 limit
	log = mgr.GetAuditLog(1)
	if len(log) != 1 {
		t.Errorf("Expected 1 audit entry, got %d", len(log))
	}
}

func TestManager_RunWithContext(t *testing.T) {
	mgr := NewManager(nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		mgr.RunWithContext(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	if !mgr.running {
		t.Error("Expected manager to be running")
	}

	cancel()
	wg.Wait()

	if mgr.running {
		t.Error("Expected manager to be stopped after context cancel")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr := NewManager(nil)

	var wg sync.WaitGroup
	const goroutines = 5

	// 并发生成密钥对
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = mgr.GenerateKeyPair(AlgorithmKyber, SecurityLevel1)
		}()
	}

	// 并发读取
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = mgr.GetSupportedAlgorithms()
			_ = mgr.ListChannels()
		}()
	}

	wg.Wait()

	mgr.mu.RLock()
	count := len(mgr.keyPairs)
	mgr.mu.RUnlock()

	if count != goroutines {
		t.Errorf("Expected %d key pairs, got %d", goroutines, count)
	}
}

func TestAlgorithmType_Constants(t *testing.T) {
	tests := []struct {
		name string
		algo AlgorithmType
		want string
	}{
		{"Kyber", AlgorithmKyber, "kyber"},
		{"Dilithium", AlgorithmDilithium, "dilithium"},
		{"SPHINCS+", AlgorithmSPHINCSPlus, "sphincs_plus"},
		{"Falcon", AlgorithmFalcon, "falcon"},
		{"NTRU", AlgorithmNTRU, "ntru"},
		{"ClassicMcEliece", AlgorithmClassicMcEliece, "classic_mceliece"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.algo) != tt.want {
				t.Errorf("got %s, want %s", tt.algo, tt.want)
			}
		})
	}
}

func TestSecurityLevel_Constants(t *testing.T) {
	if SecurityLevel1 != "level1" {
		t.Errorf("SecurityLevel1 = %s, want level1", SecurityLevel1)
	}
	if SecurityLevel3 != "level3" {
		t.Errorf("SecurityLevel3 = %s, want level3", SecurityLevel3)
	}
	if SecurityLevel5 != "level5" {
		t.Errorf("SecurityLevel5 = %s, want level5", SecurityLevel5)
	}
}

func TestHandshakeState_Constants(t *testing.T) {
	states := []HandshakeState{
		HandshakeStateInit,
		HandshakeStateKeyExchange,
		HandshakeStateAuthentication,
		HandshakeStateComplete,
		HandshakeStateFailed,
	}
	if len(states) != 5 {
		t.Errorf("Expected 5 handshake states, got %d", len(states))
	}
}
