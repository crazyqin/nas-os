package smarttierengine

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if len(engine.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(engine.files))
	}
}

func TestRecordAccess(t *testing.T) {
	engine := NewEngine()
	engine.RecordAccess("/data/test.txt", 1024, true)
	
	files := engine.GetFiles()
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].FilePath != "/data/test.txt" {
		t.Errorf("unexpected path: %s", files[0].FilePath)
	}
	if files[0].ReadCount != 1 {
		t.Errorf("expected 1 read, got %d", files[0].ReadCount)
	}
}

func TestHeatScore(t *testing.T) {
	engine := NewEngine()
	// 多次访问应提高热度
	for i := 0; i < 20; i++ {
		engine.RecordAccess("/data/hot.txt", 1024, true)
	}
	engine.RecordAccess("/data/cold.txt", 1024, true)
	
	files := engine.GetFiles()
	if len(files) < 2 {
		t.Fatal("expected at least 2 files")
	}
	// hot.txt 应该有更高热度
	if files[0].HeatScore <= files[1].HeatScore {
		t.Errorf("expected hot file to have higher score: %f vs %f", files[0].HeatScore, files[1].HeatScore)
	}
}

func TestGetStats(t *testing.T) {
	engine := NewEngine()
	engine.RecordAccess("/data/a.txt", 100, true)
	engine.RecordAccess("/data/b.txt", 200, false)
	
	// 手动触发统计更新
	engine.mu.Lock()
	engine.updateStats()
	engine.mu.Unlock()
	
	stats := engine.GetStats()
	if stats.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", stats.TotalFiles)
	}
}

func TestGetMigrations(t *testing.T) {
	engine := NewEngine()
	migrations := engine.GetMigrations()
	if migrations == nil {
		t.Fatal("expected non-nil migrations slice")
	}
}
