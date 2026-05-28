// Package complianceengine - 合规引擎管理器
// 支持合规策略管理、自动化合规检查、合规报告生成
package complianceengine

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 合规引擎管理器
type Manager struct {
	mu sync.RWMutex

	// 配置
	config EngineConfig

	// 规则管理
	rules map[string]*ComplianceRule

	// 扫描管理
	scans map[string]*ComplianceScan

	// 报告管理
	reports map[string]*ComplianceReport

	// 告警管理
	alerts map[string]*ComplianceAlert

	// 任务管理
	tasks map[string]*RemediationTask

	// 统计
	stats *ComplianceStats

	// 差距分析
	gapAnalyses map[string]*GapAnalysis
}

// NewManager 创建合规引擎管理器
func NewManager(config EngineConfig) *Manager {
	return &Manager{
		config:      config,
		rules:       make(map[string]*ComplianceRule),
		scans:       make(map[string]*ComplianceScan),
		reports:     make(map[string]*ComplianceReport),
		alerts:      make(map[string]*ComplianceAlert),
		tasks:       make(map[string]*RemediationTask),
		stats:       &ComplianceStats{},
		gapAnalyses: make(map[string]*GapAnalysis),
	}
}

// ============================================================
// 规则管理
// ============================================================

// CreateRule 创建合规规则
func (m *Manager) CreateRule(rule ComplianceRule) (*ComplianceRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	if _, exists := m.rules[rule.ID]; exists {
		return nil, fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = &rule

	log.Printf("[合规引擎] 创建规则: %s - %s", rule.ID, rule.Title)
	return &rule, nil
}

// GetRule 获取合规规则
func (m *Manager) GetRule(id string) (*ComplianceRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则 %s 不存在", id)
	}
	return rule, nil
}

// ListRules 列出所有合规规则
func (m *Manager) ListRules(standard ComplianceStandard, category RuleCategory) []*ComplianceRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ComplianceRule
	for _, rule := range m.rules {
		if standard != "" && rule.Standard != standard {
			continue
		}
		if category != "" && rule.Category != category {
			continue
		}
		result = append(result, rule)
	}
	return result
}

// UpdateRule 更新合规规则
func (m *Manager) UpdateRule(id string, rule ComplianceRule) (*ComplianceRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则 %s 不存在", id)
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.rules[id] = &rule

	log.Printf("[合规引擎] 更新规则: %s", id)
	return &rule, nil
}

// DeleteRule 删除合规规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("规则 %s 不存在", id)
	}

	delete(m.rules, id)
	log.Printf("[合规引擎] 删除规则: %s", id)
	return nil
}

// ============================================================
// 扫描管理
// ============================================================

// StartScan 启动合规扫描
func (m *Manager) StartScan(standards []ComplianceStandard) (*ComplianceScan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	scanID := uuid.New().String()
	scan := &ComplianceScan{
		ID:        scanID,
		Standards: standards,
		Status:    StatusRunning,
		StartTime: time.Now(),
	}

	m.scans[scanID] = scan

	// 异步执行扫描
	go m.executeScan(scanID)

	log.Printf("[合规引擎] 启动扫描: %s, 标准: %v", scanID, standards)
	return scan, nil
}

// executeScan 执行扫描
func (m *Manager) executeScan(scanID string) {
	m.mu.Lock()
	scan, exists := m.scans[scanID]
	if !exists {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// 收集适用的规则
	var applicableRules []*ComplianceRule
	m.mu.RLock()
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		for _, s := range scan.Standards {
			if rule.Standard == s {
				applicableRules = append(applicableRules, rule)
				break
			}
		}
	}
	m.mu.RUnlock()

	// 执行检查
	var checks []CheckDetail
	passed, failed, warned, skipped, errored := 0, 0, 0, 0, 0

	for _, rule := range applicableRules {
		check := m.checkRule(rule)
		checks = append(checks, check)

		switch check.Result {
		case ResultPass:
			passed++
		case ResultFail:
			failed++
		case ResultWarning:
			warned++
		case ResultSkip:
			skipped++
		case ResultError:
			errored++
		}
	}

	// 更新扫描结果
	m.mu.Lock()
	scan.EndTime = time.Now()
	scan.Duration = scan.EndTime.Sub(scan.StartTime)
	scan.TotalRules = len(applicableRules)
	scan.PassedRules = passed
	scan.FailedRules = failed
	scan.WarnRules = warned
	scan.SkipRules = skipped
	scan.ErrorRules = errored
	scan.Checks = checks
	scan.Status = StatusCompleted

	// 计算合规分数
	if scan.TotalRules > 0 {
		scan.Score = float64(scan.PassedRules) / float64(scan.TotalRules) * 100
	}

	// 更新统计
	m.stats.TotalScans++
	m.stats.SuccessfulScans++
	m.stats.LastScanTime = &scan.EndTime
	m.stats.LastScanStatus = scan.Status
	m.stats.UpdateScore(scan.Score)
	m.mu.Unlock()

	// 生成告警
	for _, check := range checks {
		if check.Result == ResultFail {
			m.createAlert(check.RuleID, check.Message)
		}
	}

	log.Printf("[合规引擎] 扫描完成: %s, 分数: %.2f", scanID, scan.Score)
}

// checkRule 检查单个规则
func (m *Manager) checkRule(rule *ComplianceRule) CheckDetail {
	// 模拟检查逻辑
	check := CheckDetail{
		RuleID:    rule.ID,
		CheckedAt: time.Now(),
		Duration:  time.Millisecond * 100,
	}

	// 根据规则类别执行不同检查
	switch rule.Category {
	case CategoryAccessControl:
		check.Result = ResultPass
		check.Message = "访问控制检查通过"
	case CategoryAuditLogging:
		check.Result = ResultPass
		check.Message = "审计日志检查通过"
	case CategoryDataProtection:
		check.Result = ResultWarning
		check.Message = "数据保护建议启用加密"
	case CategoryNetworkSecurity:
		check.Result = ResultPass
		check.Message = "网络安全检查通过"
	default:
		check.Result = ResultPass
		check.Message = "检查通过"
	}

	return check
}

// createAlert 创建告警
func (m *Manager) createAlert(ruleID, message string) {
	alert := &ComplianceAlert{
		ID:        uuid.New().String(),
		RuleID:    ruleID,
		Severity:  AlertHigh,
		Title:     "合规检查失败",
		Message:   message,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.alerts[alert.ID] = alert
	m.stats.TotalAlerts++
	m.stats.ActiveAlerts++

	log.Printf("[合规引擎] 创建告警: %s - %s", alert.ID, message)
}

// GetScan 获取扫描结果
func (m *Manager) GetScan(id string) (*ComplianceScan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scan, exists := m.scans[id]
	if !exists {
		return nil, fmt.Errorf("扫描 %s 不存在", id)
	}
	return scan, nil
}

// ListScans 列出所有扫描
func (m *Manager) ListScans(status ScanStatus) []*ComplianceScan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ComplianceScan
	for _, scan := range m.scans {
		if status != "" && scan.Status != status {
			continue
		}
		result = append(result, scan)
	}
	return result
}

// ============================================================
// 报告管理
// ============================================================

// GenerateReport 生成合规报告
func (m *Manager) GenerateReport(scanID string, format ReportFormat) (*ComplianceReport, error) {
	m.mu.RLock()
	scan, exists := m.scans[scanID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("扫描 %s 不存在", scanID)
	}
	m.mu.RUnlock()

	reportID := uuid.New().String()
	report := &ComplianceReport{
		ID:          reportID,
		Title:       fmt.Sprintf("合规报告 - %s", time.Now().Format("2006-01-02")),
		Format:      format,
		ScanID:      scanID,
		Standards:   scan.Standards,
		GeneratedAt: time.Now(),
		Summary: ReportSummary{
			ComplianceScore: scan.Score,
			TotalChecks:     scan.TotalRules,
			PassedChecks:    scan.PassedRules,
			FailedChecks:    scan.FailedRules,
			WarningChecks:   scan.WarnRules,
		},
	}

	// 计算各严重程度的发现数
	for _, check := range scan.Checks {
		if check.Result == ResultFail {
			rule, err := m.GetRule(check.RuleID)
			if err == nil {
				switch rule.Severity {
				case SeverityCritical:
					report.Summary.CriticalFindings++
				case SeverityHigh:
					report.Summary.HighFindings++
				case SeverityMedium:
					report.Summary.MediumFindings++
				case SeverityLow:
					report.Summary.LowFindings++
				}
			}
		}
	}

	m.mu.Lock()
	m.reports[reportID] = report
	m.mu.Unlock()

	log.Printf("[合规引擎] 生成报告: %s, 扫描: %s", reportID, scanID)
	return report, nil
}

// GetReport 获取报告
func (m *Manager) GetReport(id string) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, exists := m.reports[id]
	if !exists {
		return nil, fmt.Errorf("报告 %s 不存在", id)
	}
	return report, nil
}

// ListReports 列出所有报告
func (m *Manager) ListReports() []*ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ComplianceReport
	for _, report := range m.reports {
		result = append(result, report)
	}
	return result
}

// ============================================================
// 差距分析
// ============================================================

// PerformGapAnalysis 执行差距分析
func (m *Manager) PerformGapAnalysis(standards []ComplianceStandard) (*GapAnalysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	analysisID := uuid.New().String()
	analysis := &GapAnalysis{
		ID:          analysisID,
		Standards:   standards,
		GeneratedAt: time.Now(),
	}

	// 收集规则和检查结果
	categoryStats := make(map[RuleCategory]*CategoryGap)
	var gaps []ComplianceGap
	var actions []RecommendedAction

	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		// 检查是否适用
		applicable := false
		for _, s := range standards {
			if rule.Standard == s {
				applicable = true
				break
			}
		}
		if !applicable {
			continue
		}

		// 更新分类统计
		if _, exists := categoryStats[rule.Category]; !exists {
			categoryStats[rule.Category] = &CategoryGap{
				Category: rule.Category,
			}
		}
		catStat := categoryStats[rule.Category]
		catStat.TotalChecks++

		// 模拟检查结果
		if rule.Category == CategoryDataProtection {
			catStat.Failed++
			catStat.GapCount++

			gaps = append(gaps, ComplianceGap{
				RuleID:      rule.ID,
				Standard:    rule.Standard,
				Severity:    rule.Severity,
				Title:       rule.Title,
				Description: rule.Description,
				Current:     "未完全合规",
				Required:    rule.Requirement,
				Impact:      "可能存在数据泄露风险",
			})

			actions = append(actions, RecommendedAction{
				ID:          uuid.New().String(),
				Priority:    rule.Severity,
				RuleID:      rule.ID,
				Title:       fmt.Sprintf("修复: %s", rule.Title),
				Description: rule.Remediation,
				Action:      "执行修复建议",
				Effort:      "medium",
				AutoFixable: false,
			})
		} else {
			catStat.Passed++
		}
	}

	// 计算分类分数
	totalPassed, totalChecks := 0, 0
	for _, catStat := range categoryStats {
		if catStat.TotalChecks > 0 {
			catStat.Score = float64(catStat.Passed) / float64(catStat.TotalChecks) * 100
		}
		totalPassed += catStat.Passed
		totalChecks += catStat.TotalChecks
		analysis.Categories = append(analysis.Categories, *catStat)
	}

	// 计算总体分数
	if totalChecks > 0 {
		analysis.Score = float64(totalPassed) / float64(totalChecks) * 100
	}

	analysis.Gaps = gaps
	analysis.Actions = actions

	m.gapAnalyses[analysisID] = analysis

	log.Printf("[合规引擎] 差距分析完成: %s, 分数: %.2f", analysisID, analysis.Score)
	return analysis, nil
}

// ============================================================
// 告警管理
// ============================================================

// GetAlert 获取告警
func (m *Manager) GetAlert(id string) (*ComplianceAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, exists := m.alerts[id]
	if !exists {
		return nil, fmt.Errorf("告警 %s 不存在", id)
	}
	return alert, nil
}

// ListAlerts 列出告警
func (m *Manager) ListAlerts(severity AlertSeverity, status string) []*ComplianceAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ComplianceAlert
	for _, alert := range m.alerts {
		if severity != "" && alert.Severity != severity {
			continue
		}
		if status != "" && alert.Status != status {
			continue
		}
		result = append(result, alert)
	}
	return result
}

// AcknowledgeAlert 确认告警
func (m *Manager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[id]
	if !exists {
		return fmt.Errorf("告警 %s 不存在", id)
	}

	alert.Status = "acknowledged"
	alert.UpdatedAt = time.Now()
	m.stats.ActiveAlerts--

	log.Printf("[合规引擎] 确认告警: %s", id)
	return nil
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[id]
	if !exists {
		return fmt.Errorf("告警 %s 不存在", id)
	}

	alert.Status = "resolved"
	alert.UpdatedAt = time.Now()
	m.stats.ActiveAlerts--

	log.Printf("[合规引擎] 解决告警: %s", id)
	return nil
}

// ============================================================
// 任务管理
// ============================================================

// CreateTask 创建修复任务
func (m *Manager) CreateTask(task RemediationTask) (*RemediationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	task.Status = TaskPending
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	m.tasks[task.ID] = &task
	m.stats.TotalTasks++
	m.stats.PendingTasks++

	log.Printf("[合规引擎] 创建任务: %s - %s", task.ID, task.Title)
	return &task, nil
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) (*RemediationTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", id)
	}
	return task, nil
}

// ListTasks 列出任务
func (m *Manager) ListTasks(status TaskStatus) []*RemediationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*RemediationTask
	for _, task := range m.tasks {
		if status != "" && task.Status != status {
			continue
		}
		result = append(result, task)
	}
	return result
}

// UpdateTaskStatus 更新任务状态
func (m *Manager) UpdateTaskStatus(id string, status TaskStatus, result string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", id)
	}

	task.Status = status
	task.UpdatedAt = time.Now()
	task.Result = result

	if status == TaskCompleted {
		task.CompletedAt = &task.UpdatedAt
		m.stats.CompletedTasks++
		m.stats.PendingTasks--
	}

	log.Printf("[合规引擎] 更新任务状态: %s -> %s", id, status)
	return nil
}

// ============================================================
// 统计查询
// ============================================================

// GetStats 获取统计信息
func (m *Manager) GetStats() *ComplianceStats {
	return m.stats.GetSnapshot()
}

// GetConfig 获取配置
func (m *Manager) GetConfig() EngineConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config EngineConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	log.Printf("[合规引擎] 更新配置")
}
