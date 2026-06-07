// Package compliance 提供合规中心核心逻辑
package compliance

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 合规中心管理器
type Manager struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	rules           map[string]*ComplianceRule
	reports         map[string]*ComplianceReport
	plans           map[string]*RemediationPlan
	classifications map[string]*DataClassification
	scanResults     map[string]*ScanResult
	categories      []DataCategory
	regulations     []string
}

// NewManager 创建合规中心管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:          logger,
		rules:           make(map[string]*ComplianceRule),
		reports:         make(map[string]*ComplianceReport),
		plans:           make(map[string]*RemediationPlan),
		classifications: make(map[string]*DataClassification),
		scanResults:     make(map[string]*ScanResult),
		regulations:     []string{"GDPR", "CCPA", "HIPAA", "PCI-DSS", "SOX"},
	}

	// 初始化默认规则
	m.initDefaultRules()
	m.initDefaultCategories()

	return m
}

// initDefaultRules 初始化默认合规规则
func (m *Manager) initDefaultRules() {
	defaultRules := []*ComplianceRule{
		{
			ID: "gdpr-001", Name: "个人数据加密存储", Regulation: "GDPR",
			Category: "data-protection", Severity: "critical",
			Description: "所有个人身份信息(PII)必须加密存储",
			Condition:   "data.type == 'PII' && !data.encrypted",
			Action:      "alert",
		},
		{
			ID: "gdpr-002", Name: "数据保留期限", Regulation: "GDPR",
			Category: "privacy", Severity: "high",
			Description: "个人数据保留不得超过必要期限",
			Condition:   "data.age > data.retention_period",
			Action:      "delete",
		},
		{
			ID: "ccpa-001", Name: "数据删除请求处理", Regulation: "CCPA",
			Category: "privacy", Severity: "high",
			Description: "必须在45天内响应消费者数据删除请求",
			Condition:   "request.type == 'delete' && request.age > 45",
			Action:      "escalate",
		},
		{
			ID: "ccpa-002", Name: "数据销售披露", Regulation: "CCPA",
			Category: "privacy", Severity: "medium",
			Description: "必须向消费者披露数据销售行为",
			Condition:   "data.sold && !disclosure.provided",
			Action:      "alert",
		},
	}

	now := time.Now()
	for _, rule := range defaultRules {
		rule.Enabled = true
		rule.CreatedAt = now
		rule.UpdatedAt = now
		m.rules[rule.ID] = rule
	}
}

// initDefaultCategories 初始化默认数据分类
func (m *Manager) initDefaultCategories() {
	m.categories = []DataCategory{
		{ID: "pii", Name: "PII", Description: "个人身份信息", Guidelines: "必须加密，访问需授权"},
		{ID: "phi", Name: "PHI", Description: "个人健康信息", Guidelines: "HIPAA保护，严格访问控制"},
		{ID: "financial", Name: "Financial", Description: "财务数据", Guidelines: "PCI-DSS合规，加密传输"},
		{ID: "confidential", Name: "Confidential", Description: "机密信息", Guidelines: "仅限授权人员访问"},
		{ID: "public", Name: "Public", Description: "公开信息", Guidelines: "无特殊保护要求"},
	}
}

// ListRules 获取合规规则列表
func (m *Manager) ListRules(regulation string) []*ComplianceRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*ComplianceRule
	for _, rule := range m.rules {
		if regulation == "" || rule.Regulation == regulation {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetRule 获取单个规则
func (m *Manager) GetRule(id string) (*ComplianceRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule %s not found", id)
	}
	return rule, nil
}

// AddRule 添加合规规则
func (m *Manager) AddRule(rule *ComplianceRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if _, exists := m.rules[rule.ID]; exists {
		return fmt.Errorf("rule %s already exists", rule.ID)
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	if rule.Enabled == false {
		rule.Enabled = true
	}
	m.rules[rule.ID] = rule
	m.logger.Info("Added compliance rule", zap.String("id", rule.ID), zap.String("regulation", rule.Regulation))
	return nil
}

// UpdateRule 更新合规规则
func (m *Manager) UpdateRule(rule *ComplianceRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[rule.ID]; !exists {
		return fmt.Errorf("rule %s not found", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除合规规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	delete(m.rules, id)
	m.logger.Info("Deleted compliance rule", zap.String("id", id))
	return nil
}

// RunScan 执行合规扫描
func (m *Manager) RunScan(req *ScanRequest) (*ScanReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reportID := fmt.Sprintf("scan-%d", time.Now().UnixNano())
	var results []ScanResult

	// 获取要检查的规则
	rules := m.getRulesForScan(req)

	// 模拟扫描结果
	for _, rule := range rules {
		result := ScanResult{
			ID:           fmt.Sprintf("result-%d", time.Now().UnixNano()),
			RuleID:       rule.ID,
			ResourceID:   "sample-resource",
			ResourceType: "file",
			ScannedAt:    time.Now(),
		}

		// 简化的合规检查逻辑
		if rule.Severity == "critical" {
			result.Status = "compliant"
			result.Details = fmt.Sprintf("符合 %s 规则要求", rule.Name)
		} else {
			result.Status = "non-compliant"
			result.Details = fmt.Sprintf("发现不符合 %s 规则的情况", rule.Name)
			result.Remediation = "请按照整改建议进行修复"
		}

		results = append(results, result)
		m.scanResults[result.ID] = &result
	}

	// 统计结果
	compliant, nonCompliant, warnings := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "compliant":
			compliant++
		case "non-compliant":
			nonCompliant++
		case "warning":
			warnings++
		}
	}

	total := len(results)
	complianceRate := 0.0
	if total > 0 {
		complianceRate = float64(compliant) / float64(total) * 100
	}

	report := &ScanReport{
		ID:     reportID,
		ScanID: reportID,
		Summary: ScanSummary{
			TotalChecked:   total,
			Compliant:      compliant,
			NonCompliant:   nonCompliant,
			Warnings:       warnings,
			ComplianceRate: complianceRate,
		},
		Results:     results,
		GeneratedAt: time.Now(),
	}

	m.logger.Info("Compliance scan completed",
		zap.String("id", reportID),
		zap.Int("total", total),
		zap.Int("compliant", compliant),
		zap.Int("non-compliant", nonCompliant),
	)

	return report, nil
}

// getRulesForScan 获取扫描使用的规则
func (m *Manager) getRulesForScan(req *ScanRequest) []*ComplianceRule {
	var rules []*ComplianceRule

	if len(req.Rules) > 0 {
		// 使用指定的规则
		for _, id := range req.Rules {
			if rule, ok := m.rules[id]; ok {
				rules = append(rules, rule)
			}
		}
	} else if len(req.Regulations) > 0 {
		// 按法规筛选
		for _, rule := range m.rules {
			for _, reg := range req.Regulations {
				if rule.Regulation == reg {
					rules = append(rules, rule)
					break
				}
			}
		}
	} else {
		// 使用所有启用的规则
		for _, rule := range m.rules {
			if rule.Enabled {
				rules = append(rules, rule)
			}
		}
	}

	return rules
}

// GenerateReport 生成合规报告
func (m *Manager) GenerateReport(regulation string, period ReportPeriod) (*ComplianceReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	reportID := fmt.Sprintf("report-%d", time.Now().UnixNano())

	// 获取相关规则
	var findings []Finding
	var rules []*ComplianceRule
	for _, rule := range m.rules {
		if regulation == "" || rule.Regulation == regulation {
			rules = append(rules, rule)
		}
	}

	// 基于规则生成发现
	criticalCount := 0
	for _, rule := range rules {
		finding := Finding{
			ID:          fmt.Sprintf("finding-%d", time.Now().UnixNano()),
			RuleID:      rule.ID,
			Severity:    rule.Severity,
			Title:       rule.Name,
			Description: rule.Description,
			Status:      "open",
			DetectedAt:  time.Now(),
		}
		if rule.Severity == "critical" {
			criticalCount++
		}
		findings = append(findings, finding)
	}

	// 生成整改建议
	var recommendations []Recommendation
	for _, finding := range findings {
		rec := Recommendation{
			ID:          fmt.Sprintf("rec-%d", time.Now().UnixNano()),
			FindingID:   finding.ID,
			Priority:    finding.Severity,
			Title:       fmt.Sprintf("修复: %s", finding.Title),
			Description: fmt.Sprintf("请按照合规规则修复 %s 问题", finding.Title),
			Effort:      "medium",
		}
		recommendations = append(recommendations, rec)
	}

	report := &ComplianceReport{
		ID:         reportID,
		Title:      fmt.Sprintf("%s 合规报告", regulation),
		Regulation: regulation,
		Period:     period,
		Summary: ReportSummary{
			OverallScore:   85.5,
			TotalIssues:    len(findings),
			CriticalIssues: criticalCount,
			ResolvedIssues: 0,
			PendingIssues:  len(findings),
		},
		Findings:        findings,
		Recommendations: recommendations,
		Status:          "draft",
		GeneratedAt:     time.Now(),
	}

	m.reports[reportID] = report
	m.logger.Info("Generated compliance report",
		zap.String("id", reportID),
		zap.String("regulation", regulation),
		zap.Int("findings", len(findings)),
	)

	return report, nil
}

// GetReport 获取报告
func (m *Manager) GetReport(id string) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report %s not found", id)
	}
	return report, nil
}

// ListReports 获取报告列表
func (m *Manager) ListReports(regulation string) []*ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var reports []*ComplianceReport
	for _, report := range m.reports {
		if regulation == "" || report.Regulation == regulation {
			reports = append(reports, report)
		}
	}
	return reports
}

// CreatePlan 创建整改计划
func (m *Manager) CreatePlan(reportID string, plan *RemediationPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.reports[reportID]; !exists {
		return fmt.Errorf("report %s not found", reportID)
	}

	plan.ID = fmt.Sprintf("plan-%d", time.Now().UnixNano())
	plan.ReportID = reportID
	plan.Status = "active"
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	m.plans[plan.ID] = plan
	m.logger.Info("Created remediation plan", zap.String("id", plan.ID), zap.String("report_id", reportID))
	return nil
}

// GetPlan 获取整改计划
func (m *Manager) GetPlan(id string) (*RemediationPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", id)
	}
	return plan, nil
}

// ListPlans 获取整改计划列表
func (m *Manager) ListPlans(reportID string) []*RemediationPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var plans []*RemediationPlan
	for _, plan := range m.plans {
		if reportID == "" || plan.ReportID == reportID {
			plans = append(plans, plan)
		}
	}
	return plans
}

// ClassifyData 数据分类
func (m *Manager) ClassifyData(req *ScanDataRequest) (*DataScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	resultID := fmt.Sprintf("classify-%d", time.Now().UnixNano())

	// 模拟数据分类结果
	classifications := []DataClassification{
		{
			ID:           fmt.Sprintf("cls-%d", time.Now().UnixNano()),
			ResourceID:   "sample-file.txt",
			ResourceType: "file",
			Category:     "PII",
			Sensitivity:  "high",
			Tags:         []string{"personal-data", "encrypted"},
			Owner:        "admin",
			Location:     req.Path,
			UpdatedAt:    time.Now(),
		},
	}

	// 统计
	byCategory := make(map[string]int)
	bySensitivity := make(map[string]int)
	for _, c := range classifications {
		byCategory[c.Category]++
		bySensitivity[c.Sensitivity]++
	}

	result := &DataScanResult{
		ID:   resultID,
		Path: req.Path,
		Summary: DataScanSummary{
			TotalFiles:    1,
			TotalSize:     1024,
			ByCategory:    byCategory,
			BySensitivity: bySensitivity,
		},
		Classifications: classifications,
		ScannedAt:       time.Now(),
	}

	m.logger.Info("Data classification completed",
		zap.String("id", resultID),
		zap.String("path", req.Path),
	)

	return result, nil
}

// GetClassification 获取数据分类
func (m *Manager) GetClassification(id string) (*DataClassification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cls, ok := m.classifications[id]
	if !ok {
		return nil, fmt.Errorf("classification %s not found", id)
	}
	return cls, nil
}

// ListClassifications 获取数据分类列表
func (m *Manager) ListClassifications(category string) []*DataClassification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var classifications []*DataClassification
	for _, cls := range m.classifications {
		if category == "" || cls.Category == category {
			classifications = append(classifications, cls)
		}
	}
	return classifications
}

// GetCategories 获取数据类别
func (m *Manager) GetCategories() []DataCategory {
	return m.categories
}

// GetRegulations 获取支持的法规列表
func (m *Manager) GetRegulations() []string {
	return m.regulations
}
