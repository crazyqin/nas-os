// Package cost_analysis 提供成本分析API测试
package cost_analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestAPI(t *testing.T) (*CostAnalysisEngine, *APIHandler) {
	billing := &mockBillingProvider{storagePrice: 0.1}
	quota := &mockQuotaProvider{
		usages: []*QuotaUsageInfo{
			{
				QuotaID:      "quota-1",
				TargetID:     "user1",
				TargetName:   "User One",
				VolumeName:   "pool1",
				HardLimit:    100 * 1024 * 1024 * 1024,
				UsedBytes:    80 * 1024 * 1024 * 1024,
				UsagePercent: 80,
			},
		},
	}
	config := DefaultAnalysisConfig()

	engine := NewCostAnalysisEngine(uniqueDataDir("api"), billing, quota, config)
	handler := NewAPIHandler(engine)

	return engine, handler
}

func TestAPIStorageTrendReport(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/reports/storage-trend?days=30", nil)
	w := httptest.NewRecorder()

	handler.HandleStorageTrendReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var report CostReport
	err := json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.NotEmpty(t, report.ID)
	assert.Equal(t, CostReportStorageTrend, report.Type)
}

func TestAPIStorageTrendReportInvalidMethod(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/cost/reports/storage-trend", nil)
	w := httptest.NewRecorder()

	handler.HandleStorageTrendReport(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAPIResourceUtilizationReport(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/reports/resource-utilization", nil)
	w := httptest.NewRecorder()

	handler.HandleResourceUtilizationReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var report CostReport
	err := json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.Equal(t, CostReportResourceUtil, report.Type)
}

func TestAPIOptimizationReport(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/reports/optimization", nil)
	w := httptest.NewRecorder()

	handler.HandleOptimizationReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var report CostReport
	err := json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.Equal(t, CostReportOptimization, report.Type)
}

func TestAPIBudgetTrackingReport(t *testing.T) {
	engine, handler := setupTestAPI(t)

	// 创建预算
	budgetConfig := BudgetConfig{
		Name:        "Test Budget",
		TotalBudget: 1000,
		Period:      "monthly",
		StartDate:   time.Now().AddDate(0, -1, 0),
		EndDate:     time.Now().AddDate(0, 1, 0),
		Enabled:     true,
	}
	budget, err := engine.CreateBudget(budgetConfig)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/reports/budget-tracking?budget_id="+budget.ID, nil)
	w := httptest.NewRecorder()

	handler.HandleBudgetTrackingReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIBudgetTrackingReportMissingID(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/reports/budget-tracking", nil)
	w := httptest.NewRecorder()

	handler.HandleBudgetTrackingReport(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPIComprehensiveReport(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/reports/comprehensive", nil)
	w := httptest.NewRecorder()

	handler.HandleComprehensiveReport(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var report CostReport
	err := json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)

	assert.Equal(t, CostReportComprehensive, report.Type)
}

func TestAPIBudgetsList(t *testing.T) {
	engine, handler := setupTestAPI(t)

	// 创建多个预算
	for i := 0; i < 3; i++ {
		_, err := engine.CreateBudget(BudgetConfig{
			Name:        "Budget " + string(rune('A'+i)),
			TotalBudget: float64((i + 1) * 100),
			Period:      "monthly",
			StartDate:   time.Now(),
			EndDate:     time.Now().AddDate(0, 1, 0),
			Enabled:     true,
		})
		require.NoError(t, err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/budgets", nil)
	w := httptest.NewRecorder()

	handler.HandleBudgets(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var budgets []*BudgetConfig
	err := json.Unmarshal(w.Body.Bytes(), &budgets)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(budgets), 3)
}

func TestAPIBudgetsCreate(t *testing.T) {
	_, handler := setupTestAPI(t)

	budgetConfig := BudgetConfig{
		Name:        "New Budget",
		TotalBudget: 500,
		Period:      "monthly",
		StartDate:   time.Now(),
		EndDate:     time.Now().AddDate(0, 1, 0),
		Enabled:     true,
	}

	body, _ := json.Marshal(budgetConfig)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/cost/budgets", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleBudgets(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var created BudgetConfig
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "New Budget", created.Name)
}

func TestAPIBudgetGet(t *testing.T) {
	engine, handler := setupTestAPI(t)

	budget, err := engine.CreateBudget(BudgetConfig{
		Name:        "Test Budget",
		TotalBudget: 1000,
		Period:      "monthly",
		StartDate:   time.Now(),
		EndDate:     time.Now().AddDate(0, 1, 0),
		Enabled:     true,
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/budgets/"+budget.ID, nil)
	w := httptest.NewRecorder()

	handler.HandleBudgetByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIBudgetUpdate(t *testing.T) {
	engine, handler := setupTestAPI(t)

	budget, err := engine.CreateBudget(BudgetConfig{
		Name:        "Test Budget",
		TotalBudget: 1000,
		Period:      "monthly",
		StartDate:   time.Now(),
		EndDate:     time.Now().AddDate(0, 1, 0),
		Enabled:     true,
	})
	require.NoError(t, err)

	updatedConfig := BudgetConfig{
		Name:        "Updated Budget",
		TotalBudget: 1500,
		Period:      "monthly",
		StartDate:   time.Now(),
		EndDate:     time.Now().AddDate(0, 1, 0),
		Enabled:     true,
	}

	body, _ := json.Marshal(updatedConfig)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/cost/budgets/"+budget.ID, bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleBudgetByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updated BudgetConfig
	err = json.Unmarshal(w.Body.Bytes(), &updated)
	require.NoError(t, err)

	assert.Equal(t, "Updated Budget", updated.Name)
	assert.Equal(t, 1500.0, updated.TotalBudget)
}

func TestAPIBudgetDelete(t *testing.T) {
	engine, handler := setupTestAPI(t)

	budget, err := engine.CreateBudget(BudgetConfig{
		Name:        "Test Budget",
		TotalBudget: 1000,
		Period:      "monthly",
		StartDate:   time.Now(),
		EndDate:     time.Now().AddDate(0, 1, 0),
		Enabled:     true,
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/cost/budgets/"+budget.ID, nil)
	w := httptest.NewRecorder()

	handler.HandleBudgetByID(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAPIAlerts(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/alerts", nil)
	w := httptest.NewRecorder()

	handler.HandleAlerts(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPITrends(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/trends", nil)
	w := httptest.NewRecorder()

	handler.HandleTrends(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPITrendsPost(t *testing.T) {
	_, handler := setupTestAPI(t)

	trend := CostTrend{
		Date:          time.Now(),
		StorageCost:   100,
		BandwidthCost: 50,
		TotalCost:     150,
		StorageUsedGB: 50,
		BandwidthGB:   20,
	}

	body, _ := json.Marshal(trend)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/cost/trends", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleTrends(w, req)

	// POST 趋势数据是允许的
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAPIAlertAcknowledge(t *testing.T) {
	engine, handler := setupTestAPI(t)

	// 添加一个警报
	alert := &CostAlert{
		ID:        "alert-1",
		Type:      "budget_exceeded",
		Severity:  "warning",
		Message:   "Test alert",
		CreatedAt: time.Now(),
	}
	engine.alerts = append(engine.alerts, alert)

	ackRequest := map[string]string{"action": "acknowledge"}
	body, _ := json.Marshal(ackRequest)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/cost/alerts/alert-1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleAlertByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPIInvalidAlertAction(t *testing.T) {
	_, handler := setupTestAPI(t)

	invalidRequest := map[string]string{"action": "invalid"}
	body, _ := json.Marshal(invalidRequest)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/cost/alerts/alert-1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleAlertByID(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPIAlertInvalidMethod(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/alerts/alert-1", nil)
	w := httptest.NewRecorder()

	handler.HandleAlertByID(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAPIRegisterRoutes(t *testing.T) {
	_, handler := setupTestAPI(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 测试注册的路由
	routes := []string{
		"/api/cost/reports/storage-trend",
		"/api/cost/reports/resource-utilization",
		"/api/cost/reports/optimization",
		"/api/cost/reports/budget-tracking",
		"/api/cost/reports/comprehensive",
		"/api/cost/budgets",
		"/api/cost/alerts",
		"/api/cost/trends",
	}

	for _, route := range routes {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		// 不应该返回 404
		assert.NotEqual(t, http.StatusNotFound, w.Code, "Route: %s", route)
	}
}

func TestAPIBudgetNotFound(t *testing.T) {
	_, handler := setupTestAPI(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/cost/budgets/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.HandleBudgetByID(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPIAlertNotFound(t *testing.T) {
	_, handler := setupTestAPI(t)

	ackRequest := map[string]string{"action": "acknowledge"}
	body, _ := json.Marshal(ackRequest)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/cost/alerts/nonexistent", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleAlertByID(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPIWriteJSONError(t *testing.T) {
	_, handler := setupTestAPI(t)

	// 测试无效的 JSON 请求
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/cost/budgets", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.HandleBudgets(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
