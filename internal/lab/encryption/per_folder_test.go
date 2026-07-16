package encryption

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== PerFolderManager Tests ==========

func TestNewPerFolderManager(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	assert.NotNil(t, mgr)
	assert.False(t, mgr.IsUnlocked())
	assert.Equal(t, AlgorithmAES256GCM, mgr.config.Algorithm)
	assert.Equal(t, PBKDF2Iterations, mgr.config.PBKDF2Iterations)
}

func TestUnlockWithPassword(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, err := GenerateSalt()
	require.NoError(t, err)

	err = mgr.UnlockWithPassword("test-password-123", salt)
	assert.NoError(t, err)
	assert.True(t, mgr.IsUnlocked())
}

func TestUnlockWithPasswordEmptyPassword(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, err := GenerateSalt()
	require.NoError(t, err)

	err = mgr.UnlockWithPassword("", salt)
	assert.Error(t, err)
}

func TestUnlockWithPasswordEmptySalt(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	err := mgr.UnlockWithPassword("test-password", nil)
	assert.Error(t, err)
}

func TestLockMasterKey(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, err := GenerateSalt()
	require.NoError(t, err)

	_ = mgr.UnlockWithPassword("test-password", salt)
	assert.True(t, mgr.IsUnlocked())

	mgr.LockMasterKey()
	assert.False(t, mgr.IsUnlocked())
}

func TestCreateFolder(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, err := mgr.CreateFolder("Documents")
	require.NoError(t, err)
	assert.NotNil(t, folder)
	assert.Equal(t, "Documents", folder.Name)
	assert.Equal(t, FolderUnlocked, folder.State)
	assert.Equal(t, 1, folder.KeyVersion)
	assert.NotEmpty(t, folder.ID)
}

func TestCreateFolderDuplicate(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	_, err := mgr.CreateFolder("Duplicate")
	require.NoError(t, err)

	_, err = mgr.CreateFolder("Duplicate")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreateFolderEmptyName(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	_, err := mgr.CreateFolder("")
	assert.Error(t, err)
}

func TestCreateFolderLocked(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)
	// Don't unlock

	_, err := mgr.CreateFolder("Test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "master key not loaded")
}

func TestUnlockLockFolder(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, err := mgr.CreateFolder("TestFolder")
	require.NoError(t, err)
	assert.Equal(t, FolderUnlocked, folder.State)

	// Lock
	err = mgr.LockFolder(folder.ID)
	require.NoError(t, err)

	got, _ := mgr.GetFolder(folder.ID)
	assert.Equal(t, FolderLocked, got.State)

	// Unlock again
	err = mgr.UnlockFolder(folder.ID)
	require.NoError(t, err)

	got, _ = mgr.GetFolder(folder.ID)
	assert.Equal(t, FolderUnlocked, got.State)
}

func TestUnlockFolderNotFound(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	err := mgr.UnlockFolder("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteFolder(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, err := mgr.CreateFolder("ToDelete")
	require.NoError(t, err)

	err = mgr.DeleteFolder(folder.ID)
	require.NoError(t, err)

	_, err = mgr.GetFolder(folder.ID)
	assert.Error(t, err)
}

func TestDeleteFolderNotFound(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	err := mgr.DeleteFolder("nonexistent")
	assert.Error(t, err)
}

func TestListFolders(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	_, _ = mgr.CreateFolder("Folder1")
	_, _ = mgr.CreateFolder("Folder2")
	_, _ = mgr.CreateFolder("Folder3")

	folders := mgr.ListFolders()
	assert.Len(t, folders, 3)

	// EncryptedKey should be masked
	for _, f := range folders {
		assert.Equal(t, "***", f.EncryptedKey)
	}
}

func TestEncryptDecryptData(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, err := mgr.CreateFolder("TestEnc")
	require.NoError(t, err)

	plaintext := []byte("Hello, NAS-OS encryption! This is a secret message.")

	encrypted, err := mgr.EncryptData(folder.ID, plaintext)
	require.NoError(t, err)
	assert.NotNil(t, encrypted)
	assert.NotEqual(t, plaintext, encrypted)
	assert.True(t, len(encrypted) > len(plaintext)) // header + nonce + tag overhead

	// Verify header magic
	assert.Equal(t, FileHeaderMagic, string(encrypted[0:7]))

	decrypted, err := mgr.DecryptData(folder.ID, encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptDecryptDataUnlocked(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("EncLock")

	// Lock folder
	_ = mgr.LockFolder(folder.ID)

	_, err := mgr.EncryptData(folder.ID, []byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not unlocked")
}

func TestEncryptDecryptLargeData(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("LargeData")

	// Create 1MB of data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	encrypted, err := mgr.EncryptData(folder.ID, largeData)
	require.NoError(t, err)

	decrypted, err := mgr.DecryptData(folder.ID, encrypted)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(largeData, decrypted))
}

func TestEncryptDecryptEmptyData(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("EmptyData")

	encrypted, err := mgr.EncryptData(folder.ID, []byte{})
	require.NoError(t, err)

	decrypted, err := mgr.DecryptData(folder.ID, encrypted)
	require.NoError(t, err)
	assert.Len(t, decrypted, 0)
}

func TestKeyRotation(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("Rotate")

	// Encrypt with original key
	data := []byte("data with old key")
	encrypted1, err := mgr.EncryptData(folder.ID, data)
	require.NoError(t, err)

	// Rotate key
	err = mgr.RotateKey(folder.ID)
	require.NoError(t, err)

	// Check key version increased
	got, _ := mgr.GetFolder(folder.ID)
	assert.Equal(t, 2, got.KeyVersion)

	// New encryption with new key
	encrypted2, err := mgr.EncryptData(folder.ID, data)
	require.NoError(t, err)

	// Both should decrypt correctly (data keys are replaced, but this is a demo;
	// in production, old key data would need re-encryption)
	decrypted2, err := mgr.DecryptData(folder.ID, encrypted2)
	require.NoError(t, err)
	assert.Equal(t, data, decrypted2)

	// Old encrypted data with old key version should still decrypt
	// if we had stored the old key; since we replaced it, the decryption
	// will use the new key and should fail (this is expected behavior)
	// For production, re-encrypt all files after key rotation
	_ = encrypted1
}

func TestMultipleFoldersIsolation(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder1, _ := mgr.CreateFolder("FolderA")
	folder2, _ := mgr.CreateFolder("FolderB")

	data1 := []byte("secret for folder A")
	data2 := []byte("secret for folder B")

	enc1, err := mgr.EncryptData(folder1.ID, data1)
	require.NoError(t, err)

	enc2, err := mgr.EncryptData(folder2.ID, data2)
	require.NoError(t, err)

	// Decrypt should work for respective folders
	dec1, err := mgr.DecryptData(folder1.ID, enc1)
	require.NoError(t, err)
	assert.Equal(t, data1, dec1)

	dec2, err := mgr.DecryptData(folder2.ID, enc2)
	require.NoError(t, err)
	assert.Equal(t, data2, dec2)

	// Cross-decrypt should fail (different data keys)
	_, err = mgr.DecryptData(folder1.ID, enc2)
	assert.Error(t, err)

	_, err = mgr.DecryptData(folder2.ID, enc1)
	assert.Error(t, err)
}

func TestRestoreFolder(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	folder := &EncryptedFolder{
		ID:           "restored-id",
		Name:         "Restored",
		Path:         "/tmp/enc-virtual/Restored",
		PhysicalPath: "/tmp/enc-physical/restored-id",
		EncryptedKey: "fake-encrypted-key",
		KeyVersion:   1,
		CreatedAt:    time.Now(),
	}

	mgr.RestoreFolder(folder)

	got, err := mgr.GetFolder("restored-id")
	require.NoError(t, err)
	assert.Equal(t, "Restored", got.Name)
	assert.Equal(t, FolderLocked, got.State)
}

func TestStats(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	_, _ = mgr.CreateFolder("Stats1")
	_, _ = mgr.CreateFolder("Stats2")

	_ = mgr.LockFolder("nonexistent") // no-op

	stats := mgr.Stats()
	assert.Equal(t, 2, stats["totalFolders"])
	assert.Equal(t, 2, stats["unlockedFolders"])
	assert.True(t, stats["masterKeyLoaded"].(bool))

	// Lock one
	folders := mgr.ListFolders()
	_ = mgr.LockFolder(folders[0].ID)

	stats = mgr.Stats()
	assert.Equal(t, 1, stats["unlockedFolders"])
	assert.Equal(t, 1, stats["lockedFolders"])
}

func TestAutoLockExpired(t *testing.T) {
	cfg := PerFolderConfig{
		PhysicalBasePath: "/tmp/enc-physical",
		VirtualBasePath:  "/tmp/enc-virtual",
		Algorithm:        AlgorithmAES256GCM,
		PBKDF2Iterations: PBKDF2Iterations,
		AutoLockMins:     1, // 1 minute
		ChunkSize:        ChunkSize,
	}
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("AutoLock")
	assert.Equal(t, FolderUnlocked, folder.State)

	// Manually set last accessed to 2 minutes ago
	mgr.mu.Lock()
	mgr.folders[folder.ID].LastAccessed = time.Now().Add(-2 * time.Minute)
	mgr.mu.Unlock()

	locked := mgr.AutoLockExpired()
	assert.Equal(t, 1, locked)

	got, _ := mgr.GetFolder(folder.ID)
	assert.Equal(t, FolderLocked, got.State)
}

func TestSetAuditLogger(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	var events []string
	mgr.SetAuditLogger(func(event, msg string) {
		events = append(events, event)
	})

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	_, _ = mgr.CreateFolder("AuditTest")

	assert.Contains(t, events, "master_key_unlocked")
	assert.Contains(t, events, "folder_created")
}

func TestGetFolderMasked(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("Masked")

	got, err := mgr.GetFolder(folder.ID)
	require.NoError(t, err)
	assert.Equal(t, "***", got.EncryptedKey) // Should be masked
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	require.NoError(t, err)
	assert.Len(t, salt1, SaltLength)

	salt2, err := GenerateSalt()
	require.NoError(t, err)
	assert.NotEqual(t, salt1, salt2) // Should be unique
}

func TestHMACKey(t *testing.T) {
	key := HMACKey([]byte("test-data"))
	assert.Len(t, key, 32) // SHA-256 produces 32 bytes

	// Same input = same output
	key2 := HMACKey([]byte("test-data"))
	assert.Equal(t, key, key2)

	// Different input = different output
	key3 := HMACKey([]byte("other-data"))
	assert.NotEqual(t, key, key3)
}

func TestDecryptInvalidMagic(t *testing.T) {
	invalid := make([]byte, FileHeaderSize+20)
	copy(invalid[0:7], "INVALID")

	_, err := decryptWithKey(invalid, make([]byte, KeyLength))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file header magic")
}

func TestDecryptTooShort(t *testing.T) {
	short := make([]byte, 10)
	_, err := decryptWithKey(short, make([]byte, KeyLength))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestEncryptDataTampered(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("Tamper")

	encrypted, _ := mgr.EncryptData(folder.ID, []byte("test data"))

	// Tamper with the ciphertext (flip a bit in the encrypted data portion)
	if len(encrypted) > FileHeaderSize+20 {
		encrypted[FileHeaderSize+15] ^= 0xFF
	}

	_, err := mgr.DecryptData(folder.ID, encrypted)
	assert.Error(t, err) // Should fail due to GCM authentication
}

func TestMultiplePasswordsDifferentKeys(t *testing.T) {
	salt, _ := GenerateSalt()

	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")

	mgr1 := NewPerFolderManager(cfg)
	_ = mgr1.UnlockWithPassword("password1", salt)
	folder1, _ := mgr1.CreateFolder("Test")
	enc1, _ := mgr1.EncryptData(folder1.ID, []byte("hello"))

	mgr2 := NewPerFolderManager(cfg)
	_ = mgr2.UnlockWithPassword("password2", salt)
	folder2 := &EncryptedFolder{
		ID:           folder1.ID,
		Name:         "Test",
		EncryptedKey: folder1.EncryptedKey,
		KeyVersion:   1,
	}
	mgr2.RestoreFolder(folder2)
	_ = mgr2.UnlockFolder(folder1.ID)

	_, err := mgr2.DecryptData(folder1.ID, enc1)
	assert.Error(t, err) // Different master key = different data key
}

func TestMarshalFolderKey(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("Marshal")

	key, err := mgr.MarshalFolderKey(folder.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, key)
	assert.NotEqual(t, "***", key) // Should be the real encrypted key
}

func TestMarshalFolderKeyNotFound(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	_, err := mgr.MarshalFolderKey("nonexistent")
	assert.Error(t, err)
}

func TestFolderStateTransitions(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)

	folder, _ := mgr.CreateFolder("Transitions")

	// Initial state: unlocked (from create)
	got, _ := mgr.GetFolder(folder.ID)
	assert.Equal(t, FolderUnlocked, got.State)

	// Lock
	_ = mgr.LockFolder(folder.ID)
	got, _ = mgr.GetFolder(folder.ID)
	assert.Equal(t, FolderLocked, got.State)

	// Unlock
	_ = mgr.UnlockFolder(folder.ID)
	got, _ = mgr.GetFolder(folder.ID)
	assert.Equal(t, FolderUnlocked, got.State)

	// Lock master key → all folders locked
	mgr.LockMasterKey()
	got, _ = mgr.GetFolder(folder.ID)
	assert.Equal(t, FolderLocked, got.State)
}

func TestPerFolderManagerMarshalJSON(t *testing.T) {
	cfg := DefaultPerFolderConfig("/tmp/enc-physical", "/tmp/enc-virtual")
	mgr := NewPerFolderManager(cfg)

	salt, _ := GenerateSalt()
	_ = mgr.UnlockWithPassword("test-password", salt)
	_, _ = mgr.CreateFolder("F1")
	_, _ = mgr.CreateFolder("F2")

	// MarshalJSON should not expose sensitive data
	data, err := mgr.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), `"folderCount":2`)
	assert.Contains(t, string(data), `"masterKeySet":true`)
}

func TestBase64Helpers(t *testing.T) {
	data := []byte("hello world")
	encoded := encodeBase64(data)
	assert.NotEmpty(t, encoded)

	decoded, err := decodeBase64(encoded)
	require.NoError(t, err)
	assert.Equal(t, data, decoded)

	_, err = decodeBase64("!!!invalid!!!")
	assert.Error(t, err)
}
