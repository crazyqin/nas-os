// Package smartcachetier 提供多级缓存智能管理功能
package smartcachetier

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrCacheTierNotFound 缓存层级不存在.
	ErrCacheTierNotFound = errors.New("缓存层级不存在")
	// ErrCacheEntryNotFound 缓存条目不存在.
	ErrCacheEntryNotFound = errors.New("缓存条目不存在")
	// ErrCacheTierFull 缓存层级已满.
	ErrCacheTierFull = errors.New("缓存层级已满")
	// ErrInvalidPolicy 无效的缓存策略.
	ErrInvalidPolicy = errors.New("无效的缓存策略")
	// ErrDuplicateTier 缓存层级已存在.
	ErrDuplicateTier = errors.New("缓存层级已存在")
)

// ========== 缓存策略 ==========

// CachePolicy 缓存淘汰策略.
type CachePolicy string

const (
	// PolicyLRU 最近最少使用.
	PolicyLRU CachePolicy = "LRU"
	// PolicyLFU 最不经常使用.
	PolicyLFU CachePolicy = "LFU"
	// PolicyARC 自适应替换缓存.
	PolicyARC CachePolicy = "ARC"
)

// ========== 缓存层级 ==========

// TierLevel 缓存层级.
type TierLevel int

const (
	// TierHDD HDD 层级（最慢，容量最大）.
	TierHDD TierLevel = 0
	// TierSSD SSD 层级（中速）.
	TierSSD TierLevel = 1
	// TierNVMe NVMe 层级（最快，容量最小）.
	TierNVMe TierLevel = 2
)

// TierInfo 缓存层级信息.
type TierInfo struct {
	Level       TierLevel   `json:"level"`        // 层级
	Name        string      `json:"name"`         // 名称
	DevicePath  string      `json:"device_path"`  // 设备路径
	TotalBytes  uint64      `json:"total_bytes"`  // 总容量（字节）
	UsedBytes   uint64      `json:"used_bytes"`   // 已用容量（字节）
	EntryCount  int         `json:"entry_count"`  // 条目数量
	Policy      CachePolicy `json:"policy"`       // 淘汰策略
	MaxEntries  int         `json:"max_entries"`  // 最大条目数
	CreatedAt   time.Time   `json:"created_at"`   // 创建时间
}

// CacheEntry 缓存条目.
type CacheEntry struct {
	Key        string    `json:"key"`         // 缓存键
	Size       uint64    `json:"size"`        // 大小（字节）
	Tier       TierLevel `json:"tier"`        // 所在层级
	HitCount   int64     `json:"hit_count"`   // 命中次数
	LastAccess time.Time `json:"last_access"` // 最后访问时间
	CreatedAt  time.Time `json:"created_at"`  // 创建时间
}

// ========== 请求/响应 ==========

// TierCreateRequest 创建缓存层级请求.
type TierCreateRequest struct {
	Level      TierLevel   `json:"level" binding:"required"`      // 层级
	Name       string      `json:"name" binding:"required"`       // 名称
	DevicePath string      `json:"device_path" binding:"required"` // 设备路径
	TotalBytes uint64      `json:"total_bytes" binding:"required"` // 总容量
	MaxEntries int         `json:"max_entries"`                    // 最大条目数
	Policy     CachePolicy `json:"policy"`                         // 淘汰策略
}

// CacheSetRequest 设置缓存请求.
type CacheSetRequest struct {
	Key  string `json:"key" binding:"required"`  // 缓存键
	Size uint64 `json:"size" binding:"required"` // 大小（字节）
}

// CacheStats 缓存统计信息.
type CacheStats struct {
	TotalEntries    int              `json:"total_entries"`    // 总条目数
	TotalUsedBytes  uint64           `json:"total_used_bytes"` // 总已用字节
	TotalHitCount   int64            `json:"total_hit_count"`  // 总命中次数
	HitRate         float64          `json:"hit_rate"`         // 命中率
	Tiers           []TierStats      `json:"tiers"`            // 各层级统计
	PromotionCount  int64            `json:"promotion_count"`  // 提升次数
	DemotionCount   int64            `json:"demotion_count"`   // 降级次数
}

// TierStats 层级统计.
type TierStats struct {
	Level      TierLevel `json:"level"`       // 层级
	Name       string    `json:"name"`        // 名称
	EntryCount int       `json:"entry_count"` // 条目数
	UsedBytes  uint64    `json:"used_bytes"`  // 已用字节
	HitCount   int64     `json:"hit_count"`   // 命中次数
	HitRate    float64   `json:"hit_rate"`    // 层级命中率
}

// PromotionPolicy 提升/降级策略配置.
type PromotionPolicy struct {
	HitThreshold    int     `json:"hit_threshold"`    // 命中次数阈值
	HitRateMin      float64 `json:"hit_rate_min"`     // 最小命中率
	IdleDurationSec int     `json:"idle_duration_sec"` // 空闲时间阈值（秒）
}

// CacheConfig 缓存配置.
type CacheConfig struct {
	DefaultPolicy     CachePolicy     `json:"default_policy"`      // 默认策略
	PromotionPolicy   PromotionPolicy `json:"promotion_policy"`    // 提升策略
	EnableAutoTiering bool            `json:"enable_auto_tiering"` // 启用自动分层
}
