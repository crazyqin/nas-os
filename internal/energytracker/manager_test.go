// Package energytracker 测试
package energytracker

import (
	"strconv"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}

	config := m.GetConfig()
	if config.CarbonFactor <= 0 {
		t.Error("carbon factor should be positive")
	}
	if config.PricePerKWhCents <= 0 {
		t.Error("price per kwh should be positive")
	}
}

func TestTrackUsage(t *testing.T) {
	m := NewManager()

	req := TrackRequest{
		DeviceID:   "nas-01",
		DeviceName: "主 NAS",
		PowerWatts: 65.5,
		Service:    "file-server",
	}

	reading, err := m.TrackUsage(req)
	if err != nil {
		t.Fatalf("track usage failed: %v", err)
	}
	if reading == nil {
		t.Fatal("reading should not be nil")
	}
	if reading.ID == "" {
		t.Error("reading should have an ID")
	}
	if reading.DeviceID != "nas-01" {
		t.Errorf("expected device_id nas-01, got %s", reading.DeviceID)
	}
	if reading.PowerWatts != 65.5 {
		t.Errorf("expected power 65.5, got %f", reading.PowerWatts)
	}
	if reading.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestTrackUsageValidation(t *testing.T) {
	m := NewManager()

	// 测试缺少 device_id
	_, err := m.TrackUsage(TrackRequest{
		DeviceName: "test",
		PowerWatts: 10,
	})
	if err == nil {
		t.Error("expected error for missing device_id")
	}

	// 测试缺少 device_name
	_, err = m.TrackUsage(TrackRequest{
		DeviceID:   "test",
		PowerWatts: 10,
	})
	if err == nil {
		t.Error("expected error for missing device_name")
	}

	// 测试负功率
	_, err = m.TrackUsage(TrackRequest{
		DeviceID:   "test",
		DeviceName: "test",
		PowerWatts: -10,
	})
	if err == nil {
		t.Error("expected error for negative power")
	}
}

func TestCalculateCarbon(t *testing.T) {
	m := NewManager()

	// 添加一些读数
	now := time.Now()
	start := now.Add(-2 * time.Hour)

	// 模拟2小时前的读数
	_, err := m.TrackUsage(TrackRequest{
		DeviceID:   "nas-01",
		DeviceName: "主 NAS",
		PowerWatts: 60,
	})
	if err != nil {
		t.Fatalf("track usage failed: %v", err)
	}

	// 手动添加历史读数
	m.mu.Lock()
	m.readings = append(m.readings, &EnergyReading{
		ID:         "test-1",
		DeviceID:   "nas-01",
		DeviceName: "主 NAS",
		PowerWatts: 60,
		Timestamp:  start,
	})
	m.readings = append(m.readings, &EnergyReading{
		ID:         "test-2",
		DeviceID:   "nas-01",
		DeviceName: "主 NAS",
		PowerWatts: 70,
		Timestamp:  now.Add(-1 * time.Hour),
	})
	m.mu.Unlock()

	footprint, err := m.CalculateCarbon("nas-01", start, now)
	if err != nil {
		t.Fatalf("calculate carbon failed: %v", err)
	}
	if footprint == nil {
		t.Fatal("footprint should not be nil")
	}
	if footprint.EnergyKWh <= 0 {
		t.Error("energy should be positive")
	}
	if footprint.CarbonKg <= 0 {
		t.Error("carbon should be positive")
	}
	if footprint.CarbonFactor <= 0 {
		t.Error("carbon factor should be positive")
	}
}

func TestCalculateCarbonNoReadings(t *testing.T) {
	m := NewManager()

	_, err := m.CalculateCarbon("nonexistent", time.Now().Add(-1*time.Hour), time.Now())
	if err == nil {
		t.Error("expected error for no readings")
	}
}

func TestGenerateReport(t *testing.T) {
	m := NewManager()

	// 添加读数
	for i := 0; i < 5; i++ {
		_, err := m.TrackUsage(TrackRequest{
			DeviceID:   "nas-01",
			DeviceName: "主 NAS",
			PowerWatts: float64(50 + i*10),
			Service:    "file-server",
		})
		if err != nil {
			t.Fatalf("track usage failed: %v", err)
		}
	}

	report, err := m.GenerateReport(ReportRequest{
		Period: PeriodDaily,
	})
	if err != nil {
		t.Fatalf("generate report failed: %v", err)
	}
	if report == nil {
		t.Fatal("report should not be nil")
	}
	if report.ID == "" {
		t.Error("report should have an ID")
	}
	if report.Period != PeriodDaily {
		t.Errorf("expected period daily, got %s", report.Period)
	}
	if report.TotalEnergyKWh < 0 {
		t.Error("total energy should be non-negative")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("generated_at should be set")
	}
}

func TestGenerateReportNoReadings(t *testing.T) {
	m := NewManager()

	_, err := m.GenerateReport(ReportRequest{
		Period: PeriodDaily,
	})
	if err == nil {
		t.Error("expected error for no readings")
	}
}

func TestGenerateReportInvalidPeriod(t *testing.T) {
	m := NewManager()

	_, err := m.GenerateReport(ReportRequest{
		Period: ReportPeriod("invalid"),
	})
	if err == nil {
		t.Error("expected error for invalid period")
	}
}

func TestSuggestOptimization(t *testing.T) {
	m := NewManager()

	// 添加读数
	for i := 0; i < 10; i++ {
		_, err := m.TrackUsage(TrackRequest{
			DeviceID:   "nas-01",
			DeviceName: "主 NAS",
			PowerWatts: float64(20 + i*10),
		})
		if err != nil {
			t.Fatalf("track usage failed: %v", err)
		}
	}

	tips, err := m.SuggestOptimization("nas-01")
	if err != nil {
		t.Fatalf("suggest optimization failed: %v", err)
	}
	if tips == nil {
		t.Fatal("tips should not be nil")
	}
	if len(tips) == 0 {
		t.Error("should have at least one optimization tip")
	}

	// 验证建议结构
	for _, tip := range tips {
		if tip.Category == "" {
			t.Error("tip should have a category")
		}
		if tip.Title == "" {
			t.Error("tip should have a title")
		}
		if tip.Description == "" {
			t.Error("tip should have a description")
		}
		if tip.Priority == "" {
			t.Error("tip should have a priority")
		}
	}
}

func TestSuggestOptimizationNoReadings(t *testing.T) {
	m := NewManager()

	_, err := m.SuggestOptimization("nonexistent")
	if err == nil {
		t.Error("expected error for no readings")
	}
}

func TestGetReadings(t *testing.T) {
	m := NewManager()

	// 添加读数
	for i := 0; i < 5; i++ {
		m.TrackUsage(TrackRequest{
			DeviceID:   "nas-01",
			DeviceName: "主 NAS",
			PowerWatts: float64(50 + i*10),
		})
	}

	// 获取全部
	readings := m.GetReadings("", 0)
	if len(readings) != 5 {
		t.Errorf("expected 5 readings, got %d", len(readings))
	}

	// 获取限制数量
	readings = m.GetReadings("", 3)
	if len(readings) != 3 {
		t.Errorf("expected 3 readings, got %d", len(readings))
	}

	// 按设备筛选
	readings = m.GetReadings("nas-01", 0)
	if len(readings) != 5 {
		t.Errorf("expected 5 readings for nas-01, got %d", len(readings))
	}

	// 不存在的设备
	readings = m.GetReadings("nonexistent", 0)
	if len(readings) != 0 {
		t.Errorf("expected 0 readings for nonexistent device, got %d", len(readings))
	}
}

func TestConfig(t *testing.T) {
	m := NewManager()

	// 获取默认配置
	config := m.GetConfig()
	if config.CarbonFactor != 0.5703 {
		t.Errorf("expected carbon factor 0.5703, got %f", config.CarbonFactor)
	}

	// 更新配置
	newConfig := PowerConfig{
		CarbonFactor:     0.6,
		PricePerKWhCents: 60,
		SamplingInterval: 30,
		IdleThreshold:    5,
	}
	m.UpdateConfig(newConfig)

	config = m.GetConfig()
	if config.CarbonFactor != 0.6 {
		t.Errorf("expected carbon factor 0.6, got %f", config.CarbonFactor)
	}
	if config.PricePerKWhCents != 60 {
		t.Errorf("expected price 60, got %d", config.PricePerKWhCents)
	}

	// 部分更新
	m.UpdateConfig(PowerConfig{CarbonFactor: 0.7})
	config = m.GetConfig()
	if config.CarbonFactor != 0.7 {
		t.Errorf("expected carbon factor 0.7, got %f", config.CarbonFactor)
	}
	if config.PricePerKWhCents != 60 {
		t.Errorf("expected price 60 after partial update, got %d", config.PricePerKWhCents)
	}
}

func TestClearReadings(t *testing.T) {
	m := NewManager()

	// 添加读数
	for i := 0; i < 3; i++ {
		m.TrackUsage(TrackRequest{
			DeviceID:   "nas-01",
			DeviceName: "主 NAS",
			PowerWatts: 50,
		})
	}

	readings := m.GetReadings("", 0)
	if len(readings) != 3 {
		t.Errorf("expected 3 readings before clear, got %d", len(readings))
	}

	m.ClearReadings()

	readings = m.GetReadings("", 0)
	if len(readings) != 0 {
		t.Errorf("expected 0 readings after clear, got %d", len(readings))
	}
}

func TestReportWithMultipleDevices(t *testing.T) {
	m := NewManager()

	// 添加多个设备的读数，时间间隔10分钟
	now := time.Now()
	devices := []struct {
		id   string
		name string
		power float64
	}{
		{"nas-01", "主 NAS", 65},
		{"nas-02", "备份 NAS", 45},
		{"switch-01", "交换机", 15},
	}

	for idx, d := range devices {
		for i := 0; i < 3; i++ {
			m.mu.Lock()
			m.readings = append(m.readings, &EnergyReading{
				ID:         "test-" + d.id + "-" + strconv.Itoa(i),
				DeviceID:   d.id,
				DeviceName: d.name,
				PowerWatts: d.power,
				Service:    "storage",
				Timestamp:  now.Add(-time.Duration((idx*3+i)*10) * time.Minute),
			})
			m.mu.Unlock()
		}
	}

	report, err := m.GenerateReport(ReportRequest{
		Period: PeriodDaily,
	})
	if err != nil {
		t.Fatalf("generate report failed: %v", err)
	}

	// 验证设备分解
	if len(report.DeviceBreakdown) != 3 {
		t.Errorf("expected 3 devices in breakdown, got %d", len(report.DeviceBreakdown))
	}

	// 验证百分比总和
	totalPercentage := 0.0
	for _, dev := range report.DeviceBreakdown {
		totalPercentage += dev.Percentage
	}
	if totalPercentage < 99.0 || totalPercentage > 101.0 {
		t.Errorf("total percentage should be ~100, got %f", totalPercentage)
	}
}

func TestOptimizationWithHighPower(t *testing.T) {
	m := NewManager()

	// 添加高功耗读数
	for i := 0; i < 10; i++ {
		m.TrackUsage(TrackRequest{
			DeviceID:   "nas-01",
			DeviceName: "主 NAS",
			PowerWatts: 150,
		})
	}

	tips, err := m.SuggestOptimization("nas-01")
	if err != nil {
		t.Fatalf("suggest optimization failed: %v", err)
	}

	// 应该有功耗上限建议
	found := false
	for _, tip := range tips {
		if tip.Category == "power_cap" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should suggest power cap for high power device")
	}
}

func TestOptimizationWithIdleDevice(t *testing.T) {
	m := NewManager()

	// 添加低功耗读数（低于空闲阈值）
	for i := 0; i < 20; i++ {
		m.TrackUsage(TrackRequest{
			DeviceID:   "nas-01",
			DeviceName: "主 NAS",
			PowerWatts: 5, // 低于默认阈值10W
		})
	}

	tips, err := m.SuggestOptimization("nas-01")
	if err != nil {
		t.Fatalf("suggest optimization failed: %v", err)
	}

	// 应该有硬盘休眠建议
	found := false
	for _, tip := range tips {
		if tip.Category == "power_management" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should suggest disk hibernation for idle device")
	}
}

func TestReportWithCustomTimeRange(t *testing.T) {
	m := NewManager()

	// 添加读数
	now := time.Now()
	start := now.Add(-3 * time.Hour)
	end := now.Add(-1 * time.Hour)

	m.mu.Lock()
	m.readings = append(m.readings, &EnergyReading{
		ID:         "test-1",
		DeviceID:   "nas-01",
		DeviceName: "主 NAS",
		PowerWatts: 60,
		Timestamp:  start.Add(30 * time.Minute),
	})
	m.readings = append(m.readings, &EnergyReading{
		ID:         "test-2",
		DeviceID:   "nas-01",
		DeviceName: "主 NAS",
		PowerWatts: 70,
		Timestamp:  end.Add(-30 * time.Minute),
	})
	m.mu.Unlock()

	report, err := m.GenerateReport(ReportRequest{
		Period:    PeriodDaily,
		StartTime: &start,
		EndTime:   &end,
	})
	if err != nil {
		t.Fatalf("generate report failed: %v", err)
	}

	if report.StartTime != start {
		t.Errorf("expected start time %v, got %v", start, report.StartTime)
	}
	if report.EndTime != end {
		t.Errorf("expected end time %v, got %v", end, report.EndTime)
	}
}
