package smarthome

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:          true,
		MatterEnabled:    true,
		HomeKitEnabled:   true,
		MQTTBroker:       "localhost",
		MQTTPort:         1883,
		DiscoveryEnabled: true,
	}
	
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &Config{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	manager.Stop()
}

func TestDefaultRooms(t *testing.T) {
	config := &Config{
		Enabled: true,
	}
	
	manager := NewManager(config)
	manager.initDefaultRooms()
	
	rooms := manager.rooms
	
	expectedRooms := []string{"living_room", "bedroom", "kitchen", "bathroom", "study", "garage", "garden"}
	
	for _, name := range expectedRooms {
		if _, ok := rooms[name]; !ok {
			t.Errorf("Missing default room: %s", name)
		}
	}
}

func TestAddDevice(t *testing.T) {
	config := &Config{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	device := &Device{
		Name:     "Living Room Light",
		Type:     DeviceLight,
		Protocol: ProtocolMatter,
		Room:     "living_room",
		Properties: map[string]interface{}{
			"brightness": 100,
			"color":      "#FFFFFF",
		},
	}
	
	if err := manager.AddDevice(device); err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}
	
	if device.ID == "" {
		t.Error("Device ID not generated")
	}
	
	if device.State != StateOnline {
		t.Errorf("Expected state online, got %s", device.State)
	}
}

func TestRemoveDevice(t *testing.T) {
	config := &Config{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	device := &Device{
		Name:     "Test Device",
		Type:     DeviceLight,
		Protocol: ProtocolMatter,
	}
	
	manager.AddDevice(device)
	
	if err := manager.RemoveDevice(device.ID); err != nil {
		t.Fatalf("RemoveDevice failed: %v", err)
	}
	
	// Try to get removed device
	_, err := manager.GetDevice(device.ID)
	if err == nil {
		t.Error("Expected error for removed device")
	}
}

func TestControlDevice(t *testing.T) {
	config := &Config{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	device := &Device{
		Name:       "Test Light",
		Type:       DeviceLight,
		Protocol:   ProtocolMatter,
		Properties: make(map[string]interface{}),
	}
	
	manager.AddDevice(device)
	
	// Control device
	err := manager.controlDevice(device.ID, map[string]interface{}{
		"brightness": 50,
		"power":      true,
	})
	
	if err != nil {
		t.Fatalf("controlDevice failed: %v", err)
	}
	
	// Verify properties
	updated, _ := manager.GetDevice(device.ID)
	if updated.Properties["brightness"] != 50 {
		t.Errorf("Expected brightness 50, got %v", updated.Properties["brightness"])
	}
}

func TestAddScene(t *testing.T) {
	config := &Config{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	scene := &Scene{
		Name:        "Movie Night",
		Description: "Dim lights for movie watching",
		Devices: []SceneDevice{
			{
				DeviceID: "light1",
				Properties: map[string]interface{}{
					"brightness": 20,
				},
			},
		},
	}
	
	if err := manager.AddScene(scene); err != nil {
		t.Fatalf("AddScene failed: %v", err)
	}
	
	if scene.ID == "" {
		t.Error("Scene ID not generated")
	}
}

func TestAddAutomation(t *testing.T) {
	config := &Config{
		Enabled: true,
	}
	
	manager := NewManager(config)
	
	auto := &Automation{
		Name:        "Good Morning",
		Description: "Turn on lights at sunrise",
		Enabled:     true,
		Triggers: []Trigger{
			{
				Type: "time",
				Config: map[string]interface{}{
					"cron": "0 7 * * *",
				},
			},
		},
		Actions: []Action{
			{
				Type:     "device",
				TargetID: "light1",
				Properties: map[string]interface{}{
					"power":      true,
					"brightness": 100,
				},
			},
		},
	}
	
	if err := manager.AddAutomation(auto); err != nil {
		t.Fatalf("AddAutomation failed: %v", err)
	}
	
	if auto.ID == "" {
		t.Error("Automation ID not generated")
	}
}

func TestDeviceTypes(t *testing.T) {
	types := []DeviceType{
		DeviceLight, DeviceSwitch, DeviceSensor, DeviceThermostat,
		DeviceCamera, DeviceLock, DeviceSpeaker, DeviceTV,
		DeviceBlind, DeviceFan, DeviceAirPurifier, DeviceHumidifier,
		DeviceRobotVacuum, DeviceDoorbell, DeviceGarage, DeviceIrrigation,
	}
	
	for _, dt := range types {
		if string(dt) == "" {
			t.Errorf("Empty device type: %v", dt)
		}
	}
}

func TestProtocols(t *testing.T) {
	protocols := []Protocol{
		ProtocolMatter, ProtocolHomeKit, ProtocolZigbee, ProtocolZWave,
		ProtocolMQTT, ProtocolWiFi, ProtocolBluetooth,
	}
	
	for _, p := range protocols {
		if string(p) == "" {
			t.Errorf("Empty protocol: %v", p)
		}
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{
		Enabled:        true,
		MatterEnabled:  true,
		MQTTBroker:     "localhost",
	}
	
	manager := NewManager(config)
	
	// Add some devices
	manager.AddDevice(&Device{
		Name:     "Light 1",
		Type:     DeviceLight,
		Protocol: ProtocolMatter,
	})
	
	manager.AddDevice(&Device{
		Name:     "Light 2",
		Type:     DeviceLight,
		Protocol: ProtocolMatter,
	})
	
	stats := manager.GetStats()
	
	if stats["total_devices"] != 2 {
		t.Errorf("Expected 2 devices, got %v", stats["total_devices"])
	}
	
	if stats["online_devices"] != 2 {
		t.Errorf("Expected 2 online devices, got %v", stats["online_devices"])
	}
}
