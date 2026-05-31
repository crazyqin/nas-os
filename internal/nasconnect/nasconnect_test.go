package nasconnect

import (
	"testing"
)

type testLogger struct{}

func (l *testLogger) Info(msg string, args ...interface{})  {}
func (l *testLogger) Error(msg string, args ...interface{}) {}
func (l *testLogger) Debug(msg string, args ...interface{}) {}

func TestManager_AddDevice(t *testing.T) {
	mgr := NewManager(&testLogger{})
	defer mgr.Stop()

	device := &NASDevice{
		Name: "TestNAS",
		Host: "192.168.1.100",
		Port: 5000,
	}

	err := mgr.AddDevice(device)
	if err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}

	if device.ID == "" {
		t.Fatal("Device ID should not be empty")
	}

	devices := mgr.ListDevices("")
	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}

	if devices[0].Name != "TestNAS" {
		t.Errorf("Expected device name 'TestNAS', got '%s'", devices[0].Name)
	}
}

func TestManager_RemoveDevice(t *testing.T) {
	mgr := NewManager(&testLogger{})
	defer mgr.Stop()

	device := &NASDevice{
		Name: "TestNAS",
		Host: "192.168.1.100",
		Port: 5000,
	}

	mgr.AddDevice(device)

	err := mgr.RemoveDevice(device.ID)
	if err != nil {
		t.Fatalf("RemoveDevice failed: %v", err)
	}

	devices := mgr.ListDevices("")
	if len(devices) != 0 {
		t.Fatalf("Expected 0 devices, got %d", len(devices))
	}
}

func TestManager_Connect(t *testing.T) {
	mgr := NewManager(&testLogger{})
	defer mgr.Stop()

	device := &NASDevice{
		Name: "TestNAS",
		Host: "192.168.1.100",
		Port: 5000,
	}

	mgr.AddDevice(device)

	conn, err := mgr.Connect(device.ID)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if conn.Status != ConnectionStatusConnected {
		t.Errorf("Expected status 'connected', got '%s'", conn.Status)
	}
}

func TestManager_Groups(t *testing.T) {
	mgr := NewManager(&testLogger{})
	defer mgr.Stop()

	group := &DeviceGroup{
		Name: "Production",
	}

	err := mgr.CreateGroup(group)
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	groups := mgr.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}
}

func TestManager_Stats(t *testing.T) {
	mgr := NewManager(&testLogger{})
	defer mgr.Stop()

	device := &NASDevice{
		Name: "TestNAS",
		Host: "192.168.1.100",
		Port: 5000,
	}

	mgr.AddDevice(device)

	stats := mgr.GetStats()
	if stats.TotalDevices != 1 {
		t.Errorf("Expected 1 device, got %d", stats.TotalDevices)
	}
}
