// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ========== Excel 导出器 ==========

// ChartType 图表类型
type ChartType string

const (
	// ChartTypeBar represents bar chart type.
	ChartTypeBar ChartType = "bar" // 柱状图
	// ChartTypeLine represents line chart type.
	ChartTypeLine ChartType = "line" // 折线图
	// ChartTypePie represents pie chart type.
	ChartTypePie ChartType = "pie" // 饼图
	// ChartTypeArea represents area chart type.
	ChartTypeArea ChartType = "area" // 面积图
	// ChartTypeScatter represents scatter chart type.
	ChartTypeScatter ChartType = "scatter" // 散点图
	// ChartTypeDoughnut represents doughnut chart type.
	ChartTypeDoughnut ChartType = "doughnut" // 环形图
)

// ExcelStyleTemplate Excel 样式模板
type ExcelStyleTemplate struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Header        ExcelCellStyle    `json:"header"`
	Data          ExcelCellStyle    `json:"data"`
	Summary       ExcelCellStyle    `json:"summary"`
	Title         ExcelCellStyle    `json:"title"`
	Colors        ExcelColorScheme  `json:"colors"`
	Fonts         ExcelFontScheme   `json:"fonts"`
	Borders       ExcelBorderScheme `json:"borders"`
	NumberFormats map[string]string `json:"number_formats"`
}

// ExcelCellStyle 单元格样式
type ExcelCellStyle struct {
	Font      *ExcelFontConfig      `json:"font,omitempty"`
	Fill      *ExcelFillConfig      `json:"fill,omitempty"`
	Alignment *ExcelAlignmentConfig `json:"alignment,omitempty"`
	Border    *ExcelBorderConfig    `json:"border,omitempty"`
}

// ExcelFontConfig 字体配置
type ExcelFontConfig struct {
	Family string  `json:"family,omitempty"`
	Size   float64 `json:"size,omitempty"`
	Bold   bool    `json:"bold,omitempty"`
	Italic bool    `json:"italic,omitempty"`
	Color  string  `json:"color,omitempty"`
}

// ExcelFillConfig 填充配置
type ExcelFillConfig struct {
	Type    string   `json:"type,omitempty"` // pattern, gradient
	Color   []string `json:"color,omitempty"`
	Pattern int      `json:"pattern,omitempty"`
}

// ExcelAlignmentConfig 对齐配置
type ExcelAlignmentConfig struct {
	Horizontal string `json:"horizontal,omitempty"`
	Vertical   string `json:"vertical,omitempty"`
	WrapText   bool   `json:"wrap_text,omitempty"`
}

// ExcelBorderConfig 边框配置
type ExcelBorderConfig struct {
	Type  string `json:"type,omitempty"` // left, right, top, bottom
	Style int    `json:"style,omitempty"`
	Color string `json:"color,omitempty"`
}

// ExcelColorScheme 颜色方案
type ExcelColorScheme struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Accent    string `json:"accent"`
	Success   string `json:"success"`
	Warning   string `json:"warning"`
	Danger    string `json:"danger"`
	Neutral   string `json:"neutral"`
}

// ExcelFontScheme 字体方案
type ExcelFontScheme struct {
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	TitleSize float64 `json:"title_size"`
	BodySize  float64 `json:"body_size"`
}

// ExcelBorderScheme 边框方案
type ExcelBorderScheme struct {
	HeaderStyle int    `json:"header_style"`
	DataStyle   int    `json:"data_style"`
	Color       string `json:"color"`
}

// ChartConfig 图表配置
type ChartConfig struct {
	Type           ChartType           `json:"type"`
	Title          string              `json:"title"`
	XAxis          string              `json:"x_axis"`
	YAxis          string              `json:"y_axis"`
	DataRange      string              `json:"data_range"`
	CategoryRange  string              `json:"category_range"`
	Series         []ChartSeriesConfig `json:"series"`
	ShowLegend     bool                `json:"show_legend"`
	LegendPosition string              `json:"legend_position"`
	ShowValues     bool                `json:"show_values"`
	Width          float64             `json:"width"`
	Height         float64             `json:"height"`
}

// ChartSeriesConfig 图表系列配置
type ChartSeriesConfig struct {
	Name      string `json:"name"`
	DataRange string `json:"data_range"`
	Color     string `json:"color,omitempty"`
	LineStyle string `json:"line_style,omitempty"` // solid, dash, dot
	Marker    string `json:"marker,omitempty"`     // circle, square, triangle
}

// SheetConfig 工作表配置
type SheetConfig struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Fields      []string         `json:"fields,omitempty"`
	Filters     []TemplateFilter `json:"filters,omitempty"`
	Charts      []ChartConfig    `json:"charts,omitempty"`
	Conditional bool             `json:"conditional"`
	FrozenRows  int              `json:"frozen_rows"`
	FrozenCols  int              `json:"frozen_cols"`
	AutoWidth   bool             `json:"auto_width"`
}

// MultiSheetConfig 多工作表配置
type MultiSheetConfig struct {
	Sheets        []SheetConfig `json:"sheets"`
	StyleTemplate string        `json:"style_template,omitempty"`
	GlobalCharts  []ChartConfig `json:"global_charts,omitempty"`
	CreateSummary bool          `json:"create_summary"`
}

// ExcelExporter Excel 导出器
type ExcelExporter struct {
	dataDir string
}

// NewExcelExporter 创建 Excel 导出器
func NewExcelExporter(dataDir string) *ExcelExporter {
	_ = os.MkdirAll(dataDir, 0750)
	return &ExcelExporter{dataDir: dataDir}
}

// Export 导出报表到 Excel 文件
func (e *ExcelExporter) Export(report *GeneratedReport, outputPath string, options ExportOptions) (*ExportResult, error) {
	if outputPath == "" {
		outputPath = e.generateOutputPath()
	}

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建 Excel 文件
	f, err := e.createExcelFile(report, options)
	if err != nil {
		return nil, fmt.Errorf("创建 Excel 文件失败: %w", err)
	}

	// 保存文件
	if err := f.SaveAs(outputPath); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("保存 Excel 文件失败: %w", err)
	}
	_ = f.Close()

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
	defer func() { _ = f.Close() }()

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
			log.Printf("关闭文件失败: %v", err)
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
			log.Printf("创建图表失败: %v", err)
		}
	}

	// 删除默认的 Sheet1
	index, err := f.GetSheetIndex("Sheet1")
	if err == nil && index != 0 {
		_ = f.DeleteSheet("Sheet1")
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

	_ = f.SetDocProps(props)
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
	_ = f.MergeCell(sheetName, "A1", "D1")
	_ = f.SetCellValue(sheetName, "A1", title)
	_ = f.SetCellStyle(sheetName, "A1", "D1", titleStyle)
	_ = f.SetRowHeight(sheetName, 1, 30)

	// 写入生成时间
	_ = f.MergeCell(sheetName, "A2", "D2")
	_ = f.SetCellValue(sheetName, "A2", fmt.Sprintf("生成时间: %s", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	dateStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "#666666"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellStyle(sheetName, "A2", "D2", dateStyle)

	// 写入时间范围（如果有）
	row := 4
	if !report.Period.StartTime.IsZero() {
		_ = f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("报告周期: %s 至 %s",
			report.Period.StartTime.Format("2006-01-02"),
			report.Period.EndTime.Format("2006-01-02")))
		row++
	}

	// 写入总记录数
	_ = f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row))
	_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("总记录数: %d", report.TotalRecords))
	row += 2

	// 写入摘要数据
	if len(report.Summary) > 0 {
		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "摘要指标")
		_ = f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row))
		_ = f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), labelStyle)
		row++

		for key, value := range report.Summary {
			_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), key)
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), labelStyle)
			_ = f.MergeCell(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row))
			_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), e.formatValue(value))
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row), valueStyle)
			row++
		}
	}

	// 设置列宽
	_ = f.SetColWidth(sheetName, "A", "D", 25)

	return nil
}

// createDataSheet 创建数据工作表
func (e *ExcelExporter) createDataSheet(f *excelize.File, report *GeneratedReport, options ExportOptions) error {
	sheetName := "数据"

	// 检查是否已有摘要表，如果有则使用第一个位置
	hasSummary := options.Summary && report.Summary != nil
	if !hasSummary {
		// 使用 Sheet1 作为数据表
		_ = f.SetSheetName("Sheet1", sheetName)
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
		_ = f.SetCellValue(sheetName, "A1", "无数据")
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
		_ = f.SetCellValue(sheetName, cell, label)
		_ = f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// 写入数据
	for rowIdx, row := range report.Data {
		rowNum := rowIdx + 2
		for colIdx, field := range fields {
			col := e.getColumnLetter(colIdx + 1)
			cell := fmt.Sprintf("%s%d", col, rowNum)
			value := row[field]
			_ = f.SetCellValue(sheetName, cell, e.formatValue(value))
			_ = f.SetCellStyle(sheetName, cell, cell, dataStyle)
		}
	}

	// 设置自动筛选
	if len(fields) > 0 {
		lastCol := e.getColumnLetter(len(fields))
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:%s%d", lastCol, len(report.Data)+1), []excelize.AutoFilterOptions{})
	}

	// 冻结首行
	_ = f.SetPanes(sheetName, &excelize.Panes{
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
		_ = f.SetColWidth(sheetName, col, col, 18)
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
	_ = f.DeleteSheet("Sheet1")

	// 导出每个报表到不同工作表
	for i, report := range reports {
		sheetName := m.sanitizeSheetName(report.Name, i)
		if _, err := f.NewSheet(sheetName); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("创建工作表失败: %w", err)
		}

		// 创建数据表内容
		if err := m.createSheetContent(f, sheetName, report, options); err != nil {
			_ = f.Close()
			return nil, err
		}
	}

	// 保存文件
	if err := f.SaveAs(outputPath); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}
	_ = f.Close()

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
		_ = f.SetCellValue(sheetName, "A1", "无数据")
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
		_ = f.SetCellValue(sheetName, fmt.Sprintf("%s1", col), field)
		_ = f.SetCellStyle(sheetName, fmt.Sprintf("%s1", col), fmt.Sprintf("%s1", col), headerStyle)
	}

	// 写入数据
	for rowIdx, row := range report.Data {
		rowNum := rowIdx + 2
		for colIdx, field := range fields {
			col := m.getColumnLetter(colIdx + 1)
			_ = f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, rowNum), m.formatValue(row[field]))
		}
	}

	// 设置列宽
	for i := range fields {
		col := m.getColumnLetter(i + 1)
		_ = f.SetColWidth(sheetName, col, col, 18)
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
	defer func() { _ = f.Close() }()

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
					_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", colName, rowIdx+1), newValue)
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

// ========== v2.35.0 增强功能：多图表支持 ==========

// AdvancedExcelExporter 高级 Excel 导出器
type AdvancedExcelExporter struct {
	exporter       *ExcelExporter
	styleTemplates map[string]*ExcelStyleTemplate
}

// NewAdvancedExcelExporter 创建高级 Excel 导出器
func NewAdvancedExcelExporter(exporter *ExcelExporter) *AdvancedExcelExporter {
	adv := &AdvancedExcelExporter{
		exporter:       exporter,
		styleTemplates: make(map[string]*ExcelStyleTemplate),
	}
	// 注册内置样式模板
	adv.registerBuiltinTemplates()
	return adv
}

// registerBuiltinTemplates 注册内置样式模板
func (a *AdvancedExcelExporter) registerBuiltinTemplates() {
	// 默认蓝色主题
	a.styleTemplates["default"] = &ExcelStyleTemplate{
		ID:          "default",
		Name:        "默认蓝色主题",
		Description: "简洁的蓝色主题，适合通用报表",
		Header: ExcelCellStyle{
			Font:      &ExcelFontConfig{Bold: true, Size: 11, Color: "#FFFFFF"},
			Fill:      &ExcelFillConfig{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
			Alignment: &ExcelAlignmentConfig{Horizontal: "center", Vertical: "center"},
		},
		Colors: ExcelColorScheme{
			Primary: "#4472C4", Secondary: "#5B9BD5", Accent: "#70AD47",
			Success: "#70AD47", Warning: "#FFC000", Danger: "#C00000", Neutral: "#7F7F7F",
		},
		Fonts: ExcelFontScheme{Title: "微软雅黑", Body: "微软雅黑", TitleSize: 14, BodySize: 11},
	}

	// 专业绿色主题
	a.styleTemplates["professional"] = &ExcelStyleTemplate{
		ID:          "professional",
		Name:        "专业绿色主题",
		Description: "专业的绿色主题，适合财务报表",
		Header: ExcelCellStyle{
			Font:      &ExcelFontConfig{Bold: true, Size: 11, Color: "#FFFFFF"},
			Fill:      &ExcelFillConfig{Type: "pattern", Color: []string{"#548235"}, Pattern: 1},
			Alignment: &ExcelAlignmentConfig{Horizontal: "center", Vertical: "center"},
		},
		Colors: ExcelColorScheme{
			Primary: "#548235", Secondary: "#70AD47", Accent: "#4472C4",
			Success: "#70AD47", Warning: "#FFC000", Danger: "#C00000", Neutral: "#7F7F7F",
		},
		Fonts: ExcelFontScheme{Title: "微软雅黑", Body: "微软雅黑", TitleSize: 14, BodySize: 11},
	}

	// 简约灰色主题
	a.styleTemplates["minimal"] = &ExcelStyleTemplate{
		ID:          "minimal",
		Name:        "简约灰色主题",
		Description: "简约的灰色主题，适合数据分析",
		Header: ExcelCellStyle{
			Font:      &ExcelFontConfig{Bold: true, Size: 11, Color: "#333333"},
			Fill:      &ExcelFillConfig{Type: "pattern", Color: []string{"#E8E8E8"}, Pattern: 1},
			Alignment: &ExcelAlignmentConfig{Horizontal: "center", Vertical: "center"},
		},
		Colors: ExcelColorScheme{
			Primary: "#595959", Secondary: "#808080", Accent: "#4472C4",
			Success: "#70AD47", Warning: "#FFC000", Danger: "#C00000", Neutral: "#A6A6A6",
		},
		Fonts: ExcelFontScheme{Title: "微软雅黑", Body: "微软雅黑", TitleSize: 14, BodySize: 11},
	}
}

// ExportWithCharts 导出带图表的报表
func (a *AdvancedExcelExporter) ExportWithCharts(report *GeneratedReport, outputPath string, charts []ChartConfig, templateID string) (*ExportResult, error) {
	f := excelize.NewFile()

	// 获取样式模板
	tmpl := a.styleTemplates["default"]
	if templateID != "" {
		if t, ok := a.styleTemplates[templateID]; ok {
			tmpl = t
		}
	}

	// 创建数据工作表
	dataSheet := "数据"
	if _, err := f.NewSheet(dataSheet); err != nil {
		_ = f.Close()
		return nil, err
	}
	_ = f.DeleteSheet("Sheet1")

	// 写入数据
	if err := a.writeDataWithTemplate(f, dataSheet, report, tmpl); err != nil {
		_ = f.Close()
		return nil, err
	}

	// 创建图表工作表
	chartSheet := "图表"
	if _, err := f.NewSheet(chartSheet); err != nil {
		_ = f.Close()
		return nil, err
	}

	// 创建每个图表
	for i := range charts {
		if err := a.createChart(f, chartSheet, dataSheet, &charts[i], i); err != nil {
			// 图表创建失败不中断
			log.Printf("创建图表失败: %v", err)
		}
	}

	// 设置活动表
	f.SetActiveSheet(0)

	// 保存文件
	if err := f.SaveAs(outputPath); err != nil {
		_ = f.Close()
		return nil, err
	}
	_ = f.Close()

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

// ExportMultiSheet 导出多工作表报表
func (a *AdvancedExcelExporter) ExportMultiSheet(report *GeneratedReport, config *MultiSheetConfig, outputPath string) (*ExportResult, error) {
	f := excelize.NewFile()
	_ = f.DeleteSheet("Sheet1")

	// 获取样式模板
	tmpl := a.styleTemplates["default"]
	if config.StyleTemplate != "" {
		if t, ok := a.styleTemplates[config.StyleTemplate]; ok {
			tmpl = t
		}
	}

	// 创建汇总表
	if config.CreateSummary {
		summarySheet := "汇总"
		if _, err := f.NewSheet(summarySheet); err == nil {
			_ = a.createSummarySheet(f, summarySheet, report, tmpl)
		}
	}

	// 创建各个工作表
	for _, sheetConfig := range config.Sheets {
		if _, err := f.NewSheet(sheetConfig.Name); err != nil {
			continue
		}

		// 写入数据
		if err := a.writeSheetData(f, sheetConfig, report, tmpl); err != nil {
			log.Printf("写入工作表 %s 失败: %v", sheetConfig.Name, err)
			continue
		}

		// 创建图表
		for _, chartConfig := range sheetConfig.Charts {
			if err := a.createChart(f, sheetConfig.Name, sheetConfig.Name, &chartConfig, 0); err != nil {
				log.Printf("创建图表失败: %v", err)
			}
		}

		// 设置冻结
		if sheetConfig.FrozenRows > 0 || sheetConfig.FrozenCols > 0 {
			_ = f.SetPanes(sheetConfig.Name, &excelize.Panes{
				Freeze:      true,
				XSplit:      sheetConfig.FrozenCols,
				YSplit:      sheetConfig.FrozenRows,
				TopLeftCell: fmt.Sprintf("%s%d", a.getColumnLetter(sheetConfig.FrozenCols+1), sheetConfig.FrozenRows+1),
			})
		}
	}

	// 创建全局图表
	if len(config.GlobalCharts) > 0 && len(config.Sheets) > 0 {
		chartSheet := "总览图表"
		if _, err := f.NewSheet(chartSheet); err == nil {
			for i := range config.GlobalCharts {
				_ = a.createChart(f, chartSheet, config.Sheets[0].Name, &config.GlobalCharts[i], i)
			}
		}
	}

	// 保存
	if err := f.SaveAs(outputPath); err != nil {
		_ = f.Close()
		return nil, err
	}
	_ = f.Close()

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

// writeDataWithTemplate 使用模板写入数据
func (a *AdvancedExcelExporter) writeDataWithTemplate(f *excelize.File, sheet string, report *GeneratedReport, tmpl *ExcelStyleTemplate) error {
	if len(report.Data) == 0 {
		_ = f.SetCellValue(sheet, "A1", "无数据")
		return nil
	}

	fields := a.getFieldOrder(report.Data[0])

	// 创建样式
	headerStyleID, _ := f.NewStyle(a.convertToExcelizeStyle(tmpl.Header))
	dataStyleID, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#D9D9D9", Style: 1},
			{Type: "right", Color: "#D9D9D9", Style: 1},
			{Type: "top", Color: "#D9D9D9", Style: 1},
			{Type: "bottom", Color: "#D9D9D9", Style: 1},
		},
	})

	// 写入表头
	for i, field := range fields {
		col := a.getColumnLetter(i + 1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s1", col), field)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s1", col), fmt.Sprintf("%s1", col), headerStyleID)
	}

	// 写入数据
	for rowIdx, row := range report.Data {
		rowNum := rowIdx + 2
		for colIdx, field := range fields {
			col := a.getColumnLetter(colIdx + 1)
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, rowNum), a.formatValue(row[field]))
			_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", col, rowNum), fmt.Sprintf("%s%d", col, rowNum), dataStyleID)
		}
	}

	// 设置列宽
	for i := range fields {
		col := a.getColumnLetter(i + 1)
		_ = f.SetColWidth(sheet, col, col, 18)
	}

	return nil
}

// writeSheetData 写入工作表数据
func (a *AdvancedExcelExporter) writeSheetData(f *excelize.File, config SheetConfig, report *GeneratedReport, tmpl *ExcelStyleTemplate) error {
	// 应用过滤器获取数据
	data := a.filterData(report.Data, config.Filters)

	if len(data) == 0 {
		_ = f.SetCellValue(config.Name, "A1", "无数据")
		return nil
	}

	// 获取字段
	fields := config.Fields
	if len(fields) == 0 {
		fields = a.getFieldOrder(data[0])
	}

	// 创建样式
	headerStyleID, _ := f.NewStyle(a.convertToExcelizeStyle(tmpl.Header))

	// 写入表头
	for i, field := range fields {
		col := a.getColumnLetter(i + 1)
		_ = f.SetCellValue(config.Name, fmt.Sprintf("%s1", col), field)
		_ = f.SetCellStyle(config.Name, fmt.Sprintf("%s1", col), fmt.Sprintf("%s1", col), headerStyleID)
	}

	// 写入数据
	dataStyleID, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	for rowIdx, row := range data {
		rowNum := rowIdx + 2
		for colIdx, field := range fields {
			col := a.getColumnLetter(colIdx + 1)
			_ = f.SetCellValue(config.Name, fmt.Sprintf("%s%d", col, rowNum), a.formatValue(row[field]))
			_ = f.SetCellStyle(config.Name, fmt.Sprintf("%s%d", col, rowNum), fmt.Sprintf("%s%d", col, rowNum), dataStyleID)
		}
	}

	// 自动调整列宽
	if config.AutoWidth {
		for i := range fields {
			col := a.getColumnLetter(i + 1)
			_ = f.SetColWidth(config.Name, col, col, 18)
		}
	}

	return nil
}

// createSummarySheet 创建汇总表
func (a *AdvancedExcelExporter) createSummarySheet(f *excelize.File, sheet string, report *GeneratedReport, tmpl *ExcelStyleTemplate) error {
	titleStyleID, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	_ = f.MergeCell(sheet, "A1", "D1")
	_ = f.SetCellValue(sheet, "A1", report.Name)
	_ = f.SetCellStyle(sheet, "A1", "D1", titleStyleID)

	// 写入生成时间
	_ = f.SetCellValue(sheet, "A2", fmt.Sprintf("生成时间: %s", report.GeneratedAt.Format("2006-01-02 15:04:05")))

	// 写入摘要
	if report.Summary != nil {
		row := 4
		for key, value := range report.Summary {
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), key)
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("%v", value))
			row++
		}
	}

	return nil
}

// createChart 创建图表
func (a *AdvancedExcelExporter) createChart(f *excelize.File, chartSheet, dataSheet string, config *ChartConfig, index int) error {
	// 计算图表位置
	startRow := index*15 + 1
	startCol := 1

	// 确定图表类型
	var chartType excelize.ChartType
	switch config.Type {
	case ChartTypeBar:
		chartType = excelize.Col
	case ChartTypeLine:
		chartType = excelize.Line
	case ChartTypePie:
		chartType = excelize.Pie
	case ChartTypeArea:
		chartType = excelize.Area
	case ChartTypeScatter:
		chartType = excelize.Scatter
	case ChartTypeDoughnut:
		chartType = excelize.Doughnut
	default:
		chartType = excelize.Col
	}

	// 构建系列
	series := make([]excelize.ChartSeries, 0, len(config.Series))
	if len(config.Series) > 0 {
		for _, s := range config.Series {
			series = append(series, excelize.ChartSeries{
				Name:       s.Name,
				Categories: fmt.Sprintf("%s!%s", dataSheet, config.CategoryRange),
				Values:     fmt.Sprintf("%s!%s", dataSheet, s.DataRange),
			})
		}
	} else if config.DataRange != "" {
		// 使用默认系列
		series = append(series, excelize.ChartSeries{
			Name:       config.Title,
			Categories: fmt.Sprintf("%s!%s", dataSheet, config.CategoryRange),
			Values:     fmt.Sprintf("%s!%s", dataSheet, config.DataRange),
		})
	}

	if len(series) == 0 {
		return fmt.Errorf("没有图表数据")
	}

	// 创建图表
	chart := &excelize.Chart{
		Type:   chartType,
		Series: series,
		Title:  []excelize.RichTextRun{{Text: config.Title}},
		Legend: excelize.ChartLegend{Position: config.LegendPosition},
		PlotArea: excelize.ChartPlotArea{
			ShowVal: config.ShowValues,
		},
	}

	// 设置图表位置
	chartCell := fmt.Sprintf("%s%d", a.getColumnLetter(startCol), startRow)

	return f.AddChart(chartSheet, chartCell, chart)
}

// convertToExcelizeStyle 转换样式
func (a *AdvancedExcelExporter) convertToExcelizeStyle(style ExcelCellStyle) *excelize.Style {
	excelStyle := &excelize.Style{}

	if style.Font != nil {
		excelStyle.Font = &excelize.Font{
			Family: style.Font.Family,
			Size:   style.Font.Size,
			Bold:   style.Font.Bold,
			Italic: style.Font.Italic,
			Color:  style.Font.Color,
		}
	}

	if style.Fill != nil {
		excelStyle.Fill = excelize.Fill{
			Type:    style.Fill.Type,
			Color:   style.Fill.Color,
			Pattern: style.Fill.Pattern,
		}
	}

	if style.Alignment != nil {
		excelStyle.Alignment = &excelize.Alignment{
			Horizontal: style.Alignment.Horizontal,
			Vertical:   style.Alignment.Vertical,
			WrapText:   style.Alignment.WrapText,
		}
	}

	return excelStyle
}

// filterData 过滤数据
func (a *AdvancedExcelExporter) filterData(data []map[string]interface{}, filters []TemplateFilter) []map[string]interface{} {
	if len(filters) == 0 {
		return data
	}

	result := make([]map[string]interface{}, 0)
	for _, row := range data {
		if a.matchesFilters(row, filters) {
			result = append(result, row)
		}
	}
	return result
}

// matchesFilters 检查是否匹配过滤器
func (a *AdvancedExcelExporter) matchesFilters(row map[string]interface{}, filters []TemplateFilter) bool {
	for _, f := range filters {
		val, ok := row[f.Field]
		if !ok {
			return false
		}

		switch f.Operator {
		case "eq":
			if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", f.Value) {
				return false
			}
		case "ne":
			if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", f.Value) {
				return false
			}
		case "gt", "lt", "gte", "lte":
			// 数值比较简化处理
			valFloat, ok1 := toFloat64(val)
			filterFloat, ok2 := toFloat64(f.Value)
			if !ok1 || !ok2 {
				return false
			}
			switch f.Operator {
			case "gt":
				if valFloat <= filterFloat {
					return false
				}
			case "lt":
				if valFloat >= filterFloat {
					return false
				}
			case "gte":
				if valFloat < filterFloat {
					return false
				}
			case "lte":
				if valFloat > filterFloat {
					return false
				}
			}
		}
	}
	return true
}

// getFieldOrder 获取字段顺序
func (a *AdvancedExcelExporter) getFieldOrder(row map[string]interface{}) []string {
	fields := make([]string, 0, len(row))
	for key := range row {
		fields = append(fields, key)
	}
	return fields
}

// getColumnLetter 将列号转换为 Excel 列字母
func (a *AdvancedExcelExporter) getColumnLetter(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

// formatValue 格式化值
func (a *AdvancedExcelExporter) formatValue(val interface{}) interface{} {
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

// RegisterStyleTemplate 注册自定义样式模板
func (a *AdvancedExcelExporter) RegisterStyleTemplate(template *ExcelStyleTemplate) {
	a.styleTemplates[template.ID] = template
}

// GetStyleTemplate 获取样式模板
func (a *AdvancedExcelExporter) GetStyleTemplate(id string) (*ExcelStyleTemplate, bool) {
	t, ok := a.styleTemplates[id]
	return t, ok
}

// ListStyleTemplates 列出所有样式模板
func (a *AdvancedExcelExporter) ListStyleTemplates() []*ExcelStyleTemplate {
	templates := make([]*ExcelStyleTemplate, 0, len(a.styleTemplates))
	for _, t := range a.styleTemplates {
		templates = append(templates, t)
	}
	return templates
}
