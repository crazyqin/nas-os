// Package netdataex 提供 Netdata 高级系统监控功能
package netdataex

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 监控 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	monitoring := r.Group("/monitoring")
	{
		// 指标
		monitoring.GET("/metrics", h.getAllMetrics)
		monitoring.GET("/metrics/:name", h.getMetrics)
		monitoring.GET("/metrics/:name/latest", h.getLatestMetric)

		// 告警规则
		monitoring.POST("/alerts/rules", h.createAlertRule)
		monitoring.GET("/alerts/rules", h.getAlertRules)

		// 告警事件
		monitoring.GET("/alerts/events", h.getAlertEvents)
		monitoring.POST("/alerts/events/:id/ack", h.acknowledgeAlert)

		// 仪表板
		monitoring.POST("/dashboards", h.createDashboard)
		monitoring.GET("/dashboards", h.listDashboards)
		monitoring.GET("/dashboards/:id", h.getDashboard)
		monitoring.PUT("/dashboards/:id", h.updateDashboard)

		// 健康报告
		monitoring.GET("/health", h.getHealthReport)

		// 导出
		monitoring.GET("/export", h.exportMetrics)
	}
}

// getMetrics 获取指标数据
func (h *Handlers) getMetrics(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric name is required"})
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	var from, to time.Time
	if fromStr != "" {
		var err error
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from time format"})
			return
		}
	} else {
		from = time.Now().Add(-1 * time.Hour)
	}

	if toStr != "" {
		var err error
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to time format"})
			return
		}
	} else {
		to = time.Now()
	}

	series, err := h.manager.GetMetrics(name, from, to)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": series})
}

// getLatestMetric 获取最新指标
func (h *Handlers) getLatestMetric(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric name is required"})
		return
	}

	point, err := h.manager.GetLatestMetric(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": point})
}

// getAllMetrics 获取所有指标
func (h *Handlers) getAllMetrics(c *gin.Context) {
	metrics, err := h.manager.GetAllMetrics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

// createAlertRule 创建告警规则
func (h *Handlers) createAlertRule(c *gin.Context) {
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.CreateAlertRule(rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": rule})
}

// getAlertRules 获取所有告警规则
func (h *Handlers) getAlertRules(c *gin.Context) {
	rules, err := h.manager.GetAlertRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// getAlertEvents 获取告警事件
func (h *Handlers) getAlertEvents(c *gin.Context) {
	severity := c.Query("severity")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	events, err := h.manager.GetAlertEvents(severity, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": events})
}

// acknowledgeAlert 确认告警
func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event ID is required"})
		return
	}

	var req struct {
		User string `json:"user"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AcknowledgeAlert(id, req.User); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "alert acknowledged"})
}

// createDashboard 创建仪表板
func (h *Handlers) createDashboard(c *gin.Context) {
	var dashboard Dashboard
	if err := c.ShouldBindJSON(&dashboard); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.CreateDashboard(dashboard); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": dashboard})
}

// getDashboard 获取仪表板
func (h *Handlers) getDashboard(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboard ID is required"})
		return
	}

	dashboard, err := h.manager.GetDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dashboard})
}

// listDashboards 获取所有仪表板
func (h *Handlers) listDashboards(c *gin.Context) {
	dashboards, err := h.manager.ListDashboards()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dashboards})
}

// updateDashboard 更新仪表板
func (h *Handlers) updateDashboard(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dashboard ID is required"})
		return
	}

	var dashboard Dashboard
	if err := c.ShouldBindJSON(&dashboard); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dashboard.ID = id
	if err := h.manager.UpdateDashboard(dashboard); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dashboard})
}

// getHealthReport 获取健康报告
func (h *Handlers) getHealthReport(c *gin.Context) {
	report, err := h.manager.GetHealthReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": report})
}

// exportMetrics 导出指标
func (h *Handlers) exportMetrics(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	var from, to time.Time
	if fromStr != "" {
		var err error
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from time format"})
			return
		}
	} else {
		from = time.Now().Add(-1 * time.Hour)
	}

	if toStr != "" {
		var err error
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to time format"})
			return
		}
	} else {
		to = time.Now()
	}

	data, err := h.manager.ExportMetrics(format, from, to)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if format == "prometheus" {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
	} else {
		c.Data(http.StatusOK, "application/json", data)
	}
}
