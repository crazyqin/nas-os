package storageforecast

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Manager 单元测试 ---

func TestManager_RegisterPool(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	pool := StoragePool{
		ID:         "pool1",
		Name:       "主存储池",
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  512 * 1024 * 1024 * 1024,  // 512GB
		FreeBytes:  512 * 1024 * 1024 * 1024,
	}

	mgr.RegisterPool(pool)

	pools := mgr.ListPools()
	if len(pools) != 1 {
		t.Fatalf("应有 1 个存储池，实际 %d", len(pools))
	}

	if pools[0].Name != "主存储池" {
		t.Errorf("池名不匹配: %s", pools[0].Name)
	}
}

func TestManager_UnregisterPool(t *testing.T) {
	mgr := NewManager(DefaultConfig())
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	err := mgr.UnregisterPool("p1")
	if err != nil {
		t.Fatalf("注销失败: %v", err)
	}

	if len(mgr.ListPools()) != 0 {
		t.Error("注销后不应有存储池")
	}

	// 重复注销应报错
	err = mgr.UnregisterPool("p1")
	if err == nil {
		t.Error("重复注销应返回错误")
	}
}

func TestManager_UpdateUsage(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	pool := StoragePool{
		ID:         "pool1",
		Name:       "测试池",
		TotalBytes: 1000,
		UsedBytes:  500,
		FreeBytes:  500,
	}
	mgr.RegisterPool(pool)

	err := mgr.UpdatePoolUsage("pool1", 600)
	if err != nil {
		t.Fatalf("更新使用量失败: %v", err)
	}

	p, _ := mgr.GetPool("pool1")
	if p.UsedBytes != 600 {
		t.Errorf("使用量不匹配: %d", p.UsedBytes)
	}

	if p.FreeBytes != 400 {
		t.Errorf("剩余空间不匹配: %d", p.FreeBytes)
	}

	if p.UsedPercent != 60.0 {
		t.Errorf("使用率不匹配: %f", p.UsedPercent)
	}
}

func TestManager_UpdateNonExistent(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	err := mgr.UpdatePoolUsage("nonexistent", 100)
	if err == nil {
		t.Fatal("不存在的池应返回错误")
	}
}

func TestManager_AlertLevels(t *testing.T) {
	config := DefaultConfig()
	config.WarningThreshold = 80
	config.CriticalThreshold = 90
	config.FullThreshold = 95

	mgr := NewManager(config)

	tests := []struct {
		name     string
		usage    float64
		expected AlertLevel
	}{
		{"正常", 50, AlertInfo},
		{"警告", 85, AlertWarning},
		{"严重", 92, AlertCritical},
		{"满载", 96, AlertFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := StoragePool{
				ID:         tt.name,
				Name:       tt.name,
				TotalBytes: 1000,
				UsedBytes:  int64(tt.usage * 10),
				FreeBytes:  int64((100 - tt.usage) * 10),
			}
			mgr.RegisterPool(pool)
			mgr.UpdatePoolUsage(tt.name, pool.UsedBytes)

			alerts := mgr.GetAlerts(false)
			found := false
			for _, a := range alerts {
				if a.PoolID == tt.name {
					found = true
					if a.Level != tt.expected {
						t.Errorf("告警级别不匹配: %s vs %s", a.Level, tt.expected)
					}
				}
			}

			if tt.expected != AlertInfo && !found {
				t.Errorf("未找到 %s 的告警", tt.name)
			}
		})
	}
}

func TestManager_GetForecast(t *testing.T) {
	config := DefaultConfig()
	config.MinDataPoints = 3
	mgr := NewManager(config)

	pool := StoragePool{
		ID:         "pool1",
		Name:       "测试池",
		TotalBytes: 10000,
		UsedBytes:  5000,
		FreeBytes:  5000,
	}
	mgr.RegisterPool(pool)

	// 数据点不足
	result, _ := mgr.GetForecast("pool1")
	if result.Trend != TrendUnknown {
		t.Error("数据点不足时趋势应为 unknown")
	}

	// 添加足够数据点
	for i := 0; i < 10; i++ {
		mgr.UpdatePoolUsage("pool1", int64(5000+i*100))
		time.Sleep(10 * time.Millisecond)
	}

	result, _ = mgr.GetForecast("pool1")
	if result.PoolID != "pool1" {
		t.Errorf("池 ID 不匹配: %s", result.PoolID)
	}
}

func TestManager_AllForecasts(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})
	mgr.RegisterPool(StoragePool{ID: "p2", Name: "池2", TotalBytes: 2000, UsedBytes: 1000, FreeBytes: 1000})

	results := mgr.GetAllForecasts()
	if len(results) != 2 {
		t.Errorf("应有 2 个预测结果，实际 %d", len(results))
	}
}

func TestManager_Snapshots(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500}
	mgr.RegisterPool(pool)

	for i := 0; i < 5; i++ {
		mgr.UpdatePoolUsage("pool1", int64(500+i*10))
	}

	snapshots := mgr.GetSnapshots("pool1", 1*time.Hour)
	if len(snapshots) < 5 {
		t.Errorf("应至少有 5 个快照，实际 %d", len(snapshots))
	}
}

func TestManager_DismissAlert(t *testing.T) {
	config := DefaultConfig()
	config.WarningThreshold = 50
	mgr := NewManager(config)

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 100, UsedBytes: 60, FreeBytes: 40}
	mgr.RegisterPool(pool)
	mgr.UpdatePoolUsage("pool1", 60)

	alerts := mgr.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("应有告警")
	}

	err := mgr.DismissAlert(alerts[0].ID)
	if err != nil {
		t.Fatalf("忽略告警失败: %v", err)
	}

	activeAlerts := mgr.GetAlerts(false)
	for _, a := range activeAlerts {
		if a.ID == alerts[0].ID {
			t.Error("被忽略的告警不应出现在活跃列表中")
		}
	}
}

func TestManager_Stats(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	stats := mgr.GetStats()
	if stats["total_pools"] != 1 {
		t.Errorf("总池数应为 1，实际 %v", stats["total_pools"])
	}
}

func TestManager_ContextCancel(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	mgr.Start(ctx)

	cancel()
	time.Sleep(100 * time.Millisecond)

	// 不应 panic
	mgr.Stop()
}

func TestManager_StopIdempotent(t *testing.T) {
	mgr := NewManager(DefaultConfig())
	ctx := context.Background()
	mgr.Start(ctx)

	mgr.Stop()
	// 重复调用不应 panic
	mgr.Stop()
}

func TestManager_UpdateConfig(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	config := mgr.GetConfig()
	if config.WarningThreshold != 80.0 {
		t.Errorf("默认阈值应为 80%%，实际 %f", config.WarningThreshold)
	}

	config.WarningThreshold = 75.0
	mgr.UpdateConfig(config)

	updated := mgr.GetConfig()
	if updated.WarningThreshold != 75.0 {
		t.Errorf("更新后阈值应为 75%%，实际 %f", updated.WarningThreshold)
	}
}

func TestManager_TrendSeries(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500}
	mgr.RegisterPool(pool)

	for i := 0; i < 10; i++ {
		mgr.UpdatePoolUsage("pool1", int64(500+i*10))
	}

	series, err := mgr.GetTrendSeries("pool1", GranularityDay, 24*time.Hour)
	if err != nil {
		t.Fatalf("获取趋势失败: %v", err)
	}

	if series.PoolID != "pool1" {
		t.Errorf("池 ID 不匹配: %s", series.PoolID)
	}

	if series.Granularity != GranularityDay {
		t.Errorf("粒度不匹配: %s", series.Granularity)
	}
}

func TestManager_TrendSeriesNotFound(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	_, err := mgr.GetTrendSeries("nonexistent", GranularityDay, 24*time.Hour)
	if err == nil {
		t.Error("不存在的池应返回错误")
	}
}

func TestManager_ExpansionRecommendation(t *testing.T) {
	config := DefaultConfig()
	config.ExpansionTargetDays = 180
	config.CostPerGBMonth = 0.02
	mgr := NewManager(config)

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 1000000, UsedBytes: 500000, FreeBytes: 500000}
	mgr.RegisterPool(pool)

	// 添加增长数据
	for i := 0; i < 10; i++ {
		mgr.UpdatePoolUsage("pool1", int64(500000+i*1000))
	}

	rec, err := mgr.GetExpansionRecommendation("pool1")
	if err != nil {
		t.Fatalf("获取扩容建议失败: %v", err)
	}

	if rec.PoolID != "pool1" {
		t.Errorf("池 ID 不匹配: %s", rec.PoolID)
	}

	if rec.TargetDays != 180 {
		t.Errorf("目标天数不匹配: %d", rec.TargetDays)
	}
}

func TestManager_ExpansionRecommendationNoGrowth(t *testing.T) {
	mgr := NewManager(DefaultConfig())

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500}
	mgr.RegisterPool(pool)

	// 稳定使用量
	for i := 0; i < 10; i++ {
		mgr.UpdatePoolUsage("pool1", 500)
	}

	rec, err := mgr.GetExpansionRecommendation("pool1")
	if err != nil {
		t.Fatalf("获取扩容建议失败: %v", err)
	}

	if rec.Urgency != "none" {
		t.Errorf("无增长时紧急程度应为 none，实际 %s", rec.Urgency)
	}
}

func TestManager_CostEstimate(t *testing.T) {
	config := DefaultConfig()
	config.CostPerGBMonth = 0.02
	config.CostCurrency = "USD"
	mgr := NewManager(config)

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 1073741824, UsedBytes: 536870912, FreeBytes: 536870912} // 1GB
	mgr.RegisterPool(pool)

	est, err := mgr.GetCostEstimate("pool1")
	if err != nil {
		t.Fatalf("获取成本估算失败: %v", err)
	}

	if est.Currency != "USD" {
		t.Errorf("货币不匹配: %s", est.Currency)
	}

	if est.MonthlyCost <= 0 {
		t.Error("月成本应大于 0")
	}
}

// --- FormatBytes 测试 ---

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, 期望 %s", tt.bytes, result, tt.expected)
		}
	}
}

// --- HTTP Handlers 测试 ---

func setupTestHandlers() (*Manager, *Handlers) {
	config := DefaultConfig()
	config.MinDataPoints = 3
	mgr := NewManager(config)
	handlers := NewHandlers(mgr)
	return mgr, handlers
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	_, handlers := setupTestHandlers()
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, "/api/storageforecast")

	// 验证路由注册不 panic
	if mux == nil {
		t.Fatal("mux 不应为 nil")
	}
}

func TestHandlers_HandlePools_GET(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/pools", nil)
	w := httptest.NewRecorder()

	handlers.handlePools(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}

	var resp response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 200 {
		t.Errorf("响应码应为 200，实际 %d", resp.Code)
	}
}

func TestHandlers_HandlePools_POST(t *testing.T) {
	_, handlers := setupTestHandlers()

	body := `{"id":"p1","name":"新池","total_bytes":1000,"used_bytes":0,"free_bytes":1000}`
	req := httptest.NewRequest(http.MethodPost, "/pools", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.handlePools(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为 201，实际 %d", w.Code)
	}
}

func TestHandlers_HandlePools_POST_Invalid(t *testing.T) {
	_, handlers := setupTestHandlers()

	// 缺少必填字段
	body := `{"name":"新池"}`
	req := httptest.NewRequest(http.MethodPost, "/pools", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.handlePools(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("状态码应为 400，实际 %d", w.Code)
	}
}

func TestHandlers_HandlePools_DELETE(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodDelete, "/pools?pool_id=p1", nil)
	w := httptest.NewRecorder()

	handlers.handlePools(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandlePools_MethodNotAllowed(t *testing.T) {
	_, handlers := setupTestHandlers()

	req := httptest.NewRequest(http.MethodPatch, "/pools", nil)
	w := httptest.NewRecorder()

	handlers.handlePools(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("状态码应为 405，实际 %d", w.Code)
	}
}

func TestHandlers_HandleUpdateUsage(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	body := `{"pool_id":"p1","used_bytes":600}`
	req := httptest.NewRequest(http.MethodPost, "/pools/usage", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.handleUpdateUsage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleForecast(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/forecast?pool_id=p1", nil)
	w := httptest.NewRecorder()

	handlers.handleForecast(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleForecast_MissingPoolID(t *testing.T) {
	_, handlers := setupTestHandlers()

	req := httptest.NewRequest(http.MethodGet, "/forecast", nil)
	w := httptest.NewRecorder()

	handlers.handleForecast(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("状态码应为 400，实际 %d", w.Code)
	}
}

func TestHandlers_HandleAllForecasts(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/forecast/all", nil)
	w := httptest.NewRecorder()

	handlers.handleAllForecasts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleTrends(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/trends?pool_id=p1&granularity=day&hours=24", nil)
	w := httptest.NewRecorder()

	handlers.handleTrends(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleExpansion(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/expansion?pool_id=p1", nil)
	w := httptest.NewRecorder()

	handlers.handleExpansion(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleCost(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/cost?pool_id=p1", nil)
	w := httptest.NewRecorder()

	handlers.handleCost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleAlerts(t *testing.T) {
	config := DefaultConfig()
	config.WarningThreshold = 50
	mgr := NewManager(config)
	handlers := NewHandlers(mgr)

	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 100, UsedBytes: 60, FreeBytes: 40})
	mgr.UpdatePoolUsage("p1", 60)

	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	w := httptest.NewRecorder()

	handlers.handleAlerts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleDismissAlert(t *testing.T) {
	config := DefaultConfig()
	config.WarningThreshold = 50
	mgr := NewManager(config)
	handlers := NewHandlers(mgr)

	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 100, UsedBytes: 60, FreeBytes: 40})
	mgr.UpdatePoolUsage("p1", 60)

	alerts := mgr.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("应有告警")
	}

	body := `{"alert_id":"` + alerts[0].ID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/alerts/dismiss", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.handleDismissAlert(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleSnapshots(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/snapshots?pool_id=p1&hours=24", nil)
	w := httptest.NewRecorder()

	handlers.handleSnapshots(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleStats(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()

	handlers.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleConfig_GET(t *testing.T) {
	_, handlers := setupTestHandlers()

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()

	handlers.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_HandleConfig_PUT(t *testing.T) {
	_, handlers := setupTestHandlers()

	body := `{"enabled":true,"warning_threshold":75}`
	req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handlers.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，实际 %d", w.Code)
	}
}

func TestHandlers_GetForecastSummary(t *testing.T) {
	mgr, handlers := setupTestHandlers()
	mgr.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})
	mgr.RegisterPool(StoragePool{ID: "p2", Name: "池2", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	summary := handlers.GetForecastSummary()

	if summary["total_pools"] != 2 {
		t.Errorf("总池数应为 2，实际 %v", summary["total_pools"])
	}
}
