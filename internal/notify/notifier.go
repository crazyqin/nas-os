package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"
)

// AlertLevel 告警级别.
type AlertLevel string

// 告警级别常量.
const (
	// LevelInfo represents info alert level.
	LevelInfo     AlertLevel = "info"
	LevelWarning  AlertLevel = "warning"
	LevelCritical AlertLevel = "critical"
)

// Notification 通知消息.
type Notification struct {
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Level     AlertLevel `json:"level"`
	Timestamp time.Time  `json:"timestamp"`
	Source    string     `json:"source"`
}

// Notifier 通知接口.
type Notifier interface {
	Send(notif *Notification) error
	Name() string
}

// Manager 通知管理器.
type Manager struct {
	notifiers []Notifier
}

// NewManager 创建通知管理器.
func NewManager() *Manager {
	return &Manager{
		notifiers: make([]Notifier, 0),
	}
}

// AddNotifier 添加通知渠道.
func (m *Manager) AddNotifier(n Notifier) {
	m.notifiers = append(m.notifiers, n)
}

// Send 发送通知到所有渠道.
func (m *Manager) Send(notif *Notification) error {
	notif.Timestamp = time.Now()

	for _, n := range m.notifiers {
		if err := n.Send(notif); err != nil {
			// 记录错误但继续发送其他渠道
			fmt.Printf("[%s] 发送通知失败：%v\n", n.Name(), err)
		}
	}
	return nil
}

// ========== 邮件通知 ==========

// EmailNotifier 邮件通知器.
type EmailNotifier struct {
	SMTPServer string
	Port       int
	Username   string
	Password   string
	From       string
	To         []string
}

// NewEmailNotifier 创建邮件通知器.
func NewEmailNotifier(smtpServer string, port int, username, password, from string, to []string) *EmailNotifier {
	return &EmailNotifier{
		SMTPServer: smtpServer,
		Port:       port,
		Username:   username,
		Password:   password,
		From:       from,
		To:         to,
	}
}

// Name 返回通知器名称.
func (e *EmailNotifier) Name() string {
	return "email"
}

// Send 发送邮件通知.
func (e *EmailNotifier) Send(notif *Notification) error {
	subject := fmt.Sprintf("[%s] NAS-OS 告警：%s", e.getLevelLabel(notif.Level), notif.Title)

	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { padding: 15px; border-radius: 8px 8px 0 0; color: white; }
        .header.info { background: #3498db; }
        .header.warning { background: #f39c12; }
        .header.critical { background: #e74c3c; }
        .content { background: #f9f9f9; padding: 20px; border: 1px solid #ddd; }
        .footer { text-align: center; padding: 15px; color: #666; font-size: 12px; }
        .alert-info { margin: 15px 0; padding: 10px; background: #e8f4f8; border-left: 4px solid #3498db; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header %s">
            <h2>🔔 NAS-OS 告警通知</h2>
        </div>
        <div class="content">
            <p><strong>告警标题：</strong>%s</p>
            <p><strong>告警级别：</strong>%s</p>
            <p><strong>告警时间：</strong>%s</p>
            <p><strong>告警来源：</strong>%s</p>
            <div class="alert-info">
                <p><strong>详细信息：</strong></p>
                <p>%s</p>
            </div>
        </div>
        <div class="footer">
            <p>此邮件由 NAS-OS 系统自动发送，请勿回复。</p>
        </div>
    </div>
</body>
</html>
`,
		string(notif.Level),
		notif.Title,
		e.getLevelLabel(notif.Level),
		notif.Timestamp.Format("2006-01-02 15:04:05"),
		notif.Source,
		notif.Message,
	)

	auth := smtp.PlainAuth("", e.Username, e.Password, e.SMTPServer)
	addr := fmt.Sprintf("%s:%d", e.SMTPServer, e.Port)

	headers := make(map[string]string)
	headers["From"] = e.From
	headers["To"] = e.joinRecipients()
	headers["Subject"] = subject
	headers["Content-Type"] = "text/html; charset=UTF-8"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	return smtp.SendMail(addr, auth, e.From, e.To, []byte(message))
}

func (e *EmailNotifier) getLevelLabel(level AlertLevel) string {
	switch level {
	case LevelInfo:
		return "信息"
	case LevelWarning:
		return "警告"
	case LevelCritical:
		return "严重"
	default:
		return "未知"
	}
}

func (e *EmailNotifier) joinRecipients() string {
	result := ""
	for i, r := range e.To {
		if i > 0 {
			result += ", "
		}
		result += r
	}
	return result
}

// ========== 微信通知（企业微信） ==========

// WeChatNotifier 企业微信通知器.
type WeChatNotifier struct {
	WebhookURL string
}

// NewWeChatNotifier 创建企业微信通知器.
func NewWeChatNotifier(webhookURL string) *WeChatNotifier {
	return &WeChatNotifier{
		WebhookURL: webhookURL,
	}
}

// Name 返回通知器名称.
func (w *WeChatNotifier) Name() string {
	return "wechat"
}

// Send 发送企业微信通知.
func (w *WeChatNotifier) Send(notif *Notification) error {
	levelColor := w.getLevelColor(notif.Level)
	levelLabel := w.getLevelLabel(notif.Level)

	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf(
				"## 🔔 NAS-OS 告警通知\n"+
					"> **告警标题：** %s\n"+
					"> **告警级别：** <font color=\"%s\">%s</font>\n"+
					"> **告警时间：** %s\n"+
					"> **告警来源：** %s\n"+
					"> **详细信息：** \n"+
					"> %s",
				notif.Title,
				levelColor,
				levelLabel,
				notif.Timestamp.Format("2006-01-02 15:04:05"),
				notif.Source,
				notif.Message,
			),
		},
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", w.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("微信通知返回状态码：%d", resp.StatusCode)
	}

	return nil
}

func (w *WeChatNotifier) getLevelColor(level AlertLevel) string {
	switch level {
	case LevelInfo:
		return "info"
	case LevelWarning:
		return "warning"
	case LevelCritical:
		return "error"
	default:
		return "comment"
	}
}

func (w *WeChatNotifier) getLevelLabel(level AlertLevel) string {
	switch level {
	case LevelInfo:
		return "信息"
	case LevelWarning:
		return "警告"
	case LevelCritical:
		return "严重"
	default:
		return "未知"
	}
}

// ========== 通用 Webhook 通知 ==========

// WebhookNotifier 通用 Webhook 通知器.
type WebhookNotifier struct {
	URL string
}

// NewWebhookNotifier 创建 Webhook 通知器.
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		URL: url,
	}
}

// Name 返回通知器名称.
func (w *WebhookNotifier) Name() string {
	return "webhook"
}

// Send 发送 Webhook 通知.
func (w *WebhookNotifier) Send(notif *Notification) error {
	jsonData, err := json.Marshal(notif)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", w.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook 返回状态码：%d", resp.StatusCode)
	}

	return nil
}
