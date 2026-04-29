package aiconsole

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
)

// Store 持久化存储（SQLite）.
type Store struct {
	db *sql.DB
}

// NewStore 创建存储实例.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return s, nil
}

// migrate 创建表结构.
func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS ai_models (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			provider    TEXT NOT NULL,
			endpoint    TEXT NOT NULL,
			api_key     TEXT NOT NULL DEFAULT '',
			model_name  TEXT NOT NULL,
			max_tokens  INTEGER NOT NULL DEFAULT 4096,
			temperature REAL NOT NULL DEFAULT 0.7,
			status      TEXT NOT NULL DEFAULT 'active',
			is_default  INTEGER NOT NULL DEFAULT 0,
			enabled     INTEGER NOT NULL DEFAULT 1,
			description TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ai_redact_rules (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			pii_type    TEXT NOT NULL,
			pattern     TEXT NOT NULL,
			strategy    TEXT NOT NULL DEFAULT 'mask',
			mask_char   TEXT NOT NULL DEFAULT '*',
			show_first  INTEGER NOT NULL DEFAULT 0,
			show_last   INTEGER NOT NULL DEFAULT 0,
			replacement TEXT NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 1,
			priority    INTEGER NOT NULL DEFAULT 0,
			description TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ai_audit_logs (
			id               TEXT PRIMARY KEY,
			timestamp        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			user_id          TEXT NOT NULL DEFAULT '',
			username         TEXT NOT NULL DEFAULT '',
			model_id         TEXT NOT NULL DEFAULT '',
			model_name       TEXT NOT NULL DEFAULT '',
			action           TEXT NOT NULL DEFAULT '',
			request_summary  TEXT NOT NULL DEFAULT '',
			response_summary TEXT NOT NULL DEFAULT '',
			prompt_tokens    INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens     INTEGER NOT NULL DEFAULT 0,
			duration_ms      INTEGER NOT NULL DEFAULT 0,
			success          INTEGER NOT NULL DEFAULT 1,
			error_message    TEXT NOT NULL DEFAULT '',
			redacted         INTEGER NOT NULL DEFAULT 0,
			redact_count     INTEGER NOT NULL DEFAULT 0,
			ip_address       TEXT NOT NULL DEFAULT '',
			metadata         TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_audit_user ON ai_audit_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_audit_action ON ai_audit_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_audit_time ON ai_audit_logs(timestamp)`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("执行迁移 SQL 失败: %w\nSQL: %s", err, q)
		}
	}
	return nil
}

// ==================== 模型 CRUD ====================

// CreateModel 创建模型.
func (s *Store) CreateModel(m *AIModel) error {
	_, err := s.db.Exec(`INSERT INTO ai_models
		(id, name, provider, endpoint, api_key, model_name, max_tokens, temperature, status, is_default, enabled, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.Provider, m.Endpoint, m.APIKey, m.ModelName,
		m.MaxTokens, m.Temperature, m.Status, boolToInt(m.IsDefault),
		boolToInt(m.Enabled), m.Description, m.CreatedAt, m.UpdatedAt)
	return err
}

// GetModel 获取模型.
func (s *Store) GetModel(id string) (*AIModel, error) {
	m := &AIModel{}
	var isDefault, enabled int
	err := s.db.QueryRow(`SELECT id, name, provider, endpoint, api_key, model_name,
		max_tokens, temperature, status, is_default, enabled, description, created_at, updated_at
		FROM ai_models WHERE id = ?`, id).Scan(
		&m.ID, &m.Name, &m.Provider, &m.Endpoint, &m.APIKey, &m.ModelName,
		&m.MaxTokens, &m.Temperature, &m.Status, &isDefault, &enabled,
		&m.Description, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.IsDefault = isDefault != 0
	m.Enabled = enabled != 0
	return m, nil
}

// ListModels 列出所有模型.
func (s *Store) ListModels() ([]*AIModel, error) {
	rows, err := s.db.Query(`SELECT id, name, provider, endpoint, api_key, model_name,
		max_tokens, temperature, status, is_default, enabled, description, created_at, updated_at
		FROM ai_models ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []*AIModel
	for rows.Next() {
		m := &AIModel{}
		var isDefault, enabled int
		if err := rows.Scan(&m.ID, &m.Name, &m.Provider, &m.Endpoint, &m.APIKey, &m.ModelName,
			&m.MaxTokens, &m.Temperature, &m.Status, &isDefault, &enabled,
			&m.Description, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.IsDefault = isDefault != 0
		m.Enabled = enabled != 0
		models = append(models, m)
	}
	return models, rows.Err()
}

// UpdateModel 更新模型.
func (s *Store) UpdateModel(m *AIModel) error {
	result, err := s.db.Exec(`UPDATE ai_models SET
		name=?, provider=?, endpoint=?, api_key=?, model_name=?, max_tokens=?, temperature=?,
		status=?, is_default=?, enabled=?, description=?, updated_at=?
		WHERE id=?`,
		m.Name, m.Provider, m.Endpoint, m.APIKey, m.ModelName,
		m.MaxTokens, m.Temperature, m.Status, boolToInt(m.IsDefault),
		boolToInt(m.Enabled), m.Description, time.Now(), m.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("模型不存在: %s", m.ID)
	}
	return nil
}

// DeleteModel 删除模型.
func (s *Store) DeleteModel(id string) error {
	result, err := s.db.Exec(`DELETE FROM ai_models WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("模型不存在: %s", id)
	}
	return nil
}

// GetDefaultModel 获取默认模型.
func (s *Store) GetDefaultModel() (*AIModel, error) {
	m := &AIModel{}
	var isDefault, enabled int
	err := s.db.QueryRow(`SELECT id, name, provider, endpoint, api_key, model_name,
		max_tokens, temperature, status, is_default, enabled, description, created_at, updated_at
		FROM ai_models WHERE is_default = 1 AND enabled = 1 LIMIT 1`).Scan(
		&m.ID, &m.Name, &m.Provider, &m.Endpoint, &m.APIKey, &m.ModelName,
		&m.MaxTokens, &m.Temperature, &m.Status, &isDefault, &enabled,
		&m.Description, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.IsDefault = isDefault != 0
	m.Enabled = enabled != 0
	return m, nil
}

// ClearDefault 清除所有默认标记.
func (s *Store) ClearDefault() error {
	_, err := s.db.Exec(`UPDATE ai_models SET is_default = 0`)
	return err
}

// ==================== 脱敏规则 CRUD ====================

// CreateRule 创建脱敏规则.
func (s *Store) CreateRule(r *RedactRule) error {
	_, err := s.db.Exec(`INSERT INTO ai_redact_rules
		(id, name, pii_type, pattern, strategy, mask_char, show_first, show_last, replacement, enabled, priority, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.PIIType, r.Pattern, r.Strategy, r.MaskChar,
		r.ShowFirst, r.ShowLast, r.Replacement, boolToInt(r.Enabled),
		r.Priority, r.Description, r.CreatedAt, r.UpdatedAt)
	return err
}

// GetRule 获取脱敏规则.
func (s *Store) GetRule(id string) (*RedactRule, error) {
	r := &RedactRule{}
	var enabled int
	err := s.db.QueryRow(`SELECT id, name, pii_type, pattern, strategy, mask_char,
		show_first, show_last, replacement, enabled, priority, description, created_at, updated_at
		FROM ai_redact_rules WHERE id = ?`, id).Scan(
		&r.ID, &r.Name, &r.PIIType, &r.Pattern, &r.Strategy, &r.MaskChar,
		&r.ShowFirst, &r.ShowLast, &r.Replacement, &enabled,
		&r.Priority, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Enabled = enabled != 0
	return r, nil
}

// ListRules 列出所有脱敏规则.
func (s *Store) ListRules() ([]*RedactRule, error) {
	rows, err := s.db.Query(`SELECT id, name, pii_type, pattern, strategy, mask_char,
		show_first, show_last, replacement, enabled, priority, description, created_at, updated_at
		FROM ai_redact_rules ORDER BY priority DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*RedactRule
	for rows.Next() {
		r := &RedactRule{}
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.PIIType, &r.Pattern, &r.Strategy, &r.MaskChar,
			&r.ShowFirst, &r.ShowLast, &r.Replacement, &enabled,
			&r.Priority, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// ListEnabledRules 列出所有启用的脱敏规则.
func (s *Store) ListEnabledRules() ([]*RedactRule, error) {
	rows, err := s.db.Query(`SELECT id, name, pii_type, pattern, strategy, mask_char,
		show_first, show_last, replacement, enabled, priority, description, created_at, updated_at
		FROM ai_redact_rules WHERE enabled = 1 ORDER BY priority DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*RedactRule
	for rows.Next() {
		r := &RedactRule{}
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.PIIType, &r.Pattern, &r.Strategy, &r.MaskChar,
			&r.ShowFirst, &r.ShowLast, &r.Replacement, &enabled,
			&r.Priority, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// UpdateRule 更新脱敏规则.
func (s *Store) UpdateRule(r *RedactRule) error {
	result, err := s.db.Exec(`UPDATE ai_redact_rules SET
		name=?, pii_type=?, pattern=?, strategy=?, mask_char=?, show_first=?, show_last=?,
		replacement=?, enabled=?, priority=?, description=?, updated_at=?
		WHERE id=?`,
		r.Name, r.PIIType, r.Pattern, r.Strategy, r.MaskChar,
		r.ShowFirst, r.ShowLast, r.Replacement, boolToInt(r.Enabled),
		r.Priority, r.Description, time.Now(), r.ID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("规则不存在: %s", r.ID)
	}
	return nil
}

// DeleteRule 删除脱敏规则.
func (s *Store) DeleteRule(id string) error {
	result, err := s.db.Exec(`DELETE FROM ai_redact_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("规则不存在: %s", id)
	}
	return nil
}

// ==================== 审计日志 ====================

// CreateAuditEntry 写入审计日志.
func (s *Store) CreateAuditEntry(e *AuditEntry) error {
	metaJSON, _ := json.Marshal(e.Metadata)
	if metaJSON == nil {
		metaJSON = []byte("{}")
	}
	_, err := s.db.Exec(`INSERT INTO ai_audit_logs
		(id, timestamp, user_id, username, model_id, model_name, action,
		 request_summary, response_summary, prompt_tokens, completion_tokens,
		 total_tokens, duration_ms, success, error_message, redacted, redact_count,
		 ip_address, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Timestamp, e.UserID, e.Username, e.ModelID, e.ModelName,
		e.Action, e.RequestSummary, e.ResponseSummary,
		e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.DurationMs,
		boolToInt(e.Success), e.ErrorMessage, boolToInt(e.Redacted),
		e.RedactCount, e.IPAddress, string(metaJSON))
	return err
}

// QueryAuditLogs 查询审计日志.
func (s *Store) QueryAuditLogs(filter AuditQueryFilter) ([]*AuditEntry, int64, error) {
	// 默认分页
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 构建 WHERE 子句
	where, args := buildAuditWhere(filter)

	// 查询总数
	var total int64
	countSQL := "SELECT COUNT(*) FROM ai_audit_logs" + where
	if err := s.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 查询数据
	offset := (page - 1) * pageSize
	querySQL := `SELECT id, timestamp, user_id, username, model_id, model_name, action,
		request_summary, response_summary, prompt_tokens, completion_tokens,
		total_tokens, duration_ms, success, error_message, redacted, redact_count,
		ip_address, metadata
		FROM ai_audit_logs` + where + ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		e := &AuditEntry{}
		var success, redacted int
		var metaStr string
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.UserID, &e.Username,
			&e.ModelID, &e.ModelName, &e.Action,
			&e.RequestSummary, &e.ResponseSummary,
			&e.PromptTokens, &e.CompletionTokens, &e.TotalTokens, &e.DurationMs,
			&success, &e.ErrorMessage, &redacted, &e.RedactCount,
			&e.IPAddress, &metaStr); err != nil {
			return nil, 0, err
		}
		e.Success = success != 0
		e.Redacted = redacted != 0
		_ = json.Unmarshal([]byte(metaStr), &e.Metadata)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// buildAuditWhere 构建审计日志查询条件.
func buildAuditWhere(filter AuditQueryFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	if filter.UserID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, filter.Action)
	}
	if !filter.StartTime.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, filter.EndTime)
	}
	if filter.Success != nil {
		conditions = append(conditions, "success = ?")
		if *filter.Success {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	if len(conditions) == 0 {
		return "", args
	}
	where := " WHERE "
	for i, c := range conditions {
		if i > 0 {
			where += " AND "
		}
		where += c
	}
	return where, args
}

// boolToInt 布尔转整数.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
