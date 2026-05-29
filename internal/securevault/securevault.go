package securevault

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
)

// VaultEntry represents a vault entry
type VaultEntry struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	EncryptedData string  `json:"encrypted_data"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

// VaultConfig defines vault configuration
type VaultConfig struct {
	MaxEntries      int           `json:"max_entries"`
	MaxEntrySize    int           `json:"max_entry_size"`
	DefaultExpiry   time.Duration `json:"default_expiry"`
	AutoLockTimeout time.Duration `json:"auto_lock_timeout"`
}

// SecureVault provides encrypted storage for sensitive data
// Inspired by Synology encrypted shared folders
type SecureVault struct {
	mu          sync.RWMutex
	entries     map[string]*VaultEntry
	config      VaultConfig
	masterKey   []byte
	locked      bool
	lastAccess  time.Time
}

// NewSecureVault creates a new SecureVault instance
func NewSecureVault(masterPassword string) *SecureVault {
	key := sha256.Sum256([]byte(masterPassword))
	return &SecureVault{
		entries: make(map[string]*VaultEntry),
		config: VaultConfig{
			MaxEntries:      10000,
			MaxEntrySize:    1024 * 1024, // 1MB
			DefaultExpiry:   365 * 24 * time.Hour,
			AutoLockTimeout: 15 * time.Minute,
		},
		masterKey:  key[:],
		locked:     false,
		lastAccess: time.Now(),
	}
}

// Lock locks the vault
func (sv *SecureVault) Lock() {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.locked = true
}

// Unlock unlocks the vault with the master password
func (sv *SecureVault) Unlock(password string) error {
	key := sha256.Sum256([]byte(password))
	for i := range key {
		if key[i] != sv.masterKey[i] {
			return fmt.Errorf("invalid password")
		}
	}

	sv.mu.Lock()
	sv.locked = false
	sv.lastAccess = time.Now()
	sv.mu.Unlock()
	return nil
}

// IsLocked returns whether the vault is locked
func (sv *SecureVault) IsLocked() bool {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.locked
}

// Store stores an entry in the vault
func (sv *SecureVault) Store(name, category, data string) (*VaultEntry, error) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.locked {
		return nil, fmt.Errorf("vault is locked")
	}

	if len(sv.entries) >= sv.config.MaxEntries {
		return nil, fmt.Errorf("vault is full")
	}

	encrypted, err := sv.encrypt(data)
	if err != nil {
		return nil, err
	}

	entry := &VaultEntry{
		ID:            fmt.Sprintf("entry-%d", time.Now().UnixNano()),
		Name:          name,
		Category:      category,
		EncryptedData: encrypted,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	sv.entries[entry.ID] = entry
	sv.lastAccess = time.Now()
	return entry, nil
}

// Retrieve retrieves an entry from the vault
func (sv *SecureVault) Retrieve(entryID string) (string, error) {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	if sv.locked {
		return "", fmt.Errorf("vault is locked")
	}

	entry, exists := sv.entries[entryID]
	if !exists {
		return "", fmt.Errorf("entry not found")
	}

	decrypted, err := sv.decrypt(entry.EncryptedData)
	if err != nil {
		return "", err
	}

	sv.lastAccess = time.Now()
	return decrypted, nil
}

// Delete deletes an entry from the vault
func (sv *SecureVault) Delete(entryID string) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.locked {
		return fmt.Errorf("vault is locked")
	}

	if _, exists := sv.entries[entryID]; !exists {
		return fmt.Errorf("entry not found")
	}

	delete(sv.entries, entryID)
	return nil
}

// ListEntries lists all entries (without decrypted data)
func (sv *SecureVault) ListEntries() []*VaultEntry {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	entries := make([]*VaultEntry, 0, len(sv.entries))
	for _, e := range sv.entries {
		entries = append(entries, e)
	}
	return entries
}

// SearchByCategory searches entries by category
func (sv *SecureVault) SearchByCategory(category string) []*VaultEntry {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	var results []*VaultEntry
	for _, e := range sv.entries {
		if e.Category == category {
			results = append(results, e)
		}
	}
	return results
}

// SearchByTag searches entries by tag
func (sv *SecureVault) SearchByTag(tag string) []*VaultEntry {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	var results []*VaultEntry
	for _, e := range sv.entries {
		for _, t := range e.Tags {
			if t == tag {
				results = append(results, e)
				break
			}
		}
	}
	return results
}

// ChangePassword changes the master password
func (sv *SecureVault) ChangePassword(oldPassword, newPassword string) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	// Verify old password
	oldKey := sha256.Sum256([]byte(oldPassword))
	for i := range oldKey {
		if oldKey[i] != sv.masterKey[i] {
			return fmt.Errorf("invalid old password")
		}
	}

	// Re-encrypt all entries
	newKey := sha256.Sum256([]byte(newPassword))
	for _, entry := range sv.entries {
		// Decrypt with old key
		decrypted, err := sv.decrypt(entry.EncryptedData)
		if err != nil {
			return err
		}

		// Encrypt with new key
		sv.masterKey = newKey[:]
		encrypted, err := sv.encrypt(decrypted)
		if err != nil {
			return err
		}
		entry.EncryptedData = encrypted
	}

	sv.masterKey = newKey[:]
	return nil
}

// GetStats returns vault statistics
func (sv *SecureVault) GetStats() map[string]interface{} {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	categories := make(map[string]int)
	for _, e := range sv.entries {
		categories[e.Category]++
	}

	return map[string]interface{}{
		"total_entries": len(sv.entries),
		"max_entries":   sv.config.MaxEntries,
		"locked":        sv.locked,
		"categories":    categories,
		"last_access":   sv.lastAccess,
	}
}

func (sv *SecureVault) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(sv.masterKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (sv *SecureVault) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(sv.masterKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
