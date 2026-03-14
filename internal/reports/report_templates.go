package reports

import (
	"time"
)

// ========== 预定义报告模板 ==========

// SystemHealthTemplate 系统健康报告模板
func SystemHealthTemplate() *ReportTemplate {
	return &ReportTemplate{
		ID:          "system-health",
		Name:        "系统健康报告",
		Type:        TemplateTypeSystem,
		Description: "系统整体健康状况评估报告，包含健康评分、组件状态和趋势分析",
		Fields: []TemplateField{
			{Name: "health_score", Label: "健康评分", Type: FieldTypeNumber, Source: "health.total_score", Sortable: true},
			{Name: "health_grade", Label: "健康等级", Type: FieldTypeString, Source: "health.grade", Filterable: true},
			{Name: "cpu_score", Label: "CPU 评分", Type: FieldTypeNumber, Source: "health.components.cpu.score"},
			{Name: "cpu_status", Label: "CPU 状态", Type: FieldTypeString, Source: "health.components.cpu.status"},
			{Name: "memory_score", Label: "内存评分", Type: FieldTypeNumber, Source: "health.components.memory.score"},
			{Name: "memory_status", Label: "内存状态", Type: FieldTypeString, Source: "health.components.memory.status"},
			{Name: "disk_score", Label: "磁盘评分", Type: FieldTypeNumber, Source: "health.components.disk.score"},
			{Name: "disk_status", Label: "磁盘状态", Type: FieldTypeString, Source: "health.components.disk.status"},
			{Name: "trend_direction", Label: "趋势方向", Type: FieldTypeString, Source: "health.trend.direction"},
			{Name: "trend_change", Label: "变化值", Type: FieldTypeNumber, Source: "health.trend.change"},
			{Name: "issues_count", Label: "问题数量", Type: FieldTypeNumber, Source: "health.issues"},
		},
		Sorts: []TemplateSort{
			{Field: "timestamp", Order: "desc"},
		},
		IsDefault: true,
		IsPublic:  true,
	}
}

// ResourceUsageTemplate 资源使用报告模板
func ResourceUsageTemplate() *ReportTemplate {
	return &ReportTemplate{
		ID:          "resource-usage",
		Name:        "资源使用报告",
		Type:        TemplateTypeSystem,
		Description: "系统资源使用情况报告，包含 CPU、内存、磁盘、网络使用统计",
		Fields: []TemplateField{
			{Name: "cpu_usage", Label: "CPU 使用率", Type: FieldTypePercent, Source: "system.cpu_usage", Sortable: true},
			{Name: "cpu_avg", Label: "CPU 平均值", Type: FieldTypePercent, Source: "trend.summary.cpu_avg"},
			{Name: "cpu_peak", Label: "CPU 峰值", Type: FieldTypePercent, Source: "trend.summary.cpu_max"},
			{Name: "memory_usage", Label: "内存使用率", Type: FieldTypePercent, Source: "system.memory_usage", Sortable: true},
			{Name: "memory_avg", Label: "内存平均", Type: FieldTypePercent, Source: "trend.summary.memory_avg"},
			{Name: "memory_peak", Label: "内存峰值", Type: FieldTypePercent, Source: "trend.summary.memory_max"},
			{Name: "memory_total", Label: "内存总量", Type: FieldTypeBytes, Source: "system.memory_total"},
			{Name: "memory_used", Label: "内存使用", Type: FieldTypeBytes, Source: "system.memory_used"},
			{Name: "disk_usage", Label: "磁盘使用率", Type: FieldTypePercent, Source: "disk.usage_percent", Sortable: true},
			{Name: "disk_total", Label: "磁盘总量", Type: FieldTypeBytes, Source: "disk.total"},
			{Name: "disk_used", Label: "磁盘使用", Type: FieldTypeBytes, Source: "disk.used"},
			{Name: "net_rx_bytes", Label: "网络接收", Type: FieldTypeBytes, Source: "network.rx_bytes"},
			{Name: "net_tx_bytes", Label: "网络发送", Type: FieldTypeBytes, Source: "network.tx_bytes"},
			{Name: "uptime", Label: "运行时间", Type: FieldTypeDuration, Source: "system.uptime"},
		},
		Sorts: []TemplateSort{
			{Field: "timestamp", Order: "desc"},
		},
		IsDefault: true,
		IsPublic:  true,
	}
}

// DailyReportTemplate 日报模板
func DailyReportTemplate() *ReportTemplate {
	return &ReportTemplate{
		ID:          "daily-report",
		Name:        "系统日报",
		Type:        TemplateTypeSystem,
		Description: "每日系统运行状态报告，包含当日资源使用摘要、健康评分变化和异常事件",
		Fields: []TemplateField{
			{Name: "report_date", Label: "报告日期", Type: FieldTypeDate, Source: "report_date"},
			{Name: "health_score", Label: "健康评分", Type: FieldTypeNumber, Source: "health.total_score"},
			{Name: "health_grade", Label: "健康等级", Type: FieldTypeString, Source: "health.grade"},
			{Name: "cpu_avg", Label: "CPU 平均使用率", Type: FieldTypePercent, Source: "resources.cpu.average"},
			{Name: "cpu_peak", Label: "CPU 峰值", Type: FieldTypePercent, Source: "resources.cpu.peak"},
			{Name: "cpu_peak_time", Label: "CPU 峰值时间", Type: FieldTypeDateTime, Source: "resources.cpu.peak_time"},
			{Name: "memory_avg", Label: "内存平均使用率", Type: FieldTypePercent, Source: "resources.memory.average"},
			{Name: "memory_peak", Label: "内存峰值", Type: FieldTypePercent, Source: "resources.memory.peak"},
			{Name: "disk_trend", Label: "磁盘趋势", Type: FieldTypeString, Source: "trends.disk_trend"},
			{Name: "net_rx_total", Label: "网络接收总量", Type: FieldTypeBytes, Source: "resources.network.rx_bytes"},
			{Name: "net_tx_total", Label: "网络发送总量", Type: FieldTypeBytes, Source: "resources.network.tx_bytes"},
			{Name: "alerts_count", Label: "告警次数", Type: FieldTypeNumber, Source: "alerts.total"},
			{Name: "critical_alerts", Label: "严重告警", Type: FieldTypeNumber, Source: "alerts.critical"},
			{Name: "uptime", Label: "运行时间", Type: FieldTypeDuration, Source: "system_info.uptime"},
		},
		IsDefault: true,
		IsPublic:  true,
	}
}

// WeeklyReportTemplate 周报模板
func WeeklyReportTemplate() *ReportTemplate {
	return &ReportTemplate{
		ID:          "weekly-report",
		Name:        "系统周报",
		Type:        TemplateTypeSystem,
		Description: "每周系统运行状态总结报告，包含一周趋势分析、资源使用对比和改进建议",
		Fields: []TemplateField{
			{Name: "report_start", Label: "报告开始日期", Type: FieldTypeDate, Source: "report_start"},
			{Name: "report_end", Label: "报告结束日期", Type: FieldTypeDate, Source: "report_end"},
			{Name: "health_score_avg", Label: "健康评分均值", Type: FieldTypeNumber, Source: "health.average"},
			{Name: "health_score_min", Label: "最低健康评分", Type: FieldTypeNumber, Source: "health.min"},
			{Name: "health_score_max", Label: "最高健康评分", Type: FieldTypeNumber, Source: "health.max"},
			{Name: "health_trend", Label: "健康趋势", Type: FieldTypeString, Source: "health.trend"},
			{Name: "cpu_avg", Label: "CPU 平均使用率", Type: FieldTypePercent, Source: "resources.cpu.average"},
			{Name: "cpu_trend", Label: "CPU 趋势", Type: FieldTypeString, Source: "trends.cpu_trend"},
			{Name: "memory_avg", Label: "内存平均使用率", Type: FieldTypePercent, Source: "resources.memory.average"},
			{Name: "memory_trend", Label: "内存趋势", Type: FieldTypeString, Source: "trends.memory_trend"},
			{Name: "disk_growth", Label: "磁盘增长量", Type: FieldTypeBytes, Source: "disk.growth"},
			{Name: "net_rx_total", Label: "网络接收总量", Type: FieldTypeBytes, Source: "resources.network.rx_total"},
			{Name: "net_tx_total", Label: "网络发送总量", Type: FieldTypeBytes, Source: "resources.network.tx_total"},
			{Name: "alerts_total", Label: "总告警数", Type: FieldTypeNumber, Source: "alerts.total"},
			{Name: "alerts_critical", Label: "严重告警数", Type: FieldTypeNumber, Source: "alerts.critical"},
			{Name: "uptime_percent", Label: "可用率", Type: FieldTypePercent, Source: "availability.uptime_percent"},
		},
		IsDefault: true,
		IsPublic:  true,
	}
}

// DiskAnalysisTemplate 磁盘分析报告模板
func DiskAnalysisTemplate() *ReportTemplate {
	return &ReportTemplate{
		ID:          "disk-analysis",
		Name:        "磁盘分析报告",
		Type:        TemplateTypeStorage,
		Description: "磁盘使用和健康状况详细分析报告",
		Fields: []TemplateField{
			{Name: "device", Label: "设备", Type: FieldTypeString, Source: "disk.device", Sortable: true, Filterable: true},
			{Name: "mount_point", Label: "挂载点", Type: FieldTypeString, Source: "disk.mount_point", Filterable: true},
			{Name: "fs_type", Label: "文件系统", Type: FieldTypeString, Source: "disk.fs_type", Filterable: true},
			{Name: "total", Label: "总容量", Type: FieldTypeBytes, Source: "disk.total", Sortable: true},
			{Name: "used", Label: "已使用", Type: FieldTypeBytes, Source: "disk.used", Sortable: true},
			{Name: "free", Label: "可用空间", Type: FieldTypeBytes, Source: "disk.free", Sortable: true},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent, Source: "disk.usage_percent", Sortable: true},
			{Name: "trend", Label: "趋势", Type: FieldTypeString, Source: "disk.trend"},
			{Name: "status", Label: "状态", Type: FieldTypeString, Source: "disk.status", Filterable: true},
		},
		Sorts: []TemplateSort{
			{Field: "usage_percent", Order: "desc"},
		},
		IsDefault: true,
		IsPublic:  true,
	}
}

// TrendReportTemplate 趋势分析报告模板
func TrendReportTemplate() *ReportTemplate {
	return &ReportTemplate{
		ID:          "trend-analysis",
		Name:        "趋势分析报告",
		Type:        TemplateTypeSystem,
		Description: "系统资源使用趋势分析报告，包含历史数据对比和预测",
		Fields: []TemplateField{
			{Name: "timestamp", Label: "时间", Type: FieldTypeDateTime, Source: "timestamp", Sortable: true},
			{Name: "cpu_usage", Label: "CPU 使用率", Type: FieldTypePercent, Source: "cpu_usage", Sortable: true},
			{Name: "memory_usage", Label: "内存使用率", Type: FieldTypePercent, Source: "memory_usage", Sortable: true},
			{Name: "disk_usage", Label: "磁盘使用率", Type: FieldTypePercent, Source: "disk_usage", Sortable: true},
			{Name: "health_score", Label: "健康评分", Type: FieldTypeNumber, Source: "health_score", Sortable: true},
			{Name: "load_avg", Label: "负载", Type: FieldTypeNumber, Source: "load_avg", Sortable: true},
		},
		Sorts: []TemplateSort{
			{Field: "timestamp", Order: "asc"},
		},
		IsDefault: true,
		IsPublic:  true,
	}
}

// ========== 预设调度配置 ==========

// DailyScheduleConfig 日报调度配置
func DailyScheduleConfig() ScheduledReportInput {
	return ScheduledReportInput{
		Name:         "系统健康日报",
		TemplateID:   "daily-report",
		Frequency:    FrequencyDaily,
		CronExpr:     "0 0 8 * * *", // 每天早上 8:00
		ExportFormat: ExportPDF,
		NotifyEmail:  []string{},
		Enabled:      true,
		Retention:    30, // 保留 30 天
	}
}

// WeeklyScheduleConfig 周报调度配置
func WeeklyScheduleConfig() ScheduledReportInput {
	return ScheduledReportInput{
		Name:         "系统健康周报",
		TemplateID:   "weekly-report",
		Frequency:    FrequencyWeekly,
		CronExpr:     "0 0 9 * * 1", // 每周一早上 9:00
		ExportFormat: ExportPDF,
		NotifyEmail:  []string{},
		Enabled:      true,
		Retention:    90, // 保留 90 天
	}
}

// HourlyScheduleConfig 时报调度配置
func HourlyScheduleConfig() ScheduledReportInput {
	return ScheduledReportInput{
		Name:         "系统状态时报",
		TemplateID:   "resource-usage",
		Frequency:    FrequencyHourly,
		ExportFormat: ExportJSON,
		NotifyEmail:  []string{},
		Enabled:      false, // 默认不启用
		Retention:    7,
	}
}

// ========== 报告模板管理 ==========

// TemplateRegistry 模板注册表
type TemplateRegistry struct {
	templates map[string]*ReportTemplate
}

// NewTemplateRegistry 创建模板注册表
func NewTemplateRegistry() *TemplateRegistry {
	registry := &TemplateRegistry{
		templates: make(map[string]*ReportTemplate),
	}

	// 注册默认模板
	registry.RegisterDefaults()

	return registry
}

// RegisterDefaults 注册默认模板
func (r *TemplateRegistry) RegisterDefaults() {
	r.Register(SystemHealthTemplate())
	r.Register(ResourceUsageTemplate())
	r.Register(DailyReportTemplate())
	r.Register(WeeklyReportTemplate())
	r.Register(DiskAnalysisTemplate())
	r.Register(TrendReportTemplate())
}

// Register 注册模板
func (r *TemplateRegistry) Register(template *ReportTemplate) {
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	r.templates[template.ID] = template
}

// Get 获取模板
func (r *TemplateRegistry) Get(id string) (*ReportTemplate, bool) {
	t, ok := r.templates[id]
	return t, ok
}

// List 列出所有模板
func (r *TemplateRegistry) List(templateType TemplateType) []*ReportTemplate {
	result := make([]*ReportTemplate, 0)
	for _, t := range r.templates {
		if templateType == "" || t.Type == templateType {
			result = append(result, t)
		}
	}
	return result
}

// Remove 移除模板
func (r *TemplateRegistry) Remove(id string) {
	delete(r.templates, id)
}

// GetDefaultTemplates 获取默认模板列表
func GetDefaultTemplates() []*ReportTemplate {
	return []*ReportTemplate{
		SystemHealthTemplate(),
		ResourceUsageTemplate(),
		DailyReportTemplate(),
		WeeklyReportTemplate(),
		DiskAnalysisTemplate(),
		TrendReportTemplate(),
	}
}

// GetDefaultScheduleConfigs 获取默认调度配置
func GetDefaultScheduleConfigs() []ScheduledReportInput {
	return []ScheduledReportInput{
		DailyScheduleConfig(),
		WeeklyScheduleConfig(),
		HourlyScheduleConfig(),
	}
}
