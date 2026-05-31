// Package diskhealth 单元测试
package diskhealth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.disks == nil {
		t.Error("disks map not initialized")
	}
	if m.history == nil {
		t.Error("history map not initialized")
	}
}

func TestManager_AddDisk(t *testing.T) {
	m := NewManager()

	disk := DiskInfo{
		Device:       "sda",
		Model:        "TestDisk",
		Serial:       "123456",
		Size:         1024 * 1024 * 1024 * 1024, // 1TB
		Temperature:  45,
		PowerOnHours: 1000,
		HealthScore:  95.0,
		SmartStatus:  "passed",
	}

	m.AddDisk(disk)

	disks := m.ScanDisks()
	if len(disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(disks))
	}
	if disks[0].Device != "sda" {
		t.Errorf("Expected device sda, got %s", disks[0].Device)
	}
}

func TestManager_RemoveDisk(t *testing.T) {
	m := NewManager()

	disk := DiskInfo{
		Device:      "sda",
		Model:       "TestDisk",
		HealthScore: 95.0,
	}

	m.AddDisk(disk)
	m.RemoveDisk("sda")

	disks := m.ScanDisks()
	if len(disks) != 0 {
		t.Errorf("Expected 0 disks, got %d", len(disks))
	}
}

func TestManager_GetDiskInfo(t *testing.T) {
	m := NewManager()

	disk := DiskInfo{
		Device:      "sda",
		Model:       "TestDisk",
		HealthScore: 95.0,
	}

	m.AddDisk(disk)

	// 获取存在的磁盘
	info, exists := m.GetDiskInfo("sda")
	if !exists {
		t.Fatal("Expected disk to exist")
	}
	if info.Device != "sda" {
		t.Errorf("Expected device sda, got %s", info.Device)
	}

	// 获取不存在的磁盘
	_, exists = m.GetDiskInfo("sdb")
	if exists {
		t.Error("Expected disk not to exist")
	}
}

func TestManager_GetHealthReport(t *testing.T) {
	m := NewManager()

	// 添加健康磁盘
	disk := DiskInfo{
		Device:      "sda",
		Model:       "TestDisk",
		HealthScore: 95.0,
		Temperature: 40,
		SmartStatus: "passed",
	}
	m.AddDisk(disk)

	report, exists := m.GetHealthReport("sda")
	if !exists {
		t.Fatal("Expected report to exist")
	}

	if report.Status != "healthy" {
		t.Errorf("Expected healthy status, got %s", report.Status)
	}

	// 添加警告磁盘
	disk2 := DiskInfo{
		Device:      "sdb",
		Model:       "TestDisk2",
		HealthScore: 65.0,
		Temperature: 55,
		SmartStatus: "passed",
	}
	m.AddDisk(disk2)

	report2, exists := m.GetHealthReport("sdb")
	if !exists {
		t.Fatal("Expected report to exist")
	}
	if report2.Status != "warning" {
		t.Errorf("Expected warning status, got %s", report2.Status)
	}

	// 添加严重磁盘
	disk3 := DiskInfo{
		Device:      "sdc",
		Model:       "TestDisk3",
		HealthScore: 40.0,
		Temperature: 70,
		SmartStatus: "failed",
	}
	m.AddDisk(disk3)

	report3, exists := m.GetHealthReport("sdc")
	if !exists {
		t.Fatal("Expected report to exist")
	}
	if report3.Status != "critical" {
		t.Errorf("Expected critical status, got %s", report3.Status)
	}
}

func TestManager_CheckAlerts(t *testing.T) {
	m := NewManager()

	// 添加正常磁盘
	disk1 := DiskInfo{
		Device:      "sda",
		Temperature: 40,
		HealthScore: 95.0,
		SmartStatus: "passed",
	}
	m.AddDisk(disk1)

	alerts := m.CheckAlerts()
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts, got %d", len(alerts))
	}

	// 添加高温磁盘
	disk2 := DiskInfo{
		Device:      "sdb",
		Temperature: 65,
		HealthScore: 95.0,
		SmartStatus: "passed",
	}
	m.AddDisk(disk2)

	alerts = m.CheckAlerts()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}

	// 添加低健康评分磁盘
	disk3 := DiskInfo{
		Device:      "sdc",
		Temperature: 40,
		HealthScore: 50.0,
		SmartStatus: "passed",
	}
	m.AddDisk(disk3)

	alerts = m.CheckAlerts()
	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(alerts))
	}
}

func TestManager_GetHistory(t *testing.T) {
	m := NewManager()

	// 添加磁盘
	disk := DiskInfo{
		Device:      "sda",
		HealthScore: 95.0,
	}
	m.AddDisk(disk)

	// 获取历史（应该为空）
	history := m.GetHistory("sda")
	if history != nil {
		t.Errorf("Expected empty history, got %d records", len(history))
	}
}

func TestManager_GetConfig(t *testing.T) {
	m := NewManager()

	config := m.GetConfig()
	if config.TemperatureThreshold != 60 {
		t.Errorf("Expected temperature threshold 60, got %d", config.TemperatureThreshold)
	}
	if config.HealthScoreThreshold != 70 {
		t.Errorf("Expected health score threshold 70, got %f", config.HealthScoreThreshold)
	}
}

func TestManager_UpdateConfig(t *testing.T) {
	m := NewManager()

	newConfig := AlertConfig{
		TemperatureThreshold:        70,
		HealthScoreThreshold:        60,
		ReallocatedSectorsThreshold: 50,
	}

	m.UpdateConfig(newConfig)

	config := m.GetConfig()
	if config.TemperatureThreshold != 70 {
		t.Errorf("Expected temperature threshold 70, got %d", config.TemperatureThreshold)
	}
}

func TestHandler_Disks(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	// 添加测试磁盘
	disk := DiskInfo{
		Device:      "sda",
		Model:       "TestDisk",
		HealthScore: 95.0,
	}
	manager.AddDisk(disk)

	// 测试 GET /disk-health/disks
	req := httptest.NewRequest(http.MethodGet, "/disk-health/disks", nil)
	w := httptest.NewRecorder()

	handler.handleDisks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	disks, ok := response["disks"].([]interface{})
	if !ok {
		t.Fatal("Expected disks array in response")
	}
	if len(disks) != 1 {
		t.Errorf("Expected 1 disk, got %d", len(disks))
	}
}

func TestHandler_DiskDetail(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	disk := DiskInfo{
		Device:      "sda",
		Model:       "TestDisk",
		HealthScore: 95.0,
	}
	manager.AddDisk(disk)

	// 测试 GET /disk-health/disks/sda
	req := httptest.NewRequest(http.MethodGet, "/disk-health/disks/sda", nil)
	w := httptest.NewRecorder()

	handler.handleDiskDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response DiskInfo
	json.NewDecoder(w.Body).Decode(&response)

	if response.Device != "sda" {
		t.Errorf("Expected device sda, got %s", response.Device)
	}
}

func TestHandler_DiskSmart(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	disk := DiskInfo{
		Device:      "sda",
		Model:       "TestDisk",
		HealthScore: 95.0,
	}
	manager.AddDisk(disk)

	// 测试 GET /disk-health/disks/sda/smart
	req := httptest.NewRequest(http.MethodGet, "/disk-health/disks/sda/smart", nil)
	w := httptest.NewRecorder()

	handler.handleDiskDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandler_DiskReport(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	disk := DiskInfo{
		Device:      "sda",
		Model:       "TestDisk",
		HealthScore: 95.0,
		Temperature: 40,
		SmartStatus: "passed",
	}
	manager.AddDisk(disk)

	// 测试 GET /disk-health/disks/sda/report
	req := httptest.NewRequest(http.MethodGet, "/disk-health/disks/sda/report", nil)
	w := httptest.NewRecorder()

	handler.handleDiskDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response HealthReport
	json.NewDecoder(w.Body).Decode(&response)

	if response.Device != "sda" {
		t.Errorf("Expected device sda, got %s", response.Device)
	}
	if response.Status != "healthy" {
		t.Errorf("Expected status healthy, got %s", response.Status)
	}
}

func TestHandler_Alerts(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	// 添加高温磁盘
	disk := DiskInfo{
		Device:      "sda",
		Temperature: 65,
		HealthScore: 95.0,
	}
	manager.AddDisk(disk)

	// 测试 GET /disk-health/alerts
	req := httptest.NewRequest(http.MethodGet, "/disk-health/alerts", nil)
	w := httptest.NewRecorder()

	handler.handleAlerts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	alerts, ok := response["alerts"].([]interface{})
	if !ok {
		t.Fatal("Expected alerts array in response")
	}
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}
}

func TestHandler_Scan(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	// 测试 POST /disk-health/scan
	req := httptest.NewRequest(http.MethodPost, "/disk-health/scan", nil)
	w := httptest.NewRecorder()

	handler.handleScan(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["status"] != "completed" {
		t.Errorf("Expected status completed, got %v", response["status"])
	}
}

func TestHandler_GetConfig(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	// 测试 GET /disk-health/config
	req := httptest.NewRequest(http.MethodGet, "/disk-health/config", nil)
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response AlertConfig
	json.NewDecoder(w.Body).Decode(&response)

	if response.TemperatureThreshold != 60 {
		t.Errorf("Expected temperature threshold 60, got %d", response.TemperatureThreshold)
	}
}

func TestHandler_UpdateConfig(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	newConfig := AlertConfig{
		TemperatureThreshold:        70,
		HealthScoreThreshold:        60,
		ReallocatedSectorsThreshold: 50,
	}

	body, _ := json.Marshal(newConfig)
	req := httptest.NewRequest(http.MethodPut, "/disk-health/config", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 验证配置已更新
	config := manager.GetConfig()
	if config.TemperatureThreshold != 70 {
		t.Errorf("Expected temperature threshold 70, got %d", config.TemperatureThreshold)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	// 测试错误的 HTTP 方法
	req := httptest.NewRequest(http.MethodPost, "/disk-health/disks", nil)
	w := httptest.NewRecorder()

	handler.handleDisks(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandler_NotFound(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	// 测试不存在的磁盘
	req := httptest.NewRequest(http.MethodGet, "/disk-health/disks/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.handleDiskDetail(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestManager_ScanDisksEmpty(t *testing.T) {
	m := NewManager()

	disks := m.ScanDisks()
	if len(disks) != 0 {
		t.Errorf("Expected 0 disks, got %d", len(disks))
	}
}

func TestManager_CheckAlertsEmpty(t *testing.T) {
	m := NewManager()

	alerts := m.CheckAlerts()
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts, got %d", len(alerts))
	}
}

func TestManager_TimeField(t *testing.T) {
	m := NewManager()

	disk := DiskInfo{
		Device:      "sda",
		HealthScore: 95.0,
	}
	m.AddDisk(disk)

	info, _ := m.GetDiskInfo("sda")
	if info.LastCheck.IsZero() {
		t.Error("LastCheck should be set")
	}
	if time.Since(info.LastCheck) > time.Second {
		t.Error("LastCheck should be recent")
	}
}
