package smartdatatier

import (
	"testing"
	"time"
)

func TestNewTierManager(t *testing.T) {
	tm := NewTierManager(nil)
	if tm == nil {
		t.Fatal("NewTierManager returned nil")
	}
}

func TestRegisterFile(t *testing.T) {
	tm := NewTierManager(nil)

	file := &DataFile{
		ID:   "file1",
		Path: "/data/test.bin",
		Size: 1024 * 1024,
	}

	tm.RegisterFile(file)

	got, ok := tm.GetFile("file1")
	if !ok {
		t.Fatal("GetFile returned false")
	}
	if got.Path != "/data/test.bin" {
		t.Errorf("expected path '/data/test.bin', got '%s'", got.Path)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestRecordAccess(t *testing.T) {
	tm := NewTierManager(nil)

	file := &DataFile{
		ID:          "file1",
		Path:        "/data/test.bin",
		Size:        1024 * 1024,
		CurrentTier: TierCold,
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}
	tm.RegisterFile(file)

	// 多次访问
	for i := 0; i < 15; i++ {
		tm.RecordAccess("file1")
	}

	got, _ := tm.GetFile("file1")
	if got.AccessCount != 15 {
		t.Errorf("expected access count 15, got %d", got.AccessCount)
	}
	// 15次/1天 = 15次/天，应该升级到热层
	if got.CurrentTier != TierHot {
		t.Errorf("expected hot tier after frequent access, got %d", got.CurrentTier)
	}
}

func TestRecommendTier(t *testing.T) {
	tm := NewTierManager(&TierConfig{
		HotThreshold:  10.0,
		WarmThreshold: 1.0,
		ColdThreshold: 0.1,
	})

	// 未注册的文件
	tier := tm.RecommendTier("nonexistent")
	if tier != TierCold {
		t.Errorf("expected cold tier for unknown file, got %d", tier)
	}

	// 高频访问文件
	file := &DataFile{
		ID:          "hot-file",
		Path:        "/data/hot.bin",
		Size:        1024 * 1024,
		AccessFreq:  15.0,
		CurrentTier: TierHot,
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}
	tm.RegisterFile(file)

	tier = tm.RecommendTier("hot-file")
	if tier != TierHot {
		t.Errorf("expected hot tier, got %d", tier)
	}
}

func TestGetMigrationPlan(t *testing.T) {
	tm := NewTierManager(nil)

	// 注册一个冷数据文件但频繁访问
	file := &DataFile{
		ID:          "file1",
		Path:        "/data/migrate.bin",
		Size:        1024 * 1024,
		CurrentTier: TierCold,
		AccessFreq:  15.0, // 超过热阈值
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}
	tm.RegisterFile(file)

	plan := tm.GetMigrationPlan()
	if len(plan) == 0 {
		t.Error("expected migration plan for mis-tiered file")
	}
}

func TestListFiles(t *testing.T) {
	tm := NewTierManager(nil)

	tm.RegisterFile(&DataFile{ID: "1", Path: "/a", CurrentTier: TierHot})
	tm.RegisterFile(&DataFile{ID: "2", Path: "/b", CurrentTier: TierCold})
	tm.RegisterFile(&DataFile{ID: "3", Path: "/c", CurrentTier: TierHot})

	hotFiles := tm.ListFiles(TierHot)
	if len(hotFiles) != 2 {
		t.Errorf("expected 2 hot files, got %d", len(hotFiles))
	}

	allFiles := tm.ListFiles(-1)
	if len(allFiles) != 3 {
		t.Errorf("expected 3 total files, got %d", len(allFiles))
	}
}

func TestGetStats(t *testing.T) {
	tm := NewTierManager(nil)

	tm.RegisterFile(&DataFile{ID: "1", Path: "/a", Size: 100, CurrentTier: TierHot})
	tm.RegisterFile(&DataFile{ID: "2", Path: "/b", Size: 200, CurrentTier: TierCold})

	stats := tm.GetStats()
	if stats[TierHot].FileCount != 1 {
		t.Errorf("expected 1 hot file, got %d", stats[TierHot].FileCount)
	}
	if stats[TierHot].TotalSize != 100 {
		t.Errorf("expected hot total size 100, got %d", stats[TierHot].TotalSize)
	}
}
