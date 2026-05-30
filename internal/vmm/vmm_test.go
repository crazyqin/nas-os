// Package vmm 测试文件
package vmm

import (
	"context"
	"testing"
)

func TestManager_ListVMs(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	vms, err := m.ListVMs(ctx, true)
	if err != nil {
		t.Fatalf("ListVMs failed: %v", err)
	}

	if len(vms) != 0 {
		t.Errorf("Expected 0 VMs, got %d", len(vms))
	}
}

func TestManager_CreateVM(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	vm, err := m.CreateVM(ctx, "test-vm", "linux")
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	if vm.Name != "test-vm" {
		t.Errorf("Expected name 'test-vm', got '%s'", vm.Name)
	}

	if vm.OSType != "linux" {
		t.Errorf("Expected OS type 'linux', got '%s'", vm.OSType)
	}
}

func TestManager_ListTemplates(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	templates, err := m.ListTemplates(ctx)
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

	if stats["total_vms"] != 0 {
		t.Errorf("Expected 0 VMs, got %v", stats["total_vms"])
	}
}
