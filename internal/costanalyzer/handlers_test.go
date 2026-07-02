package costanalyzer

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

func setupTestEnv(t *testing.T) (*Handlers, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	logger, _ := zap.NewDevelopment()
	cfg := DefaultSmartCostConfig()
	mgr := NewManager(logger, cfg)
	handler := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

// ============================================================
// 资产管理测试
// ============================================================

func TestAddAsset(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{
		"id": "asset-001",
		"name": "NVMe-Storage-1",
		"type": "nvme",
		"capacity_bytes": 1099511627776,
		"used_bytes": 549755813888,
		"purchase_cost": 80000,
		"monthly_opex": 500,
		"warranty_years": 5,
		"purchase_date": "2024-01-15",
		"pool": "pool-main",
		"provider": "本地"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/assets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp apiResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestAddAssetMissingID(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{"name": "no-id", "type": "ssd", "capacity_bytes": 100}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/assets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAssets(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	mgr.AddAsset(&StorageAsset{
		ID: "a1", Name: "Test-1", Type: StorageTypeSSD,
		CapacityBytes: 1 << 30, UsedBytes: 512 << 20,
		PurchaseDate: time.Now().AddDate(0, -6, 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/assets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp.Data)
}

func TestGetAsset(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	mgr.AddAsset(&StorageAsset{
		ID: "get-1", Name: "GetTest", Type: StorageTypeHDD,
		CapacityBytes: 2 << 30, UsedBytes: 1 << 30,
		PurchaseDate: time.Now(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/assets/get-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetAssetNotFound(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/assets/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveAsset(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	mgr.AddAsset(&StorageAsset{
		ID: "rm-1", Name: "RemoveTest", Type: StorageTypeSSD,
		CapacityBytes: 1 << 30, UsedBytes: 100 << 20,
		PurchaseDate: time.Now(),
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/smart-cost/assets/rm-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// ============================================================
// 成本记录测试
// ============================================================

func TestRecordCost(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{
		"asset_id": "asset-001",
		"storage_type": "ssd",
		"capacity_gb": 500,
		"used_gb": 200,
		"price_per_gb_month": 0.5,
		"total_cost": 100,
		"period_start": "2025-01-01",
		"period_end": "2025-01-31"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/costs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestRecordCostMissingAssetID(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{"storage_type": "ssd", "total_cost": 50}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/costs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListCosts(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/costs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// ============================================================
// 成本汇总与趋势测试
// ============================================================

func TestGetCostSummary(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
}

func TestGetCostSummaryWithDates(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/smart-cost/summary?period_start=2025-01-01&period_end=2025-06-30", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyzeTrend(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/trend?granularity=monthly&months=6", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
}

func TestAnalyzeTrendDefaults(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/trend", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// ============================================================
// 优化建议测试
// ============================================================

func TestGetOptimizations(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/optimizations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)

	// 应该有至少一条建议（压缩/清理等基础建议）
	data, _ := json.Marshal(resp.Data)
	var suggestions []OptimizationSuggestion
	json.Unmarshal(data, &suggestions)
	assert.Greater(t, len(suggestions), 0, "should have at least 1 suggestion")
}

func TestGetColdData(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	// 添加一个"冷"资产（购买于 200 天前）
	mgr.AddAsset(&StorageAsset{
		ID: "cold-1", Name: "ColdAsset", Type: StorageTypeSSD,
		CapacityBytes: 1 << 30, UsedBytes: 800 << 20,
		PurchaseDate: time.Now().AddDate(0, 0, -200),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/cold-data", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// ============================================================
// ROI 计算测试
// ============================================================

func TestCalculateROI(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{
		"investment_cost": 100000,
		"annual_saving": 50000,
		"annual_opex": 10000,
		"project_years": 5,
		"discount_rate": 0.08
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/roi", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp apiResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)

	// 反序列化 ROI 结果
	data, _ := json.Marshal(resp.Data)
	var result ROIResult
	json.Unmarshal(data, &result)
	assert.Greater(t, result.ROIPercent, 0.0, "ROI should be positive")
	assert.Greater(t, result.PaybackMonths, 0.0, "payback should be positive")
	assert.Len(t, result.AnnualBreakdown, 5, "should have 5 annual entries")
}

func TestCalculateROIInvalidInput(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{"investment_cost": 100000}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/roi", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================
// 报告测试
// ============================================================

func TestGenerateReport(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{
		"report_name": "2025年6月成本报告",
		"period_start": "2025-06-01",
		"period_end": "2025-06-30"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/reports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestGenerateReportInvalidDate(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{
		"report_name": "bad date",
		"period_start": "not-a-date",
		"period_end": "2025-06-30"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/reports", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListReports(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/reports", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetReport(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	report := mgr.GenerateReport("测试报告", time.Now().AddDate(0, -1, 0), time.Now())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/reports/"+report.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGetReportNotFound(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/reports/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExportReportCSV(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	report := mgr.GenerateReport("CSV导出测试", time.Now().AddDate(0, -1, 0), time.Now())

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/smart-cost/reports/"+report.ID+"/export?format=csv", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Body.String(), "字段,值")
}

func TestExportReportJSON(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	report := mgr.GenerateReport("JSON导出测试", time.Now().AddDate(0, -1, 0), time.Now())

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/smart-cost/reports/"+report.ID+"/export?format=json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestExportReportNotFound(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/smart-cost/reports/nonexistent/export?format=csv", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================================
// 配置测试
// ============================================================

func TestGetConfig(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smart-cost/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateConfig(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{
		"enabled": true,
		"default_currency": "USD",
		"cold_threshold_days": 60,
		"utilization_warn_pct": 25.0
	}`

	req := httptest.NewRequest(http.MethodPut, "/api/v1/smart-cost/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateConfigInvalidJSON(t *testing.T) {
	_, _, router := setupTestEnv(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/smart-cost/config", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================
// 分析引擎单元测试
// ============================================================

func TestAnalyzerCalculateCostForAsset(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	asset := &StorageAsset{
		Type:      StorageTypeSSD,
		UsedBytes: 100 << 30, // 100 GB
	}

	cost := analyzer.CalculateCostForAsset(asset)
	// 100 GB * 0.50 CNY/GB/月 = 50.0
	assert.InDelta(t, 50.0, cost, 0.01)
}

func TestAnalyzerCalculateCostForAssetNil(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	cost := analyzer.CalculateCostForAsset(nil)
	assert.Equal(t, 0.0, cost)
}

func TestAnalyzerCalculateCostForAssetUnknownType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	asset := &StorageAsset{
		Type:      StorageTypeUnknown,
		UsedBytes: 100 << 30,
	}

	cost := analyzer.CalculateCostForAsset(asset)
	// 100 GB * 0.10 (default) = 10.0
	assert.InDelta(t, 10.0, cost, 0.01)
}

func TestAnalyzerCalculateROI(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	input := &ROIInput{
		InvestmentCost: 100000,
		AnnualSaving:   60000,
		AnnualOpex:     10000,
		ProjectYears:   3,
		DiscountRate:   0.10,
	}

	result, err := analyzer.CalculateROI(input)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, result.ROIPercent, 0.0)
	assert.Len(t, result.AnnualBreakdown, 3)
	assert.Greater(t, result.NPV, 0.0)
}

func TestAnalyzerCalculateROIInvalid(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	_, err := analyzer.CalculateROI(nil)
	assert.Error(t, err)

	_, err = analyzer.CalculateROI(&ROIInput{ProjectYears: 0})
	assert.Error(t, err)

	_, err = analyzer.CalculateROI(&ROIInput{ProjectYears: 3, DiscountRate: -0.5})
	assert.Error(t, err)
}

func TestAnalyzerDetectColdData(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultSmartCostConfig()
	cfg.ColdThresholdDays = 90
	analyzer := NewAnalyzer(logger, cfg)

	assets := []*StorageAsset{
		{
			ID: "warm-1", Name: "WarmAsset", Type: StorageTypeSSD,
			CapacityBytes: 1 << 30, UsedBytes: 500 << 20,
			PurchaseDate: time.Now().AddDate(0, 0, -60), // 60 天，不冷
		},
		{
			ID: "cold-1", Name: "ColdAsset", Type: StorageTypeSSD,
			CapacityBytes: 2 << 30, UsedBytes: 1500 << 20,
			PurchaseDate: time.Now().AddDate(0, 0, -200), // 200 天，冷
		},
	}

	cold := analyzer.DetectColdData(assets, time.Now())
	// 只有 cold-1 应该被检测到
	assert.Len(t, cold, 1)
	if len(cold) > 0 {
		assert.Equal(t, "cold-1", cold[0].AssetID)
	}
}

func TestAnalyzerGenerateOptimizations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	assets := []*StorageAsset{
		{
			ID: "opt-1", Name: "TestAsset", Type: StorageTypeSSD,
			CapacityBytes: 10 << 30, UsedBytes: 1 << 30, // 10% 利用率
			PurchaseDate: time.Now(),
		},
	}

	coldData := []*ColdDataInfo{
		{
			AssetID:       "cold-1",
			CurrentType:   StorageTypeSSD,
			SuggestedType: StorageTypeHDD,
			PotentialSave: 500.0,
		},
	}

	suggestions := analyzer.GenerateOptimizations(assets, coldData)
	assert.Greater(t, len(suggestions), 0)
	// 第一条建议应该是冷数据迁移
	assert.Equal(t, StrategyColdMigration, suggestions[0].Strategy)
}

func TestAnalyzerAnalyzeTrend(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	// 空数据应该生成模拟趋势
	trend := analyzer.AnalyzeTrend(nil, TrendMonthly, 6)
	assert.NotNil(t, trend)
	assert.Len(t, trend.Points, 6)
	assert.Equal(t, TrendMonthly, trend.Granularity)
}

func TestAnalyzerBucketKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	analyzer := NewAnalyzer(logger, DefaultSmartCostConfig())

	date := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, "2025-06-15", analyzer.bucketKey(date, TrendDaily))
	assert.Equal(t, "2025-06", analyzer.bucketKey(date, TrendMonthly))
	assert.Equal(t, "2025", analyzer.bucketKey(date, TrendYearly))

	weekKey := analyzer.bucketKey(date, TrendWeekly)
	assert.Contains(t, weekKey, "2025-W")
}

// ============================================================
// Manager 单元测试
// ============================================================

func TestManagerDefaultConfig(t *testing.T) {
	mgr := NewManager(nil, nil)
	cfg := mgr.GetConfig()
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "CNY", cfg.DefaultCurrency)
}

func TestManagerUpdateConfig(t *testing.T) {
	mgr := NewManager(nil, nil)

	newCfg := &SmartCostConfig{
		Enabled:         true,
		DefaultCurrency: "USD",
	}

	mgr.UpdateConfig(newCfg)
	cfg := mgr.GetConfig()
	assert.Equal(t, "USD", cfg.DefaultCurrency)
}

func TestManagerAddAndGetAsset(t *testing.T) {
	mgr := NewManager(nil, nil)

	err := mgr.AddAsset(&StorageAsset{
		ID: "test-1", Name: "TestAsset", Type: StorageTypeSSD,
		CapacityBytes: 1 << 30, UsedBytes: 500 << 20,
		PurchaseDate: time.Now(),
	})
	require.NoError(t, err)

	asset, err := mgr.GetAsset("test-1")
	require.NoError(t, err)
	assert.Equal(t, "TestAsset", asset.Name)
}

func TestManagerAddAssetNil(t *testing.T) {
	mgr := NewManager(nil, nil)
	err := mgr.AddAsset(nil)
	assert.Error(t, err)
}

func TestManagerAddAssetNoID(t *testing.T) {
	mgr := NewManager(nil, nil)
	err := mgr.AddAsset(&StorageAsset{Name: "no-id"})
	assert.Error(t, err)
}

func TestManagerRecordCost(t *testing.T) {
	mgr := NewManager(nil, nil)

	err := mgr.RecordCost(&CostEntry{
		AssetID:     "asset-1",
		StorageType: StorageTypeSSD,
		CapacityGB:  500,
		UsedGB:      200,
		PricePerGB:  0.5,
		TotalCost:   100,
	})
	require.NoError(t, err)

	entries := mgr.ListCostEntries()
	assert.Len(t, entries, 1)
}

func TestManagerRecordCostNil(t *testing.T) {
	mgr := NewManager(nil, nil)
	err := mgr.RecordCost(nil)
	assert.Error(t, err)
}

func TestManagerGenerateReport(t *testing.T) {
	mgr := NewManager(nil, nil)

	report := mgr.GenerateReport("测试", time.Now().AddDate(0, -1, 0), time.Now())
	assert.NotEmpty(t, report.ID)
	assert.NotEmpty(t, report.ReportName)
	assert.NotNil(t, report.Summary)
}

func TestManagerGetReportNotFound(t *testing.T) {
	mgr := NewManager(nil, nil)
	_, err := mgr.GetReport("nonexistent")
	assert.Error(t, err)
}

func TestManagerExportReportAsCSV(t *testing.T) {
	mgr := NewManager(nil, nil)

	report := mgr.GenerateReport("CSV测试", time.Now().AddDate(0, -1, 0), time.Now())

	csv, err := mgr.ExportReportAsCSV(report.ID)
	require.NoError(t, err)
	assert.Contains(t, csv, "字段,值")
	assert.Contains(t, csv, "CSV测试")
}

func TestManagerExportReportAsCSVNotFound(t *testing.T) {
	mgr := NewManager(nil, nil)
	_, err := mgr.ExportReportAsCSV("nonexistent")
	assert.Error(t, err)
}

func TestManagerRemoveAsset(t *testing.T) {
	mgr := NewManager(nil, nil)

	mgr.AddAsset(&StorageAsset{
		ID: "rm-test", Name: "Remove", Type: StorageTypeSSD,
		CapacityBytes: 1 << 30, UsedBytes: 100 << 20,
		PurchaseDate: time.Now(),
	})

	err := mgr.RemoveAsset("rm-test")
	require.NoError(t, err)

	_, err = mgr.GetAsset("rm-test")
	assert.Error(t, err)
}

func TestManagerRemoveAssetNotFound(t *testing.T) {
	mgr := NewManager(nil, nil)
	err := mgr.RemoveAsset("nonexistent")
	assert.Error(t, err)
}

func TestAnalyzeBudgetCapacityHandler(t *testing.T) {
	_, mgr, router := setupTestEnv(t)

	require.NoError(t, mgr.AddAsset(&StorageAsset{
		ID:            "budget-ssd",
		Name:          "Budget SSD",
		Type:          StorageTypeSSD,
		CapacityBytes: 1000 << 30,
		UsedBytes:     800 << 30,
		MonthlyOpex:   25,
		PurchaseDate:  time.Now().AddDate(-1, 0, 0),
	}))

	body := `{
		"monthly_budget": 600,
		"monthly_growth_gb": 50,
		"planning_months": 6,
		"target_utilization_pct": 80,
		"expansion_cost_per_gb": 1.2
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/budget-capacity", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)

	data, _ := json.Marshal(resp.Data)
	var report BudgetCapacityReport
	require.NoError(t, json.Unmarshal(data, &report))
	assert.Equal(t, 1000.0, report.TotalCapacityGB)
	assert.Equal(t, "warning", report.BudgetStatus)
	assert.Greater(t, report.ExpansionNeededGB, 0.0)
}

func TestAnalyzeBudgetCapacityHandlerInvalidInput(t *testing.T) {
	_, _, router := setupTestEnv(t)

	body := `{"monthly_growth_gb": -1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smart-cost/budget-capacity", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
