package filecache

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	cfg := &CacheConfig{
		Enabled:          true,
		MemoryMaxEntries: 1000,
		MemoryMaxSize:    1024 * 1024 * 100, // 100MB
	}
	logger := zap.NewNop()
	m := NewManager(logger, cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := &CacheConfig{
		Enabled:          true,
		MemoryMaxEntries: 1000,
		MemoryMaxSize:    1024 * 1024 * 100,
	}
	logger := zap.NewNop()
	m := NewManager(logger, cfg)

	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("Manager should be running")
	}

	err = m.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("Manager should be stopped")
	}
}
