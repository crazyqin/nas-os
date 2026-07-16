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
		Type:     DeviceTypeLight,
		Protocol: ProtocolMQTT,
		RoomID:   "living_room",
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

	if device.Status != DeviceStatusUnknown {
		t.Errorf("Expected status unknown, got %s", device.Status)
	}
}

func TestDeleteDevice(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	device := &Device{
		Name:     "Test Device",
		Type:     DeviceTypeLight,
		Protocol: ProtocolMQTT,
	}

	if err := manager.AddDevice(device); err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}

	if err := manager.DeleteDevice(device.ID); err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	// Try to get deleted device
	_, err := manager.GetDevice(device.ID)
	if err == nil {
		t.Error("Expected error for deleted device")
	}
}

func TestUpdateDeviceState(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	device := &Device{
		Name:       "Test Light",
		Type:       DeviceTypeLight,
		Protocol:   ProtocolMQTT,
		Properties: make(map[string]interface{}),
	}

	if err := manager.AddDevice(device); err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}

	// Update device state
	err := manager.UpdateDeviceState(device.ID, map[string]interface{}{
		"brightness": 50,
		"power":      true,
	})

	if err != nil {
		t.Fatalf("UpdateDeviceState failed: %v", err)
	}

	// Verify state
	updated, _ := manager.GetDevice(device.ID)
	if updated.State["brightness"] != 50 {
		t.Errorf("Expected brightness 50, got %v", updated.State["brightness"])
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
		Trigger: Trigger{
			Type: TriggerTypeManual,
		},
		Actions: []Action{
			{
				Type:     ActionTypeDeviceControl,
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

func TestDeviceTypes(t *testing.T) {
	types := []DeviceType{
		DeviceTypeLight, DeviceTypeSwitch, DeviceTypeSensor, DeviceTypeThermostat,
		DeviceTypeCamera, DeviceTypeLock, DeviceTypePlug, DeviceTypeFan,
		DeviceTypeCurtain, DeviceTypeSpeaker, DeviceTypeOther,
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
		ProtocolMQTT, ProtocolHTTP, ProtocolWiFi, ProtocolBluetooth,
	}

	for _, p := range protocols {
		if string(p) == "" {
			t.Errorf("Empty protocol: %v", p)
		}
	}
}
