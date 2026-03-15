package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AlertingManager 告警管理器
type AlertingManager struct {
	mu           sync.RWMutex
	alerts       []*Alert
	rules        []AlertRule
	subscribers  []AlertSubscriber
	alertHistory []*AlertHistoryEntry
	maxAlerts    int
	maxHistory   int
	// 通知回调
	sendEmailFunc   func(to, subject, body string) error
	sendWebhookFunc func(url string, payload map[string]interface{}) error
}

// AlertSubscriber 告警订阅者
type AlertSubscriber struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // email, webhook, wecom
	Target    string    `json:"target"`
	MinLevel  string    `json:"min_level"` // warning, critical
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// AlertHistoryEntry 告警历史
type AlertHistoryEntry struct {
	AlertID   string    `json:"alert_id"`
	Action    string    `json:"action"` // triggered, acknowledged, resolved
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user,omitempty"`
}

// NewAlertingManager 创建告警管理器
func NewAlertingManager() *AlertingManager {
	return &AlertingManager{
		alerts:       make([]*Alert, 0),
		rules:        make([]AlertRule, 0),
		subscribers:  make([]AlertSubscriber, 0),
		alertHistory: make([]*AlertHistoryEntry, 0),
		maxAlerts:    100,
		maxHistory:   1000,
	}
}

// SetSendEmailFunc 设置邮件发送函数
func (am *AlertingManager) SetSendEmailFunc(fn func(to, subject, body string) error) {
	am.sendEmailFunc = fn
}

// SetSendWebhookFunc 设置 Webhook 发送函数
func (am *AlertingManager) SetSendWebhookFunc(fn func(url string, payload map[string]interface{}) error) {
	am.sendWebhookFunc = fn
}

// CheckThreshold 检查阈值并触发告警
func (am *AlertingManager) CheckThreshold(metricType string, value float64, source string) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	for _, rule := range am.rules {
		if !rule.Enabled || rule.Type != metricType {
			continue
		}

		if value >= rule.Threshold {
			level := rule.Level
			message := fmt.Sprintf("%s 使用率 %.1f%% 超过阈值 %.1f%%",
				metricType, value, rule.Threshold)

			am.triggerAlert(metricType, level, message, source, map[string]interface{}{
				"current_value": value,
				"threshold":     rule.Threshold,
				"rule_name":     rule.Name,
			})
		}
	}
}

// triggerAlert 触发告警
func (am *AlertingManager) triggerAlert(alertType, level, message, source string, details map[string]interface{}) {
	alert := &Alert{
		ID:        generateAlertID(),
		Type:      alertType,
		Level:     level,
		Message:   message,
		Source:    source,
		Timestamp: time.Now(),
	}

	// 添加到告警列表
	am.alerts = append(am.alerts, alert)

	// 限制告警数量
	if len(am.alerts) > am.maxAlerts {
		am.alerts = am.alerts[1:]
	}

	// 记录历史
	am.alertHistory = append(am.alertHistory, &AlertHistoryEntry{
		AlertID:   alert.ID,
		Action:    "triggered",
		Timestamp: time.Now(),
	})

	if len(am.alertHistory) > am.maxHistory {
		am.alertHistory = am.alertHistory[1:]
	}

	// 发送通知
	go am.sendNotifications(alert, details)
}

// sendNotifications 发送告警通知
func (am *AlertingManager) sendNotifications(alert *Alert, details map[string]interface{}) {
	am.mu.RLock()
	subscribers := make([]AlertSubscriber, len(am.subscribers))
	copy(subscribers, am.subscribers)
	am.mu.RUnlock()

	for _, sub := range subscribers {
		if !sub.Enabled {
			continue
		}

		// 检查告警级别
		if !shouldNotify(sub.MinLevel, alert.Level) {
			continue
		}

		switch sub.Type {
		case "email":
			if am.sendEmailFunc != nil {
				subject := fmt.Sprintf("【NAS-OS 告警】%s - %s", alert.Level, alert.Message)
				body := am.formatEmailBody(alert, details)
				_ = am.sendEmailFunc(sub.Target, subject, body)
			}
		case "webhook":
			if am.sendWebhookFunc != nil {
				payload := am.formatWebhookPayload(alert, details)
				_ = am.sendWebhookFunc(sub.Target, payload)
			}
		case "wecom":
			if am.sendWebhookFunc != nil {
				payload := am.formatWeComPayload(alert)
				_ = am.sendWebhookFunc(sub.Target, payload)
			}
		}
	}
}

// shouldNotify 检查是否应该通知
func shouldNotify(minLevel, alertLevel string) bool {
	levelPriority := map[string]int{
		"info":     0,
		"warning":  1,
		"critical": 2,
	}

	alertPriority, ok := levelPriority[alertLevel]
	if !ok {
		return false
	}

	minPriority, ok := levelPriority[minLevel]
	if !ok {
		return true
	}

	return alertPriority >= minPriority
}

// formatEmailBody 格式化邮件正文
func (am *AlertingManager) formatEmailBody(alert *Alert, details map[string]interface{}) string {
	detailsJSON, _ := json.MarshalIndent(details, "", "  ")

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<style>
	body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
	.container { max-width: 600px; margin: 0 auto; padding: 20px; }
	.header { background: %s; color: white; padding: 15px; border-radius: 8px 8px 0 0; }
	.content { background: #f8f9fa; padding: 20px; border: 1px solid #e9ecef; }
	.badge { display: inline-block; padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: 600; }
</style>
</head>
<body>
<div class="container">
	<div class="header" style="background: %s;">
		<h2 style="margin: 0;">🚨 NAS-OS 系统告警</h2>
	</div>
	<div class="content">
		<p><strong>告警级别：</strong> <span class="badge" style="background: %s; color: white;">%s</span></p>
		<p><strong>告警类型：</strong> %s</p>
		<p><strong>告警内容：</strong> %s</p>
		<p><strong>来源：</strong> %s</p>
		<p><strong>时间：</strong> %s</p>
		<hr>
		<h4>详细信息：</h4>
		<pre style="background: white; padding: 10px; border-radius: 4px; overflow-x: auto;">%s</pre>
	</div>
</div>
</body>
</html>`,
		getLevelColor(alert.Level),
		getLevelColor(alert.Level),
		getLevelColor(alert.Level),
		alert.Level,
		alert.Type,
		alert.Message,
		alert.Source,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		string(detailsJSON),
	)
}

// formatWebhookPayload 格式化 Webhook 消息
func (am *AlertingManager) formatWebhookPayload(alert *Alert, details map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"event":     "nasos.alert",
		"alert_id":  alert.ID,
		"timestamp": alert.Timestamp,
		"level":     alert.Level,
		"type":      alert.Type,
		"message":   alert.Message,
		"source":    alert.Source,
		"details":   details,
	}
}

// formatWeComPayload 格式化企业微信消息
func (am *AlertingManager) formatWeComPayload(alert *Alert) map[string]interface{} {
	emoji := map[string]string{
		"info":     "🔵",
		"warning":  "🟡",
		"critical": "🔴",
	}

	content := fmt.Sprintf("%s **%s 告警**\n", emoji[alert.Level], alert.Level)
	content += fmt.Sprintf("> 类型：%s\n", alert.Type)
	content += fmt.Sprintf("> 内容：%s\n", alert.Message)
	content += fmt.Sprintf("> 来源：%s\n", alert.Source)
	content += fmt.Sprintf("> 时间：%s", alert.Timestamp.Format("2006-01-02 15:04:05"))

	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
}

// AcknowledgeAlert 确认告警
func (am *AlertingManager) AcknowledgeAlert(alertID, user string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true

			am.alertHistory = append(am.alertHistory, &AlertHistoryEntry{
				AlertID:   alertID,
				Action:    "acknowledged",
				Timestamp: time.Now(),
				User:      user,
			})

			return nil
		}
	}

	return fmt.Errorf("告警不存在")
}

// ResolveAlert 解决告警
func (am *AlertingManager) ResolveAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i, alert := range am.alerts {
		if alert.ID == alertID {
			// 移动到历史
			am.alertHistory = append(am.alertHistory, &AlertHistoryEntry{
				AlertID:   alertID,
				Action:    "resolved",
				Timestamp: time.Now(),
			})

			// 从活动列表移除
			am.alerts = append(am.alerts[:i], am.alerts[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("告警不存在")
}

// GetAlerts 获取告警列表
func (am *AlertingManager) GetAlerts(limit, offset int, filters map[string]string) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Alert, 0)

	for _, alert := range am.alerts {
		if !matchesFilters(alert, filters) {
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

// GetActiveAlerts 获取活跃告警列表 (v2.59.0)
func (am *AlertingManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Alert, 0)
	for _, alert := range am.alerts {
		if !alert.Acknowledged {
			result = append(result, alert)
		}
	}
	return result
}

func matchesFilters(alert *Alert, filters map[string]string) bool {
	for key, value := range filters {
		switch key {
		case "level":
			if alert.Level != value {
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
		}
	}
	return true
}

// GetAlertHistory 获取告警历史
func (am *AlertingManager) GetAlertHistory(limit, offset int) []*AlertHistoryEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	start := len(am.alertHistory) - limit - offset
	if start < 0 {
		start = 0
	}
	end := len(am.alertHistory) - offset
	if end > len(am.alertHistory) {
		end = len(am.alertHistory)
	}

	result := make([]*AlertHistoryEntry, end-start)
	copy(result, am.alertHistory[start:end])

	// 反转
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetAlertStats 获取告警统计
func (am *AlertingManager) GetAlertStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	total := len(am.alerts)
	acknowledged := 0
	critical := 0
	warning := 0

	for _, alert := range am.alerts {
		if alert.Acknowledged {
			acknowledged++
		}
		switch alert.Level {
		case "critical":
			critical++
		case "warning":
			warning++
		}
	}

	return map[string]interface{}{
		"total":          total,
		"acknowledged":   acknowledged,
		"unacknowledged": total - acknowledged,
		"critical":       critical,
		"warning":        warning,
		"history_size":   len(am.alertHistory),
	}
}

// AddRule 添加告警规则
func (am *AlertingManager) AddRule(rule AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

// GetRules 获取所有规则
func (am *AlertingManager) GetRules() []AlertRule {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]AlertRule, len(am.rules))
	copy(result, am.rules)
	return result
}

// AddSubscriber 添加订阅者
func (am *AlertingManager) AddSubscriber(sub AlertSubscriber) {
	am.mu.Lock()
	defer am.mu.Unlock()
	sub.CreatedAt = time.Now()
	am.subscribers = append(am.subscribers, sub)
}

// GetSubscribers 获取订阅者列表
func (am *AlertingManager) GetSubscribers() []AlertSubscriber {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]AlertSubscriber, len(am.subscribers))
	copy(result, am.subscribers)
	return result
}

// ClearAlerts 清除所有告警
func (am *AlertingManager) ClearAlerts() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.alerts = make([]*Alert, 0)
}

// generateAlertID 生成告警 ID
func generateAlertID() string {
	return fmt.Sprintf("alert-%d", time.Now().UnixNano())
}

// getLevelColor 获取告警级别颜色
func getLevelColor(level string) string {
	switch level {
	case "critical":
		return "#DC2626"
	case "warning":
		return "#F59E0B"
	default:
		return "#6B7280"
	}
}

// SendHTTPWebhook 发送 HTTP Webhook
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回错误状态码：%d", resp.StatusCode)
	}

	return nil
}
