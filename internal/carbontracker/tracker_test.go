package carbontracker

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewTrackerManager(t *testing.T) {
	cfg := &CarbonTrackerManagerConfig{
		MonitorInterval: 300,
		Enabled:        true,
	}
	logger, _ := zap.NewDevelopment()
	mgr := NewTrackerManager(logger, cfg)
	if mgr == nil {
		t.Fatal("NewTrackerManager returned nil")
	}
}

func TestNewTrackerManagerNilConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewTrackerManager(logger, nil)
	if mgr == nil {
		t.Fatal("NewTrackerManager with nil config returned nil")
	}
}

func TestGetEnergySources(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewTrackerManager(logger, nil)
	sources := mgr.GetEnergySources()
	if sources == nil {
		t.Fatal("GetEnergySources returned nil")
	}
}
