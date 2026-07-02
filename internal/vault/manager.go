package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// Manager 保险库管理器，负责保险库的生命周期管理和加密操作。
type Manager struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config VaultConfig
	// vaults 存储所有保险库实例，key 为保险库 ID
	vaults map[string]*Vault
	// keys 存储已解锁保险库的派生密钥，key 为保险库 ID
	keys map[string][]byte
	// failedAttempts 记录每个保险库的连续解锁失败次数
	failedAttempts map[string]int
	// encryptionOps 累计加密操作计数器
	encryptionOps int64
}

// NewManager 创建一个新的保险库管理器实例。
// logger: zap 日志实例
// config: 保险库全局配置
func NewManager(logger *zap.Logger, config VaultConfig) *Manager {
	// 设置默认值
	if config.DefaultAlgorithm == "" {
		config.DefaultAlgorithm = AlgorithmAES256GCM
	}
	if config.AutoLockMinutes <= 0 {
		config.AutoLockMinutes = 30
	}
	if config.MaxFailedAttempts <= 0 {
		config.MaxFailedAttempts = 5
	}
	if config.KeyDerivation == "" {
		config.KeyDerivation = KeyDerivationArgon2id
	}

	return &Manager{
		logger:         logger,
		config:         config,
		vaults:         make(map[string]*Vault),
		keys:           make(map[string][]byte),
		failedAttempts: make(map[string]int),
	}
}

// CreateVault 创建一个新的加密保险库。
// name: 保险库名称（必须唯一）
// description: 保险库描述
// mountPath: 挂载路径
// algorithm: 加密算法，为空时使用默认算法
// passphrase: 用于生成验证令牌的密码，为空时不生成验证令牌
func (m *Manager) CreateVault(name, description, mountPath string, algorithm string, passphrase string) (*Vault, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 参数校验
	if strings.TrimSpace(name) == "" {
		return nil, NewVaultError("INVALID_NAME", "保险库名称不能为空", nil)
	}
	if strings.TrimSpace(mountPath) == "" {
		return nil, ErrInvalidPath
	}

	// 校验算法
	if algorithm == "" {
		algorithm = m.config.DefaultAlgorithm
	}
	if algorithm != AlgorithmAES256GCM && algorithm != AlgorithmChaCha20Poly1305 {
		return nil, ErrInvalidAlgorithm
	}

	// 检查名称是否重复
	for _, v := range m.vaults {
		if v.Name == name {
			return nil, ErrVaultAlreadyExists
		}
	}

	// 生成唯一 ID 和密钥 ID
	id := generateID()
	keyID := generateID()

	now := time.Now()
	vault := &Vault{
		ID:           id,
		Name:         name,
		Description:  description,
		MountPath:    mountPath,
		KeyID:        keyID,
		Algorithm:    algorithm,
		Status:       StatusLocked,
		CreatedAt:    now,
		LastAccessAt: now,
		FileCount:    0,
		TotalSize:    0,
	}

	m.vaults[id] = vault
	m.failedAttempts[id] = 0

	// 如果提供了 passphrase，生成验证令牌
	if passphrase != "" {
		derivedKey, err := m.deriveKey(passphrase, id)
		if err != nil {
			return nil, NewVaultError("KEY_DERIVATION_FAILED", "密钥派生失败", err)
		}
		encrypted, err := m.encryptData(derivedKey, algorithm, []byte("vault-verification-token-v1"))
		if err != nil {
			return nil, NewVaultError("ENCRYPTION_FAILED", "生成验证令牌失败", err)
		}
		vault.VerificationToken = hex.EncodeToString(encrypted)
	}

	m.logger.Info("保险库创建成功",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("algorithm", algorithm),
	)

	return vault, nil
}

// UnlockVault 使用密码解锁保险库。
// id: 保险库 ID
// passphrase: 解锁密码
func (m *Manager) UnlockVault(id string, passphrase string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vault, ok := m.vaults[id]
	if !ok {
		return ErrVaultNotFound
	}

	// 检查失败次数
	if m.failedAttempts[id] >= m.config.MaxFailedAttempts {
		m.logger.Warn("保险库解锁失败次数超限",
			zap.String("id", id),
			zap.Int("attempts", m.failedAttempts[id]),
		)
		return ErrMaxAttemptsExceeded
	}

	// 已解锁状态检查
	if vault.Status == StatusUnlocked {
		return ErrVaultAlreadyUnlocked
	}

	// 派生密钥
	derivedKey, err := m.deriveKey(passphrase, id)
	if err != nil {
		m.failedAttempts[id]++
		return NewVaultError("KEY_DERIVATION_FAILED", "密钥派生失败", err)
	}

	// 验证密钥：解密验证令牌验证密码是否正确
	valid := m.verifyKey(derivedKey, vault)
	if !valid {
		m.failedAttempts[id]++
		m.logger.Warn("保险库解锁密码错误",
			zap.String("id", id),
			zap.Int("failed_attempts", m.failedAttempts[id]),
		)
		return ErrInvalidPassphrase
	}

	// 解锁成功
	vault.Status = StatusUnlocked
	vault.LastAccessAt = time.Now()
	m.keys[id] = derivedKey
	m.failedAttempts[id] = 0

	m.logger.Info("保险库已解锁", zap.String("id", id))
	return nil
}

// LockVault 锁定保险库，清除内存中的密钥。
// id: 保险库 ID
func (m *Manager) LockVault(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vault, ok := m.vaults[id]
	if !ok {
		return ErrVaultNotFound
	}

	if vault.Status == StatusLocked {
		return ErrVaultLocked
	}

	// 安全清除密钥
	if key, exists := m.keys[id]; exists {
		for i := range key {
			key[i] = 0
		}
		delete(m.keys, id)
	}

	vault.Status = StatusLocked
	m.logger.Info("保险库已锁定", zap.String("id", id))
	return nil
}

// DeleteVault 删除保险库。仅当保险库处于锁定状态时才允许删除。
// id: 保险库 ID
func (m *Manager) DeleteVault(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vault, ok := m.vaults[id]
	if !ok {
		return ErrVaultNotFound
	}

	// 解锁状态不允许删除
	if vault.Status == StatusUnlocked {
		return NewVaultError("VAULT_UNLOCKED", "请先锁定保险库再删除", nil)
	}

	// 安全清除密钥残留
	if key, exists := m.keys[id]; exists {
		for i := range key {
			key[i] = 0
		}
		delete(m.keys, id)
	}

	delete(m.vaults, id)
	delete(m.failedAttempts, id)

	m.logger.Info("保险库已删除", zap.String("id", id))
	return nil
}

// ListVaults 返回所有保险库的列表。
func (m *Manager) ListVaults() []Vault {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vaults := make([]Vault, 0, len(m.vaults))
	for _, v := range m.vaults {
		vaults = append(vaults, *v)
	}
	return vaults
}

// GetVault 根据 ID 获取保险库详情。
// id: 保险库 ID
func (m *Manager) GetVault(id string) (*Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vault, ok := m.vaults[id]
	if !ok {
		return nil, ErrVaultNotFound
	}
	return vault, nil
}

// GetStats 返回所有保险库的统计信息。
func (m *Manager) GetStats() VaultStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := VaultStats{
		TotalVaults:   len(m.vaults),
		EncryptionOps: m.encryptionOps,
	}

	for _, v := range m.vaults {
		if v.Status == StatusUnlocked {
			stats.UnlockedVaults++
		}
		stats.TotalFiles += v.FileCount
		stats.TotalSize += v.TotalSize
	}

	return stats
}

// deriveKey 使用配置的密钥派生算法从密码派生加密密钥。
// passphrase: 用户密码
// salt: 盐值（使用保险库 ID）
func (m *Manager) deriveKey(passphrase, salt string) ([]byte, error) {
	saltBytes := sha256.Sum256([]byte(salt))

	switch m.config.KeyDerivation {
	case KeyDerivationArgon2id:
		// Argon2id 参数：内存 64MB，迭代 3 次，并行度 4，密钥长度 32 字节
		key := argon2.IDKey(
			[]byte(passphrase),
			saltBytes[:],
			3,
			64*1024,
			4,
			32,
		)
		return key, nil
	case KeyDerivationPBKDF2:
		// PBKDF2 使用 SHA-256，迭代 600000 次
		key := pbkdf2Key([]byte(passphrase), saltBytes[:], 600000, 32)
		return key, nil
	default:
		return nil, NewVaultError("UNSUPPORTED_KDF", "不支持的密钥派生算法", nil)
	}
}

// verifyKey 验证派生密钥是否有效。
// 通过解密验证令牌来验证密码是否正确。
func (m *Manager) verifyKey(key []byte, vault *Vault) bool {
	const verificationPlaintext = "vault-verification-token-v1"

	// 如果没有验证令牌（首次解锁），注册令牌
	if vault.VerificationToken == "" {
		encrypted, err := m.encryptData(key, vault.Algorithm, []byte(verificationPlaintext))
		if err != nil {
			return false
		}
		vault.VerificationToken = hex.EncodeToString(encrypted)
		return true
	}

	// 解密验证令牌
	tokenBytes, err := hex.DecodeString(vault.VerificationToken)
	if err != nil {
		return false
	}
	decrypted, err := m.decryptData(key, vault.Algorithm, tokenBytes)
	if err != nil {
		return false
	}
	return string(decrypted) == verificationPlaintext
}

// encryptData 使用指定算法加密数据。
func (m *Manager) encryptData(key []byte, algorithm string, plaintext []byte) ([]byte, error) {
	m.encryptionOps++

	switch algorithm {
	case AlgorithmAES256GCM:
		return encryptAES256GCM(key, plaintext)
	case AlgorithmChaCha20Poly1305:
		return encryptChaCha20Poly1305(key, plaintext)
	default:
		return nil, ErrInvalidAlgorithm
	}
}

// decryptData 使用指定算法解密数据。
func (m *Manager) decryptData(key []byte, algorithm string, ciphertext []byte) ([]byte, error) {
	switch algorithm {
	case AlgorithmAES256GCM:
		return decryptAES256GCM(key, ciphertext)
	case AlgorithmChaCha20Poly1305:
		return decryptChaCha20Poly1305(key, ciphertext)
	default:
		return nil, ErrInvalidAlgorithm
	}
}

// encryptAES256GCM 使用 AES-256-GCM 加密数据。
// 返回格式：nonce (12 bytes) + ciphertext + tag (16 bytes).
func encryptAES256GCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES 密码块失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptAES256GCM 使用 AES-256-GCM 解密数据。
func decryptAES256GCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES 密码块失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

// encryptChaCha20Poly1305 使用 ChaCha20-Poly1305 加密数据。
func encryptChaCha20Poly1305(key, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, fmt.Errorf("创建 ChaCha20-Poly1305 失败: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptChaCha20Poly1305 使用 ChaCha20-Poly1305 解密数据。
func decryptChaCha20Poly1305(key, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, fmt.Errorf("创建 ChaCha20-Poly1305 失败: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

// generateID 生成一个随机的 16 字节十六进制字符串作为唯一标识。
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// pbkdf2Key 实现 PBKDF2-SHA256 密钥派生。
// 未使用 golang.org/x/crypto/pbkdf2 以减少外部依赖。
func pbkdf2Key(password, salt []byte, iter, keyLen int) []byte {
	hLen := sha256.Size // 32
	numBlocks := (keyLen + hLen - 1) / hLen
	key := make([]byte, 0, numBlocks*hLen)

	for block := 1; block <= numBlocks; block++ {
		// U1 = HMAC(password, salt || INT(block))
		u := hmacSHA256(password, append(salt, byte(block>>24), byte(block>>16), byte(block>>8), byte(block)))
		result := make([]byte, len(u))
		copy(result, u)

		for i := 1; i < iter; i++ {
			u = hmacSHA256(password, u)
			for j := range result {
				result[j] ^= u[j]
			}
		}
		key = append(key, result...)
	}

	return key[:keyLen]
}

// hmacSHA256 计算 HMAC-SHA256。
func hmacSHA256(key, data []byte) []byte {
	// 简单实现 HMAC-SHA256
	blockSize := 64
	if len(key) > blockSize {
		h := sha256.Sum256(key)
		key = h[:]
	}

	// 填充 key 到 blockSize
	paddedKey := make([]byte, blockSize)
	copy(paddedKey, key)

	// 内层和外层填充
	ipad := make([]byte, blockSize)
	opad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		ipad[i] = paddedKey[i] ^ 0x36
		opad[i] = paddedKey[i] ^ 0x5c
	}

	// HMAC = SHA256(opad || SHA256(ipad || data))
	inner := sha256.Sum256(append(ipad, data...))
	outer := sha256.Sum256(append(opad, inner[:]...))
	return outer[:]
}
