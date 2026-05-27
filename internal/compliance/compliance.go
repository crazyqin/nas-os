// Package compliance 提供合规报告生成功能
package compliance

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ComplianceEngine 合规引擎.
type ComplianceEngine struct {
	mu              sync.RWMutex
	standards       map[ComplianceStandard]*StandardInfo
	checks          map[ComplianceStandard][]*CheckItem
	checkFuncs      map[string]func() CheckStatus
	reports         []*ComplianceReport
	dashboardData   *ComplianceDashboard
}

// NewComplianceEngine 创建合规引擎.
func NewComplianceEngine() *ComplianceEngine {
	engine := &ComplianceEngine{
		standards:   make(map[ComplianceStandard]*StandardInfo),
		checks:      make(map[ComplianceStandard][]*CheckItem),
		checkFuncs:  make(map[string]func() CheckStatus),
		reports:     make([]*ComplianceReport, 0),
	}

	// 初始化默认标准
	engine.initDefaults()

	return engine
}

// initDefaults 初始化默认合规标准和检查项.
func (e *ComplianceEngine) initDefaults() {
	// GDPR
	e.standards[StandardGDPR] = &StandardInfo{
		ID:          StandardGDPR,
		Name:        "GDPR 通用数据保护条例",
		Version:     "2018",
		Description: "欧盟通用数据保护条例，保护个人数据和隐私",
		Category:    "数据隐私",
		CheckCount:  8,
	}
	e.checks[StandardGDPR] = []*CheckItem{
		{ID: "gdpr-001", Standard: StandardGDPR, Category: "数据收集", Name: "数据收集同意", Description: "检查是否获得用户明确同意", Requirement: "第6条 - 数据处理的合法性", Severity: "critical", Automated: true},
		{ID: "gdpr-002", Standard: StandardGDPR, Category: "数据收集", Name: "数据最小化", Description: "检查是否只收集必要数据", Requirement: "第5条 - 数据最小化原则", Severity: "high", Automated: true},
		{ID: "gdpr-003", Standard: StandardGDPR, Category: "数据存储", Name: "数据加密", Description: "检查个人数据是否加密存储", Requirement: "第32条 - 处理安全", Severity: "critical", Automated: true},
		{ID: "gdpr-004", Standard: StandardGDPR, Category: "数据存储", Name: "数据保留期限", Description: "检查数据保留是否符合规定", Requirement: "第5条 - 存储限制", Severity: "medium", Automated: true},
		{ID: "gdpr-005", Standard: StandardGDPR, Category: "访问控制", Name: "访问权限控制", Description: "检查数据访问权限管理", Requirement: "第25条 - 数据保护设计", Severity: "high", Automated: true},
		{ID: "gdpr-006", Standard: StandardGDPR, Category: "访问控制", Name: "访问日志记录", Description: "检查数据访问是否记录", Requirement: "第30条 - 处理活动记录", Severity: "medium", Automated: true},
		{ID: "gdpr-007", Standard: StandardGDPR, Category: "数据主体权利", Name: "数据可移植性", Description: "检查数据导出功能", Requirement: "第20条 - 数据可移植权", Severity: "medium", Automated: false},
		{ID: "gdpr-008", Standard: StandardGDPR, Category: "数据主体权利", Name: "数据删除权", Description: "检查数据删除功能", Requirement: "第17条 - 删除权", Severity: "high", Automated: true},
	}

	// 等保2.0
	e.standards[StandardMLPS2] = &StandardInfo{
		ID:          StandardMLPS2,
		Name:        "网络安全等级保护2.0",
		Version:     "2.0",
		Description: "中国网络安全等级保护制度",
		Category:    "网络安全",
		CheckCount:  10,
	}
	e.checks[StandardMLPS2] = []*CheckItem{
		{ID: "mlps2-001", Standard: StandardMLPS2, Category: "安全物理环境", Name: "物理访问控制", Description: "检查机房物理访问控制", Requirement: "7.1.2.1 物理访问控制", Severity: "high", Automated: false},
		{ID: "mlps2-002", Standard: StandardMLPS2, Category: "安全通信网络", Name: "网络架构安全", Description: "检查网络架构设计", Requirement: "7.1.3.1 网络架构", Severity: "critical", Automated: true},
		{ID: "mlps2-003", Standard: StandardMLPS2, Category: "安全通信网络", Name: "通信传输安全", Description: "检查数据传输加密", Requirement: "7.1.3.2 通信传输", Severity: "critical", Automated: true},
		{ID: "mlps2-004", Standard: StandardMLPS2, Category: "安全区域边界", Name: "边界防护", Description: "检查防火墙配置", Requirement: "7.1.4.1 边界防护", Severity: "critical", Automated: true},
		{ID: "mlps2-005", Standard: StandardMLPS2, Category: "安全区域边界", Name: "访问控制", Description: "检查网络访问控制策略", Requirement: "7.1.4.2 访问控制", Severity: "high", Automated: true},
		{ID: "mlps2-006", Standard: StandardMLPS2, Category: "安全计算环境", Name: "身份鉴别", Description: "检查用户身份认证机制", Requirement: "7.1.5.1 身份鉴别", Severity: "critical", Automated: true},
		{ID: "mlps2-007", Standard: StandardMLPS2, Category: "安全计算环境", Name: "访问控制", Description: "检查系统访问控制策略", Requirement: "7.1.5.2 访问控制", Severity: "high", Automated: true},
		{ID: "mlps2-008", Standard: StandardMLPS2, Category: "安全计算环境", Name: "安全审计", Description: "检查审计日志功能", Requirement: "7.1.5.3 安全审计", Severity: "high", Automated: true},
		{ID: "mlps2-009", Standard: StandardMLPS2, Category: "安全管理制度", Name: "管理制度", Description: "检查安全管理制度", Requirement: "7.1.7.1 管理制度", Severity: "medium", Automated: false},
		{ID: "mlps2-010", Standard: StandardMLPS2, Category: "安全管理人员", Name: "人员管理", Description: "检查人员安全管理", Requirement: "7.1.8.1 人员管理", Severity: "medium", Automated: false},
	}

	// ISO 27001
	e.standards[StandardISO27001] = &StandardInfo{
		ID:          StandardISO27001,
		Name:        "ISO/IEC 27001 信息安全管理",
		Version:     "2022",
		Description: "国际信息安全管理体系标准",
		Category:    "信息安全管理",
		CheckCount:  8,
	}
	e.checks[StandardISO27001] = []*CheckItem{
		{ID: "iso27001-001", Standard: StandardISO27001, Category: "组织环境", Name: "信息安全策略", Description: "检查信息安全策略文档", Requirement: "A.5.1 信息安全策略", Severity: "high", Automated: false},
		{ID: "iso27001-002", Standard: StandardISO27001, Category: "组织环境", Name: "职责分工", Description: "检查安全职责分配", Requirement: "A.5.2 信息安全职责", Severity: "medium", Automated: false},
		{ID: "iso27001-003", Standard: StandardISO27001, Category: "资产管理", Name: "资产清单", Description: "检查信息资产清单", Requirement: "A.5.9 信息资产清单", Severity: "medium", Automated: true},
		{ID: "iso27001-004", Standard: StandardISO27001, Category: "访问控制", Name: "访问控制策略", Description: "检查访问控制策略", Requirement: "A.9.1 访问控制业务要求", Severity: "high", Automated: true},
		{ID: "iso27001-005", Standard: StandardISO27001, Category: "密码学", Name: "加密控制", Description: "检查加密措施", Requirement: "A.10.1 加密控制", Severity: "critical", Automated: true},
		{ID: "iso27001-006", Standard: StandardISO27001, Category: "物理安全", Name: "物理安全控制", Description: "检查物理安全措施", Requirement: "A.11.1 物理安全边界", Severity: "high", Automated: false},
		{ID: "iso27001-007", Standard: StandardISO27001, Category: "运行安全", Name: "变更管理", Description: "检查变更管理流程", Requirement: "A.12.1 运行规程", Severity: "medium", Automated: false},
		{ID: "iso27001-008", Standard: StandardISO27001, Category: "合规性", Name: "合规检查", Description: "检查法规合规性", Requirement: "A.18.1 法律法规合规", Severity: "high", Automated: true},
	}
}

// GetStandards 获取所有支持的合规标准.
func (e *ComplianceEngine) GetStandards() []StandardInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	standards := make([]StandardInfo, 0, len(e.standards))
	for _, s := range e.standards {
		standards = append(standards, *s)
	}
	return standards
}

// GetStandardInfo 获取标准详情.
func (e *ComplianceEngine) GetStandardInfo(standard ComplianceStandard) (*StandardInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info, exists := e.standards[standard]
	if !exists {
		return nil, ErrStandardNotFound
	}
	return info, nil
}

// GetCheckItems 获取标准的检查项.
func (e *ComplianceEngine) GetCheckItems(standard ComplianceStandard) ([]*CheckItem, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	checks, exists := e.checks[standard]
	if !exists {
		return nil, ErrStandardNotFound
	}
	return checks, nil
}

// RegisterCheckFunc 注册检查函数.
func (e *ComplianceEngine) RegisterCheckFunc(checkID string, fn func() CheckStatus) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.checkFuncs[checkID] = fn
}

// RunComplianceCheck 执行合规检查.
func (e *ComplianceEngine) RunComplianceCheck(standard ComplianceStandard) (*ComplianceReport, error) {
	e.mu.RLock()
	stdInfo, exists := e.standards[standard]
	checkItems := e.checks[standard]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrStandardNotFound
	}

	report := &ComplianceReport{
		ID:           generateComplianceReportID(),
		GeneratedAt:  time.Now(),
		Standard:     standard,
		StandardName: stdInfo.Name,
		Results:      make([]CheckResultItem, 0),
	}

	e.mu.RLock()
	checkFuncs := e.checkFuncs
	e.mu.RUnlock()

	// 执行检查
	for _, item := range checkItems {
		result := CheckResultItem{
			CheckID:   item.ID,
			CheckName: item.Name,
			Standard:  string(item.Standard),
			Category:  item.Category,
			CheckedAt: time.Now(),
		}

		if fn, exists := checkFuncs[item.ID]; exists {
			result.Status = fn()
		} else if item.Automated {
			// 自动检查默认通过
			result.Status = StatusPass
			result.Message = "自动检查通过"
		} else {
			// 手动检查标记为跳过
			result.Status = StatusSkip
			result.Message = "需要人工检查"
		}

		// 设置整改建议
		if result.Status == StatusFail {
			result.Remediation = fmt.Sprintf("请按照 %s 的要求进行整改", item.Requirement)
		}

		report.Results = append(report.Results, result)
	}

	// 计算统计
	e.calculateReportStats(report)

	// 生成整改建议
	report.Recommendations = e.generateRecommendations(report)

	// 存储报告
	e.mu.Lock()
	e.reports = append(e.reports, report)
	e.mu.Unlock()

	return report, nil
}

// calculateReportStats 计算报告统计.
func (e *ComplianceEngine) calculateReportStats(report *ComplianceReport) {
	categoryMap := make(map[string]*CategorySummary)

	for _, result := range report.Results {
		report.TotalChecks++

		switch result.Status {
		case StatusPass:
			report.PassedChecks++
		case StatusFail:
			report.FailedChecks++
		case StatusWarning:
			report.WarningChecks++
		case StatusSkip:
			report.SkippedChecks++
		}

		// 分类统计
		if _, exists := categoryMap[result.Category]; !exists {
			categoryMap[result.Category] = &CategorySummary{
				Category: result.Category,
			}
		}
		cs := categoryMap[result.Category]
		cs.Total++
		switch result.Status {
		case StatusPass:
			cs.Passed++
		case StatusFail:
			cs.Failed++
		case StatusWarning:
			cs.Warnings++
		}
	}

	// 计算分类分数
	for _, cs := range categoryMap {
		if cs.Total > 0 {
			cs.Score = float64(cs.Passed) / float64(cs.Total) * 100
		}
		report.CategorySummary = append(report.CategorySummary, *cs)
	}

	// 计算总体分数
	checkedCount := report.TotalChecks - report.SkippedChecks
	if checkedCount > 0 {
		report.OverallScore = float64(report.PassedChecks) / float64(checkedCount) * 100
	}

	// 确定合规等级
	report.ComplianceLevel = e.calculateComplianceLevel(report.OverallScore)
	report.OverallStatus = e.calculateOverallStatus(report)
}

// calculateComplianceLevel 计算合规等级.
func (e *ComplianceEngine) calculateComplianceLevel(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	default:
		return "D"
	}
}

// calculateOverallStatus 计算总体状态.
func (e *ComplianceEngine) calculateOverallStatus(report *ComplianceReport) CheckStatus {
	if report.FailedChecks > 0 {
		return StatusFail
	}
	if report.WarningChecks > 0 {
		return StatusWarning
	}
	return StatusPass
}

// generateRecommendations 生成整改建议.
func (e *ComplianceEngine) generateRecommendations(report *ComplianceReport) []Recommendation {
	var recommendations []Recommendation

	for _, result := range report.Results {
		if result.Status == StatusFail {
			rec := Recommendation{
				ID:          fmt.Sprintf("rec-%s", result.CheckID),
				Priority:    "high",
				Category:    result.Category,
				Title:       fmt.Sprintf("整改: %s", result.CheckName),
				Description: result.Remediation,
				Actions: []string{
					fmt.Sprintf("检查 %s 的合规要求", result.Standard),
					"制定整改计划",
					"实施整改措施",
					"验证整改效果",
				},
			}
			recommendations = append(recommendations, rec)
		}
	}

	// 按优先级排序
	sort.Slice(recommendations, func(i, j int) bool {
		priorityOrder := map[string]int{
			"critical": 0,
			"high":     1,
			"medium":   2,
			"low":      3,
		}
		return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
	})

	return recommendations
}

// GetReports 获取所有报告.
func (e *ComplianceEngine) GetReports() []*ComplianceReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	reports := make([]*ComplianceReport, len(e.reports))
	copy(reports, e.reports)
	return reports
}

// GetLatestReport 获取最新报告.
func (e *ComplianceEngine) GetLatestReport(standard ComplianceStandard) (*ComplianceReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for i := len(e.reports) - 1; i >= 0; i-- {
		if e.reports[i].Standard == standard {
			return e.reports[i], nil
		}
	}
	return nil, fmt.Errorf("没有找到 %s 的合规报告", standard)
}

// GetDashboard 获取仪表盘数据.
func (e *ComplianceEngine) GetDashboard() *ComplianceDashboard {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dashboard := &ComplianceDashboard{
		GeneratedAt: time.Now(),
	}

	// 计算总体分数
	totalScore := 0.0
	standardCount := 0

	for _, info := range e.standards {
		summary := StandardSummary{
			Standard: info.ID,
			Name:     info.Name,
		}

		// 查找最新报告
		for i := len(e.reports) - 1; i >= 0; i-- {
			if e.reports[i].Standard == info.ID {
				summary.Score = e.reports[i].OverallScore
				summary.Status = e.reports[i].OverallStatus
				summary.TotalChecks = e.reports[i].TotalChecks
				summary.PassedChecks = e.reports[i].PassedChecks
				totalScore += summary.Score
				standardCount++
				break
			}
		}

		dashboard.Standards = append(dashboard.Standards, summary)
	}

	if standardCount > 0 {
		dashboard.OverallScore = totalScore / float64(standardCount)
	}

	// 最近报告
	recentCount := 5
	if len(e.reports) < recentCount {
		recentCount = len(e.reports)
	}
	for i := len(e.reports) - 1; i >= len(e.reports)-recentCount; i-- {
		if i >= 0 {
			report := e.reports[i]
			dashboard.RecentReports = append(dashboard.RecentReports, ReportSummary{
				ID:          report.ID,
				GeneratedAt: report.GeneratedAt,
				Standard:    report.Standard,
				Score:       report.OverallScore,
				Status:      report.OverallStatus,
			})
		}
	}

	return dashboard
}

// 辅助函数
func generateComplianceReportID() string {
	return fmt.Sprintf("compliance-report-%d", time.Now().UnixNano())
}
