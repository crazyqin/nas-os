package costpredict

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ========== 线性回归测试 ==========

func TestLinearRegression_Basic(t *testing.T) {
	x := []float64{0, 1, 2, 3, 4}
	y := []float64{1, 3, 5, 7, 9}
	slope, intercept, err := LinearRegression(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(slope-2.0) > 0.01 {
		t.Errorf("expected slope ~2.0, got %f", slope)
	}
	if math.Abs(intercept-1.0) > 0.01 {
		t.Errorf("expected intercept ~1.0, got %f", intercept)
	}
}

func TestLinearRegression_InsufficientData(t *testing.T) {
	_, _, err := LinearRegression([]float64{1}, []float64{2})
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestLinearRegression_MismatchedLengths(t *testing.T) {
	_, _, err := LinearRegression([]float64{1, 2}, []float64{3})
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestLinearRegression_ConstantData(t *testing.T) {
	x := []float64{0, 1, 2, 3}
	y := []float64{5, 5, 5, 5}
	slope, intercept, err := LinearRegression(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(slope) > 0.01 {
		t.Errorf("expected slope ~0, got %f", slope)
	}
	if math.Abs(intercept-5.0) > 0.01 {
		t.Errorf("expected intercept ~5.0, got %f", intercept)
	}
}

// ========== 指数平滑测试 ==========

func TestExponentialSmoothing_Basic(t *testing.T) {
	data := []float64{10, 12, 13, 12, 14}
	result, err := ExponentialSmoothing(data, 0.3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != len(data) {
		t.Fatalf("expected length %d, got %d", len(data), len(result))
	}
	if result[0] != data[0] {
		t.Errorf("first value should be original, got %f", result[0])
	}
}

func TestExponentialSmoothing_InvalidAlpha(t *testing.T) {
	data := []float64{10, 12, 13}
	result, err := ExponentialSmoothing(data, -0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should use default alpha=0.3
	if result[0] != data[0] {
		t.Errorf("first value should be original")
	}
}

func TestExponentialSmoothing_EmptyData(t *testing.T) {
	_, err := ExponentialSmoothing([]float64{}, 0.3)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestDoubleExponentialSmoothing_Basic(t *testing.T) {
	data := []float64{10, 12, 14, 16, 18}
	level, trend, err := DoubleExponentialSmoothing(data, 0.3, 0.1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(level) != len(data) || len(trend) != len(data) {
		t.Fatalf("unexpected lengths: level=%d trend=%d", len(level), len(trend))
	}
}

func TestDoubleExponentialSmoothing_InsufficientData(t *testing.T) {
	_, _, err := DoubleExponentialSmoothing([]float64{10}, 0.3, 0.1)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

// ========== 预测引擎测试 ==========

func newTestPredictor() *Predictor {
	p := NewPredictor()
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		p.AddRecord(CostRecord{
			Time:          baseTime.AddDate(0, i, 0),
			Department:    "IT",
			Project:       "ProjectA",
			StorageType:   StorageTypeSSD,
			Cost:          float64(1000 + i*200),
			UsedCapacity:  int64(1000000000 + i*100000000),
			TotalCapacity: 5000000000,
		})
	}
	return p
}

func TestPredictor_AddAndListRecords(t *testing.T) {
	p := NewPredictor()
	if len(p.GetRecords()) != 0 {
		t.Error("expected empty records")
	}
	p.AddRecord(CostRecord{Cost: 100})
	if len(p.GetRecords()) != 1 {
		t.Error("expected 1 record")
	}
}

func TestPredictor_PredictCost(t *testing.T) {
	p := newTestPredictor()
	results, err := p.PredictCost(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 prediction")
	}
	for _, r := range results {
		if r.PredictedCost < 0 {
			t.Errorf("predicted cost should be non-negative, got %f", r.PredictedCost)
		}
		if r.Method == "" {
			t.Error("prediction method should not be empty")
		}
	}
}

func TestPredictor_PredictCost_InsufficientData(t *testing.T) {
	p := NewPredictor()
	p.AddRecord(CostRecord{Cost: 100})
	_, err := p.PredictCost(3)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestPredictor_PredictCost_InvalidPeriods(t *testing.T) {
	p := newTestPredictor()
	_, err := p.PredictCost(0)
	if err != ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
}

func TestPredictor_PredictCostByDepartment(t *testing.T) {
	p := newTestPredictor()
	results, err := p.PredictCostByDepartment("IT", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected predictions for IT department")
	}
}

func TestPredictor_PredictCostByProject(t *testing.T) {
	p := newTestPredictor()
	results, err := p.PredictCostByProject("ProjectA", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected predictions for ProjectA")
	}
}

func TestPredictor_PredictCostByStorageType(t *testing.T) {
	p := newTestPredictor()
	results, err := p.PredictCostByStorageType(StorageTypeSSD, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected predictions for SSD storage type")
	}
}

// ========== 容量增长预测测试 ==========

func TestPredictor_PredictCapacityGrowth(t *testing.T) {
	p := newTestPredictor()
	forecasts, err := p.PredictCapacityGrowth(6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(forecasts) != 6 {
		t.Fatalf("expected 6 forecasts, got %d", len(forecasts))
	}
	for _, f := range forecasts {
		if f.PredictedUsed <= 0 {
			t.Error("predicted capacity should be positive")
		}
	}
}

func TestPredictor_PredictCapacityGrowth_InsufficientData(t *testing.T) {
	p := NewPredictor()
	p.AddRecord(CostRecord{UsedCapacity: 100})
	_, err := p.PredictCapacityGrowth(3)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

// ========== 优化建议测试 ==========

func TestPredictor_GenerateOptimizationSuggestions(t *testing.T) {
	p := newTestPredictor()
	suggestions := p.GenerateOptimizationSuggestions()
	if len(suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}
	types := make(map[string]bool)
	for _, s := range suggestions {
		types[s.Type] = true
		if s.EstimatedSaving < 0 {
			t.Errorf("saving should be non-negative for type %s", s.Type)
		}
		if s.Title == "" {
			t.Error("suggestion title should not be empty")
		}
	}
	expectedTypes := []string{"cold_archive", "deduplication", "compression", "tiering"}
	for _, et := range expectedTypes {
		if !types[et] {
			t.Errorf("expected suggestion type %s", et)
		}
	}
}

func TestPredictor_GenerateOptimizationSuggestions_EmptyData(t *testing.T) {
	p := NewPredictor()
	suggestions := p.GenerateOptimizationSuggestions()
	if len(suggestions) != 0 {
		t.Error("expected no suggestions for empty data")
	}
}

// ========== 预算告警测试 ==========

func TestPredictor_CheckBudgetAlerts(t *testing.T) {
	p := newTestPredictor()
	p.SetBudgetLimit("IT", 500) // 设置较低预算以触发告警
	alerts, err := p.CheckBudgetAlerts(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) == 0 {
		t.Error("expected budget alerts")
	}
	for _, a := range alerts {
		if a.AlertLevel == "" {
			t.Error("alert level should not be empty")
		}
	}
}

func TestPredictor_CheckBudgetAlerts_NoBudget(t *testing.T) {
	p := newTestPredictor()
	alerts, err := p.CheckBudgetAlerts(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 0 {
		t.Error("expected no alerts without budget limits")
	}
}

func TestPredictor_SetGetBudgetLimit(t *testing.T) {
	p := NewPredictor()
	p.SetBudgetLimit("dept1", 5000)
	amt, ok := p.GetBudgetLimit("dept1")
	if !ok || amt != 5000 {
		t.Errorf("expected 5000, got %f (ok=%v)", amt, ok)
	}
	_, ok = p.GetBudgetLimit("nonexistent")
	if ok {
		t.Error("expected false for nonexistent department")
	}
}

// ========== 成本报告测试 ==========

func TestPredictor_GenerateReport_Monthly(t *testing.T) {
	p := newTestPredictor()
	report, err := p.GenerateReport("monthly", CNY)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ReportType != "monthly" {
		t.Errorf("expected monthly, got %s", report.ReportType)
	}
	if report.Currency != CNY {
		t.Errorf("expected CNY, got %s", report.Currency)
	}
	if report.ID == "" {
		t.Error("report ID should not be empty")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("generated time should not be zero")
	}
}

func TestPredictor_GenerateReport_InvalidType(t *testing.T) {
	p := newTestPredictor()
	_, err := p.GenerateReport("invalid", CNY)
	if err != ErrInvalidParams {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
}

func TestPredictor_GenerateReport_UnsupportedCurrency(t *testing.T) {
	p := newTestPredictor()
	_, err := p.GenerateReport("monthly", "BTC")
	if err != ErrCurrencyNotFound {
		t.Errorf("expected ErrCurrencyNotFound, got %v", err)
	}
}

func TestPredictor_GetReport(t *testing.T) {
	p := newTestPredictor()
	report, _ := p.GenerateReport("monthly", CNY)
	got, err := p.GetReport(report.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("expected ID %s, got %s", report.ID, got.ID)
	}
}

func TestPredictor_GetReport_NotFound(t *testing.T) {
	p := NewPredictor()
	_, err := p.GetReport("nonexistent")
	if err != ErrReportNotFound {
		t.Errorf("expected ErrReportNotFound, got %v", err)
	}
}

func TestPredictor_ListReports(t *testing.T) {
	p := newTestPredictor()
	p.GenerateReport("monthly", CNY)
	p.GenerateReport("yearly", USD)
	reports := p.ListReports()
	if len(reports) < 2 {
		t.Errorf("expected at least 2 reports, got %d", len(reports))
	}
}

// ========== 币种测试 ==========

func TestPredictor_ConvertCost(t *testing.T) {
	p := NewPredictor()
	usd, err := p.ConvertCost(100, USD)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 100.0 * 0.14
	if math.Abs(usd-expected) > 0.01 {
		t.Errorf("expected %f, got %f", expected, usd)
	}
}

func TestPredictor_ConvertCost_Unsupported(t *testing.T) {
	p := NewPredictor()
	_, err := p.ConvertCost(100, "BTC")
	if err != ErrCurrencyNotFound {
		t.Errorf("expected ErrCurrencyNotFound, got %v", err)
	}
}

func TestPredictor_ListCurrencies(t *testing.T) {
	p := NewPredictor()
	currencies := p.ListCurrencies()
	if len(currencies) < 4 {
		t.Errorf("expected at least 4 currencies, got %d", len(currencies))
	}
}

func TestPredictor_SetCurrencyRate(t *testing.T) {
	p := NewPredictor()
	p.SetCurrencyRate(CurrencyRate{Code: "KRW", Name: "韩元", Rate: 180.0})
	converted, err := p.ConvertCost(100, "KRW")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(converted-18000) > 0.01 {
		t.Errorf("expected 18000, got %f", converted)
	}
}

// ========== HTTP Handler 测试 ==========

func TestHandlers_AddRecord(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	body, _ := json.Marshal(CostRecord{
		Department: "IT",
		Cost:       5000,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/costpredict/records", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "成本记录添加成功" {
		t.Errorf("unexpected message: %v", resp["message"])
	}
}

func TestHandlers_AddRecord_InvalidBody(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/costpredict/records", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlers_ListRecords(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/records", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	total := int(resp["total"].(float64))
	if total != 6 {
		t.Errorf("expected 6 records, got %d", total)
	}
}

func TestHandlers_Predict(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/predict?periods=3", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_PredictByDepartment(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/predict/department?department=IT&periods=3", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_PredictByDepartment_MissingParam(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/predict/department", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlers_PredictByProject(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/predict/project?project=ProjectA", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_PredictByStorageType(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/predict/storage-type?type=ssd", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_CapacityForecast(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/capacity/forecast?months=6", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_OptimizationSuggestions(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/suggestions", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_BudgetAlerts(t *testing.T) {
	p := newTestPredictor()
	p.SetBudgetLimit("IT", 500)
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/alerts", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_GenerateReport(t *testing.T) {
	p := newTestPredictor()
	h := NewHandlers(p)

	body, _ := json.Marshal(map[string]string{
		"report_type": "monthly",
		"currency":    "CNY",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/costpredict/reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestHandlers_ListReports(t *testing.T) {
	p := newTestPredictor()
	p.GenerateReport("monthly", CNY)
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/reports", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_GetReport(t *testing.T) {
	p := newTestPredictor()
	report, _ := p.GenerateReport("monthly", CNY)
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/reports/"+report.ID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_GetReport_NotFound(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/reports/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlers_SetBudget(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	body, _ := json.Marshal(map[string]interface{}{
		"department": "IT",
		"amount":     50000,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/costpredict/budget", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_SetBudget_MissingDept(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	body, _ := json.Marshal(map[string]interface{}{
		"amount": 50000,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/costpredict/budget", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlers_Currencies(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/currencies", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_ConvertCost(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/convert?amount=100&currency=USD", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlers_ConvertCost_MissingParam(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/convert", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlers_NotFound(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/costpredict/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlers_BatchAdd(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	body, _ := json.Marshal(map[string]interface{}{
		"records": []CostRecord{
			{Department: "IT", Cost: 1000},
			{Department: "HR", Cost: 2000},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/costpredict/records/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if len(p.GetRecords()) != 2 {
		t.Errorf("expected 2 records, got %d", len(p.GetRecords()))
	}
}

func TestHandlers_SetCurrency(t *testing.T) {
	p := NewPredictor()
	h := NewHandlers(p)

	body, _ := json.Marshal(CurrencyRate{Code: "GBP", Name: "英镑", Rate: 0.11})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/costpredict/currencies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
