// thermalmanager_test.go - 智能温控管理单元测试
package thermalmanager

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return NewManager(logger)
}

func TestNewManager(t *testing.T) {
	manager := newTestManager(t)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.logger)
	assert.NotNil(t, manager.ctx)
	assert.NotNil(t, manager.cancel)
	assert.Equal(t, 1440, manager.maxHistory)
	assert.Equal(t, 100, manager.maxAlerts)
	assert.Equal(t, 30*time.Second, manager.checkInterval)
}

func TestClassifyTemp(t *testing.T) {
	manager := newTestManager(t)

	tests := []struct {
		name     string
		temp     float64
		expected ZoneStatus
	}{
		{"normal", 40, StatusNormal},
		{"warm", 65, StatusWarm},
		{"hot", 80, StatusHot},
		{"critical", 95, StatusCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.classifyTemp(tt.temp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInterpolateFanCurve(t *testing.T) {
	manager := newTestManager(t)

	tests := []struct {
		name     string
		temp     float64
		expected float64
	}{
		{"below_min", 20, 20},
		{"above_max", 90, 100},
		{"exact_point", 55, 50},
		{"between", 50, 42.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.interpolateFanCurve(tt.temp)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}

func TestClassifySensorType(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		expected SensorType
	}{
		{"cpu", "x86_pkg_temp", SensorCPU},
		{"cpu2", "CPU Thermal", SensorCPU},
		{"gpu", "nvidia_gpu", SensorGPU},
		{"hdd", "HDD Bay", SensorHDD},
		{"mb", "Motherboard", SensorMotherboard},
		{"ambient", "ambient_temp", SensorAmbient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifySensorType(tt.typeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProfileForMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     CoolingMode
		expected string
	}{
		{"silent", CoolingSilent, "silent"},
		{"balanced", CoolingBalanced, "balanced"},
		{"performance", CoolingPerformance, "performance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := getProfileForMode(tt.mode)
			assert.Equal(t, tt.expected, profile.Name)
			assert.Equal(t, tt.mode, profile.Mode)
			assert.NotEmpty(t, profile.Curves)
		})
	}
}

func TestLoadMockZones(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()

	assert.Len(t, manager.zones, 5)
	assert.Equal(t, "CPU", manager.zones[0].Name)
	assert.Equal(t, SensorCPU, manager.zones[0].Type)
}

func TestLoadMockFans(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockFans()

	assert.Len(t, manager.fans, 3)
	assert.Equal(t, "CPU Fan", manager.fans[0].Name)
}

func TestGetOverview(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()
	manager.loadMockFans()

	overview := manager.GetOverview()

	assert.NotNil(t, overview)
	assert.Contains(t, overview, "zones")
	assert.Contains(t, overview, "fans")
	assert.Contains(t, overview, "profile")
	assert.Contains(t, overview, "overallStatus")
	assert.Contains(t, overview, "updatedAt")
}

func TestGetZones(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()

	zones := manager.GetZones()
	assert.Len(t, zones, 5)
}

func TestGetFans(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockFans()

	fans := manager.GetFans()
	assert.Len(t, fans, 3)
}

func TestGetProfile(t *testing.T) {
	manager := newTestManager(t)

	profile := manager.GetProfile()
	assert.Equal(t, "balanced", profile.Name)
	assert.Equal(t, CoolingBalanced, profile.Mode)
}

func TestSetProfile(t *testing.T) {
	manager := newTestManager(t)

	newProfile := CoolingProfile{
		Name:       "custom",
		Mode:       CoolingSilent,
		WarmThresh: 55,
		HotThresh:  70,
		CritThresh: 85,
	}

	manager.SetProfile(newProfile)
	profile := manager.GetProfile()

	assert.Equal(t, "custom", profile.Name)
	assert.Equal(t, CoolingSilent, profile.Mode)
}

func TestGetAlerts(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()

	alerts := manager.GetAlerts(10)
	assert.NotNil(t, alerts)
	assert.Len(t, alerts, 0)
}

func TestClearAlerts(t *testing.T) {
	manager := newTestManager(t)
	manager.alerts = []ThermalAlert{
		{ID: "test", Zone: "CPU", Level: "hot"},
	}

	manager.ClearAlerts()
	assert.Len(t, manager.alerts, 0)
}

func TestGetHistory(t *testing.T) {
	manager := newTestManager(t)

	history := manager.GetHistory(60)
	assert.Empty(t, history)
}

func TestGetStats(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()

	stats := manager.GetStats("CPU", 60)
	assert.NotNil(t, stats)
	assert.Equal(t, manager.zones[0].Temperature, stats.Current)
}

func TestSetFanMode(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockFans()

	err := manager.SetFanMode("fan0", FanManual)
	assert.NoError(t, err)
	assert.Equal(t, FanManual, manager.fans[0].Mode)

	err = manager.SetFanMode("nonexistent", FanManual)
	assert.Error(t, err)
}

func TestSetFanPWM(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockFans()
	manager.fans[0].Mode = FanManual

	err := manager.SetFanPWM("fan0", 128)
	assert.NoError(t, err)
	assert.Equal(t, 128, manager.fans[0].PWMValue)

	err = manager.SetFanPWM("fan0", 300)
	assert.Error(t, err)

	err = manager.SetFanPWM("nonexistent", 128)
	assert.Error(t, err)
}

func TestSetCoolingMode(t *testing.T) {
	manager := newTestManager(t)

	manager.SetCoolingMode(CoolingSilent)
	assert.Equal(t, "silent", manager.profile.Name)

	manager.SetCoolingMode(CoolingPerformance)
	assert.Equal(t, "performance", manager.profile.Name)

	manager.SetCoolingMode(CoolingBalanced)
	assert.Equal(t, "balanced", manager.profile.Name)
}

func TestRecordHistory(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()
	manager.loadMockFans()

	manager.recordHistory()

	assert.Len(t, manager.history, 1)
	assert.NotEmpty(t, manager.history[0].Temperatures)
	assert.NotEmpty(t, manager.history[0].FanSpeeds)
}

func TestRecordHistoryMaxLimit(t *testing.T) {
	manager := newTestManager(t)
	manager.maxHistory = 5
	manager.loadMockZones()
	manager.loadMockFans()

	for i := 0; i < 10; i++ {
		manager.recordHistory()
	}

	assert.Len(t, manager.history, 5)
}

func TestUpdateFanSpeeds(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()
	manager.loadMockFans()

	manager.updateFanSpeeds()

	for _, fan := range manager.fans {
		assert.GreaterOrEqual(t, fan.Percent, 0.0)
		assert.LessOrEqual(t, fan.Percent, 100.0)
	}
}

func TestStartStop(t *testing.T) {
	manager := newTestManager(t)

	err := manager.Start()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	manager.Stop()
}

func TestCheckAlerts(t *testing.T) {
	manager := newTestManager(t)
	manager.zones = []TemperatureZone{
		{ID: "zone0", Name: "CPU", Temperature: 95, Status: StatusCritical},
		{ID: "zone1", Name: "GPU", Temperature: 80, Status: StatusHot},
	}

	manager.checkAlerts()

	assert.Len(t, manager.alerts, 2)
	assert.Equal(t, "critical", manager.alerts[0].Level)
	assert.Equal(t, "hot", manager.alerts[1].Level)
}

func TestCheckTemperatures(t *testing.T) {
	manager := newTestManager(t)
	manager.loadMockZones()
	manager.loadMockFans()

	manager.checkTemperatures()

	assert.NotEmpty(t, manager.history)
}
