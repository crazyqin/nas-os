package nvmeof

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewMultipathManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultFailoverConfig()

	mm := NewMultipathManager(logger, config)
	if mm == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestDefaultFailoverConfig(t *testing.T) {
	config := DefaultFailoverConfig()

	if config.HealthCheckInterval != 5*time.Second {
		t.Errorf("expected 5s interval, got %v", config.HealthCheckInterval)
	}
	if config.FailoverThreshold != 3 {
		t.Errorf("expected threshold 3, got %d", config.FailoverThreshold)
	}
	if !config.AutoFailback {
		t.Error("expected AutoFailback true")
	}
}

func TestMultipathManager_AddPath(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mm := NewMultipathManager(logger, DefaultFailoverConfig())

	path := PathInfo{
		HostNQN:   "nqn.2024-01.test:host1",
		TrAddr:    "192.168.1.100",
		TrSvcID:   "4420",
		Transport: "tcp",
	}

	err := mm.AddPath(nil, "nqn.2024-01.test:subsystem1", path)
	if err != nil {
		t.Fatal(err)
	}

	paths := mm.GetPaths("nqn.2024-01.test:subsystem1")
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].State != "connecting" {
		t.Errorf("expected connecting, got %s", paths[0].State)
	}
}

func TestMultipathManager_RemovePath(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mm := NewMultipathManager(logger, DefaultFailoverConfig())

	path := PathInfo{
		HostNQN:   "nqn.2024-01.test:host1",
		TrAddr:    "192.168.1.100",
		Transport: "tcp",
	}

	mm.AddPath(nil, "sub1", path)
	paths := mm.GetPaths("sub1")
	if len(paths) != 1 {
		t.Fatal("expected 1 path")
	}

	err := mm.RemovePath(nil, "sub1", paths[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	paths = mm.GetPaths("sub1")
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestMultipathManager_RemovePath_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mm := NewMultipathManager(logger, DefaultFailoverConfig())

	err := mm.RemovePath(nil, "sub1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestMultipathManager_GetActivePath_Empty(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mm := NewMultipathManager(logger, DefaultFailoverConfig())

	path := mm.GetActivePath("nonexistent")
	if path != nil {
		t.Error("expected nil for empty subsystem")
	}
}

func TestPathInfo_Fields(t *testing.T) {
	path := PathInfo{
		ID:        "test-path",
		HostNQN:   "nqn.test",
		Subsystem: "sub1",
		TrAddr:    "10.0.0.1",
		TrSvcID:   "4420",
		Transport: "rdma",
		State:     "live",
		Priority:  1,
	}

	if path.Transport != "rdma" {
		t.Errorf("expected rdma, got %s", path.Transport)
	}
	if path.State != "live" {
		t.Errorf("expected live, got %s", path.State)
	}
}

func TestNewFailoverManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := DefaultFailoverConfig()

	fm := NewFailoverManager(logger, config)
	if fm == nil {
		t.Fatal("expected non-nil failover manager")
	}
}

func TestFailoverManager_TriggerFailover(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	fm := NewFailoverManager(logger, DefaultFailoverConfig())

	fm.TriggerFailover("sub1", "path1", "test_reason")

	events := fm.GetFailoverEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Subsystem != "sub1" {
		t.Errorf("expected sub1, got %s", events[0].Subsystem)
	}
	if events[0].Reason != "test_reason" {
		t.Errorf("expected test_reason, got %s", events[0].Reason)
	}
}

func TestMultipathManager_MultiplePaths(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mm := NewMultipathManager(logger, DefaultFailoverConfig())

	paths := []PathInfo{
		{TrAddr: "10.0.0.1", Transport: "tcp"},
		{TrAddr: "10.0.0.2", Transport: "tcp"},
		{TrAddr: "10.0.0.3", Transport: "rdma"},
	}

	for _, p := range paths {
		mm.AddPath(nil, "sub1", p)
	}

	result := mm.GetPaths("sub1")
	if len(result) != 3 {
		t.Errorf("expected 3 paths, got %d", len(result))
	}
}
