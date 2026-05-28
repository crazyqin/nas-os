package energydashboard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.logger)
	assert.NotNil(t, manager.config)
	assert.NotNil(t, manager.currentPower)
	assert.NotNil(t, manager.budgets)
	assert.NotNil(t, manager.alerts)
	assert.NotNil(t, manager.devices)
}

func TestNewManagerWithNilConfig(t *testing.T) {
	logger := zap.NewNop()

	manager := NewManager(logger, nil)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.True(t, manager.config.Enabled)
	assert.Equal(t, "CN", manager.config.Region)
}

func TestGetCurrentPower(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)
	power := manager.GetCurrentPower()

	assert.NotNil(t, power)
	assert.False(t, power.Timestamp.IsZero())
}

func TestGetDashboardData(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)
	data := manager.GetDashboardData()

	assert.NotNil(t, data)
	assert.NotNil(t, data.CurrentPower)
	assert.NotNil(t, data.Efficiency)
	assert.NotNil(t, data.Carbon)
	assert.False(t, data.Timestamp.IsZero())
}

func TestSetBudget(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	req := &SetBudgetRequest{
		Name:           "Test Budget",
		DailyLimitWh:   1000,
		AlertThreshold: 80,
	}

	budget := manager.SetBudget(req)

	assert.NotNil(t, budget)
	assert.NotEmpty(t, budget.ID)
	assert.Equal(t, "Test Budget", budget.Name)
	assert.Equal(t, 1000.0, budget.DailyLimitWh)
	assert.Equal(t, 80.0, budget.AlertThreshold)
	assert.True(t, budget.Enabled)
}

func TestGetBudgets(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	req := &SetBudgetRequest{
		Name:         "Budget 1",
		DailyLimitWh: 1000,
	}
	manager.SetBudget(req)

	budgets := manager.GetBudgets()

	assert.Len(t, budgets, 1)
	assert.Equal(t, "Budget 1", budgets[0].Name)
}

func TestDeleteBudget(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	req := &SetBudgetRequest{
		Name:         "Test Budget",
		DailyLimitWh: 1000,
	}
	budget := manager.SetBudget(req)

	err := manager.DeleteBudget(budget.ID)
	assert.NoError(t, err)

	budgets := manager.GetBudgets()
	assert.Len(t, budgets, 0)
}

func TestDeleteBudgetNotFound(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	err := manager.DeleteBudget("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetRegionConfig(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	region, err := manager.GetRegionConfig("CN")
	assert.NoError(t, err)
	assert.NotNil(t, region)
	assert.Equal(t, "CN", region.Code)
	assert.Equal(t, "中国大陆", region.Name)
	assert.Equal(t, "CNY", region.Currency)
	assert.Equal(t, 0.55, region.RatePerKWh)
}

func TestGetRegionConfigNotFound(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	_, err := manager.GetRegionConfig("XX")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateRegionConfig(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	req := &UpdateRegionRequest{
		Code:       "JP",
		Name:       "Japan",
		Currency:   "JPY",
		RatePerKWh: 25.0,
	}

	manager.UpdateRegionConfig(req)

	region, err := manager.GetRegionConfig("JP")
	assert.NoError(t, err)
	assert.Equal(t, "JP", region.Code)
	assert.Equal(t, "Japan", region.Name)
	assert.Equal(t, "JPY", region.Currency)
	assert.Equal(t, 25.0, region.RatePerKWh)
}

func TestAddDevice(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	dev := &DeviceConfig{
		ID:         "test-device-1",
		Name:       "Test Device",
		DeviceType: DeviceTypeCPU,
		Enabled:    true,
		MaxWatts:   100,
	}

	manager.AddDevice(dev)

	devices := manager.GetDevices()
	assert.Len(t, devices, 1)
	assert.Equal(t, "test-device-1", devices[0].ID)
	assert.Equal(t, "Test Device", devices[0].Name)
}

func TestRemoveDevice(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	dev := &DeviceConfig{
		ID:         "test-device-1",
		Name:       "Test Device",
		DeviceType: DeviceTypeCPU,
		Enabled:    true,
	}
	manager.AddDevice(dev)

	manager.RemoveDevice("test-device-1")

	devices := manager.GetDevices()
	assert.Len(t, devices, 0)
}

func TestGetDevices(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	dev1 := &DeviceConfig{
		ID:         "dev-1",
		Name:       "Device 1",
		DeviceType: DeviceTypeCPU,
	}
	dev2 := &DeviceConfig{
		ID:         "dev-2",
		Name:       "Device 2",
		DeviceType: DeviceTypeDisk,
	}

	manager.AddDevice(dev1)
	manager.AddDevice(dev2)

	devices := manager.GetDevices()
	assert.Len(t, devices, 2)
}

func TestGetConfig(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	cfg := manager.GetConfig()
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "CN", cfg.Region)
}

func TestUpdateConfig(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	newConfig := DefaultEnergyDashboardConfig()
	newConfig.Region = "US"
	newConfig.MonitorInterval = 120

	manager.UpdateConfig(newConfig)

	cfg := manager.GetConfig()
	assert.Equal(t, "US", cfg.Region)
	assert.Equal(t, 120, cfg.MonitorInterval)
}

func TestAcknowledgeAlert(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()
	config.BudgetAlerts = true

	manager := NewManager(logger, config)

	// Create a budget that will trigger alert
	req := &SetBudgetRequest{
		Name:           "Test Budget",
		DailyLimitWh:   1,
		AlertThreshold: 0,
	}
	manager.SetBudget(req)

	// Get alerts
	data := manager.GetDashboardData()
	alerts := data.Alerts

	if len(alerts) > 0 {
		err := manager.AcknowledgeAlert(alerts[0].ID)
		assert.NoError(t, err)
	}
}

func TestAcknowledgeAlertNotFound(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	err := manager.AcknowledgeAlert("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultEnergyDashboardConfig(t *testing.T) {
	config := DefaultEnergyDashboardConfig()

	assert.NotNil(t, config)
	assert.True(t, config.Enabled)
	assert.Equal(t, "CN", config.Region)
	assert.Equal(t, "CNY", config.DefaultCurrency)
	assert.Equal(t, 60, config.MonitorInterval)
	assert.Equal(t, 365, config.RetentionDays)
	assert.True(t, config.PUEEnabled)
	assert.True(t, config.CarbonTracking)
	assert.True(t, config.BudgetAlerts)
	assert.Equal(t, "0 0 * * *", config.ReportSchedule)
	assert.Len(t, config.Regions, 2)
	assert.Contains(t, config.Regions, "CN")
	assert.Contains(t, config.Regions, "US")
}

func TestGenerateReport(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEnergyDashboardConfig()

	manager := NewManager(logger, config)

	startDate := time.Now().AddDate(0, 0, -7)
	endDate := time.Now()

	report, err := manager.GenerateReport(nil, ReportTypeWeekly, startDate, endDate)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, ReportTypeWeekly, report.ReportType)
	assert.False(t, report.GeneratedAt.IsZero())
}

func TestRegionConfig(t *testing.T) {
	config := DefaultEnergyDashboardConfig()

	cn := config.Regions["CN"]
	assert.Equal(t, "CN", cn.Code)
	assert.Equal(t, "中国大陆", cn.Name)
	assert.Equal(t, "CNY", cn.Currency)
	assert.Equal(t, 0.55, cn.RatePerKWh)
	assert.Equal(t, 0.35, cn.OffPeakRate)
	assert.Equal(t, 0.55, cn.PeakRate)
	assert.Equal(t, 0.85, cn.SuperPeakRate)
	assert.Equal(t, 0.13, cn.TaxRate)

	us := config.Regions["US"]
	assert.Equal(t, "US", us.Code)
	assert.Equal(t, "United States", us.Name)
	assert.Equal(t, "USD", us.Currency)
	assert.Equal(t, 0.12, us.RatePerKWh)
}

func TestDeviceTypes(t *testing.T) {
	assert.Equal(t, DeviceType("cpu"), DeviceTypeCPU)
	assert.Equal(t, DeviceType("disk"), DeviceTypeDisk)
	assert.Equal(t, DeviceType("network"), DeviceTypeNetwork)
	assert.Equal(t, DeviceType("gpu"), DeviceTypeGPU)
	assert.Equal(t, DeviceType("memory"), DeviceTypeMemory)
	assert.Equal(t, DeviceType("fan"), DeviceTypeFan)
	assert.Equal(t, DeviceType("psu"), DeviceTypePSU)
	assert.Equal(t, DeviceType("other"), DeviceTypeOther)
}

func TestReportTypes(t *testing.T) {
	assert.Equal(t, ReportType("daily"), ReportTypeDaily)
	assert.Equal(t, ReportType("weekly"), ReportTypeWeekly)
	assert.Equal(t, ReportType("monthly"), ReportTypeMonthly)
}

func TestAlertLevels(t *testing.T) {
	assert.Equal(t, AlertLevel("info"), AlertLevelInfo)
	assert.Equal(t, AlertLevel("warning"), AlertLevelWarning)
	assert.Equal(t, AlertLevel("critical"), AlertLevelCritical)
}

func TestEnergyUnits(t *testing.T) {
	assert.Equal(t, EnergyUnit("Wh"), EnergyUnitWh)
	assert.Equal(t, EnergyUnit("kWh"), EnergyUnitKWh)
	assert.Equal(t, EnergyUnit("MWh"), EnergyUnitMWh)
}