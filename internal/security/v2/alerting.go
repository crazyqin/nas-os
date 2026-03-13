package securityv2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AlertingManager 告警通知管理器
type AlertingManager struct {
	config      AlertingConfig
	alerts      []*SecurityAlertV2
	subscribers []AlertSubscriber
	mu          sync.RWMutex
	// 通知发送函数
	sendEmailFunc   func(to, subject, body string) error
	sendWebhookFunc func(url string, payload map[string]interface{}) error
}

// AlertingConfig 告警配置
type AlertingConfig struct {
	Enabled         bool       `json:"enabled"`
	EmailEnabled    bool       `json:"email_enabled"`
	EmailRecipients []string   `json:"email_recipients"`
	WeComEnabled    bool       `json:"wecom_enabled"`
	WeComWebhook    string     `json:"wecom_webhook"`
	WebhookEnabled  bool       `json:"webhook_enabled"`
	WebhookURLs     []string   `json:"webhook_urls"`
	MinSeverity     string     `json:"min_severity"` // 最小告警级别（low, medium, high, critical）
	RateLimit       int        `json:"rate_limit"`   // 每分钟最大告警数
	QuietHours      QuietHours `json:"quiet_hours"`  // 免打扰时段
}

// QuietHours 免打扰时段配置
type QuietHours struct {
	Enabled   bool   `json:"enabled"`
	StartTime string `json:"start_time"` // HH:MM 格式
	EndTime   string `json:"end_time"`   // HH:MM 格式
}

// AlertSubscriber 告警订阅者
type AlertSubscriber struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"` // email, webhook, wecom
	Target   string   `json:"target"`
	Events   []string `json:"events"`   // 订阅的事件类型
	Severity string   `json:"severity"` // 最小告警级别
	Active   bool     `json:"active"`
}

// SecurityAlertV2 安全告警（v2 版本）
type SecurityAlertV2 struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Severity     string                 `json:"severity"` // low, medium, high, critical
	Type         string                 `json:"type"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	SourceIP     string                 `json:"source_ip,omitempty"`
	Username     string                 `json:"username,omitempty"`
	Resource     string                 `json:"resource,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Acknowledged bool                   `json:"acknowledged"`
	AckedBy      string                 `json:"acked_by,omitempty"`
	AckedAt      *time.Time             `json:"acked_at,omitempty"`
	Notified     bool                   `json:"notified"`
	NotifiedAt   *time.Time             `json:"notified_at,omitempty"`
}

// NewAlertingManager 创建告警管理器
func NewAlertingManager() *AlertingManager {
	return &AlertingManager{
		config: AlertingConfig{
			Enabled:         true,
			EmailEnabled:    false,
			EmailRecipients: []string{},
			WeComEnabled:    false,
			WebhookEnabled:  false,
			WebhookURLs:     []string{},
			MinSeverity:     "medium",
			RateLimit:       10,
			QuietHours: QuietHours{
				Enabled:   false,
				StartTime: "23:00",
				EndTime:   "07:00",
			},
		},
		alerts:      make([]*SecurityAlertV2, 0),
		subscribers: make([]AlertSubscriber, 0),
	}
}

// SetSendEmailFunc 设置邮件发送函数
func (am *AlertingManager) SetSendEmailFunc(fn func(to, subject, body string) error) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.sendEmailFunc = fn
}

// SetSendWebhookFunc 设置 Webhook 发送函数
func (am *AlertingManager) SetSendWebhookFunc(fn func(url string, payload map[string]interface{}) error) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.sendWebhookFunc = fn
}

// SendAlert 发送安全告警
func (am *AlertingManager) SendAlert(alert SecurityAlertV2) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.config.Enabled {
		return nil
	}

	// 检查告警级别
	if !am.shouldAlert(alert.Severity) {
		return nil
	}

	// 检查免打扰时段
	if am.config.QuietHours.Enabled && am.isQuietHours() {
		// 仅记录，不发送通知
		alert.Notified = false
		am.alerts = append(am.alerts, &alert)
		return nil
	}

	// 检查速率限制
	if !am.checkRateLimit() {
		return fmt.Errorf("达到告警速率限制")
	}

	// 添加告警到列表
	alert.Timestamp = time.Now()
	alert.Notified = true
	now := time.Now()
	alert.NotifiedAt = &now
	am.alerts = append(am.alerts, &alert)

	// 发送通知
	go am.sendNotifications(&alert)

	return nil
}

// shouldAlert 检查是否应该发送告警
func (am *AlertingManager) shouldAlert(severity string) bool {
	severityLevels := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	alertLevel, exists := severityLevels[severity]
	if !exists {
		return false
	}

	minLevel, exists := severityLevels[am.config.MinSeverity]
	if !exists {
		return true
	}

	return alertLevel >= minLevel
}

// isQuietHours 检查是否在免打扰时段
func (am *AlertingManager) isQuietHours() bool {
	now := time.Now()
	currentTime := now.Hour()*60 + now.Minute()

	startParts := parseTime(am.config.QuietHours.StartTime)
	endParts := parseTime(am.config.QuietHours.EndTime)

	startMinutes := startParts[0]*60 + startParts[1]
	endMinutes := endParts[0]*60 + endParts[1]

	if startMinutes <= endMinutes {
		// 同一天内
		return currentTime >= startMinutes && currentTime <= endMinutes
	} else {
		// 跨天（如 23:00 - 07:00）
		return currentTime >= startMinutes || currentTime <= endMinutes
	}
}

func parseTime(timeStr string) [2]int {
	var h, m int
	fmt.Sscanf(timeStr, "%d:%d", &h, &m)
	return [2]int{h, m}
}

// checkRateLimit 检查速率限制
func (am *AlertingManager) checkRateLimit() bool {
	now := time.Now()
	windowStart := now.Add(-time.Minute)

	count := 0
	for _, alert := range am.alerts {
		if alert.Timestamp.After(windowStart) {
			count++
		}
	}

	return count < am.config.RateLimit
}

// sendNotifications 发送通知到所有渠道
func (am *AlertingManager) sendNotifications(alert *SecurityAlertV2) {
	// 邮件通知
	if am.config.EmailEnabled && am.sendEmailFunc != nil {
		for _, recipient := range am.config.EmailRecipients {
			subject := fmt.Sprintf("【NAS-OS 安全告警】%s - %s", alert.Severity, alert.Title)
			body := am.formatEmailBody(alert)
			if err := am.sendEmailFunc(recipient, subject, body); err != nil {
				// 记录错误但不中断其他通知
				continue
			}
		}
	}

	// 企业微信通知
	if am.config.WeComEnabled && am.sendWebhookFunc != nil && am.config.WeComWebhook != "" {
		payload := am.formatWeComPayload(alert)
		if err := am.sendWebhookFunc(am.config.WeComWebhook, payload); err != nil {
			// 记录错误
		}
	}

	// 通用 Webhook 通知
	if am.config.WebhookEnabled && am.sendWebhookFunc != nil {
		for _, url := range am.config.WebhookURLs {
			payload := am.formatWebhookPayload(alert)
			if err := am.sendWebhookFunc(url, payload); err != nil {
				// 记录错误
			}
		}
	}
}

// formatEmailBody 格式化邮件正文
func (am *AlertingManager) formatEmailBody(alert *SecurityAlertV2) string {
	severityColor := map[string]string{
		"low":      "#28a745",
		"medium":   "#ffc107",
		"high":     "#fd7e14",
		"critical": "#dc3545",
	}

	color := severityColor[alert.Severity]
	if color == "" {
		color = "#6c757d"
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<style>
	body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
	.container { max-width: 600px; margin: 0 auto; padding: 20px; }
	.header { background: %s; color: white; padding: 15px; border-radius: 8px 8px 0 0; }
	.content { background: #f8f9fa; padding: 20px; border: 1px solid #e9ecef; border-top: none; border-radius: 0 0 8px 8px; }
	.label { font-weight: 600; color: #495057; }
	.value { margin-bottom: 10px; }
	.badge { display: inline-block; padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: 600; }
</style>
</head>
<body>
<div class="container">
	<div class="header">
		<h2 style="margin: 0;">🚨 NAS-OS 安全告警</h2>
	</div>
	<div class="content">
		<p><span class="label">告警级别：</span>
			<span class="badge" style="background: %s; color: white;">%s</span>
		</p>
		<p><span class="label">告警类型：</span> %s</p>
		<p><span class="label">标题：</span> %s</p>
		<p><span class="label">描述：</span> %s</p>
		%s
		<p><span class="label">发生时间：</span> %s</p>
		<hr style="border: none; border-top: 1px solid #dee2e6; margin: 20px 0;">
		<p style="color: #6c757d; font-size: 12px;">此邮件由 NAS-OS 安全系统自动发送，请勿回复。</p>
	</div>
</div>
</body>
</html>`,
		color,
		color,
		alert.Severity,
		alert.Type,
		alert.Title,
		alert.Description,
		am.formatDetailsHTML(alert.Details),
		alert.Timestamp.Format("2006-01-02 15:04:05"),
	)
}

func (am *AlertingManager) formatDetailsHTML(details map[string]interface{}) string {
	if len(details) == 0 {
		return ""
	}

	html := "<p><span class=\"label\">详细信息：</span></p><pre style=\"background: #fff; padding: 10px; border: 1px solid #dee2e6; border-radius: 4px; overflow-x: auto;\">"
	detailsJSON, _ := json.MarshalIndent(details, "", "  ")
	html += string(detailsJSON)
	html += "</pre>"
	return html
}

// formatWeComPayload 格式化企业微信消息
func (am *AlertingManager) formatWeComPayload(alert *SecurityAlertV2) map[string]interface{} {
	severityEmoji := map[string]string{
		"low":      "🟢",
		"medium":   "🟡",
		"high":     "🟠",
		"critical": "🔴",
	}

	emoji := severityEmoji[alert.Severity]
	if emoji == "" {
		emoji = "⚪"
	}

	content := fmt.Sprintf("%s **%s**\n", emoji, alert.Title)
	content += fmt.Sprintf("> 级别：%s\n", alert.Severity)
	content += fmt.Sprintf("> 类型：%s\n", alert.Type)
	content += fmt.Sprintf("> 描述：%s\n", alert.Description)

	if alert.SourceIP != "" {
		content += fmt.Sprintf("> 来源 IP：%s\n", alert.SourceIP)
	}
	if alert.Username != "" {
		content += fmt.Sprintf("> 用户：%s\n", alert.Username)
	}

	content += fmt.Sprintf("> 时间：%s", alert.Timestamp.Format("2006-01-02 15:04:05"))

	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
}

// formatWebhookPayload 格式化通用 Webhook 消息
func (am *AlertingManager) formatWebhookPayload(alert *SecurityAlertV2) map[string]interface{} {
	return map[string]interface{}{
		"event":       "security_alert",
		"alert_id":    alert.ID,
		"timestamp":   alert.Timestamp,
		"severity":    alert.Severity,
		"type":        alert.Type,
		"title":       alert.Title,
		"description": alert.Description,
		"source_ip":   alert.SourceIP,
		"username":    alert.Username,
		"details":     alert.Details,
	}
}

// AcknowledgeAlert 确认告警
func (am *AlertingManager) AcknowledgeAlert(alertID, ackedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for _, alert := range am.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			alert.AckedBy = ackedBy
			alert.AckedAt = &now
			return nil
		}
	}

	return fmt.Errorf("告警不存在")
}

// GetAlerts 获取告警列表
func (am *AlertingManager) GetAlerts(limit, offset int, filters map[string]string) []*SecurityAlertV2 {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*SecurityAlertV2, 0)

	for _, alert := range am.alerts {
		if !am.matchesFilters(alert, filters) {
			continue
		}
		result = append(result, alert)
	}

	// 按时间倒序
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	// 分页
	start := offset
	if start > len(result) {
		start = len(result)
	}
	end := start + limit
	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

func (am *AlertingManager) matchesFilters(alert *SecurityAlertV2, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "severity":
			if alert.Severity != value {
				return false
			}
		case "type":
			if alert.Type != value {
				return false
			}
		case "acknowledged":
			ack := value == "true"
			if alert.Acknowledged != ack {
				return false
			}
		case "notified":
			notified := value == "true"
			if alert.Notified != notified {
				return false
			}
		}
	}
	return true
}

// GetAlertStats 获取告警统计
func (am *AlertingManager) GetAlertStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	total := len(am.alerts)
	acknowledged := 0
	unacknowledged := 0
	notified := 0
	bySeverity := make(map[string]int)
	byType := make(map[string]int)

	for _, alert := range am.alerts {
		if alert.Acknowledged {
			acknowledged++
		} else {
			unacknowledged++
		}
		if alert.Notified {
			notified++
		}
		bySeverity[alert.Severity]++
		byType[alert.Type]++
	}

	return map[string]interface{}{
		"total":          total,
		"acknowledged":   acknowledged,
		"unacknowledged": unacknowledged,
		"notified":       notified,
		"by_severity":    bySeverity,
		"by_type":        byType,
	}
}

// AddSubscriber 添加告警订阅者
func (am *AlertingManager) AddSubscriber(subscriber AlertSubscriber) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	subscriber.Active = true
	am.subscribers = append(am.subscribers, subscriber)
	return nil
}

// RemoveSubscriber 移除告警订阅者
func (am *AlertingManager) RemoveSubscriber(subscriberID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i, sub := range am.subscribers {
		if sub.ID == subscriberID {
			am.subscribers = append(am.subscribers[:i], am.subscribers[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("订阅者不存在")
}

// GetSubscribers 获取所有订阅者
func (am *AlertingManager) GetSubscribers() []AlertSubscriber {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]AlertSubscriber, len(am.subscribers))
	copy(result, am.subscribers)
	return result
}

// GetConfig 获取告警配置
func (am *AlertingManager) GetConfig() AlertingConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.config
}

// UpdateConfig 更新告警配置
func (am *AlertingManager) UpdateConfig(config AlertingConfig) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.config = config
	return nil
}

// TestNotification 测试通知
func (am *AlertingManager) TestNotification(channel string) error {
	testAlert := SecurityAlertV2{
		ID:          "test-alert",
		Timestamp:   time.Now(),
		Severity:    "low",
		Type:        "test",
		Title:       "测试告警",
		Description: "这是一条测试告警消息，用于验证通知渠道是否正常工作。",
		Notified:    false,
	}

	switch channel {
	case "email":
		if am.sendEmailFunc != nil && len(am.config.EmailRecipients) > 0 {
			subject := "【NAS-OS】测试告警"
			body := am.formatEmailBody(&testAlert)
			return am.sendEmailFunc(am.config.EmailRecipients[0], subject, body)
		}
	case "wecom":
		if am.sendWebhookFunc != nil && am.config.WeComWebhook != "" {
			payload := am.formatWeComPayload(&testAlert)
			return am.sendWebhookFunc(am.config.WeComWebhook, payload)
		}
	case "webhook":
		if am.sendWebhookFunc != nil && len(am.config.WebhookURLs) > 0 {
			payload := am.formatWebhookPayload(&testAlert)
			return am.sendWebhookFunc(am.config.WebhookURLs[0], payload)
		}
	}

	return fmt.Errorf("无效的通知渠道")
}

// SendHTTPWebhook 发送 HTTP Webhook 通知
func SendHTTPWebhook(url string, payload map[string]interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回错误状态码：%d", resp.StatusCode)
	}

	return nil
}
