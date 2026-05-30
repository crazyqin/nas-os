// Package smartfan 提供智能风扇控制功能
// 单元测试
package smartfan

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
	"go.uber.org/zap"
)

// setupTestHandler 创建测试用的 Handler
func setupTestHandler(t *testing.T) (*Handler, *Controller) {
	t.Helper()
	logger := zap.NewNop()
	controller := NewController(logger)
	handler := NewHandler(controller, logger)
	return handler, controller
}

// setupTestRouter 创建测试用的路由
func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	handler.RegisterRoutes(rg)
	return r
}

// ========== 测试用例 ==========

func TestNewController(t *testing.T) {
	logger := zap.NewNop()
	controller := NewController(logger)

	assert.NotNil(t, controller)
	assert.NotNil(t, controller.zones)
	assert.NotNil(t, controller.fans)
	assert.NotNil(t, controller.profiles)
	assert.NotNil(t, controller.policies)

	// 验证默认配置
	assert.Equal(t, "balanced", controller.activeProfileID)
	assert.Equal(t, "default", controller.activePolicyID)
	assert.Len(t, controller.profiles, 3)   // silent, balanced, performance
	assert.Len(t, controller.zones, 3)      // cpu, gpu, nvme
	assert.Len(t, controller.fans, 3)       // cpu_fan, gpu_fan, case_fan
}

func TestListFans(t *testing.T) {
	handler, _ := setupTestHandler(t)
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/v1/smartfan/fans", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(3), resp["total"])
	assert.NotNil(t, resp["fans"])
}

func TestListZones(t *testing.T) {
	handler, _ := setupTestHandler(t)
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/v1/smartfan/zones", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(3), resp["total"])
	assert.NotNil(t, resp["zones"])
}

func TestListProfiles(t *testing.T) {
	handler, _ := setupTestHandler(t)
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/v1/smartfan/profiles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(3), resp["total"])
	assert.NotNil(t, resp["profiles"])
}

func TestCreateProfile(t *testing.T) {
	handler, controller := setupTestHandler(t)
	router := setupTestRouter(handler)

	// 正常创建
	body := CreateProfileRequest{
		Name: "自定义配置",
		Mode: FanModeCustom,
		Curve: []CurvePoint{
			{Temp: 30, Percent: 25},
			{Temp: 50, Percent: 50},
			{Temp: 70, Percent: 100},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/smartfan/profiles", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var profile FanProfile
	err := json.Unmarshal(w.Body.Bytes(), &profile)
	require.NoError(t, err)

	assert.Equal(t, "自定义配置", profile.Name)
	assert.Equal(t, FanModeCustom, profile.Mode)
	assert.Len(t, profile.Curve, 3)
	assert.NotEmpty(t, profile.ID)

	// 验证配置已添加
	profiles := controller.GetProfiles()
	assert.Len(t, profiles, 4) // 3 默认 + 1 新建
}

func TestCreateProfileInvalidCurve(t *testing.T) {
	handler, _ := setupTestHandler(t)
	router := setupTestRouter(handler)

	// 曲线点不足
	body := CreateProfileRequest{
		Name: "无效配置",
		Mode: FanModeCustom,
		Curve: []CurvePoint{
			{Temp: 30, Percent: 25},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/smartfan/profiles", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateActive(t *testing.T) {
	handler, controller := setupTestHandler(t)
	router := setupTestRouter(handler)

	// 切换到静音模式
	body := UpdateActiveRequest{
		ProfileID: "silent",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/smartfan/active", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "配置已切换", resp["message"])
	assert.Equal(t, "silent", resp["profileId"])

	// 验证已切换
	assert.Equal(t, "silent", controller.activeProfileID)
}

func TestUpdateActiveNotFound(t *testing.T) {
	handler, _ := setupTestHandler(t)
	router := setupTestRouter(handler)

	body := UpdateActiveRequest{
		ProfileID: "nonexistent",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/smartfan/active", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetStats(t *testing.T) {
	handler, _ := setupTestHandler(t)
	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/v1/smartfan/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats FanStats
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)

	assert.Equal(t, "balanced", stats.ActiveProfile)
	assert.Equal(t, "default", stats.ActivePolicy)
	assert.Len(t, stats.Zones, 3)
	assert.Len(t, stats.Fans, 3)
	assert.False(t, stats.Timestamp.IsZero())
}

func TestGetAlerts(t *testing.T) {
	handler, controller := setupTestHandler(t)
	router := setupTestRouter(handler)

	// 手动添加一些告警
	controller.mu.Lock()
	controller.alerts = append(controller.alerts, FanAlert{
		ID:        "test_alert_1",
		Type:      AlertTypeOverheat,
		Severity:  AlertSeverityWarning,
		Source:    "cpu",
		Message:   "测试告警",
		Value:     80.0,
		Threshold: 75.0,
		Timestamp: time.Now(),
	})
	controller.alerts = append(controller.alerts, FanAlert{
		ID:        "test_alert_2",
		Type:      AlertTypeFanFailure,
		Severity:  AlertSeverityCritical,
		Source:    "cpu_fan",
		Message:   "风扇故障测试",
		Value:     0,
		Threshold: 300,
		Timestamp: time.Now(),
	})
	controller.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/smartfan/alerts?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(2), resp["total"])
	assert.NotNil(t, resp["alerts"])
}

func TestControllerStartStop(t *testing.T) {
	logger := zap.NewNop()
	controller := NewController(logger)

	// 启动
	controller.Start()
	assert.True(t, controller.running)

	// 等待一次采集
	time.Sleep(100 * time.Millisecond)

	// 停止
	controller.Stop()
	assert.False(t, controller.running)

	// 重复停止不应 panic
	controller.Stop()
}

func TestPIDControllerCompute(t *testing.T) {
	config := DefaultPIDConfig()
	config.SetPoint = 60.0
	config.MinOutput = 0.0
	config.MaxOutput = 100.0
	pid := NewPIDController(config)

	// 温度低于目标，输出应为 0 (被 MinOutput 钳位)
	output := pid.Compute(50.0)
	assert.Equal(t, 0.0, output) // 50 < 60, output clamped to MinOutput

	// 温度高于目标，输出应为正 (风扇高速)
	pid.Reset()
	output = pid.Compute(70.0)
	assert.Greater(t, output, 0.0) // 70 > 60, output should be positive

	// 温度等于目标，输出应接近 0
	pid.Reset()
	output = pid.Compute(60.0)
	assert.InDelta(t, 0.0, output, 1.0) // 60 = 60, output near zero
}

func TestGetHistory(t *testing.T) {
	logger := zap.NewNop()
	controller := NewController(logger)

	// 手动添加历史记录
	controller.mu.Lock()
	controller.history = append(controller.history, HistoryRecord{
		Timestamp: time.Now().Add(-1 * time.Hour),
		Temps:     map[string]float64{"cpu": 50.0},
		RPMs:      map[string]int{"cpu_fan": 1000},
	})
	controller.history = append(controller.history, HistoryRecord{
		Timestamp: time.Now(),
		Temps:     map[string]float64{"cpu": 60.0},
		RPMs:      map[string]int{"cpu_fan": 1500},
	})
	controller.mu.Unlock()

	// 查询最近 2 小时应有 2 条
	history := controller.GetHistory(2 * time.Hour)
	assert.Len(t, history, 2)

	// 查询最近 30 分钟应有 1 条
	history = controller.GetHistory(30 * time.Minute)
	assert.Len(t, history, 1)
}

func TestCreateProfileCurveNotSorted(t *testing.T) {
	logger := zap.NewNop()
	controller := NewController(logger)

	// 温度不递增
	_, err := controller.CreateProfile("bad", FanModeCustom, []CurvePoint{
		{Temp: 50, Percent: 50},
		{Temp: 30, Percent: 30},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "递增")
}
