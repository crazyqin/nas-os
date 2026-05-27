package smarthome

import (
	"testing"
)

func testManager() *Manager {
	return NewManager(Config{
		MQTTBroker: "localhost:1883",
		MaxEvents:  100,
	})
}

func TestNewManager(t *testing.T) {
	m := testManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.devices == nil {
		t.Error("devices map not initialized")
	}
}

func TestAddDevice(t *testing.T) {
	m := testManager()
	dev := &Device{
		ID:       "light-001",
		Name:     "客厅灯",
		Type:     DeviceTypeLight,
		Protocol: ProtocolZigbee,
		RoomID:   "room-1",
	}
	if err := m.AddDevice(dev); err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}
	got, err := m.GetDevice("light-001")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if got.Name != "客厅灯" {
		t.Errorf("expected name '客厅灯', got '%s'", got.Name)
	}
}

func TestAddDuplicateDevice(t *testing.T) {
	m := testManager()
	dev := &Device{ID: "d1", Name: "test", Type: DeviceTypeSensor, Protocol: ProtocolMQTT}
	m.AddDevice(dev)
	if err := m.AddDevice(dev); err == nil {
		t.Error("expected error for duplicate device")
	}
}

func TestDeleteDevice(t *testing.T) {
	m := testManager()
	dev := &Device{ID: "d1", Name: "test", Type: DeviceTypeSensor, Protocol: ProtocolMQTT}
	m.AddDevice(dev)
	if err := m.DeleteDevice("d1"); err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}
	if _, err := m.GetDevice("d1"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestListDevices(t *testing.T) {
	m := testManager()
	m.AddDevice(&Device{ID: "d1", Name: "灯", Type: DeviceTypeLight, Protocol: ProtocolMQTT})
	m.AddDevice(&Device{ID: "d2", Name: "传感器", Type: DeviceTypeSensor, Protocol: ProtocolMQTT})
	list := m.ListDevices()
	if len(list) != 2 {
		t.Errorf("expected 2 devices, got %d", len(list))
	}
}

func TestDeviceCount(t *testing.T) {
	m := testManager()
	m.AddDevice(&Device{ID: "d1", Name: "灯", Type: DeviceTypeLight, Protocol: ProtocolMQTT, Status: DeviceStatusOnline})
	m.AddDevice(&Device{ID: "d2", Name: "传感器", Type: DeviceTypeSensor, Protocol: ProtocolMQTT, Status: DeviceStatusOffline})
	total, online, offline := m.GetDeviceCount()
	if total != 2 {
		t.Errorf("expected 2 total, got %d", total)
	}
	if online != 1 {
		t.Errorf("expected 1 online, got %d", online)
	}
	if offline != 1 {
		t.Errorf("expected 1 offline, got %d", offline)
	}
}

func TestAddRoom(t *testing.T) {
	m := testManager()
	room := &Room{ID: "room-1", Name: "客厅"}
	if err := m.AddRoom(room); err != nil {
		t.Fatalf("AddRoom failed: %v", err)
	}
	if _, err := m.GetRoom("room-1"); err != nil {
		t.Fatalf("GetRoom failed: %v", err)
	}
}

func TestScene(t *testing.T) {
	scene := Scene{
		ID:      "scene-001",
		Name:    "回家模式",
		Enabled: true,
		Trigger: Trigger{Type: TriggerTypeTime, TimeStr: "18:00"},
		Actions: []Action{{Type: ActionTypeDeviceControl, DeviceID: "light-001"}},
	}
	if scene.ID != "scene-001" {
		t.Error("scene ID mismatch")
	}
	if !scene.Enabled {
		t.Error("scene should be enabled")
	}
}
