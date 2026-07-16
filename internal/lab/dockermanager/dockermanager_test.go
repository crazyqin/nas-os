// Package dockermanager 测试文件
package dockermanager

import (
	"context"
	"testing"
)

func TestManager_ListContainers(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	containers, err := m.ListContainers(ctx, true)
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}

	if len(containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(containers))
	}
}

func TestManager_CreateContainer(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	container, err := m.CreateContainer(ctx, "test-nginx", "nginx", "latest")
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}

	if container.Name != "test-nginx" {
		t.Errorf("Expected name 'test-nginx', got '%s'", container.Name)
	}

	if container.Image != "nginx" {
		t.Errorf("Expected image 'nginx', got '%s'", container.Image)
	}
}

func TestManager_ListTemplates(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	templates, err := m.ListTemplates(ctx, "")
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	if len(templates) < 3 {
		t.Errorf("Expected at least 3 templates, got %d", len(templates))
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	stats, err := m.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["containers_running"] != 0 {
		t.Errorf("Expected 0 running containers, got %v", stats["containers_running"])
	}
}
