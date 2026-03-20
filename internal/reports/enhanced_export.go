// Package reports 提供报表生成和管理功能
package reports

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ========== 增强导出功能 v2.56.0 ==========

// EnhancedExporter 增强的报表导出器
type EnhancedExporter struct {
	dataDir       string
	templateCache map[string]*template.Template
	pdfConverter  PDFConverter
	csvEnhancer   *CSVExporterEnhanced
	excelExporter *ExcelExporter
}

// PDFConverter PDF 转换器接口
type PDFConverter interface {
	Convert(htmlPath, pdfPath string) error
	IsAvailable() bool
}

// NewEnhancedExporter 创建增强导出器
func NewEnhancedExporter(dataDir string) *EnhancedExporter {
	e := &EnhancedExporter{
		dataDir:       dataDir,
		templateCache: make(map[string]*template.Template),
	}

	// 初始化 CSV 增强导出器
	e.csvEnhancer = NewCSVExporterEnhanced()

	// 初始化 Excel 导出器
	e.excelExporter = NewExcelExporter(dataDir)

	// 初始化 PDF 转换器
	e.pdfConverter = NewWKHTMLToPDFConverter()

	// 确保输出目录存在
	_ = os.MkdirAll(filepath.Join(dataDir, "outputs"), 0755)

	return e
}

// ExportEnhanced 增强导出
func (e *EnhancedExporter) ExportEnhanced(report *GeneratedReport, format ExportFormat, outputPath string, options EnhancedExportOptions) (*ExportResult, error) {
	if outputPath == "" {
		outputPath = e.generateOutputPath(format)
	}

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var err error
	switch format {
	case ExportJSON:
		err = e.exportJSONEnhanced(report, outputPath, options)
	case ExportCSV:
		err = e.exportCSVEnhanced(report, outputPath, options)
	case ExportHTML:
		err = e.exportHTMLEnhanced(report, outputPath, options)
	case ExportPDF:
		err = e.exportPDFEnhanced(report, outputPath, options)
	case ExportExcel:
		err = e.exportExcelEnhanced(report, outputPath, options)
	default:
		err = fmt.Errorf("不支持的导出格式: %s", format)
	}

	if err != nil {
		return nil, err
	}

	// 获取文件信息
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, err
	}

	return &ExportResult{
		Format:    format,
		Filename:  filepath.Base(outputPath),
		Path:      outputPath,
		Size:      info.Size(),
		MimeType:  e.getMimeType(format),
		CreatedAt: time.Now(),
	}, nil
}

// ========== JSON 增强 ==========

func (e *EnhancedExporter) exportJSONEnhanced(report *GeneratedReport, path string, options EnhancedExportOptions) error {
	output := map[string]interface{}{
		"id":            report.ID,
		"name":          report.Name,
		"generated_at":  report.GeneratedAt,
		"total_records": report.TotalRecords,
		"data":          report.Data,
	}

	if options.IncludeSummary && report.Summary != nil {
		output["summary"] = report.Summary
	}

	if options.IncludePeriod && !report.Period.StartTime.IsZero() {
		output["period"] = report.Period
	}

	if options.Title != "" {
		output["title"] = options.Title
	}

	// 增强元数据
	if options.IncludeMetadata {
		output["metadata"] = map[string]interface{}{
			"export_format": "json",
			"exported_at":   time.Now(),
			"version":       "2.56.0",
		}
	}

	// 格式化输出
	var data []byte
	if options.PrettyPrint {
		data, _ = json.MarshalIndent(output, "", "  ")
	} else {
		data, _ = json.Marshal(output)
	}

	return os.WriteFile(path, data, 0640)
}

// ========== CSV 增强 ==========

// CSVExporterEnhanced 增强的 CSV 导出器
type CSVExporterEnhanced struct{}

// NewCSVExporterEnhanced 创建增强 CSV 导出器
func NewCSVExporterEnhanced() *CSVExporterEnhanced {
	return &CSVExporterEnhanced{}
}

func (e *EnhancedExporter) exportCSVEnhanced(report *GeneratedReport, path string, options EnhancedExportOptions) error {
	if len(report.Data) == 0 {
		return os.WriteFile(path, []byte{}, 0640)
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 写入 BOM（UTF-8）以支持 Excel 打开
	if options.IncludeBOM {
		if _, err := buf.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return err
		}
	}

	// 写入标题注释
	if options.IncludeComments && options.Title != "" {
		if err := writer.Write([]string{"# " + options.Title}); err != nil {
			return err
		}
		if err := writer.Write([]string{"# 生成时间: " + report.GeneratedAt.Format("2006-01-02 15:04:05")}); err != nil {
			return err
		}
		if err := writer.Write([]string{"# 记录数: " + fmt.Sprintf("%d", report.TotalRecords)}); err != nil {
			return err
		}
		if err := writer.Write([]string{}); err != nil { // 空行
			return err
		}
	}

	// 获取列定义
	columns := options.Columns
	if len(columns) == 0 {
		// 自动从数据推断列
		columns = e.inferColumns(report.Data[0])
	}

	// 写入表头
	if options.IncludeHeader {
		headers := make([]string, 0, len(columns))
		for _, col := range columns {
			if col.Label != "" {
				headers = append(headers, col.Label)
			} else {
				headers = append(headers, col.Name)
			}
		}
		if err := writer.Write(headers); err != nil {
			return err
		}
	}

	// 写入数据
	for _, row := range report.Data {
		values := make([]string, 0, len(columns))
		for _, col := range columns {
			val := row[col.Name]
			values = append(values, e.formatCSVValue(val, col.Type))
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}

	// 写入汇总行
	if options.IncludeSummary && report.Summary != nil {
		if err := writer.Write([]string{}); err != nil { // 空行
			return err
		}
		if err := writer.Write([]string{"# 汇总信息"}); err != nil {
			return err
		}
		for key, value := range report.Summary {
			if err := writer.Write([]string{key, e.formatCSVValue(value, FieldTypeString)}); err != nil {
				return err
			}
		}
	}

	writer.Flush()
	return os.WriteFile(path, buf.Bytes(), 0640)
}

// inferColumns 从数据推断列定义
func (e *EnhancedExporter) inferColumns(row map[string]interface{}) []CSVColumn {
	columns := make([]CSVColumn, 0, len(row))
	for key, val := range row {
		col := CSVColumn{
			Name:  key,
			Label: key,
		}
		// 推断类型
		switch val.(type) {
		case float64:
			col.Type = FieldTypeNumber
		case int, int64, uint64:
			col.Type = FieldTypeNumber
		case bool:
			col.Type = FieldTypeBoolean
		case time.Time:
			col.Type = FieldTypeDateTime
		default:
			col.Type = FieldTypeString
		}
		columns = append(columns, col)
	}
	return columns
}

// formatCSVValue 格式化 CSV 值
func (e *EnhancedExporter) formatCSVValue(val interface{}, fieldType FieldType) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case float64:
		if fieldType == FieldTypePercent {
			return fmt.Sprintf("%.2f%%", v)
		}
		return fmt.Sprintf("%.2f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case uint64:
		if fieldType == FieldTypeBytes {
			return e.formatBytes(v)
		}
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "是"
		}
		return "否"
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatBytes 格式化字节数
func (e *EnhancedExporter) formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ========== HTML 增强 ==========

func (e *EnhancedExporter) exportHTMLEnhanced(report *GeneratedReport, path string, options EnhancedExportOptions) error {
	// 使用增强的 HTML 模板
	tmpl := e.getEnhancedHTMLTemplate(options)

	// 准备数据
	data := e.prepareHTMLData(report, options)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0640)
}

// getEnhancedHTMLTemplate 获取增强的 HTML 模板
func (e *EnhancedExporter) getEnhancedHTMLTemplate(options EnhancedExportOptions) *template.Template {
	// 根据主题选择模板（主题由模板数据中的 .Theme 字段决定）
	_ = options.Theme // 主题由模板数据传入

	tmplStr := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        :root {
            --primary-color: #667eea;
            --secondary-color: #764ba2;
            --success-color: #48bb78;
            --warning-color: #ed8936;
            --danger-color: #f56565;
            --bg-color: #f7fafc;
            --card-bg: #ffffff;
            --text-color: #2d3748;
            --text-muted: #718096;
            --border-color: #e2e8f0;
        }
        
        [data-theme="dark"] {
            --bg-color: #1a202c;
            --card-bg: #2d3748;
            --text-color: #f7fafc;
            --text-muted: #a0aec0;
            --border-color: #4a5568;
        }
        
        * { margin: 0; padding: 0; box-sizing: border-box; }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: var(--text-color);
            background: var(--bg-color);
            padding: 20px;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        
        .header {
            background: linear-gradient(135deg, var(--primary-color) 0%, var(--secondary-color) 100%);
            color: white;
            padding: 40px;
            border-radius: 12px;
            margin-bottom: 30px;
            box-shadow: 0 4px 20px rgba(102, 126, 234, 0.3);
        }
        
        .header h1 {
            font-size: 28px;
            margin-bottom: 10px;
            font-weight: 700;
        }
        
        .header .subtitle {
            opacity: 0.9;
            font-size: 16px;
            margin-bottom: 20px;
        }
        
        .meta {
            display: flex;
            gap: 30px;
            font-size: 14px;
            opacity: 0.9;
        }
        
        .meta-item {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        
        .content {
            display: grid;
            gap: 30px;
        }
        
        .card {
            background: var(--card-bg);
            border-radius: 12px;
            box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
            overflow: hidden;
        }
        
        .card-header {
            padding: 20px 25px;
            border-bottom: 1px solid var(--border-color);
            font-weight: 600;
            font-size: 16px;
        }
        
        .card-body {
            padding: 25px;
        }
        
        .summary-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
            gap: 20px;
        }
        
        .summary-item {
            background: var(--bg-color);
            padding: 20px;
            border-radius: 8px;
            text-align: center;
        }
        
        .summary-item .value {
            font-size: 28px;
            font-weight: 700;
            color: var(--primary-color);
            margin-bottom: 5px;
        }
        
        .summary-item .label {
            font-size: 13px;
            color: var(--text-muted);
        }
        
        .table-container {
            overflow-x: auto;
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
            font-size: 14px;
        }
        
        th, td {
            padding: 14px 16px;
            text-align: left;
            border-bottom: 1px solid var(--border-color);
        }
        
        th {
            background: var(--bg-color);
            font-weight: 600;
            color: var(--text-muted);
            text-transform: uppercase;
            font-size: 12px;
            letter-spacing: 0.5px;
            position: sticky;
            top: 0;
        }
        
        tr:hover {
            background: var(--bg-color);
        }
        
        .badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: 600;
        }
        
        .badge-success { background: #c6f6d5; color: #276749; }
        .badge-warning { background: #feebc8; color: #c05621; }
        .badge-danger { background: #fed7d7; color: #c53030; }
        
        .progress {
            height: 8px;
            background: var(--border-color);
            border-radius: 4px;
            overflow: hidden;
        }
        
        .progress-bar {
            height: 100%;
            border-radius: 4px;
            transition: width 0.3s ease;
        }
        
        .progress-bar.low { background: var(--success-color); }
        .progress-bar.medium { background: var(--warning-color); }
        .progress-bar.high { background: var(--danger-color); }
        
        .footer {
            text-align: center;
            padding: 30px;
            color: var(--text-muted);
            font-size: 13px;
            border-top: 1px solid var(--border-color);
            margin-top: 40px;
        }
        
        .chart-placeholder {
            background: var(--bg-color);
            border-radius: 8px;
            padding: 40px;
            text-align: center;
            color: var(--text-muted);
        }
        
        @media print {
            body { background: white; padding: 0; }
            .container { max-width: none; }
            .card { box-shadow: none; border: 1px solid var(--border-color); }
            .header { box-shadow: none; }
        }
        
        @media (max-width: 768px) {
            .header { padding: 20px; }
            .meta { flex-direction: column; gap: 10px; }
            .summary-grid { grid-template-columns: repeat(2, 1fr); }
        }
    </style>
</head>
<body data-theme="{{.Theme}}">
    <div class="container">
        <div class="header">
            <h1>{{.Title}}</h1>
            {{if .Subtitle}}<div class="subtitle">{{.Subtitle}}</div>{{end}}
            <div class="meta">
                <div class="meta-item">
                    <span>📅</span>
                    <span>生成时间: {{.GeneratedAt}}</span>
                </div>
                <div class="meta-item">
                    <span>📊</span>
                    <span>记录数: {{.TotalRecords}}</span>
                </div>
                {{if .PeriodStart}}
                <div class="meta-item">
                    <span>🕐</span>
                    <span>周期: {{.PeriodStart}} ~ {{.PeriodEnd}}</span>
                </div>
                {{end}}
            </div>
        </div>
        
        <div class="content">
            {{if .Summary}}
            <div class="card">
                <div class="card-header">📊 摘要信息</div>
                <div class="card-body">
                    <div class="summary-grid">
                        {{range .SummaryItems}}
                        <div class="summary-item">
                            <div class="value">{{.Value}}</div>
                            <div class="label">{{.Label}}</div>
                        </div>
                        {{end}}
                    </div>
                </div>
            </div>
            {{end}}
            
            {{if .IncludeCharts}}
            <div class="card">
                <div class="card-header">📈 趋势图表</div>
                <div class="card-body">
                    <div class="chart-placeholder">
                        图表区域 - 请在支持图表的环境中查看
                    </div>
                </div>
            </div>
            {{end}}
            
            <div class="card">
                <div class="card-header">📋 详细数据</div>
                <div class="card-body">
                    <div class="table-container">
                        <table>
                            <thead>
                                <tr>
                                    {{range .Headers}}<th>{{.}}</th>{{end}}
                                </tr>
                            </thead>
                            <tbody>
                                {{range .Rows}}
                                <tr>
                                    {{range .}}<td>{{.}}</td>{{end}}
                                </tr>
                                {{end}}
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
        </div>
        
        <div class="footer">
            {{.Company}} · NAS-OS v2.56.0 · 报表生成于 {{.GeneratedAt}}
        </div>
    </div>
</body>
</html>`

	return template.Must(template.New("report").Parse(tmplStr))
}

// prepareHTMLData 准备 HTML 数据
func (e *EnhancedExporter) prepareHTMLData(report *GeneratedReport, options EnhancedExportOptions) map[string]interface{} {
	title := options.Title
	if title == "" {
		title = report.Name
	}

	company := options.Company
	if company == "" {
		company = "NAS-OS"
	}

	// 提取表头
	headers := make([]string, 0)
	if len(report.Data) > 0 {
		for key := range report.Data[0] {
			headers = append(headers, key)
		}
	}

	// 提取行数据
	rows := make([][]string, 0, len(report.Data))
	for _, row := range report.Data {
		rowData := make([]string, 0, len(headers))
		for _, key := range headers {
			rowData = append(rowData, e.formatHTMLValue(row[key]))
		}
		rows = append(rows, rowData)
	}

	// 准备摘要项
	summaryItems := make([]map[string]string, 0)
	if report.Summary != nil {
		for key, value := range report.Summary {
			summaryItems = append(summaryItems, map[string]string{
				"Label": key,
				"Value": e.formatHTMLValue(value),
			})
		}
	}

	return map[string]interface{}{
		"Title":         title,
		"Subtitle":      options.Subtitle,
		"Company":       company,
		"Theme":         options.Theme,
		"GeneratedAt":   report.GeneratedAt.Format("2006-01-02 15:04:05"),
		"TotalRecords":  report.TotalRecords,
		"PeriodStart":   report.Period.StartTime.Format("2006-01-02"),
		"PeriodEnd":     report.Period.EndTime.Format("2006-01-02"),
		"Summary":       report.Summary != nil,
		"SummaryItems":  summaryItems,
		"IncludeCharts": options.IncludeCharts,
		"Headers":       headers,
		"Rows":          rows,
	}
}

// formatHTMLValue 格式化 HTML 值
func (e *EnhancedExporter) formatHTMLValue(val interface{}) string {
	if val == nil {
		return "-"
	}

	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.2f", v)
	case int, int64, uint64:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "是"
		}
		return "否"
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ========== PDF 增强 ==========

func (e *EnhancedExporter) exportPDFEnhanced(report *GeneratedReport, path string, options EnhancedExportOptions) error {
	// 先生成 HTML
	htmlData, err := e.generatePDFHTML(report, options)
	if err != nil {
		return err
	}

	// 保存临时 HTML 文件
	tmpHTML := path + ".html"
	if err := os.WriteFile(tmpHTML, htmlData, 0640); err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpHTML) }()

	// 使用 PDF 转换器
	if e.pdfConverter != nil && e.pdfConverter.IsAvailable() {
		if err := e.pdfConverter.Convert(tmpHTML, path); err != nil {
			// 转换失败，保存 HTML
			htmlPath := strings.TrimSuffix(path, ".pdf") + ".html"
			return os.WriteFile(htmlPath, htmlData, 0640)
		}
		return nil
	}

	// 没有转换器，保存 HTML
	htmlPath := strings.TrimSuffix(path, ".pdf") + ".html"
	return os.WriteFile(htmlPath, htmlData, 0640)
}

// generatePDFHTML 生成适合 PDF 的 HTML
func (e *EnhancedExporter) generatePDFHTML(report *GeneratedReport, options EnhancedExportOptions) ([]byte, error) {
	// PDF 专用模板（优化打印样式）
	tmplStr := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        @page {
            size: {{.PageSize}};
            margin: 15mm;
        }
        
        * { margin: 0; padding: 0; box-sizing: border-box; }
        
        body {
            font-family: "Microsoft YaHei", "SimSun", Arial, sans-serif;
            font-size: 12px;
            line-height: 1.5;
            color: #333;
        }
        
        .header {
            text-align: center;
            padding-bottom: 20px;
            border-bottom: 2px solid #333;
            margin-bottom: 20px;
        }
        
        .header h1 {
            font-size: 20px;
            margin-bottom: 10px;
        }
        
        .header .meta {
            font-size: 11px;
            color: #666;
        }
        
        .section {
            margin-bottom: 20px;
        }
        
        .section-title {
            font-size: 14px;
            font-weight: bold;
            border-bottom: 1px solid #ddd;
            padding-bottom: 5px;
            margin-bottom: 10px;
        }
        
        .summary-table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 15px;
        }
        
        .summary-table td {
            padding: 8px;
            border: 1px solid #ddd;
        }
        
        .summary-table td:first-child {
            width: 30%;
            background: #f5f5f5;
            font-weight: bold;
        }
        
        .data-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 11px;
        }
        
        .data-table th,
        .data-table td {
            padding: 8px;
            border: 1px solid #ddd;
            text-align: left;
        }
        
        .data-table th {
            background: #f5f5f5;
            font-weight: bold;
        }
        
        .data-table tr:nth-child(even) {
            background: #fafafa;
        }
        
        .footer {
            margin-top: 30px;
            padding-top: 10px;
            border-top: 1px solid #ddd;
            text-align: center;
            font-size: 10px;
            color: #666;
        }
        
        .page-break {
            page-break-after: always;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Title}}</h1>
        <div class="meta">
            生成时间: {{.GeneratedAt}} | 记录数: {{.TotalRecords}}
            {{if .PeriodStart}} | 周期: {{.PeriodStart}} ~ {{.PeriodEnd}}{{end}}
        </div>
    </div>
    
    {{if .Summary}}
    <div class="section">
        <div class="section-title">摘要信息</div>
        <table class="summary-table">
            {{range .SummaryItems}}
            <tr>
                <td>{{.Label}}</td>
                <td>{{.Value}}</td>
            </tr>
            {{end}}
        </table>
    </div>
    {{end}}
    
    <div class="section">
        <div class="section-title">详细数据</div>
        <table class="data-table">
            <thead>
                <tr>
                    {{range .Headers}}<th>{{.}}</th>{{end}}
                </tr>
            </thead>
            <tbody>
                {{range .Rows}}
                <tr>
                    {{range .}}<td>{{.}}</td>{{end}}
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
    
    <div class="footer">
        {{.Company}} · NAS-OS v2.56.0 · 第 <span class="page-number"></span> 页
    </div>
</body>
</html>`

	tmpl := template.Must(template.New("pdf").Parse(tmplStr))
	data := e.prepareHTMLData(report, options)
	data["PageSize"] = options.PageSize
	if data["PageSize"] == "" {
		data["PageSize"] = "A4"
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ========== Excel 增强 ==========

func (e *EnhancedExporter) exportExcelEnhanced(report *GeneratedReport, path string, options EnhancedExportOptions) error {
	_, err := e.excelExporter.Export(report, path, options.ExportOptions)
	return err
}

// ========== 辅助方法 ==========

func (e *EnhancedExporter) generateOutputPath(format ExportFormat) string {
	id := uuid.New().String()
	ext := string(format)
	return filepath.Join(e.dataDir, "outputs", id+"."+ext)
}

func (e *EnhancedExporter) getMimeType(format ExportFormat) string {
	switch format {
	case ExportJSON:
		return "application/json"
	case ExportCSV:
		return "text/csv"
	case ExportHTML:
		return "text/html"
	case ExportPDF:
		return "application/pdf"
	case ExportExcel:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}

// ========== WKHTMLToPDF 转换器 ==========

// WKHTMLToPDFConverter WKHTMLToPDF 转换器
type WKHTMLToPDFConverter struct {
	path string
}

// NewWKHTMLToPDFConverter 创建转换器
func NewWKHTMLToPDFConverter() *WKHTMLToPDFConverter {
	return &WKHTMLToPDFConverter{
		path: "/usr/bin/wkhtmltopdf",
	}
}

// IsAvailable 检查是否可用
func (c *WKHTMLToPDFConverter) IsAvailable() bool {
	_, err := os.Stat(c.path)
	return err == nil
}

// Convert 转换 HTML 到 PDF
func (c *WKHTMLToPDFConverter) Convert(htmlPath, pdfPath string) error {
	cmd := exec.Command(c.path,
		"--quiet",
		"--encoding", "UTF-8",
		"--page-size", "A4",
		"--margin-top", "15mm",
		"--margin-bottom", "15mm",
		"--margin-left", "15mm",
		"--margin-right", "15mm",
		htmlPath,
		pdfPath,
	)
	return cmd.Run()
}

// ========== 类型定义 ==========

// EnhancedExportOptions 增强导出选项
type EnhancedExportOptions struct {
	ExportOptions

	// CSV 选项
	IncludeBOM      bool        `json:"include_bom"`      // 包含 BOM（支持 Excel 打开）
	IncludeComments bool        `json:"include_comments"` // 包含注释行
	Columns         []CSVColumn `json:"columns"`          // 列定义

	// JSON 选项
	PrettyPrint     bool `json:"pretty_print"`     // 美化输出
	IncludeMetadata bool `json:"include_metadata"` // 包含元数据

	// HTML/PDF 选项
	Theme           string `json:"theme"`            // 主题: default, dark
	IncludeCharts   bool   `json:"include_charts"`   // 包含图表
	PageSize        string `json:"page_size"`        // PDF 页面大小: A4, Letter
	PageOrientation string `json:"page_orientation"` // PDF 页面方向: portrait, landscape

	// 其他选项
	IncludeSummary bool `json:"include_summary"` // 包含摘要
	IncludePeriod  bool `json:"include_period"`  // 包含周期信息
}

// CSVColumn CSV 列定义
type CSVColumn struct {
	Name   string    `json:"name"`   // 字段名
	Label  string    `json:"label"`  // 显示标签
	Type   FieldType `json:"type"`   // 字段类型
	Format string    `json:"format"` // 格式化模板
}

// ========== 增强 API 处理器（已移至 resource_api.go） ==========
