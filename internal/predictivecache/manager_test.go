package predictivecache

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.predictionModel.WindowSize != 30 {
		t.Errorf("expected WindowSize 30, got %d", m.predictionModel.WindowSize)
	}
	if m.cachePolicy.EvictionPolicy != "adaptive" {
		t.Errorf("expected adaptive eviction, got %s", m.cachePolicy.EvictionPolicy)
	}
}

func TestRecordAccess(t *testing.T) {
	m := NewManager()
	record := &FileAccessRecord{
		FilePath:   "/data/test.txt",
		AccessTime: time.Now(),
		UserID:     "user1",
		Operation:  "read",
		SizeBytes:  1024,
		Duration:   100,
	}
	m.RecordAccess(record)
	records := m.accessRecords["/data/test.txt"]
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestAnalyzePatternsInsufficientData(t *testing.T) {
	m := NewManager()
	analysis := m.AnalyzePatterns("/nonexistent.txt")
	if analysis.Pattern != PatternRandom {
		t.Errorf("expected random pattern, got %s", analysis.Pattern)
	}
	if analysis.Confidence != ConfidenceLow {
		t.Errorf("expected low confidence, got %s", analysis.Confidence)
	}
}

func TestAnalyzePatternsHotspot(t *testing.T) {
	m := NewManager()
	baseTime := time.Now().Add(-24 * time.Hour)
	// 20 accesses within 24 hours = hotspot (>10/day)
	for i := 0; i < 20; i++ {
		m.RecordAccess(&FileAccessRecord{
			FilePath:   "/data/hot.txt",
			AccessTime: baseTime.Add(time.Duration(i) * time.Hour),
			UserID:     "user1",
			Operation:  "read",
			SizeBytes:  1024,
		})
	}
	analysis := m.AnalyzePatterns("/data/hot.txt")
	if analysis.Frequency < 10 {
		t.Errorf("expected high frequency, got %f", analysis.Frequency)
	}
}

func TestLoadToCache(t *testing.T) {
	m := NewManager()
	entry, err := m.LoadToCache("/data/file.txt", 1024, CacheL1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.FilePath != "/data/file.txt" {
		t.Errorf("expected path /data/file.txt, got %s", entry.FilePath)
	}
	if entry.CacheLevel != CacheL1 {
		t.Errorf("expected L1, got %s", entry.CacheLevel)
	}
}

func TestGetCacheEntry(t *testing.T) {
	m := NewManager()
	m.LoadToCache("/data/cached.txt", 1024, CacheL2)
	entry := m.GetCacheEntry("/data/cached.txt")
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.HitCount != 0 {
		t.Errorf("expected 0 hits, got %d", entry.HitCount)
	}
}

func TestGetCacheEntryNotFound(t *testing.T) {
	m := NewManager()
	entry := m.GetCacheEntry("/nonexistent.txt")
	if entry != nil {
		t.Errorf("expected nil, got %+v", entry)
	}
}

func TestPinUnpinCache(t *testing.T) {
	m := NewManager()
	m.LoadToCache("/data/pin.txt", 1024, CacheL1)
	if err := m.PinCache("/data/pin.txt"); err != nil {
		t.Fatalf("pin error: %v", err)
	}
	entry := m.GetCacheEntry("/data/pin.txt")
	if !entry.Pinned {
		t.Error("expected pinned")
	}
	if err := m.UnpinCache("/data/pin.txt"); err != nil {
		t.Fatalf("unpin error: %v", err)
	}
	entry = m.GetCacheEntry("/data/pin.txt")
	if entry.Pinned {
		t.Error("expected unpinned")
	}
}

func TestPinCacheNotFound(t *testing.T) {
	m := NewManager()
	if err := m.PinCache("/nonexistent"); err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestClearCache(t *testing.T) {
	m := NewManager()
	m.LoadToCache("/data/a.txt", 100, CacheL1)
	m.LoadToCache("/data/b.txt", 200, CacheL2)
	m.LoadToCache("/data/c.txt", 300, CacheL1)
	m.PinCache("/data/c.txt")

	cleared := m.ClearCache(CacheL1)
	if cleared != 1 { // only unpinned L1
		t.Errorf("expected 1 cleared, got %d", cleared)
	}
	if m.GetCacheEntry("/data/c.txt") == nil {
		t.Error("pinned entry should not be cleared")
	}
}

func TestCreateWarmingTask(t *testing.T) {
	m := NewManager()
	task := m.CreateWarmingTask("/data/warm.txt", CacheL2)
	if task.Status != "pending" {
		t.Errorf("expected pending, got %s", task.Status)
	}
	if task.CacheLevel != CacheL2 {
		t.Errorf("expected L2, got %s", task.CacheLevel)
	}
}

func TestGetWarmingTask(t *testing.T) {
	m := NewManager()
	task := m.CreateWarmingTask("/data/warm.txt", CacheL1)
	got, err := m.GetWarmingTask(task.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("expected %s, got %s", task.ID, got.ID)
	}
}

func TestGetWarmingTaskNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetWarmingTask("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListWarmingTasks(t *testing.T) {
	m := NewManager()
	m.CreateWarmingTask("/a.txt", CacheL1)
	m.CreateWarmingTask("/b.txt", CacheL2)
	tasks := m.ListWarmingTasks("")
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestGetCacheStats(t *testing.T) {
	m := NewManager()
	m.LoadToCache("/data/s.txt", 1024, CacheL1)
	stats := m.GetCacheStats()
	if stats["total_entries"] != 1 {
		t.Errorf("expected 1 entry, got %v", stats["total_entries"])
	}
}

func TestAutoWarm(t *testing.T) {
	m := NewManager()
	// not enough records
	tasks := m.AutoWarm()
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestCacheCapacity(t *testing.T) {
	m := NewManager()
	if !m.checkCacheCapacity(CacheL1, 1024) {
		t.Error("should have capacity")
	}
}

func TestDetermineCacheLevel(t *testing.T) {
	m := NewManager()
	small := []*FileAccessRecord{{SizeBytes: 1024}}
	if m.determineCacheLevel(small) != CacheL1 {
		t.Error("small file should be L1")
	}
	medium := []*FileAccessRecord{{SizeBytes: 100 * 1024 * 1024}}
	if m.determineCacheLevel(medium) != CacheL2 {
		t.Error("medium file should be L2")
	}
	large := []*FileAccessRecord{{SizeBytes: 2 * 1024 * 1024 * 1024}}
	if m.determineCacheLevel(large) != CacheL3 {
		t.Error("large file should be L3")
	}
}

func TestListPatterns(t *testing.T) {
	m := NewManager()
	patterns := m.ListPatterns(ConfidenceHigh)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(patterns))
	}
}

func TestDefaultPolicyValues(t *testing.T) {
	m := NewManager()
	if m.cachePolicy.MaxL1SizeGB != 8 {
		t.Errorf("expected 8GB L1, got %f", m.cachePolicy.MaxL1SizeGB)
	}
	if m.cachePolicy.TTLHours != 24 {
		t.Errorf("expected 24h TTL, got %d", m.cachePolicy.TTLHours)
	}
	if !m.cachePolicy.AutoWarming {
		t.Error("expected auto warming enabled")
	}
}
