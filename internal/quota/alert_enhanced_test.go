package quota

import (
	"testing"
	"time"
)

// ========== AlertThresholdConfig 测试 ==========

func TestDefaultAlertThresholdConfig(t *testing.T) {
	config := DefaultAlertThresholdConfig()

	if config.WarningThreshold != 70 {
		t.Errorf("expected WarningThreshold 70, got %f", config.WarningThreshold)
	}
	if config.CriticalThreshold != 85 {
		t.Errorf("expected CriticalThreshold 85, got %f", config.CriticalThreshold)
	}
	if config.EmergencyThreshold != 95 {
		t.Errorf("expected EmergencyThreshold 95, got %f", config.EmergencyThreshold)
	}
	if config.CooldownDuration != 30*time.Minute {
		t.Errorf("expected CooldownDuration 30m, got %v", config.CooldownDuration)
	}
	if !config.RepeatAlert {
		t.Error("expected RepeatAlert true")
	}
	if config.MaxRepeatCount != 3 {
		t.Errorf("expected MaxRepeatCount 3, got %d", config.MaxRepeatCount)
	}
}

// ========== AlertNotificationManager 测试 ==========

func TestNewAlertNotificationManager(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
	if len(mgr.channels) != 0 {
		t.Errorf("expected empty channels, got %d", len(mgr.channels))
	}
}

func TestAlertNotificationManager_AddChannel(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	channel := &NotificationChannel{
		ID:      "test-channel",
		Name:    "Test Channel",
		Type:    "email",
		Enabled: true,
	}

	mgr.AddChannel(channel)

	channels := mgr.GetChannels()
	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].ID != "test-channel" {
		t.Errorf("expected channel ID test-channel, got %s", channels[0].ID)
	}
}

func TestAlertNotificationManager_RemoveChannel(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	channel := &NotificationChannel{
		ID:      "test-channel",
		Name:    "Test Channel",
		Type:    "email",
		Enabled: true,
	}

	mgr.AddChannel(channel)
	mgr.RemoveChannel("test-channel")

	channels := mgr.GetChannels()
	if len(channels) != 0 {
		t.Errorf("expected 0 channels after removal, got %d", len(channels))
	}
}

func TestAlertNotificationManager_GetChannels(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	// Empty channels
	channels := mgr.GetChannels()
	if len(channels) != 0 {
		t.Errorf("expected empty channels, got %d", len(channels))
	}

	// Add multiple channels
	mgr.AddChannel(&NotificationChannel{ID: "ch1", Type: "email"})
	mgr.AddChannel(&NotificationChannel{ID: "ch2", Type: "slack"})

	channels = mgr.GetChannels()
	if len(channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(channels))
	}
}

func TestAlertNotificationManager_ShouldAlert_FirstAlert(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	// 第一次告警应该允许
	if !mgr.ShouldAlert("quota-1", AlertSeverityWarning) {
		t.Error("first alert should be allowed")
	}
}

func TestAlertNotificationManager_ShouldAlert_Cooldown(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	config.CooldownDuration = 100 * time.Millisecond
	mgr := NewAlertNotificationManager(config)

	// 第一次告警
	mgr.RecordAlert("quota-1", AlertSeverityWarning)

	// 冷却期内不应该告警
	if mgr.ShouldAlert("quota-1", AlertSeverityWarning) {
		t.Error("should not alert during cooldown")
	}

	// 等待冷却期结束
	time.Sleep(150 * time.Millisecond)

	// 冷却期后应该允许告警
	if !mgr.ShouldAlert("quota-1", AlertSeverityWarning) {
		t.Error("should alert after cooldown")
	}
}

func TestAlertNotificationManager_ShouldAlert_MaxRepeat(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	config.CooldownDuration = 1 * time.Millisecond
	config.MaxRepeatCount = 3
	mgr := NewAlertNotificationManager(config)

	// 第一次告警
	mgr.RecordAlert("quota-1", AlertSeverityWarning)
	time.Sleep(2 * time.Millisecond)

	// 第二次告警 - count=1 < 3，允许
	if !mgr.ShouldAlert("quota-1", AlertSeverityWarning) {
		t.Error("second alert should be allowed (count=1 < max)")
	}
	mgr.RecordAlert("quota-1", AlertSeverityWarning)
	time.Sleep(2 * time.Millisecond)

	// 第三次告警 - count=2 < 3，允许
	if !mgr.ShouldAlert("quota-1", AlertSeverityWarning) {
		t.Error("third alert should be allowed (count=2 < max)")
	}
	mgr.RecordAlert("quota-1", AlertSeverityWarning)
	time.Sleep(2 * time.Millisecond)

	// 第四次告警 - count=3 >= 3，不允许
	if mgr.ShouldAlert("quota-1", AlertSeverityWarning) {
		t.Error("fourth alert should not be allowed (count=3 >= max)")
	}
}

func TestAlertNotificationManager_RecordAlert(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	mgr.RecordAlert("quota-1", AlertSeverityWarning)

	// 检查冷却追踪器
	mgr.mu.RLock()
	_, exists := mgr.cooldownTracker["quota-1"]
	count := mgr.repeatTracker["quota-1"]
	mgr.mu.RUnlock()

	if !exists {
		t.Error("cooldown tracker should have quota-1")
	}
	if count != 1 {
		t.Errorf("expected repeat count 1, got %d", count)
	}
}

func TestAlertNotificationManager_ResetRepeatCounter(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	mgr.RecordAlert("quota-1", AlertSeverityWarning)
	mgr.RecordAlert("quota-1", AlertSeverityWarning)

	mgr.ResetRepeatCounter("quota-1")

	mgr.mu.RLock()
	count := mgr.repeatTracker["quota-1"]
	mgr.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected repeat count 0 after reset, got %d", count)
	}
}

func TestAlertNotificationManager_CleanupCooldown(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	// 添加一些冷却记录
	mgr.RecordAlert("quota-1", AlertSeverityWarning)
	mgr.RecordAlert("quota-2", AlertSeverityWarning)

	// 清理过期记录（默认清理24小时前的记录）
	mgr.CleanupCooldown()

	mgr.mu.RLock()
	// 当前记录不应该被清理（因为刚添加）
	count := len(mgr.cooldownTracker)
	mgr.mu.RUnlock()

	if count != 2 {
		t.Errorf("expected 2 cooldown entries (recent), got %d", count)
	}
}

// ========== DetermineSeverity 测试 ==========

func TestAlertNotificationManager_DetermineSeverity(t *testing.T) {
	config := DefaultAlertThresholdConfig()
	mgr := NewAlertNotificationManager(config)

	tests := []struct {
		percent  float64
		expected AlertSeverity
	}{
		{50, AlertSeverityInfo},
		{70, AlertSeverityWarning},
		{85, AlertSeverityCritical},
		{95, AlertSeverityEmergency},
		{100, AlertSeverityEmergency},
	}

	for _, tt := range tests {
		result := mgr.DetermineSeverity(tt.percent)
		if result != tt.expected {
			t.Errorf("DetermineSeverity(%f) = %v, want %v", tt.percent, result, tt.expected)
		}
	}
}
