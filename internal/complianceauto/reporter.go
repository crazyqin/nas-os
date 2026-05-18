// Package complianceauto 提供自动化合规检查功能
package complianceauto

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ReportFormat 报告格式
type ReportFormat string

const (
	FormatJSON ReportFormat = "json" // JSON 格式
	FormatHTML ReportFormat = "html" // HTML 格式
	FormatPDF  ReportFormat = "pdf"  // PDF 格式
)

// StoredReport 存储的报告
type StoredReport struct {
	ID          string       `json:"id"`          // 报告ID
	Title       string       `json:"title"`       // 标题
	Format      string       `json:"format"`      // 格式
	Content     []byte       `json:"content"`     // 内容
	GeneratedAt time.Time    `json:"generatedAt"` // 生成时间
	ScanID      string       `json:"scanId"`      // 关联扫描ID
	Size        int64        `json:"size"`        // 文件大小
}

// Reporter 报告生成器
type Reporter struct {
	mu      sync.RWMutex
	reports map[string]*StoredReport // 存储的报告
}

// NewReporter 创建报告生成器
func NewReporter() *Reporter {
	return &Reporter{
		reports: make(map[string]*StoredReport),
	}
}

// GenerateReport 生成合规报告
func (r *Reporter) GenerateReport(scan *ComplianceScan, title string, format string) (*StoredReport, error) {
	if scan == nil {
		return nil, fmt.Errorf("扫描结果为空")
	}

	if title == "" {
		title = fmt.Sprintf("合规审计报告 - %s", scan.StartTime.Format("2006-01-02 15:04:05"))
	}

	// 构建审计报告
	auditReport := r.buildAuditReport(scan, title)

	// 根据格式生成内容
	var content []byte
	var err error

	switch ReportFormat(format) {
	case FormatHTML:
		content, err = r.generateHTML(auditReport)
	case FormatPDF:
		content, err = r.generatePDF(auditReport)
	default:
		content, err = r.generateJSON(auditReport)
		format = "json"
	}

	if err != nil {
		return nil, fmt.Errorf("生成报告失败: %w", err)
	}

	report := &StoredReport{
		ID:          fmt.Sprintf("report-%d", time.Now().UnixNano()),
		Title:       title,
		Format:      format,
		Content:     content,
		GeneratedAt: time.Now(),
		ScanID:      scan.ID,
		Size:        int64(len(content)),
	}

	// 存储报告
	r.mu.Lock()
	r.reports[report.ID] = report
	r.mu.Unlock()

	return report, nil
}

// GetReport 获取报告
func (r *Reporter) GetReport(reportID string) (*StoredReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report, exists := r.reports[reportID]
	if !exists {
		return nil, fmt.Errorf("报告 %s 不存在", reportID)
	}

	return report, nil
}

// ListReports 列出所有报告
func (r *Reporter) ListReports() []*StoredReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reports := make([]*StoredReport, 0, len(r.reports))
	for _, report := range r.reports {
		reports = append(reports, report)
	}
	return reports
}

// buildAuditReport 构建审计报告
func (r *Reporter) buildAuditReport(scan *ComplianceScan, title string) *AuditReport {
	report := &AuditReport{
		ID:          fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Title:       title,
		ScanID:      scan.ID,
		Standards:   scan.Standards,
		GeneratedAt: time.Now(),
		Metadata: map[string]string{
			"generator": "complianceauto",
			"version":   "1.0.0",
		},
	}

	// 计算摘要
	totalChecks := len(scan.Checks)
	passedChecks := scan.PassedRules
	failedChecks := scan.FailedRules
	warningChecks := scan.WarnRules

	// 计算合规分数
	complianceScore := float64(0)
	if totalChecks > 0 {
		complianceScore = float64(passedChecks) / float64(totalChecks) * 100
	}

	report.Summary = ReportSummary{
		ComplianceScore: complianceScore,
		TotalChecks:     totalChecks,
		PassedChecks:    passedChecks,
		FailedChecks:    failedChecks,
		WarningChecks:   warningChecks,
	}

	// 统计发现项严重程度
	for _, check := range scan.Checks {
		if check.Result == ResultFail {
			// 根据规则查找严重程度
			severity := SeverityMedium // 默认中等
			report.Summary.MediumFindings++

			switch severity {
			case SeverityCritical:
				report.Summary.CriticalFindings++
			case SeverityHigh:
				report.Summary.HighFindings++
			case SeverityMedium:
				// 已计算
			case SeverityLow:
				report.Summary.LowFindings++
			}
		}
	}

	// 构建分类结果
	categories := make(map[RuleCategory]*CategoryResult)
	for _, check := range scan.Checks {
		// 根据规则ID查找规则类别
		category := r.getCategoryByRuleID(check.RuleID)

		if _, exists := categories[category]; !exists {
			categories[category] = &CategoryResult{
				Category: category,
			}
		}

		cat := categories[category]
		cat.TotalChecks++

		switch check.Result {
		case ResultPass:
			cat.Passed++
		case ResultFail:
			cat.Failed++
		}
	}

	// 计算分类分数
	for _, cat := range categories {
		if cat.TotalChecks > 0 {
			cat.Score = float64(cat.Passed) / float64(cat.TotalChecks) * 100
		}
		report.Categories = append(report.Categories, *cat)
	}

	// 构建发现项
	for _, check := range scan.Checks {
		if check.Result == ResultFail || check.Result == ResultWarning {
			finding := Finding{
				ID:          fmt.Sprintf("finding-%s-%d", check.RuleID, time.Now().UnixNano()),
				RuleID:      check.RuleID,
				Severity:    r.getSeverityByRuleID(check.RuleID),
				Title:       check.Message,
				Description: check.Evidence,
				Evidence:    check.Evidence,
				Status:      "open",
			}
			report.Findings = append(report.Findings, finding)
		}
	}

	// 构建建议
	report.Recommendations = r.buildRecommendations(scan)

	return report
}

// getCategoryByRuleID 根据规则ID获取类别
func (r *Reporter) getCategoryByRuleID(ruleID string) RuleCategory {
	// 简化版本：根据规则ID前缀判断
	switch {
	case len(ruleID) >= 4 && ruleID[:4] == "CIS-":
		return CategorySystemHardening
	case len(ruleID) >= 5 && ruleID[:5] == "NIST-":
		return CategoryAccessControl
	case len(ruleID) >= 5 && ruleID[:5] == "GDPR-":
		return CategoryDataProtection
	case len(ruleID) >= 6 && ruleID[:6] == "MLPS2-":
		return CategorySystemHardening
	default:
		return CategorySystemHardening
	}
}

// getSeverityByRuleID 根据规则ID获取严重程度
func (r *Reporter) getSeverityByRuleID(ruleID string) SeverityLevel {
	// 简化版本：返回默认严重程度
	return SeverityMedium
}

// buildRecommendations 构建建议
func (r *Reporter) buildRecommendations(scan *ComplianceScan) []Recommendation {
	var recommendations []Recommendation

	// 根据未通过的检查构建建议
	for i, check := range scan.Checks {
		if check.Result == ResultFail {
			rec := Recommendation{
				ID:          fmt.Sprintf("rec-%d", i+1),
				Priority:    r.getSeverityByRuleID(check.RuleID),
				Title:       fmt.Sprintf("修复 %s", check.RuleID),
				Description: check.Message,
				Action:      "请检查并修复相关配置",
				Effort:      "中等",
			}
			recommendations = append(recommendations, rec)
		}
	}

	return recommendations
}

// generateJSON 生成JSON格式报告
func (r *Reporter) generateJSON(report *AuditReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// generateHTML 生成HTML格式报告
func (r *Reporter) generateHTML(report *AuditReport) ([]byte, error) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #f0f0f0; padding: 20px; border-radius: 5px; }
        .summary { margin: 20px 0; }
        .score { font-size: 48px; font-weight: bold; color: %s; }
        .section { margin: 20px 0; border: 1px solid #ddd; padding: 15px; border-radius: 5px; }
        .section h2 { margin-top: 0; }
        table { width: 100%%; border-collapse: collapse; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
        .pass { color: green; }
        .fail { color: red; }
        .warning { color: orange; }
        .finding { background-color: #fff3cd; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .recommendation { background-color: #d4edda; padding: 10px; margin: 10px 0; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>%s</h1>
        <p>生成时间: %s</p>
        <p>扫描ID: %s</p>
    </div>
    
    <div class="summary">
        <h2>合规概览</h2>
        <div class="score">%.1f%%</div>
        <p>总检查项: %d | 通过: %d | 未通过: %d | 警告: %d</p>
    </div>
    
    <div class="section">
        <h2>检查结果</h2>
        <table>
            <tr>
                <th>规则ID</th>
                <th>结果</th>
                <th>信息</th>
                <th>检查时间</th>
            </tr>`,
		report.Title,
		r.getScoreColor(report.Summary.ComplianceScore),
		report.Title,
		report.GeneratedAt.Format("2006-01-02 15:04:05"),
		report.ScanID,
		report.Summary.ComplianceScore,
		report.Summary.TotalChecks,
		report.Summary.PassedChecks,
		report.Summary.FailedChecks,
		report.Summary.WarningChecks,
	)

	// 添加检查结果行
	for _, finding := range report.Findings {
		html += fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td class="%s">未通过</td>
                <td>%s</td>
                <td>%s</td>
            </tr>`,
			finding.RuleID,
			"fail",
			finding.Title,
			report.GeneratedAt.Format("2006-01-02 15:04:05"),
		)
	}

	html += `
        </table>
    </div>
    
    <div class="section">
        <h2>发现项</h2>`

	for _, finding := range report.Findings {
		html += fmt.Sprintf(`
        <div class="finding">
            <strong>%s</strong> - %s<br>
            <small>规则: %s | 严重程度: %s</small>
        </div>`,
			finding.Title,
			finding.Description,
			finding.RuleID,
			finding.Severity,
		)
	}

	html += `
    </div>
    
    <div class="section">
        <h2>修复建议</h2>`

	for _, rec := range report.Recommendations {
		html += fmt.Sprintf(`
        <div class="recommendation">
            <strong>%s</strong> - %s<br>
            <small>优先级: %s | 工作量: %s</small><br>
            %s
        </div>`,
			rec.Title,
			rec.Description,
			rec.Priority,
			rec.Effort,
			rec.Action,
		)
	}

	html += `
    </div>
</body>
</html>`

	return []byte(html), nil
}

// generatePDF 生成PDF格式报告
func (r *Reporter) generatePDF(report *AuditReport) ([]byte, error) {
	// 简化版本：生成HTML后转换为PDF
	// 实际应使用 wkhtmltopdf 或其他 PDF 生成库
	html, err := r.generateHTML(report)
	if err != nil {
		return nil, err
	}

	// 这里简化处理，实际应调用PDF生成库
	return html, nil
}

// getScoreColor 获取分数对应的颜色
func (r *Reporter) getScoreColor(score float64) string {
	switch {
	case score >= 90:
		return "green"
	case score >= 70:
		return "orange"
	default:
		return "red"
	}
}
