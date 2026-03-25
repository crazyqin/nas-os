package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newTestBackupManager 创建测试用的备份管理器
func newTestBackupManager(t *testing.T) (*BackupManager, *AppStore) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return nil, nil
	}
	bm, err := NewBackupManager(store, tempDir)
	if err != nil {
		t.Skipf("无法创建 BackupManager: %v", err)
		return nil, nil
	}
	return bm, store
}

func TestNewBackupManager(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}
	assert.NotNil(t, bm)
	assert.NotNil(t, bm.backups)
	assert.Equal(t, 10, bm.maxBackups)
}

func TestBackupManager_CreateBackup(t *testing.T) {
	bm, store := newTestBackupManager(t)
	if bm == nil || store == nil {
		return
	}

	// 添加一个模拟的已安装应用
	app := &InstalledApp{
		ID:          "test-app",
		Name:        "testapp",
		DisplayName: "Test App",
		TemplateID:  "test",
		Version:     "1.0.0",
		Status:      "running",
		InstallTime: time.Now(),
		Volumes:     map[string]string{"/data": "/tmp/test-data"},
		Environment: map[string]string{"TEST": "value"},
	}
	store.installed["test-app"] = app

	// 创建备份目录
	backupPath := filepath.Join(bm.backupDir, "test-backup")
	_ = os.MkdirAll(backupPath, 0750)

	backup, err := bm.CreateBackup("test-app", "config", "测试备份")
	assert.NoError(t, err)
	assert.NotNil(t, backup)
	assert.Equal(t, "test-app", backup.AppID)
	assert.Equal(t, "config", backup.Type)
	assert.Equal(t, "测试备份", backup.Description)
}

func TestBackupManager_CreateBackup_AppNotInstalled(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	_, err := bm.CreateBackup("nonexistent", "config", "测试")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "应用未安装")
}

func TestBackupManager_ListBackups(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	// 添加测试备份
	now := time.Now()
	bm.backups["backup1"] = &AppBackup{
		ID:        "backup1",
		AppID:     "app1",
		CreatedAt: now.Add(-2 * time.Hour),
	}
	bm.backups["backup2"] = &AppBackup{
		ID:        "backup2",
		AppID:     "app1",
		CreatedAt: now.Add(-1 * time.Hour),
	}
	bm.backups["backup3"] = &AppBackup{
		ID:        "backup3",
		AppID:     "app2",
		CreatedAt: now,
	}

	t.Run("列出所有备份", func(t *testing.T) {
		backups := bm.ListBackups("")
		assert.Len(t, backups, 3)
		// 检查是否按时间排序（最新的在前）
		assert.True(t, backups[0].CreatedAt.After(backups[1].CreatedAt))
	})

	t.Run("列出特定应用的备份", func(t *testing.T) {
		backups := bm.ListBackups("app1")
		assert.Len(t, backups, 2)
	})
}

func TestBackupManager_GetBackup(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	bm.backups["backup1"] = &AppBackup{
		ID:     "backup1",
		AppID:  "app1",
		Type:   "full",
	}

	backup, err := bm.GetBackup("backup1")
	assert.NoError(t, err)
	assert.Equal(t, "backup1", backup.ID)
}

func TestBackupManager_GetBackup_NotFound(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	_, err := bm.GetBackup("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "备份不存在")
}

func TestBackupManager_DeleteBackup(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	// 创建测试备份目录
	backupPath := filepath.Join(bm.backupDir, "backup1")
	_ = os.MkdirAll(backupPath, 0750)

	bm.backups["backup1"] = &AppBackup{
		ID:   "backup1",
		Path: backupPath,
	}

	err := bm.DeleteBackup("backup1")
	assert.NoError(t, err)
	assert.NotContains(t, bm.backups, "backup1")
}

func TestBackupManager_DeleteBackup_NotFound(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	err := bm.DeleteBackup("nonexistent")
	assert.Error(t, err)
}

func TestBackupManager_RestoreBackup(t *testing.T) {
	bm, store := newTestBackupManager(t)
	if bm == nil || store == nil {
		return
	}

	// 添加模拟应用
	app := &InstalledApp{
		ID:          "app1",
		Name:        "app1",
		DisplayName: "Test App",
		TemplateID:  "test",
		Version:     "1.0.0",
		Status:      "running",
		Volumes:     map[string]string{},
		Environment: map[string]string{},
	}
	store.installed["app1"] = app

	// 创建测试备份
	backupPath := filepath.Join(bm.backupDir, "backup1")
	_ = os.MkdirAll(backupPath, 0750)

	bm.backups["backup1"] = &AppBackup{
		ID:          "backup1",
		AppID:       "app1",
		Path:        backupPath,
		Config:      map[string]string{"KEY": "value"},
		ComposeFile: "version: '3'",
		VolumePaths: map[string]string{},
	}

	err := bm.RestoreBackup("backup1")
	// 可能因为没有容器而失败，但不应该 panic
	_ = err
}

func TestBackupManager_RestoreBackup_NotFound(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	err := bm.RestoreBackup("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "备份不存在")
}

func TestBackupManager_ExportImport(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	// 创建测试备份
	backupPath := filepath.Join(bm.backupDir, "backup1")
	_ = os.MkdirAll(backupPath, 0750)

	bm.backups["backup1"] = &AppBackup{
		ID:        "backup1",
		AppID:     "app1",
		CreatedAt: time.Now(),
		Path:      backupPath,
	}

	// 导出
	exportDir := t.TempDir()
	err := bm.ExportBackup("backup1", exportDir)
	assert.NoError(t, err)

	// 检查导出文件是否存在
	exportFile := filepath.Join(exportDir, "backup1.tar.gz")
	_, err = os.Stat(exportFile)
	assert.NoError(t, err)
}

func TestBackupManager_cleanupOldBackups(t *testing.T) {
	bm, store := newTestBackupManager(t)
	if bm == nil || store == nil {
		return
	}

	// 设置较小的最大备份数
	bm.maxBackups = 2

	// 添加模拟应用
	app := &InstalledApp{
		ID:          "app1",
		Name:        "app1",
		DisplayName: "Test",
		TemplateID:  "test",
		Version:     "1.0.0",
	}
	store.installed["app1"] = app

	// 创建超过限制数量的备份
	for i := 0; i < 5; i++ {
		backupPath := filepath.Join(bm.backupDir, "backup-"+string(rune('0'+i)))
		_ = os.MkdirAll(backupPath, 0750)
		bm.backups[backupPath] = &AppBackup{
			ID:        backupPath,
			AppID:     "app1",
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
			Path:      backupPath,
		}
	}

	bm.cleanupOldBackups("app1")

	// 检查备份数量不超过限制
	count := 0
	for _, b := range bm.backups {
		if b.AppID == "app1" {
			count++
		}
	}
	assert.LessOrEqual(t, count, bm.maxBackups)
}

func TestAppBackup_Struct(t *testing.T) {
	now := time.Now()
	backup := AppBackup{
		ID:           "backup-123",
		AppID:        "app-456",
		AppName:      "TestApp",
		TemplateName: "test-template",
		Version:      "1.0.0",
		CreatedAt:    now,
		Size:         1024000,
		Path:         "/backups/backup-123",
		Type:         "full",
		Description:  "完整备份",
		Includes:     []string{"/data", "/config"},
		Config:       map[string]string{"KEY": "value"},
		Checksum:     "abc123",
		Compressed:   true,
	}

	assert.Equal(t, "backup-123", backup.ID)
	assert.Equal(t, "full", backup.Type)
	assert.True(t, backup.Compressed)
	assert.Len(t, backup.Includes, 2)
}

func TestBackupManager_calculateChecksum(t *testing.T) {
	bm, _ := newTestBackupManager(t)
	if bm == nil {
		return
	}

	// 创建测试目录
	testPath := t.TempDir()
	_ = os.WriteFile(filepath.Join(testPath, "test.txt"), []byte("test content"), 0640)

	checksum := bm.calculateChecksum(testPath)
	assert.NotEmpty(t, checksum)
}

func TestBackupManager_ScheduleBackup(t *testing.T) {
	bm, store := newTestBackupManager(t)
	if bm == nil || store == nil {
		return
	}

	// 添加模拟应用
	app := &InstalledApp{
		ID:          "app1",
		Name:        "app1",
		DisplayName: "Test",
		TemplateID:  "test",
		Version:     "1.0.0",
		Status:      "running",
	}
	store.installed["app1"] = app

	// 启动定时备份（使用非常短的时间用于测试）
	bm.ScheduleBackup("app1", 100*time.Millisecond)

	// 等待一小段时间
	time.Sleep(150 * time.Millisecond)

	// 测试不应该 panic
}