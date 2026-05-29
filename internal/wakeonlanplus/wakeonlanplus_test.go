package wakeonlanplus

import (
	"context"
	"testing"
)

func TestNewWakeOnLANPlus(t *testing.T) {
	wol := NewWakeOnLANPlus()
	if wol == nil {
		t.Fatal("expected non-nil WakeOnLANPlus")
	}
}

func TestAddDevice(t *testing.T) {
	wol := NewWakeOnLANPlus()
	
	device := Device{
		Name:       "Test PC",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "192.168.1.100",
	}

	err := wol.AddDevice(device)
	if err != nil {
		t.Fatal(err)
	}

	devices := wol.ListDevices()
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}
}

func TestAddDeviceInvalidMAC(t *testing.T) {
	wol := NewWakeOnLANPlus()
	
	device := Device{
		Name:       "Test PC",
		MACAddress: "invalid",
	}

	err := wol.AddDevice(device)
	if err == nil {
		t.Error("expected error for invalid MAC")
	}
}

func TestRemoveDevice(t *testing.T) {
	wol := NewWakeOnLANPlus()
	
	wol.AddDevice(Device{
		Name:       "Test PC",
		MACAddress: "AA:BB:CC:DD:EE:FF",
	})

	wol.RemoveDevice("AA:BB:CC:DD:EE:FF")
	devices := wol.ListDevices()
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestGetDeviceStatus(t *testing.T) {
	wol := NewWakeOnLANPlus()
	
	wol.AddDevice(Device{
		Name:       "Test PC",
		MACAddress: "AA:BB:CC:DD:EE:FF",
	})

	status, err := wol.GetDeviceStatus("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatal(err)
	}
	if status != "unknown" {
		t.Errorf("expected status 'unknown', got '%s'", status)
	}
}

func TestStartStop(t *testing.T) {
	wol := NewWakeOnLANPlus()
	ctx := context.Background()

	err := wol.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = wol.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}

	wol.Stop()
}
