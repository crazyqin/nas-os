// Package apikey provides secure API key management
// encryption.go - Encryption at rest using system key
package apikey

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EncryptionConfig holds encryption configuration
type EncryptionConfig struct {
	// KeySource determines where the encryption key comes from
	KeySource KeySource `json:"key_source"`

	// KeyPath is the path to the key file (for file source)
	KeyPath string `json:"key_path,omitempty"`

	// RotationInterval is how often to rotate keys (days)
	RotationInterval int `json:"rotation_interval,omitempty"`

	// Algorithm specifies the encryption algorithm
	Algorithm string `json:"algorithm"` // AES-256-GCM default
}

// KeySource defines where encryption keys come from
type KeySource string

const (
	// KeySourceSystem - 系统自动生成的密钥
	KeySourceSystem KeySource = "system"
	// KeySourceFile - 从文件读取密钥
	KeySourceFile KeySource = "file"
	// KeySourceHSM - 硬件安全模块
	KeySourceHSM KeySource = "hsm"
	// KeySourceVault - 外部密钥库 (HashiCorp等)
	KeySourceVault KeySource = "vault"
)

// KeyManager manages encryption keys for API key storage
type KeyManager struct {
	config      EncryptionConfig
	masterKey   []byte
	keyVersion  int
	keyCreated  time.Time
	mu          sync.RWMutex
	keyFilePath string
}

// DefaultEncryptionConfig returns default encryption configuration
func DefaultEncryptionConfig() EncryptionConfig {
	return EncryptionConfig{
		KeySource:        KeySourceSystem,
		RotationInterval: 90, // 90 days default
		Algorithm:        "AES-256-GCM",
	}
}

// NewKeyManager creates a new key manager
func NewKeyManager(config EncryptionConfig) (*KeyManager, error) {
	km := &KeyManager{
		config: config,
	}

	// Determine key file path
	if config.KeyPath != "" {
		km.keyFilePath = config.KeyPath
	} else {
		// Default to system config directory
		configDir := "/etc/nas-os"
		if home, err := os.UserHomeDir(); err == nil {
			configDir = filepath.Join(home, ".nas-os")
		}
		km.keyFilePath = filepath.Join(configDir, "ai", "master.key")
	}

	// Load or generate master key
	if err := km.loadOrGenerateKey(); err != nil {
		return nil, fmt.Errorf("failed to initialize key manager: %w", err)
	}

	return km, nil
}

// loadOrGenerateKey loads existing key or generates a new one
func (km *KeyManager) loadOrGenerateKey() error {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Try to load existing key
	data, err := os.ReadFile(km.keyFilePath)
	if err == nil && len(data) > 0 {
		// Parse key file
		// Format: version:base64_key
		parts := splitKeyFile(string(data))
		if len(parts) >= 2 {
			version := parseInt(parts[0])
			key, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil || len(key) != 32 {
				// Invalid key, generate new one
				return km.generateNewKey()
			}
			km.masterKey = key
			km.keyVersion = version
			km.keyCreated = parseTime(parts[2])
			return nil
		}
	}

	// Generate new key
	return km.generateNewKey()
}

// generateNewKey generates a new master key
func (km *KeyManager) generateNewKey() error {
	// Generate 32-byte key for AES-256
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	km.masterKey = key
	km.keyVersion = 1
	km.keyCreated = time.Now()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(km.keyFilePath), 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write key file with restricted permissions
	keyData := fmt.Sprintf("%d:%s:%s",
		km.keyVersion,
		base64.StdEncoding.EncodeToString(km.masterKey),
		km.keyCreated.Format(time.RFC3339),
	)

	if err := os.WriteFile(km.keyFilePath, []byte(keyData), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (km *KeyManager) Encrypt(plaintext string) ([]byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if len(plaintext) == 0 {
		return nil, errors.New("plaintext cannot be empty")
	}

	block, err := aes.NewCipher(km.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (km *KeyManager) Decrypt(ciphertext []byte) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if len(ciphertext) == 0 {
		return "", errors.New("ciphertext cannot be empty")
	}

	block, err := aes.NewCipher(km.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and decrypt
	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// HashKey creates a SHA-256 hash of the key for verification
func (km *KeyManager) HashKey(key string) string {
	h := sha256.Sum256([]byte(key + "nas-os-apikey-salt"))
	return hex.EncodeToString(h[:])
}

// GetKeyPreview returns the last 4 characters of a key for display
func GetKeyPreview(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

// RotateKey rotates the master encryption key
func (km *KeyManager) RotateKey() error {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Generate new key
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return fmt.Errorf("failed to generate new key: %w", err)
	}

	// Update key
	km.masterKey = newKey
	km.keyVersion++
	km.keyCreated = time.Now()

	// Save new key
	keyData := fmt.Sprintf("%d:%s:%s",
		km.keyVersion,
		base64.StdEncoding.EncodeToString(km.masterKey),
		km.keyCreated.Format(time.RFC3339),
	)

	if err := os.WriteFile(km.keyFilePath, []byte(keyData), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

// GetKeyInfo returns information about the current key
func (km *KeyManager) GetKeyInfo() map[string]any {
	km.mu.RLock()
	defer km.mu.RUnlock()

	return map[string]any{
		"version":    km.keyVersion,
		"created_at": km.keyCreated.Format(time.RFC3339),
		"algorithm":  km.config.Algorithm,
		"key_source": string(km.config.KeySource),
	}
}

// ShouldRotate checks if key should be rotated based on interval
func (km *KeyManager) ShouldRotate() bool {
	if km.config.RotationInterval <= 0 {
		return false
	}

	km.mu.RLock()
	defer km.mu.RUnlock()

	rotationDays := time.Duration(km.config.RotationInterval) * 24 * time.Hour
	return time.Since(km.keyCreated) > rotationDays
}

// Helper functions

func splitKeyFile(data string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == ':' {
			parts = append(parts, data[start:i])
			start = i + 1
		}
		if data[i] == '\n' {
			break
		}
	}
	if start < len(data) {
		parts = append(parts, data[start:])
	}
	return parts
}

func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// SecureMemory provides secure storage for keys in memory
type SecureMemory struct {
	data   []byte
	locked bool
	mu     sync.Mutex
}

// NewSecureMemory creates a new secure memory buffer
func NewSecureMemory() *SecureMemory {
	return &SecureMemory{}
}

// Store stores data securely in memory
func (sm *SecureMemory) Store(data []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear old data
	if sm.data != nil {
		for i := range sm.data {
			sm.data[i] = 0
		}
	}

	// Copy new data
	sm.data = make([]byte, len(data))
	copy(sm.data, data)
	sm.locked = false

	return nil
}

// Retrieve retrieves data from secure memory
func (sm *SecureMemory) Retrieve() ([]byte, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.locked {
		return nil, errors.New("memory is locked")
	}

	if sm.data == nil {
		return nil, errors.New("no data stored")
	}

	// Return a copy
	result := make([]byte, len(sm.data))
	copy(result, sm.data)

	return result, nil
}

// Lock prevents further access to the data
func (sm *SecureMemory) Lock() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.locked = true
}

// Unlock allows access to the data
func (sm *SecureMemory) Unlock() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.locked = false
}

// Clear securely erases the stored data
func (sm *SecureMemory) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.data != nil {
		for i := range sm.data {
			sm.data[i] = 0
		}
		sm.data = nil
	}
}

// RandomKey generates a random API key
func RandomKey(length int) (string, error) {
	if length <= 0 {
		length = 32
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)

	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}

	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}

	return string(b), nil
}
