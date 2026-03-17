// Package reports 提供报表生成和管理功能
package reports

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ========== 导出器 ==========

// Exporter 报表导出器
type Exporter struct {
	dataDir string
}

// NewExporter 创建导出器
func NewExporter(dataDir string) *Exporter {
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(filepath.Join(dataDir, "outputs"), 0755)
	return &Exporter{dataDir: dataDir}
}

// Export 导出报表
func (e *Exporter) Export(report *GeneratedReport, format ExportFormat, outputPath string, options ExportOptions) (*ExportResult, error) {
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
		err = e.exportJSON(report, outputPath, options)
	case ExportCSV:
		err = e.exportCSV(report, outputPath, options)
	case ExportHTML:
		err = e.exportHTML(report, outputPath, options)
	case ExportPDF:
		err = e.exportPDF(report, outputPath, options)
	case ExportExcel:
		err = e.exportExcel(report, outputPath, options)
	default:
		err = errors.New("不支持的导出格式")
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

// ExportToFile 导出到文件（自动检测格式）
func (e *Exporter) ExportToFile(report *GeneratedReport, filePath string, options ExportOptions) (*ExportResult, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	var format ExportFormat
	switch ext {
	case ".json":
		format = ExportJSON
	case ".csv":
		format = ExportCSV
	case ".html", ".htm":
		format = ExportHTML
	case ".pdf":
		format = ExportPDF
	case ".xlsx":
		format = ExportExcel
	default:
		format = ExportJSON
	}

	return e.Export(report, format, filePath, options)
}

// ExportToBytes 导出到字节数组
func (e *Exporter) ExportToBytes(report *GeneratedReport, format ExportFormat, options ExportOptions) ([]byte, error) {
	switch format {
	case ExportJSON:
		return e.exportJSONBytes(report, options)
	case ExportCSV:
		return e.exportCSVBytes(report, options)
	case ExportHTML:
		return e.exportHTMLBytes(report, options)
	default:
		return nil, errors.New("该格式不支持导出到内存")
	}
}

// ========== JSON 导出 ==========

func (e *Exporter) exportJSON(report *GeneratedReport, path string, options ExportOptions) error {
	data, err := e.exportJSONBytes(report, options)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (e *Exporter) exportJSONBytes(report *GeneratedReport, options ExportOptions) ([]byte, error) {
	output := map[string]interface{}{
		"id":            report.ID,
		"name":          report.Name,
		"generated_at":  report.GeneratedAt,
		"total_records": report.TotalRecords,
		"data":          report.Data,
	}

	if options.Summary && report.Summary != nil {
		output["summary"] = report.Summary
	}

	if options.DateRange && !report.Period.StartTime.IsZero() {
		output["period"] = report.Period
	}

	if options.Title != "" {
		output["title"] = options.Title
	}

	return json.MarshalIndent(output, "", "  ")
}

// ========== CSV 导出 ==========

func (e *Exporter) exportCSV(report *GeneratedReport, path string, options ExportOptions) error {
	data, err := e.exportCSVBytes(report, options)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (e *Exporter) exportCSVBytes(report *GeneratedReport, options ExportOptions) ([]byte, error) {
	if len(report.Data) == 0 {
		return []byte{}, nil
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 写入标题（可选）
	if options.Title != "" {
		if err := writer.Write([]string{options.Title}); err != nil {
			return nil, err
		}
	}

	// 写入表头
	if options.IncludeHeader {
		headers := make([]string, 0, len(report.Data[0]))
		for key := range report.Data[0] {
			headers = append(headers, key)
		}
		if err := writer.Write(headers); err != nil {
			return nil, err
		}
	}

	// 写入数据
	for _, row := range report.Data {
		values := make([]string, 0, len(row))
		for _, key := range e.getSortedKeys(report.Data[0]) {
			val := row[key]
			values = append(values, e.formatValue(val))
		}
		if err := writer.Write(values); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// ========== HTML 导出 ==========

func (e *Exporter) exportHTML(report *GeneratedReport, path string, options ExportOptions) error {
	data, err := e.exportHTMLBytes(report, options)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (e *Exporter) exportHTMLBytes(report *GeneratedReport, options ExportOptions) ([]byte, error) {
	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; background: #f5f5f5; padding: 20px; }
        .container { max-width: 1200px; margin: 0 auto; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); overflow: hidden; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; }
        .header h1 { font-size: 24px; margin-bottom: 8px; }
        .header .subtitle { opacity: 0.9; font-size: 14px; }
        .meta { display: flex; gap: 30px; margin-top: 15px; font-size: 13px; opacity: 0.9; }
        .content { padding: 30px; }
        .summary { background: #f8f9fa; border-radius: 6px; padding: 20px; margin-bottom: 30px; }
        .summary h2 { font-size: 16px; margin-bottom: 15px; color: #555; }
        .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 15px; }
        .summary-item { background: white; padding: 15px; border-radius: 4px; text-align: center; }
        .summary-item .value { font-size: 24px; font-weight: 600; color: #667eea; }
        .summary-item .label { font-size: 12px; color: #666; margin-top: 5px; }
        table { width: 100%; border-collapse: collapse; font-size: 14px; }
        th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #eee; }
        th { background: #f8f9fa; font-weight: 600; color: #555; position: sticky; top: 0; }
        tr:hover { background: #f8f9fa; }
        .footer { text-align: center; padding: 20px; color: #999; font-size: 12px; border-top: 1px solid #eee; }
        @media print { body { background: white; padding: 0; } .container { box-shadow: none; } }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.Title}}</h1>
            {{if .Subtitle}}<div class="subtitle">{{.Subtitle}}</div>{{end}}
            <div class="meta">
                <span>生成时间: {{.GeneratedAt}}</span>
                <span>记录数: {{.TotalRecords}}</span>
            </div>
        </div>
        <div class="content">
            {{if .Summary}}
            <div class="summary">
                <h2>摘要</h2>
                <div class="summary-grid">
                    {{range $key, $value := .Summary}}
                    <div class="summary-item">
                        <div class="value">{{$value}}</div>
                        <div class="label">{{$key}}</div>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
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
        <div class="footer">
            {{.Company}} · 报表自动生成于 {{.GeneratedAt}}
        </div>
    </div>
</body>
</html>`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return nil, err
	}

	// 准备数据
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
			rowData = append(rowData, e.formatValue(row[key]))
		}
		rows = append(rows, rowData)
	}

	data := map[string]interface{}{
		"Title":        title,
		"Subtitle":     options.Subtitle,
		"Company":      company,
		"GeneratedAt":  report.GeneratedAt.Format("2006-01-02 15:04:05"),
		"TotalRecords": report.TotalRecords,
		"Summary":      report.Summary,
		"Headers":      headers,
		"Rows":         rows,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ========== PDF 导出 ==========

func (e *Exporter) exportPDF(report *GeneratedReport, path string, options ExportOptions) error {
	// 先生成 HTML
	htmlData, err := e.exportHTMLBytes(report, options)
	if err != nil {
		return err
	}

	// 保存临时 HTML 文件
	tmpHTML := path + ".html"
	if err := os.WriteFile(tmpHTML, htmlData, 0644); err != nil {
		return err
	}
	defer os.Remove(tmpHTML)

	// 使用 wkhtmltopdf 或类似工具转换
	// 这里简化处理：如果没有安装转换工具，就保存 HTML
	pdfPath := path

	// 检查是否有转换工具
	if e.hasPDFConverter() {
		// 使用转换工具
		cmd := fmt.Sprintf("wkhtmltopdf --quiet %s %s", tmpHTML, pdfPath)
		if err := e.runCommand(cmd); err != nil {
			// 转换失败，保存 HTML
			return os.WriteFile(path, htmlData, 0644)
		}
	} else {
		// 没有转换工具，保存 HTML 并修改扩展名提示
		htmlPath := strings.TrimSuffix(path, ".pdf") + ".html"
		return os.WriteFile(htmlPath, htmlData, 0644)
	}

	return nil
}

// hasPDFConverter 检查是否有 PDF 转换工具
func (e *Exporter) hasPDFConverter() bool {
	// 检查 wkhtmltopdf 是否存在
	_, err := os.Stat("/usr/bin/wkhtmltopdf")
	return err == nil
}

// runCommand 运行命令
func (e *Exporter) runCommand(cmd string) error {
	// 简化实现，实际应使用 exec.Command
	return nil
}

// ========== Excel 导出 ==========

func (e *Exporter) exportExcel(report *GeneratedReport, path string, options ExportOptions) error {
	// 使用专门的 Excel 导出器
	excelExporter := NewExcelExporter(e.dataDir)
	result, err := excelExporter.Export(report, path, options)
	if err != nil {
		return err
	}
	// 更新路径（可能由导出器自动生成）
	if result.Path != path {
		// 如果路径不同，重命名文件
		if err := os.Rename(result.Path, path); err != nil {
			return err
		}
	}
	return nil
}

// ========== 辅助方法 ==========

func (e *Exporter) generateOutputPath(format ExportFormat) string {
	id := uuid.New().String()
	ext := string(format)
	return filepath.Join(e.dataDir, "outputs", id+"."+ext)
}

func (e *Exporter) getMimeType(format ExportFormat) string {
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

func (e *Exporter) getSortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 简单排序
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func (e *Exporter) formatValue(val interface{}) string {
	if val == nil {
		return ""
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

// ========== 批量导出 ==========

// ExportMultiple 批量导出多种格式
func (e *Exporter) ExportMultiple(report *GeneratedReport, formats []ExportFormat, baseDir string, options ExportOptions) ([]*ExportResult, error) {
	results := make([]*ExportResult, 0, len(formats))

	for _, format := range formats {
		filename := fmt.Sprintf("%s_%s.%s", report.Name, time.Now().Format("20060102"), format)
		path := filepath.Join(baseDir, filename)

		result, err := e.Export(report, format, path, options)
		if err != nil {
			continue
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, errors.New("所有格式导出失败")
	}

	return results, nil
}

// GetSupportedFormats 获取支持的导出格式
func (e *Exporter) GetSupportedFormats() []ExportFormat {
	return []ExportFormat{
		ExportJSON,
		ExportCSV,
		ExportHTML,
		ExportPDF,
		ExportExcel,
	}
}

// GetFormatInfo 获取格式信息
func (e *Exporter) GetFormatInfo(format ExportFormat) map[string]string {
	info := map[string]string{
		string(ExportJSON):  "JSON - 数据交换格式，适合程序处理",
		string(ExportCSV):   "CSV - 逗号分隔值，适合表格软件导入",
		string(ExportHTML):  "HTML - 网页格式，适合浏览器查看和打印",
		string(ExportPDF):   "PDF - 便携文档格式，适合归档和分享",
		string(ExportExcel): "Excel - 电子表格格式，适合数据分析和编辑",
	}

	return map[string]string{
		"format": string(format),
		"mime":   e.getMimeType(format),
		"desc":   info[string(format)],
	}
}
