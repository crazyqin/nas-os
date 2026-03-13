package compress

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.True(t, config.Enabled)
	assert.Equal(t, AlgorithmZstd, config.DefaultAlgorithm)
	assert.Equal(t, 6, config.CompressionLevel)
	assert.Equal(t, int64(1024), config.MinSize)
	assert.True(t, config.CompressOnWrite)
	assert.True(t, config.DecompressOnRead)
	assert.True(t, config.StatsEnabled)
}

func TestNewManager(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		manager, err := NewManager(nil)
		assert.NoError(t, err)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.config)
		assert.NotNil(t, manager.stats)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			Enabled:          true,
			DefaultAlgorithm: AlgorithmGzip,
			CompressionLevel: 9,
			MinSize:          2048,
		}

		manager, err := NewManager(config)
		assert.NoError(t, err)
		assert.NotNil(t, manager)
		assert.Equal(t, AlgorithmGzip, manager.config.DefaultAlgorithm)
		assert.Equal(t, 9, manager.config.CompressionLevel)
	})
}

func TestShouldCompress(t *testing.T) {
	manager, err := NewManager(nil)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		size     int64
		expected bool
	}{
		{
			name:     "should compress text file",
			path:     "/data/file.txt",
			size:     10000,
			expected: true,
		},
		{
			name:     "should not compress small file",
			path:     "/data/small.txt",
			size:     100,
			expected: false,
		},
		{
			name:     "should not compress mp3",
			path:     "/data/music.mp3",
			size:     1000000,
			expected: false,
		},
		{
			name:     "should not compress jpg",
			path:     "/data/image.jpg",
			size:     500000,
			expected: false,
		},
		{
			name:     "should not compress in excluded dir",
			path:     "/.git/objects/abc",
			size:     10000,
			expected: false,
		},
		{
			name:     "should not compress in node_modules",
			path:     "/project/node_modules/lib/index.js",
			size:     10000,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.ShouldCompress(tt.path, tt.size)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompressFile(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test.txt")
	dstFile := filepath.Join(tmpDir, "test.txt.gz")

	// 写入测试内容
	content := make([]byte, 10000)
	for i := range content {
		content[i] = byte(i % 256)
	}
	os.WriteFile(srcFile, content, 0644)

	// 使用 gzip 算法测试
	config := &Config{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmGzip,
		CompressionLevel: 6,
		MinSize:          100,
		CompressOnWrite:  true,
		StatsEnabled:     true,
	}

	manager, err := NewManager(config)
	assert.NoError(t, err)

	result, err := manager.CompressFile(srcFile, dstFile)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Skipped)
	assert.Equal(t, AlgorithmGzip, result.Algorithm)
	assert.True(t, result.CompressedSize > 0)

	// 检查压缩文件是否存在
	_, err = os.Stat(dstFile + ".gz")
	assert.NoError(t, err)
}

func TestDecompressFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test.txt")
	dstFile := filepath.Join(tmpDir, "test.txt.gz")
	outFile := filepath.Join(tmpDir, "test_out.txt")

	// 写入测试内容
	content := make([]byte, 10000)
	for i := range content {
		content[i] = byte(i % 256)
	}
	os.WriteFile(srcFile, content, 0644)

	config := &Config{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmGzip,
		CompressionLevel: 6,
		MinSize:          100,
	}

	manager, err := NewManager(config)
	assert.NoError(t, err)

	// 压缩
	_, err = manager.CompressFile(srcFile, dstFile)
	assert.NoError(t, err)

	// 解压
	err = manager.DecompressFile(dstFile+".gz", outFile)
	assert.NoError(t, err)

	// 验证解压后的内容
	decompressed, err := os.ReadFile(outFile)
	assert.NoError(t, err)
	assert.Equal(t, content, decompressed)
}

func TestStats(t *testing.T) {
	stats := NewStats()
	assert.NotNil(t, stats)

	// 更新统计
	stats.Update(AlgorithmGzip, 10000, 5000)
	stats.Update(AlgorithmGzip, 20000, 10000)

	assert.Equal(t, int64(2), stats.TotalFiles)
	assert.Equal(t, int64(30000), stats.TotalBytes)
	assert.Equal(t, int64(15000), stats.CompressedBytes)
	assert.Equal(t, int64(15000), stats.SavedBytes)
	assert.Equal(t, 0.5, stats.AvgRatio)

	// 检查算法统计
	gzipStats := stats.ByAlgorithm[AlgorithmGzip]
	assert.NotNil(t, gzipStats)
	assert.Equal(t, int64(2), gzipStats.Files)
	assert.Equal(t, int64(30000), gzipStats.Original)
	assert.Equal(t, int64(15000), gzipStats.Compressed)
}

func TestStatsReset(t *testing.T) {
	stats := NewStats()

	stats.Update(AlgorithmGzip, 10000, 5000)
	stats.Reset()

	assert.Equal(t, int64(0), stats.TotalFiles)
	assert.Equal(t, int64(0), stats.TotalBytes)
	assert.Equal(t, int64(0), stats.CompressedBytes)
	assert.Equal(t, int64(0), stats.SavedBytes)
	assert.Equal(t, float64(0), stats.AvgRatio)
}

func TestFileSystem(t *testing.T) {
	tmpDir := t.TempDir()

	config := DefaultConfig()
	config.DefaultAlgorithm = AlgorithmGzip
	config.MinSize = 100

	manager, err := NewManager(config)
	assert.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	assert.NoError(t, err)
	assert.NotNil(t, fs)

	// 测试创建目录
	err = fs.Mkdir("/subdir", 0755)
	assert.NoError(t, err)

	// 测试创建文件
	writer, err := fs.Create("/test.txt")
	assert.NoError(t, err)
	assert.NotNil(t, writer)

	// 写入内容
	content := make([]byte, 10000)
	for i := range content {
		content[i] = byte(i % 256)
	}
	n, err := writer.Write(content)
	assert.NoError(t, err)
	assert.Equal(t, len(content), n)

	err = writer.Close()
	assert.NoError(t, err)
}

func TestDetectAlgorithm(t *testing.T) {
	manager, _ := NewManager(nil)

	tests := []struct {
		path     string
		expected Algorithm
	}{
		{"/file.gz", AlgorithmGzip},
		{"/file.zst", AlgorithmZstd},
		{"/file.lz4", AlgorithmLz4},
		{"/file.txt", AlgorithmNone},
		{"/file", AlgorithmNone},
	}

	for _, tt := range tests {
		result := manager.detectAlgorithm(tt.path)
		assert.Equal(t, tt.expected, result, "path: %s", tt.path)
	}
}

func TestGzipCompressor(t *testing.T) {
	compressor := &GzipCompressor{}

	assert.Equal(t, ".gz", compressor.Extension())
	assert.Equal(t, AlgorithmGzip, compressor.Name())
}

func TestIncludeDirs(t *testing.T) {
	config := &Config{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmGzip,
		MinSize:          100,
		IncludeDirs:      []string{"/data/compress"},
	}

	manager, err := NewManager(config)
	assert.NoError(t, err)

	// 在包含目录内的文件应该压缩
	assert.True(t, manager.ShouldCompress("/data/compress/file.txt", 10000))

	// 在包含目录外的文件不应该压缩
	assert.False(t, manager.ShouldCompress("/data/other/file.txt", 10000))
}

func TestBatchCompressResult(t *testing.T) {
	result := &BatchCompressResult{
		TotalFiles:      10,
		CompressedFiles: 8,
		SkippedFiles:    2,
		SavedBytes:      50000,
	}

	assert.Equal(t, 10, result.TotalFiles)
	assert.Equal(t, 8, result.CompressedFiles)
	assert.Equal(t, int64(50000), result.SavedBytes)
}

func TestCompressedFileInfo(t *testing.T) {
	info := &CompressedFileInfo{
		Name:           "test.txt.gz",
		Path:           "/data/test.txt.gz",
		OriginalSize:   10000,
		CompressedSize: 5000,
		Ratio:          0.5,
		Algorithm:      AlgorithmGzip,
	}

	assert.Equal(t, "test.txt.gz", info.Name)
	assert.Equal(t, AlgorithmGzip, info.Algorithm)
	assert.Equal(t, float64(0.5), info.Ratio)
}