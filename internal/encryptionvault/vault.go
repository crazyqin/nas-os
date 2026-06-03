package encryptionvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// VaultConfig 加密保险库配置
type VaultConfig struct {
	VaultPath     string        `json:"vault_path"`
	KeyIterations int           `json:"key_iterations"`
	KeyLength     int           `json:"key_length"`
	AutoLockTime  time.Duration `json:"auto_lock_time"`
	MaxAttempts   int           `json:"max_attempts"`
}

// VaultEntry 保险库条目
type VaultEntry struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	EncryptedAt time.Time `json:"encrypted_at"`
	Size        int64     `json:"size"`
	Algorithm   string    `json:"algorithm"`
	Salt        string    `json:"salt"`
	Nonce       string    `json:"nonce"`
}

// VaultState 保险库状态
type VaultState int

const (
	VaultLocked VaultState = iota
	VaultUnlocked
)

// EncryptionVault 加密保险库
type EncryptionVault struct {
	config    VaultConfig
	state     VaultState
	entries   map[string]*VaultEntry
	cipherKey []byte
	mu        sync.RWMutex
	attempts  int
	lastUnlock time.Time
}

// NewEncryptionVault 创建加密保险库
func NewEncryptionVault(config VaultConfig) *EncryptionVault {
	if config.KeyIterations == 0 {
		config.KeyIterations = 100000
	}
	if config.KeyLength == 0 {
		config.KeyLength = 32
	}
	if config.AutoLockTime == 0 {
		config.AutoLockTime = 30 * time.Minute
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 5
	}
	os.MkdirAll(config.VaultPath, 0700)
	return &EncryptionVault{
		config:  config,
		state:   VaultLocked,
		entries: make(map[string]*VaultEntry),
	}
}

// Unlock 解锁保险库
func (v *EncryptionVault) Unlock(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.attempts >= v.config.MaxAttempts {
		return fmt.Errorf("too many failed attempts, vault locked")
	}
	salt := v.loadOrCreateSalt()
	key := pbkdf2.Key([]byte(password), salt, v.config.KeyIterations, v.config.KeyLength, sha256.New)
	if !v.verifyKey(key) {
		v.attempts++
		return fmt.Errorf("invalid password, %d attempts remaining", v.config.MaxAttempts-v.attempts)
	}
	v.cipherKey = key
	v.state = VaultUnlocked
	v.attempts = 0
	v.lastUnlock = time.Now()
	v.loadEntries()
	return nil
}

// Lock 锁定保险库
func (v *EncryptionVault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cipherKey = nil
	v.state = VaultLocked
}

// EncryptFile 加密文件
func (v *EncryptionVault) EncryptFile(sourcePath, destName string) (*VaultEntry, error) {
	v.mu.RLock()
	if v.state != VaultLocked {
		v.mu.RUnlock()
		return nil, fmt.Errorf("vault must be locked to add files")
	}
	v.mu.RUnlock()

	plaintext, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}

	block, err := aes.NewCipher(v.cipherKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	destPath := filepath.Join(v.config.VaultPath, destName+".vault")
	if err := os.WriteFile(destPath, ciphertext, 0600); err != nil {
		return nil, fmt.Errorf("write encrypted: %w", err)
	}

	entry := &VaultEntry{
		ID:          generateID(),
		Name:        destName,
		Path:        destPath,
		EncryptedAt: time.Now(),
		Size:        int64(len(ciphertext)),
		Algorithm:   "AES-256-GCM",
	}

	v.mu.Lock()
	v.entries[entry.ID] = entry
	v.saveEntries()
	v.mu.Unlock()

	return entry, nil
}

// DecryptFile 解密文件
func (v *EncryptionVault) DecryptFile(entryID, destPath string) error {
	v.mu.RLock()
	if v.state != VaultUnlocked {
		v.mu.RUnlock()
		return fmt.Errorf("vault is locked")
	}
	entry, ok := v.entries[entryID]
	if !ok {
		v.mu.RUnlock()
		return fmt.Errorf("entry not found")
	}
	v.mu.RUnlock()

	ciphertext, err := os.ReadFile(entry.Path)
	if err != nil {
		return fmt.Errorf("read encrypted: %w", err)
	}

	block, err := aes.NewCipher(v.cipherKey)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	return os.WriteFile(destPath, plaintext, 0600)
}

// ListEntries 列出所有条目
func (v *EncryptionVault) ListEntries() []*VaultEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	entries := make([]*VaultEntry, 0, len(v.entries))
	for _, e := range v.entries {
		entries = append(entries, e)
	}
	return entries
}

// ChangePassword 修改密码
func (v *EncryptionVault) ChangePassword(oldPassword, newPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state != VaultUnlocked {
		return fmt.Errorf("vault is locked")
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	newKey := pbkdf2.Key([]byte(newPassword), salt, v.config.KeyIterations, v.config.KeyLength, sha256.New)
	v.cipherKey = newKey
	v.saveSalt(salt)
	v.saveKeyHash(newKey)
	return nil
}

// AutoLockCheck 自动锁定检查
func (v *EncryptionVault) AutoLockCheck() {
	v.mu.RLock()
	if v.state == VaultUnlocked && time.Since(v.lastUnlock) > v.config.AutoLockTime {
		v.mu.RUnlock()
		v.Lock()
		return
	}
	v.mu.RUnlock()
}

// GetState 获取状态
func (v *EncryptionVault) GetState() VaultState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.state
}

func (v *EncryptionVault) loadOrCreateSalt() []byte {
	saltPath := filepath.Join(v.config.VaultPath, ".salt")
	data, err := os.ReadFile(saltPath)
	if err == nil {
		return data
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	os.WriteFile(saltPath, salt, 0600)
	return salt
}

func (v *EncryptionVault) saveSalt(salt []byte) {
	saltPath := filepath.Join(v.config.VaultPath, ".salt")
	os.WriteFile(saltPath, salt, 0600)
}

func (v *EncryptionVault) verifyKey(key []byte) bool {
	hashPath := filepath.Join(v.config.VaultPath, ".keyhash")
	data, err := os.ReadFile(hashPath)
	if err != nil {
		v.saveKeyHash(key)
		return true
	}
	hash := sha256.Sum256(key)
	return base64.StdEncoding.EncodeToString(hash[:]) == string(data)
}

func (v *EncryptionVault) saveKeyHash(key []byte) {
	hash := sha256.Sum256(key)
	hashPath := filepath.Join(v.config.VaultPath, ".keyhash")
	os.WriteFile(hashPath, []byte(base64.StdEncoding.EncodeToString(hash[:])), 0600)
}

func (v *EncryptionVault) loadEntries() {
	entriesPath := filepath.Join(v.config.VaultPath, ".entries")
	data, err := os.ReadFile(entriesPath)
	if err != nil {
		return
	}
	var entries map[string]*VaultEntry
	json.Unmarshal(data, &entries)
	v.entries = entries
}

func (v *EncryptionVault) saveEntries() {
	entriesPath := filepath.Join(v.config.VaultPath, ".entries")
	data, _ := json.MarshalIndent(v.entries, "", "  ")
	os.WriteFile(entriesPath, data, 0600)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// VaultManager 保险库管理器
type VaultManager struct {
	vaults map[string]*EncryptionVault
	mu     sync.RWMutex
}

// NewVaultManager 创建管理器
func NewVaultManager() *VaultManager {
	return &VaultManager{
		vaults: make(map[string]*EncryptionVault),
	}
}

// CreateVault 创建保险库
func (m *VaultManager) CreateVault(name string, config VaultConfig) *EncryptionVault {
	m.mu.Lock()
	defer m.mu.Unlock()
	vault := NewEncryptionVault(config)
	m.vaults[name] = vault
	return vault
}

// GetVault 获取保险库
func (m *VaultManager) GetVault(name string) (*EncryptionVault, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.vaults[name]
	return v, ok
}

// ListVaults 列出所有保险库
func (m *VaultManager) ListVaults() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.vaults))
	for name := range m.vaults {
		names = append(names, name)
	}
	return names
}
