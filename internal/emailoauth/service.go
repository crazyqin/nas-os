// Package emailoauth 提供邮件 OAuth 通知功能。
package emailoauth

import (
	"fmt"
	"sync"
	"time"
)

// MailNotifier 邮件通知器
type MailNotifier struct {
	mu     sync.RWMutex
	config *OAuthConfig
}

// NewMailNotifier 创建邮件通知器
func NewMailNotifier() *MailNotifier {
	return &MailNotifier{}
}

// SetConfig 设置配置
func (n *MailNotifier) SetConfig(cfg *OAuthConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.Provider != ProviderGmail && cfg.Provider != ProviderOutlook {
		return fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return fmt.Errorf("client ID and secret are required")
	}
	if cfg.Method == "" {
		cfg.Method = SendMethodOAuth2
	}
	n.mu.Lock()
	n.config = cfg
	n.mu.Unlock()
	return nil
}

// GetConfig 获取配置（隐藏敏感字段）
func (n *MailNotifier) GetConfig() *OAuthConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.config == nil {
		return nil
	}
	copy := *n.config
	copy.ClientSecret = "***"
	copy.RefreshToken = "***"
	copy.AccessToken = "***"
	return &copy
}

// IsConfigured 是否已配置
func (n *MailNotifier) IsConfigured() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.config != nil && n.config.ClientID != ""
}

// RefreshToken 刷新 access token（模拟）
func (n *MailNotifier) RefreshToken() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.config == nil {
		return fmt.Errorf("not configured")
	}
	if n.config.RefreshToken == "" {
		return fmt.Errorf("refresh token is required")
	}
	// Simulate token refresh
	n.config.AccessToken = fmt.Sprintf("refreshed_%d", time.Now().Unix())
	n.config.TokenExpiry = time.Now().Add(1 * time.Hour)
	return nil
}

// IsTokenValid 检查 token 是否有效
func (n *MailNotifier) IsTokenValid() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.config == nil {
		return false
	}
	return n.config.AccessToken != "" && time.Now().Before(n.config.TokenExpiry)
}

// SendMail 发送邮件
func (n *MailNotifier) SendMail(msg *MailMessage) (*MailResult, error) {
	n.mu.RLock()
	config := n.config
	n.mu.RUnlock()

	if config == nil {
		return &MailResult{Success: false, Message: "not configured", SentAt: time.Now()}, fmt.Errorf("not configured")
	}

	if len(msg.To) == 0 {
		return &MailResult{Success: false, Message: "no recipients", SentAt: time.Now()}, fmt.Errorf("no recipients")
	}

	// For OAuth2, ensure token is valid
	if config.Method == SendMethodOAuth2 {
		if !n.IsTokenValid() {
			if err := n.RefreshToken(); err != nil {
				return &MailResult{
					Success:  false,
					Message:  fmt.Sprintf("token refresh failed: %v", err),
					SentAt:   time.Now(),
					Method:   config.Method,
					Provider: config.Provider,
				}, fmt.Errorf("token refresh failed: %v", err)
			}
		}
	}

	// Simulate sending (real impl would use smtp.SendMail or Gmail/Outlook API)
	result := &MailResult{
		Success:   true,
		Message:   fmt.Sprintf("sent to %d recipient(s)", len(msg.To)),
		SentAt:    time.Now(),
		Method:    config.Method,
		Provider:  config.Provider,
	}
	return result, nil
}

// SendTestMail 发送测试邮件
func (n *MailNotifier) SendTestMail(to, subject, body string) (*MailResult, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient is required")
	}
	if subject == "" {
		subject = "Test Email from NAS-OS"
	}
	if body == "" {
		body = "This is a test email from NAS-OS email notification system."
	}
	return n.SendMail(&MailMessage{
		To:      []string{to},
		Subject: subject,
		Body:    body,
	})
}