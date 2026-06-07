// Package active 全局重复数据删除模块
// 基于内容定义分块（Content-Defined Chunking, CDC）算法实现全局去重
// 使用 Rabin fingerprint 实现变长分块，配合 SHA-256 强校验去重
package active

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CDC 默认参数
const (
	DefaultCDCMinSize = 64 * 1024       // 64KB 最小块
	DefaultCDCMaxSize = 8 * 1024 * 1024 // 8MB 最大块
	CDCAverageBits    = 13              // 平均块大小约 2^13 = 8KB（目标值由 min/max 约束）
)

// Rabin 滑动窗口参数
const (
	RabinWindowSize = 48        // 滑动窗口大小
	RabinPrime      = 3         // Rabin 多项式质数
	RabinMod        = 1<<63 - 1 // 大质数取模
)

// CDCEngine 内容定义分块去重引擎
type CDCEngine struct {
	mu           sync.RWMutex
	minSize      int
	maxSize      int
	avgBits      int
	chunkIndex   map[string]*CDCChunk // checksum -> chunk
	dedupIndex   map[string]int       // checksum -> ref_count
	totalChunks  int64
	uniqueChunks int64
	dupChunks    int64
	totalBytes   int64
	savedBytes   int64
	logger       *zap.Logger
}

// CDCChunk CDC 数据块
type CDCChunk struct {
	Checksum  string    `json:"checksum"`   // SHA-256 校验和
	Size      int       `json:"size"`       // 块大小
	RefCount  int       `json:"ref_count"`  // 引用计数
	FirstSeen time.Time `json:"first_seen"` // 首次出现时间
	StorePath string    `json:"store_path"` // 存储路径（可选）
}

// CDCStats CDC 统计信息
type CDCStats struct {
	TotalChunks  int64   `json:"total_chunks"`
	UniqueChunks int64   `json:"unique_chunks"`
	DupChunks    int64   `json:"dup_chunks"`
	TotalBytes   int64   `json:"total_bytes"`
	SavedBytes   int64   `json:"saved_bytes"`
	DedupRatio   float64 `json:"dedup_ratio"`
}

// NewCDCEngine 创建 CDC 去重引擎
func NewCDCEngine(minSize, maxSize int, logger *zap.Logger) *CDCEngine {
	if minSize <= 0 {
		minSize = DefaultCDCMinSize
	}
	if maxSize <= 0 || maxSize <= minSize {
		maxSize = DefaultCDCMaxSize
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &CDCEngine{
		minSize:    minSize,
		maxSize:    maxSize,
		avgBits:    CDCAverageBits,
		chunkIndex: make(map[string]*CDCChunk),
		dedupIndex: make(map[string]int),
		logger:     logger,
	}
}

// CDCResult CDC 分块结果
type CDCResult struct {
	Chunks       []*CDCChunk `json:"chunks"`
	TotalChunks  int         `json:"total_chunks"`
	UniqueChunks int         `json:"unique_chunks"`
	DupChunks    int         `json:"dup_chunks"`
	TotalBytes   int64       `json:"total_bytes"`
	SavedBytes   int64       `json:"saved_bytes"`
}

// Deduplicate 对数据流执行 CDC 分块并去重
// 返回分块结果
func (e *CDCEngine) Deduplicate(reader io.Reader) (*CDCResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := &CDCResult{
		Chunks: make([]*CDCChunk, 0),
	}

	buf := make([]byte, e.maxSize)
	readOffset := 0

	for {
		// 读取数据到缓冲区
		n, err := reader.Read(buf[readOffset:])
		if n > 0 {
			readOffset += n
		}

		if readOffset > 0 {
			// 对缓冲区数据进行 CDC 分块
			chunks := e.cdcChunk(buf[:readOffset])
			for _, chunkData := range chunks {
				checksum := computeChecksum(chunkData)
				chunkSize := len(chunkData)

				result.TotalChunks++
				result.TotalBytes += int64(chunkSize)

				if entry, exists := e.chunkIndex[checksum]; exists {
					// 重复块
					entry.RefCount++
					e.dedupIndex[checksum]++
					result.DupChunks++
					result.SavedBytes += int64(chunkSize)
				} else {
					// 新块
					chunk := &CDCChunk{
						Checksum:  checksum,
						Size:      chunkSize,
						RefCount:  1,
						FirstSeen: time.Now(),
					}
					e.chunkIndex[checksum] = chunk
					e.dedupIndex[checksum] = 1
					result.UniqueChunks++
					result.Chunks = append(result.Chunks, chunk)
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取数据失败: %w", err)
		}

		// 如果缓冲区满了，重置（防止残留数据影响下次分块）
		if readOffset >= e.maxSize {
			readOffset = 0
		}
	}

	// 更新全局统计
	e.totalChunks += int64(result.TotalChunks)
	e.uniqueChunks += int64(result.UniqueChunks)
	e.dupChunks += int64(result.DupChunks)
	e.totalBytes += result.TotalBytes
	e.savedBytes += result.SavedBytes

	e.logger.Debug("CDC 去重完成",
		zap.Int("total_chunks", result.TotalChunks),
		zap.Int("unique", result.UniqueChunks),
		zap.Int("dup", result.DupChunks),
		zap.Int64("saved", result.SavedBytes))

	return result, nil
}

// DeduplicateBytes 对字节切片执行 CDC 分块并去重
func (e *CDCEngine) DeduplicateBytes(data []byte) (*CDCResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := &CDCResult{
		Chunks: make([]*CDCChunk, 0),
	}

	chunks := e.cdcChunk(data)
	for _, chunkData := range chunks {
		checksum := computeChecksum(chunkData)
		chunkSize := len(chunkData)

		result.TotalChunks++
		result.TotalBytes += int64(chunkSize)

		if entry, exists := e.chunkIndex[checksum]; exists {
			entry.RefCount++
			e.dedupIndex[checksum]++
			result.DupChunks++
			result.SavedBytes += int64(chunkSize)
		} else {
			chunk := &CDCChunk{
				Checksum:  checksum,
				Size:      chunkSize,
				RefCount:  1,
				FirstSeen: time.Now(),
			}
			e.chunkIndex[checksum] = chunk
			e.dedupIndex[checksum] = 1
			result.UniqueChunks++
			result.Chunks = append(result.Chunks, chunk)
		}
	}

	e.totalChunks += int64(result.TotalChunks)
	e.uniqueChunks += int64(result.UniqueChunks)
	e.dupChunks += int64(result.DupChunks)
	e.totalBytes += result.TotalBytes
	e.savedBytes += result.SavedBytes

	return result, nil
}

// cdcChunk 使用 CDC 算法将数据切分为变长块
// 基于 Rabin fingerprint 的内容定义分块
func (e *CDCEngine) cdcChunk(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}

	chunks := make([][]byte, 0)
	offset := 0

	for offset < len(data) {
		remaining := len(data) - offset
		if remaining <= e.minSize {
			// 不足最小块，作为最后一块
			chunks = append(chunks, data[offset:])
			break
		}

		// 确定当前块的搜索范围
		searchEnd := offset + e.maxSize
		if searchEnd > len(data) {
			searchEnd = len(data)
		}

		// 在 [offset+minSize, searchEnd) 范围内寻找切分点
		cutPoint := -1
		for i := offset + e.minSize; i < searchEnd; i++ {
			// 使用简化的滚动哈希检测切分点
			// 当 hash(data[i-window:i]) & mask == 0 时切分
			if e.isCutPoint(data, i) {
				cutPoint = i
				break
			}
		}

		if cutPoint == -1 {
			// 未找到切分点，使用最大块
			cutPoint = searchEnd
		}

		chunks = append(chunks, data[offset:cutPoint])
		offset = cutPoint
	}

	return chunks
}

// isCutPoint 判断当前位置是否为 CDC 切分点
// 使用简化的 Rabin fingerprint
func (e *CDCEngine) isCutPoint(data []byte, pos int) bool {
	if pos < RabinWindowSize {
		return false
	}

	// 计算窗口内的滚动哈希
	var hash uint64
	start := pos - RabinWindowSize
	for i := start; i < pos; i++ {
		hash = hash*RabinPrime + uint64(data[i])
		hash &= RabinMod
	}

	// 检查低 avgBits 位是否全为 0（平均约 2^avgBits 字节一个切分点）
	mask := uint64((1 << e.avgBits) - 1)
	return (hash & mask) == 0
}

// GetStats 获取 CDC 统计信息
func (e *CDCEngine) GetStats() CDCStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := CDCStats{
		TotalChunks:  e.totalChunks,
		UniqueChunks: e.uniqueChunks,
		DupChunks:    e.dupChunks,
		TotalBytes:   e.totalBytes,
		SavedBytes:   e.savedBytes,
	}
	if e.totalBytes > 0 {
		stats.DedupRatio = float64(e.savedBytes) / float64(e.totalBytes)
	}
	return stats
}

// HasChunk 检查数据块是否已存在
func (e *CDCEngine) HasChunk(checksum string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.chunkIndex[checksum]
	return exists
}

// GetChunk 获取数据块信息
func (e *CDCEngine) GetChunk(checksum string) (*CDCChunk, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	chunk, exists := e.chunkIndex[checksum]
	return chunk, exists
}

// ComputeChecksum 计算数据的 SHA-256 校验和
func (e *CDCEngine) ComputeChecksum(data []byte) string {
	return computeChecksum(data)
}

// computeChecksum 计算 SHA-256 校验和
func computeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SaveIndex 保存 CDC 索引到磁盘
func (e *CDCEngine) SaveIndex(path string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	index := struct {
		Chunks    map[string]*CDCChunk `json:"chunks"`
		Stats     CDCStats             `json:"stats"`
		UpdatedAt time.Time            `json:"updated_at"`
	}{
		Chunks:    e.chunkIndex,
		Stats:     e.GetStats(),
		UpdatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 CDC 索引失败: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// LoadIndex 从磁盘加载 CDC 索引
func (e *CDCEngine) LoadIndex(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var index struct {
		Chunks map[string]*CDCChunk `json:"chunks"`
		Stats  CDCStats             `json:"stats"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("解析 CDC 索引失败: %w", err)
	}

	if index.Chunks != nil {
		e.chunkIndex = index.Chunks
	}
	e.dedupIndex = make(map[string]int, len(e.chunkIndex))
	for cs, chunk := range e.chunkIndex {
		e.dedupIndex[cs] = chunk.RefCount
	}
	e.totalChunks = index.Stats.TotalChunks
	e.uniqueChunks = index.Stats.UniqueChunks
	e.dupChunks = index.Stats.DupChunks
	e.totalBytes = index.Stats.TotalBytes
	e.savedBytes = index.Stats.SavedBytes

	e.logger.Info("CDC 索引加载完成",
		zap.Int64("chunks", e.totalChunks),
		zap.Int64("saved_bytes", e.savedBytes))

	return nil
}

// DeduplicateFile 对文件执行 CDC 分块并去重
func (e *CDCEngine) DeduplicateFile(path string) (*CDCResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	return e.Deduplicate(f)
}

// Clear 清空去重索引
func (e *CDCEngine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.chunkIndex = make(map[string]*CDCChunk)
	e.dedupIndex = make(map[string]int)
	e.totalChunks = 0
	e.uniqueChunks = 0
	e.dupChunks = 0
	e.totalBytes = 0
	e.savedBytes = 0
}
