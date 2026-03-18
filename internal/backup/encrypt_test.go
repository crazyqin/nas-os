package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestKeyStore(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "keystore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ks := NewKeyStore(tempDir)

	// 测试存储密钥
	key := &EncryptionKey{
		ID:        "test-key-1",
		Key:       []byte("this-is-a-32-byte-test-key-12345"),
		Salt:      []byte("test-salt-16byte"),
		CreatedAt: "2024-01-01T00:00:00Z",
		Algorithm: "aes-256-gcm",
	}

	if err := ks.Store(key); err != nil {
		t.Fatalf("Failed to store key: %v", err)
	}

	// 测试获取密钥
	retrieved, exists := ks.Get("test-key-1")
	if !exists {
		t.Fatal("Key should exist")
	}
	if string(retrieved.Key) != string(key.Key) {
		t.Errorf("Key mismatch: got %s, want %s", retrieved.Key, key.Key)
	}

	// 测试列出密钥
	ids := ks.List()
	if len(ids) != 1 {
		t.Errorf("Expected 1 key, got %d", len(ids))
	}

	// 测试删除密钥
	if err := ks.Delete("test-key-1"); err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	_, exists = ks.Get("test-key-1")
	if exists {
		t.Fatal("Key should be deleted")
	}
}

func TestEncryptionManager(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "encrypt-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()

	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)

	// 测试生成密钥
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	if key.ID == "" {
		t.Error("Key ID should not be empty")
	}
	if len(key.Key) != 32 {
		t.Errorf("Key length should be 32, got %d", len(key.Key))
	}
	if len(key.Salt) != 16 {
		t.Errorf("Salt length should be 16, got %d", len(key.Salt))
	}

	// 测试列出密钥
	keys := em.ListKeys()
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}

	// 测试密钥信息
	info, err := em.GetKeyInfo(key.ID)
	if err != nil {
		t.Fatalf("Failed to get key info: %v", err)
	}
	if info["id"] != key.ID {
		t.Errorf("Key ID mismatch: got %v, want %s", info["id"], key.ID)
	}

	// 测试统计
	stats := em.GetStats()
	if !stats.Enabled {
		t.Error("Encryption should be enabled")
	}
	if stats.KeyCount != 1 {
		t.Errorf("Expected 1 key, got %d", stats.KeyCount)
	}
}

func TestEncryptionDecryptionGCM(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "encrypt-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plaintext := []byte("Hello, World! This is a secret message.")

	// 测试加密
	ciphertext, err := em.Encrypt(plaintext, key.ID)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if len(ciphertext) <= len(plaintext) {
		t.Errorf("Ciphertext should be longer than plaintext")
	}

	// 测试解密
	decrypted, err := em.Decrypt(ciphertext, key.ID)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text mismatch: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptionDecryptionCBC(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "encrypt-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-cbc",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plaintext := []byte("Hello, World! This is a secret message.")

	// 测试加密
	ciphertext, err := em.Encrypt(plaintext, key.ID)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// 测试解密
	decrypted, err := em.Decrypt(ciphertext, key.ID)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text mismatch: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "encrypt-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// 创建测试文件
	plaintext := []byte("This is a test file content.")
	srcFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(srcFile, plaintext, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 加密文件
	encFile := filepath.Join(tempDir, "test.enc")
	if err := em.EncryptFile(srcFile, encFile, key.ID); err != nil {
		t.Fatalf("Failed to encrypt file: %v", err)
	}

	// 解密文件
	decFile := filepath.Join(tempDir, "test.dec")
	if err := em.DecryptFile(encFile, decFile, key.ID); err != nil {
		t.Fatalf("Failed to decrypt file: %v", err)
	}

	// 验证内容
	decrypted, err := os.ReadFile(decFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted content mismatch: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptionStream(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "encrypt-stream-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plaintext := []byte("Stream encryption test data")

	// 测试流加密
	var encryptedBuf bytes.Buffer
	reader := bytes.NewReader(plaintext)
	if err := em.EncryptStream(reader, &encryptedBuf, key.ID); err != nil {
		t.Fatalf("Failed to encrypt stream: %v", err)
	}

	// 测试流解密
	var decryptedBuf bytes.Buffer
	if err := em.DecryptStream(&encryptedBuf, &decryptedBuf, key.ID); err != nil {
		t.Fatalf("Failed to decrypt stream: %v", err)
	}

	if decryptedBuf.String() != string(plaintext) {
		t.Errorf("Decrypted stream mismatch: got %s, want %s", decryptedBuf.String(), plaintext)
	}
}

func TestDeriveKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "derive-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)

	passphrase := "test-passphrase"
	salt := []byte("test-salt-16byte")

	key := em.DeriveKey(passphrase, salt)

	if len(key) != 32 {
		t.Errorf("Derived key length should be 32, got %d", len(key))
	}

	// 相同的密码和盐应该产生相同的密钥
	key2 := em.DeriveKey(passphrase, salt)
	if string(key) != string(key2) {
		t.Error("Same passphrase and salt should produce same key")
	}
}

func TestExportImportKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// 导出密钥
	exported, err := em.ExportKey(key.ID)
	if err != nil {
		t.Fatalf("Failed to export key: %v", err)
	}

	if exported == "" {
		t.Error("Exported key should not be empty")
	}

	// 导入密钥
	imported, err := em.ImportKey(exported, "aes-256-gcm")
	if err != nil {
		t.Fatalf("Failed to import key: %v", err)
	}

	if imported.Algorithm != "aes-256-gcm" {
		t.Errorf("Imported key algorithm mismatch: got %s", imported.Algorithm)
	}
}

func TestDeleteKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "delete-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// 删除密钥
	if err := em.DeleteKey(key.ID); err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// 验证密钥已删除
	_, exists := em.keyStore.Get(key.ID)
	if exists {
		t.Error("Key should be deleted from keystore")
	}

	keys := em.ListKeys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after deletion, got %d", len(keys))
	}
}

func TestEncryptionWithNonExistentKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nonexistent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)

	plaintext := []byte("test data")

	// 使用不存在的密钥加密
	_, err = em.Encrypt(plaintext, "non-existent-key")
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	// 使用不存在的密钥解密
	_, err = em.Decrypt(plaintext, "non-existent-key")
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
}

func TestDecryptInvalidData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "invalid-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, err := em.GenerateKey("test-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// 测试解密无效数据
	_, err = em.Decrypt([]byte("invalid"), key.ID)
	if err == nil {
		t.Error("Should fail to decrypt invalid data")
	}

	// 测试解密过短数据
	_, err = em.Decrypt([]byte("abc"), key.ID)
	if err == nil {
		t.Error("Should fail to decrypt short data")
	}
}

func TestRotateKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rotate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := zap.NewNop()
	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       tempDir,
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	oldKey, err := em.GenerateKey("old-passphrase")
	if err != nil {
		t.Fatalf("Failed to generate old key: %v", err)
	}

	// 轮换密钥
	newKey, err := em.RotateKey(oldKey.ID, "new-passphrase")
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	if newKey.ID == oldKey.ID {
		t.Error("New key ID should be different from old key ID")
	}

	// 验证新密钥存在
	keys := em.ListKeys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys after rotation, got %d", len(keys))
	}
}
