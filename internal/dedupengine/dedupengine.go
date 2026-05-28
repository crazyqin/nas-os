// Package dedupengine 提供基于内容寻址的块级重复数据删除功能
package dedupengine

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// DedupEngine 重复数据删除引擎.
type DedupEngine struct {
	mu     sync.RWMutex
	blocks map[string]*ContentBlock // 哈希 -> 内容块
	config *EngineConfig
}

// EngineConfig 引擎配置.
type EngineConfig struct {
	BlockSize BlockSize `json:"block_size"` // 块大小
	MaxBlocks int       `json:"max_blocks"` // 最大块数限制，0 表示无限制
}

// DefaultEngineConfig 默认引擎配置.
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		BlockSize: BlockSize64K,
		MaxBlocks: 0,
	}
}

// NewEngine 创建去重引擎.
func NewEngine(config *EngineConfig) *DedupEngine {
	if config == nil {
		config = DefaultEngineConfig()
	}
	return &DedupEngine{
		blocks: make(map[string]*ContentBlock),
		config: config,
	}
}

// hashData 计算数据的 SHA-256 哈希.
func hashData(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// Store 存储数据，返回内容哈希.
func (e *DedupEngine) Store(data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrInvalidData
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	hash := hashData(data)

	// 检查是否已存在
	if block, exists := e.blocks[hash]; exists {
		block.RefCount++
		return hash, nil
	}

	// 创建新块
	block := &ContentBlock{
		Hash:      hash,
		Data:      data,
		RefCount:  1,
		Size:      len(data),
		CreatedAt: time.Now(),
	}
	e.blocks[hash] = block

	return hash, nil
}

// Retrieve 按哈希获取数据.
func (e *DedupEngine) Retrieve(hash string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	block, exists := e.blocks[hash]
	if !exists {
		return nil, ErrBlockNotFound
	}

	// 返回数据副本，防止外部修改
	result := make([]byte, len(block.Data))
	copy(result, block.Data)
	return result, nil
}

// Delete 删除内容块（引用计数减 1）.
func (e *DedupEngine) Delete(hash string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	block, exists := e.blocks[hash]
	if !exists {
		return ErrBlockNotFound
	}

	block.RefCount--
	if block.RefCount <= 0 {
		delete(e.blocks, hash)
	}

	return nil
}

// Scan 扫描数据并分块，返回内容块列表.
func (e *DedupEngine) Scan(data []byte) []*ContentBlock {
	e.mu.RLock()
	blockSize := int(e.config.BlockSize)
	e.mu.RUnlock()

	if len(data) == 0 {
		return nil
	}

	var blocks []*ContentBlock

	for offset := 0; offset < len(data); offset += blockSize {
		end := offset + blockSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[offset:end]
		hash := hashData(chunk)

		block := &ContentBlock{
			Hash:      hash,
			Data:      chunk,
			RefCount:  0,
			Size:      len(chunk),
			CreatedAt: time.Now(),
		}
		blocks = append(blocks, block)
	}

	return blocks
}

// Stats 获取去重统计信息.
func (e *DedupEngine) Stats() DedupStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := DedupStats{
		UniqueBlocks: len(e.blocks),
	}

	for _, block := range e.blocks {
		stats.TotalBlocks += block.RefCount
		stats.UniqueSize += int64(block.Size)
	}

	stats.DuplicateBlocks = stats.TotalBlocks - stats.UniqueBlocks
	if stats.UniqueBlocks > 0 {
		avgBlockSize := stats.UniqueSize / int64(stats.UniqueBlocks)
		stats.TotalSize = avgBlockSize * int64(stats.TotalBlocks)
	}
	stats.SpaceSaved = stats.TotalSize - stats.UniqueSize

	return stats
}

// BlockCount 返回当前唯一块数量.
func (e *DedupEngine) BlockCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.blocks)
}

// HasBlock 检查是否存在指定哈希的块.
func (e *DedupEngine) HasBlock(hash string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.blocks[hash]
	return exists
}

// Clear 清空所有块.
func (e *DedupEngine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blocks = make(map[string]*ContentBlock)
}
