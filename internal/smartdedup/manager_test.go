// Package smartdedup 提供内容感知的智能文件去重功能
package smartdedup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *Config {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ScanPaths = []string{}
	cfg.MinFileSize = 0 // 允许小文件
	cfg.MaxWorkers = 2
	cfg.MaxMemoryMB = 128
	cfg.DryRun = true // 测试默认用 dry-run
	return cfg
}

func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "smartdedup-test")
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "smartdedup.json")
	mgr, err := NewManager(configPath, testConfig())
	require.NoError(t, err)

	return mgr, tmpDir
}

func TestNewManager(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.Config())
	assert.True(t, mgr.Config().Enabled)
}

func TestNewManager_NilConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "smartdedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "smartdedup.json")
	mgr, err := NewManager(configPath, nil)
	require.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.Config())
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, BackendAuto, cfg.Backend)
	assert.Equal(t, ModeHybrid, cfg.Mode)
	assert.Equal(t, ActionReflink, cfg.Action)
	assert.True(t, cfg.ScheduleEnabled)
	assert.False(t, cfg.RealtimeEnabled)
	assert.True(t, cfg.HashCache)
	assert.True(t, cfg.VerifyAfter)
	assert.Equal(t, 4, cfg.MaxWorkers)
	assert.Equal(t, 512, cfg.MaxMemoryMB)
	assert.Equal(t, 1000, cfg.MaxRefPerFile)
}

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	assert.NoError(t, err)

	// 验证默认值被设置
	assert.Equal(t, BackendAuto, cfg.Backend)
	assert.Equal(t, ModeHybrid, cfg.Mode)
	assert.Equal(t, ActionReflink, cfg.Action)
	assert.Equal(t, 1, cfg.MaxWorkers)
	assert.Equal(t, 256, cfg.MaxMemoryMB)
}

func TestConfig_ValidateMinMaxFileSize(t *testing.T) {
	cfg := &Config{
		MinFileSize: 100,
		MaxFileSize: 50,
	}
	err := cfg.Validate()
	assert.NoError(t, err)
	assert.Equal(t, int64(50), cfg.MinFileSize) // 被调整为 MaxFileSize
}

func TestConfig_ValidateNegativeValues(t *testing.T) {
	cfg := &Config{
		MinFileSize:   -1,
		MaxFileSize:   -1,
		MaxWorkers:    -1,
		MaxMemoryMB:   -1,
		MaxRefPerFile: -1,
	}
	err := cfg.Validate()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), cfg.MinFileSize)
	assert.Equal(t, int64(0), cfg.MaxFileSize)
	assert.Equal(t, 1, cfg.MaxWorkers)
	assert.Equal(t, 256, cfg.MaxMemoryMB)
	assert.Equal(t, 0, cfg.MaxRefPerFile)
}

func TestLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "smartdedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// 测试加载不存在的文件（应返回默认配置）
	cfg, err := LoadConfig(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
}

func TestSaveConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "smartdedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cfg := DefaultConfig()
	cfg.ScanPaths = []string{"/data"}

	err = SaveConfig(configPath, cfg)
	assert.NoError(t, err)

	// 验证文件存在
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// 加载并验证
	loaded, err := LoadConfig(configPath)
	assert.NoError(t, err)
	assert.Equal(t, cfg.ScanPaths, loaded.ScanPaths)
}

func TestManager_UpdateConfig(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	newCfg := DefaultConfig()
	newCfg.Enabled = false
	newCfg.Backend = BackendBtrfs

	err := mgr.UpdateConfig(newCfg)
	assert.NoError(t, err)

	updated := mgr.Config()
	assert.False(t, updated.Enabled)
	assert.Equal(t, BackendBtrfs, updated.Backend)
}

func TestManager_Scan_Disabled(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	cfg := testConfig()
	cfg.Enabled = false
	mgr.UpdateConfig(cfg)

	_, err := mgr.Scan(context.Background(), []string{tmpDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestManager_Scan_NoPaths(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	_, err := mgr.Scan(context.Background(), []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no scan paths")
}

func TestManager_Scan_DuplicateFiles(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	content := []byte("duplicate content for testing")
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")

	err := os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file3, content, 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, result.FilesScanned)
	assert.Len(t, result.DuplicateGroups, 1)
	assert.Equal(t, 2, result.TotalDuplicates)
	assert.Equal(t, int64(len(content)*2), result.PotentialSaving)
}

func TestManager_Scan_NoDuplicates(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建不同内容的文件
	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.Equal(t, 2, result.FilesScanned)
	assert.Len(t, result.DuplicateGroups, 0)
	assert.Equal(t, 0, result.TotalDuplicates)
}

func TestManager_Scan_MinFileSize(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	cfg := testConfig()
	cfg.MinFileSize = 100 // 100 bytes minimum
	mgr.UpdateConfig(cfg)

	// 创建小文件（应被跳过）
	err := os.WriteFile(filepath.Join(tmpDir, "small.txt"), []byte("tiny"), 0644)
	require.NoError(t, err)

	// 创建大文件
	err = os.WriteFile(filepath.Join(tmpDir, "large.txt"), make([]byte, 200), 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesScanned) // 只有 large.txt
}

func TestManager_Scan_ExcludePaths(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	cfg := testConfig()
	cfg.ExcludePaths = []string{filepath.Join(tmpDir, "excluded")}
	mgr.UpdateConfig(cfg)

	// 创建排除目录
	excludedDir := filepath.Join(tmpDir, "excluded")
	err := os.MkdirAll(excludedDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(excludedDir, "file.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	// 创建正常文件
	err = os.WriteFile(filepath.Join(tmpDir, "normal.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesScanned)
}

func TestManager_Scan_ExcludePatterns(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	cfg := testConfig()
	cfg.ExcludePatterns = []string{"*.log"}
	mgr.UpdateConfig(cfg)

	err := os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "app.log"), []byte("log content"), 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesScanned)
}

func TestManager_Scan_ContextCancel(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建一些文件
	for i := 0; i < 10; i++ {
		err := os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt"), []byte("content"), 0644)
		require.NoError(t, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	result, err := mgr.Scan(ctx, []string{tmpDir})
	// 可能返回错误或部分结果
	if err == nil {
		assert.NotNil(t, result)
	}
}

func TestManager_CancelScan(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// CancelScan 不应 panic
	mgr.CancelScan()
}

func TestManager_Dedup_Disabled(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	cfg := testConfig()
	cfg.Enabled = false
	mgr.UpdateConfig(cfg)

	_, err := mgr.Dedup(context.Background(), []DuplicateGroup{})
	assert.Error(t, err)
}

func TestManager_Dedup_EmptyGroups(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	result, err := mgr.Dedup(context.Background(), []DuplicateGroup{})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.ProcessedGroups)
}

func TestManager_Dedup_DryRun(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建源文件和目标文件
	content := []byte("test content for dedup")
	source := filepath.Join(tmpDir, "source.txt")
	target := filepath.Join(tmpDir, "target.txt")

	err := os.WriteFile(source, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(target, content, 0644)
	require.NoError(t, err)

	groups := []DuplicateGroup{
		{
			ContentHash: "test-hash",
			Files:       []string{source, target},
			FileCount:   2,
			UniqueSize:  int64(len(content)),
		},
	}

	result, err := mgr.Dedup(context.Background(), groups)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ProcessedGroups)
	assert.Equal(t, 1, result.DedupedFiles)
}

func TestManager_Dedup_ContextCancel(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	groups := []DuplicateGroup{
		{
			ContentHash: "test-hash",
			Files:       []string{"a.txt", "b.txt"},
			FileCount:   2,
		},
	}

	result, err := mgr.Dedup(ctx, groups)
	require.NoError(t, err)
	assert.Len(t, result.Errors, 1)
}

func TestManager_CancelDedup(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	mgr.CancelDedup() // 不应 panic
}

func TestManager_GetDuplicateGroups(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	groups := mgr.GetDuplicateGroups()
	assert.Empty(t, groups)
}

func TestManager_GetStats(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	stats := mgr.GetStats()
	assert.False(t, stats.IsScanning)
	assert.False(t, stats.IsDeduping)
	assert.Equal(t, int64(0), stats.TotalFilesScanned)
}

func TestManager_GetEntry_NotFound(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	_, ok := mgr.GetEntry("nonexistent")
	assert.False(t, ok)
}

func TestManager_ListEntries_Empty(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	entries := mgr.ListEntries()
	assert.Empty(t, entries)
}

func TestManager_GetRefCount_NotFound(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	_, ok := mgr.GetRefCount("nonexistent-hash")
	assert.False(t, ok)
}

func TestManager_ListRefCounts_Empty(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	refs := mgr.ListRefCounts()
	assert.Empty(t, refs)
}

func TestManager_DetectBackend_Auto(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	cfg := testConfig()
	cfg.Backend = BackendAuto
	mgr.UpdateConfig(cfg)

	// 自动检测在测试环境中可能失败
	backend, err := mgr.DetectBackend(tmpDir)
	if err != nil {
		assert.Contains(t, err.Error(), "unable to detect")
	} else {
		assert.NotEmpty(t, backend)
	}
}

func TestManager_DetectBackend_Specific(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	cfg := testConfig()
	cfg.Backend = BackendBtrfs
	mgr.UpdateConfig(cfg)

	backend, err := mgr.DetectBackend(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, BackendBtrfs, backend)
}

func TestDedupStats_GetSnapshot(t *testing.T) {
	stats := &DedupStats{
		TotalFilesScanned: 100,
		TotalSizeScanned:  1024 * 1024,
		SavedSpace:        512 * 1024,
	}

	snapshot := stats.GetSnapshot()
	assert.Equal(t, int64(100), snapshot.TotalFilesScanned)
	assert.Equal(t, int64(1024*1024), snapshot.TotalSizeScanned)
	assert.Equal(t, int64(512*1024), snapshot.SavedSpace)
}

func TestDedupStats_UpdateRatio(t *testing.T) {
	stats := &DedupStats{
		TotalSizeScanned: 1000,
		SavedSpace:       500,
	}

	stats.UpdateRatio()
	assert.InDelta(t, 0.5, stats.DedupRatio, 0.001)
}

func TestDedupStats_UpdateRatio_ZeroTotal(t *testing.T) {
	stats := &DedupStats{
		TotalSizeScanned: 0,
		SavedSpace:       0,
	}

	stats.UpdateRatio()
	assert.Equal(t, float64(0), stats.DedupRatio)
}

func TestRefCountEntry_IncrRef(t *testing.T) {
	ref := &RefCountEntry{
		ContentHash: "test-hash",
	}

	count := ref.IncrRef("file1.txt")
	assert.Equal(t, 1, count)
	assert.Len(t, ref.Files, 1)
	assert.Equal(t, "file1.txt", ref.Files[0])

	count = ref.IncrRef("file2.txt")
	assert.Equal(t, 2, count)
	assert.Len(t, ref.Files, 2)
}

func TestRefCountEntry_DecrRef(t *testing.T) {
	ref := &RefCountEntry{
		ContentHash: "test-hash",
		RefCount:    2,
		Files:       []string{"file1.txt", "file2.txt"},
	}

	count, hasRefs := ref.DecrRef("file1.txt")
	assert.Equal(t, 1, count)
	assert.True(t, hasRefs)
	assert.Len(t, ref.Files, 1)

	count, hasRefs = ref.DecrRef("file2.txt")
	assert.Equal(t, 0, count)
	assert.False(t, hasRefs)
	assert.Empty(t, ref.Files)
}

func TestRefCountEntry_DecrRef_AlreadyZero(t *testing.T) {
	ref := &RefCountEntry{
		ContentHash: "test-hash",
		RefCount:    0,
	}

	count, hasRefs := ref.DecrRef("file.txt")
	assert.Equal(t, 0, count)
	assert.False(t, hasRefs)
}

func TestRefCountEntry_Concurrent(t *testing.T) {
	ref := &RefCountEntry{
		ContentHash: "test-hash",
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			ref.IncrRef("file" + string(rune('0'+i)) + ".txt")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, ref.RefCount)
}

func TestManager_Scan_MultiplePaths(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	os.MkdirAll(dir1, 0755)
	os.MkdirAll(dir2, 0755)

	content := []byte("shared content")
	err := os.WriteFile(filepath.Join(dir1, "a.txt"), content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir2, "b.txt"), content, 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{dir1, dir2})
	require.NoError(t, err)
	assert.Equal(t, 2, result.FilesScanned)
	assert.Len(t, result.DuplicateGroups, 1)
}

func TestManager_Scan_Subdirectories(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	content := []byte("nested content")
	err := os.WriteFile(filepath.Join(tmpDir, "root.txt"), content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "nested.txt"), content, 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.Equal(t, 2, result.FilesScanned)
	assert.Len(t, result.DuplicateGroups, 1)
}

func TestManager_Scan_TimeTracking(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.True(t, result.Duration > 0)
	assert.True(t, result.EndTime.After(result.StartTime) || result.EndTime.Equal(result.StartTime))
}

func TestManager_Scan_StatsUpdated(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), make([]byte, 100), 0644)
	require.NoError(t, err)

	_, err = mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)

	stats := mgr.GetStats()
	assert.Equal(t, int64(1), stats.TotalFilesScanned)
	assert.Equal(t, int64(100), stats.TotalSizeScanned)
	assert.True(t, stats.LastScanTime.After(time.Time{}))
}

func TestManager_HashFile(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	content := []byte("test content for hashing")
	filePath := filepath.Join(tmpDir, "hash_test.txt")
	err := os.WriteFile(filePath, content, 0644)
	require.NoError(t, err)

	hash, size, err := mgr.hashFile(filePath)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Equal(t, int64(len(content)), size)

	// 同一文件应产生相同哈希
	hash2, _, err := mgr.hashFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)
}

func TestManager_HashFile_DifferentContent(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	err := os.WriteFile(file1, []byte("content A"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content B"), 0644)
	require.NoError(t, err)

	hash1, _, err := mgr.hashFile(file1)
	require.NoError(t, err)
	hash2, _, err := mgr.hashFile(file2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestManager_HashFile_NonExistent(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	_, _, err := mgr.hashFile(filepath.Join(tmpDir, "nonexistent.txt"))
	assert.Error(t, err)
}

func TestWalkFiles_SkipsDirectories(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建目录结构
	dir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	cfg := testConfig()
	fileCh := make(chan string, 100)

	go func() {
		mgr.walkFiles(context.Background(), tmpDir, cfg, fileCh)
		close(fileCh)
	}()

	var files []string
	for f := range fileCh {
		files = append(files, f)
	}

	assert.Len(t, files, 1)
	assert.Contains(t, files[0], "file.txt")
}

func TestManager_Scan_LargeDuplicateGroup(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	content := []byte("shared content for many files")
	for i := 0; i < 10; i++ {
		filePath := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		err := os.WriteFile(filePath, content, 0644)
		require.NoError(t, err)
	}

	result, err := mgr.Scan(context.Background(), []string{tmpDir})
	require.NoError(t, err)
	assert.Equal(t, 10, result.FilesScanned)
	assert.Len(t, result.DuplicateGroups, 1)
	assert.Equal(t, 9, result.TotalDuplicates)
	assert.Equal(t, int64(len(content)*9), result.PotentialSaving)
}
