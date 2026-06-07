package containerscanpro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Alerter 告警器
type Alerter struct {
	mu         sync.RWMutex
	config     *AlertConfig
	alerts     []Alert
	alertChan  chan Alert
	stopCh     chan struct{}
	lastAlerts map[string]time.Time // key -> last alert time (for cooldown)
	httpClient *http.Client
}

// NewAlerter 创建告警器
func NewAlerter(config *AlertConfig) *Alerter {
	return &Alerter{
		config:     config,
		alerts:     make([]Alert, 0),
		alertChan:  make(chan Alert, 100),
		stopCh:     make(chan struct{}),
		lastAlerts: make(map[string]time.Time),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start 启动告警器
func (a *Alerter) Start(ctx context.Context) error {
	if !a.config.Enabled {
		log.Println("[Alerter] Alerts disabled")
		return nil
	}

	log.Println("[Alerter] Starting alert processor...")
	go a.processAlerts(ctx)

	return nil
}

// Stop 停止告警器
func (a *Alerter) Stop() {
	close(a.stopCh)
}

// SendAlert 发送告警
func (a *Alerter) SendAlert(alert Alert) {
	if !a.config.Enabled {
		return
	}

	// 检查告警级别是否满足最低要求
	if !a.shouldAlert(alert.Level) {
		return
	}

	// 检查冷却时间
	if a.isInCooldown(alert) {
		return
	}

	a.alertChan <- alert
}

// processAlerts 处理告警队列
func (a *Alerter) processAlerts(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case alert := <-a.alertChan:
			a.handleAlert(ctx, alert)
		}
	}
}

// handleAlert 处理单个告警
func (a *Alerter) handleAlert(ctx context.Context, alert Alert) {
	// 设置默认值
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%d", time.Now().UnixNano())
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	// 存储告警
	a.mu.Lock()
	a.alerts = append(a.alerts, alert)
	a.lastAlerts[alert.Title] = time.Now()
	a.mu.Unlock()

	log.Printf("[Alerter] Alert [%s] %s: %s", alert.Level, alert.Title, alert.Message)

	// 发送到 Webhook
	if a.config.WebhookURL != "" {
		go a.sendWebhook(ctx, alert)
	}

	// 发送邮件（模拟）
	if len(a.config.EmailTo) > 0 {
		go a.sendEmail(ctx, alert)
	}
}

// sendWebhook 发送 Webhook 告警
func (a *Alerter) sendWebhook(ctx context.Context, alert Alert) {
	payload, err := json.Marshal(alert)
	if err != nil {
		log.Printf("[Alerter] Failed to marshal alert: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.WebhookURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("[Alerter] Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		log.Printf("[Alerter] Failed to send webhook: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Alerter] Webhook returned status %d", resp.StatusCode)
	}
}

// sendEmail 发送邮件告警（模拟实现）
func (a *Alerter) sendEmail(ctx context.Context, alert Alert) {
	// 实际实现中这里会调用 SMTP 或邮件服务 API
	log.Printf("[Alerter] Email alert would be sent to: %v", a.config.EmailTo)
}

// shouldAlert 检查是否应该发送告警
func (a *Alerter) shouldAlert(level AlertLevel) bool {
	levelWeight := map[AlertLevel]int{
		AlertLevelInfo:     1,
		AlertLevelWarning:  2,
		AlertLevelCritical: 3,
	}

	return levelWeight[level] >= levelWeight[a.config.MinLevel]
}

// isInCooldown 检查是否在冷却期
func (a *Alerter) isInCooldown(alert Alert) bool {
	a.mu.RLock()
	lastAlert, exists := a.lastAlerts[alert.Title]
	a.mu.RUnlock()

	if !exists {
		return false
	}

	cooldown := time.Duration(a.config.CooldownSec) * time.Second
	return time.Since(lastAlert) < cooldown
}

// GetAlerts 获取所有告警
func (a *Alerter) GetAlerts() []Alert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	alerts := make([]Alert, len(a.alerts))
	copy(alerts, a.alerts)
	return alerts
}

// GetAlertsByLevel 按级别获取告警
func (a *Alerter) GetAlertsByLevel(level AlertLevel) []Alert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var alerts []Alert
	for _, alert := range a.alerts {
		if alert.Level == level {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// GetAlertsSince 获取指定时间后的告警
func (a *Alerter) GetAlertsSince(since time.Time) []Alert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var alerts []Alert
	for _, alert := range a.alerts {
		if alert.Timestamp.After(since) {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// ClearAlerts 清除告警
func (a *Alerter) ClearAlerts() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.alerts = make([]Alert, 0)
}

// GetAlertStats 获取告警统计
func (a *Alerter) GetAlertStats() map[string]int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := map[string]int{
		"total":    len(a.alerts),
		"critical": 0,
		"warning":  0,
		"info":     0,
	}

	for _, alert := range a.alerts {
		switch alert.Level {
		case AlertLevelCritical:
			stats["critical"]++
		case AlertLevelWarning:
			stats["warning"]++
		case AlertLevelInfo:
			stats["info"]++
		}
	}

	return stats
}

// CreateVulnerabilityAlert 创建漏洞告警
func CreateVulnerabilityAlert(vuln ContainerVulnerability) Alert {
	level := AlertLevelInfo
	if vuln.CVE.Severity == SeverityCritical || vuln.CVE.Severity == SeverityHigh {
		level = AlertLevelCritical
	} else if vuln.CVE.Severity == SeverityMedium {
		level = AlertLevelWarning
	}

	return Alert{
		Level:   level,
		Title:   fmt.Sprintf("CVE Vulnerability: %s", vuln.CVE.ID),
		Message: fmt.Sprintf("Container %s has vulnerability %s (Score: %.1f)", vuln.ContainerID, vuln.CVE.ID, vuln.CVE.Score),
		Source:  "cve-scanner",
		Details: map[string]string{
			"cve_id":       vuln.CVE.ID,
			"severity":     string(vuln.CVE.Severity),
			"score":        fmt.Sprintf("%.1f", vuln.CVE.Score),
			"package":      vuln.Package.Name,
			"container_id": vuln.ContainerID,
			"image_name":   vuln.ImageName,
		},
	}
}

// CreateAnomalyAlert 创建异常告警
func CreateAnomalyAlert(anomaly RuntimeAnomaly) Alert {
	level := AlertLevelWarning
	if anomaly.Severity == SeverityCritical {
		level = AlertLevelCritical
	} else if anomaly.Severity == SeverityInfo {
		level = AlertLevelInfo
	}

	return Alert{
		Level:   level,
		Title:   fmt.Sprintf("Runtime Anomaly: %s", anomaly.Type),
		Message: anomaly.Description,
		Source:  "runtime-monitor",
		Details: map[string]string{
			"anomaly_type": string(anomaly.Type),
			"container_id": anomaly.ContainerID,
			"severity":     string(anomaly.Severity),
		},
	}
}

// CreateScoreAlert 创建安全评分告警
func CreateScoreAlert(containerID string, score *SecurityScore) Alert {
	level := AlertLevelInfo
	if score.Overall < 30 {
		level = AlertLevelCritical
	} else if score.Overall < 60 {
		level = AlertLevelWarning
	}

	return Alert{
		Level:   level,
		Title:   fmt.Sprintf("Security Score Alert: %s", containerID),
		Message: fmt.Sprintf("Container %s has security score %.1f (Grade: %s)", containerID, score.Overall, score.Grade),
		Source:  "security-scorer",
		Details: map[string]string{
			"container_id": containerID,
			"score":        fmt.Sprintf("%.1f", score.Overall),
			"grade":        score.Grade,
		},
	}
}
