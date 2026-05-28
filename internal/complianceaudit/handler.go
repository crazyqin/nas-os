// Package complianceaudit 提供 REST API 处理器
package complianceaudit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 合规审计 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ca := r.Group("/compliance")
	{
		ca.GET("/dashboard", h.getDashboard)
		ca.GET("/score", h.getScore)
		ca.GET("/checks", h.listChecks)
		ca.POST("/scan", h.runScan)
		ca.POST("/scan/:standard", h.runStandardScan)
		ca.GET("/reports", h.getReports)
		ca.GET("/reports/latest", h.getLatestReport)
		ca.GET("/config", h.getConfig)
		ca.PUT("/config", h.updateConfig)
		ca.GET("/audit-logs", h.getAuditLogs)
		ca.POST("/audit-logs", h.collectAuditLog)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getDashboard 获取合规仪表盘数据
func (h *Handlers) getDashboard(c *gin.Context) {
	dashboard := h.manager.GetDashboard()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    dashboard,
	})
}

// getScore 获取合规评分
func (h *Handlers) getScore(c *gin.Context) {
	score := h.manager.GetComplianceScore()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    score,
	})
}

// listChecks 列出所有检查项
func (h *Handlers) listChecks(c *gin.Context) {
	checks := h.manager.ListChecks()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(checks),
			"checks": checks,
		},
	})
}

// RunScanRequest 扫描请求
type RunScanRequest struct {
	Standards  []ComplianceStandard `json:"standards"`
	Categories []CheckCategory      `json:"categories"`
	Forced     bool                 `json:"forced"`
}

// runScan 执行全量合规扫描
func (h *Handlers) runScan(c *gin.Context) {
	var req RunScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 无 body 时使用默认配置
		req = RunScanRequest{}
	}

	report := h.manager.RunFullScan(c.Request.Context())

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "scan completed",
		Data:    report,
	})
}

// runStandardScan 执行指定标准的扫描
func (h *Handlers) runStandardScan(c *gin.Context) {
	standard := ComplianceStandard(c.Param("standard"))

	switch standard {
	case StandardGDPR, StandardMLPS2, StandardISO27001, StandardSOC2:
		// valid
	default:
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid standard: " + string(standard),
		})
		return
	}

	report := h.manager.RunStandardScan(c.Request.Context(), standard)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "scan completed",
		Data:    report,
	})
}

// getReports 获取历史报告
func (h *Handlers) getReports(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	reports, err := h.manager.GetReports(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(reports),
			"reports": reports,
		},
	})
}

// getLatestReport 获取最新报告
func (h *Handlers) getLatestReport(c *gin.Context) {
	report := h.manager.GetLastReport()
	if report == nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "no report available, run a scan first",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// getConfig 获取当前配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg ScanConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid config: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
		Data:    h.manager.GetConfig(),
	})
}

// getAuditLogs 获取审计日志
func (h *Handlers) getAuditLogs(c *gin.Context) {
	actor := c.Query("actor")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	logs, err := h.manager.GetAuditLogs(actor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(logs),
			"logs":  logs,
		},
	})
}

// collectAuditLog 收集审计日志
func (h *Handlers) collectAuditLog(c *gin.Context) {
	var log AuditLog
	if err := c.ShouldBindJSON(&log); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid log: " + err.Error(),
		})
		return
	}

	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	if err := h.manager.CollectAuditLog(&log); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "audit log collected",
	})
}
