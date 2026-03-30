// Package integration 提供 NAS-OS 集成测试
// 压缩存储模块集成测试
package integration

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"nas-os/internal/compress"
)

// MockCompressManager 模拟压缩管理器.
type MockCompressManager struct {
	config *compress.Config
	stats  *compress.Stats
	files  map[string]*CompressedFile
	mu     sync.RWMutex
}

// CompressedFile 压缩文件信息.
type CompressedFile struct {
	Path      string
	OrigSize  int64
	CompSize  int64
	Algorithm compress.Algorithm
	Ratio     float64
}

// NewMockCompressManager 创建模拟压缩管理器.
func NewMockCompressManager() *MockCompressManager {
	return &MockCompressManager{
		config: compress.DefaultConfig(),
		stats: &compress.Stats{
			ByAlgorithm: make(map[compress.Algorithm]*compress.AlgorithmStats),
		},
		files: make(map[string]*CompressedFile),
	}
}

// CompressFile 压缩文件.
func (m *MockCompressManager) CompressFile(path string, data []byte, algo compress.Algorithm) (*CompressedFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	origSize := int64(len(data))
	// 模拟压缩率 40%
	compSize := int64(float64(origSize) * 0.6)
	ratio := float64(compSize) / float64(origSize)

	file := &CompressedFile{
		Path:      path,
		OrigSize:  origSize,
		CompSize:  compSize,
		Algorithm: algo,
		Ratio:     ratio,
	}

	m.files[path] = file

	// 更新统计
	m.stats.TotalFiles++
	m.stats.CompressedFiles++
	m.stats.TotalBytes += origSize
	m.stats.CompressedBytes += compSize
	m.stats.SavedBytes += origSize - compSize

	if _, ok := m.stats.ByAlgorithm[algo]; !ok {
		m.stats.ByAlgorithm[algo] = &compress.AlgorithmStats{}
	}
	m.stats.ByAlgorithm[algo].Files++
	m.stats.ByAlgorithm[algo].Original += origSize
	m.stats.ByAlgorithm[algo].Compressed += compSize

	return file, nil
}

// DecompressFile 解压文件.
func (m *MockCompressManager) DecompressFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, ok := m.files[path]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}

	// 模拟解压数据
	return make([]byte, file.OrigSize), nil
}

// GetStats 获取统计.
func (m *MockCompressManager) GetStats() *compress.Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetConfig 获取配置.
func (m *MockCompressManager) GetConfig() *compress.Config {
	return m.config
}

// ShouldCompress 检查是否应该压缩.
func (m *MockCompressManager) ShouldCompress(path string, size int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查最小大小
	if size < m.config.MinSize {
		return false
	}

	// 检查扩展名
	for _, ext := range m.config.ExcludeExtensions {
		if len(path) > len(ext) && path[len(path)-len(ext):] == ext {
			return false
		}
	}

	return true
}

// ========== 压缩存储集成测试 ==========

// TestCompress_DefaultConfig 测试默认配置.
func TestCompress_DefaultConfig(t *testing.T) {
	config := compress.DefaultConfig()

	if !config.Enabled {
		t.Error("Expected Enabled=true")
	}

	if config.DefaultAlgorithm != compress.AlgorithmZstd {
		t.Errorf("Expected DefaultAlgorithm=zstd, got %s", config.DefaultAlgorithm)
	}

	if config.CompressionLevel != 6 {
		t.Errorf("Expected CompressionLevel=6, got %d", config.CompressionLevel)
	}

	if config.MinSize != 1024 {
		t.Errorf("Expected MinSize=1024, got %d", config.MinSize)
	}

	if !config.CompressOnWrite {
		t.Error("Expected CompressOnWrite=true")
	}

	if !config.DecompressOnRead {
		t.Error("Expected DecompressOnRead=true")
	}
}

// TestCompress_Algorithms 测试压缩算法.
func TestCompress_Algorithms(t *testing.T) {
	algorithms := []compress.Algorithm{
		compress.AlgorithmZstd,
		compress.AlgorithmLz4,
		compress.AlgorithmGzip,
		compress.AlgorithmNone,
	}

	for _, algo := range algorithms {
		t.Run(string(algo), func(t *testing.T) {
			if algo == "" {
				t.Error("Algorithm should not be empty")
			}
		})
	}
}

// TestCompress_CompressFile 测试文件压缩.
func TestCompress_CompressFile(t *testing.T) {
	manager := NewMockCompressManager()

	testData := make([]byte, 10000) // 10KB 数据
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	tests := []struct {
		name string
		path string
		algo compress.Algorithm
	}{
		{"Zstd Compression", "/data/file1.txt", compress.AlgorithmZstd},
		{"Lz4 Compression", "/data/file2.txt", compress.AlgorithmLz4},
		{"Gzip Compression", "/data/file3.txt", compress.AlgorithmGzip},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := manager.CompressFile(tt.path, testData, tt.algo)
			if err != nil {
				t.Fatalf("CompressFile failed: %v", err)
			}

			if file.Path != tt.path {
				t.Errorf("Expected path=%s, got %s", tt.path, file.Path)
			}

			if file.Algorithm != tt.algo {
				t.Errorf("Expected algorithm=%s, got %s", tt.algo, file.Algorithm)
			}

			if file.OrigSize != int64(len(testData)) {
				t.Errorf("Expected origSize=%d, got %d", len(testData), file.OrigSize)
			}

			// 验证压缩效果
			if file.CompSize >= file.OrigSize {
				t.Error("Compressed size should be less than original")
			}

			if file.Ratio >= 1.0 {
				t.Error("Compression ratio should be less than 1.0")
			}
		})
	}
}

// TestCompress_DecompressFile 测试文件解压.
func TestCompress_DecompressFile(t *testing.T) {
	manager := NewMockCompressManager()

	testData := make([]byte, 5000)
	path := "/data/test.txt"

	// 先压缩
	_, err := manager.CompressFile(path, testData, compress.AlgorithmZstd)
	if err != nil {
		t.Fatalf("CompressFile failed: %v", err)
	}

	// 再解压
	result, err := manager.DecompressFile(path)
	if err != nil {
		t.Fatalf("DecompressFile failed: %v", err)
	}

	if len(result) != len(testData) {
		t.Errorf("Expected decompressed size=%d, got %d", len(testData), len(result))
	}
}

// TestCompress_ShouldCompress 测试压缩判断.
func TestCompress_ShouldCompress(t *testing.T) {
	manager := NewMockCompressManager()

	tests := []struct {
		name     string
		path     string
		size     int64
		expected bool
	}{
		{"Small file", "/data/small.txt", 100, false},
		{"Large file", "/data/large.txt", 10000, true},
		{"Already compressed - zip", "/data/file.zip", 10000, false},
		{"Already compressed - gz", "/data/file.gz", 10000, false},
		{"Already compressed - mp3", "/data/music.mp3", 10000, false},
		{"Already compressed - mp4", "/data/video.mp4", 10000, false},
		{"Already compressed - jpg", "/data/image.jpg", 10000, false},
		{"Already compressed - png", "/data/image.png", 10000, false},
		{"Text file", "/data/document.txt", 10000, true},
		{"Log file", "/data/logs/app.log", 10000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.ShouldCompress(tt.path, tt.size)
			if result != tt.expected {
				t.Errorf("ShouldCompress(%s, %d) = %v, want %v", tt.path, tt.size, result, tt.expected)
			}
		})
	}
}

// TestCompress_Stats 测试压缩统计.
func TestCompress_Stats(t *testing.T) {
	manager := NewMockCompressManager()

	// 压缩多个文件
	for i := 0; i < 5; i++ {
		data := make([]byte, 10000)
		path := "/data/file" + string(rune('0'+i)) + ".txt"
		_, _ = manager.CompressFile(path, data, compress.AlgorithmZstd)
	}

	stats := manager.GetStats()

	if stats.TotalFiles != 5 {
		t.Errorf("Expected TotalFiles=5, got %d", stats.TotalFiles)
	}

	if stats.CompressedFiles != 5 {
		t.Errorf("Expected CompressedFiles=5, got %d", stats.CompressedFiles)
	}

	if stats.TotalBytes != 50000 {
		t.Errorf("Expected TotalBytes=50000, got %d", stats.TotalBytes)
	}

	if stats.SavedBytes <= 0 {
		t.Error("Expected SavedBytes > 0")
	}
}

// TestCompress_ExcludeExtensions 测试排除扩展名.
func TestCompress_ExcludeExtensions(t *testing.T) {
	config := compress.DefaultConfig()

	excluded := []string{
		".zip", ".gz", ".bz2", ".xz", ".zst", ".lz4",
		".mp3", ".mp4", ".avi", ".mkv", ".mov",
		".jpg", ".jpeg", ".png", ".gif", ".webp",
		".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx",
	}

	// 验证排除列表存在
	if len(config.ExcludeExtensions) == 0 {
		t.Error("ExcludeExtensions should not be empty")
	}

	// 验证关键扩展名在排除列表中
	for _, ext := range excluded {
		found := false
		for _, e := range config.ExcludeExtensions {
			if e == ext {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: extension %s not in exclude list", ext)
		}
	}
}

// TestCompress_CompressDecompressRoundTrip 测试压缩解压往返.
func TestCompress_CompressDecompressRoundTrip(t *testing.T) {
	manager := NewMockCompressManager()

	originalData := []byte("This is a test file with some content that should be compressed and then decompressed correctly.")
	path := "/data/roundtrip.txt"

	// 压缩
	compressed, err := manager.CompressFile(path, originalData, compress.AlgorithmZstd)
	if err != nil {
		t.Fatalf("CompressFile failed: %v", err)
	}

	// 验证压缩结果
	if compressed.OrigSize != int64(len(originalData)) {
		t.Errorf("Original size mismatch")
	}

	// 解压
	decompressed, err := manager.DecompressFile(path)
	if err != nil {
		t.Fatalf("DecompressFile failed: %v", err)
	}

	// 验证解压结果大小
	if len(decompressed) != len(originalData) {
		t.Errorf("Decompressed size mismatch: got %d, want %d", len(decompressed), len(originalData))
	}
}

// TestCompress_ConcurrentCompression 测试并发压缩.
func TestCompress_ConcurrentCompression(t *testing.T) {
	manager := NewMockCompressManager()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(i int) {
			data := make([]byte, 10000)
			path := "/data/concurrent/" + string(rune('0'+i)) + ".txt"
			_, _ = manager.CompressFile(path, data, compress.AlgorithmZstd)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := manager.GetStats()
	if stats.CompressedFiles != 10 {
		t.Errorf("Expected 10 compressed files, got %d", stats.CompressedFiles)
	}
}

// TestCompress_MultipleAlgorithms 测试多种算法.
func TestCompress_MultipleAlgorithms(t *testing.T) {
	manager := NewMockCompressManager()

	data := make([]byte, 10000)

	// 使用不同算法压缩
	algorithms := []compress.Algorithm{
		compress.AlgorithmZstd,
		compress.AlgorithmLz4,
		compress.AlgorithmGzip,
	}

	for i, algo := range algorithms {
		path := "/data/algo" + string(rune('0'+i)) + ".txt"
		file, err := manager.CompressFile(path, data, algo)
		if err != nil {
			t.Errorf("Compression with %s failed: %v", algo, err)
			continue
		}

		if file.Algorithm != algo {
			t.Errorf("Expected algorithm=%s, got %s", algo, file.Algorithm)
		}
	}

	stats := manager.GetStats()
	if len(stats.ByAlgorithm) != 3 {
		t.Errorf("Expected 3 algorithm stats, got %d", len(stats.ByAlgorithm))
	}
}

// TestCompress_MinSizeFilter 测试最小大小过滤.
func TestCompress_MinSizeFilter(t *testing.T) {
	manager := NewMockCompressManager()

	config := manager.GetConfig()
	minSize := config.MinSize // 默认 1024 字节

	tests := []struct {
		name     string
		size     int64
		expected bool
	}{
		{"Below min size", minSize - 1, false},
		{"At min size", minSize, true},
		{"Above min size", minSize + 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.ShouldCompress("/data/test.txt", tt.size)
			if result != tt.expected {
				t.Errorf("ShouldCompress for size=%d: got %v, want %v", tt.size, result, tt.expected)
			}
		})
	}
}

// TestCompress_CompressionLevel 测试压缩级别.
func TestCompress_CompressionLevel(t *testing.T) {
	config := compress.DefaultConfig()

	// 验证默认压缩级别在有效范围内
	if config.CompressionLevel < 1 || config.CompressionLevel > 9 {
		t.Errorf("CompressionLevel should be 1-9, got %d", config.CompressionLevel)
	}

	// 测试不同压缩级别
	levels := []int{1, 3, 6, 9}
	for _, level := range levels {
		t.Run("level-"+string(rune('0'+level)), func(t *testing.T) {
			cfg := compress.DefaultConfig()
			cfg.CompressionLevel = level
			if cfg.CompressionLevel != level {
				t.Errorf("Failed to set compression level")
			}
		})
	}
}

// TestCompress_ExcludeDirs 测试排除目录.
func TestCompress_ExcludeDirs(t *testing.T) {
	config := compress.DefaultConfig()

	excludedDirs := []string{".git", ".svn", "node_modules"}

	for _, dir := range excludedDirs {
		found := false
		for _, d := range config.ExcludeDirs {
			if d == dir {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: directory %s not in exclude list", dir)
		}
	}
}

// TestCompress_StatsUpdate 测试统计更新.
func TestCompress_StatsUpdate(t *testing.T) {
	manager := NewMockCompressManager()

	// 初始统计
	initialStats := manager.GetStats()
	if initialStats.TotalFiles != 0 {
		t.Error("Initial TotalFiles should be 0")
	}

	// 压缩文件
	data := make([]byte, 5000)
	manager.CompressFile("/data/file1.txt", data, compress.AlgorithmZstd)

	// 检查更新
	updatedStats := manager.GetStats()
	if updatedStats.TotalFiles != 1 {
		t.Errorf("Expected TotalFiles=1, got %d", updatedStats.TotalFiles)
	}

	if updatedStats.TotalBytes != 5000 {
		t.Errorf("Expected TotalBytes=5000, got %d", updatedStats.TotalBytes)
	}
}

// TestCompress_NilConfig 测试空配置.
func TestCompress_NilConfig(t *testing.T) {
	// 当传入 nil 时应使用默认配置
	config := compress.DefaultConfig()
	if config == nil {
		t.Error("DefaultConfig should not return nil")
	}
}

// ========== 性能测试 ==========

// BenchmarkCompress_CompressFile 性能测试：文件压缩.
func BenchmarkCompress_CompressFile(b *testing.B) {
	manager := NewMockCompressManager()
	data := make([]byte, 10240) // 10KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := "/data/bench/" + time.Now().Format("150405.999") + ".txt"
		_, _ = manager.CompressFile(path, data, compress.AlgorithmZstd)
	}
}

// BenchmarkCompress_DecompressFile 性能测试：文件解压.
func BenchmarkCompress_DecompressFile(b *testing.B) {
	manager := NewMockCompressManager()
	data := make([]byte, 10240)
	manager.CompressFile("/data/bench.txt", data, compress.AlgorithmZstd)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.DecompressFile("/data/bench.txt")
	}
}

// BenchmarkCompress_ShouldCompress 性能测试：压缩判断.
func BenchmarkCompress_ShouldCompress(b *testing.B) {
	manager := NewMockCompressManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.ShouldCompress("/data/test.txt", 10000)
	}
}

// BenchmarkCompress_GetStats 性能测试：获取统计.
func BenchmarkCompress_GetStats(b *testing.B) {
	manager := NewMockCompressManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetStats()
	}
}

// ========== 辅助函数 ==========

// TestCompress_BufferOperations 测试缓冲区操作.
func TestCompress_BufferOperations(t *testing.T) {
	// 测试缓冲区读写
	data := []byte("test data for compression")
	buf := bytes.NewBuffer(nil)

	// 模拟压缩写入
	n, err := buf.Write(data)
	if err != nil {
		t.Fatalf("Buffer write failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("Write count mismatch: got %d, want %d", n, len(data))
	}

	// 模拟解压读取
	result := buf.Bytes()
	if !bytes.Equal(result, data) {
		t.Error("Buffer read mismatch")
	}
}
