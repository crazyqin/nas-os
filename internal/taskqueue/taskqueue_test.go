package taskqueue

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := &Config{Enabled: true, PollInterval: 60 * time.Second}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := &Config{Enabled: true, PollInterval: 60 * time.Second}
	m := NewManager(cfg)
	
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
