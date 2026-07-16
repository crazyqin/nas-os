// Package snapshotmgr 提供系统配置快照与回滚功能。
// 参考 TrueNAS 的 Boot Environment 和群晖的配置备份，但更轻量易用。
// 支持在关键操作（系统更新、配置变更）前自动创建快照，出问题时一键回滚。
package snapshotmgr

import "time"

// Snapshot 系统配置快照.
type Snapshot struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Source 触发来源：manual / auto-update / auto-config / schedule
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	// Status: active / restoring / deleted
	Status string `json:"status"`
	// Items 包含的配置项
	Items []SnapshotItem `json:"items"`
}

// SnapshotItem 快照中的单项配置.
type SnapshotItem struct {
	Category string `json:"category"` // network / shares / users / services / system
	Key      string `json:"key"`
	FilePath string `json:"file_path"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"` // sha256
}

// SnapshotConfig 快照配置.
type SnapshotConfig struct {
	MaxSnapshots     int  `json:"max_snapshots"`      // 最多保留快照数，默认 20
	AutoBeforeUpdate bool `json:"auto_before_update"` // 系统更新前自动创建
	AutoBeforeConfig bool `json:"auto_before_config"` // 配置变更前自动创建
	RetentionDays    int  `json:"retention_days"`     // 保留天数，默认 90
}
