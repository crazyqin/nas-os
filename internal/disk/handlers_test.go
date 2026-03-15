// Package disk 提供磁盘监控 API 处理器测试
package disk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// mockSMARTMonitor 模拟 SMART 监控器
type mockSMARTMonitor struct {
	disks     map[string]*DiskInfo
	alerts    []*SMARTAlert
	rules     []*AlertRule
	weights   *ScoreWeights
	history   map[string][]*SMARTHistoryPoint
	scanError error
}

func newMockSMARTMonitor() *mockSMARTMonitor {
	return &mockSMARTMonitor{
		disks: make(map[string]*DiskInfo),
		alerts: []*SMARTAlert{
			{
				ID:           "alert-1",
				Device:       "/dev/sda",
				RuleID:       "temp-warning",
				Attribute:    "temperature",
				Severity:     AlertWarning,
				Message:      "温度警告",
				Value:        55,
				Threshold:    50,
				Timestamp:    time.Now(),
				Acknowledged: false,
			},
		},
		rules:   getDefaultAlertRules(),
		weights: DefaultScoreWeights,
		history: make(map[string][]*SMARTHistoryPoint),
	}
}

func (m *mockSMARTMonitor) GetAllDisks() []*DiskInfo {
	disks := make([]*DiskInfo, 0, len(m.disks))
	for _, d := range m.disks {
		disks = append(disks, d)
	}
	return disks
}

func (m *mockSMARTMonitor) GetDiskInfo(device string) (*DiskInfo, error) {
	disk, ok := m.disks[device]
	if !ok {
		return nil, nil
	}
	return disk, nil
}

func (m *mockSMARTMonitor) GetAlerts(device string, acknowledged bool) []*SMARTAlert {
	return m.alerts
}

func (m *mockSMARTMonitor) AcknowledgeAlert(id string) error {
	for _, a := range m.alerts {
		if a.ID == id {
			a.Acknowledged = true
			return nil
		}
	}
	return nil
}

func (m *mockSMARTMonitor) GetAlertRules() []*AlertRule {
	return m.rules
}

func (m *mockSMARTMonitor) SetAlertRule(rule *AlertRule) {
	for i, r := range m.rules {
		if r.ID == rule.ID {
			m.rules[i] = rule
			return
		}
	}
	m.rules = append(m.rules, rule)
}

func (m *mockSMARTMonitor) SetScoreWeights(w *ScoreWeights) {
	m.weights = w
}

func (m *mockSMARTMonitor) ScanDisks() error {
	return m.scanError
}

func (m *mockSMARTMonitor) CheckAllDisks() error {
	return nil
}

func (m *mockSMARTMonitor) GetHistory(device string, duration time.Duration) []*SMARTHistoryPoint {
	return m.history[device]
}

func (m *mockSMARTMonitor) ExportJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"disks":  m.disks,
		"alerts": m.alerts,
	})
}

func (m *mockSMARTMonitor) ImportJSON(data []byte) error {
	return nil
}

// setupTestRouter 创建测试路由
func setupTestRouter() (*gin.Engine, *Handlers) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	monitor := newMockSMARTMonitor()
	handlers := NewHandlers(monitor)
	return router, handlers
}

func TestNewHandlers(t *testing.T) {
	monitor := newMockSMARTMonitor()
	handlers := NewHandlers(monitor)
	if handlers == nil {
		t.Fatal("NewHandlers should not return nil")
	}
	if handlers.monitor == nil {
		t.Fatal("monitor should not be nil")
	}
}

func TestListDisks(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 添加测试数据
	mock := handlers.monitor.(*mockSMARTMonitor)
	mock.disks["/dev/sda"] = &DiskInfo{
		Device: "/dev/sda",
		Model:  "Samsung SSD 860",
		Size:   500 * 1024 * 1024 * 1024,
		IsSSD:  true,
		Status: StatusHealthy,
		HealthScore: &HealthScore{
			Score: 95,
			Grade: "A",
			Status: StatusHealthy,
		},
	}
	mock.disks["/dev/sdb"] = &DiskInfo{
		Device: "/dev/sdb",
		Model:  "WD Blue",
		Size:   1 * 1024 * 1024 * 1024 * 1024,
		IsSSD:  false,
		Status: StatusWarning,
		HealthScore: &HealthScore{
			Score: 75,
			Grade: "C",
			Status: StatusWarning,
		},
	}

	req, _ := http.NewRequest("GET", "/api/disk", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", resp["code"])
	}
}

func TestGetDiskInfo(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	mock := handlers.monitor.(*mockSMARTMonitor)
	mock.disks["/dev/sda"] = &DiskInfo{
		Device: "/dev/sda",
		Model:  "Samsung SSD 860",
		Size:   500 * 1024 * 1024 * 1024,
		IsSSD:  true,
		Status: StatusHealthy,
	}

	// 测试存在的磁盘
	req, _ := http.NewRequest("GET", "/api/disk/sda", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 测试不存在的磁盘
	req2, _ := http.NewRequest("GET", "/api/disk/sdz", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent disk, got %d", w2.Code)
	}
}

func TestGetAlerts(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/disk/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("Expected at least one alert")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/disk/alerts/alert-1/acknowledge", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 验证告警已确认
	mock := handlers.monitor.(*mockSMARTMonitor)
	for _, a := range mock.alerts {
		if a.ID == "alert-1" && !a.Acknowledged {
			t.Error("Alert should be acknowledged")
		}
	}
}

func TestGetAlertRules(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/disk/alerts/rules", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Error("Expected at least one alert rule")
	}
}

func TestUpdateAlertRule(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	ruleUpdate := map[string]interface{}{
		"id":        "temp-warning",
		"name":      "温度警告更新",
		"attribute": "temperature",
		"condition": "gt",
		"threshold": 55,
		"severity":  "warning",
		"enabled":   true,
	}
	body, _ := json.Marshal(ruleUpdate)

	req, _ := http.NewRequest("PUT", "/api/disk/alerts/rules/temp-warning", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSetScoreWeights(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	weights := map[string]interface{}{
		"temperature":  0.2,
		"reallocation": 0.25,
		"pending":      0.2,
		"errors":       0.2,
		"age":          0.1,
		"stability":    0.05,
	}
	body, _ := json.Marshal(weights)

	req, _ := http.NewRequest("PUT", "/api/disk/config/weights", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestScanDisks(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/disk/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCheckAllDisks(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/disk/check", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetDiskHistory(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	mock := handlers.monitor.(*mockSMARTMonitor)
	mock.history["sda"] = []*SMARTHistoryPoint{
		{
			Timestamp:   time.Now().Add(-1 * time.Hour),
			HealthScore: 90,
			Temperature: 45,
		},
		{
			Timestamp:   time.Now(),
			HealthScore: 92,
			Temperature: 43,
		},
	}

	req, _ := http.NewRequest("GET", "/api/disk/sda/history?duration=7d", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestExportData(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/disk/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestImportData(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	importData := map[string]interface{}{
		"alertRules": []map[string]interface{}{
			{
				"id":        "custom-rule",
				"name":      "自定义规则",
				"attribute": "temperature",
				"condition": "gt",
				"threshold": 70,
				"severity":  "critical",
				"enabled":   true,
			},
		},
	}
	body, _ := json.Marshal(importData)

	req, _ := http.NewRequest("POST", "/api/disk/import", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetSMARTData(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	mock := handlers.monitor.(*mockSMARTMonitor)
	mock.disks["/dev/sda"] = &DiskInfo{
		Device: "/dev/sda",
		Model:  "Samsung SSD 860",
		SmartData: &SMARTData{
			OverallHealth: "PASSED",
			Temperature: &SMARTAttribute{
				ID:    194,
				Name:  "Temperature_Celsius",
				Value: 45,
				Raw:   45,
			},
		},
	}

	req, _ := http.NewRequest("GET", "/api/disk/sda/smart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetDiskHealth(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	mock := handlers.monitor.(*mockSMARTMonitor)
	mock.disks["/dev/sda"] = &DiskInfo{
		Device: "/dev/sda",
		Model:  "Samsung SSD 860",
		HealthScore: &HealthScore{
			Score:     95,
			Grade:     "A",
			Status:    StatusHealthy,
			Timestamp: time.Now(),
			Components: &ScoreComponents{
				Temperature:  ComponentScore{Score: 100, Status: "ok"},
				Reallocation: ComponentScore{Score: 100, Status: "ok"},
				Pending:      ComponentScore{Score: 100, Status: "ok"},
				Errors:       ComponentScore{Score: 100, Status: "ok"},
				Age:          ComponentScore{Score: 90, Status: "ok"},
				Stability:    ComponentScore{Score: 100, Status: "ok"},
			},
		},
	}

	req, _ := http.NewRequest("GET", "/api/disk/sda/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetDiskPredictions(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	mock := handlers.monitor.(*mockSMARTMonitor)
	mock.disks["/dev/sda"] = &DiskInfo{
		Device: "/dev/sda",
		Model:  "Samsung SSD 860",
		Predictions: []*Prediction{
			{
				Type:        "temperature",
				Probability: 0.3,
				Description: "温度稳定",
				Confidence:  0.8,
			},
		},
	}

	req, _ := http.NewRequest("GET", "/api/disk/sda/predictions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}