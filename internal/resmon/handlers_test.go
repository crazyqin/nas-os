package resmon

import (
	"testing"
	"time"
)

func TestRecordAndGetLatest(t *testing.T) {
	mgr := NewManager()
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
		CPU:       CPUUsage{TotalPercent: 45.5, CoreCount: 4},
		Memory:    MemoryUsage{UsedPercent: 60.0, TotalBytes: 8 * 1024 * 1024 * 1024},
	}
	mgr.RecordMetrics(metrics)
	latest := mgr.GetLatest()
	if latest == nil {
		t.Fatal("expected non-nil latest")
	}
	if latest.CPU.TotalPercent != 45.5 {
		t.Errorf("expected 45.5%%, got %.1f%%", latest.CPU.TotalPercent)
	}
}

func TestGetLatestEmpty(t *testing.T) {
	mgr := NewManager()
	if mgr.GetLatest() != nil {
		t.Error("expected nil for empty manager")
	}
}

func TestHistoryRetention(t *testing.T) {
	mgr := NewManager()
	mgr.maxHistory = 5
	for i := 0; i < 10; i++ {
		mgr.RecordMetrics(&SystemMetrics{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			CPU:       CPUUsage{TotalPercent: float64(i * 10)},
		})
	}
	history := mgr.GetHistory(24)
	if len(history) != 5 {
		t.Errorf("expected 5 history points, got %d", len(history))
	}
}

func TestAlertsCPU(t *testing.T) {
	mgr := NewManager()
	mgr.config.CPUThresholdPct = 80.0
	mgr.RecordMetrics(&SystemMetrics{
		Timestamp: time.Now(),
		CPU:       CPUUsage{TotalPercent: 95.0},
		Memory:    MemoryUsage{UsedPercent: 50.0},
	})
	alerts := mgr.GetAlerts(false)
	found := false
	for _, a := range alerts {
		if a.Resource == "cpu" {
			found = true
			if a.Level != AlertWarning {
				t.Errorf("expected warning level, got %s", a.Level)
			}
		}
	}
	if !found {
		t.Error("expected CPU alert")
	}
}

func TestAlertsMemory(t *testing.T) {
	mgr := NewManager()
	mgr.config.MemThresholdPct = 80.0
	mgr.RecordMetrics(&SystemMetrics{
		Timestamp: time.Now(),
		CPU:       CPUUsage{TotalPercent: 50.0},
		Memory:    MemoryUsage{UsedPercent: 95.0},
	})
	alerts := mgr.GetAlerts(false)
	found := false
	for _, a := range alerts {
		if a.Resource == "memory" {
			found = true
		}
	}
	if !found {
		t.Error("expected memory alert")
	}
}

func TestAlertsDisk(t *testing.T) {
	mgr := NewManager()
	mgr.config.DiskThresholdPct = 85.0
	mgr.RecordMetrics(&SystemMetrics{
		Timestamp: time.Now(),
		CPU:       CPUUsage{TotalPercent: 50.0},
		Memory:    MemoryUsage{UsedPercent: 50.0},
		Disks:     []DiskUsage{{MountPoint: "/", UsedPercent: 95.0}},
	})
	alerts := mgr.GetAlerts(false)
	found := false
	for _, a := range alerts {
		if a.Resource == "disk:/" {
			found = true
			if a.Level != AlertCritical {
				t.Errorf("expected critical level, got %s", a.Level)
			}
		}
	}
	if !found {
		t.Error("expected disk alert")
	}
}

func TestAckAlert(t *testing.T) {
	mgr := NewManager()
	mgr.config.CPUThresholdPct = 80.0
	mgr.RecordMetrics(&SystemMetrics{
		Timestamp: time.Now(),
		CPU:       CPUUsage{TotalPercent: 95.0},
		Memory:    MemoryUsage{UsedPercent: 50.0},
	})
	alerts := mgr.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("expected at least 1 alert")
	}
	id := alerts[0].ID
	if !mgr.AckAlert(id) {
		t.Error("expected AckAlert to return true")
	}
	unacked := mgr.GetAlerts(true)
	for _, a := range unacked {
		if a.ID == id {
			t.Error("expected alert to be acked")
		}
	}
}

func TestAckAlertNotFound(t *testing.T) {
	mgr := NewManager()
	if mgr.AckAlert("nonexistent") {
		t.Error("expected false for nonexistent alert")
	}
}

func TestConfig(t *testing.T) {
	mgr := NewManager()
	cfg := mgr.GetConfig()
	if cfg.CollectIntervalS != 30 {
		t.Errorf("expected 30s interval, got %d", cfg.CollectIntervalS)
	}
	cfg.CollectIntervalS = 10
	mgr.UpdateConfig(cfg)
	if mgr.GetConfig().CollectIntervalS != 10 {
		t.Errorf("expected 10s after update, got %d", mgr.GetConfig().CollectIntervalS)
	}
}

func TestAddAlertRule(t *testing.T) {
	mgr := NewManager()
	mgr.AddAlertRule(AlertRule{
		ID:        "rule-1",
		Resource:  "cpu",
		Threshold: 90.0,
		Level:     AlertWarning,
		Enabled:   true,
	})
	// rules are stored; no getter but no panic either
}
