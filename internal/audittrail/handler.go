// Package audittrail 提供审计追踪 REST API 处理器
package audittrail

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 审计追踪 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	audit := r.Group("/audit")
	{
		// 事件管理
		audit.GET("/events", h.listEvents)
		audit.POST("/events", h.recordEvent)
		audit.GET("/events/:id", h.getEvent)

		// 可疑行为
		audit.GET("/suspicious", h.listSuspicious)
		audit.GET("/suspicious/:id", h.getSuspicious)
		audit.PUT("/suspicious/:id", h.updateSuspicious)

		// 报告
		audit.POST("/report", h.generateReport)
		audit.GET("/report", h.listReports)
		audit.GET("/report/:id", h.getReport)

		// 保留策略
		audit.POST("/retention", h.setRetention)
		audit.GET("/retention", h.listRetentions)
		audit.GET("/retention/:id", h.getRetention)
		audit.GET("/retention/:id/stats", h.getRetentionStats)

		// 导出
		audit.POST("/export", h.exportAudit)
		audit.GET("/export", h.listExports)
		audit.GET("/export/:id", h.getExport)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listEvents 获取事件列表
func (h *Handlers) listEvents(c *gin.Context) {
	filter := &EventFilter{}

	if start := c.Query("start_time"); start != "" {
		t, err := time.Parse(time.RFC3339, start)
		if err == nil {
			filter.StartTime = &t
		}
	}
	if end := c.Query("end_time"); end != "" {
		t, err := time.Parse(time.RFC3339, end)
		if err == nil {
			filter.EndTime = &t
		}
	}
	if eventTypes := c.QueryArray("event_type"); len(eventTypes) > 0 {
		filter.EventTypes = eventTypes
	}
	if severities := c.QueryArray("severity"); len(severities) > 0 {
		filter.Severities = severities
	}

	events, total := h.manager.ListEvents(filter)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data: map[string]interface{}{
			"events": events,
			"total":  total,
		},
	})
}

// recordEvent 记录事件
func (h *Handlers) recordEvent(c *gin.Context) {
	var event AuditEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	h.manager.RecordEvent(&event)
	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Event recorded",
		Data:    event,
	})
}

// getEvent 获取事件
func (h *Handlers) getEvent(c *gin.Context) {
	id := c.Param("id")
	event, err := h.manager.GetEvent(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    event,
	})
}

// listSuspicious 获取可疑行为列表
func (h *Handlers) listSuspicious(c *gin.Context) {
	filter := &SuspiciousFilter{}

	if types := c.QueryArray("type"); len(types) > 0 {
		filter.Types = types
	}
	if statuses := c.QueryArray("status"); len(statuses) > 0 {
		filter.Statuses = statuses
	}

	activities, total := h.manager.DetectSuspicious(filter)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data: map[string]interface{}{
			"activities": activities,
			"total":      total,
		},
	})
}

// getSuspicious 获取可疑行为详情
func (h *Handlers) getSuspicious(c *gin.Context) {
	id := c.Param("id")
	activities, _ := h.manager.DetectSuspicious(nil)
	for _, activity := range activities {
		if activity.ID == id {
			c.JSON(http.StatusOK, response{
				Code:    http.StatusOK,
				Message: "success",
				Data:    activity,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, response{
		Code:    http.StatusNotFound,
		Message: "Suspicious activity not found",
	})
}

// updateSuspicious 更新可疑行为状态
func (h *Handlers) updateSuspicious(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status     string `json:"status"`
		AssignedTo string `json:"assigned_to,omitempty"`
		Notes      string `json:"notes,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.UpdateSuspiciousStatus(id, req.Status, req.AssignedTo, req.Notes); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Status updated",
	})
}

// generateReport 生成报告
func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		Title       string       `json:"title"`
		Type        string       `json:"type"`
		Period      ReportPeriod `json:"period"`
		GeneratedBy string       `json:"generated_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if req.Period.Start.IsZero() {
		req.Period.Start = time.Now().AddDate(0, -1, 0)
	}
	if req.Period.End.IsZero() {
		req.Period.End = time.Now()
	}
	if req.Title == "" {
		req.Title = "审计报告"
	}
	if req.Type == "" {
		req.Type = "summary"
	}

	report, err := h.manager.GenerateReport(req.Title, req.Type, req.Period, req.GeneratedBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Report generated",
		Data:    report,
	})
}

// listReports 获取报告列表
func (h *Handlers) listReports(c *gin.Context) {
	reportType := c.Query("type")
	reports := h.manager.ListReports(reportType)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    reports,
	})
}

// getReport 获取报告
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    report,
	})
}

// setRetention 设置保留策略
func (h *Handlers) setRetention(c *gin.Context) {
	var policy RetentionPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.SetRetention(&policy); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Retention policy set",
		Data:    policy,
	})
}

// listRetentions 获取保留策略列表
func (h *Handlers) listRetentions(c *gin.Context) {
	policies := h.manager.ListRetentions()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    policies,
	})
}

// getRetention 获取保留策略
func (h *Handlers) getRetention(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetRetention(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    policy,
	})
}

// getRetentionStats 获取保留统计
func (h *Handlers) getRetentionStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetRetentionStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    stats,
	})
}

// exportAudit 导出审计数据
func (h *Handlers) exportAudit(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if req.Format == "" {
		req.Format = "json"
	}

	export, err := h.manager.ExportAudit(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Export initiated",
		Data:    export,
	})
}

// listExports 获取导出列表
func (h *Handlers) listExports(c *gin.Context) {
	exports := h.manager.ListExports()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    exports,
	})
}

// getExport 获取导出状态
func (h *Handlers) getExport(c *gin.Context) {
	id := c.Param("id")
	export, err := h.manager.GetExport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    export,
	})
}
