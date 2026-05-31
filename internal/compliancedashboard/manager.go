// Package compliancedashboard - 合规管理器实现
package compliancedashboard

import (
	"fmt"
	"sync"
	"time"
)

// Manager 合规管理器.
type Manager struct {
	mu       sync.RWMutex
	config   Config
	checks   map[string]*ComplianceCheck
	reports  map[string]*ComplianceReport
	findings map[string]*Finding
	auditLog []AuditEvent
	running  bool
}

// AuditEvent 审计事件.
type AuditEvent struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"userId"`
	UserName   string    `json:"userName"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resourceId"`
	Details    string    `json:"details"`
	IPAddress  string    `json:"ipAddress"`
	UserAgent  string    `json:"userAgent"`
	Result     string    `json:"result"`     // success/failure/denied
	RiskLevel  string    `json:"riskLevel"` // low/medium/high/critical
}

// ComplianceStats 合规统计.
type ComplianceStats struct {
	OverallScore      float64                         `json:"overallScore"`
	FrameworkScores   map[ComplianceFramework]float64 `json:"frameworkScores"`
	TotalChecks       int                             `json:"totalChecks"`
	PassedChecks      int                             `json:"passedChecks"`
	FailedChecks      int                             `json:"failedChecks"`
	OpenFindings      int                             `json:"openFindings"`
	CriticalFindings  int                             `json:"criticalFindings"`
	LastScanTime      time.Time                       `json:"lastScanTime"`
	NextScanTime      time.Time                       `json:"nextScanTime"`
	TrendLast30Days   []ScorePoint                    `json:"trendLast30Days"`
	RecentAuditEvents []AuditEvent                    `json:"recentAuditEvents"`
}

// NewManager 创建管理器.
func NewManager(cfg Config) *Manager {
	return &Manager{
		config:   cfg,
		checks:   make(map[string]*ComplianceCheck),
		reports:  make(map[string]*ComplianceReport),
		findings: make(map[string]*Finding),
	}
}

// Start 启动合规引擎.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return ErrAlreadyRunning
	}
	m.running = true
	m.initDefaultChecks()
	return nil
}

// Stop 停止.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return ErrNotRunning
	}
	m.running = false
	return nil
}

func (m *Manager) initDefaultChecks() {
	defaults := []ComplianceCheck{
		{ID: "gdpr-01", Framework: FrameworkGDPR, Category: "数据保护", Name: "数据加密", Description: "静态和传输中的数据加密", Status: StatusCompliant, Score: 95, MaxScore: 100, Severity: "critical"},
		{ID: "gdpr-02", Framework: FrameworkGDPR, Category: "数据保护", Name: "数据最小化", Description: "仅收集必要数据", Status: StatusCompliant, Score: 88, MaxScore: 100, Severity: "high"},
		{ID: "gdpr-03", Framework: FrameworkGDPR, Category: "用户权利", Name: "数据可移植性", Description: "支持数据导出", Status: StatusCompliant, Score: 92, MaxScore: 100, Severity: "high"},
		{ID: "gdpr-04", Framework: FrameworkGDPR, Category: "用户权利", Name: "被遗忘权", Description: "支持数据删除", Status: StatusPartial, Score: 70, MaxScore: 100, Severity: "critical"},
		{ID: "iso-01", Framework: FrameworkISO27001, Category: "安全管理", Name: "安全策略", Description: "信息安全策略文档", Status: StatusCompliant, Score: 90, MaxScore: 100, Severity: "critical"},
		{ID: "iso-02", Framework: FrameworkISO27001, Category: "资产管理", Name: "资产清单", Description: "IT资产清单和分类", Status: StatusCompliant, Score: 85, MaxScore: 100, Severity: "high"},
		{ID: "mlps-01", Framework: FrameworkMLPS2, Category: "安全通信网络", Name: "网络架构", Description: "网络安全架构设计", Status: StatusCompliant, Score: 88, MaxScore: 100, Severity: "critical"},
		{ID: "mlps-02", Framework: FrameworkMLPS2, Category: "安全区域边界", Name: "边界防护", Description: "网络边界安全防护", Status: StatusPartial, Score: 72, MaxScore: 100, Severity: "high"},
	}
	for i := range defaults {
		defaults[i].LastChecked = time.Now()
		defaults[i].CheckedBy = "system"
		m.checks[defaults[i].ID] = &defaults[i]
	}
}

// RunScan 执行合规扫描.
func (m *Manager) RunScan(framework ComplianceFramework) (*ComplianceReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, ErrNotRunning
	}

	report := &ComplianceReport{
		ID:          fmt.Sprintf("rpt-%d", time.Now().UnixNano()),
		Framework:   framework,
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(30 * 24 * time.Hour),
		GeneratedBy: "system",
	}

	categories := make(map[string]*CategoryScore)
	for _, check := range m.checks {
		if framework != "" && check.Framework != framework {
			continue
		}
		report.TotalChecks++
		report.MaxScore += check.MaxScore
		report.OverallScore += check.Score

		switch check.Status {
		case StatusCompliant:
			report.PassedChecks++
		case StatusNonCompliant:
			report.FailedChecks++
			if check.Severity == "critical" || check.Severity == "high" {
				report.Findings = append(report.Findings, Finding{
					ID:          fmt.Sprintf("find-%d", time.Now().UnixNano()),
					Framework:   check.Framework,
					CheckID:     check.ID,
					Severity:    check.Severity,
					Title:       check.Name,
					Description: check.Description,
					Remediation: check.Remediation,
					Status:      "open",
					DetectedAt:  time.Now(),
				})
			}
		case StatusPartial:
			report.PartialChecks++
		}

		cat, ok := categories[check.Category]
		if !ok {
			cat = &CategoryScore{Name: check.Category}
			categories[check.Category] = cat
		}
		cat.Score += check.Score
		cat.MaxScore += check.MaxScore
		cat.Checks++
		if check.Status == StatusCompliant {
			cat.Passed++
		}
	}

	for _, cat := range categories {
		report.Categories = append(report.Categories, *cat)
	}

	if report.MaxScore > 0 {
		pct := report.OverallScore / report.MaxScore * 100
		switch {
		case pct >= 90:
			report.Status = StatusCompliant
		case pct >= 60:
			report.Status = StatusPartial
		default:
			report.Status = StatusNonCompliant
		}
	}

	m.reports[report.ID] = report
	return report, nil
}

// GetReport 获取报告.
func (m *Manager) GetReport(id string) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rpt, ok := m.reports[id]
	if !ok {
		return nil, ErrReportNotFound
	}
	return rpt, nil
}

// ListReports 列出报告.
func (m *Manager) ListReports(framework ComplianceFramework, page, pageSize int) ([]ComplianceReport, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []ComplianceReport
	for _, rpt := range m.reports {
		if framework == "" || rpt.Framework == framework {
			result = append(result, *rpt)
		}
	}
	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// GetStats 获取统计.
func (m *Manager) GetStats() ComplianceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ComplianceStats{
		FrameworkScores: make(map[ComplianceFramework]float64),
		LastScanTime:    time.Now(),
	}

	totalScore := 0.0
	maxScore := 0.0
	for _, check := range m.checks {
		stats.TotalChecks++
		totalScore += check.Score
		maxScore += check.MaxScore
		switch check.Status {
		case StatusCompliant:
			stats.PassedChecks++
		case StatusNonCompliant:
			stats.FailedChecks++
		}
	}
	if maxScore > 0 {
		stats.OverallScore = totalScore / maxScore * 100
	}

	for _, f := range m.findings {
		if f.Status == "open" {
			stats.OpenFindings++
			if f.Severity == "critical" {
				stats.CriticalFindings++
			}
		}
	}

	stats.RecentAuditEvents = m.auditLog
	if len(stats.RecentAuditEvents) > 50 {
		stats.RecentAuditEvents = stats.RecentAuditEvents[len(stats.RecentAuditEvents)-50:]
	}

	return stats
}

// LogAuditEvent 记录审计事件.
func (m *Manager) LogAuditEvent(event AuditEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	event.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	event.Timestamp = time.Now()
	m.auditLog = append(m.auditLog, event)
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[len(m.auditLog)-5000:]
	}
}

// GetAuditLog 获取审计日志.
func (m *Manager) GetAuditLog(userID, action string, page, pageSize int) ([]AuditEvent, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []AuditEvent
	for _, event := range m.auditLog {
		if (userID == "" || event.UserID == userID) &&
			(action == "" || event.Action == action) {
			result = append(result, event)
		}
	}
	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// GetChecks 获取检查项列表.
func (m *Manager) GetChecks(framework ComplianceFramework, status ComplianceStatus) []ComplianceCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []ComplianceCheck
	for _, check := range m.checks {
		if (framework == "" || check.Framework == framework) &&
			(status == "" || check.Status == status) {
			result = append(result, *check)
		}
	}
	return result
}

// GetFindings 获取发现项.
func (m *Manager) GetFindings(status string) []Finding {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Finding
	for _, f := range m.findings {
		if status == "" || f.Status == status {
			result = append(result, *f)
		}
	}
	return result
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	return nil
}
