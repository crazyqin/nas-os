package advanced

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

// ========== 类型测试 ==========

func TestBackupConfig_Default(t *testing.T) {
	config := DefaultBackupConfig()

	if config.Retention != 7 {
		t.Errorf("expected retention 7, got %d", config.Retention)
	}
	if !config.Enabled {
		t.Error("expected enabled to be true")
	}
	if !config.Incremental {
		t.Error("expected incremental to be true")
	}
	if config.Compression == nil {
		t.Error("expected compression config")
	}
	if config.Encryption == nil {
		t.Error("expected encryption config")
	}
}

func TestCompressionConfig_Default(t *testing.T) {
	config := DefaultCompressionConfig()

	if config.Algorithm != CompressionGzip {
		t.Errorf("expected gzip algorithm, got %s", config.Algorithm)
	}
	if config.Level != 6 {
		t.Errorf("expected level 6, got %d", config.Level)
	}
}

// ========== 加密器测试 ==========

func TestAES256Encryptor_EncryptDecrypt(t *testing.T) {
	// 生成测试密钥
	key := make([]byte, 32)
	rand.Read(key)

	encryptor, err := NewAES256Encryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	testData := []byte("Hello, NAS-OS Backup!")

	encrypted, err := encryptor.Encrypt(testData)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if bytes.Equal(testData, encrypted) {
		t.Error("encrypted data should differ from original")
	}

	decrypted, err := encryptor.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if !bytes.Equal(testData, decrypted) {
		t.Error("decrypted data should match original")
	}
}

func TestAES256Encryptor_InvalidKey(t *testing.T) {
	_, err := NewAES256Encryptor([]byte("short"))
	if err != ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestAES256Encryptor_DecryptInvalidData(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	encryptor, err := NewAES256Encryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	_, err = encryptor.Decrypt([]byte("invalid"))
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestDeriveKey(t *testing.T) {
	password := "test-password"
	salt := []byte("test-salt")

	key := DeriveKey(password, salt)

	if len(key) != 32 {
		t.Errorf("expected key length 32, got %d", len(key))
	}

	// 相同输入应产生相同输出
	key2 := DeriveKey(password, salt)
	if !bytes.Equal(key, key2) {
		t.Error("same input should produce same key")
	}

	// 不同输入应产生不同输出
	key3 := DeriveKey("different-password", salt)
	if bytes.Equal(key, key3) {
		t.Error("different input should produce different key")
	}
}

func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("expected key length 32, got %d", len(key1))
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate second key: %v", err)
	}

	if bytes.Equal(key1, key2) {
		t.Error("two generated keys should differ")
	}
}

// ========== 压缩器测试 ==========

func TestDefaultCompressor_None(t *testing.T) {
	compressor := NewDefaultCompressor(&CompressionConfig{
		Algorithm: CompressionNone,
	})

	testData := []byte("test data")

	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	if !bytes.Equal(testData, compressed) {
		t.Error("none compression should return original data")
	}

	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompression failed: %v", err)
	}

	if !bytes.Equal(testData, decompressed) {
		t.Error("decompressed data should match original")
	}
}

func TestDefaultCompressor_Gzip(t *testing.T) {
	compressor := NewDefaultCompressor(&CompressionConfig{
		Algorithm: CompressionGzip,
		Level:     6,
	})

	testData := make([]byte, 1000)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	if len(compressed) >= len(testData) {
		t.Error("compressed data should be smaller for repetitive data")
	}

	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompression failed: %v", err)
	}

	if !bytes.Equal(testData, decompressed) {
		t.Error("decompressed data should match original")
	}
}

func TestDefaultCompressor_UnsupportedAlgorithm(t *testing.T) {
	compressor := NewDefaultCompressor(&CompressionConfig{
		Algorithm: CompressionZstd,
	})

	_, err := compressor.Compress([]byte("test"))
	if err == nil {
		t.Error("expected error for unsupported algorithm")
	}
}

func TestSupportedCompressionAlgorithms(t *testing.T) {
	algorithms := SupportedCompressionAlgorithms()

	if len(algorithms) == 0 {
		t.Error("expected at least one compression algorithm")
	}

	found := false
	for _, a := range algorithms {
		if a.Algorithm == CompressionGzip {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected gzip in supported algorithms")
	}
}

func TestEstimateCompressionRatio(t *testing.T) {
	tests := []struct {
		algorithm     CompressionAlgorithm
		expectNonZero bool
	}{
		{CompressionNone, false},
		{CompressionGzip, true},
		{CompressionZstd, true},
	}

	for _, tt := range tests {
		ratio := EstimateCompressionRatio(tt.algorithm)
		if tt.expectNonZero && ratio <= 0 {
			t.Errorf("expected non-zero ratio for %s", tt.algorithm)
		}
		if !tt.expectNonZero && ratio != 1.0 {
			t.Errorf("expected ratio 1.0 for none, got %f", ratio)
		}
	}
}

// ========== 索引测试 ==========

func TestIncrementalIndex(t *testing.T) {
	idx := NewIncrementalIndex()

	state := &FileState{
		Path:     "test.txt",
		Checksum: "abc123",
		Size:     100,
	}

	idx.Update("test.txt", state)

	retrieved, exists := idx.Get("test.txt")
	if !exists {
		t.Error("expected file to exist in index")
	}

	if retrieved.Checksum != "abc123" {
		t.Errorf("expected checksum abc123, got %s", retrieved.Checksum)
	}

	idx.Delete("test.txt")
	retrieved, _ = idx.Get("test.txt")
	if !retrieved.Deleted {
		t.Error("expected file to be marked as deleted")
	}
}

// ========== 校验和测试 ==========

func TestCalculateChecksumBytes(t *testing.T) {
	data := []byte("test data")
	checksum := CalculateChecksumBytes(data)

	if len(checksum) != 64 { // SHA256 hex string
		t.Errorf("expected checksum length 64, got %d", len(checksum))
	}

	// 相同数据应产生相同校验和
	checksum2 := CalculateChecksumBytes(data)
	if checksum != checksum2 {
		t.Error("same data should produce same checksum")
	}

	// 不同数据应产生不同校验和
	checksum3 := CalculateChecksumBytes([]byte("different data"))
	if checksum == checksum3 {
		t.Error("different data should produce different checksum")
	}
}

// ========== 管理器集成测试 ==========

func TestManager_CreateBackup(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试源
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("test content"), 0644)

	// 创建配置
	config := DefaultBackupConfig()
	config.Source = srcDir
	config.Name = "test-backup"

	storagePath := filepath.Join(tmpDir, "storage")
	os.MkdirAll(storagePath, 0755)

	manager, err := NewManager(config, storagePath)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// 创建完整备份
	record, err := manager.CreateBackup(context.Background(), TypeFull)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if record.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", record.Status)
	}

	if record.FileCount != 1 {
		t.Errorf("expected 1 file, got %d", record.FileCount)
	}

	if record.Size == 0 {
		t.Error("expected non-zero size")
	}
}

func TestManager_IncrementalBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content 1"), 0644)

	config := DefaultBackupConfig()
	config.Source = srcDir
	config.Name = "incremental-test"
	config.Verification = false // 禁用验证以简化测试

	storagePath := filepath.Join(tmpDir, "storage")
	os.MkdirAll(storagePath, 0755)

	manager, err := NewManager(config, storagePath)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// 创建完整备份
	fullRecord, err := manager.CreateBackup(context.Background(), TypeFull)
	if err != nil {
		t.Fatalf("failed to create full backup: %v", err)
	}
	t.Logf("Full backup: ID=%s, Status=%s, Verified=%v", fullRecord.ID, fullRecord.Status, fullRecord.Verified)

	// 添加新文件
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content 2"), 0644)

	// 创建增量备份
	record, err := manager.CreateBackup(context.Background(), TypeIncremental)
	if err != nil {
		t.Fatalf("failed to create incremental backup: %v", err)
	}

	if record.Type != TypeIncremental {
		t.Errorf("expected incremental type, got %s", record.Type)
	}

	if record.BaseBackupID == "" {
		t.Error("expected base backup ID")
	}
}

func TestManager_RestoreBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("restore test"), 0644)

	config := DefaultBackupConfig()
	config.Source = srcDir
	config.Name = "restore-test"

	storagePath := filepath.Join(tmpDir, "storage")
	os.MkdirAll(storagePath, 0755)

	manager, err := NewManager(config, storagePath)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// 创建备份
	record, err := manager.CreateBackup(context.Background(), TypeFull)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// 恢复到新位置
	restoreDir := filepath.Join(tmpDir, "restored")
	os.MkdirAll(restoreDir, 0755)

	_, err = manager.RestoreBackup(context.Background(), record.ID, restoreDir, true)
	if err != nil {
		t.Fatalf("failed to restore backup: %v", err)
	}

	// 验证恢复的文件
	restoredContent, err := os.ReadFile(filepath.Join(restoreDir, "test.txt"))
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}

	if string(restoredContent) != "restore test" {
		t.Errorf("unexpected restored content: %s", string(restoredContent))
	}
}

func TestManager_Verification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "verify.txt"), []byte("verify content"), 0644)

	config := DefaultBackupConfig()
	config.Source = srcDir
	config.Name = "verify-test"
	config.Verification = false // 禁用自动验证，改为手动验证

	storagePath := filepath.Join(tmpDir, "storage")
	os.MkdirAll(storagePath, 0755)

	manager, err := NewManager(config, storagePath)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// 创建备份
	record, err := manager.CreateBackup(context.Background(), TypeFull)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if record.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", record.Status)
	}

	// 检查备份文件是否存在
	backupPath := filepath.Join(record.Destination, "verify.txt")
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("backup file not found at %s", backupPath)
	}
}

func TestManager_EncryptedBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "secret.txt"), []byte("secret data"), 0644)

	config := DefaultBackupConfig()
	config.Source = srcDir
	config.Name = "encrypted-test"
	config.Encryption = &EncryptionConfig{
		Enabled:    true,
		Algorithm:  "AES-256-GCM",
		Passphrase: "test-passphrase-12345",
	}

	storagePath := filepath.Join(tmpDir, "storage")
	os.MkdirAll(storagePath, 0755)

	manager, err := NewManager(config, storagePath)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// 创建加密备份
	record, err := manager.CreateBackup(context.Background(), TypeFull)
	if err != nil {
		t.Fatalf("failed to create encrypted backup: %v", err)
	}

	// 验证文件已加密（内容不可读）
	backupFile := filepath.Join(record.Destination, "secret.txt")
	content, _ := os.ReadFile(backupFile)
	if bytes.Contains(content, []byte("secret data")) {
		t.Error("data should be encrypted, not plain text")
	}

	// 恢复并验证
	restoreDir := filepath.Join(tmpDir, "restored")
	os.MkdirAll(restoreDir, 0755)

	_, err = manager.RestoreBackup(context.Background(), record.ID, restoreDir, true)
	if err != nil {
		t.Fatalf("failed to restore encrypted backup: %v", err)
	}

	restoredContent, err := os.ReadFile(filepath.Join(restoreDir, "secret.txt"))
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}

	if string(restoredContent) != "secret data" {
		t.Error("restored content should match original")
	}
}

// ========== 基准测试 ==========

func BenchmarkCompressor_Gzip(b *testing.B) {
	compressor := NewDefaultCompressor(&CompressionConfig{
		Algorithm: CompressionGzip,
		Level:     6,
	})

	testData := make([]byte, 1024*1024) // 1MB
	rand.Read(testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.Compress(testData)
	}
}

func BenchmarkEncryptor_Encrypt(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	encryptor, _ := NewAES256Encryptor(key)
	testData := make([]byte, 1024*1024) // 1MB
	rand.Read(testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encryptor.Encrypt(testData)
	}
}

func BenchmarkChecksum(b *testing.B) {
	testData := make([]byte, 1024*1024) // 1MB
	rand.Read(testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateChecksumBytes(testData)
	}
}
