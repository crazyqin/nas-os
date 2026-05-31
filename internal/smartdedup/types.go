// Package smartdedup 提供内容感知的智能文件去重功能
package smartdedup

import (
	"sync"
	"time"
)

// StorageBackend 存储后端类型.
type StorageBackend string

const (
	BackendAuto   StorageBackend = "auto"   // 自动检测
	BackendBtrfs  StorageBackend = "btrfs"  // Btrfs
	BackendZFS    StorageBackend = "zfs"    // ZFS
)

// DedupMode 去重模式.
type DedupMode string

const (
	ModeContent  DedupMode = "content"  // 仅内容哈希
	ModeName     DedupMode = "name"     // 仅文件名
	ModeHybrid   DedupMode = "hybrid"   // 混合模式
)

// DedupAction 去重动作.
type DedupAction string

const (
	ActionHardlink     DedupAction = "hardlink"      // 硬链接
	ActionReflink      DedupAction = "reflink"       // CoW 引用
	ActionReflinkPlus  DedupAction = "reflink_plus"  // CoW 引用 + 压缩
	ActionReport       DedupAction = "report"        // 仅报告
)

// DedupEntry 去重条目.
type DedupEntry struct {
	ID          string    `json:"id"`          // 条目ID
	FilePath    string    `json:"filePath"`    // 文件路径
	FileSize    int64     `json:"fileSize"`    // 文件大小
	ContentHash string    `json:"contentHash"` // 内容哈希
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"`   // 更新时间
}

// RefCountEntry 引用计数条目.
type RefCountEntry struct {
	ContentHash string    `json:"contentHash"` // 内容哈希
	SourceFile  string    `json:"sourceFile"`  // 源文件
	RefCount    int       `json:"refCount"`    // 引用计数
	Files       []string  `json:"files"`       // 引用文件列表
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
	mu          sync.Mutex
}

// IncrRef 增加引用计数.
func (r *RefCountEntry) IncrRef(file string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.RefCount++
	r.Files = append(r.Files, file)
	return r.RefCount
}

// DecrRef 减少引用计数.
func (r *RefCountEntry) DecrRef(file string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.RefCount <= 0 {
		return 0, false
	}
	r.RefCount--
	// 移除文件
	for i, f := range r.Files {
		if f == file {
			r.Files = append(r.Files[:i], r.Files[i+1:]...)
			break
		}
	}
	return r.RefCount, r.RefCount > 0
}

// DedupStats 去重统计.
type DedupStats struct {
	mu                sync.Mutex  `json:"-"`
	IsScanning        bool        `json:"isScanning"`
	IsDeduping        bool        `json:"isDeduping"`
	TotalFilesScanned int64       `json:"totalFilesScanned"`
	TotalSizeScanned  int64       `json:"totalSizeScanned"`
	DedupedFiles      int64       `json:"dedupedFiles"`
	SavedSpace        int64       `json:"savedSpace"`
	DedupRatio        float64     `json:"dedupRatio"`
	LastScanTime      time.Time   `json:"lastScanTime"`
	LastDedupTime     time.Time   `json:"lastDedupTime"`
	TotalScanTime     time.Duration `json:"totalScanTime"`
	TotalDedupTime    time.Duration `json:"totalDedupTime"`
}

// GetSnapshot 获取统计快照.
func (s *DedupStats) GetSnapshot() *DedupStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &DedupStats{
		IsScanning:        s.IsScanning,
		IsDeduping:        s.IsDeduping,
		TotalFilesScanned: s.TotalFilesScanned,
		TotalSizeScanned:  s.TotalSizeScanned,
		DedupedFiles:      s.DedupedFiles,
		SavedSpace:        s.SavedSpace,
		DedupRatio:        s.DedupRatio,
		LastScanTime:      s.LastScanTime,
		LastDedupTime:     s.LastDedupTime,
		TotalScanTime:     s.TotalScanTime,
		TotalDedupTime:    s.TotalDedupTime,
	}
}

// UpdateRatio 更新去重比率.
func (s *DedupStats) UpdateRatio() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateRatioLocked()
}

func (s *DedupStats) updateRatioLocked() {
	if s.TotalSizeScanned > 0 {
		s.DedupRatio = float64(s.SavedSpace) / float64(s.TotalSizeScanned)
	}
}

// ScanResult 扫描结果.
type ScanResult struct {
	ScanID          string           `json:"scanId"`
	StartTime       time.Time        `json:"startTime"`
	EndTime         time.Time        `json:"endTime"`
	Duration        time.Duration    `json:"duration"`
	FilesScanned    int              `json:"filesScanned"`
	TotalSize       int64            `json:"totalSize"`
	TotalDuplicates int              `json:"totalDuplicates"`
	PotentialSaving int64            `json:"potentialSaving"`
	DuplicateGroups []DuplicateGroup `json:"duplicateGroups"`
	Errors          []ScanError      `json:"errors,omitempty"`
}

// DuplicateGroup 重复文件组.
type DuplicateGroup struct {
	ContentHash string   `json:"contentHash"` // 内容哈希
	Files       []string `json:"files"`       // 文件列表
	FileCount   int      `json:"fileCount"`   // 文件数量
	UniqueSize  int64    `json:"uniqueSize"`  // 单个文件大小
	TotalSize   int64    `json:"totalSize"`   // 总大小
	SavedSize   int64    `json:"savedSize"`   // 可节省空间
	Status      string   `json:"status"`      // 状态: pending/done/error
}

// ScanError 扫描错误.
type ScanError struct {
	Path  string `json:"path"`  // 文件路径
	Error string `json:"error"` // 错误信息
}

// ScanRequest 扫描请求.
type ScanRequest struct {
	Paths []string `json:"paths"` // 扫描路径
}

// DedupRequest 去重请求.
type DedupRequest struct {
	Groups []DuplicateGroup `json:"groups"` // 重复组
}

// updateConfigRequest 配置更新请求.
type updateConfigRequest struct {
	Enabled         *bool          `json:"enabled,omitempty"`
	Backend         *StorageBackend `json:"backend,omitempty"`
	Mode            *DedupMode     `json:"mode,omitempty"`
	Action          *DedupAction   `json:"action,omitempty"`
	ScanPaths       *[]string      `json:"scanPaths,omitempty"`
	ExcludePaths    *[]string      `json:"excludePaths,omitempty"`
	ExcludePatterns *[]string      `json:"excludePatterns,omitempty"`
	MinFileSize     *int64         `json:"minFileSize,omitempty"`
	MaxFileSize     *int64         `json:"maxFileSize,omitempty"`
	ScheduleCron    *string        `json:"scheduleCron,omitempty"`
	ScheduleEnabled *bool          `json:"scheduleEnabled,omitempty"`
	RealtimeEnabled *bool          `json:"realtimeEnabled,omitempty"`
	MaxWorkers      *int           `json:"maxWorkers,omitempty"`
	MaxMemoryMB     *int           `json:"maxMemoryMB,omitempty"`
	HashCache       *bool          `json:"hashCache,omitempty"`
	DryRun          *bool          `json:"dryRun,omitempty"`
	VerifyAfter     *bool          `json:"verifyAfter,omitempty"`
	MaxRefPerFile   *int           `json:"maxRefPerFile,omitempty"`
}
