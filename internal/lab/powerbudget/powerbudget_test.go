package powerbudget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// ========== 辅助函数 ==========

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	engine := NewEngine(logger)
	err := engine.Start()
	assert.NoError(t, err)
	return engine
}

func testRecordPower(t *testing.T, e *Engine, deviceID, deviceName string, watts float64) *PowerRecord {
	t.Helper()
	req := RecordPowerRequest{
		DeviceID:    deviceID,
		DeviceName:  deviceName,
		PowerWatts:  watts,
		DurationSec: 3600,
	}
	record, err := e.RecordPower(req)
	assert.NoError(t, err)
	return record
}

// ========== Engine 基础测试 ==========

func TestNewEngine(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewEngine(logger)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.records)
	assert.NotNil(t, engine.alerts)
	assert.NotNil(t, engine.devices)
	assert.NotNil(t, engine.tracker)
	assert.NotNil(t, engine.analyzer)
	assert.NotNil(t, engine.alertMgr)
	assert.False(t, engine.IsRunning())
}

func TestEngineStartStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewEngine(logger)

	err := engine.Start()
	assert.NoError(t, err)
	assert.True(t, engine.IsRunning())

	// 重复启动应该成功
	err = engine.Start()
	assert.NoError(t, err)

	err = engine.Stop()
	assert.NoError(t, err)
	assert.False(t, engine.IsRunning())

	// 重复停止应该成功
	err = engine.Stop()
	assert.NoError(t, err)
}

func TestNewEngineNilLogger(t *testing.T) {
	engine := NewEngine(nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.logger)
}

// ========== RecordPower 测试 ==========

func TestRecordPower(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	req := RecordPowerRequest{
		DeviceID:    "device-001",
		DeviceName:  "NAS主机",
		PowerWatts:  150.0,
		DurationSec: 3600,
		Service:     "storage",
		Metadata:    map[string]string{"rack": "A1"},
	}

	record, err := engine.RecordPower(req)
	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.NotEmpty(t, record.ID)
	assert.Equal(t, "device-001", record.DeviceID)
	assert.Equal(t, "NAS主机", record.DeviceName)
	assert.Equal(t, 150.0, record.PowerWatts)
	assert.Greater(t, record.EnergyKWh, 0.0)
	assert.Greater(t, record.CostCents, int64(0))
	assert.Equal(t, "storage", record.Service)
}

func TestRecordPowerDefaultDuration(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	req := RecordPowerRequest{
		DeviceID:   "device-002",
		DeviceName: "路由器",
		PowerWatts: 15.0,
	}

	record, err := engine.RecordPower(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(60), record.Duration)
}

func TestRecordPowerInvalidWatts(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	req := RecordPowerRequest{
		DeviceID:   "device-003",
		DeviceName: "设备",
		PowerWatts: -10.0,
	}

	_, err := engine.RecordPower(req)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidPowerWatts, err)
}

func TestRecordPowerEngineNotRunning(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := NewEngine(logger)

	req := RecordPowerRequest{
		DeviceID:   "device-004",
		DeviceName: "设备",
		PowerWatts: 100.0,
	}

	_, err := engine.RecordPower(req)
	assert.Error(t, err)
	assert.Equal(t, ErrEngineNotRunning, err)
}

func TestRecordPowerEnergyCalculation(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	// 设置预算以确定电价
	engine.SetBudget(SetBudgetRequest{
		MonthlyAmount:    20000,
		ElectricityPrice: 0.56,
	})

	req := RecordPowerRequest{
		DeviceID:    "device-005",
		DeviceName:  "测试设备",
		PowerWatts:  1000.0,
		DurationSec: 3600,
	}

	record, err := engine.RecordPower(req)
	assert.NoError(t, err)

	// 1000W * 1h = 1kWh
	assert.InDelta(t, 1.0, record.EnergyKWh, 0.01)
	// 1kWh * 0.56元/kWh = 0.56元 = 56分
	assert.Equal(t, int64(56), record.CostCents)
}

// ========== SetBudget 测试 ==========

func TestSetBudget(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	req := SetBudgetRequest{
		Name:              "月度用电预算",
		MonthlyAmount:     20000,
		ElectricityPrice:  0.56,
		WarningThreshold:  80,
		CriticalThreshold: 95,
	}

	budget, err := engine.SetBudget(req)
	assert.NoError(t, err)
	assert.NotNil(t, budget)
	assert.Equal(t, "月度用电预算", budget.Name)
	assert.Equal(t, 20000.0, budget.MonthlyAmount)
	assert.Equal(t, 0.56, budget.ElectricityPrice)
	assert.Equal(t, 80.0, budget.WarningThreshold)
	assert.Equal(t, 95.0, budget.CriticalThreshold)
	assert.True(t, budget.Enabled)
}

func TestSetBudgetDefaults(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	req := SetBudgetRequest{
		MonthlyAmount:    10000,
		ElectricityPrice: 50.0,
	}

	budget, err := engine.SetBudget(req)
	assert.NoError(t, err)
	assert.Equal(t, "用电预算", budget.Name)
	assert.Equal(t, DefaultWarningThreshold, budget.WarningThreshold)
	assert.Equal(t, DefaultCriticalThreshold, budget.CriticalThreshold)
}

func TestSetBudgetInvalidAmount(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	req := SetBudgetRequest{
		MonthlyAmount:    0,
		ElectricityPrice: 50.0,
	}

	_, err := engine.SetBudget(req)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidBudgetAmount, err)
}

func TestSetBudgetInvalidPrice(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	req := SetBudgetRequest{
		MonthlyAmount:    10000,
		ElectricityPrice: 0,
	}

	_, err := engine.SetBudget(req)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidElectricityPrice, err)
}

// ========== GetBudgetStatus 测试 ==========

func TestGetBudgetStatus(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.SetBudget(SetBudgetRequest{
		MonthlyAmount:    20000,
		ElectricityPrice: 0.56,
	})

	// 记录一些用电
	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-2", "设备2", 200.0)

	status, err := engine.GetBudgetStatus()
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.NotNil(t, status.Budget)
	assert.Greater(t, status.UsedEnergy, 0.0)
	assert.Greater(t, status.UsedCost, int64(0))
	assert.Greater(t, status.DaysElapsed, 0)
	assert.Greater(t, status.DaysRemaining, 0)
	assert.NotNil(t, status.Trend)
}

func TestGetBudgetStatusNotSet(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	_, err := engine.GetBudgetStatus()
	assert.Error(t, err)
	assert.Equal(t, ErrBudgetNotSet, err)
}

// ========== GetMonthlyReport 测试 ==========

func TestGetMonthlyReport(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.SetBudget(SetBudgetRequest{
		MonthlyAmount:    20000,
		ElectricityPrice: 0.56,
	})

	// 记录一些用电数据
	for i := 0; i < 5; i++ {
		testRecordPower(t, engine, "dev-"+string(rune('0'+i)), "设备"+string(rune('0'+i)), 100.0+float64(i)*50)
	}

	report, err := engine.GetMonthlyReport()
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, PeriodMonthly, report.Period)
	assert.Greater(t, report.TotalEnergy, 0.0)
	assert.Greater(t, report.TotalCost, int64(0))
	assert.NotNil(t, report.DailyTrend)
	assert.NotNil(t, report.TopDevices)
	assert.NotNil(t, report.Trend)
	assert.NotNil(t, report.Prediction)
}

func TestGetReportDaily(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)

	report, err := engine.GetReport(ReportRequest{Period: PeriodDaily})
	assert.NoError(t, err)
	assert.Equal(t, PeriodDaily, report.Period)
}

func TestGetReportWeekly(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)

	report, err := engine.GetReport(ReportRequest{Period: PeriodWeekly})
	assert.NoError(t, err)
	assert.Equal(t, PeriodWeekly, report.Period)
}

func TestGetReportWithDeviceFilter(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-2", "设备2", 200.0)

	report, err := engine.GetReport(ReportRequest{
		Period:   PeriodMonthly,
		DeviceID: "dev-1",
	})
	assert.NoError(t, err)
	// 只应该包含 dev-1 的数据
	for _, dp := range report.TopDevices {
		assert.Equal(t, "dev-1", dp.DeviceID)
	}
}

// ========== Tracker 测试 ==========

func TestTrackerGetRealtimePower(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-2", "设备2", 200.0)

	powers := engine.tracker.GetRealtimePower()
	assert.Len(t, powers, 2)
	assert.Equal(t, 100.0, powers["dev-1"])
	assert.Equal(t, 200.0, powers["dev-2"])
}

func TestTrackerGetCurrentPower(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-2", "设备2", 200.0)

	total := engine.tracker.GetCurrentPower()
	assert.InDelta(t, 300.0, total, 0.01)
}

func TestTrackerAggregateDaily(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)

	start := time.Now().AddDate(0, 0, -1)
	end := time.Now()

	daily := engine.tracker.AggregateDaily(start, end)
	assert.NotNil(t, daily)
	assert.Greater(t, len(daily), 0)
}

func TestTrackerAggregateHourly(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)

	hourly := engine.tracker.AggregateHourly(time.Now())
	assert.Len(t, hourly, 24)
}

func TestTrackerAggregateByDevice(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-2", "设备2", 200.0)
	testRecordPower(t, engine, "dev-1", "设备1", 150.0)

	devices := engine.tracker.AggregateByDevice(time.Now().Add(-1*time.Hour), time.Now())
	assert.Len(t, devices, 2)
}

func TestTrackerPeakAverageMinPower(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-1", "设备1", 300.0)
	testRecordPower(t, engine, "dev-1", "设备1", 200.0)

	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	peak, _ := engine.tracker.GetPeakPower(start, end)
	assert.Equal(t, 300.0, peak)

	avg := engine.tracker.GetAveragePower(start, end)
	assert.InDelta(t, 200.0, avg, 0.01)

	min := engine.tracker.GetMinPower(start, end)
	assert.Equal(t, 100.0, min)
}

// ========== Analyzer 测试 ==========

func TestAnalyzerCalculateTrend(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	// 记录稳定数据
	for i := 0; i < 10; i++ {
		testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	}

	trend := engine.analyzer.CalculateTrend(7)
	assert.Contains(t, []TrendDirection{TrendUp, TrendDown, TrendStable}, trend)
}

func TestAnalyzerAnalyzeDailyTrend(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)

	trend := engine.analyzer.AnalyzeDailyTrend(7)
	assert.NotNil(t, trend)
	assert.Greater(t, len(trend), 0)
}

func TestAnalyzerDetectAnomalies(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	// 记录正常数据
	for i := 0; i < 10; i++ {
		testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	}

	anomalies := engine.analyzer.DetectAnomalies(7)
	assert.NotNil(t, anomalies)
	// 正常数据不应有异常
	assert.Equal(t, 0, len(anomalies))
}

func TestAnalyzerDetectAnomaliesWithSpike(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	// 记录正常数据
	for i := 0; i < 20; i++ {
		testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	}

	// 记录异常数据
	req := RecordPowerRequest{
		DeviceID:    "dev-1",
		DeviceName:  "设备1",
		PowerWatts:  5000.0, // 50倍异常
		DurationSec: 3600,
	}
	engine.RecordPower(req)

	anomalies := engine.analyzer.DetectAnomalies(7)
	assert.NotNil(t, anomalies)
	// 应该检测到异常
	assert.Greater(t, len(anomalies), 0)
}

func TestAnalyzerGetOptimizationSuggestions(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.SetBudget(SetBudgetRequest{
		MonthlyAmount:    20000,
		ElectricityPrice: 0.56,
	})

	// 记录数据
	for i := 0; i < 10; i++ {
		testRecordPower(t, engine, "dev-1", "高耗电设备", 500.0)
	}

	suggestions := engine.analyzer.GetOptimizationSuggestions()
	assert.NotNil(t, suggestions)
}

func TestAnalyzerPredictMonthly(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.SetBudget(SetBudgetRequest{
		MonthlyAmount:    20000,
		ElectricityPrice: 0.56,
	})

	// 记录一些数据
	for i := 0; i < 5; i++ {
		testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	}

	prediction := engine.analyzer.PredictMonthly()
	assert.NotNil(t, prediction)
	assert.NotEmpty(t, prediction.Method)
	assert.Greater(t, prediction.DaysLeft, 0)
	assert.Greater(t, prediction.DailyAvg, 0.0)
	assert.Greater(t, prediction.PredictedKWh, 0.0)
	assert.NotNil(t, prediction.Confidence)
}

func TestAnalyzerPredictMonthlyNoBudget(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)

	prediction := engine.analyzer.PredictMonthly()
	assert.Nil(t, prediction)
}

func TestAnalyzerPredictDevice(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	for i := 0; i < 5; i++ {
		testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	}

	predicted := engine.analyzer.PredictDevicePredict("dev-1", 7)
	assert.Greater(t, predicted, 0.0)
}

func TestAnalyzerPredictDeviceNoData(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	predicted := engine.analyzer.PredictDevicePredict("nonexistent", 7)
	assert.Equal(t, 0.0, predicted)
}

// ========== AlertManager 测试 ==========

func TestAlertManagerCheckBudgetAlerts(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.SetBudget(SetBudgetRequest{
		MonthlyAmount:    100,
		ElectricityPrice: 0.56,
		WarningThreshold: 80,
	})

	// 记录大量用电，触发预警
	for i := 0; i < 10; i++ {
		testRecordPower(t, engine, "dev-1", "设备1", 1000.0)
	}

	engine.alertMgr.CheckBudgetAlerts()

	alerts := engine.GetActiveAlerts()
	assert.NotNil(t, alerts)
}

func TestAlertManagerGetAlertsByLevel(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	// 手动添加告警
	engine.mu.Lock()
	engine.alerts = append(engine.alerts, &Alert{
		ID:     "alert-1",
		Level:  AlertLevelWarning,
		Active: true,
	})
	engine.alerts = append(engine.alerts, &Alert{
		ID:     "alert-2",
		Level:  AlertLevelCritical,
		Active: true,
	})
	engine.mu.Unlock()

	warnings := engine.alertMgr.GetAlertsByLevel(AlertLevelWarning)
	assert.Len(t, warnings, 1)

	criticals := engine.alertMgr.GetAlertsByLevel(AlertLevelCritical)
	assert.Len(t, criticals, 1)
}

func TestAlertManagerGetAlertsByType(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.mu.Lock()
	engine.alerts = append(engine.alerts, &Alert{
		ID:   "alert-1",
		Type: AlertTypeBudgetWarning,
	})
	engine.alerts = append(engine.alerts, &Alert{
		ID:   "alert-2",
		Type: AlertTypeAnomalyPower,
	})
	engine.mu.Unlock()

	budgetAlerts := engine.alertMgr.GetAlertsByType(AlertTypeBudgetWarning)
	assert.Len(t, budgetAlerts, 1)

	anomalyAlerts := engine.alertMgr.GetAlertsByType(AlertTypeAnomalyPower)
	assert.Len(t, anomalyAlerts, 1)
}

func TestAlertManagerGetAlertStats(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.mu.Lock()
	engine.alerts = append(engine.alerts, &Alert{
		ID:     "alert-1",
		Type:   AlertTypeBudgetWarning,
		Level:  AlertLevelWarning,
		Active: true,
	})
	engine.alerts = append(engine.alerts, &Alert{
		ID:     "alert-2",
		Type:   AlertTypeAnomalyPower,
		Level:  AlertLevelCritical,
		Active: false,
	})
	engine.mu.Unlock()

	stats := engine.alertMgr.GetAlertStats()
	assert.Equal(t, 2, stats["total"])
	assert.Equal(t, 1, stats["active"])
	assert.Equal(t, 1, stats["resolved"])
	assert.Equal(t, 1, stats["warning"])
	assert.Equal(t, 1, stats["critical"])
}

func TestAlertManagerCooldown(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.alertMgr.SetCooldownMinutes(60)
	assert.Equal(t, 60, engine.alertMgr.cooldownMinutes)
}

func TestAcknowledgeAlert(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.mu.Lock()
	engine.alerts = append(engine.alerts, &Alert{
		ID:     "alert-to-ack",
		Active: true,
	})
	engine.mu.Unlock()

	err := engine.AcknowledgeAlert("alert-to-ack")
	assert.NoError(t, err)

	alerts := engine.GetActiveAlerts()
	assert.Len(t, alerts, 0)
}

func TestAcknowledgeAlertNotFound(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	err := engine.AcknowledgeAlert("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrRecordNotFound, err)
}

// ========== Device Profile 测试 ==========

func TestGetDeviceProfile(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-1", "设备1", 200.0)

	profile, err := engine.GetDeviceProfile("dev-1")
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "dev-1", profile.DeviceID)
	assert.Equal(t, "设备1", profile.DeviceName)
	assert.Equal(t, 2, profile.RecordCount)
	assert.Equal(t, 200.0, profile.PeakPower)
	assert.Greater(t, profile.TotalEnergy, 0.0)
	assert.Len(t, profile.HourlyProfile, 24)
}

func TestGetDeviceProfileNotFound(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	_, err := engine.GetDeviceProfile("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrDeviceNotFound, err)
}

func TestGetAllDeviceProfiles(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	testRecordPower(t, engine, "dev-1", "设备1", 100.0)
	testRecordPower(t, engine, "dev-2", "设备2", 200.0)
	testRecordPower(t, engine, "dev-3", "设备3", 300.0)

	profiles := engine.GetAllDeviceProfiles()
	assert.Len(t, profiles, 3)
	// 应该按总能耗降序排列
	assert.GreaterOrEqual(t, profiles[0].TotalEnergy, profiles[1].TotalEnergy)
	assert.GreaterOrEqual(t, profiles[1].TotalEnergy, profiles[2].TotalEnergy)
}

// ========== 类型测试 ==========

func TestAlertLevelConstants(t *testing.T) {
	levels := []AlertLevel{AlertLevelInfo, AlertLevelWarning, AlertLevelCritical, AlertLevelEmergency}
	for _, level := range levels {
		assert.NotEmpty(t, string(level))
	}
}

func TestAlertTypeConstants(t *testing.T) {
	types := []AlertType{AlertTypeBudgetExceeded, AlertTypeBudgetWarning, AlertTypeAnomalyPower, AlertTypeDeviceOverload}
	for _, at := range types {
		assert.NotEmpty(t, string(at))
	}
}

func TestReportPeriodConstants(t *testing.T) {
	periods := []ReportPeriod{PeriodDaily, PeriodWeekly, PeriodMonthly}
	for _, period := range periods {
		assert.NotEmpty(t, string(period))
	}
}

func TestTrendDirectionConstants(t *testing.T) {
	directions := []TrendDirection{TrendUp, TrendDown, TrendStable}
	for _, d := range directions {
		assert.NotEmpty(t, string(d))
	}
}

func TestDefaultBudgetConfig(t *testing.T) {
	config := DefaultBudgetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "默认用电预算", config.Name)
	assert.Equal(t, DefaultMonthlyBudget, config.MonthlyAmount)
	assert.Equal(t, DefaultElectricityPrice, config.ElectricityPrice)
	assert.Equal(t, DefaultWarningThreshold, config.WarningThreshold)
	assert.Equal(t, DefaultCriticalThreshold, config.CriticalThreshold)
	assert.True(t, config.Enabled)
}

func TestFormatCost(t *testing.T) {
	assert.Equal(t, "1.00", FormatCost(100))
	assert.Equal(t, "0.56", FormatCost(56))
	assert.Equal(t, "123.45", FormatCost(12345))
}

// ========== 辅助函数测试 ==========

func TestCalculateStats(t *testing.T) {
	data := []float64{100, 100, 100, 100, 100}
	mean, stddev := calculateStats(data)
	assert.Equal(t, 100.0, mean)
	assert.Equal(t, 0.0, stddev)

	data2 := []float64{10, 20, 30, 40, 50}
	mean2, stddev2 := calculateStats(data2)
	assert.Equal(t, 30.0, mean2)
	assert.Greater(t, stddev2, 0.0)
}

func TestCalculateStatsEmpty(t *testing.T) {
	mean, stddev := calculateStats([]float64{})
	assert.Equal(t, 0.0, mean)
	assert.Equal(t, 0.0, stddev)
}

func TestSortTrendPoints(t *testing.T) {
	points := []TrendPoint{
		{Date: time.Now(), Energy: 3},
		{Date: time.Now().AddDate(0, 0, -2), Energy: 1},
		{Date: time.Now().AddDate(0, 0, -1), Energy: 2},
	}

	sortTrendPoints(points)
	assert.True(t, points[0].Date.Before(points[1].Date))
	assert.True(t, points[1].Date.Before(points[2].Date))
}

// ========== 集成测试 ==========

func TestIntegrationBudgetWorkflow(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	// 1. 设置预算
	_, err := engine.SetBudget(SetBudgetRequest{
		Name:              "测试预算",
		MonthlyAmount:     10000,
		ElectricityPrice:  0.56,
		WarningThreshold:  80,
		CriticalThreshold: 95,
	})
	assert.NoError(t, err)

	// 2. 记录用电
	for i := 0; i < 10; i++ {
		_, err := engine.RecordPower(RecordPowerRequest{
			DeviceID:    "dev-1",
			DeviceName:  "NAS主机",
			PowerWatts:  100.0 + float64(i)*10,
			DurationSec: 3600,
		})
		assert.NoError(t, err)
	}

	// 3. 查看预算状态
	status, err := engine.GetBudgetStatus()
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Greater(t, status.UsedEnergy, 0.0)

	// 4. 查看月报
	report, err := engine.GetMonthlyReport()
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Greater(t, report.TotalEnergy, 0.0)

	// 5. 查看设备画像
	profile, err := engine.GetDeviceProfile("dev-1")
	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, 10, profile.RecordCount)

	// 6. 查看优化建议
	suggestions := engine.analyzer.GetOptimizationSuggestions()
	assert.NotNil(t, suggestions)

	// 7. 查看告警
	alerts := engine.GetActiveAlerts()
	assert.NotNil(t, alerts)
}

func TestIntegrationMultipleDevices(t *testing.T) {
	engine := newTestEngine(t)
	defer engine.Stop()

	engine.SetBudget(SetBudgetRequest{
		MonthlyAmount:    50000,
		ElectricityPrice: 0.56,
	})

	// 记录多个设备
	devices := []struct {
		id    string
		name  string
		power float64
	}{
		{"nas", "NAS主机", 150},
		{"router", "路由器", 15},
		{"switch", "交换机", 10},
		{"ups", "UPS", 5},
		{"cooling", "散热风扇", 20},
	}

	for _, d := range devices {
		for i := 0; i < 5; i++ {
			testRecordPower(t, engine, d.id, d.name, d.power)
		}
	}

	// 验证所有设备都被追踪
	profiles := engine.GetAllDeviceProfiles()
	assert.Len(t, profiles, len(devices))

	// 验证报告
	report, err := engine.GetMonthlyReport()
	assert.NoError(t, err)
	assert.Len(t, report.TopDevices, len(devices))
}
