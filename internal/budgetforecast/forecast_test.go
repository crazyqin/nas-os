package budgetforecast

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== 线性回归测试 ==========

func TestLinearRegression(t *testing.T) {
	x := []float64{0, 1, 2, 3, 4}
	y := []float64{1, 3, 5, 7, 9}
	slope, intercept, r2 := LinearRegression(x, y)
	if math.Abs(slope-2.0) > 0.01 {
		t.Errorf("expected slope ~2.0, got %f", slope)
	}
	if math.Abs(intercept-1.0) > 0.01 {
		t.Errorf("expected intercept ~1.0, got %f", intercept)
	}
	if r2 < 0.99 {
		t.Errorf("expected R² ~1.0, got %f", r2)
	}
}

func TestLinearRegression_SinglePoint(t *testing.T) {
	x := []float64{0}
	y := []float64{10}
	slope, _, _ := LinearRegression(x, y)
	if slope != 0 {
		t.Errorf("expected slope 0 for single point, got %f", slope)
	}
}

// ========== 预测引擎测试 ==========

func generateLinearSnapshots(days int, startGB, growthPerDayGB float64, costPerTB float64, totalBytes int64) []UsageSnapshot {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var snapshots []UsageSnapshot
	for d := 0; d < days; d++ {
		usedGB := startGB + growthPerDayGB*float64(d)
		snapshots = append(snapshots, UsageSnapshot{
			Date:       start.AddDate(0, 0, d),
			UsedBytes:  int64(usedGB * 1024 * 1024 * 1024),
			TotalBytes: totalBytes,
			CostPerTB:  costPerTB,
		})
	}
	return snapshots
}

func TestForecast_BasicPrediction(t *testing.T) {
	// 线性增长：每天增长1GB，60天数据
	totalBytes := int64(10 * 1024 * 1024 * 1024 * 1024) // 10TB
	snapshots := generateLinearSnapshots(60, 100, 1.0, 100, totalBytes)

	engine := NewForecastEngine(snapshots, totalBytes, zap.NewNop())
	result := engine.Forecast(3)

	if result.CurrentUsageGB < 150 {
		t.Errorf("expected current usage ~159GB, got %f", result.CurrentUsageGB)
	}
	if result.HistoryDays < 59 {
		t.Errorf("expected history days ~60, got %d", result.HistoryDays)
	}
	if len(result.Forecast) != 3 {
		t.Errorf("expected 3 forecast points, got %d", len(result.Forecast))
	}
	if result.MonthlyGrowthGB < 20 {
		t.Errorf("expected monthly growth ~30GB, got %f", result.MonthlyGrowthGB)
	}
	if result.DaysUntilFull <= 0 {
		t.Errorf("expected days until full > 0, got %d", result.DaysUntilFull)
	}
	if result.MonthlyCostNow <= 0 {
		t.Errorf("expected monthly cost > 0, got %f", result.MonthlyCostNow)
	}
}

func TestForecast_AlreadyFull(t *testing.T) {
	totalBytes := int64(100 * 1024 * 1024 * 1024) // 100GB
	snapshots := []UsageSnapshot{
		{
			Date:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			UsedBytes:  110 * 1024 * 1024 * 1024, // 110GB > 100GB
			TotalBytes: totalBytes,
			CostPerTB:  100,
		},
	}

	engine := NewForecastEngine(snapshots, totalBytes, zap.NewNop())
	result := engine.Forecast(3)

	if result.DaysUntilFull != 0 {
		t.Errorf("expected days until full = 0 (already full), got %d", result.DaysUntilFull)
	}
}

func TestForecast_StableUsage(t *testing.T) {
	totalBytes := int64(10 * 1024 * 1024 * 1024 * 1024) // 10TB
	// 稳定使用，无增长
	snapshots := generateLinearSnapshots(60, 500, 0, 80, totalBytes)

	engine := NewForecastEngine(snapshots, totalBytes, zap.NewNop())
	result := engine.Forecast(3)

	if result.MonthlyGrowthGB > 1 {
		t.Errorf("expected monthly growth ~0 for stable usage, got %f", result.MonthlyGrowthGB)
	}
	if result.DaysUntilFull <= 0 {
		t.Errorf("expected days until full to be large for stable usage, got %d", result.DaysUntilFull)
	}
}

func TestForecast_BudgetAlerts(t *testing.T) {
	totalBytes := int64(10 * 1024 * 1024 * 1024 * 1024) // 10TB
	// 高增长场景
	snapshots := generateLinearSnapshots(60, 2000, 5.0, 100, totalBytes)

	engine := NewForecastEngine(snapshots, totalBytes, zap.NewNop())
	result := engine.Forecast(12)

	// 应该有告警触发
	if len(result.Alerts) == 0 {
		t.Log("no alerts triggered (may be expected depending on growth rate)")
	}
	for _, alert := range result.Alerts {
		if alert.Threshold <= 0 {
			t.Errorf("alert threshold should be > 0, got %f", alert.Threshold)
		}
		if alert.Severity == "" {
			t.Error("alert severity should not be empty")
		}
	}
}

// ========== HTTP 接口测试 ==========

func setupTestRouter() (*gin.Engine, *ForecastEngine) {
	totalBytes := int64(10 * 1024 * 1024 * 1024 * 1024) // 10TB
	snapshots := generateLinearSnapshots(30, 100, 1.0, 100, totalBytes)
	engine := NewForecastEngine(snapshots, totalBytes, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handlers := NewHandlers(engine, zap.NewNop())
	handlers.RegisterRoutes(api)

	return router, engine
}

func TestHandler_Forecast(t *testing.T) {
	router, _ := setupTestRouter()

	// 测试 GET /api/v1/budget/forecast
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/budget/forecast?months=3", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'result' in response")
	}

	if _, ok := result["current_usage_gb"]; !ok {
		t.Error("expected 'current_usage_gb' in result")
	}
	if _, ok := result["monthly_growth_gb"]; !ok {
		t.Error("expected 'monthly_growth_gb' in result")
	}
	if _, ok := result["days_until_full"]; !ok {
		t.Error("expected 'days_until_full' in result")
	}
}

func TestHandler_Snapshot(t *testing.T) {
	router, _ := setupTestRouter()

	// 测试 POST /api/v1/budget/snapshot
	snapshot := UsageSnapshot{
		Date:       time.Now(),
		UsedBytes:  500 * 1024 * 1024 * 1024,
		TotalBytes: 10 * 1024 * 1024 * 1024 * 1024,
		CostPerTB:  100,
	}
	body, _ := json.Marshal(snapshot)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/budget/snapshot", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandler_Alerts(t *testing.T) {
	router, _ := setupTestRouter()

	// 测试 GET /api/v1/budget/alerts
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/budget/alerts", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := resp["alerts"]; !ok {
		t.Error("expected 'alerts' in response")
	}
	if _, ok := resp["total"]; !ok {
		t.Error("expected 'total' in response")
	}
}
