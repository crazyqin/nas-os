package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewIncrementalBackup(t *testing.T) {
	logger := zap.NewNop()

	config := &BackupConfig{
		BackupPath: t.TempDir(),
		ChunkPath:  filepath.Join(t.TempDir(), "chunks"),
	}

	ib := NewIncrementalBackup(config, logger)
	assert.NotNil(t, ib)
	assert.NotNil(t, ib.fileIndex)
	assert.NotNil(t, ib.chunkStore)
}

func TestFileIndex(t *testing.T) {
	fi := NewFileIndex()

	// 测试更新
	fi.Update("/path/to/file", "checksum123", 1024, time.Now())

	// 测试获取
	entry := fi.Get("/path/to/file")
	assert.NotNil(t, entry)
	assert.Equal(t, "checksum123", entry.Checksum)
	assert.Equal(t, int64(1024), entry.Size)

	// 测试不存在的条目
	entry = fi.Get("/nonexistent")
	assert.Nil(t, entry)

	// 测试移除
	fi.Remove("/path/to/file")
	entry = fi.Get("/path/to/file")
	assert.Nil(t, entry)
}

func TestChunkStore(t *testing.T) {
	cs := NewChunkStore(t.TempDir())

	// 测试存储
	data := []byte("test data")
	err := cs.Store("chunk1", data)
	assert.NoError(t, err)

	// 测试获取
	retrieved, exists := cs.Get("chunk1")
	assert.True(t, exists)
	assert.Equal(t, data, retrieved)

	// 测试引用计数（重复存储增加引用）
	err = cs.Store("chunk1", data)
	assert.NoError(t, err)

	// 测试移除（减少引用）
	cs.Remove("chunk1")
	_, exists = cs.Get("chunk1")
	assert.True(t, exists) // 引用还没减到0

	cs.Remove("chunk1")
	_, exists = cs.Get("chunk1")
	assert.False(t, exists) // 引用减到0，已删除
}

func TestChangeDetector(t *testing.T) {
	// 创建测试目录
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	fi := NewFileIndex()
	cd := NewChangeDetector(fi)

	// 首次检测
	added, modified, deleted, err := cd.DetectChanges(context.Background(), dir)
	assert.NoError(t, err)
	assert.Len(t, added, 1)
	assert.Len(t, modified, 0)
	assert.Len(t, deleted, 0)

	// 更新索引
	fi.Update(added[0], "checksum", 12, time.Now())

	// 再次检测（无变化）
	added, modified, deleted, err = cd.DetectChanges(context.Background(), dir)
	assert.NoError(t, err)
	assert.Len(t, added, 0)
	assert.Len(t, modified, 0)
	assert.Len(t, deleted, 0)
}

func TestEncryptionManager_GenerateKey(t *testing.T) {
	logger := zap.NewNop()

	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       t.TempDir(),
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)

	key, err := em.GenerateKey("test-passphrase")
	assert.NoError(t, err)
	assert.NotEmpty(t, key.ID)
	assert.Len(t, key.Key, 32) // AES-256
	assert.Len(t, key.Salt, 16)
}

func TestEncryptionManager_EncryptDecrypt(t *testing.T) {
	logger := zap.NewNop()

	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       t.TempDir(),
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)
	key, _ := em.GenerateKey("test-passphrase")

	plaintext := []byte("hello, world!")

	// 加密
	ciphertext, err := em.Encrypt(plaintext, key.ID)
	assert.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	// 解密
	decrypted, err := em.Decrypt(ciphertext, key.ID)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptionManager_KeyRotation(t *testing.T) {
	logger := zap.NewNop()

	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       t.TempDir(),
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)

	oldKey, _ := em.GenerateKey("old-passphrase")

	// 轮换密钥
	newKey, err := em.RotateKey(oldKey.ID, "new-passphrase")
	assert.NoError(t, err)
	assert.NotEqual(t, oldKey.ID, newKey.ID)

	// 检查两个密钥都可用
	keys := em.ListKeys()
	assert.Len(t, keys, 2)
}

func TestVerificationManager_VerifySnapshot(t *testing.T) {
	logger := zap.NewNop()

	// 创建测试快照
	config := &BackupConfig{
		BackupPath: t.TempDir(),
		ChunkPath:  filepath.Join(t.TempDir(), "chunks"),
	}

	ib := NewIncrementalBackup(config, logger)

	// 创建测试快照
	snapshot := &Snapshot{
		ID:        "test-snapshot",
		CreatedAt: time.Now(),
		Type:      SnapshotTypeFull,
		Files:     make(map[string]FileInfo),
		Chunks:    make([]string, 0),
		Status:    SnapshotStatusCompleted,
	}
	ib.mu.Lock()
	ib.snapshots["test-snapshot"] = snapshot
	ib.mu.Unlock()

	// 创建验证配置
	verifyConfig := &VerificationConfig{
		VerifyChecksum:  true,
		VerifyStructure: true,
		SampleRate:      1.0,
		MaxFiles:        0,
	}

	vm := NewVerificationManager(verifyConfig, ib, nil, logger)

	result, err := vm.VerifySnapshot(context.Background(), "test-snapshot")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-snapshot", result.SnapshotID)
}

func TestVerificationManager_QuickVerify(t *testing.T) {
	logger := zap.NewNop()

	config := &BackupConfig{
		BackupPath: t.TempDir(),
		ChunkPath:  filepath.Join(t.TempDir(), "chunks"),
	}

	ib := NewIncrementalBackup(config, logger)

	snapshot := &Snapshot{
		ID:        "test-snapshot",
		CreatedAt: time.Now(),
		Type:      SnapshotTypeFull,
		Files:     make(map[string]FileInfo),
		Chunks:    make([]string, 0),
		Status:    SnapshotStatusCompleted,
	}
	ib.mu.Lock()
	ib.snapshots["test-snapshot"] = snapshot
	ib.mu.Unlock()

	verifyConfig := &VerificationConfig{
		SampleRate: 1.0,
	}

	vm := NewVerificationManager(verifyConfig, ib, nil, logger)

	result, err := vm.QuickVerify(context.Background(), "test-snapshot")
	assert.NoError(t, err)
	assert.Equal(t, "test-snapshot", result.SnapshotID)
}

func TestConfigManager(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "backup-config.json")
	cm := NewConfigManager(configPath)

	// 测试默认配置
	config := cm.Get()
	assert.NotNil(t, config)
	assert.Equal(t, "/var/lib/nas-os/backups", config.BackupPath)

	// 测试更新
	newConfig := DefaultConfig()
	newConfig.BackupPath = "/custom/path"
	newConfig.RetentionDays = 60

	err := cm.Update(newConfig)
	assert.NoError(t, err)

	config = cm.Get()
	assert.Equal(t, "/custom/path", config.BackupPath)
	assert.Equal(t, 60, config.RetentionDays)
}

func TestRetentionPolicy(t *testing.T) {
	rp := DefaultRetentionPolicy()

	now := time.Now()

	// 今天的快照应该保留
	snapshot := &Snapshot{
		CreatedAt: now,
	}
	assert.True(t, rp.ShouldKeep(snapshot, now))

	// 一年前的快照
	snapshot = &Snapshot{
		CreatedAt: now.AddDate(-1, 0, 0),
	}
	// 默认最大年龄365天，超过的应该不保留
	assert.False(t, rp.ShouldKeep(snapshot, now))
}

func TestPolicyManager(t *testing.T) {
	pm := NewPolicyManager()

	policy := &BackupPolicy{
		Name:     "daily-backup",
		Source:   "/data",
		Schedule: "0 2 * * *",
		Type:     SnapshotTypeInc,
		Enabled:  true,
	}

	// 添加
	pm.Add(policy)

	// 获取
	retrieved := pm.Get("daily-backup")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "/data", retrieved.Source)

	// 列出
	policies := pm.List()
	assert.Len(t, policies, 1)

	// 移除
	pm.Remove("daily-backup")
	retrieved = pm.Get("daily-backup")
	assert.Nil(t, retrieved)
}

func TestIncrementalBackup_CreateSnapshot(t *testing.T) {
	logger := zap.NewNop()

	// 创建测试目录
	sourceDir := t.TempDir()
	destDir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(sourceDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	config := &BackupConfig{
		BackupPath: destDir,
		ChunkPath:  filepath.Join(destDir, "chunks"),
	}

	ib := NewIncrementalBackup(config, logger)

	// 创建完整备份
	snapshot, err := ib.CreateSnapshot(context.Background(), sourceDir, destDir, SnapshotTypeFull)
	assert.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, SnapshotStatusCompleted, snapshot.Status)
	assert.Equal(t, SnapshotTypeFull, snapshot.Type)
}

func TestIncrementalBackup_ListSnapshots(t *testing.T) {
	logger := zap.NewNop()

	config := &BackupConfig{
		BackupPath: t.TempDir(),
		ChunkPath:  filepath.Join(t.TempDir(), "chunks"),
	}

	ib := NewIncrementalBackup(config, logger)

	// 添加一些快照
	ib.mu.Lock()
	ib.snapshots["snap1"] = &Snapshot{ID: "snap1", Status: SnapshotStatusCompleted}
	ib.snapshots["snap2"] = &Snapshot{ID: "snap2", Status: SnapshotStatusCompleted}
	ib.mu.Unlock()

	snapshots := ib.ListSnapshots()
	assert.Len(t, snapshots, 2)
}

func TestIncrementalBackup_DeleteSnapshot(t *testing.T) {
	logger := zap.NewNop()

	config := &BackupConfig{
		BackupPath: t.TempDir(),
		ChunkPath:  filepath.Join(t.TempDir(), "chunks"),
	}

	ib := NewIncrementalBackup(config, logger)

	// 添加快照
	ib.mu.Lock()
	ib.snapshots["snap1"] = &Snapshot{ID: "snap1", Chunks: []string{"chunk1"}}
	ib.mu.Unlock()

	// 删除
	err := ib.DeleteSnapshot("snap1")
	assert.NoError(t, err)

	// 检查已删除
	_, err = ib.GetSnapshot("snap1")
	assert.Error(t, err)
	assert.Equal(t, ErrBackupNotFound, err)
}

func TestEncryptionManager_Stats(t *testing.T) {
	logger := zap.NewNop()

	config := &EncryptionManagerConfig{
		Enabled:       true,
		Algorithm:     "aes-256-gcm",
		KeyDerivation: "pbkdf2",
		KeyPath:       t.TempDir(),
		SaltLength:    16,
		Iterations:    10000,
	}

	em := NewEncryptionManager(config, logger)

	// 生成一些密钥
	em.GenerateKey("pass1")
	em.GenerateKey("pass2")

	stats := em.GetStats()
	assert.True(t, stats.Enabled)
	assert.Equal(t, "aes-256-gcm", stats.Algorithm)
	assert.Equal(t, 2, stats.KeyCount)
}

func TestVerificationStats(t *testing.T) {
	logger := zap.NewNop()

	config := &BackupConfig{
		BackupPath: t.TempDir(),
		ChunkPath:  filepath.Join(t.TempDir(), "chunks"),
	}

	ib := NewIncrementalBackup(config, logger)

	snapshot := &Snapshot{
		ID:        "test-snapshot",
		CreatedAt: time.Now(),
		Type:      SnapshotTypeFull,
		Files:     make(map[string]FileInfo),
		Chunks:    make([]string, 0),
		Status:    SnapshotStatusCompleted,
	}
	ib.mu.Lock()
	ib.snapshots["test-snapshot"] = snapshot
	ib.mu.Unlock()

	verifyConfig := &VerificationConfig{
		SampleRate: 1.0,
	}

	vm := NewVerificationManager(verifyConfig, ib, nil, logger)

	// 执行验证
	vm.QuickVerify(context.Background(), "test-snapshot")

	// 获取统计
	stats := vm.GetStats()
	assert.Equal(t, 1, stats.TotalVerifications)
	assert.Equal(t, 1, stats.PassedCount)
}
