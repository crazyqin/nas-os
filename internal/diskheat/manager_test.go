package diskheat

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	disks := m.ListDisks()
	if len(disks) == 0 {
		t.Error("expected default disks")
	}
}

func TestListDisks(t *testing.T) {
	m := NewManager()

	disks := m.ListDisks()
	if len(disks) < 3 {
		t.Errorf("expected at least 3 disks, got %d", len(disks))
	}

	// 应该按热度排序
	if len(disks) >= 2 && disks[0].HeatScore < disks[1].HeatScore {
		t.Error("expected sorted by heat score")
	}
}

func TestGetDiskMetrics(t *testing.T) {
	m := NewManager()

	disk, err := m.GetDiskMetrics("sda")
	if err != nil {
		t.Fatalf("get disk failed: %v", err)
	}
	if disk.Model != "WD Red Plus 4TB" {
		t.Errorf("expected 'WD Red Plus 4TB', got '%s'", disk.Model)
	}

	_, err = m.GetDiskMetrics("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent disk")
	}
}

func TestUpdateMetrics(t *testing.T) {
	m := NewManager()

	err := m.UpdateMetrics("sda", DiskMetrics{
		Device: "sda", Model: "WD Red Plus 4TB", MountPoint: "/data",
		TotalBytes: 4000000000000, UsedBytes: 3000000000000,
		ReadOPS: 5000, WriteOPS: 3000, ReadBytes: 100000000, WriteBytes: 60000000,
		ReadLatency: 0.5, WriteLatency: 0.8, Temperature: 40, Health: "good",
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	disk, _ := m.GetDiskMetrics("sda")
	if disk.ReadOPS != 5000 {
		t.Errorf("expected 5000, got %d", disk.ReadOPS)
	}
}

func TestHeatmapGeneration(t *testing.T) {
	m := NewManager()

	heatmap := m.GenerateHeatmap()
	if len(heatmap.Disks) == 0 {
		t.Error("expected heatmap data")
	}
	if heatmap.GeneratedAt.IsZero() {
		t.Error("expected timestamp")
	}

	// 验证热度等级
	for _, point := range heatmap.Disks {
		if point.Level == "" {
			t.Error("expected heat level")
		}
	}
}

func TestBottleneckDetection(t *testing.T) {
	m := NewManager()

	// 高延迟
	m.UpdateMetrics("test-disk", DiskMetrics{
		Device: "test-disk", Model: "Old HDD", MountPoint: "/old",
		TotalBytes: 1000000000000, UsedBytes: 500000000000,
		ReadOPS: 100, WriteOPS: 50, ReadBytes: 1000000, WriteBytes: 500000,
		ReadLatency: 15.0, WriteLatency: 20.0, Temperature: 35, Health: "good",
	})

	bottlenecks := m.GetBottlenecks()
	found := false
	for _, b := range bottlenecks {
		if b.Type == "high_latency" {
			found = true
		}
	}
	if !found {
		t.Error("expected high latency bottleneck")
	}
}

func TestHighUsageBottleneck(t *testing.T) {
	m := NewManager()

	m.UpdateMetrics("full-disk", DiskMetrics{
		Device: "full-disk", Model: "Small SSD", MountPoint: "/full",
		TotalBytes: 100000000000, UsedBytes: 95000000000,
		ReadOPS: 1000, WriteOPS: 500, ReadBytes: 10000000, WriteBytes: 5000000,
		ReadLatency: 0.1, WriteLatency: 0.2, Temperature: 40, Health: "good",
	})

	bottlenecks := m.GetBottlenecks()
	found := false
	for _, b := range bottlenecks {
		if b.Type == "high_usage" && b.Severity == "critical" {
			found = true
		}
	}
	if !found {
		t.Error("expected high usage bottleneck")
	}
}

func TestHighTempBottleneck(t *testing.T) {
	m := NewManager()

	m.UpdateMetrics("hot-disk", DiskMetrics{
		Device: "hot-disk", Model: "Hot HDD", MountPoint: "/hot",
		TotalBytes: 2000000000000, UsedBytes: 1000000000000,
		ReadOPS: 1000, WriteOPS: 500, ReadBytes: 10000000, WriteBytes: 5000000,
		ReadLatency: 0.5, WriteLatency: 0.8, Temperature: 65, Health: "good",
	})

	bottlenecks := m.GetBottlenecks()
	found := false
	for _, b := range bottlenecks {
		if b.Type == "high_temp" {
			found = true
		}
	}
	if !found {
		t.Error("expected high temp bottleneck")
	}
}

func TestIOStats(t *testing.T) {
	m := NewManager()

	stats := m.GetIOStats()
	if len(stats) == 0 {
		t.Error("expected IO stats")
	}

	// 验证按使用率排序
	if len(stats) >= 2 && stats[0].Utilization < stats[1].Utilization {
		t.Error("expected sorted by utilization")
	}
}

func TestDiskHealth(t *testing.T) {
	m := NewManager()

	report, err := m.GetDiskHealth("sda")
	if err != nil {
		t.Fatalf("get health failed: %v", err)
	}
	if report.Model != "WD Red Plus 4TB" {
		t.Errorf("expected 'WD Red Plus 4TB', got '%s'", report.Model)
	}
	if report.HealthScore <= 0 || report.HealthScore > 100 {
		t.Errorf("invalid health score: %.1f", report.HealthScore)
	}
}

func TestDiskHealthWithIssues(t *testing.T) {
	m := NewManager()

	// 高温 + 高使用率
	m.UpdateMetrics("issue-disk", DiskMetrics{
		Device: "issue-disk", Model: "Problem Disk", MountPoint: "/issue",
		TotalBytes: 1000000000000, UsedBytes: 900000000000,
		ReadOPS: 1000, WriteOPS: 500, ReadBytes: 10000000, WriteBytes: 5000000,
		ReadLatency: 8.0, WriteLatency: 12.0, Temperature: 55, Health: "warning",
	})

	report, _ := m.GetDiskHealth("issue-disk")
	if len(report.Recommendations) == 0 {
		t.Error("expected recommendations")
	}
}

func TestOverallStats(t *testing.T) {
	m := NewManager()

	stats := m.GetOverallStats()
	if stats["totalDisks"].(int) == 0 {
		t.Error("expected disks")
	}
	if stats["totalCapacity"].(int64) == 0 {
		t.Error("expected capacity")
	}
	if stats["avgHeat"].(float64) == 0 {
		t.Error("expected non-zero heat")
	}
}

func TestHeatLevels(t *testing.T) {
	m := NewManager()

	tests := []struct {
		heat  float64
		level HeatLevel
	}{
		{10, HeatLevelCold},
		{30, HeatLevelCool},
		{50, HeatLevelWarm},
		{70, HeatLevelHot},
		{90, HeatLevelFire},
	}

	for _, tt := range tests {
		level := m.getHeatLevel(tt.heat)
		if level != tt.level {
			t.Errorf("heat %.0f: expected %s, got %s", tt.heat, tt.level, level)
		}
	}
}
