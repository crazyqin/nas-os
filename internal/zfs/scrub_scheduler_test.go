package zfs

import (
	"testing"
	"time"
)

func TestNewScrubScheduler(t *testing.T) {
	config := DefaultScrubScheduleConfig()
	scheduler := NewScrubScheduler("testpool", config)

	if scheduler == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if scheduler.poolName != "testpool" {
		t.Errorf("expected pool name 'testpool', got '%s'", scheduler.poolName)
	}
	if scheduler.IsRunning() {
		t.Error("new scheduler should not be running")
	}
}

func TestDefaultScrubScheduleConfig(t *testing.T) {
	config := DefaultScrubScheduleConfig()

	if !config.Enabled {
		t.Error("default config should be enabled")
	}
	if config.IntervalDays != 14 {
		t.Errorf("expected interval 14 days, got %d", config.IntervalDays)
	}
	if config.PreferredHour != 2 {
		t.Errorf("expected preferred hour 2, got %d", config.PreferredHour)
	}
	if config.IOPSThreshold != 500 {
		t.Errorf("expected IOPS threshold 500, got %d", config.IOPSThreshold)
	}
	if !config.AutoPauseOnLoad {
		t.Error("default config should have auto pause enabled")
	}
}

func TestUpdateConfig(t *testing.T) {
	config := DefaultScrubScheduleConfig()
	scheduler := NewScrubScheduler("testpool", config)

	newConfig := ScrubScheduleConfig{
		Enabled:         false,
		IntervalDays:    7,
		PreferredHour:   3,
		IOPSThreshold:   1000,
		AutoPauseOnLoad: false,
		MaxErrorCount:   5,
	}
	scheduler.UpdateConfig(newConfig)

	got := scheduler.GetConfig()
	if got.Enabled {
		t.Error("expected config to be disabled")
	}
	if got.IntervalDays != 7 {
		t.Errorf("expected interval 7 days, got %d", got.IntervalDays)
	}
	if got.IOPSThreshold != 1000 {
		t.Errorf("expected IOPS threshold 1000, got %d", got.IOPSThreshold)
	}
}

func TestGetProgress_Idle(t *testing.T) {
	config := DefaultScrubScheduleConfig()
	scheduler := NewScrubScheduler("testpool", config)

	progress := scheduler.GetProgress()
	if progress.Status != ScrubStatusIdle {
		t.Errorf("expected idle status, got %s", progress.Status)
	}
	if progress.PoolName != "testpool" {
		t.Errorf("expected pool name 'testpool', got '%s'", progress.PoolName)
	}
}

func TestGetHistory_Empty(t *testing.T) {
	config := DefaultScrubScheduleConfig()
	scheduler := NewScrubScheduler("testpool", config)

	history := scheduler.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d items", len(history))
	}
}

func TestStartStop(t *testing.T) {
	config := DefaultScrubScheduleConfig()
	scheduler := NewScrubScheduler("testpool", config)

	scheduler.Start()
	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start()")
	}

	// Double start should be safe
	scheduler.Start()
	if !scheduler.IsRunning() {
		t.Error("scheduler should still be running after double Start()")
	}

	scheduler.Stop()
	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after Stop()")
	}
}

func TestScrubStatusConstants(t *testing.T) {
	statuses := []ScrubStatus{
		ScrubStatusIdle,
		ScrubStatusRunning,
		ScrubStatusPaused,
		ScrubStatusCompleted,
		ScrubStatusFailed,
	}
	expected := []string{"idle", "running", "paused", "completed", "failed"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected status '%s', got '%s'", expected[i], string(s))
		}
	}
}

func TestParseZpoolScrubOutput_Idle(t *testing.T) {
	output := `  pool: tank
 state: ONLINE
  scan: none requested
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
	  sda       ONLINE       0     0     0

errors: No known data errors`

	progress := parseZpoolScrubOutput(output, "tank")
	if progress != nil {
		t.Error("expected nil progress for idle pool")
	}
}

func TestParseZpoolScrubOutput_Running(t *testing.T) {
	output := `  pool: tank
 state: ONLINE
  scan: scrub in progress since Wed Apr 30 21:00:00 2026
	45.2G scanned at 1.5G/s, 30.1G issued at 800M/s, 45.2G total
	0B repaired, 66.67% done, 00:00:19 to go
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
	  sda       ONLINE       0     0     0

errors: No known data errors`

	progress := parseZpoolScrubOutput(output, "tank")
	if progress == nil {
		t.Fatal("expected non-nil progress for running scrub")
	}
	if progress.Status != ScrubStatusRunning {
		t.Errorf("expected running status, got %s", progress.Status)
	}
	if progress.PoolName != "tank" {
		t.Errorf("expected pool name 'tank', got '%s'", progress.PoolName)
	}
	if progress.Percent != 66.67 {
		t.Errorf("expected 66.67%%, got %.2f%%", progress.Percent)
	}
}

func TestParseZpoolScrubOutput_Completed(t *testing.T) {
	output := `  pool: tank
 state: ONLINE
  scan: scrub repaired 0B in 00:05:30 with 0 errors on Wed Apr 30 21:05:30 2026
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
	  sda       ONLINE       0     0     0

errors: No known data errors`

	progress := parseZpoolScrubOutput(output, "tank")
	if progress == nil {
		t.Fatal("expected non-nil progress for completed scrub")
	}
	if progress.Status != ScrubStatusCompleted {
		t.Errorf("expected completed status, got %s", progress.Status)
	}
	if progress.Percent != 100 {
		t.Errorf("expected 100%%, got %.2f%%", progress.Percent)
	}
}

func TestIsMainDisk(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"sda", true},
		{"sdb", true},
		{"nvme0n1", true},
		{"mmcblk0", true},
		{"vda", true},
		{"sda1", false},      // 分区
		{"nvme0n1p1", false}, // 分区
		{"loop0", false},     // loop设备
		{"dm-0", false},      // device mapper
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMainDisk(tt.name)
			if result != tt.expected {
				t.Errorf("isMainDisk(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestScrubResultFields(t *testing.T) {
	result := ScrubResult{
		ID:           "scrub-123",
		PoolName:     "tank",
		Status:       ScrubStatusCompleted,
		StartTime:    time.Now().Add(-5 * time.Minute),
		EndTime:      time.Now(),
		Duration:     "5m0s",
		BytesScanned: 1024 * 1024 * 1024,
		BytesIssued:  512 * 1024 * 1024,
		Errors:       0,
		Repairs:      0,
		ScanPercent:  100,
	}

	if result.ID != "scrub-123" {
		t.Errorf("expected ID 'scrub-123', got '%s'", result.ID)
	}
	if result.Status != ScrubStatusCompleted {
		t.Errorf("expected completed status, got %s", result.Status)
	}
	if result.BytesScanned != 1024*1024*1024 {
		t.Errorf("expected 1GB scanned, got %d", result.BytesScanned)
	}
}
