// Package complianceaudit 提供审计数据存储
package complianceaudit

import (
	"database/sql"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// Store 审计数据存储
type Store struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewStore 创建审计数据存储
func NewStore(db *sql.DB, logger *zap.Logger) *Store {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Store{
		db:     db,
		logger: logger,
	}
}

// Init 初始化数据库表
func (s *Store) Init() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS compliance_reports (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			generated_at DATETIME NOT NULL,
			period_start DATETIME,
			period_end DATETIME,
			summary TEXT,
			standards TEXT,
			findings TEXT,
			remediations TEXT,
			format TEXT DEFAULT 'json',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			resource TEXT,
			result TEXT,
			details TEXT,
			ip_address TEXT,
			user_agent TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// SaveReport 保存合规报告
func (s *Store) SaveReport(report *ComplianceReport) error {
	summaryJSON, _ := json.Marshal(report.Summary)
	standardsJSON, _ := json.Marshal(report.Standards)
	findingsJSON, _ := json.Marshal(report.Findings)
	remediationsJSON, _ := json.Marshal(report.Remediations)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO compliance_reports 
		(id, title, generated_at, period_start, period_end, summary, standards, findings, remediations, format)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, report.ID, report.Title, report.GeneratedAt,
		report.Period.Start, report.Period.End,
		string(summaryJSON), string(standardsJSON),
		string(findingsJSON), string(remediationsJSON),
		string(report.Format))

	return err
}

// GetReports 获取历史报告
func (s *Store) GetReports(limit int) ([]*ComplianceReport, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query(`
		SELECT id, title, generated_at, period_start, period_end, 
		       summary, standards, findings, remediations, format
		FROM compliance_reports
		ORDER BY generated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]*ComplianceReport, 0)
	for rows.Next() {
		r := &ComplianceReport{
			Summary:      &ReportSummary{},
			Standards:    make([]*StandardReport, 0),
			Findings:     make([]*Finding, 0),
			Remediations: make([]*RemediationItem, 0),
		}
		var summaryJSON, standardsJSON, findingsJSON, remediationsJSON string
		var format string

		err := rows.Scan(
			&r.ID, &r.Title, &r.GeneratedAt,
			&r.Period.Start, &r.Period.End,
			&summaryJSON, &standardsJSON,
			&findingsJSON, &remediationsJSON,
			&format,
		)
		if err != nil {
			s.logger.Error("failed to scan report", zap.Error(err))
			continue
		}

		_ = json.Unmarshal([]byte(summaryJSON), r.Summary)
		_ = json.Unmarshal([]byte(standardsJSON), &r.Standards)
		_ = json.Unmarshal([]byte(findingsJSON), &r.Findings)
		_ = json.Unmarshal([]byte(remediationsJSON), &r.Remediations)
		r.Format = ReportFormat(format)

		reports = append(reports, r)
	}

	return reports, nil
}

// SaveAuditLog 保存审计日志
func (s *Store) SaveAuditLog(log *AuditLog) error {
	detailsJSON, _ := json.Marshal(log.Details)

	_, err := s.db.Exec(`
		INSERT INTO audit_logs (timestamp, actor, action, resource, result, details, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, log.Timestamp, log.Actor, log.Action, log.Resource,
		log.Result, string(detailsJSON), log.IPAddress, log.UserAgent)

	return err
}

// GetAuditLogs 获取审计日志
func (s *Store) GetAuditLogs(actor string, limit int) ([]*AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if actor != "" {
		rows, err = s.db.Query(`
			SELECT id, timestamp, actor, action, resource, result, details, ip_address, user_agent
			FROM audit_logs
			WHERE actor = ?
			ORDER BY timestamp DESC
			LIMIT ?
		`, actor, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, timestamp, actor, action, resource, result, details, ip_address, user_agent
			FROM audit_logs
			ORDER BY timestamp DESC
			LIMIT ?
		`, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]*AuditLog, 0)
	for rows.Next() {
		l := &AuditLog{}
		var detailsJSON string

		err := rows.Scan(
			&l.ID, &l.Timestamp, &l.Actor, &l.Action,
			&l.Resource, &l.Result, &detailsJSON,
			&l.IPAddress, &l.UserAgent,
		)
		if err != nil {
			s.logger.Error("failed to scan audit log", zap.Error(err))
			continue
		}

		_ = json.Unmarshal([]byte(detailsJSON), &l.Details)
		logs = append(logs, l)
	}

	return logs, nil
}

// Cleanup 清理过期数据
func (s *Store) Cleanup(retention time.Duration) error {
	cutoff := time.Now().Add(-retention)

	_, err := s.db.Exec("DELETE FROM audit_logs WHERE timestamp < ?", cutoff)
	if err != nil {
		return err
	}

	_, err = s.db.Exec("DELETE FROM compliance_reports WHERE generated_at < ?", cutoff)
	return err
}
