// Package immusnap 提供不可变快照管理功能，对标群晖 Immutable Snapshot 防勒索方案。
// 快照一旦创建并锁定，在过期时间之前不可删除、不可修改。
// 支持勒索软件检测联动：当检测到异常文件修改率时自动创建不可变快照。
package immusnap

import "time"

// ThreatLevel 威胁等级
type ThreatLevel string

const (
	ThreatLevelNormal     ThreatLevel = "normal"     // 正常
	ThreatLevelSuspicious ThreatLevel = "suspicious" // 可疑
	ThreatLevelCritical   ThreatLevel = "critical"   // 危急
)

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	StatusPending  SnapshotStatus = "pending"  // 待锁定
	StatusLocked   SnapshotStatus = "locked"   // 已锁定（不可变）
	StatusExpired  SnapshotStatus = "expired"  // 已过期
	StatusVerified SnapshotStatus = "verified" // 已验证
)

// ImmutableSnapshot 不可变快照
type ImmutableSnapshot struct {
	ID          string         `json:"id"`
	DatasetName string         `json:"dataset_name"` // 数据集/卷名
	Status      SnapshotStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
	Locked      bool           `json:"locked"`            // 是否已锁定
	Size        int64          `json:"size_bytes"`         // 快照大小
	Checksum    string         `json:"checksum"`           // 完整性校验和
	Tags        []string       `json:"tags,omitempty"`
	SourcePath  string         `json:"source_path,omitempty"`  // 源数据路径
	StoragePath string         `json:"storage_path,omitempty"` // 快照存储路径
	ThreatLevel ThreatLevel    `json:"threat_level"`            // 触发时的威胁等级
}

// RetentionPolicy 快照保留策略
type RetentionPolicy struct {
	MinRetentionHours int  `json:"min_retention_hours"` // 最小保留时长（小时）
	MaxSnapshots      int  `json:"max_snapshots"`       // 最大快照数量
	AutoLockOnThreat  bool `json:"auto_lock_on_threat"` // 威胁时自动锁定
}

// DefaultRetentionPolicy 返回默认保留策略
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		MinRetentionHours: 24,
		MaxSnapshots:      100,
		AutoLockOnThreat:  true,
	}
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

// ThreatEvent 威胁事件
type ThreatEvent struct {
	Timestamp    time.Time   `json:"timestamp"`
	Level        ThreatLevel `json:"level"`
	ModifiedRate float64     `json:"modified_rate"` // 文件修改率 (0.0~1.0)
	Description  string      `json:"description"`
	SnapshotID   string      `json:"snapshot_id,omitempty"` // 自动创建的快照 ID
}

// Stats 不可变快照统计
type Stats struct {
	TotalSnapshots   int     `json:"total_snapshots"`
	LockedSnapshots  int     `json:"locked_snapshots"`
	ExpiredSnapshots int     `json:"expired_snapshots"`
	TotalSizeBytes   int64   `json:"total_size_bytes"`
	OldestSnapshot   *string `json:"oldest_snapshot,omitempty"`
	NewestSnapshot   *string `json:"newest_snapshot,omitempty"`
	ThreatEvents     int     `json:"threat_events"`
}
