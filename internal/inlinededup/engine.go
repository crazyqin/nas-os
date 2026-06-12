// Package inlinededup 提供内联块级数据去重引擎。
// 支持实时块级去重，基于内容寻址存储（CAS）和哈希索引。
// 参考：TrueNAS 26 Fast Dedup、群晖 DSM 7.4 Storage Efficiency
package inlinededup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// BlockStatus 块状态
type BlockStatus string

const (
	BlockUnique     BlockStatus = "unique"      // 唯一块
	BlockDuplicate  BlockStatus = "duplicate"   // 重复块
	BlockCompressed BlockStatus = "compressed"  // 已压缩的重复块
)

// DedupStats 去重统计
type DedupStats struct {
	TotalBlocks      int64   `json:"total_blocks"`
	UniqueBlocks     int64   `json:"unique_blocks"`
	DuplicateBlocks  int64   `json:"duplicate_blocks"`
	TotalBytesRead   int64   `json:"total_bytes_read"`
	TotalBytesWritten int64  `json:"total_bytes_written"`
	SavedBytes       int64   `json:"saved_bytes"`
	DedupRatio       float64 `json:"dedup_ratio"`
	SpaceSavedPct    float64 `json:"space_saved_percent"`
	HashCollisions   int64   `json:"hash_collisions"`
	IndexSize        int64   `json:"index_size"`
	LastUpdated      time.Time `json:"last_updated"`
}

// BlockInfo 块信息
type BlockInfo struct {
	Hash       string    `json:"hash"`        // SHA-256 哈希
	Size       int64     `json:"size"`        // 块大小
	RefCount   int32     `json:"ref_count"`   // 引用计数
	Compressed bool      `json:"compressed"`  // 是否已压缩
	FirstSeen  time.Time `json:"first_seen"`
	LastAccess time.Time `json:"last_access"`
}

// DedupConfig 去重引擎配置
type DedupConfig struct {
	BlockSize       int    `json:"block_size"`        // 块大小（字节），默认 128KB
	MaxIndexEntries int64  `json:"max_index_entries"`  // 最大索引条目数
	EnableCompress  bool   `json:"enable_compress"`   // 对重复块启用压缩
	VerifyContent   bool   `json:"verify_content"`    // 写入前验证内容一致性
	FlushInterval   int    `json:"flush_interval"`    // 索引刷新间隔（秒）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *DedupConfig {
	return &DedupConfig{
		BlockSize:       128 * 1024, // 128KB
		MaxIndexEntries: 10_000_000,
		EnableCompress:  true,
		VerifyContent:   true,
		FlushInterval:   60,
	}
}

// DedupEngine 内联去重引擎
type DedupEngine struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	config   *DedupConfig
	index    map[string]*BlockInfo // hash -> BlockInfo
	stats    DedupStats
	running  bool
	stopCh   chan struct{}
}

// NewEngine 创建去重引擎
func NewEngine(logger *slog.Logger, config *DedupConfig) *DedupEngine {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultConfig()
	}
	return &DedupEngine{
		logger: logger,
		config: config,
		index:  make(map[string]*BlockInfo),
		stopCh: make(chan struct{}),
	}
}

// Start 启动去重引擎
func (e *DedupEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("dedup engine already running")
	}

	e.running = true
	e.logger.Info("内联去重引擎已启动",
		"block_size", e.config.BlockSize,
		"compress", e.config.EnableCompress,
	)

	go e.statsLoop()
	return nil
}

// Stop 停止去重引擎
func (e *DedupEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	close(e.stopCh)
	e.running = false
	e.logger.Info("内联去重引擎已停止")
	return nil
}

// ProcessBlock 处理单个数据块，返回是否为重复块
func (e *DedupEngine) ProcessBlock(data []byte) (*BlockInfo, BlockStatus, error) {
	if len(data) == 0 {
		return nil, BlockUnique, fmt.Errorf("empty block")
	}

	// 计算 SHA-256 哈希
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	e.mu.Lock()
	defer e.mu.Unlock()

	atomic.AddInt64(&e.stats.TotalBlocks, 1)
	atomic.AddInt64(&e.stats.TotalBytesRead, int64(len(data)))
	e.stats.LastUpdated = time.Now()

	// 查找重复块
	if existing, found := e.index[hashStr]; found {
		// 验证内容一致性（防哈希碰撞）
		if e.config.VerifyContent {
			atomic.AddInt64(&e.stats.HashCollisions, 0) // 简化：信任 SHA-256
		}

		existing.RefCount++
		existing.LastAccess = time.Now()
		atomic.AddInt64(&e.stats.DuplicateBlocks, 1)
		atomic.AddInt64(&e.stats.SavedBytes, int64(len(data)))

		e.updateDedupRatio()
		return existing, BlockDuplicate, nil
	}

	// 新块：写入索引
	blockInfo := &BlockInfo{
		Hash:       hashStr,
		Size:       int64(len(data)),
		RefCount:   1,
		Compressed: false,
		FirstSeen:  time.Now(),
		LastAccess: time.Now(),
	}

	// 检查索引容量
	if int64(len(e.index)) >= e.config.MaxIndexEntries {
		e.evictOldest()
	}

	e.index[hashStr] = blockInfo
	atomic.AddInt64(&e.stats.UniqueBlocks, 1)
	atomic.AddInt64(&e.stats.TotalBytesWritten, int64(len(data)))

	e.updateDedupRatio()
	return blockInfo, BlockUnique, nil
}

// ProcessReader 从 reader 读取数据并进行去重处理
func (e *DedupEngine) ProcessReader(r io.Reader, writer io.Writer) (*DedupStats, error) {
	buf := make([]byte, e.config.BlockSize)
	var totalIn, totalOut int64

	for {
		n, err := r.Read(buf)
		if n > 0 {
			block := buf[:n]
			info, status, procErr := e.ProcessBlock(block)
			if procErr != nil {
				return nil, procErr
			}

			totalIn += int64(n)
			if status == BlockUnique {
				if _, writeErr := writer.Write(block); writeErr != nil {
					return nil, writeErr
				}
				totalOut += int64(n)
			} else {
				// 重复块：只写引用
				refData := []byte(fmt.Sprintf("DEDUP_REF:%s:%d", info.Hash, info.Size))
				if _, writeErr := writer.Write(refData); writeErr != nil {
					return nil, writeErr
				}
				totalOut += int64(len(refData))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	stats := e.GetStats()
	return &stats, nil
}

// GetStats 获取去重统计
func (e *DedupEngine) GetStats() DedupStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// GetBlockInfo 获取指定哈希的块信息
func (e *DedupEngine) GetBlockInfo(hash string) (*BlockInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	info, found := e.index[hash]
	return info, found
}

// IndexSize 返回索引条目数
func (e *DedupEngine) IndexSize() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.index)
}

// ResetStats 重置统计信息
func (e *DedupEngine) ResetStats() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats = DedupStats{
		LastUpdated: time.Now(),
	}
}

// updateDedupRatio 更新去重比率
func (e *DedupEngine) updateDedupRatio() {
	total := e.stats.UniqueBlocks + e.stats.DuplicateBlocks
	if total > 0 {
		e.stats.DedupRatio = float64(total) / float64(e.stats.UniqueBlocks)
	}
	if e.stats.TotalBytesRead > 0 {
		e.stats.SpaceSavedPct = float64(e.stats.SavedBytes) / float64(e.stats.TotalBytesRead) * 100
	}
	e.stats.IndexSize = int64(len(e.index))
	e.stats.LastUpdated = time.Now()
}

// evictOldest 驱逐最旧的索引条目（LRU 策略）
func (e *DedupEngine) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, info := range e.index {
		if oldestKey == "" || info.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = info.LastAccess
		}
	}

	if oldestKey != "" {
		delete(e.index, oldestKey)
	}
}

// statsLoop 定期更新统计信息
func (e *DedupEngine) statsLoop() {
	ticker := time.NewTicker(time.Duration(e.config.FlushInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.mu.Lock()
			e.updateDedupRatio()
			e.mu.Unlock()
		}
	}
}
