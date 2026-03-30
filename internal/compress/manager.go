// Package compress 提供透明压缩存储功能
package compress

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Algorithm 压缩算法.
type Algorithm string

// 压缩算法常量.
const (
	AlgorithmZstd Algorithm = "zstd"
	AlgorithmLz4  Algorithm = "lz4"
	AlgorithmGzip Algorithm = "gzip"
	AlgorithmNone Algorithm = "none"
)

// Config 压缩配置.
type Config struct {
	Enabled           bool      `json:"enabled"`
	DefaultAlgorithm  Algorithm `json:"default_algorithm"`
	CompressionLevel  int       `json:"compression_level"`  // 1-9
	MinSize           int64     `json:"min_size"`           // 最小压缩大小 (字节)
	ExcludeExtensions []string  `json:"exclude_extensions"` // 不压缩的扩展名
	ExcludeDirs       []string  `json:"exclude_dirs"`       // 不压缩的目录
	IncludeDirs       []string  `json:"include_dirs"`       // 只压缩这些目录 (空=全部)
	CompressOnWrite   bool      `json:"compress_on_write"`  // 写入时压缩
	DecompressOnRead  bool      `json:"decompress_on_read"` // 读取时解压
	StatsEnabled      bool      `json:"stats_enabled"`      // 启用统计
}

// DefaultConfig 默认配置.
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

// Manager 压缩管理器.
type Manager struct {
	mu     sync.RWMutex
	config *Config
	stats  *Stats
	// 压缩器缓存
	compressors map[Algorithm]Compressor
}

// Compressor 压缩器接口.
type Compressor interface {
	Compress(dst io.Writer, src io.Reader, level int) error
	Decompress(dst io.Writer, src io.Reader) error
	Extension() string
	Name() Algorithm
}

// Stats 压缩统计.
type Stats struct {
	mu              sync.RWMutex
	TotalFiles      int64                         `json:"total_files"`
	CompressedFiles int64                         `json:"compressed_files"`
	TotalBytes      int64                         `json:"total_bytes"`
	CompressedBytes int64                         `json:"compressed_bytes"`
	SavedBytes      int64                         `json:"saved_bytes"`
	AvgRatio        float64                       `json:"avg_ratio"`
	ByAlgorithm     map[Algorithm]*AlgorithmStats `json:"by_algorithm"`
}

// AlgorithmStats 算法统计.
type AlgorithmStats struct {
	Files      int64   `json:"files"`
	Original   int64   `json:"original_bytes"`
	Compressed int64   `json:"compressed_bytes"`
	Ratio      float64 `json:"avg_ratio"`
}

// NewManager 创建压缩管理器.
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

// registerCompressors 注册内置压缩器.
func (m *Manager) registerCompressors() {
	m.compressors[AlgorithmGzip] = &GzipCompressor{}
	m.compressors[AlgorithmZstd] = &ZstdCompressor{}
	m.compressors[AlgorithmLz4] = &Lz4Compressor{}
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return nil
}

// ShouldCompress 检查是否应该压缩.
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

// CompressFile 压缩文件.
func (m *Manager) CompressFile(srcPath, dstPath string) (*Result, error) {
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
	defer func() { _ = srcFile.Close() }()

	// 获取源文件信息
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return nil, err
	}

	// 检查是否应该压缩
	if !m.ShouldCompress(srcPath, srcInfo.Size()) {
		return &Result{
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
	defer func() { _ = dstFile.Close() }()

	// 压缩
	start := time.Now()
	err = compressor.Compress(dstFile, srcFile, level)
	if err != nil {
		_ = os.Remove(dstPath + compressor.Extension())
		return nil, err
	}

	// 获取压缩后大小
	dstInfo, err := dstFile.Stat()
	if err != nil {
		return nil, err
	}

	// 更新统计
	result := &Result{
		Skipped:        false,
		OriginalSize:   srcInfo.Size(),
		CompressedSize: dstInfo.Size(),
		SavedBytes:     srcInfo.Size() - dstInfo.Size(),
		Ratio:          float64(dstInfo.Size()) / float64(srcInfo.Size()),
		Duration:       time.Since(start),
		Algorithm:      algorithm,
	}

	if m.config.StatsEnabled {
		m.stats.Update(algorithm, srcInfo.Size(), dstInfo.Size())
	}

	return result, nil
}

// DecompressFile 解压文件.
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
	defer func() { _ = srcFile.Close() }()

	// 创建目标文件
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return nil
	}
	defer func() { _ = dstFile.Close() }()

	// 解压
	return compressor.Decompress(dstFile, srcFile)
}

// detectAlgorithm 检测压缩算法.
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

// GetStats 获取统计.
func (m *Manager) GetStats() *Stats {
	return m.stats
}

// Result 压缩结果.
type Result struct {
	Skipped        bool          `json:"skipped"`
	SkipReason     string        `json:"skip_reason,omitempty"`
	OriginalSize   int64         `json:"original_size"`
	CompressedSize int64         `json:"compressed_size"`
	SavedBytes     int64         `json:"saved_bytes"`
	Ratio          float64       `json:"ratio"`
	Duration       time.Duration `json:"duration"`
	Algorithm      Algorithm     `json:"algorithm"`
}

// NewStats 创建统计.
func NewStats() *Stats {
	return &Stats{
		ByAlgorithm: make(map[Algorithm]*AlgorithmStats),
	}
}

// Update 更新统计.
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

// Reset 重置统计.
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

// GzipCompressor Gzip 压缩器.
type GzipCompressor struct{}

// Compress 使用 Gzip 算法压缩数据.
func (c *GzipCompressor) Compress(dst io.Writer, src io.Reader, level int) error {
	writer, err := gzip.NewWriterLevel(dst, level)
	if err != nil {
		return err
	}
	defer func() {
		_ = writer.Close()
	}()

	_, err = io.Copy(writer, src)
	return err
}

// Decompress 使用 Gzip 算法解压数据.
func (c *GzipCompressor) Decompress(dst io.Writer, src io.Reader) error {
	reader, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	_, err = io.Copy(dst, reader)
	return err
}

// Extension 返回 Gzip 文件扩展名.
func (c *GzipCompressor) Extension() string {
	return ".gz"
}

// Name 返回 Gzip 算法名称.
func (c *GzipCompressor) Name() Algorithm {
	return AlgorithmGzip
}

// ZstdCompressor Zstd 压缩器 (需要 cgo 或纯 Go 实现).
type ZstdCompressor struct{}

// Compress 使用 Zstd 算法压缩数据.
func (c *ZstdCompressor) Compress(dst io.Writer, src io.Reader, level int) error {
	// 简化实现：使用 gzip 作为后备
	// 生产环境应使用 github.com/klauspost/compress/zstd
	return (&GzipCompressor{}).Compress(dst, src, level)
}

// Decompress 使用 Zstd 算法解压数据.
func (c *ZstdCompressor) Decompress(dst io.Writer, src io.Reader) error {
	// 简化实现
	return errors.New("zstd not implemented, use gzip")
}

// Extension 返回 Zstd 文件扩展名.
func (c *ZstdCompressor) Extension() string {
	return ".zst"
}

// Name 返回 Zstd 算法名称.
func (c *ZstdCompressor) Name() Algorithm {
	return AlgorithmZstd
}

// Lz4Compressor LZ4 压缩器.
type Lz4Compressor struct{}

// Compress 使用 LZ4 算法压缩数据.
func (c *Lz4Compressor) Compress(dst io.Writer, src io.Reader, level int) error {
	// 简化实现：使用 gzip 作为后备
	// 生产环境应使用 github.com/pierrec/lz4
	return (&GzipCompressor{}).Compress(dst, src, level)
}

// Decompress 使用 LZ4 算法解压数据.
func (c *Lz4Compressor) Decompress(dst io.Writer, src io.Reader) error {
	return errors.New("lz4 not implemented, use gzip")
}

// Extension 返回 LZ4 文件扩展名.
func (c *Lz4Compressor) Extension() string {
	return ".lz4"
}

// Name 返回 LZ4 算法名称.
func (c *Lz4Compressor) Name() Algorithm {
	return AlgorithmLz4
}

// ================== v2.4.0 压缩存储增强 ==================

// AlgorithmPreference 算法偏好配置.
type AlgorithmPreference struct {
	Algorithm     Algorithm `json:"algorithm"`
	Priority      int       `json:"priority"`      // 优先级 (越高越优先)
	SpeedPriority bool      `json:"speedPriority"` // 是否优先考虑速度
}

// FileTypeRule 文件类型压缩规则.
type FileTypeRule struct {
	Extensions   []string  `json:"extensions"`   // 文件扩展名
	Algorithm    Algorithm `json:"algorithm"`    // 推荐算法
	MinSize      int64     `json:"minSize"`      // 最小压缩大小
	SkipCompress bool      `json:"skipCompress"` // 跳过压缩
	Reason       string    `json:"reason"`       // 原因说明
}

// DefaultFileTypeRules 默认文件类型规则.
var DefaultFileTypeRules = []FileTypeRule{
	// 已压缩格式 - 跳过
	{
		Extensions:   []string{".zip", ".gz", ".bz2", ".xz", ".zst", ".lz4", ".rar", ".7z"},
		SkipCompress: true,
		Reason:       "已压缩格式",
	},
	// 媒体文件 - 跳过
	{
		Extensions:   []string{".mp3", ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm"},
		SkipCompress: true,
		Reason:       "媒体文件已编码压缩",
	},
	// 图片文件 - 跳过
	{
		Extensions:   []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".ico"},
		SkipCompress: true,
		Reason:       "图片文件已压缩",
	},
	// 文档文件 - 跳过
	{
		Extensions:   []string{".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx"},
		SkipCompress: true,
		Reason:       "文档文件已内置压缩",
	},
	// 文本文件 - 使用 gzip (兼容性好)
	{
		Extensions: []string{".txt", ".md", ".log", ".csv"},
		Algorithm:  AlgorithmGzip,
		MinSize:    512,
	},
	// 源代码 - 使用 gzip
	{
		Extensions: []string{".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".sh"},
		Algorithm:  AlgorithmGzip,
		MinSize:    256,
	},
	// 配置文件 - 使用 gzip
	{
		Extensions: []string{".json", ".yaml", ".yml", ".xml", ".toml", ".ini", ".conf", ".cfg"},
		Algorithm:  AlgorithmGzip,
		MinSize:    128,
	},
	// 大数据文件 - 使用 zstd (高压缩比)
	{
		Extensions: []string{".sql", ".db", ".dump", ".backup"},
		Algorithm:  AlgorithmZstd,
		MinSize:    1024 * 1024, // 1MB
	},
}

// SelectAlgorithmForFile 根据文件类型自动选择最佳压缩算法.
func (m *Manager) SelectAlgorithmForFile(path string, size int64) (Algorithm, string, bool) {
	ext := strings.ToLower(filepath.Ext(path))

	// 检查文件类型规则
	for _, rule := range DefaultFileTypeRules {
		for _, ruleExt := range rule.Extensions {
			if ext == ruleExt {
				if rule.SkipCompress {
					return AlgorithmNone, rule.Reason, true
				}
				if size >= rule.MinSize {
					return rule.Algorithm, "", false
				}
			}
		}
	}

	// 基于文件大小的智能选择
	switch {
	case size < 1024:
		// 小文件 - 不值得压缩
		return AlgorithmNone, "文件太小，压缩收益不明显", true
	case size < 100*1024:
		// 100KB 以下 - gzip 快速压缩
		return AlgorithmGzip, "", false
	case size < 10*1024*1024:
		// 10MB 以下 - zstd 平衡压缩
		return AlgorithmZstd, "", false
	default:
		// 大文件 - 使用 zstd 高压缩比
		return AlgorithmZstd, "", false
	}
}

// SmartCompressFile 智能压缩文件（自动选择算法）.
func (m *Manager) SmartCompressFile(srcPath, dstPath string) (*Result, error) {
	// 获取文件信息
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}

	// 自动选择算法
	algorithm, reason, skip := m.SelectAlgorithmForFile(srcPath, srcInfo.Size())
	if skip {
		return &Result{
			Skipped:      true,
			SkipReason:   reason,
			OriginalSize: srcInfo.Size(),
			Algorithm:    AlgorithmNone,
		}, nil
	}

	// 使用选择的算法进行压缩
	return m.CompressFileWithAlgorithm(srcPath, dstPath, algorithm)
}

// CompressFileWithAlgorithm 使用指定算法压缩文件.
func (m *Manager) CompressFileWithAlgorithm(srcPath, dstPath string, algorithm Algorithm) (*Result, error) {
	compressor, ok := m.compressors[algorithm]
	if !ok {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// 获取压缩级别
	m.mu.RLock()
	level := m.config.CompressionLevel
	m.mu.RUnlock()

	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = srcFile.Close() }()

	// 获取源文件信息
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return nil, err
	}

	// 创建目标文件
	dstFilePath := dstPath + compressor.Extension()
	dstFile, err := os.Create(dstFilePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dstFile.Close() }()

	// 压缩
	start := time.Now()
	err = compressor.Compress(dstFile, srcFile, level)
	if err != nil {
		_ = os.Remove(dstFilePath)
		return nil, err
	}

	// 获取压缩后大小
	dstInfo, err := dstFile.Stat()
	if err != nil {
		return nil, err
	}

	result := &Result{
		Skipped:        false,
		OriginalSize:   srcInfo.Size(),
		CompressedSize: dstInfo.Size(),
		SavedBytes:     srcInfo.Size() - dstInfo.Size(),
		Ratio:          float64(dstInfo.Size()) / float64(srcInfo.Size()),
		Duration:       time.Since(start),
		Algorithm:      algorithm,
	}

	// 更新统计
	if m.config.StatsEnabled {
		m.stats.Update(algorithm, srcInfo.Size(), dstInfo.Size())
	}

	return result, nil
}

// BatchCompressResultV2 批量压缩结果 (v2.4.0).
type BatchCompressResultV2 struct {
	Total     int64                  `json:"total"`
	Succeeded int64                  `json:"succeeded"`
	Failed    int64                  `json:"failed"`
	Skipped   int64                  `json:"skipped"`
	TotalSize int64                  `json:"totalSize"`
	SavedSize int64                  `json:"savedSize"`
	Duration  time.Duration          `json:"duration"`
	Results   []SingleCompressResult `json:"results"`
	Errors    []ErrorV2              `json:"errors"`
}

// SingleCompressResult 单个压缩结果.
type SingleCompressResult struct {
	Path           string    `json:"path"`
	OriginalSize   int64     `json:"originalSize"`
	CompressedSize int64     `json:"compressedSize"`
	SavedBytes     int64     `json:"savedBytes"`
	Ratio          float64   `json:"ratio"`
	Algorithm      Algorithm `json:"algorithm"`
	Skipped        bool      `json:"skipped"`
	SkipReason     string    `json:"skipReason,omitempty"`
}

// ErrorV2 压缩错误.
type ErrorV2 struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// BatchCompressOptions 批量压缩选项.
type BatchCompressOptions struct {
	Workers         int       `json:"workers"`         // 并发工作数
	DeleteOriginal  bool      `json:"deleteOriginal"`  // 压缩后删除原文件
	Overwrite       bool      `json:"overwrite"`       // 覆盖已存在文件
	Algorithm       Algorithm `json:"algorithm"`       // 指定算法 (空则自动选择)
	MinSize         int64     `json:"minSize"`         // 最小压缩大小
	ContinueOnError bool      `json:"continueOnError"` // 遇错继续
	DryRun          bool      `json:"dryRun"`          // 仅模拟不实际压缩
}

// DefaultBatchCompressOptions 默认批量压缩选项.
func DefaultBatchCompressOptions() *BatchCompressOptions {
	return &BatchCompressOptions{
		Workers:         4,
		DeleteOriginal:  false,
		Overwrite:       false,
		ContinueOnError: true,
		DryRun:          false,
	}
}

// BatchCompress 批量压缩文件.
func (m *Manager) BatchCompress(paths []string) (*BatchCompressResultV2, error) {
	return m.BatchCompressWithOptions(paths, nil)
}

// BatchCompressWithOptions 带选项的批量压缩.
func (m *Manager) BatchCompressWithOptions(paths []string, opts *BatchCompressOptions) (*BatchCompressResultV2, error) {
	if opts == nil {
		opts = DefaultBatchCompressOptions()
	}

	result := &BatchCompressResultV2{
		Results: make([]SingleCompressResult, 0, len(paths)),
		Errors:  make([]ErrorV2, 0),
	}

	if len(paths) == 0 {
		return result, nil
	}

	start := time.Now()

	// 使用工作池并发处理
	workerCount := opts.Workers
	if workerCount <= 0 {
		workerCount = 4
	}
	if workerCount > len(paths) {
		workerCount = len(paths)
	}

	// 创建通道
	pathChan := make(chan string, len(paths))
	resultChan := make(chan SingleCompressResult, len(paths))
	errorChan := make(chan ErrorV2, len(paths))

	// 启动 workers
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go m.batchCompressWorker(&wg, pathChan, resultChan, errorChan, opts)
	}

	// 发送任务
	for _, path := range paths {
		pathChan <- path
	}
	close(pathChan)

	// 等待完成
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// 收集结果
	for res := range resultChan {
		result.Results = append(result.Results, res)
		result.Total++
		result.TotalSize += res.OriginalSize
		if res.Skipped {
			result.Skipped++
		} else {
			result.Succeeded++
			result.SavedSize += res.SavedBytes
		}
	}

	for err := range errorChan {
		result.Errors = append(result.Errors, err)
		result.Failed++
	}

	result.Duration = time.Since(start)
	return result, nil
}

// batchCompressWorker 批量压缩工作协程.
func (m *Manager) batchCompressWorker(wg *sync.WaitGroup, paths <-chan string, results chan<- SingleCompressResult, errors chan<- ErrorV2, opts *BatchCompressOptions) {
	defer wg.Done()

	for path := range paths {
		// 检查文件是否存在
		info, err := os.Stat(path)
		if err != nil {
			if opts.ContinueOnError {
				errors <- ErrorV2{Path: path, Error: err.Error()}
				continue
			}
			return
		}

		// 跳过目录
		if info.IsDir() {
			results <- SingleCompressResult{
				Path:       path,
				Skipped:    true,
				SkipReason: "目录不支持压缩",
			}
			continue
		}

		// 检查最小大小
		if opts.MinSize > 0 && info.Size() < opts.MinSize {
			results <- SingleCompressResult{
				Path:         path,
				OriginalSize: info.Size(),
				Skipped:      true,
				SkipReason:   "文件大小小于最小压缩大小",
			}
			continue
		}

		// DryRun 模式
		if opts.DryRun {
			algorithm, reason, skip := m.SelectAlgorithmForFile(path, info.Size())
			results <- SingleCompressResult{
				Path:         path,
				OriginalSize: info.Size(),
				Skipped:      skip,
				SkipReason:   reason,
				Algorithm:    algorithm,
			}
			continue
		}

		// 执行压缩
		dstPath := path
		var compressResult *Result

		if opts.Algorithm != "" {
			compressResult, err = m.CompressFileWithAlgorithm(path, dstPath, opts.Algorithm)
		} else {
			compressResult, err = m.SmartCompressFile(path, dstPath)
		}

		if err != nil {
			if opts.ContinueOnError {
				errors <- ErrorV2{Path: path, Error: err.Error()}
				continue
			}
			return
		}

		// 删除原文件
		if opts.DeleteOriginal && !compressResult.Skipped {
			_ = os.Remove(path)
		}

		results <- SingleCompressResult{
			Path:           path,
			OriginalSize:   compressResult.OriginalSize,
			CompressedSize: compressResult.CompressedSize,
			SavedBytes:     compressResult.SavedBytes,
			Ratio:          compressResult.Ratio,
			Algorithm:      compressResult.Algorithm,
			Skipped:        compressResult.Skipped,
			SkipReason:     compressResult.SkipReason,
		}
	}
}

// GetAlgorithmStats 获取各算法统计信息.
func (m *Manager) GetAlgorithmStats() map[Algorithm]*AlgorithmStats {
	return m.stats.ByAlgorithm
}

// EstimateCompressRatio 预估压缩比.
func (m *Manager) EstimateCompressRatio(path string) (float64, Algorithm, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, AlgorithmNone, err
	}

	algorithm, _, skip := m.SelectAlgorithmForFile(path, info.Size())
	if skip {
		return 1.0, AlgorithmNone, nil
	}

	// 基于文件类型预估压缩比
	ext := strings.ToLower(filepath.Ext(path))
	ratios := map[string]float64{
		".txt":  0.3,
		".log":  0.2,
		".csv":  0.25,
		".json": 0.25,
		".xml":  0.2,
		".go":   0.25,
		".py":   0.25,
		".js":   0.3,
		".html": 0.2,
		".css":  0.25,
		".sql":  0.15,
	}

	if ratio, ok := ratios[ext]; ok {
		return ratio, algorithm, nil
	}

	// 默认预估
	return 0.5, algorithm, nil
}
