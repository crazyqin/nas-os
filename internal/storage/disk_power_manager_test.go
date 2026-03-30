package storage

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewDiskPowerManager(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	if len(manager.configs) != 0 {
		t.Errorf("expected 0 configs, got %d", len(manager.configs))
	}
}

func TestRegisterDisk(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{
		DevicePath: "/dev/sda",
	}

	err := manager.RegisterDisk(config)
	if err != nil {
		t.Fatalf("failed to register disk: %v", err)
	}

	if len(manager.configs) != 1 {
		t.Errorf("expected 1 config, got %d", len(manager.configs))
	}

	// Check defaults were applied
	registered := manager.configs["/dev/sda"]
	if registered.IdleTimeoutSeconds != 300 {
		t.Errorf("expected idle timeout 300, got %d", registered.IdleTimeoutSeconds)
	}
}

func TestRegisterDiskEmptyPath(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{}
	err := manager.RegisterDisk(config)
	if err == nil {
		t.Error("expected error for empty device path")
	}
}

func TestUnregisterDisk(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{DevicePath: "/dev/sda"}
	manager.RegisterDisk(config)

	manager.UnregisterDisk("/dev/sda")

	if len(manager.configs) != 0 {
		t.Errorf("expected 0 configs after unregister, got %d", len(manager.configs))
	}
}

func TestRecordAccess(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{DevicePath: "/dev/sda"}
	manager.RegisterDisk(config)

	// Set disk to sleeping state
	manager.states["/dev/sda"] = DiskPowerSleeping

	manager.RecordAccess("/dev/sda")

	// Should have queued a wake-up
	if manager.lastAccess["/dev/sda"].IsZero() {
		t.Error("expected last access time to be set")
	}
}

func TestGetState(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{DevicePath: "/dev/sda"}
	manager.RegisterDisk(config)

	state, err := manager.GetState("/dev/sda")
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}

	if state != DiskPowerActive {
		t.Errorf("expected active state, got %s", state)
	}
}

func TestGetStateUnregistered(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	_, err := manager.GetState("/dev/nonexistent")
	if err == nil {
		t.Error("expected error for unregistered disk")
	}
}

func TestSetState(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{DevicePath: "/dev/sda"}
	manager.RegisterDisk(config)

	err := manager.SetState("/dev/sda", DiskPowerStandby, "test")
	if err != nil {
		t.Fatalf("failed to set state: %v", err)
	}

	state, _ := manager.GetState("/dev/sda")
	if state != DiskPowerStandby {
		t.Errorf("expected standby state, got %s", state)
	}
}

func TestStateChangeCallback(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	var callbackCalled bool
	var receivedEvent DiskPowerEvent
	var mu sync.Mutex

	manager.SetCallbacks(
		func(event DiskPowerEvent) {
			mu.Lock()
			defer mu.Unlock()
			callbackCalled = true
			receivedEvent = event
		},
		nil,
	)

	config := &DiskPowerConfig{DevicePath: "/dev/sda"}
	manager.RegisterDisk(config)
	manager.SetState("/dev/sda", DiskPowerSleeping, "test_callback")

	mu.Lock()
	defer mu.Unlock()
	if !callbackCalled {
		t.Error("expected callback to be called")
	}
	if receivedEvent.NewState != DiskPowerSleeping {
		t.Errorf("expected sleeping state in event, got %s", receivedEvent.NewState)
	}
}

func TestGetEvents(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{DevicePath: "/dev/sda"}
	manager.RegisterDisk(config)

	manager.SetState("/dev/sda", DiskPowerIdle, "test1")
	manager.SetState("/dev/sda", DiskPowerStandby, "test2")
	manager.SetState("/dev/sda", DiskPowerSleeping, "test3")

	events := manager.GetEvents(2)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestGetAllStates(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	manager.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sda"})
	manager.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sdb"})

	states := manager.GetAllStates()
	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}
}

func TestGetDiskCount(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	manager.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sda"})
	manager.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sdb"})
	manager.RegisterDisk(&DiskPowerConfig{DevicePath: "/dev/sdc"})

	if count := manager.GetDiskCount(); count != 3 {
		t.Errorf("expected 3 disks, got %d", count)
	}
}

func TestGetConfig(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)

	config := &DiskPowerConfig{
		DevicePath:           "/dev/sda",
		IdleTimeoutSeconds:   600,
		StandbyTimeoutSeconds: 1200,
	}
	manager.RegisterDisk(config)

	retrieved, err := manager.GetConfig("/dev/sda")
	if err != nil {
		t.Fatalf("failed to get config: %v", err)
	}

	if retrieved.IdleTimeoutSeconds != 600 {
		t.Errorf("expected idle timeout 600, got %d", retrieved.IdleTimeoutSeconds)
	}
}

func TestWakeUpDisk(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)
	manager.Start()
	defer manager.Stop()

	config := &DiskPowerConfig{
		DevicePath:       "/dev/sda",
		DelayWakeUpMs:   10, // Fast for testing
	}
	manager.RegisterDisk(config)

	// Set to sleeping
	manager.SetState("/dev/sda", DiskPowerSleeping, "test")

	// Wake up
	err := manager.WakeUpDisk("/dev/sda")
	if err != nil {
		t.Fatalf("failed to wake up disk: %v", err)
	}

	// Give time for async operations
	time.Sleep(50 * time.Millisecond)

	state, _ := manager.GetState("/dev/sda")
	if state != DiskPowerActive {
		t.Errorf("expected active state after wake-up, got %s", state)
	}
}

func TestStop(t *testing.T) {
	ctx := context.Background()
	manager := NewDiskPowerManager(ctx)
	manager.Start()

	// Should not block
	manager.Stop()

	// Verify context is cancelled
	select {
	case <-manager.ctx.Done():
		// Expected
	default:
		t.Error("expected context to be cancelled after Stop()")
	}
}