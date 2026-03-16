package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultArchiveConfig(t *testing.T) {
	config := DefaultArchiveConfig()

	assert.Equal(t, "./archives", config.StoragePath)
	assert.Equal(t, 365, config.RetentionDays)
	assert.Equal(t, 30, config.AutoArchiveDays)
	assert.True(t, config.CompressArchive)
	assert.False(t, config.IncludeAttachments)
	assert.Equal(t, int64(100*1024*1024), config.MaxArchiveSize)
}

func TestNewArchiveManager(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "archive_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)
	assert.NotNil(t, am)
	assert.NotNil(t, am.archives)
	assert.Equal(t, config.StoragePath, am.config.StoragePath)
}

func TestArchiveManager_GetArchive_NotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	archive, err := am.GetArchive("non-existent-id")
	assert.Nil(t, archive)
	assert.Equal(t, ErrArchiveNotFound, err)
}

func TestArchiveManager_ListArchives_Empty(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	archives := am.ListArchives("", 10, 0)
	assert.Empty(t, archives)
}

func TestArchiveManager_DeleteArchive_NotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	err := am.DeleteArchive("non-existent-id")
	assert.Equal(t, ErrArchiveNotFound, err)
}

func TestArchiveManager_ExtendArchiveRetention_NotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	err := am.ExtendArchiveRetention("non-existent-id", 30)
	assert.Equal(t, ErrArchiveNotFound, err)
}

func TestArchiveManager_GetArchiveStats_Empty(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	stats := am.GetArchiveStats()
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, int64(0), stats.TotalSize)
	assert.Equal(t, 0, stats.ActiveArchives)
}

func TestArchiveManager_CleanupExpiredArchives_Empty(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	deleted, err := am.CleanupExpiredArchives()
	assert.NoError(t, err)
	assert.Empty(t, deleted)
}

func TestArchiveManager_ExportArchive_NotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	data, err := am.ExportArchive("non-existent-id")
	assert.Nil(t, data)
	assert.Equal(t, ErrArchiveNotFound, err)
}

func TestArchiveManager_GetArchivesByProject_Empty(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	archives := am.GetArchivesByProject("project-123")
	assert.Empty(t, archives)
}

func TestArchiveManager_AutoArchive_Disabled(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")
	config.AutoArchiveDays = 0 // 禁用自动归档

	am := NewArchiveManager(nil, config)

	archived, err := am.AutoArchive()
	assert.NoError(t, err)
	assert.Empty(t, archived)
}

func TestArchiveManager_SaveAndLoadArchives(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	// 添加一个归档记录
	now := time.Now()
	am.archives["test-id"] = &ProjectArchive{
		ID:          "test-id",
		ProjectID:   "project-123",
		ProjectName: "Test Project",
		ArchivePath: "/tmp/test.json",
		ArchiveSize: 1024,
		Status:      ArchiveStatusArchived,
		ArchivedAt:  now,
		ArchivedBy:  "admin",
	}

	// 保存
	indexPath := filepath.Join(tmpDir, "archives.json")
	err := am.SaveArchives(indexPath)
	assert.NoError(t, err)

	// 验证文件存在
	_, err = os.Stat(indexPath)
	assert.NoError(t, err)

	// 创建新管理器并加载
	am2 := NewArchiveManager(nil, config)
	err = am2.LoadArchives(indexPath)
	assert.NoError(t, err)

	// 验证数据
	archive, err := am2.GetArchive("test-id")
	assert.NoError(t, err)
	assert.Equal(t, "test-id", archive.ID)
	assert.Equal(t, "project-123", archive.ProjectID)
	assert.Equal(t, "Test Project", archive.ProjectName)
}

func TestArchiveManager_LoadArchives_NonExistent(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	// 加载不存在的文件应该返回 nil
	err := am.LoadArchives(filepath.Join(tmpDir, "non-existent.json"))
	assert.NoError(t, err)
}

func TestArchiveManager_sortArchives(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	now := time.Now()
	archives := []*ProjectArchive{
		{ID: "1", ArchivedAt: now.Add(-2 * time.Hour)},
		{ID: "2", ArchivedAt: now},
		{ID: "3", ArchivedAt: now.Add(-1 * time.Hour)},
	}

	am.sortArchives(archives)

	// 应该按时间倒序排列
	assert.Equal(t, "2", archives[0].ID)
	assert.Equal(t, "3", archives[1].ID)
	assert.Equal(t, "1", archives[2].ID)
}

func TestArchiveManager_ExtendArchiveRetention(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	// 添加一个归档记录
	expiry := time.Now().Add(30 * 24 * time.Hour)
	am.archives["test-id"] = &ProjectArchive{
		ID:        "test-id",
		ProjectID: "project-123",
		Status:    ArchiveStatusArchived,
		ExpiresAt: &expiry,
	}

	// 延长保留期
	err := am.ExtendArchiveRetention("test-id", 30)
	assert.NoError(t, err)

	// 验证过期时间延长
	archive, _ := am.GetArchive("test-id")
	assert.NotNil(t, archive.ExpiresAt)
}

func TestArchiveManager_ListArchives_WithStatus(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	now := time.Now()
	am.archives["1"] = &ProjectArchive{ID: "1", Status: ArchiveStatusArchived, ArchivedAt: now}
	am.archives["2"] = &ProjectArchive{ID: "2", Status: ArchiveStatusActive, ArchivedAt: now}
	am.archives["3"] = &ProjectArchive{ID: "3", Status: ArchiveStatusArchived, ArchivedAt: now}

	// 只查询已归档
	archives := am.ListArchives(ArchiveStatusArchived, 10, 0)
	assert.Len(t, archives, 2)

	// 查询全部
	archives = am.ListArchives("", 10, 0)
	assert.Len(t, archives, 3)
}

func TestArchiveManager_GetArchiveStats(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	now := time.Now()
	am.archives["1"] = &ProjectArchive{ID: "1", Status: ArchiveStatusArchived, ArchivedAt: now, ArchiveSize: 1000}
	am.archives["2"] = &ProjectArchive{ID: "2", Status: ArchiveStatusActive, ArchivedAt: now, ArchiveSize: 2000}
	am.archives["3"] = &ProjectArchive{ID: "3", Status: ArchiveStatusArchived, ArchivedAt: now, ArchiveSize: 3000}

	stats := am.GetArchiveStats()
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, int64(6000), stats.TotalSize)
	assert.Equal(t, 2, stats.ActiveArchives)
	assert.Equal(t, 2, stats.ByStatus[ArchiveStatusArchived])
	assert.Equal(t, 1, stats.ByStatus[ArchiveStatusActive])
}

func TestProjectArchive_Fields(t *testing.T) {
	now := time.Now()
	expiry := now.Add(365 * 24 * time.Hour)

	archive := &ProjectArchive{
		ID:          "archive-123",
		ProjectID:   "project-456",
		ProjectName: "Test Project",
		ArchivePath: "/archives/archive-123.json",
		ArchiveSize: 10240,
		Status:      ArchiveStatusArchived,
		ArchivedAt:  now,
		ArchivedBy:  "admin",
		ExpiresAt:   &expiry,
		Metadata: map[string]interface{}{
			"task_count": 10,
		},
	}

	assert.Equal(t, "archive-123", archive.ID)
	assert.Equal(t, "project-456", archive.ProjectID)
	assert.Equal(t, "Test Project", archive.ProjectName)
	assert.Equal(t, ArchiveStatusArchived, archive.Status)
	assert.Equal(t, "admin", archive.ArchivedBy)
	assert.NotNil(t, archive.ExpiresAt)
}

func TestArchiveStatus_Constants(t *testing.T) {
	assert.Equal(t, ArchiveStatus("active"), ArchiveStatusActive)
	assert.Equal(t, ArchiveStatus("archived"), ArchiveStatusArchived)
	assert.Equal(t, ArchiveStatus("deleted"), ArchiveStatusDeleted)
}

func TestArchiveManager_CleanupExpiredArchives(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "archive_test")
	defer os.RemoveAll(tmpDir)

	config := DefaultArchiveConfig()
	config.StoragePath = filepath.Join(tmpDir, "archives")

	am := NewArchiveManager(nil, config)

	// 创建一个已过期的归档
	pastTime := time.Now().Add(-24 * time.Hour)
	am.archives["expired"] = &ProjectArchive{
		ID:        "expired",
		ProjectID: "project-1",
		Status:    ArchiveStatusArchived,
		ExpiresAt: &pastTime,
	}

	// 创建一个未过期的归档
	futureTime := time.Now().Add(24 * time.Hour)
	am.archives["active"] = &ProjectArchive{
		ID:        "active",
		ProjectID: "project-2",
		Status:    ArchiveStatusArchived,
		ExpiresAt: &futureTime,
	}

	deleted, err := am.CleanupExpiredArchives()
	assert.NoError(t, err)
	assert.Len(t, deleted, 1)
	assert.Equal(t, "expired", deleted[0])

	// 验证过期的已删除
	_, err = am.GetArchive("expired")
	assert.Equal(t, ErrArchiveNotFound, err)

	// 验证未过期的仍存在
	_, err = am.GetArchive("active")
	assert.NoError(t, err)
}
