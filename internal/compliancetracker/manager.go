// Package compliancetracker 提供合规审计追踪功能
package compliancetracker

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager 合规管理器.
type Manager struct {
	mu       sync.RWMutex
	config   *ComplianceConfig
	rules    map[string]*ComplianceRule
	checks   []*ComplianceCheck
	auditLog []*AuditLog
}

// NewManager 创建合规管理器.
func NewManager(config *ComplianceConfig) *Manager {
	if config == nil {
		config = DefaultComplianceConfig()
	}
	return &Manager{
		config:   config,
		rules:    make(map[string]*ComplianceRule),
		checks:   make([]*ComplianceCheck, 0),
		auditLog: make([]*AuditLog, 0),
	}
}

// ========== 规则管理 ==========

// AddRule 添加合规规则.
func (m *Manager) AddRule(rule *ComplianceRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证规则
	if rule.Name == "" {
		return ErrInvalidRule
	}

	// 检查重复
	if _, exists := m.rules[rule.ID]; exists {
		return ErrDuplicateRule
	}

	// 设置默认值
	if rule.ID == "" {
		rule.ID = generateID()
	}
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	if rule.Severity == "" {
		rule.Severity = SeverityMedium
	}
	if rule.RuleType == "" {
		rule.RuleType = RuleTypeCustom
	}

	m.rules[rule.ID] = rule

	// 记录审计日志
	m.addAuditLog("add_rule", "system", rule.ID, "添加合规规则: "+rule.Name, "success")

	return nil
}

// UpdateRule 更新合规规则.
func (m *Manager) UpdateRule(rule *ComplianceRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		return ErrInvalidRule
	}

	existing, exists := m.rules[rule.ID]
	if !exists {
		return ErrRuleNotFound
	}

	// 保留创建时间
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule

	// 记录审计日志
	m.addAuditLog("update_rule", "system", rule.ID, "更新合规规则: "+rule.Name, "success")

	return nil
}

// DeleteRule 删除合规规则.
func (m *Manager) DeleteRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return ErrRuleNotFound
	}

	delete(m.rules, ruleID)

	// 记录审计日志
	m.addAuditLog("delete_rule", "system", ruleID, "删除合规规则: "+rule.Name, "success")

	return nil
}

// GetRule 获取合规规则.
func (m *Manager) GetRule(ruleID string) (*ComplianceRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}

	return rule, nil
}

// ListRules 列出所有合规规则.
func (m *Manager) ListRules(ruleType RuleType, enabled *bool) []*ComplianceRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []*ComplianceRule
	for _, rule := range m.rules {
		if ruleType != "" && rule.RuleType != ruleType {
			continue
		}
		if enabled != nil && rule.Enabled != *enabled {
			continue
		}
		rules = append(rules, rule)
	}

	// 按名称排序
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Name < rules[j].Name
	})

	return rules
}

// ========== 合规检查 ==========

// RunCheck 执行合规检查.
func (m *Manager) RunCheck(ruleID string, target string, targetType string, checkedBy string) (*ComplianceCheck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}

	if !rule.Enabled {
		return nil, ErrInvalidRule
	}

	startTime := time.Now()
	check := &ComplianceCheck{
		ID:         generateID(),
		RuleID:     ruleID,
		RuleName:   rule.Name,
		Timestamp:  startTime,
		Target:     target,
		TargetType: targetType,
		CheckedBy:  checkedBy,
	}

	// 执行检查逻辑
	violations := m.evaluateRule(rule, target)
	check.CheckDuration = time.Since(startTime).Milliseconds()

	if len(violations) == 0 {
		check.Status = StatusCompliant
	} else {
		check.Status = StatusNonCompliant
		check.Violations = violations
		check.Details = "发现 " + itoa(len(violations)) + " 个违规项"
	}

	m.checks = append(m.checks, check)

	// 记录审计日志
	m.addAuditLog("run_check", checkedBy, check.ID,
		"执行合规检查: "+rule.Name+" -> "+string(check.Status), "success")

	return check, nil
}

// RunAllChecks 执行所有启用规则的检查.
func (m *Manager) RunAllChecks(target string, targetType string, checkedBy string) ([]*ComplianceCheck, error) {
	m.mu.Lock()
	ruleIDs := make([]string, 0)
	for id, rule := range m.rules {
		if rule.Enabled {
			ruleIDs = append(ruleIDs, id)
		}
	}
	m.mu.Unlock()

	var results []*ComplianceCheck
	for _, ruleID := range ruleIDs {
		check, err := m.RunCheck(ruleID, target, targetType, checkedBy)
		if err != nil {
			continue
		}
		results = append(results, check)
	}

	return results, nil
}

// evaluateRule 评估规则条件.
func (m *Manager) evaluateRule(rule *ComplianceRule, target string) []Violation {
	var violations []Violation

	for _, condition := range rule.Conditions {
		actualValue := m.getFieldValue(target, condition.Field)
		if !m.evaluateCondition(condition, actualValue) {
			violations = append(violations, Violation{
				Field:    condition.Field,
				Expected: condition.Operator + " " + condition.Value,
				Actual:   actualValue,
				Severity: rule.Severity,
				Message:  "字段 " + condition.Field + " 不满足条件",
			})
		}
	}

	return violations
}

// evaluateCondition 评估单个条件.
func (m *Manager) evaluateCondition(condition Condition, actual string) bool {
	switch condition.Operator {
	case "eq":
		return actual == condition.Value
	case "ne":
		return actual != condition.Value
	case "gt":
		return compareNumeric(actual, condition.Value) > 0
	case "lt":
		return compareNumeric(actual, condition.Value) < 0
	case "gte":
		return compareNumeric(actual, condition.Value) >= 0
	case "lte":
		return compareNumeric(actual, condition.Value) <= 0
	case "contains":
		return strings.Contains(actual, condition.Value)
	case "regex":
		matched, _ := regexp.MatchString(condition.Value, actual)
		return matched
	default:
		return false
	}
}

// getFieldValue 获取字段值（简化实现）.
func (m *Manager) getFieldValue(target string, field string) string {
	// 简化实现，实际应根据 target 类型解析
	return target
}

// ========== 查询功能 ==========

// QueryChecks 查询合规检查记录.
func (m *Manager) QueryChecks(filter QueryFilter) []*ComplianceCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*ComplianceCheck

	for _, check := range m.checks {
		if !m.matchesCheckFilter(check, filter) {
			continue
		}
		results = append(results, check)
	}

	// 按时间降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// 分页
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results
}

// matchesCheckFilter 检查是否匹配过滤条件.
func (m *Manager) matchesCheckFilter(check *ComplianceCheck, filter QueryFilter) bool {
	if filter.StartTime != nil && check.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && check.Timestamp.After(*filter.EndTime) {
		return false
	}
	if filter.RuleID != "" && check.RuleID != filter.RuleID {
		return false
	}
	if filter.Status != "" && check.Status != filter.Status {
		return false
	}
	if filter.Target != "" && !strings.Contains(check.Target, filter.Target) {
		return false
	}
	if filter.TargetType != "" && check.TargetType != filter.TargetType {
		return false
	}
	return true
}

// ========== 报告生成 ==========

// GenerateReport 生成合规报告.
func (m *Manager) GenerateReport(startTime, endTime time.Time) (*ComplianceReport, error) {
	if startTime.After(endTime) {
		return nil, ErrInvalidTimeRange
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &ComplianceReport{
		ID:                 generateID(),
		GeneratedAt:        time.Now(),
		StartTime:          startTime,
		EndTime:            endTime,
		StatusDistribution: make(map[ComplianceStatus]int),
		Recommendations:    make([]string, 0),
	}

	// 统计检查结果
	ruleStats := make(map[string]*RuleSummary)
	violationStats := make(map[string]*ViolationSummary)

	for _, check := range m.checks {
		if check.Timestamp.Before(startTime) || check.Timestamp.After(endTime) {
			continue
		}

		report.TotalChecks++

		// 状态统计
		report.StatusDistribution[check.Status]++
		switch check.Status {
		case StatusCompliant:
			report.CompliantCount++
		case StatusNonCompliant:
			report.NonCompliantCount++
		case StatusPartial:
			report.PartialCount++
		case StatusPending:
			report.PendingCount++
		case StatusError:
			report.ErrorCount++
		}

		// 规则统计
		if _, exists := ruleStats[check.RuleID]; !exists {
			ruleStats[check.RuleID] = &RuleSummary{
				RuleID:   check.RuleID,
				RuleName: check.RuleName,
			}
		}
		rs := ruleStats[check.RuleID]
		rs.TotalChecks++
		if check.Status == StatusCompliant {
			rs.CompliantCount++
		}
		if check.Timestamp.After(rs.LastCheckTime) {
			rs.LastCheckTime = check.Timestamp
		}

		// 违规统计
		for _, v := range check.Violations {
			key := check.RuleName + ":" + v.Field
			if _, exists := violationStats[key]; !exists {
				violationStats[key] = &ViolationSummary{
					RuleName: check.RuleName,
					Field:    v.Field,
					Severity: v.Severity,
				}
			}
			vs := violationStats[key]
			vs.Count++
			if check.Timestamp.After(vs.LastSeen) {
				vs.LastSeen = check.Timestamp
			}
		}
	}

	// 计算合规率
	if report.TotalChecks > 0 {
		report.ComplianceRate = float64(report.CompliantCount) / float64(report.TotalChecks) * 100
	}

	// 转换规则摘要
	for _, rs := range ruleStats {
		if rs.TotalChecks > 0 {
			rs.ComplianceRate = float64(rs.CompliantCount) / float64(rs.TotalChecks) * 100
		}
		if rs.ComplianceRate >= 100 {
			rs.Status = StatusCompliant
		} else if rs.ComplianceRate >= 80 {
			rs.Status = StatusPartial
		} else {
			rs.Status = StatusNonCompliant
		}
		report.RuleSummary = append(report.RuleSummary, *rs)
	}

	// 转换违规摘要
	for _, vs := range violationStats {
		report.TopViolations = append(report.TopViolations, *vs)
	}
	sort.Slice(report.TopViolations, func(i, j int) bool {
		return report.TopViolations[i].Count > report.TopViolations[j].Count
	})
	if len(report.TopViolations) > 10 {
		report.TopViolations = report.TopViolations[:10]
	}

	// 生成建议
	report.Recommendations = m.generateRecommendations(report)

	return report, nil
}

// generateRecommendations 生成建议.
func (m *Manager) generateRecommendations(report *ComplianceReport) []string {
	var recommendations []string

	if report.ComplianceRate < 80 {
		recommendations = append(recommendations, "合规率低于80%，建议优先处理高严重度违规项")
	}

	if report.NonCompliantCount > 0 {
		recommendations = append(recommendations, "存在不合规项，建议立即修复")
	}

	if report.ErrorCount > 0 {
		recommendations = append(recommendations, "存在检查错误，建议检查系统配置")
	}

	for _, v := range report.TopViolations {
		if v.Severity == SeverityCritical {
			recommendations = append(recommendations,
				"严重违规: "+v.RuleName+" - "+v.Field+"，发生 "+itoa(v.Count)+" 次")
		}
	}

	return recommendations
}

// ========== 审计日志 ==========

// GetAuditLogs 获取审计日志.
func (m *Manager) GetAuditLogs(filter QueryFilter) []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*AuditLog

	for _, log := range m.auditLog {
		if !m.matchesLogFilter(log, filter) {
			continue
		}
		results = append(results, log)
	}

	// 按时间降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// 分页
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results
}

// matchesLogFilter 检查日志是否匹配过滤条件.
func (m *Manager) matchesLogFilter(log *AuditLog, filter QueryFilter) bool {
	if filter.StartTime != nil && log.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && log.Timestamp.After(*filter.EndTime) {
		return false
	}
	if filter.Target != "" && !strings.Contains(log.Target, filter.Target) {
		return false
	}
	return true
}

// addAuditLog 添加审计日志（内部方法，需要已加锁）.
func (m *Manager) addAuditLog(action, actor, target, details, status string) {
	log := &AuditLog{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    action,
		Actor:     actor,
		Target:    target,
		Details:   details,
		Status:    status,
	}
	m.auditLog = append(m.auditLog, log)
}

// ========== 统计功能 ==========

// GetComplianceStats 获取合规统计.
func (m *Manager) GetComplianceStats(startTime, endTime time.Time) (map[string]interface{}, error) {
	if startTime.After(endTime) {
		return nil, ErrInvalidTimeRange
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	totalChecks := 0
	compliantCount := 0
	nonCompliantCount := 0
	ruleCount := len(m.rules)
	enabledRuleCount := 0

	for _, rule := range m.rules {
		if rule.Enabled {
			enabledRuleCount++
		}
	}

	for _, check := range m.checks {
		if check.Timestamp.Before(startTime) || check.Timestamp.After(endTime) {
			continue
		}
		totalChecks++
		if check.Status == StatusCompliant {
			compliantCount++
		} else if check.Status == StatusNonCompliant {
			nonCompliantCount++
		}
	}

	complianceRate := 0.0
	if totalChecks > 0 {
		complianceRate = float64(compliantCount) / float64(totalChecks) * 100
	}

	stats := map[string]interface{}{
		"total_checks":        totalChecks,
		"compliant_count":     compliantCount,
		"non_compliant_count": nonCompliantCount,
		"compliance_rate":     complianceRate,
		"rule_count":          ruleCount,
		"enabled_rule_count":  enabledRuleCount,
		"audit_log_count":     len(m.auditLog),
	}

	return stats, nil
}

// ========== 辅助函数 ==========

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func compareNumeric(a, b string) int {
	aFloat, err1 := strconv.ParseFloat(a, 64)
	bFloat, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil {
		return strings.Compare(a, b)
	}
	if aFloat < bFloat {
		return -1
	} else if aFloat > bFloat {
		return 1
	}
	return 0
}
