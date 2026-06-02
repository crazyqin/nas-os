package smarthomehubpro

import (
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	hub := NewHub(nil)
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.config.MaxDevices != 500 {
		t.Errorf("expected max devices 500, got %d", hub.config.MaxDevices)
	}
}

func TestAddDevice(t *testing.T) {
	hub := NewHub(nil)

	device := &Device{
		ID:       "light-1",
		Name:     "客厅灯",
		Type:     DeviceTypeLight,
		Protocol: ProtocolMatter,
		Room:     "living-room",
	}

	err := hub.AddDevice(device)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 测试重复添加
	err = hub.AddDevice(device)
	if err == nil {
		t.Fatal("expected error for duplicate device")
	}
}

func TestRemoveDevice(t *testing.T) {
	hub := NewHub(nil)

	device := &Device{
		ID:   "switch-1",
		Name: "卧室开关",
		Type: DeviceTypeSwitch,
	}
	hub.AddDevice(device)

	err := hub.RemoveDevice("switch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = hub.GetDevice("switch-1")
	if err == nil {
		t.Fatal("expected error for removed device")
	}
}

func TestGetDevice(t *testing.T) {
	hub := NewHub(nil)

	device := &Device{
		ID:   "sensor-1",
		Name: "温度传感器",
		Type: DeviceTypeSensor,
	}
	hub.AddDevice(device)

	got, err := hub.GetDevice("sensor-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "温度传感器" {
		t.Errorf("expected name '温度传感器', got '%s'", got.Name)
	}
}

func TestListDevices(t *testing.T) {
	hub := NewHub(nil)

	hub.AddDevice(&Device{ID: "d1", Type: DeviceTypeLight})
	hub.AddDevice(&Device{ID: "d2", Type: DeviceTypeSensor})
	hub.AddDevice(&Device{ID: "d3", Type: DeviceTypeSwitch})

	devices := hub.ListDevices()
	if len(devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(devices))
	}
}

func TestUpdateDeviceState(t *testing.T) {
	hub := NewHub(nil)

	device := &Device{
		ID:   "light-2",
		Name: "书房灯",
		Type: DeviceTypeLight,
	}
	hub.AddDevice(device)

	props := map[string]interface{}{
		"brightness": 80,
		"color_temp": 4000,
	}

	err := hub.UpdateDeviceState("light-2", StateOnline, props)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := hub.GetDevice("light-2")
	if got.Properties["brightness"] != 80 {
		t.Errorf("expected brightness 80, got %v", got.Properties["brightness"])
	}
}

func TestAddRoom(t *testing.T) {
	hub := NewHub(nil)

	room := &Room{
		ID:   "living-room",
		Name: "客厅",
	}

	err := hub.AddRoom(room)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 测试重复添加
	err = hub.AddRoom(room)
	if err == nil {
		t.Fatal("expected error for duplicate room")
	}
}

func TestScene(t *testing.T) {
	hub := NewHub(nil)

	// 添加设备
	hub.AddDevice(&Device{ID: "light-1", Type: DeviceTypeLight, State: StateOnline})
	hub.AddDevice(&Device{ID: "blind-1", Type: DeviceTypeBlind, State: StateOnline})

	// 创建场景
	scene := &Scene{
		ID:      "movie-mode",
		Name:    "观影模式",
		Enabled: true,
		Actions: []*SceneAction{
			{
				DeviceID:   "light-1",
				Properties: map[string]interface{}{"brightness": 20},
			},
			{
				DeviceID:   "blind-1",
				Properties: map[string]interface{}{"position": 0},
			},
		},
	}

	err := hub.AddScene(scene)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 激活场景
	err = hub.ActivateScene("movie-mode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证设备状态
	light, _ := hub.GetDevice("light-1")
	if light.Properties["brightness"] != 20 {
		t.Errorf("expected brightness 20, got %v", light.Properties["brightness"])
	}
}

func TestAutomation(t *testing.T) {
	hub := NewHub(nil)

	automation := &Automation{
		ID:      "auto-1",
		Name:    "日落开灯",
		Enabled: true,
		Trigger: &Trigger{
			Type:     TriggerTime,
			Schedule: "0 18 * * *",
		},
		Actions: []*Action{
			{
				Type:     ActionDeviceControl,
				DeviceID: "light-1",
				Properties: map[string]interface{}{
					"power": "on",
				},
			},
		},
	}

	err := hub.AddAutomation(automation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnergyRecording(t *testing.T) {
	hub := NewHub(nil)

	reading := &EnergyReading{
		DeviceID: "plug-1",
		Power:    100.0,
		Energy:   50.0,
		Voltage:  220.0,
		Current:  0.45,
	}

	hub.RecordEnergy(reading)

	total, max := hub.GetEnergyStats("plug-1", time.Hour)
	if total != 50.0 {
		t.Errorf("expected total energy 50.0, got %f", total)
	}
	if max != 100.0 {
		t.Errorf("expected max power 100.0, got %f", max)
	}
}

func TestGetOnlineDevices(t *testing.T) {
	hub := NewHub(nil)

	hub.AddDevice(&Device{ID: "d1", State: StateOnline})
	hub.AddDevice(&Device{ID: "d2", State: StateOffline})
	hub.AddDevice(&Device{ID: "d3", State: StateOnline})

	count := hub.GetOnlineDevices()
	if count != 2 {
		t.Errorf("expected 2 online devices, got %d", count)
	}
}

func TestStartStop(t *testing.T) {
	hub := NewHub(nil)

	err := hub.Start()
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	err = hub.Start()
	if err == nil {
		t.Fatal("expected error for double start")
	}

	err = hub.Stop()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}
