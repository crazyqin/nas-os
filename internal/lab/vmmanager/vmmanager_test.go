package vmmanager

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestCreateVM(t *testing.T) {
	m := NewManager()
	vm, err := m.CreateVM("test-vm", OSLinux, 4, 8192, 100)
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}
	if vm.Name != "test-vm" {
		t.Errorf("expected 'test-vm', got '%s'", vm.Name)
	}
	if vm.CPUCores != 4 {
		t.Errorf("expected 4 cores, got %d", vm.CPUCores)
	}
	if vm.State != VMStateStopped {
		t.Errorf("expected stopped, got '%s'", vm.State)
	}
}

func TestStartStopVM(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 2, 4096, 50)

	err := m.StartVM(vm.ID)
	if err != nil {
		t.Fatalf("StartVM failed: %v", err)
	}
	running, _ := m.GetVM(vm.ID)
	if running.State != VMStateRunning {
		t.Errorf("expected running, got '%s'", running.State)
	}

	err = m.StopVM(vm.ID, false)
	if err != nil {
		t.Fatalf("StopVM failed: %v", err)
	}
	stopped, _ := m.GetVM(vm.ID)
	if stopped.State != VMStateStopped {
		t.Errorf("expected stopped, got '%s'", stopped.State)
	}
}

func TestPauseResumeVM(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 2, 4096, 50)
	m.StartVM(vm.ID)

	m.PauseVM(vm.ID)
	paused, _ := m.GetVM(vm.ID)
	if paused.State != VMStatePaused {
		t.Errorf("expected paused, got '%s'", paused.State)
	}

	m.ResumeVM(vm.ID)
	resumed, _ := m.GetVM(vm.ID)
	if resumed.State != VMStateRunning {
		t.Errorf("expected running, got '%s'", resumed.State)
	}
}

func TestDeleteVM(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 2, 4096, 50)
	err := m.DeleteVM(vm.ID)
	if err != nil {
		t.Fatalf("DeleteVM failed: %v", err)
	}
	_, err = m.GetVM(vm.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestVMSnapshot(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 2, 4096, 50)

	snap, err := m.CreateVMSnapshot(vm.ID, "before-update")
	if err != nil {
		t.Fatalf("CreateVMSnapshot failed: %v", err)
	}
	if snap.Name != "before-update" {
		t.Errorf("expected 'before-update', got '%s'", snap.Name)
	}

	snaps, _ := m.GetVMSnapshots(vm.ID)
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestRollbackSnapshot(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 2, 4096, 50)
	snap1, _ := m.CreateVMSnapshot(vm.ID, "snap1")
	m.CreateVMSnapshot(vm.ID, "snap2")

	err := m.RollbackVMSnapshot(vm.ID, snap1.ID)
	if err != nil {
		t.Fatalf("RollbackVMSnapshot failed: %v", err)
	}

	snaps, _ := m.GetVMSnapshots(vm.ID)
	for _, s := range snaps {
		if s.ID == snap1.ID && !s.Current {
			t.Error("expected snap1 to be current after rollback")
		}
	}
}

func TestUpdateVM(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 2, 4096, 50)

	err := m.UpdateVM(vm.ID, 8, 16384)
	if err != nil {
		t.Fatalf("UpdateVM failed: %v", err)
	}
	updated, _ := m.GetVM(vm.ID)
	if updated.CPUCores != 8 {
		t.Errorf("expected 8 cores, got %d", updated.CPUCores)
	}
	if updated.MemMB != 16384 {
		t.Errorf("expected 16384 MB, got %d", updated.MemMB)
	}
}

func TestAttachGPU(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 4, 8192, 100)

	err := m.AttachGPU(vm.ID, "gpu-0")
	if err != nil {
		t.Fatalf("AttachGPU failed: %v", err)
	}
	updated, _ := m.GetVM(vm.ID)
	if !updated.GPUPassthrough {
		t.Error("expected GPU passthrough to be enabled")
	}
}

func TestAttachUSB(t *testing.T) {
	m := NewManager()
	vm, _ := m.CreateVM("test-vm", OSLinux, 2, 4096, 50)

	err := m.AttachUSB(vm.ID, "usb-001")
	if err != nil {
		t.Fatalf("AttachUSB failed: %v", err)
	}
	updated, _ := m.GetVM(vm.ID)
	if len(updated.USBDevices) != 1 {
		t.Errorf("expected 1 USB device, got %d", len(updated.USBDevices))
	}
}

func TestListVMs(t *testing.T) {
	m := NewManager()
	m.CreateVM("vm1", OSLinux, 2, 4096, 50)
	m.CreateVM("vm2", OSWindows, 4, 8192, 100)
	vms := m.ListVMs()
	if len(vms) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(vms))
	}
}
