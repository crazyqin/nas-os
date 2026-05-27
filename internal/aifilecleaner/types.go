// Package aifilecleaner 提供AI智能文件清理功能
package aifilecleaner

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNoScanData 没有扫描数据.
	ErrNoScanData = errors.New("尚未执行文件扫描，请先调用 POST /scan")
	// ErrTaskNotFound 清理任务不存在.
	ErrTaskNotFound = errors.New("清理任务不存在")
	// ErrAlreadyScanning 扫描正在进行中.
	ErrAlreadyScanning = errors.New("文件扫描正在进行中")
	// ErrInvalidPath 无效扫描路径.
	ErrInvalidPath = errors.New("无效的扫描路径")
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrInvalidDeleteMode 无效的删除模式.
	ErrInvalidDeleteMode = errors.New("无效的删除模式")
	// ErrTaskAlreadyRunning 任务已在运行.
	ErrTaskAlreadyRunning = errors.New("清理任务已在运行中")
	// ErrTaskNotRunning 任务不在运行中.
	ErrTaskNotRunning = errors.New("清理任务不在运行中")
)

// ========== 删除模式 ==========

// DeleteMode 删除模式.
type DeleteMode string

const (
	// DeleteModeRecycle 回收站模式（可恢复）.
	DeleteModeRecycle DeleteMode = "recycle"
	// DeleteModePermanent 永久删除（不可恢复）.
	DeleteModePermanent DeleteMode = "permanent"
)

// ========== 扫描配置 ==========

// ScanConfig 扫描配置.
type ScanConfig struct {
	// RootPath 扫描根路径.
	RootPath string `json:"root_path"`
	// LargeFileThresholdMB 大文件阈值（MB）.
	LargeFileThresholdMB int `json:"large_file_threshold_mb"`
	// StaleDays 未访问天数阈值.
	StaleDays int `json:"stale_days"`
	// MaxDepth 最大扫描深度.
	MaxDepth int `json:"max_depth"`
	// EnableDedupCheck 是否启用重复文件检测.
	EnableDedupCheck bool `json:"enable_dedup_check"`
	// TempFilePatterns 临时文件模式列表.
	TempFilePatterns []string `json:"temp_file_patterns,omitempty"`
	// CacheDirs 缓存目录列表.
	CacheDirs []string `json:"cache_dirs,omitempty"`
}

// DefaultScanConfig 返回默认扫描配置.
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		RootPath:             "/",
		LargeFileThresholdMB: 100,
		StaleDays:            90,
		MaxDepth:             10,
		EnableDedupCheck:     true,
		TempFilePatterns: []string{
			"*.tmp", "*.temp", "*.bak", "*.swp", "*.swo",
			"*~", ".*.swp", "Thumbs.db", ".DS_Store",
		},
		CacheDirs: []string{
			".cache", "cache", "tmp", "temp",
			"node_modules", ".npm", "__pycache__",
		},
	}
}

// ========== 文件信息 ==========

// FileInfo 文件信息.
type FileInfo struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"` // 文件名
	Size         int64     `json:"size"` // 字节
	ModTime      time.Time `json:"mod_time"`
	AccessTime   time.Time `json:"access_time"`
	IsDir        bool      `json:"is_dir"`
	Extension    string    `json:"extension"`
	Hash         string    `json:"hash,omitempty"` // 文件哈希（去重用）
	DaysSinceUse int       `json:"days_since_use"`
	IsTemp       bool      `json:"is_temp"`
	IsCache      bool      `json:"is_cache"`
	SafeScore    float64   `json:"safe_score"` // 0-100，越高越安全删除
}

// ========== 重复文件检测 ==========

// DuplicateGroup 重复文件组.
type DuplicateGroup struct {
	Hash  string     `json:"hash"`
	Size  int64      `json:"size"` // 单文件大小
	Count int        `json:"count"`
	Files []*FileInfo `json:"files"`
	// WastedBytes 浪费的空间（(count-1) * size）.
	WastedBytes int64 `json:"wasted_bytes"`
	// FuzzyMatch 模糊匹配（相似文件名）.
	FuzzyMatch bool `json:"fuzzy_match,omitempty"`
}

// ========== 空间分析 ==========

// SpaceAnalysis 空间占用分析.
type SpaceAnalysis struct {
	TotalSize     int64            `json:"total_size"`
	UsedSize      int64            `json:"used_size"`
	FreeSize      int64            `json:"free_size"`
	ByExtension   []ExtStat        `json:"by_extension"`
	ByDirectory   []DirSizeStat    `json:"by_directory"`
	ByAge         []AgeStat        `json:"by_age"`
	ByCategory    []CategoryStat   `json:"by_category"`
}

// ExtStat 文件扩展名统计.
type ExtStat struct {
	Extension  string  `json:"extension"`
	Count      int     `json:"count"`
	TotalBytes int64   `json:"total_bytes"`
	Percentage float64 `json:"percentage"`
}

// DirSizeStat 目录大小统计.
type DirSizeStat struct {
	Path       string  `json:"path"`
	TotalSize  int64   `json:"total_size"`
	FileCount  int     `json:"file_count"`
	Percentage float64 `json:"percentage"`
}

// AgeStat 文件年龄统计.
type AgeStat struct {
	Range      string `json:"range"` // "0-7天", "7-30天", etc.
	Count      int    `json:"count"`
	TotalBytes int64  `json:"total_bytes"`
}

// CategoryStat 文件类别统计.
type CategoryStat struct {
	Category   string `json:"category"` // "图片", "视频", "文档", etc.
	Count      int    `json:"count"`
	TotalBytes int64  `json:"total_bytes"`
}

// ========== 清理建议 ==========

// CleanupSuggestion 清理建议.
type CleanupSuggestion struct {
	ID               string           `json:"id"`
	Type             SuggestionType   `json:"type"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	Priority         int              `json:"priority"` // 1=高 2=中 3=低
	SafeScore        float64          `json:"safe_score"` // 0-100
	EstimatedSaving  int64            `json:"estimated_saving_bytes"`
	TargetFiles      []string         `json:"target_files,omitempty"`
	TargetPath       string           `json:"target_path,omitempty"`
	Applied          bool             `json:"applied"`
	AppliedAt        *time.Time       `json:"applied_at,omitempty"`
}

// SuggestionType 建议类型.
type SuggestionType string

const (
	// SuggestionTypeDedup 重复文件清理.
	SuggestionTypeDedup SuggestionType = "dedup"
	// SuggestionTypeLargeFile 大文件清理.
	SuggestionTypeLargeFile SuggestionType = "large_file"
	// SuggestionTypeTempFile 临时文件清理.
	SuggestionTypeTempFile SuggestionType = "temp_file"
	// SuggestionTypeCacheFile 缓存文件清理.
	SuggestionTypeCacheFile SuggestionType = "cache_file"
	// SuggestionTypeStaleFile 陈旧文件归档.
	SuggestionTypeStaleFile SuggestionType = "stale_file"
	// SuggestionTypeOldBackup 旧备份清理.
	SuggestionTypeOldBackup SuggestionType = "old_backup"
)

// ========== 清理任务 ==========

// CleanupTask 清理任务.
type CleanupTask struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Status      TaskStatus   `json:"status"`
	DeleteMode  DeleteMode   `json:"delete_mode"`
	TargetFiles []string     `json:"target_files"`
	TotalSize   int64        `json:"total_size"` // 预计清理大小
	FreedSize   int64        `json:"freed_size"` // 实际释放大小
	FileCount   int          `json:"file_count"`
	CreatedAt   time.Time    `json:"created_at"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// TaskStatus 任务状态.
type TaskStatus string

const (
	// TaskStatusPending 等待执行.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 执行中.
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusCompleted 已完成.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 失败.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusCancelled 已取消.
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ========== 清理计划 ==========

// CleanupSchedule 清理计划.
type CleanupSchedule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	CronExpr    string       `json:"cron_expr"` // cron表达式
	RootPath    string       `json:"root_path"`
	DeleteMode  DeleteMode   `json:"delete_mode"`
	Enabled     bool         `json:"enabled"`
	LastRun     *time.Time   `json:"last_run,omitempty"`
	NextRun     *time.Time   `json:"next_run,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

// ========== 空间对比 ==========

// SpaceComparison 清理前后空间对比.
type SpaceComparison struct {
	Before       SpaceSnapshot `json:"before"`
	After        SpaceSnapshot `json:"after"`
	FreedBytes   int64         `json:"freed_bytes"`
	FreedPercent float64       `json:"freed_percent"`
	TasksCount   int           `json:"tasks_count"`
	FilesDeleted int           `json:"files_deleted"`
	ComparedAt   time.Time     `json:"compared_at"`
}

// SpaceSnapshot 空间快照.
type SpaceSnapshot struct {
	TotalBytes int64     `json:"total_bytes"`
	UsedBytes  int64     `json:"used_bytes"`
	FreeBytes  int64     `json:"free_bytes"`
	Timestamp  time.Time `json:"timestamp"`
}

// ========== 扫描结果 ==========

// ScanResult 扫描结果.
type ScanResult struct {
	RootPath          string             `json:"root_path"`
	ScanStartedAt     time.Time          `json:"scan_started_at"`
	ScanFinishedAt    time.Time          `json:"scan_finished_at"`
	DurationSeconds   float64            `json:"duration_seconds"`
	TotalFiles        int                `json:"total_files"`
	TotalDirs         int                `json:"total_dirs"`
	TotalSizeBytes    int64              `json:"total_size_bytes"`
	TempFiles         []FileInfo         `json:"temp_files"`
	TempFilesSize     int64              `json:"temp_files_size"`
	CacheFiles        []FileInfo         `json:"cache_files"`
	CacheFilesSize    int64              `json:"cache_files_size"`
	LargeFiles        []FileInfo         `json:"large_files"`
	StaleFiles        []FileInfo         `json:"stale_files"`
	DuplicateGroups   []DuplicateGroup   `json:"duplicate_groups"`
	DuplicateWaste    int64              `json:"duplicate_waste"`
	SpaceAnalysis     *SpaceAnalysis     `json:"space_analysis,omitempty"`
	Suggestions       []CleanupSuggestion `json:"suggestions"`
}

// ========== Mock数据类型 ==========

// MockConfig Mock数据配置.
type MockConfig struct {
	// FileCount 模拟文件数量.
	FileCount int `json:"file_count"`
	// DirCount 模拟目录数量.
	DirCount int `json:"dir_count"`
	// DuplicatePercent 重复文件比例.
	DuplicatePercent float64 `json:"duplicate_percent"`
	// TempPercent 临时文件比例.
	TempPercent float64 `json:"temp_percent"`
}
