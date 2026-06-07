// Package mailserver 实现邮件服务模块，对标群晖 MailPlus
package mailserver

import (
	"context"
	"fmt"
	"net/smtp"
	"sync"
	"time"
)

// MailServer 邮件服务器
type MailServer struct {
	mu         sync.RWMutex
	domains    map[string]*Domain
	users      map[string]*MailUser
	messages   map[string]*Message
	config     *Config
	smtpServer *SMTPServer
	imapServer *IMAPServer
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// Config 邮件服务器配置
type Config struct {
	SMTPHost       string `json:"smtp_host"`
	SMTPPort       int    `json:"smtp_port"`
	IMAPHost       string `json:"imap_host"`
	IMAPPort       int    `json:"imap_port"`
	MaxMailboxes   int    `json:"max_mailboxes"`
	MaxMessageSize int64  `json:"max_message_size"`
	EnableTLS      bool   `json:"enable_tls"`
	CertFile       string `json:"cert_file"`
	KeyFile        string `json:"key_file"`
}

// Domain 邮件域名
type Domain struct {
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserCount int       `json:"user_count"`
	Aliases   []string  `json:"aliases"`
}

// MailUser 邮件用户
type MailUser struct {
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Domain    string    `json:"domain"`
	Quota     int64     `json:"quota"` // 配额（字节）
	Used      int64     `json:"used"`  // 已使用（字节）
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	LastLogin time.Time `json:"last_login"`
	Aliases   []string  `json:"aliases"`
	ForwardTo []string  `json:"forward_to"`
}

// Message 邮件消息
type Message struct {
	ID          string       `json:"id"`
	From        string       `json:"from"`
	To          []string     `json:"to"`
	Cc          []string     `json:"cc"`
	Bcc         []string     `json:"bcc"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body"`
	HTML        string       `json:"html"`
	Attachments []Attachment `json:"attachments"`
	Size        int64        `json:"size"`
	ReceivedAt  time.Time    `json:"received_at"`
	Read        bool         `json:"read"`
	Flagged     bool         `json:"flagged"`
	Deleted     bool         `json:"deleted"`
	Folder      string       `json:"folder"`
}

// Attachment 附件
type Attachment struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Content  []byte `json:"-"`
}

// SMTPServer SMTP 服务器
type SMTPServer struct {
	host      string
	port      int
	enableTLS bool
	certFile  string
	keyFile   string
}

// IMAPServer IMAP 服务器
type IMAPServer struct {
	host      string
	port      int
	enableTLS bool
	certFile  string
	keyFile   string
}

// NewMailServer 创建邮件服务器
func NewMailServer(config *Config) *MailServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &MailServer{
		domains:  make(map[string]*Domain),
		users:    make(map[string]*MailUser),
		messages: make(map[string]*Message),
		config:   config,
		smtpServer: &SMTPServer{
			host:      config.SMTPHost,
			port:      config.SMTPPort,
			enableTLS: config.EnableTLS,
			certFile:  config.CertFile,
			keyFile:   config.KeyFile,
		},
		imapServer: &IMAPServer{
			host:      config.IMAPHost,
			port:      config.IMAPPort,
			enableTLS: config.EnableTLS,
			certFile:  config.CertFile,
			keyFile:   config.KeyFile,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动邮件服务器
func (ms *MailServer) Start() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.running {
		return fmt.Errorf("mail server is already running")
	}

	// 启动 SMTP 服务器
	go ms.startSMTPServer()

	// 启动 IMAP 服务器
	go ms.startIMAPServer()

	ms.running = true
	return nil
}

// Stop 停止邮件服务器
func (ms *MailServer) Stop() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.running {
		return fmt.Errorf("mail server is not running")
	}

	ms.cancel()
	ms.running = false
	return nil
}

// AddDomain 添加域名
func (ms *MailServer) AddDomain(name string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.domains[name]; exists {
		return fmt.Errorf("domain %s already exists", name)
	}

	ms.domains[name] = &Domain{
		Name:      name,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return nil
}

// RemoveDomain 删除域名
func (ms *MailServer) RemoveDomain(name string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	domain, exists := ms.domains[name]
	if !exists {
		return fmt.Errorf("domain %s not found", name)
	}

	if domain.UserCount > 0 {
		return fmt.Errorf("domain %s has %d users, remove them first", name, domain.UserCount)
	}

	delete(ms.domains, name)
	return nil
}

// AddUser 添加邮件用户
func (ms *MailServer) AddUser(username, domain, password string, quota int64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	d, exists := ms.domains[domain]
	if !exists {
		return fmt.Errorf("domain %s not found", domain)
	}

	email := fmt.Sprintf("%s@%s", username, domain)
	if _, exists := ms.users[email]; exists {
		return fmt.Errorf("user %s already exists", email)
	}

	ms.users[email] = &MailUser{
		Username:  username,
		Email:     email,
		Password:  password,
		Domain:    domain,
		Quota:     quota,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	d.UserCount++
	return nil
}

// RemoveUser 删除邮件用户
func (ms *MailServer) RemoveUser(email string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	user, exists := ms.users[email]
	if !exists {
		return fmt.Errorf("user %s not found", email)
	}

	domain := ms.domains[user.Domain]
	if domain != nil {
		domain.UserCount--
	}

	delete(ms.users, email)
	return nil
}

// SendMessage 发送邮件
func (ms *MailServer) SendMessage(from string, to []string, subject, body, html string, attachments []Attachment) (*Message, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 验证发件人
	if _, exists := ms.users[from]; !exists {
		return nil, fmt.Errorf("sender %s not found", from)
	}

	// 计算消息大小
	size := int64(len(body) + len(html))
	for _, att := range attachments {
		size += att.Size
	}

	// 检查大小限制
	if size > ms.config.MaxMessageSize {
		return nil, fmt.Errorf("message size %d exceeds limit %d", size, ms.config.MaxMessageSize)
	}

	// 创建消息
	msg := &Message{
		ID:          fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		From:        from,
		To:          to,
		Subject:     subject,
		Body:        body,
		HTML:        html,
		Attachments: attachments,
		Size:        size,
		ReceivedAt:  time.Now(),
		Folder:      "INBOX",
	}

	ms.messages[msg.ID] = msg

	// 发送到收件人
	for _, recipient := range to {
		if user, exists := ms.users[recipient]; exists {
			user.Used += size
		}
	}

	return msg, nil
}

// GetMessages 获取用户邮件
func (ms *MailServer) GetMessages(email, folder string, page, pageSize int) ([]*Message, int, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if _, exists := ms.users[email]; !exists {
		return nil, 0, fmt.Errorf("user %s not found", email)
	}

	var messages []*Message
	for _, msg := range ms.messages {
		if msg.Folder == folder {
			for _, to := range msg.To {
				if to == email {
					messages = append(messages, msg)
					break
				}
			}
		}
	}

	total := len(messages)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return []*Message{}, total, nil
	}
	if end > total {
		end = total
	}

	return messages[start:end], total, nil
}

// GetStats 获取服务器统计
func (ms *MailServer) GetStats() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return map[string]interface{}{
		"domains":     len(ms.domains),
		"users":       len(ms.users),
		"messages":    len(ms.messages),
		"running":     ms.running,
		"smtp_port":   ms.config.SMTPPort,
		"imap_port":   ms.config.IMAPPort,
		"tls_enabled": ms.config.EnableTLS,
	}
}

// GetDomains 获取域名列表
func (ms *MailServer) GetDomains() []*Domain {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	domains := make([]*Domain, 0, len(ms.domains))
	for _, d := range ms.domains {
		domains = append(domains, d)
	}
	return domains
}

// GetUsers 获取用户列表
func (ms *MailServer) GetUsers(domain string) []*MailUser {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var users []*MailUser
	for _, u := range ms.users {
		if domain == "" || u.Domain == domain {
			users = append(users, u)
		}
	}
	return users
}

// startSMTPServer 启动 SMTP 服务器
func (ms *MailServer) startSMTPServer() {
	// SMTP 服务器实现
	// 实际实现需要处理 SMTP 协议
}

// startIMAPServer 启动 IMAP 服务器
func (ms *MailServer) startIMAPServer() {
	// IMAP 服务器实现
	// 实际实现需要处理 IMAP 协议
}

// SendSMTP 通过 SMTP 发送邮件
func (ms *MailServer) SendSMTP(from string, to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", ms.smtpServer.host, ms.smtpServer.port)
	auth := smtp.PlainAuth("", from, "", ms.smtpServer.host)
	return smtp.SendMail(addr, auth, from, to, msg)
}
