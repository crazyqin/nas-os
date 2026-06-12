// Package webshare 测试文件
package webshare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestManager 创建测试用的 Manager
func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "webshare-test-*")
	require.NoError(t, err)

	config := DefaultWebShareConfig()
	config.RootPath = tmpDir
	config.MaxActiveLinks = 100

	manager := NewManager(config)

	t.Cleanup(func() {
		manager.Stop()
		os.RemoveAll(tmpDir)
	})

	return manager, tmpDir
}

// createTestFile 创建测试文件
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return path
}

// createTestDir 创建测试目录
func createTestDir(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.MkdirAll(path, 0755)
	require.NoError(t, err)

	return path
}

// ==================== Manager 测试 ====================

func TestNewManager(t *testing.T) {
	manager, _ := setupTestManager(t)
	assert.NotNil(t, manager)
	assert.False(t, manager.IsRunning())
}

func TestManagerStartStop(t *testing.T) {
	manager, _ := setupTestManager(t)

	// 启动
	err := manager.Start()
	require.NoError(t, err)
	assert.True(t, manager.IsRunning())

	// 重复启动应该报错
	err = manager.Start()
	assert.Error(t, err)

	// 停止
	err = manager.Stop()
	require.NoError(t, err)
	assert.False(t, manager.IsRunning())

	// 重复停止应该正常
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestManagerStartWithInvalidRoot(t *testing.T) {
	config := DefaultWebShareConfig()
	config.RootPath = "/nonexistent/path/that/does/not/exist"

	manager := NewManager(config)
	err := manager.Start()
	assert.Error(t, err)
}

func TestManagerGetConfig(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	cfg := manager.GetConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, tmpDir, cfg.RootPath)
}

func TestManagerUpdateConfig(t *testing.T) {
	manager, _ := setupTestManager(t)

	newCfg := DefaultWebShareConfig()
	newCfg.MaxFileSize = 20 * 1024 * 1024 * 1024 // 20GB

	err := manager.UpdateConfig(newCfg)
	require.NoError(t, err)

	cfg := manager.GetConfig()
	assert.Equal(t, int64(20*1024*1024*1024), cfg.MaxFileSize)

	// nil config 应该报错
	err = manager.UpdateConfig(nil)
	assert.Error(t, err)
}

// ==================== 目录浏览测试 ====================

func TestListDirectory(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	// 创建测试文件
	createTestFile(t, tmpDir, "file1.txt", "content1")
	createTestFile(t, tmpDir, "file2.txt", "content2")
	createTestDir(t, tmpDir, "subdir")
	createTestFile(t, tmpDir, ".hidden", "hidden content")

	// 列出根目录（不显示隐藏文件）
	listing, err := manager.ListDirectory("", false, "", SortByName, SortAsc)
	require.NoError(t, err)
	assert.Equal(t, 3, listing.TotalCount) // 2 files + 1 dir

	// 列出根目录（显示隐藏文件）
	listing, err = manager.ListDirectory("", true, "", SortByName, SortAsc)
	require.NoError(t, err)
	assert.Equal(t, 4, listing.TotalCount) // 2 files + 1 dir + 1 hidden

	// 测试排序
	listing, err = manager.ListDirectory("", false, "", SortByName, SortDesc)
	require.NoError(t, err)
	assert.True(t, listing.Entries[0].Name >= listing.Entries[1].Name ||
		listing.Entries[0].Type == FileTypeDirectory) // 目录优先
}

func TestListDirectoryWithFilter(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "report.pdf", "pdf content")
	createTestFile(t, tmpDir, "report.txt", "txt content")
	createTestFile(t, tmpDir, "image.jpg", "jpg content")

	// 过滤 .pdf 文件
	listing, err := manager.ListDirectory("", false, ".pdf", SortByName, SortAsc)
	require.NoError(t, err)
	assert.Equal(t, 1, listing.TotalCount)
	assert.Equal(t, "report.pdf", listing.Entries[0].Name)

	// 过滤 "report"
	listing, err = manager.ListDirectory("", false, "report", SortByName, SortAsc)
	require.NoError(t, err)
	assert.Equal(t, 2, listing.TotalCount)
}

func TestListDirectoryPathSafety(t *testing.T) {
	manager, _ := setupTestManager(t)
	manager.Start()

	// 尝试路径遍历攻击
	_, err := manager.ListDirectory("../../etc", false, "", SortByName, SortAsc)
	assert.Error(t, err)

	_, err = manager.ListDirectory("/etc/passwd", false, "", SortByName, SortAsc)
	assert.Error(t, err)
}

func TestListDirectoryWhenNotRunning(t *testing.T) {
	manager, _ := setupTestManager(t)

	_, err := manager.ListDirectory("", false, "", SortByName, SortAsc)
	assert.Error(t, err)
}

// ==================== 文件操作测试 ====================

func TestCreateFolder(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	err := manager.CreateFolder("", "new-folder")
	require.NoError(t, err)

	// 验证文件夹存在
	info, err := os.Stat(filepath.Join(tmpDir, "new-folder"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// 测试非法文件名
	err = manager.CreateFolder("", "invalid/name")
	assert.Error(t, err)

	// 测试空名称
	err = manager.CreateFolder("", "")
	assert.Error(t, err)
}

func TestDeleteEntries(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	// 创建测试文件
	filePath := createTestFile(t, tmpDir, "to-delete.txt", "content")
	dirPath := createTestDir(t, tmpDir, "to-delete-dir")

	// 删除文件
	err := manager.DeleteEntries([]string{"to-delete.txt"})
	require.NoError(t, err)
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))

	// 删除目录
	err = manager.DeleteEntries([]string{"to-delete-dir"})
	require.NoError(t, err)
	_, err = os.Stat(dirPath)
	assert.True(t, os.IsNotExist(err))

	// 删除空列表应该返回错误
	err = manager.DeleteEntries([]string{})
	assert.Error(t, err)
}

func TestRenameEntry(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "old-name.txt", "content")

	err := manager.RenameEntry("old-name.txt", "new-name.txt")
	require.NoError(t, err)

	// 验证新文件存在
	_, err = os.Stat(filepath.Join(tmpDir, "new-name.txt"))
	assert.NoError(t, err)

	// 验证旧文件不存在
	_, err = os.Stat(filepath.Join(tmpDir, "old-name.txt"))
	assert.True(t, os.IsNotExist(err))

	// 测试重命名不存在的文件
	err = manager.RenameEntry("nonexistent.txt", "new.txt")
	assert.Error(t, err)

	// 测试非法文件名
	createTestFile(t, tmpDir, "test.txt", "content")
	err = manager.RenameEntry("test.txt", "invalid/name")
	assert.Error(t, err)
}

func TestCopyEntries(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "source.txt", "content")
	createTestDir(t, tmpDir, "dest-dir")

	err := manager.CopyEntries([]string{"source.txt"}, "dest-dir")
	require.NoError(t, err)

	// 验证副本存在
	_, err = os.Stat(filepath.Join(tmpDir, "dest-dir", "source.txt"))
	assert.NoError(t, err)

	// 验证原文件仍存在
	_, err = os.Stat(filepath.Join(tmpDir, "source.txt"))
	assert.NoError(t, err)
}

func TestMoveEntries(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "source.txt", "content")
	createTestDir(t, tmpDir, "dest-dir")

	err := manager.MoveEntries([]string{"source.txt"}, filepath.Join("dest-dir", "source.txt"))
	require.NoError(t, err)

	// 验证文件在新位置
	_, err = os.Stat(filepath.Join(tmpDir, "dest-dir", "source.txt"))
	assert.NoError(t, err)

	// 验证原位置文件不存在
	_, err = os.Stat(filepath.Join(tmpDir, "source.txt"))
	assert.True(t, os.IsNotExist(err))
}

// ==================== 分享链接测试 ====================

func TestCreateShareLink(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "shared.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "My Shared File",
		Path:       "shared.txt",
		Permission: PermissionDownload,
		CreatedBy:  "user1",
	}

	link, err := manager.CreateShareLink(req)
	require.NoError(t, err)
	assert.NotEmpty(t, link.ID)
	assert.NotEmpty(t, link.Token)
	assert.Equal(t, "My Shared File", link.Name)
	assert.Equal(t, PermissionDownload, link.Permission)
	assert.True(t, link.IsActive)
	assert.Equal(t, "user1", link.CreatedBy)
	assert.NotNil(t, link.ExpiresAt)
}

func TestCreateShareLinkWithPassword(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "secret.txt", "secret content")

	req := CreateShareLinkRequest{
		Name:       "Secret File",
		Path:       "secret.txt",
		Permission: PermissionView,
		Password:   "mypassword",
		CreatedBy:  "user1",
	}

	link, err := manager.CreateShareLink(req)
	require.NoError(t, err)
	assert.NotEmpty(t, link.Password) // 密码应该被哈希
	assert.NotEqual(t, "mypassword", link.Password)
}

func TestGetShareLinkByToken(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "Test",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
	}

	created, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	// 通过令牌获取
	found, err := manager.GetShareLink(created.Token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	// 不存在的令牌
	_, err = manager.GetShareLink("nonexistent-token")
	assert.Error(t, err)
}

func TestGetShareLinkByID(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "Test",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
	}

	created, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	found, err := manager.GetShareLinkByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	// 不存在的 ID
	_, err = manager.GetShareLinkByID("nonexistent")
	assert.Error(t, err)
}

func TestDeleteShareLink(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "Test",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
	}

	link, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	// 删除链接
	err = manager.DeleteShareLink(link.ID)
	require.NoError(t, err)

	// 链接应该不再通过令牌可访问
	_, err = manager.GetShareLink(link.Token)
	assert.Error(t, err)

	// 删除不存在的链接
	err = manager.DeleteShareLink("nonexistent")
	assert.Error(t, err)
}

func TestListShareLinks(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	// 创建多个链接
	for i := 0; i < 3; i++ {
		req := CreateShareLinkRequest{
			Name:       fmt.Sprintf("Test %d", i),
			Path:       "file.txt",
			Permission: PermissionView,
			CreatedBy:  "user1",
		}
		_, err := manager.CreateShareLink(req)
		require.NoError(t, err)
	}

	// 列出用户1的链接
	links := manager.ListShareLinks("user1", false)
	assert.Len(t, links, 3)

	// 列出所有链接
	links = manager.ListShareLinks("", false)
	assert.Len(t, links, 3)

	// 创建另一个用户的链接
	req := CreateShareLinkRequest{
		Name:       "User2 Link",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user2",
	}
	_, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	links = manager.ListShareLinks("user1", false)
	assert.Len(t, links, 3)

	links = manager.ListShareLinks("", false)
	assert.Len(t, links, 4)
}

func TestUpdateShareLink(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "Original Name",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
	}

	link, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	// 更新名称和权限
	updates := map[string]interface{}{
		"name":       "Updated Name",
		"permission": PermissionEdit,
	}

	updated, err := manager.UpdateShareLink(link.ID, updates)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, PermissionEdit, updated.Permission)

	// 更新不存在的链接
	_, err = manager.UpdateShareLink("nonexistent", updates)
	assert.Error(t, err)
}

func TestVerifySharePassword(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "Protected",
		Path:       "file.txt",
		Permission: PermissionView,
		Password:   "secret123",
		CreatedBy:  "user1",
	}

	link, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	// 正确密码
	assert.True(t, manager.VerifySharePassword(link.Token, "secret123"))

	// 错误密码
	assert.False(t, manager.VerifySharePassword(link.Token, "wrongpassword"))

	// 无密码保护的链接
	req2 := CreateShareLinkRequest{
		Name:       "No Password",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
	}
	link2, err := manager.CreateShareLink(req2)
	require.NoError(t, err)
	assert.True(t, manager.VerifySharePassword(link2.Token, ""))
}

func TestMaxActiveLinks(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.config.MaxActiveLinks = 2
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	// 创建2个链接
	for i := 0; i < 2; i++ {
		req := CreateShareLinkRequest{
			Name:       fmt.Sprintf("Link %d", i),
			Path:       "file.txt",
			Permission: PermissionView,
			CreatedBy:  "user1",
		}
		_, err := manager.CreateShareLink(req)
		require.NoError(t, err)
	}

	// 第3个应该失败
	req := CreateShareLinkRequest{
		Name:       "Link 3",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
	}
	_, err := manager.CreateShareLink(req)
	assert.Error(t, err)
}

func TestExpiredShareLink(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.config.DefaultExpiry = 0 // 不自动设置过期时间
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	// 创建一个有正向过期时间的链接
	req := CreateShareLinkRequest{
		Name:       "Will Expire",
		Path:       "file.txt",
		Permission: PermissionView,
		Expiry:     1 * time.Second,
		CreatedBy:  "user1",
	}

	link, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	// 链接刚创建时应该可访问
	_, err = manager.GetShareLink(link.Token)
	assert.NoError(t, err)

	// 手动将过期时间设为过去
	manager.mu.Lock()
	pastTime := time.Now().Add(-1 * time.Hour)
	link.ExpiresAt = &pastTime
	manager.mu.Unlock()

	// 现在应该无法访问
	_, err = manager.GetShareLink(link.Token)
	assert.Error(t, err)
}

// ==================== 访问日志测试 ====================

func TestRecordAccess(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "Test",
		Path:       "file.txt",
		Permission: PermissionDownload,
		CreatedBy:  "user1",
	}

	link, err := manager.CreateShareLink(req)
	require.NoError(t, err)

	// 记录访问
	manager.RecordAccess(link.ID, "view", "file.txt", "192.168.1.1", "Mozilla/5.0", "user2")
	manager.RecordAccess(link.ID, "download", "file.txt", "192.168.1.1", "Mozilla/5.0", "user2")

	// 获取日志
	logs := manager.GetAccessLogs(link.ID, 10)
	assert.Len(t, logs, 2)
	assert.Equal(t, "download", logs[0].Action) // 按时间降序

	// 验证下载计数
	found, _ := manager.GetShareLinkByID(link.ID)
	assert.Equal(t, 1, found.DownloadCount)
	assert.Equal(t, 2, found.AccessCount)
	assert.NotNil(t, found.LastAccessAt)
}

func TestGetAccessLogsFilter(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	req := CreateShareLinkRequest{
		Name:       "Test",
		Path:       "file.txt",
		Permission: PermissionView,
		CreatedBy:  "user1",
	}

	link, _ := manager.CreateShareLink(req)

	// 添加多条日志
	for i := 0; i < 5; i++ {
		manager.RecordAccess(link.ID, "view", "file.txt", "192.168.1.1", "Mozilla/5.0", "user1")
	}

	// 限制数量
	logs := manager.GetAccessLogs(link.ID, 3)
	assert.Len(t, logs, 3)

	// 获取所有
	logs = manager.GetAccessLogs(link.ID, 0)
	assert.Len(t, logs, 5)

	// 按 shareID 过滤
	logs = manager.GetAccessLogs("nonexistent", 10)
	assert.Len(t, logs, 0)
}

// ==================== 快照测试 ====================

func TestListSnapshotsDisabled(t *testing.T) {
	manager, _ := setupTestManager(t)
	manager.config.EnableSnapshots = false
	manager.Start()

	_, err := manager.ListSnapshots("")
	assert.Error(t, err)
}

func TestListSnapshotsWhenNotRunning(t *testing.T) {
	manager, _ := setupTestManager(t)

	_, err := manager.ListSnapshots("")
	assert.Error(t, err)
}

// ==================== 统计测试 ====================

func TestGetStats(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file1.txt", "content1")
	createTestFile(t, tmpDir, "file2.txt", "content2")
	createTestDir(t, tmpDir, "subdir")

	stats := manager.GetStats()
	assert.NotNil(t, stats)
	assert.True(t, stats.SearchEnabled)
	assert.True(t, stats.FIPSEnabled)
	assert.True(t, stats.TotalFiles >= 2)
	assert.True(t, stats.TotalDirs >= 1)
}

func TestGetShareStats(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	// 创建链接
	for i := 0; i < 3; i++ {
		req := CreateShareLinkRequest{
			Name:       fmt.Sprintf("Link %d", i),
			Path:       "file.txt",
			Permission: PermissionDownload,
			CreatedBy:  "user1",
		}
		link, _ := manager.CreateShareLink(req)
		manager.RecordAccess(link.ID, "download", "file.txt", "192.168.1.1", "Mozilla/5.0", "user1")
	}

	stats := manager.GetShareStats()
	assert.Equal(t, 3, stats.TotalLinks)
	assert.Equal(t, 3, stats.ActiveLinks)
	assert.Equal(t, 3, stats.TotalDownloads)
}

// ==================== 文件索引器测试 ====================

func TestNewFileIndexer(t *testing.T) {
	config := DefaultWebShareConfig()
	config.RootPath = t.TempDir()

	indexer := NewFileIndexer(nil, config)
	assert.NotNil(t, indexer)
	assert.False(t, indexer.IsRunning())
	assert.Equal(t, IndexStatusPending, indexer.GetStatus())
}

func TestFileIndexerStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultWebShareConfig()
	config.RootPath = tmpDir

	indexer := NewFileIndexer(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := indexer.Start(ctx)
	require.NoError(t, err)
	assert.True(t, indexer.IsRunning())

	// 重复启动应该报错
	err = indexer.Start(ctx)
	assert.Error(t, err)

	err = indexer.Stop()
	require.NoError(t, err)
	assert.False(t, indexer.IsRunning())
}

func TestFileIndexerIndexFile(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultWebShareConfig()
	config.RootPath = tmpDir

	indexer := NewFileIndexer(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	indexer.Start(ctx)
	defer indexer.Stop()

	// 创建测试文件
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("test content"), 0644)

	// 索引文件
	err := indexer.IndexFile(filePath)
	require.NoError(t, err)

	// 验证索引
	entry, exists := indexer.GetEntry(filePath)
	require.True(t, exists)
	assert.Equal(t, "test.txt", entry.Name)
	assert.Equal(t, int64(12), entry.Size)
	assert.False(t, entry.IsDir)
	assert.NotEmpty(t, entry.Checksum)
}

func TestFileIndexerSearch(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultWebShareConfig()
	config.RootPath = tmpDir

	indexer := NewFileIndexer(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	indexer.Start(ctx)
	defer indexer.Stop()

	// 创建测试文件
	files := []string{"report.pdf", "report.txt", "image.jpg", "data.csv"}
	for _, name := range files {
		filePath := filepath.Join(tmpDir, name)
		os.WriteFile(filePath, []byte("content"), 0644)
		indexer.IndexFile(filePath)
	}

	// 搜索 "report"（可能包含额外条目）
	results := indexer.Search("report", 10)
	assert.GreaterOrEqual(t, len(results), 2, "应至少找到2个report文件")

	// 搜索 "image"
	results = indexer.Search("image", 10)
	assert.Len(t, results, 1)
	assert.Equal(t, "image.jpg", results[0].Name)
}

func TestFileIndexerSearchByPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultWebShareConfig()
	config.RootPath = tmpDir

	indexer := NewFileIndexer(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	indexer.Start(ctx)
	defer indexer.Stop()

	// 创建测试文件
	files := []string{"report.pdf", "report2.pdf", "readme.txt"}
	for _, name := range files {
		filePath := filepath.Join(tmpDir, name)
		os.WriteFile(filePath, []byte("content"), 0644)
		indexer.IndexFile(filePath)
	}

	// 前缀搜索 "rep"
	suggestions := indexer.SearchByPrefix("rep", 10)
	assert.Len(t, suggestions, 2)
}

func TestFileIndexerRemoveEntry(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultWebShareConfig()
	config.RootPath = tmpDir

	indexer := NewFileIndexer(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	indexer.Start(ctx)
	defer indexer.Stop()

	// 创建并索引文件
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("content"), 0644)
	indexer.IndexFile(filePath)

	// 验证存在
	_, exists := indexer.GetEntry(filePath)
	assert.True(t, exists)

	// 移除
	indexer.RemoveEntry(filePath)

	// 验证不存在
	_, exists = indexer.GetEntry(filePath)
	assert.False(t, exists)
}

func TestFileIndexerGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultWebShareConfig()
	config.RootPath = tmpDir

	indexer := NewFileIndexer(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	indexer.Start(ctx)
	defer indexer.Stop()

	// 创建测试文件
	createTestFile(t, tmpDir, "file1.txt", "content1")
	createTestFile(t, tmpDir, "file2.txt", "content2")
	createTestDir(t, tmpDir, "subdir")

	// 手动索引
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			indexer.IndexFile(path)
		}
		return nil
	})

	stats := indexer.GetStats()
	assert.True(t, stats.TotalFiles >= 2)
	assert.True(t, stats.TotalDirs >= 2) // root + subdir
	assert.True(t, stats.TotalSize > 0)
}

func TestFileIndexerListEntries(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultWebShareConfig()
	config.RootPath = tmpDir

	indexer := NewFileIndexer(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	indexer.Start(ctx)
	defer indexer.Stop()

	// 创建并索引文件
	for i := 0; i < 5; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		os.WriteFile(filePath, []byte("content"), 0644)
		indexer.IndexFile(filePath)
	}

	// 列出所有（可能包含根目录本身）
	entries := indexer.ListEntries(0)
	assert.GreaterOrEqual(t, len(entries), 5, "应至少索引 5 个文件")

	// 限制数量
	entries = indexer.ListEntries(3)
	assert.Len(t, entries, 3)
}

// ==================== 并发安全测试 ====================

func TestConcurrentAccess(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	manager.Start()

	createTestFile(t, tmpDir, "file.txt", "content")

	var wg sync.WaitGroup
	numGoroutines := 10

	// 并发创建分享链接
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req := CreateShareLinkRequest{
				Name:       fmt.Sprintf("Link %d", id),
				Path:       "file.txt",
				Permission: PermissionView,
				CreatedBy:  fmt.Sprintf("user%d", id),
			}
			_, err := manager.CreateShareLink(req)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// 并发读取
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.ListShareLinks("", false)
			manager.GetStats()
		}()
	}

	wg.Wait()

	stats := manager.GetShareStats()
	assert.Equal(t, numGoroutines, stats.TotalLinks)
}

// ==================== 辅助函数测试 ====================

func TestValidateFileName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-file.txt", false},
		{"document.pdf", false},
		{"my file (1).txt", false},
		{"", true},
		{"invalid/name.txt", true},
		{"invalid\\name.txt", true},
		{"invalid:name.txt", true},
		{"CON", true},
		{"NUL", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileName(tt.name)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	manager, _ := setupTestManager(t)

	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/file", "path/to/file"},
		{"./path/to/file", "path/to/file"},
		{"path/to/../file", "path/file"},
		{"path/to/./file", "path/to/file"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := manager.sanitizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPathSafe(t *testing.T) {
	manager, tmpDir := setupTestManager(t)

	tests := []struct {
		path    string
		isSafe  bool
	}{
		{filepath.Join(tmpDir, "file.txt"), true},
		{filepath.Join(tmpDir, "subdir/file.txt"), true},
		{"/etc/passwd", false},
		{filepath.Join(tmpDir, "../outside"), false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := manager.isPathSafe(tt.path)
			assert.Equal(t, tt.isSafe, result)
		})
	}
}

func TestGetMimeType(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"file.txt", "text/plain; charset=utf-8"},
		{"image.jpg", "image/jpeg"},
		{"document.pdf", "application/pdf"},
		{"unknown.nonexist", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMimeType(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultWebShareConfig(t *testing.T) {
	cfg := DefaultWebShareConfig()

	assert.Equal(t, ":8443", cfg.ListenAddr)
	assert.Equal(t, "/mnt", cfg.RootPath)
	assert.Equal(t, int64(10*1024*1024*1024), cfg.MaxFileSize)
	assert.Equal(t, 7*24*time.Hour, cfg.DefaultExpiry)
	assert.Equal(t, 1000, cfg.MaxActiveLinks)
	assert.True(t, cfg.EnableFIPS)
	assert.True(t, cfg.EnableSearch)
	assert.False(t, cfg.ShowHiddenFiles)
	assert.True(t, cfg.EnableSnapshots)
}

func TestSortEntries(t *testing.T) {
	entries := []Entry{
		{Name: "file-b.txt", Type: FileTypeFile},
		{Name: "dir-a", Type: FileTypeDirectory},
		{Name: "file-a.txt", Type: FileTypeFile},
		{Name: "dir-b", Type: FileTypeDirectory},
	}

	// 按名称升序
	sortEntries(entries, SortByName, SortAsc)

	// 目录应该在前面
	assert.Equal(t, FileTypeDirectory, entries[0].Type)
	assert.Equal(t, FileTypeDirectory, entries[1].Type)
	assert.Equal(t, FileTypeFile, entries[2].Type)
	assert.Equal(t, FileTypeFile, entries[3].Type)
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.True(t, len(id1) > 10)
}

func TestGenerateSecureToken(t *testing.T) {
	token1, err := generateSecureToken(32)
	require.NoError(t, err)
	assert.NotEmpty(t, token1)
	assert.Len(t, token1, 64) // 32 bytes = 64 hex chars

	token2, err := generateSecureToken(32)
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2)
}

func TestHashPassword(t *testing.T) {
	hash1 := hashPassword("password123")
	hash2 := hashPassword("password123")
	hash3 := hashPassword("different")

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
	assert.Len(t, hash1, 64) // SHA-256 = 64 hex chars
}
