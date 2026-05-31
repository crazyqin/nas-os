package smarthub

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager(HubConfig{
		DeviceTimeout:     5 * time.Minute,
		DiscoveryInterval: time.Hour,
		EnableEnergy:      true,
		TariffPerKWh:      0.55,
	})
}

func TestAddAndGetDevice(t *testing.T) {
	m := newTestManager()
	dev := &Device{
		ID:       "light-001",
		Name:     "客厅灯",
		Type:     DeviceTypeLight,
		Protocol: ProtocolZigbee,
		Room:     "客厅",
	}
	if err := m.AddDevice(dev); err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}
	got, err := m.GetDevice("light-001")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if got.Name != "客厅灯" {
		t.Errorf("name = %q, want %q", got.Name, "客厅灯")
	}
	if got.State != StateOnline {
		t.Errorf("state = %q, want %q", got.State, StateOnline)
	}
}

func TestDuplicateDevice(t *testing.T) {
	m := newTestManager()
	dev := &Device{ID: "d1", Name: "test", Type: DeviceTypeLight, Protocol: ProtocolWiFi}
	_ = m.AddDevice(dev)
	if err := m.AddDevice(dev); err != ErrDuplicateDevice {
		t.Errorf("expected ErrDuplicateDevice, got %v", err)
	}
}

func TestRemoveDevice(t *testing.T) {
	m := newTestManager()
	dev := &Device{ID: "d1", Name: "test", Type: DeviceTypeSwitch, Protocol: ProtocolWiFi}
	_ = m.AddDevice(dev)
	if err := m.RemoveDevice("d1"); err != nil {
		t.Fatalf("RemoveDevice failed: %v", err)
	}
	if _, err := m.GetDevice("d1"); err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestListDevicesFilter(t *testing.T) {
	m := newTestManager()
	_ = m.AddDevice(&Device{ID: "d1", Name: "灯", Type: DeviceTypeLight, Protocol: ProtocolZigbee, Room: "客厅"})
	_ = m.AddDevice(&Device{ID: "d2", Name: "插座", Type: DeviceTypePlug, Protocol: ProtocolWiFi, Room: "卧室"})
	_ = m.AddDevice(&Device{ID: "d3", Name: "传感器", Type: DeviceTypeSensor, Protocol: ProtocolZigbee, Room: "客厅"})

	all := m.ListDevices("", "")
	if len(all) != 3 {
		t.Errorf("ListDevices() = %d, want 3", len(all))
	}
客厅Result := m.ListDevices("客厅", "")
	if len(客厅Result) != 2 {
		t.Errorf("ListDevices(客厅) = %d, want 2", len(客厅Result))
	}
	zigbee := m.ListDevices("", ProtocolToDevType(ProtocolZigbee))
	_ = zigbee
}

func TestSendCommand(t *testing.T) {
	m := newTestManager()
	dev := &Device{ID: "d1", Name: "灯", Type: DeviceTypeLight, Protocol: ProtocolWiFi}
	_ = m.AddDevice(dev)
	params := map[string]string{"brightness": "80", "color": "warm"}
	if err := m.SendCommand("d1", "turn_on", params); err != nil {
		t.Fatalf("SendCommand failed: %v", err)
	}
	got, _ := m.GetDevice("d1")
	if got.Properties["brightness"] != "80" {
		t.Errorf("brightness = %q, want %q", got.Properties["brightness"], "80")
	}
}

func TestSendCommandOffline(t *testing.T) {
	m := newTestManager()
	dev := &Device{ID: "d1", Name: "test", Type: DeviceTypeLight, Protocol: ProtocolWiFi, State: StateOffline}
	m.devices["d1"] = dev
	if err := m.SendCommand("d1", "on", nil); err != ErrDeviceOffline {
		t.Errorf("expected ErrDeviceOffline, got %v", err)
	}
}

func TestSceneCreateAndActivate(t *testing.T) {
	m := newTestManager()
	_ = m.AddDevice(&Device{ID: "d1", Name: "灯", Type: DeviceTypeLight, Protocol: ProtocolWiFi})
	scene := &Scene{
		ID:   "s1",
		Name: "回家模式",
		Actions: []SceneAction{
			{DeviceID: "d1", Command: "turn_on", Parameters: map[string]string{"brightness": "100"}},
		},
	}
	if err := m.CreateScene(scene); err != nil {
		t.Fatalf("CreateScene failed: %v", err)
	}
	if err := m.ActivateScene("s1"); err != nil {
		t.Fatalf("ActivateScene failed: %v", err)
	}
	s, _ := m.GetScene("s1")
	if s.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", s.RunCount)
	}
}

func TestGetScene(t *testing.T) {
	m := newTestManager()
	_ = m.CreateScene(&Scene{ID: "s1", Name: "test"})
	s, err := m.GetScene("s1")
	if err != nil {
		t.Fatalf("GetScene failed: %v", err)
	}
	if s.Name != "test" {
		t.Errorf("name = %q, want %q", s.Name, "test")
	}
}

func TestDeleteScene(t *testing.T) {
	m := newTestManager()
	_ = m.CreateScene(&Scene{ID: "s1", Name: "test"})
	if err := m.DeleteScene("s1"); err != nil {
		t.Fatalf("DeleteScene failed: %v", err)
	}
	if _, err := m.GetScene("s1"); err != ErrSceneNotFound {
		t.Errorf("expected ErrSceneNotFound, got %v", err)
	}
}

func TestAutomation(t *testing.T) {
	m := newTestManager()
	_ = m.AddDevice(&Device{
		ID: "sensor-1", Name: "温度传感器", Type: DeviceTypeSensor, Protocol: ProtocolZigbee,
		Properties: map[string]string{"temperature": "28"},
	})
	auto := &Automation{
		ID:   "a1",
		Name: "温度过高开空调",
		Conditions: []Condition{
			{DeviceID: "sensor-1", Property: "temperature", Operator: "gt", Value: "26"},
		},
		Actions: []SceneAction{
			{DeviceID: "sensor-1", Command: "cool_on", Parameters: map[string]string{"temp": "24"}},
		},
		LogicOp: "and",
	}
	if err := m.CreateAutomation(auto); err != nil {
		t.Fatalf("CreateAutomation failed: %v", err)
	}
	autoList := m.ListAutomations()
	if len(autoList) != 1 {
		t.Errorf("ListAutomations() = %d, want 1", len(autoList))
	}
}

func TestEnergyStats(t *testing.T) {
	m := newTestManager()
	_ = m.AddDevice(&Device{ID: "d1", Name: "服务器", Type: DeviceTypePlug, Protocol: ProtocolWiFi})
	m.energyLog = append(m.energyLog, EnergyRecord{
		DeviceID: "d1", Timestamp: time.Now(), Power: 100, Energy: 0.5,
	})
	m.energyLog = append(m.energyLog, EnergyRecord{
		DeviceID: "d1", Timestamp: time.Now(), Power: 120, Energy: 0.6,
	})
	stats := m.GetEnergyStats("d1", time.Now().Add(-time.Hour))
	if stats.TotalEnergy != 1.1 {
		t.Errorf("TotalEnergy = %f, want 1.1", stats.TotalEnergy)
	}
	if stats.PeakPower != 120 {
		t.Errorf("PeakPower = %f, want 120", stats.PeakPower)
	}
}

func TestRooms(t *testing.T) {
	m := newTestManager()
	_ = m.AddDevice(&Device{ID: "d1", Name: "灯1", Type: DeviceTypeLight, Protocol: ProtocolWiFi, Room: "客厅"})
	_ = m.AddDevice(&Device{ID: "d2", Name: "灯2", Type: DeviceTypeLight, Protocol: ProtocolWiFi, Room: "卧室"})
	_ = m.AddDevice(&Device{ID: "d3", Name: "灯3", Type: DeviceTypeLight, Protocol: ProtocolWiFi, Room: "客厅"})
	rooms := m.ListRooms()
	if len(rooms) != 2 {
		t.Errorf("ListRooms() = %d, want 2", len(rooms))
	}
}

func TestHubStats(t *testing.T) {
	m := newTestManager()
	_ = m.AddDevice(&Device{ID: "d1", Name: "test1", Type: DeviceTypeLight, Protocol: ProtocolWiFi})
	_ = m.AddDevice(&Device{ID: "d2", Name: "test2", Type: DeviceTypeSensor, Protocol: ProtocolZigbee})
	_ = m.CreateScene(&Scene{ID: "s1", Name: "scene"})
	stats := m.GetStats()
	if stats.TotalDevices != 2 {
		t.Errorf("TotalDevices = %d, want 2", stats.TotalDevices)
	}
	if stats.OnlineDevices != 2 {
		t.Errorf("OnlineDevices = %d, want 2", stats.OnlineDevices)
	}
	if stats.TotalScenes != 1 {
		t.Errorf("TotalScenes = %d, want 1", stats.TotalScenes)
	}
}

func TestStartStop(t *testing.T) {
	m := newTestManager()
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected running")
	}
	m.Stop()
	if m.IsRunning() {
		t.Error("expected stopped")
	}
}

// ProtocolToDevType 辅助函数.
func ProtocolToDevType(p Protocol) DeviceType {
	return ""
}
