package carbontracker

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := &Config{
		UpdateInterval: 5 * time.Minute,
		Enabled:        true,
	}
	mgr := NewManager(cfg)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewManagerNilConfig(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("NewManager with nil config returned nil")
	}
}

func TestGetCarbonMetrics(t *testing.T) {
	mgr := NewManager(&Config{Enabled: true})
	metrics := mgr.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}
}

func TestUpdatePowerUsage(t *testing.T) {
	mgr := NewManager(&Config{Enabled: true})
	err := mgr.UpdatePowerUsage("server-1", 150.5)
	if err != nil {
		t.Fatalf("UpdatePowerUsage failed: %v", err)
	}
}

func TestGetCarbonReport(t *testing.T) {
	mgr := NewManager(&Config{Enabled: true})
	report := mgr.GetReport()
	if report == nil {
		t.Fatal("GetReport returned nil")
	}
}
