package smartfancontrol

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.currentMode != FanModeBalanced {
		t.Errorf("expected default mode balanced, got %s", m.currentMode)
	}
	if m.maxAlerts != 100 {
		t.Errorf("expected maxAlerts 100, got %d", m.maxAlerts)
	}
}

func TestGetStatusReport(t *testing.T) {
	m := NewManager()
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	report := m.GetStatusReport()
	if report == nil {
		t.Fatal("expected report")
	}
	if report.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if report.CurrentMode != FanModeBalanced {
		t.Errorf("expected balanced mode, got %s", report.CurrentMode)
	}
}

func TestGetFanBeforeCollect(t *testing.T) {
	m := NewManager()

	_, err := m.GetFan("cpu-fan-1")
	if err == nil {
		t.Error("expected error before collect")
	}
}

func TestGetFan(t *testing.T) {
	m := NewManager()
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	fan, err := m.GetFan("cpu-fan-1")
	if err != nil {
		t.Fatalf("get fan failed: %v", err)
	}
	if fan.ID != "cpu-fan-1" {
		t.Errorf("expected cpu-fan-1, got %s", fan.ID)
	}
	if fan.MaxRPM == 0 {
		t.Error("expected non-zero max RPM")
	}
}

func TestGetTemperature(t *testing.T) {
	m := NewManager()
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	temp, err := m.GetTemperature("cpu-temp")
	if err != nil {
		t.Fatalf("get temperature failed: %v", err)
	}
	if temp.Temp == 0 {
		t.Error("expected non-zero temperature")
	}
}

func TestSetMode(t *testing.T) {
	m := NewManager()

	// 设置有效模式
	err := m.SetMode(FanModeSilent)
	if err != nil {
		t.Fatalf("set mode failed: %v", err)
	}
	if m.GetMode() != FanModeSilent {
		t.Errorf("expected silent mode, got %s", m.GetMode())
	}

	// 设置无效模式
	err = m.SetMode(FanMode("invalid"))
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestGetMode(t *testing.T) {
	m := NewManager()

	if m.GetMode() != FanModeBalanced {
		t.Errorf("expected balanced mode, got %s", m.GetMode())
	}

	m.SetMode(FanModePerformance)
	if m.GetMode() != FanModePerformance {
		t.Errorf("expected performance mode, got %s", m.GetMode())
	}
}

func TestSetFanCurve(t *testing.T) {
	m := NewManager()

	// 有效曲线
	curve := []FanCurvePoint{
		{Temp: 30, DutyCycle: 20},
		{Temp: 50, DutyCycle: 50},
		{Temp: 70, DutyCycle: 80},
	}
	err := m.SetFanCurve("custom", curve)
	if err != nil {
		t.Fatalf("set fan curve failed: %v", err)
	}

	profile, err := m.GetProfile("custom")
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}
	if len(profile.Curve) != 3 {
		t.Errorf("expected 3 curve points, got %d", len(profile.Curve))
	}

	// 空曲线
	err = m.SetFanCurve("empty", nil)
	if err == nil {
		t.Error("expected error for empty curve")
	}

	// 无效温度
	invalidCurve := []FanCurvePoint{
		{Temp: 150, DutyCycle: 50},
	}
	err = m.SetFanCurve("invalid", invalidCurve)
	if err == nil {
		t.Error("expected error for invalid temperature")
	}
}

func TestGetProfile(t *testing.T) {
	m := NewManager()

	profile, err := m.GetProfile("balanced")
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}
	if profile.Name != "均衡模式" {
		t.Errorf("expected 均衡模式, got %s", profile.Name)
	}

	_, err = m.GetProfile("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestListProfiles(t *testing.T) {
	m := NewManager()

	profiles := m.ListProfiles()
	if len(profiles) < 3 {
		t.Errorf("expected at least 3 profiles, got %d", len(profiles))
	}
}

func TestSetFanSpeed(t *testing.T) {
	m := NewManager()
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	// 有效转速
	err := m.SetFanSpeed("cpu-fan-1", 75.0)
	if err != nil {
		t.Fatalf("set fan speed failed: %v", err)
	}

	fan, _ := m.GetFan("cpu-fan-1")
	if fan.DutyCycle != 75.0 {
		t.Errorf("expected duty cycle 75.0, got %.1f", fan.DutyCycle)
	}

	// 无效转速
	err = m.SetFanSpeed("cpu-fan-1", 150.0)
	if err == nil {
		t.Error("expected error for invalid duty cycle")
	}

	// 不存在的风扇
	err = m.SetFanSpeed("nonexistent", 50.0)
	if err == nil {
		t.Error("expected error for nonexistent fan")
	}
}

func TestGetAlerts(t *testing.T) {
	m := NewManager()

	alerts := m.GetAlerts()
	if alerts == nil {
		t.Error("expected empty alerts, got nil")
	}
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestClearAlerts(t *testing.T) {
	m := NewManager()

	m.ClearAlerts()
	alerts := m.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()

	// 启动
	m.Start(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	report := m.GetStatusReport()
	if report == nil {
		t.Fatal("expected report")
	}

	// 停止
	m.Stop()

	// 重复停止不应 panic
	m.Stop()
}

func TestAddFan(t *testing.T) {
	m := NewManager()

	m.AddFan("test-fan", "测试风扇", 3000)
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	fan, err := m.GetFan("test-fan")
	if err != nil {
		t.Fatalf("get fan failed: %v", err)
	}
	if fan.Name != "测试风扇" {
		t.Errorf("expected 测试风扇, got %s", fan.Name)
	}
	if fan.MaxRPM != 3000 {
		t.Errorf("expected 3000 RPM, got %d", fan.MaxRPM)
	}
}

func TestAddTemperatureSensor(t *testing.T) {
	m := NewManager()

	m.AddTemperatureSensor("test-sensor", "测试传感器")
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	temp, err := m.GetTemperature("test-sensor")
	if err != nil {
		t.Fatalf("get temperature failed: %v", err)
	}
	if temp.Name != "测试传感器" {
		t.Errorf("expected 测试传感器, got %s", temp.Name)
	}
}

func TestUpdateTemperature(t *testing.T) {
	m := NewManager()
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	err := m.UpdateTemperature("cpu-temp", 75.5)
	if err != nil {
		t.Fatalf("update temperature failed: %v", err)
	}

	temp, _ := m.GetTemperature("cpu-temp")
	if temp.Temp != 75.5 {
		t.Errorf("expected 75.5, got %.1f", temp.Temp)
	}

	// 不存在的传感器
	err = m.UpdateTemperature("nonexistent", 50.0)
	if err == nil {
		t.Error("expected error for nonexistent sensor")
	}
}

func TestUpdateFanStatus(t *testing.T) {
	m := NewManager()
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	err := m.UpdateFanStatus("cpu-fan-1", FanStatusWarning)
	if err != nil {
		t.Fatalf("update fan status failed: %v", err)
	}

	fan, _ := m.GetFan("cpu-fan-1")
	if fan.Status != FanStatusWarning {
		t.Errorf("expected warning status, got %s", fan.Status)
	}

	// 不存在的风扇
	err = m.UpdateFanStatus("nonexistent", FanStatusOK)
	if err == nil {
		t.Error("expected error for nonexistent fan")
	}
}

func TestCalculateDutyCycle(t *testing.T) {
	m := NewManager()

	profile := &FanProfile{
		Curve: []FanCurvePoint{
			{Temp: 30, DutyCycle: 20},
			{Temp: 50, DutyCycle: 50},
			{Temp: 70, DutyCycle: 80},
		},
	}

	// 低于最低温度
	duty := m.calculateDutyCycle(25, profile)
	if duty != 20 {
		t.Errorf("expected 20, got %.1f", duty)
	}

	// 高于最高温度
	duty = m.calculateDutyCycle(80, profile)
	if duty != 80 {
		t.Errorf("expected 80, got %.1f", duty)
	}

	// 中间温度 (线性插值)
	duty = m.calculateDutyCycle(40, profile)
	expected := 35.0 // (20 + (50-20) * (40-30)/(50-30))
	if duty != expected {
		t.Errorf("expected %.1f, got %.1f", expected, duty)
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager()
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	report := m.GetReport()
	if report == nil {
		t.Fatal("expected report")
	}
	if report.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}
