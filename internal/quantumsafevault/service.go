package quantumsafevault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// Algorithm 后量子算法
type Algorithm string

const (
	AlgorithmKyber     Algorithm = "kyber"     // CRYSTALS-Kyber (密钥封装)
	AlgorithmDilithium Algorithm = "dilithium" // CRYSTALS-Dilithium (数字签名)
	AlgorithmFalcon    Algorithm = "falcon"    // Falcon (数字签名)
	AlgorithmSPHINCS   Algorithm = "sphincs+"  // SPHINCS+ (数字签名)
	AlgorithmNTRU      Algorithm = "ntru"      // NTRU (密钥封装)
	AlgorithmSABER     Algorithm = "saber"     // SABER (密钥封装)
)

// SecurityLevel 安全等级
type SecurityLevel int

const (
	SecurityLevel1 SecurityLevel = 1 // 128-bit classical security
	SecurityLevel3 SecurityLevel = 3 // 192-bit classical security
	SecurityLevel5 SecurityLevel = 5 // 256-bit classical security
)

// KeyType 密钥类型
type KeyType string

const (
	KeyTypeEncryption KeyType = "encryption"
	KeyTypeSigning    KeyType = "signing"
)

// KeyPair 密钥对
type KeyPair struct {
	ID         string        `json:"id"`
	Algorithm  Algorithm     `json:"algorithm"`
	KeyType    KeyType       `json:"key_type"`
	Security   SecurityLevel `json:"security"`
	PublicKey  []byte        `json:"public_key"`
	PrivateKey []byte        `json:"private_key"`
	CreatedAt  time.Time     `json:"created_at"`
	ExpiresAt  time.Time     `json:"expires_at"`
	Tags       []string      `json:"tags"`
}

// EncryptedData 加密数据
type EncryptedData struct {
	ID        string            `json:"id"`
	KeyID     string            `json:"key_id"`
	Algorithm Algorithm         `json:"algorithm"`
	Data      []byte            `json:"data"`
	IV        []byte            `json:"iv"`
	AuthTag   []byte            `json:"auth_tag"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata"`
}

// Signature 数字签名
type Signature struct {
	ID        string    `json:"id"`
	KeyID     string    `json:"key_id"`
	Algorithm Algorithm `json:"algorithm"`
	Data      []byte    `json:"data"`
	Signature []byte    `json:"signature"`
	CreatedAt time.Time `json:"created_at"`
	Verified  bool      `json:"verified"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	User      string    `json:"user"`
	IP        string    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details"`
	Success   bool      `json:"success"`
}

// Service 量子安全保险库服务
type Service struct {
	keys       map[string]*KeyPair
	encrypted  map[string]*EncryptedData
	signatures map[string]*Signature
	auditLog   []AuditLog
	mu         sync.RWMutex
}

// NewService 创建服务
func NewService() *Service {
	return &Service{
		keys:       make(map[string]*KeyPair),
		encrypted:  make(map[string]*EncryptedData),
		signatures: make(map[string]*Signature),
		auditLog:   make([]AuditLog, 0),
	}
}

// GenerateKeyPair 生成密钥对
func (s *Service) GenerateKeyPair(algorithm Algorithm, keyType KeyType, security SecurityLevel, tags []string) (*KeyPair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyPair := &KeyPair{
		ID:        fmt.Sprintf("key_%d", time.Now().UnixNano()),
		Algorithm: algorithm,
		KeyType:   keyType,
		Security:  security,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1年有效期
		Tags:      tags,
	}

	// 模拟密钥生成
	switch algorithm {
	case AlgorithmKyber:
		keyPair.PublicKey = s.generateKyberPublicKey(security)
		keyPair.PrivateKey = s.generateKyberPrivateKey(security)
	case AlgorithmDilithium:
		keyPair.PublicKey = s.generateDilithiumPublicKey(security)
		keyPair.PrivateKey = s.generateDilithiumPrivateKey(security)
	case AlgorithmFalcon:
		keyPair.PublicKey = s.generateFalconPublicKey(security)
		keyPair.PrivateKey = s.generateFalconPrivateKey(security)
	default:
		keyPair.PublicKey = s.generateGenericKey(32)
		keyPair.PrivateKey = s.generateGenericKey(64)
	}

	s.keys[keyPair.ID] = keyPair

	s.addAuditLog("generate_key", keyPair.ID, "system", "", fmt.Sprintf("Generated %s key pair", algorithm), true)

	return keyPair, nil
}

// GetKey 获取密钥
func (s *Service) GetKey(id string) (*KeyPair, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.keys[id]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", id)
	}

	return key, nil
}

// ListKeys 列出密钥
func (s *Service) ListKeys(keyType KeyType, algorithm Algorithm) []*KeyPair {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*KeyPair
	for _, key := range s.keys {
		if (keyType == "" || key.KeyType == keyType) &&
			(algorithm == "" || key.Algorithm == algorithm) {
			result = append(result, key)
		}
	}

	return result
}

// DeleteKey 删除密钥
func (s *Service) DeleteKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[id]; !ok {
		return fmt.Errorf("key not found: %s", id)
	}

	delete(s.keys, id)
	s.addAuditLog("delete_key", id, "system", "", "Key deleted", true)

	return nil
}

// Encrypt 加密数据
func (s *Service) Encrypt(keyID string, data []byte, metadata map[string]string) (*EncryptedData, error) {
	s.mu.RLock()
	key, ok := s.keys[keyID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	s.mu.RUnlock()

	// 使用AES-GCM进行实际加密（后量子算法用于密钥交换）
	block, err := aes.NewCipher(s.deriveSymmetricKey(key))
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	iv := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	encrypted := aesGCM.Seal(nil, iv, data, nil)
	authTag := encrypted[len(encrypted)-aesGCM.Overhead():]
	ciphertext := encrypted[:len(encrypted)-aesGCM.Overhead()]

	encryptedData := &EncryptedData{
		ID:        fmt.Sprintf("enc_%d", time.Now().UnixNano()),
		KeyID:     keyID,
		Algorithm: key.Algorithm,
		Data:      ciphertext,
		IV:        iv,
		AuthTag:   authTag,
		CreatedAt: time.Now(),
		Metadata:  metadata,
	}

	s.mu.Lock()
	s.encrypted[encryptedData.ID] = encryptedData
	s.addAuditLog("encrypt", encryptedData.ID, "system", "", fmt.Sprintf("Encrypted with key %s", keyID), true)
	s.mu.Unlock()

	return encryptedData, nil
}

// Decrypt 解密数据
func (s *Service) Decrypt(encryptedID string) ([]byte, error) {
	s.mu.RLock()
	enc, ok := s.encrypted[encryptedID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("encrypted data not found: %s", encryptedID)
	}

	key, ok := s.keys[enc.KeyID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("key not found: %s", enc.KeyID)
	}
	s.mu.RUnlock()

	block, err := aes.NewCipher(s.deriveSymmetricKey(key))
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 合并密文和认证标签
	sealedData := append(enc.Data, enc.AuthTag...)

	plaintext, err := aesGCM.Open(nil, enc.IV, sealedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	s.mu.Lock()
	s.addAuditLog("decrypt", encryptedID, "system", "", fmt.Sprintf("Decrypted with key %s", enc.KeyID), true)
	s.mu.Unlock()

	return plaintext, nil
}

// Sign 签名
func (s *Service) Sign(keyID string, data []byte) (*Signature, error) {
	s.mu.RLock()
	key, ok := s.keys[keyID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	s.mu.RUnlock()

	if key.KeyType != KeyTypeSigning {
		return nil, fmt.Errorf("key is not for signing")
	}

	// 模拟签名生成
	signature := &Signature{
		ID:        fmt.Sprintf("sig_%d", time.Now().UnixNano()),
		KeyID:     keyID,
		Algorithm: key.Algorithm,
		Data:      data,
		Signature: s.generateSignature(key, data),
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.signatures[signature.ID] = signature
	s.addAuditLog("sign", signature.ID, "system", "", fmt.Sprintf("Signed with key %s", keyID), true)
	s.mu.Unlock()

	return signature, nil
}

// Verify 验证签名
func (s *Service) Verify(signatureID string) (bool, error) {
	s.mu.RLock()
	sig, ok := s.signatures[signatureID]
	if !ok {
		s.mu.RUnlock()
		return false, fmt.Errorf("signature not found: %s", signatureID)
	}

	key, ok := s.keys[sig.KeyID]
	if !ok {
		s.mu.RUnlock()
		return false, fmt.Errorf("key not found: %s", sig.KeyID)
	}
	s.mu.RUnlock()

	// 模拟签名验证
	verified := s.verifySignature(key, sig.Data, sig.Signature)

	s.mu.Lock()
	sig.Verified = verified
	s.addAuditLog("verify", signatureID, "system", "", fmt.Sprintf("Verification result: %v", verified), true)
	s.mu.Unlock()

	return verified, nil
}

// GetAuditLog 获取审计日志
func (s *Service) GetAuditLog(action string, limit int) []AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []AuditLog
	for i := len(s.auditLog) - 1; i >= 0 && len(result) < limit; i-- {
		if action == "" || s.auditLog[i].Action == action {
			result = append(result, s.auditLog[i])
		}
	}

	return result
}

// 辅助方法

func (s *Service) generateKyberPublicKey(security SecurityLevel) []byte {
	size := 800
	if security == SecurityLevel5 {
		size = 1568
	}
	return s.generateGenericKey(size)
}

func (s *Service) generateKyberPrivateKey(security SecurityLevel) []byte {
	size := 1632
	if security == SecurityLevel5 {
		size = 3168
	}
	return s.generateGenericKey(size)
}

func (s *Service) generateDilithiumPublicKey(security SecurityLevel) []byte {
	size := 1312
	if security == SecurityLevel5 {
		size = 2592
	}
	return s.generateGenericKey(size)
}

func (s *Service) generateDilithiumPrivateKey(security SecurityLevel) []byte {
	size := 2528
	if security == SecurityLevel5 {
		size = 4864
	}
	return s.generateGenericKey(size)
}

func (s *Service) generateFalconPublicKey(security SecurityLevel) []byte {
	size := 897
	if security == SecurityLevel5 {
		size = 1793
	}
	return s.generateGenericKey(size)
}

func (s *Service) generateFalconPrivateKey(security SecurityLevel) []byte {
	size := 1281
	if security == SecurityLevel5 {
		size = 2305
	}
	return s.generateGenericKey(size)
}

func (s *Service) generateGenericKey(size int) []byte {
	key := make([]byte, size)
	rand.Read(key)
	return key
}

func (s *Service) deriveSymmetricKey(key *KeyPair) []byte {
	hash := sha256.Sum256(key.PrivateKey)
	return hash[:]
}

func (s *Service) generateSignature(key *KeyPair, data []byte) []byte {
	hash := sha256.Sum256(append(key.PrivateKey, data...))
	return hash[:]
}

func (s *Service) verifySignature(key *KeyPair, data []byte, signature []byte) bool {
	expected := s.generateSignature(key, data)
	return base64.StdEncoding.EncodeToString(expected) == base64.StdEncoding.EncodeToString(signature)
}

func (s *Service) addAuditLog(action, resource, user, ip, details string, success bool) {
	s.auditLog = append(s.auditLog, AuditLog{
		ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
		Action:    action,
		Resource:  resource,
		User:      user,
		IP:        ip,
		Timestamp: time.Now(),
		Details:   details,
		Success:   success,
	})
}
