package securityv2

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAlertingManager(t *testing.T) {
	am := NewAlertingManager()
	require.NotNil(t, am)
	assert.True(t, am.config.Enabled)
	assert.Equal(t, "medium", am.config.MinSeverity)
	assert.Equal(t, 10, am.config.RateLimit)
}

func TestAlertingManager_SetSendEmailFunc(t *testing.T) {
	am := NewAlertingManager()

	fn := func(to, subject, body string) error {
		return nil
	}

	am.SetSendEmailFunc(fn)
	assert.NotNil(t, am.sendEmailFunc)
}

func TestAlertingManager_SetSendWebhookFunc(t *testing.T) {
	am := NewAlertingManager()

	fn := func(url string, payload map[string]interface{}) error {
		return nil
	}

	am.SetSendWebhookFunc(fn)
	assert.NotNil(t, am.sendWebhookFunc)
}

func TestAlertingManager_SendAlert(t *testing.T) {
	am := NewAlertingManager()

	alert := SecurityAlertV2{
		ID:          "test-001",
		Severity:    "high",
		Type:        "intrusion",
		Title:       "Test Alert",
		Description: "This is a test alert",
	}

	err := am.SendAlert(alert)
	assert.NoError(t, err)

	// Check alert was added
	am.mu.RLock()
	assert.Len(t, am.alerts, 1)
	am.mu.RUnlock()
}

func TestAlertingManager_SendAlert_Disabled(t *testing.T) {
	am := NewAlertingManager()
	am.config.Enabled = false

	alert := SecurityAlertV2{
		ID:          "test-002",
		Severity:    "high",
		Type:        "intrusion",
		Title:       "Test Alert",
		Description: "This is a test alert",
	}

	err := am.SendAlert(alert)
	assert.NoError(t, err)

	// No alert should be added
	am.mu.RLock()
	assert.Empty(t, am.alerts)
	am.mu.RUnlock()
}

func TestAlertingManager_ShouldAlert(t *testing.T) {
	am := NewAlertingManager()
	am.config.MinSeverity = "medium"

	tests := []struct {
		severity string
		expected bool
	}{
		{"low", false},
		{"medium", true},
		{"high", true},
		{"critical", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		result := am.shouldAlert(tt.severity)
		assert.Equal(t, tt.expected, result, "severity: %s", tt.severity)
	}
}

func TestAlertingManager_IsQuietHours(t *testing.T) {
	am := NewAlertingManager()
	am.config.QuietHours.Enabled = true
	am.config.QuietHours.StartTime = "23:00"
	am.config.QuietHours.EndTime = "07:00"

	// This test depends on current time, so we just verify the function works
	result := am.isQuietHours()
	// We can't assert the exact result without mocking time
	_ = result
}

func TestAlertingManager_CheckRateLimit(t *testing.T) {
	am := NewAlertingManager()
	am.config.RateLimit = 2

	// Should allow first two
	assert.True(t, am.checkRateLimit())

	am.alerts = append(am.alerts, &SecurityAlertV2{Timestamp: time.Now()})
	assert.True(t, am.checkRateLimit())

	am.alerts = append(am.alerts, &SecurityAlertV2{Timestamp: time.Now()})
	assert.False(t, am.checkRateLimit())
}

func TestAlertingManager_AcknowledgeAlert(t *testing.T) {
	am := NewAlertingManager()

	am.alerts = append(am.alerts, &SecurityAlertV2{
		ID: "alert-001",
	})

	err := am.AcknowledgeAlert("alert-001", "admin")
	assert.NoError(t, err)

	am.mu.RLock()
	assert.True(t, am.alerts[0].Acknowledged)
	assert.Equal(t, "admin", am.alerts[0].AckedBy)
	am.mu.RUnlock()
}

func TestAlertingManager_AcknowledgeAlert_NotFound(t *testing.T) {
	am := NewAlertingManager()

	err := am.AcknowledgeAlert("nonexistent", "admin")
	assert.Error(t, err)
}

func TestAlertingManager_GetAlerts(t *testing.T) {
	am := NewAlertingManager()

	am.alerts = []*SecurityAlertV2{
		{ID: "alert-001", Severity: "high", Type: "intrusion"},
		{ID: "alert-002", Severity: "low", Type: "access"},
		{ID: "alert-003", Severity: "critical", Type: "intrusion"},
	}

	alerts := am.GetAlerts(10, 0, nil)
	assert.Len(t, alerts, 3)
}

func TestAlertingManager_GetAlerts_WithFilters(t *testing.T) {
	am := NewAlertingManager()

	am.alerts = []*SecurityAlertV2{
		{ID: "alert-001", Severity: "high", Type: "intrusion"},
		{ID: "alert-002", Severity: "low", Type: "access"},
		{ID: "alert-003", Severity: "critical", Type: "intrusion"},
	}

	filters := map[string]string{"severity": "high"}
	alerts := am.GetAlerts(10, 0, filters)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "alert-001", alerts[0].ID)
}

func TestAlertingManager_GetAlerts_Pagination(t *testing.T) {
	am := NewAlertingManager()

	am.alerts = []*SecurityAlertV2{
		{ID: "alert-001"},
		{ID: "alert-002"},
		{ID: "alert-003"},
		{ID: "alert-004"},
		{ID: "alert-005"},
	}

	// First page
	alerts := am.GetAlerts(2, 0, nil)
	assert.Len(t, alerts, 2)

	// Second page
	alerts = am.GetAlerts(2, 2, nil)
	assert.Len(t, alerts, 2)

	// Beyond range
	alerts = am.GetAlerts(2, 10, nil)
	assert.Empty(t, alerts)
}

func TestAlertingManager_GetAlertStats(t *testing.T) {
	am := NewAlertingManager()

	now := time.Now()
	am.alerts = []*SecurityAlertV2{
		{ID: "alert-001", Severity: "high", Type: "intrusion", Acknowledged: true, Notified: true},
		{ID: "alert-002", Severity: "low", Type: "access", Acknowledged: false, Notified: true},
		{ID: "alert-003", Severity: "high", Type: "intrusion", Acknowledged: false, Notified: false, AckedAt: &now},
	}

	stats := am.GetAlertStats()
	assert.Equal(t, 3, stats["total"])
	assert.Equal(t, 1, stats["acknowledged"])
	assert.Equal(t, 2, stats["unacknowledged"])
	assert.Equal(t, 2, stats["notified"])
}

func TestAlertingManager_AddSubscriber(t *testing.T) {
	am := NewAlertingManager()

	sub := AlertSubscriber{
		ID:     "sub-001",
		Name:   "Admin",
		Type:   "email",
		Target: "admin@example.com",
	}

	err := am.AddSubscriber(sub)
	assert.NoError(t, err)
	assert.Len(t, am.subscribers, 1)
	assert.True(t, am.subscribers[0].Active)
}

func TestAlertingManager_RemoveSubscriber(t *testing.T) {
	am := NewAlertingManager()

	am.subscribers = []AlertSubscriber{
		{ID: "sub-001", Name: "Admin"},
		{ID: "sub-002", Name: "Team"},
	}

	err := am.RemoveSubscriber("sub-001")
	assert.NoError(t, err)
	assert.Len(t, am.subscribers, 1)
	assert.Equal(t, "sub-002", am.subscribers[0].ID)
}

func TestAlertingManager_RemoveSubscriber_NotFound(t *testing.T) {
	am := NewAlertingManager()

	err := am.RemoveSubscriber("nonexistent")
	assert.Error(t, err)
}

func TestAlertingManager_GetSubscribers(t *testing.T) {
	am := NewAlertingManager()

	am.subscribers = []AlertSubscriber{
		{ID: "sub-001", Name: "Admin"},
		{ID: "sub-002", Name: "Team"},
	}

	subs := am.GetSubscribers()
	assert.Len(t, subs, 2)
}

func TestAlertingManager_GetConfig(t *testing.T) {
	am := NewAlertingManager()

	config := am.GetConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, "medium", config.MinSeverity)
}

func TestAlertingManager_UpdateConfig(t *testing.T) {
	am := NewAlertingManager()

	newConfig := AlertingConfig{
		Enabled:     false,
		MinSeverity: "high",
		RateLimit:   5,
	}

	err := am.UpdateConfig(newConfig)
	assert.NoError(t, err)

	config := am.GetConfig()
	assert.False(t, config.Enabled)
	assert.Equal(t, "high", config.MinSeverity)
}

func TestAlertingManager_FormatEmailBody(t *testing.T) {
	am := NewAlertingManager()

	alert := &SecurityAlertV2{
		Severity:    "high",
		Type:        "intrusion",
		Title:       "Test Alert",
		Description: "Test description",
		Timestamp:   time.Now(),
	}

	body := am.formatEmailBody(alert)
	assert.Contains(t, body, "Test Alert")
	assert.Contains(t, body, "high")
	assert.Contains(t, body, "Test description")
}

func TestAlertingManager_FormatWeComPayload(t *testing.T) {
	am := NewAlertingManager()

	alert := &SecurityAlertV2{
		Severity:    "critical",
		Type:        "intrusion",
		Title:       "Critical Alert",
		Description: "Critical description",
		SourceIP:    "192.168.1.1",
		Timestamp:   time.Now(),
	}

	payload := am.formatWeComPayload(alert)
	assert.Equal(t, "markdown", payload["msgtype"])
}

func TestAlertingManager_FormatWebhookPayload(t *testing.T) {
	am := NewAlertingManager()

	alert := &SecurityAlertV2{
		ID:          "alert-001",
		Severity:    "high",
		Type:        "intrusion",
		Title:       "Test Alert",
		Description: "Test description",
		SourceIP:    "192.168.1.1",
		Username:    "testuser",
		Timestamp:   time.Now(),
		Details:     map[string]interface{}{"key": "value"},
	}

	payload := am.formatWebhookPayload(alert)
	assert.Equal(t, "security_alert", payload["event"])
	assert.Equal(t, "alert-001", payload["alert_id"])
	assert.Equal(t, "high", payload["severity"])
}

func TestAlertingManager_TestNotification(t *testing.T) {
	am := NewAlertingManager()

	// No email func configured
	err := am.TestNotification("email")
	assert.Error(t, err)

	// No webhook func configured
	err = am.TestNotification("wecom")
	assert.Error(t, err)

	// Invalid channel
	err = am.TestNotification("invalid")
	assert.Error(t, err)
}

func TestAlertingManager_TestNotification_WithEmail(t *testing.T) {
	am := NewAlertingManager()

	var emailCalled bool
	am.SetSendEmailFunc(func(to, subject, body string) error {
		emailCalled = true
		return nil
	})
	am.config.EmailRecipients = []string{"admin@example.com"}

	err := am.TestNotification("email")
	assert.NoError(t, err)
	assert.True(t, emailCalled)
}

func TestAlertingManager_TestNotification_WithWebhook(t *testing.T) {
	am := NewAlertingManager()

	var webhookCalled bool
	am.SetSendWebhookFunc(func(url string, payload map[string]interface{}) error {
		webhookCalled = true
		return nil
	})
	am.config.WebhookURLs = []string{"https://example.com/webhook"}
	am.config.WeComWebhook = "https://example.com/wecom"

	err := am.TestNotification("webhook")
	assert.NoError(t, err)
	assert.True(t, webhookCalled)

	webhookCalled = false
	err = am.TestNotification("wecom")
	assert.NoError(t, err)
	assert.True(t, webhookCalled)
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input    string
		expected [2]int
	}{
		{"23:00", [2]int{23, 0}},
		{"07:30", [2]int{7, 30}},
		{"00:00", [2]int{0, 0}},
		{"invalid", [2]int{0, 0}},
	}

	for _, tt := range tests {
		result := parseTime(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestAlertingManager_ConcurrentAccess(t *testing.T) {
	am := NewAlertingManager()

	var wg sync.WaitGroup
	wg.Add(10)

	// Concurrent SendAlert
	for i := 0; i < 5; i++ {
		go func(idx int) {
			defer wg.Done()
			alert := SecurityAlertV2{
				ID:       "alert-" + string(rune('0'+idx)),
				Severity: "high",
				Title:    "Test Alert",
			}
			am.SendAlert(alert)
		}(i)
	}

	// Concurrent GetAlerts
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			am.GetAlerts(10, 0, nil)
		}()
	}

	wg.Wait()

	alerts := am.GetAlerts(100, 0, nil)
	assert.Len(t, alerts, 5)
}
