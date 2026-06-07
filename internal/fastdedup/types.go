// Package fastdedup NVMe优化快速去重引擎
// 基于内容感知的高性能去重，专为全闪存/NVMe阵列优化。
// 对标 TrueNAS Fast Deduplication，支持实时和批量去重模式。
package fastdedup

import (
	"errors"
	"sync"
	"time"
)

// DedupMode 去重模式
type DedupMode string

const (
	ModeRealtime DedupMode = "realtime" // 实时去重
	ModeBatch    DedupMode = "batch"    // 批量去重
	ModeInline   DedupMode = "inline"   // 内联去重
)

// DedupAlgorithm 去重算法
type DedupAlgorithm string

const (
	AlgoFingerprint DedupAlgorithm = "fingerprint" // 指纹去重
	AlgoChunkBased  DedupAlgorithm = "chunk_based" // 分块去重
	AlgoHybrid      DedupAlgorithm = "hybrid"      // 混合去重
)

// StorageTier 存储层
type StorageTier string

const (
	TierNVMe StorageTier = "nvme"
	TierSSD  StorageTier = "ssd"
	TierHDD  StorageTier = "hdd"
)

// DedupStats 去重统计
type DedupStats struct {
	TotalBlocks     int64     `json:"total_blocks"`
	UniqueBlocks    int64     `json:"unique_blocks"`
	DuplicateBlocks int64     `json:"duplicate_blocks"`
	SpaceSaved      int64     `json:"space_saved_bytes"`
	DedupRatio      float64   `json:"dedup_ratio"`
	LastRunAt       time.Time `json:"last_run_at"`
	Duration        int64     `json:"duration_ms"`
}

// DedupBlock 去重块
type DedupBlock struct {
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	RefCount  int       `json:"ref_count"`
	Tier      StorageTier `json:"tier"`
	CreatedAt time.Time `json:"created_at"`
}

// DedupPolicy 去重策略
type DedupPolicy struct {
	Name        string          `json:"name"`
	Mode        DedupMode       `json:"mode"`
	Algorithm   DedupAlgorithm  `json:"algorithm"`
	MinBlockSize int64          `json:"min_block_size"`
	MaxBlockSize int64          `json:"max_block_size"`
	Tiers       []StorageTier   `json:"tiers"`
	Enabled     bool            `json:"enabled"`
}

// DedupResult 去重结果
type DedupResult struct {
	ScannedBlocks   int64   `json:"scanned_blocks"`
	DedupedBlocks   int64   `json:"deduped_blocks"`
	SpaceSavedBytes int64   `json:"space_saved_bytes"`
	Duration        int64   `json:"duration_ms"`
	DedupRatio      float64 `json:"dedup_ratio"`
}

// FastDedupEngine NVMe优化快速去重引擎
type FastDedupEngine struct {
	mu        sync.RWMutex
	config    EngineConfig
	policies  map[string]*DedupPolicy
	blockIndex map[string]*DedupBlock
	stats     DedupStats
	running   bool
}

// EngineConfig 引擎配置
type EngineConfig struct {
	DefaultMode     DedupMode       `json:"default_mode"`
	DefaultAlgo     DedupAlgorithm  `json:"default_algo"`
	HashAlgorithm   string          `json:"hash_algorithm"`
	IndexBackend    string          `json:"index_backend"`
	MaxMemoryMB     int             `json:"max_memory_mb"`
	NVMeOptimized   bool            `json:"nvme_optimized"`
	WorkerCount     int             `json:"worker_count"`
	BatchSizeMB     int             `json:"batch_size_mb"`
}

// DefaultEngineConfig 默认引擎配置
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		DefaultMode:   ModeRealtime,
		DefaultAlgo:   AlgoHybrid,
		HashAlgorithm: "xxhash64",
		IndexBackend:  "memory",
		MaxMemoryMB:   512,
		NVMeOptimized: true,
		WorkerCount:   8,
		BatchSizeMB:   64,
	}
}

// 预定义错误
var (
	ErrEngineRunning    = errors.New("engine is already running")
	ErrEngineNotRunning = errors.New("engine is not running")
	ErrPolicyExists     = errors.New("policy already exists")
	ErrPolicyNotFound   = errors.New("policy not found")
	ErrBlockSizeInvalid = errors.New("invalid block size")
	ErrHashFailed       = errors.New("hash computation failed")
)
