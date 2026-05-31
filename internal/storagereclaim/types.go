// Package storagereclaim 提供智能存储空间回收功能。
// 参考 TrueNAS 存储管理特性：
// - 垃圾文件检测（临时文件、缓存、重复文件、孤立快照）
// - 智能评分系统（根据文件大小、最后访问时间、重要性评分）
// - 自动回收策略（可配置阈值触发自动清理）
// - 重复文件检测（基于内容哈希 xxhash）
// - 存储空间分析（按目录/用户/文件类型统计）
// - 回收站集成（安全删除，可恢复）
// - 定时扫描任务
package storagereclaim

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrFileNotFound 表示文件不存在。
	ErrFileNotFound = errors.New("文件不存在")
	// ErrInvalidConfig 表示配置无效。
	ErrInvalidConfig = errors.New("配置无效")
	// ErrScanInProgress 表示扫描正在进行中。
	ErrScanInProgress = errors.New("扫描正在进行中")
	// ErrAlreadyDeleted 表示文件已被删除。
	ErrAlreadyDeleted = errors.New("文件已被删除")
	// ErrRestoreFailed 表示恢复失败。
	ErrRestoreFailed = errors.New("恢复失败")
)

// ========== 垃圾文件类型 ==========

// JunkFileType 垃圾文件类型。
type JunkFileType string

const (
	// JunkTypeTemp 临时文件。
	JunkTypeTemp JunkFileType = "temp"
	// JunkTypeCache 缓存文件。
	JunkTypeCache JunkFileType = "cache"
	// JunkTypeDuplicate 重复文件。
	JunkTypeDuplicate JunkFileType = "duplicate"
	// JunkTypeOrphanSnapshot 孤立快照。
	JunkTypeOrphanSnapshot JunkFileType = "orphan_snapshot"
	// JunkTypeOldLog 旧日志文件。
	JunkTypeOldLog JunkFileType = "old_log"
)

// AllJunkFileTypes 返回所有支持的垃圾文件类型。
func AllJunkFileTypes() []JunkFileType {
	return []JunkFileType{
		JunkTypeTemp,
		JunkTypeCache,
		JunkTypeDuplicate,
		JunkTypeOrphanSnapshot,
		JunkTypeOldLog,
	}
}

// ========== 文件重要性等级 ==========

// ImportanceLevel 文件重要性等级。
type ImportanceLevel int

const (
	// ImportanceLow 低重要性。
	ImportanceLow ImportanceLevel = 1
	// ImportanceMedium 中等重要性。
	ImportanceMedium ImportanceLevel = 2
	// ImportanceHigh 高重要性。
	ImportanceHigh ImportanceLevel = 3
	// ImportanceCritical 关键文件。
	ImportanceCritical ImportanceLevel = 4
)

// ========== 扫描状态 ==========

// ScanStatus 扫描状态。
type ScanStatus string

const (
	// ScanStatusIdle 空闲。
	ScanStatusIdle ScanStatus = "idle"
	// ScanStatusScanning 扫描中。
	ScanStatusScanning ScanStatus = "scanning"
	// ScanStatusCompleted 已完成。
	ScanStatusCompleted ScanStatus = "completed"
	// ScanStatusFailed 失败。
	ScanStatusFailed ScanStatus = "failed"
)

// ========== 回收站状态 ==========

// RecycleStatus 回收站文件状态。
type RecycleStatus string

const (
	// RecycleStatusActive 可恢复。
	RecycleStatusActive RecycleStatus = "active"
	// RecycleStatusPurged 已彻底删除。
	RecycleStatusPurged RecycleStatus = "purged"
)

// ========== 文件信息 ==========

// FileInfo 文件信息。
type FileInfo struct {
	ID           string          `json:"id"`             // 文件唯一标识
	Path         string          `json:"path"`           // 文件路径
	Name         string          `json:"name"`           // 文件名
	Size         int64           `json:"size"`           // 文件大小（字节）
	Extension    string          `json:"extension"`      // 文件扩展名
	Owner        string          `json:"owner"`          // 文件所有者
	CreatedAt    time.Time       `json:"created_at"`     // 创建时间
	ModifiedAt   time.Time       `json:"modified_at"`    // 修改时间
	AccessedAt   time.Time       `json:"accessed_at"`    // 最后访问时间
	IsJunk       bool            `json:"is_junk"`        // 是否为垃圾文件
	JunkType     JunkFileType    `json:"junk_type"`      // 垃圾文件类型
	Importance   ImportanceLevel `json:"importance"`     // 重要性等级
	ContentHash  string          `json:"content_hash"`   // 内容哈希（xxhash）
	DuplicateOf  string          `json:"duplicate_of"`   // 重复文件的源文件ID
	ReclaimScore float64         `json:"reclaim_score"`  // 回收评分 0-100（越高越应该回收）
	IsDeleted    bool            `json:"is_deleted"`     // 是否已移入回收站
	DeletedAt    *time.Time      `json:"deleted_at"`     // 删除时间
}

// ========== 重复文件组 ==========

// DuplicateGroup 重复文件组。
type DuplicateGroup struct {
	Hash       string      `json:"hash"`        // 文件内容哈希
	FileSize   int64       `json:"file_size"`   // 单个文件大小
	TotalSize  int64       `json:"total_size"`  // 组内总大小（含所有副本）
	FileCount  int         `json:"file_count"`  // 文件数量
	Files      []*FileInfo `json:"files"`       // 文件列表
	WastedSize int64       `json:"wasted_size"` // 浪费的空间（总大小 - 单个文件大小）
}

// ========== 空间统计 ==========

// DirectoryStats 目录统计。
type DirectoryStats struct {
	Path          string  `json:"path"`            // 目录路径
	FileCount     int     `json:"file_count"`      // 文件数量
	TotalSize     int64   `json:"total_size"`      // 总大小
	JunkCount     int     `json:"junk_count"`      // 垃圾文件数量
	JunkSize      int64   `json:"junk_size"`       // 垃圾文件大小
	Reclaimable   int64   `json:"reclaimable"`     // 可回收空间
	ReclaimRatio  float64 `json:"reclaim_ratio"`   // 可回收比例
}

// FileTypeStats 文件类型统计。
type FileTypeStats struct {
	Extension string `json:"extension"` // 文件扩展名
	Count     int    `json:"count"`     // 文件数量
	TotalSize int64  `json:"total_size"` // 总大小
	AvgSize   int64  `json:"avg_size"`  // 平均大小
}

// UserStats 用户统计。
type UserStats struct {
	Owner       string `json:"owner"`        // 用户名
	FileCount   int    `json:"file_count"`   // 文件数量
	TotalSize   int64  `json:"total_size"`   // 总大小
	JunkCount   int    `json:"junk_count"`   // 垃圾文件数量
	JunkSize    int64  `json:"junk_size"`    // 垃圾文件大小
	QuotaUsage  int64  `json:"quota_usage"`  // 配额使用量
	QuotaLimit  int64  `json:"quota_limit"`  // 配额限制
}

// StorageOverview 存储总览。
type StorageOverview struct {
	TotalCapacity  int64            `json:"total_capacity"`  // 总容量
	UsedSpace      int64            `json:"used_space"`      // 已用空间
	FreeSpace      int64            `json:"free_space"`      // 剩余空间
	JunkSpace      int64            `json:"junk_space"`      // 垃圾文件占用
	Reclaimable    int64            `json:"reclaimable"`     // 可回收空间
	DuplicateSpace int64            `json:"duplicate_space"` // 重复文件占用
	FileCount      int              `json:"file_count"`      // 文件总数
	JunkCount      int              `json:"junk_count"`      // 垃圾文件数
	DirectoryStats []*DirectoryStats `json:"directory_stats"` // 目录统计
	FileTypeStats  []*FileTypeStats  `json:"file_type_stats"` // 文件类型统计
	UserStats      []*UserStats      `json:"user_stats"`      // 用户统计
}

// ========== 回收站 ==========

// RecycleBinItem 回收站项目。
type RecycleBinItem struct {
	FileID      string     `json:"file_id"`      // 文件ID
	OriginalPath string    `json:"original_path"` // 原始路径
	Name        string     `json:"name"`         // 文件名
	Size        int64      `json:"size"`         // 文件大小
	DeletedAt   time.Time  `json:"deleted_at"`   // 删除时间
	DeletedBy   string     `json:"deleted_by"`   // 删除者
	Status      RecycleStatus `json:"status"`     // 状态
	PurgeAt     *time.Time `json:"purge_at"`     // 预计彻底删除时间
}

// RecycleBinStats 回收站统计。
type RecycleBinStats struct {
	ItemCount    int   `json:"item_count"`    // 项目数量
	TotalSize    int64 `json:"total_size"`    // 总占用空间
	OldestItem   *time.Time `json:"oldest_item"` // 最旧项目时间
	NewestItem   *time.Time `json:"newest_item"` // 最新项目时间
}

// ========== 扫描结果 ==========

// ScanResult 扫描结果。
type ScanResult struct {
	ScanID       string        `json:"scan_id"`        // 扫描ID
	StartedAt    time.Time     `json:"started_at"`     // 开始时间
	FinishedAt   time.Time     `json:"finished_at"`    // 结束时间
	Duration     time.Duration `json:"duration"`       // 耗时
	TotalFiles   int           `json:"total_files"`    // 扫描文件总数
	JunkFiles    int           `json:"junk_files"`     // 发现垃圾文件数
	Duplicates   int           `json:"duplicates"`     // 发现重复文件数
	TotalSize    int64         `json:"total_size"`     // 扫描文件总大小
	JunkSize     int64         `json:"junk_size"`      // 垃圾文件大小
	Reclaimable  int64         `json:"reclaimable"`    // 可回收空间
	Errors       []string      `json:"errors,omitempty"` // 扫描错误
}

// ========== 回收任务 ==========

// ReclaimTask 回收任务。
type ReclaimTask struct {
	ID          string        `json:"id"`           // 任务ID
	StartedAt   time.Time     `json:"started_at"`   // 开始时间
	FinishedAt  *time.Time    `json:"finished_at"`  // 完成时间
	Status      string        `json:"status"`       // 状态: pending, running, completed, failed
	FileCount   int           `json:"file_count"`   // 回收文件数
	Reclaimed   int64         `json:"reclaimed"`    // 回收空间（字节）
	FailedCount int           `json:"failed_count"` // 失败数量
	Errors      []string      `json:"errors,omitempty"` // 错误信息
	DryRun      bool          `json:"dry_run"`      // 是否为模拟运行
}

// ========== 定时任务配置 ==========

// ScheduleConfig 定时扫描配置。
type ScheduleConfig struct {
	Enabled    bool   `json:"enabled"`     // 是否启用
	Cron       string `json:"cron"`        // Cron 表达式
	ScanPaths  []string `json:"scan_paths"` // 扫描路径
	AutoReclaim bool  `json:"auto_reclaim"` // 是否自动回收
}

// ========== 回收配置 ==========

// ReclaimConfig 回收配置。
type ReclaimConfig struct {
	// 垃圾文件检测配置
	TempExtensions    []string `json:"temp_extensions"`    // 临时文件扩展名列表
	CachePaths        []string `json:"cache_paths"`        // 缓存目录列表
	OldLogDays        int      `json:"old_log_days"`       // 旧日志天数阈值
	OldSnapshotDays   int      `json:"old_snapshot_days"`  // 旧快照天数阈值

	// 评分权重配置
	SizeWeight        float64 `json:"size_weight"`        // 文件大小权重
	AccessWeight      float64 `json:"access_weight"`      // 访问时间权重
	ImportanceWeight  float64 `json:"importance_weight"`  // 重要性权重

	// 回收阈值配置
	ReclaimThreshold  float64 `json:"reclaim_threshold"`  // 回收评分阈值（高于此值将被回收）
	StorageThreshold  float64 `json:"storage_threshold"`  // 存储使用率阈值（触发自动回收）
	DryRun            bool    `json:"dry_run"`            // 模拟运行模式

	// 回收站配置
	RecycleBinPath    string `json:"recycle_bin_path"`    // 回收站路径
	RetentionDays     int    `json:"retention_days"`      // 回收站保留天数
	MaxRecycleSize    int64  `json:"max_recycle_size"`    // 回收站最大容量（字节）

	// 扫描配置
	ScanConcurrency   int    `json:"scan_concurrency"`   // 并发扫描数
	ScanPaths         []string `json:"scan_paths"`       // 默认扫描路径
	ScheduleEnabled   bool   `json:"schedule_enabled"`   // 是否启用定时扫描
	ScheduleCron      string `json:"schedule_cron"`      // 定时扫描 Cron 表达式
}

// DefaultReclaimConfig 返回默认回收配置。
func DefaultReclaimConfig() *ReclaimConfig {
	return &ReclaimConfig{
		TempExtensions: []string{
			".tmp", ".temp", ".bak", ".swp", ".swo",
			"~", ".cache", ".log.old",
		},
		CachePaths: []string{
			"/var/cache",
			"/tmp",
			"/var/tmp",
		},
		OldLogDays:      30,
		OldSnapshotDays: 90,

		SizeWeight:       0.3,
		AccessWeight:     0.4,
		ImportanceWeight: 0.3,

		ReclaimThreshold: 60.0,
		StorageThreshold: 85.0,
		DryRun:           false,

		RecycleBinPath: "/.recycle",
		RetentionDays:  30,
		MaxRecycleSize: 10 * 1024 * 1024 * 1024, // 10GB

		ScanConcurrency: 4,
		ScanPaths:       []string{"/data"},
		ScheduleEnabled: false,
		ScheduleCron:    "0 3 * * *", // 每天凌晨3点
	}
}

// ========== 查询参数 ==========

// ScanQuery 扫描查询参数。
type ScanQuery struct {
	Paths []string `form:"paths"` // 扫描路径
	Types []string `form:"types"` // 垃圾文件类型过滤
}

// ReclaimQuery 回收查询参数。
type ReclaimQuery struct {
	DryRun      bool     `form:"dry_run"`      // 模拟运行
	MinScore    float64  `form:"min_score"`    // 最低回收评分
	Types       []string `form:"types"`        // 回收的垃圾文件类型
	MaxFiles    int      `form:"max_files"`    // 最大回收文件数
}

// RecycleBinQuery 回收站查询参数。
type RecycleBinQuery struct {
	Limit  int    `form:"limit"`  // 最大条数
	Offset int    `form:"offset"` // 偏移量
	Status string `form:"status"` // 状态过滤
}

// AnalysisQuery 分析查询参数。
type AnalysisQuery struct {
	GroupBy string `form:"group_by"` // 分组方式: directory, user, type
	Path    string `form:"path"`     // 指定路径
}
