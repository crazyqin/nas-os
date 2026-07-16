package quantumcrypto

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:            true,
		DefaultAlgorithm:   AlgorithmKyber768,
		Mode:               ModeHybrid,
		KeyRotationEnabled: true,
		DefaultKeyExpiry:   365 * 24 * time.Hour,
		MaxKeys:            100,
		AuditEnabled:       true,
		BenchmarkEnabled:   true,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.config.DefaultAlgorithm != AlgorithmKyber768 {
		t.Errorf("Expected algorithm kyber768, got %s", manager.config.DefaultAlgorithm)
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:            true,
		DefaultAlgorithm:   AlgorithmKyber768,
		Mode:               ModeHybrid,
		KeyRotationEnabled: false,
		DefaultKeyExpiry:   365 * 24 * time.Hour,
		MaxKeys:            100,
		AuditEnabled:       true,
	}

	manager := NewManager(config)

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	manager.Stop()
}

func TestGenerateKey(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmKyber768,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		AuditEnabled:     true,
	}

	manager := NewManager(config)

	key, err := manager.GenerateKey("test-key", AlgorithmKyber768, KeyTypeEncryption)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if key.ID == "" {
		t.Error("Key ID not generated")
	}

	if key.PublicKey == "" {
		t.Error("Public key not generated")
	}

	if key.PrivateKey == "" {
		t.Error("Private key not generated")
	}

	if key.Status != KeyStatusActive {
		t.Errorf("Expected status active, got %s", key.Status)
	}
}

func TestGetKey(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmKyber768,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		AuditEnabled:     true,
	}

	manager := NewManager(config)

	key, _ := manager.GenerateKey("test-key", AlgorithmKyber768, KeyTypeEncryption)

	got, err := manager.GetKey(key.ID)
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}

	if got.ID != key.ID {
		t.Errorf("Expected key ID %s, got %s", key.ID, got.ID)
	}
}

func TestListKeys(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmKyber768,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		AuditEnabled:     true,
	}

	manager := NewManager(config)

	manager.GenerateKey("key-1", AlgorithmKyber768, KeyTypeEncryption)
	manager.GenerateKey("key-2", AlgorithmKyber1024, KeyTypeSigning)

	keys := manager.ListKeys(KeyStatusActive)

	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestRevokeKey(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmKyber768,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		AuditEnabled:     true,
	}

	manager := NewManager(config)

	key, _ := manager.GenerateKey("test-key", AlgorithmKyber768, KeyTypeEncryption)

	if err := manager.RevokeKey(key.ID); err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}

	if key.Status != KeyStatusRevoked {
		t.Errorf("Expected status revoked, got %s", key.Status)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmKyber768,
		Mode:             ModeHybrid,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		AuditEnabled:     true,
	}

	manager := NewManager(config)

	key, _ := manager.GenerateKey("test-key", AlgorithmKyber768, KeyTypeEncryption)

	plaintext := []byte("Hello, World!")

	encrypted, err := manager.Encrypt(key.ID, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted.Ciphertext == "" {
		t.Error("Ciphertext is empty")
	}

	if encrypted.KeyID != key.ID {
		t.Errorf("Expected key ID %s, got %s", key.ID, encrypted.KeyID)
	}

	decrypted, err := manager.Decrypt(key.ID, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if len(decrypted) == 0 {
		t.Error("Decrypted data is empty")
	}
}

func TestSignVerify(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmDilithium3,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		AuditEnabled:     true,
	}

	manager := NewManager(config)

	key, _ := manager.GenerateKey("signing-key", AlgorithmDilithium3, KeyTypeSigning)

	message := []byte("Message to sign")

	signature, err := manager.Sign(key.ID, message)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if signature.Signature == "" {
		t.Error("Signature is empty")
	}

	verified, err := manager.Verify(key.ID, signature)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !verified {
		t.Error("Signature verification failed")
	}
}

func TestCreateRotationPolicy(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:            true,
		DefaultAlgorithm:   AlgorithmKyber768,
		KeyRotationEnabled: true,
		DefaultKeyExpiry:   365 * 24 * time.Hour,
		MaxKeys:            100,
		AuditEnabled:       true,
	}

	manager := NewManager(config)

	policy := manager.CreateRotationPolicy("test-policy", AlgorithmKyber768, 30*24*time.Hour, 90*24*time.Hour)

	if policy == nil {
		t.Fatal("CreateRotationPolicy returned nil")
	}

	if policy.Name != "test-policy" {
		t.Errorf("Expected name test-policy, got %s", policy.Name)
	}

	if !policy.AutoRotate {
		t.Error("Expected AutoRotate to be true")
	}
}

func TestRunBenchmark(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmKyber768,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		BenchmarkEnabled: true,
	}

	manager := NewManager(config)

	benchmark := manager.RunBenchmark(AlgorithmKyber768, 100)

	if benchmark == nil {
		t.Fatal("RunBenchmark returned nil")
	}

	if benchmark.Iterations != 100 {
		t.Errorf("Expected 100 iterations, got %d", benchmark.Iterations)
	}

	if benchmark.Duration == 0 {
		t.Error("Duration should not be 0")
	}
}

func TestGetAuditLogs(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmKyber768,
		DefaultKeyExpiry: 365 * 24 * time.Hour,
		MaxKeys:          100,
		AuditEnabled:     true,
	}

	manager := NewManager(config)

	manager.GenerateKey("test-key", AlgorithmKyber768, KeyTypeEncryption)

	logs := manager.GetAuditLogs()

	if len(logs) == 0 {
		t.Error("Expected audit logs, got 0")
	}
}

func TestGetDashboard(t *testing.T) {
	config := &QuantumCryptoConfig{
		Enabled:            true,
		DefaultAlgorithm:   AlgorithmKyber768,
		Mode:               ModeHybrid,
		KeyRotationEnabled: true,
		DefaultKeyExpiry:   365 * 24 * time.Hour,
		MaxKeys:            100,
		AuditEnabled:       true,
	}

	manager := NewManager(config)

	dashboard := manager.GetDashboard()

	if dashboard["total_keys"] != 0 {
		t.Errorf("Expected 0 total_keys, got %v", dashboard["total_keys"])
	}

	if dashboard["default_algorithm"] != AlgorithmKyber768 {
		t.Errorf("Expected algorithm kyber768, got %v", dashboard["default_algorithm"])
	}
}

func TestAlgorithms(t *testing.T) {
	algorithms := []Algorithm{
		AlgorithmKyber512,
		AlgorithmKyber768,
		AlgorithmKyber1024,
		AlgorithmDilithium2,
		AlgorithmDilithium3,
		AlgorithmDilithium5,
		AlgorithmSPHINCSPlus,
	}

	for _, a := range algorithms {
		if string(a) == "" {
			t.Errorf("Empty algorithm: %v", a)
		}
	}
}

func TestEncryptionModes(t *testing.T) {
	modes := []EncryptionMode{
		ModePostQuantum,
		ModeHybrid,
		ModeClassical,
	}

	for _, m := range modes {
		if string(m) == "" {
			t.Errorf("Empty encryption mode: %v", m)
		}
	}
}

func TestKeyStatuses(t *testing.T) {
	statuses := []KeyStatus{
		KeyStatusActive,
		KeyStatusRotating,
		KeyStatusDeprecated,
		KeyStatusRevoked,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("Empty key status: %v", s)
		}
	}
}
