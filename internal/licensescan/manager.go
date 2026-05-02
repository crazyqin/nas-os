package licensescan

import (
	"fmt"
	"sync"
	"time"
)

// Manager 许可证合规扫描管理器.
// 管理扫描任务、策略、报告和告警.
type Manager struct {
	mu       sync.RWMutex
	scanner  *Scanner
	policies map[string]*Policy
	scans    map[string]*ScanResult
	reports  map[string]*Report
	alerts   []Alert
	history  []ScanResult

	// 告警回调
	alertFunc func(alert Alert)
}

// NewManager 创建许可证合规扫描管理器.
func NewManager() *Manager {
	defaultPolicy := &Policy{
		ID:          "default",
		Name:        "默认策略",
		Description: "系统默认的许可证合规策略",
		Whitelist:   []string{"MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "ISC", "Unlicense", "CC0-1.0", "0BSD", "Zlib"},
		Blacklist:   []string{"AGPL-3.0", "AGPL-3.0-only", "AGPL-3.0-or-later", "SSPL-1.0", "OSL-3.0"},
		Graylist:    []string{"GPL-2.0", "GPL-3.0", "GPL-2.0-only", "GPL-3.0-only", "LGPL-2.0", "LGPL-2.1", "LGPL-3.0", "MPL-2.0", "EPL-1.0", "EPL-2.0"},
		DefaultList: ListGraylist,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m := &Manager{
		scanner:  NewScanner(defaultPolicy),
		policies: map[string]*Policy{"default": defaultPolicy},
		scans:    make(map[string]*ScanResult),
		reports:  make(map[string]*Report),
	}

	return m
}

// SetAlertFunc 设置告警回调函数.
func (m *Manager) SetAlertFunc(fn func(alert Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertFunc = fn
}

// ========== 策略管理 ==========

// CreatePolicy 创建合规策略.
func (m *Manager) CreatePolicy(p *Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p.ID == "" {
		p.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}
	if p.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	if _, exists := m.policies[p.ID]; exists {
		return fmt.Errorf("策略 %s 已存在", p.ID)
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	m.policies[p.ID] = p
	return nil
}

// UpdatePolicy 更新合规策略.
func (m *Manager) UpdatePolicy(p *Policy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[p.ID]; !exists {
		return fmt.Errorf("策略 %s 不存在", p.ID)
	}

	p.UpdatedAt = time.Now()
	m.policies[p.ID] = p
	return nil
}

// GetPolicy 获取合规策略.
func (m *Manager) GetPolicy(id string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("策略 %s 不存在", id)
	}
	return p, nil
}

// ListPolicies 列出所有合规策略.
func (m *Manager) ListPolicies() []Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]Policy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, *p)
	}
	return policies
}

// DeletePolicy 删除合规策略（默认策略不可删除）.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "default" {
		return fmt.Errorf("默认策略不可删除")
	}

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略 %s 不存在", id)
	}

	delete(m.policies, id)
	return nil
}

// ========== 扫描管理 ==========

// RunDockerScan 执行Docker镜像扫描.
func (m *Manager) RunDockerScan(imageRef, policyID string) (*ScanResult, error) {
	m.mu.RLock()
	policy := m.getPolicyOrDefault(policyID)
	m.scanner.SetPolicy(policy)
	m.mu.RUnlock()

	result, err := m.scanner.ScanDockerImage(imageRef)
	if err == nil {
		m.mu.Lock()
		m.scans[result.ID] = result
		m.history = append(m.history, *result)
		m.checkAndAlert(result)
		m.mu.Unlock()
	}
	return result, err
}

// RunGoModScan 执行Go模块扫描.
func (m *Manager) RunGoModScan(goModPath, policyID string) (*ScanResult, error) {
	m.mu.RLock()
	policy := m.getPolicyOrDefault(policyID)
	m.scanner.SetPolicy(policy)
	m.mu.RUnlock()

	result, err := m.scanner.ScanGoMod(goModPath)
	if err == nil {
		m.mu.Lock()
		m.scans[result.ID] = result
		m.history = append(m.history, *result)
		m.checkAndAlert(result)
		m.mu.Unlock()
	}
	return result, err
}

// GetScanResult 获取扫描结果.
func (m *Manager) GetScanResult(id string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.scans[id]
	if !ok {
		return nil, fmt.Errorf("扫描结果 %s 不存在", id)
	}
	return r, nil
}

// ListScans 列出所有扫描结果.
func (m *Manager) ListScans() []ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scans := make([]ScanResult, 0, len(m.scans))
	for _, s := range m.scans {
		scans = append(scans, *s)
	}
	return scans
}

// ========== 报告管理 ==========

// GenerateReport 生成扫描报告.
func (m *Manager) GenerateReport(title string, format ReportFormat, scanIDs []string) (*Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []ScanResult
	if len(scanIDs) == 0 {
		// 使用所有扫描结果
		for _, r := range m.scans {
			results = append(results, *r)
		}
	} else {
		for _, id := range scanIDs {
			if r, ok := m.scans[id]; ok {
				results = append(results, *r)
			}
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("没有可用的扫描结果")
	}

	report := &Report{
		ID:          fmt.Sprintf("report-%d", time.Now().UnixNano()),
		Title:       title,
		Format:      format,
		Results:     results,
		Summary:     buildReportSummary(results),
		GeneratedAt: time.Now(),
	}

	m.reports[report.ID] = report
	return report, nil
}

// GetReport 获取报告.
func (m *Manager) GetReport(id string) (*Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("报告 %s 不存在", id)
	}
	return r, nil
}

// ListReports 列出所有报告.
func (m *Manager) ListReports() []Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]Report, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, *r)
	}
	return reports
}

// ========== 仪表盘 ==========

// GetDashboardData 获取合规仪表盘数据.
func (m *Manager) GetDashboardData() DashboardData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := DashboardData{
		LicenseBreakdown: make(map[Category]int),
		TopViolations:    make([]ViolationSummary, 0),
		LastScanTime:     time.Time{},
	}

	totalCompliant := 0
	totalScans := len(m.scans)
	totalViolations := 0
	violationMap := make(map[string]*ViolationSummary)

	for _, scan := range m.scans {
		if scan.Status == StatusComplete {
			if len(scan.Violations) == 0 {
				totalCompliant++
			}
			totalViolations += len(scan.Violations)

			for _, lic := range scan.Licenses {
				data.LicenseBreakdown[lic.Category]++
			}

			for _, v := range scan.Violations {
				if vs, ok := violationMap[v.LicenseName]; ok {
					vs.Count++
					if severityRank(v.Severity) > severityRank(vs.Severity) {
						vs.Severity = v.Severity
					}
				} else {
					violationMap[v.LicenseName] = &ViolationSummary{
						LicenseName: v.LicenseName,
						Count:       1,
						Severity:    v.Severity,
					}
				}
			}

			if scan.FinishedAt.After(data.LastScanTime) {
				data.LastScanTime = scan.FinishedAt
			}
		}
	}

	// 构建高频违规列表
	for _, vs := range violationMap {
		data.TopViolations = append(data.TopViolations, *vs)
	}
	// 简单排序（按次数降序）
	for i := 0; i < len(data.TopViolations); i++ {
		for j := i + 1; j < len(data.TopViolations); j++ {
			if data.TopViolations[j].Count > data.TopViolations[i].Count {
				data.TopViolations[i], data.TopViolations[j] = data.TopViolations[j], data.TopViolations[i]
			}
		}
	}
	if len(data.TopViolations) > 10 {
		data.TopViolations = data.TopViolations[:10]
	}

	data.TotalScans = totalScans
	data.TotalViolations = totalViolations
	if totalScans > 0 {
		data.ComplianceRate = float64(totalCompliant) / float64(totalScans) * 100
	}

	// 策略状态
	if p, ok := m.policies["default"]; ok {
		data.PolicyStatus = PolicyStatus{
			WhitelistCount: len(p.Whitelist),
			BlacklistCount: len(p.Blacklist),
			GraylistCount:  len(p.Graylist),
		}
	}

	// 最近扫描
	recent := make([]ScanResult, 0)
	for _, scan := range m.scans {
		recent = append(recent, *scan)
	}
	for i := 0; i < len(recent); i++ {
		for j := i + 1; j < len(recent); j++ {
			if recent[j].FinishedAt.After(recent[i].FinishedAt) {
				recent[i], recent[j] = recent[j], recent[i]
			}
		}
	}
	if len(recent) > 5 {
		recent = recent[:5]
	}
	data.RecentScans = recent

	return data
}

// ========== 告警管理 ==========

// GetAlerts 获取所有告警.
func (m *Manager) GetAlerts() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]Alert, len(m.alerts))
	copy(alerts, m.alerts)
	return alerts
}

// ========== 内部方法 ==========

// getPolicyOrDefault 获取策略，不存在则使用默认策略.
func (m *Manager) getPolicyOrDefault(policyID string) *Policy {
	if policyID == "" {
		policyID = "default"
	}
	if p, ok := m.policies[policyID]; ok {
		return p
	}
	return m.policies["default"]
}

// checkAndAlert 检查扫描结果并触发告警.
func (m *Manager) checkAndAlert(result *ScanResult) {
	if len(result.Violations) == 0 {
		return
	}

	// 确定最高严重程度
	highestSeverity := SeverityLow
	for _, v := range result.Violations {
		if severityRank(v.Severity) > severityRank(highestSeverity) {
			highestSeverity = v.Severity
		}
	}

	alert := Alert{
		ID:         fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		ScanID:     result.ID,
		Severity:   highestSeverity,
		Message:    fmt.Sprintf("扫描 %s 发现 %d 个合规违规项", result.Target, len(result.Violations)),
		Violations: result.Violations,
		CreatedAt:  time.Now(),
	}

	m.alerts = append(m.alerts, alert)

	if m.alertFunc != nil {
		m.alertFunc(alert)
	}
}

// buildReportSummary 构建报告摘要.
func buildReportSummary(results []ScanResult) ReportSummary {
	summary := ReportSummary{
		TotalScans: len(results),
		ScanTime:   time.Now(),
	}

	for _, r := range results {
		summary.TotalLicenses += r.Summary.TotalLicenses
		summary.TotalViolations += len(r.Violations)
		if len(r.Violations) == 0 && r.Status == StatusComplete {
			summary.Compliant++
		} else if len(r.Violations) > 0 {
			summary.NonCompliant++
		}
		for _, v := range r.Violations {
			if v.ListType == ListGraylist {
				summary.NeedsReview++
			}
		}
	}

	return summary
}

// severityRank 严重程度排序权重.
func severityRank(s Severity) int {
	switch s {
	case SeverityLow:
		return 1
	case SeverityMedium:
		return 2
	case SeverityHigh:
		return 3
	case SeverityCritical:
		return 4
	default:
		return 0
	}
}
