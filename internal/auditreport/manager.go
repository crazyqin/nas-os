// Package auditreport 提供安全审计报告核心业务逻辑
package auditreport

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 审计报告管理器.
type Manager struct {
	reports  map[string]*AuditReport
	findings map[string]*Finding
	checks   map[string]*ComplianceCheck
	events   []*AuditEvent
	scans    map[string]*SecurityScanResult
	mu       sync.RWMutex
}

// NewManager 创建审计报告管理器.
func NewManager() *Manager {
	return &Manager{
		reports:  make(map[string]*AuditReport),
		findings: make(map[string]*Finding),
		checks:   make(map[string]*ComplianceCheck),
		events:   make([]*AuditEvent, 0),
		scans:    make(map[string]*SecurityScanResult),
	}
}

// ========== 报告管理 ==========

// GenerateReport 生成审计报告.
func (m *Manager) GenerateReport(req GenerateReportRequest) *AuditReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 收集当前所有未解决的发现
	var reportFindings []Finding
	for _, f := range m.findings {
		if f.Status != StatusResolved {
			reportFindings = append(reportFindings, *f)
		}
	}

	// 基于发现计算评分
	score := m.calculateScoreFromFindings(reportFindings)

	// 生成摘要
	summary := m.generateSummary(reportFindings, score)

	report := &AuditReport{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Period:      req.Period,
		GeneratedAt: now,
		Score:       score,
		Findings:    reportFindings,
		Summary:     summary,
	}

	m.reports[report.ID] = report
	return report
}

// GetReport 获取报告详情.
func (m *Manager) GetReport(id string) (*AuditReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report %q not found", id)
	}
	return report, nil
}

// ListReports 列出所有报告.
func (m *Manager) ListReports() []*AuditReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*AuditReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].GeneratedAt.After(reports[j].GeneratedAt)
	})

	return reports
}

// DeleteReport 删除报告.
func (m *Manager) DeleteReport(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.reports[id]; !ok {
		return fmt.Errorf("report %q not found", id)
	}
	delete(m.reports, id)
	return nil
}

// ========== 发现管理 ==========

// AddFinding 添加安全发现.
func (m *Manager) AddFinding(finding Finding) *Finding {
	m.mu.Lock()
	defer m.mu.Unlock()

	if finding.ID == "" {
		finding.ID = uuid.New().String()
	}
	if finding.Status == "" {
		finding.Status = StatusOpen
	}

	m.findings[finding.ID] = &finding
	return &finding
}

// UpdateFinding 更新发现.
func (m *Manager) UpdateFinding(id string, req UpdateFindingRequest) (*Finding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	finding, ok := m.findings[id]
	if !ok {
		return nil, fmt.Errorf("finding %q not found", id)
	}

	if req.Status != nil {
		finding.Status = *req.Status
	}
	if req.Recommendation != nil {
		finding.Recommendation = *req.Recommendation
	}

	return finding, nil
}

// ResolveFinding 解决发现.
func (m *Manager) ResolveFinding(id string) (*Finding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	finding, ok := m.findings[id]
	if !ok {
		return nil, fmt.Errorf("finding %q not found", id)
	}

	finding.Status = StatusResolved
	return finding, nil
}

// ListFindings 列出所有发现.
func (m *Manager) ListFindings() []*Finding {
	m.mu.RLock()
	defer m.mu.RUnlock()

	findings := make([]*Finding, 0, len(m.findings))
	for _, f := range m.findings {
		findings = append(findings, f)
	}

	// 按严重程度排序: critical > high > medium > low > info
	severityOrder := map[Severity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
		SeverityInfo:     4,
	}

	sort.Slice(findings, func(i, j int) bool {
		return severityOrder[findings[i].Severity] < severityOrder[findings[j].Severity]
	})

	return findings
}

// ========== 合规检查 ==========

// RunComplianceCheck 运行合规检查.
func (m *Manager) RunComplianceCheck(req RunComplianceCheckRequest) *ComplianceCheck {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟合规检查
	items := m.runComplianceItems(req.Standard)

	passed, failed := 0, 0
	for _, item := range items {
		if item.Passed {
			passed++
		} else {
			failed++
		}
	}

	total := passed + failed
	score := float64(0)
	if total > 0 {
		score = float64(passed) / float64(total) * 100
	}

	check := &ComplianceCheck{
		ID:       uuid.New().String(),
		Standard: req.Standard,
		Score:    score,
		Passed:   passed,
		Failed:   failed,
		Items:    items,
	}

	m.checks[check.ID] = check
	return check
}

// GetComplianceStatus 获取合规状态.
func (m *Manager) GetComplianceStatus() map[string]*ComplianceCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ComplianceCheck)
	for _, c := range m.checks {
		// 每个标准只保留最新的检查结果
		if existing, ok := result[c.Standard]; !ok || c.ID > existing.ID {
			result[c.Standard] = c
		}
	}
	return result
}

// ListComplianceChecks 列出所有合规检查.
func (m *Manager) ListComplianceChecks() []*ComplianceCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	checks := make([]*ComplianceCheck, 0, len(m.checks))
	for _, c := range m.checks {
		checks = append(checks, c)
	}
	return checks
}

// ========== 审计日志 ==========

// LogEvent 记录审计事件.
func (m *Manager) LogEvent(event AuditEvent) *AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	m.events = append(m.events, &event)
	return &event
}

// QueryEvents 查询审计事件.
func (m *Manager) QueryEvents(req QueryEventsRequest) []*AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*AuditEvent
	for _, e := range m.events {
		if req.UserID != "" && e.UserID != req.UserID {
			continue
		}
		if req.Action != "" && !strings.Contains(strings.ToLower(e.Action), strings.ToLower(req.Action)) {
			continue
		}
		if req.Resource != "" && !strings.Contains(strings.ToLower(e.Resource), strings.ToLower(req.Resource)) {
			continue
		}
		if req.Result != "" && e.Result != req.Result {
			continue
		}
		result = append(result, e)
	}

	// 按时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 限制数量
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > len(result) {
		limit = len(result)
	}

	return result[:limit]
}

// ExportEvents 导出审计事件.
func (m *Manager) ExportEvents(req ExportEventsRequest) []*AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*AuditEvent
	for _, e := range m.events {
		if req.StartTime != nil && e.Timestamp.Before(*req.StartTime) {
			continue
		}
		if req.EndTime != nil && e.Timestamp.After(*req.EndTime) {
			continue
		}
		result = append(result, e)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// ========== 安全扫描 ==========

// RunSecurityScan 运行安全扫描.
func (m *Manager) RunSecurityScan(req RunSecurityScanRequest) *SecurityScanResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	findings := m.runSecurityChecks(req.ScanType)

	// 统计各严重级别数量
	critical, high, medium, low, info := 0, 0, 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityHigh:
			high++
		case SeverityMedium:
			medium++
		case SeverityLow:
			low++
		case SeverityInfo:
			info++
		}

		// 将发现添加到全局发现列表
		m.findings[f.ID] = &f
	}

	result := &SecurityScanResult{
		ID:          uuid.New().String(),
		ScanType:    req.ScanType,
		StartedAt:   now,
		CompletedAt: now, // 简化处理，实际应该记录完成时间
		Total:       len(findings),
		Critical:    critical,
		High:        high,
		Medium:      medium,
		Low:         low,
		Info:        info,
		Findings:    findings,
	}

	m.scans[result.ID] = result
	return result
}

// GetScanResults 获取扫描结果.
func (m *Manager) GetScanResults(id string) (*SecurityScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.scans[id]
	if !ok {
		return nil, fmt.Errorf("scan result %q not found", id)
	}
	return result, nil
}

// ========== 内部辅助方法 ==========

// calculateScoreFromFindings 基于发现计算安全评分.
func (m *Manager) calculateScoreFromFindings(findings []Finding) float64 {
	score := 100.0
	for _, f := range findings {
		if f.Status == StatusResolved {
			continue
		}
		switch f.Severity {
		case SeverityCritical:
			score -= 15
		case SeverityHigh:
			score -= 10
		case SeverityMedium:
			score -= 5
		case SeverityLow:
			score -= 2
		case SeverityInfo:
			score -= 0.5
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// generateSummary 生成报告摘要.
func (m *Manager) generateSummary(findings []Finding, score float64) string {
	critical, high, medium, low, info := 0, 0, 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityHigh:
			high++
		case SeverityMedium:
			medium++
		case SeverityLow:
			low++
		case SeverityInfo:
			info++
		}
	}

	return fmt.Sprintf("安全评分: %.1f/100. 发现 %d 个问题: %d 严重, %d 高危, %d 中危, %d 低危, %d 信息.",
		score, len(findings), critical, high, medium, low, info)
}

// runComplianceItems 运行合规检查项.
func (m *Manager) runComplianceItems(standard string) []ComplianceItem {
	// 模拟不同合规标准的检查项
	switch strings.ToUpper(standard) {
	case "SOC2":
		return []ComplianceItem{
			{ID: "SOC2-001", Description: "访问控制策略已实施", Passed: true, Detail: "RBAC 已配置"},
			{ID: "SOC2-002", Description: "数据加密传输", Passed: true, Detail: "TLS 1.3 已启用"},
			{ID: "SOC2-003", Description: "审计日志完整性", Passed: true, Detail: "日志不可篡改"},
			{ID: "SOC2-004", Description: "事件响应计划", Passed: false, Detail: "缺少书面事件响应计划"},
			{ID: "SOC2-005", Description: "定期安全培训", Passed: true, Detail: "培训记录完整"},
		}
	case "GDPR":
		return []ComplianceItem{
			{ID: "GDPR-001", Description: "数据处理同意机制", Passed: true, Detail: "同意记录已保存"},
			{ID: "GDPR-002", Description: "数据主体访问权", Passed: true, Detail: "API 支持数据导出"},
			{ID: "GDPR-003", Description: "数据泄露通知", Passed: false, Detail: "72小时通知流程未建立"},
			{ID: "GDPR-004", Description: "数据保护影响评估", Passed: true, Detail: "DPIA 已完成"},
			{ID: "GDPR-005", Description: "数据保留策略", Passed: true, Detail: "自动清理机制已配置"},
		}
	case "HIPAA":
		return []ComplianceItem{
			{ID: "HIPAA-001", Description: "PHI 数据加密", Passed: true, Detail: "静态和传输加密已启用"},
			{ID: "HIPAA-002", Description: "访问控制", Passed: true, Detail: "最小权限原则已实施"},
			{ID: "HIPAA-003", Description: "审计追踪", Passed: true, Detail: "所有 PHI 访问已记录"},
			{ID: "HIPAA-004", Description: "业务伙伴协议", Passed: false, Detail: "BAA 未全部签署"},
			{ID: "HIPAA-005", Description: "安全风险评估", Passed: true, Detail: "年度评估已完成"},
		}
	default:
		return []ComplianceItem{
			{ID: "GEN-001", Description: "密码策略", Passed: true, Detail: "复杂度要求已实施"},
			{ID: "GEN-002", Description: "会话管理", Passed: true, Detail: "超时和安全标志已配置"},
			{ID: "GEN-003", Description: "输入验证", Passed: true, Detail: "参数化查询已使用"},
			{ID: "GEN-004", Description: "错误处理", Passed: false, Detail: "部分错误信息泄露内部细节"},
		}
	}
}

// runSecurityChecks 运行安全检查.
func (m *Manager) runSecurityChecks(scanType string) []Finding {
	switch strings.ToLower(scanType) {
	case "vulnerability":
		return []Finding{
			{ID: uuid.New().String(), Severity: SeverityHigh, Category: "依赖漏洞", Description: "第三方库存在已知漏洞 CVE-2024-XXXX", Recommendation: "升级受影响的依赖包到最新版本", Status: StatusOpen},
			{ID: uuid.New().String(), Severity: SeverityMedium, Category: "配置问题", Description: "TLS 版本配置可优化", Recommendation: "禁用 TLS 1.0/1.1，仅保留 TLS 1.2+", Status: StatusOpen},
			{ID: uuid.New().String(), Severity: SeverityLow, Category: "信息泄露", Description: "服务器版本号在响应头中暴露", Recommendation: "移除 Server 响应头中的版本信息", Status: StatusOpen},
		}
	case "configuration":
		return []Finding{
			{ID: uuid.New().String(), Severity: SeverityMedium, Category: "默认配置", Description: "使用默认管理员密码", Recommendation: "修改所有默认密码", Status: StatusOpen},
			{ID: uuid.New().String(), Severity: SeverityLow, Category: "日志配置", Description: "日志级别设置为 debug", Recommendation: "生产环境使用 info 或 warn 级别", Status: StatusOpen},
			{ID: uuid.New().String(), Severity: SeverityInfo, Category: "备份配置", Description: "自动备份未配置", Recommendation: "配置定期自动备份", Status: StatusOpen},
		}
	case "network":
		return []Finding{
			{ID: uuid.New().String(), Severity: SeverityCritical, Category: "端口暴露", Description: "不必要的端口对外开放", Recommendation: "关闭非必需端口，配置防火墙规则", Status: StatusOpen},
			{ID: uuid.New().String(), Severity: SeverityHigh, Category: "网络隔离", Description: "管理接口可从公网访问", Recommendation: "限制管理接口仅内网访问", Status: StatusOpen},
			{ID: uuid.New().String(), Severity: SeverityMedium, Category: "DNS配置", Description: "DNS 解析未启用 DNSSEC", Recommendation: "启用 DNSSEC 防止 DNS 欺骗", Status: StatusOpen},
		}
	default:
		return []Finding{
			{ID: uuid.New().String(), Severity: SeverityMedium, Category: "综合检查", Description: "发现潜在安全问题", Recommendation: "建议进行全面安全审查", Status: StatusOpen},
		}
	}
}
