package storagehealth

import (
	"context"
	"testing"
	"time"
)

func TestStorageHealthMonitor_RegisterPool(t *testing.T) {
	thresholds := &HealthThresholds{
		SpaceWarningPercent:  80,
		SpaceCriticalPercent: 95,
		TempWarningCelsius:   45,
		TempCriticalCelsius:  55,
		MaxReallocatedSectors: 10,
		MaxPendingSectors:    5,
		MaxErrors:            100,
	}

	monitor := NewStorageHealthMonitor(thresholds)

	pool := &StoragePool{
		ID:        "pool1",
		Name:      "Main Storage",
		Status:    PoolStatusOnline,
		Health:    HealthHealthy,
		TotalSize: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedSize:  1024 * 1024 * 1024 * 512,   // 512GB
		FreeSize:  1024 * 1024 * 1024 * 512,   // 512GB
		Devices: []*StorageDevice{
			{
				ID:          "disk1",
				Name:        "sda",
				Type:        "HDD",
				Status:      DeviceOnline,
				Health:      HealthHealthy,
				Size:        1024 * 1024 * 1024 * 500, // 500GB
				Temperature: 35.5,
				PowerOnHours: 1000,
			},
		},
	}

	monitor.RegisterPool(pool)

	retrieved, err := monitor.GetPoolHealth("pool1")
	if err != nil {
		t.Fatalf("Failed to get pool: %v", err)
	}

	if retrieved.Name != "Main Storage" {
		t.Errorf("Expected pool name 'Main Storage', got '%s'", retrieved.Name)
	}
}

func TestStorageHealthMonitor_UpdatePoolStatus(t *testing.T) {
	thresholds := &HealthThresholds{
		SpaceWarningPercent:  80,
		SpaceCriticalPercent: 95,
	}

	monitor := NewStorageHealthMonitor(thresholds)

	pool := &StoragePool{
		ID:        "pool1",
		Name:      "Test Pool",
		Status:    PoolStatusOnline,
		Health:    HealthHealthy,
		TotalSize: 100,
		UsedSize:  50,
		Devices:   []*StorageDevice{},
	}

	monitor.RegisterPool(pool)

	// Update to critical space usage
	pool.UsedSize = 96
	monitor.UpdatePoolStatus("pool1", PoolStatusOnline, HealthWarning)

	// Check for alerts
	alerts := monitor.GetAlerts(SeverityCritical, false)
	if len(alerts) == 0 {
		t.Error("Expected critical alert for high space usage")
	}
}

func TestStorageHealthMonitor_DeviceAlerts(t *testing.T) {
	thresholds := &HealthThresholds{
		TempWarningCelsius:   45,
		TempCriticalCelsius:  55,
		MaxReallocatedSectors: 10,
	}

	monitor := NewStorageHealthMonitor(thresholds)

	pool := &StoragePool{
		ID:   "pool1",
		Name: "Test Pool",
		Devices: []*StorageDevice{
			{
				ID:                "disk1",
				Name:              "sda",
				Temperature:       50, // Above warning threshold
				ReallocatedSectors: 15, // Above threshold
			},
		},
	}

	monitor.RegisterPool(pool)

	// Trigger alert check
	monitor.UpdatePoolStatus("pool1", PoolStatusOnline, HealthHealthy)

	// Should have temperature warning and SMART critical alerts
	alerts := monitor.GetAlerts("", false)
	if len(alerts) < 2 {
		t.Errorf("Expected at least 2 alerts, got %d", len(alerts))
	}
}

func TestStorageHealthMonitor_ResolveAlert(t *testing.T) {
	monitor := NewStorageHealthMonitor(nil)

	// Add a test alert
	alert := &HealthAlert{
		ID:       "alert1",
		Type:     AlertDiskWarning,
		Severity: SeverityWarning,
		Source:   "disk1",
		Message:  "Test alert",
	}

	monitor.mu.Lock()
	monitor.alerts[alert.ID] = alert
	monitor.mu.Unlock()

	// Resolve alert
	err := monitor.ResolveAlert("alert1")
	if err != nil {
		t.Fatalf("Failed to resolve alert: %v", err)
	}

	// Check alert is resolved
	alerts := monitor.GetAlerts(SeverityWarning, true)
	if len(alerts) != 1 {
		t.Errorf("Expected 1 resolved alert, got %d", len(alerts))
	}
}

func TestStorageHealthMonitor_GetHealthSummary(t *testing.T) {
	monitor := NewStorageHealthMonitor(nil)

	pool := &StoragePool{
		ID:        "pool1",
		Name:      "Test Pool",
		Health:    HealthHealthy,
		TotalSize: 1000,
		UsedSize:  500,
	}

	monitor.RegisterPool(pool)

	summary := monitor.GetHealthSummary()

	if summary["total_pools"] != 1 {
		t.Errorf("Expected 1 pool, got %v", summary["total_pools"])
	}

	if summary["total_size"] != int64(1000) {
		t.Errorf("Expected total size 1000, got %v", summary["total_size"])
	}
}

func TestStorageHealthMonitor_StartStop(t *testing.T) {
	monitor := NewStorageHealthMonitor(nil)
	ctx, cancel := context.WithCancel(context.Background())

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	cancel()
	monitor.Stop()
}
