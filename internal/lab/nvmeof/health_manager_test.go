package nvmeof

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupHealthTest(t *testing.T) (*HealthManager, *HealthHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")

	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	healthMgr := NewHealthManager(mgr, configPath, zap.NewNop())
	handler := NewHealthHandler(healthMgr, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return healthMgr, handler, router
}

// ============================================================
// 温度监控测试
// ============================================================

func TestRecordTemperature(t *testing.T) {
	healthMgr, _, router := setupHealthTest(t)

	body := `{"device":"nvme0n1","subsystem_nqn":"nqn.2026-01.com.nas-os:test","temperature":45.5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify status was recorded
	status, err := healthMgr.GetDeviceTemperatureStatus("nvme0n1")
	require.NoError(t, err)
	assert.Equal(t, 45.5, status.CurrentTemp)
	assert.Equal(t, "normal", status.Status)
	assert.Equal(t, "nqn.2026-01.com.nas-os:test", status.SubsystemNQN)
}

func TestRecordTemperatureWarning(t *testing.T) {
	healthMgr, _, router := setupHealthTest(t)

	body := `{"device":"nvme0n1","temperature":75.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	status, err := healthMgr.GetDeviceTemperatureStatus("nvme0n1")
	require.NoError(t, err)
	assert.Equal(t, "warning", status.Status)
	assert.Equal(t, 1, status.AlertCount)
}

func TestRecordTemperatureCritical(t *testing.T) {
	healthMgr, _, router := setupHealthTest(t)

	body := `{"device":"nvme0n1","temperature":90.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	status, err := healthMgr.GetDeviceTemperatureStatus("nvme0n1")
	require.NoError(t, err)
	assert.Equal(t, "critical", status.Status)
	assert.Equal(t, 90.0, status.CurrentTemp)
}

func TestGetDeviceTemperatureStatus(t *testing.T) {
	_, _, router := setupHealthTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/temperature/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAllDeviceTemperatureStatuses(t *testing.T) {
	_, _, router := setupHealthTest(t)

	// Record temperatures for two devices
	body1 := `{"device":"nvme0n1","temperature":45.0}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBufferString(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	body2 := `{"device":"nvme1n1","temperature":55.0}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/temperature", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Devices []*DeviceTemperatureStatus `json:"devices"`
		Total   int                        `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Total)
}

func TestGetTemperatureHistory(t *testing.T) {
	_, _, router := setupHealthTest(t)

	// Record multiple readings
	for _, temp := range []float64{40, 42, 45, 43, 48} {
		b, _ := json.Marshal(map[string]interface{}{"device": "nvme0n1", "temperature": temp})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/temperature/nvme0n1/history?limit=3", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Device  string               `json:"device"`
		History []TemperatureReading `json:"history"`
		Total   int                  `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 3, resp.Total)
}

func TestGetTemperatureConfig(t *testing.T) {
	_, _, router := setupHealthTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/temperature/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var cfg TemperatureConfig
	err := json.Unmarshal(w.Body.Bytes(), &cfg)
	require.NoError(t, err)
	assert.Equal(t, 70.0, cfg.WarningThreshold)
	assert.Equal(t, 85.0, cfg.CriticalThreshold)
}

func TestUpdateTemperatureConfig(t *testing.T) {
	_, _, router := setupHealthTest(t)

	body := `{"enabled":true,"interval_sec":30,"warning_threshold":65,"critical_threshold":80}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nvmeof/health/temperature/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRecentAlerts(t *testing.T) {
	_, _, router := setupHealthTest(t)

	// Generate a warning alert
	body := `{"device":"nvme0n1","temperature":75.0}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/temperature/alerts?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Alerts []TemperatureAlert `json:"alerts"`
		Total  int                `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, "warning", resp.Alerts[0].Level)
}

// ============================================================
// 寿命预测测试
// ============================================================

func TestPredictDeviceLife(t *testing.T) {
	_, _, router := setupHealthTest(t)

	// First record some temperature data for better prediction
	tempBody := `{"device":"nvme0n1","temperature":45.0}`
	tempReq := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/temperature", bytes.NewBufferString(tempBody))
	tempReq.Header.Set("Content-Type", "application/json")
	tempW := httptest.NewRecorder()
	router.ServeHTTP(tempW, tempReq)

	body := `{
		"subsystem_nqn": "nqn.2026-01.com.nas-os:test",
		"model": "Samsung 980 PRO",
		"serial": "S123456",
		"total_write_capacity_tb": 600,
		"total_written_tb": 100,
		"percentage_used": 16,
		"available_spare": 95,
		"power_on_hours": 5000,
		"unsafe_shutdowns": 3,
		"media_errors": 0
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/life-prediction/nvme0n1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var pred DeviceLifePrediction
	err := json.Unmarshal(w.Body.Bytes(), &pred)
	require.NoError(t, err)
	assert.Equal(t, "nvme0n1", pred.Device)
	assert.Equal(t, "Samsung 980 PRO", pred.Model)
	assert.True(t, pred.RemainingLifePercent > 0)
	assert.True(t, pred.RemainingLifePercent <= 100)
	assert.NotEmpty(t, pred.WearLevel)
}

func TestGetLifePredictionNotFound(t *testing.T) {
	_, _, router := setupHealthTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/life-prediction/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAllLifePredictions(t *testing.T) {
	_, _, router := setupHealthTest(t)

	// Create a prediction first
	body := `{"total_write_capacity_tb":600,"total_written_tb":100,"percentage_used":16}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/life-prediction/nvme0n1", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/life-prediction", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Predictions []*DeviceLifePrediction `json:"predictions"`
		Total       int                     `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

func TestUpdateWritePattern(t *testing.T) {
	_, _, router := setupHealthTest(t)

	body := `{
		"subsystem_nqn": "nqn.2026-01.com.nas-os:test",
		"total_write_tb": 50.5,
		"total_read_tb": 100.2,
		"daily_write_avg_gb": 10.5,
		"weekly_write_avg_gb": 73.5,
		"peak_write_rate_gbps": 3.5,
		"write_amplification": 1.8,
		"sample_period_days": 30
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nvmeof/health/write-pattern/nvme0n1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================
// 性能基准测试测试
// ============================================================

func TestStartBenchmark(t *testing.T) {
	hm, _, router := setupHealthTest(t)

	tmpDir := t.TempDir()
	body := `{
		"device_path": "` + tmpDir + `",
		"subsystem_nqn": "nqn.2026-01.com.nas-os:test",
		"block_size_kb": 64,
		"file_size_mb": 1,
		"test_types": ["seq_read", "seq_write"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/benchmark", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var result BenchmarkResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.True(t, result.Status == "pending" || result.Status == "running")

	// 等待异步benchmark完成，避免TempDir清理竞争
	hm.WaitForBenchmarks()
}

func TestStartBenchmarkInvalidSize(t *testing.T) {
	_, _, router := setupHealthTest(t)

	tmpDir := t.TempDir()
	body := `{
		"device_path": "` + tmpDir + `",
		"file_size_mb": 9999
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/health/benchmark", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestGetBenchmarkResultNotFound(t *testing.T) {
	_, _, router := setupHealthTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/benchmark/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListBenchmarkResults(t *testing.T) {
	_, _, router := setupHealthTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/health/benchmarks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Results []*BenchmarkResult `json:"results"`
		Total   int                `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)
}

// ============================================================
// Manager 单元测试
// ============================================================

func TestHealthManager_RecordTemperature(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	// Record multiple readings
	for _, temp := range []float64{40.0, 42.0, 45.0, 43.0, 48.0} {
		err := hm.RecordTemperature(nil, "nvme0n1", "nqn.test", temp)
		require.NoError(t, err)
	}

	status, err := hm.GetDeviceTemperatureStatus("nvme0n1")
	require.NoError(t, err)
	assert.Equal(t, 48.0, status.CurrentTemp)
	assert.Equal(t, 40.0, status.MinTemp)
	assert.Equal(t, 48.0, status.MaxTemp)
	assert.Equal(t, "normal", status.Status)
}

func TestHealthManager_TemperatureAlerts(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	// Record warning temperature
	err = hm.RecordTemperature(nil, "nvme0n1", "nqn.test", 72.0)
	require.NoError(t, err)

	status, _ := hm.GetDeviceTemperatureStatus("nvme0n1")
	assert.Equal(t, "warning", status.Status)
	assert.Equal(t, 1, status.AlertCount)
	assert.NotNil(t, status.LastAlert)
	assert.Equal(t, "warning", status.LastAlert.Level)

	// Record critical temperature
	err = hm.RecordTemperature(nil, "nvme0n1", "nqn.test", 90.0)
	require.NoError(t, err)

	status, _ = hm.GetDeviceTemperatureStatus("nvme0n1")
	assert.Equal(t, "critical", status.Status)
	assert.Equal(t, 2, status.AlertCount)

	// Get recent alerts
	alerts := hm.GetRecentAlerts(10)
	assert.Equal(t, 2, len(alerts))
}

func TestHealthManager_TemperatureHistory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	for i := 0; i < 10; i++ {
		_ = hm.RecordTemperature(nil, "nvme0n1", "nqn.test", float64(30+i))
	}

	history := hm.GetTemperatureHistory("nvme0n1", 5)
	assert.Equal(t, 5, len(history))
	// Last 5 readings should be 35-39
	assert.Equal(t, 35.0, history[0].Temperature)
	assert.Equal(t, 39.0, history[4].Temperature)

	// Nonexistent device
	history = hm.GetTemperatureHistory("nonexistent", 10)
	assert.Equal(t, 0, len(history))
}

func TestHealthManager_LifePrediction(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	// Record temperature history for better prediction
	for i := 0; i < 50; i++ {
		_ = hm.RecordTemperature(nil, "nvme0n1", "nqn.test", 45.0)
	}

	pred, err := hm.PredictDeviceLife(
		nil, "nvme0n1", "nqn.test", "Samsung 980 PRO", "S123456",
		600, 100, 16, 95, 5000, 3, 0,
	)
	require.NoError(t, err)

	assert.Equal(t, "nvme0n1", pred.Device)
	assert.Equal(t, "Samsung 980 PRO", pred.Model)
	assert.Equal(t, 600.0, pred.TotalWriteCapacityTB)
	assert.Equal(t, 100.0, pred.TotalWrittenTB)
	assert.True(t, pred.RemainingLifePercent > 0)
	assert.True(t, pred.RemainingLifePercent <= 100)
	assert.NotEmpty(t, pred.WearLevel)
	assert.NotEmpty(t, pred.ConfidenceLevel)
	assert.Equal(t, uint64(5000), pred.PowerOnHours)
}

func TestHealthManager_WritePatternAffectsPrediction(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	// Update write pattern
	hm.UpdateWritePattern("nvme0n1", "nqn.test", 50, 100, 20.0, 140.0, 3.5, 2.0, 60)

	pred, err := hm.PredictDeviceLife(
		nil, "nvme0n1", "nqn.test", "Model", "Serial",
		600, 100, 16, 95, 5000, 0, 0,
	)
	require.NoError(t, err)

	assert.Equal(t, 20.0, pred.DailyWriteRateGB)
	assert.Equal(t, 2.0, pred.WriteAmplification)
	// With 20GB/day writes and 500TB remaining, days should be calculable
	assert.True(t, pred.EstimatedDaysLeft > 0)
}

func TestHealthManager_WearLevelClassification(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	tests := []struct {
		name         string
		tbw          float64
		capacity     float64
		expectedWear string
	}{
		{"low wear", 10, 600, "low"},
		{"medium wear", 250, 600, "medium"},
		{"high wear", 400, 600, "high"},
		{"critical wear", 580, 600, "critical"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pred, err := hm.PredictDeviceLife(
				nil, "nvme0n1", "nqn.test", "Model", "Serial",
				tc.capacity, tc.tbw, 50, 95, 5000, 0, 0,
			)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedWear, pred.WearLevel)
		})
	}
}

func TestHealthManager_Benchmark(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	cfg := DefaultBenchmarkConfig(tmpDir, "nqn.test")
	cfg.FileSizeMB = 1 // Small for fast test
	cfg.TestTypes = []string{"seq_read", "seq_write"}

	result, err := hm.StartBenchmark(nil, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)

	// Wait for completion
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		r, err := hm.GetBenchmarkResult(result.ID)
		require.NoError(t, err)
		if r.Status == "completed" || r.Status == "failed" {
			if r.Status == "completed" {
				assert.NotNil(t, r.Results)
				assert.True(t, r.Results.SeqReadMBps >= 0)
				assert.True(t, r.Results.SeqWriteMBps >= 0)
				assert.True(t, r.Results.OverallScore >= 0)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("benchmark did not complete within timeout")
}

func TestHealthManager_BenchmarkDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	cfg := DefaultBenchmarkConfig(tmpDir, "nqn.test")
	cfg.FileSizeMB = 1

	_, err = hm.StartBenchmark(nil, cfg)
	require.NoError(t, err)

	// Second benchmark on same device should fail immediately
	// (benchRunning is set synchronously, no sleep needed)
	_, err = hm.StartBenchmark(nil, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// 等待 benchmark 完成，避免 TempDir 清理失败
	hm.WaitForBenchmarks()
}

func TestHealthManager_GetDeviceTemperatureStatus_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	_, err = hm.GetDeviceTemperatureStatus("nonexistent")
	assert.Error(t, err)
}

func TestHealthManager_GetLifePrediction_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	_, err = hm.GetLifePrediction("nonexistent")
	assert.Error(t, err)
}

func TestDefaultConfigs(t *testing.T) {
	tc := DefaultTemperatureConfig()
	assert.True(t, tc.Enabled)
	assert.Equal(t, 60, tc.IntervalSec)
	assert.Equal(t, 70.0, tc.WarningThreshold)
	assert.Equal(t, 85.0, tc.CriticalThreshold)
	assert.Equal(t, 1440, tc.MaxHistoryLen)

	lc := DefaultLifePredictionConfig()
	assert.True(t, lc.Enabled)
	assert.Equal(t, 0.02, lc.TempDegradationRate)
	assert.Equal(t, 1.0, lc.WriteDegradationRate)
	assert.Equal(t, 3.0, lc.MaxWriteAmplification)

	bc := DefaultBenchmarkConfig("/dev/nvme0", "nqn.test")
	assert.Equal(t, "/dev/nvme0", bc.DevicePath)
	assert.Equal(t, 64, bc.BlockSizeKB)
	assert.Equal(t, 256, bc.FileSizeMB)
	assert.Equal(t, 30, bc.DurationSec)
	assert.Equal(t, 1, bc.NumThreads)
	assert.Len(t, bc.TestTypes, 4)
}

func TestHealthManager_AllDeviceStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nvmeof.json")
	mgr, err := NewManager(configPath, zap.NewNop())
	require.NoError(t, err)

	hm := NewHealthManager(mgr, configPath, zap.NewNop())

	_ = hm.RecordTemperature(nil, "nvme0n1", "nqn.test", 45.0)
	_ = hm.RecordTemperature(nil, "nvme1n1", "nqn.test", 55.0)

	statuses := hm.GetAllDeviceStatuses()
	assert.Equal(t, 2, len(statuses))
}
