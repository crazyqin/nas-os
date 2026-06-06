// Package smartdedup 提供智能文件去重功能。
//
// 支持多种哈希算法（SHA-256、XXHash、Blake3）和增量扫描，
// 包含硬链接/符号链接处理、智能保留策略和安全删除机制。
package smartdedup

import (
	"fmt"
	"sync"
	"time"
)

// ContentType 文件内容类型。
type ContentType int

const (
	ContentTypeUnknown ContentType = iota
	ContentTypeImage               // 图片文件
	ContentTypeAudio               // 音频文件
	ContentTypeVideo               // 视频文件
	ContentTypeDocument            // 文档文件
	ContentTypeArchive             // 压缩文件
	ContentTypeBinary              // 其他二进制文件
)

// String 返回内容类型的字符串表示。
func (ct ContentType) String() string {
	switch ct {
	case ContentTypeImage:
		return "image"
	case ContentTypeAudio:
		return "audio"
	case ContentTypeVideo:
		return "video"
	case ContentTypeDocument:
		return "document"
	case ContentTypeArchive:
		return "archive"
	case ContentTypeBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// RetentionPolicy 保留策略类型。
type RetentionPolicy int

const (
	RetainNewest  RetentionPolicy = iota // 保留最新文件
	RetainOldest                         // 保留最旧文件
	RetainLargest                        // 保留最大文件
	RetainSmallest                       // 保留最小文件
	RetainMostUsed                       // 保留最常用文件
	RetainShortestPath                   // 保留路径最短的文件
)

// String 返回保留策略的字符串表示。
func (rp RetentionPolicy) String() string {
	switch rp {
	case RetainNewest:
		return "newest"
	case RetainOldest:
		return "oldest"
	case RetainLargest:
		return "largest"
	case RetainSmallest:
		return "smallest"
	case RetainMostUsed:
		return "most_used"
	case RetainShortestPath:
		return "shortest_path"
	default:
		return "unknown"
	}
}

// ScanMode 扫描模式。
type ScanMode int

const (
	ScanModeFull        ScanMode = iota // 全量扫描
	ScanModeIncremental                 // 增量扫描（仅扫描变更文件）
)

// String 返回扫描模式名称。
func (m ScanMode) String() string {
	switch m {
	case ScanModeFull:
		return "full"
	case ScanModeIncremental:
		return "incremental"
	default:
		return "unknown"
	}
}

// FileInfo 文件元信息。
type FileInfo struct {
	Path           string      `json:"path"`            // 文件路径
	Size           int64       `json:"size"`            // 文件大小（字节）
	ModTime        time.Time   `json:"modTime"`         // 修改时间
	AccessTime     time.Time   `json:"accessTime"`      // 访问时间
	ContentHash    string      `json:"contentHash"`     // 内容哈希（精确匹配）
	PerceptHash    string      `json:"perceptHash"`     // 感知哈希（相似匹配）
	ContentType    ContentType `json:"contentType"`     // 内容类型
	IsDeduped      bool        `json:"isDeduped"`       // 是否已去重
	RefCount       int         `json:"refCount"`        // 引用次数
	UsageCount     int         `json:"usageCount"`      // 使用次数（用于 most_used 策略）
	IsHardLink     bool        `json:"isHardLink"`      // 是否为硬链接
	IsSymLink      bool        `json:"isSymLink"`       // 是否为符号链接
	SymLinkTarget  string      `json:"symLinkTarget,omitempty"` // 符号链接目标
	Inode          uint64      `json:"inode,omitempty"` // inode 号（硬链接检测）
	Nlink          uint64      `json:"nlink,omitempty"` // 硬链接数
	HashAlgorithm  string      `json:"hashAlgorithm"`   // 使用的哈希算法
}

// DuplicateGroup 重复文件组。
type DuplicateGroup struct {
	ContentHash string     `json:"contentHash"` // 组标识哈希
	Files       []*FileInfo `json:"files"`       // 组内文件列表
	TotalSize   int64      `json:"totalSize"`   // 组内文件总大小
	SavedSize   int64      `json:"savedSize"`   // 去重后可节省空间
}

// SimilarGroup 相似文件组。
type SimilarGroup struct {
	GroupID    string     `json:"groupId"`
	HashValue  string     `json:"hashValue"`
	Files      []*FileInfo `json:"files"`
	Threshold  float64    `json:"threshold"`
	Similarity float64    `json:"similarity"`
}

// ScanResult 扫描结果。
type ScanResult struct {
	StartTime       time.Time        `json:"startTime"`
	EndTime         time.Time        `json:"endTime"`
	Duration        time.Duration    `json:"duration"`
	ScanMode        ScanMode         `json:"scanMode"`
	TotalFiles      int              `json:"totalFiles"`
	TotalSize       int64            `json:"totalSize"`
	DuplicateGroups []*DuplicateGroup `json:"duplicateGroups"`
	SimilarGroups   []*SimilarGroup  `json:"similarGroups"`
	DuplicateCount  int              `json:"duplicateCount"`
	DuplicateSize   int64            `json:"duplicateSize"`
	HardLinkCount   int              `json:"hardLinkCount"`   // 硬链接文件数
	SymLinkCount    int              `json:"symLinkCount"`    // 符号链接文件数
	SkippedFiles    int              `json:"skippedFiles"`    // 增量扫描跳过的文件数
	Errors          []ScanError      `json:"errors,omitempty"`
}

// ScanError 扫描过程中的错误。
type ScanError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// DedupResult 去重执行结果。
type DedupResult struct {
	StartTime     time.Time     `json:"startTime"`
	EndTime       time.Time     `json:"endTime"`
	Duration      time.Duration `json:"duration"`
	Processed     int           `json:"processed"`     // 处理的文件数
	Deleted       int           `json:"deleted"`       // 删除的文件数
	Trashed       int           `json:"trashed"`       // 移到回收站的文件数
	SavedBytes    int64         `json:"savedBytes"`    // 节省的空间
	Failed        int           `json:"failed"`        // 失败的文件数
	HardLinked    int           `json:"hardLinked"`    // 转换为硬链接的文件数
	Errors        []DedupError  `json:"errors,omitempty"`
}

// DedupError 去重执行错误。
type DedupError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// DedupReport 去重报告（汇总多轮去重的完整报告）。
type DedupReport struct {
	GeneratedAt    time.Time     `json:"generatedAt"`
	TotalScans     int           `json:"totalScans"`
	TotalDedups    int           `json:"totalDedups"`
	TotalFiles     int           `json:"totalFiles"`
	TotalSize      int64         `json:"totalSize"`
	DuplicateCount int           `json:"duplicateCount"`
	DuplicateSize  int64         `json:"duplicateSize"`
	SavedBytes     int64         `json:"savedBytes"`
	TrashedCount   int           `json:"trashedCount"`
	DeletedCount   int           `json:"deletedCount"`
	HardLinkedCount int          `json:"hardLinkedCount"`
	SpaceReclaimed int64         `json:"spaceReclaimed"`
	RecoveryRatio  float64       `json:"recoveryRatio"` // 节省空间/总扫描空间
	GroupsByType   map[string]int `json:"groupsByType"` // 按文件类型的重复组数量
	TopDuplicates  []*DuplicateGroup `json:"topDuplicates,omitempty"` // 最大的重复组
}

// Config 智能去重配置。
type Config struct {
	Enabled           bool            `json:"enabled"`
	ScanPaths         []string        `json:"scanPaths"`
	ExcludePaths      []string        `json:"excludePaths"`
	ExcludePatterns   []string        `json:"excludePatterns"`
	MinFileSize       int64           `json:"minFileSize"`
	MaxFileSize       int64           `json:"maxFileSize"`
	RetentionPolicy   RetentionPolicy `json:"retentionPolicy"`
	HashAlgorithm     HashAlgorithm   `json:"hashAlgorithm"`     // 哈希算法
	PerceptualEnabled bool            `json:"perceptualEnabled"`
	PerceptThreshold  float64         `json:"perceptThreshold"`
	SafeDelete        bool            `json:"safeDelete"`
	TrashPath         string          `json:"trashPath"`
	MaxWorkers        int             `json:"maxWorkers"`
	DryRun            bool            `json:"dryRun"`
	HandleHardLinks   bool            `json:"handleHardLinks"`   // 处理硬链接（相同inode视为同一文件）
	HandleSymLinks    bool            `json:"handleSymLinks"`    // 处理符号链接（跟踪目标文件）
	ConvertToHardLink bool            `json:"convertToHardLink"` // 去重时将重复文件转为硬链接
	IncrementalMode   bool            `json:"incrementalMode"`   // 启用增量扫描
	ReportTopN        int             `json:"reportTopN"`        // 报告中显示的最大重复组数
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Enabled:           true,
		ScanPaths:         []string{},
		ExcludePaths:      []string{"/proc", "/sys", "/dev", "/tmp"},
		ExcludePatterns:   []string{".git", "node_modules", ".cache"},
		MinFileSize:       0,
		MaxFileSize:       0,
		RetentionPolicy:   RetainNewest,
		HashAlgorithm:     HashSHA256,
		PerceptualEnabled: false,
		PerceptThreshold:  0.95,
		SafeDelete:        true,
		TrashPath:         "/tmp/smartdedup-trash",
		MaxWorkers:        4,
		DryRun:            false,
		HandleHardLinks:   true,
		HandleSymLinks:    true,
		ConvertToHardLink: false,
		IncrementalMode:   false,
		ReportTopN:        10,
	}
}

// DedupStats 去重空间回收统计。
type DedupStats struct {
	mu                sync.RWMutex
	TotalScans        int64         `json:"totalScans"`
	TotalFilesScanned int64         `json:"totalFilesScanned"`
	TotalSizeScanned  int64         `json:"totalSizeScanned"`
	TotalDuplicates   int64         `json:"totalDuplicates"`
	TotalSavedBytes   int64         `json:"totalSavedBytes"`
	TotalDeleted      int64         `json:"totalDeleted"`
	TotalTrashed      int64         `json:"totalTrashed"`
	TotalHardLinked   int64         `json:"totalHardLinked"`
	TotalErrors       int64         `json:"totalErrors"`
	LastScanTime      time.Time     `json:"lastScanTime"`
	LastScanDuration  time.Duration `json:"lastScanDuration"`
	RecoveryRatio     float64       `json:"recoveryRatio"`
}

// Snapshot 返回统计信息的快照。
func (s *DedupStats) Snapshot() DedupStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return DedupStats{
		TotalScans:        s.TotalScans,
		TotalFilesScanned: s.TotalFilesScanned,
		TotalSizeScanned:  s.TotalSizeScanned,
		TotalDuplicates:   s.TotalDuplicates,
		TotalSavedBytes:   s.TotalSavedBytes,
		TotalDeleted:      s.TotalDeleted,
		TotalTrashed:      s.TotalTrashed,
		TotalHardLinked:   s.TotalHardLinked,
		TotalErrors:       s.TotalErrors,
		LastScanTime:      s.LastScanTime,
		LastScanDuration:  s.LastScanDuration,
		RecoveryRatio:     s.RecoveryRatio,
	}
}

// AddScan 记录一次扫描结果。
func (s *DedupStats) AddScan(result *ScanResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalScans++
	s.TotalFilesScanned += int64(result.TotalFiles)
	s.TotalSizeScanned += result.TotalSize
	s.TotalDuplicates += int64(result.DuplicateCount)
	s.TotalSavedBytes += result.DuplicateSize
	s.TotalErrors += int64(len(result.Errors))
	s.LastScanTime = result.EndTime
	s.LastScanDuration = result.Duration
	if s.TotalSizeScanned > 0 {
		s.RecoveryRatio = float64(s.TotalSavedBytes) / float64(s.TotalSizeScanned)
	}
}

// AddDedup 记录一次去重结果。
func (s *DedupStats) AddDedup(result *DedupResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalDeleted += int64(result.Deleted)
	s.TotalTrashed += int64(result.Trashed)
	s.TotalHardLinked += int64(result.HardLinked)
	s.TotalErrors += int64(result.Failed)
}

// FormatSize 格式化文件大小为人类可读格式。
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// incrementIndex 增量扫描索引条目。
// 记录文件的哈希和修改时间，用于增量扫描时判断文件是否变更。
type incrementIndex struct {
	mu      sync.RWMutex
	entries map[string]*indexEntry // path -> entry
}

// indexEntry 索引条目。
type indexEntry struct {
	ContentHash string    `json:"contentHash"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
	ScanTime    time.Time `json:"scanTime"`
}

// newIncrementIndex 创建增量扫描索引。
func newIncrementIndex() *incrementIndex {
	return &incrementIndex{
		entries: make(map[string]*indexEntry),
	}
}

// get 获取索引条目。
func (idx *incrementIndex) get(path string) (*indexEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	e, ok := idx.entries[path]
	return e, ok
}

// set 设置索引条目。
func (idx *incrementIndex) set(path string, entry *indexEntry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.entries[path] = entry
}

// remove 删除索引条目。
func (idx *incrementIndex) remove(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.entries, path)
}

// size 返回索引条目数。
func (idx *incrementIndex) size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// needsRescan 判断文件是否需要重新扫描。
func (idx *incrementIndex) needsRescan(path string, info *indexEntry) bool {
	existing, ok := idx.get(path)
	if !ok {
		return true // 新文件
	}
	return existing.ModTime.Before(info.ModTime) || existing.Size != info.Size
}
