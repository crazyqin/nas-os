// Package snapviz 提供存储快照可视化时间轴功能
// 对标 Synology Snapshot Replication 的时间轴浏览体验
package snapviz

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// TimelineEvent 时间轴事件
type TimelineEvent struct {
	ID          string    `json:"id"`
	SnapshotID  string    `json:"snapshot_id"`
	Volume      string    `json:"volume"`
	Dataset     string    `json:"dataset"`
	Timestamp   time.Time `json:"timestamp"`
	Type        EventType `json:"type"`
	Size        int64     `json:"size"`
	Used        int64     `json:"used"`
	Referenced  int64     `json:"referenced"`
	IsAuto      bool      `json:"is_auto"`
	Label       string    `json:"label,omitempty"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

// EventType 事件类型
type EventType string

const (
	EventSnapshotCreated    EventType = "snapshot_created"
	EventSnapshotDeleted    EventType = "snapshot_deleted"
	EventSnapshotRestored   EventType = "snapshot_restored"
	EventSnapshotReplicated EventType = "snapshot_replicated"
	EventSnapshotCloned     EventType = "snapshot_cloned"
	EventScheduleTrigger    EventType = "schedule_triggered"
)

// TimelineFilter 时间轴过滤器
type TimelineFilter struct {
	Volume   string    `json:"volume,omitempty"`
	Dataset  string    `json:"dataset,omitempty"`
	Type     EventType `json:"type,omitempty"`
	FromTime time.Time `json:"from_time,omitempty"`
	ToTime   time.Time `json:"to_time,omitempty"`
	IsAuto   *bool     `json:"is_auto,omitempty"`
}

// TimelineConfig 时间轴配置
type TimelineConfig struct {
	MaxEvents       int           `json:"max_events"`
	AutoGranularity bool          `json:"auto_granularity"`
	BucketSize      time.Duration `json:"bucket_size"`
	EnableDiff      bool          `json:"enable_diff"`
	MaxDiffItems    int           `json:"max_diff_items"`
}

// TimelineStats 时间轴统计
type TimelineStats struct {
	TotalSnapshots  int               `json:"total_snapshots"`
	TotalSize       int64             `json:"total_size"`
	AutoSnapshots   int               `json:"auto_snapshots"`
	ManualSnapshots int               `json:"manual_snapshots"`
	SpaceSaved      int64             `json:"space_saved"`
	OldestSnapshot  *time.Time        `json:"oldest_snapshot,omitempty"`
	NewestSnapshot  *time.Time        `json:"newest_snapshot,omitempty"`
	TypeCounts      map[EventType]int `json:"type_counts"`
	HourlyBuckets   map[int]int       `json:"hourly_buckets"`
	DailyBuckets    map[string]int    `json:"daily_buckets"`
}

// TimelineBucket 时间轴桶（聚合）
type TimelineBucket struct {
	StartTime  time.Time        `json:"start_time"`
	EndTime    time.Time        `json:"end_time"`
	Events     []*TimelineEvent `json:"events"`
	EventCount int              `json:"event_count"`
	TotalSize  int64            `json:"total_size"`
}

// SnapshotDiff 快照差异
type SnapshotDiff struct {
	SnapshotA     string      `json:"snapshot_a"`
	SnapshotB     string      `json:"snapshot_b"`
	Volume        string      `json:"volume"`
	AddedFiles    []DiffEntry `json:"added_files"`
	RemovedFiles  []DiffEntry `json:"removed_files"`
	ModifiedFiles []DiffEntry `json:"modified_files"`
	AddedSize     int64       `json:"added_size"`
	RemovedSize   int64       `json:"removed_size"`
	TotalChanges  int         `json:"total_changes"`
}

// DiffEntry 差异条目
type DiffEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}

// Timeline 时间轴管理器
type Timeline struct {
	mu     sync.RWMutex
	events []*TimelineEvent
	config *TimelineConfig
	stats  *TimelineStats
}

// NewTimeline 创建时间轴管理器
func NewTimeline(config *TimelineConfig) *Timeline {
	if config == nil {
		config = &TimelineConfig{
			MaxEvents:       5000,
			AutoGranularity: true,
			BucketSize:      time.Hour,
			EnableDiff:      true,
			MaxDiffItems:    100,
		}
	}
	return &Timeline{
		events: make([]*TimelineEvent, 0),
		config: config,
		stats: &TimelineStats{
			TypeCounts:    make(map[EventType]int),
			HourlyBuckets: make(map[int]int),
			DailyBuckets:  make(map[string]int),
		},
	}
}

// AddEvent 添加事件到时间轴
func (t *Timeline) AddEvent(event *TimelineEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.events = append(t.events, event)
	t.stats.TotalSnapshots++
	t.stats.TotalSize += event.Size

	if event.IsAuto {
		t.stats.AutoSnapshots++
	} else {
		t.stats.ManualSnapshots++
	}

	t.stats.TypeCounts[event.Type]++

	// 按时间排序
	sort.Slice(t.events, func(i, j int) bool {
		return t.events[i].Timestamp.Before(t.events[j].Timestamp)
	})

	// 更新最旧/最新快照
	ts := event.Timestamp
	if t.stats.OldestSnapshot == nil || ts.Before(*t.stats.OldestSnapshot) {
		t.stats.OldestSnapshot = &ts
	}
	if t.stats.NewestSnapshot == nil || ts.After(*t.stats.NewestSnapshot) {
		t.stats.NewestSnapshot = &ts
	}

	// 按小时/天分桶
	t.stats.HourlyBuckets[ts.Hour()]++
	t.stats.DailyBuckets[ts.Format("2006-01-02")]++

	// 限制最大事件数
	if len(t.events) > t.config.MaxEvents {
		t.events = t.events[len(t.events)-t.config.MaxEvents:]
	}
}

// Query 查询时间轴
func (t *Timeline) Query(filter *TimelineFilter) []*TimelineEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*TimelineEvent
	for _, event := range t.events {
		if filter == nil || matchFilter(event, filter) {
			result = append(result, event)
		}
	}
	return result
}

// matchFilter 检查事件是否匹配过滤器
func matchFilter(event *TimelineEvent, filter *TimelineFilter) bool {
	if filter.Volume != "" && event.Volume != filter.Volume {
		return false
	}
	if filter.Dataset != "" && event.Dataset != filter.Dataset {
		return false
	}
	if filter.Type != "" && event.Type != filter.Type {
		return false
	}
	if !filter.FromTime.IsZero() && event.Timestamp.Before(filter.FromTime) {
		return false
	}
	if !filter.ToTime.IsZero() && event.Timestamp.After(filter.ToTime) {
		return false
	}
	if filter.IsAuto != nil && event.IsAuto != *filter.IsAuto {
		return false
	}
	return true
}

// GetBuckets 获取时间桶
func (t *Timeline) GetBuckets(from, to time.Time, bucketSize time.Duration) []*TimelineBucket {
	t.mu.RLock()
	defer t.mu.RUnlock()

	buckets := make(map[time.Time]*TimelineBucket)
	for _, event := range t.events {
		if event.Timestamp.Before(from) || event.Timestamp.After(to) {
			continue
		}
		bucketStart := event.Timestamp.Truncate(bucketSize)
		bucket, exists := buckets[bucketStart]
		if !exists {
			bucket = &TimelineBucket{
				StartTime: bucketStart,
				EndTime:   bucketStart.Add(bucketSize),
			}
			buckets[bucketStart] = bucket
		}
		bucket.Events = append(bucket.Events, event)
		bucket.EventCount++
		bucket.TotalSize += event.Size
	}

	var result []*TimelineBucket
	for _, b := range buckets {
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.Before(result[j].StartTime)
	})
	return result
}

// GetStats 获取统计信息
func (t *Timeline) GetStats() *TimelineStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &TimelineStats{
		TotalSnapshots:  t.stats.TotalSnapshots,
		TotalSize:       t.stats.TotalSize,
		AutoSnapshots:   t.stats.AutoSnapshots,
		ManualSnapshots: t.stats.ManualSnapshots,
		SpaceSaved:      t.stats.SpaceSaved,
		TypeCounts:      make(map[EventType]int),
		HourlyBuckets:   make(map[int]int),
		DailyBuckets:    make(map[string]int),
	}
	for k, v := range t.stats.TypeCounts {
		stats.TypeCounts[k] = v
	}
	for k, v := range t.stats.HourlyBuckets {
		stats.HourlyBuckets[k] = v
	}
	for k, v := range t.stats.DailyBuckets {
		stats.DailyBuckets[k] = v
	}
	if t.stats.OldestSnapshot != nil {
		oldest := *t.stats.OldestSnapshot
		stats.OldestSnapshot = &oldest
	}
	if t.stats.NewestSnapshot != nil {
		newest := *t.stats.NewestSnapshot
		stats.NewestSnapshot = &newest
	}
	return stats
}

// CalculateDiff 计算两个快照之间的差异
func (t *Timeline) CalculateDiff(snapshotA, snapshotB string) (*SnapshotDiff, error) {
	// 在实际实现中会调用 ZFS/btrfs diff 命令
	// 这里提供框架逻辑
	return &SnapshotDiff{
		SnapshotA:     snapshotA,
		SnapshotB:     snapshotB,
		AddedFiles:    []DiffEntry{},
		RemovedFiles:  []DiffEntry{},
		ModifiedFiles: []DiffEntry{},
	}, nil
}

// FormatTimeline 格式化时间轴为文本报告
func (t *Timeline) FormatTimeline(events []*TimelineEvent) string {
	if len(events) == 0 {
		return "无快照事件"
	}

	var sb strings.Builder
	sb.WriteString("快照时间轴:\n")
	sb.WriteString(strings.Repeat("─", 60) + "\n")
	for _, event := range events {
		sb.WriteString(fmt.Sprintf("[%s] %s | %s/%s | 大小: %s",
			event.Timestamp.Format("2006-01-02 15:04:05"),
			event.Type,
			event.Volume,
			event.Dataset,
			formatSize(event.Size),
		))
		if event.IsAuto {
			sb.WriteString(" [自动]")
		}
		if event.Label != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", event.Label))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatSize 格式化大小
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case size >= TB:
		return fmt.Sprintf("%.2fTB", float64(size)/float64(TB))
	case size >= GB:
		return fmt.Sprintf("%.2fGB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2fMB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2fKB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%dB", size)
	}
}
