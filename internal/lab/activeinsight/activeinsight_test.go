package activeinsight

import (
	"testing"
	"time"
)

func TestNewActiveInsight(t *testing.T) {
	config := &Config{
		CollectInterval: 1 * time.Minute,
		RetentionDays:   30,
		MaxMetrics:      10000,
		AlertThresholds: map[string]float64{
			"cpu":         80.0,
			"memory":      90.0,
			"temperature": 70.0,
		},
		EnableAlerts: true,
	}

	ai := NewActiveInsight(config)
	if ai == nil {
		t.Fatal("NewActiveInsight returned nil")
	}

	if ai.config != config {
		t.Error("config not set correctly")
	}
}

func TestRegisterDevice(t *testing.T) {
	config := &Config{
		CollectInterval: 1 * time.Minute,
		RetentionDays:   30,
		MaxMetrics:      10000,
	}

	ai := NewActiveInsight(config)

	device := &Device{
		ID:       "device1",
		Name:     "Test Device",
		Type:     "nas",
		IP:       "192.168.1.100",
		Hostname: "test-nas",
		OS:       "Linux",
		Version:  "1.0.0",
	}

	// 注册设备
	err := ai.RegisterDevice(device)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	// 验证设备已注册
	devices := ai.GetDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}

	if devices[0].Name != "Test Device" {
		t.Errorf("expected device name 'Test Device', got '%s'", devices[0].Name)
	}
}

func TestRegisterDuplicateDevice(t *testing.T) {
	config := &Config{
		CollectInterval: 1 * time.Minute,
		RetentionDays:   30,
		MaxMetrics:      10000,
	}

	ai := NewActiveInsight(config)

	device := &Device{
		ID:   "device1",
		Name: "Test Device",
	}

	// 注册设备
	ai.RegisterDevice(device)

	// 尝试注册重复设备
	err := ai.RegisterDevice(device)
	if err == nil {
		t.Error("expected error when registering duplicate device")
	}
}

func TestRecordMetric(t *testing.T) {
	config := &Config{
		CollectInterval: 1 * time.Minute,
		RetentionDays:   30,
		MaxMetrics:      10000,
	}

	ai := NewActiveInsight(config)

	// 注册设备
	device := &Device{
		ID:   "device1",
		Name: "Test Device",
	}
	ai.RegisterDevice(device)

	// 记录指标
	metric := &Metric{
		Name:     "cpu_usage",
		Value:    75.5,
		Unit:     "%",
		DeviceID: "device1",
	}

	err := ai.RecordMetric(metric)
	if err != nil {
		t.Fatalf("RecordMetric failed: %v", err)
	}

	// 获取指标
	since := time.Now().Add(-1 * time.Hour)
	metrics, err := ai.GetMetrics("device1", "cpu_usage", since)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	if metrics[0].Value != 75.5 {
		t.Errorf("expected metric value 75.5, got %f", metrics[0].Value)
	}
}

func TestCreateAlert(t *testing.T) {
	config := &Config{
		CollectInterval: 1 * time.Minute,
		RetentionDays:   30,
		MaxMetrics:      10000,
		AlertThresholds: map[string]float64{
			"cpu":         80.0,
			"memory":      90.0,
			"temperature": 70.0,
		},
		EnableAlerts: true,
	}

	ai := NewActiveInsight(config)

	// 注册设备
	device := &Device{
		ID:   "device1",
		Name: "Test Device",
		Hardware: &HardwareInfo{
			CPUUsage:    85.0,
			MemoryTotal: 8 * 1024 * 1024 * 1024,
			MemoryUsed:  7 * 1024 * 1024 * 1024,
			Temperature: 65.0,
		},
	}
	ai.RegisterDevice(device)

	// 手动创建告警
	alert := &Alert{
		ID:         "alert1",
		DeviceID:   "device1",
		DeviceName: "Test Device",
		Type:       AlertTypeCPU,
		Severity:   AlertSeverityWarning,
		Title:      "CPU 使用率过高",
		Message:    "CPU 使用率 85.0% 超过阈值 80.0%",
		Value:      85.0,
		Threshold:  80.0,
		CreatedAt:  time.Now(),
	}

	ai.alerts = append(ai.alerts, alert)

	// 获取告警
	alerts := ai.GetAlerts("device1", AlertSeverityWarning, false)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	if alerts[0].Title != "CPU 使用率过高" {
		t.Errorf("expected alert title 'CPU 使用率过高', got '%s'", alerts[0].Title)
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{
		CollectInterval: 1 * time.Minute,
		RetentionDays:   30,
		MaxMetrics:      10000,
	}

	ai := NewActiveInsight(config)

	// 添加数据
	device := &Device{
		ID:     "device1",
		Name:   "Test Device",
		Status: DeviceStatusOnline,
	}
	ai.RegisterDevice(device)

	stats := ai.GetStats()

	if stats["devices"] != 1 {
		t.Errorf("expected 1 device, got %v", stats["devices"])
	}

	if stats["online_devices"] != 1 {
		t.Errorf("expected 1 online device, got %v", stats["online_devices"])
	}
}
