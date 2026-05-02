package licensescan

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"
)

// ReportGenerator 报告生成器.
type ReportGenerator struct{}

// NewReportGenerator 创建报告生成器.
func NewReportGenerator() *ReportGenerator {
	return &ReportGenerator{}
}

// GenerateJSON 生成JSON格式报告.
func (rg *ReportGenerator) GenerateJSON(report *Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// GenerateHTML 生成HTML格式报告.
func (rg *ReportGenerator) GenerateHTML(report *Report) ([]byte, error) {
	var buf strings.Builder
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"severityClass": func(s Severity) string {
			switch s {
			case SeverityCritical:
				return "severity-critical"
			case SeverityHigh:
				return "severity-high"
			case SeverityMedium:
				return "severity-medium"
			case SeverityLow:
				return "severity-low"
			default:
				return ""
			}
		},
		"complianceClass": func(c Compliance) string {
			switch c {
			case ComplianceAllowed:
				return "compliance-allowed"
			case ComplianceDenied:
				return "compliance-denied"
			case ComplianceReview:
				return "compliance-review"
			default:
				return "compliance-unknown"
			}
		},
		"complianceText": func(c Compliance) string {
			switch c {
			case ComplianceAllowed:
				return "允许"
			case ComplianceDenied:
				return "禁止"
			case ComplianceReview:
				return "需审批"
			default:
				return "未知"
			}
		},
		"categoryText": func(c Category) string {
			switch c {
			case CategoryPermissive:
				return "宽松"
			case CategoryWeakCopyleft:
				return "弱传染"
			case CategoryStrongCopyleft:
				return "强传染"
			case CategoryCustom:
				return "自定义"
			default:
				return "未知"
			}
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.Format("2006-01-02 15:04:05")
		},
	}).Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析HTML模板失败: %w", err)
	}

	if err := tmpl.Execute(&buf, report); err != nil {
		return nil, fmt.Errorf("渲染HTML报告失败: %w", err)
	}

	return []byte(buf.String()), nil
}

// htmlTemplate HTML报告模板.
const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - 许可证合规报告</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; padding: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { font-size: 24px; margin-bottom: 8px; }
        .meta { color: #666; margin-bottom: 24px; font-size: 14px; }
        .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; margin-bottom: 24px; }
        .summary-card { background: #fff; border-radius: 8px; padding: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); text-align: center; }
        .summary-card .number { font-size: 32px; font-weight: bold; }
        .summary-card .label { color: #666; font-size: 14px; margin-top: 4px; }
        .compliant .number { color: #22c55e; }
        .non-compliant .number { color: #ef4444; }
        .review .number { color: #f59e0b; }
        .total .number { color: #3b82f6; }
        .violations .number { color: #ef4444; }
        table { width: 100%; border-collapse: collapse; background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); margin-bottom: 24px; }
        th, td { padding: 12px 16px; text-align: left; border-bottom: 1px solid #e5e7eb; }
        th { background: #f8fafc; font-weight: 600; font-size: 14px; color: #475569; }
        tr:hover { background: #f8fafc; }
        .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 12px; font-weight: 500; }
        .compliance-allowed { background: #dcfce7; color: #166534; }
        .compliance-denied { background: #fee2e2; color: #991b1b; }
        .compliance-review { background: #fef3c7; color: #92400e; }
        .compliance-unknown { background: #e5e7eb; color: #374151; }
        .severity-critical { background: #dc2626; color: #fff; }
        .severity-high { background: #ef4444; color: #fff; }
        .severity-medium { background: #f59e0b; color: #fff; }
        .severity-low { background: #3b82f6; color: #fff; }
        .section { margin-bottom: 24px; }
        .section h2 { font-size: 18px; margin-bottom: 12px; padding-bottom: 8px; border-bottom: 2px solid #e5e7eb; }
        .no-data { text-align: center; color: #9ca3af; padding: 40px; }
        footer { text-align: center; color: #9ca3af; font-size: 12px; margin-top: 40px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>{{.Title}}</h1>
        <div class="meta">
            <span>报告ID: {{.ID}}</span> | 
            <span>生成时间: {{formatTime .GeneratedAt}}</span> | 
            <span>扫描数量: {{.Summary.TotalScans}}</span>
        </div>

        <div class="summary-grid">
            <div class="summary-card total">
                <div class="number">{{.Summary.TotalLicenses}}</div>
                <div class="label">总许可证数</div>
            </div>
            <div class="summary-card compliant">
                <div class="number">{{.Summary.Compliant}}</div>
                <div class="label">合规</div>
            </div>
            <div class="summary-card non-compliant">
                <div class="number">{{.Summary.NonCompliant}}</div>
                <div class="label">不合规</div>
            </div>
            <div class="summary-card review">
                <div class="number">{{.Summary.NeedsReview}}</div>
                <div class="label">需审查</div>
            </div>
            <div class="summary-card violations">
                <div class="number">{{.Summary.TotalViolations}}</div>
                <div class="label">违规项</div>
            </div>
        </div>

        {{range .Results}}
        <div class="section">
            <h2>扫描: {{.Target}} ({{if eq .ScanType "docker"}}Docker镜像{{else if eq .ScanType "go_mod"}}Go模块{{else}}全量{{end}})</h2>
            <p style="color:#666; margin-bottom:12px;">
                状态: {{.Status}} | 开始: {{formatTime .StartedAt}} | 完成: {{formatTime .FinishedAt}}
                {{if .Error}} | 错误: <span style="color:#ef4444;">{{.Error}}</span>{{end}}
            </p>
            
            {{if .Licenses}}
            <table>
                <thead>
                    <tr>
                        <th>许可证</th>
                        <th>类别</th>
                        <th>合规状态</th>
                        <th>来源</th>
                        <th>版本</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Licenses}}
                    <tr>
                        <td>{{.Name}}</td>
                        <td>{{categoryText .Category}}</td>
                        <td><span class="badge {{complianceClass .Compliance}}">{{complianceText .Compliance}}</span></td>
                        <td>{{.Source}}</td>
                        <td>{{.Version}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <div class="no-data">未发现许可证信息</div>
            {{end}}

            {{if .Violations}}
            <h3 style="font-size:16px; margin: 16px 0 8px;">违规项</h3>
            <table>
                <thead>
                    <tr>
                        <th>许可证</th>
                        <th>严重程度</th>
                        <th>来源</th>
                        <th>描述</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Violations}}
                    <tr>
                        <td>{{.LicenseName}}</td>
                        <td><span class="badge {{severityClass .Severity}}">{{.Severity}}</span></td>
                        <td>{{.Source}}</td>
                        <td>{{.Message}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{end}}
        </div>
        {{end}}

        <footer>
            <p>NAS-OS 许可证合规扫描系统 · 报告生成于 {{formatTime .GeneratedAt}}</p>
        </footer>
    </div>
</body>
</html>`
