package stocatcalc

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// absDiff 返回两个浮点数之差的绝对值.
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

// === 服务层测试 ===

func TestCalculatePureHDD(t *testing.T) {
	svc := NewService()
	req := CalcRequest{
		Disks: []DiskSpec{
			{Type: DiskTypeHDD, CapacityTB: 8, Price: 1200, Quantity: 4},
		},
		PowerPriceKWh: 0.5,
		Years:         3,
		RaidLevel:     "raid5",
	}

	result, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 硬件成本 = 1200 × 4 = 4800
	if result.HardwareCost != 4800 {
		t.Errorf("硬件成本期望4800, 实际%.2f", result.HardwareCost)
	}

	// 原始容量 = 8 × 4 = 32 TB
	if result.RawCapacityTB != 32 {
		t.Errorf("原始容量期望32, 实际%.2f", result.RawCapacityTB)
	}

	// RAID5可用容量 = 32 × 0.75 = 24 TB
	if result.UsableCapacityTB != 24 {
		t.Errorf("可用容量期望24, 实际%.2f", result.UsableCapacityTB)
	}

	// 功耗 = 6.5 × 4 = 26W
	if result.PowerWatts != 26 {
		t.Errorf("功耗期望26, 实际%.2f", result.PowerWatts)
	}

	// 方案类型
	if result.Scheme != SchemePureHDD {
		t.Errorf("方案类型期望pure_hdd, 实际%s", result.Scheme)
	}

	// 总成本应大于硬件成本
	if result.TotalCost <= result.HardwareCost {
		t.Error("总成本应包含电力成本，大于硬件成本")
	}

	// 每TB成本 = 总成本 / 24（允许浮点误差）
	expectedPerTB := result.TotalCost / 24
	if absDiff(result.CostPerTB, expectedPerTB) > 0.01 {
		t.Errorf("每TB成本期望%.2f, 实际%.2f", expectedPerTB, result.CostPerTB)
	}
}

func TestCalculatePureSSD(t *testing.T) {
	svc := NewService()
	req := CalcRequest{
		Disks: []DiskSpec{
			{Type: DiskTypeSSD, CapacityTB: 4, Price: 2000, Quantity: 4},
		},
		PowerPriceKWh: 0.6,
		Years:         5,
		RaidLevel:     "raid10",
	}

	result, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 硬件成本 = 2000 × 4 = 8000
	if result.HardwareCost != 8000 {
		t.Errorf("硬件成本期望8000, 实际%.2f", result.HardwareCost)
	}

	// RAID10可用容量 = 16 × 0.5 = 8 TB
	if result.UsableCapacityTB != 8 {
		t.Errorf("可用容量期望8, 实际%.2f", result.UsableCapacityTB)
	}

	if result.Scheme != SchemePureSSD {
		t.Errorf("方案类型期望pure_ssd, 实际%s", result.Scheme)
	}

	// 年化成本 = 总成本 / 5（允许浮点误差）
	expectedAnnual := result.TotalCost / 5
	if absDiff(result.AnnualCost, expectedAnnual) > 0.01 {
		t.Errorf("年化成本期望%.2f, 实际%.2f", expectedAnnual, result.AnnualCost)
	}
}

func TestCalculateHybrid(t *testing.T) {
	svc := NewService()
	req := CalcRequest{
		Disks: []DiskSpec{
			{Type: DiskTypeNVMe, CapacityTB: 2, Price: 800, Quantity: 2},
			{Type: DiskTypeHDD, CapacityTB: 16, Price: 2200, Quantity: 4},
		},
		PowerPriceKWh: 0.5,
		Years:         3,
		RaidLevel:     "raid5",
	}

	result, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 硬件成本 = 800×2 + 2200×4 = 1600 + 8800 = 10400
	if result.HardwareCost != 10400 {
		t.Errorf("硬件成本期望10400, 实际%.2f", result.HardwareCost)
	}

	// 原始容量 = 2×2 + 16×4 = 4 + 64 = 68 TB
	if result.RawCapacityTB != 68 {
		t.Errorf("原始容量期望68, 实际%.2f", result.RawCapacityTB)
	}

	// 混合方案
	if result.Scheme != SchemeHybrid {
		t.Errorf("方案类型期望hybrid, 实际%s", result.Scheme)
	}
}

func TestCalculatePowerCost(t *testing.T) {
	svc := NewService()
	req := CalcRequest{
		Disks: []DiskSpec{
			{Type: DiskTypeHDD, CapacityTB: 4, Price: 600, Quantity: 2},
		},
		PowerPriceKWh: 1.0, // 1元/kWh方便计算
		Years:         1,
		RaidLevel:     "raid1",
	}

	result, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 功耗 = 6.5 × 2 = 13W
	// 年用电量 = 13 × 24 × 365 / 1000 = 113.88 kWh
	// 电力成本 = 113.88 × 1.0 = 113.88元
	expectedKWh := 13.0 * 24 * 365 / 1000
	expectedPower := expectedKWh * 1.0
	if result.PowerCost != expectedPower {
		t.Errorf("电力成本期望%.2f, 实际%.2f", expectedPower, result.PowerCost)
	}
}

func TestCalculateEmptyDisks(t *testing.T) {
	svc := NewService()
	req := CalcRequest{
		Disks:         []DiskSpec{},
		PowerPriceKWh: 0.5,
		Years:         3,
		RaidLevel:     "raid5",
	}

	_, err := svc.Calculate(req)
	if err == nil {
		t.Error("空磁盘配置应返回错误")
	}
}

func TestCalculateInvalidQuantity(t *testing.T) {
	svc := NewService()
	req := CalcRequest{
		Disks: []DiskSpec{
			{Type: DiskTypeHDD, CapacityTB: 8, Price: 1200, Quantity: 0},
		},
		PowerPriceKWh: 0.5,
		Years:         3,
		RaidLevel:     "raid5",
	}

	_, err := svc.Calculate(req)
	if err == nil {
		t.Error("数量为0应返回错误")
	}
}

func TestCalculateDefaultValues(t *testing.T) {
	svc := NewService()
	req := CalcRequest{
		Disks: []DiskSpec{
			{Type: DiskTypeHDD, CapacityTB: 4, Price: 600, Quantity: 2},
		},
		// 不设置Years和PowerPriceKWh
		RaidLevel: "raid1",
	}

	result, err := svc.Calculate(req)
	if err != nil {
		t.Fatalf("计算失败: %v", err)
	}

	// 默认1年
	if result.AnnualCost != result.TotalCost {
		t.Errorf("默认1年时年化成本应等于总成本")
	}
}

func TestCompare(t *testing.T) {
	svc := NewService()
	reqs := []CalcRequest{
		{
			Disks: []DiskSpec{
				{Type: DiskTypeHDD, CapacityTB: 8, Price: 1200, Quantity: 4},
			},
			PowerPriceKWh: 0.5,
			Years:         3,
			RaidLevel:     "raid5",
		},
		{
			Disks: []DiskSpec{
				{Type: DiskTypeSSD, CapacityTB: 4, Price: 2000, Quantity: 4},
			},
			PowerPriceKWh: 0.5,
			Years:         3,
			RaidLevel:     "raid5",
		},
	}

	result, err := svc.Compare(reqs)
	if err != nil {
		t.Fatalf("对比失败: %v", err)
	}

	if len(result.Results) != 2 {
		t.Errorf("期望2个结果, 实际%d", len(result.Results))
	}

	if result.BestByTotal == nil {
		t.Error("应返回总成本最低方案")
	}

	if result.BestByPerTB == nil {
		t.Error("应返回每TB成本最低方案")
	}

	// HDD方案总成本应低于SSD方案
	if result.BestByTotal.Scheme != SchemePureHDD {
		t.Errorf("总成本最低应为pure_hdd, 实际%s", result.BestByTotal.Scheme)
	}
}

func TestCompareEmpty(t *testing.T) {
	svc := NewService()
	_, err := svc.Compare([]CalcRequest{})
	if err == nil {
		t.Error("空方案列表应返回错误")
	}
}

func TestGetTemplates(t *testing.T) {
	svc := NewService()
	templates := svc.GetTemplates()

	if len(templates) == 0 {
		t.Error("应返回预置模板")
	}

	// 检查模板ID唯一性
	ids := make(map[string]bool)
	for _, tpl := range templates {
		if ids[tpl.ID] {
			t.Errorf("模板ID重复: %s", tpl.ID)
		}
		ids[tpl.ID] = true
	}
}

func TestInferScheme(t *testing.T) {
	tests := []struct {
		disks  []DiskSpec
		expect StorageScheme
	}{
		{
			disks:  []DiskSpec{{Type: DiskTypeHDD, CapacityTB: 8, Price: 1200, Quantity: 4}},
			expect: SchemePureHDD,
		},
		{
			disks:  []DiskSpec{{Type: DiskTypeSSD, CapacityTB: 4, Price: 2000, Quantity: 4}},
			expect: SchemePureSSD,
		},
		{
			disks:  []DiskSpec{{Type: DiskTypeNVMe, CapacityTB: 8, Price: 6000, Quantity: 6}},
			expect: SchemePureNVMe,
		},
		{
			disks: []DiskSpec{
				{Type: DiskTypeNVMe, CapacityTB: 2, Price: 800, Quantity: 2},
				{Type: DiskTypeHDD, CapacityTB: 16, Price: 2200, Quantity: 4},
			},
			expect: SchemeHybrid,
		},
	}

	for _, tt := range tests {
		got := inferScheme(tt.disks)
		if got != tt.expect {
			t.Errorf("期望%s, 实际%s", tt.expect, got)
		}
	}
}

// === HTTP Handler 测试 ===

func TestHandlerCalculate(t *testing.T) {
	router := setupTestRouter()

	body := `{
		"disks": [
			{"type": "hdd", "capacity_tb": 8, "price": 1200, "quantity": 4}
		],
		"power_price_kwh": 0.5,
		"years": 3,
		"raid_level": "raid5"
	}`

	req, _ := http.NewRequest("POST", "/api/v1/stocatcalc/calculate", bytes.NewBufferString(body))
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

	if data["hardware_cost"].(float64) != 4800 {
		t.Errorf("硬件成本期望4800, 实际%v", data["hardware_cost"])
	}
}

func TestHandlerCalculateBadJSON(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/stocatcalc/calculate", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400, 实际%d", w.Code)
	}
}

func TestHandlerTemplates(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/stocatcalc/templates", nil)
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
		t.Error("应返回至少一个模板")
	}
}

func TestHandlerCompare(t *testing.T) {
	router := setupTestRouter()

	body := `[
		{
			"disks": [{"type": "hdd", "capacity_tb": 8, "price": 1200, "quantity": 4}],
			"power_price_kwh": 0.5,
			"years": 3,
			"raid_level": "raid5"
		},
		{
			"disks": [{"type": "ssd", "capacity_tb": 4, "price": 2000, "quantity": 4}],
			"power_price_kwh": 0.5,
			"years": 3,
			"raid_level": "raid5"
		}
	]`

	req, _ := http.NewRequest("POST", "/api/v1/stocatcalc/compare", bytes.NewBufferString(body))
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

	results := data["results"].([]interface{})
	if len(results) != 2 {
		t.Errorf("期望2个结果, 实际%d", len(results))
	}
}
