// Package snapviz 提供快照可视化时间轴测试
package snapviz

import (
	"testing"
	"time"
)

func TestNewTimeline(t *testing.T) {
	tl := NewTimeline(nil)
	if tl == nil {
		t.Fatal("NewTimeline 返回 nil")
	}
	if tl.GetStats().TotalSnapshots != 0 {
		t.Error("新时间轴应该没有快照")
	}
}

func TestAddEvent(t *testing.T) {
	tl := NewTimeline(nil)
	now := time.Now()

	// 添加自动快照事件
	tl.AddEvent(&TimelineEvent{
		ID:         "snap-001",
		SnapshotID: "pool/data@auto-20260101",
		Volume:     "pool",
		Dataset:    "data",
		Timestamp:  now,
		Type:       EventSnapshotCreated,
		Size:       1024 * 1024 * 100,
		IsAuto:     true,
	})

	stats := tl.GetStats()
	if stats.TotalSnapshots != 1 {
		t.Errorf("快照数量应为 1, 实际 %d", stats.TotalSnapshots)
	}
	if stats.AutoSnapshots != 1 {
		t.Errorf("自动快照应为 1, 实际 %d", stats.AutoSnapshots)
	}
}

func TestAddMultipleEvents(t *testing.T) {
	tl := NewTimeline(nil)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		tl.AddEvent(&TimelineEvent{
			ID:         "snap-" + string(rune(i)),
			SnapshotID: "pool/data@snap-" + string(rune(i)),
			Volume:     "pool",
			Dataset:    "data",
			Timestamp:  base.Add(time.Duration(i) * time.Hour),
			Type:       EventSnapshotCreated,
			Size:       int64(i * 1024 * 1024),
			IsAuto:     i%2 == 0,
		})
	}

	stats := tl.GetStats()
	if stats.TotalSnapshots != 100 {
		t.Errorf("快照数量应为 100, 实际 %d", stats.TotalSnapshots)
	}
	if stats.AutoSnapshots != 50 {
		t.Errorf("自动快照应为 50, 实际 %d", stats.AutoSnapshots)
	}
}

func TestQueryFilter(t *testing.T) {
	tl := NewTimeline(nil)
	base := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	tl.AddEvent(&TimelineEvent{
		ID: "snap-1", Volume: "pool1", Dataset: "data",
		Timestamp: base, Type: EventSnapshotCreated, Size: 1024,
	})
	tl.AddEvent(&TimelineEvent{
		ID: "snap-2", Volume: "pool2", Dataset: "data",
		Timestamp: base.Add(time.Hour), Type: EventSnapshotCreated, Size: 2048,
	})

	// 按卷过滤
	results := tl.Query(&TimelineFilter{Volume: "pool1"})
	if len(results) != 1 {
		t.Errorf("过滤 pool1 应返回 1 条, 实际 %d", len(results))
	}

	// 按时间范围过滤
	results = tl.Query(&TimelineFilter{
		FromTime: base.Add(30 * time.Minute),
		ToTime:   base.Add(2 * time.Hour),
	})
	if len(results) != 1 {
		t.Errorf("时间范围过滤应返回 1 条, 实际 %d", len(results))
	}
}

func TestGetBuckets(t *testing.T) {
	tl := NewTimeline(nil)
	base := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 24; i++ {
		tl.AddEvent(&TimelineEvent{
			ID:     "snap-" + string(rune('A'+i)),
			Volume: "pool", Dataset: "data",
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			Type:      EventSnapshotCreated,
			Size:      1024,
		})
	}

	buckets := tl.GetBuckets(base, base.Add(24*time.Hour), 6*time.Hour)
	if len(buckets) != 4 {
		t.Errorf("6小时桶应返回 4 个, 实际 %d", len(buckets))
	}
	for _, b := range buckets {
		if b.EventCount != 6 {
			t.Errorf("每个桶应有 6 个事件, 实际 %d", b.EventCount)
		}
	}
}

func TestMaxEventsLimit(t *testing.T) {
	cfg := &TimelineConfig{MaxEvents: 10}
	tl := NewTimeline(cfg)
	base := time.Now()

	for i := 0; i < 20; i++ {
		tl.AddEvent(&TimelineEvent{
			ID:     "snap-" + string(rune(i)),
			Volume: "pool", Dataset: "data",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Type:      EventSnapshotCreated,
			Size:      1024,
		})
	}

	if len(tl.events) > 10 {
		t.Errorf("事件数应限制在 10, 实际 %d", len(tl.events))
	}
}

func TestFormatTimeline(t *testing.T) {
	tl := NewTimeline(nil)
	result := tl.FormatTimeline(nil)
	if result != "无快照事件" {
		t.Errorf("空时间轴应返回提示, 实际: %s", result)
	}

	now := time.Now()
	tl.AddEvent(&TimelineEvent{
		ID: "snap-1", Volume: "pool", Dataset: "data",
		Timestamp: now, Type: EventSnapshotCreated, Size: 1024 * 1024 * 100,
		IsAuto: true, Label: "每日自动",
	})

	result = tl.FormatTimeline(tl.Query(nil))
	if result == "无快照事件" {
		t.Error("有事件时不应返回空提示")
	}
}
