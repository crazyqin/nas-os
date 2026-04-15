// Package sync provides the core synchronization engine for Cloud Drive Sync.
// It implements bidirectional rsync-style delta-sync, file versioning,
// conflict detection, incremental change tracking, and event notifications.
package sync

import (
	"time"
)

// SyncDirection 同步方向.
type SyncDirection string

const (
	DirectionUpload      SyncDirection = "upload"      // 本地 → 云端
	DirectionDownload    SyncDirection = "download"    // 云端 → 本地
	DirectionBidirectional SyncDirection = "bidirectional" // 双向同步
)

// ConflictStrategy 冲突解决策略.
type ConflictStrategy string

const (
	ConflictSkip    ConflictStrategy = "skip"    // 跳过
	ConflictLocal   ConflictStrategy = "local"   // 本地优先
	ConflictRemote  ConflictStrategy = "remote"  // 远程优先
	ConflictNewer   ConflictStrategy = "newer"   // 较新优先
	ConflictRename  ConflictStrategy = "rename"  // 重命名保留双方
	ConflictAsk     ConflictStrategy = "ask"     // 询问用户
)

// ChangeType 文件变化类型.
type ChangeType int

const (
	ChangeCreate ChangeType = iota
	ChangeModify
	ChangeDelete
	ChangeRename
)

// Task 同步任务配置.
type Task struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	LocalPath   string        `json:"localPath"`
	RemotePath  string        `json:"remotePath"`
	Direction   SyncDirection `json:"direction"`
	VersionKeep int           `json:"versionKeep"` // 保留版本数量，0=禁用
	Concurrency int           `json:"concurrency"` // 并发数

	// 冲突处理
	ConflictStrategy ConflictStrategy `json:"conflictStrategy"`

	// 过滤规则（glob 模式）
	IncludePatterns []string `json:"includePatterns,omitempty"`
	ExcludePatterns []string `json:"excludePatterns,omitempty"`
	MaxFileSize      int64    `json:"maxFileSize,omitempty"` // 字节，0=不限制

	// 行为选项
	DeleteOrphan   bool `json:"deleteOrphan"`   // 删除孤立文件（本地删则远程删，远程删则本地删）
	PreserveModTime bool `json:"preserveModTime"` // 保留修改时间
	ChecksumVerify   bool `json:"checksumVerify"`  // 用校验和比较而非仅 mtime+size

	// 增量跟踪
	WatchMode string `json:"watchMode"` // "inotify" | "scan" | "hybrid"

	Enabled bool      `json:"enabled"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// FileEntry 单个文件的同步记录.
type FileEntry struct {
	Path          string    `json:"path"`          // 相对路径
	Size          int64     `json:"size"`
	ModTime       time.Time `json:"modTime"`
	Checksum      string    `json:"checksum,omitempty"` // xxhash64
	IsDir         bool      `json:"isDir"`
	LastSyncedAt  time.Time `json:"lastSyncedAt"`
	SyncedRev     int64     `json:"syncedRev"` // 同步版本号
}

// Snapshot 目录快照，描述某一时刻的完整状态.
type Snapshot struct {
	Rev      int64                 `json:"rev"`
	RootPath string                `json:"rootPath"`
	Entries  map[string]*FileEntry `json:"entries"` // key = 相对路径
	Mtime    time.Time             `json:"mtime"`   // 快照生成时间
}

// Delta 两个 Snapshot 之间的差异.
type Delta struct {
	Adds    []*DeltaItem `json:"adds"`    // 新增文件
	Mods    []*DeltaItem `json:"mods"`    // 修改文件
	Dels    []*DeltaItem `json:"dels"`    // 删除文件
	Renames []*RenameItem `json:"renames"` // 重命名（检测到）
}

// DeltaItem 一次变化.
type DeltaItem struct {
	RelPath   string    `json:"relPath"`   // 相对路径
	OldEntry  *FileEntry `json:"oldEntry,omitempty"`  // 变化前的记录
	NewEntry  *FileEntry `json:"newEntry,omitempty"`  // 变化后的记录
	ChangeType ChangeType `json:"changeType"`
}

// RenameItem 检测到的重命名操作.
type RenameItem struct {
	SrcPath  string    `json:"srcPath"`  // 原路径
	DstPath  string    `json:"dstPath"`  // 新路径
	SrcEntry *FileEntry `json:"srcEntry"` // 源文件快照
	DstEntry *FileEntry `json:"dstEntry"` // 目标文件快照
	Score    float64   `json:"score"`   // 重命名置信度 (0-1)
}

// Conflict 两端同时修改产生的冲突.
type Conflict struct {
	ID             string         `json:"id"`
	TaskID         string         `json:"taskId"`
	RelPath        string         `json:"relPath"`
	LocalModTime   time.Time      `json:"localModTime"`
	LocalSize      int64          `json:"localSize"`
	LocalChecksum  string         `json:"localChecksum"`
	RemoteModTime  time.Time      `json:"remoteModTime"`
	RemoteSize     int64          `json:"remoteSize"`
	RemoteChecksum string         `json:"remoteChecksum"`
	Strategy       ConflictStrategy `json:"strategy"`
	DetectedAt     time.Time      `json:"detectedAt"`
	ResolvedAt     *time.Time     `json:"resolvedAt,omitempty"`
	ResolvedPath   string         `json:"resolvedPath,omitempty"` // rename 时的保留路径
}

// VersionEntry 版本记录.
type VersionEntry struct {
	Rev      int64     `json:"rev"`      // 快照版本号
	RelPath  string    `json:"relPath"`  // 文件相对路径（当前名）
	AbsPath  string    `json:"absPath"`  // 存储的实际路径
	Size     int64     `json:"size"`
	Checksum string    `json:"checksum"`
	ModTime  time.Time `json:"modTime"` // 文件原始修改时间
	Created  time.Time `json:"created"`  // 版本创建时间
	Marked   bool      `json:"marked"`   // 是否已标记删除
}

// ChunkInfo 分块信息，用于 delta-sync.
type ChunkInfo struct {
	Index    int64  `json:"index"`
	Hash     string `json:"hash"`      // 本块的 xxhash64
	Offset   int64  `json:"offset"`    // 在文件中的字节偏移
	Length   int64  `json:"length"`    // 块大小
	Checksum string `json:"checksum"`  // 整文件 checksum（仅末块）
}

// ChecksumResult 增量同步时的校验和比较结果.
type ChecksumResult struct {
	RelPath  string   `json:"relPath"`
	LocalCS  string   `json:"localCs"`
	RemoteCS string   `json:"remoteCs"`
	Equal    bool     `json:"equal"`
	Chunks   []ChunkInfo `json:"chunks,omitempty"` // 不同时，差异块列表
}

// Progress 同步进度.
type Progress struct {
	TaskID          string    `json:"taskId"`
	Direction       SyncDirection `json:"direction"`
	State           string    `json:"state"` // "idle","running","paused","completed","failed"
	StartTime       time.Time `json:"startTime"`
	EndTime         *time.Time `json:"endTime,omitempty"`
	TotalFiles      int64     `json:"totalFiles"`
	ProcessedFiles  int64     `json:"processedFiles"`
	TotalBytes      int64     `json:"totalBytes"`
	TransferredBytes int64   `json:"transferredBytes"`
	SpeedBps        int64     `json:"speedBps"`   // 字节/秒
	ProgressPct     float64   `json:"progressPct"` // 0-100
	CurrentFile     string    `json:"currentFile,omitempty"`
	CurrentOp       string    `json:"currentOp,omitempty"` // "upload","download","hash","resolve"
	ErrorCount      int       `json:"errorCount"`
	Conflicts       int       `json:"conflicts"`
	SkippedFiles    int64     `json:"skippedFiles"`
	UploadedFiles   int64     `json:"uploadedFiles"`
	DownloadedFiles int64     `json:"downloadedFiles"`
	DeletedFiles    int64     `json:"deletedFiles"`
}
