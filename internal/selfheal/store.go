// Package selfheal 提供系统健康自检与自愈功能
package selfheal

import (
	"database/sql"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// Store 自检记录持久化存储.
type Store struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewStore 创建存储实例.
func NewStore(db *sql.DB, logger *zap.Logger) *Store {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Store{db: db, logger: logger}
}

// Init 初始化数据库表.
func (s *Store) Init() error {
	query := `
	CREATE TABLE IF NOT EXISTS self_heal_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		check_name TEXT NOT NULL,
		category TEXT NOT NULL,
		status TEXT NOT NULL,
		message TEXT NOT NULL,
		details TEXT DEFAULT '',
		heal_action TEXT NOT NULL DEFAULT 'none',
		heal_success INTEGER,
		heal_message TEXT DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_self_heal_records_name ON self_heal_records(check_name);
	CREATE INDEX IF NOT EXISTS idx_self_heal_records_status ON self_heal_records(status);
	CREATE INDEX IF NOT EXISTS idx_self_heal_records_created ON self_heal_records(created_at);
	`
	_, err := s.db.Exec(query)
	if err != nil {
		s.logger.Error("failed to init self-heal store", zap.Error(err))
	}
	return err
}

// SaveRecord 保存检查记录.
func (s *Store) SaveRecord(result *CheckResult, action HealAction) error {
	detailsJSON := ""
	if result.Details != nil {
		if b, err := json.Marshal(result.Details); err == nil {
			detailsJSON = string(b)
		}
	}

	_, err := s.db.Exec(`
		INSERT INTO self_heal_records (check_name, category, status, message, details, heal_action, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, result.Name, string(result.Category), string(result.Status),
		result.Message, detailsJSON, string(action), result.Timestamp.Format(time.RFC3339))

	return err
}

// UpdateHealResult 更新修复结果.
func (s *Store) UpdateHealResult(checkName string, result *HealResult) error {
	success := 0
	if result.Success {
		success = 1
	}

	// 更新最近一条该检查项的记录
	_, err := s.db.Exec(`
		UPDATE self_heal_records
		SET heal_success = ?, heal_message = ?
		WHERE id = (
			SELECT id FROM self_heal_records
			WHERE check_name = ?
			ORDER BY id DESC
			LIMIT 1
		)
	`, success, result.Message, checkName)

	return err
}

// GetHistory 获取检查历史.
func (s *Store) GetHistory(limit int) ([]*HealRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT id, check_name, category, status, message, details,
		       heal_action, heal_success, heal_message, created_at
		FROM self_heal_records
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []*HealRecord
	for rows.Next() {
		r := &HealRecord{}
		var details sql.NullString
		var healSuccess sql.NullInt64
		var healMessage sql.NullString
		var createdAt string

		err := rows.Scan(
			&r.ID, &r.CheckName, &r.Category, &r.Status, &r.Message,
			&details, &r.HealAction, &healSuccess, &healMessage, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		if details.Valid {
			r.Details = details.String
		}
		if healSuccess.Valid {
			b := healSuccess.Int64 == 1
			r.HealSuccess = &b
		}
		if healMessage.Valid {
			r.HealMessage = healMessage.String
		}
		t, _ := time.Parse(time.RFC3339, createdAt)
		r.CreatedAt = t

		records = append(records, r)
	}

	return records, rows.Err()
}

// GetHistoryByCheck 获取指定检查项的历史.
func (s *Store) GetHistoryByCheck(checkName string, limit int) ([]*HealRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, check_name, category, status, message, details,
		       heal_action, heal_success, heal_message, created_at
		FROM self_heal_records
		WHERE check_name = ?
		ORDER BY id DESC
		LIMIT ?
	`, checkName, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []*HealRecord
	for rows.Next() {
		r := &HealRecord{}
		var details sql.NullString
		var healSuccess sql.NullInt64
		var healMessage sql.NullString
		var createdAt string

		err := rows.Scan(
			&r.ID, &r.CheckName, &r.Category, &r.Status, &r.Message,
			&details, &r.HealAction, &healSuccess, &healMessage, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		if details.Valid {
			r.Details = details.String
		}
		if healSuccess.Valid {
			b := healSuccess.Int64 == 1
			r.HealSuccess = &b
		}
		if healMessage.Valid {
			r.HealMessage = healMessage.String
		}
		t, _ := time.Parse(time.RFC3339, createdAt)
		r.CreatedAt = t

		records = append(records, r)
	}

	return records, rows.Err()
}

// Cleanup 清理过期记录.
func (s *Store) Cleanup(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	result, err := s.db.Exec(`
		DELETE FROM self_heal_records WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		s.logger.Info("cleaned up old self-heal records", zap.Int64("count", n))
	}
	return nil
}
