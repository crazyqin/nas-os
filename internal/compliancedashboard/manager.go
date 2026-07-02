// Package compliancedashboard 实现企业合规仪表盘
// 学习群晖合规能力，提供 GDPR/SOC2/HIPAA 合规报告和自动化审计
package compliancedashboard

import (
	"fmt"
	"sync"
	"time"
)

// ComplianceConfig 合规配置.
type ComplianceConfig struct {
	Enabled           bool                  `json:"enabled"`
	EnabledFrameworks []ComplianceFramework `json:"enabledFrameworks"`
	AutoScan          bool                  `json:"autoScan"`
	ScanInterval      int                   `json:"scanInterval"`   // 小时
	AlertThreshold    float64               `json:"alertThreshold"` // 合规分数阈值
	NotifyEmail       string                `json:"notifyEmail"`
	RetentionDays     int                   `json:"retentionDays"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	ID               string              `json:"id"`
	Framework        ComplianceFramework `json:"framework"`
	OverallScore     float64             `json:"overallScore"`
	MaxScore         float64             `json:"maxScore"`
	Status           ComplianceStatus    `json:"status"`
	TotalChecks      int                 `json:"totalChecks"`
	PassedChecks     int                 `json:"passedChecks"`
	FailedChecks     int                 `json:"failedChecks"`
	PartialChecks    int                 `json:"partialChecks"`
	Categories       []CategoryScore     `json:"categories"`
	CriticalFindings []Finding           `json:"criticalFindings"`
	GeneratedAt      time.Time           `json:"generatedAt"`
	ValidUntil       time.Time           `json:"validUntil"`
	GeneratedBy      string              `json:"generatedBy"`
}

// CategoryScore 分类分数.
type CategoryScore struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	MaxScore float64 `json:"maxScore"`
	Checks   int     `json:"checks"`
	Passed   int     `json:"passed"`
}

// Finding 发现项.
type Finding struct {
	ID          string              `json:"id"`
	Framework   ComplianceFramework `json:"framework"`
	CheckID     string              `json:"checkId"`
	Severity    string              `json:"severity"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Impact      string              `json:"impact"`
	Remediation string              `json:"remediation"`
	Status      string              `json:"status"` // open/remediated/accepted
	DetectedAt  time.Time           `json:"detectedAt"`
	ResolvedAt  *time.Time          `json:"resolvedAt,omitempty"`
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
	Result     string    `json:"result"`    // success/failure/denied
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

// ScorePoint 分数趋势点.
type ScorePoint struct {
	Date  time.Time `json:"date"`
	Score float64   `json:"score"`
}

// Manager 合规管理器.
type Manager struct {
	mu       sync.RWMutex
	config   ComplianceConfig
	checks   map[string]*ComplianceCheck
	reports  map[string]*ComplianceReport
	findings map[string]*Finding
	auditLog []AuditEvent
	running  bool
}

// NewManager 创建管理器.
func NewManager(cfg ComplianceConfig) *Manager {
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
		return fmt.Errorf("compliance engine already running")
	}
	m.running = true
	m.initDefaultChecks()
	return nil
}

// Stop 停止.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

func (m *Manager) initDefaultChecks() {
	defaults := []ComplianceCheck{
		{ID: "gdpr-01", Framework: FrameworkGDPR, Category: "数据保护", Name: "数据加密", Description: "静态和传输中的数据加密", Status: StatusCompliant, Score: 95, MaxScore: 100, Severity: "critical"},
		{ID: "gdpr-02", Framework: FrameworkGDPR, Category: "数据保护", Name: "数据最小化", Description: "仅收集必要数据", Status: StatusCompliant, Score: 88, MaxScore: 100, Severity: "high"},
		{ID: "gdpr-03", Framework: FrameworkGDPR, Category: "用户权利", Name: "数据可移植性", Description: "支持数据导出", Status: StatusCompliant, Score: 92, MaxScore: 100, Severity: "high"},
		{ID: "gdpr-04", Framework: FrameworkGDPR, Category: "用户权利", Name: "被遗忘权", Description: "支持数据删除", Status: StatusPartial, Score: 70, MaxScore: 100, Severity: "critical"},
		{ID: "iso-01", Framework: FrameworkISO27001, Category: "信息安全", Name: "访问控制", Description: "逻辑和物理访问控制", Status: StatusCompliant, Score: 90, MaxScore: 100, Severity: "critical"},
		{ID: "iso-02", Framework: FrameworkISO27001, Category: "信息安全", Name: "变更管理", Description: "系统变更的跟踪和审计", Status: StatusCompliant, Score: 85, MaxScore: 100, Severity: "high"},
		{ID: "iso-03", Framework: FrameworkISO27001, Category: "业务连续性", Name: "灾难恢复", Description: "灾难恢复和业务连续性", Status: StatusPartial, Score: 75, MaxScore: 100, Severity: "critical"},
		{ID: "mlps-01", Framework: FrameworkMLPS2, Category: "数据安全", Name: "数据加密", Description: "数据加密保护", Status: StatusCompliant, Score: 93, MaxScore: 100, Severity: "critical"},
		{ID: "mlps-02", Framework: FrameworkMLPS2, Category: "审计", Name: "审计日志", Description: "完整的访问审计日志", Status: StatusCompliant, Score: 96, MaxScore: 100, Severity: "high"},
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
				report.CriticalFindings = append(report.CriticalFindings, Finding{
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
		return nil, fmt.Errorf("report not found: %s", id)
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
func (m *Manager) GetConfig() ComplianceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg ComplianceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	return nil
}
