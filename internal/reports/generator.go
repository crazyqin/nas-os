// Package reports 提供报表生成和管理功能
package reports

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 报表生成器 ==========

// ReportGenerator 报表生成器.
type ReportGenerator struct {
	mu              sync.RWMutex
	templateManager *TemplateManager
	DataSources     map[string]DataSource // 导出供 handlers 访问
	customReports   map[string]*CustomReport
	dataDir         string
}

// NewReportGenerator 创建报表生成器.
func NewReportGenerator(templateManager *TemplateManager, dataDir string) *ReportGenerator {
	rg := &ReportGenerator{
		templateManager: templateManager,
		DataSources:     make(map[string]DataSource),
		customReports:   make(map[string]*CustomReport),
		dataDir:         dataDir,
	}

	_ = os.MkdirAll(dataDir, 0750)
	rg.loadCustomReports()

	return rg
}

// RegisterDataSource 注册数据源.
func (rg *ReportGenerator) RegisterDataSource(source DataSource) {
	rg.DataSources[source.Name()] = source
}

// loadCustomReports 加载自定义报表.
func (rg *ReportGenerator) loadCustomReports() {
	files, err := filepath.Glob(filepath.Join(rg.dataDir, "custom_*.json"))
	if err != nil {
		return
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var report CustomReport
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}

		rg.customReports[report.ID] = &report
	}
}

// saveCustomReport 保存自定义报表.
func (rg *ReportGenerator) saveCustomReport(report *CustomReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(rg.dataDir, "custom_"+report.ID+".json")
	return os.WriteFile(path, data, 0640)
}

// CreateCustomReport 创建自定义报表.
func (rg *ReportGenerator) CreateCustomReport(input CustomReportInput, createdBy string) (*CustomReport, error) {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	// 验证数据源
	if _, exists := rg.DataSources[input.DataSource]; !exists {
		return nil, ErrDataSourceNotFound
	}

	// 如果基于模板，验证模板存在
	if input.TemplateID != "" {
		if _, err := rg.templateManager.GetTemplate(input.TemplateID); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	report := &CustomReport{
		ID:           uuid.New().String(),
		Name:         input.Name,
		Description:  input.Description,
		TemplateID:   input.TemplateID,
		DataSource:   input.DataSource,
		Query:        input.Query,
		Fields:       input.Fields,
		Filters:      input.Filters,
		Sorts:        input.Sorts,
		Aggregations: input.Aggregations,
		GroupBy:      input.GroupBy,
		Parameters:   input.Parameters,
		Limit:        input.Limit,
		Offset:       input.Offset,
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    createdBy,
	}

	rg.customReports[report.ID] = report

	if err := rg.saveCustomReport(report); err != nil {
		delete(rg.customReports, report.ID)
		return nil, err
	}

	return report, nil
}

// GetCustomReport 获取自定义报表.
func (rg *ReportGenerator) GetCustomReport(id string) (*CustomReport, error) {
	rg.mu.RLock()
	defer rg.mu.RUnlock()

	report, exists := rg.customReports[id]
	if !exists {
		return nil, ErrReportNotFound
	}

	return report, nil
}

// ListCustomReports 列出自定义报表.
func (rg *ReportGenerator) ListCustomReports(dataSource string) []*CustomReport {
	rg.mu.RLock()
	defer rg.mu.RUnlock()

	result := make([]*CustomReport, 0)
	for _, r := range rg.customReports {
		if dataSource != "" && r.DataSource != dataSource {
			continue
		}
		result = append(result, r)
	}

	return result
}

// UpdateCustomReport 更新自定义报表.
func (rg *ReportGenerator) UpdateCustomReport(id string, input CustomReportInput) (*CustomReport, error) {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	report, exists := rg.customReports[id]
	if !exists {
		return nil, ErrReportNotFound
	}

	// 验证数据源
	if _, exists := rg.DataSources[input.DataSource]; !exists {
		return nil, ErrDataSourceNotFound
	}

	report.Name = input.Name
	report.Description = input.Description
	report.TemplateID = input.TemplateID
	report.DataSource = input.DataSource
	report.Query = input.Query
	report.Fields = input.Fields
	report.Filters = input.Filters
	report.Sorts = input.Sorts
	report.Aggregations = input.Aggregations
	report.GroupBy = input.GroupBy
	report.Parameters = input.Parameters
	report.Limit = input.Limit
	report.Offset = input.Offset
	report.UpdatedAt = time.Now()

	if err := rg.saveCustomReport(report); err != nil {
		return nil, err
	}

	return report, nil
}

// DeleteCustomReport 删除自定义报表.
func (rg *ReportGenerator) DeleteCustomReport(id string) error {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	if _, exists := rg.customReports[id]; !exists {
		return ErrReportNotFound
	}

	delete(rg.customReports, id)

	path := filepath.Join(rg.dataDir, "custom_"+id+".json")
	return os.Remove(path)
}

// GenerateFromTemplate 从模板生成报表.
func (rg *ReportGenerator) GenerateFromTemplate(templateID string, parameters map[string]interface{}, period *ReportPeriod) (*GeneratedReport, error) {
	template, err := rg.templateManager.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	return rg.generateReport(template.Fields, template.Filters, template.Sorts,
		template.Aggregations, template.GroupBy, template.Limit, template.Offset,
		parameters, period, template.Name)
}

// GenerateFromCustomReport 从自定义报表生成.
func (rg *ReportGenerator) GenerateFromCustomReport(reportID string, parameters map[string]interface{}, period *ReportPeriod) (*GeneratedReport, error) {
	report, err := rg.GetCustomReport(reportID)
	if err != nil {
		return nil, err
	}

	// 合并参数
	mergedParams := make(map[string]interface{})
	for k, v := range report.Parameters {
		mergedParams[k] = v
	}
	for k, v := range parameters {
		mergedParams[k] = v
	}

	return rg.generateReport(report.Fields, report.Filters, report.Sorts,
		report.Aggregations, report.GroupBy, report.Limit, report.Offset,
		mergedParams, period, report.Name)
}

// generateReport 核心生成逻辑.
func (rg *ReportGenerator) generateReport(
	fields []TemplateField,
	filters []TemplateFilter,
	sorts []TemplateSort,
	aggregations []TemplateAggregation,
	groupBy []string,
	limit, offset int,
	parameters map[string]interface{},
	period *ReportPeriod,
	name string,
) (*GeneratedReport, error) {

	// 确定数据源
	dataSourceName := ""
	if ds, ok := parameters["data_source"].(string); ok {
		dataSourceName = ds
	} else {
		// 默认使用第一个注册的数据源
		for name := range rg.DataSources {
			dataSourceName = name
			break
		}
	}

	dataSource, exists := rg.DataSources[dataSourceName]
	if !exists {
		return nil, ErrDataSourceNotFound
	}

	// 应用参数到过滤器
	appliedFilters := rg.applyParametersToFilters(filters, parameters)

	// 添加时间范围过滤器
	if period != nil {
		appliedFilters = append(appliedFilters, TemplateFilter{
			Field:    "timestamp",
			Operator: "gte",
			Value:    period.StartTime,
		}, TemplateFilter{
			Field:    "timestamp",
			Operator: "lte",
			Value:    period.EndTime,
		})
	}

	// 查询数据
	query := make(map[string]interface{})
	if q, ok := parameters["query"].(map[string]interface{}); ok {
		query = q
	}

	data, err := dataSource.Query(query, fields, appliedFilters, sorts, aggregations, groupBy, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询数据失败: %w", err)
	}

	// 应用聚合
	if len(aggregations) > 0 {
		data = rg.applyAggregations(data, aggregations)
	}

	// 应用排序
	if len(sorts) > 0 {
		data = rg.applySorts(data, sorts)
	}

	// 应用分页
	if limit > 0 {
		start := offset
		end := offset + limit
		if start > len(data) {
			data = []map[string]interface{}{}
		} else {
			if end > len(data) {
				end = len(data)
			}
			data = data[start:end]
		}
	}

	// 格式化字段
	data = rg.formatFields(data, fields)

	// 获取摘要
	summary, _ := dataSource.GetSummary(query)

	now := time.Now()
	report := &GeneratedReport{
		ID:           uuid.New().String(),
		Name:         name,
		GeneratedAt:  now,
		Parameters:   parameters,
		Summary:      summary,
		Data:         data,
		TotalRecords: len(data),
	}

	if period != nil {
		report.Period = *period
	}

	return report, nil
}

// applyParametersToFilters 将参数应用到过滤器.
func (rg *ReportGenerator) applyParametersToFilters(filters []TemplateFilter, parameters map[string]interface{}) []TemplateFilter {
	result := make([]TemplateFilter, len(filters))
	copy(result, filters)

	for i, f := range result {
		// 检查参数中是否有对应的值
		if paramValue, exists := parameters[f.Field]; exists {
			result[i].Value = paramValue
		}
	}

	return result
}

// applyAggregations 应用聚合.
func (rg *ReportGenerator) applyAggregations(data []map[string]interface{}, aggregations []TemplateAggregation) []map[string]interface{} {
	if len(aggregations) == 0 {
		return data
	}

	result := make([]map[string]interface{}, 0)

	for _, agg := range aggregations {
		aggResult := make(map[string]interface{})
		alias := agg.Alias
		if alias == "" {
			alias = agg.Function + "_" + agg.Field
		}

		values := make([]float64, 0)
		for _, row := range data {
			if val, ok := row[agg.Field]; ok {
				switch v := val.(type) {
				case float64:
					values = append(values, v)
				case int:
					values = append(values, float64(v))
				case int64:
					values = append(values, float64(v))
				case uint64:
					values = append(values, float64(v))
				}
			}
		}

		if len(values) == 0 {
			continue
		}

		switch agg.Function {
		case "sum":
			var sum float64
			for _, v := range values {
				sum += v
			}
			aggResult[alias] = sum
		case "avg":
			var sum float64
			for _, v := range values {
				sum += v
			}
			aggResult[alias] = sum / float64(len(values))
		case "count":
			aggResult[alias] = len(values)
		case "min":
			min := values[0]
			for _, v := range values {
				if v < min {
					min = v
				}
			}
			aggResult[alias] = min
		case "max":
			max := values[0]
			for _, v := range values {
				if v > max {
					max = v
				}
			}
			aggResult[alias] = max
		}

		result = append(result, aggResult)
	}

	return result
}

// applySorts 应用排序.
func (rg *ReportGenerator) applySorts(data []map[string]interface{}, sorts []TemplateSort) []map[string]interface{} {
	if len(sorts) == 0 {
		return data
	}

	result := make([]map[string]interface{}, len(data))
	copy(result, data)

	// 多字段排序（从后往前）
	for i := len(sorts) - 1; i >= 0; i-- {
		s := sorts[i]
		sort.Slice(result, func(a, b int) bool {
			valA := result[a][s.Field]
			valB := result[b][s.Field]

			cmp := compareValues(valA, valB)
			if s.Order == "desc" {
				return cmp > 0
			}
			return cmp < 0
		})
	}

	return result
}

// compareValues 比较两个值.
func compareValues(a, b interface{}) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	// 尝试数值比较
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)
	if aOk && bOk {
		if aFloat < bFloat {
			return -1
		} else if aFloat > bFloat {
			return 1
		}
		return 0
	}

	// 字符串比较
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Compare(aStr, bStr)
}

// toFloat64 转换为 float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int()), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint()), true
		case reflect.Float32, reflect.Float64:
			return rv.Float(), true
		}
	}
	return 0, false
}

// formatFields 格式化字段.
func (rg *ReportGenerator) formatFields(data []map[string]interface{}, fields []TemplateField) []map[string]interface{} {
	for i, row := range data {
		for _, field := range fields {
			if val, exists := row[field.Name]; exists {
				row[field.Name] = rg.formatValue(val, field.Type, field.Format)
			}
		}
		data[i] = row
	}
	return data
}

// formatValue 格式化值.
func (rg *ReportGenerator) formatValue(val interface{}, fieldType FieldType, format string) interface{} {
	switch fieldType {
	case FieldTypeBytes:
		if v, ok := toFloat64(val); ok {
			return formatBytes(uint64(v))
		}
	case FieldTypePercent:
		if v, ok := toFloat64(val); ok {
			return fmt.Sprintf("%.2f%%", v)
		}
	case FieldTypeDate:
		if t, ok := val.(time.Time); ok {
			if format != "" {
				return t.Format(format)
			}
			return t.Format("2006-01-02")
		}
	case FieldTypeDateTime:
		if t, ok := val.(time.Time); ok {
			if format != "" {
				return t.Format(format)
			}
			return t.Format("2006-01-02 15:04:05")
		}
	case FieldTypeDuration:
		if v, ok := toFloat64(val); ok {
			d := time.Duration(int64(v)) * time.Second
			return formatDuration(d)
		}
	}
	return val
}

// formatBytes 格式化字节.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration 格式化持续时间.
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时 %d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}

// GenerateQuickReport 快速生成报表（不保存配置）.
func (rg *ReportGenerator) GenerateQuickReport(
	dataSource string,
	fields []TemplateField,
	filters []TemplateFilter,
	sorts []TemplateSort,
	limit int,
	period *ReportPeriod,
) (*GeneratedReport, error) {
	parameters := map[string]interface{}{
		"data_source": dataSource,
	}

	return rg.generateReport(fields, filters, sorts, nil, nil, limit, 0, parameters, period, "快速报表")
}

// PreviewReport 预览报表（限制条数）.
func (rg *ReportGenerator) PreviewReport(templateID string, parameters map[string]interface{}) (*GeneratedReport, error) {
	// 限制预览条数为 10 条
	return rg.GenerateFromTemplate(templateID, parameters, nil)
}

// GetAvailableFields 获取数据源可用字段.
func (rg *ReportGenerator) GetAvailableFields(dataSource string) ([]TemplateField, error) {
	ds, exists := rg.DataSources[dataSource]
	if !exists {
		return nil, ErrDataSourceNotFound
	}

	return ds.GetAvailableFields(), nil
}

// ListDataSources 列出数据源.
func (rg *ReportGenerator) ListDataSources() []string {
	result := make([]string, 0, len(rg.DataSources))
	for name := range rg.DataSources {
		result = append(result, name)
	}
	return result
}
