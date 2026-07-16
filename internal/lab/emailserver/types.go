// Package emailserver 提供邮件服务器功能，对标群晖 MailPlus Server
package emailserver

import (
	"time"
)

// SMTPConfig SMTP 服务配置.
type SMTPConfig struct {
	Enabled      bool   `json:"enabled"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Domain       string `json:"domain"`
	MaxMessageMB int    `json:"max_message_mb"` // 最大邮件大小(MB)
	RequireAuth  bool   `json:"require_auth"`   // 是否要求认证
	EnableTLS    bool   `json:"enable_tls"`     // 是否启用 TLS
	TLSCertFile  string `json:"tls_cert_file,omitempty"`
	TLSKeyFile   string `json:"tls_key_file,omitempty"`
}

// IMAPConfig IMAP 服务配置.
type IMAPConfig struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	EnableTLS   bool   `json:"enable_tls"`
	TLSCertFile string `json:"tls_cert_file,omitempty"`
	TLSKeyFile  string `json:"tls_key_file,omitempty"`
}

// AntispamConfig 反垃圾邮件配置.
type AntispamConfig struct {
	Enabled          bool     `json:"enabled"`
	Threshold        int      `json:"threshold"` // 评分阈值 0-100
	BlacklistDomains []string `json:"blacklist_domains"`
	BlacklistAddrs   []string `json:"blacklist_addrs"` // 黑名单邮箱
	WhitelistAddrs   []string `json:"whitelist_addrs"` // 白名单邮箱
	RejectSpam       bool     `json:"reject_spam"`     // 是否直接拒绝垃圾邮件
}

// FilterRule 邮件过滤规则.
type FilterRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Priority    int    `json:"priority"` // 优先级，数值越小越优先
	Enabled     bool   `json:"enabled"`
	Condition   string `json:"condition"`  // from, to, subject, body
	MatchType   string `json:"match_type"` // contains, equals, starts_with, regex
	MatchValue  string `json:"match_value"`
	Action      string `json:"action"`       // move, delete, mark_read, forward, reject
	ActionValue string `json:"action_value"` // action 的目标，如文件夹名或转发地址
}

// EmailAccount 邮件账户.
type EmailAccount struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"` // 用户名 (不含@domain)
	Domain      string    `json:"domain"`   // 域名
	Email       string    `json:"email"`    // 完整邮箱地址
	Password    string    `json:"password"` // 密码 (hash存储)
	DisplayName string    `json:"display_name"`
	QuotaMB     int       `json:"quota_mb"`  // 邮箱配额(MB)
	UsedMB      int       `json:"used_mb"`   // 已用空间(MB)
	IsAdmin     bool      `json:"is_admin"`  // 是否管理员
	IsActive    bool      `json:"is_active"` // 是否启用
	Aliases     []string  `json:"aliases"`   // 邮箱别名
	Forwards    []string  `json:"forwards"`  // 自动转发地址
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EmailMessage 邮件消息.
type EmailMessage struct {
	ID          string       `json:"id"`
	AccountID   string       `json:"account_id"`
	Folder      string       `json:"folder"` // inbox, sent, drafts, trash, spam
	From        string       `json:"from"`
	To          []string     `json:"to"`
	CC          []string     `json:"cc,omitempty"`
	BCC         []string     `json:"bcc,omitempty"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body"` // 纯文本或 HTML
	IsRead      bool         `json:"is_read"`
	IsStarred   bool         `json:"is_starred"`
	HasAttach   bool         `json:"has_attachment"`
	Attachments []Attachment `json:"attachments,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	Size        int64        `json:"size"` // 邮件大小(字节)
}

// Attachment 邮件附件.
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// ========== 请求/响应结构 ==========

// CreateAccountRequest 创建账户请求.
type CreateAccountRequest struct {
	Username    string `json:"username" binding:"required"`
	Domain      string `json:"domain" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"display_name"`
	QuotaMB     int    `json:"quota_mb"`
	IsAdmin     bool   `json:"is_admin"`
}

// UpdateAccountRequest 更新账户请求.
type UpdateAccountRequest struct {
	DisplayName *string  `json:"display_name,omitempty"`
	Password    *string  `json:"password,omitempty"`
	QuotaMB     *int     `json:"quota_mb,omitempty"`
	IsAdmin     *bool    `json:"is_admin,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Forwards    []string `json:"forwards,omitempty"`
}

// SendEmailRequest 发送邮件请求.
type SendEmailRequest struct {
	From    string   `json:"from" binding:"required"`
	To      []string `json:"to" binding:"required"`
	CC      []string `json:"cc,omitempty"`
	BCC     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject" binding:"required"`
	Body    string   `json:"body" binding:"required"`
	IsHTML  bool     `json:"is_html"`
}

// ListMessagesRequest 获取邮件列表请求.
type ListMessagesRequest struct {
	AccountID string `form:"account_id" binding:"required"`
	Folder    string `form:"folder"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// UpdateSMTPRequest 更新 SMTP 配置请求.
type UpdateSMTPRequest struct {
	Enabled      *bool   `json:"enabled,omitempty"`
	Port         *int    `json:"port,omitempty"`
	Domain       *string `json:"domain,omitempty"`
	MaxMessageMB *int    `json:"max_message_mb,omitempty"`
	RequireAuth  *bool   `json:"require_auth,omitempty"`
	EnableTLS    *bool   `json:"enable_tls,omitempty"`
}

// UpdateIMAPRequest 更新 IMAP 配置请求.
type UpdateIMAPRequest struct {
	Enabled   *bool `json:"enabled,omitempty"`
	Port      *int  `json:"port,omitempty"`
	EnableTLS *bool `json:"enable_tls,omitempty"`
}

// CreateFilterRuleRequest 创建过滤规则请求.
type CreateFilterRuleRequest struct {
	Name        string `json:"name" binding:"required"`
	Priority    int    `json:"priority"`
	Condition   string `json:"condition" binding:"required"`
	MatchType   string `json:"match_type" binding:"required"`
	MatchValue  string `json:"match_value" binding:"required"`
	Action      string `json:"action" binding:"required"`
	ActionValue string `json:"action_value"`
}

// UpdateAntispamRequest 更新反垃圾邮件配置请求.
type UpdateAntispamRequest struct {
	Enabled          *bool    `json:"enabled,omitempty"`
	Threshold        *int     `json:"threshold,omitempty"`
	BlacklistDomains []string `json:"blacklist_domains,omitempty"`
	BlacklistAddrs   []string `json:"blacklist_addrs,omitempty"`
	WhitelistAddrs   []string `json:"whitelist_addrs,omitempty"`
	RejectSpam       *bool    `json:"reject_spam,omitempty"`
}
