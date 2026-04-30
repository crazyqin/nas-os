// Package encryption provides per-folder transparent encryption using AES-256-GCM.
// Implements a hierarchical key management system: Master Key → Data Keys.
//
// Architecture:
//   - Master Key: Protects data keys, derived from user password via PBKDF2
//   - Data Keys: One per encrypted folder, used for actual file encryption
//   - Transparent Encrypt/Decrypt: Files are encrypted on write, decrypted on read
//
// Key hierarchy:
//
//	User Password → PBKDF2 → Master Key (AES-256)
//	                              ├── Data Key A → encrypts Folder A files
//	                              ├── Data Key B → encrypts Folder B files
//	                              └── Data Key C → encrypts Folder C files
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
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

	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
)

// ========== Per-Folder Encryption Constants ==========

const (
	// FileHeaderMagic is the magic bytes identifying an encrypted file.
	FileHeaderMagic = "NAS-ENC"
	// FileHeaderVersion is the current encrypted file format version.
	FileHeaderVersion = 1
	// FileHeaderSize is the total size of the encrypted file header (bytes).
	FileHeaderSize = 64
	// ChunkSize is the size of data chunks for streaming encryption (64KB).
	ChunkSize = 64 * 1024
)

// ========== Per-Folder Types ==========

// EncryptedFolder represents an encrypted folder with its own data key.
type EncryptedFolder struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Path          string        `json:"path"`           // Virtual mount path
	PhysicalPath  string        `json:"physicalPath"`   // Actual encrypted data location
	State         FolderState   `json:"state"`
	Algorithm     Algorithm     `json:"algorithm"`
	EncryptedKey  string        `json:"encryptedKey"`  // Base64: data key encrypted by master key
	KeyVersion    int           `json:"keyVersion"`
	CreatedAt     time.Time     `json:"createdAt"`
	LastAccessed  time.Time     `json:"lastAccessed"`
	FileCount     int64         `json:"fileCount"`
	TotalSize     int64         `json:"totalSize"`      // Encrypted size
	OriginalSize  int64         `json:"originalSize"`   // Decrypted size
}

// FolderState represents the state of an encrypted folder.
type FolderState string

const (
	FolderLocked   FolderState = "locked"
	FolderUnlocked FolderState = "unlocked"
	FolderError    FolderState = "error"
)

// EncryptedFileHeader is stored at the beginning of each encrypted file.
type EncryptedFileHeader struct {
	Magic      [7]byte  // "NAS-ENC"
	Version    uint8    // File format version
	FolderID   [36]byte // UUID of the encrypted folder
	KeyVersion uint32   // Key version used for encryption
	Nonce      [12]byte // AES-GCM nonce for this file
	ChunkSize  uint32   // Chunk size used
	Flags      uint32   // Reserved flags
	Reserved   [4]byte  // Reserved for future use
}

// PerFolderManager manages per-folder encryption with hierarchical keys.
type PerFolderManager struct {
	mu           sync.RWMutex
	folders      map[string]*EncryptedFolder     // folderID -> folder
	decrypted    map[string]*DecryptedFolder     // folderID -> decrypted state
	masterKey    []byte                          // Master key (derived from password)
	keyStore     map[string][]byte               // folderID -> decrypted data key
	config       PerFolderConfig
	auditLogger  func(event, msg string)
}

// DecryptedFolder holds the runtime state for an unlocked folder.
type DecryptedFolder struct {
	Folder   *EncryptedFolder
	DataKey  []byte // Decrypted data key
}

// PerFolderConfig configuration for per-folder encryption.
type PerFolderConfig struct {
	PhysicalBasePath string        `json:"physicalBasePath"` // Base path for encrypted data storage
	VirtualBasePath  string        `json:"virtualBasePath"`  // Base path for mount points
	Algorithm        Algorithm     `json:"algorithm"`
	PBKDF2Iterations int           `json:"pbkdf2Iterations"`
	AutoLockMins     int           `json:"autoLockMins"`
	ChunkSize        int           `json:"chunkSize"`
}

// DefaultPerFolderConfig returns production defaults.
func DefaultPerFolderConfig(physicalBase, virtualBase string) PerFolderConfig {
	return PerFolderConfig{
		PhysicalBasePath: physicalBase,
		VirtualBasePath:  virtualBase,
		Algorithm:        AlgorithmAES256GCM,
		PBKDF2Iterations: PBKDF2Iterations,
		AutoLockMins:     60,
		ChunkSize:        ChunkSize,
	}
}

// ========== Per-Folder Manager Constructor ==========

// NewPerFolderManager creates a new per-folder encryption manager.
func NewPerFolderManager(cfg PerFolderConfig) *PerFolderManager {
	if cfg.Algorithm == "" {
		cfg.Algorithm = AlgorithmAES256GCM
	}
	if cfg.PBKDF2Iterations == 0 {
		cfg.PBKDF2Iterations = PBKDF2Iterations
	}
	if cfg.AutoLockMins == 0 {
		cfg.AutoLockMins = 60
	}
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = ChunkSize
	}
	return &PerFolderManager{
		folders:   make(map[string]*EncryptedFolder),
		decrypted: make(map[string]*DecryptedFolder),
		keyStore:  make(map[string][]byte),
		config:    cfg,
	}
}

// SetAuditLogger sets the audit log callback.
func (m *PerFolderManager) SetAuditLogger(fn func(event, msg string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditLogger = fn
}

// ========== Master Key Management ==========

// UnlockWithPassword derives the master key from a user password.
// Must be called before any folder operations.
func (m *PerFolderManager) UnlockWithPassword(password string, salt []byte) error {
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	if len(salt) == 0 {
		return fmt.Errorf("salt cannot be empty")
	}

	derived := pbkdf2.Key([]byte(password), salt, m.config.PBKDF2Iterations, KeyLength, sha256.New)

	m.mu.Lock()
	m.masterKey = derived
	m.mu.Unlock()

	if m.auditLogger != nil {
		m.auditLogger("master_key_unlocked", "master key derived from password")
	}
	return nil
}

// LockMasterKey clears the master key from memory, locking all folders.
func (m *PerFolderManager) LockMasterKey() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Lock all unlocked folders
	for id, df := range m.decrypted {
		zeroBytes(df.DataKey)
		if f, ok := m.folders[id]; ok {
			f.State = FolderLocked
		}
	}
	m.decrypted = make(map[string]*DecryptedFolder)

	// Clear master key
	if m.masterKey != nil {
		zeroBytes(m.masterKey)
		m.masterKey = nil
	}

	// Clear all data keys from store
	for _, k := range m.keyStore {
		zeroBytes(k)
	}
	m.keyStore = make(map[string][]byte)

	if m.auditLogger != nil {
		m.auditLogger("master_key_locked", "all folders locked and master key cleared")
	}
}

// IsUnlocked returns true if the master key is loaded.
func (m *PerFolderManager) IsUnlocked() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.masterKey != nil
}

// ========== Folder CRUD ==========

// CreateFolder creates a new encrypted folder with its own data key.
func (m *PerFolderManager) CreateFolder(name string) (*EncryptedFolder, error) {
	if name == "" {
		return nil, fmt.Errorf("folder name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.masterKey == nil {
		return nil, fmt.Errorf("master key not loaded; unlock with password first")
	}

	// Check duplicate name
	for _, f := range m.folders {
		if f.Name == name {
			return nil, fmt.Errorf("folder '%s' already exists", name)
		}
	}

	// Generate random data key
	dataKey := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, dataKey); err != nil {
		return nil, fmt.Errorf("generate data key: %w", err)
	}

	// Encrypt data key with master key
	encryptedKey, err := encryptAESGCM(dataKey, m.masterKey)
	if err != nil {
		zeroBytes(dataKey)
		return nil, fmt.Errorf("encrypt data key: %w", err)
	}

	folderID := uuid.New().String()
	now := time.Now()

	folder := &EncryptedFolder{
		ID:           folderID,
		Name:         name,
		Path:         filepath.Join(m.config.VirtualBasePath, name),
		PhysicalPath: filepath.Join(m.config.PhysicalBasePath, folderID),
		State:        FolderLocked,
		Algorithm:    m.config.Algorithm,
		EncryptedKey: base64.StdEncoding.EncodeToString(encryptedKey),
		KeyVersion:   1,
		CreatedAt:    now,
		LastAccessed: now,
	}

	m.folders[folderID] = folder

	// Store data key in memory (folder is created unlocked)
	m.decrypted[folderID] = &DecryptedFolder{
		Folder:  folder,
		DataKey: dataKey,
	}
	m.keyStore[folderID] = dataKey
	folder.State = FolderUnlocked

	if m.auditLogger != nil {
		m.auditLogger("folder_created", fmt.Sprintf("folder '%s' (id=%s)", name, folderID))
	}

	return folder, nil
}

// UnlockFolder decrypts a folder's data key using the master key.
func (m *PerFolderManager) UnlockFolder(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.unlockFolderInternal(folderID)
}

func (m *PerFolderManager) unlockFolderInternal(folderID string) error {
	if m.masterKey == nil {
		return fmt.Errorf("master key not loaded")
	}

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	if folder.State == FolderUnlocked {
		return nil // already unlocked
	}

	// Decrypt data key
	encKeyBytes, err := base64.StdEncoding.DecodeString(folder.EncryptedKey)
	if err != nil {
		return fmt.Errorf("decode encrypted key: %w", err)
	}

	dataKey, err := decryptAESGCM(encKeyBytes, m.masterKey)
	if err != nil {
		return fmt.Errorf("decrypt data key (wrong master key?): %w", err)
	}

	m.decrypted[folderID] = &DecryptedFolder{
		Folder:  folder,
		DataKey: dataKey,
	}
	m.keyStore[folderID] = dataKey
	folder.State = FolderUnlocked
	folder.LastAccessed = time.Now()

	if m.auditLogger != nil {
		m.auditLogger("folder_unlocked", fmt.Sprintf("folder '%s' (id=%s)", folder.Name, folderID))
	}
	return nil
}

// LockFolder locks a folder and clears its data key from memory.
func (m *PerFolderManager) LockFolder(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	if df, ok := m.decrypted[folderID]; ok {
		zeroBytes(df.DataKey)
		delete(m.decrypted, folderID)
	}
	if k, ok := m.keyStore[folderID]; ok {
		zeroBytes(k)
		delete(m.keyStore, folderID)
	}

	folder.State = FolderLocked

	if m.auditLogger != nil {
		m.auditLogger("folder_locked", fmt.Sprintf("folder '%s' (id=%s)", folder.Name, folderID))
	}
	return nil
}

// DeleteFolder deletes an encrypted folder and its data key.
func (m *PerFolderManager) DeleteFolder(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	// Clean up decrypted state
	if df, ok := m.decrypted[folderID]; ok {
		zeroBytes(df.DataKey)
		delete(m.decrypted, folderID)
	}
	if k, ok := m.keyStore[folderID]; ok {
		zeroBytes(k)
		delete(m.keyStore, folderID)
	}

	delete(m.folders, folderID)

	if m.auditLogger != nil {
		m.auditLogger("folder_deleted", fmt.Sprintf("folder '%s' (id=%s)", folder.Name, folderID))
	}
	return nil
}

// GetFolder returns folder info.
func (m *PerFolderManager) GetFolder(folderID string) (*EncryptedFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	f, ok := m.folders[folderID]
	if !ok {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}
	copy := *f
	copy.EncryptedKey = "***"
	return &copy, nil
}

// ListFolders returns all encrypted folders.
func (m *PerFolderManager) ListFolders() []*EncryptedFolder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*EncryptedFolder, 0, len(m.folders))
	for _, f := range m.folders {
		copy := *f
		copy.EncryptedKey = "***"
		result = append(result, &copy)
	}
	return result
}

// ========== Transparent Encrypt / Decrypt ==========

// EncryptData encrypts plaintext data using a folder's data key.
// Returns: encrypted bytes with file header prepended.
func (m *PerFolderManager) EncryptData(folderID string, plaintext []byte) ([]byte, error) {
	m.mu.RLock()
	df, ok := m.decrypted[folderID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("folder not unlocked: %s", folderID)
	}
	dataKey := df.DataKey
	folder := df.Folder
	m.mu.RUnlock()

	return encryptWithKey(plaintext, dataKey, folderID, folder.KeyVersion)
}

// DecryptData decrypts data that was encrypted with EncryptData.
func (m *PerFolderManager) DecryptData(folderID string, ciphertext []byte) ([]byte, error) {
	m.mu.RLock()
	df, ok := m.decrypted[folderID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("folder not unlocked: %s", folderID)
	}
	dataKey := df.DataKey
	m.mu.RUnlock()

	return decryptWithKey(ciphertext, dataKey)
}

// EncryptFile encrypts a file from srcPath and writes to dstPath.
func (m *PerFolderManager) EncryptFile(folderID, srcPath, dstPath string) error {
	m.mu.RLock()
	df, ok := m.decrypted[folderID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("folder not unlocked: %s", folderID)
	}
	dataKey := make([]byte, len(df.DataKey))
	copy(dataKey, df.DataKey)
	folder := df.Folder
	m.mu.RUnlock()

	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	encrypted, err := encryptWithKey(plaintext, dataKey, folderID, folder.KeyVersion)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(dstPath, encrypted, 0600); err != nil {
		return fmt.Errorf("write encrypted file: %w", err)
	}

	// Update folder stats
	m.mu.Lock()
	if f, exists := m.folders[folderID]; exists {
		f.FileCount++
		f.TotalSize += int64(len(encrypted))
		f.OriginalSize += int64(len(plaintext))
		f.LastAccessed = time.Now()
	}
	m.mu.Unlock()

	return nil
}

// DecryptFile decrypts a file from srcPath and writes to dstPath.
func (m *PerFolderManager) DecryptFile(folderID, srcPath, dstPath string) error {
	m.mu.RLock()
	df, ok := m.decrypted[folderID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("folder not unlocked: %s", folderID)
	}
	dataKey := make([]byte, len(df.DataKey))
	copy(dataKey, df.DataKey)
	m.mu.RUnlock()

	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read encrypted file: %w", err)
	}

	plaintext, err := decryptWithKey(ciphertext, dataKey)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(dstPath, plaintext, 0600); err != nil {
		return fmt.Errorf("write decrypted file: %w", err)
	}

	m.mu.Lock()
	if f, exists := m.folders[folderID]; exists {
		f.LastAccessed = time.Now()
	}
	m.mu.Unlock()

	return nil
}

// ========== Key Rotation ==========

// RotateKey generates a new data key for a folder.
// Re-encrypts the data key with the master key.
func (m *PerFolderManager) RotateKey(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.masterKey == nil {
		return fmt.Errorf("master key not loaded")
	}

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	// Generate new data key
	newKey := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return fmt.Errorf("generate new data key: %w", err)
	}

	// Encrypt with master key
	encryptedKey, err := encryptAESGCM(newKey, m.masterKey)
	if err != nil {
		zeroBytes(newKey)
		return fmt.Errorf("encrypt new data key: %w", err)
	}

	// Clear old key
	if oldKey, ok := m.keyStore[folderID]; ok {
		zeroBytes(oldKey)
	}

	// Update state
	folder.EncryptedKey = base64.StdEncoding.EncodeToString(encryptedKey)
	folder.KeyVersion++
	m.keyStore[folderID] = newKey

	if df, ok := m.decrypted[folderID]; ok {
		zeroBytes(df.DataKey)
		df.DataKey = newKey
	}

	if m.auditLogger != nil {
		m.auditLogger("key_rotated", fmt.Sprintf("folder '%s' key rotated to version %d", folder.Name, folder.KeyVersion))
	}
	return nil
}

// ========== Auto-Lock ==========

// AutoLockExpired locks folders that haven't been accessed within the timeout.
func (m *PerFolderManager) AutoLockExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.AutoLockMins <= 0 {
		return 0
	}

	deadline := time.Now().Add(-time.Duration(m.config.AutoLockMins) * time.Minute)
	locked := 0

	for id, folder := range m.folders {
		if folder.State == FolderUnlocked && folder.LastAccessed.Before(deadline) {
			if df, ok := m.decrypted[id]; ok {
				zeroBytes(df.DataKey)
				delete(m.decrypted, id)
			}
			if k, ok := m.keyStore[id]; ok {
				zeroBytes(k)
				delete(m.keyStore, id)
			}
			folder.State = FolderLocked
			locked++
		}
	}
	return locked
}

// ========== Internal Encryption Helpers ==========

func encryptWithKey(plaintext, key []byte, folderID string, keyVersion int) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Build header
	header := make([]byte, FileHeaderSize)
	copy(header[0:7], []byte(FileHeaderMagic))
	header[7] = FileHeaderVersion
	copy(header[8:44], folderID) // UUID is 36 chars
	// keyVersion at offset 44-47 (big endian)
	header[44] = byte(keyVersion >> 24)
	header[45] = byte(keyVersion >> 16)
	header[46] = byte(keyVersion >> 8)
	header[47] = byte(keyVersion)
	copy(header[48:60], nonce)

	// Encrypt: nonce prepended to ciphertext (GCM standard)
	encrypted := gcm.Seal(nonce, nonce, plaintext, nil)

	// Final: header + encrypted(nonce + ciphertext + tag)
	return append(header, encrypted...), nil
}

func decryptWithKey(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) < FileHeaderSize {
		return nil, fmt.Errorf("ciphertext too short: need at least %d bytes, got %d", FileHeaderSize, len(ciphertext))
	}

	// Verify magic
	if string(ciphertext[0:7]) != FileHeaderMagic {
		return nil, fmt.Errorf("invalid file header magic")
	}

	// Verify version
	if ciphertext[7] != FileHeaderVersion {
		return nil, fmt.Errorf("unsupported file format version: %d", ciphertext[7])
	}

	// Extract encrypted payload
	payload := ciphertext[FileHeaderSize:]
	if len(payload) < 12 {
		return nil, fmt.Errorf("encrypted payload too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := payload[:gcm.NonceSize()]
	encrypted := payload[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong key or corrupted data): %w", err)
	}

	return plaintext, nil
}

// ========== JSON Serialization ==========

// MarshalFolderKey marshals the encrypted folder key for persistence.
func (m *PerFolderManager) MarshalFolderKey(folderID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	f, ok := m.folders[folderID]
	if !ok {
		return "", fmt.Errorf("folder not found")
	}
	return f.EncryptedKey, nil
}

// RestoreFolder restores a folder from persisted data (without requiring master key).
func (m *PerFolderManager) RestoreFolder(folder *EncryptedFolder) {
	m.mu.Lock()
	defer m.mu.Unlock()
	folder.State = FolderLocked
	m.folders[folder.ID] = folder
}

// Stats returns encryption statistics.
func (m *PerFolderManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.folders)
	unlocked := 0
	var totalSize, originalSize int64
	var totalFiles int64

	for _, f := range m.folders {
		if f.State == FolderUnlocked {
			unlocked++
		}
		totalSize += f.TotalSize
		originalSize += f.OriginalSize
		totalFiles += f.FileCount
	}

	return map[string]interface{}{
		"totalFolders":    total,
		"unlockedFolders": unlocked,
		"lockedFolders":   total - unlocked,
		"totalFiles":      totalFiles,
		"totalEncryptedSize": totalSize,
		"totalOriginalSize":  originalSize,
		"masterKeyLoaded": m.masterKey != nil,
	}
}

// HMACKey generates an HMAC-SHA256 key for trust token signing.
func HMACKey(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// GenerateSalt generates a random salt for PBKDF2.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// computeHMAC computes HMAC-SHA256.
func computeHMAC(data, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// MarshalJSON implements json.Marshaler for PerFolderManager (non-sensitive fields only).
func (m *PerFolderManager) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return json.Marshal(struct {
		FolderCount  int  `json:"folderCount"`
		MasterKeySet bool `json:"masterKeySet"`
	}{
		FolderCount:  len(m.folders),
		MasterKeySet: m.masterKey != nil,
	})
}
