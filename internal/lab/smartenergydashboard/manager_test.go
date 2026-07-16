package smartenergydashboard

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	// 验证默认设置
	settings := m.GetSettings()
	if settings.ElectricityRate != 0.56 {
		t.Errorf("expected electricity rate 0.56, got %f", settings.ElectricityRate)
	}
	if settings.CarbonFactor != 0.785 {
		t.Errorf("expected carbon factor 0.785, got %f", settings.CarbonFactor)
	}
	if !settings.MonitoringEnabled {
		t.Error("expected monitoring enabled")
	}
}

func TestRecordPowerReading(t *testing.T) {
	m := NewManager()

	reading := m.RecordPowerReading("cpu", 10.5, 12.0, 0.875)
	if reading == nil {
		t.Fatal("RecordPowerReading returned nil")
	}
	if reading.Wattage != 10.5 {
		t.Errorf("expected wattage 10.5, got %f", reading.Wattage)
	}
	if reading.Source != "cpu" {
		t.Errorf("expected source 'cpu', got %s", reading.Source)
	}
	if reading.ID == "" {
		t.Error("expected non-empty ID")
	}
	if reading.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRecordMultipleReadings(t *testing.T) {
	m := NewManager()

	for i := 0; i < 10; i++ {
		m.RecordPowerReading("system", float64(i)*10, 12.0, float64(i)*10/12.0)
	}

	// 验证读数已记录
	m.mu.RLock()
	count := len(m.readings)
	m.mu.RUnlock()

	if count != 10 {
		t.Errorf("expected 10 readings, got %d", count)
	}
}

func TestGetCurrentPower(t *testing.T) {
	m := NewManager()

	reading := m.GetCurrentPower()
	if reading == nil {
		t.Fatal("GetCurrentPower returned nil")
	}
	if reading.Source != "system" {
		t.Errorf("expected source 'system', got %s", reading.Source)
	}
	if reading.Wattage <= 0 {
		t.Error("expected positive wattage for online devices")
	}
}

func TestGetHistory(t *testing.T) {
	m := NewManager()

	// 测试 daily
	daily := m.GetHistory("daily")
	if len(daily) == 0 {
		t.Error("expected daily records, got 0")
	}

	// 测试 weekly
	weekly := m.GetHistory("weekly")
	if len(weekly) == 0 {
		t.Error("expected weekly records, got 0")
	}
	if len(weekly) < len(daily) {
		t.Error("expected weekly records >= daily records")
	}

	// 测试 monthly
	monthly := m.GetHistory("monthly")
	if len(monthly) == 0 {
		t.Error("expected monthly records, got 0")
	}

	// 测试默认
	defaultHistory := m.GetHistory("unknown")
	if len(defaultHistory) == 0 {
		t.Error("expected default records, got 0")
	}
}

func TestGetDevicePower(t *testing.T) {
	m := NewManager()

	devices := m.GetDevicePower()
	if len(devices) == 0 {
		t.Fatal("expected devices, got 0")
	}

	// 验证有不同类型的设备
	types := make(map[string]bool)
	for _, d := range devices {
		types[d.DeviceType] = true
	}

	expectedTypes := []string{"cpu", "psu", "fan", "ssd", "hdd", "nic"}
	for _, et := range expectedTypes {
		if !types[et] {
			t.Errorf("expected device type %s", et)
		}
	}
}

func TestSetAndGetBudget(t *testing.T) {
	m := NewManager()

	// 设置预算
	budget := m.SetBudget(100.0, 56.0, 80.0)
	if budget == nil {
		t.Fatal("SetBudget returned nil")
	}
	if budget.MonthlyLimitKWh != 100.0 {
		t.Errorf("expected limit 100, got %f", budget.MonthlyLimitKWh)
	}
	if budget.MonthlyLimitCost != 56.0 {
		t.Errorf("expected cost 56, got %f", budget.MonthlyLimitCost)
	}
	if budget.AlertThreshold != 80.0 {
		t.Errorf("expected threshold 80, got %f", budget.AlertThreshold)
	}

	// 获取预算
	got := m.GetBudget()
	if got == nil {
		t.Fatal("GetBudget returned nil after SetBudget")
	}
	if got.MonthlyLimitKWh != 100.0 {
		t.Errorf("expected limit 100, got %f", got.MonthlyLimitKWh)
	}
}

func TestGetBudgetNil(t *testing.T) {
	m := NewManager()

	budget := m.GetBudget()
	if budget != nil {
		t.Error("expected nil budget when not set")
	}
}

func TestUpdateBudget(t *testing.T) {
	m := NewManager()

	m.SetBudget(100.0, 56.0, 80.0)
	m.SetBudget(200.0, 112.0, 70.0)

	budget := m.GetBudget()
	if budget.MonthlyLimitKWh != 200.0 {
		t.Errorf("expected limit 200, got %f", budget.MonthlyLimitKWh)
	}
	if budget.AlertThreshold != 70.0 {
		t.Errorf("expected threshold 70, got %f", budget.AlertThreshold)
	}
}

func TestGenerateReport(t *testing.T) {
	m := NewManager()

	report := m.GenerateReport("monthly")
	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
	if report.Period != "monthly" {
		t.Errorf("expected period 'monthly', got %s", report.Period)
	}
	if report.TotalKWh <= 0 {
		t.Error("expected positive total kWh")
	}
	if report.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if report.CarbonKg <= 0 {
		t.Error("expected positive carbon kg")
	}
	if len(report.TopDevices) == 0 {
		t.Error("expected top devices")
	}
	if report.Trend == "" {
		t.Error("expected non-empty trend")
	}
	if len(report.SavingsTips) == 0 {
		t.Error("expected savings tips")
	}
}

func TestForecastCost(t *testing.T) {
	m := NewManager()

	forecasts := m.ForecastCost()
	if len(forecasts) == 0 {
		t.Fatal("expected forecasts, got 0")
	}
	if len(forecasts) != 3 {
		t.Errorf("expected 3 forecasts, got %d", len(forecasts))
	}

	for _, f := range forecasts {
		if f.Month == "" {
			t.Error("expected non-empty month")
		}
		if f.ProjectedKWh <= 0 {
			t.Error("expected positive projected kWh")
		}
		if f.ProjectedCost <= 0 {
			t.Error("expected positive projected cost")
		}
		if f.Confidence < 50 || f.Confidence > 100 {
			t.Errorf("expected confidence between 50-100, got %f", f.Confidence)
		}
		if len(f.Factors) == 0 {
			t.Error("expected factors")
		}
	}
}

func TestGetTips(t *testing.T) {
	m := NewManager()

	tips := m.GetTips()
	if len(tips) == 0 {
		t.Fatal("expected tips, got 0")
	}

	// 验证有不同类别的建议
	categories := make(map[string]bool)
	for _, tip := range tips {
		categories[tip.Category] = true
		if tip.Title == "" {
			t.Error("expected non-empty title")
		}
		if tip.Description == "" {
			t.Error("expected non-empty description")
		}
		if tip.Impact == "" {
			t.Error("expected non-empty impact")
		}
		if tip.SavingsKWh <= 0 {
			t.Error("expected positive savings kWh")
		}
		if tip.SavingsCost <= 0 {
			t.Error("expected positive savings cost")
		}
	}

	if !categories["hardware"] {
		t.Error("expected hardware tips")
	}
	if !categories["software"] {
		t.Error("expected software tips")
	}
}

func TestUpdateSettings(t *testing.T) {
	m := NewManager()

	settings := &EnergySettings{
		ElectricityRate:   0.80,
		CarbonFactor:      0.9,
		Currency:          "USD",
		MonitoringEnabled: false,
		AlertEnabled:      false,
	}

	m.UpdateSettings(settings)

	got := m.GetSettings()
	if got.ElectricityRate != 0.80 {
		t.Errorf("expected rate 0.80, got %f", got.ElectricityRate)
	}
	if got.CarbonFactor != 0.9 {
		t.Errorf("expected factor 0.9, got %f", got.CarbonFactor)
	}
	if got.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", got.Currency)
	}
	if got.MonitoringEnabled {
		t.Error("expected monitoring disabled")
	}
	if got.AlertEnabled {
		t.Error("expected alert disabled")
	}
}

func TestGetSettings(t *testing.T) {
	m := NewManager()

	settings := m.GetSettings()
	if settings == nil {
		t.Fatal("GetSettings returned nil")
	}
}

func TestReportTopDevices(t *testing.T) {
	m := NewManager()

	report := m.GenerateReport("monthly")
	if len(report.TopDevices) == 0 {
		t.Fatal("expected top devices")
	}

	// 验证排序：第一个设备的月耗电量 >= 第二个
	for i := 1; i < len(report.TopDevices); i++ {
		if report.TopDevices[i-1].MonthlyKWh < report.TopDevices[i].MonthlyKWh {
			t.Error("expected devices sorted by monthly kWh descending")
			break
		}
	}
}

func TestReportTrends(t *testing.T) {
	m := NewManager()

	report := m.GenerateReport("daily")
	if report.Trend != "up" && report.Trend != "down" && report.Trend != "stable" {
		t.Errorf("expected valid trend, got %s", report.Trend)
	}
}

func TestBudgetProjectedUsage(t *testing.T) {
	m := NewManager()

	m.SetBudget(100.0, 56.0, 80.0)
	budget := m.GetBudget()

	// 预测用量应该 > 0（有历史数据）
	if budget.ProjectedUsage <= 0 {
		t.Error("expected positive projected usage")
	}
}

func TestSettingsTimeUpdate(t *testing.T) {
	m := NewManager()

	oldSettings := m.GetSettings()
	oldTime := oldSettings.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	newSettings := &EnergySettings{
		ElectricityRate: 1.0,
		CarbonFactor:    1.0,
		Currency:        "CNY",
	}
	m.UpdateSettings(newSettings)

	got := m.GetSettings()
	if !got.UpdatedAt.After(oldTime) {
		t.Error("expected updated time to be newer")
	}
}

func TestBudgetCurrentUsage(t *testing.T) {
	m := NewManager()

	m.SetBudget(100.0, 56.0, 80.0)
	budget := m.GetBudget()

	// 当前用量应该 > 0（有 30 天历史数据）
	if budget.CurrentUsage <= 0 {
		t.Error("expected positive current usage")
	}
}

func TestForecastConfidenceDecay(t *testing.T) {
	m := NewManager()

	forecasts := m.ForecastCost()
	if len(forecasts) < 3 {
		t.Fatal("expected 3 forecasts")
	}

	// 信心值应该随时间递减
	if forecasts[0].Confidence <= forecasts[1].Confidence {
		t.Error("expected confidence to decrease over time")
	}
	if forecasts[1].Confidence <= forecasts[2].Confidence {
		t.Error("expected confidence to decrease over time")
	}
}
