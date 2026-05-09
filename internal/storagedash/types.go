// Package storagedash 提供统一存储仪表盘功能，
// 参考群晖"One view for all storage"和TrueNAS Dashboard设计理念。
package storagedash

import (
	"time"
)

// StorageOverview 存储总体概览
type StorageOverview struct {
	// TotalCapacity 总容量（字节）
	TotalCapacity int64 `json:"total_capacity"`
	// UsedCapacity 已用容量（字节）
	UsedCapacity int64 `json:"used_capacity"`
	// FreeCapacity 剩余容量（字节）
	FreeCapacity int64 `json:"free_capacity"`
	// Utilization 使用率（0.0~1.0）
	Utilization float64 `json:"utilization"`
	// Health 健康状态：healthy / warning / critical
	Health string `json:"health"`
	// Pools 存储池摘要列表
	Pools []PoolSummary `json:"pools"`
	// Tiers 存储分层摘要列表
	Tiers []TierSummary `json:"tiers"`
}

// PoolSummary 存储池摘要
type PoolSummary struct {
	// Name 池名称
	Name string `json:"name"`
	// Status 池状态：online / degraded / faulted / offline
	Status string `json:"status"`
	// UsedBytes 已用空间（字节）
	UsedBytes int64 `json:"used_bytes"`
	// TotalBytes 总空间（字节）
	TotalBytes int64 `json:"total_bytes"`
	// DiskCount 磁盘数量
	DiskCount int `json:"disk_count"`
	// RAIDLevel RAID级别（如raidz1, raidz2, mirror等）
	RAIDLevel string `json:"raid_level"`
}

// TierSummary 存储分层摘要
type TierSummary struct {
	// Tier 分层名称（如 hot, warm, cold）
	Tier string `json:"tier"`
	// UsedBytes 已用空间（字节）
	UsedBytes int64 `json:"used_bytes"`
	// TotalBytes 总空间（字节）
	TotalBytes int64 `json:"total_bytes"`
	// FileCount 文件数量
	FileCount int64 `json:"file_count"`
	// MigrationPending 待迁移任务数
	MigrationPending int `json:"migration_pending"`
}

// CapacityTrend 容量趋势数据
type CapacityTrend struct {
	// Date 日期
	Date time.Time `json:"date"`
	// UsedBytes 当日已用容量（字节）
	UsedBytes int64 `json:"used_bytes"`
	// GrowthRate 增长率（字节/天）
	GrowthRate float64 `json:"growth_rate"`
	// DaysUntilFull 预计剩余天数（-1 表示无法预测）
	DaysUntilFull int `json:"days_until_full"`
}

// AlertSummary 告警汇总
type AlertSummary struct {
	// Critical 严重告警数量
	Critical int `json:"critical"`
	// Warning 警告告警数量
	Warning int `json:"warning"`
	// Info 信息级告警数量
	Info int `json:"info"`
	// RecentAlerts 最近告警列表（最多50条）
	RecentAlerts []Alert `json:"recent_alerts"`
}

// Alert 单条告警记录
type Alert struct {
	// Level 告警级别：critical / warning / info
	Level string `json:"level"`
	// Message 告警描述
	Message string `json:"message"`
	// Source 来源组件（如 pool, disk, tier）
	Source string `json:"source"`
	// Time 告警时间
	Time time.Time `json:"time"`
}

// PoolProvider 存储池数据提供函数类型
type PoolProvider func() ([]PoolSummary, error)

// TierProvider 存储分层数据提供函数类型
type TierProvider func() ([]TierSummary, error)
