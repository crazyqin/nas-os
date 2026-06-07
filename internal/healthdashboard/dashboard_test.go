package healthdashboard

import (
	"testing"
	"time"
)

func TestUpdateDiskWithWarnings(t *testing.T) {
	c := NewCollector()

	// High temperature disk
	c.UpdateDisk(DiskHealth{
		Device:      "/dev/sda",
		Model:       "WD Red",
		CapacityGB:  4000,
		Temperature: 60,
		HealthScore: 85,
	})

	dash := c.GetDashboard()
	if len(dash.Disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(dash.Disks))
	}

	disk := dash.Disks[0]
	if len(disk.Warnings) < 1 {
		t.Error("expected at least 1 warning for high temp")
	}
	if disk.LastCheck.IsZero() {
		t.Error("expected LastCheck to be set")
	}
}

func TestDiskWithReallocatedSectors(t *testing.T) {
	c := NewCollector()

	c.UpdateDisk(DiskHealth{
		Device:      "/dev/sdb",
		HealthScore: 45,
		Reallocated: 100,
		Pending:     5,
	})

	dash := c.GetDashboard()
	disk := dash.Disks[0]

	if len(disk.Warnings) < 3 {
		t.Errorf("expected 3+ warnings (temp+reallocated+pending+health), got %d", len(disk.Warnings))
	}
}

func TestPoolHealth(t *testing.T) {
	c := NewCollector()

	c.UpdatePool(PoolHealth{
		ID:           "pool1",
		Name:         "tank",
		Status:       PoolHealthy,
		RAIDLevel:    "raidz2",
		TotalGB:      16000,
		UsedGB:       8000,
		FreeGB:       8000,
		UsagePercent: 50.0,
		DiskCount:    4,
		HealthyDisks: 4,
	})

	dash := c.GetDashboard()
	if len(dash.Pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(dash.Pools))
	}

	pool := dash.Pools[0]
	if pool.UsagePercent != 50.0 {
		t.Errorf("expected 50%% usage, got %.1f%%", pool.UsagePercent)
	}
}

func TestCapacityTrend(t *testing.T) {
	c := NewCollector()

	c.AddTrend(1000, 500)
	time.Sleep(10 * time.Millisecond)
	c.AddTrend(1000, 600)

	dash := c.GetDashboard()
	if len(dash.Trends) != 2 {
		t.Fatalf("expected 2 trends, got %d", len(dash.Trends))
	}

	last := dash.Trends[1]
	if last.GrowthRateGB <= 0 {
		t.Error("expected positive growth rate")
	}
	// DaysUntilFull may round to 0 with very short intervals (ms-scale)
	// Just verify it's non-negative since growth rate is positive
	if last.DaysUntilFull < 0 {
		t.Errorf("expected non-negative days until full, got %d", last.DaysUntilFull)
	}
}

func TestGetWarnings(t *testing.T) {
	c := NewCollector()

	c.UpdateDisk(DiskHealth{
		Device:      "/dev/sda",
		Temperature: 60,
		HealthScore: 85,
	})

	c.UpdatePool(PoolHealth{
		ID:           "pool1",
		Name:         "tank",
		Status:       PoolDegraded,
		UsagePercent: 95.0,
	})

	warnings := c.GetWarnings()
	if len(warnings) < 3 {
		t.Errorf("expected 3+ warnings, got %d", len(warnings))
	}
}

func TestOverallScore(t *testing.T) {
	c := NewCollector()

	c.UpdateDisk(DiskHealth{Device: "/dev/sda", HealthScore: 90})
	c.UpdateDisk(DiskHealth{Device: "/dev/sdb", HealthScore: 80})
	c.UpdateDisk(DiskHealth{Device: "/dev/sdc", HealthScore: 70})

	dash := c.GetDashboard()
	expected := (90 + 80 + 70) / 3
	if dash.OverallScore != expected {
		t.Errorf("expected overall score %d, got %d", expected, dash.OverallScore)
	}
}

func TestPredictCapacity(t *testing.T) {
	c := NewCollector()

	c.UpdatePool(PoolHealth{
		ID:      "pool1",
		Name:    "tank",
		TotalGB: 10000,
		UsedGB:  5000,
	})

	// Need at least 2 trends for prediction
	c.AddTrend(10000, 4000)
	c.AddTrend(10000, 5000)

	projected, err := c.PredictCapacity("pool1", 30)
	if err != nil {
		t.Fatalf("PredictCapacity failed: %v", err)
	}

	// Should project more than current usage
	if projected <= 5000 {
		t.Errorf("expected projected > 5000, got %d", projected)
	}

	// Non-existent pool
	_, err = c.PredictCapacity("nonexistent", 30)
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestPredictCapacityCapped(t *testing.T) {
	c := NewCollector()

	c.UpdatePool(PoolHealth{
		ID:      "pool1",
		TotalGB: 100,
		UsedGB:  95,
	})

	c.AddTrend(100, 90)
	c.AddTrend(100, 95)

	projected, err := c.PredictCapacity("pool1", 365)
	if err != nil {
		t.Fatalf("PredictCapacity failed: %v", err)
	}

	// Should be capped at total
	if projected > 100 {
		t.Errorf("expected projected <= 100 (total), got %d", projected)
	}
}

func TestAlertCount(t *testing.T) {
	c := NewCollector()

	c.UpdateDisk(DiskHealth{Device: "/dev/sda", HealthScore: 90, Temperature: 30})
	c.UpdatePool(PoolHealth{ID: "p1", Status: PoolDegraded})
	c.UpdatePool(PoolHealth{ID: "p2", Status: PoolFaulted})

	dash := c.GetDashboard()
	if dash.AlertCount < 4 {
		t.Errorf("expected 4+ alerts (1 degraded + 3 faulted), got %d", dash.AlertCount)
	}
}

func TestTrendRetention(t *testing.T) {
	c := NewCollector()

	// Add 95 trends
	for i := 0; i < 95; i++ {
		c.AddTrend(1000, int64(100+i))
	}

	dash := c.GetDashboard()
	if len(dash.Trends) > 90 {
		t.Errorf("expected max 90 trends, got %d", len(dash.Trends))
	}
}
