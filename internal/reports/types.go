// Package reports 提供报表生成和管理功能
package reports

import (
	"time"
)

// ========== 报表模板 ==========

// TemplateType 模板类型.
type TemplateType string

const (
	// TemplateTypeQuota represents quota report template type.
	TemplateTypeQuota TemplateType = "quota" // 配额报表
	// TemplateTypeStorage represents storage report template type.
	TemplateTypeStorage TemplateType = "storage" // 存储报表
	// TemplateTypeUser represents user report template type.
	TemplateTypeUser TemplateType = "user" // 用户报表
	// TemplateTypeSystem represents system report template type.
	TemplateTypeSystem TemplateType = "system" // 系统报表
	// TemplateTypeCustom represents custom template type.
	TemplateTypeCustom TemplateType = "custom" // 自定义模板
)

// FieldType 字段类型.
type FieldType string

const (
	// FieldTypeString represents string field type.
	FieldTypeString FieldType = "string"
	// FieldTypeNumber represents number field type.
	FieldTypeNumber FieldType = "number"
	// FieldTypePercent represents percent field type.
	FieldTypePercent FieldType = "percent"
	// FieldTypeBytes represents bytes field type.
	FieldTypeBytes FieldType = "bytes"
	// FieldTypeDate represents date field type.
	FieldTypeDate FieldType = "date"
	// FieldTypeDateTime represents datetime field type.
	FieldTypeDateTime FieldType = "datetime"
	// FieldTypeDuration represents duration field type.
	FieldTypeDuration FieldType = "duration"
	// FieldTypeBoolean represents boolean field type.
	FieldTypeBoolean FieldType = "boolean"
	// FieldTypeList represents list field type.
	FieldTypeList FieldType = "list"
)

// TemplateField 模板字段.
type TemplateField struct {
	Name         string    `json:"name"`         // 字段名（英文标识）
	Label        string    `json:"label"`        // 显示标签
	Type         FieldType `json:"type"`         // 字段类型
	Source       string    `json:"source"`       // 数据源路径（如 "quota.used_bytes"）
	Format       string    `json:"format"`       // 格式化模板（如日期格式）
	Default      string    `json:"default"`      // 默认值
	Required     bool      `json:"required"`     // 是否必需
	Sortable     bool      `json:"sortable"`     // 是否可排序
	Filterable   bool      `json:"filterable"`   // 是否可过滤
	Aggregatable bool      `json:"aggregatable"` // 是否可聚合
	Description  string    `json:"description"`  // 字段说明
}

// TemplateFilter 模板过滤器.
type TemplateFilter struct {
	Field    string      `json:"field"`    // 字段名
	Operator string      `json:"operator"` // 操作符: eq, ne, gt, lt, gte, lte, contains, in, between
	Value    interface{} `json:"value"`    // 值
}

// TemplateSort 模板排序.
type TemplateSort struct {
	Field string `json:"field"` // 字段名
	Order string `json:"order"` // asc 或 desc
}

// TemplateAggregation 模板聚合.
type TemplateAggregation struct {
	Field    string `json:"field"`    // 字段名
	Function string `json:"function"` // sum, avg, count, min, max
	Alias    string `json:"alias"`    // 结果别名
}

// ReportTemplate 报表模板.
type ReportTemplate struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Type         TemplateType          `json:"type"`
	Description  string                `json:"description"`
	Fields       []TemplateField       `json:"fields"`
	Filters      []TemplateFilter      `json:"filters,omitempty"`
	Sorts        []TemplateSort        `json:"sorts,omitempty"`
	Aggregations []TemplateAggregation `json:"aggregations,omitempty"`
	GroupBy      []string              `json:"group_by,omitempty"`
	Limit        int                   `json:"limit,omitempty"`
	Offset       int                   `json:"offset,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	CreatedBy    string                `json:"created_by,omitempty"`
	IsDefault    bool                  `json:"is_default"`
	IsPublic     bool                  `json:"is_public"` // 是否公开给所有用户
}

// TemplateInput 创建/更新模板输入.
type TemplateInput struct {
	Name         string                `json:"name" binding:"required"`
	Type         TemplateType          `json:"type" binding:"required"`
	Description  string                `json:"description"`
	Fields       []TemplateField       `json:"fields" binding:"required"`
	Filters      []TemplateFilter      `json:"filters,omitempty"`
	Sorts        []TemplateSort        `json:"sorts,omitempty"`
	Aggregations []TemplateAggregation `json:"aggregations,omitempty"`
	GroupBy      []string              `json:"group_by,omitempty"`
	Limit        int                   `json:"limit,omitempty"`
	Offset       int                   `json:"offset,omitempty"`
	IsPublic     bool                  `json:"is_public"`
}

// ========== 自定义报表 ==========

// CustomReport 自定义报表.
type CustomReport struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	TemplateID   string                 `json:"template_id,omitempty"` // 基于的模板
	DataSource   string                 `json:"data_source"`           // 数据源类型
	Query        map[string]interface{} `json:"query"`                 // 查询参数
	Fields       []TemplateField        `json:"fields"`                // 输出字段
	Filters      []TemplateFilter       `json:"filters,omitempty"`
	Sorts        []TemplateSort         `json:"sorts,omitempty"`
	Aggregations []TemplateAggregation  `json:"aggregations,omitempty"`
	GroupBy      []string               `json:"group_by,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"` // 运行时参数
	Limit        int                    `json:"limit,omitempty"`
	Offset       int                    `json:"offset,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CreatedBy    string                 `json:"created_by,omitempty"`
}

// CustomReportInput 自定义报表输入.
type CustomReportInput struct {
	Name         string                 `json:"name" binding:"required"`
	Description  string                 `json:"description"`
	TemplateID   string                 `json:"template_id,omitempty"`
	DataSource   string                 `json:"data_source" binding:"required"`
	Query        map[string]interface{} `json:"query"`
	Fields       []TemplateField        `json:"fields" binding:"required"`
	Filters      []TemplateFilter       `json:"filters,omitempty"`
	Sorts        []TemplateSort         `json:"sorts,omitempty"`
	Aggregations []TemplateAggregation  `json:"aggregations,omitempty"`
	GroupBy      []string               `json:"group_by,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Limit        int                    `json:"limit,omitempty"`
	Offset       int                    `json:"offset,omitempty"`
}

// ========== 定时报表 ==========

// ScheduleFrequency 调度频率.
type ScheduleFrequency string

const (
	// FrequencyHourly represents hourly schedule frequency.
	FrequencyHourly ScheduleFrequency = "hourly"
	// FrequencyDaily represents daily schedule frequency.
	FrequencyDaily ScheduleFrequency = "daily"
	// FrequencyWeekly represents weekly schedule frequency.
	FrequencyWeekly ScheduleFrequency = "weekly"
	// FrequencyMonthly represents monthly schedule frequency.
	FrequencyMonthly ScheduleFrequency = "monthly"
	// FrequencyCustom represents custom schedule frequency.
	FrequencyCustom ScheduleFrequency = "custom"
)

// ScheduledReport 定时报表.
type ScheduledReport struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	ReportID      string            `json:"report_id"`             // 关联的自定义报表
	TemplateID    string            `json:"template_id,omitempty"` // 或关联的模板
	Frequency     ScheduleFrequency `json:"frequency"`
	CronExpr      string            `json:"cron_expr,omitempty"` // 自定义 cron 表达式
	Timezone      string            `json:"timezone"`            // 时区
	NextRun       *time.Time        `json:"next_run,omitempty"`
	LastRun       *time.Time        `json:"last_run,omitempty"`
	LastStatus    string            `json:"last_status,omitempty"`
	Enabled       bool              `json:"enabled"`
	ExportFormat  ExportFormat      `json:"export_format"`
	OutputPath    string            `json:"output_path,omitempty"`
	NotifyEmail   []string          `json:"notify_email,omitempty"`
	NotifyWebhook []string          `json:"notify_webhook,omitempty"`
	Retention     int               `json:"retention"` // 保留天数
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CreatedBy     string            `json:"created_by,omitempty"`
}

// ScheduledReportInput 定时报表输入.
type ScheduledReportInput struct {
	Name          string            `json:"name" binding:"required"`
	Description   string            `json:"description"`
	ReportID      string            `json:"report_id"`
	TemplateID    string            `json:"template_id,omitempty"`
	Frequency     ScheduleFrequency `json:"frequency" binding:"required"`
	CronExpr      string            `json:"cron_expr,omitempty"`
	Timezone      string            `json:"timezone"`
	ExportFormat  ExportFormat      `json:"export_format"`
	OutputPath    string            `json:"output_path,omitempty"`
	NotifyEmail   []string          `json:"notify_email,omitempty"`
	NotifyWebhook []string          `json:"notify_webhook,omitempty"`
	Retention     int               `json:"retention"`
	Enabled       bool              `json:"enabled"`
}

// ScheduledReportExecution 调度执行记录.
type ScheduledReportExecution struct {
	ID           string                 `json:"id"`
	ScheduleID   string                 `json:"schedule_id"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Status       string                 `json:"status"` // running, success, failed
	OutputPath   string                 `json:"output_path,omitempty"`
	OutputSize   int64                  `json:"output_size,omitempty"`
	RecordsCount int                    `json:"records_count,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ========== 导出格式 ==========

// ExportFormat 导出格式.
type ExportFormat string

const (
	// ExportJSON represents JSON export format.
	ExportJSON ExportFormat = "json"
	// ExportCSV represents CSV export format.
	ExportCSV ExportFormat = "csv"
	// ExportHTML represents HTML export format.
	ExportHTML ExportFormat = "html"
	// ExportPDF represents PDF export format.
	ExportPDF ExportFormat = "pdf"
	// ExportExcel represents Excel export format.
	ExportExcel ExportFormat = "xlsx"
)

// ExportOptions 导出选项.
type ExportOptions struct {
	Format        ExportFormat `json:"format"`
	Filename      string       `json:"filename,omitempty"`
	IncludeHeader bool         `json:"include_header"` // CSV/Excel 是否包含表头
	DateRange     bool         `json:"date_range"`     // 是否包含日期范围
	Summary       bool         `json:"summary"`        // 是否包含摘要
	Charts        bool         `json:"charts"`         // PDF 是否包含图表
	Orientation   string       `json:"orientation"`    // PDF 页面方向: portrait, landscape
	PageSize      string       `json:"page_size"`      // PDF 页面大小: A4, Letter
	Title         string       `json:"title,omitempty"`
	Subtitle      string       `json:"subtitle,omitempty"`
	Logo          string       `json:"logo,omitempty"`
	Company       string       `json:"company,omitempty"`
	Footer        string       `json:"footer,omitempty"`
}

// ExportResult 导出结果.
type ExportResult struct {
	Format    ExportFormat `json:"format"`
	Filename  string       `json:"filename"`
	Path      string       `json:"path"`
	Size      int64        `json:"size"`
	MimeType  string       `json:"mime_type"`
	CreatedAt time.Time    `json:"created_at"`
}

// ========== 报表输出 ==========

// GeneratedReport 生成的报表.
type GeneratedReport struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	TemplateID     string                   `json:"template_id,omitempty"`
	CustomReportID string                   `json:"custom_report_id,omitempty"`
	GeneratedAt    time.Time                `json:"generated_at"`
	Period         ReportPeriod             `json:"period"`
	Parameters     map[string]interface{}   `json:"parameters,omitempty"`
	Summary        map[string]interface{}   `json:"summary,omitempty"`
	Data           []map[string]interface{} `json:"data"`
	TotalRecords   int                      `json:"total_records"`
	ExportFormat   ExportFormat             `json:"export_format"`
	ExportPath     string                   `json:"export_path,omitempty"`
}

// ReportPeriod 报告时间范围.
type ReportPeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// ========== 数据源适配器 ==========

// DataSource 数据源接口.
type DataSource interface {
	// Name 数据源名称
	Name() string

	// Query 查询数据
	Query(query map[string]interface{}, fields []TemplateField,
		filters []TemplateFilter, sorts []TemplateSort,
		aggregations []TemplateAggregation, groupBy []string,
		limit, offset int) ([]map[string]interface{}, error)

	// GetSummary 获取摘要
	GetSummary(query map[string]interface{}) (map[string]interface{}, error)

	// GetAvailableFields 获取可用字段
	GetAvailableFields() []TemplateField
}

// DataSourceType 数据源类型.
type DataSourceType string

const (
	// DataSourceQuota represents quota data source type.
	DataSourceQuota DataSourceType = "quota"
	// DataSourceStorage represents storage data source type.
	DataSourceStorage DataSourceType = "storage"
	// DataSourceUser represents user data source type.
	DataSourceUser DataSourceType = "user"
	// DataSourceBackup represents backup data source type.
	DataSourceBackup DataSourceType = "backup"
	// DataSourceSystem represents system data source type.
	DataSourceSystem DataSourceType = "system"
	// DataSourceMonitor represents monitor data source type.
	DataSourceMonitor DataSourceType = "monitor"
)
