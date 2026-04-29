package emailmod

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

// Store 邮件审核数据存储.
type Store struct {
	db *sql.DB
}

// NewStore 创建数据存储.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.initDB(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败：%w", err)
	}
	return s, nil
}

// initDB 初始化数据库表.
func (s *Store) initDB() error {
	ctx := context.Background()
	schema := `
	CREATE TABLE IF NOT EXISTS emailmod_policies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1,
		priority INTEGER DEFAULT 100,
		sender_patterns TEXT DEFAULT '[]',
		recipient_patterns TEXT DEFAULT '[]',
		keywords TEXT DEFAULT '[]',
		attachment_types TEXT DEFAULT '[]',
		max_size_mb INTEGER DEFAULT 0,
		reviewers TEXT NOT NULL DEFAULT '[]',
		match_type TEXT DEFAULT 'exact',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_emailmod_policies_enabled ON emailmod_policies(enabled);
	CREATE INDEX IF NOT EXISTS idx_emailmod_policies_priority ON emailmod_policies(priority);

	CREATE TABLE IF NOT EXISTS emailmod_queue (
		id TEXT PRIMARY KEY,
		policy_id TEXT NOT NULL,
		policy_name TEXT DEFAULT '',
		message_id TEXT NOT NULL,
		from_addr TEXT NOT NULL,
		to_addrs TEXT NOT NULL DEFAULT '[]',
		cc_addrs TEXT DEFAULT '[]',
		subject TEXT DEFAULT '',
		body_preview TEXT DEFAULT '',
		attachments TEXT DEFAULT '[]',
		size_mb REAL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		current_level INTEGER DEFAULT 1,
		max_level INTEGER DEFAULT 1,
		reviews TEXT DEFAULT '[]',
		created_at DATETIME NOT NULL,
		reviewed_at DATETIME,
		expires_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_emailmod_queue_status ON emailmod_queue(status);
	CREATE INDEX IF NOT EXISTS idx_emailmod_queue_policy ON emailmod_queue(policy_id);
	CREATE INDEX IF NOT EXISTS idx_emailmod_queue_from ON emailmod_queue(from_addr);
	CREATE INDEX IF NOT EXISTS idx_emailmod_queue_created ON emailmod_queue(created_at);

	CREATE TABLE IF NOT EXISTS emailmod_audit (
		id TEXT PRIMARY KEY,
		queue_id TEXT NOT NULL,
		message_id TEXT NOT NULL,
		policy_id TEXT NOT NULL,
		policy_name TEXT DEFAULT '',
		from_addr TEXT NOT NULL,
		to_addrs TEXT DEFAULT '',
		subject TEXT DEFAULT '',
		status TEXT NOT NULL,
		reviewer_id TEXT DEFAULT '',
		reviewer_name TEXT DEFAULT '',
		comment TEXT DEFAULT '',
		action TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_emailmod_audit_status ON emailmod_audit(status);
	CREATE INDEX IF NOT EXISTS idx_emailmod_audit_policy ON emailmod_audit(policy_id);
	CREATE INDEX IF NOT EXISTS idx_emailmod_audit_reviewer ON emailmod_audit(reviewer_id);
	CREATE INDEX IF NOT EXISTS idx_emailmod_audit_created ON emailmod_audit(created_at);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Close 关闭存储.
func (s *Store) Close() error {
	return s.db.Close()
}

// ==================== 策略 CRUD ====================

// CreatePolicy 创建策略.
func (s *Store) CreatePolicy(p *Policy) error {
	ctx := context.Background()
	query := `INSERT INTO emailmod_policies (id, name, description, enabled, priority, sender_patterns,
		recipient_patterns, keywords, attachment_types, max_size_mb, reviewers, match_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		p.ID, p.Name, p.Description, boolToInt(p.Enabled), p.Priority,
		marshalJSON(p.SenderPatterns), marshalJSON(p.RecipientPatterns),
		marshalJSON(p.Keywords), marshalJSON(p.AttachmentTypes), p.MaxSizeMB,
		marshalJSON(p.Reviewers), string(p.MatchType), p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// GetPolicy 获取策略.
func (s *Store) GetPolicy(id string) (*Policy, error) {
	ctx := context.Background()
	query := `SELECT id, name, description, enabled, priority, sender_patterns, recipient_patterns,
		keywords, attachment_types, max_size_mb, reviewers, match_type, created_at, updated_at
		FROM emailmod_policies WHERE id = ?`

	p := &Policy{}
	var senderJSON, recipJSON, kwJSON, attachJSON, reviewersJSON string
	var enabled int
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Description, &enabled, &p.Priority,
		&senderJSON, &recipJSON, &kwJSON, &attachJSON, &p.MaxSizeMB,
		&reviewersJSON, &p.MatchType, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.Enabled = enabled != 0
	json.Unmarshal([]byte(senderJSON), &p.SenderPatterns)
	json.Unmarshal([]byte(recipJSON), &p.RecipientPatterns)
	json.Unmarshal([]byte(kwJSON), &p.Keywords)
	json.Unmarshal([]byte(attachJSON), &p.AttachmentTypes)
	json.Unmarshal([]byte(reviewersJSON), &p.Reviewers)

	return p, nil
}

// ListPolicies 列出所有策略.
func (s *Store) ListPolicies() ([]*Policy, error) {
	ctx := context.Background()
	query := `SELECT id, name, description, enabled, priority, sender_patterns, recipient_patterns,
		keywords, attachment_types, max_size_mb, reviewers, match_type, created_at, updated_at
		FROM emailmod_policies ORDER BY priority ASC, created_at DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*Policy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

// UpdatePolicy 更新策略.
func (s *Store) UpdatePolicy(p *Policy) error {
	ctx := context.Background()
	query := `UPDATE emailmod_policies SET name=?, description=?, enabled=?, priority=?,
		sender_patterns=?, recipient_patterns=?, keywords=?, attachment_types=?, max_size_mb=?,
		reviewers=?, match_type=?, updated_at=? WHERE id=?`

	result, err := s.db.ExecContext(ctx, query,
		p.Name, p.Description, boolToInt(p.Enabled), p.Priority,
		marshalJSON(p.SenderPatterns), marshalJSON(p.RecipientPatterns),
		marshalJSON(p.Keywords), marshalJSON(p.AttachmentTypes), p.MaxSizeMB,
		marshalJSON(p.Reviewers), string(p.MatchType), p.UpdatedAt, p.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf(ErrPolicyNotFound)
	}
	return nil
}

// DeletePolicy 删除策略.
func (s *Store) DeletePolicy(id string) error {
	ctx := context.Background()
	result, err := s.db.ExecContext(ctx, "DELETE FROM emailmod_policies WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf(ErrPolicyNotFound)
	}
	return nil
}

// GetEnabledPolicies 获取所有启用的策略.
func (s *Store) GetEnabledPolicies() ([]*Policy, error) {
	ctx := context.Background()
	query := `SELECT id, name, description, enabled, priority, sender_patterns, recipient_patterns,
		keywords, attachment_types, max_size_mb, reviewers, match_type, created_at, updated_at
		FROM emailmod_policies WHERE enabled = 1 ORDER BY priority ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*Policy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

// ==================== 队列 CRUD ====================

// CreateQueueItem 创建审核队列条目.
func (s *Store) CreateQueueItem(item *QueueItem) error {
	ctx := context.Background()
	query := `INSERT INTO emailmod_queue (id, policy_id, policy_name, message_id, from_addr, to_addrs,
		cc_addrs, subject, body_preview, attachments, size_mb, status, current_level, max_level,
		reviews, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		item.ID, item.PolicyID, item.PolicyName, item.MessageID,
		item.From, marshalJSON(item.To), marshalJSON(item.CC),
		item.Subject, item.BodyPreview, marshalJSON(item.Attachments),
		item.SizeMB, string(item.Status), item.CurrentLevel, item.MaxLevel,
		marshalJSON(item.Reviews), item.CreatedAt, item.ExpiresAt,
	)
	return err
}

// GetQueueItem 获取审核队列条目.
func (s *Store) GetQueueItem(id string) (*QueueItem, error) {
	ctx := context.Background()
	query := `SELECT id, policy_id, policy_name, message_id, from_addr, to_addrs,
		cc_addrs, subject, body_preview, attachments, size_mb, status, current_level, max_level,
		reviews, created_at, reviewed_at, expires_at
		FROM emailmod_queue WHERE id = ?`

	item := &QueueItem{}
	var toJSON, ccJSON, attachJSON, reviewsJSON string
	var reviewedAt, expiresAt sql.NullTime
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID, &item.PolicyID, &item.PolicyName, &item.MessageID,
		&item.From, &toJSON, &ccJSON, &item.Subject, &item.BodyPreview,
		&attachJSON, &item.SizeMB, &item.Status, &item.CurrentLevel, &item.MaxLevel,
		&reviewsJSON, &item.CreatedAt, &reviewedAt, &expiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(toJSON), &item.To)
	json.Unmarshal([]byte(ccJSON), &item.CC)
	json.Unmarshal([]byte(attachJSON), &item.Attachments)
	json.Unmarshal([]byte(reviewsJSON), &item.Reviews)
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}

	return item, nil
}

// QueryQueue 查询审核队列.
func (s *Store) QueryQueue(opts QueueQueryOptions) ([]*QueueItem, int, error) {
	ctx := context.Background()

	where := []string{"1=1"}
	args := []interface{}{}

	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(opts.Status))
	}
	if opts.PolicyID != "" {
		where = append(where, "policy_id = ?")
		args = append(args, opts.PolicyID)
	}
	if opts.From != "" {
		where = append(where, "from_addr LIKE ?")
		args = append(args, "%"+opts.From+"%")
	}
	if opts.Keyword != "" {
		where = append(where, "(subject LIKE ? OR from_addr LIKE ?)")
		kw := "%" + opts.Keyword + "%"
		args = append(args, kw, kw)
	}

	whereSQL := strings.Join(where, " AND ")

	// 统计总数
	var total int
	countSQL := "SELECT COUNT(*) FROM emailmod_queue WHERE " + whereSQL
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 分页查询
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	querySQL := `SELECT id, policy_id, policy_name, message_id, from_addr, to_addrs,
		cc_addrs, subject, body_preview, attachments, size_mb, status, current_level, max_level,
		reviews, created_at, reviewed_at, expires_at
		FROM emailmod_queue WHERE ` + whereSQL + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*QueueItem
	for rows.Next() {
		item, err := scanQueueItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

// UpdateQueueItem 更新审核队列条目.
func (s *Store) UpdateQueueItem(item *QueueItem) error {
	ctx := context.Background()
	query := `UPDATE emailmod_queue SET status=?, current_level=?, reviews=?, reviewed_at=? WHERE id=?`

	result, err := s.db.ExecContext(ctx, query,
		string(item.Status), item.CurrentLevel,
		marshalJSON(item.Reviews), item.ReviewedAt, item.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf(ErrQueueItemNotFound)
	}
	return nil
}

// ==================== 审计记录 ====================

// CreateAuditEntry 创建审计记录.
func (s *Store) CreateAuditEntry(entry *AuditEntry) error {
	ctx := context.Background()
	query := `INSERT INTO emailmod_audit (id, queue_id, message_id, policy_id, policy_name,
		from_addr, to_addrs, subject, status, reviewer_id, reviewer_name, comment, action, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		entry.ID, entry.QueueID, entry.MessageID, entry.PolicyID, entry.PolicyName,
		entry.From, entry.To, entry.Subject, string(entry.Status),
		entry.ReviewerID, entry.ReviewerName, entry.Comment, entry.Action, entry.CreatedAt,
	)
	return err
}

// QueryAudit 查询审计记录.
func (s *Store) QueryAudit(opts AuditQueryOptions) ([]*AuditEntry, int, error) {
	ctx := context.Background()

	where := []string{"1=1"}
	args := []interface{}{}

	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(opts.Status))
	}
	if opts.PolicyID != "" {
		where = append(where, "policy_id = ?")
		args = append(args, opts.PolicyID)
	}
	if opts.ReviewerID != "" {
		where = append(where, "reviewer_id = ?")
		args = append(args, opts.ReviewerID)
	}
	if opts.Keyword != "" {
		where = append(where, "(subject LIKE ? OR from_addr LIKE ? OR comment LIKE ?)")
		kw := "%" + opts.Keyword + "%"
		args = append(args, kw, kw, kw)
	}
	if opts.StartTime != nil {
		where = append(where, "created_at >= ?")
		args = append(args, *opts.StartTime)
	}
	if opts.EndTime != nil {
		where = append(where, "created_at <= ?")
		args = append(args, *opts.EndTime)
	}

	whereSQL := strings.Join(where, " AND ")

	// 总数
	var total int
	countSQL := "SELECT COUNT(*) FROM emailmod_audit WHERE " + whereSQL
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 分页
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	querySQL := `SELECT id, queue_id, message_id, policy_id, policy_name, from_addr, to_addrs,
		subject, status, reviewer_id, reviewer_name, comment, action, created_at
		FROM emailmod_audit WHERE ` + whereSQL + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, entry)
	}
	return entries, total, nil
}

// GetStats 获取审核统计.
func (s *Store) GetStats() (*Stats, error) {
	ctx := context.Background()
	stats := &Stats{
		ByPolicy:   make(map[string]int),
		ByReviewer: make(map[string]int),
	}

	// 队列统计
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emailmod_queue WHERE status = 'pending'").Scan(&stats.TotalPending)
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emailmod_queue WHERE status = 'approved'").Scan(&stats.TotalApproved)
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emailmod_queue WHERE status = 'rejected'").Scan(&stats.TotalRejected)

	// 今日处理
	today := time.Now().Format("2006-01-02")
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emailmod_audit WHERE created_at >= ?", today).Scan(&stats.TodayProcessed)

	// 按策略统计
	policyRows, err := s.db.QueryContext(ctx, "SELECT policy_name, COUNT(*) FROM emailmod_queue GROUP BY policy_name")
	if err == nil {
		defer policyRows.Close()
		for policyRows.Next() {
			var name string
			var count int
			if policyRows.Scan(&name, &count) == nil {
				stats.ByPolicy[name] = count
			}
		}
	}

	// 按审核人统计
	reviewerRows, err := s.db.QueryContext(ctx, "SELECT reviewer_name, COUNT(*) FROM emailmod_audit WHERE reviewer_name != '' GROUP BY reviewer_name")
	if err == nil {
		defer reviewerRows.Close()
		for reviewerRows.Next() {
			var name string
			var count int
			if reviewerRows.Scan(&name, &count) == nil {
				stats.ByReviewer[name] = count
			}
		}
	}

	return stats, nil
}

// ==================== 辅助函数 ====================

// scanPolicy 扫描策略行.
func scanPolicy(rows *sql.Rows) (*Policy, error) {
	p := &Policy{}
	var senderJSON, recipJSON, kwJSON, attachJSON, reviewersJSON string
	var enabled int
	err := rows.Scan(
		&p.ID, &p.Name, &p.Description, &enabled, &p.Priority,
		&senderJSON, &recipJSON, &kwJSON, &attachJSON, &p.MaxSizeMB,
		&reviewersJSON, &p.MatchType, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	p.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(senderJSON), &p.SenderPatterns)
	_ = json.Unmarshal([]byte(recipJSON), &p.RecipientPatterns)
	_ = json.Unmarshal([]byte(kwJSON), &p.Keywords)
	_ = json.Unmarshal([]byte(attachJSON), &p.AttachmentTypes)
	_ = json.Unmarshal([]byte(reviewersJSON), &p.Reviewers)

	return p, nil
}

// scanQueueItem 扫描队列条目行.
func scanQueueItem(rows *sql.Rows) (*QueueItem, error) {
	item := &QueueItem{}
	var toJSON, ccJSON, attachJSON, reviewsJSON string
	var reviewedAt, expiresAt sql.NullTime
	err := rows.Scan(
		&item.ID, &item.PolicyID, &item.PolicyName, &item.MessageID,
		&item.From, &toJSON, &ccJSON, &item.Subject, &item.BodyPreview,
		&attachJSON, &item.SizeMB, &item.Status, &item.CurrentLevel, &item.MaxLevel,
		&reviewsJSON, &item.CreatedAt, &reviewedAt, &expiresAt,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(toJSON), &item.To)
	_ = json.Unmarshal([]byte(ccJSON), &item.CC)
	_ = json.Unmarshal([]byte(attachJSON), &item.Attachments)
	_ = json.Unmarshal([]byte(reviewsJSON), &item.Reviews)
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}

	return item, nil
}

// scanAuditEntry 扫描审计行.
func scanAuditEntry(rows *sql.Rows) (*AuditEntry, error) {
	entry := &AuditEntry{}
	err := rows.Scan(
		&entry.ID, &entry.QueueID, &entry.MessageID, &entry.PolicyID, &entry.PolicyName,
		&entry.From, &entry.To, &entry.Subject, &entry.Status,
		&entry.ReviewerID, &entry.ReviewerName, &entry.Comment, &entry.Action, &entry.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// marshalJSON 序列化为 JSON 字符串.
func marshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// boolToInt 布尔转整数.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
