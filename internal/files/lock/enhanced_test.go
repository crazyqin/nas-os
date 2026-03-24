package lock

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 标签管理器测试 ==========

func TestTagManager_CreateTag(t *testing.T) {
	tm := NewTagManager()

	// 测试创建标签
	tag, err := tm.CreateTag("重要", TagTypeUser, TagColorRed, "user1")
	require.NoError(t, err)
	assert.Equal(t, "重要", tag.Name)
	assert.Equal(t, TagTypeUser, tag.Type)
	assert.Equal(t, TagColorRed, tag.Color)
	assert.Equal(t, "user1", tag.CreatedBy)

	// 测试重名检查
	_, err = tm.CreateTag("重要", TagTypeUser, TagColorBlue, "user1")
	assert.Error(t, err)
}

func TestTagManager_AddTagToFile(t *testing.T) {
	tm := NewTagManager()

	// 创建标签
	tag, err := tm.CreateTag("工作", TagTypeCategory, TagColorBlue, "user1")
	require.NoError(t, err)

	// 添加标签到文件
	assoc, err := tm.AddTagToFile("/data/report.doc", "report.doc", tag.ID, "user1", "重要文档")
	require.NoError(t, err)
	assert.Equal(t, "/data/report.doc", assoc.FilePath)
	assert.Equal(t, tag.ID, assoc.TagID)

	// 检查标签是否添加成功
	assocs := tm.GetFileTags("/data/report.doc")
	assert.Len(t, assocs, 1)

	// 测试重复添加
	_, err = tm.AddTagToFile("/data/report.doc", "report.doc", tag.ID, "user1", "")
	assert.Error(t, err)
}

func TestTagManager_ShareTag(t *testing.T) {
	tm := NewTagManager()

	// 创建标签
	tag, err := tm.CreateTag("项目", TagTypeUser, TagColorGreen, "user1")
	require.NoError(t, err)

	// 共享标签
	err = tm.ShareTag(tag.ID, "user1", []string{"user2", "user3"})
	require.NoError(t, err)

	// 检查可见性
	assert.True(t, tag.IsVisibleTo("user1"))
	assert.True(t, tag.IsVisibleTo("user2"))
	assert.True(t, tag.IsVisibleTo("user3"))
	assert.False(t, tag.IsVisibleTo("user4"))
}

func TestTagManager_SearchByTags(t *testing.T) {
	tm := NewTagManager()

	// 创建标签
	tag1, _ := tm.CreateTag("标签1", TagTypeUser, TagColorRed, "user1")
	tag2, _ := tm.CreateTag("标签2", TagTypeUser, TagColorBlue, "user1")

	// 添加标签到文件
	tm.AddTagToFile("/file1.txt", "file1.txt", tag1.ID, "user1", "")
	tm.AddTagToFile("/file2.txt", "file2.txt", tag1.ID, "user1", "")
	tm.AddTagToFile("/file2.txt", "file2.txt", tag2.ID, "user1", "")
	tm.AddTagToFile("/file3.txt", "file3.txt", tag2.ID, "user1", "")

	// 搜索匹配任意标签
	files := tm.SearchByTags([]string{tag1.ID, tag2.ID}, false)
	assert.Len(t, files, 3)

	// 搜索匹配所有标签
	files = tm.SearchByTags([]string{tag1.ID, tag2.ID}, true)
	assert.Len(t, files, 1)
	assert.Contains(t, files, "/file2.txt")
}

// ========== 版本管理器测试 ==========

func TestVersionManager_CreateVersion(t *testing.T) {
	// 创建临时文件
	tmpDir, err := os.MkdirTemp("", "version-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建版本存储目录
	versionDir := filepath.Join(tmpDir, "versions")
	config := DefaultVersionConfig()
	config.StoragePath = versionDir

	vm, err := NewVersionManager(config)
	require.NoError(t, err)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// 创建版本
	ctx := context.Background()
	version, err := vm.CreateVersion(ctx, testFile, "user1", "User One", VersionTypeManual, "Initial version")
	require.NoError(t, err)
	assert.NotEmpty(t, version.ID)
	assert.Equal(t, 1, version.VersionNumber)
	assert.NotEmpty(t, version.Checksum)
	assert.Equal(t, VersionTypeManual, version.VersionType)

	// 获取版本列表
	versions := vm.GetFileVersions(testFile)
	assert.Len(t, versions, 1)
}

func TestVersionManager_CompareVersions(t *testing.T) {
	// 创建临时文件
	tmpDir, err := os.MkdirTemp("", "version-compare-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	versionDir := filepath.Join(tmpDir, "versions")
	config := DefaultVersionConfig()
	config.StoragePath = versionDir

	vm, err := NewVersionManager(config)
	require.NoError(t, err)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")

	// 创建第一个版本
	os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)
	ctx := context.Background()
	v1, _ := vm.CreateVersion(ctx, testFile, "user1", "User", VersionTypeManual, "v1")

	// 修改文件并创建第二个版本
	os.WriteFile(testFile, []byte("line1\nline2 modified\nline3\nline4\n"), 0644)
	v2, _ := vm.CreateVersion(ctx, testFile, "user1", "User", VersionTypeManual, "v2")

	// 比较版本
	diff, err := vm.CompareVersions(ctx, v1.ID, v2.ID)
	require.NoError(t, err)
	assert.Equal(t, DiffTypeText, diff.DiffType)
	assert.NotEmpty(t, diff.Changes)
}

func TestVersionManager_RestoreVersion(t *testing.T) {
	// 创建临时文件
	tmpDir, err := os.MkdirTemp("", "version-restore-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	versionDir := filepath.Join(tmpDir, "versions")
	config := DefaultVersionConfig()
	config.StoragePath = versionDir

	vm, err := NewVersionManager(config)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.txt")

	// 创建初始版本
	os.WriteFile(testFile, []byte("original content"), 0644)
	ctx := context.Background()
	v1, _ := vm.CreateVersion(ctx, testFile, "user1", "User", VersionTypeManual, "original")

	// 修改文件
	os.WriteFile(testFile, []byte("modified content"), 0644)

	// 恢复到第一个版本
	err = vm.RestoreVersion(ctx, v1.ID, "user1")
	require.NoError(t, err)

	// 验证文件内容
	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "original content", string(data))
}

// ========== 冲突检测器测试 ==========

func TestConflictDetector_DetectLockConflict(t *testing.T) {
	// 创建锁管理器
	lockConfig := DefaultConfig()
	lockManager := NewManager(lockConfig, nil)

	// 创建冲突检测器
	conflictConfig := DefaultConflictDetectorConfig()
	detector := NewConflictDetector(conflictConfig, lockManager, nil)
	defer detector.Close()

	// 获取锁
	_, _, err := lockManager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
		Protocol: "test",
	})
	require.NoError(t, err)

	// 检测冲突
	ctx := context.Background()
	conflict, err := detector.DetectConflict(ctx, &ConflictDetectionRequest{
		FilePath:  "/test/file.txt",
		LockType:  LockTypeExclusive,
		UserID:    "user2",
		UserName:  "User Two",
		Operation: "edit",
	})
	require.NoError(t, err)
	require.NotNil(t, conflict)
	assert.Equal(t, ExtendedConflictExclusiveLock, conflict.Type)
	assert.Len(t, conflict.Participants, 2)
}

func TestConflictDetector_ResolveConflict(t *testing.T) {
	lockConfig := DefaultConfig()
	lockManager := NewManager(lockConfig, nil)

	conflictConfig := DefaultConflictDetectorConfig()
	detector := NewConflictDetector(conflictConfig, lockManager, nil)
	defer detector.Close()

	// 创建冲突
	conflict := detector.CreateConflict(
		ExtendedConflictSharedLock,
		"/test/file.txt",
		[]*ConflictParticipant{
			{UserID: "user1", Role: RoleOwner},
			{UserID: "user2", Role: RoleInitiator},
		},
		"Shared lock conflict",
	)

	// 解决冲突
	err := detector.ResolveConflict(conflict.ID, "admin", "ignore", nil)
	require.NoError(t, err)

	// 检查状态
	resolved, err := detector.GetConflict(conflict.ID)
	require.NoError(t, err)
	assert.Equal(t, ConflictStatusResolved, resolved.Status)
	assert.NotEmpty(t, resolved.ResolvedBy)
}

func TestConflictDetector_ConflictStats(t *testing.T) {
	conflictConfig := DefaultConflictDetectorConfig()
	detector := NewConflictDetector(conflictConfig, nil, nil)
	defer detector.Close()

	// 创建多个冲突
	detector.CreateConflict(ExtendedConflictExclusiveLock, "/file1.txt", nil, "conflict 1")
	detector.CreateConflict(ExtendedConflictSharedLock, "/file2.txt", nil, "conflict 2")
	detector.CreateConflict(ExtendedConflictEditCollision, "/file1.txt", nil, "conflict 3")

	// 获取统计
	stats := detector.GetConflictStats()
	assert.Equal(t, int64(3), stats.TotalConflicts)
	assert.Equal(t, int64(1), stats.ByType[string(ExtendedConflictExclusiveLock)])
	assert.Equal(t, int64(1), stats.ByType[string(ExtendedConflictSharedLock)])
	assert.Equal(t, int64(1), stats.ByType[string(ExtendedConflictEditCollision)])
}

// ========== 集成测试 ==========

func TestIntegration_LockWithTags(t *testing.T) {
	// 创建锁管理器和标签管理器
	lockConfig := DefaultConfig()
	lockManager := NewManager(lockConfig, nil)

	tagManager := NewTagManager()
	integrated := NewTagManagerWithLock(tagManager, lockManager)

	// 创建标签
	tag, _ := tagManager.CreateTag("协作", TagTypeShared, TagColorBlue, "user1")

	// 获取锁
	_, _, err := lockManager.Lock(&LockRequest{
		FilePath: "/collab/doc.txt",
		LockType: LockTypeShared,
		Owner:    "user1",
		Protocol: "test",
	})
	require.NoError(t, err)

	// 安全添加标签
	ctx := context.Background()
	_, err = integrated.AddTagToFileSafe(ctx, "/collab/doc.txt", tag.ID, "user1")
	require.NoError(t, err)

	// 验证标签已添加
	assocs := tagManager.GetFileTags("/collab/doc.txt")
	assert.Len(t, assocs, 1)
}

func TestIntegration_VersionOnLockRelease(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "integration-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建管理器
	lockConfig := DefaultConfig()
	lockManager := NewManager(lockConfig, nil)

	versionDir := filepath.Join(tmpDir, "versions")
	versionConfig := DefaultVersionConfig()
	versionConfig.StoragePath = versionDir
	versionManager, _ := NewVersionManager(versionConfig)

	integrated := NewVersionManagerWithLock(versionManager, lockManager)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	// 获取锁
	lock, _, _ := lockManager.Lock(&LockRequest{
		FilePath: testFile,
		LockType: LockTypeExclusive,
		Owner:    "user1",
		Protocol: "test",
	})
	require.NotNil(t, lock)

	// 释放锁并创建版本
	integrated.OnLockReleased(lock)

	// 验证版本已创建
	versions := versionManager.GetFileVersions(testFile)
	assert.Len(t, versions, 1)
	assert.Equal(t, VersionTypeLocked, versions[0].VersionType)
}

// ========== 基准测试 ==========

func BenchmarkTagManager_AddTagToFile(b *testing.B) {
	tm := NewTagManager()
	tag, _ := tm.CreateTag("test", TagTypeUser, TagColorRed, "user1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.AddTagToFile("/file.txt", "file.txt", tag.ID, "user1", "")
	}
}

func BenchmarkConflictDetector_DetectConflict(b *testing.B) {
	lockConfig := DefaultConfig()
	lockManager := NewManager(lockConfig, nil)

	conflictConfig := DefaultConflictDetectorConfig()
	detector := NewConflictDetector(conflictConfig, lockManager, nil)
	defer detector.Close()

	// 预先创建锁
	lockManager.Lock(&LockRequest{
		FilePath: "/test/file.txt",
		LockType: LockTypeExclusive,
		Owner:    "user1",
		Protocol: "test",
	})

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectConflict(ctx, &ConflictDetectionRequest{
			FilePath:  "/test/file.txt",
			LockType:  LockTypeExclusive,
			UserID:    "user2",
			Operation: "edit",
		})
	}
}

// ========== 预定义标签测试 ==========

func TestTagManager_InitPredefinedTags(t *testing.T) {
	tm := NewTagManager()
	tm.InitPredefinedTags()

	// 验证预定义标签已创建
	tags := tm.ListTags("system", TagTypeSystem)
	assert.GreaterOrEqual(t, len(tags), len(PredefinedTags))

	// 验证标签可见性
	for _, pt := range PredefinedTags {
		tag := tm.FindTagByName(pt.Name, "anyone")
		assert.NotNil(t, tag, "预定义标签 %s 应该存在", pt.Name)
	}
}

// ========== 批量操作测试 ==========

func TestTagManager_BatchOperations(t *testing.T) {
	tm := NewTagManager()

	// 创建标签
	tag1, _ := tm.CreateTag("tag1", TagTypeUser, TagColorRed, "user1")
	tag2, _ := tm.CreateTag("tag2", TagTypeUser, TagColorBlue, "user1")

	// 批量添加
	count, err := tm.BatchAddTags(
		[]string{"/file1.txt", "/file2.txt", "/file3.txt"},
		[]string{tag1.ID, tag2.ID},
		"user1",
	)
	require.NoError(t, err)
	assert.Equal(t, 6, count) // 3 files * 2 tags

	// 批量移除
	count, err = tm.BatchRemoveTags(
		[]string{"/file1.txt", "/file2.txt"},
		[]string{tag1.ID},
		"user1",
	)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// ========== 版本过滤器测试 ==========

func TestVersionManager_ListVersionsWithFilter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "version-filter-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	versionDir := filepath.Join(tmpDir, "versions")
	config := DefaultVersionConfig()
	config.StoragePath = versionDir

	vm, _ := NewVersionManager(config)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	ctx := context.Background()
	vm.CreateVersion(ctx, testFile, "user1", "User 1", VersionTypeManual, "v1")
	vm.CreateVersion(ctx, testFile, "user2", "User 2", VersionTypeAuto, "v2")
	vm.CreateVersion(ctx, testFile, "user1", "User 1", VersionTypeCheckpoint, "v3")

	// 按类型过滤
	versions := vm.ListVersions(&VersionFilter{
		FilePath:    testFile,
		VersionType: VersionTypeManual,
	})
	assert.Len(t, versions, 1)
	assert.Equal(t, VersionTypeManual, versions[0].VersionType)

	// 按创建者过滤
	versions = vm.ListVersions(&VersionFilter{
		FilePath:  testFile,
		CreatedBy: "user1",
	})
	assert.Len(t, versions, 2)
}

// ========== 冲突升级测试 ==========

func TestConflictDetector_EscalateConflict(t *testing.T) {
	conflictConfig := DefaultConflictDetectorConfig()
	conflictConfig.NotificationEnabled = false // 禁用通知以简化测试
	detector := NewConflictDetector(conflictConfig, nil, nil)
	defer detector.Close()

	// 创建冲突
	conflict := detector.CreateConflict(
		ExtendedConflictEditCollision,
		"/shared/doc.txt",
		[]*ConflictParticipant{
			{UserID: "user1", Role: RoleOwner},
			{UserID: "user2", Role: RoleInitiator},
		},
		"Edit collision detected",
	)

	// 升级冲突
	err := detector.EscalateConflict(conflict.ID, "admin", "长时间未解决")
	require.NoError(t, err)

	// 验证状态
	updated, _ := detector.GetConflict(conflict.ID)
	assert.Equal(t, ConflictStatusEscalated, updated.Status)
	assert.Equal(t, SeverityCritical, updated.Severity)
}
