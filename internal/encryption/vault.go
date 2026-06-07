// Package encryption 提供加密卷（Vault）管理功能
// 刑部 Round 241 - Vault Password 加密卷
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
)

// ========== 常量 ==========

const (
	// SaltLength 盐值长度（字节）.
	SaltLength = 32
	// NonceLength AES-GCM nonce 长度（字节）.
	NonceLength = 12
	// KeyLength AES-256 密钥长度（字节）.
	KeyLength = 32
	// PBKDF2Iterations PBKDF2 迭代次数.
	PBKDF2Iterations = 100000
)

// Algorithm 加密算法标识.
type Algorithm string

const (
	// AlgorithmAES256GCM AES-256-GCM 认证加密.
	AlgorithmAES256GCM Algorithm = "AES-256-GCM"
)

// VaultState vault 状态.
type VaultState string

const (
	// VaultLocked 已锁定.
	VaultLocked VaultState = "locked"
	// VaultUnlocked 已解锁.
	VaultUnlocked VaultState = "unlocked"
)

// ========== 数据类型 ==========

// Vault 加密卷结构体.
type Vault struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	EncryptedKey string     `json:"encrypted_key"` // Base64 编码的加密数据
	Salt         string     `json:"salt"`          // Base64 编码的盐值
	Algorithm    Algorithm  `json:"algorithm"`
	State        VaultState `json:"state"`
	CreatedAt    time.Time  `json:"created_at"`
	LastAccessed time.Time  `json:"last_accessed"`
}

// DecryptedVault 解锁后的 vault（内存中临时持有密钥）.
type DecryptedVault struct {
	Vault      *Vault
	DecodedKey []byte // 解密后的密钥材料，仅在内存中存在
}

// VaultConfig vault 管理器配置.
type VaultConfig struct {
	Algorithm    Algorithm `json:"algorithm"`
	Iterations   int       `json:"iterations"`
	AutoLockMins int       `json:"auto_lock_mins"` // 自动锁定时间（分钟）
}

// VaultManager 加密卷管理器.
type VaultManager struct {
	vaults    map[string]*Vault
	decrypted map[string]*DecryptedVault // vaultID -> 解密数据
	config    VaultConfig
	mu        sync.RWMutex
}

// ========== 构造函数 ==========

// NewVaultManager 创建加密卷管理器.
func NewVaultManager() *VaultManager {
	return &VaultManager{
		vaults:    make(map[string]*Vault),
		decrypted: make(map[string]*DecryptedVault),
		config: VaultConfig{
			Algorithm:    AlgorithmAES256GCM,
			Iterations:   PBKDF2Iterations,
			AutoLockMins: 30,
		},
	}
}

// SetConfig 设置配置.
func (vm *VaultManager) SetConfig(config VaultConfig) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.config = config
}

// GetConfig 获取配置.
func (vm *VaultManager) GetConfig() VaultConfig {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.config
}

// ========== 核心操作 ==========

// CreateVault 创建新的加密卷.
func (vm *VaultManager) CreateVault(name, password string) (*Vault, error) {
	if name == "" {
		return nil, fmt.Errorf("vault 名称不能为空")
	}
	if password == "" {
		return nil, fmt.Errorf("密码不能为空")
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("密码长度不能少于 8 个字符")
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	// 检查名称是否已存在
	for _, v := range vm.vaults {
		if v.Name == name {
			return nil, fmt.Errorf("vault 名称 '%s' 已存在", name)
		}
	}

	// 生成随机密钥材料
	randomKey := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, randomKey); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}

	// 生成盐值
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("生成盐值失败: %w", err)
	}

	// 用 PBKDF2 从密码派生加密密钥
	derivedKey := pbkdf2.Key([]byte(password), salt, vm.config.Iterations, KeyLength, sha256.New)

	// 用 AES-256-GCM 加密随机密钥
	encryptedKey, err := encryptAESGCM(randomKey, derivedKey)
	if err != nil {
		return nil, fmt.Errorf("加密密钥失败: %w", err)
	}

	now := time.Now()
	vault := &Vault{
		ID:           uuid.New().String(),
		Name:         name,
		EncryptedKey: base64.StdEncoding.EncodeToString(encryptedKey),
		Salt:         base64.StdEncoding.EncodeToString(salt),
		Algorithm:    vm.config.Algorithm,
		State:        VaultLocked,
		CreatedAt:    now,
		LastAccessed: now,
	}

	vm.vaults[vault.ID] = vault
	return vault, nil
}

// UnlockVault 解锁加密卷.
func (vm *VaultManager) UnlockVault(vaultID, password string) (*DecryptedVault, error) {
	if vaultID == "" {
		return nil, fmt.Errorf("vault ID 不能为空")
	}
	if password == "" {
		return nil, fmt.Errorf("密码不能为空")
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	vault, exists := vm.vaults[vaultID]
	if !exists {
		return nil, fmt.Errorf("vault 不存在: %s", vaultID)
	}

	if vault.State == VaultUnlocked {
		// 已解锁，直接返回
		if dv, ok := vm.decrypted[vaultID]; ok {
			vault.LastAccessed = time.Now()
			return dv, nil
		}
	}

	// 解码盐值和加密数据
	salt, err := base64.StdEncoding.DecodeString(vault.Salt)
	if err != nil {
		return nil, fmt.Errorf("解码盐值失败: %w", err)
	}

	encryptedKey, err := base64.StdEncoding.DecodeString(vault.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("解码加密密钥失败: %w", err)
	}

	// 派生解密密钥
	derivedKey := pbkdf2.Key([]byte(password), salt, vm.config.Iterations, KeyLength, sha256.New)

	// 解密
	decryptedKey, err := decryptAESGCM(encryptedKey, derivedKey)
	if err != nil {
		return nil, fmt.Errorf("密码错误或解密失败")
	}

	vault.State = VaultUnlocked
	vault.LastAccessed = time.Now()

	dv := &DecryptedVault{
		Vault:      vault,
		DecodedKey: decryptedKey,
	}
	vm.decrypted[vaultID] = dv

	return dv, nil
}

// LockVault 锁定加密卷.
func (vm *VaultManager) LockVault(vaultID string) error {
	if vaultID == "" {
		return fmt.Errorf("vault ID 不能为空")
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	vault, exists := vm.vaults[vaultID]
	if !exists {
		return fmt.Errorf("vault 不存在: %s", vaultID)
	}

	if vault.State == VaultLocked {
		return nil // 已经锁定
	}

	// 清除内存中的解密数据
	if dv, ok := vm.decrypted[vaultID]; ok {
		zeroBytes(dv.DecodedKey)
		delete(vm.decrypted, vaultID)
	}

	vault.State = VaultLocked
	return nil
}

// DeleteVault 删除加密卷.
func (vm *VaultManager) DeleteVault(vaultID string) error {
	if vaultID == "" {
		return fmt.Errorf("vault ID 不能为空")
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	vault, exists := vm.vaults[vaultID]
	if !exists {
		return fmt.Errorf("vault 不存在: %s", vaultID)
	}

	// 清除内存中的解密数据
	if dv, ok := vm.decrypted[vaultID]; ok {
		zeroBytes(dv.DecodedKey)
		delete(vm.decrypted, vaultID)
	}

	// 清除 vault 数据
	vault.EncryptedKey = ""
	vault.Salt = ""

	delete(vm.vaults, vaultID)
	return nil
}

// ListVaults 列出所有加密卷.
func (vm *VaultManager) ListVaults() []*Vault {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	result := make([]*Vault, 0, len(vm.vaults))
	for _, v := range vm.vaults {
		// 返回副本，隐藏敏感字段
		copy := *v
		copy.EncryptedKey = "***"
		copy.Salt = "***"
		result = append(result, &copy)
	}
	return result
}

// GetVault 获取单个 vault 信息.
func (vm *VaultManager) GetVault(vaultID string) (*Vault, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	vault, exists := vm.vaults[vaultID]
	if !exists {
		return nil, fmt.Errorf("vault 不存在: %s", vaultID)
	}

	copy := *vault
	copy.EncryptedKey = "***"
	copy.Salt = "***"
	return &copy, nil
}

// IsUnlocked 检查 vault 是否已解锁.
func (vm *VaultManager) IsUnlocked(vaultID string) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	vault, exists := vm.vaults[vaultID]
	if !exists {
		return false
	}
	return vault.State == VaultUnlocked
}

// AutoLockExpired 自动锁定超时的 vault.
func (vm *VaultManager) AutoLockExpired() int {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.config.AutoLockMins <= 0 {
		return 0
	}

	deadline := time.Now().Add(-time.Duration(vm.config.AutoLockMins) * time.Minute)
	locked := 0

	for id, vault := range vm.vaults {
		if vault.State == VaultUnlocked && vault.LastAccessed.Before(deadline) {
			if dv, ok := vm.decrypted[id]; ok {
				zeroBytes(dv.DecodedKey)
				delete(vm.decrypted, id)
			}
			vault.State = VaultLocked
			locked++
		}
	}

	return locked
}

// ========== 加密原语 ==========

// encryptAESGCM 使用 AES-256-GCM 加密.
func encryptAESGCM(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 密文格式: nonce + ciphertext + tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptAESGCM 使用 AES-256-GCM 解密.
func decryptAESGCM(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败（密码错误或数据损坏）: %w", err)
	}

	return plaintext, nil
}

// zeroBytes 安全清零字节切片.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
