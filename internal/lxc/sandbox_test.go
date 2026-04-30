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

func TestDefaultTemplates(t *testing.T) {
	mgr := NewLXCSandboxManager(nil)
	tmpls := mgr.templates
	if len(tmpls) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(tmpls))
	}
	if _, ok := tmpls["ubuntu-24.04"]; !ok {
		t.Error("missing ubuntu-24.04 template")
	}
	if _, ok := tmpls["alpine-3.20"]; !ok {
		t.Error("missing alpine-3.20 template")
	}
}

func TestListSandboxesEmpty(t *testing.T) {
	mgr := NewLXCSandboxManager(nil)
	sandboxes := mgr.ListSandboxes()
	if len(sandboxes) != 0 {
		t.Errorf("expected 0 sandboxes, got %d", len(sandboxes))
	}
}
