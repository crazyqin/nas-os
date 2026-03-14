// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ========== Excel 导出器 ==========

// ExcelExporter Excel 导出器
type ExcelExporter struct {
	dataDir string
}

// NewExcelExporter 创建 Excel 导出器
func NewExcelExporter(dataDir string) *ExcelExporter {
	os.MkdirAll(dataDir, 0755)
	return &ExcelExporter{dataDir: dataDir}
}

// Export 导出报表到 Excel 文件
func (e *ExcelExporter) Export(report *GeneratedReport, outputPath string, options ExportOptions) (*ExportResult, error) {
	if outputPath == "" {
		outputPath = e.generateOutputPath()
	}

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建 Excel 文件
	f, err := e.createExcelFile(report, options)
	if err != nil {
		return nil, fmt.Errorf("创建 Excel 文件失败: %w", err)
	}

	// 保存文件
	if err := f.SaveAs(outputPath); err != nil {
		f.Close()
		return nil, fmt.Errorf("保存 Excel 文件失败: %w", err)
	}
	f.Close()

	// 获取文件信息
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	return &ExportResult{
		Format:    ExportExcel,
		Filename:  filepath.Base(outputPath),
		Path:      outputPath,
		Size:      info.Size(),
		MimeType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		CreatedAt: time.Now(),
	}, nil
}

// ExportToBytes 导出到字节数组
func (e *ExcelExporter) ExportToBytes(report *GeneratedReport, options ExportOptions) ([]byte, error) {
	f, err := e.createExcelFile(report, options)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("写入缓冲区失败: %w", err)
	}

	return buf.Bytes(), nil
}

// createExcelFile 创建 Excel 文件对象
func (e *ExcelExporter) createExcelFile(report *GeneratedReport, options ExportOptions) (*excelize.File, error) {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("关闭文件失败: %v\n", err)
		}
	}()

	// 设置文档属性
	e.setDocumentProperties(f, report, options)

	// 创建摘要工作表
	if options.Summary && report.Summary != nil {
		if err := e.createSummarySheet(f, report, options); err != nil {
			return nil, err
		}
	}

	// 创建数据工作表
	if err := e.createDataSheet(f, report, options); err != nil {
		return nil, err
	}

	// 创建图表工作表（如果数据足够）
	if options.Charts && len(report.Data) > 1 {
		if err := e.createChartSheet(f, report, options); err != nil {
			// 图表创建失败不影响导出
			fmt.Printf("创建图表失败: %v\n", err)
		}
	}

	// 删除默认的 Sheet1
	index, err := f.GetSheetIndex("Sheet1")
	if err == nil && index != 0 {
		f.DeleteSheet("Sheet1")
	}

	return f, nil
}

// setDocumentProperties 设置文档属性
func (e *ExcelExporter) setDocumentProperties(f *excelize.File, report *GeneratedReport, options ExportOptions) {
	props := &excelize.DocProperties{
		Title:          options.Title,
		Subject:        report.Name,
		Creator:        options.Company,
		Description:    report.Name,
		LastModifiedBy: "NAS-OS Reports System",
		Created:        time.Now().Format(time.RFC3339),
		Modified:       time.Now().Format(time.RFC3339),
	}

	if props.Title == "" {
		props.Title = report.Name
	}
	if props.Creator == "" {
		props.Creator = "NAS-OS"
	}

	f.SetDocProps(props)
}

// createSummarySheet 创建摘要工作表
func (e *ExcelExporter) createSummarySheet(f *excelize.File, report *GeneratedReport, options ExportOptions) error {
	sheetName := "摘要"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.SetActiveSheet(index)

	// 设置标题样式
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 16,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
		},
	})

	// 设置标签样式
	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E8E8E8"},
			Pattern: 1,
		},
	})

	// 设置值样式
	valueStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size: 11,
		},
	})

	// 写入标题
	title := options.Title
	if title == "" {
		title = report.Name
	}
	f.MergeCell(sheetName, "A1", "D1")
	f.SetCellValue(sheetName, "A1", title)
	f.SetCellStyle(sheetName, "A1", "D1", titleStyle)
	f.SetRowHeight(sheetName, 1, 30)

	// 写入生成时间
	f.MergeCell(sheetName, "A2", "D2")
	f.SetCellValue(sheetName, "A2", fmt.Sprintf("生成时间: %s", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	dateStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "#666666"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	f.SetCellStyle(sheetName, "A2", "D2", dateStyle)

	// 写入时间范围（如果有）
	row := 4
	if !report.Period.StartTime.IsZero() {
		f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row))
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("报告周期: %s 至 %s",
			report.Period.StartTime.Format("2006-01-02"),
			report.Period.EndTime.Format("2006-01-02")))
		row++
	}

	// 写入总记录数
	f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row))
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("总记录数: %d", report.TotalRecords))
	row += 2

	// 写入摘要数据
	if report.Summary != nil && len(report.Summary) > 0 {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "摘要指标")
		f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row))
		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), labelStyle)
		row++

		for key, value := range report.Summary {
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), key)
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), labelStyle)
			f.MergeCell(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row))
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), e.formatValue(value))
			f.SetCellStyle(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row), valueStyle)
			row++
		}
	}

	// 设置列宽
	f.SetColWidth(sheetName, "A", "D", 25)

	return nil
}

// createDataSheet 创建数据工作表
func (e *ExcelExporter) createDataSheet(f *excelize.File, report *GeneratedReport, options ExportOptions) error {
	sheetName := "数据"

	// 检查是否已有摘要表，如果有则使用第一个位置
	hasSummary := options.Summary && report.Summary != nil
	if !hasSummary {
		// 使用 Sheet1 作为数据表
		f.SetSheetName("Sheet1", sheetName)
	} else {
		// 创建新的数据表
		index, err := f.NewSheet(sheetName)
		if err != nil {
			return err
		}
		// 将数据表设为活动表
		f.SetActiveSheet(index)
	}

	if len(report.Data) == 0 {
		f.SetCellValue(sheetName, "A1", "无数据")
		return nil
	}

	// 获取字段顺序
	fields := e.getFieldOrder(report.Data[0])

	// 设置表头样式
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Size:  11,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#B4B4B4", Style: 1},
			{Type: "top", Color: "#B4B4B4", Style: 1},
			{Type: "bottom", Color: "#B4B4B4", Style: 1},
			{Type: "right", Color: "#B4B4B4", Style: 1},
		},
	})

	// 设置数据单元格样式
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Vertical: "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#D9D9D9", Style: 1},
			{Type: "top", Color: "#D9D9D9", Style: 1},
			{Type: "bottom", Color: "#D9D9D9", Style: 1},
			{Type: "right", Color: "#D9D9D9", Style: 1},
		},
	})

	// 写入表头
	for i, field := range fields {
		col := e.getColumnLetter(i + 1)
		cell := fmt.Sprintf("%s1", col)
		// 使用 label 作为表头显示
		label := e.getFieldLabel(field, options)
		f.SetCellValue(sheetName, cell, label)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// 写入数据
	for rowIdx, row := range report.Data {
		rowNum := rowIdx + 2
		for colIdx, field := range fields {
			col := e.getColumnLetter(colIdx + 1)
			cell := fmt.Sprintf("%s%d", col, rowNum)
			value := row[field]
			f.SetCellValue(sheetName, cell, e.formatValue(value))
			f.SetCellStyle(sheetName, cell, cell, dataStyle)
		}
	}

	// 设置自动筛选
	if len(fields) > 0 {
		lastCol := e.getColumnLetter(len(fields))
		f.AutoFilter(sheetName, fmt.Sprintf("A1:%s%d", lastCol, len(report.Data)+1), []excelize.AutoFilterOptions{})
	}

	// 冻结首行
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// 自动调整列宽
	for i := range fields {
		col := e.getColumnLetter(i + 1)
		f.SetColWidth(sheetName, col, col, 18)
	}

	return nil
}

// createChartSheet 创建图表工作表
func (e *ExcelExporter) createChartSheet(f *excelize.File, report *GeneratedReport, options ExportOptions) error {
	sheetName := "图表"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}

	// 如果有数值型数据，创建简单的柱状图
	if len(report.Data) < 2 {
		return nil
	}

	// 找出数值型字段
	fields := e.getFieldOrder(report.Data[0])
	var numericFields []string
	for _, field := range fields {
		if e.isNumericField(report.Data, field) {
			numericFields = append(numericFields, field)
		}
	}

	if len(numericFields) == 0 {
		return nil
	}

	// 创建图表数据区域
	chartSheet := sheetName
	dataSheet := "数据"

	// 创建图表 - 使用第一个数值字段
	if len(numericFields) > 0 {
		// 获取字段索引
		fieldIdx := 0
		for i, f := range fields {
			if f == numericFields[0] {
				fieldIdx = i
				break
			}
		}

		// 创建图表
		chartCol := e.getColumnLetter(fieldIdx + 1)
		dataRange := fmt.Sprintf("%s!%s2:%s%d", dataSheet, chartCol, chartCol, len(report.Data)+1)

		if err := f.AddChart(chartSheet, "A1", &excelize.Chart{
			Type: excelize.Col,
			Series: []excelize.ChartSeries{
				{
					Name:       numericFields[0],
					Categories: dataRange,
					Values:     dataRange,
				},
			},
			Title: []excelize.RichTextRun{
				{
					Text: fmt.Sprintf("%s 统计", numericFields[0]),
				},
			},
		}); err != nil {
			return err
		}
	}

	_ = index // 使用 index 避免编译警告
	return nil
}

// getFieldOrder 获取字段顺序
func (e *ExcelExporter) getFieldOrder(row map[string]interface{}) []string {
	fields := make([]string, 0, len(row))
	for key := range row {
		fields = append(fields, key)
	}
	return fields
}

// getFieldLabel 获取字段显示标签
func (e *ExcelExporter) getFieldLabel(field string, options ExportOptions) string {
	// 可以根据 options 中的字段映射返回对应的标签
	// 这里简单返回字段名
	return field
}

// isNumericField 检查字段是否为数值类型
func (e *ExcelExporter) isNumericField(data []map[string]interface{}, field string) bool {
	for _, row := range data {
		if val, ok := row[field]; ok {
			switch val.(type) {
			case int, int64, float64, uint64:
				return true
			}
		}
	}
	return false
}

// getColumnLetter 将列号转换为 Excel 列字母
func (e *ExcelExporter) getColumnLetter(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

// formatValue 格式化值
func (e *ExcelExporter) formatValue(val interface{}) interface{} {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case float64:
		return v
	case int:
		return v
	case int64:
		return v
	case uint64:
		return v
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

// generateOutputPath 生成输出路径
func (e *ExcelExporter) generateOutputPath() string {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("report_%s.xlsx", timestamp)
	return filepath.Join(e.dataDir, "outputs", filename)
}

// ========== 高级 Excel 功能 ==========

// ExcelReportBuilder Excel 报表构建器
type ExcelReportBuilder struct {
	exporter    *ExcelExporter
	report      *GeneratedReport
	options     ExportOptions
	styles      map[string]int
	sheets      []string
	conditional bool
}

// NewExcelReportBuilder 创建 Excel 报表构建器
func NewExcelReportBuilder(exporter *ExcelExporter, report *GeneratedReport) *ExcelReportBuilder {
	return &ExcelReportBuilder{
		exporter: exporter,
		report:   report,
		options: ExportOptions{
			IncludeHeader: true,
			Summary:       true,
		},
		styles: make(map[string]int),
	}
}

// SetOptions 设置导出选项
func (b *ExcelReportBuilder) SetOptions(options ExportOptions) *ExcelReportBuilder {
	b.options = options
	return b
}

// EnableConditionalFormatting 启用条件格式
func (b *ExcelReportBuilder) EnableConditionalFormatting(enable bool) *ExcelReportBuilder {
	b.conditional = enable
	return b
}

// Build 构建 Excel 文件
func (b *ExcelReportBuilder) Build(outputPath string) (*ExportResult, error) {
	return b.exporter.Export(b.report, outputPath, b.options)
}

// ========== 多工作表 Excel 导出 ==========

// MultiSheetExporter 多工作表 Excel 导出器
type MultiSheetExporter struct {
	exporter *ExcelExporter
}

// NewMultiSheetExporter 创建多工作表导出器
func NewMultiSheetExporter(exporter *ExcelExporter) *MultiSheetExporter {
	return &MultiSheetExporter{exporter: exporter}
}

// ExportMultiple 导出多个报表到一个 Excel 文件的不同工作表
func (m *MultiSheetExporter) ExportMultiple(reports []*GeneratedReport, outputPath string, options ExportOptions) (*ExportResult, error) {
	if len(reports) == 0 {
		return nil, fmt.Errorf("没有报表可导出")
	}

	// 创建 Excel 文件
	f := excelize.NewFile()

	// 删除默认的 Sheet1
	f.DeleteSheet("Sheet1")

	// 导出每个报表到不同工作表
	for i, report := range reports {
		sheetName := m.sanitizeSheetName(report.Name, i)
		if _, err := f.NewSheet(sheetName); err != nil {
			f.Close()
			return nil, fmt.Errorf("创建工作表失败: %w", err)
		}

		// 创建数据表内容
		if err := m.createSheetContent(f, sheetName, report, options); err != nil {
			f.Close()
			return nil, err
		}
	}

	// 保存文件
	if err := f.SaveAs(outputPath); err != nil {
		f.Close()
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}
	f.Close()

	// 获取文件信息
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, err
	}

	return &ExportResult{
		Format:    ExportExcel,
		Filename:  filepath.Base(outputPath),
		Path:      outputPath,
		Size:      info.Size(),
		MimeType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		CreatedAt: time.Now(),
	}, nil
}

// createSheetContent 创建工作表内容
func (m *MultiSheetExporter) createSheetContent(f *excelize.File, sheetName string, report *GeneratedReport, options ExportOptions) error {
	if len(report.Data) == 0 {
		f.SetCellValue(sheetName, "A1", "无数据")
		return nil
	}

	// 获取字段顺序
	fields := m.getFieldOrder(report.Data[0])

	// 设置表头样式
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "#FFFFFF"},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// 写入表头
	for i, field := range fields {
		col := m.getColumnLetter(i + 1)
		f.SetCellValue(sheetName, fmt.Sprintf("%s1", col), field)
		f.SetCellStyle(sheetName, fmt.Sprintf("%s1", col), fmt.Sprintf("%s1", col), headerStyle)
	}

	// 写入数据
	for rowIdx, row := range report.Data {
		rowNum := rowIdx + 2
		for colIdx, field := range fields {
			col := m.getColumnLetter(colIdx + 1)
			f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, rowNum), m.formatValue(row[field]))
		}
	}

	// 设置列宽
	for i := range fields {
		col := m.getColumnLetter(i + 1)
		f.SetColWidth(sheetName, col, col, 18)
	}

	return nil
}

// sanitizeSheetName 清理工作表名称
func (m *MultiSheetExporter) sanitizeSheetName(name string, index int) string {
	// Excel 工作表名称限制：31 字符，不能包含 : \ / ? * [ ]
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")

	if len(name) > 28 {
		name = name[:28]
	}

	// 添加序号避免重复
	if index > 0 {
		name = fmt.Sprintf("%s_%d", name, index)
	}

	return name
}

// getFieldOrder 获取字段顺序
func (m *MultiSheetExporter) getFieldOrder(row map[string]interface{}) []string {
	fields := make([]string, 0, len(row))
	for key := range row {
		fields = append(fields, key)
	}
	return fields
}

// getColumnLetter 将列号转换为 Excel 列字母
func (m *MultiSheetExporter) getColumnLetter(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

// formatValue 格式化值
func (m *MultiSheetExporter) formatValue(val interface{}) interface{} {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64, int, int64, uint64:
		return v
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

// ========== Excel 模板导出 ==========

// ExcelTemplateExporter Excel 模板导出器
type ExcelTemplateExporter struct {
	exporter *ExcelExporter
}

// NewExcelTemplateExporter 创建 Excel 模板导出器
func NewExcelTemplateExporter(exporter *ExcelExporter) *ExcelTemplateExporter {
	return &ExcelTemplateExporter{exporter: exporter}
}

// ExportWithTemplate 使用模板导出
func (t *ExcelTemplateExporter) ExportWithTemplate(report *GeneratedReport, templatePath, outputPath string, options ExportOptions) (*ExportResult, error) {
	// 打开模板文件
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("打开模板失败: %w", err)
	}
	defer f.Close()

	// 查找数据占位符并填充
	if err := t.fillTemplateData(f, report, options); err != nil {
		return nil, err
	}

	// 保存到新文件
	if err := f.SaveAs(outputPath); err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	// 获取文件信息
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, err
	}

	return &ExportResult{
		Format:    ExportExcel,
		Filename:  filepath.Base(outputPath),
		Path:      outputPath,
		Size:      info.Size(),
		MimeType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		CreatedAt: time.Now(),
	}, nil
}

// fillTemplateData 填充模板数据
func (t *ExcelTemplateExporter) fillTemplateData(f *excelize.File, report *GeneratedReport, options ExportOptions) error {
	// 遍历所有工作表
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		// 获取所有行
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}

		for rowIdx, row := range rows {
			for colIdx, cell := range row {
				// 查找占位符 {{field}} 并替换
				if strings.Contains(cell, "{{") {
					newValue := t.replacePlaceholders(cell, report)
					colName := t.getColumnLetter(colIdx + 1)
					f.SetCellValue(sheet, fmt.Sprintf("%s%d", colName, rowIdx+1), newValue)
				}
			}
		}
	}

	return nil
}

// replacePlaceholders 替换占位符
func (t *ExcelTemplateExporter) replacePlaceholders(template string, report *GeneratedReport) string {
	result := template

	// 替换基本字段
	result = strings.ReplaceAll(result, "{{report_name}}", report.Name)
	result = strings.ReplaceAll(result, "{{generated_at}}", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	result = strings.ReplaceAll(result, "{{total_records}}", fmt.Sprintf("%d", report.TotalRecords))

	// 替换摘要字段
	for key, value := range report.Summary {
		placeholder := fmt.Sprintf("{{summary.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	// 替换时间范围
	if !report.Period.StartTime.IsZero() {
		result = strings.ReplaceAll(result, "{{period.start}}", report.Period.StartTime.Format("2006-01-02"))
		result = strings.ReplaceAll(result, "{{period.end}}", report.Period.EndTime.Format("2006-01-02"))
	}

	return result
}

// getColumnLetter 将列号转换为 Excel 列字母
func (t *ExcelTemplateExporter) getColumnLetter(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}
