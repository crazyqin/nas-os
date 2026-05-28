package storagecost

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestHandler(t *testing.T) (*Handlers, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mgr := NewManager()
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
