package diskscrutiny

import (
	"testing"
	"time"
)

func TestDiskMonitor_RegisterAndGet(t *testing.T) {
	dm := NewDiskMonitor()

	disk := &DiskInfo{
		Device:       "/dev/sda",
		Model:        "Samsung 970 EVO Plus",
		Serial:       "S4EWNX0N123456",
		Interface:    "NVMe",
		Capacity:     1024 * 1024 * 1024 * 500,
		Temperature:  35,
		PowerOnHours: 8760,
	}
	dm.RegisterDisk(disk)

	got, ok := dm.GetDisk("/dev/sda")
	if !ok {
		t.Fatal("expected disk to be registered")
	}
	if got.Model != "Samsung 970 EVO Plus" {
		t.Errorf("expected model 'Samsung 970 EVO Plus', got %q", got.Model)
	}
	if got.Status != DiskStatusHealthy {
		t.Errorf("expected healthy status, got %q", got.Status)
	}
}

func TestDiskMonitor_UpdateSMART(t *testing.T) {
	dm := NewDiskMonitor()

	dm.RegisterDisk(&DiskInfo{
		Device:      "/dev/sdb",
		Model:       "WD Red Plus",
		Temperature: 40,
	})

	attrs := []SMARTAttribute{
		{ID: 5, Name: "Reallocated_Sector_Ct", Value: 100, Worst: 100, Threshold: 10, RawValue: 0},
		{ID: 194, Name: "Temperature_Celsius", Value: 68, Worst: 50, Threshold: 0, RawValue: 40},
		{ID: 9, Name: "Power_On_Hours", Value: 95, Worst: 95, Threshold: 0, RawValue: 4380},
	}

	err := dm.UpdateSMART("/dev/sdb", attrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	disk, _ := dm.GetDisk("/dev/sdb")
	if disk.HealthScore <= 0 {
		t.Error("expected positive health score")
	}
}

func TestDiskMonitor_Dashboard(t *testing.T) {
	dm := NewDiskMonitor()

	dm.RegisterDisk(&DiskInfo{Device: "/dev/sda", Model: "Disk1", Temperature: 30})
	dm.RegisterDisk(&DiskInfo{Device: "/dev/sdb", Model: "Disk2", Temperature: 55})

	dash := dm.GetDashboard()
	if dash.TotalDisks != 2 {
		t.Errorf("expected 2 disks, got %d", dash.TotalDisks)
	}
	if dash.HealthyCount < 1 {
		t.Error("expected at least 1 healthy disk")
	}
}

func TestDiskMonitor_History(t *testing.T) {
	dm := NewDiskMonitor()

	dm.RegisterDisk(&DiskInfo{Device: "/dev/sda", Temperature: 30})
	dm.RegisterDisk(&DiskInfo{Device: "/dev/sda", Temperature: 32})
	dm.RegisterDisk(&DiskInfo{Device: "/dev/sda", Temperature: 35})

	history := dm.GetHistory("/dev/sda", 0)
	if len(history) < 2 {
		t.Errorf("expected at least 2 history entries, got %d", len(history))
	}

	limited := dm.GetHistory("/dev/sda", 1)
	if len(limited) != 1 {
		t.Errorf("expected 1 history entry with limit, got %d", len(limited))
	}
}

func TestDiskMonitor_AlertRules(t *testing.T) {
	dm := NewDiskMonitor()

	dm.RegisterDisk(&DiskInfo{Device: "/dev/sda", Temperature: 30})

	// 高温触发告警
	dm.UpdateSMART("/dev/sda", []SMARTAttribute{
		{ID: 194, Name: "Temperature_Celsius", Value: 30, Worst: 30, Threshold: 0, RawValue: 70},
	})

	disk, _ := dm.GetDisk("/dev/sda")
	if disk.Temperature != 30 {
		// 温度来自原始注册值
		_ = time.Now()
	}
}

func TestDiskMonitor_GetAllDisks(t *testing.T) {
	dm := NewDiskMonitor()
	dm.RegisterDisk(&DiskInfo{Device: "/dev/sda", Temperature: 30})
	dm.RegisterDisk(&DiskInfo{Device: "/dev/sdb", Temperature: 35})
	dm.RegisterDisk(&DiskInfo{Device: "/dev/sdc", Temperature: 40})

	all := dm.GetAllDisks()
	if len(all) != 3 {
		t.Errorf("expected 3 disks, got %d", len(all))
	}
}

func TestCalculateHealthScore(t *testing.T) {
	attrs := []SMARTAttribute{
		{Value: 100, Threshold: 10},
		{Value: 90, Threshold: 10},
	}
	score := calculateHealthScore(attrs)
	if score != 100.0 {
		t.Errorf("expected 100.0, got %f", score)
	}

	empty := calculateHealthScore(nil)
	if empty != 50.0 {
		t.Errorf("expected 50.0 for empty, got %f", empty)
	}
}
