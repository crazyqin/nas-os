// Package health 提供系统健康检查功能
// alerts_test.go - 告警管理器单元测试 v2.51.0
package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewAlertManager(t *testing.T) {
	hc := NewHealthChecker()
	am := NewAlertManager(hc)

	require.NotNil(t, am)
	assert.NotNil(t, am.alerts)
	assert.NotNil(t, am.rules)
	assert.NotNil(t, am.channels)
	assert.NotNil(t, am.silences)
	assert.NotNil(t, am.notifyQueue)
}

func TestNewAlertManagerWithOptions(t *testing.T) {
	hc := NewHealthChecker()
	logger := zap.NewNop()
	config := AlertManagerConfig{
		DefaultRepeatInterval: 10 * time.Minute,
		DefaultDuration:       2 * time.Minute,
		MaxAlerts:             500,
	}

	am := NewAlertManager(hc,
		WithAlertLogger(logger),
		WithAlertConfig(config),
	)

	require.NotNil(t, am)
	assert.Equal(t, logger, am.logger)
	assert.Equal(t, 10*time.Minute, am.config.DefaultRepeatInterval)
	assert.Equal(t, 2*time.Minute, am.config.DefaultDuration)
	assert.Equal(t, 500, am.config.MaxAlerts)
}

func TestAlertManager_AddRule(t *testing.T) {
	am := NewAlertManager(nil)

	rule := AlertRule{
		Name:      "high-memory",
		CheckName: "memory",
		Condition: AlertCondition{
			Type:      "threshold",
			Threshold: 80.0,
			Operator:  "gt",
		},
		Severity: SeverityWarning,
		Enabled:  true,
	}

	err := am.AddRule(rule)
	require.NoError(t, err)

	am.mu.RLock()
	_, exists := am.rules["high-memory"]
	am.mu.RUnlock()

	assert.True(t, exists)
}

func TestAlertManager_RemoveRule(t *testing.T) {
	am := NewAlertManager(nil)

	rule := AlertRule{
		Name:      "test-rule",
		CheckName: "memory",
		Severity:  SeverityWarning,
		Enabled:   true,
	}

	am.AddRule(rule)
	am.RemoveRule("test-rule")

	am.mu.RLock()
	_, exists := am.rules["test-rule"]
	am.mu.RUnlock()

	assert.False(t, exists)
}

func TestAlertManager_AddChannel(t *testing.T) {
	am := NewAlertManager(nil)

	channel := NotificationChannel{
		ID:      "email-1",
		Name:    "Admin Email",
		Type:    "email",
		Enabled: true,
		Config: map[string]interface{}{
			"smtp_host": "smtp.example.com",
			"to":        "admin@example.com",
		},
	}

	am.AddChannel(channel)

	am.mu.RLock()
	_, exists := am.channels["email-1"]
	am.mu.RUnlock()

	assert.True(t, exists)
}

func TestAlertManager_RemoveChannel(t *testing.T) {
	am := NewAlertManager(nil)

	channel := NotificationChannel{
		ID:      "test-channel",
		Name:    "Test",
		Type:    "webhook",
		Enabled: true,
	}

	am.AddChannel(channel)
	am.RemoveChannel("test-channel")

	am.mu.RLock()
	_, exists := am.channels["test-channel"]
	am.mu.RUnlock()

	assert.False(t, exists)
}

func TestAlertManager_Evaluate(t *testing.T) {
	hc := NewHealthChecker(WithMemoryThreshold(10.0)) // 低阈值触发告警
	am := NewAlertManager(hc)

	// 添加告警规则
	rule := AlertRule{
		Name:      "memory-alert",
		CheckName: "memory",
		Condition: AlertCondition{
			Type:   "status",
			Status: "unhealthy",
		},
		Severity: SeverityCritical,
		Enabled:  true,
	}
	am.AddRule(rule)

	// 执行健康检查
	ctx := context.Background()
	report := hc.Check(ctx)

	// 评估告警
	am.Evaluate(ctx, report)

	// 检查是否生成了告警
	alerts := am.GetAlerts()
	t.Logf("Alerts generated: %d", len(alerts))
}

func TestAlertManager_CheckCondition(t *testing.T) {
	am := NewAlertManager(nil)

	tests := []struct {
		name      string
		condition AlertCondition
		result    CheckResult
		expected  bool
	}{
		{
			name: "status unhealthy match",
			condition: AlertCondition{
				Type:   "status",
				Status: "unhealthy",
			},
			result: CheckResult{
				Status: StatusUnhealthy,
			},
			expected: true,
		},
		{
			name: "status no match",
			condition: AlertCondition{
				Type:   "status",
				Status: "unhealthy",
			},
			result: CheckResult{
				Status: StatusHealthy,
			},
			expected: false,
		},
		{
			name: "threshold gt match",
			condition: AlertCondition{
				Type:      "threshold",
				Threshold: 80.0,
				Operator:  "gt",
			},
			result: CheckResult{
				Details: map[string]interface{}{
					"used_percent": 85.0,
				},
			},
			expected: true,
		},
		{
			name: "threshold gt no match",
			condition: AlertCondition{
				Type:      "threshold",
				Threshold: 80.0,
				Operator:  "gt",
			},
			result: CheckResult{
				Details: map[string]interface{}{
					"used_percent": 75.0,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := am.checkCondition(&tt.condition, tt.result)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompareThreshold(t *testing.T) {
	tests := []struct {
		value     float64
		threshold float64
		operator  string
		expected  bool
	}{
		{85.0, 80.0, "gt", true},
		{75.0, 80.0, "gt", false},
		{80.0, 80.0, "gte", true},
		{79.0, 80.0, "gte", false},
		{75.0, 80.0, "lt", true},
		{85.0, 80.0, "lt", false},
		{80.0, 80.0, "lte", true},
		{81.0, 80.0, "lte", false},
		{80.0, 80.0, "eq", true},
		{81.0, 80.0, "eq", false},
	}

	for _, tt := range tests {
		result := compareThreshold(tt.value, tt.threshold, tt.operator)
		assert.Equal(t, tt.expected, result, "value=%v, threshold=%v, operator=%s", tt.value, tt.threshold, tt.operator)
	}
}

func TestAlertManager_CreateSilence(t *testing.T) {
	am := NewAlertManager(nil)

	silence := SilenceConfig{
		Matchers: map[string]string{
			"check": "memory",
		},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		Reason:    "Maintenance",
		CreatedBy: "admin",
	}

	id := am.CreateSilence(silence)
	assert.NotEmpty(t, id)

	silences := am.GetSilences()
	assert.Len(t, silences, 1)
}

func TestAlertManager_DeleteSilence(t *testing.T) {
	am := NewAlertManager(nil)

	silence := SilenceConfig{
		Matchers:  map[string]string{"check": "test"},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		CreatedBy: "admin",
	}

	id := am.CreateSilence(silence)
	am.DeleteSilence(id)

	silences := am.GetSilences()
	assert.Empty(t, silences)
}

func TestAlertManager_IsSilenced(t *testing.T) {
	am := NewAlertManager(nil)

	// 创建静默规则
	am.CreateSilence(SilenceConfig{
		Matchers: map[string]string{
			"check": "memory",
		},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(1 * time.Hour),
		CreatedBy: "admin",
	})

	// 匹配的告警应该被静默
	alert := &Alert{
		Name:   "memory-alert",
		Labels: map[string]string{"check": "memory"},
	}
	assert.True(t, am.isSilenced(alert))

	// 不匹配的告警不应该被静默
	alert2 := &Alert{
		Name:   "cpu-alert",
		Labels: map[string]string{"check": "cpu"},
	}
	assert.False(t, am.isSilenced(alert2))
}

func TestAlertManager_GetAlerts(t *testing.T) {
	am := NewAlertManager(nil)

	// 添加一些告警
	am.mu.Lock()
	am.alerts["alert1"] = &Alert{
		ID:    "1",
		Name:  "test1",
		State: AlertStateFiring,
	}
	am.alerts["alert2"] = &Alert{
		ID:    "2",
		Name:  "test2",
		State: AlertStateResolved,
	}
	am.mu.Unlock()

	alerts := am.GetAlerts()
	assert.Len(t, alerts, 2)
}

func TestAlertManager_GetActiveAlerts(t *testing.T) {
	am := NewAlertManager(nil)

	am.mu.Lock()
	am.alerts["alert1"] = &Alert{
		ID:    "1",
		Name:  "active",
		State: AlertStateFiring,
	}
	am.alerts["alert2"] = &Alert{
		ID:    "2",
		Name:  "resolved",
		State: AlertStateResolved,
	}
	am.mu.Unlock()

	active := am.GetActiveAlerts()
	assert.Len(t, active, 1)
	assert.Equal(t, "active", active[0].Name)
}

func TestAlertManager_Stats(t *testing.T) {
	am := NewAlertManager(nil)

	am.mu.Lock()
	am.alerts["critical"] = &Alert{
		Name:     "critical-alert",
		Severity: SeverityCritical,
		State:    AlertStateFiring,
	}
	am.alerts["warning"] = &Alert{
		Name:     "warning-alert",
		Severity: SeverityWarning,
		State:    AlertStateFiring,
	}
	am.alerts["resolved"] = &Alert{
		Name:     "resolved-alert",
		Severity: SeverityInfo,
		State:    AlertStateResolved,
	}
	am.rules["rule1"] = &AlertRule{Name: "rule1"}
	am.channels["channel1"] = &NotificationChannel{ID: "channel1"}
	am.silences["silence1"] = &SilenceConfig{ID: "silence1"}
	am.mu.Unlock()

	stats := am.Stats()

	assert.Equal(t, 3, stats.TotalAlerts)
	assert.Equal(t, 2, stats.ActiveAlerts)
	assert.Equal(t, 1, stats.CriticalAlerts)
	assert.Equal(t, 1, stats.WarningAlerts)
	assert.Equal(t, 1, stats.InfoAlerts)
	assert.Equal(t, 1, stats.TotalRules)
	assert.Equal(t, 1, stats.TotalChannels)
	assert.Equal(t, 1, stats.TotalSilences)
}

func TestAlertManager_ClearResolvedAlerts(t *testing.T) {
	am := NewAlertManager(nil)

	am.mu.Lock()
	am.alerts["resolved-old"] = &Alert{
		Name:       "resolved-old",
		State:      AlertStateResolved,
		ResolvedAt: time.Now().Add(-25 * time.Hour),
	}
	am.alerts["resolved-new"] = &Alert{
		Name:       "resolved-new",
		State:      AlertStateResolved,
		ResolvedAt: time.Now().Add(-1 * time.Hour),
	}
	am.alerts["firing"] = &Alert{
		Name:  "firing",
		State: AlertStateFiring,
	}
	am.mu.Unlock()

	am.ClearResolvedAlerts()

	am.mu.RLock()
	assert.Len(t, am.alerts, 2)
	_, exists := am.alerts["resolved-old"]
	am.mu.RUnlock()
	assert.False(t, exists)
}

func TestAlertManager_ClearExpiredSilences(t *testing.T) {
	am := NewAlertManager(nil)

	am.mu.Lock()
	am.silences["expired"] = &SilenceConfig{
		ID:     "expired",
		EndsAt: time.Now().Add(-1 * time.Hour),
	}
	am.silences["active"] = &SilenceConfig{
		ID:     "active",
		EndsAt: time.Now().Add(1 * time.Hour),
	}
	am.mu.Unlock()

	am.ClearExpiredSilences()

	am.mu.RLock()
	assert.Len(t, am.silences, 1)
	_, exists := am.silences["active"]
	am.mu.RUnlock()
	assert.True(t, exists)
}

func TestAlertManager_SendWebhook(t *testing.T) {
	am := NewAlertManager(nil)

	channel := &NotificationChannel{
		ID:   "webhook",
		Type: "webhook",
		Config: map[string]interface{}{
			"url": "http://example.com/webhook",
		},
	}

	alert := &Alert{
		ID:       "test-alert",
		Name:     "Test Alert",
		Severity: SeverityWarning,
		State:    AlertStateFiring,
		Message:  "Test message",
	}

	// Webhook 发送到不存在的地址会失败，但不会 panic
	err := am.sendWebhook(channel, alert)
	// 由于目标不存在，预期会出错
	assert.Error(t, err)
}

func TestAlertManager_SendDingTalk(t *testing.T) {
	am := NewAlertManager(nil)

	channel := &NotificationChannel{
		ID:   "dingtalk",
		Type: "dingtalk",
		Config: map[string]interface{}{
			"webhook": "http://example.com/dingtalk/webhook",
		},
	}

	alert := &Alert{
		ID:       "test-alert",
		Name:     "Test Alert",
		Severity: SeverityCritical,
		State:    AlertStateFiring,
		Message:  "Critical test message",
		FiredAt:  time.Now(),
	}

	// 钉钉发送到不存在的地址会失败，但不会 panic
	err := am.sendDingTalk(channel, alert)
	assert.Error(t, err)
}

func TestAlertManager_SendEmail(t *testing.T) {
	am := NewAlertManager(nil)

	channel := &NotificationChannel{
		ID:   "email",
		Type: "email",
		Config: map[string]interface{}{
			"smtp_host": "smtp.example.com",
			"smtp_port": "587",
			"to":        "admin@example.com",
		},
	}

	alert := &Alert{
		ID:       "test-alert",
		Name:     "Test Alert",
		Severity: SeverityWarning,
		Message:  "Test message",
		FiredAt:  time.Now(),
	}

	// 邮件发送会失败，但不会 panic
	err := am.sendEmail(channel, alert)
	assert.Error(t, err)
}

func TestAlertManager_FormatAlertEmail(t *testing.T) {
	am := NewAlertManager(nil)

	alert := &Alert{
		Name:      "Memory Alert",
		CheckName: "memory",
		Severity:  SeverityCritical,
		State:     AlertStateFiring,
		Message:   "Memory usage exceeds 90%",
		FiredAt:   time.Now(),
		Details: map[string]interface{}{
			"used_percent": 92.5,
			"threshold":    90.0,
		},
	}

	body := am.formatAlertEmail(alert)
	assert.Contains(t, body, "Memory Alert")
	assert.Contains(t, body, "critical")
	assert.Contains(t, body, "Memory usage exceeds 90%")
}

func TestAlertManager_Stop(t *testing.T) {
	am := NewAlertManager(nil)

	// 启动后立即停止
	am.Stop()

	// 应该正常完成，不会阻塞
}

func TestGenerateAlertID(t *testing.T) {
	id1 := generateAlertID()
	id2 := generateAlertID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestGenerateSilenceID(t *testing.T) {
	id1 := generateSilenceID()
	id2 := generateSilenceID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestAlertManager_FullWorkflow(t *testing.T) {
	// 完整工作流测试
	hc := NewHealthChecker()
	am := NewAlertManager(hc, WithAlertLogger(zap.NewNop()))

	// 添加通知渠道
	am.AddChannel(NotificationChannel{
		ID:       "test-webhook",
		Name:     "Test Webhook",
		Type:     "webhook",
		Enabled:  false, // 禁用以避免实际发送
		Severity: []AlertSeverity{SeverityCritical, SeverityWarning},
		Config: map[string]interface{}{
			"url": "http://example.com/webhook",
		},
	})

	// 添加告警规则
	am.AddRule(AlertRule{
		Name:      "cpu-critical",
		CheckName: "cpu",
		Condition: AlertCondition{
			Type:   "status",
			Status: "unhealthy",
		},
		Severity:       SeverityCritical,
		RepeatInterval: 5 * time.Minute,
		Enabled:        true,
	})

	// 执行健康检查
	ctx := context.Background()
	report := hc.Check(ctx)

	// 评估告警
	am.Evaluate(ctx, report)

	// 获取统计
	stats := am.Stats()
	assert.GreaterOrEqual(t, stats.TotalRules, 1)
	assert.GreaterOrEqual(t, stats.TotalChannels, 1)

	// 停止
	am.Stop()
}

// 基准测试
func BenchmarkAlertManager_Evaluate(b *testing.B) {
	hc := NewHealthChecker()
	am := NewAlertManager(hc)

	am.AddRule(AlertRule{
		Name:      "memory-rule",
		CheckName: "memory",
		Condition: AlertCondition{
			Type:   "status",
			Status: "unhealthy",
		},
		Severity: SeverityWarning,
		Enabled:  true,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		report := hc.Check(ctx)
		am.Evaluate(ctx, report)
	}
}

func BenchmarkAlertManager_CheckCondition(b *testing.B) {
	am := NewAlertManager(nil)
	condition := AlertCondition{
		Type:      "threshold",
		Threshold: 80.0,
		Operator:  "gt",
	}
	result := CheckResult{
		Details: map[string]interface{}{
			"used_percent": 85.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		am.checkCondition(&condition, result)
	}
}
