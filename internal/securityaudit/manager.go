package securityaudit

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 安全审计管理器.
type Manager struct {
	config         SecurityAuditConfig
	checker        *SecurityChecker
	vulnScanner    *VulnerabilityScanner
	hardening      *HardeningAdvisor
	scoreEngine    *ScoreEngine
	auditLogger    *AuditLogger
	notifyFunc     func(event AuditEvent)
	mu             sync.RWMutex
	stopCh         chan struct{}
}

// NewManager 创建安全审计管理器.
func NewManager() *Manager {
	m := &Manager{
		config: SecurityAuditConfig{
			Enabled:          true,
			AutoScan:         true,
			ScanInterval:     24 * time.Hour,
			ScoreCalculation: true,
			HardeningEnabled: true,
			VulnScanConfig: VulnerabilityScanConfig{
				ScanPackages: true,
				ScanServices: true,
				ScanConfig:   true,
				ScanNetwork:  true,
				ScanPorts:    true,
				AutoFix:      false,
			},
			AlertThreshold:   70,
			RetentionDays:    90,
			NotifyOnCritical: true,
			AutoRemediate:    false,
		},
		checker:     NewSecurityChecker(),
		vulnScanner: NewVulnerabilityScanner(),
		hardening:   NewHardeningAdvisor(),
		scoreEngine: NewScoreEngine(),
		auditLogger: NewAuditLogger(),
		stopCh:      make(chan struct{}),
	}

	// 启动自动扫描
	if m.config.AutoScan {
		go m.autoScanRoutine()
	}

	return m
}

// autoScanRoutine 自动扫描例程.
func (m *Manager) autoScanRoutine() {
	ticker := time.NewTicker(m.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.RunFullAudit()
		case <-m.stopCh:
			return
		}
	}
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// SetNotifyFunc 设置通知回调.
func (m *Manager) SetNotifyFunc(fn func(event AuditEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyFunc = fn
}

// notify 发送通知.
func (m *Manager) notify(event AuditEvent) {
	m.mu.RLock()
	fn := m.notifyFunc
	m.mu.RUnlock()

	if fn != nil {
		go fn(event)
	}
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() SecurityAuditConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config SecurityAuditConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果自动扫描状态改变，需要处理
	if config.AutoScan != m.config.AutoScan {
		if config.AutoScan {
			go m.autoScanRoutine()
		}
		// 注意：停止自动扫描需要通过 Stop 方法
	}

	m.config = config
	return nil
}

// ========== 安全配置检查 ==========

// RunSecurityChecks 运行所有安全检查.
func (m *Manager) RunSecurityChecks() []SecurityCheckResult {
	m.auditLogger.Log(AuditEvent{
		EventType: EventSecurityCheck,
		Severity:  SeverityMedium,
		Actor:     "system",
		Action:    "run_all_checks",
		Status:    "success",
		Message:   "运行所有安全检查",
	})

	return m.checker.RunAllChecks()
}

// RunSecurityChecksByCategory 按类别运行安全检查.
func (m *Manager) RunSecurityChecksByCategory(category SecurityCheckCategory) []SecurityCheckResult {
	m.auditLogger.Log(AuditEvent{
		EventType: EventSecurityCheck,
		Severity:  SeverityMedium,
		Actor:     "system",
		Action:    "run_category_checks",
		Details:   map[string]interface{}{"category": string(category)},
		Status:    "success",
		Message:   fmt.Sprintf("运行 %s 类别安全检查", category),
	})

	return m.checker.RunChecksByCategory(category)
}

// GetSecurityCheckList 获取安全检查列表.
func (m *Manager) GetSecurityCheckList() []SecurityCheck {
	return m.checker.GetCheckList()
}

// ========== 安全评分 ==========

// GetSecurityScore 获取安全评分.
func (m *Manager) GetSecurityScore() SecurityScore {
	checkResults := m.checker.RunAllChecks()
	score := m.scoreEngine.CalculateScore(checkResults)

	m.auditLogger.Log(AuditEvent{
		EventType: EventSecurityCheck,
		Severity:  SeverityLow,
		Actor:     "system",
		Action:    "calculate_score",
		Details: map[string]interface{}{
			"overall": score.Overall,
			"grade":   score.Grade,
		},
		Status:  "success",
		Message: fmt.Sprintf("安全评分: %d (%s)", score.Overall, score.Grade),
	})

	// 检查是否需要告警
	if score.Overall < m.config.AlertThreshold {
		m.notify(AuditEvent{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			EventType: EventSecurityCheck,
			Severity:  SeverityHigh,
			Actor:     "system",
			Action:    "score_alert",
			Details: map[string]interface{}{
				"score":   score.Overall,
				"grade":   score.Grade,
				"threshold": m.config.AlertThreshold,
			},
			Status:  "warning",
			Message: fmt.Sprintf("安全评分低于阈值: %d < %d", score.Overall, m.config.AlertThreshold),
		})
	}

	return score
}

// GetScoreHistory 获取评分历史.
func (m *Manager) GetScoreHistory(days int) []SecurityScoreHistory {
	return m.scoreEngine.GetHistory(days)
}

// ========== 漏洞扫描 ==========

// RunVulnerabilityScan 运行漏洞扫描.
func (m *Manager) RunVulnerabilityScan() VulnerabilityScanReport {
	report := m.vulnScanner.Scan(m.config.VulnScanConfig)

	m.auditLogger.Log(AuditEvent{
		EventType: EventVulnScan,
		Severity:  SeverityMedium,
		Actor:     "system",
		Action:    "vulnerability_scan",
		Details: map[string]interface{}{
			"total_found":    report.TotalFound,
			"critical_count": report.CriticalCount,
			"high_count":     report.HighCount,
		},
		Status:  "success",
		Message: fmt.Sprintf("漏洞扫描完成，发现 %d 个漏洞", report.TotalFound),
	})

	// 严重漏洞通知
	if m.config.NotifyOnCritical && report.CriticalCount > 0 {
		m.notify(AuditEvent{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			EventType: EventVulnScan,
			Severity:  SeverityCritical,
			Actor:     "system",
			Action:    "critical_vuln_alert",
			Details: map[string]interface{}{
				"critical_count": report.CriticalCount,
				"report_id":      report.ReportID,
			},
			Status:  "warning",
			Message: fmt.Sprintf("发现 %d 个严重漏洞", report.CriticalCount),
		})
	}

	return report
}

// GetVulnerabilities 获取漏洞列表.
func (m *Manager) GetVulnerabilities(severity VulnerabilitySeverity, status VulnerabilityStatus) []Vulnerability {
	return m.vulnScanner.GetVulnerabilities(severity, status)
}

// GetVulnerability 获取漏洞详情.
func (m *Manager) GetVulnerability(id string) (*Vulnerability, error) {
	return m.vulnScanner.GetVulnerability(id)
}

// UpdateVulnerabilityStatus 更新漏洞状态.
func (m *Manager) UpdateVulnerabilityStatus(id string, status VulnerabilityStatus, actor string) error {
	if err := m.vulnScanner.UpdateStatus(id, status); err != nil {
		return err
	}

	m.auditLogger.Log(AuditEvent{
		EventType: EventVulnScan,
		Severity:  SeverityMedium,
		Actor:     actor,
		Resource:  id,
		Action:    "update_vuln_status",
		Details: map[string]interface{}{
			"new_status": string(status),
		},
		Status:  "success",
		Message: fmt.Sprintf("更新漏洞 %s 状态为 %s", id, status),
	})

	return nil
}

// FixVulnerability 修复漏洞.
func (m *Manager) FixVulnerability(id string, actor string) error {
	if err := m.vulnScanner.FixVulnerability(id); err != nil {
		return err
	}

	m.auditLogger.Log(AuditEvent{
		EventType: EventVulnScan,
		Severity:  SeverityMedium,
		Actor:     actor,
		Resource:  id,
		Action:    "fix_vulnerability",
		Status:    "success",
		Message:   fmt.Sprintf("修复漏洞 %s", id),
	})

	return nil
}

// GetLatestScanReport 获取最新扫描报告.
func (m *Manager) GetLatestScanReport() *VulnerabilityScanReport {
	return m.vulnScanner.GetLatestReport()
}

// ========== 安全加固建议 ==========

// GetHardeningSuggestions 获取加固建议.
func (m *Manager) GetHardeningSuggestions() []HardeningSuggestion {
	checkResults := m.checker.RunAllChecks()
	return m.hardening.GenerateSuggestions(checkResults)
}

// GetHardeningSuggestionsByCategory 按类别获取加固建议.
func (m *Manager) GetHardeningSuggestionsByCategory(category HardeningCategory) []HardeningSuggestion {
	return m.hardening.GetSuggestionsByCategory(category)
}

// GetHardeningReport 获取加固报告.
func (m *Manager) GetHardeningReport() HardeningReport {
	checkResults := m.checker.RunAllChecks()
	return m.hardening.GenerateReport(checkResults)
}

// ApplyHardeningSuggestion 应用加固建议.
func (m *Manager) ApplyHardeningSuggestion(id string, actor string) error {
	if err := m.hardening.Apply(id); err != nil {
		return err
	}

	m.auditLogger.Log(AuditEvent{
		EventType: EventHardening,
		Severity:  SeverityMedium,
		Actor:     actor,
		Resource:  id,
		Action:    "apply_hardening",
		Status:    "success",
		Message:   fmt.Sprintf("应用加固建议 %s", id),
	})

	return nil
}

// DismissHardeningSuggestion 忽略加固建议.
func (m *Manager) DismissHardeningSuggestion(id string, actor string, reason string) error {
	if err := m.hardening.Dismiss(id); err != nil {
		return err
	}

	m.auditLogger.Log(AuditEvent{
		EventType: EventHardening,
		Severity:  SeverityLow,
		Actor:     actor,
		Resource:  id,
		Action:    "dismiss_hardening",
		Details: map[string]interface{}{
			"reason": reason,
		},
		Status:  "success",
		Message: fmt.Sprintf("忽略加固建议 %s: %s", id, reason),
	})

	return nil
}

// ========== 审计日志 ==========

// LogEvent 记录审计事件.
func (m *Manager) LogEvent(event AuditEvent) {
	m.auditLogger.Log(event)
}

// GetAuditLogs 获取审计日志.
func (m *Manager) GetAuditLogs(limit, offset int, filters map[string]string) []AuditEvent {
	return m.auditLogger.GetLogs(limit, offset, filters)
}

// GetAuditReport 获取审计报告.
func (m *Manager) GetAuditReport(startTime, endTime time.Time) AuditReport {
	return m.auditLogger.GenerateReport(startTime, endTime)
}

// ExportAuditLogs 导出审计日志.
func (m *Manager) ExportAuditLogs(startTime, endTime time.Time, format string) ([]byte, error) {
	return m.auditLogger.ExportLogs(startTime, endTime, format)
}

// ========== 完整审计 ==========

// RunFullAudit 运行完整安全审计.
func (m *Manager) RunFullAudit() map[string]interface{} {
	startTime := time.Now()

	// 运行安全检查
	checkResults := m.RunSecurityChecks()

	// 计算评分
	score := m.scoreEngine.CalculateScore(checkResults)

	// 运行漏洞扫描
	vulnReport := m.RunVulnerabilityScan()

	// 生成加固建议
	hardeningReport := m.hardening.GenerateReport(checkResults)

	duration := time.Since(startTime)

	result := map[string]interface{}{
		"audit_id":          uuid.New().String(),
		"timestamp":         startTime,
		"duration":          duration.String(),
		"security_score":    score,
		"check_results":     checkResults,
		"vulnerability_report": vulnReport,
		"hardening_report":  hardeningReport,
		"summary": map[string]interface{}{
			"total_checks":     len(checkResults),
			"passed_checks":    countByStatus(checkResults, StatusPass),
			"failed_checks":    countByStatus(checkResults, StatusFail),
			"warning_checks":   countByStatus(checkResults, StatusWarning),
			"total_vulns":      vulnReport.TotalFound,
			"critical_vulns":   vulnReport.CriticalCount,
			"hardening_items":  hardeningReport.TotalItems,
			"overall_score":    score.Overall,
			"grade":            score.Grade,
		},
	}

	m.auditLogger.Log(AuditEvent{
		EventType: EventSecurityCheck,
		Severity:  SeverityMedium,
		Actor:     "system",
		Action:    "full_audit",
		Details: map[string]interface{}{
			"duration":       duration.String(),
			"overall_score":  score.Overall,
			"grade":          score.Grade,
		},
		Status:  "success",
		Message: fmt.Sprintf("完整安全审计完成，评分: %d (%s)", score.Overall, score.Grade),
	})

	return result
}

// GetDashboard 获取安全仪表板数据.
func (m *Manager) GetDashboard() map[string]interface{} {
	score := m.GetSecurityScore()
	vulnReport := m.GetLatestScanReport()
	hardeningReport := m.GetHardeningReport()
	recentLogs := m.auditLogger.GetLogs(10, 0, nil)

	dashboard := map[string]interface{}{
		"timestamp":      time.Now(),
		"security_score": score,
		"recent_logs":    recentLogs,
	}

	if vulnReport != nil {
		dashboard["vulnerability_summary"] = map[string]interface{}{
			"total":    vulnReport.TotalFound,
			"critical": vulnReport.CriticalCount,
			"high":     vulnReport.HighCount,
			"medium":   vulnReport.MediumCount,
			"low":      vulnReport.LowCount,
			"last_scan": vulnReport.ScanTime,
		}
	}

	dashboard["hardening_summary"] = map[string]interface{}{
		"total":   hardeningReport.TotalItems,
		"applied": hardeningReport.AppliedCount,
		"critical": hardeningReport.CriticalCount,
		"high":    hardeningReport.HighCount,
	}

	return dashboard
}

// countByStatus 按状态统计.
func countByStatus(results []SecurityCheckResult, status SecurityCheckStatus) int {
	count := 0
	for _, r := range results {
		if r.Status == status {
			count++
		}
	}
	return count
}
