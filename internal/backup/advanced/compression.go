package advanced

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
)

// ========== 默认压缩器 ==========

// DefaultCompressor 默认压缩器（支持多种算法）
type DefaultCompressor struct {
	algorithm CompressionAlgorithm
	level     int
	mu        sync.Mutex
}

// NewDefaultCompressor 创建默认压缩器
func NewDefaultCompressor(config *CompressionConfig) *DefaultCompressor {
	if config == nil {
		config = DefaultCompressionConfig()
	}
	return &DefaultCompressor{
		algorithm: config.Algorithm,
		level:     config.Level,
	}
}

// Compress 压缩数据
func (c *DefaultCompressor) Compress(data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.algorithm {
	case CompressionNone:
		return data, nil
	case CompressionGzip:
		return c.compressGzip(data)
	case CompressionBzip2:
		// bzip2只支持解压，压缩需要外部库
		return nil, fmt.Errorf("%w: bzip2 compression not implemented", ErrUnsupportedAlgorithm)
	case CompressionZstd:
		return nil, fmt.Errorf("%w: zstd compression not implemented", ErrUnsupportedAlgorithm)
	case CompressionLz4:
		return nil, fmt.Errorf("%w: lz4 compression not implemented", ErrUnsupportedAlgorithm)
	case CompressionXz:
		return nil, fmt.Errorf("%w: xz compression not implemented", ErrUnsupportedAlgorithm)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, c.algorithm)
	}
}

// Decompress 解压数据
func (c *DefaultCompressor) Decompress(data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.algorithm {
	case CompressionNone:
		return data, nil
	case CompressionGzip:
		return c.decompressGzip(data)
	case CompressionBzip2:
		return c.decompressBzip2(data)
	case CompressionZstd:
		return nil, fmt.Errorf("%w: zstd decompression not implemented", ErrUnsupportedAlgorithm)
	case CompressionLz4:
		return nil, fmt.Errorf("%w: lz4 decompression not implemented", ErrUnsupportedAlgorithm)
	case CompressionXz:
		return nil, fmt.Errorf("%w: xz decompression not implemented", ErrUnsupportedAlgorithm)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, c.algorithm)
	}
}

// Algorithm 获取算法名称
func (c *DefaultCompressor) Algorithm() CompressionAlgorithm {
	return c.algorithm
}

// compressGzip Gzip压缩
func (c *DefaultCompressor) compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	level := c.level
	if level < 1 {
		level = gzip.DefaultCompression
	} else if level > 9 {
		level = gzip.BestCompression
	}

	writer, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		writer = gzip.NewWriter(&buf)
	}

	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("gzip compression failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gzip writer close failed: %w", err)
	}

	return buf.Bytes(), nil
}

// decompressGzip Gzip解压
func (c *DefaultCompressor) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader creation failed: %w", err)
	}
	defer func() { _ = reader.Close() }()

	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("gzip decompression failed: %w", err)
	}

	return result, nil
}

// decompressBzip2 Bzip2解压
func (c *DefaultCompressor) decompressBzip2(data []byte) ([]byte, error) {
	reader := bzip2.NewReader(bytes.NewReader(data))

	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("bzip2 decompression failed: %w", err)
	}

	return result, nil
}

// ========== 压缩算法信息 ==========

// CompressionInfo 压缩算法信息
type CompressionInfo struct {
	Name         string               `json:"name"`
	Algorithm    CompressionAlgorithm `json:"algorithm"`
	Speed        string               `json:"speed"` // fast, medium, slow
	Ratio        string               `json:"ratio"` // low, medium, high
	LevelMin     int                  `json:"levelMin"`
	LevelMax     int                  `json:"levelMax"`
	LevelDefault int                  `json:"levelDefault"`
	Description  string               `json:"description"`
}

// SupportedCompressionAlgorithms 支持的压缩算法列表
func SupportedCompressionAlgorithms() []CompressionInfo {
	return []CompressionInfo{
		{
			Name:         "无压缩",
			Algorithm:    CompressionNone,
			Speed:        "fastest",
			Ratio:        "none",
			LevelMin:     0,
			LevelMax:     0,
			LevelDefault: 0,
			Description:  "不进行压缩，速度最快但无空间节省",
		},
		{
			Name:         "Gzip",
			Algorithm:    CompressionGzip,
			Speed:        "medium",
			Ratio:        "medium",
			LevelMin:     1,
			LevelMax:     9,
			LevelDefault: 6,
			Description:  "最常用的压缩算法，平衡速度和压缩率",
		},
		{
			Name:         "Zstandard",
			Algorithm:    CompressionZstd,
			Speed:        "fast",
			Ratio:        "high",
			LevelMin:     1,
			LevelMax:     22,
			LevelDefault: 3,
			Description:  "现代压缩算法，高速高压缩率（需要外部库）",
		},
		{
			Name:         "LZ4",
			Algorithm:    CompressionLz4,
			Speed:        "fastest",
			Ratio:        "low",
			LevelMin:     0,
			LevelMax:     0,
			LevelDefault: 0,
			Description:  "极速压缩算法，适合实时备份（需要外部库）",
		},
		{
			Name:         "Bzip2",
			Algorithm:    CompressionBzip2,
			Speed:        "slow",
			Ratio:        "high",
			LevelMin:     1,
			LevelMax:     9,
			LevelDefault: 9,
			Description:  "高压缩率算法，适合存储空间紧张场景",
		},
		{
			Name:         "XZ",
			Algorithm:    CompressionXz,
			Speed:        "slowest",
			Ratio:        "highest",
			LevelMin:     0,
			LevelMax:     9,
			LevelDefault: 6,
			Description:  "最高压缩率算法，适合长期归档（需要外部库）",
		},
	}
}

// GetCompressionInfo 获取压缩算法信息
func GetCompressionInfo(algorithm CompressionAlgorithm) *CompressionInfo {
	for _, info := range SupportedCompressionAlgorithms() {
		if info.Algorithm == algorithm {
			return &info
		}
	}
	return nil
}

// EstimateCompressionRatio 估算压缩率
func EstimateCompressionRatio(algorithm CompressionAlgorithm) float64 {
	switch algorithm {
	case CompressionNone:
		return 1.0
	case CompressionGzip:
		return 0.35 // 平均65%压缩
	case CompressionZstd:
		return 0.30 // 平均70%压缩
	case CompressionLz4:
		return 0.50 // 平均50%压缩
	case CompressionBzip2:
		return 0.30 // 平均70%压缩
	case CompressionXz:
		return 0.25 // 平均75%压缩
	default:
		return 0.5
	}
}

// ========== 压缩统计 ==========

// CompressionStats 压缩统计
type CompressionStats struct {
	Algorithm         CompressionAlgorithm `json:"algorithm"`
	OriginalSize      int64                `json:"originalSize"`
	CompressedSize    int64                `json:"compressedSize"`
	CompressionRatio  float64              `json:"compressionRatio"`
	SpaceSaved        int64                `json:"spaceSaved"`
	SpaceSavedPercent float64              `json:"spaceSavedPercent"`
	CompressionTime   int64                `json:"compressionTimeMs"`
	DecompressionTime int64                `json:"decompressionTimeMs"`
}

// CalculateStats 计算压缩统计
func CalculateStats(algorithm CompressionAlgorithm, originalSize, compressedSize int64) *CompressionStats {
	var ratio float64
	if originalSize > 0 {
		ratio = float64(compressedSize) / float64(originalSize)
	}

	var savedPercent float64
	if originalSize > 0 {
		savedPercent = (1 - float64(compressedSize)/float64(originalSize)) * 100
	}

	return &CompressionStats{
		Algorithm:         algorithm,
		OriginalSize:      originalSize,
		CompressedSize:    compressedSize,
		CompressionRatio:  ratio,
		SpaceSaved:        originalSize - compressedSize,
		SpaceSavedPercent: savedPercent,
	}
}
