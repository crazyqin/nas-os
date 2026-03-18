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

// ========== CleanupEnhancedManager 测试 ==========

func TestNewCleanupEnhancedManager(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
	if mgr.largeFileRules == nil {
		t.Error("largeFileRules should be initialized")
	}
	if mgr.expiredFileRules == nil {
		t.Error("expiredFileRules should be initialized")
	}
	if mgr.ruleSets == nil {
		t.Error("ruleSets should be initialized")
	}
}

// ========== 大文件规则测试 ==========

func TestCleanupEnhancedManager_CreateLargeFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	rule := LargeFileRule{
		Name:       "Test Rule",
		VolumeName: "volume1",
		MinSize:    1024 * 1024 * 100, // 100MB
		Enabled:    true,
	}

	created, err := mgr.CreateLargeFileRule(rule)
	if err != nil {
		t.Fatalf("CreateLargeFileRule failed: %v", err)
	}

	if created.ID == "" {
		t.Error("rule ID should be auto-generated")
	}
	if created.Name != "Test Rule" {
		t.Errorf("expected name 'Test Rule', got %s", created.Name)
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestCleanupEnhancedManager_GetLargeFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	rule := LargeFileRule{
		Name:    "Test Rule",
		MinSize: 1024,
		Enabled: true,
	}
	created, _ := mgr.CreateLargeFileRule(rule)

	// 获取存在的规则
	got, err := mgr.GetLargeFileRule(created.ID)
	if err != nil {
		t.Fatalf("GetLargeFileRule failed: %v", err)
	}
	if got.Name != "Test Rule" {
		t.Errorf("expected name 'Test Rule', got %s", got.Name)
	}

	// 获取不存在的规则
	_, err = mgr.GetLargeFileRule("non-existent")
	if err == nil {
		t.Error("should return error for non-existent rule")
	}
}

func TestCleanupEnhancedManager_ListLargeFileRules(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	// 创建多个规则
	mgr.CreateLargeFileRule(LargeFileRule{Name: "Rule 1", VolumeName: "vol1"})
	mgr.CreateLargeFileRule(LargeFileRule{Name: "Rule 2", VolumeName: "vol1"})
	mgr.CreateLargeFileRule(LargeFileRule{Name: "Rule 3", VolumeName: "vol2"})

	// 列出所有规则
	all := mgr.ListLargeFileRules("")
	if len(all) != 3 {
		t.Errorf("expected 3 rules, got %d", len(all))
	}

	// 按卷名过滤
	vol1 := mgr.ListLargeFileRules("vol1")
	if len(vol1) != 2 {
		t.Errorf("expected 2 rules for vol1, got %d", len(vol1))
	}
}

func TestCleanupEnhancedManager_UpdateLargeFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	created, _ := mgr.CreateLargeFileRule(LargeFileRule{
		Name:    "Original",
		MinSize: 1024,
		Enabled: true,
	})

	updated, err := mgr.UpdateLargeFileRule(created.ID, LargeFileRule{
		Name:    "Updated",
		MinSize: 2048,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("UpdateLargeFileRule failed: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", updated.Name)
	}
	if updated.MinSize != 2048 {
		t.Errorf("expected MinSize 2048, got %d", updated.MinSize)
	}
	if updated.CreatedAt.IsZero() {
		t.Error("CreatedAt should be preserved")
	}

	// 更新不存在的规则
	_, err = mgr.UpdateLargeFileRule("non-existent", LargeFileRule{})
	if err == nil {
		t.Error("should return error for non-existent rule")
	}
}

func TestCleanupEnhancedManager_DeleteLargeFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	created, _ := mgr.CreateLargeFileRule(LargeFileRule{
		Name:    "Test",
		Enabled: true,
	})

	// 删除存在的规则
	err := mgr.DeleteLargeFileRule(created.ID)
	if err != nil {
		t.Fatalf("DeleteLargeFileRule failed: %v", err)
	}

	// 确认删除
	_, err = mgr.GetLargeFileRule(created.ID)
	if err == nil {
		t.Error("rule should be deleted")
	}

	// 删除不存在的规则
	err = mgr.DeleteLargeFileRule("non-existent")
	if err == nil {
		t.Error("should return error for non-existent rule")
	}
}

// ========== 过期文件规则测试 ==========

func TestCleanupEnhancedManager_CreateExpiredFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	rule := ExpiredFileRule{
		Name:       "Test Expired Rule",
		VolumeName: "volume1",
		MaxAge:     30, // 30 days
		AccessType: "modification",
		Enabled:    true,
	}

	created, err := mgr.CreateExpiredFileRule(rule)
	if err != nil {
		t.Fatalf("CreateExpiredFileRule failed: %v", err)
	}

	if created.ID == "" {
		t.Error("rule ID should be auto-generated")
	}
	if created.MaxAge != 30 {
		t.Errorf("expected MaxAge 30, got %d", created.MaxAge)
	}
}

func TestCleanupEnhancedManager_GetExpiredFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	created, _ := mgr.CreateExpiredFileRule(ExpiredFileRule{
		Name:    "Test",
		MaxAge:  30,
		Enabled: true,
	})

	got, err := mgr.GetExpiredFileRule(created.ID)
	if err != nil {
		t.Fatalf("GetExpiredFileRule failed: %v", err)
	}
	if got.Name != "Test" {
		t.Errorf("expected name 'Test', got %s", got.Name)
	}

	_, err = mgr.GetExpiredFileRule("non-existent")
	if err == nil {
		t.Error("should return error for non-existent rule")
	}
}

func TestCleanupEnhancedManager_ListExpiredFileRules(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	mgr.CreateExpiredFileRule(ExpiredFileRule{Name: "Rule 1", VolumeName: "vol1"})
	mgr.CreateExpiredFileRule(ExpiredFileRule{Name: "Rule 2", VolumeName: "vol2"})

	all := mgr.ListExpiredFileRules("")
	if len(all) != 2 {
		t.Errorf("expected 2 rules, got %d", len(all))
	}

	vol1 := mgr.ListExpiredFileRules("vol1")
	if len(vol1) != 1 {
		t.Errorf("expected 1 rule for vol1, got %d", len(vol1))
	}
}

func TestCleanupEnhancedManager_UpdateExpiredFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	created, _ := mgr.CreateExpiredFileRule(ExpiredFileRule{
		Name:    "Original",
		MaxAge:  30,
		Enabled: true,
	})

	updated, err := mgr.UpdateExpiredFileRule(created.ID, ExpiredFileRule{
		Name:    "Updated",
		MaxAge:  60,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("UpdateExpiredFileRule failed: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", updated.Name)
	}
	if updated.MaxAge != 60 {
		t.Errorf("expected MaxAge 60, got %d", updated.MaxAge)
	}
}

func TestCleanupEnhancedManager_DeleteExpiredFileRule(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	created, _ := mgr.CreateExpiredFileRule(ExpiredFileRule{Name: "Test", Enabled: true})

	err := mgr.DeleteExpiredFileRule(created.ID)
	if err != nil {
		t.Fatalf("DeleteExpiredFileRule failed: %v", err)
	}

	_, err = mgr.GetExpiredFileRule(created.ID)
	if err == nil {
		t.Error("rule should be deleted")
	}
}

// ========== 规则集测试 ==========

func TestCleanupEnhancedManager_CreateRuleSet(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	ruleSet := CleanupRuleSet{
		Name:       "Test RuleSet",
		VolumeName: "volume1",
		Enabled:    true,
	}

	created, err := mgr.CreateRuleSet(ruleSet)
	if err != nil {
		t.Fatalf("CreateRuleSet failed: %v", err)
	}

	if created.ID == "" {
		t.Error("ruleSet ID should be auto-generated")
	}
}

func TestCleanupEnhancedManager_GetRuleSet(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	created, _ := mgr.CreateRuleSet(CleanupRuleSet{Name: "Test", Enabled: true})

	got, err := mgr.GetRuleSet(created.ID)
	if err != nil {
		t.Fatalf("GetRuleSet failed: %v", err)
	}
	if got.Name != "Test" {
		t.Errorf("expected name 'Test', got %s", got.Name)
	}

	_, err = mgr.GetRuleSet("non-existent")
	if err == nil {
		t.Error("should return error for non-existent ruleSet")
	}
}

func TestCleanupEnhancedManager_ListRuleSets(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	mgr.CreateRuleSet(CleanupRuleSet{Name: "Set 1", VolumeName: "vol1"})
	mgr.CreateRuleSet(CleanupRuleSet{Name: "Set 2", VolumeName: "vol2"})

	all := mgr.ListRuleSets("")
	if len(all) != 2 {
		t.Errorf("expected 2 ruleSets, got %d", len(all))
	}

	vol1 := mgr.ListRuleSets("vol1")
	if len(vol1) != 1 {
		t.Errorf("expected 1 ruleSet for vol1, got %d", len(vol1))
	}
}

func TestCleanupEnhancedManager_ExecuteRuleSet(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	// 创建规则集
	created, _ := mgr.CreateRuleSet(CleanupRuleSet{
		Name:    "Test Set",
		Enabled: true,
	})

	// 执行规则集
	result, err := mgr.ExecuteRuleSet(created.ID)
	if err != nil {
		t.Fatalf("ExecuteRuleSet failed: %v", err)
	}

	if result == nil {
		t.Error("result should not be nil")
	}

	// 执行不存在的规则集
	_, err = mgr.ExecuteRuleSet("non-existent")
	if err == nil {
		t.Error("should return error for non-existent ruleSet")
	}
}

// ========== 规则持久化测试 ==========

func TestCleanupEnhancedManager_SaveAndLoadRules(t *testing.T) {
	mgr := NewCleanupEnhancedManager(nil)

	// 创建一些规则
	mgr.CreateLargeFileRule(LargeFileRule{Name: "Large File Rule", Enabled: true})
	mgr.CreateExpiredFileRule(ExpiredFileRule{Name: "Expired File Rule", Enabled: true})
	mgr.CreateRuleSet(CleanupRuleSet{Name: "Rule Set", Enabled: true})

	// 保存规则
	err := mgr.SaveRules("/tmp/test-quota-rules.json")
	if err != nil {
		t.Fatalf("SaveRules failed: %v", err)
	}

	// 创建新管理器并加载规则
	mgr2 := NewCleanupEnhancedManager(nil)
	err = mgr2.LoadRules("/tmp/test-quota-rules.json")
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	// 验证加载结果
	if len(mgr2.ListLargeFileRules("")) != 1 {
		t.Error("should have loaded 1 large file rule")
	}
	if len(mgr2.ListExpiredFileRules("")) != 1 {
		t.Error("should have loaded 1 expired file rule")
	}
	if len(mgr2.ListRuleSets("")) != 1 {
		t.Error("should have loaded 1 ruleSet")
	}
}
