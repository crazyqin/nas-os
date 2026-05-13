// thermal_test.go - 温控管理测试
package thermal

import (
	"testing"

	"go.uber.org/zap"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	m := NewManager(logger)
	m.loadMockZones()
	return m
}

func TestNewManager(t *testing.T) {
	m := newTestManager(t)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
}

func TestLoadMockZones(t *testing.T) {
	m := newTestManager(t)
	if len(m.zones) != 4 {
		t.Fatalf("期望 4 个温度区域，实际 %d", len(m.zones))
	}
	if len(m.fans) != 2 {
		t.Fatalf("期望 2 个风扇，实际 %d", len(m.fans))
	}
}

func TestGetOverview(t *testing.T) {
	m := newTestManager(t)
	overview := m.GetOverview()

	if overview.CPUTemp != 45 {
		t.Errorf("CPU温度: 期望 45, 实际 %.1f", overview.CPUTemp)
	}
	if overview.GPUTemp != 38 {
		t.Errorf("GPU温度: 期望 38, 实际 %.1f", overview.GPUTemp)
	}
	if overview.HottestZone != "CPU" {
		t.Errorf("最热区域: 期望 CPU, 实际 %s", overview.HottestZone)
	}
	if overview.OverallStatus != StatusNormal {
		t.Errorf("整体状态: 期望 normal, 实际 %s", overview.OverallStatus)
	}
	if len(overview.Zones) != 4 {
		t.Errorf("区域数: 期望 4, 实际 %d", len(overview.Zones))
	}
}

func TestClassifyTemp(t *testing.T) {
	m := newTestManager(t)

	tests := []struct {
		temp   float64
		expect ZoneStatus
	}{
		{30, StatusNormal},
		{59, StatusNormal},
		{60, StatusWarm},
		{74, StatusWarm},
		{75, StatusHot},
		{89, StatusHot},
		{90, StatusCritical},
		{100, StatusCritical},
	}

	for _, tt := range tests {
		got := m.classifyTemp(tt.temp)
		if got != tt.expect {
			t.Errorf("classifyTemp(%.0f): 期望 %s, 实际 %s", tt.temp, tt.expect, got)
		}
	}
}

func TestRefresh(t *testing.T) {
	m := newTestManager(t)
	m.Refresh()

	if len(m.history) != 1 {
		t.Fatalf("历史记录: 期望 1, 实际 %d", len(m.history))
	}
}

func TestRefreshAlerts(t *testing.T) {
	m := newTestManager(t)
	// 设置高温触发告警
	m.mu.Lock()
	for i, z := range m.zones {
		if z.Name == "CPU" {
			m.zones[i].Temp = 80
			m.zones[i].Status = StatusHot
		}
	}
	m.mu.Unlock()

	m.Refresh()

	alerts := m.GetAlerts(10)
	if len(alerts) == 0 {
		t.Error("高温应产生告警")
	}
}

func TestInterpolateFanCurve(t *testing.T) {
	m := newTestManager(t)

	tests := []struct {
		temp   float64
		expect float64
	}{
		{20, 20},   // 低于最低点
		{30, 20},   // 最低点
		{40, 30},   // 插值
		{50, 40},   // 精确点
		{65, 60},   // 精确点
		{85, 100},  // 最高点
		{95, 100},  // 高于最高点
	}

	for _, tt := range tests {
		got := m.interpolateFanCurve(tt.temp)
		if got != tt.expect {
			t.Errorf("interpolateFanCurve(%.0f): 期望 %.0f, 实际 %.0f", tt.temp, tt.expect, got)
		}
	}
}

func TestSetFanMode(t *testing.T) {
	m := newTestManager(t)

	err := m.SetFanMode("fan0", FanManual)
	if err != nil {
		t.Fatalf("SetFanMode 失败: %v", err)
	}

	if m.fans[0].Mode != FanManual {
		t.Errorf("风扇模式: 期望 manual, 实际 %s", m.fans[0].Mode)
	}

	// 不存在的风扇
	err = m.SetFanMode("fan999", FanAuto)
	if err != ErrFanNotFound {
		t.Errorf("不存在的风扇应返回 ErrFanNotFound, 实际 %v", err)
	}
}

func TestSetFanSpeed(t *testing.T) {
	m := newTestManager(t)

	// 先设置为手动模式
	m.SetFanMode("fan0", FanManual)

	err := m.SetFanSpeed("fan0", 75)
	if err != nil {
		t.Fatalf("SetFanSpeed 失败: %v", err)
	}
	if m.fans[0].Percent != 75 {
		t.Errorf("风扇百分比: 期望 75, 实际 %.1f", m.fans[0].Percent)
	}

	// 无效百分比
	err = m.SetFanSpeed("fan0", 150)
	if err == nil {
		t.Error("百分比 >100 应返回错误")
	}

	// 非手动模式
	m.SetFanMode("fan1", FanAuto)
	err = m.SetFanSpeed("fan1", 50)
	if err == nil {
		t.Error("非手动模式应返回错误")
	}
}

func TestGetHistory(t *testing.T) {
	m := newTestManager(t)
	m.Refresh()
	m.Refresh()

	history := m.GetHistory(60)
	if len(history) != 2 {
		t.Fatalf("历史记录: 期望 2, 实际 %d", len(history))
	}
}

func TestUpdatePolicy(t *testing.T) {
	m := newTestManager(t)

	policy := ThermalPolicy{
		Name:       "aggressive",
		WarmThresh: 50,
		HotThresh:  65,
		CritThresh: 80,
	}
	m.UpdatePolicy(policy)

	got := m.GetPolicy()
	if got.Name != "aggressive" {
		t.Errorf("策略名称: 期望 aggressive, 实际 %s", got.Name)
	}
	if got.WarmThresh != 50 {
		t.Errorf("WarmThresh: 期望 50, 实际 %.0f", got.WarmThresh)
	}
}

func TestClearAlerts(t *testing.T) {
	m := newTestManager(t)
	// 产生告警
	m.mu.Lock()
	m.zones[0].Temp = 100
	m.zones[0].Status = StatusCritical
	m.mu.Unlock()
	m.Refresh()

	if len(m.GetAlerts(10)) == 0 {
		t.Fatal("应有告警")
	}

	m.ClearAlerts()
	if len(m.GetAlerts(10)) != 0 {
		t.Error("清空后不应有告警")
	}
}

func TestSortZonesByTemp(t *testing.T) {
	zones := []ThermalZone{
		{Name: "A", Temp: 30},
		{Name: "B", Temp: 60},
		{Name: "C", Temp: 45},
	}
	sorted := SortZonesByTemp(zones)

	if sorted[0].Name != "B" {
		t.Errorf("第一个: 期望 B, 实际 %s", sorted[0].Name)
	}
	if sorted[1].Name != "C" {
		t.Errorf("第二个: 期望 C, 实际 %s", sorted[1].Name)
	}
	if sorted[2].Name != "A" {
		t.Errorf("第三个: 期望 A, 实际 %s", sorted[2].Name)
	}
}

func TestParseSensorsOutput(t *testing.T) {
	output := `coretemp-isa-0000
Package id 0:  +45.0°C  (high = +80.0°C, crit = +100.0°C)
Core 0:        +42.0°C  (high = +80.0°C, crit = +100.0°C)
Core 1:        +43.0°C  (high = +80.0°C, crit = +100.0°C)

acpitz-acpi-0
temp1:         +28.0°C  (crit = +100.0°C)
`
	zones := ParseSensorsOutput(output)
	if len(zones) != 4 {
		t.Fatalf("期望 4 个区域，实际 %d", len(zones))
	}
	if zones[0].Temp != 45.0 {
		t.Errorf("第一个温度: 期望 45, 实际 %.1f", zones[0].Temp)
	}
}
