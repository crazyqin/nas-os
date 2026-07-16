package smarthdd

import (
	"testing"
)

func TestNewSmartHDDManager(t *testing.T) {
	manager := NewSmartHDDManager(nil)
	if manager == nil {
		t.Fatal("Expected manager")
	}
	if manager.config.TempThreshold != 55 {
		t.Errorf("Expected temp threshold 55, got %d", manager.config.TempThreshold)
	}
}

func TestRegisterDisk(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{
		Device:      "/dev/sda",
		Model:       "WD Red",
		Serial:      "WD123456",
		Size:        1024 * 1024 * 1024 * 1024,
		Temperature: 35,
		SMARTPassed: true,
	}

	err := manager.RegisterDisk(disk)
	if err != nil {
		t.Fatalf("Failed to register disk: %v", err)
	}

	if disk.ID == "" {
		t.Error("Expected disk ID to be set")
	}

	if disk.Health != HealthGood {
		t.Errorf("Expected health 'good', got '%s'", disk.Health)
	}

	// 测试空设备路径
	err = manager.RegisterDisk(&DiskInfo{})
	if err == nil {
		t.Error("Expected error for empty device")
	}
}

func TestUnregisterDisk(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{Device: "/dev/sda", SMARTPassed: true}
	manager.RegisterDisk(disk)

	err := manager.UnregisterDisk(disk.ID)
	if err != nil {
		t.Fatalf("Failed to unregister: %v", err)
	}

	_, err = manager.GetDisk(disk.ID)
	if err == nil {
		t.Error("Expected error for unregistered disk")
	}

	// 测试注销不存在的磁盘
	err = manager.UnregisterDisk("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent disk")
	}
}

func TestGetDisk(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{Device: "/dev/sda", Model: "Test", SMARTPassed: true}
	manager.RegisterDisk(disk)

	fetched, err := manager.GetDisk(disk.ID)
	if err != nil {
		t.Fatalf("Failed to get disk: %v", err)
	}

	if fetched.Device != "/dev/sda" {
		t.Errorf("Expected device '/dev/sda', got '%s'", fetched.Device)
	}
}

func TestListDisks(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	manager.RegisterDisk(&DiskInfo{Device: "/dev/sda", SMARTPassed: true})
	manager.RegisterDisk(&DiskInfo{Device: "/dev/sdb", SMARTPassed: true})

	disks := manager.ListDisks()
	if len(disks) != 2 {
		t.Errorf("Expected 2 disks, got %d", len(disks))
	}
}

func TestTemperatureAlert(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{
		Device:      "/dev/sda",
		Temperature: 60,
		SMARTPassed: true,
	}

	manager.RegisterDisk(disk)

	if disk.Health != HealthCritical {
		t.Errorf("Expected health 'critical', got '%s'", disk.Health)
	}

	alerts := manager.GetAlerts(false)
	if len(alerts) == 0 {
		t.Error("Expected temperature alert")
	}
}

func TestReallocSectorsAlert(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{
		Device:         "/dev/sda",
		ReallocSectors: 150,
		SMARTPassed:    true,
	}

	manager.RegisterDisk(disk)

	if disk.Health != HealthCritical {
		t.Errorf("Expected health 'critical', got '%s'", disk.Health)
	}
}

func TestSMARTFailureAlert(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{
		Device:      "/dev/sda",
		SMARTPassed: false,
	}

	manager.RegisterDisk(disk)

	if disk.Health != HealthCritical {
		t.Errorf("Expected health 'critical', got '%s'", disk.Health)
	}
}

func TestGetStats(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	manager.RegisterDisk(&DiskInfo{Device: "/dev/sda", Size: 1024, Temperature: 30, SMARTPassed: true})
	manager.RegisterDisk(&DiskInfo{Device: "/dev/sdb", Size: 2048, Temperature: 40, SMARTPassed: true})

	stats := manager.GetStats()
	if stats.TotalDisks != 2 {
		t.Errorf("Expected 2 disks, got %d", stats.TotalDisks)
	}
	if stats.HealthyDisks != 2 {
		t.Errorf("Expected 2 healthy disks, got %d", stats.HealthyDisks)
	}
	if stats.TotalCapacity != 3072 {
		t.Errorf("Expected total capacity 3072, got %d", stats.TotalCapacity)
	}
}

func TestResolveAlert(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{Device: "/dev/sda", Temperature: 60, SMARTPassed: true}
	manager.RegisterDisk(disk)

	alerts := manager.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("Expected at least one alert")
	}

	err := manager.ResolveAlert(alerts[0].ID)
	if err != nil {
		t.Fatalf("Failed to resolve alert: %v", err)
	}

	resolvedAlerts := manager.GetAlerts(true)
	if len(resolvedAlerts) != 1 {
		t.Errorf("Expected 1 resolved alert, got %d", len(resolvedAlerts))
	}
}

func TestScanDisk(t *testing.T) {
	manager := NewSmartHDDManager(nil)

	disk := &DiskInfo{Device: "/dev/sda", SMARTPassed: true}
	manager.RegisterDisk(disk)

	scanned, err := manager.ScanDisk(disk.ID)
	if err != nil {
		t.Fatalf("Failed to scan disk: %v", err)
	}

	if scanned.LastScan.IsZero() {
		t.Error("Expected last scan to be set")
	}
}
