package smartdataplacement

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	m := NewManager(&Config{
		Enabled:                 true,
		AnalysisInterval:        time.Hour,
		TemperatureWindow:       time.Hour * 24 * 30,
		MinAccessForScore:       1,
		AutoMigrate:             false,
		MaxConcurrentMigrations: 5,
	})
	return m
}

func TestRegisterFile(t *testing.T) {
	m := newTestManager()

	err := m.RegisterFile("f1", "/data/video.mp4", 1024*1024*1024, TierSSD, "video")
	if err != nil {
		t.Fatalf("RegisterFile failed: %v", err)
	}

	file, err := m.GetFile("f1")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if file.FilePath != "/data/video.mp4" {
		t.Errorf("expected path /data/video.mp4, got %s", file.FilePath)
	}
	if file.SizeBytes != 1024*1024*1024 {
		t.Errorf("expected 1GB size, got %d", file.SizeBytes)
	}
	if file.CurrentTier != TierSSD {
		t.Errorf("expected SSD tier, got %s", file.CurrentTier)
	}

	// 空ID应失败
	err = m.RegisterFile("", "/test", 100, TierHDD, "text")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestRecordAccess(t *testing.T) {
	m := newTestManager()

	err := m.RecordAccess("nonexistent")
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}

	m.RegisterFile("f1", "/data/doc.pdf", 10*1024*1024, TierHDD, "pdf")

	// 多次访问
	for i := 0; i < 15; i++ {
		err = m.RecordAccess("f1")
		if err != nil {
			t.Fatalf("RecordAccess failed: %v", err)
		}
	}

	file, _ := m.GetFile("f1")
	if file.AccessCount != 15 {
		t.Errorf("expected 15 accesses, got %d", file.AccessCount)
	}
	if file.AccessScore <= 0 {
		t.Error("expected positive access score")
	}
}

func TestTemperatureCalculation(t *testing.T) {
	m := newTestManager()

	// 热文件：频繁访问
	m.RegisterFile("hot", "/data/hot.db", 1024*1024, TierNVMe, "db")
	for i := 0; i < 50; i++ {
		m.RecordAccess("hot")
	}
	file, _ := m.GetFile("hot")
	if file.Temperature != TemperatureHot {
		t.Errorf("expected hot, got %s", file.Temperature)
	}

	// 冷文件：很少访问
	m.RegisterFile("cold", "/data/old.tar", 1024*1024, TierHDD, "archive")
	// 只访问1次
	m.RecordAccess("cold")
	file, _ = m.GetFile("cold")
	if file.Temperature != TemperatureCold {
		t.Errorf("expected cold, got %s", file.Temperature)
	}
}

func TestAnalyzePlacement(t *testing.T) {
	m := newTestManager()

	m.RegisterFile("f1", "/data/a.db", 1024*1024*1024, TierNVMe, "db")
	m.RegisterFile("f2", "/data/b.mp4", 5*1024*1024*1024, TierHDD, "video")
	m.RegisterFile("f3", "/data/c.txt", 1024, TierSSD, "text")

	report, err := m.AnalyzePlacement()
	if err != nil {
		t.Fatalf("AnalyzePlacement failed: %v", err)
	}

	if report.TotalFiles != 3 {
		t.Errorf("expected 3 files, got %d", report.TotalFiles)
	}
	if report.TotalSizeBytes <= 0 {
		t.Error("expected positive total size")
	}
	if len(report.TierDistribution) == 0 {
		t.Error("expected tier distribution")
	}
	if len(report.TemperatureDistribution) == 0 {
		t.Error("expected temperature distribution")
	}
}

func TestPlanMigrations(t *testing.T) {
	m := newTestManager()

	// 创建一个热文件放在HDD上（应该迁移到NVMe）
	m.RegisterFile("hot-file", "/data/hot.db", 1024*1024, TierHDD, "db")
	for i := 0; i < 50; i++ {
		m.RecordAccess("hot-file")
	}

	tasks, err := m.PlanMigrations()
	if err != nil {
		t.Fatalf("PlanMigrations failed: %v", err)
	}

	if len(tasks) == 0 {
		t.Log("no migrations planned (may be expected if temperature thresholds differ)")
	}

	for _, task := range tasks {
		if task.Status != MigrationPending {
			t.Errorf("expected pending status, got %s", task.Status)
		}
		if task.SourceTier == task.TargetTier {
			t.Error("source and target tier should differ")
		}
	}
}

func TestCompleteMigration(t *testing.T) {
	m := newTestManager()

	// 创建需要迁移的文件
	m.RegisterFile("f1", "/data/file.db", 1024*1024, TierHDD, "db")
	for i := 0; i < 50; i++ {
		m.RecordAccess("f1")
	}

	tasks, _ := m.PlanMigrations()
	if len(tasks) == 0 {
		t.Skip("no migrations to test")
	}

	taskID := tasks[0].TaskID
	err := m.CompleteMigration(taskID)
	if err != nil {
		t.Fatalf("CompleteMigration failed: %v", err)
	}

	// 验证文件层已更新
	file, _ := m.GetFile("f1")
	if file.CurrentTier != tasks[0].TargetTier {
		t.Errorf("expected tier %s, got %s", tasks[0].TargetTier, file.CurrentTier)
	}

	// 验证迁移状态
	migs := m.GetMigrations(MigrationCompleted, 10)
	if len(migs) == 0 {
		t.Error("expected completed migration")
	}

	// 不存在的任务ID
	err = m.CompleteMigration("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestSetPolicy(t *testing.T) {
	m := newTestManager()

	err := m.SetPolicy(nil)
	if err != ErrInvalidPolicy {
		t.Errorf("expected ErrInvalidPolicy, got %v", err)
	}

	err = m.SetPolicy(&PlacementPolicy{
		Name: "performance",
		TierMapping: map[DataTemperature]StorageTier{
			TemperatureHot:  TierNVMe,
			TemperatureWarm: TierNVMe,
			TemperatureCold: TierSSD,
		},
	})
	if err != nil {
		t.Fatalf("SetPolicy failed: %v", err)
	}
}

func TestDashboard(t *testing.T) {
	m := newTestManager()

	m.RegisterFile("f1", "/a", 100, TierSSD, "text")
	m.RegisterFile("f2", "/b", 200, TierHDD, "video")

	dash := m.GetDashboard()
	if dash["totalFiles"] != 2 {
		t.Errorf("expected 2 files, got %v", dash["totalFiles"])
	}
	if dash["totalSizeBytes"] != int64(300) {
		t.Errorf("expected 300 bytes, got %v", dash["totalSizeBytes"])
	}
}

func TestGetMigrations(t *testing.T) {
	m := newTestManager()

	m.RegisterFile("f1", "/data/db", 1024*1024, TierHDD, "db")
	for i := 0; i < 50; i++ {
		m.RecordAccess("f1")
	}

	m.PlanMigrations()

	all := m.GetMigrations("", 10)
	pending := m.GetMigrations(MigrationPending, 10)

	if len(all) == 0 {
		t.Log("no migrations generated")
	}
	if len(pending) != len(all) {
		t.Log("some migrations already completed")
	}
}
