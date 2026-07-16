// Package emailserver 提供邮件服务器核心业务逻辑
package emailserver

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 邮件服务器管理器.
type Manager struct {
	accounts map[string]*EmailAccount
	messages map[string]*EmailMessage
	rules    map[string]*FilterRule
	smtpCfg  *SMTPConfig
	imapCfg  *IMAPConfig
	antispam *AntispamConfig
	mu       sync.RWMutex
}

// NewManager 创建邮件服务器管理器.
func NewManager() *Manager {
	return &Manager{
		accounts: make(map[string]*EmailAccount),
		messages: make(map[string]*EmailMessage),
		rules:    make(map[string]*FilterRule),
		smtpCfg: &SMTPConfig{
			Enabled:      true,
			Port:         25,
			Domain:       "localhost",
			MaxMessageMB: 25,
			RequireAuth:  true,
			EnableTLS:    true,
		},
		imapCfg: &IMAPConfig{
			Enabled:   true,
			Port:      143,
			EnableTLS: true,
		},
		antispam: &AntispamConfig{
			Enabled:    true,
			Threshold:  50,
			RejectSpam: false,
		},
	}
}

// ========== SMTP/IMAP 服务管理 ==========

// GetSMTPConfig 获取 SMTP 配置.
func (m *Manager) GetSMTPConfig() *SMTPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.smtpCfg
}

// UpdateSMTPConfig 更新 SMTP 配置.
func (m *Manager) UpdateSMTPConfig(req UpdateSMTPRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Enabled != nil {
		m.smtpCfg.Enabled = *req.Enabled
	}
	if req.Port != nil {
		m.smtpCfg.Port = *req.Port
	}
	if req.Domain != nil {
		m.smtpCfg.Domain = *req.Domain
	}
	if req.MaxMessageMB != nil {
		m.smtpCfg.MaxMessageMB = *req.MaxMessageMB
	}
	if req.RequireAuth != nil {
		m.smtpCfg.RequireAuth = *req.RequireAuth
	}
	if req.EnableTLS != nil {
		m.smtpCfg.EnableTLS = *req.EnableTLS
	}

	log.Printf("[emailserver] SMTP 配置已更新: port=%d, domain=%s, tls=%v",
		m.smtpCfg.Port, m.smtpCfg.Domain, m.smtpCfg.EnableTLS)
}

// GetIMAPConfig 获取 IMAP 配置.
func (m *Manager) GetIMAPConfig() *IMAPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.imapCfg
}

// UpdateIMAPConfig 更新 IMAP 配置.
func (m *Manager) UpdateIMAPConfig(req UpdateIMAPRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Enabled != nil {
		m.imapCfg.Enabled = *req.Enabled
	}
	if req.Port != nil {
		m.imapCfg.Port = *req.Port
	}
	if req.EnableTLS != nil {
		m.imapCfg.EnableTLS = *req.EnableTLS
	}

	log.Printf("[emailserver] IMAP 配置已更新: port=%d, tls=%v",
		m.imapCfg.Port, m.imapCfg.EnableTLS)
}

// ========== 邮件账户管理 ==========

// CreateAccount 创建邮件账户.
func (m *Manager) CreateAccount(req CreateAccountRequest) *EmailAccount {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	quota := req.QuotaMB
	if quota == 0 {
		quota = 1024 // 默认 1GB
	}

	acct := &EmailAccount{
		ID:          uuid.New().String(),
		Username:    req.Username,
		Domain:      req.Domain,
		Email:       fmt.Sprintf("%s@%s", req.Username, req.Domain),
		Password:    req.Password, // 实际应 hash
		DisplayName: req.DisplayName,
		QuotaMB:     quota,
		UsedMB:      0,
		IsAdmin:     req.IsAdmin,
		IsActive:    true,
		Aliases:     []string{},
		Forwards:    []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.accounts[acct.ID] = acct
	log.Printf("[emailserver] 创建邮件账户: %s (%s)", acct.Email, acct.ID)
	return acct
}

// GetAccount 获取邮件账户.
func (m *Manager) GetAccount(id string) (*EmailAccount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	acct, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account %q not found", id)
	}
	return acct, nil
}

// ListAccounts 列出所有邮件账户.
func (m *Manager) ListAccounts() []*EmailAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()

	accounts := make([]*EmailAccount, 0, len(m.accounts))
	for _, a := range m.accounts {
		accounts = append(accounts, a)
	}

	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Email < accounts[j].Email
	})

	return accounts
}

// UpdateAccount 更新邮件账户.
func (m *Manager) UpdateAccount(id string, req UpdateAccountRequest) (*EmailAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	acct, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account %q not found", id)
	}

	if req.DisplayName != nil {
		acct.DisplayName = *req.DisplayName
	}
	if req.Password != nil {
		acct.Password = *req.Password // 实际应 hash
	}
	if req.QuotaMB != nil {
		acct.QuotaMB = *req.QuotaMB
	}
	if req.IsAdmin != nil {
		acct.IsAdmin = *req.IsAdmin
	}
	if req.IsActive != nil {
		acct.IsActive = *req.IsActive
	}
	if req.Aliases != nil {
		acct.Aliases = req.Aliases
	}
	if req.Forwards != nil {
		acct.Forwards = req.Forwards
	}
	acct.UpdatedAt = time.Now()

	log.Printf("[emailserver] 更新邮件账户: %s", acct.Email)
	return acct, nil
}

// DeleteAccount 删除邮件账户.
func (m *Manager) DeleteAccount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acct, ok := m.accounts[id]
	if !ok {
		return fmt.Errorf("account %q not found", id)
	}

	// 删除该账户的所有邮件
	for mid, msg := range m.messages {
		if msg.AccountID == id {
			delete(m.messages, mid)
		}
	}

	delete(m.accounts, id)
	log.Printf("[emailserver] 删除邮件账户: %s", acct.Email)
	return nil
}

// ========== 邮件收发功能 ==========

// SendEmail 发送邮件.
func (m *Manager) SendEmail(req SendEmailRequest) (*EmailMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证发件人账户存在
	var senderID string
	for _, acct := range m.accounts {
		if acct.Email == req.From && acct.IsActive {
			senderID = acct.ID
			break
		}
	}
	if senderID == "" {
		return nil, fmt.Errorf("sender %q not found or inactive", req.From)
	}

	// 检查邮件大小限制
	msgSize := int64(len(req.Subject) + len(req.Body))

	now := time.Now()
	msg := &EmailMessage{
		ID:        uuid.New().String(),
		AccountID: senderID,
		Folder:    "sent",
		From:      req.From,
		To:        req.To,
		CC:        req.CC,
		BCC:       req.BCC,
		Subject:   req.Subject,
		Body:      req.Body,
		IsRead:    true,
		IsStarred: false,
		HasAttach: false,
		CreatedAt: now,
		Size:      msgSize,
	}

	m.messages[msg.ID] = msg

	// 模拟投递到收件人 (实际应通过 SMTP 转发)
	for _, to := range req.To {
		for _, acct := range m.accounts {
			if acct.Email == to && acct.IsActive {
				inboxMsg := &EmailMessage{
					ID:        uuid.New().String(),
					AccountID: acct.ID,
					Folder:    "inbox",
					From:      req.From,
					To:        []string{to},
					CC:        req.CC,
					Subject:   req.Subject,
					Body:      req.Body,
					IsRead:    false,
					IsStarred: false,
					CreatedAt: now,
					Size:      msgSize,
				}
				m.messages[inboxMsg.ID] = inboxMsg
			}
		}
	}

	log.Printf("[emailserver] 邮件已发送: %s -> %v, subject=%q",
		req.From, req.To, req.Subject)
	return msg, nil
}

// ListMessages 获取邮件列表.
func (m *Manager) ListMessages(accountID, folder string, page, pageSize int) ([]*EmailMessage, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var result []*EmailMessage
	for _, msg := range m.messages {
		if msg.AccountID == accountID {
			if folder == "" || msg.Folder == folder {
				result = append(result, msg)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	total := len(result)
	start := (page - 1) * pageSize
	if start >= total {
		return []*EmailMessage{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return result[start:end], total
}

// GetMessagesForAccount 获取账户邮件列表（简化版本，不分页）.
func (m *Manager) GetMessagesForAccount(accountID, folder string) []*EmailMessage {
	msgs, _ := m.ListMessages(accountID, folder, 1, 1000)
	return msgs
}

// MarkAsRead 标记邮件已读.
func (m *Manager) MarkAsRead(messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[messageID]
	if !ok {
		return fmt.Errorf("message %q not found", messageID)
	}
	msg.IsRead = true
	return nil
}

// ToggleStar 切换星标.
func (m *Manager) ToggleStar(messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[messageID]
	if !ok {
		return fmt.Errorf("message %q not found", messageID)
	}
	msg.IsStarred = !msg.IsStarred
	return nil
}

// MoveMessage 移动邮件到指定文件夹.
func (m *Manager) MoveMessage(messageID, folder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[messageID]
	if !ok {
		return fmt.Errorf("message %q not found", messageID)
	}

	validFolders := map[string]bool{
		"inbox": true, "sent": true, "drafts": true,
		"trash": true, "spam": true, "archive": true,
	}
	if !validFolders[folder] {
		return fmt.Errorf("invalid folder %q", folder)
	}

	msg.Folder = folder
	return nil
}

// DeleteMessage 删除邮件（移到回收站）.
func (m *Manager) DeleteMessage(messageID string) error {
	return m.MoveMessage(messageID, "trash")
}

// ========== 邮件过滤规则 ==========

// CreateFilterRule 创建过滤规则.
func (m *Manager) CreateFilterRule(req CreateFilterRuleRequest) *FilterRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule := &FilterRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Priority:    req.Priority,
		Enabled:     true,
		Condition:   req.Condition,
		MatchType:   req.MatchType,
		MatchValue:  req.MatchValue,
		Action:      req.Action,
		ActionValue: req.ActionValue,
	}

	m.rules[rule.ID] = rule
	log.Printf("[emailserver] 创建过滤规则: %s", rule.Name)
	return rule
}

// ListFilterRules 列出所有过滤规则.
func (m *Manager) ListFilterRules() []*FilterRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*FilterRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	return rules
}

// DeleteFilterRule 删除过滤规则.
func (m *Manager) DeleteFilterRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("filter rule %q not found", id)
	}

	delete(m.rules, id)
	return nil
}

// ApplyFilterRules 对邮件应用过滤规则.
func (m *Manager) ApplyFilterRules(msg *EmailMessage) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := m.ListFilterRules()
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		matched := false
		switch rule.Condition {
		case "from":
			matched = matchValue(msg.From, rule.MatchType, rule.MatchValue)
		case "to":
			for _, to := range msg.To {
				if matchValue(to, rule.MatchType, rule.MatchValue) {
					matched = true
					break
				}
			}
		case "subject":
			matched = matchValue(msg.Subject, rule.MatchType, rule.MatchValue)
		case "body":
			matched = matchValue(msg.Body, rule.MatchType, rule.MatchValue)
		}

		if matched {
			return rule.Action
		}
	}

	return ""
}

// matchValue 匹配值.
func matchValue(value, matchType, matchValue string) bool {
	switch matchType {
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(matchValue))
	case "equals":
		return strings.EqualFold(value, matchValue)
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(matchValue))
	default:
		return false
	}
}

// ========== 反垃圾邮件配置 ==========

// GetAntispamConfig 获取反垃圾邮件配置.
func (m *Manager) GetAntispamConfig() *AntispamConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.antispam
}

// UpdateAntispamConfig 更新反垃圾邮件配置.
func (m *Manager) UpdateAntispamConfig(req UpdateAntispamRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Enabled != nil {
		m.antispam.Enabled = *req.Enabled
	}
	if req.Threshold != nil {
		m.antispam.Threshold = *req.Threshold
	}
	if req.BlacklistDomains != nil {
		m.antispam.BlacklistDomains = req.BlacklistDomains
	}
	if req.BlacklistAddrs != nil {
		m.antispam.BlacklistAddrs = req.BlacklistAddrs
	}
	if req.WhitelistAddrs != nil {
		m.antispam.WhitelistAddrs = req.WhitelistAddrs
	}
	if req.RejectSpam != nil {
		m.antispam.RejectSpam = *req.RejectSpam
	}

	log.Printf("[emailserver] 反垃圾邮件配置已更新: enabled=%v, threshold=%d",
		m.antispam.Enabled, m.antispam.Threshold)
}

// IsSpam 检查邮件是否为垃圾邮件.
func (m *Manager) IsSpam(from string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.antispam.Enabled {
		return false
	}

	// 检查白名单
	for _, addr := range m.antispam.WhitelistAddrs {
		if strings.EqualFold(addr, from) {
			return false
		}
	}

	// 检查黑名单
	for _, addr := range m.antispam.BlacklistAddrs {
		if strings.EqualFold(addr, from) {
			return true
		}
	}

	// 检查域名黑名单
	parts := strings.Split(from, "@")
	if len(parts) == 2 {
		domain := parts[1]
		for _, d := range m.antispam.BlacklistDomains {
			if strings.EqualFold(d, domain) {
				return true
			}
		}
	}

	return false
}

// ========== 邮件备份与归档 ==========

// ArchiveMessage 归档邮件.
func (m *Manager) ArchiveMessage(messageID string) error {
	return m.MoveMessage(messageID, "archive")
}

// GetStats 获取邮件服务器统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalAccounts := len(m.accounts)
	activeAccounts := 0
	for _, a := range m.accounts {
		if a.IsActive {
			activeAccounts++
		}
	}

	totalMessages := len(m.messages)
	folderCounts := make(map[string]int)
	for _, msg := range m.messages {
		folderCounts[msg.Folder]++
	}

	return map[string]interface{}{
		"total_accounts":   totalAccounts,
		"active_accounts":  activeAccounts,
		"total_messages":   totalMessages,
		"folder_counts":    folderCounts,
		"filter_rules":     len(m.rules),
		"smtp_enabled":     m.smtpCfg.Enabled,
		"imap_enabled":     m.imapCfg.Enabled,
		"antispam_enabled": m.antispam.Enabled,
	}
}
