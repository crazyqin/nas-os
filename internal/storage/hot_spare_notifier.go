// Package storage 提供热备盘告警通知功能
// 支持多种通知渠道：邮件、Webhook、Telegram、企业微信等
package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 通知配置 ==========

// NotificationConfig 通知配置.
type NotificationConfig struct {
	Enabled bool `json:"enabled"` // 是否启用通知

	// 邮件配置
	Email EmailConfig `json:"email"`

	// Webhook配置
	Webhook WebhookConfig `json:"webhook"`

	// Telegram配置
	Telegram TelegramConfig `json:"telegram"`

	// 企业微信配置
	WeChat WeChatConfig `json:"wechat"`

	// 钉钉配置
	DingTalk DingTalkConfig `json:"dingtalk"`

	// 通知级别
	NotifyLevels []string `json:"notifyLevels"` // info, warning, critical

	// 静默时间（避免频繁通知）
	SilenceDuration time.Duration `json:"silenceDuration"`

	// 重试配置
	MaxRetries    int           `json:"maxRetries"`
	RetryInterval time.Duration `json:"retryInterval"`
}

// EmailConfig 邮件配置.
type EmailConfig struct {
	Enabled     bool     `json:"enabled"`
	SMTPHost    string   `json:"smtpHost"`
	SMTPPort    int      `json:"smtpPort"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	UseTLS      bool     `json:"useTLS"`
	UseStartTLS bool     `json:"useStartTLS"`
}

// WebhookConfig Webhook配置.
type WebhookConfig struct {
	Enabled  bool              `json:"enabled"`
	URL      string            `json:"url"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
	Template string            `json:"template"` // 自定义模板
}

// TelegramConfig Telegram配置.
type TelegramConfig struct {
	Enabled   bool   `json:"enabled"`
	BotToken  string `json:"botToken"`
	ChatID    string `json:"chatId"`
	ParseMode string `json:"parseMode"` // HTML, Markdown
}

// WeChatConfig 企业微信配置.
type WeChatConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl"`
}

// DingTalkConfig 钉钉配置.
type DingTalkConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl"`
	Secret     string `json:"secret"` // 加签密钥
}

// DefaultNotificationConfig 默认配置.
var DefaultNotificationConfig = NotificationConfig{
	Enabled:         true,
	NotifyLevels:    []string{"warning", "critical"},
	SilenceDuration: 5 * time.Minute,
	MaxRetries:      3,
	RetryInterval:   30 * time.Second,
}

// ========== 通知管理器 ==========

// NotificationManager 通知管理器.
type NotificationManager struct {
	config       NotificationConfig
	lastNotified map[string]time.Time // 事件类型 -> 最后通知时间
	templates    map[string]*template.Template
	httpClient   *http.Client
}

// NewNotificationManager 创建通知管理器.
func NewNotificationManager(config NotificationConfig) *NotificationManager {
	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	nm := &NotificationManager{
		config:       config,
		lastNotified: make(map[string]time.Time),
		templates:    make(map[string]*template.Template),
		httpClient:   httpClient,
	}

	// 加载内置模板
	nm.loadTemplates()

	return nm
}

// loadTemplates 加载通知模板.
func (nm *NotificationManager) loadTemplates() {
	// 事件通知模板
	nm.templates["event"] = template.Must(template.New("event").Parse(`
【NAS-OS 热备盘通知】

事件类型: {{.Type}}
时间: {{.Timestamp.Format "2006-01-02 15:04:05"}}

设备: {{.Device}}
卷名: {{.VolumeName}}
{{if .FailedDevice}}故障设备: {{.FailedDevice}}{{end}}

详细信息:
{{.Message}}

---
NAS-OS 存储管理系统
`))

	// 邮件HTML模板
	nm.templates["email"] = template.Must(template.New("email").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #2c3e50; color: white; padding: 20px; text-align: center; }
        .content { background: #f9f9f9; padding: 20px; }
        .info { background: #e8f4f8; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .warning { background: #fff3cd; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .critical { background: #f8d7da; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>NAS-OS 热备盘通知</h1>
        </div>
        <div class="content">
            <div class="{{.Level}}">
                <h2>事件: {{.Type}}</h2>
                <p><strong>时间:</strong> {{.Timestamp.Format "2006-01-02 15:04:05"}}</p>
                <p><strong>设备:</strong> {{.Device}}</p>
                <p><strong>卷名:</strong> {{.VolumeName}}</p>
                {{if .FailedDevice}}<p><strong>故障设备:</strong> {{.FailedDevice}}</p>{{end}}
                <hr>
                <p>{{.Message}}</p>
            </div>
        </div>
        <div class="footer">
            NAS-OS 存储管理系统 | 自动发送，请勿回复
        </div>
    </div>
</body>
</html>
`))

	// Webhook JSON模板
	nm.templates["webhook"] = template.Must(template.New("webhook").Parse(`{
    "event_type": "{{.Type}}",
    "timestamp": "{{.Timestamp.Format "2006-01-02T15:04:05Z07:00"}}",
    "device": "{{.Device}}",
    "volume_name": "{{.VolumeName}}",
    "failed_device": "{{.FailedDevice}}",
    "message": "{{.Message}}",
    "level": "{{.Level}}"
}`))
}

// Send 发送通知.
func (nm *NotificationManager) Send(event HotSpareEvent) error {
	if !nm.config.Enabled {
		return nil
	}

	// 检查事件级别
	level := nm.getEventLevel(event)
	if !nm.shouldNotify(level) {
		return nil
	}

	// 检查静默时间
	if nm.isSilenced(event) {
		return nil
	}

	// 更新最后通知时间
	nm.lastNotified[event.Type] = time.Now()

	// 准备模板数据
	data := struct {
		HotSpareEvent
		Level string
	}{
		HotSpareEvent: event,
		Level:         level,
	}

	// 并发发送到各渠道
	errChan := make(chan error, 5)
	var wg sync.WaitGroup

	// 邮件
	if nm.config.Email.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- nm.sendEmail(data)
		}()
	}

	// Webhook
	if nm.config.Webhook.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- nm.sendWebhook(data)
		}()
	}

	// Telegram
	if nm.config.Telegram.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- nm.sendTelegram(data)
		}()
	}

	// 企业微信
	if nm.config.WeChat.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- nm.sendWeChat(data)
		}()
	}

	// 钉钉
	if nm.config.DingTalk.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- nm.sendDingTalk(data)
		}()
	}

	// 等待所有发送完成
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// 收集错误
	var errors []string
	for err := range errChan {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("通知发送失败: %s", strings.Join(errors, "; "))
	}

	return nil
}

// getEventLevel 获取事件级别.
func (nm *NotificationManager) getEventLevel(event HotSpareEvent) string {
	switch event.Type {
	case "rebuild_start", "rebuild_complete":
		return "info"
	case "rebuild_cancelled", "error":
		return "warning"
	case "rebuild_failed":
		return "critical"
	default:
		return "info"
	}
}

// shouldNotify 检查是否应该发送通知.
func (nm *NotificationManager) shouldNotify(level string) bool {
	for _, l := range nm.config.NotifyLevels {
		if l == level || (l == "warning" && level == "critical") {
			return true
		}
	}
	return false
}

// isSilenced 检查是否在静默期.
func (nm *NotificationManager) isSilenced(event HotSpareEvent) bool {
	lastTime, exists := nm.lastNotified[event.Type]
	if !exists {
		return false
	}
	return time.Since(lastTime) < nm.config.SilenceDuration
}

// ========== 邮件发送 ==========

func (nm *NotificationManager) sendEmail(data interface{}) error {
	cfg := nm.config.Email

	// 渲染HTML模板
	var body bytes.Buffer
	if err := nm.templates["email"].Execute(&body, data); err != nil {
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	// 构建邮件
	eventData, ok := data.(struct {
		HotSpareEvent
		Level string
	})
	if !ok {
		return fmt.Errorf("无效的事件数据类型")
	}
	subject := fmt.Sprintf("[NAS-OS] 热备盘通知 - %s", eventData.Type)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		cfg.From, strings.Join(cfg.To, ","), subject, body.String())

	// 发送邮件
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	// 使用TLS
	if cfg.UseTLS {
		return nm.sendEmailWithTLS(addr, auth, cfg, []byte(msg))
	}

	return smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(msg))
}

func (nm *NotificationManager) sendEmailWithTLS(addr string, auth smtp.Auth, cfg EmailConfig, msg []byte) error {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config: &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         cfg.SMTPHost,
		},
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %w", err)
	}
	defer func() { _ = conn.Close() }() //nolint:errcheck

	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer func() { _ = client.Close() }() //nolint:errcheck

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP认证失败: %w", err)
		}
	}

	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	for _, to := range cfg.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("设置收件人失败: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("准备邮件数据失败: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}

	return w.Close()
}

// ========== Webhook发送 ==========

func (nm *NotificationManager) sendWebhook(data interface{}) error {
	cfg := nm.config.Webhook

	// 渲染模板
	var body bytes.Buffer
	tmpl := nm.templates["webhook"]
	if cfg.Template != "" {
		tmpl = template.Must(template.New("custom").Parse(cfg.Template))
	}

	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("渲染Webhook模板失败: %w", err)
	}

	// 创建请求
	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(context.Background(), method, cfg.URL, &body)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := nm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook返回错误: %d", resp.StatusCode)
	}

	return nil
}

// ========== Telegram发送 ==========

func (nm *NotificationManager) sendTelegram(data interface{}) error {
	cfg := nm.config.Telegram

	// 渲染消息
	var text bytes.Buffer
	if err := nm.templates["event"].Execute(&text, data); err != nil {
		return fmt.Errorf("渲染Telegram消息失败: %w", err)
	}

	// 构建请求
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)

	payload := map[string]interface{}{
		"chat_id": cfg.ChatID,
		"text":    text.String(),
	}

	if cfg.ParseMode != "" {
		payload["parse_mode"] = cfg.ParseMode
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("编码请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := nm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送Telegram消息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API返回错误: %d", resp.StatusCode)
	}

	return nil
}

// ========== 企业微信发送 ==========

func (nm *NotificationManager) sendWeChat(data interface{}) error {
	cfg := nm.config.WeChat

	// 渲染消息
	var text bytes.Buffer
	if err := nm.templates["event"].Execute(&text, data); err != nil {
		return fmt.Errorf("渲染企业微信消息失败: %w", err)
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": text.String(),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("编码请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := nm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送企业微信消息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("企业微信API返回错误: %d", resp.StatusCode)
	}

	return nil
}

// ========== 钉钉发送 ==========

func (nm *NotificationManager) sendDingTalk(data interface{}) error {
	cfg := nm.config.DingTalk

	// 渲染消息
	var text bytes.Buffer
	if err := nm.templates["event"].Execute(&text, data); err != nil {
		return fmt.Errorf("渲染钉钉消息失败: %w", err)
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": text.String(),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("编码请求失败: %w", err)
	}

	// 添加签名
	url := cfg.WebhookURL
	if cfg.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := nm.signDingTalk(timestamp, cfg.Secret)
		url = fmt.Sprintf("%s&timestamp=%d&sign=%s", url, timestamp, sign)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := nm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送钉钉消息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("钉钉API返回错误: %d", resp.StatusCode)
	}

	return nil
}

// signDingTalk 钉钉签名.
func (nm *NotificationManager) signDingTalk(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ========== 测试通知 ==========

// TestNotification 测试通知.
func (nm *NotificationManager) TestNotification(channel string) error {
	event := HotSpareEvent{
		Type:       "test",
		Device:     "/dev/sdx",
		VolumeName: "test-volume",
		Message:    "这是一条测试通知消息",
		Timestamp:  time.Now(),
	}

	switch channel {
	case "email":
		return nm.sendEmail(struct {
			HotSpareEvent
			Level string
		}{event, "info"})
	case "webhook":
		return nm.sendWebhook(struct {
			HotSpareEvent
			Level string
		}{event, "info"})
	case "telegram":
		return nm.sendTelegram(struct {
			HotSpareEvent
			Level string
		}{event, "info"})
	case "wechat":
		return nm.sendWeChat(struct {
			HotSpareEvent
			Level string
		}{event, "info"})
	case "dingtalk":
		return nm.sendDingTalk(struct {
			HotSpareEvent
			Level string
		}{event, "info"})
	default:
		return nm.Send(event)
	}
}

// ========== API Handler ==========

// NotificationAPI 通知API处理器.
type NotificationAPI struct {
	manager *NotificationManager
}

// NewNotificationAPI 创建通知API.
func NewNotificationAPI(manager *NotificationManager) *NotificationAPI {
	return &NotificationAPI{manager: manager}
}

// RegisterRoutes 注册路由.
func (api *NotificationAPI) RegisterRoutes(r *gin.RouterGroup) {
	notify := r.Group("/notification")
	{
		notify.GET("/config", api.GetConfig)
		notify.PUT("/config", api.UpdateConfig)
		notify.POST("/test", api.TestNotification)
		notify.GET("/history", api.GetHistory)
	}
}

// GetConfig 获取配置.
func (api *NotificationAPI) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, api.manager.config)
}

// UpdateConfig 更新配置.
func (api *NotificationAPI) UpdateConfig(c *gin.Context) {
	var config NotificationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	api.manager.config = config
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// TestNotification 测试通知.
func (api *NotificationAPI) TestNotification(c *gin.Context) {
	channel := c.Query("channel")
	if channel == "" {
		channel = "all"
	}

	if err := api.manager.TestNotification(channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "测试通知已发送"})
}

// GetHistory 获取通知历史.
func (api *NotificationAPI) GetHistory(c *gin.Context) {
	// 返回最近的通知历史
	// TODO: 实现持久化存储
	c.JSON(http.StatusOK, []interface{}{})
}
