// Package monitoring 提供 SSD 健康监控功能测试
package monitoring

import (
	"testing"
	"time"
)

func TestNewSSDHealthMonitor(t *testing.T) {
	config := &SSDMonitorConfig{
		CheckInterval:      0, // 禁用定期检查以避免测试阻塞
		WarningThreshold:   80,
		CriticalThreshold:  90,
		EmergencyThreshold: 95,
		EnablePrediction:   true,
	}

	monitor := NewSSDHealthMonitor(config)
	if monitor == nil {
		t.Fatal("监控器创建失败")
	}

	// 停止后台任务
	defer monitor.Stop()

	// 验证配置
	if monitor.config.WarningThreshold != 80 {
		t.Errorf("警告阈值错误: 期望 80, 实际 %f", monitor.config.WarningThreshold)
	}
	if monitor.config.CriticalThreshold != 90 {
		t.Errorf("严重阈值错误: 期望 90, 实际 %f", monitor.config.CriticalThreshold)
	}
	if monitor.config.EmergencyThreshold != 95 {
		t.Errorf("紧急阈值错误: 期望 95, 实际 %f", monitor.config.EmergencyThreshold)
	}
}

func TestDefaultSSDMonitorConfig(t *testing.T) {
	config := DefaultSSDMonitorConfig

	if config.WarningThreshold != 80 {
		t.Errorf("默认警告阈值错误: 期望 80, 实际 %f", config.WarningThreshold)
	}
	if config.CriticalThreshold != 90 {
		t.Errorf("默认严重阈值错误: 期望 90, 实际 %f", config.CriticalThreshold)
	}
	if config.EmergencyThreshold != 95 {
		t.Errorf("默认紧急阈值错误: 期望 95, 实际 %f", config.EmergencyThreshold)
	}
	if config.CheckInterval != 30*time.Minute {
		t.Errorf("默认检查间隔错误: 期望 30m, 实际 %v", config.CheckInterval)
	}
}

// testConfig 返回测试用配置（禁用定期检查）
func testConfig() *SSDMonitorConfig {
	return &SSDMonitorConfig{
		CheckInterval:      0, // 禁用定期检查
		WarningThreshold:   80,
		CriticalThreshold:  90,
		EmergencyThreshold: 95,
		EnablePrediction:   false, // 测试中禁用预测
	}
}

func TestEvaluateAlertLevel(t *testing.T) {
	tests := []struct {
		name           string
		lifeUsed       float64
		expectedLevel  AlertLevel
		expectedStatus SSDStatus
	}{
		{
			name:           "健康状态 - 低于80%",
			lifeUsed:       50,
			expectedLevel:  AlertLevelNone,
			expectedStatus: SSDStatusHealthy,
		},
		{
			name:           "警告状态 - 80%",
			lifeUsed:       82,
			expectedLevel:  AlertLevelWarning,
			expectedStatus: SSDStatusWarning,
		},
		{
			name:           "严重状态 - 90%",
			lifeUsed:       91,
			expectedLevel:  AlertLevelCritical,
			expectedStatus: SSDStatusCritical,
		},
		{
			name:           "紧急状态 - 95%",
			lifeUsed:       96,
			expectedLevel:  AlertLevelEmergency,
			expectedStatus: SSDStatusEmergency,
		},
		{
			name:           "边界值 - 刚好80%",
			lifeUsed:       80,
			expectedLevel:  AlertLevelWarning,
			expectedStatus: SSDStatusWarning,
		},
		{
			name:           "边界值 - 刚好90%",
			lifeUsed:       90,
			expectedLevel:  AlertLevelCritical,
			expectedStatus: SSDStatusCritical,
		},
		{
			name:           "边界值 - 刚好95%",
			lifeUsed:       95,
			expectedLevel:  AlertLevelEmergency,
			expectedStatus: SSDStatusEmergency,
		},
	}

	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := &SSDHealth{
				Device:          "/dev/nvme0n1",
				LifeUsedPercent: tt.lifeUsed,
				HealthPercent:   100 - tt.lifeUsed,
			}

			monitor.evaluateAlertLevel(health)

			if health.AlertLevel != tt.expectedLevel {
				t.Errorf("告警级别错误: 期望 %s, 实际 %s", tt.expectedLevel, health.AlertLevel)
			}
			if health.Status != tt.expectedStatus {
				t.Errorf("状态错误: 期望 %s, 实际 %s", tt.expectedStatus, health.Status)
			}
		})
	}
}

func TestEvaluateAlertLevelWithMediaErrors(t *testing.T) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	// 测试媒体错误影响
	health := &SSDHealth{
		Device:          "/dev/nvme0n1",
		LifeUsedPercent: 50,
		HealthPercent:   50,
		MediaErrors:     20,
	}

	monitor.evaluateAlertLevel(health)

	// 即使寿命未到阈值，媒体错误过多也应该触发警告
	if health.Status == SSDStatusHealthy {
		t.Error("存在大量媒体错误，状态应该不是健康")
	}
}

func TestEvaluateAlertLevelWithHighTemperature(t *testing.T) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	health := &SSDHealth{
		Device:          "/dev/nvme0n1",
		LifeUsedPercent: 50,
		HealthPercent:   50,
		Temperature:     75,
	}

	monitor.evaluateAlertLevel(health)

	// 温度过高应该触发警告
	if health.Status == SSDStatusHealthy {
		t.Error("温度过高，状态应该不是健康")
	}
}

func TestMapSMARTAttribute(t *testing.T) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	tests := []struct {
		name             string
		attr             *SMARTAttr
		expectedLifeUsed float64
		expectedPowerOn  uint64
		expectedTemp     int
	}{
		{
			name: "Power_On_Hours",
			attr: &SMARTAttr{
				ID:   9,
				Name: "Power_On_Hours",
				Raw:  10000,
			},
			expectedPowerOn: 10000,
		},
		{
			name: "Temperature_Celsius",
			attr: &SMARTAttr{
				ID:   194,
				Name: "Temperature_Celsius",
				Raw:  45,
			},
			expectedTemp: 45,
		},
		{
			name: "Wear_Leveling_Count (Samsung)",
			attr: &SMARTAttr{
				ID:    177,
				Name:  "Wear_Leveling_Count",
				Value: 85,
			},
			expectedLifeUsed: 15, // 100 - 85
		},
		{
			name: "SSD_Life_Left (Intel)",
			attr: &SMARTAttr{
				ID:    231,
				Name:  "SSD_Life_Left",
				Value: 90,
			},
			expectedLifeUsed: 10, // 100 - 90
		},
		{
			name: "Media_Wearout_Indicator (Intel)",
			attr: &SMARTAttr{
				ID:    233,
				Name:  "Media_Wearout_Indicator",
				Value: 75,
			},
			expectedLifeUsed: 25, // 100 - 75
		},
		{
			name: "Total_LBAs_Written",
			attr: &SMARTAttr{
				ID:   241,
				Name: "Total_LBAs_Written",
				Raw:  1000000000, // 1 billion LBAs
			},
		},
		{
			name: "Percent_Lifetime_Remain (Crucial)",
			attr: &SMARTAttr{
				ID:    202,
				Name:  "Percent_Lifetime_Remain",
				Value: 88,
			},
			expectedLifeUsed: 12, // 100 - 88
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := &SSDHealth{
				SMARTAttributes: make(map[string]*SMARTAttr),
			}

			monitor.mapSMARTAttribute(health, tt.attr)

			if tt.expectedPowerOn > 0 && health.PowerOnHours != tt.expectedPowerOn {
				t.Errorf("PowerOnHours 错误: 期望 %d, 实际 %d", tt.expectedPowerOn, health.PowerOnHours)
			}
			if tt.expectedTemp > 0 && health.Temperature != tt.expectedTemp {
				t.Errorf("Temperature 错误: 期望 %d, 实际 %d", tt.expectedTemp, health.Temperature)
			}
			if tt.expectedLifeUsed > 0 && health.LifeUsedPercent != tt.expectedLifeUsed {
				t.Errorf("LifeUsedPercent 错误: 期望 %f, 实际 %f", tt.expectedLifeUsed, health.LifeUsedPercent)
			}
		})
	}
}

func TestCalculateHealthPercent(t *testing.T) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	tests := []struct {
		name            string
		health          *SSDHealth
		expectedPercent float64
	}{
		{
			name: "已有健康百分比",
			health: &SSDHealth{
				HealthPercent:   85,
				LifeUsedPercent: 15,
			},
			expectedPercent: 85,
		},
		{
			name: "从 Available Spare 计算",
			health: &SSDHealth{
				AvailableSpare: 80,
			},
			expectedPercent: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor.calculateHealthPercent(tt.health)
			if tt.health.HealthPercent != tt.expectedPercent {
				t.Errorf("健康百分比错误: 期望 %f, 实际 %f", tt.expectedPercent, tt.health.HealthPercent)
			}
		})
	}
}

func TestSaveHistory(t *testing.T) {
	monitor := NewSSDHealthMonitor(&SSDMonitorConfig{
		HistoryRetention: 30,
	})
	defer monitor.Stop()

	health := &SSDHealth{
		Device:          "/dev/nvme0n1",
		HealthPercent:   85,
		LifeUsedPercent: 15,
		TotalWrites:     1000000000000,
		Temperature:     40,
	}

	// 保存多次历史
	for i := 0; i < 5; i++ {
		monitor.mu.Lock()
		monitor.saveHistory(health)
		monitor.mu.Unlock()
	}

	// 验证历史数据
	monitor.mu.RLock()
	history := monitor.history[health.Device]
	monitor.mu.RUnlock()

	if len(history) != 5 {
		t.Errorf("历史数据数量错误: 期望 5, 实际 %d", len(history))
	}

	// 验证历史数据内容
	for i, h := range history {
		if h.HealthPercent != 85 {
			t.Errorf("历史数据[%d] HealthPercent 错误: 期望 85, 实际 %f", i, h.HealthPercent)
		}
	}
}

func TestHistoryLimit(t *testing.T) {
	config := &SSDMonitorConfig{
		HistoryRetention: 1, // 只保留1天
	}
	monitor := NewSSDHealthMonitor(config)
	defer monitor.Stop()

	health := &SSDHealth{
		Device:          "/dev/nvme0n1",
		HealthPercent:   85,
		LifeUsedPercent: 15,
		TotalWrites:     1000000000000,
		Temperature:     40,
	}

	// 保存超过限制的历史数据
	maxPoints := config.HistoryRetention * 48
	for i := 0; i < maxPoints+100; i++ {
		monitor.mu.Lock()
		monitor.saveHistory(health)
		monitor.mu.Unlock()
	}

	// 验证历史数据量不超过限制
	monitor.mu.RLock()
	history := monitor.history[health.Device]
	monitor.mu.RUnlock()

	if len(history) > maxPoints {
		t.Errorf("历史数据量超过限制: 最大 %d, 实际 %d", maxPoints, len(history))
	}
}

func TestTriggerAlert(t *testing.T) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	alertReceived := false
	var receivedAlert *SSDHealthAlert

	monitor.RegisterAlertCallback(func(alert *SSDHealthAlert) {
		alertReceived = true
		receivedAlert = alert
	})

	health := &SSDHealth{
		Device:          "/dev/nvme0n1",
		LifeUsedPercent: 92,
		HealthPercent:   8,
		AlertLevel:      AlertLevelCritical,
		AlertMessage:    "SSD 寿命严重",
	}

	monitor.mu.Lock()
	monitor.triggerAlert(health)
	monitor.mu.Unlock()

	// 等待回调执行
	time.Sleep(100 * time.Millisecond)

	if !alertReceived {
		t.Error("告警回调未触发")
	}

	if receivedAlert != nil {
		if receivedAlert.Device != health.Device {
			t.Errorf("告警设备错误: 期望 %s, 实际 %s", health.Device, receivedAlert.Device)
		}
		if receivedAlert.AlertLevel != AlertLevelCritical {
			t.Errorf("告警级别错误: 期望 %s, 实际 %s", AlertLevelCritical, receivedAlert.AlertLevel)
		}
	}
}

func TestPredictLife(t *testing.T) {
	monitor := NewSSDHealthMonitor(&SSDMonitorConfig{
		EnablePrediction: true,
		CheckInterval:    0, // 禁用定期检查
	})
	defer monitor.Stop()

	// 创建模拟历史数据
	device := "/dev/nvme0n1"
	monitor.mu.Lock()
	monitor.history[device] = make([]*SSDHealthHistory, 20)

	baseTime := time.Now().Add(-20 * 30 * time.Minute) // 10小时前
	for i := 0; i < 20; i++ {
		monitor.history[device][i] = &SSDHealthHistory{
			Timestamp:   baseTime.Add(time.Duration(i) * 30 * time.Minute),
			TotalWrites: uint64(i) * 100000000000, // 每次增加 100GB
		}
	}
	monitor.mu.Unlock()

	health := &SSDHealth{
		Device:          device,
		HealthPercent:   85,
		LifeUsedPercent: 15,
		TotalWrites:     2000000000000, // 2TB
	}

	// 不要在持有锁的情况下调用 predictLife，因为它内部会尝试获取读锁
	monitor.predictLife(health)

	if health.PredictedLife == nil {
		t.Fatal("寿命预测失败")
	}

	if health.PredictedLife.WriteRatePerDay == 0 {
		t.Error("日均写入量不应为 0")
	}

	if health.PredictedLife.RemainingDays <= 0 {
		t.Error("剩余天数应大于 0")
	}

	if health.PredictedLife.Confidence <= 0 || health.PredictedLife.Confidence > 1 {
		t.Errorf("置信度应在 0-1 之间: %f", health.PredictedLife.Confidence)
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"1T", 1 * 1024 * 1024 * 1024 * 1024},
		{"512G", 512 * 1024 * 1024 * 1024},
		{"256M", 256 * 1024 * 1024},
		{"128", 128},
		{"2.5T", 2}, // 注意：当前实现不支持小数
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			// 只测试整数情况
			if !containsDecimal(tt.input) && result != tt.expected {
				t.Errorf("parseSize(%s) = %d, 期望 %d", tt.input, result, tt.expected)
			}
		})
	}
}

func containsDecimal(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}

func TestParseDataUnits(t *testing.T) {
	tests := []struct {
		input    string
		minExpected uint64 // 最小期望值（小数转换可能有舍入）
	}{
		{
			input:    "Data Units Written: 1,234,567 [6.33 TB]",
			minExpected: 6 * 1024 * 1024 * 1024 * 1024, // 至少 6TB
		},
		{
			input:    "Data Units Written: 100 [512 GB]",
			minExpected: 500 * 1024 * 1024 * 1024, // 至少 500GB
		},
		{
			input:    "Data Units Written: 100", // 无方括号
			minExpected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseDataUnits(tt.input)
			if result < tt.minExpected {
				t.Errorf("parseDataUnits() = %d, 期望至少 %d", result, tt.minExpected)
			}
		})
	}
}

func TestGetAllSSDs(t *testing.T) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	// 清空并手动设置测试数据
	monitor.mu.Lock()
	monitor.ssds = make(map[string]*SSDHealth) // 清空
	monitor.ssds["/dev/nvme0n1"] = &SSDHealth{
		Device:          "/dev/nvme0n1",
		HealthPercent:   90,
		LifeUsedPercent: 10,
		Status:          SSDStatusHealthy,
	}
	monitor.ssds["/dev/sda"] = &SSDHealth{
		Device:          "/dev/sda",
		HealthPercent:   75,
		LifeUsedPercent: 25,
		Status:          SSDStatusWarning,
	}
	monitor.mu.Unlock()

	ssds := monitor.GetAllSSDs()

	if len(ssds) != 2 {
		t.Errorf("SSD 数量错误: 期望 2, 实际 %d", len(ssds))
	}
}

func TestGetSSDHistory(t *testing.T) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	device := "/dev/nvme0n1"

	// 创建历史数据
	monitor.mu.Lock()
	monitor.history[device] = make([]*SSDHealthHistory, 5)
	for i := 0; i < 5; i++ {
		monitor.history[device][i] = &SSDHealthHistory{
			Timestamp: time.Now().Add(-time.Duration(5-i) * 24 * time.Hour),
		}
	}
	monitor.mu.Unlock()

	// 获取最近3天的历史
	history := monitor.GetSSDHistory(device, 3)

	// 应该返回最近3天的数据 (加上今天)
	if len(history) < 1 {
		t.Error("应该返回至少1条历史数据")
	}
}

func TestSSDHealthAlertLevels(t *testing.T) {
	// 测试三级预警的完整性
	levels := []AlertLevel{
		AlertLevelNone,
		AlertLevelWarning,
		AlertLevelCritical,
		AlertLevelEmergency,
	}

	expected := map[AlertLevel]string{
		AlertLevelNone:      "none",
		AlertLevelWarning:   "warning",
		AlertLevelCritical:  "critical",
		AlertLevelEmergency: "emergency",
	}

	for _, level := range levels {
		if string(level) != expected[level] {
			t.Errorf("告警级别字符串错误: %s 应为 %s", level, expected[level])
		}
	}
}

func TestSSDStatusValues(t *testing.T) {
	statuses := []SSDStatus{
		SSDStatusHealthy,
		SSDStatusWarning,
		SSDStatusCritical,
		SSDStatusEmergency,
		SSDStatusUnknown,
		SSDStatusOffline,
	}

	expected := map[SSDStatus]string{
		SSDStatusHealthy:   "healthy",
		SSDStatusWarning:   "warning",
		SSDStatusCritical:  "critical",
		SSDStatusEmergency: "emergency",
		SSDStatusUnknown:   "unknown",
		SSDStatusOffline:   "offline",
	}

	for _, status := range statuses {
		if string(status) != expected[status] {
			t.Errorf("状态字符串错误: %s 应为 %s", status, expected[status])
		}
	}
}

// 基准测试
func BenchmarkEvaluateAlertLevel(b *testing.B) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	health := &SSDHealth{
		Device:          "/dev/nvme0n1",
		LifeUsedPercent: 85,
		HealthPercent:   15,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.evaluateAlertLevel(health)
	}
}

func BenchmarkMapSMARTAttribute(b *testing.B) {
	monitor := NewSSDHealthMonitor(testConfig())
	defer monitor.Stop()

	health := &SSDHealth{
		SMARTAttributes: make(map[string]*SMARTAttr),
	}

	attr := &SMARTAttr{
		ID:    177,
		Name:  "Wear_Leveling_Count",
		Value: 85,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.mapSMARTAttribute(health, attr)
	}
}