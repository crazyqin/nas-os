package audit

import (
	"fmt"
	"sort"
	"time"
)

// ComplianceReporter 合规报告生成器
type ComplianceReporter struct {
	manager *Manager
}

// NewComplianceReporter 创建合规报告生成器
func NewComplianceReporter(manager *Manager) *ComplianceReporter {
	return &ComplianceReporter{manager: manager}
}

// GenerateReport 生成合规报告
func (r *ComplianceReporter) GenerateReport(standard ComplianceStandard, startTime, endTime time.Time) (*ComplianceReport, error) {
	// 获取时间范围内的所有日志
	opts := QueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     100000, // 获取全部
	}

	result, err := r.manager.Query(opts)
	if err != nil {
		return nil, err
	}

	report := &ComplianceReport{
		ReportID:        fmt.Sprintf("RPT-%d", time.Now().UnixNano()),
		Standard:        standard,
		GeneratedAt:     time.Now(),
		PeriodStart:     startTime,
		PeriodEnd:       endTime,
		Findings:        make([]ComplianceFinding, 0),
		Recommendations: make([]string, 0),
	}

	// 计算摘要统计
	report.Summary = r.calculateSummary(result.Entries)

	// 根据合规标准生成发现
	switch standard {
	case ComplianceGDPR:
		r.analyzeGDPR(result.Entries, report)
	case ComplianceMLPS:
		r.analyzeMLPS(result.Entries, report)
	case ComplianceISO27001:
		r.analyzeISO27001(result.Entries, report)
	case ComplianceHIPAA:
		r.analyzeHIPAA(result.Entries, report)
	case CompliancePCI:
		r.analyzePCI(result.Entries, report)
	default:
		r.analyzeGeneric(result.Entries, report)
	}

	return report, nil
}

// calculateSummary 计算摘要统计
func (r *ComplianceReporter) calculateSummary(entries []*Entry) ComplianceSummary {
	summary := ComplianceSummary{
		TotalEvents:      len(entries),
		EventsByCategory: make(map[string]int),
		EventsByLevel:    make(map[string]int),
		EventsByHour:     make(map[int]int),
	}

	users := make(map[string]bool)
	ips := make(map[string]bool)

	for _, e := range entries {
		// 分类统计
		summary.EventsByCategory[string(e.Category)]++
		summary.EventsByLevel[string(e.Level)]++
		summary.EventsByHour[e.Timestamp.Hour()]++

		// 认证事件
		if e.Category == CategoryAuth {
			summary.AuthEvents++
			if e.Status == StatusFailure {
				summary.FailedAuthAttempts++
			}
		}

		// 数据访问事件
		if e.Category == CategoryData || e.Category == CategoryAccess {
			summary.DataAccessEvents++
		}

		// 配置变更
		if e.Category == CategorySystem && e.Event == "config_change" {
			summary.ConfigChanges++
		}

		// 安全告警
		if e.Category == CategorySecurity || e.Level == LevelCritical || e.Level == LevelError {
			summary.SecurityAlerts++
		}

		// 活跃用户和IP
		if e.UserID != "" {
			users[e.UserID] = true
		}
		if e.IP != "" {
			ips[e.IP] = true
		}
	}

	summary.UniqueUsers = len(users)
	summary.UniqueIPs = len(ips)

	return summary
}

// analyzeGDPR GDPR合规分析
func (r *ComplianceReporter) analyzeGDPR(entries []*Entry, report *ComplianceReport) {
	// GDPR 第32条：安全处理措施
	// 检查是否有未授权的数据访问

	var (
		unauthorizedAccess []*Entry
		dataBreaches       []*Entry
		failedAccess       []*Entry
	)

	for _, e := range entries {
		// 检查未授权访问
		if e.Category == CategoryAccess && e.Status == StatusFailure {
			failedAccess = append(failedAccess, e)
		}

		// 检查数据泄露风险
		if e.Category == CategorySecurity {
			if e.Event == "data_exposure" || e.Event == "unauthorized_access" {
				dataBreaches = append(dataBreaches, e)
			}
		}

		// 检查敏感数据访问
		if e.Category == CategoryData && e.Level == LevelWarning {
			unauthorizedAccess = append(unauthorizedAccess, e)
		}
	}

	// 添加发现
	if len(dataBreaches) > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "GDPR-001",
			Severity:    LevelCritical,
			Category:    CategorySecurity,
			Title:       "潜在数据泄露事件",
			Description: fmt.Sprintf("发现 %d 起潜在的数据泄露事件，需要进一步调查", len(dataBreaches)),
			Evidence:    dataBreaches,
		})
	}

	if len(failedAccess) > 10 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "GDPR-002",
			Severity:    LevelWarning,
			Category:    CategoryAccess,
			Title:       "频繁的访问失败",
			Description: fmt.Sprintf("发现 %d 次访问失败尝试，可能存在未授权访问尝试", len(failedAccess)),
			Evidence:    failedAccess[:min(10, len(failedAccess))],
		})
	}

	// 添加建议
	report.Recommendations = append(report.Recommendations,
		"建议启用数据访问的完整审计日志",
		"建议实施最小权限原则",
		"建议定期审查用户访问权限",
		"建议对敏感数据实施加密保护",
	)
}

// analyzeMLPS 等级保护合规分析
func (r *ComplianceReporter) analyzeMLPS(entries []*Entry, report *ComplianceReport) {
	// 等级保护要求检查

	var (
		loginFailures  []*Entry
		privilegeEsc   []*Entry
		configChanges  []*Entry
		auditTampering []*Entry
	)

	for _, e := range entries {
		// 登录失败检查
		if e.Category == CategoryAuth && e.Status == StatusFailure {
			loginFailures = append(loginFailures, e)
		}

		// 权限提升检查
		if e.Event == "privilege_escalation" || e.Event == "role_change" {
			privilegeEsc = append(privilegeEsc, e)
		}

		// 配置变更检查
		if e.Category == CategorySystem && e.Event == "config_change" {
			configChanges = append(configChanges, e)
		}

		// 审计日志篡改检查
		if e.Category == CategoryAudit {
			auditTampering = append(auditTampering, e)
		}
	}

	// 身份鉴别检查
	if len(loginFailures) > 5 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "MLPS-001",
			Severity:    LevelWarning,
			Category:    CategoryAuth,
			Title:       "身份鉴别失败次数较多",
			Description: fmt.Sprintf("统计期间内有 %d 次身份鉴别失败", len(loginFailures)),
			Evidence:    loginFailures[:min(10, len(loginFailures))],
		})
	}

	// 访问控制检查
	if len(privilegeEsc) > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "MLPS-002",
			Severity:    LevelWarning,
			Category:    CategoryAccess,
			Title:       "存在权限变更操作",
			Description: fmt.Sprintf("发现 %d 次权限变更操作，需要审查是否合规", len(privilegeEsc)),
			Evidence:    privilegeEsc,
		})
	}

	// 安全审计检查
	if report.Summary.ConfigChanges > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "MLPS-003",
			Severity:    LevelInfo,
			Category:    CategorySystem,
			Title:       "系统配置变更记录",
			Description: fmt.Sprintf("统计期间内有 %d 次配置变更", len(configChanges)),
			Evidence:    configChanges,
		})
	}

	// 添加等级保护建议
	report.Recommendations = append(report.Recommendations,
		"建议开启审计日志完整性保护功能",
		"建议配置登录失败处理策略（如账户锁定）",
		"建议实施双因素认证",
		"建议定期进行安全审计",
		"建议实施安全审计日志的定期备份",
	)
}

// analyzeISO27001 ISO 27001 合规分析
func (r *ComplianceReporter) analyzeISO27001(entries []*Entry, report *ComplianceReport) {
	var (
		securityIncidents []*Entry
		accessViolations  []*Entry
		systemChanges     []*Entry
	)

	for _, e := range entries {
		if e.Level == LevelCritical || e.Level == LevelError {
			securityIncidents = append(securityIncidents, e)
		}

		if e.Category == CategoryAccess && e.Status == StatusFailure {
			accessViolations = append(accessViolations, e)
		}

		if e.Category == CategorySystem {
			systemChanges = append(systemChanges, e)
		}
	}

	if len(securityIncidents) > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "ISO-001",
			Severity:    LevelCritical,
			Category:    CategorySecurity,
			Title:       "安全事件",
			Description: fmt.Sprintf("发现 %d 起安全事件", len(securityIncidents)),
			Evidence:    securityIncidents[:min(20, len(securityIncidents))],
		})
	}

	if len(accessViolations) > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "ISO-002",
			Severity:    LevelWarning,
			Category:    CategoryAccess,
			Title:       "访问控制违规",
			Description: fmt.Sprintf("发现 %d 次访问控制违规", len(accessViolations)),
			Evidence:    accessViolations[:min(10, len(accessViolations))],
		})
	}

	report.Recommendations = append(report.Recommendations,
		"建议建立信息安全事件响应流程",
		"建议定期进行风险评估",
		"建议实施访问控制策略",
		"建议建立变更管理流程",
	)
}

// analyzeHIPAA HIPAA 合规分析
func (r *ComplianceReporter) analyzeHIPAA(entries []*Entry, report *ComplianceReport) {
	var (
		phiAccess []*Entry
		breaches  []*Entry
	)

	for _, e := range entries {
		// PHI (Protected Health Information) 访问
		if e.Category == CategoryData && e.Details != nil {
			if _, ok := e.Details["data_type"]; ok {
				phiAccess = append(phiAccess, e)
			}
		}

		if e.Category == CategorySecurity && e.Level == LevelCritical {
			breaches = append(breaches, e)
		}
	}

	if len(breaches) > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "HIPAA-001",
			Severity:    LevelCritical,
			Category:    CategorySecurity,
			Title:       "潜在PHI泄露",
			Description: fmt.Sprintf("发现 %d 起潜在的安全事件", len(breaches)),
			Evidence:    breaches,
		})
	}

	report.Recommendations = append(report.Recommendations,
		"建议实施PHI访问控制",
		"建议启用PHI访问审计",
		"建议建立数据泄露响应流程",
		"建议对PHI数据实施加密",
	)
}

// analyzePCI PCI DSS 合规分析
func (r *ComplianceReporter) analyzePCI(entries []*Entry, report *ComplianceReport) {
	var (
		cardDataAccess []*Entry
		unauthorized   []*Entry
	)

	for _, e := range entries {
		if e.Category == CategoryData {
			if e.Details != nil {
				if dataType, ok := e.Details["data_type"]; ok {
					if dataType == "payment" || dataType == "card" {
						cardDataAccess = append(cardDataAccess, e)
					}
				}
			}
		}

		if e.Status == StatusFailure {
			unauthorized = append(unauthorized, e)
		}
	}

	if len(cardDataAccess) > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "PCI-001",
			Severity:    LevelInfo,
			Category:    CategoryData,
			Title:       "支付卡数据访问记录",
			Description: fmt.Sprintf("发现 %d 次支付卡数据访问", len(cardDataAccess)),
			Evidence:    cardDataAccess,
		})
	}

	report.Recommendations = append(report.Recommendations,
		"建议限制支付卡数据的存储",
		"建议实施支付卡数据加密",
		"建议定期进行PCI DSS合规审计",
		"建议实施网络分段隔离支付系统",
	)
}

// analyzeGeneric 通用合规分析
func (r *ComplianceReporter) analyzeGeneric(entries []*Entry, report *ComplianceReport) {
	// 通用安全分析

	var (
		criticalEvents []*Entry
		failedOps      []*Entry
	)

	for _, e := range entries {
		if e.Level == LevelCritical {
			criticalEvents = append(criticalEvents, e)
		}
		if e.Status == StatusFailure {
			failedOps = append(failedOps, e)
		}
	}

	if len(criticalEvents) > 0 {
		report.Findings = append(report.Findings, ComplianceFinding{
			ID:          "GEN-001",
			Severity:    LevelCritical,
			Category:    CategorySecurity,
			Title:       "严重安全事件",
			Description: fmt.Sprintf("发现 %d 起严重安全事件", len(criticalEvents)),
			Evidence:    criticalEvents,
		})
	}

	report.Recommendations = append(report.Recommendations,
		"建议定期审查审计日志",
		"建议实施安全事件监控",
		"建议建立应急响应流程",
	)
}

// GenerateDashboardData 生成仪表板数据
func (r *ComplianceReporter) GenerateDashboardData() map[string]interface{} {
	stats := r.manager.GetStatistics()

	// 获取最近24小时的数据
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	opts := QueryOptions{
		StartTime: &yesterday,
		EndTime:   &now,
		Limit:     10000,
	}

	result, _ := r.manager.Query(opts)

	// 统计最近24小时的事件
	hourlyEvents := make(map[int]int)
	authFailures24h := 0
	authSuccess24h := 0

	for _, e := range result.Entries {
		hourlyEvents[e.Timestamp.Hour()]++
		if e.Category == CategoryAuth {
			if e.Status == StatusFailure {
				authFailures24h++
			} else if e.Event == "login" {
				authSuccess24h++
			}
		}
	}

	return map[string]interface{}{
		"total_events":       stats.TotalEntries,
		"today_events":       stats.TodayEntries,
		"auth_failures_24h":  authFailures24h,
		"auth_success_24h":   authSuccess24h,
		"hourly_events":      hourlyEvents,
		"events_by_category": stats.EventsByCategory,
		"events_by_level":    stats.EventsByLevel,
		"top_users":          stats.TopUsers,
		"top_ips":            stats.TopIPs,
	}
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GenerateTimeline 生成事件时间线
func (r *ComplianceReporter) GenerateTimeline(startTime, endTime time.Time, category Category) []*TimelineEvent {
	opts := QueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
		Category:  category,
		Limit:     1000,
	}

	result, err := r.manager.Query(opts)
	if err != nil {
		return nil
	}

	// 按时间排序
	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Timestamp.Before(result.Entries[j].Timestamp)
	})

	timeline := make([]*TimelineEvent, 0, len(result.Entries))
	for _, e := range result.Entries {
		timeline = append(timeline, &TimelineEvent{
			Timestamp:   e.Timestamp,
			Event:       e.Event,
			User:        e.Username,
			IP:          e.IP,
			Resource:    e.Resource,
			Status:      string(e.Status),
			Severity:    string(e.Level),
			Description: e.Message,
		})
	}

	return timeline
}

// TimelineEvent 时间线事件
type TimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Event       string    `json:"event"`
	User        string    `json:"user,omitempty"`
	IP          string    `json:"ip,omitempty"`
	Resource    string    `json:"resource,omitempty"`
	Status      string    `json:"status"`
	Severity    string    `json:"severity"`
	Description string    `json:"description,omitempty"`
}
