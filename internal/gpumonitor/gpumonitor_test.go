package gpumonitor

import (
	"testing"
)

func TestNewMonitor(t *testing.T) {
	m := NewMonitor()
	if m == nil {
		t.Fatal("NewMonitor returned nil")
	}
}

func TestGetGPUsEmpty(t *testing.T) {
	m := NewMonitor()
	gpus := m.GetGPUs()
	if len(gpus) != 0 {
		t.Errorf("expected 0 gpus, got %d", len(gpus))
	}
}

func TestGetGPUNotFound(t *testing.T) {
	m := NewMonitor()
	_, err := m.GetGPU("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent gpu")
	}
}

func TestGetGPUSummary(t *testing.T) {
	m := NewMonitor()
	summary := m.GetGPUSummary()
	if summary["total"] != 0 {
		t.Errorf("expected total=0, got %v", summary["total"])
	}
	if summary["healthy"] != 0 {
		t.Errorf("expected healthy=0, got %v", summary["healthy"])
	}
}

func TestGetAlertsEmpty(t *testing.T) {
	m := NewMonitor()
	alerts := m.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestGetGPUProcessesNotFound(t *testing.T) {
	m := NewMonitor()
	_, err := m.GetGPUProcesses("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent gpu")
	}
}

func TestGetGPUHistoryNotFound(t *testing.T) {
	m := NewMonitor()
	_, err := m.GetGPUHistory("nonexistent", 24)
	if err == nil {
		t.Fatal("expected error for nonexistent gpu")
	}
}

func TestSetPowerLimitNotFound(t *testing.T) {
	m := NewMonitor()
	err := m.SetPowerLimit("nonexistent", 200)
	if err == nil {
		t.Fatal("expected error for nonexistent gpu")
	}
}
