package quotaalert

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager("/tmp/test-quotaalert")
	if manager == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if manager.rules == nil {
		t.Fatal("rules map 未初始化")
	}
	if manager.usages == nil {
		t.Fatal("usages map 未初始化")
	}
	if manager.alerts == nil {
		t.Fatal("alerts map 未初始化")
	}
}

func TestSetAndGetQuota(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100, // 100MB
		MaxFiles:          1000,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}

	// 测试设置配额
	if err := manager.SetQuota(ctx, rule); err != nil {
		t.Fatalf("设置配额失败: %v", err)
	}

	// 测试获取配额
	got, err := manager.GetQuota(ctx, "user1", "/data/user1")
	if err != nil {
		t.Fatalf("获取配额失败: %v", err)
	}

	if got.UserID != "user1" {
		t.Errorf("UserID 不匹配: got %s, want user1", got.UserID)
	}
	if got.MaxBytes != 1024*1024*100 {
		t.Errorf("MaxBytes 不匹配: got %d, want %d", got.MaxBytes, 1024*1024*100)
	}
}

func TestSetQuotaInvalidThreshold(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 测试无效阈值
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100,
		WarnThreshold:     0.95,
		CriticalThreshold: 0.8, // 严重告警阈值小于告警阈值
	}

	if err := manager.SetQuota(ctx, rule); err == nil {
		t.Error("期望返回错误，但没有")
	}
}

func TestUpdateUsage(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 先设置配额
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100,
		MaxFiles:          1000,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}
	manager.SetQuota(ctx, rule)

	// 更新使用量
	usedBytes := int64(1024 * 1024 * 50) // 50MB
	usedFiles := int64(500)
	if err := manager.UpdateUsage(ctx, "user1", "/data/user1", usedBytes, usedFiles); err != nil {
		t.Fatalf("更新使用量失败: %v", err)
	}

	// 验证使用量
	key := ruleKey("user1", "/data/user1")
	usage, ok := manager.usages[key]
	if !ok {
		t.Fatal("使用量记录不存在")
	}
	if usage.UsedBytes != usedBytes {
		t.Errorf("UsedBytes 不匹配: got %d, want %d", usage.UsedBytes, usedBytes)
	}
	if usage.UsedFiles != usedFiles {
		t.Errorf("UsedFiles 不匹配: got %d, want %d", usage.UsedFiles, usedFiles)
	}
}

func TestCheckQuota(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 设置配额
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100, // 100MB
		MaxFiles:          1000,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}
	manager.SetQuota(ctx, rule)

	// 测试正常情况（低于警告阈值）
	manager.UpdateUsage(ctx, "user1", "/data/user1", 1024*1024*50, 500) // 50MB = 50%
	alert, err := manager.CheckQuota(ctx, "user1", "/data/user1")
	if err != nil {
		t.Fatalf("检查配额失败: %v", err)
	}
	if alert != nil {
		t.Errorf("不应生成告警，但生成了: %v", alert)
	}

	// 测试警告情况（超过警告阈值）
	manager.UpdateUsage(ctx, "user1", "/data/user1", 1024*1024*85, 850) // 85MB = 85%
	alert, err = manager.CheckQuota(ctx, "user1", "/data/user1")
	if err != nil {
		t.Fatalf("检查配额失败: %v", err)
	}
	if alert == nil {
		t.Fatal("应生成告警，但没有")
	}
	if alert.Level != AlertWarning {
		t.Errorf("告警级别不匹配: got %s, want warning", alert.Level)
	}

	// 测试严重告警情况（超过严重告警阈值）
	manager.UpdateUsage(ctx, "user1", "/data/user1", 1024*1024*96, 960) // 96MB = 96%
	alert, err = manager.CheckQuota(ctx, "user1", "/data/user1")
	if err != nil {
		t.Fatalf("检查配额失败: %v", err)
	}
	if alert == nil {
		t.Fatal("应生成告警，但没有")
	}
	if alert.Level != AlertCritical {
		t.Errorf("告警级别不匹配: got %s, want critical", alert.Level)
	}

	// 测试超限情况
	manager.UpdateUsage(ctx, "user1", "/data/user1", 1024*1024*105, 1050) // 105MB = 105%
	alert, err = manager.CheckQuota(ctx, "user1", "/data/user1")
	if err != nil {
		t.Fatalf("检查配额失败: %v", err)
	}
	if alert == nil {
		t.Fatal("应生成告警，但没有")
	}
	if alert.Level != AlertExceeded {
		t.Errorf("告警级别不匹配: got %s, want exceeded", alert.Level)
	}
}

func TestGetAlerts(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 设置配额并生成告警
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}
	manager.SetQuota(ctx, rule)
	manager.UpdateUsage(ctx, "user1", "/data/user1", 1024*1024*85, 850)
	manager.CheckQuota(ctx, "user1", "/data/user1")

	// 获取所有告警
	alerts := manager.GetAlerts(ctx, "", false)
	if len(alerts) == 0 {
		t.Fatal("应有告警，但没有")
	}

	// 获取未确认的告警
	unackAlerts := manager.GetAlerts(ctx, "", true)
	if len(unackAlerts) == 0 {
		t.Fatal("应有未确认告警，但没有")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 设置配额并生成告警
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}
	manager.SetQuota(ctx, rule)
	manager.UpdateUsage(ctx, "user1", "/data/user1", 1024*1024*85, 850)
	alert, _ := manager.CheckQuota(ctx, "user1", "/data/user1")

	if alert == nil {
		t.Fatal("告警未生成")
	}

	// 确认告警
	if err := manager.AcknowledgeAlert(ctx, alert.ID); err != nil {
		t.Fatalf("确认告警失败: %v", err)
	}

	// 验证告警已确认
	unackAlerts := manager.GetAlerts(ctx, "", true)
	for _, a := range unackAlerts {
		if a.ID == alert.ID {
			t.Error("告警应已确认，但未确认")
		}
	}
}

func TestPredictFullDate(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 设置配额
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100, // 100MB
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}
	manager.SetQuota(ctx, rule)

	// 添加历史数据
	key := ruleKey("user1", "/data/user1")
	manager.history[key] = []UsageHistory{
		{UsedBytes: 1024 * 1024 * 30, UsedFiles: 300, Timestamp: time.Now().Add(-2 * 24 * time.Hour)},
		{UsedBytes: 1024 * 1024 * 40, UsedFiles: 400, Timestamp: time.Now().Add(-1 * 24 * time.Hour)},
		{UsedBytes: 1024 * 1024 * 50, UsedFiles: 500, Timestamp: time.Now()},
	}
	manager.usages[key] = &QuotaUsage{
		UsedBytes: 1024 * 1024 * 50,
		UsedFiles: 500,
	}

	// 预测用满日期
	trend, err := manager.PredictFullDate(ctx, "user1", "/data/user1")
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if trend.PredictedFullDate == nil {
		t.Error("预测日期不应为 nil")
	}
	if trend.TrendDirection != TrendGrowing {
		t.Errorf("趋势方向不匹配: got %s, want growing", trend.TrendDirection)
	}
}

func TestGenerateCleanupSuggestions(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 设置配额
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}
	manager.SetQuota(ctx, rule)

	// 设置高使用量
	manager.usages[ruleKey("user1", "/data/user1")] = &QuotaUsage{
		UsedBytes:    1024 * 1024 * 90,
		UsedFiles:    900,
		UsagePercent: 90,
	}

	// 获取清理建议
	suggestions, err := manager.GenerateCleanupSuggestions(ctx, "user1", "/data/user1")
	if err != nil {
		t.Fatalf("获取清理建议失败: %v", err)
	}

	if len(suggestions) == 0 {
		t.Fatal("应有清理建议，但没有")
	}

	// 验证建议类型
	hasTemp := false
	for _, s := range suggestions {
		if s.SuggestionType == SuggestionTemp {
			hasTemp = true
		}
	}
	if !hasTemp {
		t.Error("应包含临时文件清理建议")
	}
}

func TestGenerateReport(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	// 设置配额
	rule := QuotaRule{
		Path:              "/data/user1",
		UserID:            "user1",
		MaxBytes:          1024 * 1024 * 100,
		WarnThreshold:     0.8,
		CriticalThreshold: 0.95,
		Enabled:           true,
	}
	manager.SetQuota(ctx, rule)
	manager.UpdateUsage(ctx, "user1", "/data/user1", 1024*1024*50, 500)

	// 生成报告
	report, err := manager.GenerateReport(ctx)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt 不应为零值")
	}
	if len(report.Users) == 0 {
		t.Fatal("应有用户数据，但没有")
	}

	// 验证用户数据
	found := false
	for _, u := range report.Users {
		if u.UserID == "user1" {
			found = true
			if u.TotalQuota != 1024*1024*100 {
				t.Errorf("TotalQuota 不匹配: got %d, want %d", u.TotalQuota, 1024*1024*100)
			}
			if u.UsedQuota != 1024*1024*50 {
				t.Errorf("UsedQuota 不匹配: got %d, want %d", u.UsedQuota, 1024*1024*50)
			}
		}
	}
	if !found {
		t.Error("报告中未找到 user1")
	}
}

func TestGetQuotaNotFound(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	_, err := manager.GetQuota(ctx, "nonexistent", "/data/path")
	if err != ErrQuotaNotFound {
		t.Errorf("期望 ErrQuotaNotFound，但得到: %v", err)
	}
}

func TestUpdateUsageNotFound(t *testing.T) {
	manager := NewManager("")
	ctx := context.Background()

	err := manager.UpdateUsage(ctx, "nonexistent", "/data/path", 100, 10)
	if err != ErrQuotaNotFound {
		t.Errorf("期望 ErrQuotaNotFound，但得到: %v", err)
	}
}
