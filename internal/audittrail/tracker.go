// Package audittrail 提供合规审计追踪功能
package audittrail

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 审计追踪管理器.
type Manager struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	records         map[string]*AuditRecord
	recordsByUser   map[string][]*AuditRecord
	recordsByIP     map[string][]*AuditRecord
	recordsByReq    map[string][]*AuditRecord
	recordsByAction map[ActionType][]*AuditRecord
	recordsByResult map[ActionResult][]*AuditRecord
	anomalies       map[string]*AnomalyDetection
	anomalyRules    map[string]*AnomalyRule
	reports         map[string]*ComplianceReport
	retentionPolicy RetentionPolicy
}

// NewManager 创建审计追踪管理器.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:          logger,
		records:         make(map[string]*AuditRecord),
		recordsByUser:   make(map[string][]*AuditRecord),
		recordsByIP:     make(map[string][]*AuditRecord),
		recordsByReq:    make(map[string][]*AuditRecord),
		recordsByAction: make(map[ActionType][]*AuditRecord),
		recordsByResult: make(map[ActionResult][]*AuditRecord),
		anomalies:       make(map[string]*AnomalyDetection),
		anomalyRules:    make(map[string]*AnomalyRule),
		reports:         make(map[string]*ComplianceReport),
		retentionPolicy: Retention7Years,
	}
}

// SetRetentionPolicy 设置保留策略.
func (m *Manager) SetRetentionPolicy(policy RetentionPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retentionPolicy = policy
	m.logger.Info("更新保留策略", zap.String("policy", string(policy)))
}

// RecordOperation 记录操作（不可变）.
func (m *Manager) RecordOperation(record *AuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	if record.ID == "" {
		record.ID = generateID()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	if record.RetentionPolicy == "" {
		record.RetentionPolicy = m.retentionPolicy
	}

	// 计算过期时间
	record.ExpiresAt = m.calculateExpiry(record.RetentionPolicy, record.Timestamp)

	// 计算校验和（确保不可篡改）
	record.Checksum = m.calculateChecksum(record)

	// 存储记录
	m.records[record.ID] = record

	// 更新索引
	m.recordsByUser[record.UserID] = append(m.recordsByUser[record.UserID], record)
	m.recordsByIP[record.UserIP] = append(m.recordsByIP[record.UserIP], record)
	if record.RequestID != "" {
		m.recordsByReq[record.RequestID] = append(m.recordsByReq[record.RequestID], record)
	}
	m.recordsByAction[record.Action] = append(m.recordsByAction[record.Action], record)
	m.recordsByResult[record.Result] = append(m.recordsByResult[record.Result], record)

	// 异常检测
	m.detectAnomalies(record)

	m.logger.Debug("审计记录已创建",
		zap.String("id", record.ID),
		zap.String("user", record.UserID),
		zap.String("action", string(record.Action)),
		zap.String("result", string(record.Result)),
	)

	return nil
}

// GetRecord 获取审计记录.
func (m *Manager) GetRecord(id string) (*AuditRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[id]
	if !exists {
		return nil, ErrRecordNotFound
	}
	return record, nil
}

// QueryRecords 查询审计记录.
func (m *Manager) QueryRecords(query AuditQuery) []*AuditRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*AuditRecord

	// 根据查询条件选择最优索引
	if query.UserID != "" {
		results = m.recordsByUser[query.UserID]
	} else if query.UserIP != "" {
		results = m.recordsByIP[query.UserIP]
	} else if query.RequestID != "" {
		results = m.recordsByReq[query.RequestID]
	} else if query.Action != "" {
		results = m.recordsByAction[query.Action]
	} else if query.Result != "" {
		results = m.recordsByResult[query.Result]
	} else {
		// 无过滤条件，返回所有记录
		results = make([]*AuditRecord, 0, len(m.records))
		for _, r := range m.records {
			results = append(results, r)
		}
	}

	// 应用额外过滤条件
	filtered := make([]*AuditRecord, 0)
	for _, r := range results {
		if !m.matchesQuery(r, query) {
			continue
		}
		filtered = append(filtered, r)
	}

	// 按时间排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// 应用分页
	if query.Offset > 0 && query.Offset < len(filtered) {
		filtered = filtered[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(filtered) {
		filtered = filtered[:query.Limit]
	}

	return filtered
}

// GetOperationChain 获取操作链.
func (m *Manager) GetOperationChain(requestID string) (*OperationChain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records, exists := m.recordsByReq[requestID]
	if !exists || len(records) == 0 {
		return nil, ErrRecordNotFound
	}

	// 按时间排序
	sorted := make([]*AuditRecord, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	chain := &OperationChain{
		RequestID: requestID,
		Records:   sorted,
		StartTime: sorted[0].Timestamp,
		EndTime:   sorted[len(sorted)-1].Timestamp,
		Duration:  sorted[len(sorted)-1].Timestamp.Sub(sorted[0].Timestamp),
	}

	// 确定最终结果
	for _, r := range sorted {
		chain.FinalResult = r.Result
	}

	return chain, nil
}

// AddAnomalyRule 添加异常检测规则.
func (m *Manager) AddAnomalyRule(rule *AnomalyRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}
	m.anomalyRules[rule.ID] = rule
	m.logger.Info("添加异常检测规则", zap.String("id", rule.ID), zap.String("name", rule.Name))
}

// UpdateAnomalyRule 更新异常检测规则.
func (m *Manager) UpdateAnomalyRule(rule *AnomalyRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.anomalyRules[rule.ID]; !exists {
		return ErrRuleNotFound
	}
	m.anomalyRules[rule.ID] = rule
	return nil
}

// DeleteAnomalyRule 删除异常检测规则.
func (m *Manager) DeleteAnomalyRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.anomalyRules[id]; !exists {
		return ErrRuleNotFound
	}
	delete(m.anomalyRules, id)
	return nil
}

// GetAnomalyRules 获取所有异常检测规则.
func (m *Manager) GetAnomalyRules() []*AnomalyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AnomalyRule, 0, len(m.anomalyRules))
	for _, r := range m.anomalyRules {
		rules = append(rules, r)
	}
	return rules
}

// GetAnomalies 获取异常检测结果.
func (m *Manager) GetAnomalies(resolved *bool) []*AnomalyDetection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	anomalies := make([]*AnomalyDetection, 0)
	for _, a := range m.anomalies {
		if resolved != nil && a.Resolved != *resolved {
			continue
		}
		anomalies = append(anomalies, a)
	}

	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].DetectedAt.After(anomalies[j].DetectedAt)
	})

	return anomalies
}

// ResolveAnomaly 解决异常.
func (m *Manager) ResolveAnomaly(id, resolvedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	anomaly, exists := m.anomalies[id]
	if !exists {
		return ErrRecordNotFound
	}

	now := time.Now()
	anomaly.Resolved = true
	anomaly.ResolvedAt = &now
	anomaly.ResolvedBy = resolvedBy

	return nil
}

// GenerateComplianceReport 生成合规报告.
func (m *Manager) GenerateComplianceReport(standard ComplianceStandard, start, end time.Time) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if start.After(end) {
		return nil, ErrInvalidTimeRange
	}

	report := &ComplianceReport{
		ID:          generateID(),
		Standard:    standard,
		GeneratedAt: time.Now(),
		PeriodStart: start,
		PeriodEnd:   end,
		Status:      "COMPLETED",
	}

	// 收集时间范围内的记录
	records := make([]*AuditRecord, 0)
	for _, r := range m.records {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			if standard == StandardAll || m.hasComplianceTag(r, standard) {
				records = append(records, r)
			}
		}
	}

	// 统计摘要
	uniqueUsers := make(map[string]bool)
	summary := ReportSummary{
		TotalRecords: len(records),
	}

	for _, r := range records {
		switch r.Result {
		case ResultSuccess:
			summary.SuccessCount++
		case ResultFailed:
			summary.FailedCount++
		case ResultDenied:
			summary.DeniedCount++
		}
		uniqueUsers[r.UserID] = true
	}
	summary.UniqueUsers = len(uniqueUsers)

	// 计算合规评分
	summary.ComplianceScore = m.calculateComplianceScore(records, standard)
	report.Summary = summary

	// 生成发现项
	report.Findings = m.generateFindings(records, standard)
	report.Recommendations = m.generateRecommendations(report.Findings)

	// 存储报告
	m.reports[report.ID] = report

	return report, nil
}

// GetComplianceReport 获取合规报告.
func (m *Manager) GetComplianceReport(id string) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, exists := m.reports[id]
	if !exists {
		return nil, ErrRecordNotFound
	}
	return report, nil
}

// GetComplianceStats 获取合规统计.
func (m *Manager) GetComplianceStats() *ComplianceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ComplianceStats{
		TotalRecords:      len(m.records),
		RecordsByStandard: make(map[ComplianceStandard]int),
		RecordsByAction:   make(map[ActionType]int),
		RecordsByResult:   make(map[ActionResult]int),
	}

	for _, r := range m.records {
		stats.RecordsByAction[r.Action]++
		stats.RecordsByResult[r.Result]++
		for _, tag := range r.ComplianceTags {
			stats.RecordsByStandard[tag]++
		}
	}

	for _, a := range m.anomalies {
		stats.AnomalyCount++
		if a.Resolved {
			stats.ResolvedAnomalies++
		}
	}

	return stats
}

// ExportRecords 导出记录.
func (m *Manager) ExportRecords(req ExportRequest) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.queryRecordsInternal(req.Query)

	switch req.Format {
	case FormatJSON:
		return m.exportJSON(records)
	case FormatCSV:
		return m.exportCSV(records)
	case FormatPDF:
		return m.exportPDF(records)
	default:
		return nil, ErrExportFailed
	}
}

// ========== 内部方法 ==========

// matchesQuery 检查记录是否匹配查询条件.
func (m *Manager) matchesQuery(record *AuditRecord, query AuditQuery) bool {
	if query.UserID != "" && record.UserID != query.UserID {
		return false
	}
	if query.UserName != "" && record.UserName != query.UserName {
		return false
	}
	if query.UserIP != "" && record.UserIP != query.UserIP {
		return false
	}
	if query.Action != "" && record.Action != query.Action {
		return false
	}
	if query.Resource != "" && record.Resource != query.Resource {
		return false
	}
	if query.ResourceType != "" && record.ResourceType != query.ResourceType {
		return false
	}
	if query.Result != "" && record.Result != query.Result {
		return false
	}
	if query.RequestID != "" && record.RequestID != query.RequestID {
		return false
	}
	if query.StartTime != nil && record.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && record.Timestamp.After(*query.EndTime) {
		return false
	}
	if query.ComplianceTag != "" {
		found := false
		for _, tag := range record.ComplianceTags {
			if tag == query.ComplianceTag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// queryRecordsInternal 内部查询（无锁）.
func (m *Manager) queryRecordsInternal(query AuditQuery) []*AuditRecord {
	results := make([]*AuditRecord, 0)
	for _, r := range m.records {
		if m.matchesQuery(r, query) {
			results = append(results, r)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}
	return results
}

// detectAnomalies 异常检测.
func (m *Manager) detectAnomalies(record *AuditRecord) {
	for _, rule := range m.anomalyRules {
		if !rule.Enabled {
			continue
		}
		if m.evaluateRule(rule, record) {
			anomaly := &AnomalyDetection{
				ID:             generateID(),
				RuleID:         rule.ID,
				RuleName:       rule.Name,
				Level:          rule.Level,
				DetectedAt:     time.Now(),
				RelatedRecords: []string{record.ID},
				Description:    fmt.Sprintf("规则 [%s] 触发: %s", rule.Name, rule.Description),
			}
			m.anomalies[anomaly.ID] = anomaly
			m.logger.Warn("检测到异常",
				zap.String("rule", rule.Name),
				zap.String("level", string(rule.Level)),
				zap.String("record_id", record.ID),
			)
		}
	}
}

// evaluateRule 评估规则.
func (m *Manager) evaluateRule(rule *AnomalyRule, record *AuditRecord) bool {
	for _, cond := range rule.Conditions {
		if !m.evaluateCondition(cond, record) {
			return false
		}
	}
	return true
}

// evaluateCondition 评估条件.
func (m *Manager) evaluateCondition(cond RuleCondition, record *AuditRecord) bool {
	var fieldValue interface{}

	switch cond.Field {
	case "action":
		fieldValue = string(record.Action)
	case "result":
		fieldValue = string(record.Result)
	case "user_id":
		fieldValue = record.UserID
	case "user_ip":
		fieldValue = record.UserIP
	case "resource":
		fieldValue = record.Resource
	case "resource_type":
		fieldValue = record.ResourceType
	default:
		return false
	}

	switch cond.Operator {
	case "eq":
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", cond.Value)
	case "neq":
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", cond.Value)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", cond.Value))
	case "in":
		if values, ok := cond.Value.([]interface{}); ok {
			for _, v := range values {
				if fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", v) {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// calculateChecksum 计算校验和.
func (m *Manager) calculateChecksum(record *AuditRecord) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
		record.ID,
		record.Timestamp.Format(time.RFC3339Nano),
		record.UserID,
		record.Action,
		record.Resource,
		record.Result,
		record.RequestID,
		record.RetentionPolicy,
		record.ExpiresAt.Format(time.RFC3339Nano),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// calculateExpiry 计算过期时间.
func (m *Manager) calculateExpiry(policy RetentionPolicy, baseTime time.Time) time.Time {
	switch policy {
	case Retention7Years:
		return baseTime.AddDate(7, 0, 0)
	case Retention10Years:
		return baseTime.AddDate(10, 0, 0)
	case RetentionPermanent:
		return time.Time{} // 零值表示永不过期
	default:
		return baseTime.AddDate(7, 0, 0)
	}
}

// hasComplianceTag 检查是否有合规标签.
func (m *Manager) hasComplianceTag(record *AuditRecord, standard ComplianceStandard) bool {
	for _, tag := range record.ComplianceTags {
		if tag == standard {
			return true
		}
	}
	return false
}

// calculateComplianceScore 计算合规评分.
func (m *Manager) calculateComplianceScore(records []*AuditRecord, standard ComplianceStandard) float64 {
	if len(records) == 0 {
		return 100.0
	}

	score := 100.0
	penalties := 0.0

	for _, r := range records {
		switch r.Result {
		case ResultFailed:
			penalties += 0.5
		case ResultDenied:
			penalties += 0.3
		case ResultError:
			penalties += 1.0
		}
	}

	score -= (penalties / float64(len(records))) * 100
	if score < 0 {
		score = 0
	}
	return score
}

// generateFindings 生成发现项.
func (m *Manager) generateFindings(records []*AuditRecord, standard ComplianceStandard) []ComplianceFinding {
	findings := make([]ComplianceFinding, 0)

	// 统计失败操作
	failedRecords := make([]string, 0)
	for _, r := range records {
		if r.Result == ResultFailed || r.Result == ResultError {
			failedRecords = append(failedRecords, r.ID)
		}
	}

	if len(failedRecords) > 0 {
		findings = append(findings, ComplianceFinding{
			Category:        "ACCESS_CONTROL",
			Description:     fmt.Sprintf("发现 %d 次失败操作", len(failedRecords)),
			Severity:        "MEDIUM",
			Evidence:        failedRecords,
			Recommendation:  "审查访问控制策略，确保权限配置正确",
		})
	}

	// 检查敏感操作
	sensitiveOps := make([]string, 0)
	for _, r := range records {
		if r.Action == ActionDelete || r.Action == ActionConfig {
			sensitiveOps = append(sensitiveOps, r.ID)
		}
	}

	if len(sensitiveOps) > 0 {
		findings = append(findings, ComplianceFinding{
			Category:        "CHANGE_MANAGEMENT",
			Description:     fmt.Sprintf("发现 %d 次敏感操作", len(sensitiveOps)),
			Severity:        "HIGH",
			Evidence:        sensitiveOps,
			Recommendation:  "确保敏感操作有审批流程",
		})
	}

	return findings
}

// generateRecommendations 生成建议.
func (m *Manager) generateRecommendations(findings []ComplianceFinding) []string {
	recommendations := make([]string, 0)
	seen := make(map[string]bool)

	for _, f := range findings {
		if !seen[f.Recommendation] {
			recommendations = append(recommendations, f.Recommendation)
			seen[f.Recommendation] = true
		}
	}

	// 添加通用建议
	generalRecs := []string{
		"定期审查审计日志，确保所有操作可追溯",
		"实施最小权限原则，减少不必要的访问",
		"建立异常响应流程，及时处理安全事件",
	}

	for _, rec := range generalRecs {
		if !seen[rec] {
			recommendations = append(recommendations, rec)
			seen[rec] = true
		}
	}

	return recommendations
}

// exportJSON 导出JSON格式.
func (m *Manager) exportJSON(records []*AuditRecord) ([]byte, error) {
	return json.MarshalIndent(records, "", "  ")
}

// exportCSV 导出CSV格式.
func (m *Manager) exportCSV(records []*AuditRecord) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("ID,Timestamp,UserID,UserName,UserIP,Action,Resource,ResourceType,Result,RequestID,RetentionPolicy\n")

	for _, r := range records {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			r.ID,
			r.Timestamp.Format(time.RFC3339),
			r.UserID,
			r.UserName,
			r.UserIP,
			r.Action,
			r.Resource,
			r.ResourceType,
			r.Result,
			r.RequestID,
			r.RetentionPolicy,
		))
	}

	return []byte(sb.String()), nil
}

// exportPDF 导出PDF格式（简化实现）.
func (m *Manager) exportPDF(records []*AuditRecord) ([]byte, error) {
	// 简化实现：返回文本格式的报告
	var sb strings.Builder
	sb.WriteString("=== 审计报告 ===\n\n")
	sb.WriteString(fmt.Sprintf("生成时间: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("记录数量: %d\n\n", len(records)))

	for _, r := range records {
		sb.WriteString(fmt.Sprintf("ID: %s\n", r.ID))
		sb.WriteString(fmt.Sprintf("时间: %s\n", r.Timestamp.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("用户: %s (%s)\n", r.UserName, r.UserID))
		sb.WriteString(fmt.Sprintf("操作: %s %s\n", r.Action, r.Resource))
		sb.WriteString(fmt.Sprintf("结果: %s\n", r.Result))
		sb.WriteString("---\n")
	}

	return []byte(sb.String()), nil
}

// generateID 生成唯一ID.
func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(8))
}

// randomHex 生成随机十六进制字符串.
func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1) // 确保不同值
	}
	return string(b)
}
