// Package aidatadedup 提供 AI 驱动的数据去重功能，支持相似文件检测、内容哈希、智能合并。
package aidatadedup

import "time"

// DedupStrategy 去重策略.
type DedupStrategy string

const (
	StrategyExactHash    DedupStrategy = "exact_hash"    // 精确哈希匹配
	StrategyFuzzyMatch   DedupStrategy = "fuzzy_match"   // 模糊相似度匹配
	StrategyContentAware DedupStrategy = "content_aware" // 内容感知去重
	StrategyAuto         DedupStrategy = "auto"          // 自动选择最优策略
)

// FileType 文件类型.
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeDocument FileType = "document"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeOther    FileType = "other"
)

// DuplicateStatus 去重状态.
type DuplicateStatus string

const (
	StatusPending   DuplicateStatus = "pending"
	StatusAnalyzing DuplicateStatus = "analyzing"
	StatusDuplicate DuplicateStatus = "duplicate"
	StatusUnique    DuplicateStatus = "unique"
	StatusMerged    DuplicateStatus = "merged"
	StatusDeleted   DuplicateStatus = "deleted"
)

// MergeStrategy 合并策略.
type MergeStrategy string

const (
	MergeKeepNewest  MergeStrategy = "keep_newest"  // 保留最新
	MergeKeepOldest  MergeStrategy = "keep_oldest"  // 保留最旧
	MergeKeepLargest MergeStrategy = "keep_largest" // 保留最大
	MergeKeepBest    MergeStrategy = "keep_best"    // 保留质量最佳
	MergeManual      MergeStrategy = "manual"       // 手动选择
)

// FileEntry 文件条目.
type FileEntry struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	FileType    FileType  `json:"file_type"`
	Hash        string    `json:"hash"`         // 精确哈希
	SimHash     string    `json:"sim_hash"`     // 相似度哈希
	PerceptHash string    `json:"percept_hash"` // 感知哈希（图片/音频）
	ModTime     time.Time `json:"mod_time"`
	CreateTime  time.Time `json:"create_time"`
	Extension   string    `json:"extension"`
	IsProcessed bool      `json:"is_processed"`
	GroupID     string    `json:"group_id,omitempty"`
}

// DuplicateGroup 重复文件组.
type DuplicateGroup struct {
	ID           string          `json:"id"`
	Files        []*FileEntry    `json:"files"`
	Similarity   float64         `json:"similarity"` // 0.0 ~ 1.0
	DedupType    DedupStrategy   `json:"dedup_type"`
	Status       DuplicateStatus `json:"status"`
	Recommended  *FileEntry      `json:"recommended,omitempty"` // 推荐保留的文件
	TotalSize    int64           `json:"total_size"`
	SaveableSize int64           `json:"saveable_size"` // 可节省空间
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	ID                  string            `json:"id"`
	ScanPath            string            `json:"scan_path"`
	TotalFiles          int               `json:"total_files"`
	ScannedFiles        int               `json:"scanned_files"`
	DuplicateGroups     []*DuplicateGroup `json:"duplicate_groups"`
	TotalSize           int64             `json:"total_size"`
	DuplicateSize       int64             `json:"duplicate_size"`
	SaveableSize        int64             `json:"saveable_size"`
	SimilarityThreshold float64           `json:"similarity_threshold"`
	StartTime           time.Time         `json:"start_time"`
	EndTime             time.Time         `json:"end_time"`
	Duration            time.Duration     `json:"duration"`
	Status              DuplicateStatus   `json:"status"`
}

// DedupRequest 去重请求.
type DedupRequest struct {
	Paths               []string      `json:"paths" binding:"required"`
	Strategy            DedupStrategy `json:"strategy,omitempty"`
	SimilarityThreshold float64       `json:"similarity_threshold,omitempty"` // 0.0 ~ 1.0
	FileTypes           []FileType    `json:"file_types,omitempty"`
	MinSize             int64         `json:"min_size,omitempty"`
	MaxSize             int64         `json:"max_size,omitempty"`
	ExcludePatterns     []string      `json:"exclude_patterns,omitempty"`
	Recursive           bool          `json:"recursive"`
	DryRun              bool          `json:"dry_run"`
}

// MergeRequest 合并请求.
type MergeRequest struct {
	GroupID      string        `json:"group_id" binding:"required"`
	KeepFileID   string        `json:"keep_file_id,omitempty"`
	Strategy     MergeStrategy `json:"strategy,omitempty"`
	DeleteOthers bool          `json:"delete_others"`
	CreateBackup bool          `json:"create_backup"`
}

// DedupReport 去重报告.
type DedupReport struct {
	ID              string        `json:"id"`
	ScanResultID    string        `json:"scan_result_id"`
	TotalFiles      int           `json:"total_files"`
	DuplicatesFound int           `json:"duplicates_found"`
	GroupsResolved  int           `json:"groups_resolved"`
	FilesDeleted    int           `json:"files_deleted"`
	FilesMerged     int           `json:"files_merged"`
	SpaceSaved      int64         `json:"space_saved"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Duration        time.Duration `json:"duration"`
	Errors          []string      `json:"errors,omitempty"`
}

// AIAnalysisResult AI 分析结果.
type AIAnalysisResult struct {
	FileID      string             `json:"file_id"`
	Similarity  float64            `json:"similarity"`
	Features    map[string]float64 `json:"features"` // 特征向量
	ContentType string             `json:"content_type"`
	IsDuplicate bool               `json:"is_duplicate"`
	Confidence  float64            `json:"confidence"`
	Suggestions []string           `json:"suggestions,omitempty"`
	AnalyzedAt  time.Time          `json:"analyzed_at"`
}

// DedupConfig 去重配置.
type DedupConfig struct {
	Enabled               bool          `json:"enabled"`
	DefaultStrategy       DedupStrategy `json:"default_strategy"`
	SimilarityThreshold   float64       `json:"similarity_threshold"`
	MaxConcurrentScans    int           `json:"max_concurrent_scans"`
	ScanIntervalMinutes   int           `json:"scan_interval_minutes"`
	AutoMerge             bool          `json:"auto_merge"`
	AutoMergeStrategy     MergeStrategy `json:"auto_merge_strategy"`
	CreateBackup          bool          `json:"create_backup"`
	BackupPath            string        `json:"backup_path"`
	ExcludedPaths         []string      `json:"excluded_paths,omitempty"`
	MinFileSize           int64         `json:"min_file_size"`
	MaxFileSize           int64         `json:"max_file_size"`
	EnableAI              bool          `json:"enable_ai"`
	AIConfidenceThreshold float64       `json:"ai_confidence_threshold"`
}

// DefaultDedupConfig 默认去重配置.
func DefaultDedupConfig() *DedupConfig {
	return &DedupConfig{
		Enabled:               true,
		DefaultStrategy:       StrategyAuto,
		SimilarityThreshold:   0.85,
		MaxConcurrentScans:    3,
		ScanIntervalMinutes:   60,
		AutoMerge:             false,
		AutoMergeStrategy:     MergeKeepNewest,
		CreateBackup:          true,
		BackupPath:            "/backup/dedup",
		MinFileSize:           1024,       // 1KB
		MaxFileSize:           1073741824, // 1GB
		EnableAI:              true,
		AIConfidenceThreshold: 0.9,
	}
}

// ScanStatus 扫描状态.
type ScanStatus string

const (
	ScanStatusIdle     ScanStatus = "idle"
	ScanStatusRunning  ScanStatus = "running"
	ScanStatusPaused   ScanStatus = "paused"
	ScanStatusComplete ScanStatus = "complete"
	ScanStatusError    ScanStatus = "error"
)
