package containresmon

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:            true,
		MonitorIntervalSec: 5,
		HistoryRetentionH:  24,
		DefaultCPUWarning:  80,
		DefaultCPUCritical: 95,
		DefaultMemWarning:  80,
		DefaultMemCritical: 95,
		AlertCooldownMin:   5,
	}
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestRegisterContainer(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	container := &Container{
		ID:     "container-1",
		Name:   "nginx",
		Image:  "nginx:latest",
		Status: ContainerRunning,
	}
	manager.RegisterContainer(container)

	containers := manager.ListContainers()
	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}
}

func TestRecordUsage(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	container := &Container{ID: "c1", Name: "app", Status: ContainerRunning}
	manager.RegisterContainer(container)

	usage := &ResourceUsage{
		ContainerID: "c1",
		CPUPercent:  45.5,
		MemUsage:    1024 * 1024 * 512,
		MemLimit:    1024 * 1024 * 1024,
		MemPercent:  50.0,
		NetRx:       1024,
		NetTx:       512,
	}
	manager.RecordUsage(usage)

	history := manager.GetUsageHistory("c1")
	if len(history) != 1 {
		t.Errorf("expected 1 usage record, got %d", len(history))
	}
}

func TestResolveAlert(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	alert := &ResourceAlert{
		ID:          "alert-1",
		ContainerID: "c1",
		Type:        AlertCPUHigh,
		Message:     "CPU usage high",
	}
	manager.alerts = append(manager.alerts, alert)

	if err := manager.ResolveAlert("alert-1"); err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}
	if !alert.Resolved {
		t.Error("expected alert to be resolved")
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	stats := manager.GetStats()
	if stats.TotalContainers != 0 {
		t.Errorf("expected 0 containers, got %d", stats.TotalContainers)
	}
}
