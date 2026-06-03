package lxcdashboard

import (
	"testing"
)

func TestContainerLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dashboard := NewLXCDashboard(tmpDir)

	container, err := dashboard.CreateContainer("test-nginx", "nginx:alpine", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if container.Name != "test-nginx" {
		t.Fatal("name mismatch")
	}

	err = dashboard.StartContainer(container.ID)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	c, ok := dashboard.GetContainer(container.ID)
	if !ok || c.State != StateRunning {
		t.Fatal("container should be running")
	}

	err = dashboard.StopContainer(container.ID)
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	c, _ = dashboard.GetContainer(container.ID)
	if c.State != StateStopped {
		t.Fatal("container should be stopped")
	}
}

func TestContainerStats(t *testing.T) {
	tmpDir := t.TempDir()
	dashboard := NewLXCDashboard(tmpDir)

	c1, _ := dashboard.CreateContainer("c1", "nginx", nil)
	dashboard.CreateContainer("c2", "postgres", nil)

	dashboard.StartContainer(c1.ID)

	stats := dashboard.GetStats()
	if stats.TotalContainers != 2 {
		t.Fatalf("expected 2 containers, got %d", stats.TotalContainers)
	}
	if stats.RunningContainers != 1 {
		t.Fatalf("expected 1 running, got %d", stats.RunningContainers)
	}
}

func TestTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	dashboard := NewLXCDashboard(tmpDir)

	templates := dashboard.GetTemplates()
	if len(templates) == 0 {
		t.Fatal("expected templates")
	}
}

func TestContainerQuota(t *testing.T) {
	tmpDir := t.TempDir()
	dashboard := NewLXCDashboard(tmpDir)
	dashboard.quotas.MaxContainers = 2

	_, err := dashboard.CreateContainer("c1", "img", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	_, err = dashboard.CreateContainer("c2", "img", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	_, err = dashboard.CreateContainer("c3", "img", nil)
	if err == nil {
		t.Fatal("should fail when quota exceeded")
	}
}
