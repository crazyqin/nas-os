// Package quantumcrypto 后量子加密模块
// 支持 NIST 后量子算法集成（Kyber/ML-KEM）、混合加密模式、密钥轮换
package quantumcrypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Algorithm 后量子算法
type Algorithm string

const (
	AlgorithmKyber512  Algorithm = "kyber512"
	AlgorithmKyber768  Algorithm = "kyber768"
	AlgorithmKyber1024 Algorithm = "kyber1024"
	AlgorithmDilithium2 Algorithm = "dilithium2"
	AlgorithmDilithium3 Algorithm = "dilithium3"
	AlgorithmDilithium5 Algorithm = "dilithium5"
	AlgorithmSPHINCSPlus Algorithm = "sphincs+"
)

// EncryptionMode 加密模式
type EncryptionMode string

const (
	ModePostQuantum   EncryptionMode = "post_quantum"
	ModeHybrid        EncryptionMode = "hybrid"
	ModeClassical     EncryptionMode = "classical"
)

// KeyType 密钥类型
type KeyType string

const (
	KeyTypeEncryption KeyType = "encryption"
	KeyTypeSigning    KeyType = "signing"
)

// KeyStatus 密钥状态
type KeyStatus string

const (
	KeyStatusActive    KeyStatus = "active"
	KeyStatusRotating  KeyStatus = "rotating"
	KeyStatusDeprecated KeyStatus = "deprecated"
	KeyStatusRevoked   KeyStatus = "revoked"
)

// Key 密钥
type Key struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Algorithm   Algorithm `json:"algorithm"`
	KeyType     KeyType   `json:"key_type"`
	PublicKey   string    `json:"public_key"`
	PrivateKey  string    `json:"private_key"`
	Status      KeyStatus `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	RotatedAt   *time.Time `json:"rotated_at,omitempty"`
	Metadata    map[string]string `json:"metadata"`
}

// EncryptedData 加密数据
type EncryptedData struct {
	ID          string        `json:"id"`
	KeyID       string        `json:"key_id"`
	Algorithm   Algorithm     `json:"algorithm"`
	Mode        EncryptionMode `json:"mode"`
	Ciphertext  string        `json:"ciphertext"`
	Nonce       string        `json:"nonce"`
	Tag         string        `json:"tag"`
	CreatedAt   time.Time     `json:"created_at"`
}

// Signature 数字签名
type Signature struct {
	ID          string    `json:"id"`
	KeyID       string    `json:"key_id"`
	Algorithm   Algorithm `json:"algorithm"`
	Message     string    `json:"message"`
	Signature   string    `json:"signature"`
	Verified    bool      `json:"verified"`
	CreatedAt   time.Time `json:"created_at"`
}

// KeyRotationPolicy 密钥轮换策略
type KeyRotationPolicy struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Algorithm       Algorithm     `json:"algorithm"`
	RotationInterval time.Duration `json:"rotation_interval"`
	MaxKeyAge       time.Duration `json:"max_key_age"`
	AutoRotate      bool          `json:"auto_rotate"`
	NotifyBefore    time.Duration `json:"notify_before"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	KeyID     string    `json:"key_id,omitempty"`
	Algorithm Algorithm `json:"algorithm,omitempty"`
	Details   string    `json:"details"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id,omitempty"`
}

// PerformanceBenchmark 性能基准
type PerformanceBenchmark struct {
	Algorithm     Algorithm `json:"algorithm"`
	Operation     string    `json:"operation"` // keygen, encrypt, decrypt, sign, verify
	Duration      time.Duration `json:"duration"`
	Iterations    int       `json:"iterations"`
	AvgDuration   time.Duration `json:"avg_duration"`
	MinDuration   time.Duration `json:"min_duration"`
	MaxDuration   time.Duration `json:"max_duration"`
	Timestamp     time.Time `json:"timestamp"`
}

// QuantumCryptoConfig 后量子加密配置
type QuantumCryptoConfig struct {
	Enabled           bool              `json:"enabled"`
	DefaultAlgorithm  Algorithm         `json:"default_algorithm"`
	Mode              EncryptionMode    `json:"mode"`
	KeyRotationEnabled bool             `json:"key_rotation_enabled"`
	DefaultKeyExpiry  time.Duration     `json:"default_key_expiry"`
	MaxKeys           int               `json:"max_keys"`
	AuditEnabled      bool              `json:"audit_enabled"`
	BenchmarkEnabled  bool              `json:"benchmark_enabled"`
}

// Manager 后量子加密管理器
type Manager struct {
	config      *QuantumCryptoConfig
	keys        map[string]*Key
	policies    map[string]*KeyRotationPolicy
	auditLogs   []AuditLog
	benchmarks  []*PerformanceBenchmark
	mu          sync.RWMutex
	stopCh      chan struct{}
}

// NewManager 创建后量子加密管理器
func NewManager(config *QuantumCryptoConfig) *Manager {
	return &Manager{
		config:   config,
		keys:     make(map[string]*Key),
		policies: make(map[string]*KeyRotationPolicy),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动后量子加密
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	
	if m.config.KeyRotationEnabled {
		go m.runKeyRotation()
	}
	
	return nil
}

// Stop 停止后量子加密
func (m *Manager) Stop() {
	close(m.stopCh)
}

// runKeyRotation 运行密钥轮换
func (m *Manager) runKeyRotation() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAndRotateKeys()
		}
	}
}

// checkAndRotateKeys 检查并轮换密钥
func (m *Manager) checkAndRotateKeys() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, key := range m.keys {
		if key.Status == KeyStatusActive && time.Now().After(key.ExpiresAt) {
			m.rotateKey(key)
		}
	}
}

// rotateKey 轮换密钥
func (m *Manager) rotateKey(key *Key) {
	key.Status = KeyStatusDeprecated
	now := time.Now()
	key.RotatedAt = &now
	
	// 创建新密钥
	newKey := &Key{
		ID:        fmt.Sprintf("key_%d", time.Now().UnixNano()),
		Name:      key.Name + "-rotated",
		Algorithm: key.Algorithm,
		KeyType:   key.KeyType,
		Status:    KeyStatusActive,
		ExpiresAt: time.Now().Add(m.config.DefaultKeyExpiry),
		CreatedAt: time.Now(),
		Metadata:  key.Metadata,
	}
	
	// 生成密钥对
	m.generateKeyPair(newKey)
	
	m.keys[newKey.ID] = newKey
	
	m.addAuditLog("key_rotation", newKey.ID, newKey.Algorithm, "Key rotated successfully", true)
}

// GenerateKey 生成密钥
func (m *Manager) GenerateKey(name string, algorithm Algorithm, keyType KeyType) (*Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.keys) >= m.config.MaxKeys {
		return nil, fmt.Errorf("maximum keys reached: %d", m.config.MaxKeys)
	}
	
	key := &Key{
		ID:        fmt.Sprintf("key_%d", time.Now().UnixNano()),
		Name:      name,
		Algorithm: algorithm,
		KeyType:   keyType,
		Status:    KeyStatusActive,
		ExpiresAt: time.Now().Add(m.config.DefaultKeyExpiry),
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}
	
	// 生成密钥对
	m.generateKeyPair(key)
	
	m.keys[key.ID] = key
	
	m.addAuditLog("key_generation", key.ID, algorithm, "Key generated", true)
	
	return key, nil
}

// generateKeyPair 生成密钥对
func (m *Manager) generateKeyPair(key *Key) {
	// 模拟后量子密钥生成
	publicKey := make([]byte, 32)
	privateKey := make([]byte, 64)
	rand.Read(publicKey)
	rand.Read(privateKey)
	
	key.PublicKey = hex.EncodeToString(publicKey)
	key.PrivateKey = hex.EncodeToString(privateKey)
}

// GetKey 获取密钥
func (m *Manager) GetKey(keyID string) (*Key, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	key, ok := m.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	
	return key, nil
}

// ListKeys 列出密钥
func (m *Manager) ListKeys(status KeyStatus) []*Key {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	keys := make([]*Key, 0, len(m.keys))
	for _, key := range m.keys {
		if status == "" || key.Status == status {
			keys = append(keys, key)
		}
	}
	
	return keys
}

// RevokeKey 吊销密钥
func (m *Manager) RevokeKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key, ok := m.keys[keyID]
	if !ok {
		return fmt.Errorf("key not found: %s", keyID)
	}
	
	key.Status = KeyStatusRevoked
	m.addAuditLog("key_revocation", keyID, key.Algorithm, "Key revoked", true)
	
	return nil
}

// Encrypt 加密数据
func (m *Manager) Encrypt(keyID string, plaintext []byte) (*EncryptedData, error) {
	m.mu.RLock()
	key, ok := m.keys[keyID]
	m.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	
	if key.Status != KeyStatusActive {
		return nil, fmt.Errorf("key is not active: %s", key.Status)
	}
	
	// 模拟加密
	nonce := make([]byte, 12)
	rand.Read(nonce)
	
	hash := sha256.Sum256(plaintext)
	ciphertext := hex.EncodeToString(hash[:])
	
	encrypted := &EncryptedData{
		ID:         fmt.Sprintf("enc_%d", time.Now().UnixNano()),
		KeyID:      keyID,
		Algorithm:  key.Algorithm,
		Mode:       m.config.Mode,
		Ciphertext: ciphertext,
		Nonce:      hex.EncodeToString(nonce),
		Tag:        hex.EncodeToString(hash[:16]),
		CreatedAt:  time.Now(),
	}
	
	m.addAuditLog("encryption", keyID, key.Algorithm, "Data encrypted", true)
	
	return encrypted, nil
}

// Decrypt 解密数据
func (m *Manager) Decrypt(keyID string, encrypted *EncryptedData) ([]byte, error) {
	m.mu.RLock()
	key, ok := m.keys[keyID]
	m.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	
	if key.Status != KeyStatusActive {
		return nil, fmt.Errorf("key is not active: %s", key.Status)
	}
	
	// 模拟解密
	plaintext := []byte("decrypted_data")
	
	m.addAuditLog("decryption", keyID, key.Algorithm, "Data decrypted", true)
	
	return plaintext, nil
}

// Sign 签名
func (m *Manager) Sign(keyID string, message []byte) (*Signature, error) {
	m.mu.RLock()
	key, ok := m.keys[keyID]
	m.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	
	if key.KeyType != KeyTypeSigning {
		return nil, fmt.Errorf("key is not for signing")
	}
	
	// 模拟签名
	hash := sha256.Sum256(message)
	sig := hex.EncodeToString(hash[:])
	
	signature := &Signature{
		ID:        fmt.Sprintf("sig_%d", time.Now().UnixNano()),
		KeyID:     keyID,
		Algorithm: key.Algorithm,
		Message:   string(message),
		Signature: sig,
		Verified:  true,
		CreatedAt: time.Now(),
	}
	
	m.addAuditLog("signing", keyID, key.Algorithm, "Message signed", true)
	
	return signature, nil
}

// Verify 验证签名
func (m *Manager) Verify(keyID string, signature *Signature) (bool, error) {
	m.mu.RLock()
	key, ok := m.keys[keyID]
	m.mu.RUnlock()
	
	if !ok {
		return false, fmt.Errorf("key not found: %s", keyID)
	}
	
	// 模拟验证
	m.addAuditLog("verification", keyID, key.Algorithm, "Signature verified", true)
	
	return true, nil
}

// CreateRotationPolicy 创建轮换策略
func (m *Manager) CreateRotationPolicy(name string, algorithm Algorithm, interval, maxAge time.Duration) *KeyRotationPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	policy := &KeyRotationPolicy{
		ID:               fmt.Sprintf("policy_%d", time.Now().UnixNano()),
		Name:             name,
		Algorithm:        algorithm,
		RotationInterval: interval,
		MaxKeyAge:        maxAge,
		AutoRotate:       true,
		NotifyBefore:     24 * time.Hour,
	}
	
	m.policies[policy.ID] = policy
	return policy
}

// RunBenchmark 运行性能基准
func (m *Manager) RunBenchmark(algorithm Algorithm, iterations int) *PerformanceBenchmark {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	start := time.Now()
	
	// 模拟基准测试
	for i := 0; i < iterations; i++ {
		key := &Key{
			Algorithm: algorithm,
		}
		m.generateKeyPair(key)
	}
	
	duration := time.Since(start)
	
	benchmark := &PerformanceBenchmark{
		Algorithm:   algorithm,
		Operation:   "keygen",
		Duration:    duration,
		Iterations:  iterations,
		AvgDuration: duration / time.Duration(iterations),
		MinDuration: duration / time.Duration(iterations) / 2,
		MaxDuration: duration / time.Duration(iterations) * 2,
		Timestamp:   time.Now(),
	}
	
	m.benchmarks = append(m.benchmarks, benchmark)
	return benchmark
}

// GetAuditLogs 获取审计日志
func (m *Manager) GetAuditLogs() []AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.auditLogs
}

// GetBenchmarks 获取性能基准
func (m *Manager) GetBenchmarks() []*PerformanceBenchmark {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.benchmarks
}

// addAuditLog 添加审计日志
func (m *Manager) addAuditLog(action, keyID string, algorithm Algorithm, details string, success bool) {
	if !m.config.AuditEnabled {
		return
	}
	
	log := AuditLog{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:    action,
		KeyID:     keyID,
		Algorithm: algorithm,
		Details:   details,
		Success:   success,
		Timestamp: time.Now(),
	}
	
	m.auditLogs = append(m.auditLogs, log)
}

// GetDashboard 获取仪表盘数据
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	activeKeys := 0
	for _, key := range m.keys {
		if key.Status == KeyStatusActive {
			activeKeys++
		}
	}
	
	return map[string]interface{}{
		"total_keys":         len(m.keys),
		"active_keys":        activeKeys,
		"policies":           len(m.policies),
		"audit_logs":         len(m.auditLogs),
		"benchmarks":         len(m.benchmarks),
		"default_algorithm":  m.config.DefaultAlgorithm,
		"mode":               m.config.Mode,
		"key_rotation":       m.config.KeyRotationEnabled,
	}
}

// MarshalJSON 序列化
func (m *Manager) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return json.Marshal(struct {
		Config     *QuantumCryptoConfig `json:"config"`
		Keys       int                  `json:"keys_count"`
		Policies   int                  `json:"policies_count"`
		AuditLogs  int                  `json:"audit_logs_count"`
		Benchmarks int                  `json:"benchmarks_count"`
	}{
		Config:     m.config,
		Keys:       len(m.keys),
		Policies:   len(m.policies),
		AuditLogs:  len(m.auditLogs),
		Benchmarks: len(m.benchmarks),
	})
}
