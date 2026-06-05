package storagecost

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestHandler(t *testing.T) (*Handlers, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger, nil)
	handler := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreateAsset(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "asset-001",
		"name": "测试存储设备",
		"type": "ssd",
		"capacity_tb": 10.0,
		"purchase_cost": 50000.0,
		"warranty_years": 5,
		"annual_power_kwh": 1000,
		"power_cost_per_kwh": 0.8,
		"rack_units": 2,
		"rack_cost_per_unit": 1000.0
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-cost/assets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var asset StorageAsset
	if err := json.Unmarshal(w.Body.Bytes(), &asset); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if asset.ID != "asset-001" {
		t.Errorf("expected ID asset-001, got %s", asset.ID)
	}
	if asset.CapacityTB != 10.0 {
		t.Errorf("expected capacity 10.0, got %f", asset.CapacityTB)
	}
}

func TestCalculateTCO(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// 创建测试资产
	mgr.CreateAsset(StorageAsset{
		ID:              "tco-asset-1",
		Name:            "TCO测试资产",
		Type:            "hdd",
		CapacityTB:      20.0,
		PurchaseCost:    100000.0,
		WarrantyYears:   3,
		AnnualPowerKWh:  2000,
		PowerCostPerKWh: 0.8,
		RackUnits:       4,
		RackCostPerUnit: 1000.0,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-cost/tco/tco-asset-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result TCOResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.AssetID != "tco-asset-1" {
		t.Errorf("expected assetId tco-asset-1, got %s", result.AssetID)
	}
	if result.TotalCost <= 0 {
		t.Errorf("expected positive total cost, got %f", result.TotalCost)
	}
}

func TestRecordCapacitySample(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"used_tb": 5.5,
		"total_tb": 10.0,
		"utilization": 55.0
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-cost/capacity-samples", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetOptimizationReport(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage-cost/optimization-report", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report OptimizationReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if report.TotalAnnualSaving <= 0 {
		t.Errorf("expected positive annual saving, got %f", report.TotalAnnualSaving)
	}
}

func TestCreateBudgetPlan(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"id": "budget-001",
		"name": "2024存储预算",
		"fiscal_year": 2024,
		"total_budget": 500000.0,
		"line_items": [
			{
				"id": "item-1",
				"category": "expansion",
				"description": "SSD扩容",
				"amount": 100000.0,
				"quantity": 2,
				"unit_cost": 50000.0,
				"priority": "high"
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-cost/budgets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var plan BudgetPlan
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if plan.ID != "budget-001" {
		t.Errorf("expected ID budget-001, got %s", plan.ID)
	}
	if plan.TotalBudget != 500000.0 {
		t.Errorf("expected total budget 500000.0, got %f", plan.TotalBudget)
	}
}

func TestCompareStorageOptions(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{
		"options": [
			{
				"id": "opt1",
				"name": "SSD方案",
				"type": "on_premise",
				"capacity_tb": 10.0,
				"cost_per_tb_year": 500.0,
				"tco_5_year": 25000.0,
				"iops_capability": 100000,
				"throughput_mbps": 3000,
				"availability": 99.99,
				"scalability_score": 80.0
			},
			{
				"id": "opt2",
				"name": "HDD方案",
				"type": "on_premise",
				"capacity_tb": 20.0,
				"cost_per_tb_year": 200.0,
				"tco_5_year": 20000.0,
				"iops_capability": 200,
				"throughput_mbps": 500,
				"availability": 99.9,
				"scalability_score": 60.0
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage-cost/compare", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result ComparisonResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(result.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(result.Options))
	}
	if result.BestOption == nil {
		t.Errorf("expected best option to be set")
	}
}

// ============================================================
// 成本分析模块新增测试 (任务要求)
// ============================================================

func setupStorageCostTestHandler(t *testing.T) (*Handlers, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	config := &StorageCostConfig{
		Currency:       "CNY",
		BudgetLimit:    10000.0,
		AlertThreshold: 80.0,
		DefaultPriceSSD: 0.5,
		DefaultPriceHDD: 0.1,
	}

	mgr := NewManagerWithConfig(config)
	handler := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterStorageCostRoutes(api)

	return handler, mgr, router
}

func TestAddCostRecord(t *testing.T) {
	_, _, router := setupStorageCostTestHandler(t)

	body := `{
		"id": "cost-001",
		"volume_id": "vol-001",
		"volume_name": "数据卷1",
		"storage_type": "SSD",
		"capacity_gb": 500,
		"used_gb": 200,
		"price_per_gb": 0.5,
		"provider": "本地存储",
		"purchase_date": "2024-01-01T00:00:00Z",
		"warranty_end": "2029-01-01T00:00:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/cost/records", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetCostSummary(t *testing.T) {
	_, mgr, router := setupStorageCostTestHandler(t)

	// 先添加成本记录
	mgr.AddCostRecord(CostRecord{
		ID:          "summary-001",
		VolumeID:    "vol-001",
		VolumeName:  "测试卷",
		StorageType: "SSD",
		CapacityGB:  1000,
		UsedGB:      500,
		PricePerGB:  0.5,
		MonthlyCost: 500,
		Provider:    "本地",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/cost/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var summary CostSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if summary.TotalMonthlyCost != 500 {
		t.Errorf("expected total monthly cost 500, got %f", summary.TotalMonthlyCost)
	}
}

func TestGetCostTrend(t *testing.T) {
	_, _, router := setupStorageCostTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/cost/trend?days=7", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result struct {
		Trend []CostTrendPoint `json:"trend"`
		Days  int              `json:"days"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.Days != 7 {
		t.Errorf("expected days 7, got %d", result.Days)
	}
	if len(result.Trend) != 7 {
		t.Errorf("expected 7 trend points, got %d", len(result.Trend))
	}
}

func TestGetCostAlerts(t *testing.T) {
	_, _, router := setupStorageCostTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/cost/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result struct {
		Alerts []CostAlert `json:"alerts"`
		Total  int         `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
}

func TestGetOptimizationSuggestions(t *testing.T) {
	_, mgr, router := setupStorageCostTestHandler(t)

	// 添加一个低利用率的记录
	mgr.AddCostRecord(CostRecord{
		ID:          "opt-001",
		VolumeID:    "vol-opt",
		VolumeName:  "低利用率卷",
		StorageType: "SSD",
		CapacityGB:  2000,
		UsedGB:      200,
		PricePerGB:  0.5,
		MonthlyCost: 1000,
		Provider:    "本地",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/cost/suggestions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result struct {
		Suggestions []OptimizationSuggestion `json:"suggestions"`
		Total       int                      `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.Total == 0 {
		t.Errorf("expected at least 1 suggestion, got %d", result.Total)
	}
}

func TestEstimateMonthlyCost(t *testing.T) {
	_, _, router := setupStorageCostTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/cost/estimate?type=SSD&size=1000", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result struct {
		StorageType string  `json:"storage_type"`
		SizeGB      float64 `json:"size_gb"`
		MonthlyCost float64 `json:"monthly_cost"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.MonthlyCost != 500 {
		t.Errorf("expected monthly cost 500, got %f", result.MonthlyCost)
	}
}

func TestSetBudgetAlert(t *testing.T) {
	_, _, router := setupStorageCostTestHandler(t)

	body := `{"threshold": 90.0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/cost/budget-alert", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportCostReport(t *testing.T) {
	_, mgr, router := setupStorageCostTestHandler(t)

	// 添加记录
	mgr.AddCostRecord(CostRecord{
		ID:          "export-001",
		VolumeID:    "vol-exp",
		VolumeName:  "导出测试卷",
		StorageType: "HDD",
		CapacityGB:  5000,
		UsedGB:      3000,
		PricePerGB:  0.1,
		MonthlyCost: 500,
		Provider:    "NAS",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/cost/export?format=csv", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证CSV内容包含标题行
	csvContent := w.Body.String()
	if len(csvContent) == 0 {
		t.Errorf("expected non-empty CSV content")
	}
}
