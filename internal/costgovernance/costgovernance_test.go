package costgovernance

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestManager() *Manager {
	m := NewManager()
	m.CreatePolicy(&CostPolicy{
		ID:               "policy-aws-01",
		Name:             "AWS生产环境策略",
		Description:      "限制AWS生产环境月度支出",
		Provider:         ProviderAWS,
		MaxMonthlySpend:  50000,
		MaxResourceCount: 100,
		AllowedRegions:   []string{"us-east-1", "us-west-2"},
		Enabled:          true,
	})
	m.CreateBudget(&Budget{
		ID:              "budget-aws-01",
		Name:            "AWS月度预算",
		Provider:        ProviderAWS,
		Amount:          50000,
		Period:          PeriodMonthly,
		AlertThresholds: []float64{50, 80, 100},
		StartDate:       time.Now().AddDate(0, 0, -30),
		EndDate:         time.Now(),
	})
	m.UpdateResourceUsage(&ResourceUsage{
		ID:             "usage-01",
		ResourceID:     "i-abc123",
		ResourceName:   "web-server-1",
		ResourceType:   ResourceCompute,
		Provider:       ProviderAWS,
		Region:         "us-east-1",
		CPUPercent:     45.0,
		MemoryPercent:  60.0,
		StorageUsedGB:  100,
		StorageTotalGB: 500,
		DailyCost:      50.0,
		Tags:           map[string]string{"env": "prod"},
	})
	return m
}

func TestCreateAndGetPolicy(t *testing.T) {
	m := newTestManager()
	policy, err := m.GetPolicy("policy-aws-01")
	require.NoError(t, err)
	assert.Equal(t, "AWS生产环境策略", policy.Name)
	assert.Equal(t, ProviderAWS, policy.Provider)
	assert.True(t, policy.Enabled)
}

func TestPolicyNotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetPolicy("nonexistent")
	assert.ErrorIs(t, err, ErrPolicyNotFound)
}

func TestDeletePolicy(t *testing.T) {
	m := newTestManager()
	err := m.DeletePolicy("policy-aws-01")
	require.NoError(t, err)
	_, err = m.GetPolicy("policy-aws-01")
	assert.ErrorIs(t, err, ErrPolicyNotFound)
}

func TestListPolicies(t *testing.T) {
	m := newTestManager()
	policies := m.ListPolicies()
	assert.Len(t, policies, 1)
}

func TestCreateAndGetBudget(t *testing.T) {
	m := newTestManager()
	budget, err := m.GetBudget("budget-aws-01")
	require.NoError(t, err)
	assert.Equal(t, "AWS月度预算", budget.Name)
	assert.Equal(t, 50000.0, budget.Amount)
}

func TestBudgetNotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetBudget("nonexistent")
	assert.ErrorIs(t, err, ErrBudgetNotFound)
}

func TestUpdateBudgetSpent(t *testing.T) {
	m := newTestManager()
	err := m.UpdateBudgetSpent("budget-aws-01", 42000)
	require.NoError(t, err)
	budget, _ := m.GetBudget("budget-aws-01")
	assert.Equal(t, 42000.0, budget.Spent)
}

func TestBudgetAlertGeneration(t *testing.T) {
	m := newTestManager()
	// 84% 超过80%阈值
	err := m.UpdateBudgetSpent("budget-aws-01", 42000)
	require.NoError(t, err)
	alerts := m.ListAlerts(ProviderAWS, false)
	assert.NotEmpty(t, alerts)
	found := false
	for _, a := range alerts {
		if a.BudgetID == "budget-aws-01" && a.Threshold == 80 {
			found = true
			assert.Equal(t, SeverityWarning, a.Severity)
		}
	}
	assert.True(t, found, "应生成80%阈值告警")
}

func TestBudgetCriticalAlert(t *testing.T) {
	m := newTestManager()
	m.UpdateBudgetSpent("budget-aws-01", 55000) // 110%
	alerts := m.ListAlerts(ProviderAWS, false)
	found := false
	for _, a := range alerts {
		if a.BudgetID == "budget-aws-01" && a.Severity == SeverityCritical {
			found = true
		}
	}
	assert.True(t, found, "应生成严重级别告警")
}

func TestAcknowledgeAlert(t *testing.T) {
	m := newTestManager()
	m.UpdateBudgetSpent("budget-aws-01", 42000)
	alerts := m.ListAlerts(ProviderAWS, false)
	require.NotEmpty(t, alerts)
	err := m.AcknowledgeAlert(alerts[0].ID)
	require.NoError(t, err)
}

func TestResourceUsage(t *testing.T) {
	m := newTestManager()
	usage, err := m.GetResourceUsage("usage-01")
	require.NoError(t, err)
	assert.Equal(t, "web-server-1", usage.ResourceName)
	assert.Equal(t, 45.0, usage.CPUPercent)
}

func TestListResourceUsages(t *testing.T) {
	m := newTestManager()
	usages := m.ListResourceUsages(ProviderAWS)
	assert.Len(t, usages, 1)
}

func TestGenerateReport(t *testing.T) {
	m := newTestManager()
	report := m.GenerateReport(ProviderAWS, time.Now().AddDate(0, -1, 0), time.Now())
	assert.NotNil(t, report)
	assert.Equal(t, ProviderAWS, report.Provider)
	assert.GreaterOrEqual(t, report.TotalCost, 0.0)
}

func TestGetCostSummary(t *testing.T) {
	m := newTestManager()
	summary := m.GetCostSummary(ProviderAWS)
	assert.Contains(t, summary, "daily_total")
	assert.Contains(t, summary, "monthly_est")
	assert.Contains(t, summary, "yearly_est")
}

func TestInvalidInput(t *testing.T) {
	m := NewManager()
	err := m.CreatePolicy(&CostPolicy{})
	assert.ErrorIs(t, err, ErrInvalidInput)

	err = m.CreateBudget(&Budget{ID: "b1"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ========== Analyzer 测试 ==========

func TestAnalyzerResourceUtilization(t *testing.T) {
	m := newTestManager()
	a := NewAnalyzer(m)
	result := a.AnalyzeResourceUtilization(ProviderAWS)
	assert.Equal(t, 1, result["total"])
}

func TestAnalyzerOptimizationSuggestions(t *testing.T) {
	m := newTestManager()
	// 添加低使用率资源
	m.UpdateResourceUsage(&ResourceUsage{
		ID:             "usage-low",
		ResourceID:     "i-low",
		ResourceName:   "闲置服务器",
		ResourceType:   ResourceCompute,
		Provider:       ProviderAWS,
		Region:         "us-east-1",
		CPUPercent:     5.0,
		MemoryPercent:  10.0,
		StorageUsedGB:  10,
		StorageTotalGB: 500,
		DailyCost:      100.0,
	})
	a := NewAnalyzer(m)
	suggestions := a.GenerateOptimizationSuggestions(ProviderAWS)
	assert.NotEmpty(t, suggestions)
}

func TestAnalyzerPredictCost(t *testing.T) {
	m := newTestManager()
	a := NewAnalyzer(m)

	baseTime := time.Now().AddDate(0, 0, -10)
	for i := 0; i < 10; i++ {
		a.AddTrendData("aws", &CostTrend{
			Date: baseTime.AddDate(0, 0, i),
			Cost: 1000 + float64(i)*50,
		})
	}

	predictions, err := a.PredictCost("aws", 7)
	require.NoError(t, err)
	assert.Len(t, predictions, 7)
	// 趋势上升，预测值应大于最后数据点
	assert.Greater(t, predictions[6].Cost, float64(1000+9*50))
}

func TestAnalyzerDetectAnomalies(t *testing.T) {
	m := newTestManager()
	a := NewAnalyzer(m)

	// 正常数据
	for i := 0; i < 20; i++ {
		a.AddTrendData("aws", &CostTrend{
			Date: time.Now().AddDate(0, 0, -20+i),
			Cost: 1000 + float64(i%3)*10,
		})
	}
	// 异常数据
	a.AddTrendData("aws", &CostTrend{
		Date: time.Now(),
		Cost: 5000,
	})

	anomalies := a.DetectAnomalies("aws", 95.0)
	assert.NotEmpty(t, anomalies)
}

// ========== Handler 测试 ==========

func setupRouter() (*gin.Engine, *Handlers) {
	gin.SetMode(gin.TestMode)
	m := newTestManager()
	a := NewAnalyzer(m)
	h := NewHandlers(m, a)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r, h
}

func TestHandlerListPolicies(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/costgovernance/policies", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "AWS生产环境策略")
}

func TestHandlerCreatePolicy(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	body := `{"id":"policy-gcp-01","name":"GCP策略","provider":"gcp","max_monthly_spend":30000,"enabled":true}`
	req, _ := http.NewRequest("POST", "/api/v1/costgovernance/policies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "GCP策略")
}

func TestHandlerListBudgets(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/costgovernance/budgets", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "AWS月度预算")
}

func TestHandlerGetCostSummary(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/costgovernance/summary?provider=aws", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "daily_total")
}

func TestHandlerGetResourceUtilization(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/costgovernance/analysis/utilization?provider=aws", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total")
}

func TestHandlerGetOptimizationSuggestions(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/costgovernance/analysis/suggestions?provider=aws", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "suggestions")
}

func TestHandlerPredictCost(t *testing.T) {
	r, h := setupRouter()
	// 添加趋势数据
	for i := 0; i < 10; i++ {
		h.analyzer.AddTrendData("aws", &CostTrend{
			Date: time.Now().AddDate(0, 0, -10+i),
			Cost: 1000 + float64(i)*50,
		})
	}
	w := httptest.NewRecorder()
	body := `{"provider":"aws","future_days":7}`
	req, _ := http.NewRequest("POST", "/api/v1/costgovernance/analysis/predict", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "predictions")
}

func TestHandlerDeletePolicy(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/costgovernance/policies/policy-aws-01", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "策略已删除")
}

func TestHandlerDeletePolicyNotFound(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/costgovernance/policies/nonexistent", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
