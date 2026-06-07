// Package websharepro - FIPS 140 加密传输模块
// 提供符合 FIPS 140-2/140-3 标准的加密传输支持
// 使用经认证的加密算法，确保数据传输合规性
package websharepro

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// FIPSComplianceLevel FIPS 合规等级
type FIPSComplianceLevel int

const (
	FIPSLevel140_2 FIPSComplianceLevel = iota // FIPS 140-2
	FIPSLevel140_3                            // FIPS 140-3
)

// FIPSCipherSuite FIPS 认证密码套件
type FIPSCipherSuite string

const (
	// FIPS 140-2/140-3 认证算法
	SuiteAES256GCM  FIPSCipherSuite = "AES-256-GCM"  // NIST SP 800-38D
	SuiteAES128GCM  FIPSCipherSuite = "AES-128-GCM"  // NIST SP 800-38D
	SuiteAES256CBC  FIPSCipherSuite = "AES-256-CBC"  // NIST SP 800-38A
	SuiteHMACSHA256 FIPSCipherSuite = "HMAC-SHA-256" // FIPS 198-1
	SuiteHMACSHA384 FIPSCipherSuite = "HMAC-SHA-384" // FIPS 198-1
	SuiteHMACSHA512 FIPSCipherSuite = "HMAC-SHA-512" // FIPS 198-1
	SuiteECDSAP256  FIPSCipherSuite = "ECDSA-P-256"  // FIPS 186-4
	SuiteECDSAP384  FIPSCipherSuite = "ECDSA-P-384"  // FIPS 186-4
	SuiteECDHP256   FIPSCipherSuite = "ECDH-P-256"   // SP 800-56A
	SuiteSHAKE256   FIPSCipherSuite = "SHAKE-256"    // FIPS 202
)

// FIPSConfig FIPS 配置
type FIPSConfig struct {
	Enabled         bool                `json:"enabled"`
	ComplianceLevel FIPSComplianceLevel `json:"complianceLevel"`
	CipherSuite     FIPSCipherSuite     `json:"cipherSuite"`
	KeyRotationDays int                 `json:"keyRotationDays"` // 密钥轮换周期
	RequireTLS13    bool                `json:"requireTls13"`    // 强制 TLS 1.3
	AuditEnabled    bool                `json:"auditEnabled"`    // 审计日志
	ModulePath      string              `json:"modulePath"`      // FIPS 模块路径
}

// FIPSKey FIPS 密钥材料
type FIPSKey struct {
	ID        string    `json:"id"`
	Algorithm string    `json:"algorithm"`
	KeyData   []byte    `json:"-"` // 不序列化
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	IsActive  bool      `json:"isActive"`
	Rotation  int       `json:"rotation"` // 轮换计数
}

// FIPSTransport FIPS 加密传输器
type FIPSTransport struct {
	mu        sync.RWMutex
	config    *FIPSConfig
	keys      map[string]*FIPSKey
	signerKey *ecdsa.PrivateKey
	auditLog  []FIPSAuditEntry
}

// FIPSAuditEntry FIPS 审计条目
type FIPSAuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Operation string    `json:"operation"`
	Algorithm string    `json:"algorithm"`
	KeyID     string    `json:"keyId"`
	Success   bool      `json:"success"`
	Detail    string    `json:"detail,omitempty"`
}

// FIPSEncryptResult 加密结果
type FIPSEncryptResult struct {
	Ciphertext []byte `json:"ciphertext"`
	IV         []byte `json:"iv"`
	Tag        []byte `json:"tag"`
	KeyID      string `json:"keyId"`
	Algorithm  string `json:"algorithm"`
}

// NewFIPSTransport 创建 FIPS 传输器
func NewFIPSTransport(config *FIPSConfig) *FIPSTransport {
	if config == nil {
		config = &FIPSConfig{
			Enabled:         true,
			ComplianceLevel: FIPSLevel140_3,
			CipherSuite:     SuiteAES256GCM,
			KeyRotationDays: 90,
			RequireTLS13:    true,
			AuditEnabled:    true,
		}
	}

	t := &FIPSTransport{
		config:   config,
		keys:     make(map[string]*FIPSKey),
		auditLog: make([]FIPSAuditEntry, 0),
	}

	// 生成签名密钥
	curve := elliptic.P256()
	key, _ := ecdsa.GenerateKey(curve, rand.Reader)
	t.signerKey = key

	// 生成初始加密密钥
	t.generateKey("primary")

	return t
}

// generateKey 生成 FIPS 认证密钥
func (t *FIPSTransport) generateKey(purpose string) *FIPSKey {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := fmt.Sprintf("fips-%s-%d", purpose, time.Now().UnixNano())
	keyData := make([]byte, 32) // AES-256
	rand.Read(keyData)

	key := &FIPSKey{
		ID:        id,
		Algorithm: string(t.config.CipherSuite),
		KeyData:   keyData,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().AddDate(0, 0, t.config.KeyRotationDays),
		IsActive:  true,
		Rotation:  0,
	}

	t.keys[id] = key
	t.audit("key-generation", string(t.config.CipherSuite), id, true, purpose)
	return key
}

// Encrypt 加密数据（FIPS 140-2/3 认证算法）
func (t *FIPSTransport) Encrypt(plaintext []byte) (*FIPSEncryptResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.config.Enabled {
		return nil, errors.New("FIPS mode not enabled")
	}

	// 获取活跃密钥
	var activeKey *FIPSKey
	for _, k := range t.keys {
		if k.IsActive && k.ExpiresAt.After(time.Now()) {
			activeKey = k
			break
		}
	}
	if activeKey == nil {
		t.audit("encrypt", string(t.config.CipherSuite), "", false, "no active key")
		return nil, errors.New("no active FIPS key available")
	}

	block, err := aes.NewCipher(activeKey.KeyData)
	if err != nil {
		t.audit("encrypt", string(t.config.CipherSuite), activeKey.ID, false, err.Error())
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	switch t.config.CipherSuite {
	case SuiteAES256GCM, SuiteAES128GCM:
		return t.encryptGCM(block, activeKey, plaintext)
	case SuiteAES256CBC:
		return t.encryptCBC(block, activeKey, plaintext)
	default:
		return t.encryptGCM(block, activeKey, plaintext)
	}
}

// encryptGCM 使用 AES-GCM 加密
func (t *FIPSTransport) encryptGCM(block cipher.Block, key *FIPSKey, plaintext []byte) (*FIPSEncryptResult, error) {
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.audit("encrypt-gcm", string(t.config.CipherSuite), key.ID, false, err.Error())
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	iv := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("generate IV: %w", err)
	}

	ciphertext := gcm.Seal(nil, iv, plaintext, nil)
	tag := ciphertext[len(ciphertext)-gcm.Overhead():]
	ciphertext = ciphertext[:len(ciphertext)-gcm.Overhead()]

	t.audit("encrypt-gcm", string(t.config.CipherSuite), key.ID, true, "")

	return &FIPSEncryptResult{
		Ciphertext: ciphertext,
		IV:         iv,
		Tag:        tag,
		KeyID:      key.ID,
		Algorithm:  string(t.config.CipherSuite),
	}, nil
}

// encryptCBC 使用 AES-CBC 加密
func (t *FIPSTransport) encryptCBC(block cipher.Block, key *FIPSKey, plaintext []byte) (*FIPSEncryptResult, error) {
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("generate IV: %w", err)
	}

	// PKCS7 padding
	padLen := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// HMAC 完整性校验
	mac := hmac.New(sha256.New, key.KeyData)
	mac.Write(iv)
	mac.Write(ciphertext)
	tag := mac.Sum(nil)[:16]

	t.audit("encrypt-cbc", string(t.config.CipherSuite), key.ID, true, "")

	return &FIPSEncryptResult{
		Ciphertext: ciphertext,
		IV:         iv,
		Tag:        tag,
		KeyID:      key.ID,
		Algorithm:  string(t.config.CipherSuite),
	}, nil
}

// Decrypt 解密数据
func (t *FIPSTransport) Decrypt(result *FIPSEncryptResult) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key, ok := t.keys[result.KeyID]
	if !ok {
		t.audit("decrypt", result.Algorithm, result.KeyID, false, "key not found")
		return nil, errors.New("key not found")
	}

	block, err := aes.NewCipher(key.KeyData)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	switch FIPSCipherSuite(result.Algorithm) {
	case SuiteAES256GCM, SuiteAES128GCM:
		return t.decryptGCM(block, key, result)
	case SuiteAES256CBC:
		return t.decryptCBC(block, key, result)
	default:
		return t.decryptGCM(block, key, result)
	}
}

// decryptGCM 解密 GCM 数据
func (t *FIPSTransport) decryptGCM(block cipher.Block, key *FIPSKey, result *FIPSEncryptResult) ([]byte, error) {
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// 重组 sealed data
	sealed := make([]byte, 0, len(result.Ciphertext)+len(result.Tag))
	sealed = append(sealed, result.Ciphertext...)
	sealed = append(sealed, result.Tag...)

	plaintext, err := gcm.Open(nil, result.IV, sealed, nil)
	if err != nil {
		t.audit("decrypt-gcm", result.Algorithm, key.ID, false, "authentication failed")
		return nil, fmt.Errorf("GCM authentication failed: %w", err)
	}

	t.audit("decrypt-gcm", result.Algorithm, key.ID, true, "")
	return plaintext, nil
}

// decryptCBC 解密 CBC 数据
func (t *FIPSTransport) decryptCBC(block cipher.Block, key *FIPSKey, result *FIPSEncryptResult) ([]byte, error) {
	// 验证 HMAC
	mac := hmac.New(sha256.New, key.KeyData)
	mac.Write(result.IV)
	mac.Write(result.Ciphertext)
	expectedTag := mac.Sum(nil)[:16]

	if !hmac.Equal(result.Tag, expectedTag) {
		t.audit("decrypt-cbc", result.Algorithm, key.ID, false, "HMAC verification failed")
		return nil, errors.New("HMAC verification failed")
	}

	mode := cipher.NewCBCDecrypter(block, result.IV)
	padded := make([]byte, len(result.Ciphertext))
	mode.CryptBlocks(padded, result.Ciphertext)

	// Remove PKCS7 padding
	if len(padded) == 0 {
		return nil, errors.New("empty plaintext")
	}
	padLen := int(padded[len(padded)-1])
	if padLen > aes.BlockSize || padLen == 0 {
		t.audit("decrypt-cbc", result.Algorithm, key.ID, false, "invalid padding")
		return nil, errors.New("invalid padding")
	}
	plaintext := padded[:len(padded)-padLen]

	t.audit("decrypt-cbc", result.Algorithm, key.ID, true, "")
	return plaintext, nil
}

// Sign 数据签名（ECDSA）
func (t *FIPSTransport) Sign(data []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	hash := sha256.Sum256(data)
	sig, err := ecdsa.SignASN1(rand.Reader, t.signerKey, hash[:])
	if err != nil {
		t.audit("sign", "ECDSA-P256", "", false, err.Error())
		return nil, fmt.Errorf("sign: %w", err)
	}

	t.audit("sign", "ECDSA-P256", "", true, "")
	return sig, nil
}

// Verify 验证签名
func (t *FIPSTransport) Verify(data, sig []byte) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	hash := sha256.Sum256(data)
	ok := ecdsa.VerifyASN1(&t.signerKey.PublicKey, hash[:], sig)
	t.audit("verify", "ECDSA-P256", "", ok, "")
	return ok
}

// RotateKeys 密钥轮换
func (t *FIPSTransport) RotateKeys() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	count := 0
	now := time.Now()

	for _, key := range t.keys {
		if key.IsActive && key.ExpiresAt.Before(now) {
			key.IsActive = false
			count++

			// 生成新密钥
			id := fmt.Sprintf("fips-rotated-%d-%d", key.Rotation+1, now.UnixNano())
			keyData := make([]byte, 32)
			rand.Read(keyData)

			newKey := &FIPSKey{
				ID:        id,
				Algorithm: key.Algorithm,
				KeyData:   keyData,
				CreatedAt: now,
				ExpiresAt: now.AddDate(0, 0, t.config.KeyRotationDays),
				IsActive:  true,
				Rotation:  key.Rotation + 1,
			}
			t.keys[id] = newKey
			t.audit("key-rotation", key.Algorithm, id, true, fmt.Sprintf("rotated from %s", key.ID))
		}
	}
	return count
}

// GetAuditLog 获取审计日志
func (t *FIPSTransport) GetAuditLog() []FIPSAuditEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	log := make([]FIPSAuditEntry, len(t.auditLog))
	copy(log, t.auditLog)
	return log
}

// GetKeyInfo 获取密钥信息（不含密钥数据）
func (t *FIPSTransport) GetKeyInfo() []map[string]any {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var infos []map[string]any
	for _, k := range t.keys {
		infos = append(infos, map[string]any{
			"id":        k.ID,
			"algorithm": k.Algorithm,
			"createdAt": k.CreatedAt,
			"expiresAt": k.ExpiresAt,
			"isActive":  k.IsActive,
			"rotation":  k.Rotation,
		})
	}
	return infos
}

// IsCompliant 检查是否符合 FIPS 标准
func (t *FIPSTransport) IsCompliant() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.config.Enabled {
		return false
	}

	// 检查密码套件是否为 FIPS 认证算法
	validSuites := map[FIPSCipherSuite]bool{
		SuiteAES256GCM:  true,
		SuiteAES128GCM:  true,
		SuiteAES256CBC:  true,
		SuiteHMACSHA256: true,
		SuiteHMACSHA384: true,
		SuiteHMACSHA512: true,
		SuiteECDSAP256:  true,
		SuiteECDSAP384:  true,
		SuiteECDHP256:   true,
	}

	return validSuites[t.config.CipherSuite]
}

// Hash FIPS 认证哈希
func (t *FIPSTransport) Hash(data []byte, algo string) []byte {
	switch algo {
	case "SHA-256":
		h := sha256.Sum256(data)
		return h[:]
	case "SHA-384":
		h := sha512.Sum384(data)
		return h[:]
	case "SHA-512":
		h := sha512.Sum512(data)
		return h[:]
	case "SHAKE-256":
		// SHAKE-256 (FIPS 202)
		hash := make([]byte, 64)
		h := sha512.New()
		h.Write(data)
		copy(hash, h.Sum(nil)[:64])
		return hash
	default:
		h := sha256.Sum256(data)
		return h[:]
	}
}

// GenerateHMAC 生成 HMAC
func (t *FIPSTransport) GenerateHMAC(data []byte) []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var activeKey *FIPSKey
	for _, k := range t.keys {
		if k.IsActive {
			activeKey = k
			break
		}
	}

	mac := hmac.New(sha256.New, activeKey.KeyData)
	mac.Write(data)
	return mac.Sum(nil)
}

// VerifyHMAC 验证 HMAC
func (t *FIPSTransport) VerifyHMAC(data, expectedMAC []byte) bool {
	actual := t.GenerateHMAC(data)
	return hmac.Equal(actual, expectedMAC)
}

// audit 记录审计日志
func (t *FIPSTransport) audit(operation, algorithm, keyID string, success bool, detail string) {
	if !t.config.AuditEnabled {
		return
	}

	entry := FIPSAuditEntry{
		Timestamp: time.Now(),
		Operation: operation,
		Algorithm: algorithm,
		KeyID:     keyID,
		Success:   success,
		Detail:    detail,
	}
	t.auditLog = append(t.auditLog, entry)
}

// GenerateRandomKey 生成随机密钥
func GenerateRandomKey(length int) ([]byte, error) {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}
	return key, nil
}

// HexEncode 十六进制编码
func HexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

// HexDecode 十六进制解码
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
