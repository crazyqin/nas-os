package smarthomehub

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	manager := NewManager(nil)
	handler := NewHandler(manager)

	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)

	return r, manager
}

func TestAddDevice(t *testing.T) {
	r, _ := setupTestRouter()

	device := Device{
		ID:       "test-device-1",
		Name:     "Test Light",
		Type:     DeviceTypeLight,
		Protocol: ProtocolMatter,
		Room:     "living-room",
		Capabilities: []string{"on", "brightness"},
	}

	body, _ := json.Marshal(device)
	req, _ := http.NewRequest("POST", "/api/v1/smarthome/devices", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["device"] == nil {
		t.Error("Expected device in response")
	}
}

func TestGetDevice(t *testing.T) {
	r, manager := setupTestRouter()

	// 添加测试设备
	device := Device{
		ID:       "test-device-2",
		Name:     "Test Thermostat",
		Type:     DeviceTypeThermostat,
		Protocol: ProtocolZigbee,
		Room:     "bedroom",
	}
	manager.AddDevice(device)

	req, _ := http.NewRequest("GET", "/api/v1/smarthome/devices/test-device-2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	deviceResp, ok := response["device"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected device in response")
	}

	if deviceResp["name"] != "Test Thermostat" {
		t.Errorf("Expected name 'Test Thermostat', got %v", deviceResp["name"])
	}
}

func TestListDevices(t *testing.T) {
	r, manager := setupTestRouter()

	// 添加测试设备
	manager.AddDevice(Device{ID: "dev1", Name: "Light 1", Type: DeviceTypeLight, Room: "room1"})
	manager.AddDevice(Device{ID: "dev2", Name: "Light 2", Type: DeviceTypeLight, Room: "room2"})
	manager.AddDevice(Device{ID: "dev3", Name: "Sensor 1", Type: DeviceTypeSensor, Room: "room1"})

	// 测试列出所有设备
	req, _ := http.NewRequest("GET", "/api/v1/smarthome/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	devices, ok := response["devices"].([]interface{})
	if !ok {
		t.Fatal("Expected devices array in response")
	}

	if len(devices) != 3 {
		t.Errorf("Expected 3 devices, got %d", len(devices))
	}

	// 测试按房间过滤
	req, _ = http.NewRequest("GET", "/api/v1/smarthome/devices?roomId=room1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &response)
	devices = response["devices"].([]interface{})

	if len(devices) != 2 {
		t.Errorf("Expected 2 devices in room1, got %d", len(devices))
	}
}

func TestControlDevice(t *testing.T) {
	r, manager := setupTestRouter()

	device := Device{
		ID:     "test-device-3",
		Name:   "Test Switch",
		Type:   DeviceTypeSwitch,
		Room:   "kitchen",
		Online: true,
	}
	manager.AddDevice(device)

	controlReq := map[string]interface{}{
		"command": "turn_on",
		"parameters": map[string]interface{}{
			"brightness": 100,
		},
	}

	body, _ := json.Marshal(controlReq)
	req, _ := http.NewRequest("POST", "/api/v1/smarthome/devices/test-device-3/control", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", response["status"])
	}
}

func TestCreateScene(t *testing.T) {
	r, _ := setupTestRouter()

	scene := Scene{
		ID:   "scene-1",
		Name: "Good Night",
		Icon: "moon",
		Actions: []SceneAction{
			{
				DeviceID: "light-1",
				Command:  "turn_off",
			},
			{
				DeviceID: "lock-1",
				Command:  "lock",
			},
		},
		TriggerType: TriggerManual,
		Enabled:     true,
	}

	body, _ := json.Marshal(scene)
	req, _ := http.NewRequest("POST", "/api/v1/smarthome/scenes", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["scene"] == nil {
		t.Error("Expected scene in response")
	}
}

func TestActivateScene(t *testing.T) {
	r, manager := setupTestRouter()

	scene := Scene{
		ID:      "scene-2",
		Name:    "Movie Mode",
		Enabled: true,
		Actions: []SceneAction{
			{DeviceID: "light-1", Command: "dim"},
		},
	}
	manager.CreateScene(scene)

	req, _ := http.NewRequest("POST", "/api/v1/smarthome/scenes/scene-2/activate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", response["status"])
	}
}

func TestCreateAutomation(t *testing.T) {
	r, _ := setupTestRouter()

	automation := Automation{
		ID:   "auto-1",
		Name: "Temperature Control",
		Conditions: []Condition{
			{
				DeviceID: "sensor-1",
				Operator: OperatorGT,
				Value:    25,
			},
		},
		Actions: []SceneAction{
			{
				DeviceID: "fan-1",
				Command:  "turn_on",
			},
		},
		Enabled:  true,
		Cooldown: 300,
	}

	body, _ := json.Marshal(automation)
	req, _ := http.NewRequest("POST", "/api/v1/smarthome/automations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["automation"] == nil {
		t.Error("Expected automation in response")
	}
}

func TestAddRoom(t *testing.T) {
	r, _ := setupTestRouter()

	room := Room{
		ID:        "room-1",
		Name:      "Living Room",
		Icon:      "sofa",
		DeviceIDs: []string{"device-1", "device-2"},
	}

	body, _ := json.Marshal(room)
	req, _ := http.NewRequest("POST", "/api/v1/smarthome/rooms", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["room"] == nil {
		t.Error("Expected room in response")
	}
}

func TestGetRooms(t *testing.T) {
	r, manager := setupTestRouter()

	manager.AddRoom(Room{ID: "room-1", Name: "Living Room"})
	manager.AddRoom(Room{ID: "room-2", Name: "Bedroom"})
	manager.AddRoom(Room{ID: "room-3", Name: "Kitchen"})

	req, _ := http.NewRequest("GET", "/api/v1/smarthome/rooms", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	rooms, ok := response["rooms"].([]interface{})
	if !ok {
		t.Fatal("Expected rooms array in response")
	}

	if len(rooms) != 3 {
		t.Errorf("Expected 3 rooms, got %d", len(rooms))
	}
}

func TestGetHubStatus(t *testing.T) {
	r, manager := setupTestRouter()

	// 添加测试数据
	manager.AddDevice(Device{ID: "dev1", Name: "Light 1", Online: true})
	manager.AddDevice(Device{ID: "dev2", Name: "Light 2", Online: false})
	manager.CreateScene(Scene{ID: "scene1", Name: "Scene 1"})
	manager.AddRoom(Room{ID: "room1", Name: "Room 1"})

	req, _ := http.NewRequest("GET", "/api/v1/smarthome/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	status, ok := response["status"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected status in response")
	}

	if status["total_devices"] != float64(2) {
		t.Errorf("Expected 2 total devices, got %v", status["total_devices"])
	}

	if status["online_devices"] != float64(1) {
		t.Errorf("Expected 1 online device, got %v", status["online_devices"])
	}

	if status["total_scenes"] != float64(1) {
		t.Errorf("Expected 1 scene, got %v", status["total_scenes"])
	}

	if status["total_rooms"] != float64(1) {
		t.Errorf("Expected 1 room, got %v", status["total_rooms"])
	}
}

func TestDiscoverDevices(t *testing.T) {
	r, _ := setupTestRouter()

	discoverReq := map[string]interface{}{
		"timeout": 10,
	}

	body, _ := json.Marshal(discoverReq)
	req, _ := http.NewRequest("POST", "/api/v1/smarthome/devices/discover", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["devices"] == nil {
		t.Error("Expected devices in response")
	}

	if response["count"] == nil {
		t.Error("Expected count in response")
	}
}

func TestDeviceNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/smarthome/devices/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestControlOfflineDevice(t *testing.T) {
	r, manager := setupTestRouter()

	device := Device{
		ID:     "offline-device",
		Name:   "Offline Light",
		Type:   DeviceTypeLight,
		Online: false,
	}
	manager.AddDevice(device)

	controlReq := map[string]interface{}{
		"command": "turn_on",
	}

	body, _ := json.Marshal(controlReq)
	req, _ := http.NewRequest("POST", "/api/v1/smarthome/devices/offline-device/control", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
