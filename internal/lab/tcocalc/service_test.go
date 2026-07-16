package tcocalc

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

func makeRequest() TCORequest {
	return TCORequest{
		Hardware: []HardwareSpec{
			{Name: "NAS主机", Price: 3000, Quantity: 1, Lifespan: 5},
			{Name: "8TB HDD", Price: 1200, Quantity: 4, Lifespan: 5},
		},
		Power: PowerSpec{
			Watts:       80,
			HoursPerDay: 24,
			PriceKWh:    0.5,
		},
		Maintenance: MaintenanceSpec{
			AnnualCost:      500,
			ReplacementCost: 2000,
			ReplaceInterval: 5,
		},
		Licenses: []LicenseSpec{
			{Name: "DSM系统", Type: "perpetual", Price: 0, Quantity: 1},
			{Name: "套件订阅", Type: "subscription", AnnualFee: 300, Quantity: 1},
		},
		StorageTB:       24, // 8TB × 4 × 0.75 (RAID5)
		Years:           5,
		MonthlyEgressGB: 50,
		MonthlyAPI10K:   10,
	}
}

// === 服务层测试 ===

func TestCalculateBasic(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	if report.Years != 5 {
		t.Errorf("期望年限5, 实际%d", report.Years)
	}

	if report.NASFiveYearTotal <= 0 {
		t.Errorf("NAS 5年总成本应大于0, 实际%.2f", report.NASFiveYearTotal)
	}

	if report.NASUpfrontTotal <= 0 {
		t.Errorf("NAS 一次性成本应大于0, 实际%.2f", report.NASUpfrontTotal)
	}
}

func TestCalculateUpfrontCost(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 一次性 = NAS主机3000 + 4×1200 HDD = 3000 + 4800 = 7800
	expectedUpfront := 3000.0 + 1200.0*4
	if absDiff(report.NASUpfrontTotal, expectedUpfront) > 0.01 {
		t.Errorf("一次性成本期望%.2f, 实际%.2f", expectedUpfront, report.NASUpfrontTotal)
	}
}

func TestCalculatePowerCost(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 电力: 80W × 24h × 365 / 1000 = 700.8 kWh/年
	// 年成本 = 700.8 × 0.5 = 350.4 元/年
	// 5年 = 1752 元
	var powerBreakdown *CostBreakdown
	for i := range report.Breakdowns {
		if report.Breakdowns[i].Category == CategoryPower {
			powerBreakdown = &report.Breakdowns[i]
			break
		}
	}
	if powerBreakdown == nil {
		t.Fatal("未找到电力成本明细")
	}

	expectedYearly := 80.0 * 24 * 365 / 1000 * 0.5
	if absDiff(powerBreakdown.Yearly, expectedYearly) > 0.01 {
		t.Errorf("电力年成本期望%.2f, 实际%.2f", expectedYearly, powerBreakdown.Yearly)
	}
}

func TestCalculateHardwareCost(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	var hwBreakdown *CostBreakdown
	for i := range report.Breakdowns {
		if report.Breakdowns[i].Category == CategoryHardware {
			hwBreakdown = &report.Breakdowns[i]
			break
		}
	}
	if hwBreakdown == nil {
		t.Fatal("未找到硬件成本明细")
	}

	// 硬件年化: (3000 + 4800) / 5 = 1560 元/年
	expectedYearly := 7800.0 / 5
	if absDiff(hwBreakdown.Yearly, expectedYearly) > 0.01 {
		t.Errorf("硬件年成本期望%.2f, 实际%.2f", expectedYearly, hwBreakdown.Yearly)
	}
}

func TestCalculateLicenseCost(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	var licBreakdown *CostBreakdown
	for i := range report.Breakdowns {
		if report.Breakdowns[i].Category == CategoryLicense {
			licBreakdown = &report.Breakdowns[i]
			break
		}
	}
	if licBreakdown == nil {
		t.Fatal("未找到许可成本明细")
	}

	// 仅订阅费用年化: 300 元/年
	if absDiff(licBreakdown.Yearly, 300.0) > 0.01 {
		t.Errorf("许可年成本期望300.00, 实际%.2f", licBreakdown.Yearly)
	}
}

func TestCalculateMaintenanceCost(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	var maintBreakdown *CostBreakdown
	for i := range report.Breakdowns {
		if report.Breakdowns[i].Category == CategoryMaint {
			maintBreakdown = &report.Breakdowns[i]
			break
		}
	}
	if maintBreakdown == nil {
		t.Fatal("未找到维护成本明细")
	}

	// 年维护500 + 年化更换 2000/5 = 400 → 共900
	expectedYearly := 500.0 + 2000.0/5
	if absDiff(maintBreakdown.Yearly, expectedYearly) > 0.01 {
		t.Errorf("维护年成本期望%.2f, 实际%.2f", expectedYearly, maintBreakdown.Yearly)
	}
}

func TestCalculateFiveYearTotal(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 年成本 = 硬件1560 + 电力350.4 + 维护900 + 许可300 = 3110.4
	// 5年总 = 7800 + 3110.4 × 5 = 7800 + 15552 = 23352
	annualCost := 1560.0 + 350.4 + 900.0 + 300.0
	expected := 7800.0 + annualCost*5
	if absDiff(report.NASFiveYearTotal, expected) > 1 {
		t.Errorf("5年总成本期望%.2f, 实际%.2f", expected, report.NASFiveYearTotal)
	}
}

func TestCalculateAnnualAverage(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	expected := report.NASFiveYearTotal / 5
	if absDiff(report.NASAnnualAverage, expected) > 0.01 {
		t.Errorf("年均成本期望%.2f, 实际%.2f", expected, report.NASAnnualAverage)
	}
}

func TestCloudComparison(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	if len(report.CloudComparisons) == 0 {
		t.Fatal("应有至少一个云方案对比")
	}

	for _, cc := range report.CloudComparisons {
		if cc.FiveYearCost <= 0 {
			t.Errorf("云方案 %s 5年成本应大于0", cc.ProviderName)
		}
		if cc.StorageSizeTB != req.StorageTB {
			t.Errorf("云方案 %s 存储容量期望%.1f TB, 实际%.1f TB", cc.ProviderName, req.StorageTB, cc.StorageSizeTB)
		}
	}
}

func TestCheaperChoice(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// NAS 一般比云便宜
	if report.CheaperChoice != "nas" && report.CheaperChoice != "cloud" {
		t.Errorf("更经济方案应为 nas 或 cloud, 实际 %s", report.CheaperChoice)
	}

	if report.SavingsAmount < 0 {
		t.Error("节省金额不应为负")
	}
}

func TestCostPerTB(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	if report.NASCostPerTB <= 0 {
		t.Error("NAS 每TB成本应大于0")
	}

	// NAS年成本 / 24TB
	expected := report.NASAnnualAverage / req.StorageTB
	if absDiff(report.NASCostPerTB, roundToTwo(expected)) > 0.01 {
		t.Errorf("NAS 每TB成本期望%.2f, 实际%.2f", expected, report.NASCostPerTB)
	}
}

func TestEmptyHardware(t *testing.T) {
	svc := NewService()
	req := TCORequest{
		Hardware: []HardwareSpec{},
	}

	_, err := svc.Calculate(req)
	if err == nil {
		t.Error("空硬件配置应返回错误")
	}
}

func TestYearsTooLong(t *testing.T) {
	svc := NewService()
	req := makeRequest()
	req.Years = 25

	_, err := svc.Calculate(req)
	if err == nil {
		t.Error("超过20年应返回错误")
	}
}

func TestDefaultYears(t *testing.T) {
	svc := NewService()
	req := makeRequest()
	req.Years = 0

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	if report.Years != 5 {
		t.Errorf("默认年限应为5, 实际%d", report.Years)
	}
}

func TestTCOItems(t *testing.T) {
	svc := NewService()
	req := makeRequest()

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	if len(report.Items) == 0 {
		t.Fatal("应有TCO单项")
	}

	// 检查有硬件项
	foundHardware := false
	for _, item := range report.Items {
		if item.Category == CategoryHardware {
			foundHardware = true
			if item.UpfrontCost <= 0 {
				t.Error("硬件项 upfront 成本应大于0")
			}
		}
	}
	if !foundHardware {
		t.Error("未找到硬件TCO项")
	}
}

func TestGetCloudPricing(t *testing.T) {
	svc := NewService()
	pricing := svc.GetCloudPricing()

	if len(pricing) == 0 {
		t.Fatal("应有预置云定价")
	}

	for _, p := range pricing {
		if p.ProviderName == "" {
			t.Error("服务商名称不能为空")
		}
		if p.PricePerGBMonth <= 0 {
			t.Errorf("服务商 %s 每GB价格应大于0", p.ProviderName)
		}
	}
}

// === HTTP Handler 测试 ===

func TestHandlerCalculate(t *testing.T) {
	router := setupTestRouter()

	body, _ := json.Marshal(makeRequest())
	req, _ := http.NewRequest("POST", "/api/v1/tcocalc/calculate", bytes.NewBuffer(body))
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

	if data["cheaper_choice"] == nil {
		t.Fatal("响应应包含 cheaper_choice")
	}
}

func TestHandlerCalculateBadJSON(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/tcocalc/calculate", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400, 实际%d", w.Code)
	}
}

func TestHandlerCalculateEmptyHardware(t *testing.T) {
	router := setupTestRouter()

	body, _ := json.Marshal(TCORequest{Hardware: []HardwareSpec{}})
	req, _ := http.NewRequest("POST", "/api/v1/tcocalc/calculate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400, 实际%d, 响应: %s", w.Code, w.Body.String())
	}
}

func TestHandlerCloudPricing(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tcocalc/cloud-pricing", nil)
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
		t.Fatal("应返回至少一个云定价")
	}
}

// === 边界条件测试 ===

func TestZeroStorageTB(t *testing.T) {
	svc := NewService()
	req := makeRequest()
	req.StorageTB = 0

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// StorageTB 为 0 时每TB成本为 0
	if report.NASCostPerTB != 0 {
		t.Errorf("StorageTB=0 时每TB成本应为0, 实际%.2f", report.NASCostPerTB)
	}
}

func TestZeroPowerWatts(t *testing.T) {
	svc := NewService()
	req := makeRequest()
	req.Power.Watts = 0

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 应使用默认50W
	var powerBreakdown *CostBreakdown
	for i := range report.Breakdowns {
		if report.Breakdowns[i].Category == CategoryPower {
			powerBreakdown = &report.Breakdowns[i]
			break
		}
	}
	if powerBreakdown == nil {
		t.Fatal("未找到电力成本明细")
	}

	expectedYearly := 50.0 * 24 * 365 / 1000 * 0.5
	if absDiff(powerBreakdown.Yearly, expectedYearly) > 0.01 {
		t.Errorf("默认电力年成本期望%.2f, 实际%.2f", expectedYearly, powerBreakdown.Yearly)
	}
}

func TestPerpetualLicense(t *testing.T) {
	svc := NewService()
	req := makeRequest()
	req.Licenses = []LicenseSpec{
		{Name: "永久许可", Type: "perpetual", Price: 2000, Quantity: 1},
	}

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 永久许可 upfront 应包含 2000
	expectedUpfront := 3000.0 + 1200.0*4 + 2000.0
	if absDiff(report.NASUpfrontTotal, expectedUpfront) > 0.01 {
		t.Errorf("含永久许可一次性成本期望%.2f, 实际%.2f", expectedUpfront, report.NASUpfrontTotal)
	}

	// 许可年度成本应为 0（仅永久许可）
	var licBreakdown *CostBreakdown
	for i := range report.Breakdowns {
		if report.Breakdowns[i].Category == CategoryLicense {
			licBreakdown = &report.Breakdowns[i]
			break
		}
	}
	if licBreakdown == nil {
		t.Fatal("未找到许可成本明细")
	}
	if licBreakdown.Yearly != 0 {
		t.Errorf("永久许可年成本应为0, 实际%.2f", licBreakdown.Yearly)
	}
}

func TestMaintenanceNoReplacement(t *testing.T) {
	svc := NewService()
	req := makeRequest()
	req.Maintenance.ReplacementCost = 0
	req.Maintenance.ReplaceInterval = 0

	report, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	var maintBreakdown *CostBreakdown
	for i := range report.Breakdowns {
		if report.Breakdowns[i].Category == CategoryMaint {
			maintBreakdown = &report.Breakdowns[i]
			break
		}
	}
	if maintBreakdown == nil {
		t.Fatal("未找到维护成本明细")
	}

	// 仅年维护费500，无更换成本
	if absDiff(maintBreakdown.Yearly, 500.0) > 0.01 {
		t.Errorf("无更换维护年成本期望500.00, 实际%.2f", maintBreakdown.Yearly)
	}
}

// 保留 time 引用避免 unused import.
var _ = time.Now
