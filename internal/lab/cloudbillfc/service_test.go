package cloudbillfc

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// === 辅助函数 ===

func absDiff(a, b float64) float64 {
	return math.Abs(a - b)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService()
	h := NewHandler(svc)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)
	return router
}

func makeConfig() ForecastConfig {
	return ForecastConfig{
		Provider:        "",
		StorageGB:       5000,
		MonthlyEgressGB: 100,
		MonthlyAPI10K:   50,
		GrowthRateMonth: 0.05,
		Months:          12,
	}
}

// === 服务层测试 ===

func TestForecastBasic(t *testing.T) {
	svc := NewService()
	config := makeConfig()

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if report.Months != 12 {
		t.Errorf("期望预测12个月, 实际%d", report.Months)
	}

	if len(report.Forecasts) == 0 {
		t.Fatal("应有至少一个服务商预测结果")
	}

	if report.BestProvider == "" {
		t.Fatal("应返回最优服务商")
	}
}

func TestForecastTrends(t *testing.T) {
	svc := NewService()
	config := makeConfig()

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	for _, f := range report.Forecasts {
		if len(f.Trends) != config.Months+1 {
			t.Errorf("服务商 %s 趋势点数期望%d, 实际%d", f.ProviderName, config.Months+1, len(f.Trends))
		}

		// 验证趋势递增（增长率为正）
		if f.GrowthPercent <= 0 {
			t.Errorf("服务商 %s 增长率应大于0（5%%月增长）, 实际%.2f%%", f.ProviderName, f.GrowthPercent)
		}

		// 最终月费应大于初始月费
		if f.FinalMonthlyCost <= f.InitialMonthlyCost {
			t.Errorf("服务商 %s 最终月费应大于初始月费", f.ProviderName)
		}
	}
}

func TestForecastTotalCost(t *testing.T) {
	svc := NewService()
	config := makeConfig()

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	for _, f := range report.Forecasts {
		if f.TotalCost <= 0 {
			t.Errorf("服务商 %s 预测期总费用应大于0", f.ProviderName)
		}

		// 总费用 ≈ 月均 × 月数
		expectedTotal := f.AvgMonthly * float64(config.Months+1)
		if absDiff(f.TotalCost, roundToTwo(expectedTotal)) > 1 {
			t.Errorf("服务商 %s 总费用期望约%.2f, 实际%.2f", f.ProviderName, expectedTotal, f.TotalCost)
		}
	}
}

func TestForecastSpecificProvider(t *testing.T) {
	svc := NewService()
	config := makeConfig()
	config.Provider = "阿里云 OSS 标准存储"

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if len(report.Forecasts) != 1 {
		t.Fatalf("指定provider应只返回1个结果, 实际%d", len(report.Forecasts))
	}

	if report.Forecasts[0].ProviderName != "阿里云 OSS 标准存储" {
		t.Errorf("期望服务商 阿里云 OSS 标准存储, 实际 %s", report.Forecasts[0].ProviderName)
	}
}

func TestForecastInvalidProvider(t *testing.T) {
	svc := NewService()
	config := makeConfig()
	config.Provider = "不存在的服务商"

	_, err := svc.Forecast(config)
	if err == nil {
		t.Error("不存在的服务商应返回错误")
	}
}

func TestForecastDefaultMonths(t *testing.T) {
	svc := NewService()
	config := makeConfig()
	config.Months = 0

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if report.Months != 12 {
		t.Errorf("默认月数应为12, 实际%d", report.Months)
	}
}

func TestForecastMonthsTooLong(t *testing.T) {
	svc := NewService()
	config := makeConfig()
	config.Months = 100

	_, err := svc.Forecast(config)
	if err == nil {
		t.Error("超过60个月应返回错误")
	}
}

func TestForecastNegativeStorage(t *testing.T) {
	svc := NewService()
	config := makeConfig()
	config.StorageGB = -100

	_, err := svc.Forecast(config)
	if err == nil {
		t.Error("负存储用量应返回错误")
	}
}

func TestForecastZeroGrowth(t *testing.T) {
	svc := NewService()
	config := makeConfig()
	config.GrowthRateMonth = 0

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	for _, f := range report.Forecasts {
		if f.GrowthPercent != 0 {
			t.Errorf("服务商 %s 零增长时增长率应为0, 实际%.2f", f.ProviderName, f.GrowthPercent)
		}

		// 所有月费应相同
		for _, trend := range f.Trends {
			if absDiff(trend.MonthlyCost, f.InitialMonthlyCost) > 0.01 {
				t.Errorf("服务商 %s 零增长月费应恒定, 月份%d 费用%.2f ≠ %.2f", f.ProviderName, trend.Month, trend.MonthlyCost, f.InitialMonthlyCost)
			}
		}
	}
}

func TestCompareProviders(t *testing.T) {
	svc := NewService()

	comparisons := svc.CompareProviders(5000, 100, 50)
	if len(comparisons) == 0 {
		t.Fatal("应返回服务商对比结果")
	}

	for _, c := range comparisons {
		if c.MonthlyCost <= 0 {
			t.Errorf("服务商 %s 月费应大于0", c.ProviderName)
		}
		if c.YearlyCost <= 0 {
			t.Errorf("服务商 %s 年费应大于0", c.ProviderName)
		}
		if c.FiveYearCost <= 0 {
			t.Errorf("服务商 %s 5年费应大于0", c.ProviderName)
		}
	}
}

func TestCompareProvidersZeroStorage(t *testing.T) {
	svc := NewService()

	comparisons := svc.CompareProviders(0, 0, 0)

	for _, c := range comparisons {
		if c.MonthlyCost < 0 {
			t.Errorf("服务商 %s 零用量月费不应为负", c.ProviderName)
		}
		if c.CostPerGB != 0 {
			t.Errorf("服务商 %s 零存储时每GB成本应为0", c.ProviderName)
		}
	}
}

func TestCompareProvidersCostPerGB(t *testing.T) {
	svc := NewService()

	comparisons := svc.CompareProviders(10000, 0, 0)

	for _, c := range comparisons {
		// 使用国内服务商验证每GB成本（定价较高，roundToTwo后不为0）
		if c.ProviderName == "阿里云 OSS 标准存储" && c.CostPerGB <= 0 {
			t.Errorf("阿里云 10000GB时每GB成本应大于0, 实际%.4f", c.CostPerGB)
		}
		if c.MonthlyCost <= 0 {
			t.Errorf("服务商 %s 10000GB月费应大于0", c.ProviderName)
		}
	}
}

func TestTieredPricing(t *testing.T) {
	svc := NewService()

	// 阿里云有分级定价
	// 0-1024GB: 0.12/GB, 1024-10240GB: 0.099/GB
	// 5000GB: 1024×0.12 + (5000-1024)×0.099 = 122.88 + 393.28 = 512.16
	aliyun := ProviderPricing{
		ProviderName:  "Test Tiered",
		StoragePerGB:  0.12,
		TieredPricing: true,
		Tiers: []PriceTier{
			{MinGB: 0, MaxGB: 1024, PriceGB: 0.12},
			{MinGB: 1024, MaxGB: 10240, PriceGB: 0.099},
			{MinGB: 10240, MaxGB: -1, PriceGB: 0.085},
		},
	}

	cost := svc.calcStorageCost(aliyun, 5000)
	expected := 1024*0.12 + (5000-1024)*0.099
	if absDiff(cost, expected) > 0.01 {
		t.Errorf("分级存储费 5000GB 期望%.2f, 实际%.2f", expected, cost)
	}

	// 500GB: 在第一层
	cost = svc.calcStorageCost(aliyun, 500)
	expected = 500 * 0.12
	if absDiff(cost, expected) > 0.01 {
		t.Errorf("分级存储费 500GB 期望%.2f, 实际%.2f", expected, cost)
	}

	// 20000GB: 跨三层
	cost = svc.calcStorageCost(aliyun, 20000)
	expected = 1024*0.12 + (10240-1024)*0.099 + (20000-10240)*0.085
	if absDiff(cost, expected) > 0.01 {
		t.Errorf("分级存储费 20000GB 期望%.2f, 实际%.2f", expected, cost)
	}
}

func TestOptimizations(t *testing.T) {
	svc := NewService()

	// 高增长 + 大流量 + 多API
	config := ForecastConfig{
		StorageGB:       2000,
		MonthlyEgressGB: 200,
		MonthlyAPI10K:   800,
		GrowthRateMonth: 0.08,
		Months:          12,
	}

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if len(report.Optimizations) == 0 {
		t.Fatal("应有优化建议")
	}

	// 应包含生命周期建议（高增长）
	foundLifecycle := false
	for _, opt := range report.Optimizations {
		if opt.Type == "lifecycle" {
			foundLifecycle = true
		}
	}
	if !foundLifecycle {
		t.Error("高增长率应触发生命周期优化建议")
	}

	// 应包含出口流量建议
	foundEgress := false
	for _, opt := range report.Optimizations {
		if opt.Type == "egress" {
			foundEgress = true
		}
	}
	if !foundEgress {
		t.Error("高出口流量应触发CDN优化建议")
	}

	// 应包含API建议
	foundAPI := false
	for _, opt := range report.Optimizations {
		if opt.Type == "api" {
			foundAPI = true
		}
	}
	if !foundAPI {
		t.Error("高API调用应触发API优化建议")
	}
}

func TestGetProviders(t *testing.T) {
	svc := NewService()
	providers := svc.GetProviders()

	if len(providers) == 0 {
		t.Fatal("应有预置服务商")
	}

	for _, p := range providers {
		if p.ProviderName == "" {
			t.Error("服务商名称不能为空")
		}
		if p.StoragePerGB < 0 {
			t.Errorf("服务商 %s 存储价格不能为负", p.ProviderName)
		}
	}
}

func TestEgressFreeTier(t *testing.T) {
	svc := NewService()

	// 出口流量在免费额度内
	config := ForecastConfig{
		Provider:        "AWS S3 Standard",
		StorageGB:       1000,
		MonthlyEgressGB: 50, // AWS免费100GB
		MonthlyAPI10K:   10,
		GrowthRateMonth: 0,
		Months:          1,
	}

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	for _, f := range report.Forecasts {
		// 出口费应为0
		if f.Trends[0].EgressCost != 0 {
			t.Errorf("服务商 %s 免费额度内出口费应为0, 实际%.2f", f.ProviderName, f.Trends[0].EgressCost)
		}
	}
}

func TestPeakMonthly(t *testing.T) {
	svc := NewService()
	config := makeConfig()

	report, err := svc.Forecast(config)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	for _, f := range report.Forecasts {
		if f.PeakMonthly <= 0 {
			t.Errorf("服务商 %s 峰值月费应大于0", f.ProviderName)
		}

		// 峰值应等于最后一个月的费用（正增长时）
		if absDiff(f.PeakMonthly, f.FinalMonthlyCost) > 0.01 {
			t.Errorf("服务商 %s 正增长峰值应等于末期月费, 峰值%.2f vs 末期%.2f", f.ProviderName, f.PeakMonthly, f.FinalMonthlyCost)
		}
	}
}

// === HTTP Handler 测试 ===

func TestHandlerForecast(t *testing.T) {
	router := setupTestRouter()

	body, _ := json.Marshal(makeConfig())
	req, _ := http.NewRequest("POST", "/api/v1/cloudbillfc/forecast", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d, 响应: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("响应应包含data字段")
	}

	if data["best_provider"] == nil {
		t.Fatal("响应应包含 best_provider")
	}
}

func TestHandlerForecastBadJSON(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/cloudbillfc/forecast", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400, 实际%d", w.Code)
	}
}

func TestHandlerProviders(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/cloudbillfc/providers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("响应应包含data数组")
	}

	if len(data) == 0 {
		t.Fatal("应返回至少一个服务商")
	}
}

func TestHandlerCompare(t *testing.T) {
	router := setupTestRouter()

	reqBody := CompareRequest{
		StorageGB:       5000,
		MonthlyEgressGB: 100,
		MonthlyAPI10K:   50,
	}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/cloudbillfc/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d, 响应: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("响应应包含data数组")
	}

	if len(data) == 0 {
		t.Fatal("应返回至少一个服务商对比")
	}
}

func TestHandlerCompareBadJSON(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/cloudbillfc/compare", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400, 实际%d", w.Code)
	}
}

func TestHandlerCompareNegativeStorage(t *testing.T) {
	router := setupTestRouter()

	reqBody := CompareRequest{StorageGB: -100}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/cloudbillfc/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400, 实际%d", w.Code)
	}
}

// 保留 time 引用避免 unused import.
var _ = time.Now
