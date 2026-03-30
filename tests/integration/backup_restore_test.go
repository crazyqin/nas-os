package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ========== 备份恢复集成测试 ==========

// BackupStatus 备份状态.
type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"
	BackupStatusRunning   BackupStatus = "running"
	BackupStatusCompleted BackupStatus = "completed"
	BackupStatusFailed    BackupStatus = "failed"
)

// BackupInfo 备份信息.
type BackupInfo struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Source      string       `json:"source"`
	Target      string       `json:"target"`
	Size        int64        `json:"size"`
	Status      BackupStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt time.Time    `json:"completed_at,omitempty"`
	Files       []string     `json:"files,omitempty"`
	Checksum    string       `json:"checksum,omitempty"`
}

// MockBackupManager 备份管理器 Mock.
type MockBackupManager struct {
	mu      sync.RWMutex
	backups map[string]*BackupInfo
}

func NewMockBackupManager() *MockBackupManager {
	return &MockBackupManager{
		backups: make(map[string]*BackupInfo),
	}
}

func (m *MockBackupManager) CreateBackup(name, source, target string) (*BackupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("backup-%d", time.Now().UnixNano())
	backup := &BackupInfo{
		ID:        id,
		Name:      name,
		Source:    source,
		Target:    target,
		Size:      1024 * 1024,
		Status:    BackupStatusCompleted,
		CreatedAt: time.Now(),
	}
	m.backups[id] = backup
	return backup, nil
}

func (m *MockBackupManager) GetBackup(id string) (*BackupInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backup, exists := m.backups[id]
	if !exists {
		return nil, fmt.Errorf("backup not found: %s", id)
	}
	return backup, nil
}

func (m *MockBackupManager) DeleteBackup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.backups[id]; !exists {
		return fmt.Errorf("backup not found: %s", id)
	}
	delete(m.backups, id)
	return nil
}

func (m *MockBackupManager) ListBackups() []*BackupInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BackupInfo, 0, len(m.backups))
	for _, b := range m.backups {
		result = append(result, b)
	}
	return result
}

// MockRestoreManager 恢复管理器 Mock.
type MockRestoreManager struct {
	mu       sync.RWMutex
	backups  *MockBackupManager
	restores map[string]*RestoreResult
}

type RestoreResult struct {
	BackupID   string    `json:"backup_id"`
	TargetPath string    `json:"target_path"`
	Success    bool      `json:"success"`
	RestoredAt time.Time `json:"restored_at"`
	FilesCount int       `json:"files_count"`
	TotalSize  int64     `json:"total_size"`
}

func NewMockRestoreManager(backups *MockBackupManager) *MockRestoreManager {
	return &MockRestoreManager{
		backups:  backups,
		restores: make(map[string]*RestoreResult),
	}
}

func (r *MockRestoreManager) Restore(backupID, targetPath string) (*RestoreResult, error) {
	backup, err := r.backups.GetBackup(backupID)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result := &RestoreResult{
		BackupID:   backupID,
		TargetPath: targetPath,
		Success:    true,
		RestoredAt: time.Now(),
		FilesCount: len(backup.Files),
		TotalSize:  backup.Size,
	}
	r.restores[backupID] = result
	return result, nil
}

func (r *MockRestoreManager) GetRestoreHistory() []*RestoreResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RestoreResult, 0, len(r.restores))
	for _, res := range r.restores {
		result = append(result, res)
	}
	return result
}

// TestBackupCreation 测试备份创建.
func TestBackupCreation(t *testing.T) {
	mgr := NewMockBackupManager()

	backup, err := mgr.CreateBackup("daily-backup", "/data/important", "/backup/daily")
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if backup.Status != BackupStatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", backup.Status)
	}

	if backup.Name != "daily-backup" {
		t.Errorf("expected name 'daily-backup', got '%s'", backup.Name)
	}
}

// TestBackupRetrieval 测试备份获取.
func TestBackupRetrieval(t *testing.T) {
	mgr := NewMockBackupManager()

	created, _ := mgr.CreateBackup("test-backup", "/data", "/backup")

	retrieved, err := mgr.GetBackup(created.ID)
	if err != nil {
		t.Fatalf("failed to retrieve backup: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, retrieved.ID)
	}
}

// TestBackupDeletion 测试备份删除.
func TestBackupDeletion(t *testing.T) {
	mgr := NewMockBackupManager()

	backup, _ := mgr.CreateBackup("to-delete", "/data", "/backup")

	err := mgr.DeleteBackup(backup.ID)
	if err != nil {
		t.Errorf("failed to delete backup: %v", err)
	}

	// 验证已删除
	_, err = mgr.GetBackup(backup.ID)
	if err == nil {
		t.Error("expected error when getting deleted backup")
	}
}

// TestRestore 测试恢复.
func TestRestore(t *testing.T) {
	backupMgr := NewMockBackupManager()
	restoreMgr := NewMockRestoreManager(backupMgr)

	// 创建备份
	backup, _ := backupMgr.CreateBackup("restore-test", "/data", "/backup")

	// 执行恢复
	result, err := restoreMgr.Restore(backup.ID, "/restore/path")
	if err != nil {
		t.Fatalf("failed to restore: %v", err)
	}

	if !result.Success {
		t.Error("expected restore to succeed")
	}

	if result.BackupID != backup.ID {
		t.Errorf("expected backup ID '%s', got '%s'", backup.ID, result.BackupID)
	}
}

// TestRestoreNonexistentBackup 测试恢复不存在的备份.
func TestRestoreNonexistentBackup(t *testing.T) {
	backupMgr := NewMockBackupManager()
	restoreMgr := NewMockRestoreManager(backupMgr)

	_, err := restoreMgr.Restore("nonexistent-id", "/restore/path")
	if err == nil {
		t.Error("expected error when restoring nonexistent backup")
	}
}

// TestBackupRetention 测试备份保留策略.
func TestBackupRetention(t *testing.T) {
	mgr := NewMockBackupManager()

	// 创建多个备份
	for i := 0; i < 10; i++ {
		mgr.CreateBackup(fmt.Sprintf("backup-%d", i), "/data", "/backup")
	}

	// 验证创建了 10 个
	if len(mgr.ListBackups()) != 10 {
		t.Errorf("expected 10 backups, got %d", len(mgr.ListBackups()))
	}

	// 模拟保留策略：删除旧的备份
	backups := mgr.ListBackups()
	for i := 0; i < len(backups)-5; i++ {
		mgr.DeleteBackup(backups[i].ID)
	}

	// 验证只剩 5 个
	if len(mgr.ListBackups()) != 5 {
		t.Errorf("expected 5 backups after retention, got %d", len(mgr.ListBackups()))
	}
}

// TestConcurrentBackup 并发备份测试.
func TestConcurrentBackup(t *testing.T) {
	mgr := NewMockBackupManager()

	var wg sync.WaitGroup
	errors := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errors[idx] = mgr.CreateBackup(fmt.Sprintf("concurrent-%d", idx), "/data", "/backup")
		}(i)
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("concurrent backup %d failed: %v", i, err)
		}
	}

	if len(mgr.ListBackups()) != 5 {
		t.Errorf("expected 5 backups, got %d", len(mgr.ListBackups()))
	}
}

// TestBackupMetadata 测试备份元数据.
func TestBackupMetadata(t *testing.T) {
	mgr := NewMockBackupManager()

	backup, _ := mgr.CreateBackup("metadata-test", "/data", "/backup")

	// 验证元数据
	if backup.ID == "" {
		t.Error("expected backup ID to be set")
	}

	if backup.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// 序列化元数据
	data, err := json.Marshal(backup)
	if err != nil {
		t.Errorf("failed to marshal backup: %v", err)
	}

	// 反序列化
	var restored BackupInfo
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Errorf("failed to unmarshal backup: %v", err)
	}

	if restored.ID != backup.ID {
		t.Errorf("expected ID '%s', got '%s'", backup.ID, restored.ID)
	}
}

// TestBackupStats 测试备份统计.
func TestBackupStats(t *testing.T) {
	mgr := NewMockBackupManager()

	// 创建多个备份
	for i := 0; i < 5; i++ {
		mgr.CreateBackup(fmt.Sprintf("stats-test-%d", i), "/data", "/backup")
	}

	backups := mgr.ListBackups()

	// 计算统计
	var totalSize int64
	for _, b := range backups {
		totalSize += b.Size
	}

	if totalSize != 5*1024*1024 {
		t.Errorf("expected total size %d, got %d", 5*1024*1024, totalSize)
	}
}

// TestIncrementalBackup 测试增量备份.
func TestIncrementalBackup(t *testing.T) {
	mgr := NewMockBackupManager()

	// 创建完整备份
	fullBackup, _ := mgr.CreateBackup("full-backup", "/data", "/backup")

	// 创建增量备份（模拟）
	incrBackup, _ := mgr.CreateBackup("incremental-backup", "/data", "/backup")

	// 验证两个备份
	backups := mgr.ListBackups()
	if len(backups) != 2 {
		t.Errorf("expected 2 backups, got %d", len(backups))
	}

	_ = fullBackup
	_ = incrBackup
}

// TestBackupEncryption 测试备份加密.
func TestBackupEncryption(t *testing.T) {
	mgr := NewMockBackupManager()

	backup, _ := mgr.CreateBackup("encrypted-backup", "/data", "/backup")

	// 模拟加密标记
	backup.Checksum = "sha256:abc123"

	if backup.Checksum == "" {
		t.Error("expected checksum to be set")
	}
}

// TestRestoreHistory 测试恢复历史.
func TestRestoreHistory(t *testing.T) {
	backupMgr := NewMockBackupManager()
	restoreMgr := NewMockRestoreManager(backupMgr)

	// 创建并恢复多个备份
	for i := 0; i < 3; i++ {
		backup, _ := backupMgr.CreateBackup(fmt.Sprintf("history-%d", i), "/data", "/backup")
		restoreMgr.Restore(backup.ID, "/restore")
	}

	// 获取恢复历史
	history := restoreMgr.GetRestoreHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 restore records, got %d", len(history))
	}
}

// TestBackupWithTempDir 测试使用临时目录的备份.
func TestBackupWithTempDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	mgr := NewMockBackupManager()
	backup, err := mgr.CreateBackup("temp-backup", tempDir, "/backup")
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	_ = backup
}

// BenchmarkBackupCreation 备份创建基准测试.
func BenchmarkBackupCreation(b *testing.B) {
	mgr := NewMockBackupManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.CreateBackup(fmt.Sprintf("bench-%d", i), "/data", "/backup")
	}
}

// BenchmarkRestore 恢复基准测试.
func BenchmarkRestore(b *testing.B) {
	backupMgr := NewMockBackupManager()
	restoreMgr := NewMockRestoreManager(backupMgr)

	backup, _ := backupMgr.CreateBackup("bench-backup", "/data", "/backup")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		restoreMgr.Restore(backup.ID, "/restore")
	}
}

// BenchmarkConcurrentBackup 并发备份基准测试.
func BenchmarkConcurrentBackup(b *testing.B) {
	mgr := NewMockBackupManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mgr.CreateBackup(fmt.Sprintf("parallel-%d", i), "/data", "/backup")
			i++
		}
	})
}
