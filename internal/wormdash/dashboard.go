package wormdash

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// NewDashboardWithLogger 创建带日志的仪表盘引擎
func NewDashboardWithLogger(logger *zap.Logger) *Dashboard {
	d := NewDashboard()
	d.logger = logger
	return d
}

// Overview 获取合规概览
func (d *Dashboard) Overview() *Overview {
	d.mu.RLock()
	defer d.mu.RUnlock()

	overview := &Overview{
		TotalPolicies: len(d.policies),
		LastAuditAt:   d.lastAuditTime(),
	}

	active := 0
	for _, p := range d.policies {
		if p.Status == PolicyActive {
			active++
		}
	}
	overview.ActivePolicies = active

	open := 0
	for _, a := range d.alerts {
		if !a.Resolved {
			open++
		}
	}
	overview.OpenAlerts = open

	// 基于保留记录统计
	protected := int64(0)
	totalSize := int64(0)
	expired := 0
	for _, r := range d.retention {
		protected++
		if r.ExpiresAt != nil && time.Now().After(*r.ExpiresAt) {
			expired++
		}
	}
	overview.ProtectedFiles = protected
	overview.TotalSizeBytes = totalSize
	overview.ExpiredFiles = expired

	// 合规率 = (受保护文件 - 未解决告警数) / 受保护文件 * 100
	if protected > 0 {
		broken := 0
		for _, a := range d.alerts {
			if !a.Resolved {
				broken++
			}
		}
		overview.BrokenFiles = broken
		overview.ComplianceRate = float64(protected-int64(broken)) / float64(protected) * 100
	} else {
		overview.ComplianceRate = 100.0
	}

	return overview
}

// AddPolicy 添加WORM策略
func (d *Dashboard) AddPolicy(name string, scope PolicyScope, target string, retentionDays int, description, createdBy string) *WORMPolicy {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextID++
	now := time.Now()
	policy := &WORMPolicy{
		ID:            fmt.Sprintf("WP-%d", d.nextID),
		Name:          name,
		Scope:         scope,
		Target:        target,
		RetentionDays: retentionDays,
		Status:        PolicyActive,
		Description:   description,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	d.policies[policy.ID] = policy

	d.addAuditEntry(ActionPolicyAdd, createdBy, policy.ID, fmt.Sprintf("创建策略: %s", name), "", true)
	return policy
}

// ListPolicies 列出所有策略
func (d *Dashboard) ListPolicies(status PolicyStatus) []*WORMPolicy {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*WORMPolicy, 0, len(d.policies))
	for _, p := range d.policies {
		if status != "" && p.Status != status {
			continue
		}
		result = append(result, p)
	}
	return result
}

// GetPolicy 获取策略
func (d *Dashboard) GetPolicy(id string) (*WORMPolicy, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	p, ok := d.policies[id]
	return p, ok
}

// UpdatePolicy 更新策略
func (d *Dashboard) UpdatePolicy(id string, name *string, retentionDays *int, status *PolicyStatus, actor string) (*WORMPolicy, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	p, ok := d.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	if name != nil {
		p.Name = *name
	}
	if retentionDays != nil {
		p.RetentionDays = *retentionDays
	}
	if status != nil {
		p.Status = *status
	}
	p.UpdatedAt = time.Now()

	d.addAuditEntry(ActionPolicyAdd, actor, id, "更新策略", "", true)
	return p, nil
}

// DeletePolicy 删除策略
func (d *Dashboard) DeletePolicy(id, actor string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}
	delete(d.policies, id)
	d.addAuditEntry(ActionPolicyDel, actor, id, "删除策略", "", true)
	return nil
}

// GenerateReport 生成合规报告
func (d *Dashboard) GenerateReport(req *ReportRequest) (*ComplianceReport, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var start, end time.Time
	switch req.ReportType {
	case "monthly":
		if req.Month < 1 || req.Month > 12 {
			return nil, fmt.Errorf("invalid month: %d", req.Month)
		}
		start = time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.Local)
		end = start.AddDate(0, 1, 0).Add(-time.Second)
	case "quarterly":
		if req.Quarter < 1 || req.Quarter > 4 {
			return nil, fmt.Errorf("invalid quarter: %d", req.Quarter)
		}
		startMonth := time.Month((req.Quarter-1)*3 + 1)
		start = time.Date(req.Year, startMonth, 1, 0, 0, 0, 0, time.Local)
		end = start.AddDate(0, 3, 0).Add(-time.Second)
	default:
		return nil, fmt.Errorf("unsupported report type: %s", req.ReportType)
	}

	d.nextID++
	report := &ComplianceReport{
		ID:             fmt.Sprintf("WCR-%d", d.nextID),
		ReportType:     req.ReportType,
		PeriodStart:    start,
		PeriodEnd:      end,
		GeneratedAt:    time.Now(),
		GeneratedBy:    req.GeneratedBy,
		TotalFiles:     len(d.retention),
		ProtectedFiles: d.countProtected(),
		RetentionStats: d.buildRetentionStats(),
		PolicyStats:    d.buildPolicyStats(),
	}

	if report.TotalFiles > 0 {
		violations := d.countViolations(start, end)
		report.Violations = violations
		report.ComplianceRate = float64(report.ProtectedFiles-violations) / float64(report.ProtectedFiles) * 100
		if report.ComplianceRate < 0 {
			report.ComplianceRate = 0
		}
	} else {
		report.ComplianceRate = 100.0
	}

	report.Summary = fmt.Sprintf("%d年%s: 共%d个受保护文件，合规率%.1f%%，%d项违规",
		req.Year, reportTypeLabel(req.ReportType, req.Month, req.Quarter),
		report.ProtectedFiles, report.ComplianceRate, report.Violations)

	d.reports = append(d.reports, report)
	d.addAuditEntry(ActionReport, req.GeneratedBy, report.ID, report.Summary, "", true)
	return report, nil
}

// ListReports 列出报告
func (d *Dashboard) ListReports() []*ComplianceReport {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]*ComplianceReport, len(d.reports))
	copy(result, d.reports)
	return result
}

// ReportBypassAttempt 报告绕过尝试
func (d *Dashboard) ReportBypassAttempt(sourcePath, sourceIP, userID, description string, severity AlertSeverity) *AnomalyAlert {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextID++
	alert := &AnomalyAlert{
		ID:          fmt.Sprintf("AA-%d", d.nextID),
		Severity:    severity,
		Type:        "bypass_attempt",
		Description: description,
		SourcePath:  sourcePath,
		SourceIP:    sourceIP,
		UserID:      userID,
		DetectedAt:  time.Now(),
	}
	d.alerts[alert.ID] = alert
	d.addAuditEntry(ActionBypass, userID, sourcePath, description, sourceIP, false)
	return alert
}

// ListAlerts 列出告警
func (d *Dashboard) ListAlerts(resolved *bool) []*AnomalyAlert {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*AnomalyAlert, 0, len(d.alerts))
	for _, a := range d.alerts {
		if resolved != nil && a.Resolved != *resolved {
			continue
		}
		result = append(result, a)
	}
	return result
}

// ResolveAlert 解决告警
func (d *Dashboard) ResolveAlert(id, actor string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	alert, ok := d.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	now := time.Now()
	alert.Resolved = true
	alert.ResolvedAt = &now
	d.addAuditEntry(ActionBypass, actor, id, "告警已解决", "", true)
	return nil
}

// AddRetention 添加保留期记录
func (d *Dashboard) AddRetention(fileID, filePath string, retentionDays int, actor string) *RetentionEntry {
	d.mu.Lock()
	defer d.mu.Unlock()

	var expiresAt *time.Time
	if retentionDays > 0 {
		t := time.Now().AddDate(0, 0, retentionDays)
		expiresAt = &t
	}
	entry := &RetentionEntry{
		FileID:        fileID,
		FilePath:      filePath,
		RetentionDays: retentionDays,
		LockedAt:      time.Now(),
		ExpiresAt:     expiresAt,
	}
	d.retention[fileID] = entry
	d.addAuditEntry(ActionRetention, actor, fileID, fmt.Sprintf("设置保留期: %d天", retentionDays), "", true)
	return entry
}

// ListRetention 列出保留记录
func (d *Dashboard) ListRetention() []*RetentionEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*RetentionEntry, 0, len(d.retention))
	for _, r := range d.retention {
		result = append(result, r)
	}
	return result
}

// ExtendRetention 延长保留期
func (d *Dashboard) ExtendRetention(fileID string, extraDays int, actor string) (*RetentionEntry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.retention[fileID]
	if !ok {
		return nil, fmt.Errorf("retention entry %s not found", fileID)
	}
	if entry.ExpiresAt != nil {
		*entry.ExpiresAt = entry.ExpiresAt.AddDate(0, 0, extraDays)
	}
	entry.RetentionDays += extraDays
	entry.Extended++
	d.addAuditEntry(ActionRetention, actor, fileID, fmt.Sprintf("延期%d天", extraDays), "", true)
	return entry, nil
}

// ListAudit 列出审计日志
func (d *Dashboard) ListAudit(action AuditAction, limit int) []*AuditEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*AuditEntry, 0)
	// 从最新到最旧遍历
	for i := len(d.auditLog) - 1; i >= 0; i-- {
		entry := d.auditLog[i]
		if action != "" && entry.Action != action {
			continue
		}
		result = append(result, entry)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// --- 内部辅助方法 ---

func (d *Dashboard) addAuditEntry(action AuditAction, actor, target, details, sourceIP string, success bool) {
	d.nextID++
	entry := &AuditEntry{
		ID:        fmt.Sprintf("AE-%d", d.nextID),
		Timestamp: time.Now(),
		Action:    action,
		Actor:     actor,
		Target:    target,
		Details:   details,
		SourceIP:  sourceIP,
		Success:   success,
	}
	d.auditLog = append(d.auditLog, entry)
}

func (d *Dashboard) lastAuditTime() string {
	if len(d.auditLog) == 0 {
		return ""
	}
	return d.auditLog[len(d.auditLog)-1].Timestamp.Format(time.RFC3339)
}

func (d *Dashboard) countProtected() int {
	count := 0
	for _, r := range d.retention {
		if r.ExpiresAt == nil || time.Now().Before(*r.ExpiresAt) {
			count++
		}
	}
	return count
}

func (d *Dashboard) countViolations(start, end time.Time) int {
	count := 0
	for _, a := range d.alerts {
		if a.DetectedAt.After(start) && a.DetectedAt.Before(end) && !a.Resolved {
			count++
		}
	}
	return count
}

func (d *Dashboard) buildRetentionStats() map[string]int {
	stats := make(map[string]int)
	for _, r := range d.retention {
		if r.RetentionDays == 0 {
			stats["permanent"]++
		} else if r.ExpiresAt != nil && time.Now().After(*r.ExpiresAt) {
			stats["expired"]++
		} else {
			stats["active"]++
		}
	}
	return stats
}

func (d *Dashboard) buildPolicyStats() map[string]int {
	stats := make(map[string]int)
	for _, p := range d.policies {
		stats[string(p.Status)]++
	}
	return stats
}

func reportTypeLabel(reportType string, month, quarter int) string {
	switch reportType {
	case "monthly":
		return fmt.Sprintf("%d月报告", month)
	case "quarterly":
		return fmt.Sprintf("Q%d报告", quarter)
	default:
		return reportType
	}
}
