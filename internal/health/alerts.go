// Package health 提供系统健康检查功能
// alerts.go - 告警管理器 v2.51.0
package health

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"sync"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertState 告警状态
type AlertState string

const (
	AlertStateFiring   AlertState = "firing"
	AlertStateResolved AlertState = "resolved"
)

// Alert 告警信息
type Alert struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	CheckName   string                 `json:"check_name"`
	CheckType   CheckType              `json:"check_type"`
	Severity    AlertSeverity          `json:"severity"`
	State       AlertState             `json:"state"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Annotations map[string]string      `json:"annotations,omitempty"`
	StartsAt    time.Time              `json:"starts_at"`
	EndsAt      time.Time              `json:"ends_at,omitempty"`
	FiredAt     time.Time              `json:"fired_at"`
	ResolvedAt  time.Time              `json:"resolved_at,omitempty"`
	LastSentAt  time.Time              `json:"last_sent_at,omitempty"`
	RepeatCount int                    `json:"repeat_count"`
	Silenced    bool                   `json:"silenced"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name           string            `json:"name"`
	CheckName      string            `json:"check_name"`
	Condition      AlertCondition    `json:"condition"`
	Severity       AlertSeverity     `json:"severity"`
	Duration       time.Duration     `json:"duration"`        // 持续多久后触发
	RepeatInterval time.Duration     `json:"repeat_interval"` // 重复告警间隔
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Enabled        bool              `json:"enabled"`
}

// AlertCondition 告警条件
type AlertCondition struct {
	Type      string  `json:"type"`      // threshold, status
	Threshold float64 `json:"threshold"` // 阈值
	Operator  string  `json:"operator"`  // gt, gte, lt, lte, eq
	Status    string  `json:"status"`    // unhealthy, degraded
}

// NotificationChannel 通知渠道配置
type NotificationChannel struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"` // email, webhook, dingtalk
	Config   map[string]interface{} `json:"config"`
	Enabled  bool                   `json:"enabled"`
	Severity []AlertSeverity        `json:"severity"` // 接收的严重级别
}

// SilenceConfig 静默配置
type SilenceConfig struct {
	ID        string            `json:"id"`
	Matchers  map[string]string `json:"matchers"` // 匹配条件
	StartsAt  time.Time         `json:"starts_at"`
	EndsAt    time.Time         `json:"ends_at"`
	Reason    string            `json:"reason"`
	CreatedBy string            `json:"created_by"`
}

// AlertManagerConfig 告警管理器配置
type AlertManagerConfig struct {
	DefaultRepeatInterval time.Duration `json:"default_repeat_interval"`
	DefaultDuration       time.Duration `json:"default_duration"`
	MaxAlerts             int           `json:"max_alerts"`
	RetentionPeriod       time.Duration `json:"retention_period"`
}

// AlertManager 告警管理器
type AlertManager struct {
	mu          sync.RWMutex
	alerts      map[string]*Alert
	rules       map[string]*AlertRule
	channels    map[string]*NotificationChannel
	silences    map[string]*SilenceConfig
	config      AlertManagerConfig
	logger      *zap.Logger
	healthCheck *HealthChecker

	// 通知发送队列
	notifyQueue chan *Alert
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewAlertManager 创建告警管理器
func NewAlertManager(healthCheck *HealthChecker, opts ...AlertManagerOption) *AlertManager {
	am := &AlertManager{
		alerts:   make(map[string]*Alert),
		rules:    make(map[string]*AlertRule),
		channels: make(map[string]*NotificationChannel),
		silences: make(map[string]*SilenceConfig),
		config: AlertManagerConfig{
			DefaultRepeatInterval: 5 * time.Minute,
			DefaultDuration:       1 * time.Minute,
			MaxAlerts:             1000,
			RetentionPeriod:       24 * time.Hour,
		},
		healthCheck: healthCheck,
		notifyQueue: make(chan *Alert, 100),
		stopCh:      make(chan struct{}),
	}

	for _, opt := range opts {
		opt(am)
	}

	// 启动通知发送协程
	am.wg.Add(1)
	go am.processNotifications()

	return am
}

// AlertManagerOption 告警管理器选项
type AlertManagerOption func(*AlertManager)

// WithAlertLogger 设置日志器
func WithAlertLogger(logger *zap.Logger) AlertManagerOption {
	return func(am *AlertManager) {
		am.logger = logger
	}
}

// WithAlertConfig 设置配置
func WithAlertConfig(config AlertManagerConfig) AlertManagerOption {
	return func(am *AlertManager) {
		am.config = config
	}
}

// AddRule 添加告警规则
func (am *AlertManager) AddRule(rule AlertRule) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if rule.Duration == 0 {
		rule.Duration = am.config.DefaultDuration
	}
	if rule.RepeatInterval == 0 {
		rule.RepeatInterval = am.config.DefaultRepeatInterval
	}
	if rule.Enabled {
		rule.Enabled = true
	}

	am.rules[rule.Name] = &rule
	return nil
}

// RemoveRule 移除告警规则
func (am *AlertManager) RemoveRule(name string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.rules, name)
}

// AddChannel 添加通知渠道
func (am *AlertManager) AddChannel(channel NotificationChannel) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.channels[channel.ID] = &channel
}

// RemoveChannel 移除通知渠道
func (am *AlertManager) RemoveChannel(id string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.channels, id)
}

// Evaluate 评估健康检查结果并触发告警
func (am *AlertManager) Evaluate(ctx context.Context, report *HealthReport) {
	am.mu.RLock()
	rules := make([]*AlertRule, 0, len(am.rules))
	for _, r := range am.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	am.mu.RUnlock()

	for _, rule := range rules {
		result, exists := report.Checks[rule.CheckName]
		if !exists {
			continue
		}

		am.evaluateRule(ctx, rule, result)
	}
}

// evaluateRule 评估单个规则
func (am *AlertManager) evaluateRule(ctx context.Context, rule *AlertRule, result CheckResult) {
	am.mu.Lock()
	defer am.mu.Unlock()

	shouldFire := am.checkCondition(&rule.Condition, result)

	alertKey := fmt.Sprintf("%s:%s", rule.CheckName, rule.Name)

	if shouldFire {
		// 检查是否已有告警
		if alert, exists := am.alerts[alertKey]; exists {
			// 更新现有告警
			alert.RepeatCount++
			alert.Details = result.Details
			alert.Message = result.Message

			// 检查是否需要重复发送
			if time.Since(alert.LastSentAt) >= rule.RepeatInterval && !alert.Silenced {
				am.notifyQueue <- alert
				alert.LastSentAt = time.Now()
			}
		} else {
			// 创建新告警
			alert := &Alert{
				ID:          generateAlertID(),
				Name:        rule.Name,
				CheckName:   rule.CheckName,
				CheckType:   result.Type,
				Severity:    rule.Severity,
				State:       AlertStateFiring,
				Message:     result.Message,
				Details:     result.Details,
				Labels:      rule.Labels,
				Annotations: rule.Annotations,
				StartsAt:    time.Now(),
				FiredAt:     time.Now(),
			}

			am.alerts[alertKey] = alert

			// 检查静默
			if !am.isSilenced(alert) {
				am.notifyQueue <- alert
				alert.LastSentAt = time.Now()
			}
		}
	} else {
		// 条件不满足，解决告警
		if alert, exists := am.alerts[alertKey]; exists && alert.State == AlertStateFiring {
			alert.State = AlertStateResolved
			alert.EndsAt = time.Now()
			alert.ResolvedAt = time.Now()
			alert.Message = fmt.Sprintf("Alert resolved: %s", alert.Message)

			if !alert.Silenced {
				am.notifyQueue <- alert
			}
		}
	}
}

// checkCondition 检查告警条件
func (am *AlertManager) checkCondition(condition *AlertCondition, result CheckResult) bool {
	switch condition.Type {
	case "status":
		return string(result.Status) == condition.Status
	case "threshold":
		if value, ok := result.Details["used_percent"].(float64); ok {
			return compareThreshold(value, condition.Threshold, condition.Operator)
		}
	}
	return false
}

// compareThreshold 比较阈值
func compareThreshold(value, threshold float64, operator string) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

// isSilenced 检查告警是否被静默
func (am *AlertManager) isSilenced(alert *Alert) bool {
	now := time.Now()
	for _, silence := range am.silences {
		if now.Before(silence.StartsAt) || now.After(silence.EndsAt) {
			continue
		}

		// 检查匹配条件
		matched := true
		for key, value := range silence.Matchers {
			if alert.Labels[key] != value {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// CreateSilence 创建静默
func (am *AlertManager) CreateSilence(config SilenceConfig) string {
	am.mu.Lock()
	defer am.mu.Unlock()

	if config.ID == "" {
		config.ID = generateSilenceID()
	}

	am.silences[config.ID] = &config
	return config.ID
}

// DeleteSilence 删除静默
func (am *AlertManager) DeleteSilence(id string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.silences, id)
}

// GetSilences 获取所有静默
func (am *AlertManager) GetSilences() []*SilenceConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*SilenceConfig, 0, len(am.silences))
	for _, s := range am.silences {
		result = append(result, s)
	}
	return result
}

// GetAlerts 获取所有告警
func (am *AlertManager) GetAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Alert, 0, len(am.alerts))
	for _, a := range am.alerts {
		result = append(result, a)
	}
	return result
}

// GetActiveAlerts 获取活跃告警
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*Alert, 0)
	for _, a := range am.alerts {
		if a.State == AlertStateFiring {
			result = append(result, a)
		}
	}
	return result
}

// processNotifications 处理通知发送队列
func (am *AlertManager) processNotifications() {
	defer am.wg.Done()

	for {
		select {
		case <-am.stopCh:
			return
		case alert := <-am.notifyQueue:
			am.sendNotification(alert)
		}
	}
}

// sendNotification 发送通知
func (am *AlertManager) sendNotification(alert *Alert) {
	am.mu.RLock()
	channels := make([]*NotificationChannel, 0)
	for _, ch := range am.channels {
		if !ch.Enabled {
			continue
		}
		// 检查严重级别是否匹配
		for _, sev := range ch.Severity {
			if sev == alert.Severity {
				channels = append(channels, ch)
				break
			}
		}
	}
	am.mu.RUnlock()

	for _, channel := range channels {
		var err error
		switch channel.Type {
		case "email":
			err = am.sendEmail(channel, alert)
		case "webhook":
			err = am.sendWebhook(channel, alert)
		case "dingtalk":
			err = am.sendDingTalk(channel, alert)
		}

		if err != nil && am.logger != nil {
			am.logger.Error("Failed to send notification",
				zap.String("channel", channel.Name),
				zap.String("type", channel.Type),
				zap.Error(err),
			)
		}
	}
}

// sendEmail 发送邮件通知
func (am *AlertManager) sendEmail(channel *NotificationChannel, alert *Alert) error {
	config := channel.Config

	smtpHost, _ := config["smtp_host"].(string)
	smtpPort, _ := config["smtp_port"].(string)
	smtpUser, _ := config["smtp_user"].(string)
	smtpPass, _ := config["smtp_pass"].(string)
	from, _ := config["from"].(string)
	to, _ := config["to"].(string)

	if smtpHost == "" || to == "" {
		return fmt.Errorf("email config incomplete")
	}

	subject := fmt.Sprintf("[%s] %s - %s", alert.Severity, alert.Name, alert.State)
	body := am.formatAlertEmail(alert)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// formatAlertEmail 格式化邮件内容
func (am *AlertManager) formatAlertEmail(alert *Alert) string {
	tmpl := `健康检查告警通知

告警名称: {{.Name}}
检查项: {{.CheckName}}
严重级别: {{.Severity}}
状态: {{.State}}
消息: {{.Message}}
触发时间: {{.FiredAt.Format "2006-01-02 15:04:05"}}

{{if .Details}}详细信息:
{{range $k, $v := .Details}}  {{$k}}: {{$v}}
{{end}}{{end}}
`

	t, _ := template.New("alert").Parse(tmpl)
	var buf bytes.Buffer
	t.Execute(&buf, alert)
	return buf.String()
}

// sendWebhook 发送 Webhook 通知
func (am *AlertManager) sendWebhook(channel *NotificationChannel, alert *Alert) error {
	url, _ := channel.Config["url"].(string)
	if url == "" {
		return fmt.Errorf("webhook url is empty")
	}

	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 添加自定义头部
	if headers, ok := channel.Config["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}

	// 跳过 TLS 验证（如果配置）
	if skipTLS, _ := channel.Config["skip_tls_verify"].(bool); skipTLS {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// DingTalkMessage 钉钉消息结构
type DingTalkMessage struct {
	MsgType  string            `json:"msgtype"`
	Markdown *DingTalkMarkdown `json:"markdown,omitempty"`
	Text     *DingTalkText     `json:"text,omitempty"`
}

// DingTalkMarkdown 钉钉 Markdown 消息
type DingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// DingTalkText 钉钉文本消息
type DingTalkText struct {
	Content string `json:"content"`
}

// sendDingTalk 发送钉钉通知
func (am *AlertManager) sendDingTalk(channel *NotificationChannel, alert *Alert) error {
	webhook, _ := channel.Config["webhook"].(string)
	if webhook == "" {
		return fmt.Errorf("dingtalk webhook is empty")
	}

	// 构建 Markdown 消息
	severityEmoji := map[AlertSeverity]string{
		SeverityInfo:     "ℹ️",
		SeverityWarning:  "⚠️",
		SeverityCritical: "🔴",
	}

	emoji := severityEmoji[alert.Severity]
	title := fmt.Sprintf("%s %s", emoji, alert.Name)
	text := fmt.Sprintf(`### %s

**状态**: %s
**检查项**: %s
**严重级别**: %s
**消息**: %s
**触发时间**: %s
`,
		emoji,
		alert.State,
		alert.CheckName,
		alert.Severity,
		alert.Message,
		alert.FiredAt.Format("2006-01-02 15:04:05"),
	)

	msg := DingTalkMessage{
		MsgType: "markdown",
		Markdown: &DingTalkMarkdown{
			Title: title,
			Text:  text,
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal dingtalk message: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhook, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("dingtalk request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}

	return nil
}

// Stop 停止告警管理器
func (am *AlertManager) Stop() {
	close(am.stopCh)
	am.wg.Wait()
}

// generateAlertID 生成告警 ID
func generateAlertID() string {
	return fmt.Sprintf("alert-%d", time.Now().UnixNano())
}

// generateSilenceID 生成静默 ID
func generateSilenceID() string {
	return fmt.Sprintf("silence-%d", time.Now().UnixNano())
}

// ClearResolvedAlerts 清理已解决的告警
func (am *AlertManager) ClearResolvedAlerts() {
	am.mu.Lock()
	defer am.mu.Unlock()

	for key, alert := range am.alerts {
		if alert.State == AlertStateResolved && time.Since(alert.ResolvedAt) > am.config.RetentionPeriod {
			delete(am.alerts, key)
		}
	}
}

// ClearExpiredSilences 清理过期的静默
func (am *AlertManager) ClearExpiredSilences() {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for id, silence := range am.silences {
		if now.After(silence.EndsAt) {
			delete(am.silences, id)
		}
	}
}

// Stats 获取告警统计
func (am *AlertManager) Stats() AlertStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := AlertStats{
		TotalAlerts:   len(am.alerts),
		TotalRules:    len(am.rules),
		TotalChannels: len(am.channels),
		TotalSilences: len(am.silences),
	}

	for _, a := range am.alerts {
		if a.State == AlertStateFiring {
			stats.ActiveAlerts++
		}
		switch a.Severity {
		case SeverityCritical:
			stats.CriticalAlerts++
		case SeverityWarning:
			stats.WarningAlerts++
		case SeverityInfo:
			stats.InfoAlerts++
		}
	}

	return stats
}

// AlertStats 告警统计
type AlertStats struct {
	TotalAlerts    int `json:"total_alerts"`
	ActiveAlerts   int `json:"active_alerts"`
	CriticalAlerts int `json:"critical_alerts"`
	WarningAlerts  int `json:"warning_alerts"`
	InfoAlerts     int `json:"info_alerts"`
	TotalRules     int `json:"total_rules"`
	TotalChannels  int `json:"total_channels"`
	TotalSilences  int `json:"total_silences"`
}
