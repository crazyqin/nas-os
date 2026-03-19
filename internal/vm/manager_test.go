package vm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLibvirt(t *testing.T) {
	// This will return false if libvirt is not installed
	result := checkLibvirt()
	// We're just testing that the function runs without panicking
	t.Logf("checkLibvirt() = %v", result)
}

func TestManager_NewManager(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
	if manager.storagePath == "" {
		t.Error("storagePath should not be empty")
	}
	if manager.vms == nil {
		t.Error("vms map should be initialized")
	}
}

func TestManager_ListVMs(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	vms := manager.ListVMs()
	if vms == nil {
		t.Error("ListVMs should not return nil")
	}
}

func TestManager_GetVM(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Non-existent VM
	vm, err := manager.GetVM("nonexistent")
	if err == nil {
		t.Error("GetVM should return error for non-existent VM")
	}
	if vm != nil {
		t.Error("GetVM should return nil for non-existent VM")
	}
}

func TestManager_ListTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	templates := manager.ListTemplates()
	if templates == nil {
		t.Error("ListTemplates should not return nil")
	}
}

func TestManager_GetTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Non-existent template
	template, err := manager.GetTemplate("nonexistent")
	if err == nil {
		t.Error("GetTemplate should return error for non-existent template")
	}
	if template != nil {
		t.Error("GetTemplate should return nil for non-existent template")
	}
}

func TestManager_ListUSBDevices(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	devices, err := manager.ListUSBDevices()
	// This may fail on systems without lsusb
	t.Logf("ListUSBDevices() = %d devices, err = %v", len(devices), err)
}

func TestManager_ListPCIDevices(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	devices, err := manager.ListPCIDevices()
	// This may fail on systems without lspci
	t.Logf("ListPCIDevices() = %d devices, err = %v", len(devices), err)
}

func TestManager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Non-existent VM
	stats, err := manager.GetStats("nonexistent")
	if err == nil {
		t.Error("GetStats should return error for non-existent VM")
	}
	if stats != nil {
		t.Error("GetStats should return nil for non-existent VM")
	}
}

func TestManager_StartVM(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Non-existent VM
	err = manager.StartVM(context.Background(), "nonexistent")
	if err == nil {
		t.Error("StartVM should return error for non-existent VM")
	}
}

func TestManager_StopVM(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Non-existent VM
	err = manager.StopVM(context.Background(), "nonexistent", false)
	if err == nil {
		t.Error("StopVM should return error for non-existent VM")
	}
}

func TestManager_DeleteVM(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Non-existent VM
	err = manager.DeleteVM(context.Background(), "nonexistent", false)
	if err == nil {
		t.Error("DeleteVM should return error for non-existent VM")
	}
}

func TestManager_GetVNCConnection(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Non-existent VM
	conn, err := manager.GetVNCConnection("nonexistent")
	if err == nil {
		t.Error("GetVNCConnection should return error for non-existent VM")
	}
	if conn != nil {
		t.Error("GetVNCConnection should return nil for non-existent VM")
	}
}

func TestManager_ValidateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Empty config should fail
	err = manager.validateConfig(Config{})
	if err == nil {
		t.Error("validateConfig should fail for empty config")
	}

	// Valid config
	config := Config{
		Name:     "test-vm",
		Type:     TypeLinux,
		CPU:      2,
		Memory:   2048,
		DiskSize: 20,
		Network:  "bridge",
	}
	err = manager.validateConfig(config)
	if err != nil {
		t.Errorf("validateConfig should pass for valid config: %v", err)
	}
}

func TestManager_CreateVM(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Invalid config
	_, err = manager.CreateVM(context.Background(), Config{})
	if err == nil {
		t.Error("CreateVM should fail for empty config")
	}
}

func TestManager_SaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create a VM config file manually
	config := Config{
		Name:     "test-vm",
		Type:     TypeLinux,
		CPU:      2,
		Memory:   2048,
		DiskSize: 20,
		Network:  "bridge",
	}

	vm, err := manager.CreateVM(context.Background(), config)
	if err != nil {
		// Skip if qemu-img is not available
		t.Skipf("CreateVM failed (likely qemu-img not installed): %v", err)
	}

	// Verify VM was created
	if vm.ID == "" {
		t.Error("VM ID should be set")
	}
	if vm.Name != "test-vm" {
		t.Error("VM name mismatch")
	}

	// Verify we can get the VM
	loadedVM, err := manager.GetVM(vm.ID)
	if err != nil {
		t.Fatalf("GetVM failed: %v", err)
	}
	if loadedVM.Name != "test-vm" {
		t.Error("Loaded VM name mismatch")
	}
}

func TestManager_LoadConfig(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-vm.json")

	// Write invalid JSON
	os.WriteFile(configPath, []byte("invalid json"), 0644)

	_, err := loadConfig(configPath)
	if err == nil {
		t.Error("loadConfig should fail for invalid JSON")
	}
}
