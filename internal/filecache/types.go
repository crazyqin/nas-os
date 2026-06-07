// Package filecache 提供智能文件缓存功能，支持 LRU/LFU 混合策略和多级缓存。
// 对标 NAS 系统中的文件缓存加速层，支持内存、SSD、HDD 三级缓存。
package filecache

import (
	"time"
)

// CacheLevel 缓存层级
type CacheLevel int

const (
	LevelMemory CacheLevel = iota // 内存缓存（最快）
	LevelSSD                      // SSD 缓存（较快）
	LevelHDD                      // HDD 缓存（最慢但容量大）
)

// String 返回层级名称
func (l CacheLevel) String() string {
	switch l {
	case LevelMemory:
		return "memory"
	case LevelSSD:
		return "ssd"
	case LevelHDD:
		return "hdd"
	default:
		return "unknown"
	}
}

// EvictionPolicy 淘汰策略
type EvictionPolicy string

const (
	PolicyLRU    EvictionPolicy = "lru"    // 最近最少使用
	PolicyLFU    EvictionPolicy = "lfu"    // 最不经常使用
	PolicyHybrid EvictionPolicy = "hybrid" // LRU/LFU 混合策略
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string     `json:"key"`         // 缓存键（通常是文件路径或 hash）
	Path       string     `json:"path"`        // 原始文件路径
	Size       int64      `json:"size"`        // 文件大小
	Level      CacheLevel `json:"level"`       // 所在缓存层级
	HitCount   int64      `json:"hit_count"`   // 命中次数
	Frequency  float64    `json:"frequency"`   // 访问频率（次/小时）
	LastAccess time.Time  `json:"last_access"` // 最后访问时间
	CreatedAt  time.Time  `json:"created_at"`  // 创建时间
	ExpiresAt  *time.Time `json:"expires_at"`  // 过期时间（可选）
	Checksum   string     `json:"checksum"`    // 文件校验和
	Pinned     bool       `json:"pinned"`      // 是否固定（不被淘汰）
	score      float64    // 混合评分（内部使用）
}

// CacheStats 缓存统计
type CacheStats struct {
	// 总体统计
	TotalEntries int64   `json:"total_entries"`
	TotalSize    int64   `json:"total_size"`
	TotalHits    int64   `json:"total_hits"`
	TotalMisses  int64   `json:"total_misses"`
	HitRate      float64 `json:"hit_rate"`
	MissRate     float64 `json:"miss_rate"`

	// 按层级统计
	LevelStats map[CacheLevel]*LevelStats `json:"level_stats"`

	// 淘汰统计
	EvictionCount int64 `json:"eviction_count"`
	WarmupCount   int64 `json:"warmup_count"`

	// 时间戳
	LastEviction *time.Time `json:"last_eviction,omitempty"`
	LastWarmup   *time.Time `json:"last_warmup,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
}

// LevelStats 层级统计
type LevelStats struct {
	Level     CacheLevel `json:"level"`
	Entries   int64      `json:"entries"`
	Size      int64      `json:"size"`
	MaxSize   int64      `json:"max_size"`
	Hits      int64      `json:"hits"`
	Misses    int64      `json:"misses"`
	HitRate   float64    `json:"hit_rate"`
	Evictions int64      `json:"evictions"`
	Usage     float64    `json:"usage"` // 使用率 0-1
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// 基础配置
	Enabled bool           `json:"enabled"`
	Policy  EvictionPolicy `json:"policy"`   // 淘汰策略
	KeyFunc string         `json:"key_func"` // 键生成函数: "path", "hash", "custom"

	// 内存缓存配置
	MemoryMaxEntries int   `json:"memory_max_entries"` // 最大条目数
	MemoryMaxSize    int64 `json:"memory_max_size"`    // 最大字节数

	// SSD 缓存配置
	SSDEnabled bool   `json:"ssd_enabled"`
	SSDPath    string `json:"ssd_path"`     // SSD 缓存目录
	SSDMaxSize int64  `json:"ssd_max_size"` // SSD 最大字节数

	// HDD 缓存配置
	HDDEnabled bool   `json:"hdd_enabled"`
	HDDPath    string `json:"hdd_path"`     // HDD 缓存目录
	HDDMaxSize int64  `json:"hdd_max_size"` // HDD 最大字节数

	// 混合策略权重
	HybridLRUWeight float64 `json:"hybrid_lru_weight"` // LRU 权重 (0-1)
	HybridLFUWeight float64 `json:"hybrid_lfu_weight"` // LFU 权重 (0-1)

	// 预热配置
	WarmupEnabled  bool     `json:"warmup_enabled"`
	WarmupPaths    []string `json:"warmup_paths"`     // 预热路径
	WarmupMaxFiles int      `json:"warmup_max_files"` // 最大预热文件数
	WarmupMaxSize  int64    `json:"warmup_max_size"`  // 最大预热总大小

	// TTL 配置
	DefaultTTL time.Duration `json:"default_ttl"` // 默认 TTL
	MaxTTL     time.Duration `json:"max_ttl"`     // 最大 TTL

	// 清理配置
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
	ExpiredCheck    bool          `json:"expired_check"`    // 是否检查过期
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:          true,
		Policy:           PolicyHybrid,
		KeyFunc:          "path",
		MemoryMaxEntries: 10000,
		MemoryMaxSize:    1 << 30, // 1GB
		SSDEnabled:       false,
		SSDPath:          "/var/cache/filecache/ssd",
		SSDMaxSize:       10 << 30, // 10GB
		HDDEnabled:       false,
		HDDPath:          "/var/cache/filecache/hdd",
		HDDMaxSize:       100 << 30, // 100GB
		HybridLRUWeight:  0.4,
		HybridLFUWeight:  0.6,
		WarmupEnabled:    false,
		WarmupPaths:      []string{},
		WarmupMaxFiles:   1000,
		WarmupMaxSize:    5 << 30, // 5GB
		DefaultTTL:       24 * time.Hour,
		MaxTTL:           7 * 24 * time.Hour,
		CleanupInterval:  10 * time.Minute,
		ExpiredCheck:     true,
	}
}

// WarmupRequest 预热请求
type WarmupRequest struct {
	Paths    []string `json:"paths" binding:"required"`
	MaxFiles int      `json:"max_files,omitempty"`
	MaxSize  int64    `json:"max_size,omitempty"`
	Level    string   `json:"level,omitempty"` // 目标层级
}

// WarmupResult 预热结果
type WarmupResult struct {
	TotalFiles  int           `json:"total_files"`
	CachedFiles int           `json:"cached_files"`
	FailedFiles int           `json:"failed_files"`
	TotalSize   int64         `json:"total_size"`
	CachedSize  int64         `json:"cached_size"`
	Duration    time.Duration `json:"duration"`
	Errors      []string      `json:"errors,omitempty"`
}

// EvictionRequest 淘汰请求
type EvictionRequest struct {
	Level      string `json:"level,omitempty"`       // 目标层级
	TargetSize int64  `json:"target_size,omitempty"` // 目标大小
	MaxEntries int    `json:"max_entries,omitempty"` // 最大保留条目
	DryRun     bool   `json:"dry_run"`
}

// EvictionResult 淘汰结果
type EvictionResult struct {
	EvictedCount int           `json:"evicted_count"`
	FreedSize    int64         `json:"freed_size"`
	Remaining    int           `json:"remaining"`
	DryRun       bool          `json:"dry_run"`
	Duration     time.Duration `json:"duration"`
}

// GetRequest 获取缓存请求
type GetRequest struct {
	Key string `json:"key" binding:"required"`
}

// PutRequest 放入缓存请求
type PutRequest struct {
	Key      string `json:"key" binding:"required"`
	Path     string `json:"path" binding:"required"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum,omitempty"`
	TTL      int64  `json:"ttl,omitempty"` // 秒
	Pinned   bool   `json:"pinned,omitempty"`
	Level    string `json:"level,omitempty"` // 目标层级
}

// DeleteRequest 删除缓存请求
type DeleteRequest struct {
	Key   string `json:"key" binding:"required"`
	Level string `json:"level,omitempty"` // 目标层级，空表示所有层级
}

// CacheInfoResponse 缓存信息响应
type CacheInfoResponse struct {
	Exists  bool          `json:"exists"`
	Entry   *CacheEntry   `json:"entry,omitempty"`
	Content *CacheContent `json:"content,omitempty"`
}

// CacheContent 缓存内容（用于文件数据）
type CacheContent struct {
	Data     []byte `json:"data,omitempty"`      // 内存中的数据
	FilePath string `json:"file_path,omitempty"` // 磁盘上的路径
	Level    string `json:"level"`
	Size     int64  `json:"size"`
}

// ListRequest 列表请求
type ListRequest struct {
	Level  string `json:"level,omitempty"`  // 过滤层级
	Prefix string `json:"prefix,omitempty"` // 键前缀
	Limit  int    `json:"limit,omitempty"`  // 最大返回数
	Offset int    `json:"offset,omitempty"` // 偏移量
}

// ListResponse 列表响应
type ListResponse struct {
	Entries []*CacheEntry `json:"entries"`
	Total   int           `json:"total"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}
