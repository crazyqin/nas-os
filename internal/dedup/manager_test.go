package dedup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	require.NotNil(t, mgr)
}

func TestScan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")

	err = os.WriteFile(file1, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file3, []byte("unique content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	config.MinFileSize = 1 // 设置最小文件大小为 1 字节

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 执行扫描
	result, err := mgr.Scan([]string{tmpDir})
	require.NoError(t, err)
	require.NotNil(t, result)

	// 验证扫描结果
	assert.Equal(t, 3, result.FilesScanned)
	assert.Equal(t, 1, result.DuplicateGroups) // 一组重复文件
	assert.Equal(t, 1, result.DuplicatesFound)  // 1 个重复文件
}

func TestGetDuplicates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	err = os.WriteFile(file1, []byte("same content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("same content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	config.MinFileSize = 1

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 执行扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取重复文件列表
	duplicates, err := mgr.GetDuplicates()
	require.NoError(t, err)
	require.Len(t, duplicates, 1)

	group := duplicates[0]
	assert.Equal(t, int64(12), group.Size)
	assert.Len(t, group.Files, 2)
	assert.Contains(t, group.Files, file1)
	assert.Contains(t, group.Files, file2)
}

func TestDeduplicate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	content := []byte("duplicate content for test")
	err = os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	config.MinFileSize = 1

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 执行扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取重复文件组
	duplicates, err := mgr.GetDuplicates()
	require.NoError(t, err)
	require.Len(t, duplicates, 1)

	group := duplicates[0]

	// 执行去重（使用软链接）
	policy := DedupPolicy{
		Mode:          "file",
		Action:        "softlink",
		MinMatchCount: 1,
		PreserveAttrs: true,
	}

	err = mgr.Deduplicate(group.Checksum, file1, policy)
	require.NoError(t, err)

	// 验证 file2 现在是软链接
	linkTarget, err := os.Readlink(file2)
	require.NoError(t, err)
	assert.Equal(t, file1, linkTarget)
}

func TestGetReport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "unique.txt")

	err = os.WriteFile(file1, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("duplicate content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file3, []byte("unique"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	config.MinFileSize = 1

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 执行扫描
	_, err = mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 获取报告
	report, err := mgr.GetReport()
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, 3, report.Stats.TotalFiles)
	assert.Equal(t, 1, report.Stats.DuplicateFiles)
	assert.Len(t, report.DuplicateGroups, 1)
}

func TestGetStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 获取统计信息
	stats := mgr.GetStats()
	assert.Equal(t, 0, stats.TotalFiles)
	assert.Equal(t, int64(0), stats.TotalSize)
}

func TestCreateChunk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 创建数据块
	data := []byte("test chunk data")
	chunk, err := mgr.CreateChunk(data)
	require.NoError(t, err)
	require.NotNil(t, chunk)

	assert.NotEmpty(t, chunk.Hash)
	assert.Equal(t, int64(len(data)), chunk.Size)
	assert.Equal(t, 1, chunk.RefCount)

	// 再次创建相同的数据块
	chunk2, err := mgr.CreateChunk(data)
	require.NoError(t, err)

	// 应该返回相同的块，引用计数增加
	assert.Equal(t, chunk.Hash, chunk2.Hash)
	assert.Equal(t, 2, chunk2.RefCount)
}

func TestGetChunk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 创建数据块
	data := []byte("test chunk data")
	chunk, err := mgr.CreateChunk(data)
	require.NoError(t, err)

	// 获取数据块
	retrieved, err := mgr.GetChunk(chunk.Hash)
	require.NoError(t, err)
	assert.Equal(t, chunk.Hash, retrieved.Hash)

	// 获取不存在的块
	_, err = mgr.GetChunk("nonexistent")
	assert.Error(t, err)
}

func TestExcludePaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建排除目录
	excludeDir := filepath.Join(tmpDir, "exclude")
	err = os.MkdirAll(excludeDir, 0755)
	require.NoError(t, err)

	// 在排除目录中创建文件
	excludeFile := filepath.Join(excludeDir, "file.txt")
	err = os.WriteFile(excludeFile, []byte("excluded content"), 0644)
	require.NoError(t, err)

	// 在正常目录中创建文件
	normalFile := filepath.Join(tmpDir, "normal.txt")
	err = os.WriteFile(normalFile, []byte("normal content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	config.MinFileSize = 1
	config.ExcludePaths = []string{excludeDir}

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 执行扫描
	result, err := mgr.Scan([]string{tmpDir})
	require.NoError(t, err)

	// 只有 normal.txt 被扫描
	assert.Equal(t, 1, result.FilesScanned)
}

func TestCancelScan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 取消扫描（即使没有扫描也应该能正常处理）
	mgr.CancelScan()

	// 验证管理器仍然可用
	stats := mgr.GetStats()
	assert.Equal(t, 0, stats.TotalFiles)
}

func TestUpdateConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	config := DefaultConfig()
	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)

	// 更新配置
	newConfig := DefaultConfig()
	newConfig.ChunkSize = 8 * 1024 * 1024 // 8MB
	newConfig.MinFileSize = 2048

	err = mgr.UpdateConfig(newConfig)
	require.NoError(t, err)

	// 验证配置已更新
	assert.Equal(t, int64(8*1024*1024), mgr.config.ChunkSize)
	assert.Equal(t, int64(2048), mgr.config.MinFileSize)
}