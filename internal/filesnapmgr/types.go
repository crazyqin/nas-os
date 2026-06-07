// Package filesnapmgr 提供文件系统快照管理功能，对标 ZFS/Btrfs 快照管理。
// 支持快照创建/删除/回滚/列表、定时快照策略、快照保留策略、快照克隆和挂载。
package filesnapmgr

import "time"

// SnapshotType 快照类型
type SnapshotType string

const (
	SnapshotTypeZFS    SnapshotType = "zfs"    // ZFS 快照
	SnapshotTypeBtrfs  SnapshotType = "btrfs"  // Btrfs 快照
	SnapshotTypeLVM    SnapshotType = "lvm"    // LVM 快照
	SnapshotTypeCustom SnapshotType = "custom" // 自定义快照
)

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	SnapshotStatusActive   SnapshotStatus = "active"   // 活跃
	SnapshotStatusMounting SnapshotStatus = "mounting" // 挂载中
	SnapshotStatusMounted  SnapshotStatus = "mounted"  // 已挂载
	SnapshotStatusRolling  SnapshotStatus = "rolling"  // 回滚中
	SnapshotStatusDeleting SnapshotStatus = "deleting" // 删除中
	SnapshotStatusDeleted  SnapshotStatus = "deleted"  // 已删除
	SnapshotStatusCloning  SnapshotStatus = "cloning"  // 克隆中
	SnapshotStatusCloned   SnapshotStatus = "cloned"   // 已克隆
)

// FilesystemSnapshot 文件系统快照
type FilesystemSnapshot struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Volume          string            `json:"volume"` // 源卷/文件系统
	Path            string            `json:"path"`   // 快照路径
	Type            SnapshotType      `json:"type"`   // 快照类型
	Status          SnapshotStatus    `json:"status"`
	Description     string            `json:"description,omitempty"`
	SizeBytes       int64             `json:"size_bytes"`       // 快照占用空间
	ReferencedBytes int64             `json:"referenced_bytes"` // 引用数据量
	CreatedAt       time.Time         `json:"created_at"`
	ParentID        string            `json:"parent_id,omitempty"`    // 父快照 ID
	ChildrenIDs     []string          `json:"children_ids,omitempty"` // 子快照 ID 列表
	Tags            []string          `json:"tags,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// SnapshotPolicy 快照策略
type SnapshotPolicy struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Volume       string       `json:"volume"` // 目标卷
	Type         SnapshotType `json:"type"`   // 快照类型
	Enabled      bool         `json:"enabled"`
	Schedule     string       `json:"schedule"`                // cron 表达式
	Retention    Retention    `json:"retention"`               // 保留策略
	PreSnapshot  string       `json:"pre_snapshot,omitempty"`  // 快照前脚本
	PostSnapshot string       `json:"post_snapshot,omitempty"` // 快照后脚本
	Tags         []string     `json:"tags,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	LastRunAt    *time.Time   `json:"last_run_at,omitempty"`
	NextRunAt    *time.Time   `json:"next_run_at,omitempty"`
	RunCount     int64        `json:"run_count"`
	ErrorCount   int64        `json:"error_count"`
}

// Retention 保留策略
type Retention struct {
	MaxCount   int `json:"max_count"`             // 最大快照数
	MaxAgeDays int `json:"max_age_days"`          // 最大保留天数
	MinKeep    int `json:"min_keep"`              // 最少保留数
	MaxSizeGB  int `json:"max_size_gb,omitempty"` // 最大总大小 (GB)
}

// DefaultRetention 默认保留策略
func DefaultRetention() Retention {
	return Retention{
		MaxCount:   10,
		MaxAgeDays: 30,
		MinKeep:    2,
	}
}

// CloneRequest 克隆请求
type CloneRequest struct {
	SnapshotID string `json:"snapshot_id" binding:"required"`
	CloneName  string `json:"clone_name" binding:"required"`
	MountPoint string `json:"mount_point,omitempty"` // 挂载点
	ReadOnly   bool   `json:"read_only"`             // 是否只读
}

// CloneResult 克隆结果
type CloneResult struct {
	CloneID    string    `json:"clone_id"`
	CloneName  string    `json:"clone_name"`
	SnapshotID string    `json:"snapshot_id"`
	MountPoint string    `json:"mount_point,omitempty"`
	SizeBytes  int64     `json:"size_bytes"`
	CreatedAt  time.Time `json:"created_at"`
}

// MountRequest 挂载请求
type MountRequest struct {
	SnapshotID string `json:"snapshot_id" binding:"required"`
	MountPoint string `json:"mount_point" binding:"required"`
	ReadOnly   bool   `json:"read_only"` // 是否只读挂载
}

// RollbackRequest 回滚请求
type RollbackRequest struct {
	SnapshotID     string `json:"snapshot_id" binding:"required"`
	Force          bool   `json:"force"`           // 强制回滚（丢弃后续变更）
	CreateSnapshot bool   `json:"create_snapshot"` // 回滚前创建快照
}

// RollbackResult 回滚结果
type RollbackResult struct {
	SnapshotID    string    `json:"snapshot_id"`
	BackupID      string    `json:"backup_id,omitempty"` // 回滚前创建的备份快照 ID
	RolledBackAt  time.Time `json:"rolled_back_at"`
	FilesRestored int       `json:"files_restored"`
	SizeRestored  int64     `json:"size_restored"`
}

// SnapshotStats 快照统计
type SnapshotStats struct {
	TotalSnapshots  int            `json:"total_snapshots"`
	ActiveSnapshots int            `json:"active_snapshots"`
	TotalSizeBytes  int64          `json:"total_size_bytes"`
	ByType          map[string]int `json:"by_type"`
	ByVolume        map[string]int `json:"by_volume"`
	OldestSnapshot  *time.Time     `json:"oldest_snapshot,omitempty"`
	NewestSnapshot  *time.Time     `json:"newest_snapshot,omitempty"`
	PolicyCount     int            `json:"policy_count"`
	ActivePolicies  int            `json:"active_policies"`
}

// DiffResult 快照差异
type DiffResult struct {
	Snapshot1ID  string       `json:"snapshot1_id"`
	Snapshot2ID  string       `json:"snapshot2_id"`
	Added        []FileChange `json:"added"`
	Modified     []FileChange `json:"modified"`
	Deleted      []FileChange `json:"deleted"`
	TotalChanges int          `json:"total_changes"`
	SizeDelta    int64        `json:"size_delta"`
}

// FileChange 文件变更
type FileChange struct {
	Path       string     `json:"path"`
	OldSize    int64      `json:"old_size,omitempty"`
	NewSize    int64      `json:"new_size,omitempty"`
	OldModTime *time.Time `json:"old_mod_time,omitempty"`
	NewModTime *time.Time `json:"new_mod_time,omitempty"`
}

// SnapshotConfig 快照管理配置
type SnapshotConfig struct {
	DefaultType       SnapshotType `json:"default_type"`
	MaxConcurrent     int          `json:"max_concurrent"`     // 最大并发操作数
	TempDir           string       `json:"temp_dir"`           // 临时目录
	MountBaseDir      string       `json:"mount_base_dir"`     // 挂载基目录
	CloneBaseDir      string       `json:"clone_base_dir"`     // 克隆基目录
	EnableScheduler   bool         `json:"enable_scheduler"`   // 启用调度器
	SchedulerInterval int          `json:"scheduler_interval"` // 调度间隔（秒）
}

// DefaultSnapshotConfig 默认配置
func DefaultSnapshotConfig() *SnapshotConfig {
	return &SnapshotConfig{
		DefaultType:       SnapshotTypeZFS,
		MaxConcurrent:     3,
		TempDir:           "/tmp/nas-snapshots",
		MountBaseDir:      "/mnt/snapshots",
		CloneBaseDir:      "/mnt/clones",
		EnableScheduler:   true,
		SchedulerInterval: 60,
	}
}
