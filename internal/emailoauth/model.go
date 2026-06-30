// Package emailoauth 提供邮件 OAuth 通知功能，
// 支持 Gmail 和 Outlook 的 OAuth2 授权发送邮件通知。
// 对标 fnOS v1.2 的邮件通知能力。
package emailoauth

import "time"

// EmailProvider 邮件提供商
type EmailProvider string

const (
	ProviderGmail   EmailProvider = "gmail"
	ProviderOutlook EmailProvider = "outlook"
)

// SendMethod 发送方式
type SendMethod string

const (
	SendMethodSMTP   SendMethod = "smtp"
	SendMethodOAuth2 SendMethod = "oauth2"
)

// OAuthConfig OAuth2 配置
type OAuthConfig struct {
	// 提供商
	Provider EmailProvider `json:"provider"`
	// OAuth2 Client ID
	ClientID string `json:"client_id"`
	// OAuth2 Client Secret
	ClientSecret string `json:"client_secret"`
	// Refresh Token
	RefreshToken string `json:"refresh_token"`
	// Access Token（自动刷新）
	AccessToken string `json:"access_token,omitempty"`
	// Token 过期时间
	TokenExpiry time.Time `json:"token_expiry,omitempty"`
	// 发件人邮箱
	FromEmail string `json:"from_email"`
	// SMTP 服务器（SMTP 方式时使用）
	SMTPHost string `json:"smtp_host,omitempty"`
	// SMTP 端口
	SMTPPort int `json:"smtp_port,omitempty"`
	// 发送方式
	Method SendMethod `json:"method"`
}

// MailMessage 邮件消息
type MailMessage struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

// MailResult 发送结果
type MailResult struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	SentAt    time.Time `json:"sent_at"`
	Method    SendMethod `json:"method"`
	Provider  EmailProvider `json:"provider"`
}

// SetConfigRequest 设置配置请求
type SetConfigRequest struct {
	Provider     EmailProvider `json:"provider"`
	ClientID     string        `json:"client_id"`
	ClientSecret string        `json:"client_secret"`
	RefreshToken string        `json:"refresh_token"`
	FromEmail    string        `json:"from_email"`
	Method       SendMethod    `json:"method"`
	SMTPHost     string        `json:"smtp_host,omitempty"`
	SMTPPort     int           `json:"smtp_port,omitempty"`
}

// TestMailRequest 测试邮件请求
type TestMailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}