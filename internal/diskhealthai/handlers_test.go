// Package diskhealthai HTTP 处理器单元测试
package diskhealthai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockService 实现 Service 接口用于测试
type mockService struct {
	disks    []DiskInfo
	smart    *SMARTSnapshot
	report   *HealthReport
	alerts   []Alert
	trend    *TrendAnalysis
	scanErr  error
}

func (m *mockService) ListDisks() []DiskInfo {
	return m.disks
}

func (m *mockService) GetDisk(device string) (*DiskInfo, error) {
	for _, d := range m.disks {
		if d.Device == device {
			return &d, nil
		}
	}
	return nil, assert.AnError
}

func (m *mockService) GetSMART(device string) (*SMARTSnapshot, error) {
	if m.smart != nil && m.smart.Device == device {
		return m.smart, nil
	}
	return nil, assert.AnError
}

func (m *mockService) Predict(device string) (*HealthReport, error) {
	if m.report != nil && m.report.Device == device {
		return m.report, nil
	}
	return nil, assert.AnError
}

func (m *mockService) TriggerScan() error {
	return m.scanErr
}

func (m *mockService) ListAlerts() []Alert {
	return m.alerts
}

func (m *mockService) GetHistory(device string, days int) (*TrendAnalysis, error) {
	if m.trend != nil {
		return m.trend, nil
	}
	return nil, assert.AnError
}

func setupTestRouter(svc Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(svc)
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return router
}

func TestNewHandler(t *testing.T) {
	svc := &mockService{}
	h := NewHandler(svc)
	assert.NotNil(t, h)
	assert.Equal(t, svc, h.svc)
}

func TestRegisterRoutes(t *testing.T) {
	svc := &mockService{}
	router := setupTestRouter(svc)

	// 验证所有路由都已注册
	routes := router.Routes()
	expectedPaths := []string{
		"/api/v1/disk-health-ai/disks",
		"/api/v1/disk-health-ai/disks/:device",
		"/api/v1/disk-health-ai/disks/:device/smart",
		"/api/v1/disk-health-ai/disks/:device/predict",
		"/api/v1/disk-health-ai/scan",
		"/api/v1/disk-health-ai/alerts",
		"/api/v1/disk-health-ai/history",
	}

	registeredPaths := make(map[string]bool)
	for _, r := range routes {
		registeredPaths[r.Path] = true
	}

	for _, expected := range expectedPaths {
		assert.True(t, registeredPaths[expected], "missing route: %s", expected)
	}
}

func TestListDisks_Empty(t *testing.T) {
	svc := &mockService{disks: []DiskInfo{}}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestListDisks_WithData(t *testing.T) {
	svc := &mockService{
		disks: []DiskInfo{
			{Device: "/dev/sda", Model: "WD Red Plus", Status: StatusGood},
			{Device: "/dev/sdb", Model: "Seagate IronWolf", Status: StatusExcellent},
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestGetDisk_Success(t *testing.T) {
	svc := &mockService{
		disks: []DiskInfo{
			{Device: "sda", Model: "WD Red Plus", Status: StatusGood},
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks/sda", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestGetDisk_NotFound(t *testing.T) {
	svc := &mockService{disks: []DiskInfo{}}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestGetSMART_Success(t *testing.T) {
	svc := &mockService{
		smart: &SMARTSnapshot{
			Device:      "sda",
			Temperature: 35,
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks/sda/smart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestGetSMART_NotFound(t *testing.T) {
	svc := &mockService{smart: nil}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks/nonexistent/smart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPredict_Success(t *testing.T) {
	svc := &mockService{
		report: &HealthReport{
			Device:      "sda",
			HealthScore: 85.5,
			Status:      StatusGood,
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks/sda/predict", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestPredict_NotFound(t *testing.T) {
	svc := &mockService{report: nil}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/disks/nonexistent/predict", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTriggerScan_Success(t *testing.T) {
	svc := &mockService{scanErr: nil}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/disk-health-ai/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, "扫描任务已触发", resp["message"])
}

func TestTriggerScan_Error(t *testing.T) {
	svc := &mockService{scanErr: assert.AnError}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/disk-health-ai/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestListAlerts_Empty(t *testing.T) {
	svc := &mockService{alerts: []Alert{}}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestListAlerts_WithData(t *testing.T) {
	svc := &mockService{
		alerts: []Alert{
			{
				ID:      "alert-1",
				Device:  "/dev/sda",
				Level:   "warning",
				Type:    "temperature",
				Message: "温度偏高",
			},
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestGetHistory_Success(t *testing.T) {
	svc := &mockService{
		trend: &TrendAnalysis{
			HealthTrend:    "stable",
			TemperatureTrend: "stable",
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/history?device=/dev/sda&days=90", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestGetHistory_MissingDevice(t *testing.T) {
	svc := &mockService{}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp["success"].(bool))
}

func TestGetHistory_CustomDays(t *testing.T) {
	svc := &mockService{
		trend: &TrendAnalysis{
			HealthTrend: "improving",
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/history?device=/dev/sda&days=30", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_DefaultDays(t *testing.T) {
	svc := &mockService{
		trend: &TrendAnalysis{
			HealthTrend: "stable",
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/history?device=/dev/sda", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_InvalidDays(t *testing.T) {
	svc := &mockService{
		trend: &TrendAnalysis{
			HealthTrend: "stable",
		},
	}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/history?device=/dev/sda&days=invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // 应该使用默认值90
}

func TestGetHistory_ServiceError(t *testing.T) {
	svc := &mockService{trend: nil}
	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/disk-health-ai/history?device=/dev/sda", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
