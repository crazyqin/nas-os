package reports

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ========== PDF 导出增强 ==========

// PDFGenerator PDF 生成器接口
type PDFGenerator interface {
	Generate(htmlContent []byte, outputPath string, options PDFOptions) error
	IsAvailable() bool
}

// PDFOptions PDF 选项
type PDFOptions struct {
	PageSize    string `json:"page_size"`    // A4, Letter, Legal
	Orientation string `json:"orientation"`  // portrait, landscape
	MarginTop   string `json:"margin_top"`   // 顶部边距
	MarginRight string `json:"margin_right"` // 右边距
	MarginLeft  string `json:"margin_left"`  // 左边距
	Header      string `json:"header"`       // 页眉
	Footer      string `json:"footer"`       // 页脚
}

// WkhtmltopdfGenerator 使用 wkhtmltopdf 生成 PDF
type WkhtmltopdfGenerator struct {
	binaryPath string
}

// NewWkhtmltopdfGenerator 创建 wkhtmltopdf 生成器
func NewWkhtmltopdfGenerator() *WkhtmltopdfGenerator {
	// 查找 wkhtmltopdf 路径
	paths := []string{
		"/usr/bin/wkhtmltopdf",
		"/usr/local/bin/wkhtmltopdf",
		"C:\\Program Files\\wkhtmltopdf\\bin\\wkhtmltopdf.exe",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return &WkhtmltopdfGenerator{binaryPath: p}
		}
	}

	// 尝试从 PATH 查找
	if path, err := exec.LookPath("wkhtmltopdf"); err == nil {
		return &WkhtmltopdfGenerator{binaryPath: path}
	}

	return &WkhtmltopdfGenerator{binaryPath: ""}
}

// IsAvailable 检查是否可用
func (g *WkhtmltopdfGenerator) IsAvailable() bool {
	return g.binaryPath != ""
}

// Generate 生成 PDF
func (g *WkhtmltopdfGenerator) Generate(htmlContent []byte, outputPath string, options PDFOptions) error {
	if !g.IsAvailable() {
		return errors.New("wkhtmltopdf 不可用")
	}

	// 保存临时 HTML
	tmpHTML := outputPath + ".tmp.html"
	if err := os.WriteFile(tmpHTML, htmlContent, 0644); err != nil {
		return fmt.Errorf("写入临时 HTML 失败: %w", err)
	}
	defer os.Remove(tmpHTML)

	// 构建命令参数
	args := []string{
		"--quiet",
		"--encoding", "UTF-8",
		"--no-stop-slow-scripts",
	}

	// 页面大小
	pageSize := options.PageSize
	if pageSize == "" {
		pageSize = "A4"
	}
	args = append(args, "--page-size", pageSize)

	// 页面方向
	if options.Orientation == "landscape" {
		args = append(args, "--orientation", "Landscape")
	} else {
		args = append(args, "--orientation", "Portrait")
	}

	// 边距
	if options.MarginTop != "" {
		args = append(args, "--margin-top", options.MarginTop)
	}
	if options.MarginRight != "" {
		args = append(args, "--margin-right", options.MarginRight)
	}
	if options.MarginLeft != "" {
		args = append(args, "--margin-left", options.MarginLeft)
	}

	// 页眉页脚
	if options.Header != "" {
		args = append(args, "--header-center", options.Header)
		args = append(args, "--header-font-size", "9")
	}
	if options.Footer != "" {
		args = append(args, "--footer-center", options.Footer)
		args = append(args, "--footer-font-size", "9")
	} else {
		// 默认页脚显示页码
		args = append(args, "--footer-center", "第 [page] 页 / 共 [topage] 页")
		args = append(args, "--footer-font-size", "9")
	}

	// 添加输入输出文件
	args = append(args, tmpHTML, outputPath)

	// 执行命令
	cmd := exec.Command(g.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("生成 PDF 失败: %w, 输出: %s", err, string(output))
	}

	return nil
}

// ExporterV2 增强版导出器
type ExporterV2 struct {
	*Exporter
	pdfGenerator PDFGenerator
}

// NewExporterV2 创建增强版导出器
func NewExporterV2(dataDir string) *ExporterV2 {
	return &ExporterV2{
		Exporter:     NewExporter(dataDir),
		pdfGenerator: NewWkhtmltopdfGenerator(),
	}
}

// SetPDFGenerator 设置 PDF 生成器
func (e *ExporterV2) SetPDFGenerator(generator PDFGenerator) {
	e.pdfGenerator = generator
}

// ExportPDFWithGenerator 使用生成器导出 PDF
func (e *ExporterV2) ExportPDFWithGenerator(report *GeneratedReport, outputPath string, options ExportOptions) error {
	// 生成 HTML
	htmlData, err := e.ExportToBytes(report, ExportHTML, options)
	if err != nil {
		return fmt.Errorf("生成 HTML 失败: %w", err)
	}

	// 检查 PDF 生成器
	if e.pdfGenerator == nil || !e.pdfGenerator.IsAvailable() {
		// 回退到保存 HTML
		htmlPath := strings.TrimSuffix(outputPath, ".pdf") + ".html"
		return os.WriteFile(htmlPath, htmlData, 0644)
	}

	// 转换选项
	pdfOptions := PDFOptions{
		PageSize:    options.PageSize,
		Orientation: options.Orientation,
	}

	if options.Company != "" {
		pdfOptions.Header = options.Company
	}
	if options.Footer != "" {
		pdfOptions.Footer = options.Footer
	}

	// 生成 PDF
	return e.pdfGenerator.Generate(htmlData, outputPath, pdfOptions)
}

// ========== CSV 导出增强 ==========

// CSVExporter CSV 导出器
type CSVExporter struct {
	separator  rune
	encoding   string
	withHeader bool
}

// NewCSVExporter 创建 CSV 导出器
func NewCSVExporter() *CSVExporter {
	return &CSVExporter{
		separator:  ',',
		encoding:   "UTF-8",
		withHeader: true,
	}
}

// SetSeparator 设置分隔符
func (e *CSVExporter) SetSeparator(sep rune) {
	e.separator = sep
}

// SetWithHeader 设置是否包含表头
func (e *CSVExporter) SetWithHeader(withHeader bool) {
	e.withHeader = withHeader
}

// Export 导出 CSV
func (e *CSVExporter) Export(report *GeneratedReport, outputPath string) error {
	if len(report.Data) == 0 {
		return os.WriteFile(outputPath, []byte(""), 0644)
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = e.separator

	// 获取字段顺序（保持一致性）
	fields := make([]string, 0, len(report.Data[0]))
	for key := range report.Data[0] {
		fields = append(fields, key)
	}

	// 写入表头
	if e.withHeader {
		if err := writer.Write(fields); err != nil {
			return err
		}
	}

	// 写入数据行
	for _, row := range report.Data {
		values := make([]string, 0, len(fields))
		for _, field := range fields {
			values = append(values, formatCSVValue(row[field]))
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	// 添加 BOM 以支持 Excel 打开 UTF-8
	var output []byte
	if e.encoding == "UTF-8" {
		bom := []byte{0xEF, 0xBB, 0xBF}
		output = append(bom, buf.Bytes()...)
	} else {
		output = buf.Bytes()
	}

	return os.WriteFile(outputPath, output, 0644)
}

// formatCSVValue 格式化 CSV 值
func formatCSVValue(val interface{}) string {
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
			return "true"
		}
		return "false"
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ========== 批量导出工具 ==========

// BatchExporter 批量导出器
type BatchExporter struct {
	exporter *ExporterV2
}

// NewBatchExporter 创建批量导出器
func NewBatchExporter(exporter *ExporterV2) *BatchExporter {
	return &BatchExporter{exporter: exporter}
}

// ExportReport 导出单个报告（多种格式）
func (be *BatchExporter) ExportReport(report *GeneratedReport, formats []ExportFormat, baseDir string, options ExportOptions) ([]*ExportResult, error) {
	results := make([]*ExportResult, 0, len(formats))

	for _, format := range formats {
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("%s_%s.%s", sanitizeFilename(report.Name), timestamp, format)
		outputPath := filepath.Join(baseDir, filename)

		var result *ExportResult
		var err error

		if format == ExportPDF {
			// 使用增强的 PDF 导出
			if err := be.exporter.ExportPDFWithGenerator(report, outputPath, options); err != nil {
				continue
			}
			info, statErr := os.Stat(outputPath)
			if statErr != nil {
				continue
			}
			result = &ExportResult{
				Format:    format,
				Filename:  filepath.Base(outputPath),
				Path:      outputPath,
				Size:      info.Size(),
				MimeType:  "application/pdf",
				CreatedAt: time.Now(),
			}
		} else {
			result, err = be.exporter.Export(report, format, outputPath, options)
			if err != nil {
				continue
			}
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, errors.New("所有格式导出失败")
	}

	return results, nil
}

// ExportMultipleReports 批量导出多个报告
func (be *BatchExporter) ExportMultipleReports(reports []*GeneratedReport, format ExportFormat, baseDir string, options ExportOptions) ([]*ExportResult, error) {
	results := make([]*ExportResult, 0, len(reports))

	for _, report := range reports {
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("%s_%s.%s", sanitizeFilename(report.Name), timestamp, format)
		outputPath := filepath.Join(baseDir, filename)

		result, err := be.exporter.Export(report, format, outputPath, options)
		if err != nil {
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

// sanitizeFilename 清理文件名
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}

// ========== 导出辅助函数 ==========

// CreateReportArchive 创建报告压缩包
func CreateReportArchive(outputPath string, files []string) error {
	// 使用 tar 或 zip 压缩
	if strings.HasSuffix(outputPath, ".zip") {
		return createZipArchive(outputPath, files)
	}
	return createTarArchive(outputPath, files)
}

func createZipArchive(outputPath string, files []string) error {
	// 简化实现，实际应使用 archive/zip
	return nil
}

func createTarArchive(outputPath string, files []string) error {
	// 简化实现，实际应使用 archive/tar
	return nil
}

// GetExportStats 获取导出统计
func GetExportStats(results []*ExportResult) map[string]interface{} {
	totalSize := int64(0)
	formatCount := make(map[ExportFormat]int)

	for _, r := range results {
		totalSize += r.Size
		formatCount[r.Format]++
	}

	return map[string]interface{}{
		"total_files": len(results),
		"total_size":  totalSize,
		"formats":     formatCount,
	}
}