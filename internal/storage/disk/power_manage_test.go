// Package disk provides disk power management tests
package disk

import (
	"context"
	"testing"
	"time"
)

func TestDefaultPowerPolicy(t *testing.T) {
	policy := DefaultPowerPolicy()
	
	if policy.IdleTimeout != 5*time.Minute {
		t.Errorf("expected IdleTimeout 5min, got %v", policy.IdleTimeout)
	}
	
	if policy.StandbyTimeout != 30*time.Minute {
		t.Errorf("expected StandbyTimeout 30min, got %v", policy.StandbyTimeout)
	}
	
	if policy.APMLevel != 128 {
		t.Errorf("expected APMLevel 128, got %d", policy.APMLevel)
	}
}

func TestNewDiskPowerManager(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	
	if manager == nil {
		t.Fatal("expected manager, got nil")
	}
	
	if manager.policy == nil {
		t.Fatal("expected default policy, got nil")
	}
}

func TestRegisterDisk(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	
	err := manager.RegisterDisk("disk-001")
	if err != nil {
		t.Fatalf("failed to register disk: %v", err)
	}
	
	// Duplicate registration should fail
	err = manager.RegisterDisk("disk-001")
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestUnregisterDisk(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	
	manager.RegisterDisk("disk-001")
	
	err := manager.UnregisterDisk("disk-001")
	if err != nil {
		t.Fatalf("failed to unregister disk: %v", err)
	}
	
	// Unregistering non-existent disk should fail
	err = manager.UnregisterDisk("disk-001")
	if err == nil {
		t.Error("expected error for non-existent disk")
	}
}

func TestUpdateActivity(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	manager.RegisterDisk("disk-001")
	
	// Simulate disk in standby
	state, _ := manager.GetDiskPowerState("disk-001")
	state.CurrentMode = PowerModeStandby
	state.SpinupCount = 0
	
	err := manager.UpdateActivity("disk-001")
	if err != nil {
		t.Fatalf("failed to update activity: %v", err)
	}
	
	state, _ = manager.GetDiskPowerState("disk-001")
	
	if state.CurrentMode != PowerModeActive {
		t.Errorf("expected Active mode, got %s", state.CurrentMode)
	}
	
	if state.SpinupCount != 1 {
		t.Errorf("expected SpinupCount 1, got %d", state.SpinupCount)
	}
}

func TestGetPowerStats(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	
	manager.RegisterDisk("disk-001")
	manager.RegisterDisk("disk-002")
	manager.RegisterDisk("disk-003")
	
	// Set different states
	state1, _ := manager.GetDiskPowerState("disk-001")
	state1.CurrentMode = PowerModeActive
	
	state2, _ := manager.GetDiskPowerState("disk-002")
	state2.CurrentMode = PowerModeStandby
	state2.SpindownCount = 5
	
	state3, _ := manager.GetDiskPowerState("disk-003")
	state3.CurrentMode = PowerModeIdle
	
	stats := manager.GetPowerStats()
	
	if stats.TotalDisks != 3 {
		t.Errorf("expected TotalDisks 3, got %d", stats.TotalDisks)
	}
	
	if stats.ActiveDisks != 1 {
		t.Errorf("expected ActiveDisks 1, got %d", stats.ActiveDisks)
	}
	
	if stats.StandbyDisks != 1 {
		t.Errorf("expected StandbyDisks 1, got %d", stats.StandbyDisks)
	}
	
	if stats.IdleDisks != 1 {
		t.Errorf("expected IdleDisks 1, got %d", stats.IdleDisks)
	}
}

func TestSetPowerPolicy(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	
	newPolicy := &PowerPolicy{
		IdleTimeout:    10 * time.Minute,
		StandbyTimeout: 60 * time.Minute,
		EnableAPM:      false,
	}
	
	err := manager.SetPowerPolicy(newPolicy)
	if err != nil {
		t.Fatalf("failed to set policy: %v", err)
	}
	
	policy := manager.GetPowerPolicy()
	if policy.IdleTimeout != 10*time.Minute {
		t.Errorf("expected IdleTimeout 10min, got %v", policy.IdleTimeout)
	}
}

func TestSpinupDisk(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	manager.RegisterDisk("disk-001")
	
	state, _ := manager.GetDiskPowerState("disk-001")
	state.CurrentMode = PowerModeStandby
	
	err := manager.SpinupDisk("disk-001")
	if err != nil {
		t.Fatalf("failed to spinup disk: %v", err)
	}
	
	state, _ = manager.GetDiskPowerState("disk-001")
	
	if state.CurrentMode != PowerModeActive {
		t.Errorf("expected Active mode after spinup, got %s", state.CurrentMode)
	}
}

func TestStartPowerMonitoring(t *testing.T) {
	manager := NewDiskPowerManager(nil, "/tmp/test")
	manager.RegisterDisk("disk-001")
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	manager.StartPowerMonitoring(ctx)
	
	// Wait for context to cancel
	time.Sleep(3 * time.Second)
}