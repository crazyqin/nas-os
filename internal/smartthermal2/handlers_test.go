// handlers_test.go - SmartThermal2 测试
package smartthermal2

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestEnv(t *testing.T) (*Handlers, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()
	engine := NewThermalEngine(logger)
	fc := NewFanController(logger, engine)
	noise := NewNoiseOptimizer(logger, fc)
	predictor := NewThermalPredictor(logger, engine)
	profiles := NewProfileManager(logger)
	alerts := NewAlertManager(logger, engine, fc)

	handler := NewHandlers(logger, engine, fc, noise, predictor, profiles, alerts)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return handler, router
}

func TestGetSensors(t *testing.T) {
	_, router := setupTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/sensors", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("期望 code=0，实际 %d", resp.Code)
	}
	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("data 应为数组")
	}
	if len(data) < 5 {
		t.Errorf("期望至少5个传感器，实际 %d", len(data))
	}
}

func TestGetSensorHistory(t *testing.T) {
	handler, router := setupTestEnv(t)
	// 采样几次生成历史
	handler.engine.Sample()
	handler.engine.Sample()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/sensors/cpu-0/history?minutes=60", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
}

func TestGetSensorHistoryNotFound(t *testing.T) {
	_, router := setupTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/sensors/nonexistent/history", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
	// 不存在的传感器应返回空数组
}

func TestGetFans(t *testing.T) {
	_, router := setupTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/fans", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, ok := resp.Data.([]interface{})
	if !ok || len(data) != 4 {
		t.Fatalf("期望4个风扇，实际 %v", resp.Data)
	}
}

func TestUpdateFan(t *testing.T) {
	_, router := setupTestEnv(t)
	pwm := 75.0
	body, _ := json.Marshal(FanUpdateRequest{PWM: &pwm})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/smartthermal2/fans/fan-cpu", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateFanInvalidPWM(t *testing.T) {
	_, router := setupTestEnv(t)
	pwm := 150.0
	body, _ := json.Marshal(FanUpdateRequest{PWM: &pwm})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/smartthermal2/fans/fan-cpu", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("期望状态码 404，实际 %d", w.Code)
	}
}

func TestGetProfiles(t *testing.T) {
	_, router := setupTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/profiles", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	if len(data) < 4 {
		t.Errorf("期望至少4个预设方案，实际 %d", len(data))
	}
}

func TestCreateProfile(t *testing.T) {
	_, router := setupTestEnv(t)
	body, _ := json.Marshal(ProfileCreateRequest{
		Name:        "自定义方案",
		Description: "测试方案",
		Scenario:    "test",
		NoiseLimit:  35,
		MaxTemp:     70,
		FanCurve: FanCurve{
			Type: FanProfileStandard,
			Points: []FanCurvePoint{
				{Temp: 25, PWM: 20}, {Temp: 50, PWM: 50}, {Temp: 80, PWM: 100},
			},
		},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/smartthermal2/profiles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望状态码 201，实际 %d: %s", w.Code, w.Body.String())
	}
}

func TestSetActiveProfile(t *testing.T) {
	_, router := setupTestEnv(t)
	body, _ := json.Marshal(ProfileSwitchRequest{ProfileID: "bedroom"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/smartthermal2/active-profile", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d: %s", w.Code, w.Body.String())
	}
}

func TestSetActiveProfileNotFound(t *testing.T) {
	_, router := setupTestEnv(t)
	body, _ := json.Marshal(ProfileSwitchRequest{ProfileID: "nonexistent"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/smartthermal2/active-profile", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("期望状态码 404，实际 %d", w.Code)
	}
}

func TestGetDashboard(t *testing.T) {
	handler, router := setupTestEnv(t)
	handler.engine.Sample()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/dashboard", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("期望 code=0，实际 %d", resp.Code)
	}
}

func TestGetPredict(t *testing.T) {
	handler, router := setupTestEnv(t)
	// 生成历史数据
	for i := 0; i < 5; i++ {
		handler.engine.Sample()
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/predict?sensor=cpu-0&minutes=30", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
}

func TestGetPredictAll(t *testing.T) {
	handler, router := setupTestEnv(t)
	for i := 0; i < 5; i++ {
		handler.engine.Sample()
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/predict?minutes=15", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
}

func TestGetNoise(t *testing.T) {
	_, router := setupTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/noise", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("噪音数据格式错误")
	}
	if _, ok := data["totalDba"]; !ok {
		t.Error("应包含 totalDba 字段")
	}
}

func TestGetAlerts(t *testing.T) {
	handler, router := setupTestEnv(t)
	// 设置高温触发告警
	handler.engine.UpdateSensorTemp("cpu-0", 85)
	handler.engine.Sample()
	handler.alerts.Check()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/smartthermal2/alerts?limit=10", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
}

func TestEmergencyCooling(t *testing.T) {
	_, router := setupTestEnv(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/smartthermal2/emergency", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}
}

func TestInterpolateCurve(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewThermalEngine(logger)
	fc := NewFanController(logger, engine)

	curve := &FanCurve{
		Type: FanProfileStandard,
		Points: []FanCurvePoint{
			{Temp: 25, PWM: 20},
			{Temp: 50, PWM: 50},
			{Temp: 80, PWM: 100},
		},
	}

	tests := []struct {
		temp   float64
		expect float64
	}{
		{20, 20},   // 低于最低点
		{25, 20},   // 最低点
		{37.5, 35}, // 中间插值
		{50, 50},   // 精确点
		{65, 75},   // 插值
		{80, 100},  // 最高点
		{90, 100},  // 高于最高点
	}

	for _, tt := range tests {
		got := fc.InterpolateCurve(tt.temp, curve)
		if got != tt.expect {
			t.Errorf("InterpolateCurve(%.1f): 期望 %.1f, 实际 %.1f", tt.temp, tt.expect, got)
		}
	}
}

func TestClassifySensorStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewThermalEngine(logger)

	tests := []struct {
		temp   float64
		expect SensorStatus
	}{
		{40, SensorNormal},
		{69, SensorNormal},
		{70, SensorWarning},
		{79, SensorWarning},
		{80, SensorCritical},
		{89, SensorCritical},
		{90, SensorEmergency},
		{100, SensorEmergency},
	}
	for _, tt := range tests {
		got := engine.classifySensorStatus(tt.temp)
		if got != tt.expect {
			t.Errorf("classifySensorStatus(%.0f): 期望 %s, 实际 %s", tt.temp, tt.expect, got)
		}
	}
}

func TestNoiseEstimate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewThermalEngine(logger)
	fc := NewFanController(logger, engine)

	tests := []struct {
		rpm, max int
		minDBA   float64
		maxDBA   float64
	}{
		{0, 3000, 0, 0},
		{600, 3000, 2, 8},    // 低转速噪音低
		{1500, 3000, 10, 15},  // 中转速
		{3000, 3000, 18, 22},  // 全速
	}
	for _, tt := range tests {
		got := fc.estimateFanNoise(tt.rpm, tt.max)
		if got < tt.minDBA || got > tt.maxDBA {
			t.Errorf("estimateNoise(%d/%d): 期望 %.0f-%.0f, 实际 %.1f", tt.rpm, tt.max, tt.minDBA, tt.maxDBA, got)
		}
	}
}

func TestEWMAAdaptive(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewThermalEngine(logger)
	fc := NewFanController(logger, engine)

	// 首次调用应初始化EWMA
	result1 := fc.adaptiveTarget("test-fan", 50)
	if result1 < 20 || result1 > 80 {
		t.Errorf("EWMA初始值异常: %.1f", result1)
	}

	// 连续调用应平滑
	result2 := fc.adaptiveTarget("test-fan", 50)
	if result2 < result1*0.5 || result2 > result1*1.5 {
		t.Errorf("EWMA未平滑: %.1f -> %.1f", result1, result2)
	}
}
