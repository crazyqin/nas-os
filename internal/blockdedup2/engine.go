package blockdedup2

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// BlockInfo 存储块信息。
type BlockInfo struct {
	Hash      string `json:"hash"`
	Size      int64  `json:"size"`
	RefCount  int    `json:"ref_count"`
	FilePaths []string `json:"file_paths"`
}

// Engine 块级去重2.0引擎，支持BRT(Block Reference Table)。
type Engine struct {
	mu       sync.RWMutex
	blocks   map[string]*BlockInfo
	brt      map[string][]string // Block Reference Table
	stats    Stats
}

// Stats 去重统计。
type Stats struct {
	TotalBlocks    int64  `json:"total_blocks"`
	UniqueBlocks   int64  `json:"unique_blocks"`
	DuplicateBlocks int64 `json:"duplicate_blocks"`
	SavedBytes     int64  `json:"saved_bytes"`
	ScanDuration   int64  `json:"scan_duration_ms"`
}

// NewEngine 创建新的去重引擎。
func NewEngine() *Engine {
	return &Engine{
		blocks: make(map[string]*BlockInfo),
		brt:    make(map[string][]string),
	}
}

// ComputeHash 计算数据块的SHA-256哈希。
func ComputeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// AddBlock 添加数据块到引擎。
func (e *Engine) AddBlock(hash string, size int64, filePath string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if block, exists := e.blocks[hash]; exists {
		block.RefCount++
		block.FilePaths = append(block.FilePaths, filePath)
		e.stats.DuplicateBlocks++
		e.stats.SavedBytes += size
	} else {
		e.blocks[hash] = &BlockInfo{
			Hash:      hash,
			Size:      size,
			RefCount:  1,
			FilePaths: []string{filePath},
		}
		e.stats.UniqueBlocks++
	}
	e.stats.TotalBlocks++
	
	// 更新BRT
	e.brt[hash] = append(e.brt[hash], filePath)
}

// GetBlock 获取块信息。
func (e *Engine) GetBlock(hash string) (*BlockInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	block, exists := e.blocks[hash]
	return block, exists
}

// GetStats 获取统计信息。
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// GetBRT 获取Block Reference Table。
func (e *Engine) GetBRT() map[string][]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(map[string][]string)
	for k, v := range e.brt {
		result[k] = v
	}
	return result
}

// Deduplicate 执行去重操作，返回去重后的块列表。
func (e *Engine) Deduplicate() []BlockInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	result := make([]BlockInfo, 0, len(e.blocks))
	for _, block := range e.blocks {
		result = append(result, *block)
	}
	return result
}
