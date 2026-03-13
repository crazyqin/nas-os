// Package compress 提供透明压缩存储功能
package compress

import (
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Algorithm 压缩算法
type Algorithm string

const (
	AlgorithmZstd Algorithm = "zstd"
	AlgorithmLz4  Algorithm = "lz4"
	AlgorithmGzip Algorithm = "gzip"
	AlgorithmNone Algorithm = "none"
)

// Config 压缩配置
type Config struct {
	Enabled           bool              `json:"enabled"`
	DefaultAlgorithm  Algorithm         `json:"default_algorithm"`
	CompressionLevel  int               `json:"compression_level"` // 1-9
	MinSize           int64             `json:"min_size"`          // 最小压缩大小 (字节)
	ExcludeExtensions []string          `json:"exclude_extensions"` // 不压缩的扩展名
	ExcludeDirs       []string          `json:"exclude_dirs"`       // 不压缩的目录
	IncludeDirs       []string          `json:"include_dirs"`       // 只压缩这些目录 (空=全部)
	CompressOnWrite   bool              `json:"compress_on_write"`  // 写入时压缩
	DecompressOnRead  bool              `json:"decompress_on_read"` // 读取时解压
	StatsEnabled      bool              `json:"stats_enabled"`      // 启用统计
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		DefaultAlgorithm: AlgorithmZstd,
		CompressionLevel: 6,
		MinSize:          1024, // 1KB
		ExcludeExtensions: []string{
			".zip", ".gz", ".bz2", ".xz", ".zst", ".lz4",
			".mp3", ".mp4", ".avi", ".mkv", ".mov",
			".jpg", ".jpeg", ".png", ".gif", ".webp",
			".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx",
		},
		ExcludeDirs:      []string{".git", ".svn", "node_modules"},
		IncludeDirs:      []string{},
		CompressOnWrite:  true,
		DecompressOnRead: true,
		StatsEnabled:     true,
	}
}

// Manager 压缩管理器
type Manager struct {
	mu     sync.RWMutex
	config *Config
	stats  *Stats
	// 压缩器缓存
	compressors map[Algorithm]Compressor
}

// Compressor 压缩器接口
type Compressor interface {
	Compress(dst io.Writer, src io.Reader, level int) error
	Decompress(dst io.Writer, src io.Reader) error
	Extension() string
	Name() Algorithm
}

// Stats 压缩统计
type Stats struct {
	mu              sync.RWMutex
	TotalFiles      int64   `json:"total_files"`
	CompressedFiles int64   `json:"compressed_files"`
	TotalBytes      int64   `json:"total_bytes"`
	CompressedBytes int64   `json:"compressed_bytes"`
	SavedBytes      int64   `json:"saved_bytes"`
	AvgRatio        float64 `json:"avg_ratio"`
	ByAlgorithm     map[Algorithm]*AlgorithmStats `json:"by_algorithm"`
}

// AlgorithmStats 算法统计
type AlgorithmStats struct {
	Files    int64   `json:"files"`
	Original int64   `json:"original_bytes"`
	Compressed int64 `json:"compressed_bytes"`
	Ratio    float64 `json:"avg_ratio"`
}

// NewManager 创建压缩管理器
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		config:      config,
		stats:       NewStats(),
		compressors: make(map[Algorithm]Compressor),
	}

	// 注册内置压缩器
	m.registerCompressors()

	return m, nil
}

// registerCompressors 注册内置压缩器
func (m *Manager) registerCompressors() {
	m.compressors[AlgorithmGzip] = &GzipCompressor{}
	m.compressors[AlgorithmZstd] = &ZstdCompressor{}
	m.compressors[AlgorithmLz4] = &Lz4Compressor{}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return nil
}

// ShouldCompress 检查是否应该压缩
func (m *Manager) ShouldCompress(path string, size int64) bool {
	if !m.config.Enabled {
		return false
	}

	// 检查最小大小
	if size < m.config.MinSize {
		return false
	}

	// 检查扩展名
	ext := strings.ToLower(filepath.Ext(path))
	for _, exclude := range m.config.ExcludeExtensions {
		if ext == exclude {
			return false
		}
	}

	// 检查目录
	dir := filepath.Dir(path)
	for _, excludeDir := range m.config.ExcludeDirs {
		if strings.Contains(dir, excludeDir) {
			return false
		}
	}

	// 如果指定了包含目录，检查是否在列表中
	if len(m.config.IncludeDirs) > 0 {
		included := false
		for _, includeDir := range m.config.IncludeDirs {
			if strings.HasPrefix(dir, includeDir) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	return true
}

// CompressFile 压缩文件
func (m *Manager) CompressFile(srcPath, dstPath string) (*CompressResult, error) {
	m.mu.RLock()
	algorithm := m.config.DefaultAlgorithm
	level := m.config.CompressionLevel
	m.mu.RUnlock()

	compressor, ok := m.compressors[algorithm]
	if !ok {
		return nil, errors.New("unsupported algorithm")
	}

	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()

	// 获取源文件信息
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return nil, err
	}

	// 检查是否应该压缩
	if !m.ShouldCompress(srcPath, srcInfo.Size()) {
		return &CompressResult{
			Skipped:      true,
			SkipReason:   "不符合压缩条件",
			OriginalSize: srcInfo.Size(),
		}, nil
	}

	// 创建目标文件
	dstFile, err := os.Create(dstPath + compressor.Extension())
	if err != nil {
		return nil, err
	}
	defer dstFile.Close()

	// 压缩
	start := time.Now()
	err = compressor.Compress(dstFile, srcFile, level)
	if err != nil {
		os.Remove(dstPath + compressor.Extension())
		return nil, err
	}

	// 获取压缩后大小
	dstInfo, err := dstFile.Stat()
	if err != nil {
		return nil, err
	}

	// 更新统计
	result := &CompressResult{
		Skipped:       false,
		OriginalSize:  srcInfo.Size(),
		CompressedSize: dstInfo.Size(),
		SavedBytes:    srcInfo.Size() - dstInfo.Size(),
		Ratio:         float64(dstInfo.Size()) / float64(srcInfo.Size()),
		Duration:      time.Since(start),
		Algorithm:     algorithm,
	}

	if m.config.StatsEnabled {
		m.stats.Update(algorithm, srcInfo.Size(), dstInfo.Size())
	}

	return result, nil
}

// DecompressFile 解压文件
func (m *Manager) DecompressFile(srcPath, dstPath string) error {
	// 检测算法
	algorithm := m.detectAlgorithm(srcPath)
	if algorithm == AlgorithmNone {
		return errors.New("unknown compression format")
	}

	compressor, ok := m.compressors[algorithm]
	if !ok {
		return errors.New("unsupported algorithm")
	}

	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return nil
	}
	defer dstFile.Close()

	// 解压
	return compressor.Decompress(dstFile, srcFile)
}

// detectAlgorithm 检测压缩算法
func (m *Manager) detectAlgorithm(path string) Algorithm {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".gz":
		return AlgorithmGzip
	case ".zst":
		return AlgorithmZstd
	case ".lz4":
		return AlgorithmLz4
	default:
		return AlgorithmNone
	}
}

// GetStats 获取统计
func (m *Manager) GetStats() *Stats {
	return m.stats
}

// CompressResult 压缩结果
type CompressResult struct {
	Skipped        bool      `json:"skipped"`
	SkipReason     string    `json:"skip_reason,omitempty"`
	OriginalSize   int64     `json:"original_size"`
	CompressedSize int64     `json:"compressed_size"`
	SavedBytes     int64     `json:"saved_bytes"`
	Ratio          float64   `json:"ratio"`
	Duration       time.Duration `json:"duration"`
	Algorithm      Algorithm `json:"algorithm"`
}

// NewStats 创建统计
func NewStats() *Stats {
	return &Stats{
		ByAlgorithm: make(map[Algorithm]*AlgorithmStats),
	}
}

// Update 更新统计
func (s *Stats) Update(algorithm Algorithm, original, compressed int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalFiles++
	s.TotalBytes += original
	s.CompressedBytes += compressed
	s.SavedBytes += original - compressed

	if s.TotalFiles > 0 {
		s.AvgRatio = float64(s.CompressedBytes) / float64(s.TotalBytes)
	}

	// 更新算法统计
	if _, ok := s.ByAlgorithm[algorithm]; !ok {
		s.ByAlgorithm[algorithm] = &AlgorithmStats{}
	}
	stats := s.ByAlgorithm[algorithm]
	stats.Files++
	stats.Original += original
	stats.Compressed += compressed
	if stats.Original > 0 {
		stats.Ratio = float64(stats.Compressed) / float64(stats.Original)
	}

	if compressed < original {
		s.CompressedFiles++
	}
}

// Reset 重置统计
func (s *Stats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalFiles = 0
	s.CompressedFiles = 0
	s.TotalBytes = 0
	s.CompressedBytes = 0
	s.SavedBytes = 0
	s.AvgRatio = 0
	s.ByAlgorithm = make(map[Algorithm]*AlgorithmStats)
}

// GzipCompressor Gzip 压缩器
type GzipCompressor struct{}

func (c *GzipCompressor) Compress(dst io.Writer, src io.Reader, level int) error {
	writer, err := gzip.NewWriterLevel(dst, level)
	if err != nil {
		return err
	}
	defer writer.Close()

	_, err = io.Copy(writer, src)
	return err
}

func (c *GzipCompressor) Decompress(dst io.Writer, src io.Reader) error {
	reader, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(dst, reader)
	return err
}

func (c *GzipCompressor) Extension() string {
	return ".gz"
}

func (c *GzipCompressor) Name() Algorithm {
	return AlgorithmGzip
}

// ZstdCompressor Zstd 压缩器 (需要 cgo 或纯 Go 实现)
type ZstdCompressor struct{}

func (c *ZstdCompressor) Compress(dst io.Writer, src io.Reader, level int) error {
	// 简化实现：使用 gzip 作为后备
	// 生产环境应使用 github.com/klauspost/compress/zstd
	return (&GzipCompressor{}).Compress(dst, src, level)
}

func (c *ZstdCompressor) Decompress(dst io.Writer, src io.Reader) error {
	// 简化实现
	return errors.New("zstd not implemented, use gzip")
}

func (c *ZstdCompressor) Extension() string {
	return ".zst"
}

func (c *ZstdCompressor) Name() Algorithm {
	return AlgorithmZstd
}

// Lz4Compressor LZ4 压缩器
type Lz4Compressor struct{}

func (c *Lz4Compressor) Compress(dst io.Writer, src io.Reader, level int) error {
	// 简化实现：使用 gzip 作为后备
	// 生产环境应使用 github.com/pierrec/lz4
	return (&GzipCompressor{}).Compress(dst, src, level)
}

func (c *Lz4Compressor) Decompress(dst io.Writer, src io.Reader) error {
	return errors.New("lz4 not implemented, use gzip")
}

func (c *Lz4Compressor) Extension() string {
	return ".lz4"
}

func (c *Lz4Compressor) Name() Algorithm {
	return AlgorithmLz4
}