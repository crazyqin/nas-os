// Package audittrail 合规审计追踪 - HTTP API
package audittrail

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/audit-trail")
	{
		group.GET("/events", h.QueryEvents)
		group.POST("/events", h.LogEvent)
		group.GET("/events/:id", h.GetEvent)
		group.GET("/reports", h.GetReports)
		group.POST("/reports/generate", h.GenerateReport)
		group.GET("/alert-rules", h.GetAlertRules)
		group.POST("/alert-rules", h.AddAlertRule)
		group.PUT("/alert-rules/:id", h.UpdateAlertRule)
		group.DELETE("/alert-rules/:id", h.DeleteAlertRule)
		group.GET("/export", h.ExportEvents)
	}
}

// LogEvent 记录事件
func (h *Handler) LogEvent(c *gin.Context) {
	var event AuditEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效参数"})
		return
	}
	h.manager.LogEvent(event)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "事件已记录"})
}

// GetEvent 获取事件详情
func (h *Handler) GetEvent(c *gin.Context) {
	id := c.Param("id")
	event, err := h.manager.GetEvent(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": event})
}

// QueryEvents 查询事件
func (h *Handler) QueryEvents(c *gin.Context) {
	filter := EventFilter{
		UserID:    c.Query("user_id"),
		Action:    c.Query("action"),
		RiskLevel: RiskLevel(c.Query("risk_level")),
	}

	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			filter.Limit = v
		}
	}
	if t := c.Query("start_time"); t != "" {
		if v, err := time.Parse(time.RFC3339, t); err == nil {
			filter.StartTime = v
		}
	}
	if t := c.Query("end_time"); t != "" {
		if v, err := time.Parse(time.RFC3339, t); err == nil {
			filter.EndTime = v
		}
	}

	events := h.manager.QueryEvents(filter)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": events})
}

// GetReports 获取报告列表
func (h *Handler) GetReports(c *gin.Context) {
	reports := h.manager.GetReports()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": reports})
}

// GenerateReport 生成报告
func (h *Handler) GenerateReport(c *gin.Context) {
	var req struct {
		Period string `json:"period"`
	}
	c.ShouldBindJSON(&req)
	if req.Period == "" {
		req.Period = "last_30_days"
	}
	report := h.manager.GenerateReport(req.Period)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "报告已生成", "data": report})
}

// GetAlertRules 获取告警规则列表
func (h *Handler) GetAlertRules(c *gin.Context) {
	rules := h.manager.GetAlertRules()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": rules})
}

// AddAlertRule 创建告警规则
func (h *Handler) AddAlertRule(c *gin.Context) {
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效参数"})
		return
	}
	created := h.manager.AddAlertRule(rule)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "规则已创建", "data": created})
}

// UpdateAlertRule 更新告警规则
func (h *Handler) UpdateAlertRule(c *gin.Context) {
	id := c.Param("id")
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效参数"})
		return
	}
	if err := h.manager.UpdateAlertRule(id, rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已更新"})
}

// DeleteAlertRule 删除告警规则
func (h *Handler) DeleteAlertRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteAlertRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已删除"})
}

// ExportEvents 导出事件
func (h *Handler) ExportEvents(c *gin.Context) {
	format := ExportFormat(c.DefaultQuery("format", "json"))
	data, err := h.manager.ExportEvents(format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	contentType := "application/json"
	if format == FormatCSV {
		contentType = "text/csv"
	}

	c.Data(http.StatusOK, contentType, data)
}
