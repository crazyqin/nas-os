// Package brtprefetch 实现 BRT（Block Reference Table）预取加速功能，
// 参考 TrueNAS 26 的 BRT 支持，管理块引用元数据，追踪块引用计数，
// 执行预取策略并提供缓存加速能力。
package brtprefetch

import "time"

// ========== BRT 元数据类型 ==========

// BlockState 块状态.
type BlockState string

const (
	// BlockStateActive 活跃块（正在被引用）.
	BlockStateActive BlockState = "active"
	// BlockStateFree 空闲块（引用计数为零）.
	BlockStateFree BlockState = "free"
	// BlockStatePrefetched 已预取块（缓存中）.
	BlockStatePrefetched BlockState = "prefetched"
	// BlockStateStale 过期块（缓存过期）.
	BlockStateStale BlockState = "stale"
)

// PrefetchStrategy 预取策略类型.
type PrefetchStrategy string

const (
	// StrategySequential 顺序预取（适用于顺序读取场景）.
	StrategySequential PrefetchStrategy = "sequential"
	// StrategyAdaptive 自适应预取（根据访问模式动态调整）.
	StrategyAdaptive PrefetchStrategy = "adaptive"
	// StrategyLookahead 前瞻预取（基于历史模式预测）.
	StrategyLookahead PrefetchStrategy = "lookahead"
	// StrategyOnDemand 按需预取（仅在被请求时预取）.
	StrategyOnDemand PrefetchStrategy = "on_demand"
)

// CachePolicy 缓存策略.
type CachePolicy string

const (
	// CachePolicyLRU 最近最少使用.
	CachePolicyLRU CachePolicy = "lru"
	// CachePolicyLFU 最少使用频率.
	CachePolicyLFU CachePolicy = "lfu"
	// CachePolicyFIFO 先进先出.
	CachePolicyFIFO CachePolicy = "fifo"
	// CachePolicyTTL 基于 TTL.
	CachePolicyTTL CachePolicy = "ttl"
)

// ========== 核心结构定义 ==========

// BRTEntry BRT 条目，记录单个块的引用信息.
type BRTEntry struct {
	BlockID      uint64     `json:"block_id"`      // 块唯一标识
	RefCount     int        `json:"ref_count"`     // 引用计数
	BlockSize    int        `json:"block_size"`    // 块大小（字节）
	Checksum     string     `json:"checksum"`      // 块校验和
	StoragePath  string     `json:"storage_path"`  // 存储路径
	State        BlockState `json:"state"`         // 块状态
	LastAccessed time.Time  `json:"last_accessed"` // 最后访问时间
	CreatedAt    time.Time  `json:"created_at"`    // 创建时间
	PoolID       string     `json:"pool_id"`       // 所属存储池 ID
}

// BRTMetadata BRT 元数据，包含块引用表信息.
type BRTMetadata struct {
	ID           string     `json:"id"`                // BRT 元数据唯一标识
	PoolID       string     `json:"pool_id"`           // 存储池 ID
	TotalBlocks  uint64     `json:"total_blocks"`      // 总块数
	RefCounted   uint64     `json:"ref_counted"`       // 有引用计数的块数
	ClonedBlocks uint64     `json:"cloned_blocks"`     // 克隆块数
	SavedSpace   uint64     `json:"saved_space"`       // 通过 BRT 节省的空间（字节）
	Entries      []BRTEntry `json:"entries,omitempty"` // 条目列表
	UpdatedAt    time.Time  `json:"updated_at"`        // 最后更新时间
	CreatedAt    time.Time  `json:"created_at"`        // 创建时间
}

// PrefetchConfig 预取配置.
type PrefetchConfig struct {
	Enabled         bool             `json:"enabled"`           // 是否启用预取
	Strategy        PrefetchStrategy `json:"strategy"`          // 预取策略
	CachePolicy     CachePolicy      `json:"cache_policy"`      // 缓存策略
	CacheSize       int              `json:"cache_size"`        // 缓存大小（块数）
	MaxBlockSize    int              `json:"max_block_size"`    // 最大块大小（字节）
	PrefetchDepth   int              `json:"prefetch_depth"`    // 预取深度（预取后续 N 个块）
	TTLSeconds      int              `json:"ttl_seconds"`       // 缓存 TTL（秒）
	MinRefThreshold int              `json:"min_ref_threshold"` // 最低引用计数阈值（低于此值不预取）
}

// PrefetchTask 预取任务.
type PrefetchTask struct {
	ID          string           `json:"id"`                     // 任务唯一标识
	BlockID     uint64           `json:"block_id"`               // 目标块 ID
	PoolID      string           `json:"pool_id"`                // 存储池 ID
	Strategy    PrefetchStrategy `json:"strategy"`               // 预取策略
	Status      string           `json:"status"`                 // 任务状态：pending/running/completed/failed
	Blocks      []uint64         `json:"blocks"`                 // 预取的块列表
	CreatedAt   time.Time        `json:"created_at"`             // 创建时间
	CompletedAt *time.Time       `json:"completed_at,omitempty"` // 完成时间
	Error       string           `json:"error,omitempty"`        // 错误信息
}

// CacheEntry 缓存条目.
type CacheEntry struct {
	BlockID    uint64    `json:"block_id"`    // 块 ID
	Data       []byte    `json:"-"`           // 块数据（不序列化）
	Size       int       `json:"size"`        // 数据大小
	HitCount   int       `json:"hit_count"`   // 命中次数
	LastAccess time.Time `json:"last_access"` // 最后访问时间
	CachedAt   time.Time `json:"cached_at"`   // 缓存时间
	ExpiresAt  time.Time `json:"expires_at"`  // 过期时间
}

// ========== 请求/响应类型 ==========

// PrefetchRequest 预取请求.
type PrefetchRequest struct {
	PoolID   string   `json:"pool_id" binding:"required"`         // 存储池 ID
	BlockIDs []uint64 `json:"block_ids" binding:"required,min=1"` // 块 ID 列表
}

// PrefetchResponse 预取响应.
type PrefetchResponse struct {
	TaskID   string   `json:"task_id"`           // 任务 ID
	BlockIDs []uint64 `json:"block_ids"`         // 预取的块列表
	Status   string   `json:"status"`            // 任务状态
	Message  string   `json:"message,omitempty"` // 消息
}

// CacheStatsResponse 缓存统计响应.
type CacheStatsResponse struct {
	TotalEntries  int     `json:"total_entries"`  // 总缓存条目
	TotalSize     int     `json:"total_size"`     // 总缓存大小（字节）
	HitRate       float64 `json:"hit_rate"`       // 命中率
	MissRate      float64 `json:"miss_rate"`      // 未命中率
	Hits          uint64  `json:"hits"`           // 命中次数
	Misses        uint64  `json:"misses"`         // 未命中次数
	Evictions     uint64  `json:"evictions"`      // 驱逐次数
	PrefetchCount uint64  `json:"prefetch_count"` // 预取次数
}

// BRTMetadataRequest 创建/更新 BRT 元数据请求.
type BRTMetadataRequest struct {
	PoolID      string `json:"pool_id" binding:"required"` // 存储池 ID
	TotalBlocks uint64 `json:"total_blocks,omitempty"`     // 总块数
}

// AddEntryRequest 添加 BRT 条目请求.
type AddEntryRequest struct {
	PoolID      string `json:"pool_id" binding:"required"`      // 存储池 ID
	BlockID     uint64 `json:"block_id" binding:"required"`     // 块 ID
	BlockSize   int    `json:"block_size" binding:"required"`   // 块大小
	Checksum    string `json:"checksum" binding:"required"`     // 校验和
	StoragePath string `json:"storage_path" binding:"required"` // 存储路径
	RefCount    int    `json:"ref_count,omitempty"`             // 引用计数
}

// ListResponse 列表响应.
type ListResponse struct {
	Items interface{} `json:"items"` // 列表项
	Total int         `json:"total"` // 总数
}
