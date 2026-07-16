package lxc

import (
	"testing"
)

func TestNewLXCSandboxManager(t *testing.T) {
	mgr := NewLXCSandboxManager(nil)
	if mgr == nil {
		t.Fatal("NewLXCSandboxManager returned nil")
	}
	if mgr.config.MaxSandboxes != 50 {
		t.Errorf("expected max 50, got %d", mgr.config.MaxSandboxes)
	}
	if mgr.config.DefaultCPU != 2 {
		t.Errorf("expected default cpu 2, got %d", mgr.config.DefaultCPU)
	}
}

func TestListSandboxesEmpty(t *testing.T) {
	mgr := NewLXCSandboxManager(nil)
	sandboxes := mgr.ListSandboxes()
	if len(sandboxes) != 0 {
		t.Errorf("expected 0 sandboxes, got %d", len(sandboxes))
	}
}
