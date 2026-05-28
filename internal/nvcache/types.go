// Package nvcache 提供 NVMe 智能缓存加速功能，支持 NVMe SSD 作为 HDD 的读写缓存层。
// 提供自动分层存储、缓存策略管理、命中率监控、缓存预热和淘汰策略等功能。
package nvcache

import "time"

// CachePolicy 缓存策略
type CachePolicy string

const (
	// PolicyWriteBack 写回策略，数据先写入缓存，异步刷回底层存储
	PolicyWriteBack CachePolicy = "write-back"
	// PolicyWriteThrough 写穿策略，数据同时写入缓存和底层存储
	PolicyWriteThrough CachePolicy = "write-through"
	// PolicyReadAhead 预读策略，提前加载可能访问的数据到缓存
	PolicyReadAhead CachePolicy = "read-ahead"
)

// EvictionPolicy 淘汰策略
type EvictionPolicy string

const (
	// EvictionLRU 最近最少使用淘汰策略
	EvictionLRU EvictionPolicy = "lru"
	// EvictionLFU 最不经常使用淘汰策略
	EvictionLFU EvictionPolicy = "lfu"
	// EvictionARC 自适应替换缓存策略
	EvictionARC EvictionPolicy = "arc"
)

// CacheStatus 缓存状态
type CacheStatus string

const (
	// StatusActive 活跃状态
	StatusActive CacheStatus = "active"
	// StatusInactive 非活跃状态
	StatusInactive CacheStatus = "inactive"
	// StatusSyncing 同步中状态
	StatusSyncing CacheStatus = "syncing"
	// StatusError 错误状态
	StatusError CacheStatus = "error"
)

// DeviceRole 设备角色
type DeviceRole string

const (
	// RoleCache 缓存设备角色
	RoleCache DeviceRole = "cache"
	// RoleBackend 后端存储设备角色
	RoleBackend DeviceRole = "backend"
)

// RAIDLevel RAID 级别
type RAIDLevel string

const (
	// RAID0 条带化，提高性能无冗余
	RAID0 RAIDLevel = "raid0"
	// RAID1 镜像，数据冗余
	RAID1 RAIDLevel = "raid1"
	// RAID5 分布式奇偶校验
	RAID5 RAIDLevel = "raid5"
	// RAID10 镜像+条带化
	RAID10 RAIDLevel = "raid10"
)

// CacheDevice 缓存设备信息
type CacheDevice struct {
	// ID 设备唯一标识
	ID string `json:"id"`
	// Name 设备名称
	Name string `json:"name"`
	// Path 设备路径，如 /dev/nvme0n1
	Path string `json:"path"`
	// Role 设备角色
	Role DeviceRole `json:"role"`
	// CapacityGB 设备容量（GB）
	CapacityGB int64 `json:"capacity_gb"`
	// UsedGB 已使用容量（GB）
	UsedGB int64 `json:"used_gb"`
	// Model 设备型号
	Model string `json:"model,omitempty"`
	// Serial 设备序列号
	Serial string `json:"serial,omitempty"`
	// TemperatureC 设备温度（摄氏度）
	TemperatureC int `json:"temperature_c,omitempty"`
	// HealthPercent 设备健康度百分比
	HealthPercent int `json:"health_percent,omitempty"`
	// IsActive 是否活跃
	IsActive bool `json:"is_active"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// CachePool 缓存池
type CachePool struct {
	// ID 缓存池唯一标识
	ID string `json:"id"`
	// Name 缓存池名称
	Name string `json:"name"`
	// Devices 缓存设备列表
	Devices []*CacheDevice `json:"devices"`
	// RAIDLevel RAID 级别
	RAIDLevel RAIDLevel `json:"raid_level,omitempty"`
	// TotalCapacityGB 总容量（GB）
	TotalCapacityGB int64 `json:"total_capacity_gb"`
	// UsedCapacityGB 已使用容量（GB）
	UsedCapacityGB int64 `json:"used_capacity_gb"`
	// Policy 缓存策略
	Policy CachePolicy `json:"policy"`
	// EvictionPolicy 淘汰策略
	EvictionPolicy EvictionPolicy `json:"eviction_policy"`
	// Status 缓存池状态
	Status CacheStatus `json:"status"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// CacheMapping 缓存映射关系
type CacheMapping struct {
	// ID 映射唯一标识
	ID string `json:"id"`
	// CachePoolID 缓存池 ID
	CachePoolID string `json:"cache_pool_id"`
	// BackendDevice 后端存储设备路径
	BackendDevice string `json:"backend_device"`
	// MountPoint 挂载点
	MountPoint string `json:"mount_point"`
	// Policy 缓存策略，覆盖缓存池默认策略
	Policy CachePolicy `json:"policy,omitempty"`
	// BlockSizeKB 缓存块大小（KB）
	BlockSizeKB int `json:"block_size_kb"`
	// IsActive 是否活跃
	IsActive bool `json:"is_active"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// CacheStats 缓存统计信息
type CacheStats struct {
	// CachePoolID 缓存池 ID
	CachePoolID string `json:"cache_pool_id"`
	// HitCount 缓存命中次数
	HitCount int64 `json:"hit_count"`
	// MissCount 缓存未命中次数
	MissCount int64 `json:"miss_count"`
	// HitRate 命中率百分比
	HitRate float64 `json:"hit_rate"`
	// ReadHitCount 读命中次数
	ReadHitCount int64 `json:"read_hit_count"`
	// ReadMissCount 读未命中次数
	ReadMissCount int64 `json:"read_miss_count"`
	// WriteHitCount 写命中次数
	WriteHitCount int64 `json:"write_hit_count"`
	// WriteMissCount 写未命中次数
	WriteMissCount int64 `json:"write_miss_count"`
	// TotalReadBytes 总读取字节数
	TotalReadBytes int64 `json:"total_read_bytes"`
	// TotalWriteBytes 总写入字节数
	TotalWriteBytes int64 `json:"total_write_bytes"`
	// DirtyBlocks 脏块数量（待刷回）
	DirtyBlocks int64 `json:"dirty_blocks"`
	// DirtyBytes 脏数据字节数
	DirtyBytes int64 `json:"dirty_bytes"`
	// CachedBlocks 缓存块数量
	CachedBlocks int64 `json:"cached_blocks"`
	// CachedBytes 缓存数据字节数
	CachedBytes int64 `json:"cached_bytes"`
	// AverageLatencyUs 平均延迟（微秒）
	AverageLatencyUs float64 `json:"average_latency_us"`
	// IOPS 读写 IOPS
	IOPS int64 `json:"iops"`
	// BandwidthMBps 带宽（MB/s）
	BandwidthMBps float64 `json:"bandwidth_mbps"`
	// CollectedAt 统计收集时间
	CollectedAt time.Time `json:"collected_at"`
}

// TierRule 分层规则
type TierRule struct {
	// ID 规则唯一标识
	ID string `json:"id"`
	// Name 规则名称
	Name string `json:"name"`
	// Description 规则描述
	Description string `json:"description,omitempty"`
	// HotThreshold 热数据阈值（访问次数/小时）
	HotThreshold int `json:"hot_threshold"`
	// ColdThreshold 冷数据阈值（未访问小时数）
	ColdThreshold int `json:"cold_threshold"`
	// PromoteEnabled 是否启用自动提升（冷->热）
	PromoteEnabled bool `json:"promote_enabled"`
	// DemoteEnabled 是否启用自动降级（热->冷）
	DemoteEnabled bool `json:"demote_enabled"`
	// PromoteScheduleMB 提升调度阈值（MB/次）
	PromoteScheduleMB int `json:"promote_schedule_mb"`
	// IsActive 是否启用
	IsActive bool `json:"is_active"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// WarmupTask 缓存预热任务
type WarmupTask struct {
	// ID 任务唯一标识
	ID string `json:"id"`
	// Name 任务名称
	Name string `json:"name"`
	// CachePoolID 缓存池 ID
	CachePoolID string `json:"cache_pool_id"`
	// SourcePath 数据源路径
	SourcePath string `json:"source_path"`
	// FilePattern 文件匹配模式，如 *.mp4, /data/hot/*
	FilePattern string `json:"file_pattern,omitempty"`
	// TotalFiles 总文件数
	TotalFiles int `json:"total_files"`
	// WarmedFiles 已预热文件数
	WarmedFiles int `json:"warmed_files"`
	// TotalBytes 总字节数
	TotalBytes int64 `json:"total_bytes"`
	// WarmedBytes 已预热字节数
	WarmedBytes int64 `json:"warmed_bytes"`
	// Status 任务状态
	Status CacheStatus `json:"status"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// StartedAt 开始时间
	StartedAt time.Time `json:"started_at,omitempty"`
	// CompletedAt 完成时间
	CompletedAt time.Time `json:"completed_at,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// ConsistencyCheck 一致性检查结果
type ConsistencyCheck struct {
	// ID 检查唯一标识
	ID string `json:"id"`
	// CachePoolID 缓存池 ID
	CachePoolID string `json:"cache_pool_id"`
	// Status 检查状态
	Status CacheStatus `json:"status"`
	// TotalBlocks 总块数
	TotalBlocks int64 `json:"total_blocks"`
	// CheckedBlocks 已检查块数
	CheckedBlocks int64 `json:"checked_blocks"`
	// InconsistentBlocks 不一致块数
	InconsistentBlocks int64 `json:"inconsistent_blocks"`
	// RepairedBlocks 已修复块数
	RepairedBlocks int64 `json:"repaired_blocks"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// StartedAt 开始时间
	StartedAt time.Time `json:"started_at"`
	// CompletedAt 完成时间
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// CreatePoolRequest 创建缓存池请求
type CreatePoolRequest struct {
	// Name 缓存池名称
	Name string `json:"name" binding:"required"`
	// DeviceIDs 设备 ID 列表
	DeviceIDs []string `json:"device_ids" binding:"required,min=1"`
	// RAIDLevel RAID 级别
	RAIDLevel RAIDLevel `json:"raid_level,omitempty"`
	// Policy 缓存策略
	Policy CachePolicy `json:"policy" binding:"required"`
	// EvictionPolicy 淘汰策略
	EvictionPolicy EvictionPolicy `json:"eviction_policy" binding:"required"`
}

// CreateMappingRequest 创建缓存映射请求
type CreateMappingRequest struct {
	// CachePoolID 缓存池 ID
	CachePoolID string `json:"cache_pool_id" binding:"required"`
	// BackendDevice 后端存储设备路径
	BackendDevice string `json:"backend_device" binding:"required"`
	// MountPoint 挂载点
	MountPoint string `json:"mount_point" binding:"required"`
	// Policy 缓存策略，可选覆盖缓存池默认策略
	Policy CachePolicy `json:"policy,omitempty"`
	// BlockSizeKB 缓存块大小（KB），默认 256
	BlockSizeKB int `json:"block_size_kb,omitempty"`
}

// CreateTierRuleRequest 创建分层规则请求
type CreateTierRuleRequest struct {
	// Name 规则名称
	Name string `json:"name" binding:"required"`
	// Description 规则描述
	Description string `json:"description,omitempty"`
	// HotThreshold 热数据阈值
	HotThreshold int `json:"hot_threshold" binding:"required,min=1"`
	// ColdThreshold 冷数据阈值
	ColdThreshold int `json:"cold_threshold" binding:"required,min=1"`
	// PromoteEnabled 是否启用自动提升
	PromoteEnabled bool `json:"promote_enabled"`
	// DemoteEnabled 是否启用自动降级
	DemoteEnabled bool `json:"demote_enabled"`
	// PromoteScheduleMB 提升调度阈值
	PromoteScheduleMB int `json:"promote_schedule_mb,omitempty"`
}

// CreateWarmupRequest 创建预热任务请求
type CreateWarmupRequest struct {
	// Name 任务名称
	Name string `json:"name" binding:"required"`
	// CachePoolID 缓存池 ID
	CachePoolID string `json:"cache_pool_id" binding:"required"`
	// SourcePath 数据源路径
	SourcePath string `json:"source_path" binding:"required"`
	// FilePattern 文件匹配模式
	FilePattern string `json:"file_pattern,omitempty"`
}

// RegisterDeviceRequest 注册设备请求
type RegisterDeviceRequest struct {
	// Name 设备名称
	Name string `json:"name" binding:"required"`
	// Path 设备路径
	Path string `json:"path" binding:"required"`
	// Role 设备角色
	Role DeviceRole `json:"role" binding:"required"`
	// CapacityGB 容量（GB），为 0 时自动检测
	CapacityGB int64 `json:"capacity_gb,omitempty"`
}

// UpdatePolicyRequest 更新策略请求
type UpdatePolicyRequest struct {
	// Policy 缓存策略
	Policy CachePolicy `json:"policy" binding:"required"`
	// EvictionPolicy 淘汰策略
	EvictionPolicy EvictionPolicy `json:"eviction_policy,omitempty"`
}

// FlushRequest 刷回请求
type FlushRequest struct {
	// CachePoolID 缓存池 ID
	CachePoolID string `json:"cache_pool_id" binding:"required"`
	// Force 强制刷回，忽略同步限制
	Force bool `json:"force,omitempty"`
}

// CacheGlobalConfig 全局缓存配置
type CacheGlobalConfig struct {
	// Enabled 是否启用缓存模块
	Enabled bool `json:"enabled"`
	// DefaultPolicy 默认缓存策略
	DefaultPolicy CachePolicy `json:"default_policy"`
	// DefaultEviction 默认淘汰策略
	DefaultEviction EvictionPolicy `json:"default_eviction"`
	// DefaultBlockSizeKB 默认块大小（KB）
	DefaultBlockSizeKB int `json:"default_block_size_kb"`
	// StatsIntervalSec 统计采集间隔（秒）
	StatsIntervalSec int `json:"stats_interval_sec"`
	// AutoTierEnabled 是否启用自动分层
	AutoTierEnabled bool `json:"auto_tier_enabled"`
	// FlushIntervalSec 脏数据刷回间隔（秒）
	FlushIntervalSec int `json:"flush_interval_sec"`
	// DirtyRatioThreshold 脏数据比例阈值（百分比）
	DirtyRatioThreshold int `json:"dirty_ratio_threshold"`
	// ConsistencyCheckEnabled 是否启用一致性检查
	ConsistencyCheckEnabled bool `json:"consistency_check_enabled"`
}

// DefaultCacheGlobalConfig 返回默认全局配置
func DefaultCacheGlobalConfig() *CacheGlobalConfig {
	return &CacheGlobalConfig{
		Enabled:                 true,
		DefaultPolicy:           PolicyWriteBack,
		DefaultEviction:         EvictionLRU,
		DefaultBlockSizeKB:      256,
		StatsIntervalSec:        60,
		AutoTierEnabled:         true,
		FlushIntervalSec:        300,
		DirtyRatioThreshold:     40,
		ConsistencyCheckEnabled: true,
	}
}

// IsValidPolicy 检查缓存策略是否有效
func IsValidPolicy(p CachePolicy) bool {
	switch p {
	case PolicyWriteBack, PolicyWriteThrough, PolicyReadAhead:
		return true
	}
	return false
}

// IsValidEviction 检查淘汰策略是否有效
func IsValidEviction(p EvictionPolicy) bool {
	switch p {
	case EvictionLRU, EvictionLFU, EvictionARC:
		return true
	}
	return false
}

// IsValidRAIDLevel 检查 RAID 级别是否有效
func IsValidRAIDLevel(level RAIDLevel) bool {
	switch level {
	case RAID0, RAID1, RAID5, RAID10, "":
		return true
	}
	return false
}

// SupportedPolicies 返回所有支持的缓存策略
func SupportedPolicies() []CachePolicy {
	return []CachePolicy{PolicyWriteBack, PolicyWriteThrough, PolicyReadAhead}
}

// SupportedEvictions 返回所有支持的淘汰策略
func SupportedEvictions() []EvictionPolicy {
	return []EvictionPolicy{EvictionLRU, EvictionLFU, EvictionARC}
}

// SupportedRAIDLevels 返回所有支持的 RAID 级别
func SupportedRAIDLevels() []RAIDLevel {
	return []RAIDLevel{RAID0, RAID1, RAID5, RAID10}
}
