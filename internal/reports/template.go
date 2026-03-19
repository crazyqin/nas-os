// Package reports 提供报表生成和管理功能
package reports

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrTemplateNotFound is the error when a template is not found.
	ErrTemplateNotFound = errors.New("模板不存在")
	// ErrTemplateExists is the error when a template already exists.
	ErrTemplateExists = errors.New("模板已存在")
	// ErrReportNotFound is the error when a report is not found.
	ErrReportNotFound = errors.New("报表不存在")
	// ErrScheduleNotFound is the error when a schedule is not found.
	ErrScheduleNotFound = errors.New("定时任务不存在")
	// ErrDataSourceNotFound is the error when a data source is not found.
	ErrDataSourceNotFound = errors.New("数据源不存在")
	// ErrInvalidQuery is the error when an invalid query is provided.
	ErrInvalidQuery = errors.New("无效的查询参数")
	// ErrExportFailed is the error when export fails.
	ErrExportFailed = errors.New("导出失败")
	// ErrInvalidCronExpr is the error when an invalid cron expression is provided.
	ErrInvalidCronExpr = errors.New("无效的 cron 表达式")
)

// ========== 模板管理器 ==========

// TemplateManager 模板管理器
type TemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*ReportTemplate
	dataDir   string
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(dataDir string) (*TemplateManager, error) {
	tm := &TemplateManager{
		templates: make(map[string]*ReportTemplate),
		dataDir:   dataDir,
	}

	// 确保目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	// 加载已有模板
	if err := tm.loadTemplates(); err != nil {
		return nil, err
	}

	// 初始化默认模板
	tm.initDefaultTemplates()

	return tm, nil
}

// loadTemplates 加载模板
func (tm *TemplateManager) loadTemplates() error {
	files, err := filepath.Glob(filepath.Join(tm.dataDir, "*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var template ReportTemplate
		if err := json.Unmarshal(data, &template); err != nil {
			continue
		}

		tm.templates[template.ID] = &template
	}

	return nil
}

// saveTemplate 保存模板到文件
func (tm *TemplateManager) saveTemplate(template *ReportTemplate) error {
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(tm.dataDir, template.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteTemplateFile 删除模板文件
func (tm *TemplateManager) deleteTemplateFile(id string) error {
	path := filepath.Join(tm.dataDir, id+".json")
	return os.Remove(path)
}

// CreateTemplate 创建模板
func (tm *TemplateManager) CreateTemplate(input TemplateInput, createdBy string) (*ReportTemplate, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 检查名称是否重复
	for _, t := range tm.templates {
		if t.Name == input.Name {
			return nil, ErrTemplateExists
		}
	}

	now := time.Now()
	template := &ReportTemplate{
		ID:           uuid.New().String(),
		Name:         input.Name,
		Type:         input.Type,
		Description:  input.Description,
		Fields:       input.Fields,
		Filters:      input.Filters,
		Sorts:        input.Sorts,
		Aggregations: input.Aggregations,
		GroupBy:      input.GroupBy,
		Limit:        input.Limit,
		Offset:       input.Offset,
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    createdBy,
		IsDefault:    false,
		IsPublic:     input.IsPublic,
	}

	tm.templates[template.ID] = template

	if err := tm.saveTemplate(template); err != nil {
		delete(tm.templates, template.ID)
		return nil, err
	}

	return template, nil
}

// GetTemplate 获取模板
func (tm *TemplateManager) GetTemplate(id string) (*ReportTemplate, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	template, exists := tm.templates[id]
	if !exists {
		return nil, ErrTemplateNotFound
	}

	return template, nil
}

// ListTemplates 列出模板
func (tm *TemplateManager) ListTemplates(templateType TemplateType, publicOnly bool) []*ReportTemplate {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*ReportTemplate, 0)
	for _, t := range tm.templates {
		// 过滤类型
		if templateType != "" && t.Type != templateType {
			continue
		}
		// 过滤公开状态
		if publicOnly && !t.IsPublic {
			continue
		}
		result = append(result, t)
	}

	return result
}

// UpdateTemplate 更新模板
func (tm *TemplateManager) UpdateTemplate(id string, input TemplateInput) (*ReportTemplate, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	template, exists := tm.templates[id]
	if !exists {
		return nil, ErrTemplateNotFound
	}

	// 检查名称是否重复（排除自身）
	for _, t := range tm.templates {
		if t.ID != id && t.Name == input.Name {
			return nil, ErrTemplateExists
		}
	}

	template.Name = input.Name
	template.Type = input.Type
	template.Description = input.Description
	template.Fields = input.Fields
	template.Filters = input.Filters
	template.Sorts = input.Sorts
	template.Aggregations = input.Aggregations
	template.GroupBy = input.GroupBy
	template.Limit = input.Limit
	template.Offset = input.Offset
	template.IsPublic = input.IsPublic
	template.UpdatedAt = time.Now()

	if err := tm.saveTemplate(template); err != nil {
		return nil, err
	}

	return template, nil
}

// DeleteTemplate 删除模板
func (tm *TemplateManager) DeleteTemplate(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	_, exists := tm.templates[id]
	if !exists {
		return ErrTemplateNotFound
	}

	// 不允许删除默认模板
	if tm.templates[id].IsDefault {
		return errors.New("不能删除默认模板")
	}

	delete(tm.templates, id)
	return tm.deleteTemplateFile(id)
}

// GetDefaultTemplate 获取默认模板
func (tm *TemplateManager) GetDefaultTemplate(templateType TemplateType) *ReportTemplate {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, t := range tm.templates {
		if t.Type == templateType && t.IsDefault {
			return t
		}
	}

	return nil
}

// initDefaultTemplates 初始化默认模板
func (tm *TemplateManager) initDefaultTemplates() {
	// 检查是否已有默认模板
	if len(tm.templates) > 0 {
		return
	}

	// 配额报表默认模板
	quotaTemplate := &ReportTemplate{
		ID:          uuid.New().String(),
		Name:        "配额使用报表",
		Type:        TemplateTypeQuota,
		Description: "默认的存储配额使用报表模板",
		Fields: []TemplateField{
			{Name: "quota_id", Label: "配额ID", Type: FieldTypeString, Source: "quota.id", Required: true, Sortable: true},
			{Name: "target_name", Label: "用户/组名", Type: FieldTypeString, Source: "quota.target_name", Required: true, Sortable: true, Filterable: true},
			{Name: "volume_name", Label: "卷名", Type: FieldTypeString, Source: "quota.volume_name", Sortable: true, Filterable: true},
			{Name: "hard_limit", Label: "硬限制", Type: FieldTypeBytes, Source: "quota.hard_limit", Sortable: true},
			{Name: "soft_limit", Label: "软限制", Type: FieldTypeBytes, Source: "quota.soft_limit", Sortable: true},
			{Name: "used_bytes", Label: "已使用", Type: FieldTypeBytes, Source: "usage.used_bytes", Sortable: true},
			{Name: "available", Label: "可用空间", Type: FieldTypeBytes, Source: "usage.available", Sortable: true},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent, Source: "usage.usage_percent", Sortable: true},
			{Name: "is_over_soft", Label: "超软限制", Type: FieldTypeBoolean, Source: "usage.is_over_soft", Filterable: true},
			{Name: "is_over_hard", Label: "超硬限制", Type: FieldTypeBoolean, Source: "usage.is_over_hard", Filterable: true},
			{Name: "last_checked", Label: "最后检查", Type: FieldTypeDateTime, Source: "usage.last_checked", Sortable: true},
		},
		Sorts: []TemplateSort{
			{Field: "usage_percent", Order: "desc"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDefault: true,
		IsPublic:  true,
	}

	// 存储报表默认模板
	storageTemplate := &ReportTemplate{
		ID:          uuid.New().String(),
		Name:        "存储使用报表",
		Type:        TemplateTypeStorage,
		Description: "默认的存储使用报表模板",
		Fields: []TemplateField{
			{Name: "volume_name", Label: "卷名", Type: FieldTypeString, Source: "volume.name", Required: true, Sortable: true},
			{Name: "total_size", Label: "总容量", Type: FieldTypeBytes, Source: "volume.total_size", Sortable: true},
			{Name: "used_size", Label: "已使用", Type: FieldTypeBytes, Source: "volume.used_size", Sortable: true},
			{Name: "free_size", Label: "可用空间", Type: FieldTypeBytes, Source: "volume.free_size", Sortable: true},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent, Source: "volume.usage_percent", Sortable: true},
			{Name: "file_count", Label: "文件数量", Type: FieldTypeNumber, Source: "volume.file_count", Sortable: true},
			{Name: "dir_count", Label: "目录数量", Type: FieldTypeNumber, Source: "volume.dir_count", Sortable: true},
		},
		Sorts: []TemplateSort{
			{Field: "usage_percent", Order: "desc"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDefault: true,
		IsPublic:  true,
	}

	// 用户报表默认模板
	userTemplate := &ReportTemplate{
		ID:          uuid.New().String(),
		Name:        "用户活动报表",
		Type:        TemplateTypeUser,
		Description: "默认的用户活动报表模板",
		Fields: []TemplateField{
			{Name: "username", Label: "用户名", Type: FieldTypeString, Source: "user.username", Required: true, Sortable: true, Filterable: true},
			{Name: "display_name", Label: "显示名", Type: FieldTypeString, Source: "user.display_name", Sortable: true},
			{Name: "storage_used", Label: "存储使用量", Type: FieldTypeBytes, Source: "user.storage_used", Sortable: true},
			{Name: "file_count", Label: "文件数量", Type: FieldTypeNumber, Source: "user.file_count", Sortable: true},
			{Name: "last_login", Label: "最后登录", Type: FieldTypeDateTime, Source: "user.last_login", Sortable: true},
			{Name: "login_count", Label: "登录次数", Type: FieldTypeNumber, Source: "user.login_count", Sortable: true},
		},
		Sorts: []TemplateSort{
			{Field: "storage_used", Order: "desc"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDefault: true,
		IsPublic:  true,
	}

	// 系统报表默认模板
	systemTemplate := &ReportTemplate{
		ID:          uuid.New().String(),
		Name:        "系统状态报表",
		Type:        TemplateTypeSystem,
		Description: "默认的系统状态报表模板",
		Fields: []TemplateField{
			{Name: "timestamp", Label: "时间戳", Type: FieldTypeDateTime, Source: "system.timestamp", Required: true, Sortable: true},
			{Name: "cpu_usage", Label: "CPU使用率", Type: FieldTypePercent, Source: "system.cpu_usage", Sortable: true},
			{Name: "memory_usage", Label: "内存使用率", Type: FieldTypePercent, Source: "system.memory_usage", Sortable: true},
			{Name: "disk_io_read", Label: "磁盘读取", Type: FieldTypeBytes, Source: "system.disk_io_read", Sortable: true},
			{Name: "disk_io_write", Label: "磁盘写入", Type: FieldTypeBytes, Source: "system.disk_io_write", Sortable: true},
			{Name: "network_in", Label: "网络入站", Type: FieldTypeBytes, Source: "system.network_in", Sortable: true},
			{Name: "network_out", Label: "网络出站", Type: FieldTypeBytes, Source: "system.network_out", Sortable: true},
			{Name: "uptime", Label: "运行时间", Type: FieldTypeDuration, Source: "system.uptime", Sortable: true},
		},
		Sorts: []TemplateSort{
			{Field: "timestamp", Order: "desc"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDefault: true,
		IsPublic:  true,
	}

	// 保存默认模板
	for _, template := range []*ReportTemplate{quotaTemplate, storageTemplate, userTemplate, systemTemplate} {
		tm.templates[template.ID] = template
		_ = tm.saveTemplate(template)
	}
}
