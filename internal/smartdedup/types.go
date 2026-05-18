// Package smartdedup 提供内容感知的智能文件去重功能
// 参考 TrueNAS 的 dedup 特性，支持 Btrfs 和 ZFS 存储后端
package smartdedup

import (
	"sync"
	"time"
)

// StorageBackend 存储后端类型.
type StorageBackend string

const (
	BackendBtrfs StorageBackend = "btrfs"
	BackendZFS   StorageBackend = "zfs"
	BackendAuto  StorageBackend = "auto" // 自动检测
)

// DedupMode 去重模式.
type DedupMode string

const (
	ModeScheduled DedupMode = "scheduled" // 定时扫描
	ModeRealtime  DedupMode = "realtime"  // 实时去重
	ModeHybrid    DedupMode = "hybrid"    // 混合模式
)

// DedupAction 去重动作.
type DedupAction string

const (
	ActionHardlink DedupAction = "hardlink" // 硬链接
	ActionReflink  DedupAction = "reflink"  // CoW 引用（Btrfs/ZFS）
	ActionReflinkPlus DedupAction = "reflink_plus" // CoW 引用 + 元数据标记
	ActionReport   DedupAction = "report"   // 仅报告
)

// EntryStatus 去重条目状态.
type EntryStatus string

const (
	StatusPending    EntryStatus = "pending"
	StatusDeduped    EntryStatus = "deduped"
	StatusSkipped    EntryStatus = "skipped"
	StatusFailed     EntryStatus = "failed"
	StatusReferenced EntryStatus = "referenced" // 被其他文件引用
)

// DedupEntry 去重条目.
type DedupEntry struct {
	ID          string      `json:"id"`
	ContentHash string      `json:"contentHash"` // 内容哈希（SHA-256）
	FilePath    string      `json:"filePath"`
	FileSize    int64       `json:"fileSize"`
	RefCount    int         `json:"refCount"`
	Status      EntryStatus `json:"status"`
	Backend     StorageBackend `json:"backend"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`

	// 引用关系
	References []string `json:"references,omitempty"` // 引用此内容的文件路径列表
	SourceFile string   `json:"sourceFile,omitempty"` // 源文件路径（去重后的主文件）
}

// DuplicateGroup 重复文件组.
type DuplicateGroup struct {
	ContentHash string   `json:"contentHash"`
	TotalSize   int64    `json:"totalSize"`   // 组内总大小
	UniqueSize  int64    `json:"uniqueSize"`  // 去重后实际大小（一份）
	SavedSize   int64    `json:"savedSize"`   // 节省的空间
	FileCount   int      `json:"fileCount"`
	Files       []string `json:"files"`
	Status      string   `json:"status"` // pending, processed
}

// DedupStats 去重统计.
type DedupStats struct {
	mu sync.RWMutex `json:"-"`

	// 扫描统计
	TotalFilesScanned int64 `json:"totalFilesScanned"`
	TotalSizeScanned  int64 `json:"totalSizeScanned"`

	// 去重统计
	DedupedFiles int64 `json:"dedupedFiles"`
	DedupedSize  int64 `json:"dedupedSize"`

	// 空间节省
	SavedSpace   int64   `json:"savedSpace"`
	DedupRatio   float64 `json:"dedupRatio"`   // 去重比率 (节省空间 / 总空间)
	UniqueBlocks int64   `json:"uniqueBlocks"` // 唯一块数量

	// 引用统计
	TotalRefs    int64 `json:"totalRefs"`
	ActiveRefs   int64 `json:"activeRefs"`

	// 时间统计
	LastScanTime  time.Time     `json:"lastScanTime"`
	LastDedupTime time.Time     `json:"lastDedupTime"`
	TotalScanTime time.Duration `json:"totalScanTime"`
	TotalDedupTime time.Duration `json:"totalDedupTime"`

	// 状态
	IsScanning  bool `json:"isScanning"`
	IsDeduping  bool `json:"isDeduping"`
}

// GetSnapshot 获取统计快照（线程安全）.
func (s *DedupStats) GetSnapshot() *DedupStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &DedupStats{
		TotalFilesScanned: s.TotalFilesScanned,
		TotalSizeScanned:  s.TotalSizeScanned,
		DedupedFiles:      s.DedupedFiles,
		DedupedSize:       s.DedupedSize,
		SavedSpace:        s.SavedSpace,
		DedupRatio:        s.DedupRatio,
		UniqueBlocks:      s.UniqueBlocks,
		TotalRefs:         s.TotalRefs,
		ActiveRefs:        s.ActiveRefs,
		LastScanTime:      s.LastScanTime,
		LastDedupTime:     s.LastDedupTime,
		TotalScanTime:     s.TotalScanTime,
		TotalDedupTime:    s.TotalDedupTime,
		IsScanning:        s.IsScanning,
		IsDeduping:        s.IsDeduping,
	}
}

// UpdateRatio 更新去重比率（调用者需持有锁）.
func (s *DedupStats) updateRatioLocked() {
	if s.TotalSizeScanned > 0 {
		s.DedupRatio = float64(s.SavedSpace) / float64(s.TotalSizeScanned)
	}
}

// UpdateRatio 更新去重比率（线程安全）.
func (s *DedupStats) UpdateRatio() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateRatioLocked()
}

// ScanResult 扫描结果.
type ScanResult struct {
	ScanID        string           `json:"scanId"`
	StartTime     time.Time        `json:"startTime"`
	EndTime       time.Time        `json:"endTime"`
	Duration      time.Duration    `json:"duration"`
	FilesScanned  int              `json:"filesScanned"`
	TotalSize     int64            `json:"totalSize"`
	DuplicateGroups []DuplicateGroup `json:"duplicateGroups"`
	TotalDuplicates int            `json:"totalDuplicates"`
	PotentialSaving int64          `json:"potentialSaving"`
	Errors        []ScanError      `json:"errors,omitempty"`
}

// ScanError 扫描错误.
type ScanError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// RefCountEntry 引用计数条目（用于安全的引用计数管理）.
type RefCountEntry struct {
	mu        sync.Mutex `json:"-"`
	ContentHash string   `json:"contentHash"`
	RefCount    int      `json:"refCount"`
	Files       []string `json:"files"`       // 引用此内容的所有文件
	SourceFile  string   `json:"sourceFile"`  // 源文件（第一个被扫描到的）
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// IncrRef 增加引用计数（线程安全）.
func (r *RefCountEntry) IncrRef(filePath string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.RefCount++
	r.Files = append(r.Files, filePath)
	r.UpdatedAt = time.Now()
	return r.RefCount
}

// DecrRef 减少引用计数（线程安全）.
// 返回减少后的引用计数和是否还有引用.
func (r *RefCountEntry) DecrRef(filePath string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.RefCount <= 0 {
		return 0, false
	}
	r.RefCount--
	// 从文件列表中移除
	for i, f := range r.Files {
		if f == filePath {
			r.Files = append(r.Files[:i], r.Files[i+1:]...)
			break
		}
	}
	r.UpdatedAt = time.Now()
	return r.RefCount, r.RefCount > 0
}
