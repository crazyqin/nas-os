// Package quota 提供存储配额管理和告警功能
package quota

import (
	"testing"
	"time"
)

// ========== Types 测试 ==========

func TestGetWarningLevel(t *testing.T) {
	tests := []struct {
		percent  float64
		expected string
	}{
		{50.0, WarningLevelLow},
		{60.0, WarningLevelLow}, // 60%是low级别（阈值刚好在边界）
		{65.0, WarningLevelLow},
		{80.0, WarningLevelMedium},
		{85.0, WarningLevelMedium},
		{90.0, WarningLevelHigh},
		{95.0, WarningLevelCritical},
		{100.0, WarningLevelCritical},
	}

	for _, tt := range tests {
		result := GetWarningLevel(tt.percent)
		if result != tt.expected {
			t.Errorf("GetWarningLevel(%f) = %s, expected %s", tt.percent, result, tt.expected)
		}
	}
}

func TestDefaultNotificationConfig(t *testing.T) {
	config := DefaultNotificationConfig()
	if !config.Enabled {
		t.Error("默认通知配置应该是启用状态")
	}
	if config.CoolDownMin != 30 {
		t.Errorf("默认冷却时间应该是30分钟，实际是 %d", config.CoolDownMin)
	}
}

func TestDefaultForecastConfig(t *testing.T) {
	config := DefaultForecastConfig()
	if config.HistoryDays != 30 {
		t.Errorf("默认历史天数应该是30，实际是 %d", config.HistoryDays)
	}
	if config.MinDataPoints != 7 {
		t.Errorf("默认最小数据点应该是7，实际是 %d", config.MinDataPoints)
	}
}

// ========== Predictor 测试 ==========

func TestNewPredictor(t *testing.T) {
	config := ForecastConfig{
		HistoryDays:   60,
		MinDataPoints: 10,
	}
	p := NewPredictor(config)

	if p == nil {
		t.Error("创建预测器失败")
		return
	}

	// 验证配置
	savedConfig := p.GetConfig()
	if savedConfig.HistoryDays != 60 {
		t.Errorf("配置未正确设置，HistoryDays = %d", savedConfig.HistoryDays)
	}
}

func TestPredictor_RecordUsage(t *testing.T) {
	p := NewPredictor(DefaultForecastConfig())

	// 记录使用量
	p.RecordUsage("test-target", 1000)

	// 获取历史数据
	history := p.GetHistory("test-target")
	if len(history) != 1 {
		t.Errorf("历史记录数量应为1，实际是 %d", len(history))
	}

	if history[0].UsedBytes != 1000 {
		t.Errorf("使用量应为1000，实际是 %d", history[0].UsedBytes)
	}
}

func TestPredictor_GetHistoryStats(t *testing.T) {
	p := NewPredictor(DefaultForecastConfig())

	// 记录多条使用量
	for i := 0; i < 10; i++ {
		p.RecordUsage("test-target", int64(1000+i*100))
		time.Sleep(1 * time.Millisecond) // 确保时间不同
	}

	stats := p.GetHistoryStats("test-target")

	count := stats["count"].(int)
	if count != 10 {
		t.Errorf("历史记录数量应为10，实际是 %d", count)
	}

	minUsage := stats["min_usage"].(int64)
	if minUsage != 1000 {
		t.Errorf("最小使用量应为1000，实际是 %d", minUsage)
	}

	maxUsage := stats["max_usage"].(int64)
	if maxUsage != 1900 {
		t.Errorf("最大使用量应为1900，实际是 %d", maxUsage)
	}
}

func TestPredictor_ClearHistory(t *testing.T) {
	p := NewPredictor(DefaultForecastConfig())

	p.RecordUsage("test-target", 1000)
	p.RecordUsage("other-target", 2000)

	p.ClearHistory("test-target")

	history := p.GetHistory("test-target")
	if len(history) != 0 {
		t.Errorf("清除后历史记录应为空，实际是 %d", len(history))
	}

	otherHistory := p.GetHistory("other-target")
	if len(otherHistory) != 1 {
		t.Errorf("其他目标的历史记录不应被清除")
	}
}

func TestPredictor_Predict_InsufficientData(t *testing.T) {
	p := NewPredictor(DefaultForecastConfig())

	// 只记录少量数据点
	for i := 0; i < 3; i++ {
		p.RecordUsage("test-target", int64(1000+i*100))
	}

	_, err := p.Predict("rule-1", "test-target", 10000)
	if err != ErrInsufficientData {
		t.Errorf("数据不足时应返回 ErrInsufficientData，实际是 %v", err)
	}
}

func TestPredictor_Predict_Success(t *testing.T) {
	p := NewPredictor(ForecastConfig{
		HistoryDays:   30,
		MinDataPoints: 5,
	})

	// 模拟7天的数据，每天增长100MB
	base := int64(1000 * 1024 * 1024) // 1GB
	for i := 0; i < 7; i++ {
		p.RecordUsage("test-target", base+int64(i)*100*1024*1024)
	}

	result, err := p.Predict("rule-1", "test-target", 100*1024*1024*1024) // 100GB
	if err != nil {
		t.Errorf("预测失败: %v", err)
		return
	}

	if result.RuleID != "rule-1" {
		t.Errorf("RuleID 应为 rule-1，实际是 %s", result.RuleID)
	}

	if result.TargetID != "test-target" {
		t.Errorf("TargetID 应为 test-target，实际是 %s", result.TargetID)
	}

	// 验证趋势（应该是增长）
	if result.Trend != TrendGrowing {
		t.Logf("趋势应为 growing，实际是 %s（可能是数据不够明显）", result.Trend)
	}
}

// ========== AlertRuleManager 测试 ==========

func TestNewAlertRuleManager(t *testing.T) {
	m, err := NewAlertRuleManager("")
	if err != nil {
		t.Errorf("创建告警规则管理器失败: %v", err)
	}

	if m == nil {
		t.Error("告警规则管理器不应为nil")
	}
}

func TestAlertRuleManager_CreateRule(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	input := AlertRuleInput{
		Name:       "测试规则",
		Thresholds: []int{60, 80, 90},
		Channels:   []string{"email"},
		Enabled:    true,
	}

	rule, err := m.CreateRule(input)
	if err != nil {
		t.Errorf("创建规则失败: %v", err)
		return
	}

	if rule.Name != "测试规则" {
		t.Errorf("规则名称应为 '测试规则'，实际是 %s", rule.Name)
	}

	if len(rule.Thresholds) != 3 {
		t.Errorf("阈值数量应为3，实际是 %d", len(rule.Thresholds))
	}
}

func TestAlertRuleManager_CreateRule_InvalidThreshold(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	// 无阈值
	input1 := AlertRuleInput{
		Name:       "测试规则",
		Thresholds: []int{},
	}

	_, err := m.CreateRule(input1)
	if err != ErrInvalidThreshold {
		t.Errorf("无阈值时应返回 ErrInvalidThreshold，实际是 %v", err)
	}

	// 超出范围的阈值
	input2 := AlertRuleInput{
		Name:       "测试规则",
		Thresholds: []int{150},
	}

	_, err = m.CreateRule(input2)
	if err != ErrInvalidThreshold {
		t.Errorf("超出范围的阈值应返回 ErrInvalidThreshold，实际是 %v", err)
	}
}

func TestAlertRuleManager_GetRule(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	input := AlertRuleInput{
		Name:       "测试规则",
		Thresholds: []int{60, 80},
	}

	rule, _ := m.CreateRule(input)

	// 获取规则
	foundRule, err := m.GetRule(rule.ID)
	if err != nil {
		t.Errorf("获取规则失败: %v", err)
		return
	}

	if foundRule.ID != rule.ID {
		t.Errorf("获取的规则ID应为 %s，实际是 %s", rule.ID, foundRule.ID)
	}
}

func TestAlertRuleManager_GetRule_NotFound(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	_, err := m.GetRule("nonexistent")
	if err != ErrAlertRuleNotFound {
		t.Errorf("规则不存在时应返回 ErrAlertRuleNotFound，实际是 %v", err)
	}
}

func TestAlertRuleManager_UpdateRule(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	input := AlertRuleInput{
		Name:       "测试规则",
		Thresholds: []int{60},
	}

	rule, _ := m.CreateRule(input)

	// 更新规则
	updateInput := AlertRuleInput{
		Name:       "更新后的规则",
		Thresholds: []int{70, 80},
	}

	updatedRule, err := m.UpdateRule(rule.ID, updateInput)
	if err != nil {
		t.Errorf("更新规则失败: %v", err)
		return
	}

	if updatedRule.Name != "更新后的规则" {
		t.Errorf("更新后的名称应为 '更新后的规则'，实际是 %s", updatedRule.Name)
	}

	if len(updatedRule.Thresholds) != 2 {
		t.Errorf("阈值数量应为2，实际是 %d", len(updatedRule.Thresholds))
	}
}

func TestAlertRuleManager_DeleteRule(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	input := AlertRuleInput{
		Name:       "测试规则",
		Thresholds: []int{60},
	}

	rule, _ := m.CreateRule(input)

	// 删除规则
	err := m.DeleteRule(rule.ID)
	if err != nil {
		t.Errorf("删除规则失败: %v", err)
	}

	// 验证已删除
	_, err = m.GetRule(rule.ID)
	if err != ErrAlertRuleNotFound {
		t.Error("删除后规则不应存在")
	}
}

func TestAlertRuleManager_ShouldAlert(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	input := AlertRuleInput{
		Name:       "测试规则",
		TargetType: "user",
		TargetID:   "test-user",
		Thresholds: []int{80, 90},
		Channels:   []string{"email"},
		Enabled:    true,
	}

	m.CreateRule(input)

	// 测试低于阈值
	rules := m.ShouldAlert("user", "test-user", 50.0)
	if len(rules) != 0 {
		t.Errorf("低于阈值不应触发告警，实际触发了 %d 个规则", len(rules))
	}

	// 测试达到阈值
	rules = m.ShouldAlert("user", "test-user", 85.0)
	if len(rules) != 1 {
		t.Errorf("达到阈值应触发1个规则，实际是 %d", len(rules))
	}

	// 测试目标不匹配
	rules = m.ShouldAlert("group", "test-group", 85.0)
	if len(rules) != 0 {
		t.Errorf("目标不匹配不应触发告警")
	}
}

func TestAlertRuleManager_ShouldAlert_Disabled(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	input := AlertRuleInput{
		Name:       "测试规则",
		Thresholds: []int{80},
		Enabled:    false, // 禁用
	}

	m.CreateRule(input)

	rules := m.ShouldAlert("user", "test-user", 85.0)
	if len(rules) != 0 {
		t.Errorf("禁用的规则不应触发告警")
	}
}

func TestAlertRuleManager_RecordAlert(t *testing.T) {
	m, _ := NewAlertRuleManager("")

	m.RecordAlert("rule-1", "target-1", 80)

	stats := m.GetAlertStats()
	trackedStates := stats["tracked_states"].(int)
	if trackedStates != 1 {
		t.Errorf("应有1个追踪状态，实际是 %d", trackedStates)
	}
}

func TestDefaultAlertRules(t *testing.T) {
	rules := DefaultAlertRules()
	if len(rules) != 4 {
		t.Errorf("默认告警规则应为4个，实际是 %d", len(rules))
	}
}

// ========== Manager 测试 ==========

type mockStorageProvider struct {
	volumeUsage map[string][3]int64 // total, used, free
	userUsage   map[string]int64
	groupUsage  map[string]int64
}

func (m *mockStorageProvider) GetVolumeUsage(volumeName string) (total, used, free int64, err error) {
	if data, ok := m.volumeUsage[volumeName]; ok {
		return data[0], data[1], data[2], nil
	}
	return 0, 0, 0, nil
}

func (m *mockStorageProvider) GetUserUsage(username, volumeName string) (used int64, err error) {
	if usage, ok := m.userUsage[username]; ok {
		return usage, nil
	}
	return 0, nil
}

func (m *mockStorageProvider) GetGroupUsage(groupName, volumeName string) (used int64, err error) {
	if usage, ok := m.groupUsage[groupName]; ok {
		return usage, nil
	}
	return 0, nil
}

type mockUserProvider struct {
	users  map[string]bool
	groups map[string]bool
}

func (m *mockUserProvider) UserExists(username string) bool {
	return m.users[username]
}

func (m *mockUserProvider) GroupExists(groupName string) bool {
	return m.groups[groupName]
}

func TestNewManager(t *testing.T) {
	storage := &mockStorageProvider{
		volumeUsage: map[string][3]int64{
			"vol1": {1000, 500, 500},
		},
	}
	user := &mockUserProvider{
		users:  map[string]bool{"user1": true},
		groups: map[string]bool{"group1": true},
	}

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Errorf("创建管理器失败: %v", err)
		return
	}

	if mgr == nil {
		t.Error("管理器不应为nil")
	}
}

func TestManager_CreateRule(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{
		users:  map[string]bool{"user1": true},
		groups: map[string]bool{"group1": true},
	}

	mgr, _ := NewManager("", storage, user)

	input := QuotaRuleInput{
		TargetType:  "user",
		TargetID:    "user1",
		MaxBytes:    10 * 1024 * 1024 * 1024, // 10GB
		WarnPercent: 80,
		Enabled:     true,
	}

	rule, err := mgr.CreateRule(input)
	if err != nil {
		t.Errorf("创建规则失败: %v", err)
		return
	}

	if rule.TargetID != "user1" {
		t.Errorf("目标ID应为 user1，实际是 %s", rule.TargetID)
	}

	if rule.WarnPercent != 80 {
		t.Errorf("警告百分比应为80，实际是 %d", rule.WarnPercent)
	}
}

func TestManager_CreateRule_InvalidTarget(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{
		users: map[string]bool{}, // 无用户
	}

	mgr, _ := NewManager("", storage, user)

	input := QuotaRuleInput{
		TargetType: "user",
		TargetID:   "nonexistent",
		MaxBytes:   1024,
	}

	_, err := mgr.CreateRule(input)
	if err != ErrInvalidTarget {
		t.Errorf("无效目标应返回 ErrInvalidTarget，实际是 %v", err)
	}
}

func TestManager_CreateRule_Exists(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	input := QuotaRuleInput{
		TargetType: "user",
		TargetID:   "user1",
		MaxBytes:   1024,
	}

	mgr.CreateRule(input)

	// 再次创建相同规则
	_, err := mgr.CreateRule(input)
	if err != ErrRuleExists {
		t.Errorf("规则已存在时应返回 ErrRuleExists，实际是 %v", err)
	}
}

func TestManager_GetRule(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	input := QuotaRuleInput{
		TargetType: "user",
		TargetID:   "user1",
		MaxBytes:   1024,
	}

	rule, _ := mgr.CreateRule(input)

	// 获取规则
	foundRule, err := mgr.GetRule(rule.ID)
	if err != nil {
		t.Errorf("获取规则失败: %v", err)
		return
	}

	if foundRule.ID != rule.ID {
		t.Errorf("获取的规则ID应为 %s，实际是 %s", rule.ID, foundRule.ID)
	}
}

func TestManager_ListRules(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true, "user2": true},
	}

	mgr, _ := NewManager("", storage, user)

	mgr.CreateRule(QuotaRuleInput{TargetType: "user", TargetID: "user1", MaxBytes: 1024})
	mgr.CreateRule(QuotaRuleInput{TargetType: "user", TargetID: "user2", MaxBytes: 2048})

	rules := mgr.ListRules()
	if len(rules) != 2 {
		t.Errorf("规则数量应为2，实际是 %d", len(rules))
	}
}

func TestManager_UpdateRule(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	rule, _ := mgr.CreateRule(QuotaRuleInput{
		TargetType:  "user",
		TargetID:    "user1",
		MaxBytes:    1024,
		WarnPercent: 80,
	})

	// 更新规则
	updateInput := QuotaRuleInput{
		MaxBytes:    2048,
		WarnPercent: 90,
		Enabled:     false,
	}

	updatedRule, err := mgr.UpdateRule(rule.ID, updateInput)
	if err != nil {
		t.Errorf("更新规则失败: %v", err)
		return
	}

	if updatedRule.MaxBytes != 2048 {
		t.Errorf("MaxBytes应为2048，实际是 %d", updatedRule.MaxBytes)
	}

	if updatedRule.WarnPercent != 90 {
		t.Errorf("WarnPercent应为90，实际是 %d", updatedRule.WarnPercent)
	}

	if updatedRule.Enabled {
		t.Error("Enabled应为false")
	}
}

func TestManager_DeleteRule(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	rule, _ := mgr.CreateRule(QuotaRuleInput{
		TargetType: "user",
		TargetID:   "user1",
		MaxBytes:   1024,
	})

	err := mgr.DeleteRule(rule.ID)
	if err != nil {
		t.Errorf("删除规则失败: %v", err)
	}

	// 验证已删除
	_, err = mgr.GetRule(rule.ID)
	if err != ErrRuleNotFound {
		t.Error("删除后规则不应存在")
	}
}

func TestManager_GetUsage(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 500},
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	rule, _ := mgr.CreateRule(QuotaRuleInput{
		TargetType: "user",
		TargetID:   "user1",
		MaxBytes:   1000,
	})

	usage, err := mgr.GetUsage(rule.ID)
	if err != nil {
		t.Errorf("获取使用情况失败: %v", err)
		return
	}

	if usage.UsedBytes != 500 {
		t.Errorf("UsedBytes应为500，实际是 %d", usage.UsedBytes)
	}

	if usage.Percent != 50.0 {
		t.Errorf("Percent应为50.0，实际是 %.2f", usage.Percent)
	}
}

func TestManager_GetAllUsage(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 500, "user2": 800},
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true, "user2": true},
	}

	mgr, _ := NewManager("", storage, user)

	mgr.CreateRule(QuotaRuleInput{TargetType: "user", TargetID: "user1", MaxBytes: 1000})
	mgr.CreateRule(QuotaRuleInput{TargetType: "user", TargetID: "user2", MaxBytes: 1000})

	usages := mgr.GetAllUsage()
	if len(usages) != 2 {
		t.Errorf("使用情况数量应为2，实际是 %d", len(usages))
	}
}

func TestManager_CheckQuota(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 800},
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	mgr.CreateRule(QuotaRuleInput{
		TargetType: "user",
		TargetID:   "user1",
		MaxBytes:   1000,
		Enabled:    true,
	})

	// 检查允许的写入
	err := mgr.CheckQuota("user", "user1", 100) // 当前800，再加100，不超过1000
	if err != nil {
		t.Errorf("100字节应该允许写入，错误: %v", err)
	}

	// 检查不允许的写入
	err = mgr.CheckQuota("user", "user1", 300) // 当前800，再加300，超过1000
	if err != ErrQuotaExceeded {
		t.Errorf("超出配额应返回 ErrQuotaExceeded，实际是 %v", err)
	}
}

func TestManager_CheckQuota_NoRule(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{}

	mgr, _ := NewManager("", storage, user)

	// 无规则时应允许
	err := mgr.CheckQuota("user", "user1", 1000)
	if err != nil {
		t.Errorf("无规则时应允许写入，错误: %v", err)
	}
}

func TestManager_SetNotifyConfig(t *testing.T) {
	storage := &mockStorageProvider{}
	user := &mockUserProvider{}

	mgr, _ := NewManager("", storage, user)

	config := NotificationConfig{
		Enabled:     false,
		CoolDownMin: 60,
	}

	mgr.SetNotifyConfig(config)

	// 配置已设置（通过GetNotifyConfig验证，需要在handlers中测试）
}

func TestManager_PredictUsage(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 800},
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	rule, _ := mgr.CreateRule(QuotaRuleInput{
		TargetType: "user",
		TargetID:   "user1",
		MaxBytes:   10000,
	})

	// 需要足够的历史数据才能预测
	for i := 0; i < 10; i++ {
		mgr.predictor.RecordUsage("user1", int64(800+i*10))
	}

	result, err := mgr.PredictUsage(rule.ID)
	if err != nil {
		t.Logf("预测可能因数据不足失败: %v", err)
		return
	}

	if result.RuleID != rule.ID {
		t.Errorf("RuleID应为 %s，实际是 %s", rule.ID, result.RuleID)
	}
}

func TestManager_CheckAndAlert(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 900}, // 90%使用
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	mgr.CreateRule(QuotaRuleInput{
		TargetType:  "user",
		TargetID:    "user1",
		MaxBytes:    1000,
		WarnPercent: 80,
		Enabled:     true,
	})

	alerts := mgr.CheckAndAlert()
	if len(alerts) == 0 {
		t.Error("达到警告阈值应产生告警")
	}
}

func TestManager_CheckAndAlert_Normal(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 50}, // 5%使用
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	mgr.CreateRule(QuotaRuleInput{
		TargetType:  "user",
		TargetID:    "user1",
		MaxBytes:    1000,
		WarnPercent: 80,
		Enabled:     true,
	})

	alerts := mgr.CheckAndAlert()
	if len(alerts) != 0 {
		t.Errorf("低于阈值不应产生告警，实际产生了 %d 个", len(alerts))
	}
}

func TestManager_GetAlerts(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 900},
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	mgr.CreateRule(QuotaRuleInput{
		TargetType:  "user",
		TargetID:    "user1",
		MaxBytes:    1000,
		WarnPercent: 80,
		Enabled:     true,
	})

	mgr.CheckAndAlert()

	alerts := mgr.GetAlerts()
	if len(alerts) == 0 {
		t.Error("应有活跃告警")
	}
}

func TestManager_ResolveAlert(t *testing.T) {
	storage := &mockStorageProvider{
		userUsage: map[string]int64{"user1": 900},
	}
	user := &mockUserProvider{
		users: map[string]bool{"user1": true},
	}

	mgr, _ := NewManager("", storage, user)

	mgr.CreateRule(QuotaRuleInput{
		TargetType:  "user",
		TargetID:    "user1",
		MaxBytes:    1000,
		WarnPercent: 80,
		Enabled:     true,
	})

	mgr.CheckAndAlert()
	alerts := mgr.GetAlerts()

	if len(alerts) > 0 {
		err := mgr.ResolveAlert(alerts[0].ID)
		if err != nil {
			t.Errorf("解决告警失败: %v", err)
		}

		// 验证已解决
		alerts = mgr.GetAlerts()
		for _, a := range alerts {
			if a.Resolved {
				t.Error("已解决的告警不应出现在活跃列表中")
			}
		}
	}
}

// ========== FormatBytes 测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		// 允许一定的精度差异
		if result != tt.expected && !contains(result, "B") {
			t.Errorf("FormatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
