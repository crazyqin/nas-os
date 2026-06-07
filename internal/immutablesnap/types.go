// Package immutablesnap 提供企业级不可变快照管理功能。
// 参考 TrueNAS ZFS 不可变快照和群晖 Snapshot Replication 实现。
// 支持 WORM（Write Once Read Many）、GFS 保留策略、快照复制、勒索软件防护等特性。
package immutablesnap

import "time"

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	StatusPending   SnapshotStatus = "pending"     // 待锁定
	StatusLocked    SnapshotStatus = "locked"      // 已锁定（不可变）
	StatusExpired   SnapshotStatus = "expired"     // 已过期
	StatusReplicate SnapshotStatus = "replicating" // 复制中
	StatusVerified  SnapshotStatus = "verified"    // 已验证
)

// ScheduleType 快照计划类型
type ScheduleType string

const (
	ScheduleHourly  ScheduleType = "hourly"  // 每小时
	ScheduleDaily   ScheduleType = "daily"   // 每天
	ScheduleWeekly  ScheduleType = "weekly"  // 每周
	ScheduleMonthly ScheduleType = "monthly" // 每月
)

// RetentionType 保留策略类型
type RetentionType string

const (
	RetentionSimple RetentionType = "simple" // 简单保留
	RetentionGFS    RetentionType = "gfs"    // GFS（祖父-父-子）
)

// ReplicationStatus 复制任务状态
type ReplicationStatus string

const (
	RepStatusPending   ReplicationStatus = "pending"
	RepStatusRunning   ReplicationStatus = "running"
	RepStatusCompleted ReplicationStatus = "completed"
	RepStatusFailed    ReplicationStatus = "failed"
)

// ThreatLevel 威胁等级
type ThreatLevel string

const (
	ThreatLevelNormal     ThreatLevel = "normal"
	ThreatLevelSuspicious ThreatLevel = "suspicious"
	ThreatLevelCritical   ThreatLevel = "critical"
)

// Snapshot 不可变快照
type Snapshot struct {
	ID            string         `json:"id"`
	DatasetName   string         `json:"dataset_name"`
	Status        SnapshotStatus `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	ExpiresAt     time.Time      `json:"expires_at"`
	Locked        bool           `json:"locked"`
	LockedAt      *time.Time     `json:"locked_at,omitempty"`
	WORM          bool           `json:"worm"` // Write Once Read Many 标记
	Size          int64          `json:"size_bytes"`
	Checksum      string         `json:"checksum"`
	Tags          []string       `json:"tags,omitempty"`
	SourcePath    string         `json:"source_path,omitempty"`
	StoragePath   string         `json:"storage_path,omitempty"`
	ParentID      string         `json:"parent_id,omitempty"`     // 父快照 ID（用于增量复制）
	ReplicatedTo  []string       `json:"replicated_to,omitempty"` // 已复制到的远端列表
	RollbackCount int            `json:"rollback_count"`          // 回滚次数
}

// SnapshotPolicy 自动快照策略
type SnapshotPolicy struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Enabled       bool          `json:"enabled"`
	DatasetName   string        `json:"dataset_name"`
	Schedule      ScheduleType  `json:"schedule"`
	RetentionType RetentionType `json:"retention_type"`
	// 简单保留配置
	RetentionHours int `json:"retention_hours,omitempty"`
	MaxSnapshots   int `json:"max_snapshots,omitempty"`
	// GFS 保留配置
	GFSGrandfather int `json:"gfs_grandfather,omitempty"` // 保留的月份数
	GFSFather      int `json:"gfs_father,omitempty"`      // 保留的周数
	GFSSon         int `json:"gfs_son,omitempty"`         // 保留的天数
	// 其他配置
	AutoLock  bool       `json:"auto_lock"`
	Tags      []string   `json:"tags,omitempty"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// RetentionRule 保留规则
type RetentionRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	DatasetName string        `json:"dataset_name"`
	Type        RetentionType `json:"type"`
	// 简单保留
	MaxAge   time.Duration `json:"max_age,omitempty"`
	MaxCount int           `json:"max_count,omitempty"`
	// GFS 保留
	GFSGrandfather int       `json:"gfs_grandfather,omitempty"` // 月
	GFSFather      int       `json:"gfs_father,omitempty"`      // 周
	GFSSon         int       `json:"gfs_son,omitempty"`         // 天
	CreatedAt      time.Time `json:"created_at"`
}

// DefaultGFSTemplate 默认 GFS 模板（参考业界最佳实践）
func DefaultGFSTemplate() RetentionRule {
	return RetentionRule{
		Type:           RetentionGFS,
		GFSGrandfather: 12, // 12 个月
		GFSFather:      8,  // 8 周
		GFSSon:         14, // 14 天
		CreatedAt:      time.Now(),
	}
}

// ReplicationJob 快照复制任务
type ReplicationJob struct {
	ID             string            `json:"id"`
	SnapshotID     string            `json:"snapshot_id"`
	RemoteEndpoint string            `json:"remote_endpoint"` // 远端地址
	RemotePath     string            `json:"remote_path"`     // 远端路径
	Status         ReplicationStatus `json:"status"`
	Progress       float64           `json:"progress"` // 0.0-1.0
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	Error          string            `json:"error,omitempty"`
	RetryCount     int               `json:"retry_count"`
	MaxRetries     int               `json:"max_retries"`
	CreatedAt      time.Time         `json:"created_at"`
}

// ThreatEvent 威胁事件
type ThreatEvent struct {
	ID           string      `json:"id"`
	Timestamp    time.Time   `json:"timestamp"`
	Level        ThreatLevel `json:"level"`
	ModifiedRate float64     `json:"modified_rate"`
	Description  string      `json:"description"`
	SnapshotID   string      `json:"snapshot_id,omitempty"`
	DatasetName  string      `json:"dataset_name,omitempty"`
	AlertSent    bool        `json:"alert_sent"`
}

// SpaceUsage 空间使用统计
type SpaceUsage struct {
	DatasetName     string  `json:"dataset_name"`
	TotalSnapshots  int     `json:"total_snapshots"`
	TotalSizeBytes  int64   `json:"total_size_bytes"`
	LockedSize      int64   `json:"locked_size_bytes"`
	UnlockedSize    int64   `json:"unlocked_size_bytes"`
	AvgSnapshotSize int64   `json:"avg_snapshot_size_bytes"`
	OldestSnapshot  *string `json:"oldest_snapshot,omitempty"`
	NewestSnapshot  *string `json:"newest_snapshot,omitempty"`
}

// Stats 全局统计
type Stats struct {
	TotalSnapshots   int          `json:"total_snapshots"`
	LockedSnapshots  int          `json:"locked_snapshots"`
	ExpiredSnapshots int          `json:"expired_snapshots"`
	ReplicatingCount int          `json:"replicating_count"`
	TotalSizeBytes   int64        `json:"total_size_bytes"`
	ThreatEvents     int          `json:"threat_events"`
	ActivePolicies   int          `json:"active_policies"`
	ReplicationJobs  int          `json:"replication_jobs"`
	SpaceByDataset   []SpaceUsage `json:"space_by_dataset,omitempty"`
}

// RollbackResult 回滚结果
type RollbackResult struct {
	SnapshotID   string    `json:"snapshot_id"`
	DatasetName  string    `json:"dataset_name"`
	RolledBack   bool      `json:"rolled_back"`
	RolledBackAt time.Time `json:"rolled_back_at"`
	Details      string    `json:"details,omitempty"`
}

// IntegrityResult 完整性验证结果
type IntegrityResult struct {
	SnapshotID   string    `json:"snapshot_id"`
	Valid        bool      `json:"valid"`
	ExpectedHash string    `json:"expected_hash"`
	ActualHash   string    `json:"actual_hash"`
	VerifiedAt   time.Time `json:"verified_at"`
	Details      string    `json:"details,omitempty"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled               bool     `json:"enabled"`
	ModifiedRateThreshold float64  `json:"modified_rate_threshold"` // 触发告警的修改率阈值
	AutoSnapshotOnAlert   bool     `json:"auto_snapshot_on_alert"`
	NotifyChannels        []string `json:"notify_channels,omitempty"` // 通知渠道
}
