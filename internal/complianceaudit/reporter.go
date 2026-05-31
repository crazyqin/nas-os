// Package complianceaudit 提供审计报告生成功能
package complianceaudit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Reporter 审计报告生成器
type Reporter struct {
	logger *zap.Logger
}

// NewReporter 创建报告生成器
func NewReporter(logger *zap.Logger) *Reporter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Reporter{logger: logger}
}

// FullReport 完整审计报告
type FullReport struct {
	Summary          *FullReportSummary        `json:"summary"`
	ComplianceReport *ComplianceReport          `json:"compliance_report"`
	PolicyResults    []*PolicyResult            `json:"policy_results"`
	Remediations     []*RemediationItem         `json:"remediations"`
	Recommendations  []*Recommendation          `json:"recommendations"`
	RiskMatrix       *RiskMatrix                `json:"risk_matrix"`
	GeneratedAt      time.Time                  `json:"generated_at"`
}

// FullReportSummary 完整报告摘要
type FullReportSummary struct {
	OverallScore       float64                      `json:"overall_score"`
	RiskLevel          RiskLevel                    `json:"risk_level"`
	TotalChecks        int                          `json:"total_checks"`
	Passed             int                          `json:"passed"`
	Failed             int                          `json:"failed"`
	Warnings           int                          `json:"warnings"`
	ComplianceStatus   map[string]bool              `json:"compliance_status"`
	CategoryScores     map[CheckCategory]float64    `json:"category_scores"`
	StandardScores     map[ComplianceStandard]float64 `json:"standard_scores"`
	ActiveRemediations int                          `json:"active_remediations"`
	TopIssues          []*Finding                   `json:"top_issues"`
}

// Recommendation 改进建议
type Recommendation struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Priority    int      `json:"priority"`
	Impact      string   `json:"impact"`
	Effort      string   `json:"effort"`
}

// RiskMatrix 风险矩阵
type RiskMatrix struct {
	Critical []*Finding `json:"critical"`
	High     []*Finding `json:"high"`
	Medium   []*Finding `json:"medium"`
	Low      []*Finding `json:"low"`
}

// GenerateFullReport 生成完整审计报告
func (r *Reporter) GenerateFullReport(
	report *ComplianceReport,
	policyResults []*PolicyResult,
	remediations []*RemediationItem,
) *FullReport {
	fullReport := &FullReport{
		ComplianceReport: report,
		PolicyResults:    policyResults,
		Remediations:     remediations,
		Recommendations:  r.generateRecommendations(report, policyResults),
		RiskMatrix:       r.buildRiskMatrix(report.Findings),
		GeneratedAt:      time.Now(),
	}

	fullReport.Summary = r.buildSummary(report, policyResults, remediations)

	return fullReport
}

// buildSummary 构建报告摘要
func (r *Reporter) buildSummary(
	report *ComplianceReport,
	policyResults []*PolicyResult,
	remediations []*RemediationItem,
) *FullReportSummary {
	summary := &FullReportSummary{
		OverallScore:     report.Summary.OverallScore,
		RiskLevel:        report.Summary.RiskLevel,
		TotalChecks:      report.Summary.TotalChecks,
		Passed:           report.Summary.Passed,
		Failed:           report.Summary.Failed,
		Warnings:         report.Summary.Warnings,
		ComplianceStatus: make(map[string]bool),
		CategoryScores:   make(map[CheckCategory]float64),
		StandardScores:   make(map[ComplianceStandard]float64),
	}

	// 合规状态
	for _, pr := range policyResults {
		summary.ComplianceStatus[string(pr.Standard)] = pr.Compliant
		summary.StandardScores[pr.Standard] = pr.Score
	}

	// 按类别计算得分
	categoryPass := make(map[CheckCategory]int)
	categoryTotal := make(map[CheckCategory]int)

	for _, sr := range report.Standards {
		for _, check := range sr.Checks {
			categoryTotal[check.Category]++
			if check.Status == StatusPass {
				categoryPass[check.Category]++
			}
		}
	}

	for cat, total := range categoryTotal {
		if total > 0 {
			summary.CategoryScores[cat] = float64(categoryPass[cat]) / float64(total) * 100
		}
	}

	// 整改项统计
	for _, r := range remediations {
		if r.Status == "pending" || r.Status == "in_progress" {
			summary.ActiveRemediations++
		}
	}

	// Top 问题（按风险排序）
	summary.TopIssues = r.getTopIssues(report.Findings, 5)

	return summary
}

// getTopIssues 获取最重要的问题
func (r *Reporter) getTopIssues(findings []*Finding, limit int) []*Finding {
	if len(findings) == 0 {
		return make([]*Finding, 0)
	}

	// 按风险等级排序
	sorted := make([]*Finding, len(findings))
	copy(sorted, findings)
	sort.Slice(sorted, func(i, j int) bool {
		return riskPriority(sorted[i].RiskLevel) > riskPriority(sorted[j].RiskLevel)
	})

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

// riskPriority 风险等级优先级
func riskPriority(level RiskLevel) int {
	switch level {
	case RiskCritical:
		return 4
	case RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}

// buildRiskMatrix 构建风险矩阵
func (r *Reporter) buildRiskMatrix(findings []*Finding) *RiskMatrix {
	matrix := &RiskMatrix{
		Critical: make([]*Finding, 0),
		High:     make([]*Finding, 0),
		Medium:   make([]*Finding, 0),
		Low:      make([]*Finding, 0),
	}

	for _, f := range findings {
		switch f.RiskLevel {
		case RiskCritical:
			matrix.Critical = append(matrix.Critical, f)
		case RiskHigh:
			matrix.High = append(matrix.High, f)
		case RiskMedium:
			matrix.Medium = append(matrix.Medium, f)
		case RiskLow:
			matrix.Low = append(matrix.Low, f)
		}
	}

	return matrix
}

// generateRecommendations 生成改进建议
func (r *Reporter) generateRecommendations(
	report *ComplianceReport,
	policyResults []*PolicyResult,
) []*Recommendation {
	recs := make([]*Recommendation, 0)
	recID := 0

	// 基于失败项生成建议
	for _, finding := range report.Findings {
		recID++
		rec := &Recommendation{
			ID:       fmt.Sprintf("rec_%d", recID),
			Title:    fmt.Sprintf("修复: %s", finding.Title),
			Category: string(finding.Category),
		}

		switch finding.Category {
		case CategoryPasswordPolicy:
			rec.Description = "加强密码策略：要求最小长度8位，包含大小写字母、数字和特殊字符"
			rec.Impact = "降低账户被暴力破解的风险"
			rec.Effort = "低"
			rec.Priority = 4
		case CategoryAccessControl:
			rec.Description = "加强访问控制：限制敏感目录权限，禁用不必要的系统账户"
			rec.Impact = "减少未授权访问的风险"
			rec.Effort = "中"
			rec.Priority = 3
		case CategoryEncryption:
			rec.Description = "启用数据加密：配置磁盘加密 (LUKS)，启用 TLS 1.2+，使用强加密算法"
			rec.Impact = "保护数据在传输和存储中的安全"
			rec.Effort = "中"
			rec.Priority = 4
		case CategoryNetworkSecurity:
			rec.Description = "加强网络安全：关闭不必要的端口，配置防火墙规则，使用 VPN"
			rec.Impact = "减少网络攻击面"
			rec.Effort = "中"
			rec.Priority = 3
		case CategoryAuditLog:
			rec.Description = "完善审计日志：启用 auditd，配置远程日志服务器，设置日志保留策略"
			rec.Impact = "提高安全事件检测和追溯能力"
			rec.Effort = "低"
			rec.Priority = 3
		case CategoryDataProtection:
			rec.Description = "加强数据保护：实施数据分类，配置数据保留策略，定期备份"
			rec.Impact = "保护敏感数据，满足合规要求"
			rec.Effort = "高"
			rec.Priority = 4
		case CategoryIncidentResponse:
			rec.Description = "建立应急响应机制：制定应急预案，配置告警通知，定期演练"
			rec.Impact = "提高安全事件响应速度"
			rec.Effort = "高"
			rec.Priority = 3
		default:
			rec.Description = fmt.Sprintf("解决 %s 类别的安全问题", finding.Category)
			rec.Impact = "提高整体安全水平"
			rec.Effort = "中"
			rec.Priority = 2
		}

		recs = append(recs, rec)
	}

	// 基于策略评估生成通用建议
	for _, pr := range policyResults {
		if !pr.Compliant {
			recID++
			recs = append(recs, &Recommendation{
				ID:          fmt.Sprintf("rec_%d", recID),
				Title:       fmt.Sprintf("达到 %s 合规要求", pr.Standard),
				Description: fmt.Sprintf("当前合规评分 %.1f%%，需要提升到 70%% 以上", pr.Score),
				Category:    "compliance",
				Priority:    5,
				Impact:      "满足法规合规要求，降低法律风险",
				Effort:      "高",
			})
		}
	}

	return recs
}

// ExportJSON 导出 JSON 格式报告
func (r *Reporter) ExportJSON(report *FullReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// ExportMarkdown 导出 Markdown 格式报告
func (r *Reporter) ExportMarkdown(report *FullReport) string {
	var sb strings.Builder

	sb.WriteString("# 合规审计报告\n\n")
	sb.WriteString(fmt.Sprintf("**生成时间:** %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))

	// 摘要
	sb.WriteString("## 摘要\n\n")
	sb.WriteString(fmt.Sprintf("- **总分:** %.1f%%\n", report.Summary.OverallScore))
	sb.WriteString(fmt.Sprintf("- **风险等级:** %s\n", report.Summary.RiskLevel))
	sb.WriteString(fmt.Sprintf("- **总检查项:** %d\n", report.Summary.TotalChecks))
	sb.WriteString(fmt.Sprintf("- **通过:** %d\n", report.Summary.Passed))
	sb.WriteString(fmt.Sprintf("- **失败:** %d\n", report.Summary.Failed))
	sb.WriteString(fmt.Sprintf("- **警告:** %d\n", report.Summary.Warnings))
	sb.WriteString("\n")

	// 合规状态
	sb.WriteString("## 合规状态\n\n")
	for standard, compliant := range report.Summary.ComplianceStatus {
		status := "✅ 合规"
		if !compliant {
			status = "❌ 不合规"
		}
		sb.WriteString(fmt.Sprintf("- **%s:** %s (%.1f%%)\n", standard, status, report.Summary.StandardScores[ComplianceStandard(standard)]))
	}
	sb.WriteString("\n")

	// 风险矩阵
	if report.RiskMatrix != nil {
		sb.WriteString("## 风险矩阵\n\n")
		if len(report.RiskMatrix.Critical) > 0 {
			sb.WriteString(fmt.Sprintf("### 🔴 严重 (%d)\n\n", len(report.RiskMatrix.Critical)))
			for _, f := range report.RiskMatrix.Critical {
				sb.WriteString(fmt.Sprintf("- %s\n", f.Title))
			}
			sb.WriteString("\n")
		}
		if len(report.RiskMatrix.High) > 0 {
			sb.WriteString(fmt.Sprintf("### 🟠 高风险 (%d)\n\n", len(report.RiskMatrix.High)))
			for _, f := range report.RiskMatrix.High {
				sb.WriteString(fmt.Sprintf("- %s\n", f.Title))
			}
			sb.WriteString("\n")
		}
		if len(report.RiskMatrix.Medium) > 0 {
			sb.WriteString(fmt.Sprintf("### 🟡 中风险 (%d)\n\n", len(report.RiskMatrix.Medium)))
			for _, f := range report.RiskMatrix.Medium {
				sb.WriteString(fmt.Sprintf("- %s\n", f.Title))
			}
			sb.WriteString("\n")
		}
		if len(report.RiskMatrix.Low) > 0 {
			sb.WriteString(fmt.Sprintf("### 🟢 低风险 (%d)\n\n", len(report.RiskMatrix.Low)))
			for _, f := range report.RiskMatrix.Low {
				sb.WriteString(fmt.Sprintf("- %s\n", f.Title))
			}
			sb.WriteString("\n")
		}
	}

	// 改进建议
	if len(report.Recommendations) > 0 {
		sb.WriteString("## 改进建议\n\n")
		for _, rec := range report.Recommendations {
			sb.WriteString(fmt.Sprintf("### %s\n\n", rec.Title))
			sb.WriteString(fmt.Sprintf("- **优先级:** %d/5\n", rec.Priority))
			sb.WriteString(fmt.Sprintf("- **影响:** %s\n", rec.Impact))
			sb.WriteString(fmt.Sprintf("- **工作量:** %s\n", rec.Effort))
			sb.WriteString(fmt.Sprintf("- **说明:** %s\n\n", rec.Description))
		}
	}

	// 整改项
	if len(report.Remediations) > 0 {
		sb.WriteString("## 整改项\n\n")
		for _, rem := range report.Remediations {
			sb.WriteString(fmt.Sprintf("### %s\n\n", rem.Title))
			sb.WriteString(fmt.Sprintf("- **状态:** %s\n", rem.Status))
			sb.WriteString(fmt.Sprintf("- **优先级:** %d\n", rem.Priority))
			sb.WriteString(fmt.Sprintf("- **说明:** %s\n", rem.Description))
			if len(rem.Steps) > 0 {
				sb.WriteString("- **步骤:**\n")
				for i, step := range rem.Steps {
					sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step))
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
