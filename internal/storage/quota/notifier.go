// Package quota 提供存储配额管理和告警功能
package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ========== 内置通知器实现 ==========

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	From         string
}

// WebhookNotifier Webhook通知器
type WebhookNotifier struct {
	URL       string
	Timeout   time.Duration
	Headers   map[string]string
}

// CompositeNotifier 组合通知器（支持多种通知渠道）
type CompositeNotifier struct {
	notifiers map[string]Notifier // channelType -> Notifier
}

// NewCompositeNotifier 创建组合通知器
func NewCompositeNotifier() *CompositeNotifier {
	return &CompositeNotifier{
		notifiers: make(map[string]Notifier),
	}
}

// AddNotifier 添加通知器
func (cn *CompositeNotifier) AddNotifier(channelType string, n Notifier) {
	cn.notifiers[channelType] = n
}

// SendAlert 发送告警
func (cn *CompositeNotifier) SendAlert(alert *Alert, config *NotificationConfig) error {
	for _, channel := range config.Channels {
		if n, ok := cn.notifiers[channel]; ok {
			if err := n.SendAlert(alert, config); err != nil {
				// 记录错误但继续尝试其他渠道
				fmt.Printf("发送告警失败 [%s]: %v\n", channel, err)
			}
		}
	}
	return nil
}

// SendAlert 发送告警（Webhook）
func (wn *WebhookNotifier) SendAlert(alert *Alert, config *NotificationConfig) error {
	if wn.URL == "" {
		return fmt.Errorf("webhook URL 未配置")
	}

	payload := map[string]interface{}{
		"id":        alert.ID,
		"type":      alert.Type,
		"target":    alert.Target,
		"percent":   alert.Percent,
		"message":   alert.Message,
		"timestamp": alert.CreatedAt.Format(time.RFC3339),
		"resolved":  alert.Resolved,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化告警失败: %w", err)
	}

	client := &http.Client{
		Timeout: wn.Timeout,
	}

	req, err := http.NewRequest("POST", wn.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range wn.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook 返回错误: %d", resp.StatusCode)
	}

	return nil
}

// ========== 告警消息格式化 ==========

// FormatAlertMessage 格式化告警消息
func FormatAlertMessage(alert *Alert) string {
	var icon string
	switch AlertType(alert.Type) {
	case AlertTypeWarning:
		icon = "⚠️"
	case AlertTypeExceeded:
		icon = "🔴"
	default:
		icon = "📊"
	}

	return fmt.Sprintf("%s 存储配额告警\n\n目标: %s\n使用率: %.1f%%\n详情: %s\n时间: %s",
		icon, alert.Target, alert.Percent, alert.Message, alert.CreatedAt.Format(time.RFC3339))
}

// FormatEmailSubject 格式化邮件主题
func FormatEmailSubject(alert *Alert) string {
	switch AlertType(alert.Type) {
	case AlertTypeWarning:
		return fmt.Sprintf("[警告] 存储配额告警 - %s", alert.Target)
	case AlertTypeExceeded:
		return fmt.Sprintf("[紧急] 存储配额超限 - %s", alert.Target)
	default:
		return fmt.Sprintf("存储配额通知 - %s", alert.Target)
	}
}

// FormatSlackPayload 格式化Slack消息
func FormatSlackPayload(alert *Alert) map[string]interface{} {
	var color string
	switch AlertType(alert.Type) {
	case AlertTypeWarning:
		color = "warning" // 黄色
	case AlertTypeExceeded:
		color = "danger" // 红色
	default:
		color = "good" // 绿色
	}

	return map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": FormatEmailSubject(alert),
				"text":  FormatAlertMessage(alert),
				"ts":    alert.CreatedAt.Unix(),
			},
		},
	}
}

// FormatDiscordPayload 格式化Discord消息
func FormatDiscordPayload(alert *Alert) map[string]interface{} {
	return map[string]interface{}{
		"content": FormatAlertMessage(alert),
	}
}