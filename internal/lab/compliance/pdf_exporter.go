// Package compliancereport 提供 PDF 报告导出功能
package compliance

import (
	"fmt"
	"strings"
	"time"
)

// PDFExporter PDF 报告导出器.
type PDFExporter struct {
	standards *StandardsManager
}

// NewPDFExporter 创建 PDF 报告导出器.
func NewPDFExporter(standards *StandardsManager) *PDFExporter {
	return &PDFExporter{standards: standards}
}

// ExportToText 导出报告为纯文本格式（PDF 前端渲染用）.
func (e *PDFExporter) ExportToText(report *ComplianceReport) string {
	var b strings.Builder

	stdName := string(report.Standard)
	if info, ok := e.standards.GetStandard(report.Standard); ok {
		stdName = info.Name
	}

	// 标题
	b.WriteString("══════════════════════════════════════════════════\n")
	b.WriteString("           NAS-OS 合规检查报告\n")
	b.WriteString("══════════════════════════════════════════════════\n\n")

	// 基本信息
	fmt.Fprintf(&b, "报告 ID:     %s\n", report.ID)
	fmt.Fprintf(&b, "合规标准:    %s\n", stdName)
	fmt.Fprintf(&b, "生成时间:    %s\n", report.CreatedAt.Format("2006-01-02 15:04:05"))
	if report.CompletedAt != nil {
		fmt.Fprintf(&b, "完成时间:    %s\n", report.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	b.WriteString("\n")

	// 合规评分
	b.WriteString("──────────────────────────────────────────────────\n")
	b.WriteString("  合规评分\n")
	b.WriteString("──────────────────────────────────────────────────\n")
	fmt.Fprintf(&b, "  得分:       %d / 100\n", report.Score)
	fmt.Fprintf(&b, "  状态:       %s\n", e.formatStatus(report.ComplianceStatus))
	fmt.Fprintf(&b, "  总检查项:   %d\n", report.TotalChecks)
	fmt.Fprintf(&b, "  通过:       %d\n", report.Passed)
	fmt.Fprintf(&b, "  失败:       %d\n", report.Failed)
	fmt.Fprintf(&b, "  警告:       %d\n", report.Warnings)
	fmt.Fprintf(&b, "  跳过:       %d\n", report.Skipped)
	b.WriteString("\n")

	// 扫描结果
	b.WriteString("──────────────────────────────────────────────────\n")
	b.WriteString("  扫描结果详情\n")
	b.WriteString("──────────────────────────────────────────────────\n")
	for _, r := range report.Results {
		statusIcon := "✓"
		switch r.Status {
		case CheckItemFail:
			statusIcon = "✗"
		case CheckItemWarning:
			statusIcon = "⚠"
		}
		fmt.Fprintf(&b, "  %s [%s] %s\n", statusIcon, r.Category, r.Name)
		fmt.Fprintf(&b, "    %s\n", r.Message)
		if r.Details != "" {
			fmt.Fprintf(&b, "    详情: %s\n", r.Details)
		}
		b.WriteString("\n")
	}

	// 整改建议
	if len(report.Remediations) > 0 {
		b.WriteString("──────────────────────────────────────────────────\n")
		b.WriteString("  整改建议\n")
		b.WriteString("──────────────────────────────────────────────────\n")
		for i, rem := range report.Remediations {
			fmt.Fprintf(&b, "\n  %d. [%s] %s\n", i+1, rem.Priority, rem.Title)
			fmt.Fprintf(&b, "     %s\n", rem.Description)
			b.WriteString("     处置步骤:\n")
			for _, step := range rem.Steps {
				fmt.Fprintf(&b, "       %s\n", step)
			}
		}
		b.WriteString("\n")
	}

	// 摘要
	b.WriteString("──────────────────────────────────────────────────\n")
	b.WriteString("  摘要\n")
	b.WriteString("──────────────────────────────────────────────────\n")
	fmt.Fprintf(&b, "  %s\n", report.Summary)
	b.WriteString("\n══════════════════════════════════════════════════\n")
	fmt.Fprintf(&b, "  报告由 NAS-OS 合规引擎自动生成 | %s\n", time.Now().Format("2006-01-02 15:04:05"))
	b.WriteString("══════════════════════════════════════════════════\n")

	return b.String()
}

// ExportToHTML 导出报告为 HTML 格式（用于 PDF 渲染）.
func (e *PDFExporter) ExportToHTML(report *ComplianceReport) string {
	stdName := string(report.Standard)
	if info, ok := e.standards.GetStandard(report.Standard); ok {
		stdName = info.Name
	}

	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>NAS-OS 合规检查报告</title>
<style>
  body { font-family: "Microsoft YaHei", "Helvetica Neue", Arial, sans-serif; margin: 40px; color: #333; }
  h1 { text-align: center; color: #1a237e; border-bottom: 3px solid #1a237e; padding-bottom: 10px; }
  .meta { margin: 20px 0; }
  .meta table { width: 100%; border-collapse: collapse; }
  .meta td { padding: 8px 12px; border-bottom: 1px solid #eee; }
  .meta td:first-child { font-weight: bold; width: 120px; color: #555; }
  .score-box { text-align: center; padding: 20px; margin: 20px 0; border-radius: 8px; }
  .score-compliant { background: #e8f5e9; border: 2px solid #4caf50; }
  .score-pending { background: #fff3e0; border: 2px solid #ff9800; }
  .score-noncompliant { background: #ffebee; border: 2px solid #f44336; }
  .score-value { font-size: 48px; font-weight: bold; }
  .stats { display: flex; justify-content: space-around; margin: 20px 0; }
  .stat-item { text-align: center; padding: 10px; }
  .stat-value { font-size: 24px; font-weight: bold; }
  .stat-label { color: #666; font-size: 14px; }
  table.results { width: 100%; border-collapse: collapse; margin: 20px 0; }
  table.results th { background: #f5f5f5; padding: 10px; text-align: left; border-bottom: 2px solid #ddd; }
  table.results td { padding: 10px; border-bottom: 1px solid #eee; }
  .status-pass { color: #4caf50; font-weight: bold; }
  .status-fail { color: #f44336; font-weight: bold; }
  .status-warning { color: #ff9800; font-weight: bold; }
  .remediation { background: #fafafa; padding: 15px; margin: 10px 0; border-left: 4px solid #ff9800; border-radius: 4px; }
  .remediation h4 { margin: 0 0 8px; color: #e65100; }
  .remediation ol { margin: 5px 0; padding-left: 20px; }
  .footer { text-align: center; margin-top: 30px; padding-top: 15px; border-top: 1px solid #ddd; color: #999; font-size: 12px; }
</style>
</head>
<body>
<h1>🛡️ NAS-OS 合规检查报告</h1>
`)

	// 元信息
	b.WriteString(`<div class="meta"><table>
`)
	fmt.Fprintf(&b, "<tr><td>报告 ID</td><td>%s</td></tr>\n", report.ID)
	fmt.Fprintf(&b, "<tr><td>合规标准</td><td>%s</td></tr>\n", stdName)
	fmt.Fprintf(&b, "<tr><td>生成时间</td><td>%s</td></tr>\n", report.CreatedAt.Format("2006-01-02 15:04:05"))
	b.WriteString("</table></div>\n")

	// 评分
	scoreClass := "score-compliant"
	switch report.ComplianceStatus {
	case StatusNonCompliant:
		scoreClass = "score-noncompliant"
	case StatusPendingReview:
		scoreClass = "score-pending"
	}
	fmt.Fprintf(&b, `<div class="score-box %s">
  <div class="score-value">%d</div>
  <div>合规得分 (满分 100) | 状态: %s</div>
</div>
`, scoreClass, report.Score, e.formatStatus(report.ComplianceStatus))

	// 统计
	b.WriteString(`<div class="stats">
`)
	fmt.Fprintf(&b, `<div class="stat-item"><div class="stat-value">%d</div><div class="stat-label">总检查项</div></div>`, report.TotalChecks)
	fmt.Fprintf(&b, `<div class="stat-item"><div class="stat-value" style="color:#4caf50">%d</div><div class="stat-label">通过</div></div>`, report.Passed)
	fmt.Fprintf(&b, `<div class="stat-item"><div class="stat-value" style="color:#f44336">%d</div><div class="stat-label">失败</div></div>`, report.Failed)
	fmt.Fprintf(&b, `<div class="stat-item"><div class="stat-value" style="color:#ff9800">%d</div><div class="stat-label">警告</div></div>`, report.Warnings)
	b.WriteString("</div>\n")

	// 详细结果表格
	b.WriteString(`<h2>扫描结果详情</h2>
<table class="results">
<tr><th>状态</th><th>类别</th><th>检查项</th><th>严重程度</th><th>说明</th></tr>
`)
	for _, r := range report.Results {
		statusClass := "status-pass"
		statusText := "通过"
		switch r.Status {
		case CheckItemFail:
			statusClass = "status-fail"
			statusText = "失败"
		case CheckItemWarning:
			statusClass = "status-warning"
			statusText = "警告"
		}
		fmt.Fprintf(&b, "<tr><td class=\"%s\">%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			statusClass, statusText, FormatCategoryName(r.Category), r.Name, r.Severity, r.Message)
	}
	b.WriteString("</table>\n")

	// 整改建议
	if len(report.Remediations) > 0 {
		b.WriteString("<h2>整改建议</h2>\n")
		for _, rem := range report.Remediations {
			fmt.Fprintf(&b, `<div class="remediation">
<h4>[%s] %s</h4>
<p>%s</p>
<ol>
`, rem.Priority, rem.Title, rem.Description)
			for _, step := range rem.Steps {
				fmt.Fprintf(&b, "<li>%s</li>\n", step)
			}
			b.WriteString("</ol></div>\n")
		}
	}

	// 页脚
	fmt.Fprintf(&b, `<div class="footer">
<p>报告由 NAS-OS 合规引擎自动生成 | %s</p>
</div>
</body>
</html>`, time.Now().Format("2006-01-02 15:04:05"))

	return b.String()
}

// formatStatus 格式化合规状态.
func (e *PDFExporter) formatStatus(status ComplianceStatus) string {
	switch status {
	case StatusCompliant:
		return "✅ 合规"
	case StatusNonCompliant:
		return "❌ 不合规"
	case StatusPendingReview:
		return "⏳ 待审查"
	default:
		return string(status)
	}
}
