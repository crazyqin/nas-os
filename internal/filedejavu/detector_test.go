// Package filedejavu 提供重复文件智能检测功能
package filedejavu

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeSHA256(t *testing.T) {
	// 创建临时文件
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 测试文件内容
	content := []byte("hello world, this is a test file for SHA-256 hash computation")
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// 计算哈希
	hash, err := ComputeSHA256(testFile)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// 验证哈希格式（64 字符十六进制）
	assert.Len(t, hash, 64)
	_, err = hex.DecodeString(hash)
	assert.NoError(t, err)

	// 相同文件应产生相同哈希
	hash2, err := ComputeSHA256(testFile)
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)
}

func TestComputeSHA256_DifferentContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建两个不同内容的文件
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	err = os.WriteFile(file1, []byte("content A"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content B"), 0644)
	require.NoError(t, err)

	hash1, err := ComputeSHA256(file1)
	require.NoError(t, err)

	hash2, err := ComputeSHA256(file2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestComputeSHA256_LargeFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建大于 chunkSize 的文件（测试分块读取）
	largeFile := filepath.Join(tmpDir, "large.bin")
	f, err := os.Create(largeFile)
	require.NoError(t, err)

	// 写入 256KB 数据
	data := make([]byte, 256*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	_, err = f.Write(data)
	require.NoError(t, err)
	f.Close()

	hash, err := ComputeSHA256(largeFile)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64)
}

func TestGroupBySize(t *testing.T) {
	detector := NewDetector(DefaultScanConfig())

	files := []*FileFingerprint{
		{Path: "/a/file1.txt", Size: 100},
		{Path: "/a/file2.txt", Size: 100},
		{Path: "/b/file3.txt", Size: 200},
		{Path: "/b/file4.txt", Size: 200},
		{Path: "/c/file5.txt", Size: 300}, // 唯一大小
	}

	groups := detector.groupBySize(files)

	// 应该有两个组（大小 100 和 200），大小 300 被过滤掉
	assert.Len(t, groups, 2)
	assert.Len(t, groups[100], 2)
	assert.Len(t, groups[200], 2)
	assert.Nil(t, groups[300])
}

func TestGroupByHash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	content := []byte("duplicate content")
	file1 := filepath.Join(tmpDir, "dup1.txt")
	file2 := filepath.Join(tmpDir, "dup2.txt")
	file3 := filepath.Join(tmpDir, "unique.txt")

	err = os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file3, []byte("unique content"), 0644)
	require.NoError(t, err)

	config := DefaultScanConfig()
	config.Paths = []string{tmpDir}
	config.MinFileSize = 0
	detector := NewDetector(config)

	files := []*FileFingerprint{
		{Path: file1, Size: int64(len(content))},
		{Path: file2, Size: int64(len(content))},
		{Path: file3, Size: int64(len("unique content"))},
	}

	sizeGroups := detector.groupBySize(files)
	hashGroups := detector.groupByHash(context.Background(), sizeGroups)

	// 应该有一个哈希组（两个重复文件）
	totalGroups := 0
	for _, group := range hashGroups {
		if len(group) >= 2 {
			totalGroups++
		}
	}
	assert.Equal(t, 1, totalGroups)
}

func TestScan_SmallFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	content := []byte("test content for dedup")
	for i := 0; i < 3; i++ {
		file := filepath.Join(tmpDir, "dup"+string(rune('A'+i))+".txt")
		err = os.WriteFile(file, content, 0644)
		require.NoError(t, err)
	}

	config := DefaultScanConfig()
	config.Paths = []string{tmpDir}
	config.MinFileSize = 0
	config.ScanImages = false

	detector := NewDetector(config)
	result, err := detector.Scan(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, int64(3), result.TotalFiles)
	assert.True(t, result.DuplicateCount > 0)
	assert.True(t, result.SavingsTotal > 0)
}

func TestScan_MixedFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	dupContent := []byte("duplicate content here")
	err = os.WriteFile(filepath.Join(tmpDir, "a.txt"), dupContent, 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "b.txt"), dupContent, 0644)
	require.NoError(t, err)

	// 创建唯一文件
	err = os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("unique"), 0644)
	require.NoError(t, err)

	config := DefaultScanConfig()
	config.Paths = []string{tmpDir}
	config.MinFileSize = 0
	config.ScanImages = false

	detector := NewDetector(config)
	result, err := detector.Scan(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(3), result.TotalFiles)
	assert.Equal(t, int64(1), result.DuplicateCount) // 1 个重复文件
}

func TestScan_Cancel(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建大量文件使扫描耗时更长
	for i := 0; i < 100; i++ {
		file := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
		err = os.WriteFile(file, []byte(fmt.Sprintf("content %d", i)), 0644)
		require.NoError(t, err)
	}

	config := DefaultScanConfig()
	config.Paths = []string{tmpDir}
	config.MinFileSize = 0
	config.MaxWorkers = 1 // 降低并行度以延长扫描时间
	config.ScanImages = false

	detector := NewDetector(config)

	// 使用 channel 同步
	started := make(chan struct{})
	var result *ScanResult
	var scanErr error

	go func() {
		close(started)
		result, scanErr = detector.Scan(context.Background())
	}()

	<-started
	time.Sleep(1 * time.Millisecond)
	detector.Cancel()
	time.Sleep(100 * time.Millisecond)

	// 取消操作最终会产生 cancelled 状态
	if result != nil {
		assert.Contains(t, []string{"cancelled", "completed"}, result.Status)
	}
	// 不管结果如何，扫描不应该 panic
	assert.Nil(t, scanErr)
}

func TestScan_ExcludePatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	content := []byte("content")
	err = os.WriteFile(filepath.Join(tmpDir, "keep.txt"), content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "exclude.log"), content, 0644)
	require.NoError(t, err)

	config := DefaultScanConfig()
	config.Paths = []string{tmpDir}
	config.MinFileSize = 0
	config.ExcludePatterns = []string{"*.log"}
	config.ScanImages = false

	detector := NewDetector(config)
	result, err := detector.Scan(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.TotalFiles) // .log 文件被排除
}

func TestBuildDuplicateGroups(t *testing.T) {
	detector := NewDetector(DefaultScanConfig())
	detector.result = &ScanResult{
		StartTime: time.Now(),
		Status:    "running",
	}

	hashGroups := map[string][]*FileFingerprint{
		"abc123": {
			{Path: "/a/file1.txt", Size: 100, ModTime: time.Now()},
			{Path: "/b/file2.txt", Size: 100, ModTime: time.Now().Add(-time.Hour)},
		},
	}

	detector.buildDuplicateGroups(hashGroups)

	groups := detector.result.GetGroups()
	assert.Len(t, groups, 1)
	assert.Equal(t, DupExact, groups[0].Type)
	assert.Len(t, groups[0].Files, 2)
	assert.Equal(t, int64(100), groups[0].Savings)
}

func TestRecommendKeep(t *testing.T) {
	detector := NewDetector(&ScanConfig{
		KeepStrategy: KeepNewest,
	})

	now := time.Now()
	files := []*FileFingerprint{
		{Path: "/old.txt", ModTime: now.Add(-2 * time.Hour)},
		{Path: "/new.txt", ModTime: now},
		{Path: "/middle.txt", ModTime: now.Add(-time.Hour)},
	}

	keep := detector.recommendKeep(files)
	assert.Equal(t, "/new.txt", keep.Path)
}

func TestRecommendKeep_Oldest(t *testing.T) {
	detector := NewDetector(&ScanConfig{
		KeepStrategy: KeepOldest,
	})

	now := time.Now()
	files := []*FileFingerprint{
		{Path: "/old.txt", ModTime: now.Add(-2 * time.Hour)},
		{Path: "/new.txt", ModTime: now},
	}

	keep := detector.recommendKeep(files)
	assert.Equal(t, "/old.txt", keep.Path)
}

func TestRecommendKeep_Largest(t *testing.T) {
	detector := NewDetector(&ScanConfig{
		KeepStrategy: KeepLargest,
	})

	files := []*FileFingerprint{
		{Path: "/small.txt", Size: 100},
		{Path: "/large.txt", Size: 1000},
	}

	keep := detector.recommendKeep(files)
	assert.Equal(t, "/large.txt", keep.Path)
}

func TestRecommendKeep_First(t *testing.T) {
	detector := NewDetector(&ScanConfig{
		KeepStrategy: KeepFirst,
	})

	files := []*FileFingerprint{
		{Path: "/z.txt", Size: 100},
		{Path: "/a.txt", Size: 100},
	}

	keep := detector.recommendKeep(files)
	assert.Equal(t, "/a.txt", keep.Path)
}

func TestComparePHash(t *testing.T) {
	// 完全相同
	sim := ComparePHash("abcdef01", "abcdef01")
	assert.Equal(t, 1.0, sim)

	// 不同长度
	sim = ComparePHash("abc", "abcdef")
	assert.Equal(t, 0.0, sim)

	// 相似（1 bit 差异）
	sim = ComparePHash("00", "01")
	assert.True(t, sim > 0.5)
	assert.True(t, sim < 1.0)
}

func TestIsImageFile(t *testing.T) {
	assert.True(t, IsImageFile("photo.jpg"))
	assert.True(t, IsImageFile("photo.jpeg"))
	assert.True(t, IsImageFile("image.png"))
	assert.True(t, IsImageFile("animation.gif"))
	assert.True(t, IsImageFile("pic.webp"))
	assert.False(t, IsImageFile("document.pdf"))
	assert.False(t, IsImageFile("video.mp4"))
	assert.False(t, IsImageFile("file.txt"))
}

func TestScanResult_ThreadSafety(t *testing.T) {
	result := &ScanResult{
		StartTime: time.Now(),
		Status:    "running",
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			group := &DuplicateGroup{
				ID:    fmt.Sprintf("grp_%d", i),
				Files: []*FileFingerprint{{Path: "/test", Size: 100}},
			}
			result.AddGroup(group)
		}(i)
	}

	wg.Wait()

	groups := result.GetGroups()
	assert.Len(t, groups, 100)
}

func TestBatchDedup_DryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建重复文件
	content := []byte("dedup test content")
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	err = os.WriteFile(file1, content, 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err)

	config := DefaultScanConfig()
	config.Paths = []string{tmpDir}
	config.MinFileSize = 0
	config.ScanImages = false
	config.DryRun = true

	detector := NewDetector(config)
	_, err = detector.Scan(context.Background())
	require.NoError(t, err)

	// 执行 dry-run 去重
	dedupReq := &BatchDedupRequest{
		Action:   ActionDelete,
		Strategy: KeepNewest,
		DryRun:   true,
	}

	dedupResult, err := detector.BatchDedup(context.Background(), dedupReq)
	require.NoError(t, err)
	assert.Equal(t, 1, dedupResult.ProcessedGroups)
	assert.Equal(t, 1, dedupResult.DeletedFiles)

	// 验证文件仍然存在（dry-run 模式）
	_, err = os.Stat(file1)
	assert.NoError(t, err)
	_, err = os.Stat(file2)
	assert.NoError(t, err)
}

func TestDefaultScanConfig(t *testing.T) {
	config := DefaultScanConfig()

	assert.Equal(t, int64(1024), config.MinFileSize)
	assert.Equal(t, 0.85, config.Threshold)
	assert.True(t, config.ScanImages)
	assert.Equal(t, KeepNewest, config.KeepStrategy)
	assert.Equal(t, ActionReport, config.Action)
	assert.True(t, config.DryRun)
	assert.Equal(t, 4, config.MaxWorkers)
}

func TestNewDetector_NilConfig(t *testing.T) {
	detector := NewDetector(nil)
	assert.NotNil(t, detector)
	assert.NotNil(t, detector.Config())
	assert.Equal(t, 0.85, detector.Config().Threshold)
}

func TestScan_MinFileSize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filedejavu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建小文件（小于 MinFileSize）
	smallFile := filepath.Join(tmpDir, "small.txt")
	err = os.WriteFile(smallFile, []byte("tiny"), 0644) // 4 bytes
	require.NoError(t, err)

	// 创建大文件
	largeFile := filepath.Join(tmpDir, "large.txt")
	err = os.WriteFile(largeFile, make([]byte, 2048), 0644) // 2KB
	require.NoError(t, err)

	config := DefaultScanConfig()
	config.Paths = []string{tmpDir}
	config.MinFileSize = 1024 // 1KB
	config.ScanImages = false

	detector := NewDetector(config)
	result, err := detector.Scan(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.TotalFiles) // 小文件被过滤
}

func TestHexToBits(t *testing.T) {
	bits := hexToBits("ff")
	assert.Len(t, bits, 8)
	for _, b := range bits {
		assert.Equal(t, byte(1), b)
	}

	bits = hexToBits("00")
	for _, b := range bits {
		assert.Equal(t, byte(0), b)
	}
}

func TestBatchDedup_NoScanResult(t *testing.T) {
	detector := NewDetector(DefaultScanConfig())

	req := &BatchDedupRequest{
		Action:   ActionDelete,
		Strategy: KeepNewest,
	}

	_, err := detector.BatchDedup(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no scan result available")
}
