// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"nas-os/internal/api"
)

// Handlers 报表 HTTP 处理器
type Handlers struct {
	templateManager   *TemplateManager
	generator         *ReportGenerator
	scheduleManager   *ScheduleManager
	exporter          *Exporter
	resourceAPI       *ResourceAPIHandlers
	visualizationAPI  *ResourceVisualizationHandlers // v2.30.0 资源可视化
	enhancedExportAPI *EnhancedExportAPIHandlers     // v2.35.0 增强导出
	costAnalysisAPI   *CostAnalysisAPIHandlers       // v2.35.0 成本分析
	enhancedReportAPI *ResourceReportEnhancedAPI     // v2.76.0 资源报告增强
}

// NewHandlers 创建处理器
func NewHandlers(tm *TemplateManager, gen *ReportGenerator, sm *ScheduleManager, exp *Exporter) *Handlers {
	// 默认成本配置
	costConfig := StorageCostConfig{
		CostPerGBMonthly:        0.5,
		CostPerIOPSMonthly:      0.01,
		CostPerBandwidthMonthly: 1.0,
		ElectricityCostPerKWh:   0.6,
		DevicePowerWatts:        100,
		OpsCostMonthly:          500,
		DepreciationYears:       5,
		HardwareCost:            50000,
	}

	return &Handlers{
		templateManager:   tm,
		generator:         gen,
		scheduleManager:   sm,
		exporter:          exp,
		resourceAPI:       NewResourceAPIHandlers(gen),
		visualizationAPI:  NewResourceVisualizationHandlers(),     // v2.30.0
		enhancedExportAPI: NewEnhancedExportAPIHandlers(exp),      // v2.35.0
		costAnalysisAPI:   NewCostAnalysisAPIHandlers(costConfig), // v2.35.0
		enhancedReportAPI: NewResourceReportEnhancedAPI(),         // v2.76.0
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	// ========== 模板管理 ==========
	templates := apiGroup.Group("/report-templates")
	{
		templates.GET("", h.listTemplates)
		templates.POST("", h.createTemplate)
		templates.GET("/:id", h.getTemplate)
		templates.PUT("/:id", h.updateTemplate)
		templates.DELETE("/:id", h.deleteTemplate)
		templates.GET("/:id/fields", h.getTemplateFields)
		templates.POST("/:id/generate", h.generateFromTemplate)
	}

	// ========== 自定义报表 ==========
	reports := apiGroup.Group("/custom-reports")
	{
		reports.GET("", h.listCustomReports)
		reports.POST("", h.createCustomReport)
		reports.GET("/:id", h.getCustomReport)
		reports.PUT("/:id", h.updateCustomReport)
		reports.DELETE("/:id", h.deleteCustomReport)
		reports.POST("/:id/generate", h.generateFromCustomReport)
		reports.POST("/:id/preview", h.previewCustomReport)
	}

	// ========== 定时报表 ==========
	schedules := apiGroup.Group("/scheduled-reports")
	{
		schedules.GET("", h.listSchedules)
		schedules.POST("", h.createSchedule)
		schedules.GET("/:id", h.getSchedule)
		schedules.PUT("/:id", h.updateSchedule)
		schedules.DELETE("/:id", h.deleteSchedule)
		schedules.POST("/:id/enable", h.enableSchedule)
		schedules.POST("/:id/disable", h.disableSchedule)
		schedules.POST("/:id/run", h.runScheduleNow)
		schedules.GET("/:id/executions", h.getScheduleExecutions)
	}

	// ========== 导出 ==========
	export := apiGroup.Group("/export")
	{
		export.POST("", h.exportReport)
		export.POST("/batch", h.exportBatch)
		export.GET("/formats", h.getSupportedFormats)
	}

	// ========== 数据源 ==========
	dataSources := apiGroup.Group("/data-sources")
	{
		dataSources.GET("", h.listDataSources)
		dataSources.GET("/:name/fields", h.getDataSourceFields)
	}

	// ========== 快速报表 ==========
	quick := apiGroup.Group("/quick-reports")
	{
		quick.POST("/generate", h.generateQuickReport)
	}

	// ========== v2.29.0 资源分析增强 ==========
	// 注册存储成本、成本优化、容量规划、带宽报告 API
	h.resourceAPI.RegisterResourceRoutes(apiGroup)

	// ========== v2.30.0 资源可视化报告 ==========
	// 注册资源可视化 API
	h.visualizationAPI.RegisterResourceVisualizationRoutes(apiGroup)

	// ========== v2.35.0 增强导出 API ==========
	// 注册增强导出 API
	h.enhancedExportAPI.RegisterEnhancedExportRoutes(apiGroup)

	// ========== v2.35.0 成本分析 API ==========
	// 注册成本分析 API
	h.costAnalysisAPI.RegisterCostAnalysisRoutes(apiGroup)

	// ========== v2.76.0 资源报告增强 API ==========
	// 注册资源报告增强 API
	h.enhancedReportAPI.RegisterEnhancedRoutes(apiGroup)
}

// ========== 模板管理 API ==========

func (h *Handlers) listTemplates(c *gin.Context) {
	templateType := TemplateType(c.Query("type"))
	publicOnly := c.Query("public") == "true"

	templates := h.templateManager.ListTemplates(templateType, publicOnly)
	api.OK(c, templates)
}

func (h *Handlers) createTemplate(c *gin.Context) {
	var input TemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	createdBy := c.GetString("username")
	template, err := h.templateManager.CreateTemplate(input, createdBy)
	if err != nil {
		if err == ErrTemplateExists {
			api.Conflict(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, template)
}

func (h *Handlers) getTemplate(c *gin.Context) {
	id := c.Param("id")
	template, err := h.templateManager.GetTemplate(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, template)
}

func (h *Handlers) updateTemplate(c *gin.Context) {
	id := c.Param("id")
	var input TemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	template, err := h.templateManager.UpdateTemplate(id, input)
	if err != nil {
		if err == ErrTemplateNotFound {
			api.NotFound(c, err.Error())
			return
		}
		if err == ErrTemplateExists {
			api.Conflict(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, template)
}

func (h *Handlers) deleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.templateManager.DeleteTemplate(id); err != nil {
		if err == ErrTemplateNotFound {
			api.NotFound(c, err.Error())
			return
		}
		api.BadRequest(c, err.Error())
		return
	}
	api.OK(c, nil)
}

func (h *Handlers) getTemplateFields(c *gin.Context) {
	id := c.Param("id")
	template, err := h.templateManager.GetTemplate(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, template.Fields)
}

func (h *Handlers) generateFromTemplate(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Parameters map[string]interface{} `json:"parameters"`
		StartTime  *time.Time             `json:"start_time"`
		EndTime    *time.Time             `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body
		req.Parameters = make(map[string]interface{})
	}

	var period *ReportPeriod
	if req.StartTime != nil && req.EndTime != nil {
		period = &ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	}

	report, err := h.generator.GenerateFromTemplate(id, req.Parameters, period)
	if err != nil {
		if err == ErrTemplateNotFound {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, report)
}

// ========== 自定义报表 API ==========

func (h *Handlers) listCustomReports(c *gin.Context) {
	dataSource := c.Query("data_source")
	reports := h.generator.ListCustomReports(dataSource)
	api.OK(c, reports)
}

func (h *Handlers) createCustomReport(c *gin.Context) {
	var input CustomReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	createdBy := c.GetString("username")
	report, err := h.generator.CreateCustomReport(input, createdBy)
	if err != nil {
		if err == ErrDataSourceNotFound {
			api.BadRequest(c, "数据源不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, report)
}

func (h *Handlers) getCustomReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.generator.GetCustomReport(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, report)
}

func (h *Handlers) updateCustomReport(c *gin.Context) {
	id := c.Param("id")
	var input CustomReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	report, err := h.generator.UpdateCustomReport(id, input)
	if err != nil {
		if err == ErrReportNotFound {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, report)
}

func (h *Handlers) deleteCustomReport(c *gin.Context) {
	id := c.Param("id")
	if err := h.generator.DeleteCustomReport(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, nil)
}

func (h *Handlers) generateFromCustomReport(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Parameters map[string]interface{} `json:"parameters"`
		StartTime  *time.Time             `json:"start_time"`
		EndTime    *time.Time             `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		req.Parameters = make(map[string]interface{})
	}

	var period *ReportPeriod
	if req.StartTime != nil && req.EndTime != nil {
		period = &ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	}

	report, err := h.generator.GenerateFromCustomReport(id, req.Parameters, period)
	if err != nil {
		if err == ErrReportNotFound {
			api.NotFound(c, err.Error())
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, report)
}

func (h *Handlers) previewCustomReport(c *gin.Context) {
	id := c.Param("id")

	report, err := h.generator.PreviewReport(id, nil)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, report)
}

// ========== 定时报表 API ==========

func (h *Handlers) listSchedules(c *gin.Context) {
	var enabled *bool
	if e := c.Query("enabled"); e != "" {
		val := e == "true"
		enabled = &val
	}

	schedules := h.scheduleManager.ListSchedules(enabled)
	api.OK(c, schedules)
}

func (h *Handlers) createSchedule(c *gin.Context) {
	var input ScheduledReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	createdBy := c.GetString("username")
	schedule, err := h.scheduleManager.CreateSchedule(input, createdBy)
	if err != nil {
		if err == ErrInvalidCronExpr {
			api.BadRequest(c, "无效的 cron 表达式")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, schedule)
}

func (h *Handlers) getSchedule(c *gin.Context) {
	id := c.Param("id")
	schedule, err := h.scheduleManager.GetSchedule(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, schedule)
}

func (h *Handlers) updateSchedule(c *gin.Context) {
	id := c.Param("id")
	var input ScheduledReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	schedule, err := h.scheduleManager.UpdateSchedule(id, input)
	if err != nil {
		if err == ErrScheduleNotFound {
			api.NotFound(c, err.Error())
			return
		}
		if err == ErrInvalidCronExpr {
			api.BadRequest(c, "无效的 cron 表达式")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, schedule)
}

func (h *Handlers) deleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduleManager.DeleteSchedule(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, nil)
}

func (h *Handlers) enableSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduleManager.EnableSchedule(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, gin.H{"enabled": true})
}

func (h *Handlers) disableSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduleManager.DisableSchedule(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, gin.H{"enabled": false})
}

func (h *Handlers) runScheduleNow(c *gin.Context) {
	id := c.Param("id")
	execution, err := h.scheduleManager.RunNow(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, execution)
}

func (h *Handlers) getScheduleExecutions(c *gin.Context) {
	id := c.Param("id")
	limit := 20
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	executions := h.scheduleManager.GetExecutions(id, limit)
	api.OK(c, executions)
}

// ========== 导出 API ==========

func (h *Handlers) exportReport(c *gin.Context) {
	var req struct {
		ReportID   string                 `json:"report_id"`
		TemplateID string                 `json:"template_id"`
		Format     ExportFormat           `json:"format" binding:"required"`
		OutputPath string                 `json:"output_path"`
		Options    ExportOptions          `json:"options"`
		Parameters map[string]interface{} `json:"parameters"`
		StartTime  *time.Time             `json:"start_time"`
		EndTime    *time.Time             `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 生成报表
	var report *GeneratedReport
	var err error

	if req.ReportID != "" {
		var period *ReportPeriod
		if req.StartTime != nil && req.EndTime != nil {
			period = &ReportPeriod{StartTime: *req.StartTime, EndTime: *req.EndTime}
		}
		report, err = h.generator.GenerateFromCustomReport(req.ReportID, req.Parameters, period)
	} else if req.TemplateID != "" {
		var period *ReportPeriod
		if req.StartTime != nil && req.EndTime != nil {
			period = &ReportPeriod{StartTime: *req.StartTime, EndTime: *req.EndTime}
		}
		report, err = h.generator.GenerateFromTemplate(req.TemplateID, req.Parameters, period)
	} else {
		api.BadRequest(c, "必须指定 report_id 或 template_id")
		return
	}

	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 导出
	result, err := h.exporter.Export(report, req.Format, req.OutputPath, req.Options)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

func (h *Handlers) exportBatch(c *gin.Context) {
	var req struct {
		ReportID   string         `json:"report_id"`
		TemplateID string         `json:"template_id"`
		Formats    []ExportFormat `json:"formats" binding:"required"`
		OutputDir  string         `json:"output_dir"`
		Options    ExportOptions  `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 生成报表
	var report *GeneratedReport
	var err error

	if req.ReportID != "" {
		report, err = h.generator.GenerateFromCustomReport(req.ReportID, nil, nil)
	} else if req.TemplateID != "" {
		report, err = h.generator.GenerateFromTemplate(req.TemplateID, nil, nil)
	} else {
		api.BadRequest(c, "必须指定 report_id 或 template_id")
		return
	}

	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 批量导出
	results, err := h.exporter.ExportMultiple(report, req.Formats, req.OutputDir, req.Options)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, results)
}

func (h *Handlers) getSupportedFormats(c *gin.Context) {
	formats := h.exporter.GetSupportedFormats()

	info := make([]map[string]string, 0, len(formats))
	for _, f := range formats {
		info = append(info, h.exporter.GetFormatInfo(f))
	}

	api.OK(c, info)
}

// ========== 数据源 API ==========

func (h *Handlers) listDataSources(c *gin.Context) {
	sources := h.generator.ListDataSources()
	api.OK(c, sources)
}

func (h *Handlers) getDataSourceFields(c *gin.Context) {
	name := c.Param("name")
	fields, err := h.generator.GetAvailableFields(name)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}
	api.OK(c, fields)
}

// ========== 快速报表 API ==========

func (h *Handlers) generateQuickReport(c *gin.Context) {
	var req struct {
		DataSource string           `json:"data_source" binding:"required"`
		Fields     []TemplateField  `json:"fields" binding:"required"`
		Filters    []TemplateFilter `json:"filters"`
		Sorts      []TemplateSort   `json:"sorts"`
		Limit      int              `json:"limit"`
		StartTime  *time.Time       `json:"start_time"`
		EndTime    *time.Time       `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	var period *ReportPeriod
	if req.StartTime != nil && req.EndTime != nil {
		period = &ReportPeriod{
			StartTime: *req.StartTime,
			EndTime:   *req.EndTime,
		}
	}

	report, err := h.generator.GenerateQuickReport(
		req.DataSource,
		req.Fields,
		req.Filters,
		req.Sorts,
		req.Limit,
		period,
	)

	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, report)
}

func (h *Handlers) getVolumeStats(c *gin.Context) {
	stats := make([]map[string]interface{}, 0)

	if ds, exists := h.generator.DataSources["storage"]; exists {
		fields := []TemplateField{
			{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
			{Name: "total_bytes", Label: "总容量", Type: FieldTypeBytes},
			{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
			{Name: "free_bytes", Label: "可用空间", Type: FieldTypeBytes},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		}
		data, err := ds.Query(nil, fields, nil, nil, nil, nil, 0, 0)
		if err == nil {
			stats = data
		}
	}

	api.OK(c, stats)
}

func (h *Handlers) getVolumeStorageStats(c *gin.Context) {
	volumeName := c.Param("volumeName")

	filters := []TemplateFilter{
		{Field: "volume_name", Operator: "eq", Value: volumeName},
	}

	fields := []TemplateField{
		{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
		{Name: "total_bytes", Label: "总容量", Type: FieldTypeBytes},
		{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
		{Name: "free_bytes", Label: "可用空间", Type: FieldTypeBytes},
		{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		{Name: "user_count", Label: "用户数", Type: FieldTypeNumber},
		{Name: "quota_count", Label: "配额数", Type: FieldTypeNumber},
	}

	var stats map[string]interface{}
	if ds, exists := h.generator.DataSources["storage"]; exists {
		data, err := ds.Query(nil, fields, filters, nil, nil, nil, 0, 0)
		if err == nil && len(data) > 0 {
			stats = data[0]
		}
	}

	if stats == nil {
		api.NotFound(c, "卷不存在或无数据")
		return
	}

	api.OK(c, stats)
}

func (h *Handlers) getTopStorageUsers(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	volumeName := c.Query("volume")

	sorts := []TemplateSort{
		{Field: "used_bytes", Order: "desc"},
	}

	filters := []TemplateFilter{}
	if volumeName != "" {
		filters = append(filters, TemplateFilter{Field: "volume_name", Operator: "eq", Value: volumeName})
	}

	fields := []TemplateField{
		{Name: "username", Label: "用户名", Type: FieldTypeString},
		{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
		{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
		{Name: "limit_bytes", Label: "配额限制", Type: FieldTypeBytes},
		{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
	}

	var users []map[string]interface{}
	if ds, exists := h.generator.DataSources["quota"]; exists {
		data, err := ds.Query(nil, fields, filters, sorts, nil, nil, limit, 0)
		if err == nil {
			users = data
		}
	}

	api.OK(c, users)
}

func (h *Handlers) getStorageDistribution(c *gin.Context) {
	distribution := map[string]interface{}{
		"by_type": map[string]uint64{
			"user":      0,
			"group":     0,
			"directory": 0,
		},
		"by_volume": make(map[string]map[string]interface{}),
	}

	if ds, exists := h.generator.DataSources["quota"]; exists {
		fields := []TemplateField{
			{Name: "type", Label: "类型", Type: FieldTypeString},
			{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
			{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
		}

		data, err := ds.Query(nil, fields, nil, nil, nil, nil, 0, 0)
		if err == nil {
			for _, row := range data {
				if t, ok := row["type"].(string); ok {
					if vol, ok := row["volume_name"].(string); ok {
						if used, ok := row["used_bytes"].(uint64); ok {
							if t == "user" || t == "group" || t == "directory" {
								distribution["by_type"].(map[string]uint64)[t] += used
							}
							if distribution["by_volume"].(map[string]map[string]interface{})[vol] == nil {
								distribution["by_volume"].(map[string]map[string]interface{})[vol] = map[string]interface{}{
									"total": uint64(0),
								}
							}
							distribution["by_volume"].(map[string]map[string]interface{})[vol]["total"] = used
						}
					}
				}
			}
		}
	}

	api.OK(c, distribution)
}

func (h *Handlers) getStorageTrend(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	trend := map[string]interface{}{
		"start_time":  startTime,
		"end_time":    endTime,
		"data_points": []map[string]interface{}{},
	}

	if ds, exists := h.generator.DataSources["monitor"]; exists {
		filters := []TemplateFilter{
			{Field: "timestamp", Operator: "gte", Value: startTime},
			{Field: "timestamp", Operator: "lte", Value: endTime},
		}

		fields := []TemplateField{
			{Name: "timestamp", Label: "时间", Type: FieldTypeDateTime},
			{Name: "total_used", Label: "总使用量", Type: FieldTypeBytes},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		}

		sorts := []TemplateSort{
			{Field: "timestamp", Order: "asc"},
		}

		data, err := ds.Query(nil, fields, filters, sorts, nil, nil, 0, 0)
		if err == nil {
			trend["data_points"] = data
		}
	}

	api.OK(c, trend)
}

// ========== v2.9.0 新增：用户资源使用报告 API 实现 ==========

func (h *Handlers) getUserResourceReport(c *gin.Context) {
	username := c.Param("username")

	report := map[string]interface{}{
		"username":     username,
		"generated_at": time.Now(),
		"quotas":       []map[string]interface{}{},
		"summary":      map[string]interface{}{},
	}

	if ds, exists := h.generator.DataSources["quota"]; exists {
		filters := []TemplateFilter{
			{Field: "username", Operator: "eq", Value: username},
		}

		fields := []TemplateField{
			{Name: "quota_id", Label: "配额ID", Type: FieldTypeString},
			{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
			{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
			{Name: "limit_bytes", Label: "配额限制", Type: FieldTypeBytes},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
			{Name: "status", Label: "状态", Type: FieldTypeString},
		}

		data, err := ds.Query(nil, fields, filters, nil, nil, nil, 0, 0)
		if err == nil {
			report["quotas"] = data

			var totalUsed, totalLimit uint64
			for _, q := range data {
				if used, ok := q["used_bytes"].(uint64); ok {
					totalUsed += used
				}
				if limit, ok := q["limit_bytes"].(uint64); ok {
					totalLimit += limit
				}
			}

			report["summary"] = map[string]interface{}{
				"total_quotas":  len(data),
				"total_used":    totalUsed,
				"total_limit":   totalLimit,
				"usage_percent": float64(0),
			}
			if totalLimit > 0 {
				report["summary"].(map[string]interface{})["usage_percent"] = float64(totalUsed) / float64(totalLimit) * 100
			}
		}
	}

	api.OK(c, report)
}

func (h *Handlers) getUserQuotaReport(c *gin.Context) {
	username := c.Param("username")

	filters := []TemplateFilter{
		{Field: "username", Operator: "eq", Value: username},
		{Field: "type", Operator: "eq", Value: "user"},
	}

	fields := []TemplateField{
		{Name: "quota_id", Label: "配额ID", Type: FieldTypeString},
		{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
		{Name: "path", Label: "路径", Type: FieldTypeString},
		{Name: "hard_limit", Label: "硬限制", Type: FieldTypeBytes},
		{Name: "soft_limit", Label: "软限制", Type: FieldTypeBytes},
		{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
		{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		{Name: "is_over_soft", Label: "超软限制", Type: FieldTypeBoolean},
		{Name: "is_over_hard", Label: "超硬限制", Type: FieldTypeBoolean},
	}

	var quotas []map[string]interface{}
	if ds, exists := h.generator.DataSources["quota"]; exists {
		data, err := ds.Query(nil, fields, filters, nil, nil, nil, 0, 0)
		if err == nil {
			quotas = data
		}
	}

	api.OK(c, quotas)
}

func (h *Handlers) getUserStorageReport(c *gin.Context) {
	username := c.Param("username")

	report := map[string]interface{}{
		"username":     username,
		"generated_at": time.Now(),
		"by_volume":    []map[string]interface{}{},
	}

	if ds, exists := h.generator.DataSources["quota"]; exists {
		filters := []TemplateFilter{
			{Field: "username", Operator: "eq", Value: username},
		}

		fields := []TemplateField{
			{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
			{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
			{Name: "limit_bytes", Label: "配额限制", Type: FieldTypeBytes},
		}

		groupBy := []string{"volume_name"}

		data, err := ds.Query(nil, fields, filters, nil, nil, groupBy, 0, 0)
		if err == nil {
			report["by_volume"] = data
		}
	}

	api.OK(c, report)
}

func (h *Handlers) getUserResourceTrend(c *gin.Context) {
	username := c.Param("username")
	days := 7
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	trend := map[string]interface{}{
		"username":    username,
		"start_time":  startTime,
		"end_time":    endTime,
		"data_points": []map[string]interface{}{},
	}

	if ds, exists := h.generator.DataSources["monitor"]; exists {
		filters := []TemplateFilter{
			{Field: "username", Operator: "eq", Value: username},
			{Field: "timestamp", Operator: "gte", Value: startTime},
			{Field: "timestamp", Operator: "lte", Value: endTime},
		}

		fields := []TemplateField{
			{Name: "timestamp", Label: "时间", Type: FieldTypeDateTime},
			{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
			{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		}

		sorts := []TemplateSort{
			{Field: "timestamp", Order: "asc"},
		}

		data, err := ds.Query(nil, fields, filters, sorts, nil, nil, 0, 0)
		if err == nil {
			trend["data_points"] = data
		}
	}

	api.OK(c, trend)
}

func (h *Handlers) exportUserResourceReport(c *gin.Context) {
	username := c.Param("username")
	format := c.Query("format")
	if format == "" {
		format = "json"
	}

	report, err := h.generator.GenerateQuickReport(
		"quota",
		[]TemplateField{
			{Name: "username", Label: "用户名", Type: FieldTypeString},
			{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
			{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
			{Name: "limit_bytes", Label: "配额限制", Type: FieldTypeBytes},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		},
		[]TemplateFilter{
			{Field: "username", Operator: "eq", Value: username},
		},
		nil,
		0,
		nil,
	)

	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	result, err := h.exporter.Export(report, ExportFormat(format), "", ExportOptions{
		Title:         "用户资源使用报告 - " + username,
		IncludeHeader: true,
		Summary:       true,
	})

	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// ========== v2.9.0 新增：系统资源趋势分析 API 实现 ==========

func (h *Handlers) getSystemTrendAnalysis(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	analysis := map[string]interface{}{
		"generated_at":  time.Now(),
		"start_time":    startTime,
		"end_time":      endTime,
		"duration_days": days,
		"storage":       map[string]interface{}{},
		"quotas":        map[string]interface{}{},
		"alerts":        map[string]interface{}{},
		"trend":         "stable",
	}

	if ds, exists := h.generator.DataSources["monitor"]; exists {
		filters := []TemplateFilter{
			{Field: "timestamp", Operator: "gte", Value: startTime},
			{Field: "timestamp", Operator: "lte", Value: endTime},
		}

		storageFields := []TemplateField{
			{Name: "timestamp", Label: "时间", Type: FieldTypeDateTime},
			{Name: "total_used", Label: "总使用量", Type: FieldTypeBytes},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		}

		storageData, err := ds.Query(nil, storageFields, filters, nil, nil, nil, 0, 0)
		if err == nil {
			analysis["storage"] = map[string]interface{}{
				"data_points": storageData,
			}

			if len(storageData) >= 2 {
				first := storageData[0]
				last := storageData[len(storageData)-1]

				var firstVal, lastVal float64
				if v, ok := first["usage_percent"].(float64); ok {
					firstVal = v
				}
				if v, ok := last["usage_percent"].(float64); ok {
					lastVal = v
				}

				change := lastVal - firstVal
				if change > 5 {
					analysis["trend"] = "increasing"
				} else if change < -5 {
					analysis["trend"] = "decreasing"
				}
			}
		}

		quotaFields := []TemplateField{
			{Name: "timestamp", Label: "时间", Type: FieldTypeDateTime},
			{Name: "total_quotas", Label: "总配额数", Type: FieldTypeNumber},
			{Name: "over_soft_count", Label: "超软限制数", Type: FieldTypeNumber},
			{Name: "over_hard_count", Label: "超硬限制数", Type: FieldTypeNumber},
		}

		quotaData, err := ds.Query(nil, quotaFields, filters, nil, nil, nil, 0, 0)
		if err == nil {
			analysis["quotas"] = map[string]interface{}{
				"data_points": quotaData,
			}
		}
	}

	api.OK(c, analysis)
}

func (h *Handlers) getSystemTrendSummary(c *gin.Context) {
	summary := map[string]interface{}{
		"generated_at":        time.Now(),
		"total_quotas":        0,
		"total_users":         0,
		"total_storage_used":  uint64(0),
		"total_storage_limit": uint64(0),
		"avg_usage_percent":   float64(0),
		"trend_direction":     "stable",
		"growth_rate":         float64(0),
	}

	if ds, exists := h.generator.DataSources["quota"]; exists {
		dsSummary, err := ds.GetSummary(nil)
		if err == nil {
			if v, ok := dsSummary["total_quotas"].(int); ok {
				summary["total_quotas"] = v
			}
			if v, ok := dsSummary["total_users"].(int); ok {
				summary["total_users"] = v
			}
			if v, ok := dsSummary["total_used"].(uint64); ok {
				summary["total_storage_used"] = v
			}
			if v, ok := dsSummary["total_limit"].(uint64); ok {
				summary["total_storage_limit"] = v
			}
		}
	}

	if limit, ok := summary["total_storage_limit"].(uint64); ok && limit > 0 {
		if used, ok := summary["total_storage_used"].(uint64); ok {
			summary["avg_usage_percent"] = float64(used) / float64(limit) * 100
		}
	}

	api.OK(c, summary)
}

func (h *Handlers) getSystemPrediction(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	prediction := map[string]interface{}{
		"generated_at":      time.Now(),
		"prediction_days":   days,
		"method":            "linear",
		"current_usage":     float64(0),
		"predicted_usage":   float64(0),
		"growth_rate_daily": float64(0),
		"days_to_capacity":  0,
		"confidence":        float64(0.7),
		"warning_message":   "",
	}

	if ds, exists := h.generator.DataSources["monitor"]; exists {
		startTime := time.Now().AddDate(0, 0, -7)
		filters := []TemplateFilter{
			{Field: "timestamp", Operator: "gte", Value: startTime},
		}

		fields := []TemplateField{
			{Name: "timestamp", Label: "时间", Type: FieldTypeDateTime},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		}

		sorts := []TemplateSort{
			{Field: "timestamp", Order: "asc"},
		}

		data, err := ds.Query(nil, fields, filters, sorts, nil, nil, 0, 0)
		if err == nil && len(data) >= 2 {
			var firstVal, lastVal float64
			if v, ok := data[0]["usage_percent"].(float64); ok {
				firstVal = v
			}
			if v, ok := data[len(data)-1]["usage_percent"].(float64); ok {
				lastVal = v
			}

			prediction["current_usage"] = lastVal

			dailyGrowth := (lastVal - firstVal) / 7.0
			prediction["growth_rate_daily"] = dailyGrowth
			prediction["predicted_usage"] = lastVal + dailyGrowth*float64(days)

			if dailyGrowth > 0 {
				daysToFull := int((100.0 - lastVal) / dailyGrowth)
				prediction["days_to_capacity"] = daysToFull

				if daysToFull <= 30 {
					prediction["warning_message"] = fmt.Sprintf("按当前增长趋势，预计 %d 天后将达到存储容量上限", daysToFull)
				}
			}
		}
	}

	api.OK(c, prediction)
}

func (h *Handlers) getSystemAlertTrend(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	trend := map[string]interface{}{
		"generated_at": time.Now(),
		"start_time":   startTime,
		"end_time":     endTime,
		"data_points":  []map[string]interface{}{},
		"summary": map[string]interface{}{
			"total_alerts":   0,
			"critical_count": 0,
			"warning_count":  0,
			"resolved_count": 0,
		},
	}

	if ds, exists := h.generator.DataSources["alert"]; exists {
		filters := []TemplateFilter{
			{Field: "timestamp", Operator: "gte", Value: startTime},
			{Field: "timestamp", Operator: "lte", Value: endTime},
		}

		fields := []TemplateField{
			{Name: "timestamp", Label: "时间", Type: FieldTypeDateTime},
			{Name: "severity", Label: "严重级别", Type: FieldTypeString},
			{Name: "status", Label: "状态", Type: FieldTypeString},
			{Name: "count", Label: "数量", Type: FieldTypeNumber},
		}

		sorts := []TemplateSort{
			{Field: "timestamp", Order: "asc"},
		}

		data, err := ds.Query(nil, fields, filters, sorts, nil, nil, 0, 0)
		if err == nil {
			trend["data_points"] = data

			var totalAlerts, criticalCount, warningCount, resolvedCount int
			for _, row := range data {
				if count, ok := row["count"].(int); ok {
					totalAlerts += count
				}
				if severity, ok := row["severity"].(string); ok {
					if severity == "critical" || severity == "emergency" {
						criticalCount++
					} else if severity == "warning" {
						warningCount++
					}
				}
				if status, ok := row["status"].(string); ok {
					if status == "resolved" {
						resolvedCount++
					}
				}
			}

			trend["summary"] = map[string]interface{}{
				"total_alerts":   totalAlerts,
				"critical_count": criticalCount,
				"warning_count":  warningCount,
				"resolved_count": resolvedCount,
			}
		}
	}

	api.OK(c, trend)
}

func (h *Handlers) getCapacityAnalysis(c *gin.Context) {
	analysis := map[string]interface{}{
		"generated_at":    time.Now(),
		"volumes":         []map[string]interface{}{},
		"total_capacity":  uint64(0),
		"total_used":      uint64(0),
		"total_available": uint64(0),
		"utilization":     float64(0),
		"recommendations": []string{},
	}

	if ds, exists := h.generator.DataSources["storage"]; exists {
		fields := []TemplateField{
			{Name: "volume_name", Label: "卷名称", Type: FieldTypeString},
			{Name: "total_bytes", Label: "总容量", Type: FieldTypeBytes},
			{Name: "used_bytes", Label: "已用空间", Type: FieldTypeBytes},
			{Name: "free_bytes", Label: "可用空间", Type: FieldTypeBytes},
			{Name: "usage_percent", Label: "使用率", Type: FieldTypePercent},
		}

		data, err := ds.Query(nil, fields, nil, nil, nil, nil, 0, 0)
		if err == nil {
			analysis["volumes"] = data

			var totalCapacity, totalUsed, totalAvailable uint64
			recommendations := make([]string, 0)

			for _, vol := range data {
				if total, ok := vol["total_bytes"].(uint64); ok {
					totalCapacity += total
				}
				if used, ok := vol["used_bytes"].(uint64); ok {
					totalUsed += used
				}
				if free, ok := vol["free_bytes"].(uint64); ok {
					totalAvailable += free
				}

				if usage, ok := vol["usage_percent"].(float64); ok {
					volName, _ := vol["volume_name"].(string)
					if usage >= 90 {
						recommendations = append(recommendations,
							fmt.Sprintf("卷 %s 使用率已达 %.1f%%，建议立即扩容或清理", volName, usage))
					} else if usage >= 80 {
						recommendations = append(recommendations,
							fmt.Sprintf("卷 %s 使用率已达 %.1f%%，建议规划扩容", volName, usage))
					}
				}
			}

			analysis["total_capacity"] = totalCapacity
			analysis["total_used"] = totalUsed
			analysis["total_available"] = totalAvailable

			if totalCapacity > 0 {
				analysis["utilization"] = float64(totalUsed) / float64(totalCapacity) * 100
			}

			analysis["recommendations"] = recommendations
		}
	}

	api.OK(c, analysis)
}
