// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"sort"
	"time"
)

// ========== 配额数据源适配器 ==========

// QuotaDataSource 配额数据源
type QuotaDataSource struct {
	quotaManager interface {
		GetAllUsage() ([]interface{}, error)
		GetUserUsage(username string) ([]interface{}, error)
		GetAlerts() []interface{}
	}
}

// NewQuotaDataSource 创建配额数据源
func NewQuotaDataSource(manager interface{}) *QuotaDataSource {
	quotaManager, ok := manager.(interface {
		GetAllUsage() ([]interface{}, error)
		GetUserUsage(username string) ([]interface{}, error)
		GetAlerts() []interface{}
	})
	if !ok {
		return &QuotaDataSource{quotaManager: nil}
	}
	return &QuotaDataSource{
		quotaManager: quotaManager,
	}
}

// Name 数据源名称
func (ds *QuotaDataSource) Name() string {
	return "quota"
}

// Query 查询数据
func (ds *QuotaDataSource) Query(
	query map[string]interface{},
	fields []TemplateField,
	filters []TemplateFilter,
	sorts []TemplateSort,
	aggregations []TemplateAggregation,
	groupBy []string,
	limit, offset int,
) ([]map[string]interface{}, error) {

	// 获取配额使用数据
	usages, err := ds.quotaManager.GetAllUsage()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(usages))

	for _, usage := range usages {
		// 转换为 map
		row := ds.convertToMap(usage)

		// 应用过滤器
		if !ds.matchFilters(row, filters) {
			continue
		}

		// 提取指定字段
		extractedRow := make(map[string]interface{})
		for _, field := range fields {
			if val, ok := ds.getNestedValue(row, field.Source); ok {
				extractedRow[field.Name] = val
			}
		}

		result = append(result, extractedRow)
	}

	// 应用排序
	if len(sorts) > 0 {
		result = ds.applySorts(result, sorts)
	}

	// 应用分页
	if limit > 0 || offset > 0 {
		start := offset
		if start > len(result) {
			return []map[string]interface{}{}, nil
		}
		end := len(result)
		if limit > 0 && start+limit < end {
			end = start + limit
		}
		result = result[start:end]
	}

	return result, nil
}

// GetSummary 获取摘要
func (ds *QuotaDataSource) GetSummary(query map[string]interface{}) (map[string]interface{}, error) {
	usages, err := ds.quotaManager.GetAllUsage()
	if err != nil {
		return nil, err
	}

	summary := map[string]interface{}{
		"total_quotas":    len(usages),
		"total_limit":     0,
		"total_used":      0,
		"over_soft_limit": 0,
		"over_hard_limit": 0,
	}

	// 使用 int64 存储累加值，防止溢出
	var totalLimit, totalUsed int64

	for _, usage := range usages {
		row := ds.convertToMap(usage)

		if limit, ok := row["hard_limit"].(uint64); ok {
			// 安全转换：检查溢出
			if limit > uint64(1<<63-1) {
				totalLimit = 1<<63 - 1
			} else {
				totalLimit += int64(limit)
			}
		}
		if used, ok := row["used_bytes"].(uint64); ok {
			// 安全转换：检查溢出
			if used > uint64(1<<63-1) {
				totalUsed = 1<<63 - 1
			} else {
				totalUsed += int64(used)
			}
		}
		if overSoft, ok := row["is_over_soft"].(bool); ok && overSoft {
			if overSoftLimit, ok := summary["over_soft_limit"].(int); ok {
				summary["over_soft_limit"] = overSoftLimit + 1
			}
		}
		if overHard, ok := row["is_over_hard"].(bool); ok && overHard {
			if overHardLimit, ok := summary["over_hard_limit"].(int); ok {
				summary["over_hard_limit"] = overHardLimit + 1
			}
		}
	}

	// 更新 summary 为 int64 类型
	summary["total_limit"] = totalLimit
	summary["total_used"] = totalUsed

	return summary, nil
}

// GetAvailableFields 获取可用字段
func (ds *QuotaDataSource) GetAvailableFields() []TemplateField {
	return []TemplateField{
		{Name: "quota_id", Label: "配额ID", Type: FieldTypeString, Source: "quota.id", Sortable: true},
		{Name: "type", Label: "类型", Type: FieldTypeString, Source: "quota.type", Filterable: true},
		{Name: "target_id", Label: "目标ID", Type: FieldTypeString, Source: "quota.target_id", Sortable: true, Filterable: true},
		{Name: "target_name", Label: "目标名称", Type: FieldTypeString, Source: "quota.target_name", Sortable: true, Filterable: true},
		{Name: "volume_name", Label: "卷名", Type: FieldTypeString, Source: "quota.volume_name", Sortable: true, Filterable: true},
		{Name: "path", Label: "路径", Type: FieldTypeString, Source: "quota.path"},
		{Name: "hard_limit", Label: "硬限制", Type: FieldTypeBytes, Source: "quota.hard_limit", Sortable: true},
		{Name: "soft_limit", Label: "软限制", Type: FieldTypeBytes, Source: "quota.soft_limit", Sortable: true},
		{Name: "used_bytes", Label: "已使用", Type: FieldTypeBytes, Source: "usage.used_bytes", Sortable: true},
		{Name: "available", Label: "可用空间", Type: FieldTypeBytes, Source: "usage.available", Sortable: true},
		{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent, Source: "usage.usage_percent", Sortable: true},
		{Name: "is_over_soft", Label: "超软限制", Type: FieldTypeBoolean, Source: "usage.is_over_soft", Filterable: true},
		{Name: "is_over_hard", Label: "超硬限制", Type: FieldTypeBoolean, Source: "usage.is_over_hard", Filterable: true},
		{Name: "last_checked", Label: "最后检查时间", Type: FieldTypeDateTime, Source: "usage.last_checked", Sortable: true},
	}
}

// convertToMap 转换为 map
func (ds *QuotaDataSource) convertToMap(usage interface{}) map[string]interface{} {
	// 简化实现，实际应使用反射或具体类型断言
	return map[string]interface{}{
		"quota": map[string]interface{}{
			"id":          "",
			"type":        "",
			"target_id":   "",
			"target_name": "",
			"volume_name": "",
			"path":        "",
			"hard_limit":  uint64(0),
			"soft_limit":  uint64(0),
		},
		"usage": map[string]interface{}{
			"used_bytes":    uint64(0),
			"available":     uint64(0),
			"usage_percent": float64(0),
			"is_over_soft":  false,
			"is_over_hard":  false,
			"last_checked":  time.Now(),
		},
	}
}

// getNestedValue 获取嵌套值
func (ds *QuotaDataSource) getNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	keys := splitPath(path)
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := current[key]
			return val, ok
		}

		next, ok := current[key].(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}

	return nil, false
}

// matchFilters 匹配过滤器
func (ds *QuotaDataSource) matchFilters(row map[string]interface{}, filters []TemplateFilter) bool {
	for _, filter := range filters {
		val, ok := row[filter.Field]
		if !ok {
			return false
		}

		if !ds.matchFilter(val, filter.Operator, filter.Value) {
			return false
		}
	}
	return true
}

// matchFilter 匹配单个过滤器
func (ds *QuotaDataSource) matchFilter(val interface{}, op string, expected interface{}) bool {
	switch op {
	case "eq":
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", expected)
	case "ne":
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", expected)
	case "gt", "lt", "gte", "lte":
		return compareNumeric(val, expected, op)
	case "contains":
		return contains(val, expected)
	case "in":
		return inList(val, expected)
	default:
		return true
	}
}

// applySorts 应用排序
func (ds *QuotaDataSource) applySorts(data []map[string]interface{}, sorts []TemplateSort) []map[string]interface{} {
	if len(sorts) == 0 {
		return data
	}

	result := make([]map[string]interface{}, len(data))
	copy(result, data)

	sort.Slice(result, func(i, j int) bool {
		for _, s := range sorts {
			cmp := compareValues(result[i][s.Field], result[j][s.Field])
			if cmp != 0 {
				if s.Order == "desc" {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})

	return result
}

// ========== 存储数据源适配器 ==========

// StorageDataSource 存储数据源
type StorageDataSource struct {
	storageManager interface {
		ListVolumes() []interface{}
		GetVolumeInfo(name string) (interface{}, error)
	}
}

// NewStorageDataSource 创建存储数据源
func NewStorageDataSource(manager interface{}) *StorageDataSource {
	storageManager, ok := manager.(interface {
		ListVolumes() []interface{}
		GetVolumeInfo(name string) (interface{}, error)
	})
	if !ok {
		return &StorageDataSource{storageManager: nil}
	}
	return &StorageDataSource{
		storageManager: storageManager,
	}
}

// Name 数据源名称
func (ds *StorageDataSource) Name() string {
	return "storage"
}

// Query 查询数据
func (ds *StorageDataSource) Query(
	query map[string]interface{},
	fields []TemplateField,
	filters []TemplateFilter,
	sorts []TemplateSort,
	aggregations []TemplateAggregation,
	groupBy []string,
	limit, offset int,
) ([]map[string]interface{}, error) {

	volumes := ds.storageManager.ListVolumes()
	result := make([]map[string]interface{}, 0, len(volumes))

	for _, vol := range volumes {
		row := ds.convertVolumeToMap(vol)

		// 应用过滤器
		if !ds.matchFilters(row, filters) {
			continue
		}

		// 提取字段
		extractedRow := make(map[string]interface{})
		for _, field := range fields {
			if val, ok := ds.getNestedValue(row, field.Source); ok {
				extractedRow[field.Name] = val
			}
		}

		result = append(result, extractedRow)
	}

	return result, nil
}

// GetSummary 获取摘要
func (ds *StorageDataSource) GetSummary(query map[string]interface{}) (map[string]interface{}, error) {
	volumes := ds.storageManager.ListVolumes()

	summary := map[string]interface{}{
		"total_volumes": len(volumes),
		"total_size":    int64(0),
		"total_used":    int64(0),
		"total_free":    int64(0),
	}

	// 使用 int64 存储累加值，防止溢出
	var totalSize, totalUsed, totalFree int64

	for _, vol := range volumes {
		row := ds.convertVolumeToMap(vol)
		if size, ok := row["total_size"].(uint64); ok {
			if size > uint64(1<<63-1) {
				totalSize = 1<<63 - 1
			} else {
				totalSize += int64(size)
			}
		}
		if used, ok := row["used_size"].(uint64); ok {
			if used > uint64(1<<63-1) {
				totalUsed = 1<<63 - 1
			} else {
				totalUsed += int64(used)
			}
		}
		if free, ok := row["free_size"].(uint64); ok {
			if free > uint64(1<<63-1) {
				totalFree = 1<<63 - 1
			} else {
				totalFree += int64(free)
			}
		}
	}

	// 更新 summary 为 int64 类型
	summary["total_size"] = totalSize
	summary["total_used"] = totalUsed
	summary["total_free"] = totalFree

	return summary, nil
}

// GetAvailableFields 获取可用字段
func (ds *StorageDataSource) GetAvailableFields() []TemplateField {
	return []TemplateField{
		{Name: "volume_name", Label: "卷名", Type: FieldTypeString, Source: "volume.name", Sortable: true, Filterable: true},
		{Name: "type", Label: "类型", Type: FieldTypeString, Source: "volume.type", Filterable: true},
		{Name: "total_size", Label: "总容量", Type: FieldTypeBytes, Source: "volume.total_size", Sortable: true},
		{Name: "used_size", Label: "已使用", Type: FieldTypeBytes, Source: "volume.used_size", Sortable: true},
		{Name: "free_size", Label: "可用空间", Type: FieldTypeBytes, Source: "volume.free_size", Sortable: true},
		{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent, Source: "volume.usage_percent", Sortable: true},
		{Name: "file_count", Label: "文件数", Type: FieldTypeNumber, Source: "volume.file_count", Sortable: true},
		{Name: "status", Label: "状态", Type: FieldTypeString, Source: "volume.status", Filterable: true},
	}
}

// convertVolumeToMap 转换卷信息为 map
func (ds *StorageDataSource) convertVolumeToMap(vol interface{}) map[string]interface{} {
	return map[string]interface{}{
		"volume": map[string]interface{}{
			"name":          "",
			"type":          "",
			"total_size":    uint64(0),
			"used_size":     uint64(0),
			"free_size":     uint64(0),
			"usage_percent": float64(0),
			"file_count":    0,
			"status":        "healthy",
		},
	}
}

// matchFilters 匹配过滤器
func (ds *StorageDataSource) matchFilters(row map[string]interface{}, filters []TemplateFilter) bool {
	for _, filter := range filters {
		val, ok := row[filter.Field]
		if !ok {
			return false
		}

		if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", filter.Value) {
			return false
		}
	}
	return true
}

// getNestedValue 获取嵌套值
func (ds *StorageDataSource) getNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	keys := splitPath(path)
	current := data

	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := current[key]
			return val, ok
		}

		next, ok := current[key].(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}

	return nil, false
}

// ========== 辅助函数 ==========

func splitPath(path string) []string {
	result := make([]string, 0)
	for _, part := range splitString(path, ".") {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

func compareNumeric(a, b interface{}, op string) bool {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if !aOk || !bOk {
		return false
	}

	switch op {
	case "gt":
		return aFloat > bFloat
	case "lt":
		return aFloat < bFloat
	case "gte":
		return aFloat >= bFloat
	case "lte":
		return aFloat <= bFloat
	default:
		return false
	}
}

func contains(val, expected interface{}) bool {
	str := fmt.Sprintf("%v", val)
	sub := fmt.Sprintf("%v", expected)
	return len(str) >= len(sub) && str[:len(sub)] == sub
}

func inList(val, expected interface{}) bool {
	str := fmt.Sprintf("%v", val)
	switch list := expected.(type) {
	case []string:
		for _, item := range list {
			if item == str {
				return true
			}
		}
	case []interface{}:
		for _, item := range list {
			if fmt.Sprintf("%v", item) == str {
				return true
			}
		}
	}
	return false
}
