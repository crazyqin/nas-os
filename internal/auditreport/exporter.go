// Package auditreport 提供报告导出功能
package auditreport

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ExportFormat 导出格式.
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatPDF  ExportFormat = "pdf"
	FormatCSV  ExportFormat = "csv"
	FormatHTML ExportFormat = "html"
)

// ExportRequest 导出请求.
type ExportRequest struct {
	ReportID string       `json:"report_id" binding:"required"`
	Format   ExportFormat `json:"format" binding:"required"`
}

// ExportResult 导出结果.
type ExportResult struct {
	Format    ExportFormat `json:"format"`
	Content   string       `json:"content"`
	Size      int          `json:"size"`
	CreatedAt time.Time    `json:"created_at"`
}

// Exporter 报告导出器.
type Exporter struct {
	manager *Manager
}

// NewExporter 创建导出器.
func NewExporter(manager *Manager) *Exporter {
	return &Exporter{
		manager: manager,
	}
}

// ExportReport 导出报告.
func (e *Exporter) ExportReport(req ExportRequest) (*ExportResult, error) {
	report, err := e.manager.GetReport(req.ReportID)
	if err != nil {
		return nil, err
	}

	switch req.Format {
	case FormatJSON:
		return e.exportJSON(report)
	case FormatPDF:
		return e.exportPDF(report)
	case FormatHTML:
		return e.exportHTML(report)
	case FormatCSV:
		return e.exportCSV(report)
	default:
		return nil, fmt.Errorf("unsupported format: %s", req.Format)
	}
}

// ExportComplianceReport 导出合规报告.
func (e *Exporter) ExportComplianceReport(report *ComplianceReport, format ExportFormat) (*ExportResult, error) {
	switch format {
	case FormatJSON:
		return e.exportComplianceJSON(report)
	case FormatPDF:
		return e.exportCompliancePDF(report)
	case FormatHTML:
		return e.exportComplianceHTML(report)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// ExportRiskReport 导出风险报告.
func (e *Exporter) ExportRiskReport(results []*RiskScoreResult, format ExportFormat) (*ExportResult, error) {
	switch format {
	case FormatJSON:
		return e.exportRiskJSON(results)
	case FormatPDF:
		return e.exportRiskPDF(results)
	case FormatHTML:
		return e.exportRiskHTML(results)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// ========== JSON 导出 ==========

func (e *Exporter) exportJSON(report *AuditReport) (*ExportResult, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report: %w", err)
	}

	return &ExportResult{
		Format:    FormatJSON,
		Content:   string(data),
		Size:      len(data),
		CreatedAt: time.Now(),
	}, nil
}

func (e *Exporter) exportComplianceJSON(report *ComplianceReport) (*ExportResult, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compliance report: %w", err)
	}

	return &ExportResult{
		Format:    FormatJSON,
		Content:   string(data),
		Size:      len(data),
		CreatedAt: time.Now(),
	}, nil
}

func (e *Exporter) exportRiskJSON(results []*RiskScoreResult) (*ExportResult, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal risk results: %w", err)
	}

	return &ExportResult{
		Format:    FormatJSON,
		Content:   string(data),
		Size:      len(data),
		CreatedAt: time.Now(),
	}, nil
}

// ========== PDF 导出 ==========

func (e *Exporter) exportPDF(report *AuditReport) (*ExportResult, error) {
	// PDF 导出需要生成 PDF 格式的内容
	// 简化处理：生成一个 PDF 风格的文本表示
	content := e.generatePDFContent(report)

	return &ExportResult{
		Format:    FormatPDF,
		Content:   content,
		Size:      len(content),
		CreatedAt: time.Now(),
	}, nil
}

func (e *Exporter) exportCompliancePDF(report *ComplianceReport) (*ExportResult, error) {
	content := e.generateCompliancePDFContent(report)

	return &ExportResult{
		Format:    FormatPDF,
		Content:   content,
		Size:      len(content),
		CreatedAt: time.Now(),
	}, nil
}

func (e *Exporter) exportRiskPDF(results []*RiskScoreResult) (*ExportResult, error) {
	content := e.generateRiskPDFContent(results)

	return &ExportResult{
		Format:    FormatPDF,
		Content:   content,
		Size:      len(content),
		CreatedAt: time.Now(),
	}, nil
}

// ========== HTML 导出 ==========

func (e *Exporter) exportHTML(report *AuditReport) (*ExportResult, error) {
	content := e.generateHTMLContent(report)

	return &ExportResult{
		Format:    FormatHTML,
		Content:   content,
		Size:      len(content),
		CreatedAt: time.Now(),
	}, nil
}

func (e *Exporter) exportComplianceHTML(report *ComplianceReport) (*ExportResult, error) {
	content := e.generateComplianceHTMLContent(report)

	return &ExportResult{
		Format:    FormatHTML,
		Content:   content,
		Size:      len(content),
		CreatedAt: time.Now(),
	}, nil
}

func (e *Exporter) exportRiskHTML(results []*RiskScoreResult) (*ExportResult, error) {
	content := e.generateRiskHTMLContent(results)

	return &ExportResult{
		Format:    FormatHTML,
		Content:   content,
		Size:      len(content),
		CreatedAt: time.Now(),
	}, nil
}

// ========== CSV 导出 ==========

func (e *Exporter) exportCSV(report *AuditReport) (*ExportResult, error) {
	var sb strings.Builder

	// CSV 头
	sb.WriteString("ID,Severity,Category,Description,Status\n")

	// 数据行
	for _, f := range report.Findings {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,\"%s\",%s\n",
			f.ID, f.Severity, f.Category, f.Description, f.Status))
	}

	content := sb.String()
	return &ExportResult{
		Format:    FormatCSV,
		Content:   content,
		Size:      len(content),
		CreatedAt: time.Now(),
	}, nil
}

// ========== 内容生成方法 ==========

func (e *Exporter) generatePDFContent(report *AuditReport) string {
	var sb strings.Builder

	sb.WriteString("========================================\n")
	sb.WriteString("       安全审计报告 (PDF 格式)\n")
	sb.WriteString("========================================\n\n")

	sb.WriteString(fmt.Sprintf("报告标题: %s\n", report.Title))
	sb.WriteString(fmt.Sprintf("报告期间: %s\n", report.Period))
	sb.WriteString(fmt.Sprintf("生成时间: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("安全评分: %.1f/100\n\n", report.Score))

	sb.WriteString("报告摘要:\n")
	sb.WriteString(report.Summary + "\n\n")

	sb.WriteString("发现列表:\n")
	sb.WriteString("----------------------------------------\n")
	for i, f := range report.Findings {
		sb.WriteString(fmt.Sprintf("\n%d. [%s] %s\n", i+1, f.Severity, f.Category))
		sb.WriteString(fmt.Sprintf("   描述: %s\n", f.Description))
		sb.WriteString(fmt.Sprintf("   状态: %s\n", f.Status))
		if f.Recommendation != "" {
			sb.WriteString(fmt.Sprintf("   建议: %s\n", f.Recommendation))
		}
	}

	sb.WriteString("\n========================================\n")
	sb.WriteString("               报告结束\n")
	sb.WriteString("========================================\n")

	return sb.String()
}

func (e *Exporter) generateCompliancePDFContent(report *ComplianceReport) string {
	var sb strings.Builder

	sb.WriteString("========================================\n")
	sb.WriteString("       合规审计报告 (PDF 格式)\n")
	sb.WriteString("========================================\n\n")

	sb.WriteString(fmt.Sprintf("合规标准: %s\n", report.Standard))
	sb.WriteString(fmt.Sprintf("生成时间: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("合规评分: %.1f%%\n", report.Score))
	sb.WriteString(fmt.Sprintf("通过项目: %d/%d\n\n", report.Passed, report.Total))

	sb.WriteString("各章节详情:\n")
	sb.WriteString("----------------------------------------\n")
	for _, section := range report.Sections {
		sb.WriteString(fmt.Sprintf("\n章节: %s\n", section.Title))
		sb.WriteString(fmt.Sprintf("评分: %.1f%%\n", section.Score))
		for _, item := range section.Items {
			status := "✓"
			if item.Status == "failed" {
				status = "✗"
			}
			sb.WriteString(fmt.Sprintf("  %s %s: %s\n", status, item.Title, item.Detail))
		}
	}

	sb.WriteString("\n========================================\n")
	sb.WriteString("               报告结束\n")
	sb.WriteString("========================================\n")

	return sb.String()
}

func (e *Exporter) generateRiskPDFContent(results []*RiskScoreResult) string {
	var sb strings.Builder

	sb.WriteString("========================================\n")
	sb.WriteString("       风险评估报告 (PDF 格式)\n")
	sb.WriteString("========================================\n\n")

	sb.WriteString(fmt.Sprintf("生成时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. 用户: %s\n", i+1, r.UserID))
		sb.WriteString(fmt.Sprintf("   风险评分: %.1f\n", r.OverallScore))
		sb.WriteString(fmt.Sprintf("   风险等级: %s\n", r.RiskLevel))

		if len(r.TopRisks) > 0 {
			sb.WriteString("   主要风险:\n")
			for _, risk := range r.TopRisks {
				sb.WriteString(fmt.Sprintf("     - %s (%.1f分)\n", risk.Description, risk.Score))
			}
		}

		if len(r.Recommendations) > 0 {
			sb.WriteString("   建议:\n")
			for _, rec := range r.Recommendations {
				sb.WriteString(fmt.Sprintf("     - %s\n", rec))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("========================================\n")
	sb.WriteString("               报告结束\n")
	sb.WriteString("========================================\n")

	return sb.String()
}

func (e *Exporter) generateHTMLContent(report *AuditReport) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	sb.WriteString("<meta charset=\"UTF-8\">\n")
	sb.WriteString("<title>安全审计报告</title>\n")
	sb.WriteString("<style>\n")
	sb.WriteString("body { font-family: Arial, sans-serif; margin: 40px; }\n")
	sb.WriteString("h1 { color: #333; }\n")
	sb.WriteString(".score { font-size: 24px; color: #007bff; margin: 20px 0; }\n")
	sb.WriteString(".finding { border: 1px solid #ddd; padding: 15px; margin: 10px 0; border-radius: 5px; }\n")
	sb.WriteString(".critical { border-left: 5px solid #dc3545; }\n")
	sb.WriteString(".high { border-left: 5px solid #fd7e14; }\n")
	sb.WriteString(".medium { border-left: 5px solid #ffc107; }\n")
	sb.WriteString(".low { border-left: 5px solid #28a745; }\n")
	sb.WriteString("</style>\n</head>\n<body>\n")

	sb.WriteString("<h1>安全审计报告</h1>\n")
	sb.WriteString(fmt.Sprintf("<p><strong>报告标题:</strong> %s</p>\n", report.Title))
	sb.WriteString(fmt.Sprintf("<p><strong>报告期间:</strong> %s</p>\n", report.Period))
	sb.WriteString(fmt.Sprintf("<p><strong>生成时间:</strong> %s</p>\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("<div class=\"score\">安全评分: %.1f/100</div>\n", report.Score))

	sb.WriteString("<h2>报告摘要</h2>\n")
	sb.WriteString(fmt.Sprintf("<p>%s</p>\n", report.Summary))

	sb.WriteString("<h2>发现列表</h2>\n")
	for _, f := range report.Findings {
		sb.WriteString(fmt.Sprintf("<div class=\"finding %s\">\n", f.Severity))
		sb.WriteString(fmt.Sprintf("<strong>%s</strong> - %s<br>\n", f.Category, f.Severity))
		sb.WriteString(fmt.Sprintf("<p>%s</p>\n", f.Description))
		sb.WriteString(fmt.Sprintf("<p><em>状态: %s</em></p>\n", f.Status))
		sb.WriteString("</div>\n")
	}

	sb.WriteString("</body>\n</html>")

	return sb.String()
}

func (e *Exporter) generateComplianceHTMLContent(report *ComplianceReport) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	sb.WriteString("<meta charset=\"UTF-8\">\n")
	sb.WriteString("<title>合规审计报告</title>\n")
	sb.WriteString("<style>\n")
	sb.WriteString("body { font-family: Arial, sans-serif; margin: 40px; }\n")
	sb.WriteString("h1 { color: #333; }\n")
	sb.WriteString(".score { font-size: 24px; color: #28a745; margin: 20px 0; }\n")
	sb.WriteString("table { width: 100%; border-collapse: collapse; margin: 20px 0; }\n")
	sb.WriteString("th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }\n")
	sb.WriteString("th { background-color: #f2f2f2; }\n")
	sb.WriteString(".passed { color: #28a745; }\n")
	sb.WriteString(".failed { color: #dc3545; }\n")
	sb.WriteString("</style>\n</head>\n<body>\n")

	sb.WriteString("<h1>合规审计报告</h1>\n")
	sb.WriteString(fmt.Sprintf("<p><strong>合规标准:</strong> %s</p>\n", report.Standard))
	sb.WriteString(fmt.Sprintf("<p><strong>生成时间:</strong> %s</p>\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("<div class=\"score\">合规评分: %.1f%%</div>\n", report.Score))
	sb.WriteString(fmt.Sprintf("<p>通过: %d / 总计: %d</p>\n", report.Passed, report.Total))

	sb.WriteString("<h2>各章节详情</h2>\n")
	for _, section := range report.Sections {
		sb.WriteString(fmt.Sprintf("<h3>%s (%.1f%%)</h3>\n", section.Title, section.Score))
		sb.WriteString("<table>\n")
		sb.WriteString("<tr><th>项目</th><th>状态</th><th>详情</th></tr>\n")
		for _, item := range section.Items {
			statusClass := "passed"
			if item.Status == "failed" {
				statusClass = "failed"
			}
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td class=\"%s\">%s</td><td>%s</td></tr>\n",
				item.Title, statusClass, item.Status, item.Detail))
		}
		sb.WriteString("</table>\n")
	}

	sb.WriteString("</body>\n</html>")

	return sb.String()
}

func (e *Exporter) generateRiskHTMLContent(results []*RiskScoreResult) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	sb.WriteString("<meta charset=\"UTF-8\">\n")
	sb.WriteString("<title>风险评估报告</title>\n")
	sb.WriteString("<style>\n")
	sb.WriteString("body { font-family: Arial, sans-serif; margin: 40px; }\n")
	sb.WriteString("h1 { color: #333; }\n")
	sb.WriteString(".user-card { border: 1px solid #ddd; padding: 20px; margin: 15px 0; border-radius: 8px; }\n")
	sb.WriteString(".risk-critical { border-left: 5px solid #dc3545; }\n")
	sb.WriteString(".risk-high { border-left: 5px solid #fd7e14; }\n")
	sb.WriteString(".risk-medium { border-left: 5px solid #ffc107; }\n")
	sb.WriteString(".risk-low { border-left: 5px solid #28a745; }\n")
	sb.WriteString(".risk-safe { border-left: 5px solid #6c757d; }\n")
	sb.WriteString("</style>\n</head>\n<body>\n")

	sb.WriteString("<h1>风险评估报告</h1>\n")
	sb.WriteString(fmt.Sprintf("<p>生成时间: %s</p>\n", time.Now().Format("2006-01-02 15:04:05")))

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("<div class=\"user-card risk-%s\">\n", r.RiskLevel))
		sb.WriteString(fmt.Sprintf("<h3>用户: %s</h3>\n", r.UserID))
		sb.WriteString(fmt.Sprintf("<p><strong>风险评分:</strong> %.1f</p>\n", r.OverallScore))
		sb.WriteString(fmt.Sprintf("<p><strong>风险等级:</strong> %s</p>\n", r.RiskLevel))

		if len(r.TopRisks) > 0 {
			sb.WriteString("<p><strong>主要风险:</strong></p><ul>\n")
			for _, risk := range r.TopRisks {
				sb.WriteString(fmt.Sprintf("<li>%s (%.1f分)</li>\n", risk.Description, risk.Score))
			}
			sb.WriteString("</ul>\n")
		}

		if len(r.Recommendations) > 0 {
			sb.WriteString("<p><strong>建议:</strong></p><ul>\n")
			for _, rec := range r.Recommendations {
				sb.WriteString(fmt.Sprintf("<li>%s</li>\n", rec))
			}
			sb.WriteString("</ul>\n")
		}

		sb.WriteString("</div>\n")
	}

	sb.WriteString("</body>\n</html>")

	return sb.String()
}
