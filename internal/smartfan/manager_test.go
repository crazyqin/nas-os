package smartfan

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.GetMode() != FanModeAuto {
		t.Errorf("expected auto mode, got '%s'", m.GetMode())
	}
}

func TestListZones(t *testing.T) {
	m := NewManager()

	zones := m.ListZones()
	if len(zones) < 3 {
		t.Errorf("expected at least 3 zones, got %d", len(zones))
	}
}

func TestGetZone(t *testing.T) {
	m := NewManager()

	zone, err := m.GetZone("cpu")
	if err != nil {
		t.Fatalf("get zone failed: %v", err)
	}
	if zone.Name != "CPU 散热" {
		t.Errorf("expected 'CPU 散热', got '%s'", zone.Name)
	}
	if !zone.Enabled {
		t.Error("expected enabled")
	}

	_, err = m.GetZone("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent zone")
	}
}

func TestListFans(t *testing.T) {
	m := NewManager()

	fans := m.ListFans()
	if len(fans) < 3 {
		t.Errorf("expected at least 3 fans, got %d", len(fans))
	}
}

func TestGetFan(t *testing.T) {
	m := NewManager()

	fan, err := m.GetFan("cpu-fan-1")
	if err != nil {
		t.Fatalf("get fan failed: %v", err)
	}
	if fan.Name != "CPU 风扇" {
		t.Errorf("expected 'CPU 风扇', got '%s'", fan.Name)
	}
	if !fan.IsHealthy {
		t.Error("expected healthy")
	}
}

func TestListSensors(t *testing.T) {
	m := NewManager()

	sensors := m.ListSensors()
	if len(sensors) < 3 {
		t.Errorf("expected at least 3 sensors, got %d", len(sensors))
	}
}

func TestGetSensor(t *testing.T) {
	m := NewManager()

	sensor, err := m.GetSensor("cpu")
	if err != nil {
		t.Fatalf("get sensor failed: %v", err)
	}
	if sensor.Name != "CPU 温度" {
		t.Errorf("expected 'CPU 温度', got '%s'", sensor.Name)
	}
}

func TestUpdateTemperature(t *testing.T) {
	m := NewManager()

	// 正常温度
	err := m.UpdateTemperature("cpu", 50.0)
	if err != nil {
		t.Fatalf("update temp failed: %v", err)
	}

	sensor, _ := m.GetSensor("cpu")
	if sensor.Temp != 50.0 {
		t.Errorf("expected 50.0, got %.1f", sensor.Temp)
	}

	// 不存在的传感器
	err = m.UpdateTemperature("nonexistent", 50.0)
	if err == nil {
		t.Error("expected error for nonexistent sensor")
	}
}

func TestHighTempAlert(t *testing.T) {
	m := NewManager()

	// 高温
	m.UpdateTemperature("cpu", 85.0)

	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Error("expected high temp alert")
	}
	if alerts[0].Severity != "warning" {
		t.Errorf("expected warning, got '%s'", alerts[0].Severity)
	}
}

func TestCriticalTempAlert(t *testing.T) {
	m := NewManager()

	m.UpdateTemperature("cpu", 96.0)

	alerts := m.GetAlerts(false)
	found := false
	for _, a := range alerts {
		if a.Severity == "critical" {
			found = true
		}
	}
	if !found {
		t.Error("expected critical alert")
	}
}

func TestSetMode(t *testing.T) {
	m := NewManager()

	err := m.SetMode(FanModeSilent)
	if err != nil {
		t.Fatalf("set mode failed: %v", err)
	}
	if m.GetMode() != FanModeSilent {
		t.Errorf("expected silent, got '%s'", m.GetMode())
	}

	err = m.SetMode(FanModePerformance)
	if err != nil {
		t.Fatalf("set mode failed: %v", err)
	}

	zone, _ := m.GetZone("cpu")
	if zone.Mode != FanModePerformance {
		t.Errorf("expected performance, got '%s'", zone.Mode)
	}
}

func TestUpdateZoneCurve(t *testing.T) {
	m := NewManager()

	curve := []CurvePoint{
		{Temp: 30, Duty: 10},
		{Temp: 50, Duty: 40},
		{Temp: 70, Duty: 70},
		{Temp: 90, Duty: 100},
	}

	err := m.UpdateZoneCurve("cpu", curve)
	if err != nil {
		t.Fatalf("update curve failed: %v", err)
	}

	zone, _ := m.GetZone("cpu")
	if zone.Mode != FanModeCustom {
		t.Errorf("expected custom, got '%s'", zone.Mode)
	}
	if len(zone.Curve) != 4 {
		t.Errorf("expected 4 points, got %d", len(zone.Curve))
	}
}

func TestResolveAlert(t *testing.T) {
	m := NewManager()

	m.UpdateTemperature("cpu", 85.0)
	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("expected alert")
	}

	err := m.ResolveAlert(alerts[0].ID)
	if err != nil {
		t.Fatalf("resolve alert failed: %v", err)
	}

	unresolved := m.GetAlerts(false)
	resolved := m.GetAlerts(true)
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}
	if len(resolved) == 0 {
		t.Error("expected resolved alerts")
	}
}

func TestStats(t *testing.T) {
	m := NewManager()

	m.UpdateTemperature("cpu", 55.0)
	m.UpdateTemperature("hdd", 42.0)

	stats := m.GetStats()
	if stats.AvgTemp == 0 {
		t.Error("expected non-zero avg temp")
	}
	if stats.MaxTemp == 0 {
		t.Error("expected non-zero max temp")
	}
	if stats.Uptime == 0 {
		t.Error("expected non-zero uptime")
	}
}
