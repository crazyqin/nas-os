// Package complianceengine - 合规引擎 HTTP API 处理器
package complianceengine

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 合规引擎 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/compliance 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	compliance := r.Group("/compliance")
	{
		// 规则管理
		compliance.POST("/rules", h.createRule)
		compliance.GET("/rules", h.listRules)
		compliance.GET("/rules/:id", h.getRule)
		compliance.PUT("/rules/:id", h.updateRule)
		compliance.DELETE("/rules/:id", h.deleteRule)

		// 扫描管理
		compliance.POST("/scans", h.startScan)
		compliance.GET("/scans", h.listScans)
		compliance.GET("/scans/:id", h.getScan)

		// 报告管理
		compliance.POST("/reports", h.generateReport)
		compliance.GET("/reports", h.listReports)
		compliance.GET("/reports/:id", h.getReport)

		// 差距分析
		compliance.POST("/gap-analysis", h.performGapAnalysis)

		// 告警管理
		compliance.GET("/alerts", h.listAlerts)
		compliance.GET("/alerts/:id", h.getAlert)
		compliance.PUT("/alerts/:id/acknowledge", h.acknowledgeAlert)
		compliance.PUT("/alerts/:id/resolve", h.resolveAlert)

		// 任务管理
		compliance.POST("/tasks", h.createTask)
		compliance.GET("/tasks", h.listTasks)
		compliance.GET("/tasks/:id", h.getTask)
		compliance.PUT("/tasks/:id/status", h.updateTaskStatus)

		// 统计
		compliance.GET("/stats", h.getStats)
		compliance.GET("/config", h.getConfig)
		compliance.PUT("/config", h.updateConfig)
	}
}

// ========== 规则管理处理器 ==========

func (h *Handlers) createRule(c *gin.Context) {
	var req ComplianceRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.manager.CreateRule(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (h *Handlers) listRules(c *gin.Context) {
	standard := ComplianceStandard(c.Query("standard"))
	category := RuleCategory(c.Query("category"))

	rules := h.manager.ListRules(standard, category)
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var req ComplianceRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.manager.UpdateRule(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "规则已删除"})
}

// ========== 扫描管理处理器 ==========

func (h *Handlers) startScan(c *gin.Context) {
	var req struct {
		Standards []ComplianceStandard `json:"standards" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scan, err := h.manager.StartScan(req.Standards)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scan)
}

func (h *Handlers) listScans(c *gin.Context) {
	status := ScanStatus(c.Query("status"))
	scans := h.manager.ListScans(status)
	c.JSON(http.StatusOK, gin.H{
		"scans": scans,
		"total": len(scans),
	})
}

func (h *Handlers) getScan(c *gin.Context) {
	id := c.Param("id")
	scan, err := h.manager.GetScan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scan)
}

// ========== 报告管理处理器 ==========

func (h *Handlers) generateReport(c *gin.Context) {
	var req struct {
		ScanID string       `json:"scanId" binding:"required"`
		Format ReportFormat `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Format == "" {
		req.Format = FormatJSON
	}

	report, err := h.manager.GenerateReport(req.ScanID, req.Format)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

func (h *Handlers) listReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"total":   len(reports),
	})
}

func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// ========== 差距分析处理器 ==========

func (h *Handlers) performGapAnalysis(c *gin.Context) {
	var req struct {
		Standards []ComplianceStandard `json:"standards" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analysis, err := h.manager.PerformGapAnalysis(req.Standards)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

// ========== 告警管理处理器 ==========

func (h *Handlers) listAlerts(c *gin.Context) {
	severity := AlertSeverity(c.Query("severity"))
	status := c.Query("status")

	alerts := h.manager.ListAlerts(severity, status)
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (h *Handlers) getAlert(c *gin.Context) {
	id := c.Param("id")
	alert, err := h.manager.GetAlert(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, alert)
}

func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.AcknowledgeAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "告警已确认"})
}

func (h *Handlers) resolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "告警已解决"})
}

// ========== 任务管理处理器 ==========

func (h *Handlers) createTask(c *gin.Context) {
	var req RemediationTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.manager.CreateTask(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) listTasks(c *gin.Context) {
	status := TaskStatus(c.Query("status"))
	tasks := h.manager.ListTasks(status)
	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) updateTaskStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status TaskStatus `json:"status" binding:"required"`
		Result string     `json:"result"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.UpdateTaskStatus(id, req.Status, req.Result); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务状态已更新"})
}

// ========== 统计和配置处理器 ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, config)
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var config EngineConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}
