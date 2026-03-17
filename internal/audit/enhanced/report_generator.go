package enhanced

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReportGenerator 审计报告生成器
type ReportGenerator struct {
	loginAuditor     *LoginAuditor
	operationAuditor *OperationAuditor
	sensitiveManager *SensitiveOperationManager
	storageDir       string
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(
	loginAuditor *LoginAuditor,
	operationAuditor *OperationAuditor,
	sensitiveManager *SensitiveOperationManager,
) *ReportGenerator {
	storageDir := "/var/log/nas-os/audit/reports"
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		// 创建目录失败时使用当前目录
		storageDir = "."
	}

	return &ReportGenerator{
		loginAuditor:     loginAuditor,
		operationAuditor: operationAuditor,
		sensitiveManager: sensitiveManager,
		storageDir:       storageDir,
	}
}

// GenerateReport 生成审计报告
func (rg *ReportGenerator) GenerateReport(opts ReportGenerateOptions) (*AuditReport, error) {
	report := &AuditReport{
		ReportID:        fmt.Sprintf("RPT-%s", time.Now().Format("20060102150405")),
		ReportType:      opts.ReportType,
		GeneratedAt:     time.Now(),
		PeriodStart:     opts.PeriodStart,
		PeriodEnd:       opts.PeriodEnd,
		Title:           rg.getReportTitle(opts.ReportType),
		Description:     rg.getReportDescription(opts.ReportType),
		Recommendations: make([]string, 0),
		ChartData:       make(map[string]interface{}),
	}

	// 根据报告类型生成不同内容
	switch opts.ReportType {
	case ReportTypeLogin:
		report.LoginAnalysis = rg.generateLoginAnalysis(opts.PeriodStart, opts.PeriodEnd)
		report.Summary = rg.generateLoginSummary(report.LoginAnalysis)

	case ReportTypeOperation:
		report.OperationStats = rg.generateOperationStatistics(opts.PeriodStart, opts.PeriodEnd)
		report.Summary = rg.generateOperationSummary(report.OperationStats)

	case ReportTypeSensitive:
		report.SensitiveOps = rg.generateSensitiveSummary(opts.PeriodStart, opts.PeriodEnd)
		report.Summary = rg.generateSensitiveReportSummary(report.SensitiveOps)

	case ReportTypeSecurity:
		report.RiskAnalysis = rg.generateRiskAnalysis(opts.PeriodStart, opts.PeriodEnd)
		report.LoginAnalysis = rg.generateLoginAnalysis(opts.PeriodStart, opts.PeriodEnd)
		report.SensitiveOps = rg.generateSensitiveSummary(opts.PeriodStart, opts.PeriodEnd)
		report.Summary = rg.generateSecuritySummary(report)

	case ReportTypeUserActivity:
		report.Summary = rg.generateUserActivitySummary(opts.PeriodStart, opts.PeriodEnd, opts.UserIDs)

	case ReportTypeRiskAnalysis:
		report.RiskAnalysis = rg.generateRiskAnalysis(opts.PeriodStart, opts.PeriodEnd)
		report.Summary = rg.generateRiskSummary(report.RiskAnalysis)

	case ReportTypeExecutive:
		// 执行摘要报告包含所有关键信息
		report.LoginAnalysis = rg.generateLoginAnalysis(opts.PeriodStart, opts.PeriodEnd)
		report.OperationStats = rg.generateOperationStatistics(opts.PeriodStart, opts.PeriodEnd)
		report.SensitiveOps = rg.generateSensitiveSummary(opts.PeriodStart, opts.PeriodEnd)
		report.RiskAnalysis = rg.generateRiskAnalysis(opts.PeriodStart, opts.PeriodEnd)
		report.Summary = rg.generateExecutiveSummary(report)
	}

	// 生成建议
	report.Recommendations = rg.generateRecommendations(report)

	// 生成图表数据
	report.ChartData = rg.generateChartData(report)

	return report, nil
}

// ========== 各类报告生成 ==========

// generateLoginAnalysis 生成登录分析
func (rg *ReportGenerator) generateLoginAnalysis(start, end time.Time) *LoginAnalysis {
	if rg.loginAuditor == nil {
		return nil
	}
	return rg.loginAuditor.GetLoginStatistics(start, end)
}

// generateOperationStatistics 生成操作统计
func (rg *ReportGenerator) generateOperationStatistics(start, end time.Time) *OperationStatistics {
	if rg.operationAuditor == nil {
		return nil
	}
	return rg.operationAuditor.GetStatistics(start, end)
}

// generateSensitiveSummary 生成敏感操作摘要
func (rg *ReportGenerator) generateSensitiveSummary(start, end time.Time) *SensitiveOpsSummary {
	if rg.sensitiveManager == nil {
		return nil
	}
	return rg.sensitiveManager.GetSummary(start, end)
}

// generateRiskAnalysis 生成风险分析
func (rg *ReportGenerator) generateRiskAnalysis(start, end time.Time) *RiskAnalysis {
	analysis := &RiskAnalysis{
		RiskLevel:         "low",
		HighRiskEvents:    0,
		HighRiskUsers:     make([]UserRisk, 0),
		HighRiskIPs:       make([]IPRisk, 0),
		RiskTrends:        make([]RiskTrend, 0),
		ThreatIndicators:  make([]ThreatIndicator, 0),
		MitigationActions: make([]MitigationAction, 0),
	}

	// 从登录审计获取风险数据
	if rg.loginAuditor != nil {
		highRiskLogins := rg.loginAuditor.GetHighRiskLogins(70, 100)

		// 统计高风险用户和IP
		userRiskScores := make(map[string]int)
		userRiskFactors := make(map[string][]string)
		ipRiskScores := make(map[string]int)
		ipRiskFactors := make(map[string][]string)
		ipEventCounts := make(map[string]int)

		for _, login := range highRiskLogins {
			analysis.HighRiskEvents++

			userRiskScores[login.UserID] += login.RiskScore
			if len(login.RiskFactors) > 0 {
				userRiskFactors[login.UserID] = append(userRiskFactors[login.UserID], login.RiskFactors...)
			}

			ipRiskScores[login.IP] += login.RiskScore
			if len(login.RiskFactors) > 0 {
				ipRiskFactors[login.IP] = append(ipRiskFactors[login.IP], login.RiskFactors...)
			}
			ipEventCounts[login.IP]++
		}

		// 生成高风险用户列表
		for userID, score := range userRiskScores {
			if score >= 100 {
				analysis.HighRiskUsers = append(analysis.HighRiskUsers, UserRisk{
					UserID:      userID,
					RiskScore:   score,
					RiskFactors: uniqueStrings(userRiskFactors[userID]),
				})
			}
		}
		sort.Slice(analysis.HighRiskUsers, func(i, j int) bool {
			return analysis.HighRiskUsers[i].RiskScore > analysis.HighRiskUsers[j].RiskScore
		})
		if len(analysis.HighRiskUsers) > 10 {
			analysis.HighRiskUsers = analysis.HighRiskUsers[:10]
		}

		// 生成高风险IP列表
		for ip, score := range ipRiskScores {
			if score >= 100 {
				analysis.HighRiskIPs = append(analysis.HighRiskIPs, IPRisk{
					IP:          ip,
					RiskScore:   score,
					RiskFactors: uniqueStrings(ipRiskFactors[ip]),
					EventCount:  ipEventCounts[ip],
				})
			}
		}
		sort.Slice(analysis.HighRiskIPs, func(i, j int) bool {
			return analysis.HighRiskIPs[i].RiskScore > analysis.HighRiskIPs[j].RiskScore
		})
		if len(analysis.HighRiskIPs) > 10 {
			analysis.HighRiskIPs = analysis.HighRiskIPs[:10]
		}

		// 检测威胁指标
		analysis.ThreatIndicators = rg.detectThreatIndicators(start, end)
	}

	// 计算整体风险分数
	analysis.OverallRiskScore = rg.calculateOverallRiskScore(analysis)

	// 确定风险等级
	if analysis.OverallRiskScore >= 80 {
		analysis.RiskLevel = "critical"
	} else if analysis.OverallRiskScore >= 60 {
		analysis.RiskLevel = "high"
	} else if analysis.OverallRiskScore >= 40 {
		analysis.RiskLevel = "medium"
	}

	// 生成缓解措施
	analysis.MitigationActions = rg.generateMitigationActions(analysis)

	return analysis
}

// detectThreatIndicators 检测威胁指标
func (rg *ReportGenerator) detectThreatIndicators(start, end time.Time) []ThreatIndicator {
	indicators := make([]ThreatIndicator, 0)

	if rg.loginAuditor == nil {
		return indicators
	}

	// 检测暴力破解
	entries, _ := rg.loginAuditor.Query(LoginQueryOptions{
		StartTime: &start,
		EndTime:   &end,
		EventType: LoginEventFailure,
		Limit:     10000,
	})

	ipFailures := make(map[string]int)
	for _, entry := range entries {
		ipFailures[entry.IP]++
	}

	bruteForceIPs := make([]string, 0)
	for ip, count := range ipFailures {
		if count >= 10 { // 同一IP失败10次以上
			bruteForceIPs = append(bruteForceIPs, ip)
		}
	}

	if len(bruteForceIPs) > 0 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "brute_force",
			Description: "检测到潜在的暴力破解攻击",
			Severity:    "high",
			Count:       len(bruteForceIPs),
			Sources:     bruteForceIPs,
		})
	}

	// 检测异地登录
	highRiskLogins := rg.loginAuditor.GetHighRiskLogins(60, 1000)
	unusualLocationCount := 0
	for _, login := range highRiskLogins {
		for _, factor := range login.RiskFactors {
			if factor == "unusual_location" {
				unusualLocationCount++
				break
			}
		}
	}

	if unusualLocationCount > 0 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "unusual_location",
			Description: "检测到异常地理位置登录",
			Severity:    "medium",
			Count:       unusualLocationCount,
			Sources:     []string{},
		})
	}

	return indicators
}

// calculateOverallRiskScore 计算整体风险分数
func (rg *ReportGenerator) calculateOverallRiskScore(analysis *RiskAnalysis) int {
	score := 0

	// 高风险事件贡献
	score += analysis.HighRiskEvents * 2
	if score > 50 {
		score = 50
	}

	// 高风险用户贡献
	score += len(analysis.HighRiskUsers) * 5
	if score > 70 {
		score = 70
	}

	// 高风险IP贡献
	score += len(analysis.HighRiskIPs) * 3
	if score > 80 {
		score = 80
	}

	// 威胁指标贡献
	for _, indicator := range analysis.ThreatIndicators {
		switch indicator.Severity {
		case "critical":
			score += 20
		case "high":
			score += 10
		case "medium":
			score += 5
		}
	}

	if score > 100 {
		score = 100
	}

	return score
}

// generateMitigationActions 生成缓解措施
func (rg *ReportGenerator) generateMitigationActions(analysis *RiskAnalysis) []MitigationAction {
	actions := make([]MitigationAction, 0)

	// 根据风险级别生成建议
	if analysis.OverallRiskScore >= 60 {
		actions = append(actions, MitigationAction{
			Priority:    1,
			Action:      "review_high_risk_events",
			Description: "立即审查高风险安全事件",
			Status:      "pending",
		})
	}

	if len(analysis.HighRiskIPs) > 0 {
		actions = append(actions, MitigationAction{
			Priority:    2,
			Action:      "block_suspicious_ips",
			Description: "考虑封禁可疑IP地址",
			Status:      "pending",
		})
	}

	if len(analysis.HighRiskUsers) > 0 {
		actions = append(actions, MitigationAction{
			Priority:    3,
			Action:      "review_user_accounts",
			Description: "审查高风险用户账户活动",
			Status:      "pending",
		})
	}

	for _, indicator := range analysis.ThreatIndicators {
		if indicator.Type == "brute_force" {
			actions = append(actions, MitigationAction{
				Priority:    1,
				Action:      "enable_rate_limiting",
				Description: "启用登录速率限制防止暴力破解",
				Status:      "pending",
			})
		}
	}

	// 按优先级排序
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority < actions[j].Priority
	})

	return actions
}

// ========== 摘要生成 ==========

// generateLoginSummary 生成登录摘要
func (rg *ReportGenerator) generateLoginSummary(analysis *LoginAnalysis) *ReportSummary {
	if analysis == nil {
		return nil
	}

	summary := &ReportSummary{
		TotalEvents:   analysis.TotalLogins,
		UniqueUsers:   analysis.UniqueUsers,
		UniqueIPs:     analysis.UniqueIPs,
		SuccessfulOps: analysis.SuccessfulLogins,
		FailedOps:     analysis.FailedLogins,
		EventsByCategory: map[string]int{
			"login": analysis.TotalLogins,
		},
		TopUsers: make([]UserActivitySummary, 0),
		TopOperations: []OperationCount{
			{Operation: "successful_login", Count: analysis.SuccessfulLogins},
			{Operation: "failed_login", Count: analysis.FailedLogins},
		},
	}

	if len(analysis.AnomalousLogins) > 0 {
		summary.HighRiskEvents = len(analysis.AnomalousLogins)
	}

	return summary
}

// generateOperationSummary 生成操作摘要
func (rg *ReportGenerator) generateOperationSummary(stats *OperationStatistics) *ReportSummary {
	if stats == nil {
		return nil
	}

	summary := &ReportSummary{
		TotalEvents:      stats.TotalOperations,
		SuccessfulOps:    stats.SuccessfulOps,
		FailedOps:        stats.FailedOps,
		SensitiveOps:     stats.SensitiveOpCount,
		EventsByCategory: stats.OpsByCategory,
		TopOperations:    make([]OperationCount, 0),
	}

	// 转换操作统计
	for action, count := range stats.OpsByAction {
		summary.TopOperations = append(summary.TopOperations, OperationCount{
			Operation: action,
			Count:     count,
		})
	}
	sort.Slice(summary.TopOperations, func(i, j int) bool {
		return summary.TopOperations[i].Count > summary.TopOperations[j].Count
	})
	if len(summary.TopOperations) > 10 {
		summary.TopOperations = summary.TopOperations[:10]
	}

	// 转换用户统计
	for _, u := range stats.OpsByUser {
		summary.TopUsers = append(summary.TopUsers, UserActivitySummary{
			UserID:         u.UserID,
			Username:       u.Username,
			OperationCount: u.Count,
		})
	}

	return summary
}

// generateSensitiveReportSummary 生成敏感操作报告摘要
func (rg *ReportGenerator) generateSensitiveReportSummary(summary *SensitiveOpsSummary) *ReportSummary {
	if summary == nil {
		return nil
	}

	return &ReportSummary{
		TotalEvents:  summary.TotalSensitiveOps,
		SensitiveOps: summary.TotalSensitiveOps,
		EventsByCategory: map[string]int{
			"approved": summary.ApprovedOps,
			"blocked":  summary.BlockedOps,
			"pending":  summary.PendingApprovals,
		},
	}
}

// generateSecuritySummary 生成安全报告摘要
func (rg *ReportGenerator) generateSecuritySummary(report *AuditReport) *ReportSummary {
	summary := &ReportSummary{
		EventsByCategory: make(map[string]int),
		TopUsers:         make([]UserActivitySummary, 0),
		TopOperations:    make([]OperationCount, 0),
	}

	if report.LoginAnalysis != nil {
		summary.TotalEvents += report.LoginAnalysis.TotalLogins
		summary.UniqueUsers += report.LoginAnalysis.UniqueUsers
		summary.UniqueIPs += report.LoginAnalysis.UniqueIPs
		summary.HighRiskEvents += len(report.LoginAnalysis.AnomalousLogins)
	}

	if report.SensitiveOps != nil {
		summary.TotalEvents += report.SensitiveOps.TotalSensitiveOps
		summary.SensitiveOps = report.SensitiveOps.TotalSensitiveOps
	}

	if report.RiskAnalysis != nil {
		summary.SecurityAlerts = len(report.RiskAnalysis.ThreatIndicators)
	}

	return summary
}

// generateUserActivitySummary 生成用户活动摘要
func (rg *ReportGenerator) generateUserActivitySummary(start, end time.Time, userIDs []string) *ReportSummary {
	summary := &ReportSummary{
		EventsByCategory: make(map[string]int),
		TopUsers:         make([]UserActivitySummary, 0),
		TopOperations:    make([]OperationCount, 0),
	}

	// 从操作审计获取用户活动
	if rg.operationAuditor != nil {
		for _, userID := range userIDs {
			ops := rg.operationAuditor.GetUserOperations(userID, 1000)
			if len(ops) > 0 {
				summary.TopUsers = append(summary.TopUsers, UserActivitySummary{
					UserID:         userID,
					Username:       ops[0].Username,
					OperationCount: len(ops),
				})
				summary.TotalEvents += len(ops)
			}
		}
	}

	return summary
}

// generateRiskSummary 生成风险摘要
func (rg *ReportGenerator) generateRiskSummary(analysis *RiskAnalysis) *ReportSummary {
	return &ReportSummary{
		HighRiskEvents: analysis.HighRiskEvents,
		SecurityAlerts: len(analysis.ThreatIndicators),
		EventsByCategory: map[string]int{
			"overall_risk_score": analysis.OverallRiskScore,
		},
	}
}

// generateExecutiveSummary 生成执行摘要
func (rg *ReportGenerator) generateExecutiveSummary(report *AuditReport) *ReportSummary {
	summary := &ReportSummary{
		EventsByCategory: make(map[string]int),
		TopUsers:         make([]UserActivitySummary, 0),
		TopOperations:    make([]OperationCount, 0),
	}

	// 汇总所有数据
	if report.LoginAnalysis != nil {
		summary.TotalEvents += report.LoginAnalysis.TotalLogins
		summary.UniqueUsers += report.LoginAnalysis.UniqueUsers
		summary.UniqueIPs += report.LoginAnalysis.UniqueIPs
		summary.SuccessfulOps += report.LoginAnalysis.SuccessfulLogins
		summary.FailedOps += report.LoginAnalysis.FailedLogins
	}

	if report.OperationStats != nil {
		summary.TotalEvents += report.OperationStats.TotalOperations
		summary.SuccessfulOps += report.OperationStats.SuccessfulOps
		summary.FailedOps += report.OperationStats.FailedOps
		summary.SensitiveOps = report.OperationStats.SensitiveOpCount
	}

	if report.RiskAnalysis != nil {
		summary.HighRiskEvents = report.RiskAnalysis.HighRiskEvents
		summary.SecurityAlerts = len(report.RiskAnalysis.ThreatIndicators)
	}

	return summary
}

// ========== 建议生成 ==========

// generateRecommendations 生成建议
func (rg *ReportGenerator) generateRecommendations(report *AuditReport) []string {
	recommendations := make([]string, 0)

	// 基于登录分析
	if report.LoginAnalysis != nil {
		if report.LoginAnalysis.FailedLogins > report.LoginAnalysis.SuccessfulLogins/10 {
			recommendations = append(recommendations, "失败登录次数较高，建议检查是否存在暴力破解攻击")
		}

		if report.LoginAnalysis.MFAUsageRate < 50 {
			recommendations = append(recommendations, "MFA使用率较低，建议推广多因素认证")
		}

		if len(report.LoginAnalysis.AnomalousLogins) > 0 {
			recommendations = append(recommendations, "检测到异常登录行为，建议进一步调查")
		}
	}

	// 基于操作分析
	if report.OperationStats != nil {
		if report.OperationStats.FailedOps > report.OperationStats.SuccessfulOps/10 {
			recommendations = append(recommendations, "操作失败率较高，建议检查系统配置和权限设置")
		}

		if report.OperationStats.SensitiveOpCount > 0 {
			recommendations = append(recommendations, "存在敏感操作，建议定期审查敏感操作审批流程")
		}
	}

	// 基于风险分析
	if report.RiskAnalysis != nil {
		if report.RiskAnalysis.OverallRiskScore >= 60 {
			recommendations = append(recommendations, "整体风险分数较高，建议立即采取缓解措施")
		}

		for _, indicator := range report.RiskAnalysis.ThreatIndicators {
			if indicator.Severity == "high" || indicator.Severity == "critical" {
				recommendations = append(recommendations,
					fmt.Sprintf("检测到高严重性威胁[%s]: %s", indicator.Type, indicator.Description))
			}
		}
	}

	return recommendations
}

// ========== 图表数据生成 ==========

// generateChartData 生成图表数据
func (rg *ReportGenerator) generateChartData(report *AuditReport) map[string]interface{} {
	data := make(map[string]interface{})

	// 登录趋势图
	if report.LoginAnalysis != nil {
		data["login_by_hour"] = map[string]interface{}{
			"type": "bar",
			"data": generateHourlyDistribution(),
		}

		data["login_status"] = map[string]interface{}{
			"type": "pie",
			"data": map[string]int{
				"success": report.LoginAnalysis.SuccessfulLogins,
				"failure": report.LoginAnalysis.FailedLogins,
			},
		}
	}

	// 操作分布图
	if report.OperationStats != nil {
		data["ops_by_category"] = map[string]interface{}{
			"type": "bar",
			"data": report.OperationStats.OpsByCategory,
		}

		data["ops_by_action"] = map[string]interface{}{
			"type": "pie",
			"data": report.OperationStats.OpsByAction,
		}
	}

	// 风险仪表盘
	if report.RiskAnalysis != nil {
		data["risk_gauge"] = map[string]interface{}{
			"type":  "gauge",
			"value": report.RiskAnalysis.OverallRiskScore,
			"level": report.RiskAnalysis.RiskLevel,
		}
	}

	return data
}

// ========== 辅助方法 ==========

// getReportTitle 获取报告标题
func (rg *ReportGenerator) getReportTitle(reportType AuditReportType) string {
	titles := map[AuditReportType]string{
		ReportTypeLogin:        "登录审计报告",
		ReportTypeOperation:    "操作审计报告",
		ReportTypeSensitive:    "敏感操作报告",
		ReportTypeSecurity:     "安全审计报告",
		ReportTypeCompliance:   "合规审计报告",
		ReportTypeUserActivity: "用户活动报告",
		ReportTypeRiskAnalysis: "风险分析报告",
		ReportTypeExecutive:    "执行摘要报告",
	}

	if title, ok := titles[reportType]; ok {
		return title
	}
	return "审计报告"
}

// getReportDescription 获取报告描述
func (rg *ReportGenerator) getReportDescription(reportType AuditReportType) string {
	descriptions := map[AuditReportType]string{
		ReportTypeLogin:        "系统用户登录活动的详细分析报告",
		ReportTypeOperation:    "系统操作活动的详细分析报告",
		ReportTypeSensitive:    "敏感操作活动的详细分析报告",
		ReportTypeSecurity:     "系统安全状况的综合分析报告",
		ReportTypeCompliance:   "合规性检查和审计报告",
		ReportTypeUserActivity: "用户活动行为分析报告",
		ReportTypeRiskAnalysis: "系统风险分析和评估报告",
		ReportTypeExecutive:    "高层管理视角的审计摘要报告",
	}

	if desc, ok := descriptions[reportType]; ok {
		return desc
	}
	return "审计分析报告"
}

// generateHourlyDistribution 生成小时分布
func generateHourlyDistribution() map[int]int {
	distribution := make(map[int]int)
	for i := 0; i < 24; i++ {
		distribution[i] = 0
	}
	return distribution
}

// uniqueStrings 去重字符串切片
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ========== 持久化 ==========

// SaveReport 保存报告
func (rg *ReportGenerator) SaveReport(report *AuditReport) error {
	filename := filepath.Join(rg.storageDir, report.ReportID+".json")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0640)
}

// LoadReport 加载报告
func (rg *ReportGenerator) LoadReport(reportID string) (*AuditReport, error) {
	filename := filepath.Join(rg.storageDir, reportID+".json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var report AuditReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	return &report, nil
}

// ListReports 列出报告
func (rg *ReportGenerator) ListReports(limit int) ([]*AuditReport, error) {
	entries, err := os.ReadDir(rg.storageDir)
	if err != nil {
		return nil, err
	}

	reports := make([]*AuditReport, 0)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			reportID := strings.TrimSuffix(entry.Name(), ".json")
			report, err := rg.LoadReport(reportID)
			if err == nil {
				reports = append(reports, report)
			}
		}
	}

	// 按生成时间倒序
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].GeneratedAt.After(reports[j].GeneratedAt)
	})

	if limit > 0 && len(reports) > limit {
		reports = reports[:limit]
	}

	return reports, nil
}
