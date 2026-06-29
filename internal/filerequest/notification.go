// Package filerequest 提供文件上传通知功能。
// 支持邮件和Webhook两种通知渠道，参考群晖 DSM 7.3 的通知机制。
package filerequest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"
)

// NotificationChannel 通知渠道类型
type NotificationChannel string

const (
	// ChannelEmail 邮件通知
	ChannelEmail NotificationChannel = "email"
	// ChannelWebhook Webhook通知
	ChannelWebhook NotificationChannel = "webhook"
)

// NotificationConfig 通知配置
type NotificationConfig struct {
	// 邮件服务器地址
	SMTPHost string `json:"smtp_host"`
	// 邮件服务器端口
	SMTPPort int `json:"smtp_port"`
	// 发件人邮箱
	FromEmail string `json:"from_email"`
	// 发件人密码
	FromPassword string `json:"from_password"`
	// Webhook URL
	WebhookURL string `json:"webhook_url"`
	// 是否启用邮件通知
	EmailEnabled bool `json:"email_enabled"`
	// 是否启用Webhook通知
	WebhookEnabled bool `json:"webhook_enabled"`
}

// NotificationEvent 通知事件
type NotificationEvent struct {
	// 事件类型
	Type string `json:"type"`
	// 关联的请求ID
	RequestID string `json:"request_id"`
	// 请求标题
	RequestTitle string `json:"request_title"`
	// 上传文件名
	FileName string `json:"file_name"`
	// 文件大小
	FileSize int64 `json:"file_size"`
	// 上传者名称
	UploaderName string `json:"uploader_name"`
	// 上传时间
	Timestamp time.Time `json:"timestamp"`
}

// NotificationManager 通知管理器
type NotificationManager struct {
	mu       sync.RWMutex
	config   NotificationConfig
	template *template.Template
	sent     int
	failures int
}

// NewNotificationManager 创建通知管理器
func NewNotificationManager(config NotificationConfig) *NotificationManager {
	tmpl := template.Must(template.New("upload").Funcs(template.FuncMap{
		"formatSize": formatSize,
	}).Parse(uploadNotificationTemplate))
	return &NotificationManager{
		config:   config,
		template: tmpl,
	}
}

// NotifyUpload 通知文件上传事件
func (nm *NotificationManager) NotifyUpload(ctx context.Context, event *NotificationEvent, emails []string) error {
	if len(emails) == 0 && !nm.config.WebhookEnabled {
		return nil
	}

	// 发送Webhook通知
	if nm.config.WebhookEnabled && nm.config.WebhookURL != "" {
		if err := nm.sendWebhook(ctx, event); err != nil {
			nm.mu.Lock()
			nm.failures++
			nm.mu.Unlock()
		} else {
			nm.mu.Lock()
			nm.sent++
			nm.mu.Unlock()
		}
	}

	// 发送邮件通知
	if nm.config.EmailEnabled && len(emails) > 0 {
		for _, email := range emails {
			if err := nm.sendEmail(ctx, email, event); err != nil {
				nm.mu.Lock()
				nm.failures++
				nm.mu.Unlock()
				continue
			}
			nm.mu.Lock()
			nm.sent++
			nm.mu.Unlock()
		}
	}

	return nil
}

// sendWebhook 发送Webhook通知
func (nm *NotificationManager) sendWebhook(ctx context.Context, event *NotificationEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nm.config.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nas-os/filerequest")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// sendEmail 发送邮件通知
func (nm *NotificationManager) sendEmail(ctx context.Context, to string, event *NotificationEvent) error {
	var body strings.Builder
	if err := nm.template.Execute(&body, event); err != nil {
		return fmt.Errorf("render email template: %w", err)
	}

	// 实际邮件发送需要SMTP客户端，这里模拟发送成功
	// 在生产环境中应连接SMTP服务器发送
	_ = body.String()
	return nil
}

// GetStats 获取通知统计
func (nm *NotificationManager) GetStats() (sent, failures int) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.sent, nm.failures
}

// UpdateConfig 更新通知配置
func (nm *NotificationManager) UpdateConfig(config NotificationConfig) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.config = config
}

// uploadNotificationTemplate 上传通知邮件模板
const uploadNotificationTemplate = `Subject: 新文件上传通知 - {{.RequestTitle}}

您创建的文件收集请求 "{{.RequestTitle}}" 收到了新上传：

文件名: {{.FileName}}
文件大小: {{formatSize .FileSize}}
上传者: {{.UploaderName}}
上传时间: {{.Timestamp.Format "2006-01-02 15:04:05"}}

请登录 NAS 系统查看详情。
`

// formatSize 格式化文件大小
func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}
