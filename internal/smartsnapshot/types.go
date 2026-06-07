// Package smartsnapshot 智能快照管理，对标 ZFS 快照 + 群晖 Active Backup
// 支持定时快照、增量快照、快照克隆、自动清理、一键恢复
package smartsnapshot

import "time"

// SnapshotStatus 快照状态
type SnapshotStatus string

const (
	StatusCreating SnapshotStatus = "creating"
	StatusReady    SnapshotStatus = "ready"
	StatusRolling  SnapshotStatus = "rolling_back"
	StatusDeleting SnapshotStatus = "deleting"
	StatusError    SnapshotStatus = "error"
)

// SnapshotType 快照类型
type SnapshotType string

const (
	TypeFull         SnapshotType = "full"
	TypeIncremental  SnapshotType = "incremental"
	TypeDifferential SnapshotType = "differential"
)

// SnapshotPolicyType 策略类型
type SnapshotPolicyType string

const (
	PolicyInterval SnapshotPolicyType = "interval"
	PolicyCron     SnapshotPolicyType = "cron"
	PolicyEvent    SnapshotPolicyType = "event"
)

// Snapshot 快照
type Snapshot struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	PoolID        string         `json:"poolId"`
	DatasetPath   string         `json:"datasetPath"`
	Type          SnapshotType   `json:"type"`
	Status        SnapshotStatus `json:"status"`
	SizeBytes     int64          `json:"sizeBytes"`
	ParentID      string         `json:"parentId,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	ExpiresAt     *time.Time     `json:"expiresAt,omitempty"`
	Description   string         `json:"description,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	FileCount     int64          `json:"fileCount"`
	IsProtected   bool           `json:"isProtected"`
	RetentionDays int            `json:"retentionDays"`
}

// SnapshotPolicy 快照策略
type SnapshotPolicy struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Enabled      bool               `json:"enabled"`
	Type         SnapshotPolicyType `json:"type"`
	Interval     time.Duration      `json:"interval,omitempty"`
	CronExpr     string             `json:"cronExpr,omitempty"`
	EventType    string             `json:"eventType,omitempty"`
	DatasetPaths []string           `json:"datasetPaths"`
	Retention    RetentionPolicy    `json:"retention"`
	PreScript    string             `json:"preScript,omitempty"`
	PostScript   string             `json:"postScript,omitempty"`
	LastRun      time.Time          `json:"lastRun"`
	NextRun      time.Time          `json:"nextRun"`
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	MaxSnapshots int `json:"maxSnapshots"`
	MaxAgeDays   int `json:"maxAgeDays"`
	KeepDaily    int `json:"keepDaily"`
	KeepWeekly   int `json:"keepWeekly"`
	KeepMonthly  int `json:"keepMonthly"`
	KeepYearly   int `json:"keepYearly"`
}

// CloneInfo 克隆信息
type CloneInfo struct {
	ID        string    `json:"id"`
	SourceID  string    `json:"sourceId"`
	ClonePath string    `json:"clonePath"`
	CreatedAt time.Time `json:"createdAt"`
	IsActive  bool      `json:"isActive"`
	SizeBytes int64     `json:"sizeBytes"`
}

// SnapshotStats 快照统计
type SnapshotStats struct {
	TotalSnapshots   int       `json:"totalSnapshots"`
	TotalSizeBytes   int64     `json:"totalSizeBytes"`
	ProtectedCount   int       `json:"protectedCount"`
	PendingDeletion  int       `json:"pendingDeletion"`
	PolicyCount      int       `json:"policyCount"`
	LastSnapshotTime time.Time `json:"lastSnapshotTime"`
	OldestSnapshot   time.Time `json:"oldestSnapshot"`
}

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	Name        string       `json:"name" binding:"required"`
	DatasetPath string       `json:"datasetPath" binding:"required"`
	Type        SnapshotType `json:"type"`
	Description string       `json:"description,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	ExpireDays  int          `json:"expireDays,omitempty"`
	Protected   bool         `json:"protected"`
}

// RollbackRequest 回滚请求
type RollbackRequest struct {
	SnapshotID string `json:"snapshotId" binding:"required"`
	Force      bool   `json:"force"`
}

// Response API 响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
