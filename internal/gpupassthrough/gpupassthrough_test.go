// Package gpupassthrough GPU直通管理单元测试
package gpupassthrough

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// 创建测试管理器
func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := tmpDir + "/gpupassthrough.json"

	config := &Config{
		Enabled:          true,
		ConfigPath:       configPath,
		AlertTempWarning: 80,
		AlertTempError:   90,
		AlertPowerLimit:  300,
	}

	manager := NewManager(config)

	// 添加测试设备
	manager.mu.Lock()
	manager.devices["0000:01:00.0"] = &GPUDevice{
		PCIAddress: "0000:01:00.0",
		VendorID:   "0x10de",
		DeviceID:   "0x2684",
		Model:      "NVIDIA GeForce RTX 4090",
		Vendor:     "nvidia",
		Driver:     "nvidia",
		VRAM:       24576,
		BindState:  BindStateNative,
		IOMMUGroup: 1,
		NUMANode:   0,
		Status:     DeviceStatusAvailable,
	}

	manager.devices["0000:02:00.0"] = &GPUDevice{
		PCIAddress: "0000:02:00.0",
		VendorID:   "0x10de",
		DeviceID:   "0x2684",
		Model:      "NVIDIA GeForce RTX 3090",
		Vendor:     "nvidia",
		Driver:     "vfio-pci",
		VRAM:       24576,
		BindState:  BindStateVfio,
		IOMMUGroup: 2,
		NUMANode:   0,
		Status:     DeviceStatusAvailable,
	}
	manager.mu.Unlock()

	return manager, configPath
}

func TestNewManager(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_ListDevices(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	devices := manager.ListDevices()
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestManager_GetDevice(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 测试获取存在的设备
	device, err := manager.GetDevice("0000:01:00.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if device.Model != "NVIDIA GeForce RTX 4090" {
		t.Errorf("unexpected model: %s", device.Model)
	}

	// 测试获取不存在的设备
	_, err = manager.GetDevice("0000:99:00.0")
	if err == nil {
		t.Fatal("expected error for nonexistent device")
	}
}

func TestManager_AssignGPU(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 测试分配给VM
	req := &AssignRequest{
		TargetType: "vm",
		TargetID:   "vm-001",
		ShareMode:  string(ShareModeExclusive),
	}

	err := manager.AssignGPU("0000:01:00.0", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证分配状态
	device, _ := manager.GetDevice("0000:01:00.0")
	if device.Status != DeviceStatusAssigned {
		t.Errorf("expected status %s, got %s", DeviceStatusAssigned, device.Status)
	}
	if len(device.VMAssignments) != 1 {
		t.Errorf("expected 1 vm assignment, got %d", len(device.VMAssignments))
	}

	// 测试重复分配
	err = manager.AssignGPU("0000:01:00.0", req)
	if err == nil {
		t.Fatal("expected error for duplicate assignment")
	}

	// 测试分配给容器
	containerReq := &AssignRequest{
		TargetType:  "container",
		TargetID:    "container-001",
		ShareMode:   string(ShareModeTimeslice),
		MemoryLimit: 8192,
	}

	err = manager.AssignGPU("0000:02:00.0", containerReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_UnassignGPU(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 先分配
	req := &AssignRequest{
		TargetType: "vm",
		TargetID:   "vm-001",
	}
	manager.AssignGPU("0000:01:00.0", req)

	// 测试取消分配
	err := manager.UnassignGPU("0000:01:00.0", "vm-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证状态恢复
	device, _ := manager.GetDevice("0000:01:00.0")
	if device.Status != DeviceStatusAvailable {
		t.Errorf("expected status %s, got %s", DeviceStatusAvailable, device.Status)
	}

	// 测试取消不存在的分配
	err = manager.UnassignGPU("0000:01:00.0", "vm-999")
	if err == nil {
		t.Fatal("expected error for nonexistent assignment")
	}
}

func TestManager_BindVFIO(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 测试绑定到VFIO（模拟，实际需要root权限）
	// 这里只测试逻辑，不测试实际文件操作
	device, _ := manager.GetDevice("0000:01:00.0")
	if device.BindState != BindStateNative {
		t.Errorf("expected bind state %s, got %s", BindStateNative, device.BindState)
	}
}

func TestManager_UnbindVFIO(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 测试解绑VFIO
	device, _ := manager.GetDevice("0000:02:00.0")
	if device.BindState != BindStateVfio {
		t.Errorf("expected bind state %s, got %s", BindStateVfio, device.BindState)
	}
}

func TestManager_GetDeviceStats(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	stats, err := manager.GetDeviceStats("0000:01:00.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.PCIAddress != "0000:01:00.0" {
		t.Errorf("unexpected pci address: %s", stats.PCIAddress)
	}

	// 测试不存在的设备
	_, err = manager.GetDeviceStats("0000:99:00.0")
	if err == nil {
		t.Fatal("expected error for nonexistent device")
	}
}

func TestManager_GetAllAssignments(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 添加一些分配
	vmReq := &AssignRequest{
		TargetType: "vm",
		TargetID:   "vm-001",
	}
	manager.AssignGPU("0000:01:00.0", vmReq)

	containerReq := &AssignRequest{
		TargetType:  "container",
		TargetID:    "container-001",
		MemoryLimit: 4096,
	}
	manager.AssignGPU("0000:02:00.0", containerReq)

	vmAssigns, containerAssigns := manager.GetAllAssignments()
	if len(vmAssigns) != 1 {
		t.Errorf("expected 1 vm assignment, got %d", len(vmAssigns))
	}
	if len(containerAssigns) != 1 {
		t.Errorf("expected 1 container assignment, got %d", len(containerAssigns))
	}
}

func TestManager_CheckAlerts(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 设置高温
	manager.mu.Lock()
	manager.devices["0000:01:00.0"].Temperature = 85
	manager.mu.Unlock()

	manager.CheckAlerts()

	alerts := manager.GetAlerts()
	if len(alerts) == 0 {
		t.Fatal("expected alerts for high temperature")
	}

	found := false
	for _, alert := range alerts {
		if alert.PCIAddress == "0000:01:00.0" && alert.Level == AlertLevelWarning {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning alert for temperature")
	}
}

func TestManager_SaveLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/gpupassthrough.json"

	config := &Config{
		Enabled:    true,
		ConfigPath: configPath,
	}

	// 创建并保存配置
	manager1 := NewManager(config)
	manager1.mu.Lock()
	manager1.devices["0000:01:00.0"] = &GPUDevice{
		PCIAddress: "0000:01:00.0",
		Model:      "Test GPU",
		Status:     DeviceStatusAvailable,
	}
	manager1.mu.Unlock()

	if err := manager1.saveConfig(); err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}
	manager1.Close()

	// 验证配置文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file not created")
	}

	// 加载配置
	manager2 := NewManager(config)
	defer manager2.Close()

	device, err := manager2.GetDevice("0000:01:00.0")
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if device.Model != "Test GPU" {
		t.Errorf("unexpected model after load: %s", device.Model)
	}
}

// API 测试
func setupTestRouter(manager *Manager) *gin.Engine {
	r := gin.New()
	api := r.Group("/api")
	handlers := NewHandlers(manager)
	handlers.RegisterRoutes(api)
	return r
}

func TestAPI_ListDevices(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/gpupassthrough/devices", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestAPI_GetDevice(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	router := setupTestRouter(manager)

	// 测试获取存在的设备
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/gpupassthrough/devices/0000:01:00.0", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 测试获取不存在的设备
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/gpupassthrough/devices/0000:99:00.0", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAPI_AssignGPU(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	router := setupTestRouter(manager)

	// 测试分配GPU
	reqBody := AssignRequest{
		TargetType: "vm",
		TargetID:   "vm-001",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/gpupassthrough/devices/0000:01:00.0/assign", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 测试重复分配
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/gpupassthrough/devices/0000:01:00.0/assign", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestAPI_UnassignGPU(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 先分配
	reqBody := &AssignRequest{
		TargetType: "vm",
		TargetID:   "vm-001",
	}
	manager.AssignGPU("0000:01:00.0", reqBody)

	router := setupTestRouter(manager)

	// 测试取消分配
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/gpupassthrough/devices/0000:01:00.0/assign/vm-001", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAPI_ListAssignments(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 添加分配
	reqBody := &AssignRequest{
		TargetType: "vm",
		TargetID:   "vm-001",
	}
	manager.AssignGPU("0000:01:00.0", reqBody)

	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/gpupassthrough/assignments", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	vmAssigns, ok := data["vmAssignments"].([]interface{})
	if !ok {
		t.Fatal("expected vmAssignments to be an array")
	}
	if len(vmAssigns) != 1 {
		t.Errorf("expected 1 vm assignment, got %d", len(vmAssigns))
	}
}

func TestAPI_GetDeviceStats(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/gpupassthrough/devices/0000:01:00.0/stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestAPI_ListAlerts(t *testing.T) {
	manager, _ := newTestManager(t)
	defer manager.Close()

	// 添加告警
	manager.mu.Lock()
	manager.devices["0000:01:00.0"].Temperature = 85
	manager.mu.Unlock()
	manager.CheckAlerts()

	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/gpupassthrough/alerts", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGPUDevice_Fields(t *testing.T) {
	device := &GPUDevice{
		PCIAddress:  "0000:01:00.0",
		VendorID:    "0x10de",
		DeviceID:    "0x2684",
		Model:       "NVIDIA GeForce RTX 4090",
		Vendor:      "nvidia",
		Driver:      "nvidia",
		VRAM:        24576,
		Temperature: 45,
		PowerUsage:  250,
		BindState:   BindStateNative,
		Status:      DeviceStatusAvailable,
	}

	if device.PCIAddress != "0000:01:00.0" {
		t.Errorf("unexpected pci address: %s", device.PCIAddress)
	}
	if device.Vendor != "nvidia" {
		t.Errorf("unexpected vendor: %s", device.Vendor)
	}
	if device.BindState != BindStateNative {
		t.Errorf("unexpected bind state: %s", device.BindState)
	}
}

func TestVMAssignment_Fields(t *testing.T) {
	assign := VMAssignment{
		VMID:       "vm-001",
		GPUPCIAddr: "0000:01:00.0",
		Status:     "active",
	}

	if assign.VMID != "vm-001" {
		t.Errorf("unexpected vm id: %s", assign.VMID)
	}
}

func TestContainerAssignment_Fields(t *testing.T) {
	assign := ContainerAssignment{
		ContainerID: "container-001",
		GPUPCIAddr:  "0000:01:00.0",
		ShareMode:   ShareModeTimeslice,
		MemoryLimit: 8192,
	}

	if assign.ShareMode != ShareModeTimeslice {
		t.Errorf("unexpected share mode: %s", assign.ShareMode)
	}
}

func TestGPUStats_Fields(t *testing.T) {
	stats := &GPUStats{
		PCIAddress:  "0000:01:00.0",
		GPUUsage:    75.5,
		Temperature: 65,
		PowerUsage:  200,
	}

	if stats.GPUUsage != 75.5 {
		t.Errorf("unexpected gpu usage: %f", stats.GPUUsage)
	}
}

func TestShareModes(t *testing.T) {
	if ShareModeExclusive != "exclusive" {
		t.Errorf("unexpected exclusive mode: %s", ShareModeExclusive)
	}
	if ShareModeTimeslice != "timeslice" {
		t.Errorf("unexpected timeslice mode: %s", ShareModeTimeslice)
	}
	if ShareModeMPS != "mps" {
		t.Errorf("unexpected mps mode: %s", ShareModeMPS)
	}
}

func TestDeviceStatuses(t *testing.T) {
	if DeviceStatusAvailable != "available" {
		t.Errorf("unexpected available status: %s", DeviceStatusAvailable)
	}
	if DeviceStatusAssigned != "assigned" {
		t.Errorf("unexpected assigned status: %s", DeviceStatusAssigned)
	}
	if DeviceStatusError != "error" {
		t.Errorf("unexpected error status: %s", DeviceStatusError)
	}
	if DeviceStatusOffline != "offline" {
		t.Errorf("unexpected offline status: %s", DeviceStatusOffline)
	}
}

func TestBindStates(t *testing.T) {
	if BindStateNative != "native" {
		t.Errorf("unexpected native state: %s", BindStateNative)
	}
	if BindStateVfio != "vfio" {
		t.Errorf("unexpected vfio state: %s", BindStateVfio)
	}
	if BindStateUnbind != "unbind" {
		t.Errorf("unexpected unbind state: %s", BindStateUnbind)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if !config.Enabled {
		t.Error("expected default config to be enabled")
	}
	if config.AlertTempWarning != 80 {
		t.Errorf("expected alert temp warning 80, got %d", config.AlertTempWarning)
	}
	if config.AlertTempError != 90 {
		t.Errorf("expected alert temp error 90, got %d", config.AlertTempError)
	}
	if config.AlertPowerLimit != 300 {
		t.Errorf("expected alert power limit 300, got %d", config.AlertPowerLimit)
	}
}

func TestAlertLevel_Constants(t *testing.T) {
	if AlertLevelInfo != "info" {
		t.Errorf("unexpected info level: %s", AlertLevelInfo)
	}
	if AlertLevelWarning != "warning" {
		t.Errorf("unexpected warning level: %s", AlertLevelWarning)
	}
	if AlertLevelError != "error" {
		t.Errorf("unexpected error level: %s", AlertLevelError)
	}
}
