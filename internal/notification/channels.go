package notification

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ChannelManager 渠道管理器
type ChannelManager struct {
	channels map[string]*ChannelConfig
	mu       sync.RWMutex
}

// NewChannelManager 创建渠道管理器
func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]*ChannelConfig),
	}
}

// AddChannel 添加渠道
func (cm *ChannelManager) AddChannel(config *ChannelConfig) error {
	if config.ID == "" {
		return fmt.Errorf("渠道 ID 不能为空")
	}

	if config.Name == "" {
		return fmt.Errorf("渠道名称不能为空")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	cm.channels[config.ID] = config

	return nil
}

// UpdateChannel 更新渠道
func (cm *ChannelManager) UpdateChannel(config *ChannelConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.channels[config.ID]; !exists {
		return fmt.Errorf("渠道不存在: %s", config.ID)
	}

	config.UpdatedAt = time.Now()
	cm.channels[config.ID] = config

	return nil
}

// RemoveChannel 移除渠道
func (cm *ChannelManager) RemoveChannel(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.channels[id]; !exists {
		return fmt.Errorf("渠道不存在: %s", id)
	}

	delete(cm.channels, id)
	return nil
}

// GetChannel 获取渠道
func (cm *ChannelManager) GetChannel(id string) (*ChannelConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	config, exists := cm.channels[id]
	if !exists {
		return nil, fmt.Errorf("渠道不存在: %s", id)
	}

	return config, nil
}

// ListChannels 列出渠道
func (cm *ChannelManager) ListChannels(channelType ChannelType) []*ChannelConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*ChannelConfig, 0)
	for _, c := range cm.channels {
		if channelType == "" || c.Type == channelType {
			result = append(result, c)
		}
	}

	return result
}

// GetEnabledChannels 获取启用的渠道
func (cm *ChannelManager) GetEnabledChannels() []*ChannelConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*ChannelConfig, 0)
	for _, c := range cm.channels {
		if c.Enabled {
			result = append(result, c)
		}
	}

	return result
}

// ChannelSender 渠道发送器接口
type ChannelSender interface {
	Send(config *ChannelConfig, notification *Notification) error
	Type() ChannelType
}

// EmailSender 邮件发送器
type EmailSender struct{}

func NewEmailSender() *EmailSender {
	return &EmailSender{}
}

func (s *EmailSender) Type() ChannelType {
	return ChannelEmail
}

func (s *EmailSender) Send(config *ChannelConfig, notification *Notification) error {
	emailConfig, err := parseEmailConfig(config.Config)
	if err != nil {
		return err
	}

	// 安全清理邮件头部
	from := sanitizeEmailHeader(emailConfig.From)
	to := make([]string, len(emailConfig.To))
	for i, recipient := range emailConfig.To {
		to[i] = sanitizeEmailHeader(recipient)
	}
	subject := sanitizeEmailHeader(notification.Title)

	// 构建邮件
	msg := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\n", from, strings.Join(to, ", "), subject)
	msg += "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg += s.buildHTMLBody(notification)

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", emailConfig.SMTPHost, emailConfig.SMTPPort)
	var auth smtp.Auth
	if emailConfig.Username != "" {
		auth = smtp.PlainAuth("", emailConfig.Username, emailConfig.Password, emailConfig.SMTPHost)
	}

	return smtp.SendMail(addr, auth, emailConfig.From, emailConfig.To, []byte(msg))
}

func (s *EmailSender) buildHTMLBody(notification *Notification) string {
	levelColor := s.getLevelColor(notification.Level)
	levelLabel := s.getLevelLabel(notification.Level)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { padding: 20px; border-radius: 8px 8px 0 0; color: white; text-align: center; }
        .header.%s { background: %s; }
        .content { background: #f9f9f9; padding: 20px; border: 1px solid #ddd; border-top: none; }
        .footer { text-align: center; padding: 15px; color: #666; font-size: 12px; border-top: 1px solid #eee; }
        .info-box { margin: 15px 0; padding: 15px; background: white; border-left: 4px solid %s; border-radius: 4px; }
        .label { font-weight: bold; color: #555; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header %s">
            <h2>🔔 NAS-OS 通知</h2>
        </div>
        <div class="content">
            <p><span class="label">标题：</span>%s</p>
            <p><span class="label">级别：</span>%s</p>
            <p><span class="label">时间：</span>%s</p>
            %s
            <div class="info-box">
                <p><span class="label">详细内容：</span></p>
                <p>%s</p>
            </div>
        </div>
        <div class="footer">
            <p>此邮件由 NAS-OS 系统自动发送，请勿回复。</p>
        </div>
    </div>
</body>
</html>`,
		string(notification.Level),
		levelColor,
		levelColor,
		string(notification.Level),
		notification.Title,
		levelLabel,
		notification.CreatedAt.Format("2006-01-02 15:04:05"),
		s.buildExtraInfo(notification),
		strings.ReplaceAll(notification.Message, "\n", "<br>"),
	)
}

func (s *EmailSender) buildExtraInfo(notification *Notification) string {
	var info string
	if notification.Source != "" {
		info += fmt.Sprintf(`<p><span class="label">来源：</span>%s</p>`, notification.Source)
	}
	if notification.Category != "" {
		info += fmt.Sprintf(`<p><span class="label">类别：</span>%s</p>`, notification.Category)
	}
	return info
}

func (s *EmailSender) getLevelColor(level NotificationLevel) string {
	colors := map[NotificationLevel]string{
		LevelInfo:     "#3498db",
		LevelSuccess:  "#27ae60",
		LevelWarning:  "#f39c12",
		LevelError:    "#e74c3c",
		LevelCritical: "#c0392b",
	}
	if color, ok := colors[level]; ok {
		return color
	}
	return "#95a5a6"
}

func (s *EmailSender) getLevelLabel(level NotificationLevel) string {
	labels := map[NotificationLevel]string{
		LevelInfo:     "信息",
		LevelSuccess:  "成功",
		LevelWarning:  "警告",
		LevelError:    "错误",
		LevelCritical: "严重",
	}
	if label, ok := labels[level]; ok {
		return label
	}
	return "未知"
}

// WebhookSender Webhook 发送器
type WebhookSender struct {
	client *http.Client
}

func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *WebhookSender) Type() ChannelType {
	return ChannelWebhook
}

func (s *WebhookSender) Send(config *ChannelConfig, notification *Notification) error {
	webhookConfig, err := parseWebhookConfig(config.Config)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"id":        notification.ID,
		"title":     notification.Title,
		"message":   notification.Message,
		"level":     notification.Level,
		"category":  notification.Category,
		"source":    notification.Source,
		"timestamp": notification.CreatedAt.Unix(),
		"data":      notification.Data,
	}

	if notification.Data != nil {
		for k, v := range notification.Data {
			payload[k] = v
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	method := webhookConfig.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, webhookConfig.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range webhookConfig.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook 返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

// WebSocketSender WebSocket 发送器
type WebSocketSender struct {
	broadcaster WebSocketBroadcaster
}

// WebSocketBroadcaster WebSocket 广播接口
type WebSocketBroadcaster interface {
	Broadcast(message []byte) error
	SendToRoom(roomID string, message []byte) error
	SendToUser(userID string, message []byte) error
}

func NewWebSocketSender(broadcaster WebSocketBroadcaster) *WebSocketSender {
	return &WebSocketSender{
		broadcaster: broadcaster,
	}
}

func (s *WebSocketSender) Type() ChannelType {
	return ChannelWebSocket
}

func (s *WebSocketSender) Send(config *ChannelConfig, notification *Notification) error {
	wsConfig, err := parseWebSocketConfig(config.Config)
	if err != nil {
		return err
	}

	message := map[string]interface{}{
		"type":      "notification",
		"id":        notification.ID,
		"title":     notification.Title,
		"message":   notification.Message,
		"level":     notification.Level,
		"category":  notification.Category,
		"source":    notification.Source,
		"timestamp": notification.CreatedAt.Unix(),
		"data":      notification.Data,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// 广播到所有客户端
	if wsConfig.Broadcast || (len(wsConfig.RoomIDs) == 0 && len(wsConfig.UserIDs) == 0) {
		return s.broadcaster.Broadcast(jsonData)
	}

	// 发送到指定房间
	for _, roomID := range wsConfig.RoomIDs {
		if err := s.broadcaster.SendToRoom(roomID, jsonData); err != nil {
			return fmt.Errorf("发送到房间 %s 失败: %w", roomID, err)
		}
	}

	// 发送到指定用户
	for _, userID := range wsConfig.UserIDs {
		if err := s.broadcaster.SendToUser(userID, jsonData); err != nil {
			return fmt.Errorf("发送给用户 %s 失败: %w", userID, err)
		}
	}

	return nil
}

// WeChatSender 企业微信发送器
type WeChatSender struct {
	client *http.Client
}

func NewWeChatSender() *WeChatSender {
	return &WeChatSender{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *WeChatSender) Type() ChannelType {
	return ChannelWeChat
}

func (s *WeChatSender) Send(config *ChannelConfig, notification *Notification) error {
	wechatConfig, err := parseWeChatConfig(config.Config)
	if err != nil {
		return err
	}

	levelColor := s.getLevelColor(notification.Level)
	levelLabel := s.getLevelLabel(notification.Level)

	content := fmt.Sprintf(
		"## 🔔 NAS-OS 通知\n"+
			"> **标题：** %s\n"+
			"> **级别：** <font color=\"%s\">%s</font>\n"+
			"> **时间：** %s\n"+
			"> **来源：** %s\n\n"+
			"**详细内容：**\n%s",
		notification.Title,
		levelColor,
		levelLabel,
		notification.CreatedAt.Format("2006-01-02 15:04:05"),
		notification.Source,
		notification.Message,
	)

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := s.client.Post(wechatConfig.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("企业微信返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

func (s *WeChatSender) getLevelColor(level NotificationLevel) string {
	colors := map[NotificationLevel]string{
		LevelInfo:     "info",
		LevelSuccess:  "info",
		LevelWarning:  "warning",
		LevelError:    "warning",
		LevelCritical: "warning",
	}
	if color, ok := colors[level]; ok {
		return color
	}
	return "comment"
}

func (s *WeChatSender) getLevelLabel(level NotificationLevel) string {
	labels := map[NotificationLevel]string{
		LevelInfo:     "信息",
		LevelSuccess:  "成功",
		LevelWarning:  "警告",
		LevelError:    "错误",
		LevelCritical: "严重",
	}
	if label, ok := labels[level]; ok {
		return label
	}
	return "未知"
}

// DingTalkSender 钉钉发送器
type DingTalkSender struct {
	client *http.Client
}

func NewDingTalkSender() *DingTalkSender {
	return &DingTalkSender{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *DingTalkSender) Type() ChannelType {
	return ChannelDingTalk
}

func (s *DingTalkSender) Send(config *ChannelConfig, notification *Notification) error {
	dingConfig, err := parseDingTalkConfig(config.Config)
	if err != nil {
		return err
	}

	webhookURL := dingConfig.WebhookURL
	if dingConfig.Secret != "" {
		webhookURL = s.signURL(webhookURL, dingConfig.Secret)
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": notification.Title,
			"text": fmt.Sprintf(
				"### %s\n\n**级别：%s**\n\n**时间：%s**\n\n**来源：%s**\n\n%s",
				notification.Title,
				notification.Level,
				notification.CreatedAt.Format("2006-01-02 15:04:05"),
				notification.Source,
				notification.Message,
			),
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := s.client.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("钉钉返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

func (s *DingTalkSender) signURL(webhookURL, secret string) string {
	timestamp := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%s&timestamp=%d&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
}

// TelegramSender Telegram 发送器
type TelegramSender struct {
	client *http.Client
}

func NewTelegramSender() *TelegramSender {
	return &TelegramSender{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *TelegramSender) Type() ChannelType {
	return ChannelTelegram
}

func (s *TelegramSender) Send(config *ChannelConfig, notification *Notification) error {
	telegramConfig, err := parseTelegramConfig(config.Config)
	if err != nil {
		return err
	}

	text := fmt.Sprintf(
		"🔔 *NAS-OS 通知*\n\n"+
			"*标题：* %s\n"+
			"*级别：* %s\n"+
			"*时间：* %s\n"+
			"*来源：* %s\n\n"+
			"%s",
		escapeTelegramMarkdown(notification.Title),
		notification.Level,
		notification.CreatedAt.Format("2006-01-02 15:04:05"),
		notification.Source,
		escapeTelegramMarkdown(notification.Message),
	)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramConfig.BotToken)

	payload := map[string]interface{}{
		"chat_id":    telegramConfig.ChatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := s.client.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Telegram 返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

func escapeTelegramMarkdown(text string) string {
	// Telegram MarkdownV2 需要转义的字符
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

// SenderRegistry 发送器注册表
type SenderRegistry struct {
	senders map[ChannelType]ChannelSender
	mu      sync.RWMutex
}

// NewSenderRegistry 创建发送器注册表
func NewSenderRegistry() *SenderRegistry {
	registry := &SenderRegistry{
		senders: make(map[ChannelType]ChannelSender),
	}

	// 注册默认发送器
	registry.Register(NewEmailSender())
	registry.Register(NewWebhookSender())
	registry.Register(NewWeChatSender())
	registry.Register(NewDingTalkSender())
	registry.Register(NewTelegramSender())

	return registry
}

// Register 注册发送器
func (r *SenderRegistry) Register(sender ChannelSender) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.senders[sender.Type()] = sender
}

// Get 获取发送器
func (r *SenderRegistry) Get(channelType ChannelType) (ChannelSender, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sender, exists := r.senders[channelType]
	return sender, exists
}

// SetWebSocketSender 设置 WebSocket 发送器（需要外部注入）
func (r *SenderRegistry) SetWebSocketSender(broadcaster WebSocketBroadcaster) {
	r.Register(NewWebSocketSender(broadcaster))
}

// 辅助函数

func sanitizeEmailHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' {
			return -1
		}
		return r
	}, s)
	return s
}

func parseEmailConfig(config map[string]interface{}) (*EmailChannelConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var emailConfig EmailChannelConfig
	if err := json.Unmarshal(jsonData, &emailConfig); err != nil {
		return nil, err
	}

	if emailConfig.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP 主机地址不能为空")
	}

	if emailConfig.SMTPPort == 0 {
		emailConfig.SMTPPort = 587
	}

	return &emailConfig, nil
}

func parseWebhookConfig(config map[string]interface{}) (*WebhookChannelConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var webhookConfig WebhookChannelConfig
	if err := json.Unmarshal(jsonData, &webhookConfig); err != nil {
		return nil, err
	}

	if webhookConfig.URL == "" {
		return nil, fmt.Errorf("Webhook URL 不能为空")
	}

	return &webhookConfig, nil
}

func parseWebSocketConfig(config map[string]interface{}) (*WebSocketChannelConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var wsConfig WebSocketChannelConfig
	if err := json.Unmarshal(jsonData, &wsConfig); err != nil {
		return nil, err
	}

	return &wsConfig, nil
}

func parseWeChatConfig(config map[string]interface{}) (*WeChatChannelConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var wechatConfig WeChatChannelConfig
	if err := json.Unmarshal(jsonData, &wechatConfig); err != nil {
		return nil, err
	}

	if wechatConfig.WebhookURL == "" {
		return nil, fmt.Errorf("企业微信 Webhook URL 不能为空")
	}

	return &wechatConfig, nil
}

func parseDingTalkConfig(config map[string]interface{}) (*DingTalkChannelConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var dingConfig DingTalkChannelConfig
	if err := json.Unmarshal(jsonData, &dingConfig); err != nil {
		return nil, err
	}

	if dingConfig.WebhookURL == "" {
		return nil, fmt.Errorf("钉钉 Webhook URL 不能为空")
	}

	return &dingConfig, nil
}

func parseTelegramConfig(config map[string]interface{}) (*TelegramChannelConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var telegramConfig TelegramChannelConfig
	if err := json.Unmarshal(jsonData, &telegramConfig); err != nil {
		return nil, err
	}

	if telegramConfig.BotToken == "" {
		return nil, fmt.Errorf("Telegram Bot Token 不能为空")
	}

	if telegramConfig.ChatID == "" {
		return nil, fmt.Errorf("Telegram Chat ID 不能为空")
	}

	return &telegramConfig, nil
}

// GenerateID 生成唯一 ID
func GenerateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// HexEncode 十六进制编码
func HexEncode(data []byte) string {
	return hex.EncodeToString(data)
}
