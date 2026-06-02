package smartenergyforecast

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	config := DefaultConfig()
	forecast := New(config)

	if forecast == nil {
		t.Fatal("New returned nil")
	}

	if forecast.config != config {
		t.Error("config not set correctly")
	}

	if forecast.readings == nil {
		t.Error("readings not initialized")
	}

	if forecast.forecasts == nil {
		t.Error("forecasts not initialized")
	}

	if forecast.anomalies == nil {
		t.Error("anomalies not initialized")
	}

	if forecast.optimizations == nil {
		t.Error("optimizations not initialized")
	}

	if forecast.dailyReports == nil {
		t.Error("dailyReports not initialized")
	}

	if forecast.running {
		t.Error("should not be running initially")
	}
}

func TestNewWithNilConfig(t *testing.T) {
	forecast := New(nil)

	if forecast == nil {
		t.Fatal("New returned nil")
	}

	if forecast.config == nil {
		t.Fatal("default config not applied")
	}

	if forecast.config.ForecastWindow != 24*time.Hour {
		t.Errorf("expected ForecastWindow 24h, got %v", forecast.config.ForecastWindow)
	}

	if forecast.config.HistoryDepth != 7*24*time.Hour {
		t.Errorf("expected HistoryDepth 7d, got %v", forecast.config.HistoryDepth)
	}
}

func TestStartStop(t *testing.T) {
	config := DefaultConfig()
	config.UpdateInterval = 100 * time.Millisecond
	forecast := New(config)

	ctx := context.Background()

	// 测试启动
	err := forecast.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !forecast.running {
		t.Error("should be running after Start")
	}

	// 测试重复启动
	err = forecast.Start(ctx)
	if err == nil {
		t.Error("expected error on double Start")
	}

	// 测试停止
	forecast.Stop()

	if forecast.running {
		t.Error("should not be running after Stop")
	}
}

func TestRecordReading(t *testing.T) {
	config := DefaultConfig()
	forecast := New(config)

	// 测试 nil reading
	err := forecast.RecordReading(nil)
	if err == nil {
		t.Error("expected error for nil reading")
	}

	// 测试正常记录
	reading := &EnergyReading{
		Timestamp:   time.Now(),
		Consumption: 1.5,
		Device:      "test-device",
		Category:    "test",
		PowerFactor: 0.95,
		Voltage:     220,
		Current:     10,
	}

	err = forecast.RecordReading(reading)
	if err != nil {
		t.Fatalf("RecordReading failed: %v", err)
	}

	if len(forecast.readings) != 1 {
		t.Errorf("expected 1 reading, got %d", len(forecast.readings))
	}

	if forecast.readings[0].Consumption != 1.5 {
		t.Errorf("expected consumption 1.5, got %f", forecast.readings[0].Consumption)
	}
}

func TestRecordReadingHistoryDepth(t *testing.T) {
	config := DefaultConfig()
	config.HistoryDepth = 1 * time.Hour
	forecast := New(config)

	// 记录旧数据
	oldReading := &EnergyReading{
		Timestamp:   time.Now().Add(-2 * time.Hour),
		Consumption: 1.0,
		Device:      "old-device",
	}
	forecast.RecordReading(oldReading)

	// 记录新数据
	newReading := &EnergyReading{
		Timestamp:   time.Now(),
		Consumption: 2.0,
		Device:      "new-device",
	}
	forecast.RecordReading(newReading)

	// 旧数据应该被裁剪
	if len(forecast.readings) != 1 {
		t.Errorf("expected 1 reading after trim, got %d", len(forecast.readings))
	}

	if forecast.readings[0].Device != "new-device" {
		t.Error("old reading not removed")
	}
}

func TestForecastInsufficientData(t *testing.T) {
	config := DefaultConfig()
	config.MinDataPoints = 5
	forecast := New(config)

	ctx := context.Background()

	// 只记录3个数据点
	for i := 0; i < 3; i++ {
		forecast.RecordReading(&EnergyReading{
			Timestamp:   time.Now(),
			Consumption: 1.0,
			Device:      "test",
		})
	}

	_, err := forecast.Forecast(ctx, 24*time.Hour)
	if err == nil {
		t.Error("expected error for insufficient data")
	}
}

func TestForecastWithSufficientData(t *testing.T) {
	config := DefaultConfig()
	config.MinDataPoints = 5
	forecast := New(config)

	ctx := context.Background()

	// 记录足够的数据
	for i := 0; i < 10; i++ {
		forecast.RecordReading(&EnergyReading{
			Timestamp:   time.Now().Add(-time.Duration(10-i) * time.Hour),
			Consumption: float64(i) * 0.5,
			Device:      "test",
		})
	}

	forecasts, err := forecast.Forecast(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("Forecast failed: %v", err)
	}

	if len(forecasts) == 0 {
		t.Fatal("expected forecasts, got empty")
	}

	// 验证预测结果
	for _, f := range forecasts {
		if f.Expected < 0 {
			t.Error("expected should not be negative")
		}

		if f.Lower > f.Expected {
			t.Error("lower bound should be <= expected")
		}

		if f.Upper < f.Expected {
			t.Error("upper bound should be >= expected")
		}

		if f.Confidence < 0.5 || f.Confidence > 1.0 {
			t.Errorf("confidence should be 0.5-1.0, got %f", f.Confidence)
		}
	}
}

func TestDetectAnomalies(t *testing.T) {
	config := DefaultConfig()
	config.MinDataPoints = 5
	config.AnomalyThreshold = 2.0
	forecast := New(config)

	ctx := context.Background()

	// 记录正常数据
	for i := 0; i < 10; i++ {
		forecast.RecordReading(&EnergyReading{
			Timestamp:   time.Now().Add(-time.Duration(10-i) * time.Hour),
			Consumption: 1.0 + float64(i)*0.1,
			Device:      "test",
		})
	}

	// 记录异常数据
	forecast.RecordReading(&EnergyReading{
		Timestamp:   time.Now(),
		Consumption: 100.0, // 异常高
		Device:      "test",
	})

	anomalies, err := forecast.DetectAnomalies(ctx)
	if err != nil {
		t.Fatalf("DetectAnomalies failed: %v", err)
	}

	if len(anomalies) == 0 {
		t.Error("expected to detect anomaly")
	}

	// 验证异常检测
	for _, a := range anomalies {
		if a.Severity == "" {
			t.Error("severity not set")
		}

		if a.Description == "" {
			t.Error("description not set")
		}

		if a.Reading == nil {
			t.Error("reading not set")
		}
	}
}

func TestGetOptimizations(t *testing.T) {
	config := DefaultConfig()
	config.MinDataPoints = 5
	forecast := New(config)

	// 记录高能耗数据
	for i := 0; i < 10; i++ {
		forecast.RecordReading(&EnergyReading{
			Timestamp:   time.Now().Add(-time.Duration(10-i) * time.Hour),
			Consumption: 2.0,
			Device:      "high-consumption-device",
			PowerFactor: 0.7,
		})
	}

	optimizations := forecast.GetOptimizations()

	if len(optimizations) == 0 {
		t.Error("expected optimizations")
	}

	for _, opt := range optimizations {
		if opt.ID == "" {
			t.Error("optimization ID not set")
		}

		if opt.Title == "" {
			t.Error("optimization title not set")
		}

		if opt.Description == "" {
			t.Error("optimization description not set")
		}

		if opt.Priority == "" {
			t.Error("optimization priority not set")
		}

		if opt.Savings < 0 {
			t.Error("savings should not be negative")
		}
	}
}

func TestGetDailyReport(t *testing.T) {
	config := DefaultConfig()
	forecast := New(config)

	// 记录今日数据
	for i := 0; i < 5; i++ {
		forecast.RecordReading(&EnergyReading{
			Timestamp:   time.Now().Add(-time.Duration(i) * time.Hour),
			Consumption: float64(i) * 0.5,
			Device:      "test-device",
		})
	}

	report := forecast.GetDailyReport()

	if report == nil {
		t.Fatal("expected report, got nil")
	}

	if report.TotalConsumed < 0 {
		t.Error("total consumed should not be negative")
	}

	if report.DeviceBreakdown == nil {
		t.Error("device breakdown not set")
	}

	if report.CostEstimate < 0 {
		t.Error("cost estimate should not be negative")
	}
}

func TestGetWeeklyTrend(t *testing.T) {
	config := DefaultConfig()
	forecast := New(config)

	// 记录一周的数据
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			forecast.RecordReading(&EnergyReading{
				Timestamp:   time.Now().AddDate(0, 0, -d).Add(-time.Duration(h) * time.Hour),
				Consumption: 1.0 + float64(d)*0.1,
				Device:      "test-device",
			})
		}
	}

	trend := forecast.GetWeeklyTrend()

	if trend == nil {
		t.Fatal("expected trend, got nil")
	}

	if len(trend.DailyData) != 7 {
		t.Errorf("expected 7 days of data, got %d", len(trend.DailyData))
	}

	if trend.TotalConsumed < 0 {
		t.Error("total consumed should not be negative")
	}

	if trend.TrendDirection == "" {
		t.Error("trend direction not set")
	}
}

func TestSetBudgetAndGetStatus(t *testing.T) {
	config := DefaultConfig()
	forecast := New(config)

	// 设置预算
	budget := &EnergyBudget{
		DailyBudget:   10.0,
		WeeklyBudget:  70.0,
		MonthlyBudget: 300.0,
		PricePerKwh:   0.56,
	}
	forecast.SetBudget(budget)

	// 记录一些数据
	for i := 0; i < 5; i++ {
		forecast.RecordReading(&EnergyReading{
			Timestamp:   time.Now().Add(-time.Duration(i) * time.Hour),
			Consumption: 3.0,
			Device:      "test",
		})
	}

	status := forecast.GetBudgetStatus()

	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.DailyBudget != 10.0 {
		t.Errorf("expected daily budget 10.0, got %f", status.DailyBudget)
	}

	if status.WeeklyBudget != 70.0 {
		t.Errorf("expected weekly budget 70.0, got %f", status.WeeklyBudget)
	}

	if status.MonthlyBudget != 300.0 {
		t.Errorf("expected monthly budget 300.0, got %f", status.MonthlyBudget)
	}

	if status.Status == "" {
		t.Error("status not set")
	}
}

func TestGetBudgetStatusNoBudget(t *testing.T) {
	config := DefaultConfig()
	forecast := New(config)

	status := forecast.GetBudgetStatus()

	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.Status != "no_budget" {
		t.Errorf("expected status 'no_budget', got '%s'", status.Status)
	}
}

func TestCalculateCost(t *testing.T) {
	config := DefaultConfig()
	forecast := New(config)

	// 测试 nil reading
	cost := forecast.CalculateCost(nil, 0.56)
	if cost != 0 {
		t.Error("expected 0 for nil reading")
	}

	// 测试无效价格
	reading := &EnergyReading{
		Consumption: 10.0,
	}
	cost = forecast.CalculateCost(reading, 0)
	if cost != 0 {
		t.Error("expected 0 for zero price")
	}

	cost = forecast.CalculateCost(reading, -1)
	if cost != 0 {
		t.Error("expected 0 for negative price")
	}

	// 测试正常计算
	cost = forecast.CalculateCost(reading, 0.56)
	expected := 10.0 * 0.56
	if cost < expected-0.0001 || cost > expected+0.0001 {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ForecastWindow != 24*time.Hour {
		t.Errorf("expected ForecastWindow 24h, got %v", config.ForecastWindow)
	}

	if config.HistoryDepth != 7*24*time.Hour {
		t.Errorf("expected HistoryDepth 7d, got %v", config.HistoryDepth)
	}

	if config.AnomalyThreshold != 2.0 {
		t.Errorf("expected AnomalyThreshold 2.0, got %f", config.AnomalyThreshold)
	}

	if config.BudgetAlertRatio != 0.8 {
		t.Errorf("expected BudgetAlertRatio 0.8, got %f", config.BudgetAlertRatio)
	}

	if config.MinDataPoints != 10 {
		t.Errorf("expected MinDataPoints 10, got %d", config.MinDataPoints)
	}

	if config.SmoothingFactor != 0.3 {
		t.Errorf("expected SmoothingFactor 0.3, got %f", config.SmoothingFactor)
	}

	if config.UpdateInterval != time.Hour {
		t.Errorf("expected UpdateInterval 1h, got %v", config.UpdateInterval)
	}

	if config.DefaultPricePerKwh != 0.56 {
		t.Errorf("expected DefaultPricePerKwh 0.56, got %f", config.DefaultPricePerKwh)
	}
}
