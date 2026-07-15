// Package snapviz types
package snapviz

import "time"

// SnapshotTimelineResponse API 响应
type SnapshotTimelineResponse struct {
	Events []*TimelineEvent `json:"events"`
	Stats  *TimelineStats   `json:"stats"`
}

// SnapshotTimelineRequest API 请求
type SnapshotTimelineRequest struct {
	Filter     *TimelineFilter `json:"filter,omitempty"`
	BucketSize string          `json:"bucket_size,omitempty"`
}

// RestorePoint 恢复点
type RestorePoint struct {
	SnapshotID string    `json:"snapshot_id"`
	Volume     string    `json:"volume"`
	Dataset    string    `json:"dataset"`
	Timestamp  time.Time `json:"timestamp"`
	IsVerified bool      `json:"is_verified"`
	Size       int64     `json:"size"`
	Writable   bool      `json:"writable"`
}

// RestorePointList 恢复点列表
type RestorePointList struct {
	Points []*RestorePoint `json:"points"`
	Total  int             `json:"total"`
	Volume string          `json:"volume"`
}
