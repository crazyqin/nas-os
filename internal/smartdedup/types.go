// Package smartdedup 提供智能文件去重功能。
//
// 支持基于内容哈希的精确去重和基于感知哈希的相似文件检测，
// 包含智能保留策略和安全删除机制。
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

// FileInfo 文件元信息。
type FileInfo struct {
	Path         string      `json:"path"`          // 文件路径
	Size         int64       `json:"size"`          // 文件大小（字节）
	ModTime      time.Time   `json:"modTime"`       // 修改时间
	AccessTime   time.Time   `json:"accessTime"`    // 访问时间
	ContentHash  string      `json:"contentHash"`   // 内容哈希（精确匹配）
	PerceptHash  string      `json:"perceptHash"`   // 感知哈希（相似匹配）
	ContentType  ContentType `json:"contentType"`   // 内容类型
	IsDeduped    bool        `json:"isDeduped"`     // 是否已去重
	RefCount     int         `json:"refCount"`      // 引用次数
	UsageCount   int         `json:"usageCount"`    // 使用次数（用于 most_used 策略）
}

// DuplicateGroup 重复文件组。
// ContentHash 相同的文件归为一组。
type DuplicateGroup struct {
	ContentHash string     `json:"contentHash"` // 组标识哈希
	Files       []*FileInfo `json:"files"`       // 组内文件列表
	TotalSize   int64      `json:"totalSize"`   // 组内文件总大小
	SavedSize   int64      `json:"savedSize"`   // 去重后可节省空间
}

// SimilarGroup 相似文件组。
// PerceptHash 相似度超过阈值的文件归为一组。
type SimilarGroup struct {
	GroupID    string     `json:"groupId"`    // 组标识
	HashValue  string     `json:"hashValue"`  // 感知哈希值
	Files      []*FileInfo `json:"files"`      // 组内文件列表
	Threshold  float64    `json:"threshold"`  // 相似度阈值
	Similarity float64    `json:"similarity"` // 组内平均相似度
}

// ScanResult 扫描结果。
type ScanResult struct {
	StartTime       time.Time        `json:"startTime"`
	EndTime         time.Time        `json:"endTime"`
	Duration        time.Duration    `json:"duration"`
	TotalFiles      int              `json:"totalFiles"`
	TotalSize       int64            `json:"totalSize"`
	DuplicateGroups []*DuplicateGroup `json:"duplicateGroups"`
	SimilarGroups   []*SimilarGroup  `json:"similarGroups"`
	DuplicateCount  int              `json:"duplicateCount"`
	DuplicateSize   int64            `json:"duplicateSize"`
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
	Errors        []DedupError  `json:"errors,omitempty"`
}

// DedupError 去重执行错误。
type DedupError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
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
	PerceptualEnabled bool            `json:"perceptualEnabled"`
	PerceptThreshold  float64         `json:"perceptThreshold"`
	SafeDelete        bool            `json:"safeDelete"`
	TrashPath         string          `json:"trashPath"`
	MaxWorkers        int             `json:"maxWorkers"`
	DryRun            bool            `json:"dryRun"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Enabled:           true,
		ScanPaths:         []string{},
		ExcludePaths:      []string{"/proc", "/sys", "/dev", "/tmp"},
		ExcludePatterns:   []string{".git", "node_modules", ".cache"},
		MinFileSize:       0,
		MaxFileSize:       0, // 无限制
		RetentionPolicy:   RetainNewest,
		PerceptualEnabled: false,
		PerceptThreshold:  0.95,
		SafeDelete:        true,
		TrashPath:         "/tmp/smartdedup-trash",
		MaxWorkers:        4,
		DryRun:            false,
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

// AddDedup 记录一次去重结果。
func (s *DedupStats) AddDedup(result *DedupResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalDeleted += int64(result.Deleted)
	s.TotalTrashed += int64(result.Trashed)
	s.TotalErrors += int64(result.Failed)
}
