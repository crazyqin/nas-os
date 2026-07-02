// Package stigreport 提供STIG合规自动化报告
package stigreport

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 错误定义.
var (
	ErrRuleNotFound = errors.New("合规规则不存在")
	ErrInvalidInput = errors.New("无效输入参数")
)

// Severity 严重程度.
type Severity string

const (
	SeverityHigh   Severity = "high"   // 高
	SeverityMedium Severity = "medium" // 中
	SeverityLow    Severity = "low"    // 低
)

// ComplianceStatus 合规状态.
type ComplianceStatus string

const (
	StatusCompliant     ComplianceStatus = "compliant"      // 合规
	StatusNonCompliant  ComplianceStatus = "non_compliant"  // 不合规
	StatusNotApplicable ComplianceStatus = "not_applicable" // 不适用
	StatusNotChecked    ComplianceStatus = "not_checked"    // 未检查
)

// CheckCategory 检查类别.
type CheckCategory string

const (
	CategoryAccessControl   CheckCategory = "access_control"   // 访问控制
	CategoryAuditLogging    CheckCategory = "audit_logging"    // 审计日志
	CategoryAuthentication  CheckCategory = "authentication"   // 身份认证
	CategoryEncryption      CheckCategory = "encryption"       // 加密
	CategoryNetworkSecurity CheckCategory = "network_security" // 网络安全
	CategorySystemHardening CheckCategory = "system_hardening" // 系统加固
	CategoryDataProtection  CheckCategory = "data_protection"  // 数据保护
	CategoryPatchManagement CheckCategory = "patch_management" // 补丁管理
)

// STIGRule STIG规则.
type STIGRule struct {
	ID          string        `json:"id"`
	Category    CheckCategory `json:"category"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Severity    Severity      `json:"severity"`
	CheckText   string        `json:"check_text"`
	FixText     string        `json:"fix_text"`
	Reference   string        `json:"reference,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// RuleCheck 规则检查结果.
type RuleCheck struct {
	RuleID    string           `json:"rule_id"`
	Status    ComplianceStatus `json:"status"`
	Details   string           `json:"details,omitempty"`
	CheckedAt time.Time        `json:"checked_at"`
	CheckedBy string           `json:"checked_by,omitempty"`
	Evidence  []string         `json:"evidence,omitempty"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID                string         `json:"id"`
	Title             string         `json:"title"`
	GeneratedAt       time.Time      `json:"generated_at"`
	Period            string         `json:"period"`
	TotalRules        int            `json:"total_rules"`
	CompliantCount    int            `json:"compliant_count"`
	NonCompliantCount int            `json:"non_compliant_count"`
	NotApplicable     int            `json:"not_applicable_count"`
	ComplianceRate    float64        `json:"compliance_rate"`
	Checks            []*RuleCheck   `json:"checks"`
	Summary           *ReportSummary `json:"summary"`
	Recommendations   []string       `json:"recommendations"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	HighRiskFindings   int      `json:"high_risk_findings"`
	MediumRiskFindings int      `json:"medium_risk_findings"`
	LowRiskFindings    int      `json:"low_risk_findings"`
	TopIssues          []string `json:"top_issues"`
	RemediationPlan    string   `json:"remediation_plan"`
}

// ScheduledScan 定时扫描.
type ScheduledScan struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Cron      string     `json:"cron"`
	Enabled   bool       `json:"enabled"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Manager STIG合规管理器.
type Manager struct {
	mu        sync.RWMutex
	rules     map[string]*STIGRule
	checks    map[string]*RuleCheck
	reports   []*ComplianceReport
	schedules []*ScheduledScan
	startTime time.Time
}

// NewManager 创建管理器.
func NewManager() *Manager {
	m := &Manager{
		rules:     make(map[string]*STIGRule),
		checks:    make(map[string]*RuleCheck),
		reports:   make([]*ComplianceReport, 0),
		schedules: make([]*ScheduledScan, 0),
		startTime: time.Now(),
	}

	// 初始化默认STIG规则
	m.initDefaultRules()

	return m
}

// initDefaultRules 初始化默认规则.
func (m *Manager) initDefaultRules() {
	defaults := []*STIGRule{
		{
			ID: "STIG-AC-001", Category: CategoryAccessControl,
			Title: "密码复杂度要求", Description: "系统密码必须满足复杂度要求",
			Severity: SeverityHigh, CheckText: "检查密码策略配置", FixText: "配置密码复杂度策略",
		},
		{
			ID: "STIG-AC-002", Category: CategoryAccessControl,
			Title: "账户锁定策略", Description: "配置账户锁定策略防止暴力破解",
			Severity: SeverityMedium, CheckText: "检查账户锁定配置", FixText: "配置账户锁定阈值",
		},
		{
			ID: "STIG-AU-001", Category: CategoryAuditLogging,
			Title: "审计日志启用", Description: "系统必须启用审计日志",
			Severity: SeverityHigh, CheckText: "检查审计日志配置", FixText: "启用审计日志服务",
		},
		{
			ID: "STIG-AU-002", Category: CategoryAuditLogging,
			Title: "日志保留期限", Description: "审计日志必须保留至少90天",
			Severity: SeverityMedium, CheckText: "检查日志保留配置", FixText: "配置日志保留策略",
		},
		{
			ID: "STIG-SC-001", Category: CategoryEncryption,
			Title: "传输加密", Description: "所有数据传输必须使用TLS 1.2+",
			Severity: SeverityHigh, CheckText: "检查TLS配置", FixText: "配置TLS 1.2或更高版本",
		},
		{
			ID: "STIG-SC-002", Category: CategoryEncryption,
			Title: "静态数据加密", Description: "敏感数据必须加密存储",
			Severity: SeverityHigh, CheckText: "检查加密配置", FixText: "启用静态数据加密",
		},
		{
			ID: "STIG-NW-001", Category: CategoryNetworkSecurity,
			Title: "防火墙配置", Description: "启用并正确配置防火墙",
			Severity: SeverityHigh, CheckText: "检查防火墙规则", FixText: "配置防火墙规则",
		},
		{
			ID: "STIG-SH-001", Category: CategorySystemHardening,
			Title: "系统更新", Description: "系统必须安装最新安全补丁",
			Severity: SeverityHigh, CheckText: "检查系统更新状态", FixText: "安装系统更新",
		},
	}

	for _, rule := range defaults {
		rule.CreatedAt = time.Now()
		rule.UpdatedAt = time.Now()
		m.rules[rule.ID] = rule
	}
}

// AddRule 添加规则.
func (m *Manager) AddRule(rule *STIGRule) error {
	if rule == nil || rule.ID == "" {
		return ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule

	return nil
}

// GetRule 获取规则.
func (m *Manager) GetRule(ruleID string) (*STIGRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules 列出规则.
func (m *Manager) ListRules(category *CheckCategory) []*STIGRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*STIGRule, 0)
	for _, rule := range m.rules {
		if category == nil || rule.Category == *category {
			result = append(result, rule)
		}
	}
	return result
}

// RunCheck 执行检查.
func (m *Manager) RunCheck(ruleID string, status ComplianceStatus, details string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[ruleID]; !exists {
		return ErrRuleNotFound
	}

	check := &RuleCheck{
		RuleID:    ruleID,
		Status:    status,
		Details:   details,
		CheckedAt: time.Now(),
	}

	m.checks[ruleID] = check
	return nil
}

// RunAutomatedScan 运行自动扫描.
func (m *Manager) RunAutomatedScan() []*RuleCheck {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]*RuleCheck, 0)
	for ruleID, rule := range m.rules {
		// 模拟自动检查
		status := StatusCompliant
		details := fmt.Sprintf("自动检查通过 - %s", rule.Title)

		// 简单模拟：高严重度规则有30%概率不合规
		if rule.Severity == SeverityHigh && time.Now().UnixNano()%10 < 3 {
			status = StatusNonCompliant
			details = fmt.Sprintf("发现问题 - %s", rule.Title)
		}

		check := &RuleCheck{
			RuleID:    ruleID,
			Status:    status,
			Details:   details,
			CheckedAt: time.Now(),
		}

		m.checks[ruleID] = check
		results = append(results, check)
	}

	return results
}

// GenerateReport 生成报告.
func (m *Manager) GenerateReport(title, period string) *ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &ComplianceReport{
		ID:          fmt.Sprintf("report-%d", time.Now().UnixNano()),
		Title:       title,
		GeneratedAt: time.Now(),
		Period:      period,
		Checks:      make([]*RuleCheck, 0),
		Summary:     &ReportSummary{},
	}

	totalCompliant := 0
	totalNonCompliant := 0
	totalNotApplicable := 0

	for ruleID, check := range m.checks {
		report.Checks = append(report.Checks, check)

		rule, exists := m.rules[ruleID]
		if exists {
			report.TotalRules++
			switch check.Status {
			case StatusCompliant:
				totalCompliant++
			case StatusNonCompliant:
				totalNonCompliant++
				switch rule.Severity {
				case SeverityHigh:
					report.Summary.HighRiskFindings++
				case SeverityMedium:
					report.Summary.MediumRiskFindings++
				case SeverityLow:
					report.Summary.LowRiskFindings++
				}
			case StatusNotApplicable:
				totalNotApplicable++
			}
		}
	}

	report.CompliantCount = totalCompliant
	report.NonCompliantCount = totalNonCompliant
	report.NotApplicable = totalNotApplicable

	totalChecked := totalCompliant + totalNonCompliant
	if totalChecked > 0 {
		report.ComplianceRate = float64(totalCompliant) / float64(totalChecked) * 100
	}

	// 生成建议
	if report.Summary.HighRiskFindings > 0 {
		report.Recommendations = append(report.Recommendations, "立即修复高风险合规问题")
	}
	if report.Summary.MediumRiskFindings > 0 {
		report.Recommendations = append(report.Recommendations, "计划修复中风险合规问题")
	}

	m.reports = append(m.reports, report)
	return report
}

// GetReports 获取报告列表.
func (m *Manager) GetReports(limit int) []*ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.reports) {
		limit = len(m.reports)
	}

	start := len(m.reports) - limit
	if start < 0 {
		start = 0
	}

	return m.reports[start:]
}

// GetComplianceRate 获取合规率.
func (m *Manager) GetComplianceRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	compliant := 0
	total := 0
	for _, check := range m.checks {
		if check.Status != StatusNotApplicable {
			total++
			if check.Status == StatusCompliant {
				compliant++
			}
		}
	}

	if total == 0 {
		return 0
	}
	return float64(compliant) / float64(total) * 100
}

// GetFindingsBySeverity 按严重程度获取发现.
func (m *Manager) GetFindingsBySeverity() map[Severity]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	findings := map[Severity]int{
		SeverityHigh:   0,
		SeverityMedium: 0,
		SeverityLow:    0,
	}

	for ruleID, check := range m.checks {
		if check.Status == StatusNonCompliant {
			if rule, exists := m.rules[ruleID]; exists {
				findings[rule.Severity]++
			}
		}
	}

	return findings
}

// ScheduleScan 调度扫描.
func (m *Manager) ScheduleScan(schedule *ScheduledScan) error {
	if schedule == nil || schedule.ID == "" || schedule.Cron == "" {
		return ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	schedule.CreatedAt = time.Now()
	m.schedules = append(m.schedules, schedule)

	return nil
}

// GetSchedules 获取调度列表.
func (m *Manager) GetSchedules() []*ScheduledScan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.schedules
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_rules":     len(m.rules),
		"total_checks":    len(m.checks),
		"total_reports":   len(m.reports),
		"total_schedules": len(m.schedules),
		"compliance_rate": m.GetComplianceRate(),
		"uptime_seconds":  int64(time.Since(m.startTime).Seconds()),
	}
}
