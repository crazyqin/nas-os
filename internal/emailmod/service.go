package emailmod

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
)

// Manager 邮件审核管理器.
type Manager struct {
	store *Store
}

// NewManager 创建邮件审核管理器.
func NewManager(db *sql.DB) (*Manager, error) {
	store, err := NewStore(db)
	if err != nil {
		return nil, err
	}
	return &Manager{store: store}, nil
}

// NewManagerWithStore 使用已有的 Store 创建管理器.
func NewManagerWithStore(store *Store) *Manager {
	return &Manager{store: store}
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	return m.store.Close()
}

// Store 获取底层存储.
func (m *Manager) Store() *Store {
	return m.store
}

// ==================== 策略管理 ====================

// CreatePolicy 创建审核策略.
func (m *Manager) CreatePolicy(input PolicyInput) (*Policy, error) {
	if len(input.Reviewers) == 0 {
		return nil, errors.New(ErrEmptyReviewers)
	}

	// 设置默认值
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	matchType := input.MatchType
	if matchType == "" {
		matchType = MatchExact
	}

	now := time.Now()
	p := &Policy{
		ID:                generateID(),
		Name:              input.Name,
		Description:       input.Description,
		Enabled:           enabled,
		Priority:          input.Priority,
		SenderPatterns:    input.SenderPatterns,
		RecipientPatterns: input.RecipientPatterns,
		Keywords:          input.Keywords,
		AttachmentTypes:   input.AttachmentTypes,
		MaxSizeMB:         input.MaxSizeMB,
		Reviewers:         input.Reviewers,
		MatchType:         matchType,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := m.store.CreatePolicy(p); err != nil {
		return nil, fmt.Errorf("创建策略失败：%w", err)
	}
	return p, nil
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(id string) (*Policy, error) {
	p, err := m.store.GetPolicy(id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New(ErrPolicyNotFound)
	}
	return p, nil
}

// ListPolicies 列出策略.
func (m *Manager) ListPolicies() ([]*Policy, error) {
	return m.store.ListPolicies()
}

// UpdatePolicy 更新策略.
func (m *Manager) UpdatePolicy(id string, input PolicyInput) (*Policy, error) {
	existing, err := m.store.GetPolicy(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New(ErrPolicyNotFound)
	}

	if len(input.Reviewers) == 0 {
		return nil, errors.New(ErrEmptyReviewers)
	}

	// 更新字段
	existing.Name = input.Name
	existing.Description = input.Description
	existing.Priority = input.Priority
	existing.SenderPatterns = input.SenderPatterns
	existing.RecipientPatterns = input.RecipientPatterns
	existing.Keywords = input.Keywords
	existing.AttachmentTypes = input.AttachmentTypes
	existing.MaxSizeMB = input.MaxSizeMB
	existing.Reviewers = input.Reviewers
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.MatchType != "" {
		existing.MatchType = input.MatchType
	}
	existing.UpdatedAt = time.Now()

	if err := m.store.UpdatePolicy(existing); err != nil {
		return nil, fmt.Errorf("更新策略失败：%w", err)
	}
	return existing, nil
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	_, err := m.store.GetPolicy(id)
	if err != nil {
		return err
	}
	return m.store.DeletePolicy(id)
}

// ==================== 邮件审核 ====================

// SubmitEmail 提交邮件到审核队列（匹配策略并创建审核记录）.
func (m *Manager) SubmitEmail(from string, to []string, cc []string, subject, body string, attachments []Attachment) (*QueueItem, error) {
	policies, err := m.store.GetEnabledPolicies()
	if err != nil {
		return nil, fmt.Errorf("获取策略失败：%w", err)
	}

	// 按优先级顺序匹配策略
	for _, policy := range policies {
		if m.matchPolicy(policy, from, to, subject, body, attachments) {
			// 计算附件总大小
			var totalSizeMB float64
			for _, a := range attachments {
				totalSizeMB += a.SizeMB
			}

			// 计算最大审核级别
			maxLevel := 1
			for _, r := range policy.Reviewers {
				if r.Level > maxLevel {
					maxLevel = r.Level
				}
			}

			item := &QueueItem{
				ID:           generateID(),
				PolicyID:     policy.ID,
				PolicyName:   policy.Name,
				MessageID:    uuid.New().String(),
				From:         from,
				To:           to,
				CC:           cc,
				Subject:      subject,
				BodyPreview:  truncateString(body, 500),
				Attachments:  attachments,
				SizeMB:       totalSizeMB,
				Status:       StatusPending,
				CurrentLevel: 1,
				MaxLevel:     maxLevel,
				Reviews:      make([]ReviewLog, 0),
				CreatedAt:    time.Now(),
			}

			if err := m.store.CreateQueueItem(item); err != nil {
				return nil, fmt.Errorf("创建审核队列失败：%w", err)
			}

			// 写审计日志
			_ = m.store.CreateAuditEntry(&AuditEntry{
				ID:         generateID(),
				QueueID:    item.ID,
				MessageID:  item.MessageID,
				PolicyID:   policy.ID,
				PolicyName: policy.Name,
				From:       from,
				To:         strings.Join(to, ","),
				Subject:    subject,
				Status:     StatusPending,
				Action:     "submit",
				CreatedAt:  time.Now(),
			})

			return item, nil
		}
	}

	return nil, nil // 无匹配策略，放行
}

// Approve 批准邮件.
func (m *Manager) Approve(queueID, reviewerID, reviewerName, comment string) (*QueueItem, error) {
	item, err := m.store.GetQueueItem(queueID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New(ErrQueueItemNotFound)
	}
	if item.Status != StatusPending {
		return nil, errors.New(ErrAlreadyReviewed)
	}

	// 验证审核人是否是当前级别的审核人
	if !m.isReviewer(item, reviewerID, item.CurrentLevel) {
		return nil, errors.New(ErrNotCurrentReviewer)
	}

	// 记录审核
	now := time.Now()
	review := ReviewLog{
		Level:      item.CurrentLevel,
		UserID:     reviewerID,
		Username:   reviewerName,
		Status:     StatusApproved,
		Comment:    comment,
		ReviewedAt: now,
	}
	item.Reviews = append(item.Reviews, review)

	if item.CurrentLevel < item.MaxLevel {
		// 还有上级审核，推进到下一级
		item.CurrentLevel++
	} else {
		// 最高级别批准，通过
		item.Status = StatusApproved
		item.ReviewedAt = &now
	}

	if err := m.store.UpdateQueueItem(item); err != nil {
		return nil, err
	}

	// 审计
	_ = m.store.CreateAuditEntry(&AuditEntry{
		ID:           generateID(),
		QueueID:      item.ID,
		MessageID:    item.MessageID,
		PolicyID:     item.PolicyID,
		PolicyName:   item.PolicyName,
		From:         item.From,
		To:           strings.Join(item.To, ","),
		Subject:      item.Subject,
		Status:       item.Status,
		ReviewerID:   reviewerID,
		ReviewerName: reviewerName,
		Comment:      comment,
		Action:       "approve",
		CreatedAt:    now,
	})

	return item, nil
}

// Reject 拒绝邮件.
func (m *Manager) Reject(queueID, reviewerID, reviewerName, comment string) (*QueueItem, error) {
	item, err := m.store.GetQueueItem(queueID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New(ErrQueueItemNotFound)
	}
	if item.Status != StatusPending {
		return nil, errors.New(ErrAlreadyReviewed)
	}

	// 验证审核人
	if !m.isReviewer(item, reviewerID, item.CurrentLevel) {
		return nil, errors.New(ErrNotCurrentReviewer)
	}

	// 记录审核
	now := time.Now()
	review := ReviewLog{
		Level:      item.CurrentLevel,
		UserID:     reviewerID,
		Username:   reviewerName,
		Status:     StatusRejected,
		Comment:    comment,
		ReviewedAt: now,
	}
	item.Reviews = append(item.Reviews, review)
	item.Status = StatusRejected
	item.ReviewedAt = &now

	if err := m.store.UpdateQueueItem(item); err != nil {
		return nil, err
	}

	// 审计
	_ = m.store.CreateAuditEntry(&AuditEntry{
		ID:           generateID(),
		QueueID:      item.ID,
		MessageID:    item.MessageID,
		PolicyID:     item.PolicyID,
		PolicyName:   item.PolicyName,
		From:         item.From,
		To:           strings.Join(item.To, ","),
		Subject:      item.Subject,
		Status:       StatusRejected,
		ReviewerID:   reviewerID,
		ReviewerName: reviewerName,
		Comment:      comment,
		Action:       "reject",
		CreatedAt:    now,
	})

	return item, nil
}

// GetQueueItem 获取队列条目.
func (m *Manager) GetQueueItem(id string) (*QueueItem, error) {
	item, err := m.store.GetQueueItem(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New(ErrQueueItemNotFound)
	}
	return item, nil
}

// QueryQueue 查询审核队列.
func (m *Manager) QueryQueue(opts QueueQueryOptions) ([]*QueueItem, int, error) {
	return m.store.QueryQueue(opts)
}

// QueryAudit 查询审计记录.
func (m *Manager) QueryAudit(opts AuditQueryOptions) ([]*AuditEntry, int, error) {
	return m.store.QueryAudit(opts)
}

// GetStats 获取统计.
func (m *Manager) GetStats() (*Stats, error) {
	return m.store.GetStats()
}

// ==================== 策略匹配 ====================

// matchPolicy 检查邮件是否匹配策略.
func (m *Manager) matchPolicy(p *Policy, from string, to []string, subject, body string, attachments []Attachment) bool {
	matched := false
	hasCondition := false

	// 发件人匹配
	if len(p.SenderPatterns) > 0 {
		hasCondition = true
		if matchPatterns(from, p.SenderPatterns, p.MatchType) {
			matched = true
		} else {
			return false
		}
	}

	// 收件人匹配
	if len(p.RecipientPatterns) > 0 {
		hasCondition = true
		recipMatched := false
		for _, r := range to {
			if matchPatterns(r, p.RecipientPatterns, p.MatchType) {
				recipMatched = true
				break
			}
		}
		if !recipMatched {
			return false
		}
		matched = true
	}

	// 关键词匹配（主题+正文）
	if len(p.Keywords) > 0 {
		hasCondition = true
		content := strings.ToLower(subject + " " + body)
		kwMatched := false
		for _, kw := range p.Keywords {
			if strings.Contains(content, strings.ToLower(kw)) {
				kwMatched = true
				break
			}
		}
		if !kwMatched {
			return false
		}
		matched = true
	}

	// 附件类型匹配
	if len(p.AttachmentTypes) > 0 {
		hasCondition = true
		attachMatched := false
		for _, att := range attachments {
			for _, ext := range p.AttachmentTypes {
				if strings.HasSuffix(strings.ToLower(att.Name), strings.ToLower(ext)) {
					attachMatched = true
					break
				}
			}
			if attachMatched {
				break
			}
		}
		if !attachMatched {
			return false
		}
		matched = true
	}

	// 附件大小匹配
	if p.MaxSizeMB > 0 {
		hasCondition = true
		var totalSize float64
		for _, att := range attachments {
			totalSize += att.SizeMB
		}
		if totalSize <= float64(p.MaxSizeMB) {
			return false
		}
		matched = true
	}

	if !hasCondition {
		return false
	}

	return matched
}

// isReviewer 检查用户是否为指定级别的审核人.
func (m *Manager) isReviewer(item *QueueItem, userID string, level int) bool {
	// 从策略中获取审核人列表
	policy, err := m.store.GetPolicy(item.PolicyID)
	if err != nil {
		return false
	}

	for _, r := range policy.Reviewers {
		if r.UserID == userID && r.Level == level {
			return true
		}
	}
	return false
}

// matchPatterns 检查值是否匹配模式列表.
func matchPatterns(value string, patterns []string, matchType MatchType) bool {
	lower := strings.ToLower(value)
	for _, p := range patterns {
		pattern := strings.ToLower(p)
		switch matchType {
		case MatchExact:
			if lower == pattern {
				return true
			}
		case MatchDomain:
			// 匹配域名后缀，如 @example.com
			if strings.HasSuffix(lower, pattern) || lower == strings.TrimPrefix(pattern, "@") {
				return true
			}
		case MatchGlob:
			if matchGlob(lower, pattern) {
				return true
			}
		case MatchRegex:
			if matched, _ := regexp.MatchString(pattern, lower); matched {
				return true
			}
		default:
			if strings.Contains(lower, pattern) {
				return true
			}
		}
	}
	return false
}

// matchGlob 简单通配符匹配（* 和 ?）.
func matchGlob(s, pattern string) bool {
	// 简单实现：将 * 转换为正则 .*
	regexStr := "^"
	for _, c := range pattern {
		switch c {
		case '*':
			regexStr += ".*"
		case '?':
			regexStr += "."
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			regexStr += "\\" + string(c)
		default:
			regexStr += string(c)
		}
	}
	regexStr += "$"
	matched, _ := regexp.MatchString(regexStr, s)
	return matched
}

// truncateString 截断字符串.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// generateID 生成随机 ID.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}
