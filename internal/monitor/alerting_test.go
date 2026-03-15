package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== AlertingManager 基础测试 ==========

func TestNewAlertingManager(t *testing.T) {
	am := NewAlertingManager()
	assert.NotNil(t, am)
	assert.NotNil(t, am.alerts)
	assert.NotNil(t, am.rules)
	assert.NotNil(t, am.subscribers)
	assert.Equal(t, 100, am.maxAlerts)
	assert.Equal(t, 1000, am.maxHistory)
}

func TestAlertingManager_AddRule(t *testing.T) {
	am := NewAlertingManager()

	rule := AlertRule{
		Name:      "high-cpu",
		Type:      "cpu",
		Threshold: 80.0,
		Level:     "warning",
		Enabled:   true,
	}

	am.AddRule(rule)

	rules := am.GetRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, "high-cpu", rules[0].Name)
}

func TestAlertingManager_GetRules(t *testing.T) {
	am := NewAlertingManager()

	// 空规则列表
	rules := am.GetRules()
	assert.Len(t, rules, 0)

	// 添加规则
	am.AddRule(AlertRule{Name: "rule1", Type: "cpu", Threshold: 80})
	am.AddRule(AlertRule{Name: "rule2", Type: "memory", Threshold: 85})

	rules = am.GetRules()
	assert.Len(t, rules, 2)
}

func TestAlertingManager_AddSubscriber(t *testing.T) {
	am := NewAlertingManager()

	sub := AlertSubscriber{
		ID:       "sub-1",
		Name:     "Admin Email",
		Type:     "email",
		Target:   "admin@example.com",
		MinLevel: "warning",
		Enabled:  true,
	}

	am.AddSubscriber(sub)

	subs := am.GetSubscribers()
	assert.Len(t, subs, 1)
	assert.Equal(t, "admin@example.com", subs[0].Target)
	assert.False(t, subs[0].CreatedAt.IsZero())
}

func TestAlertingManager_GetSubscribers(t *testing.T) {
	am := NewAlertingManager()

	// 空订阅列表
	subs := am.GetSubscribers()
	assert.Len(t, subs, 0)

	// 添加订阅者
	am.AddSubscriber(AlertSubscriber{ID: "sub-1", Type: "email", Target: "admin@example.com"})
	am.AddSubscriber(AlertSubscriber{ID: "sub-2", Type: "webhook", Target: "https://example.com/hook"})

	subs = am.GetSubscribers()
	assert.Len(t, subs, 2)
}

// ========== 告警触发测试 ==========

func TestAlertingManager_CheckThreshold(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{
		Name:      "cpu-warning",
		Type:      "cpu",
		Threshold: 80.0,
		Level:     "warning",
		Enabled:   true,
	})

	// 正常值，不触发告警
	am.CheckThreshold("cpu", 50.0, "test-source")
	alerts := am.GetAlerts(10, 0, nil)
	assert.Len(t, alerts, 0)

	// 超过阈值，触发告警
	am.CheckThreshold("cpu", 90.0, "test-source")
	alerts = am.GetAlerts(10, 0, nil)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "warning", alerts[0].Level)
	assert.Contains(t, alerts[0].Message, "90.0%")
}

func TestAlertingManager_CheckThreshold_DisabledRule(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{
		Name:      "cpu-warning",
		Type:      "cpu",
		Threshold: 80.0,
		Level:     "warning",
		Enabled:   false, // 禁用
	})

	am.CheckThreshold("cpu", 90.0, "test-source")
	alerts := am.GetAlerts(10, 0, nil)
	assert.Len(t, alerts, 0)
}

func TestAlertingManager_CheckThreshold_MultipleRules(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warning", Type: "cpu", Threshold: 70.0, Level: "warning", Enabled: true})
	am.AddRule(AlertRule{Name: "cpu-critical", Type: "cpu", Threshold: 90.0, Level: "critical", Enabled: true})
	am.AddRule(AlertRule{Name: "memory-warning", Type: "memory", Threshold: 80.0, Level: "warning", Enabled: true})

	am.CheckThreshold("cpu", 95.0, "test-source")
	alerts := am.GetAlerts(10, 0, nil)
	// 两个规则都满足条件
	assert.GreaterOrEqual(t, len(alerts), 1)
}

// ========== 告警确认和解决测试 ==========

func TestAlertingManager_AcknowledgeAlert(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warning", Type: "cpu", Threshold: 80.0, Level: "warning", Enabled: true})

	am.CheckThreshold("cpu", 90.0, "test-source")
	alerts := am.GetAlerts(10, 0, nil)
	require.Len(t, alerts, 1)

	// 确认告警
	err := am.AcknowledgeAlert(alerts[0].ID, "admin")
	assert.NoError(t, err)

	// 验证状态
	alerts = am.GetAlerts(10, 0, nil)
	assert.True(t, alerts[0].Acknowledged)

	// 检查历史记录
	history := am.GetAlertHistory(10, 0)
	assert.GreaterOrEqual(t, len(history), 1)
}

func TestAlertingManager_AcknowledgeAlert_NotFound(t *testing.T) {
	am := NewAlertingManager()

	err := am.AcknowledgeAlert("nonexistent", "admin")
	assert.Error(t, err)
}

func TestAlertingManager_ResolveAlert(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warning", Type: "cpu", Threshold: 80.0, Level: "warning", Enabled: true})

	am.CheckThreshold("cpu", 90.0, "test-source")
	alerts := am.GetAlerts(10, 0, nil)
	require.Len(t, alerts, 1)
	alertID := alerts[0].ID

	// 解决告警
	err := am.ResolveAlert(alertID)
	assert.NoError(t, err)

	// 验证已从活动列表移除
	alerts = am.GetAlerts(10, 0, nil)
	assert.Len(t, alerts, 0)

	// 验证在历史记录中
	history := am.GetAlertHistory(10, 0)
	found := false
	for _, h := range history {
		if h.AlertID == alertID && h.Action == "resolved" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestAlertingManager_ResolveAlert_NotFound(t *testing.T) {
	am := NewAlertingManager()

	err := am.ResolveAlert("nonexistent")
	assert.Error(t, err)
}

// ========== 告警查询测试 ==========

func TestAlertingManager_GetAlerts_Filters(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warn", Type: "cpu", Threshold: 70.0, Level: "warning", Enabled: true})
	am.AddRule(AlertRule{Name: "memory-warn", Type: "memory", Threshold: 80.0, Level: "warning", Enabled: true})

	am.CheckThreshold("cpu", 90.0, "source1")
	am.CheckThreshold("memory", 95.0, "source2")

	// 按类型过滤
	alerts := am.GetAlerts(10, 0, map[string]string{"type": "cpu"})
	assert.Len(t, alerts, 1)

	// 按级别过滤
	alerts = am.GetAlerts(10, 0, map[string]string{"level": "warning"})
	assert.Len(t, alerts, 2)
}

func TestAlertingManager_GetAlerts_Pagination(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warn", Type: "cpu", Threshold: 70.0, Level: "warning", Enabled: true})

	// 触发多个告警
	for i := 0; i < 5; i++ {
		am.CheckThreshold("cpu", 80.0+float64(i), "source")
	}

	// 分页
	alerts := am.GetAlerts(2, 0, nil)
	assert.Len(t, alerts, 2)

	alerts = am.GetAlerts(2, 2, nil)
	assert.LessOrEqual(t, len(alerts), 2)
}

func TestAlertingManager_GetAlertStats(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warn", Type: "cpu", Threshold: 70.0, Level: "warning", Enabled: true})
	am.AddRule(AlertRule{Name: "cpu-crit", Type: "cpu", Threshold: 90.0, Level: "critical", Enabled: true})

	am.CheckThreshold("cpu", 85.0, "source") // warning
	am.CheckThreshold("cpu", 95.0, "source") // critical

	stats := am.GetAlertStats()
	assert.GreaterOrEqual(t, stats["total"].(int), 2)
	assert.GreaterOrEqual(t, stats["warning"].(int), 1)
	assert.GreaterOrEqual(t, stats["critical"].(int), 1)
}

func TestAlertingManager_ClearAlerts(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warn", Type: "cpu", Threshold: 70.0, Level: "warning", Enabled: true})

	am.CheckThreshold("cpu", 90.0, "source")
	assert.GreaterOrEqual(t, len(am.GetAlerts(10, 0, nil)), 1)

	am.ClearAlerts()
	assert.Len(t, am.GetAlerts(10, 0, nil), 0)
}

// ========== 通知测试 ==========

func TestAlertingManager_ShouldNotify(t *testing.T) {
	tests := []struct {
		minLevel    string
		alertLevel  string
		shouldNotify bool
	}{
		{"warning", "warning", true},
		{"warning", "critical", true},
		{"critical", "warning", false},
		{"critical", "critical", true},
		{"info", "warning", true},
		{"warning", "info", false},
	}

	for _, tt := range tests {
		result := shouldNotify(tt.minLevel, tt.alertLevel)
		assert.Equal(t, tt.shouldNotify, result, "minLevel=%s, alertLevel=%s", tt.minLevel, tt.alertLevel)
	}
}

func TestAlertingManager_FormatWebhookPayload(t *testing.T) {
	am := NewAlertingManager()
	alert := &Alert{
		ID:        "alert-1",
		Type:      "cpu",
		Level:     "warning",
		Message:   "CPU usage high",
		Source:    "server-1",
		Timestamp: time.Now(),
	}

	payload := am.formatWebhookPayload(alert, map[string]interface{}{"value": 85.0})

	assert.Equal(t, "nasos.alert", payload["event"])
	assert.Equal(t, "alert-1", payload["alert_id"])
	assert.Equal(t, "warning", payload["level"])
	assert.Equal(t, "cpu", payload["type"])
}

func TestAlertingManager_FormatWeComPayload(t *testing.T) {
	am := NewAlertingManager()
	alert := &Alert{
		ID:        "alert-1",
		Type:      "memory",
		Level:     "critical",
		Message:   "Memory usage critical",
		Source:    "server-1",
		Timestamp: time.Now(),
	}

	payload := am.formatWeComPayload(alert)

	assert.Equal(t, "markdown", payload["msgtype"])
	markdown := payload["markdown"].(map[string]string)
	assert.Contains(t, markdown["content"], "critical")
	assert.Contains(t, markdown["content"], "Memory")
}

func TestAlertingManager_FormatEmailBody(t *testing.T) {
	am := NewAlertingManager()
	alert := &Alert{
		ID:        "alert-1",
		Type:      "disk",
		Level:     "warning",
		Message:   "Disk space low",
		Source:    "server-1",
		Timestamp: time.Now(),
	}

	body := am.formatEmailBody(alert, map[string]interface{}{"usage": 85.0})

	assert.Contains(t, body, "NAS-OS 系统告警")
	assert.Contains(t, body, "Disk space low")
	assert.Contains(t, body, "warning")
}

// ========== 邮件/Webhook 回调测试 ==========

func TestAlertingManager_SetSendEmailFunc(t *testing.T) {
	am := NewAlertingManager()
	called := false

	am.SetSendEmailFunc(func(to, subject, body string) error {
		called = true
		assert.Equal(t, "admin@example.com", to)
		assert.Contains(t, subject, "告警")
		return nil
	})

	assert.NotNil(t, am.sendEmailFunc)
	// 调用回调
	_ = am.sendEmailFunc("admin@example.com", "Test 告警", "body")
	assert.True(t, called)
}

func TestAlertingManager_SetSendWebhookFunc(t *testing.T) {
	am := NewAlertingManager()
	called := false

	am.SetSendWebhookFunc(func(url string, payload map[string]interface{}) error {
		called = true
		assert.Equal(t, "https://example.com/hook", url)
		return nil
	})

	assert.NotNil(t, am.sendWebhookFunc)
	_ = am.sendWebhookFunc("https://example.com/hook", map[string]interface{}{"test": true})
	assert.True(t, called)
}

// ========== 颜色测试 ==========

func TestGetLevelColor(t *testing.T) {
	assert.Equal(t, "#DC2626", getLevelColor("critical"))
	assert.Equal(t, "#F59E0B", getLevelColor("warning"))
	assert.Equal(t, "#6B7280", getLevelColor("info"))
	assert.Equal(t, "#6B7280", getLevelColor("unknown"))
}

// ========== 告警历史测试 ==========

func TestAlertingManager_GetAlertHistory(t *testing.T) {
	am := NewAlertingManager()
	am.AddRule(AlertRule{Name: "cpu-warn", Type: "cpu", Threshold: 70.0, Level: "warning", Enabled: true})

	am.CheckThreshold("cpu", 90.0, "source")
	alerts := am.GetAlerts(10, 0, nil)
	require.GreaterOrEqual(t, len(alerts), 1)

	_ = am.AcknowledgeAlert(alerts[0].ID, "admin")

	history := am.GetAlertHistory(10, 0)
	assert.GreaterOrEqual(t, len(history), 1)
}

func require.Len(t *testing.T, actual interface{}, expected int, msgAndArgs ...interface{}) {
	if !assert.Len(t, actual, expected, msgAndArgs...) {
		t.FailNow()
	}
}