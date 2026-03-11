// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 报表 HTTP 处理器
type Handlers struct {
	templateManager *TemplateManager
	generator       *ReportGenerator
	scheduleManager *ScheduleManager
	exporter        *Exporter
}

// NewHandlers 创建处理器
func NewHandlers(tm *TemplateManager, gen *ReportGenerator, sm *ScheduleManager, exp *Exporter) *Handlers {
	return &Handlers{
		templateManager: tm,
		generator:       gen,
		scheduleManager: sm,
		exporter:        exp,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	// ========== 模板管理 ==========
	templates := api.Group("/report-templates")
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
	reports := api.Group("/custom-reports")
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
	schedules := api.Group("/scheduled-reports")
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
	export := api.Group("/export")
	{
		export.POST("", h.exportReport)
		export.POST("/batch", h.exportBatch)
		export.GET("/formats", h.getSupportedFormats)
	}

	// ========== 数据源 ==========
	dataSources := api.Group("/data-sources")
	{
		dataSources.GET("", h.listDataSources)
		dataSources.GET("/:name/fields", h.getDataSourceFields)
	}

	// ========== 快速报表 ==========
	quick := api.Group("/quick-reports")
	{
		quick.POST("/generate", h.generateQuickReport)
	}
}

// ========== 通用响应 ==========

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== 模板管理 API ==========

func (h *Handlers) listTemplates(c *gin.Context) {
	templateType := TemplateType(c.Query("type"))
	publicOnly := c.Query("public") == "true"
	
	templates := h.templateManager.ListTemplates(templateType, publicOnly)
	c.JSON(http.StatusOK, Success(templates))
}

func (h *Handlers) createTemplate(c *gin.Context) {
	var input TemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	
	createdBy := c.GetString("username")
	template, err := h.templateManager.CreateTemplate(input, createdBy)
	if err != nil {
		if err == ErrTemplateExists {
			c.JSON(http.StatusConflict, Error(409, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusCreated, Success(template))
}

func (h *Handlers) getTemplate(c *gin.Context) {
	id := c.Param("id")
	template, err := h.templateManager.GetTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(template))
}

func (h *Handlers) updateTemplate(c *gin.Context) {
	id := c.Param("id")
	var input TemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	
	template, err := h.templateManager.UpdateTemplate(id, input)
	if err != nil {
		if err == ErrTemplateNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		if err == ErrTemplateExists {
			c.JSON(http.StatusConflict, Error(409, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(template))
}

func (h *Handlers) deleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.templateManager.DeleteTemplate(id); err != nil {
		if err == ErrTemplateNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getTemplateFields(c *gin.Context) {
	id := c.Param("id")
	template, err := h.templateManager.GetTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(template.Fields))
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
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(report))
}

// ========== 自定义报表 API ==========

func (h *Handlers) listCustomReports(c *gin.Context) {
	dataSource := c.Query("data_source")
	reports := h.generator.ListCustomReports(dataSource)
	c.JSON(http.StatusOK, Success(reports))
}

func (h *Handlers) createCustomReport(c *gin.Context) {
	var input CustomReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	
	createdBy := c.GetString("username")
	report, err := h.generator.CreateCustomReport(input, createdBy)
	if err != nil {
		if err == ErrDataSourceNotFound {
			c.JSON(http.StatusBadRequest, Error(400, "数据源不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusCreated, Success(report))
}

func (h *Handlers) getCustomReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.generator.GetCustomReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(report))
}

func (h *Handlers) updateCustomReport(c *gin.Context) {
	id := c.Param("id")
	var input CustomReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	
	report, err := h.generator.UpdateCustomReport(id, input)
	if err != nil {
		if err == ErrReportNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(report))
}

func (h *Handlers) deleteCustomReport(c *gin.Context) {
	id := c.Param("id")
	if err := h.generator.DeleteCustomReport(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
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
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(report))
}

func (h *Handlers) previewCustomReport(c *gin.Context) {
	id := c.Param("id")
	
	report, err := h.generator.PreviewReport(id, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(report))
}

// ========== 定时报表 API ==========

func (h *Handlers) listSchedules(c *gin.Context) {
	var enabled *bool
	if e := c.Query("enabled"); e != "" {
		val := e == "true"
		enabled = &val
	}
	
	schedules := h.scheduleManager.ListSchedules(enabled)
	c.JSON(http.StatusOK, Success(schedules))
}

func (h *Handlers) createSchedule(c *gin.Context) {
	var input ScheduledReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	
	createdBy := c.GetString("username")
	schedule, err := h.scheduleManager.CreateSchedule(input, createdBy)
	if err != nil {
		if err == ErrInvalidCronExpr {
			c.JSON(http.StatusBadRequest, Error(400, "无效的 cron 表达式"))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusCreated, Success(schedule))
}

func (h *Handlers) getSchedule(c *gin.Context) {
	id := c.Param("id")
	schedule, err := h.scheduleManager.GetSchedule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(schedule))
}

func (h *Handlers) updateSchedule(c *gin.Context) {
	id := c.Param("id")
	var input ScheduledReportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}
	
	schedule, err := h.scheduleManager.UpdateSchedule(id, input)
	if err != nil {
		if err == ErrScheduleNotFound {
			c.JSON(http.StatusNotFound, Error(404, err.Error()))
			return
		}
		if err == ErrInvalidCronExpr {
			c.JSON(http.StatusBadRequest, Error(400, "无效的 cron 表达式"))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(schedule))
}

func (h *Handlers) deleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduleManager.DeleteSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) enableSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduleManager.EnableSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(gin.H{"enabled": true}))
}

func (h *Handlers) disableSchedule(c *gin.Context) {
	id := c.Param("id")
	if err := h.scheduleManager.DisableSchedule(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(gin.H{"enabled": false}))
}

func (h *Handlers) runScheduleNow(c *gin.Context) {
	id := c.Param("id")
	execution, err := h.scheduleManager.RunNow(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(execution))
}

func (h *Handlers) getScheduleExecutions(c *gin.Context) {
	id := c.Param("id")
	limit := 20
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	
	executions := h.scheduleManager.GetExecutions(id, limit)
	c.JSON(http.StatusOK, Success(executions))
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
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
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
		c.JSON(http.StatusBadRequest, Error(400, "必须指定 report_id 或 template_id"))
		return
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	// 导出
	result, err := h.exporter.Export(report, req.Format, req.OutputPath, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(result))
}

func (h *Handlers) exportBatch(c *gin.Context) {
	var req struct {
		ReportID   string        `json:"report_id"`
		TemplateID string        `json:"template_id"`
		Formats    []ExportFormat `json:"formats" binding:"required"`
		OutputDir  string        `json:"output_dir"`
		Options    ExportOptions `json:"options"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
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
		c.JSON(http.StatusBadRequest, Error(400, "必须指定 report_id 或 template_id"))
		return
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	// 批量导出
	results, err := h.exporter.ExportMultiple(report, req.Formats, req.OutputDir, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(results))
}

func (h *Handlers) getSupportedFormats(c *gin.Context) {
	formats := h.exporter.GetSupportedFormats()
	
	info := make([]map[string]string, 0, len(formats))
	for _, f := range formats {
		info = append(info, h.exporter.GetFormatInfo(f))
	}
	
	c.JSON(http.StatusOK, Success(info))
}

// ========== 数据源 API ==========

func (h *Handlers) listDataSources(c *gin.Context) {
	sources := h.generator.ListDataSources()
	c.JSON(http.StatusOK, Success(sources))
}

func (h *Handlers) getDataSourceFields(c *gin.Context) {
	name := c.Param("name")
	fields, err := h.generator.GetAvailableFields(name)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(fields))
}

// ========== 快速报表 API ==========

func (h *Handlers) generateQuickReport(c *gin.Context) {
	var req struct {
		DataSource string                 `json:"data_source" binding:"required"`
		Fields     []TemplateField        `json:"fields" binding:"required"`
		Filters    []TemplateFilter       `json:"filters"`
		Sorts      []TemplateSort         `json:"sorts"`
		Limit      int                    `json:"limit"`
		StartTime  *time.Time             `json:"start_time"`
		EndTime    *time.Time             `json:"end_time"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
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
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	
	c.JSON(http.StatusOK, Success(report))
}