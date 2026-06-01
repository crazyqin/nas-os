// Package storagesimulator HTTP 处理器单元测试
package storagesimulator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(manager *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(manager)
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return router
}

func addTestUsageData(m *Manager, count int) {
	baseTime := time.Now().AddDate(0, 0, -count)
	for i := 0; i < count; i++ {
		m.AddUsageRecord(StorageUsage{
			Timestamp:   baseTime.AddDate(0, 0, i),
			UsedBytes:   int64(5*1024*1024*1024) + int64(i)*int64(100*1024*1024),
			TotalBytes:  10 * 1024 * 1024 * 1024 * 1024,
			UsedPercent: float64(5*1024*1024*1024+int64(i)*int64(100*1024*1024)) / float64(10*1024*1024*1024*1024) * 100,
		})
	}
}

func TestNewHandler(t *testing.T) {
	mgr := NewManager()
	handler := NewHandler(mgr)
	assert.NotNil(t, handler)
	assert.Equal(t, mgr, handler.manager)
}

func TestRegisterRoutes(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	routes := router.Routes()
	expectedPaths := []string{
		"/api/v1/storage-simulator/usage",
		"/api/v1/storage-simulator/forecast",
		"/api/v1/storage-simulator/scenarios",
		"/api/v1/storage-simulator/alerts",
		"/api/v1/storage-simulator/config",
	}

	registeredPaths := make(map[string]bool)
	for _, r := range routes {
		registeredPaths[r.Path] = true
	}

	for _, expected := range expectedPaths {
		assert.True(t, registeredPaths[expected], "missing route: %s", expected)
	}
}

func TestGetUsage_Empty(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/usage", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.Nil(t, data["current"])
	assert.Equal(t, float64(0), data["count"])
}

func TestGetUsage_WithData(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 5)
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/usage", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.NotNil(t, data["current"])
	assert.Equal(t, float64(5), data["count"])
}

func TestPostUsage_Success(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{"used_bytes": 5368709120, "total_bytes": 10737418240}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/usage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	// 验证数据已保存
	history := mgr.GetUsageHistory()
	assert.Len(t, history, 1)
}

func TestPostUsage_InvalidJSON(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{"invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/usage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostUsage_AutoTimestamp(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	before := time.Now()
	body := `{"used_bytes": 1073741824}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/usage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	after := time.Now()

	assert.Equal(t, http.StatusCreated, w.Code)

	history := mgr.GetUsageHistory()
	require.Len(t, history, 1)
	assert.True(t, history[0].Timestamp.After(before) || history[0].Timestamp.Equal(before))
	assert.True(t, history[0].Timestamp.Before(after) || history[0].Timestamp.Equal(after))
}

func TestPostUsage_AutoTotalBytes(t *testing.T) {
	mgr := NewManager()
	mgr.SetTotalCapacity(1099511627776) // 1TB
	router := setupTestRouter(mgr)

	body := `{"used_bytes": 5368709120}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/usage", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	history := mgr.GetUsageHistory()
	require.Len(t, history, 1)
	assert.Equal(t, int64(1099511627776), history[0].TotalBytes)
}

func TestGetForecast_Success(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 10)
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/forecast?period=daily&duration=30&scenario=medium", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "daily", data["period"])
	assert.Equal(t, "medium", data["scenario"])
}

func TestGetForecast_DefaultParams(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 5)
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/forecast", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetForecast_NoHistory(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/forecast", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetScenarios_List(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 10)
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/scenarios", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].([]interface{})
	assert.GreaterOrEqual(t, len(data), 4) // 至少4个内置场景
}

func TestGetScenarios_Single(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 10)
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/scenarios?id=builtin-high", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "builtin-high", data["config"].(map[string]interface{})["id"])
}

func TestGetScenarios_NotFound(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 10)
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/scenarios?id=nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetScenarios_SimulateAll(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 10)
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/scenarios?simulate=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.NotNil(t, data["scenarios"])
	assert.NotNil(t, data["simulations"])
}

func TestGetAlerts_Empty(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["count"])
}

func TestGetAlerts_WithData(t *testing.T) {
	mgr := NewManager()
	mgr.AddAlert(&CapacityAlert{
		ID:        "test-alert",
		Name:      "测试告警",
		Threshold: 80,
		Level:     AlertWarning,
		Enabled:   true,
	})
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["count"])
}

func TestGetAlerts_FilterTriggered(t *testing.T) {
	mgr := NewManager()
	addTestUsageData(mgr, 1) // 添加一些数据
	mgr.AddAlert(&CapacityAlert{
		ID:        "low-threshold",
		Name:      "低阈值告警",
		Threshold: 1, // 1% 阈值，会被触发
		Level:     AlertCritical,
		Enabled:   true,
	})
	mgr.AddAlert(&CapacityAlert{
		ID:        "high-threshold",
		Name:      "高阈值告警",
		Threshold: 99, // 99% 阈值，不会被触发
		Level:     AlertWarning,
		Enabled:   true,
	})
	router := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-simulator/alerts?triggered=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	assert.True(t, data["filtered"].(bool))
}

func TestPostConfig_Alert(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{"alert": {"id": "new-alert", "name": "新告警", "threshold": 85, "level": "warning", "enabled": true}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestPostConfig_Cost(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{"cost_per_gb": 0.05, "currency": "EUR"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPostConfig_TotalCapacity(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{"total_capacity": 2199023255552}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPostConfig_EmptyBody(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostConfig_InvalidJSON(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{"invalid`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostConfig_MultipleConfigs(t *testing.T) {
	mgr := NewManager()
	router := setupTestRouter(mgr)

	body := `{"alert": {"id": "multi-alert", "name": "多配置告警", "threshold": 90, "level": "critical", "enabled": true}, "cost_per_gb": 0.03, "currency": "USD", "total_capacity": 1099511627776}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-simulator/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	assert.NotNil(t, data["alert"])
	assert.NotNil(t, data["cost"])
	assert.NotNil(t, data["capacity"])
}
