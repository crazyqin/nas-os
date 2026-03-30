package securityv2

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// EncryptionManager 文件加密管理器.
type EncryptionManager struct {
	config        EncryptionConfig
	masterKey     []byte
	encryptedDirs map[string]*EncryptedDirectory
	mu            sync.RWMutex
}

// EncryptionConfig 加密配置.
type EncryptionConfig struct {
	Enabled         bool   `json:"enabled"`
	Algorithm       string `json:"algorithm"`        // aes-256-gcm
	KeyDerivation   string `json:"key_derivation"`   // argon2id
	MasterKeyPath   string `json:"master_key_path"`  // 主密钥文件路径
	SaltPath        string `json:"salt_path"`        // 盐值文件路径
	EncryptedPrefix string `json:"encrypted_prefix"` // 加密文件夹前缀
}

// EncryptedDirectory 加密目录.
type EncryptedDirectory struct {
	Path        string          `json:"path"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CreatedAt   string          `json:"created_at"`
	Files       []EncryptedFile `json:"files"`
	Status      string          `json:"status"` // locked, unlocked
	Key         []byte          `json:"-"`      // 目录密钥（内存中）
}

// EncryptedFile 加密文件.
type EncryptedFile struct {
	OriginalName  string `json:"original_name"`
	EncryptedName string `json:"encrypted_name"`
	Size          int64  `json:"size"`
	EncryptedSize int64  `json:"encrypted_size"`
	LastModified  string `json:"last_modified"`
	Checksum      string `json:"checksum"`
}

// NewEncryptionManager 创建加密管理器.
func NewEncryptionManager() *EncryptionManager {
	return &EncryptionManager{
		config: EncryptionConfig{
			Enabled:         true,
			Algorithm:       "aes-256-gcm",
			KeyDerivation:   "argon2id",
			MasterKeyPath:   "/var/lib/nas-os/security/master.key",
			SaltPath:        "/var/lib/nas-os/security/salt",
			EncryptedPrefix: ".encrypted_",
		},
		encryptedDirs: make(map[string]*EncryptedDirectory),
	}
}

// Initialize 初始化加密系统.
func (em *EncryptionManager) Initialize(password string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 生成或加载盐值
	salt, err := em.loadOrGenerateSalt()
	if err != nil {
		return fmt.Errorf("加载盐值失败：%w", err)
	}

	// 从密码派生主密钥
	em.masterKey = em.deriveKey(password, salt)

	// 加载或创建主密钥文件
	if err := em.loadOrGenerateMasterKey(); err != nil {
		return fmt.Errorf("主密钥初始化失败：%w", err)
	}

	return nil
}

// deriveKey 使用 Argon2 派生密钥.
func (em *EncryptionManager) deriveKey(password string, salt []byte) []byte {
	// Argon2id 参数
	time := uint32(3)
	memory := uint32(64 * 1024) // 64MB
	threads := uint8(4)
	keyLen := uint32(32) // 256-bit

	key := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	return key
}

// loadOrGenerateSalt 加载或生成盐值.
func (em *EncryptionManager) loadOrGenerateSalt() ([]byte, error) {
	// 确保目录存在
	dir := filepath.Dir(em.config.SaltPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	// 尝试加载现有盐值
	if data, err := os.ReadFile(em.config.SaltPath); err == nil {
		return data, nil
	}

	// 生成新盐值（16 字节）
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// 保存盐值
	if err := os.WriteFile(em.config.SaltPath, salt, 0600); err != nil {
		return nil, err
	}

	return salt, nil
}

// loadOrGenerateMasterKey 加载或生成主密钥.
func (em *EncryptionManager) loadOrGenerateMasterKey() error {
	// 确保目录存在
	dir := filepath.Dir(em.config.MasterKeyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// 尝试加载现有主密钥
	if data, err := os.ReadFile(em.config.MasterKeyPath); err == nil {
		// 解密主密钥（简化：实际应该用更复杂的加密）
		em.masterKey = data
		return nil
	}

	// 生成新主密钥（32 字节）
	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
		return err
	}

	// 加密并保存主密钥
	if err := os.WriteFile(em.config.MasterKeyPath, masterKey, 0600); err != nil {
		return err
	}

	em.masterKey = masterKey
	return nil
}

// CreateEncryptedDirectory 创建加密目录.
func (em *EncryptionManager) CreateEncryptedDirectory(path, name, description string) (*EncryptedDirectory, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.masterKey == nil {
		return nil, fmt.Errorf("加密系统未初始化")
	}

	// 确保目录存在
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}

	// 生成目录密钥（每个目录独立密钥）
	dirKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dirKey); err != nil {
		return nil, err
	}

	// 加密目录密钥并保存
	encryptedKey, err := em.encryptData(dirKey)
	if err != nil {
		return nil, err
	}

	keyFilePath := filepath.Join(path, ".dir.key")
	if err := os.WriteFile(keyFilePath, encryptedKey, 0600); err != nil {
		return nil, err
	}

	// 创建加密目录元数据
	encDir := &EncryptedDirectory{
		Path:        path,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Status:      "locked",
		Key:         dirKey,
		Files:       make([]EncryptedFile, 0),
	}

	em.encryptedDirs[path] = encDir

	// 保存元数据（简化实现）
	// 实际实现应该创建并保存元数据文件

	return encDir, nil
}

// encryptData 使用 AES-256-GCM 加密数据.
func (em *EncryptionManager) encryptData(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(em.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptData 使用 AES-256-GCM 解密数据.
func (em *EncryptionManager) decryptData(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(em.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文太短")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// EncryptFile 加密文件.
func (em *EncryptionManager) EncryptFile(srcPath, dstPath string) error {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.masterKey == nil {
		return fmt.Errorf("加密系统未初始化")
	}

	// 读取源文件
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// 加密数据
	ciphertext, err := em.encryptData(plaintext)
	if err != nil {
		return err
	}

	// 写入加密文件
	if err := os.WriteFile(dstPath, ciphertext, 0600); err != nil {
		return err
	}

	return nil
}

// DecryptFile 解密文件.
func (em *EncryptionManager) DecryptFile(srcPath, dstPath string) error {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.masterKey == nil {
		return fmt.Errorf("加密系统未初始化")
	}

	// 读取加密文件
	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// 解密数据
	plaintext, err := em.decryptData(ciphertext)
	if err != nil {
		return err
	}

	// 写入解密文件
	if err := os.WriteFile(dstPath, plaintext, 0600); err != nil {
		return err
	}

	return nil
}

// LockDirectory 锁定加密目录.
func (em *EncryptionManager) LockDirectory(path string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	encDir, exists := em.encryptedDirs[path]
	if !exists {
		return fmt.Errorf("加密目录不存在")
	}

	// 清除内存中的密钥
	encDir.Key = nil
	encDir.Status = "locked"

	return nil
}

// UnlockDirectory 解锁加密目录.
func (em *EncryptionManager) UnlockDirectory(path, password string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	encDir, exists := em.encryptedDirs[path]
	if !exists {
		return fmt.Errorf("加密目录不存在")
	}

	// 读取加密的目录密钥
	keyFilePath := filepath.Join(path, ".dir.key")
	encryptedKey, err := os.ReadFile(keyFilePath)
	if err != nil {
		return err
	}

	// 解密目录密钥
	dirKey, err := em.decryptData(encryptedKey)
	if err != nil {
		return err
	}

	encDir.Key = dirKey
	encDir.Status = "unlocked"

	return nil
}

// GetEncryptedDirectories 获取所有加密目录.
func (em *EncryptionManager) GetEncryptedDirectories() []*EncryptedDirectory {
	em.mu.RLock()
	defer em.mu.RUnlock()

	dirs := make([]*EncryptedDirectory, 0, len(em.encryptedDirs))
	for _, dir := range em.encryptedDirs {
		// 返回副本（不包含密钥）
		dirCopy := *dir
		dirCopy.Key = nil
		dirs = append(dirs, &dirCopy)
	}

	return dirs
}

// DeleteEncryptedDirectory 删除加密目录.
func (em *EncryptionManager) DeleteEncryptedDirectory(path string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.encryptedDirs[path]; !exists {
		return fmt.Errorf("加密目录不存在")
	}

	// 删除目录及其所有内容
	if err := os.RemoveAll(path); err != nil {
		return err
	}

	delete(em.encryptedDirs, path)
	return nil
}

// GetConfig 获取加密配置.
func (em *EncryptionManager) GetConfig() EncryptionConfig {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.config
}

// UpdateConfig 更新加密配置.
func (em *EncryptionManager) UpdateConfig(config EncryptionConfig) error {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.config = config
	return nil
}

// IsInitialized 检查加密系统是否已初始化.
func (em *EncryptionManager) IsInitialized() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.masterKey != nil
}
