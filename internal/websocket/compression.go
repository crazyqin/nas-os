// Package websocket 提供消息压缩功能
// Version: v2.48.0 - WebSocket 消息压缩支持
package websocket

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// CompressionAlgorithm 压缩算法类型
type CompressionAlgorithm string

const (
	CompressionNone   CompressionAlgorithm = "none"
	CompressionGzip   CompressionAlgorithm = "gzip"
	CompressionZlib   CompressionAlgorithm = "zlib"
	CompressionFlate  CompressionAlgorithm = "flate"
	CompressionSnappy CompressionAlgorithm = "snappy" // 需要外部依赖，当前仅定义
)

// String 返回算法字符串
func (a CompressionAlgorithm) String() string {
	return string(a)
}

// CompressionConfig 压缩配置
type CompressionConfig struct {
	// Enabled 是否启用压缩
	Enabled bool `json:"enabled"`

	// Algorithm 压缩算法
	Algorithm CompressionAlgorithm `json:"algorithm"`

	// Level 压缩级别 (1-9, 1=最快, 9=最佳压缩)
	Level int `json:"level"`

	// MinSize 最小压缩大小（字节），小于此大小不压缩
	MinSize int `json:"minSize"`

	// MaxSize 最大压缩大小（字节），超过此大小拒绝压缩
	MaxSize int `json:"maxSize"`

	// Threshold 压缩阈值比例，压缩后需小于原始大小的此比例才使用
	Threshold float64 `json:"threshold"`

	// EnableDict 是否启用字典压缩
	EnableDict bool `json:"enableDict"`

	// DictData 预设字典数据（用于 flate/zlib）
	DictData []byte `json:"dictData,omitempty"`

	// PoolSize 压缩器池大小
	PoolSize int `json:"poolSize"`
}

// DefaultCompressionConfig 默认压缩配置
var DefaultCompressionConfig = &CompressionConfig{
	Enabled:    true,
	Algorithm:  CompressionGzip,
	Level:      6, // 默认压缩级别，平衡速度和压缩率
	MinSize:    100,
	MaxSize:    10 * 1024 * 1024, // 10MB
	Threshold:  0.9,              // 压缩后需小于原始大小 90% 才使用
	EnableDict: false,
	PoolSize:   10,
}

// CompressionResult 压缩结果
type CompressionResult struct {
	OriginalSize   int                  `json:"originalSize"`
	CompressedSize int                  `json:"compressedSize"`
	Ratio          float64              `json:"ratio"` // 压缩率 = compressed/original
	Algorithm      CompressionAlgorithm `json:"algorithm"`
	UsedFallback   bool                 `json:"usedFallback"` // 是否回退到原始数据
	Duration       time.Duration        `json:"duration"`
}

// CompressedMessage 压缩后的消息
type CompressedMessage struct {
	OriginalType   string               `json:"originalType"`
	Algorithm      CompressionAlgorithm `json:"algorithm"`
	OriginalSize   int                  `json:"originalSize"`
	CompressedSize int                  `json:"compressedSize"`
	Data           []byte               `json:"data"`
	CompressedAt   time.Time            `json:"compressedAt"`
}

// Compressor 压缩器接口
type Compressor interface {
	// Compress 压缩数据
	Compress(data []byte) ([]byte, error)
	// Decompress 解压数据
	Decompress(data []byte) ([]byte, error)
	// Algorithm 返回算法类型
	Algorithm() CompressionAlgorithm
}

// GzipCompressor Gzip 压缩器
type GzipCompressor struct {
	level int
	pool  sync.Pool
}

// NewGzipCompressor 创建 Gzip 压缩器
func NewGzipCompressor(level int) *GzipCompressor {
	return &GzipCompressor{
		level: level,
		pool: sync.Pool{
			New: func() interface{} {
				w, _ := gzip.NewWriterLevel(nil, level)
				return w
			},
		},
	}
}

// Compress 压缩数据
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := c.pool.Get().(*gzip.Writer)
	defer c.pool.Put(w)

	w.Reset(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("gzip compress failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gzip close failed: %w", err)
	}
	return buf.Bytes(), nil
}

// Decompress 解压数据
func (c *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader failed: %w", err)
	}
	defer r.Close()

	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress failed: %w", err)
	}
	return result, nil
}

// Algorithm 返回算法类型
func (c *GzipCompressor) Algorithm() CompressionAlgorithm {
	return CompressionGzip
}

// ZlibCompressor Zlib 压缩器
type ZlibCompressor struct {
	level int
	pool  sync.Pool
	dict  []byte
}

// NewZlibCompressor 创建 Zlib 压缩器
func NewZlibCompressor(level int, dict []byte) *ZlibCompressor {
	return &ZlibCompressor{
		level: level,
		dict:  dict,
		pool: sync.Pool{
			New: func() interface{} {
				w, _ := zlib.NewWriterLevel(nil, level)
				return w
			},
		},
	}
}

// Compress 压缩数据
func (c *ZlibCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := c.pool.Get().(*zlib.Writer)
	defer c.pool.Put(w)

	if c.dict != nil {
		w.Reset(&buf)
	} else {
		w.Reset(&buf)
	}

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("zlib compress failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("zlib close failed: %w", err)
	}
	return buf.Bytes(), nil
}

// Decompress 解压数据
func (c *ZlibCompressor) Decompress(data []byte) ([]byte, error) {
	var r io.ReadCloser
	var err error

	if c.dict != nil {
		r, err = zlib.NewReaderDict(bytes.NewReader(data), c.dict)
	} else {
		r, err = zlib.NewReader(bytes.NewReader(data))
	}

	if err != nil {
		return nil, fmt.Errorf("zlib reader failed: %w", err)
	}
	defer r.Close()

	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("zlib decompress failed: %w", err)
	}
	return result, nil
}

// Algorithm 返回算法类型
func (c *ZlibCompressor) Algorithm() CompressionAlgorithm {
	return CompressionZlib
}

// FlateCompressor Flate (DEFLATE) 压缩器
type FlateCompressor struct {
	level int
	pool  sync.Pool
	dict  []byte
}

// NewFlateCompressor 创建 Flate 压缩器
func NewFlateCompressor(level int, dict []byte) *FlateCompressor {
	return &FlateCompressor{
		level: level,
		dict:  dict,
		pool: sync.Pool{
			New: func() interface{} {
				w, _ := flate.NewWriter(nil, level)
				return w
			},
		},
	}
}

// Compress 压缩数据
func (c *FlateCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := c.pool.Get().(*flate.Writer)
	defer c.pool.Put(w)

	w.Reset(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("flate compress failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("flate close failed: %w", err)
	}
	return buf.Bytes(), nil
}

// Decompress 解压数据
func (c *FlateCompressor) Decompress(data []byte) ([]byte, error) {
	var r io.ReadCloser
	var err error

	if c.dict != nil {
		r = flate.NewReaderDict(bytes.NewReader(data), c.dict)
	} else {
		r = flate.NewReader(bytes.NewReader(data))
	}
	defer r.Close()

	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("flate decompress failed: %w", err)
	}
	return result, nil
}

// Algorithm 返回算法类型
func (c *FlateCompressor) Algorithm() CompressionAlgorithm {
	return CompressionFlate
}

// NoneCompressor 无压缩器（透传）
type NoneCompressor struct{}

// NewNoneCompressor 创建无压缩器
func NewNoneCompressor() *NoneCompressor {
	return &NoneCompressor{}
}

// Compress 直接返回原数据
func (c *NoneCompressor) Compress(data []byte) ([]byte, error) {
	return data, nil
}

// Decompress 直接返回原数据
func (c *NoneCompressor) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

// Algorithm 返回算法类型
func (c *NoneCompressor) Algorithm() CompressionAlgorithm {
	return CompressionNone
}

// MessageCompressor 消息压缩管理器
type MessageCompressor struct {
	config      *CompressionConfig
	compressor  Compressor
	pool        sync.Pool
	stats       CompressionStats
	mu          sync.RWMutex
	initialized bool
}

// CompressionStats 压缩统计
type CompressionStats struct {
	TotalCompressed      int64 `json:"totalCompressed"`
	TotalDecompressed    int64 `json:"totalDecompressed"`
	TotalOriginalBytes   int64 `json:"totalOriginalBytes"`
	TotalCompressedBytes int64 `json:"totalCompressedBytes"`
	TotalBytesSaved      int64 `json:"totalBytesSaved"`
	TotalErrors          int64 `json:"totalErrors"`
	SkippedTooSmall      int64 `json:"skippedTooSmall"`
	SkippedTooLarge      int64 `json:"skippedTooLarge"`
	SkippedBadRatio      int64 `json:"skippedBadRatio"`
}

// NewMessageCompressor 创建消息压缩管理器
func NewMessageCompressor(config *CompressionConfig) *MessageCompressor {
	if config == nil {
		config = DefaultCompressionConfig
	}

	mc := &MessageCompressor{
		config: config,
	}

	// 创建压缩器
	mc.compressor = mc.createCompressor(config.Algorithm, config.Level, config.DictData)

	mc.pool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 4096))
		},
	}

	mc.initialized = true
	return mc
}

// createCompressor 创建压缩器
func (mc *MessageCompressor) createCompressor(algorithm CompressionAlgorithm, level int, dict []byte) Compressor {
	switch algorithm {
	case CompressionGzip:
		return NewGzipCompressor(level)
	case CompressionZlib:
		return NewZlibCompressor(level, dict)
	case CompressionFlate:
		return NewFlateCompressor(level, dict)
	case CompressionNone:
		return NewNoneCompressor()
	default:
		return NewGzipCompressor(level) // 默认使用 gzip
	}
}

// CompressMessage 压缩消息
func (mc *MessageCompressor) CompressMessage(msg *Message) (*CompressedMessage, error) {
	if !mc.config.Enabled {
		return nil, ErrCompressionDisabled
	}

	// 序列化消息
	data, err := json.Marshal(msg)
	if err != nil {
		atomic.AddInt64(&mc.stats.TotalErrors, 1)
		return nil, fmt.Errorf("serialize message failed: %w", err)
	}

	originalSize := len(data)

	// 检查大小限制
	if originalSize < mc.config.MinSize {
		atomic.AddInt64(&mc.stats.SkippedTooSmall, 1)
		return nil, ErrMessageTooSmall
	}

	if originalSize > mc.config.MaxSize {
		atomic.AddInt64(&mc.stats.SkippedTooLarge, 1)
		return nil, ErrMessageTooLarge
	}

	// 压缩
	compressed, err := mc.compressor.Compress(data)
	if err != nil {
		atomic.AddInt64(&mc.stats.TotalErrors, 1)
		return nil, err
	}

	compressedSize := len(compressed)
	ratio := float64(compressedSize) / float64(originalSize)

	// 检查压缩比
	if ratio > mc.config.Threshold {
		atomic.AddInt64(&mc.stats.SkippedBadRatio, 1)
		return nil, ErrBadCompressionRatio
	}

	// 更新统计
	atomic.AddInt64(&mc.stats.TotalCompressed, 1)
	atomic.AddInt64(&mc.stats.TotalOriginalBytes, int64(originalSize))
	atomic.AddInt64(&mc.stats.TotalCompressedBytes, int64(compressedSize))
	atomic.AddInt64(&mc.stats.TotalBytesSaved, int64(originalSize-compressedSize))

	return &CompressedMessage{
		OriginalType:   msg.Type,
		Algorithm:      mc.compressor.Algorithm(),
		OriginalSize:   originalSize,
		CompressedSize: compressedSize,
		Data:           compressed,
		CompressedAt:   time.Now(),
	}, nil
}

// DecompressMessage 解压消息
func (mc *MessageCompressor) DecompressMessage(cm *CompressedMessage) (*Message, error) {
	if !mc.config.Enabled {
		return nil, ErrCompressionDisabled
	}

	// 选择合适的解压器
	decompressor := mc.compressor
	if cm.Algorithm != mc.compressor.Algorithm() {
		decompressor = mc.createCompressor(cm.Algorithm, mc.config.Level, mc.config.DictData)
	}

	// 解压
	data, err := decompressor.Decompress(cm.Data)
	if err != nil {
		atomic.AddInt64(&mc.stats.TotalErrors, 1)
		return nil, err
	}

	// 反序列化
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		atomic.AddInt64(&mc.stats.TotalErrors, 1)
		return nil, fmt.Errorf("deserialize message failed: %w", err)
	}

	// 更新统计
	atomic.AddInt64(&mc.stats.TotalDecompressed, 1)

	return &msg, nil
}

// CompressData 直接压缩数据
func (mc *MessageCompressor) CompressData(data []byte) ([]byte, *CompressionResult, error) {
	if !mc.config.Enabled {
		return data, &CompressionResult{
			Algorithm:    CompressionNone,
			UsedFallback: true,
		}, nil
	}

	start := time.Now()
	originalSize := len(data)

	// 检查大小限制
	if originalSize < mc.config.MinSize {
		return data, &CompressionResult{
			OriginalSize: originalSize,
			Algorithm:    CompressionNone,
			UsedFallback: true,
			Duration:     time.Since(start),
		}, nil
	}

	if originalSize > mc.config.MaxSize {
		return data, &CompressionResult{
			OriginalSize: originalSize,
			Algorithm:    CompressionNone,
			UsedFallback: true,
			Duration:     time.Since(start),
		}, nil
	}

	// 压缩
	compressed, err := mc.compressor.Compress(data)
	if err != nil {
		atomic.AddInt64(&mc.stats.TotalErrors, 1)
		return nil, nil, err
	}

	compressedSize := len(compressed)
	ratio := float64(compressedSize) / float64(originalSize)

	// 检查压缩比
	if ratio > mc.config.Threshold {
		return data, &CompressionResult{
			OriginalSize: originalSize,
			Algorithm:    CompressionNone,
			UsedFallback: true,
			Duration:     time.Since(start),
		}, nil
	}

	// 更新统计
	atomic.AddInt64(&mc.stats.TotalCompressed, 1)
	atomic.AddInt64(&mc.stats.TotalOriginalBytes, int64(originalSize))
	atomic.AddInt64(&mc.stats.TotalCompressedBytes, int64(compressedSize))
	atomic.AddInt64(&mc.stats.TotalBytesSaved, int64(originalSize-compressedSize))

	return compressed, &CompressionResult{
		OriginalSize:   originalSize,
		CompressedSize: compressedSize,
		Ratio:          ratio,
		Algorithm:      mc.compressor.Algorithm(),
		UsedFallback:   false,
		Duration:       time.Since(start),
	}, nil
}

// DecompressData 直接解压数据
func (mc *MessageCompressor) DecompressData(data []byte, algorithm CompressionAlgorithm) ([]byte, error) {
	if !mc.config.Enabled || algorithm == CompressionNone {
		return data, nil
	}

	decompressor := mc.compressor
	if algorithm != mc.compressor.Algorithm() {
		decompressor = mc.createCompressor(algorithm, mc.config.Level, mc.config.DictData)
	}

	result, err := decompressor.Decompress(data)
	if err != nil {
		atomic.AddInt64(&mc.stats.TotalErrors, 1)
		return nil, err
	}

	atomic.AddInt64(&mc.stats.TotalDecompressed, 1)
	return result, nil
}

// GetStats 获取压缩统计
func (mc *MessageCompressor) GetStats() CompressionStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.stats
}

// ResetStats 重置统计
func (mc *MessageCompressor) ResetStats() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.stats = CompressionStats{}
}

// SetAlgorithm 设置压缩算法
func (mc *MessageCompressor) SetAlgorithm(algorithm CompressionAlgorithm) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.config.Algorithm = algorithm
	mc.compressor = mc.createCompressor(algorithm, mc.config.Level, mc.config.DictData)
}

// SetLevel 设置压缩级别
func (mc *MessageCompressor) SetLevel(level int) error {
	if level < 1 || level > 9 {
		return fmt.Errorf("invalid compression level: %d, must be 1-9", level)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.config.Level = level
	mc.compressor = mc.createCompressor(mc.config.Algorithm, level, mc.config.DictData)
	return nil
}

// IsEnabled 是否启用压缩
func (mc *MessageCompressor) IsEnabled() bool {
	return mc.config.Enabled
}

// SetEnabled 设置是否启用
func (mc *MessageCompressor) SetEnabled(enabled bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.config.Enabled = enabled
}

// GetConfig 获取配置
func (mc *MessageCompressor) GetConfig() CompressionConfig {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return *mc.config
}

// BatchCompress 批量压缩消息
func (mc *MessageCompressor) BatchCompress(messages []*Message) ([]*CompressedMessage, []error) {
	results := make([]*CompressedMessage, 0, len(messages))
	var errors []error

	for _, msg := range messages {
		cm, err := mc.CompressMessage(msg)
		if err != nil {
			if err != ErrMessageTooSmall && err != ErrMessageTooLarge && err != ErrBadCompressionRatio {
				errors = append(errors, err)
			}
			continue
		}
		results = append(results, cm)
	}

	return results, errors
}

// BatchDecompress 批量解压消息
func (mc *MessageCompressor) BatchDecompress(messages []*CompressedMessage) ([]*Message, []error) {
	results := make([]*Message, 0, len(messages))
	var errors []error

	for _, cm := range messages {
		msg, err := mc.DecompressMessage(cm)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		results = append(results, msg)
	}

	return results, errors
}

// CompressionMiddleware 压缩中间件，用于 WebSocket 连接
type CompressionMiddleware struct {
	compressor *MessageCompressor
	config     *CompressionConfig
}

// NewCompressionMiddleware 创建压缩中间件
func NewCompressionMiddleware(config *CompressionConfig) *CompressionMiddleware {
	return &CompressionMiddleware{
		compressor: NewMessageCompressor(config),
		config:     config,
	}
}

// ProcessOutgoing 处理出站消息（压缩）
func (m *CompressionMiddleware) ProcessOutgoing(data []byte) ([]byte, *CompressionResult, error) {
	return m.compressor.CompressData(data)
}

// ProcessIncoming 处理入站消息（解压）
func (m *CompressionMiddleware) ProcessIncoming(data []byte, algorithm CompressionAlgorithm) ([]byte, error) {
	return m.compressor.DecompressData(data, algorithm)
}

// GetStats 获取统计
func (m *CompressionMiddleware) GetStats() CompressionStats {
	return m.compressor.GetStats()
}

// 错误定义
var (
	ErrCompressionDisabled  = fmt.Errorf("压缩功能已禁用")
	ErrMessageTooSmall      = fmt.Errorf("消息太小，无需压缩")
	ErrMessageTooLarge      = fmt.Errorf("消息太大，拒绝压缩")
	ErrBadCompressionRatio  = fmt.Errorf("压缩比不佳，使用原始数据")
	ErrUnsupportedAlgorithm = fmt.Errorf("不支持的压缩算法")
	ErrDecompressionFailed  = fmt.Errorf("解压失败")
)
