// Package auditreport 提供 REST API 处理器
package auditreport

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 审计报告模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/audit 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	audit := r.Group("/audit")
	{
		// 报告管理
		audit.POST("/reports/generate", h.generateReport)
		audit.GET("/reports", h.listReports)
		audit.GET("/reports/:id", h.getReport)
		audit.DELETE("/reports/:id", h.deleteReport)

		// 发现管理
		audit.GET("/findings", h.listFindings)
		audit.PUT("/findings/:id", h.updateFinding)
		audit.PUT("/findings/:id/resolve", h.resolveFinding)

		// 合规检查
		audit.POST("/compliance/run", h.runComplianceCheck)
		audit.GET("/compliance/status", h.getComplianceStatus)
		audit.GET("/compliance/checks", h.listComplianceChecks)

		// 审计日志
		audit.GET("/events", h.queryEvents)
		audit.POST("/events/export", h.exportEvents)

		// 安全扫描
		audit.POST("/scan", h.runSecurityScan)
		audit.GET("/scan/:id", h.getScanResults)
	}
}

// ========== 报告 Handlers ==========

func (h *Handlers) generateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	report := h.manager.GenerateReport(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "generated", Data: report})
}

func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

func (h *Handlers) listReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(reports),
			"reports": reports,
		},
	})
}

func (h *Handlers) deleteReport(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteReport(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 发现 Handlers ==========

func (h *Handlers) listFindings(c *gin.Context) {
	findings := h.manager.ListFindings()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(findings),
			"findings": findings,
		},
	})
}

func (h *Handlers) updateFinding(c *gin.Context) {
	id := c.Param("id")
	var req UpdateFindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	finding, err := h.manager.UpdateFinding(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: finding})
}

func (h *Handlers) resolveFinding(c *gin.Context) {
	id := c.Param("id")
	finding, err := h.manager.ResolveFinding(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "resolved", Data: finding})
}

// ========== 合规检查 Handlers ==========

func (h *Handlers) runComplianceCheck(c *gin.Context) {
	var req RunComplianceCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	check := h.manager.RunComplianceCheck(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "completed", Data: check})
}

func (h *Handlers) getComplianceStatus(c *gin.Context) {
	status := h.manager.GetComplianceStatus()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: status})
}

func (h *Handlers) listComplianceChecks(c *gin.Context) {
	checks := h.manager.ListComplianceChecks()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(checks),
			"checks": checks,
		},
	})
}

// ========== 审计日志 Handlers ==========

func (h *Handlers) queryEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	req := QueryEventsRequest{
		UserID:   c.Query("user_id"),
		Action:   c.Query("action"),
		Resource: c.Query("resource"),
		Result:   c.Query("result"),
		Limit:    limit,
	}

	events := h.manager.QueryEvents(req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

func (h *Handlers) exportEvents(c *gin.Context) {
	var req ExportEventsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	events := h.manager.ExportEvents(req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "exported",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// ========== 安全扫描 Handlers ==========

func (h *Handlers) runSecurityScan(c *gin.Context) {
	var req RunSecurityScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	result := h.manager.RunSecurityScan(req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "scan completed", Data: result})
}

func (h *Handlers) getScanResults(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetScanResults(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}
